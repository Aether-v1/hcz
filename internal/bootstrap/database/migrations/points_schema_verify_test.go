package migrations

import (
	"fmt"
	"path/filepath"
	"testing"

	categorydomain "github.com/Aether-v1/hcz/internal/modules/catalog/category/domain"
	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// pointsSystemTables 是 Points 系统 V1 的全部事实表（新增表必须同步登记）。
var pointsSystemTables = []string{
	"points_accounts",
	"points_ledger",
	"user_checkins",
	"points_products",
	"points_exchange_orders",
}

// openPointsMigrationDB 在临时文件 SQLite 上接管全局 gormdb.DB，
// 让包内 AutoMigrate() 走真实注册表（与生产启动路径完全一致）。
func openPointsMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "hcz_points_migration.db")
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open temp sqlite failed: %v", err)
	}
	prevDB := gormdb.DB
	t.Cleanup(func() {
		gormdb.DB = prevDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	gormdb.DB = db
	return db
}

// hasUniqueIndexOn 判断表上是否存在恰好覆盖 cols 的唯一索引。
func hasUniqueIndexOn(t *testing.T, db *gorm.DB, table string, cols ...string) bool {
	t.Helper()
	var indexNames []string
	if err := db.Raw(
		`SELECT name FROM pragma_index_list(?) WHERE "unique" = 1`, table,
	).Scan(&indexNames).Error; err != nil {
		t.Fatalf("pragma_index_list %s: %v", table, err)
	}
	for _, name := range indexNames {
		var rows []struct {
			Name string `gorm:"column:name"`
		}
		if err := db.Raw(`SELECT name FROM pragma_index_info(?) ORDER BY seqno`, name).Scan(&rows).Error; err != nil {
			t.Fatalf("pragma_index_info %s: %v", name, err)
		}
		if len(rows) != len(cols) {
			continue
		}
		matched := true
		for i := range rows {
			if rows[i].Name != cols[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// TestPointsFreshInstallSchema 验证全新空库执行完整 AutoMigrate 后，
// Points 系统五张表、关键唯一约束与积分快照列一次性就位（P4 §53）。
func TestPointsFreshInstallSchema(t *testing.T) {
	db := openPointsMigrationDB(t)

	if err := AutoMigrate(); err != nil {
		t.Fatalf("fresh AutoMigrate failed: %v", err)
	}

	for _, table := range pointsSystemTables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("fresh install missing table %s", table)
		}
	}

	constraints := []struct {
		table string
		cols  []string
		desc  string
	}{
		{"points_accounts", []string{"user_id"}, "账户与用户一一对应"},
		{"points_ledger", []string{"reference"}, "流水幂等最终防线"},
		{"user_checkins", []string{"user_id", "checkin_date"}, "单日单人唯一签到"},
		{"points_exchange_orders", []string{"user_id", "idempotency_key"}, "同用户同幂等键只创建一单"},
	}
	for _, c := range constraints {
		if !hasUniqueIndexOn(t, db, c.table, c.cols...) {
			t.Errorf("missing unique index on %s(%v) — %s", c.table, c.cols, c.desc)
		}
	}

	snapshots := []struct{ table, column string }{
		{"orders", "reward_enabled"},
		{"orders", "reward_points"},
		{"products", "reward_enabled"},
		{"products", "reward_points"},
	}
	for _, s := range snapshots {
		if !db.Migrator().HasColumn(s.table, s.column) {
			t.Errorf("fresh install missing snapshot column %s.%s", s.table, s.column)
		}
	}
}

// TestPointsExistingUpgradeKeepsLegacyRowsUnrewarded 验证既有库升级到 Points 后：
//   - 五张表与快照列重新就位；
//   - 历史订单/商品/钱包数据一行不丢、金额不变；
//   - 历史订单默认 reward_enabled=false / reward_points=0（不补发、不回填流水）。
func TestPointsExistingUpgradeKeepsLegacyRowsUnrewarded(t *testing.T) {
	db := openPointsMigrationDB(t)
	if err := AutoMigrate(); err != nil {
		t.Fatalf("baseline AutoMigrate failed: %v", err)
	}

	user := userdomain.User{Email: "legacy-upgrade@example.invalid", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}
	category := categorydomain.Category{
		Slug: "legacy-upgrade-cat", NameJSON: map[string]interface{}{"zh": "历史分类"}, IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("seed legacy category: %v", err)
	}
	product := productdomain.Product{
		CategoryID: category.ID, Slug: "legacy-upgrade", TitleJSON: map[string]interface{}{"zh": "历史商品"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(50)), PurchaseType: "single",
		StockDisplayMode: "available", FulfillmentType: "manual", IsActive: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("seed legacy product: %v", err)
	}
	order := orderdomain.Order{
		OrderNo:     "LEGACY-UPGRADE-1",
		UserID:      user.ID,
		Status:      "completed",
		Currency:    "CNY",
		TotalAmount: money.FromDecimal(decimal.NewFromInt(50)),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed legacy order: %v", err)
	}
	wallet := walletdomain.Account{UserID: user.ID, AvailableBalance: money.FromDecimal(decimal.NewFromInt(1234))}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatalf("seed legacy wallet: %v", err)
	}

	// 回退到"接入 Points 之前"的 schema：删掉 Points 事实表与快照列。
	for _, table := range pointsSystemTables {
		if err := db.Migrator().DropTable(table); err != nil {
			t.Fatalf("drop %s to simulate pre-points schema: %v", table, err)
		}
	}
	for _, spec := range []struct{ table, column string }{
		{"orders", "reward_points"}, {"orders", "reward_enabled"},
		{"products", "reward_points"}, {"products", "reward_enabled"},
	} {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", spec.table, spec.column)).Error; err != nil {
			t.Fatalf("drop %s.%s to simulate pre-points schema: %v", spec.table, spec.column, err)
		}
	}
	if db.Migrator().HasColumn("orders", "reward_points") {
		t.Fatalf("pre-points simulation failed: orders.reward_points still present")
	}

	if err := AutoMigrate(); err != nil {
		t.Fatalf("upgrade AutoMigrate failed: %v", err)
	}

	for _, table := range pointsSystemTables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("upgrade did not recreate table %s", table)
		}
	}
	for _, spec := range []struct{ table, column string }{
		{"orders", "reward_enabled"}, {"orders", "reward_points"},
		{"products", "reward_enabled"}, {"products", "reward_points"},
	} {
		if !db.Migrator().HasColumn(spec.table, spec.column) {
			t.Errorf("upgrade did not recreate column %s.%s", spec.table, spec.column)
		}
	}

	var upgradedOrder orderdomain.Order
	if err := db.First(&upgradedOrder, order.ID).Error; err != nil {
		t.Fatalf("legacy order lost during upgrade: %v", err)
	}
	if upgradedOrder.OrderNo != order.OrderNo || upgradedOrder.Status != order.Status {
		t.Errorf("legacy order changed: got %s/%s want %s/%s",
			upgradedOrder.OrderNo, upgradedOrder.Status, order.OrderNo, order.Status)
	}
	if !upgradedOrder.TotalAmount.Decimal.Equal(decimal.NewFromInt(50)) {
		t.Errorf("legacy order total changed: got %s want 50", upgradedOrder.TotalAmount.String())
	}
	if upgradedOrder.RewardEnabled || upgradedOrder.RewardPoints != 0 {
		t.Errorf("legacy order must default to no reward, got enabled=%v points=%d",
			upgradedOrder.RewardEnabled, upgradedOrder.RewardPoints)
	}

	var upgradedProduct productdomain.Product
	if err := db.First(&upgradedProduct, product.ID).Error; err != nil {
		t.Fatalf("legacy product lost during upgrade: %v", err)
	}
	if upgradedProduct.RewardEnabled || upgradedProduct.RewardPoints != 0 {
		t.Errorf("legacy product must default to no reward, got enabled=%v points=%d",
			upgradedProduct.RewardEnabled, upgradedProduct.RewardPoints)
	}

	var upgradedWallet walletdomain.Account
	if err := db.First(&upgradedWallet, wallet.ID).Error; err != nil {
		t.Fatalf("legacy wallet lost during upgrade: %v", err)
	}
	if !upgradedWallet.AvailableBalance.Decimal.Equal(decimal.NewFromInt(1234)) {
		t.Errorf("legacy wallet balance changed: got %s want 1234", upgradedWallet.AvailableBalance.String())
	}

	// 升级不得为历史订单补发积分：ledger 必须保持为空。
	var ledgerRows int64
	if err := db.Table("points_ledger").Count(&ledgerRows).Error; err != nil {
		t.Fatalf("count points_ledger: %v", err)
	}
	if ledgerRows != 0 {
		t.Errorf("upgrade backfilled %d ledger rows for legacy orders; expected 0", ledgerRows)
	}
	var accountRows int64
	if err := db.Table("points_accounts").Count(&accountRows).Error; err != nil {
		t.Fatalf("count points_accounts: %v", err)
	}
	if accountRows != 0 {
		t.Errorf("upgrade created %d points accounts; expected 0", accountRows)
	}
}
