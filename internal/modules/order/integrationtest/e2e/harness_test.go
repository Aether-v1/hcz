// Package e2e_test 是 HCZ 跨域资金流端到端（E2E）测试。
//
// 真实业务链（非 mock 业务逻辑）：
//
//	User Wallet → OrderService.CreateOrder(Profit Guard + Global FX + Pricing Snapshot)
//	            → PaymentService.CreatePayment(UseBalance=true，事务内 wallet debit + order paid)
//	            → 订单 completed → RefundService.AdminRefundToWallet(钱包退款 + Affiliate 冲正)
//
// 每一步都用 shopspring/decimal 断言金额守恒（wallet total = available + frozen）。
//
// DB：SQLite 内存库（glebarez/sqlite，纯 Go 无 CGO）。每个测试独立 DSN，互不污染。
package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"
	categorydomain "github.com/Aether-v1/hcz/internal/modules/catalog/category/domain"
	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	productgormstore "github.com/Aether-v1/hcz/internal/modules/catalog/product/store/gormstore"
	coupondomain "github.com/Aether-v1/hcz/internal/modules/coupon/domain"
	coupongormstore "github.com/Aether-v1/hcz/internal/modules/coupon/infrastructure/gormstore"
	exchangerateapp "github.com/Aether-v1/hcz/internal/modules/exchangerate/application"
	exchangeratecontract "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	usergormstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	. "github.com/Aether-v1/hcz/internal/modules/order/application"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	refundapp "github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	paymentapp "github.com/Aether-v1/hcz/internal/modules/payment/application"
	paymentdomain "github.com/Aether-v1/hcz/internal/modules/payment/domain"
	paymentgormstore "github.com/Aether-v1/hcz/internal/modules/payment/infrastructure/gormstore"
	promotiondomain "github.com/Aether-v1/hcz/internal/modules/promotion/domain"
	promotiongormstore "github.com/Aether-v1/hcz/internal/modules/promotion/infrastructure/gormstore"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const testGuestSecret = "test-guest-credential-secret-with-32-bytes"

// ---- fake exchange rate store（settings KV 端口的内存实现）----

type fakeRateStore struct {
	mu    sync.Mutex
	state exchangeratecontract.State
}

func (f *fakeRateStore) GetState() (exchangeratecontract.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, nil
}

func (f *fakeRateStore) SaveState(s exchangeratecontract.State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = s
	return nil
}

// ---- mock queue（Enabled=true，满足 createOrder 最低门槛）----

type e2eQueue struct{}

func (q *e2eQueue) Enabled() bool                         { return true }
func (q *e2eQueue) EnqueueTimeoutCancel(_ uint, _ time.Duration) error { return nil }
func (q *e2eQueue) EnqueueStatusEmail(_ uint, _ string) error           { return nil }

// ---- mock affiliate lifecycle（下单时不绑定推广关系；佣金由退款测试直接 seed）----

type mockAffiliateLifecycle struct{}

func (m *mockAffiliateLifecycle) ResolveOrderAffiliateSnapshot(userID uint, rawCode, rawVisitorKey string) (*uint, string, error) {
	return nil, "", nil
}
func (m *mockAffiliateLifecycle) HandleOrderPaid(orderID uint) error     { return nil }
func (m *mockAffiliateLifecycle) HandleOrderCanceled(orderID uint, reason string) error { return nil }
func (m *mockAffiliateLifecycle) HandleOrderCompleted(orderID uint) error { return nil }

// ---- fixture ----

type e2eFixture struct {
	db         *gorm.DB
	orderSvc   *OrderService
	paySvc     *paymentapp.PaymentService
	refundSvc  *refundapp.Service
	walletSvc  *walletapp.Service
	settingSvc *settingsapp.Service
	rateSvc    *exchangerateapp.Service
	rateStore  *fakeRateStore
	user       *userdomain.User
	category   *categorydomain.Category
}

var fixtureCounter int

func newE2EFixture(t *testing.T) *e2eFixture {
	t.Helper()
	fixtureCounter++
	// shared-cache 内存库：允许多连接，使“事务内读 settings”不锁死（生产 PG 本就多连接）。
	// busy_timeout 避免并发写时 hard "database is locked"。
	dsn := fmt.Sprintf("file:e2e_%d_%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", fixtureCounter, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(
		&userdomain.User{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&fulfillmentdomain.Fulfillment{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&walletdomain.RechargeOrder{},
		&paymentdomain.Payment{},
		&paymentdomain.PaymentChannel{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Commission{},
		&affiliatedomain.CommissionLedger{},
		&affiliatedomain.WithdrawRequest{},
		&settingsstore.SettingRecord{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	gormdb.DB = db

	// wallet
	walletRepo := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletRepo,
		Transactions: walletRepo,
	})

	// settings
	settingSvc := settingsapp.NewService(settingsstore.New(db))

	// exchange rate（真实 Resolve 逻辑 + 内存 store）
	rateStore := &fakeRateStore{state: exchangeratecontract.State{
		Currency:      "CNY",
		AutoRate:      decimal.RequireFromString("7.18"),
		AutoFetchedAt: time.Now(),
	}}
	rateSvc := exchangerateapp.NewService(nil, rateStore, "CNY", 30*time.Minute)

	// stores
	orderStore := ordergormstore.New(db, testGuestSecret)
	userRepo := usergormstore.New(db)
	productRepo := productgormstore.NewProductStore(db)
	productSKURepo := productgormstore.NewSKUStore(db)
	paymentRepo := paymentgormstore.New(db, testGuestSecret)
	paymentChannelRepo := paymentgormstore.NewChannelStore(db)
	affiliateStore := affiliategormstore.New(db)

	// order service
	orderSvc := NewOrderService(OrderServiceOptions{
		OrderStore:       orderStore,
		UserStore:        userRepo,
		ProductStore:     productRepo,
		ProductSKUStore:  productSKURepo,
		CouponStore:      coupongormstore.New(db),
		CouponUsageStore: coupongormstore.NewUsageStore(db),
		PromotionRepo:    promotiongormstore.New(db),
		Queue:            &e2eQueue{},
		SettingService:   settingSvc,
		WalletService:    walletSvc,
		AffiliateService: &mockAffiliateLifecycle{},
		ExpireMinutes:    30,
	})
	orderSvc.SetRateResolver(rateSvc)

	// payment service（钱包支付）
	paySvc := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:     orderStore,
		ProductRepo:    productRepo,
		ProductSKURepo: productSKURepo,
		PaymentStore:   paymentRepo,
		ChannelStore:   paymentChannelRepo,
		WalletRepo:     walletRepo,
		UserStore:      userRepo,
		WalletService:  walletSvc,
		SettingService: settingSvc,
		ExpireMinutes:  30,
	})

	// refund service（钱包退款 + affiliate 冲正）
	refundSvc := refundapp.New(orderStore, userRepo, affiliateapp.NewService(affiliateStore, nil, nil, nil, nil), settingSvc, walletSvc, paymentRepo)

	user := &userdomain.User{
		Email:       fmt.Sprintf("e2e_user_%d@test.com", fixtureCounter),
		DisplayName: "e2e-user",
		Status:      constants.UserStatusActive,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	category := &categorydomain.Category{
		Slug:     fmt.Sprintf("e2e-cat-%d", fixtureCounter),
		NameJSON: jsonmap.JSON{"zh-CN": "E2E 测试分类"},
	}
	if err := db.Create(category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	return &e2eFixture{
		db: db, orderSvc: orderSvc, paySvc: paySvc, refundSvc: refundSvc,
		walletSvc: walletSvc, settingSvc: settingSvc,
		rateSvc: rateSvc, rateStore: rateStore,
		user: user, category: category,
	}
}

// ---- 配置辅助 ----

// enableProfitGuard 开启 Profit Guard V1。
func (f *e2eFixture) enableProfitGuard(t *testing.T, requireCost bool, minAmount float64, minRatePct float64) {
	t.Helper()
	_, err := f.settingSvc.UpdateProfitGuardSetting(settingsintegration.ProfitGuardSetting{
		Enabled:                  true,
		RequireCostPrice:         requireCost,
		RateSafetyBufferPercent:  1.0,
		MinimumProfitAmountCNY:   minAmount,
		MinimumProfitRatePercent: minRatePct,
	})
	if err != nil {
		t.Fatalf("update profit guard setting: %v", err)
	}
}

// setRateState 直接覆盖内存汇率状态（用于 FX stale / 无配置场景）。
func (f *e2eFixture) setRateState(state exchangeratecontract.State) {
	f.rateStore.state = state
}

// ---- 数据辅助 ----

// productSpec 描述一个商品/SKU。
type productSpec struct {
	priceCNY      string
	costCNY       string
	costExempt    bool
	manualStock   int
	fulfillType   string
}

// createProduct 创建一个 manual 履约商品 + 默认 SKU，返回 (productID, skuID)。
func (f *e2eFixture) createProduct(t *testing.T, spec productSpec) (uint, uint) {
	t.Helper()
	now := time.Now()
	if spec.fulfillType == "" {
		spec.fulfillType = constants.FulfillmentTypeManual
	}
	if spec.manualStock <= 0 {
		spec.manualStock = 1000
	}
	prod := &productdomain.Product{
		CategoryID:      f.category.ID,
		Slug:            fmt.Sprintf("e2e-prod-%d-%d", fixtureCounter, now.UnixNano()),
		TitleJSON:       jsonmap.JSON{"zh-CN": "E2E 商品"},
		PriceAmount:     money.FromDecimal(mustDec(spec.priceCNY)),
		IsActive:        true,
		FulfillmentType: spec.fulfillType,
		IsCostExempt:    spec.costExempt,
		ManualStockTotal: spec.manualStock,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := f.db.Create(prod).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	sku := &productdomain.ProductSKU{
		ProductID:        prod.ID,
		SKUCode:          productdomain.DefaultSKUCode,
		SpecValuesJSON:   jsonmap.JSON{},
		PriceAmount:      money.FromDecimal(mustDec(spec.priceCNY)),
		CostPriceAmount:  money.FromDecimal(mustDec(spec.costCNY)),
		IsActive:         true,
		ManualStockTotal: spec.manualStock,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := f.db.Create(sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}
	return prod.ID, sku.ID
}

// recharge 给用户钱包充值（USDT）。
func (f *e2eFixture) recharge(t *testing.T, userID uint, amount string) {
	t.Helper()
	if _, _, err := f.walletSvc.Recharge(walletcontract.RechargeInput{
		UserID: userID, Amount: money.FromDecimal(mustDec(amount)), Remark: "e2e recharge",
	}); err != nil {
		t.Fatalf("recharge user=%d: %v", userID, err)
	}
}

// walletTotal 返回 (available, frozen, total)，均 round 2。
func (f *e2eFixture) walletTotal(t *testing.T, userID uint) (decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	t.Helper()
	acc, err := f.walletSvc.GetAccount(userID)
	if err != nil {
		t.Fatalf("get account user=%d: %v", userID, err)
	}
	avail := acc.AvailableBalance.Decimal.Round(2)
	frozen := acc.FrozenBalance.Decimal.Round(2)
	return avail, frozen, avail.Add(frozen)
}

func (f *e2eFixture) countOrders(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&orderdomain.Order{}).
		Where("deleted_at IS NULL AND parent_id IS NULL").Count(&n).Error; err != nil {
		t.Fatalf("count parent orders: %v", err)
	}
	return n
}

// placeOrder 调 OrderService.CreateOrder（跳过风控）。
func (f *e2eFixture) placeOrder(t *testing.T, productID, skuID uint, qty int) (*orderdomain.Order, error) {
	t.Helper()
	return f.orderSvc.CreateOrder(CreateOrderInput{
		UserID:         f.user.ID,
		SkipRiskControl: true,
		Items: []CreateOrderItem{{
			ProductID: productID,
			SKUID:     skuID,
			Quantity:  qty,
		}},
	})
}

// payWithWallet 调 PaymentService 用钱包余额支付订单，原子扣款+置已支付。
func (f *e2eFixture) payWithWallet(t *testing.T, orderID uint) (*paymentapp.CreatePaymentResult, error) {
	t.Helper()
	return f.paySvc.CreatePayment(paymentapp.CreatePaymentInput{
		OrderID:   orderID,
		UseBalance: true,
		Context:   context.Background(),
	})
}

// markCompleted 直接把订单（父子）状态置为 completed（模拟履约完成）。
func (f *e2eFixture) markCompleted(t *testing.T, orderID uint) {
	t.Helper()
	now := time.Now()
	if err := f.db.Model(&orderdomain.Order{}).Where("id = ?", orderID).
		Updates(map[string]interface{}{"status": constants.OrderStatusCompleted, "paid_at": now}).Error; err != nil {
		t.Fatalf("mark order completed: %v", err)
	}
	if err := f.db.Model(&orderdomain.Order{}).Where("parent_id = ?", orderID).
		Update("status", constants.OrderStatusCompleted).Error; err != nil {
		t.Fatalf("mark child orders completed: %v", err)
	}
}

// mustDec 解析十进制字符串。
func mustDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}
