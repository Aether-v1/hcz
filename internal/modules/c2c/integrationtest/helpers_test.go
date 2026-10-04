// Package integrationtest 提供 C2C 模块的全量集成测试 fixture：
// SQLite 内存数据库 + wallet/c2c store + mocks + c2c service。
// 每个 test 使用独立 DSN，互不污染。
package integrationtest

import (
	"fmt"
	"testing"
	"time"

	c2capp "github.com/Aether-v1/hcz/internal/modules/c2c/application"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	c2cgormstore "github.com/Aether-v1/hcz/internal/modules/c2c/infrastructure/gormstore"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

// 静默 gorm 日志，避免 record-not-found 噪声。
var silentLogger = glogger.Default.LogMode(glogger.Silent)

// ---- mocks ----

// mockC2CConfig 返回固定的 C2C 设置（关闭新用户冷却，便于测试）。
type mockC2CConfig struct {
	cfg settingsintegration.C2CSetting
}

func (m *mockC2CConfig) GetC2CConfig() (settingsintegration.C2CSetting, error) {
	return m.cfg, nil
}

// mockUserReader 内存用户表，同时实现 UserAdminStore（Update 写回 map）。
type mockUserReader struct {
	users map[uint]*userdomain.User
}

func (m *mockUserReader) GetByID(userID uint) (*userdomain.User, error) {
	if u, ok := m.users[userID]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *mockUserReader) Update(u *userdomain.User) error {
	if u == nil {
		return nil
	}
	m.users[u.ID] = u
	return nil
}

// mockNotifier 记录所有入队事件类型。
type mockNotifier struct {
	events []string
}

func (m *mockNotifier) Enqueue(input notificationcontract.EnqueueInput) error {
	m.events = append(m.events, input.EventType)
	return nil
}

// mockScheduler 空实现（不真的异步排期）。
type mockScheduler struct{}

func (m *mockScheduler) EnqueueC2CTradeExpire(tradeID uint, delay time.Duration) error {
	return nil
}

// mockAudit 空实现（不写审计）。
type mockAudit struct{}

func (m *mockAudit) WriteC2CArbitration(entry c2ccontract.ArbitrationAuditEntry) error {
	return nil
}

// ---- fixture ----

type fixture struct {
	db        *gorm.DB
	walletDB  *walletgormstore.Store
	c2cDB     *c2cgormstore.Store
	walletSvc *walletapp.Service
	svc       *c2capp.Service
	config    *mockC2CConfig
	notifier  *mockNotifier
	users     *mockUserReader
	scheduler *mockScheduler
	audit     *mockAudit
}

var dbCounter int

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dbCounter++
	dsn := fmt.Sprintf("file:c2c%d?mode=memory&cache=shared", dbCounter)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: silentLogger})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
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
		t.Fatalf("automigrate: %v", err)
	}

	walletDB := walletgormstore.New(db)
	c2cDB := c2cgormstore.New(db, walletDB)

	// wallet service
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletDB,
		Transactions: walletDB,
	})

	// config: 关闭新用户冷却，放开限额
	cfg := settingsintegration.C2CSetting{
		Enabled:              true,
		TradeTimeoutMinutes:  30,
		NewUserCooldownHours: 0,
		MinTradeUSDT:         0,
		MaxTradeUSDT:         0,
		DailyTradeLimitUSDT:  0,
		MaxCancelCount:       5,
		FeeRate:              0,
	}
	config := &mockC2CConfig{cfg: cfg}
	notifier := &mockNotifier{}
	users := &mockUserReader{users: map[uint]*userdomain.User{}}
	scheduler := &mockScheduler{}
	audit := &mockAudit{}

	svc := c2capp.NewService(c2capp.Options{
		Repository:    c2cDB,
		UnitOfWork:    c2cDB,
		WalletService: walletSvc,
		Users:         users,
		UserAdmin:     users,
		Config:        config,
		Scheduler:     scheduler,
		Notifier:      notifier,
		Audit:         audit,
	})

	return &fixture{
		db:        db,
		walletDB:  walletDB,
		c2cDB:     c2cDB,
		walletSvc: walletSvc,
		svc:       svc,
		config:    config,
		notifier:  notifier,
		users:     users,
		scheduler: scheduler,
		audit:     audit,
	}
}

// ---- 辅助方法 ----

// mustDec 解析十进制字符串，失败 panic（测试用）。
func mustDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// setBalance 创建/重置用户钱包账户可用余额。
func (f *fixture) setBalance(t *testing.T, userID uint, amount string) {
	t.Helper()
	acc := &walletdomain.Account{
		UserID:           userID,
		AvailableBalance: money.FromDecimal(mustDec(amount)),
		FrozenBalance:    money.FromDecimal(decimal.Zero),
	}
	if err := f.walletDB.CreateAccount(acc); err != nil {
		t.Fatalf("create account user=%d: %v", userID, err)
	}
}

// getAccount 读取钱包账户（不存在返回零值）。
func (f *fixture) getAccount(t *testing.T, userID uint) *walletdomain.Account {
	t.Helper()
	acc, err := f.walletDB.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("get account user=%d: %v", userID, err)
	}
	if acc == nil {
		return &walletdomain.Account{
			AvailableBalance: money.FromDecimal(decimal.Zero),
			FrozenBalance:    money.FromDecimal(decimal.Zero),
		}
	}
	return acc
}

func (f *fixture) getAvailable(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	return f.getAccount(t, userID).AvailableBalance.Decimal.Round(2)
}

func (f *fixture) getFrozen(t *testing.T, userID uint) decimal.Decimal {
	t.Helper()
	return f.getAccount(t, userID).FrozenBalance.Decimal.Round(2)
}

// createUser 登记一个已激活、已启用 TOTP、未封禁的用户。
// opts 为可选覆盖字段（如 C2CBanned、CreatedAt）。
func (f *fixture) createUser(t *testing.T, userID uint, opts ...func(*userdomain.User)) {
	t.Helper()
	now := time.Now()
	u := &userdomain.User{
		ID:            userID,
		Status:        "active",
		TOTPEnabledAt: &now,
		CreatedAt:     now.Add(-48 * time.Hour),
	}
	for _, opt := range opts {
		opt(u)
	}
	f.users.users[userID] = u
}

// createPaymentMethod 为用户创建一个启用的支付方式。
func (f *fixture) createPaymentMethod(t *testing.T, userID uint) *c2cdomain.PaymentMethod {
	t.Helper()
	pm := &c2cdomain.PaymentMethod{
		UserID:            userID,
		Type:              "bank_card",
		AccountName:       "Test",
		AccountIdentifier: "6222020000000000",
		Enabled:           true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := f.c2cDB.CreatePaymentMethod(pm); err != nil {
		t.Fatalf("create payment method user=%d: %v", userID, err)
	}
	return pm
}

// createListing 为卖家创建一个 active 挂单。
// 调用前需已 setBalance(sellerID, >= totalUSDT) 和 createPaymentMethod(sellerID)。
func (f *fixture) createListing(t *testing.T, sellerID uint, totalUSDT string) *c2cdomain.Listing {
	t.Helper()
	l, err := f.svc.CreateListing(c2ccontract.CreateListingInput{
		UserID:        sellerID,
		FiatCurrency:  "CNY",
		Price:         money.FromDecimal(mustDec("7.00")),
		MinFiatAmount: money.FromDecimal(mustDec("1.00")),
		MaxFiatAmount: money.FromDecimal(mustDec("100000.00")),
		TotalUSDT:     money.FromDecimal(mustDec(totalUSDT)),
		Terms:         "test",
	})
	if err != nil {
		t.Fatalf("create listing seller=%d total=%s: %v", sellerID, totalUSDT, err)
	}
	return l
}

// createTrade 买家发起一笔交易（幂等键自动生成以避免冲突）。
func (f *fixture) createTrade(t *testing.T, buyerID, listingID uint, amountUSDT string) *c2cdomain.Trade {
	t.Helper()
	key := fmt.Sprintf("idem-%d-%d-%s", buyerID, listingID, amountUSDT)
	tr, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID:    buyerID,
		ListingID:      listingID,
		USDTAmount:     money.FromDecimal(mustDec(amountUSDT)),
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("create trade buyer=%d listing=%d amount=%s: %v", buyerID, listingID, amountUSDT, err)
	}
	return tr
}

// countLedger 统计某用户某类型的 ledger 数量。
func (f *fixture) countLedger(t *testing.T, userID uint, txnType string) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&walletdomain.Transaction{}).
		Where("user_id = ? AND type = ?", userID, txnType).
		Count(&n).Error; err != nil {
		t.Fatalf("count ledger user=%d type=%s: %v", userID, txnType, err)
	}
	return n
}

// listLedgers 列出某用户某类型的 ledger（按 id 升序）。
func (f *fixture) listLedgers(t *testing.T, userID uint, txnType string) []walletdomain.Transaction {
	t.Helper()
	var rows []walletdomain.Transaction
	if err := f.db.Model(&walletdomain.Transaction{}).
		Where("user_id = ? AND type = ?", userID, txnType).
		Order("id asc").
		Find(&rows).Error; err != nil {
		t.Fatalf("list ledger user=%d type=%s: %v", userID, txnType, err)
	}
	return rows
}

// getListing 从 DB 直接读取挂单（断言 available_usdt）。
func (f *fixture) getListing(t *testing.T, id uint) *c2cdomain.Listing {
	t.Helper()
	l, err := f.c2cDB.GetListingByID(id)
	if err != nil {
		t.Fatalf("get listing id=%d: %v", id, err)
	}
	if l == nil {
		t.Fatalf("listing %d not found", id)
	}
	return l
}

// getTrade 从 DB 直接读取交易。
func (f *fixture) getTrade(t *testing.T, id uint) *c2cdomain.Trade {
	t.Helper()
	tr, err := f.c2cDB.GetTradeByID(id)
	if err != nil {
		t.Fatalf("get trade id=%d: %v", id, err)
	}
	if tr == nil {
		t.Fatalf("trade %d not found", id)
	}
	return tr
}
