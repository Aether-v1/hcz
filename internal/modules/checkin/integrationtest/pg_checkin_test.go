//go:build integration
// +build integration

// pg_checkin_test.go — Daily Check-in 在真实 PostgreSQL 上的并发验证。
//
// 运行（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=postgres password=postgres dbname=hcz_test sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestPGCheckin -v -count=1 -timeout 180s ./internal/modules/checkin/integrationtest/
//
// 验证矩阵：
//  1. 20 goroutine 同一天签到 → 恰好 1 条 user_checkins、1 条 CHECKIN_REWARD、
//     balance 只增加一次（UNIQUE(user_id, checkin_date) + reference 唯一索引最终防线）。
//  2. 数据库不变量：points_accounts.balance == SUM(points_ledger.amount)。
package integrationtest

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	checkinapp "github.com/Aether-v1/hcz/internal/modules/checkin/application"
	checkindomain "github.com/Aether-v1/hcz/internal/modules/checkin/domain"
	checkingormstore "github.com/Aether-v1/hcz/internal/modules/checkin/infrastructure/gormstore"
	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func openPGCheckin(t *testing.T) (*checkinapp.Service, *gorm.DB) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip pg checkin concurrency test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: glogger.Default.LogMode(glogger.Silent)})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlDB: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	db.Exec("SET lock_timeout = '5s'")

	models := []interface{}{&checkindomain.UserCheckin{}, &pointsdomain.Account{}, &pointsdomain.LedgerEntry{}}
	if err := db.Migrator().DropTable(models...); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	store := checkingormstore.New(db)
	pointsStore := pointsgormstore.New(db)
	cfg := &mutableCheckinConfig{enabled: true, rewards: []int64{1, 2, 3, 4, 5, 6, 10}}
	clock := &mutableClock{at: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	pointsSvc := pointsapp.NewService(pointsapp.Options{Repository: pointsStore, Transactions: pointsStore})
	svc := checkinapp.NewService(checkinapp.Options{
		Repository: store,
		UnitOfWork: store,
		Config:     cfg,
		Clock:      clock,
		Points:     pointsSvc,
	})
	return svc, db
}

// TestPGCheckinConcurrentSingleRecord 20 goroutine 同一天签到：1 checkin + 1 ledger。
func TestPGCheckinConcurrentSingleRecord(t *testing.T) {
	svc, db := openPGCheckin(t)
	const userID = 1001
	const workers = 20

	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.CheckIn(userID); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent check-in: %v", err)
	}

	var checkins int64
	if err := db.Model(&checkindomain.UserCheckin{}).Where("user_id = ?", userID).Count(&checkins).Error; err != nil {
		t.Fatalf("count checkins: %v", err)
	}
	if checkins != 1 {
		t.Fatalf("UNIQUE(user_id, checkin_date) violated: want 1 checkin, got %d", checkins)
	}

	var ledgers int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).
		Where("user_id = ? AND action_type = ?", userID, pointscontract.ActionCheckinReward).
		Count(&ledgers).Error; err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if ledgers != 1 {
		t.Fatalf("reference uniqueness violated: want 1 CHECKIN_REWARD ledger, got %d", ledgers)
	}

	var account pointsdomain.Account
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	if account.Balance != 1 || account.TotalEarned != 1 {
		t.Fatalf("balance must increase exactly once: account=%+v", account)
	}

	// 数据库不变量
	var sum int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error; err != nil {
		t.Fatalf("sum: %v", err)
	}
	if account.Balance != sum {
		t.Fatalf("invariant violated: balance=%d sum=%d", account.Balance, sum)
	}
}
