package integrationtest

import (
	"strconv"
	"testing"

	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func mustMoney(s string) money.Amount {
	return money.FromDecimal(mustDec(s))
}

// openMemDB 打开一个新的 SQLite 内存数据库（每 test 独立）。
func openMemDB(t *testing.T, counter int) *gorm.DB {
	t.Helper()
	dsn := sqliteMemDSN(counter)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: glogger.Default.LogMode(glogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func sqliteMemDSN(counter int) string {
	return sqliteMemDSNPrefix + strconv.Itoa(counter) + "?mode=memory&cache=shared"
}

const sqliteMemDSNPrefix = "file:mig"

// TestMigration_FreshInstall 验证全新 AutoMigrate 创建所有 5 张 c2c 表 + wallet 表 + user 表。
func TestMigration_FreshInstall(t *testing.T) {
	db := openMemDB(t, nextMigCounter())
	models := []interface{}{
		&userdomain.User{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&c2cdomain.PaymentMethod{},
		&c2cdomain.Listing{},
		&c2cdomain.Trade{},
		&c2cdomain.Dispute{},
		&c2cdomain.RiskSignal{},
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("automigrate fresh: %v", err)
	}

	// 验证 5 张 c2c 表存在
	wantTables := []string{
		"users",
		"wallet_accounts",
		"wallet_transactions",
		"c2c_payment_methods",
		"c2c_listings",
		"c2c_trades",
		"c2c_disputes",
		"c2c_risk_signals",
	}
	for _, tbl := range wantTables {
		if !db.Migrator().HasTable(tbl) {
			t.Fatalf("expected table %s to exist", tbl)
		}
	}

	// 验证 users.c2c_banned 列存在（GORM 列名为 c2_c_banned）
	if !db.Migrator().HasColumn(&userdomain.User{}, "c2_c_banned") {
		t.Fatal("users.c2_c_banned column missing")
	}
	// 验证 c2c_listings.available_usdt 列存在
	if !db.Migrator().HasColumn(&c2cdomain.Listing{}, "available_usdt") {
		t.Fatal("c2c_listings.available_usdt column missing")
	}
	// 验证 c2c_trades.idempotency_key 列存在
	if !db.Migrator().HasColumn(&c2cdomain.Trade{}, "idempotency_key") {
		t.Fatal("c2c_trades.idempotency_key column missing")
	}
}

// TestMigration_ExistingDBUpgrade 验证旧 users 表（无 c2c_banned）升级后自动加列且旧数据保留。
func TestMigration_ExistingDBUpgrade(t *testing.T) {
	db := openMemDB(t, nextMigCounter())

	// 1. 建一个"旧" users 表：不含 c2c_banned 列
	if err := db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		password_hash TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy users: %v", err)
	}
	// 插入一条旧数据
	if err := db.Exec(`INSERT INTO users (email, password_hash, status, created_at, updated_at)
		VALUES ('legacy@example.com', 'hash', 'active', datetime('now'), datetime('now'))`).Error; err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}

	// 升级前：c2c_banned 列不存在
	if db.Migrator().HasColumn(&userdomain.User{}, "c2_c_banned") {
		t.Fatal("c2_c_banned should not exist before migrate")
	}

	// 2. AutoMigrate 所有模型（包含新 userdomain.User）
	if err := db.AutoMigrate(
		&userdomain.User{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&c2cdomain.PaymentMethod{},
		&c2cdomain.Listing{},
		&c2cdomain.Trade{},
		&c2cdomain.Dispute{},
		&c2cdomain.RiskSignal{},
	); err != nil {
		t.Fatalf("automigrate upgrade: %v", err)
	}

	// 3. c2c_banned 列已添加
	if !db.Migrator().HasColumn(&userdomain.User{}, "c2_c_banned") {
		t.Fatal("c2_c_banned column should be added after migrate")
	}
	// 4. 旧数据未丢失
	var count int64
	if err := db.Model(&userdomain.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("legacy user row should survive, want 1, got %d", count)
	}
	var u userdomain.User
	if err := db.First(&u, 1).Error; err != nil {
		t.Fatalf("read legacy user: %v", err)
	}
	if u.Email != "legacy@example.com" {
		t.Fatalf("legacy email want legacy@example.com, got %s", u.Email)
	}
	// 新列默认 false
	if u.C2CBanned != false {
		t.Fatalf("default c2c_banned want false, got %v", u.C2CBanned)
	}
}

// TestMigration_Idempotent 验证连续两次 AutoMigrate 不报错且表结构不变。
func TestMigration_Idempotent(t *testing.T) {
	db := openMemDB(t, nextMigCounter())
	models := []interface{}{
		&userdomain.User{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&c2cdomain.PaymentMethod{},
		&c2cdomain.Listing{},
		&c2cdomain.Trade{},
		&c2cdomain.Dispute{},
		&c2cdomain.RiskSignal{},
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("first automigrate: %v", err)
	}
	// 第二次应幂等成功
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("second automigrate (idempotent): %v", err)
	}
	// 表仍存在
	if !db.Migrator().HasTable("c2c_trades") {
		t.Fatal("c2c_trades missing after double migrate")
	}
	if !db.Migrator().HasTable("users") {
		t.Fatal("users missing after double migrate")
	}
}

// TestMigration_NonDestructive 验证 AutoMigrate 不删除已有表和数据。
func TestMigration_NonDestructive(t *testing.T) {
	db := openMemDB(t, nextMigCounter())
	models := []interface{}{
		&userdomain.User{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&c2cdomain.PaymentMethod{},
		&c2cdomain.Listing{},
		&c2cdomain.Trade{},
		&c2cdomain.Dispute{},
		&c2cdomain.RiskSignal{},
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// 写入一些数据
	pm := &c2cdomain.PaymentMethod{UserID: 1, Type: "alipay", AccountIdentifier: "x", Enabled: true}
	if err := db.Create(pm).Error; err != nil {
		t.Fatalf("create pm: %v", err)
	}
	l := &c2cdomain.Listing{
		ListingNo:     "C2CL-TEST",
		SellerUserID:  1,
		Price:         mustMoney("7.00"),
		MinFiatAmount: mustMoney("1.00"),
		MaxFiatAmount: mustMoney("100.00"),
		TotalUSDT:     mustMoney("100.00"),
		AvailableUSDT: mustMoney("100.00"),
		Status:        "active",
	}
	if err := db.Create(l).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}

	// 再跑一次 AutoMigrate
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("second automigrate: %v", err)
	}

	// 数据仍在
	var pmCount, listCount int64
	db.Model(&c2cdomain.PaymentMethod{}).Count(&pmCount)
	db.Model(&c2cdomain.Listing{}).Count(&listCount)
	if pmCount != 1 {
		t.Fatalf("payment method row should survive, want 1, got %d", pmCount)
	}
	if listCount != 1 {
		t.Fatalf("listing row should survive, want 1, got %d", listCount)
	}
}

var migCounter int

func nextMigCounter() int {
	migCounter++
	return migCounter
}
