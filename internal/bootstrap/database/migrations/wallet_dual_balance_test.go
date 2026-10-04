package migrations

import (
	"fmt"
	"testing"
	"time"

	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupWalletDualBalanceFreshDB 搭建全新安装场景：
//   - settings 表已建（迁移标记需要）
//   - wallet_accounts / wallet_transactions 通过 GORM AutoMigrate 创建（含新列）
//   - 旧 balance 列不存在
func setupWalletDualBalanceFreshDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:wallet_fresh_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&settingsstore.SettingRecord{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	prev := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = prev })
	return db
}

// setupWalletDualBalanceLegacyDB 搭建"升级前旧版本"的历史库：
//   - settings 表已建
//   - wallet_accounts 只有旧 balance 列，没有 available_balance / frozen_balance
//   - wallet_transactions 只有旧 balance_before / balance_after，没有新的 4 列
func setupWalletDualBalanceLegacyDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:wallet_legacy_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("auto migrate settings: %v", err)
	}

	// 手动创建旧 schema 的 wallet_accounts（只有 balance，没有 available/frozen）
	if err := db.Exec(`CREATE TABLE wallet_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL UNIQUE,
		balance DECIMAL(20,2) NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy wallet_accounts: %v", err)
	}

	// 手动创建旧 schema 的 wallet_transactions（只有 balance_before/after，没有新 4 列）
	if err := db.Exec(`CREATE TABLE wallet_transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		operator_admin_id INTEGER,
		order_id INTEGER,
		type VARCHAR(40) NOT NULL,
		direction VARCHAR(16) NOT NULL,
		amount DECIMAL(20,2) NOT NULL,
		balance_before DECIMAL(20,2) NOT NULL DEFAULT 0,
		balance_after DECIMAL(20,2) NOT NULL DEFAULT 0,
		currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
		reference VARCHAR(120) UNIQUE,
		remark VARCHAR(255),
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy wallet_transactions: %v", err)
	}

	prev := gormdb.DB
	gormdb.DB = db
	t.Cleanup(func() { gormdb.DB = prev })
	return db
}

func seedLegacyAccount(t *testing.T, db *gorm.DB, userID uint, balance float64) {
	t.Helper()
	if err := db.Exec(`INSERT INTO wallet_accounts (user_id, balance, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		userID, balance, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("seed legacy account user_id=%d: %v", userID, err)
	}
}

func seedLegacyTransaction(t *testing.T, db *gorm.DB, userID uint, typ, direction string, amount, balBefore, balAfter float64, ref string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO wallet_transactions
		(user_id, type, direction, amount, balance_before, balance_after, reference, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, typ, direction, amount, balBefore, balAfter, ref, time.Now(), time.Now()).Error; err != nil {
		t.Fatalf("seed legacy transaction ref=%s: %v", ref, err)
	}
}

type accountRow struct {
	ID               uint
	UserID           uint
	AvailableBalance float64
	FrozenBalance    float64
}

func loadAccountRows(t *testing.T, db *gorm.DB) []accountRow {
	t.Helper()
	var rows []accountRow
	if err := db.Raw(`SELECT id, user_id, available_balance, frozen_balance FROM wallet_accounts ORDER BY id`).
		Scan(&rows).Error; err != nil {
		t.Fatalf("load account rows: %v", err)
	}
	return rows
}

type ledgerRow struct {
	ID              uint
	BalanceBefore   float64
	BalanceAfter    float64
	AvailableBefore float64
	AvailableAfter  float64
	FrozenBefore    float64
	FrozenAfter     float64
}

func loadLedgerRows(t *testing.T, db *gorm.DB) []ledgerRow {
	t.Helper()
	var rows []ledgerRow
	if err := db.Raw(`SELECT id, balance_before, balance_after, available_before, available_after, frozen_before, frozen_after FROM wallet_transactions ORDER BY id`).
		Scan(&rows).Error; err != nil {
		t.Fatalf("load ledger rows: %v", err)
	}
	return rows
}

// 1. fresh install: AutoMigrate 建表后直接有新列，migration 函数幂等跳过。
func TestMigrateWalletDualBalance_FreshInstall(t *testing.T) {
	db := setupWalletDualBalanceFreshDB(t)

	// 插入一个用户（用 GORM model，新列默认 0）
	acct := walletdomain.Account{UserID: 1001}
	if err := db.Create(&acct).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}

	if err := migrateWalletDualBalance(); err != nil {
		t.Fatalf("migration on fresh DB: %v", err)
	}

	rows := loadAccountRows(t, db)
	if len(rows) != 1 {
		t.Fatalf("expected 1 account, got %d", len(rows))
	}
	if rows[0].AvailableBalance != 0 || rows[0].FrozenBalance != 0 {
		t.Fatalf("fresh install expected available=0 frozen=0, got avail=%.2f frozen=%.2f",
			rows[0].AvailableBalance, rows[0].FrozenBalance)
	}

	// 幂等：再跑一次
	if err := migrateWalletDualBalance(); err != nil {
		t.Fatalf("idempotent re-run on fresh DB: %v", err)
	}
}

// 2. existing DB upgrade with historical data: 旧 schema 升级，回填正确。
func TestMigrateWalletDualBalance_LegacyUpgrade(t *testing.T) {
	db := setupWalletDualBalanceLegacyDB(t)

	// fixture: user1 balance=0, user2 balance=100.50, user3 balance=9999.99
	seedLegacyAccount(t, db, 1, 0)
	seedLegacyAccount(t, db, 2, 100.50)
	seedLegacyAccount(t, db, 3, 9999.99)

	// fixture: 旧 ledger
	seedLegacyTransaction(t, db, 2, "recharge", "in", 100.50, 0, 100.50, "LEG-REF-001")
	seedLegacyTransaction(t, db, 3, "recharge", "in", 5000.00, 0, 5000.00, "LEG-REF-002")
	seedLegacyTransaction(t, db, 3, "consume", "out", 100.00, 5000.00, 4900.00, "LEG-REF-003")

	if err := migrateWalletDualBalance(); err != nil {
		t.Fatalf("migration on legacy DB: %v", err)
	}

	// 校验 accounts
	rows := loadAccountRows(t, db)
	if len(rows) != 3 {
		t.Fatalf("expected 3 accounts, got %d", len(rows))
	}
	// user1: available=0, frozen=0
	if rows[0].AvailableBalance != 0 || rows[0].FrozenBalance != 0 {
		t.Fatalf("user1 expected avail=0 frozen=0, got avail=%.2f frozen=%.2f",
			rows[0].AvailableBalance, rows[0].FrozenBalance)
	}
	// user2: available=100.50, frozen=0
	if rows[1].AvailableBalance != 100.50 || rows[1].FrozenBalance != 0 {
		t.Fatalf("user2 expected avail=100.50 frozen=0, got avail=%.2f frozen=%.2f",
			rows[1].AvailableBalance, rows[1].FrozenBalance)
	}
	// user3: available=9999.99, frozen=0
	if rows[2].AvailableBalance != 9999.99 || rows[2].FrozenBalance != 0 {
		t.Fatalf("user3 expected avail=9999.99 frozen=0, got avail=%.2f frozen=%.2f",
			rows[2].AvailableBalance, rows[2].FrozenBalance)
	}

	// 校验 ledger conservation: balance_before = available_before + frozen_before
	ledgers := loadLedgerRows(t, db)
	if len(ledgers) != 3 {
		t.Fatalf("expected 3 ledger rows, got %d", len(ledgers))
	}
	for i, l := range ledgers {
		if l.FrozenBefore != 0 || l.FrozenAfter != 0 {
			t.Fatalf("ledger row %d expected frozen=0/0, got frozen_before=%.2f frozen_after=%.2f",
				i, l.FrozenBefore, l.FrozenAfter)
		}
		if l.AvailableBefore != l.BalanceBefore {
			t.Fatalf("ledger row %d: available_before=%.2f != balance_before=%.2f",
				i, l.AvailableBefore, l.BalanceBefore)
		}
		if l.AvailableAfter != l.BalanceAfter {
			t.Fatalf("ledger row %d: available_after=%.2f != balance_after=%.2f",
				i, l.AvailableAfter, l.BalanceAfter)
		}
	}

	// 校验总资产守恒（旧 balance 列仍存在）
	var sumBal, sumAf struct {
		Total float64
	}
	db.Raw(`SELECT COALESCE(SUM(balance),0) AS total FROM wallet_accounts`).Scan(&sumBal)
	db.Raw(`SELECT COALESCE(SUM(available_balance + frozen_balance),0) AS total FROM wallet_accounts`).Scan(&sumAf)
	if sumBal.Total != sumAf.Total {
		t.Fatalf("total balance mismatch: SUM(balance)=%.2f vs SUM(avail+frz)=%.2f", sumBal.Total, sumAf.Total)
	}
}

// 3. idempotent re-run: 连续执行两次，结果一致。
func TestMigrateWalletDualBalance_IdempotentReRun(t *testing.T) {
	db := setupWalletDualBalanceLegacyDB(t)

	seedLegacyAccount(t, db, 1, 500.00)
	seedLegacyAccount(t, db, 2, 250.25)
	seedLegacyTransaction(t, db, 1, "recharge", "in", 500.00, 0, 500.00, "IDEM-001")

	if err := migrateWalletDualBalance(); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	rowsAfterFirst := loadAccountRows(t, db)
	ledgersAfterFirst := loadLedgerRows(t, db)

	if err := migrateWalletDualBalance(); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	rowsAfterSecond := loadAccountRows(t, db)
	ledgersAfterSecond := loadLedgerRows(t, db)

	if len(rowsAfterFirst) != len(rowsAfterSecond) {
		t.Fatalf("row count changed between runs: %d -> %d", len(rowsAfterFirst), len(rowsAfterSecond))
	}
	for i := range rowsAfterFirst {
		if rowsAfterFirst[i].AvailableBalance != rowsAfterSecond[i].AvailableBalance ||
			rowsAfterFirst[i].FrozenBalance != rowsAfterSecond[i].FrozenBalance {
			t.Fatalf("row %d changed after re-run: %+v -> %+v", i, rowsAfterFirst[i], rowsAfterSecond[i])
		}
	}
	for i := range ledgersAfterFirst {
		if ledgersAfterFirst[i].AvailableBefore != ledgersAfterSecond[i].AvailableBefore ||
			ledgersAfterFirst[i].AvailableAfter != ledgersAfterSecond[i].AvailableAfter {
			t.Fatalf("ledger row %d changed after re-run", i)
		}
	}
}

// 4. multi-user: 5+ 用户不同余额，验证总资产守恒。
func TestMigrateWalletDualBalance_MultiUserConservation(t *testing.T) {
	db := setupWalletDualBalanceLegacyDB(t)

	balances := []float64{0, 10.00, 99.99, 1234.56, 5000.00, 88888.88}
	for i, b := range balances {
		seedLegacyAccount(t, db, uint(i+1), b)
	}

	if err := migrateWalletDualBalance(); err != nil {
		t.Fatalf("migration: %v", err)
	}

	rows := loadAccountRows(t, db)
	if len(rows) != len(balances) {
		t.Fatalf("expected %d accounts, got %d", len(balances), len(rows))
	}

	// 每个用户 available = 旧 balance, frozen = 0
	for i, r := range rows {
		expected := balances[i]
		if r.AvailableBalance != expected {
			t.Fatalf("user %d: expected available=%.2f, got %.2f", i+1, expected, r.AvailableBalance)
		}
		if r.FrozenBalance != 0 {
			t.Fatalf("user %d: expected frozen=0, got %.2f", i+1, r.FrozenBalance)
		}
	}

	// 总资产守恒
	var sumBal, sumAf struct {
		Total float64
	}
	db.Raw(`SELECT COALESCE(SUM(balance),0) AS total FROM wallet_accounts`).Scan(&sumBal)
	db.Raw(`SELECT COALESCE(SUM(available_balance + frozen_balance),0) AS total FROM wallet_accounts`).Scan(&sumAf)
	if sumBal.Total != sumAf.Total {
		t.Fatalf("total conservation failed: SUM(balance)=%.2f vs SUM(avail+frz)=%.2f", sumBal.Total, sumAf.Total)
	}

	// 期望值 = 10.00 + 99.99 + 1234.56 + 5000.00 + 88888.88 = 95233.43
	expectedTotal := 0.0
	for _, b := range balances {
		expectedTotal += b
	}
	if sumAf.Total != expectedTotal {
		t.Fatalf("expected total=%.2f, got %.2f", expectedTotal, sumAf.Total)
	}
}
