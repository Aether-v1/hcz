//go:build integration
// +build integration

// admin_adjust_pg_concurrency_test.go — Wallet Adjust 幂等在真实 PostgreSQL 上的并发验证。
//
// 运行（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=postgres password=postgres dbname=hcz_test sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestPGAdminAdjustIdempotency -v -count=1 -timeout 120s ./internal/modules/wallet/integrationtest/
//
// 验证矩阵：
//  1. 同一 Idempotency-Key、同一 payload，10 goroutine 并发 →
//     wallet movement = 1，balance 只变化一次（不得 10 次）。
//  2. 同一 Idempotency-Key、不同 payload → ErrIdempotencyConflict。
//
// 防线是 wallet_transactions.reference 唯一索引 + changeBalance 行锁 SELECT ... FOR UPDATE，
// SQLite 无法复现该并发语义，故仅在 integration tag + 真实 PG 下运行。
package integrationtest

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func openPGWalletDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip pg wallet concurrency test: TEST_POSTGRES_DSN is empty")
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
	sqlDB.SetMaxOpenConns(20)
	db.Exec("SET lock_timeout = '5s'")

	models := []interface{}{
		&walletdomain.Account{},
		&walletdomain.Transaction{},
	}
	if err := db.Migrator().DropTable(models...); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestPGAdminAdjustIdempotencyConcurrentSameKey(t *testing.T) {
	db := openPGWalletDB(t)
	store := walletgormstore.New(db)
	svc := walletapp.NewService(walletapp.Options{Repository: store, Transactions: store})

	const (
		userID  = uint(90001)
		adminID = uint(42)
	)
	const idemKey = "pg-idem-wallet-adjust-001"

	const workers = 10
	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	var okCount int64
	var errCount int64

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			start.Wait() // 栅栏：同时开始，最大化并发撞键窗口
			_, _, err := svc.AdminAdjustBalance(walletcontract.AdjustBalanceInput{
				UserID:          userID,
				OperatorAdminID: adminID,
				Delta:           money.FromDecimal(decimal.NewFromInt(100)),
				Currency:        "CNY",
				Remark:          fmt.Sprintf("pg concurrent adjust %d", worker),
				Reference:       idemKey,
			})
			if err != nil {
				atomic.AddInt64(&errCount, 1)
			} else {
				atomic.AddInt64(&okCount, 1)
			}
		}(i)
	}
	start.Done() // 放行
	wg.Wait()

	// 断言 1：同一 reference 只能产生 1 条流水。
	var txCount int64
	if err := db.Model(&walletdomain.Transaction{}).
		Where("reference = ?", idemKey).Count(&txCount).Error; err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if txCount != 1 {
		t.Fatalf("FAIL: wallet movement for idem key = %d, want exactly 1 (ok=%d err=%d)", txCount, okCount, errCount)
	}

	// 断言 2：余额只变化一次（+100，而非 +1000）。
	acct, err := svc.GetAccount(userID)
	if err != nil || acct == nil {
		t.Fatalf("get account: %v", err)
	}
	if !acct.AvailableBalance.Decimal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("FAIL: balance = %s, want exactly 100 (double-credit detected, ok=%d err=%d)",
			acct.AvailableBalance.Decimal.String(), okCount, errCount)
	}

	// 断言 3：审计/流水方向与金额一致（1 笔入账）。
	var rows []walletdomain.Transaction
	db.Where("reference = ?", idemKey).Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("audit: expected 1 movement, got %d", len(rows))
	}

	t.Logf("PASS: 10 并发同 key → movement=1, balance=100, ok=%d err=%d", okCount, errCount)
}

func TestPGAdminAdjustIdempotencyDifferentPayloadSameKey(t *testing.T) {
	db := openPGWalletDB(t)
	store := walletgormstore.New(db)
	svc := walletapp.NewService(walletapp.Options{Repository: store, Transactions: store})

	const (
		userID  = uint(90002)
		adminID = uint(42)
	)
	const idemKey = "pg-idem-wallet-adjust-002"

	// 第一次：+100，成功。
	if _, _, err := svc.AdminAdjustBalance(walletcontract.AdjustBalanceInput{
		UserID:          userID,
		OperatorAdminID: adminID,
		Delta:           money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Remark:          "first",
		Reference:       idemKey,
	}); err != nil {
		t.Fatalf("first adjust should succeed: %v", err)
	}

	// 同 key、不同 payload（金额 50）→ 必须 conflict。
	_, _, err := svc.AdminAdjustBalance(walletcontract.AdjustBalanceInput{
		UserID:          userID,
		OperatorAdminID: adminID,
		Delta:           money.FromDecimal(decimal.NewFromInt(50)),
		Currency:        "CNY",
		Remark:          "different payload",
		Reference:       idemKey,
	})
	if !errors.Is(err, walletcontract.ErrIdempotencyConflict) {
		t.Fatalf("FAIL: same key different payload → err=%v, want ErrIdempotencyConflict", err)
	}

	// 余额仍只有 +100。
	acct, _ := svc.GetAccount(userID)
	if !acct.AvailableBalance.Decimal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("FAIL: balance = %s, want 100 after conflict", acct.AvailableBalance.Decimal.String())
	}
	t.Log("PASS: same key different payload → conflict, balance unchanged")
}
