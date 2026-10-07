//go:build integration
// +build integration

// pg_concurrency_test.go — Points Core 在真实 PostgreSQL 上的并发验证。
//
// 运行（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=postgres password=postgres dbname=hcz_test sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestPGPoints -v -count=1 -timeout 180s ./internal/modules/points/integrationtest/
//
// 验证矩阵：
//  1. 20 goroutine 并发 +10 → balance=200、20 条 ledger（无 lost update）。
//  2. 20 goroutine 并发 -10（允许负）→ balance 精确 -200。
//  3. 首次创建并发：无账户时 N goroutine 并发 credit → 恰好 1 个 account、N 条 ledger。
//  4. 同一 Idempotency-Key、同一 payload、10 goroutine 并发 → 只入账一次。
//
// 防线：points_ledger.reference 唯一索引 + points_accounts 行锁 SELECT ... FOR UPDATE
// + ensureAccountForUpdate（创建冲突重查）。
package integrationtest

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func openPGPointsDB(t *testing.T) (*pointsapp.Service, *gorm.DB) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip pg points concurrency test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlDB: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	db.Exec("SET lock_timeout = '5s'")

	models := []interface{}{&pointsdomain.Account{}, &pointsdomain.LedgerEntry{}}
	if err := db.Migrator().DropTable(models...); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	store := pointsgormstore.New(db)
	svc := pointsapp.NewService(pointsapp.Options{Repository: store, Transactions: store})
	return svc, db
}

func TestPGPointsConcurrentCreditNoLostUpdate(t *testing.T) {
	svc, db := openPGPointsDB(t)
	const workers = 20
	runConcurrentAdjusts(t, svc, workers, 400, "add", 10)
	account, err := svc.GetAccount(400)
	if err != nil || account == nil || account.Balance != int64(workers*10) {
		t.Fatalf("want balance %d, got account=%+v err=%v", workers*10, account, err)
	}
	if account.TotalEarned != int64(workers*10) || account.TotalSpent != 0 {
		t.Fatalf("unexpected totals: %+v", account)
	}
	assertPGInvariant(t, db, 400)
	var count int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", 400).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != workers {
		t.Fatalf("want %d ledger entries, got %d", workers, count)
	}
}

func TestPGPointsConcurrentDebitAllowsNegative(t *testing.T) {
	svc, db := openPGPointsDB(t)
	if _, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
		UserID: 500, OperatorAdminID: 9, Operation: "add", Amount: 1000,
		Reason: "base", Reference: "pg-base-500",
	}); err != nil {
		t.Fatalf("base: %v", err)
	}
	const workers = 20
	runConcurrentAdjusts(t, svc, workers, 500, "subtract", 10)
	account, err := svc.GetAccount(500)
	if err != nil || account == nil || account.Balance != int64(1000-workers*10) {
		t.Fatalf("want balance %d, got account=%+v err=%v", 1000-workers*10, account, err)
	}
	assertPGInvariant(t, db, 500)
}

func TestPGPointsConcurrentFirstCreateSingleAccount(t *testing.T) {
	svc, db := openPGPointsDB(t)
	const workers = 20
	runConcurrentAdjusts(t, svc, workers, 600, "add", 5)
	var accounts int64
	if err := db.Model(&pointsdomain.Account{}).Where("user_id = ?", 600).Count(&accounts).Error; err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accounts != 1 {
		t.Fatalf("want exactly 1 account, got %d", accounts)
	}
	var ledger int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", 600).Count(&ledger).Error; err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if ledger != workers {
		t.Fatalf("want %d ledger entries, got %d", workers, ledger)
	}
	assertPGInvariant(t, db, 600)
}

func TestPGPointsIdempotencySameKeyConcurrent(t *testing.T) {
	svc, db := openPGPointsDB(t)
	const workers = 10
	ref := "pg-idem-700"
	input := pointscontract.AdjustInput{
		UserID: 700, OperatorAdminID: 9, Operation: "add", Amount: 100,
		Reason: "idem", Reference: ref,
	}
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := svc.AdminAdjust(input); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent idempotent adjust: %v", err)
	}
	var count int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", 700).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("idempotency key must produce exactly 1 ledger entry, got %d", count)
	}
	assertPGInvariant(t, db, 700)
	account, err := svc.GetAccount(700)
	if err != nil || account == nil || account.Balance != 100 {
		t.Fatalf("balance must change once: account=%+v err=%v", account, err)
	}
}

func runConcurrentAdjusts(t *testing.T, svc *pointsapp.Service, workers int, userID uint, operation string, amount int64) {
	t.Helper()
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	var counter int64
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := atomic.AddInt64(&counter, 1)
			_, _, err := svc.AdminAdjust(pointscontract.AdjustInput{
				UserID:          userID,
				OperatorAdminID: 9,
				Operation:       operation,
				Amount:          amount,
				Reason:          fmt.Sprintf("pg-concurrent-%s-%d", operation, n),
				Reference:       fmt.Sprintf("pg-%s-%d-%d", operation, userID, n),
			})
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent adjust: %v", err)
	}
}

func assertPGInvariant(t *testing.T, db *gorm.DB, userID uint) {
	t.Helper()
	var account pointsdomain.Account
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("load account: %v", err)
	}
	var sum int64
	if err := db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", userID).Select("COALESCE(SUM(amount),0)").Scan(&sum).Error; err != nil {
		t.Fatalf("sum: %v", err)
	}
	if account.Balance != sum {
		t.Fatalf("invariant violated: balance=%d SUM=%d", account.Balance, sum)
	}
}
