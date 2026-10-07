package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"
	cardsecretdomain "github.com/Aether-v1/hcz/internal/modules/cardsecret/domain"
	categorydomain "github.com/Aether-v1/hcz/internal/modules/catalog/category/domain"
	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	productgormstore "github.com/Aether-v1/hcz/internal/modules/catalog/product/store/gormstore"
	coupondomain "github.com/Aether-v1/hcz/internal/modules/coupon/domain"
	coupongormstore "github.com/Aether-v1/hcz/internal/modules/coupon/infrastructure/gormstore"
	exchangerateapp "github.com/Aether-v1/hcz/internal/modules/exchangerate/application"
	exchangeratecontract "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	fulfillmentapp "github.com/Aether-v1/hcz/internal/modules/fulfillment/application"
	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	fulfillmentgormstore "github.com/Aether-v1/hcz/internal/modules/fulfillment/infrastructure/gormstore"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	usergormstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	. "github.com/Aether-v1/hcz/internal/modules/order/application"
	ordercontract "github.com/Aether-v1/hcz/internal/modules/order/contract"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	refundapp "github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	paymentapp "github.com/Aether-v1/hcz/internal/modules/payment/application"
	paymentdomain "github.com/Aether-v1/hcz/internal/modules/payment/domain"
	paymentgormstore "github.com/Aether-v1/hcz/internal/modules/payment/infrastructure/gormstore"
	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"
	promotiondomain "github.com/Aether-v1/hcz/internal/modules/promotion/domain"
	promotiongormstore "github.com/Aether-v1/hcz/internal/modules/promotion/infrastructure/gormstore"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/Aether-v1/hcz/internal/config"
	"github.com/shopspring/decimal"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// pointsP1Fixture 是 P1 Order Reward 集成测试 fixture：
// 真实 Order / Points / Affiliate / Fulfillment / Refund 全链路，真实 Settings + Wallet + FX。
// 与 e2eFixture 的区别：Affiliate 与 Points 均为真实服务（用于三完成路径佣金/积分断言）。
type pointsP1Fixture struct {
	db             *gorm.DB
	orderSvc       *OrderService
	orderStore     ordercontract.Store
	pointsSvc      *pointsapp.Service
	pointsStore    *pointsgormstore.Store
	fulfillmentSvc *fulfillmentapp.Service
	refundSvc      *refundapp.Service
	paySvc         *paymentapp.PaymentService
	walletSvc      *walletapp.Service
	settingSvc     *settingsapp.Service
	rateStore      *fakeRateStore
	user           *userdomain.User
	inviter        *userdomain.User
	category       *categorydomain.Category
}

var p1FixtureCounter int

// newPointsP1Fixture 使用 shared-cache 内存库（普通顺序测试）。
// 注意：内存 shared-cache 多连接下并发写会触发读锁升级死锁（busy_timeout 无效），
// 并发测试必须使用 newPointsP1ConcurrentFixture（文件型 WAL）。
func newPointsP1Fixture(t *testing.T) *pointsP1Fixture {
	t.Helper()
	p1FixtureCounter++
	dsn := fmt.Sprintf("file:p1_%d_%d?mode=memory&cache=shared&_pragma=busy_timeout(8000)", p1FixtureCounter, time.Now().UnixNano())
	return newPointsP1FixtureWithDB(t, dsn)
}

// newPointsP1ConcurrentFixture 使用文件型 WAL 数据库：
// WAL 下 writer 唯一、reader 不阻塞 writer，多连接并发写由 busy_timeout 排队（无死锁），
// 用于重复完成 / 并发退款等真并发测试（近似 PostgreSQL 行为）。
func newPointsP1ConcurrentFixture(t *testing.T) *pointsP1Fixture {
	t.Helper()
	p1FixtureCounter++
	dir := t.TempDir()
	dsn := fmt.Sprintf("file:%s/p1_%d.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(8000)&_pragma=synchronous(NORMAL)",
		filepath.ToSlash(dir), time.Now().UnixNano())
	return newPointsP1FixtureWithDB(t, dsn)
}

func newPointsP1FixtureWithDB(t *testing.T, dsn string) *pointsP1Fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	// 多连接：CreateOrder/CreatePayment 含"事务外读 settings + 事务内写"，单连接池会自锁。
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
		&cardsecretdomain.Secret{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&walletdomain.RechargeOrder{},
		&paymentdomain.Payment{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Commission{},
		&affiliatedomain.CommissionLedger{},
		&affiliatedomain.WithdrawRequest{},
		&pointsdomain.Account{},
		&pointsdomain.LedgerEntry{},
		&settingsstore.SettingRecord{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// wallet
	walletRepo := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletRepo,
		Transactions: walletRepo,
	})

	// settings
	settingSvc := settingsapp.NewService(settingsstore.New(db))

	// exchange rate
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
	pointsStore := pointsgormstore.New(db)

	// points
	pointsSvc := pointsapp.NewService(pointsapp.Options{
		Repository:   pointsStore,
		Transactions: pointsStore,
	})

	// affiliate（真实：带 settings + userRepo，用于 inviter 链佣金）
	affiliateSvc := affiliateapp.NewService(affiliateStore, userRepo, nil, nil, settingSvc)
	if _, err := settingSvc.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled:        true,
		CommissionRate: 10,
		ConfirmDays:    0,
		MinWithdrawAmount: 1,
		MaxLevel:       1,
		LevelRates: []settingsintegration.LevelRate{
			{Level: 1, Enabled: true, Rate: 10},
		},
	}); err != nil {
		t.Fatalf("update affiliate setting: %v", err)
	}

	// order service（Points + Affiliate 均真实）
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
		AffiliateService: affiliateSvc,
		PointsService:    pointsSvc,
		ExpireMinutes:    30,
	})
	orderSvc.SetRateResolver(rateSvc)

	// payment service
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

	// fulfillment service（统一完成生命周期注入）
	fulfillmentSvc := fulfillmentapp.New(fulfillmentapp.Options{
		OrderStore:            orderStore,
		FulfillmentStore:      fulfillmentgormstore.New(db),
		OrderQueue:            &e2eQueue{},
		SettingService:        settingSvc,
		DefaultEmailConfig:    config.EmailConfig{},
		ExternalIdentityStore: nil,
		OrderCompletion:       orderSvc,
	})

	// refund service（Points 冲正接入）
	refundSvc := refundapp.New(orderStore, userRepo, affiliateSvc, settingSvc, walletSvc, paymentRepo, pointsSvc)

	// users：buyer（被 inviter 推广）+ inviter（有 active profile）
	now := time.Now()
	inviter := &userdomain.User{
		Email:       fmt.Sprintf("p1_inviter_%d@test.com", p1FixtureCounter),
		DisplayName: "p1-inviter",
		Status:      constants.UserStatusActive,
	}
	if err := db.Create(inviter).Error; err != nil {
		t.Fatalf("create inviter: %v", err)
	}
	if err := db.Create(&affiliatedomain.Profile{
		UserID:        inviter.ID,
		AffiliateCode: fmt.Sprintf("P1CODE%d", p1FixtureCounter),
		Status:        constants.AffiliateProfileStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("create affiliate profile: %v", err)
	}
	user := &userdomain.User{
		Email:       fmt.Sprintf("p1_user_%d@test.com", p1FixtureCounter),
		DisplayName: "p1-user",
		Status:      constants.UserStatusActive,
		InviterID:   &inviter.ID,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	category := &categorydomain.Category{
		Slug:     fmt.Sprintf("p1-cat-%d", p1FixtureCounter),
		NameJSON: jsonmap.JSON{"zh-CN": "P1 测试分类"},
	}
	if err := db.Create(category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	// profit guard 默认开启（cost 校验）
	if _, err := settingSvc.UpdateProfitGuardSetting(settingsintegration.ProfitGuardSetting{
		Enabled:                  true,
		RequireCostPrice:         true,
		RateSafetyBufferPercent:  1.0,
		MinimumProfitAmountCNY:   0,
		MinimumProfitRatePercent: 0,
	}); err != nil {
		t.Fatalf("update profit guard: %v", err)
	}

	return &pointsP1Fixture{
		db: db, orderSvc: orderSvc, orderStore: orderStore,
		pointsSvc: pointsSvc, pointsStore: pointsStore,
		fulfillmentSvc: fulfillmentSvc, refundSvc: refundSvc, paySvc: paySvc,
		walletSvc: walletSvc, settingSvc: settingSvc, rateStore: rateStore,
		user: user, inviter: inviter, category: category,
	}
}

// ---- fixture 辅助 ----

// createRewardProduct 创建带固定积分奖励的 manual 商品，返回 (productID, skuID, childOrderID 由下单产生)。
func (f *pointsP1Fixture) createRewardProduct(t *testing.T, priceCNY string, rewardPoints int64) (uint, uint) {
	t.Helper()
	now := time.Now()
	prod := &productdomain.Product{
		CategoryID:       f.category.ID,
		Slug:             fmt.Sprintf("p1-prod-%d-%d", p1FixtureCounter, now.UnixNano()),
		TitleJSON:        jsonmap.JSON{"zh-CN": "P1 商品"},
		PriceAmount:      money.FromDecimal(mustDec(priceCNY)),
		IsActive:         true,
		FulfillmentType:  constants.FulfillmentTypeManual,
		IsCostExempt:     false,
		ManualStockTotal: 1000,
		RewardEnabled:    true,
		RewardPoints:     rewardPoints,
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
		PriceAmount:      money.FromDecimal(mustDec(priceCNY)),
		CostPriceAmount:  money.FromDecimal(mustDec(priceCNY).Mul(decimal.NewFromFloat(0.4))),
		IsActive:         true,
		ManualStockTotal: 1000,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := f.db.Create(sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}
	return prod.ID, sku.ID
}

// createAutoProduct 创建带固定积分奖励的 auto 商品 + 卡密库存。
func (f *pointsP1Fixture) createAutoProduct(t *testing.T, priceCNY string, rewardPoints int64, secretCount int) (uint, uint) {
	t.Helper()
	now := time.Now()
	prod := &productdomain.Product{
		CategoryID:       f.category.ID,
		Slug:             fmt.Sprintf("p1-auto-%d-%d", p1FixtureCounter, now.UnixNano()),
		TitleJSON:        jsonmap.JSON{"zh-CN": "P1 自动商品"},
		PriceAmount:      money.FromDecimal(mustDec(priceCNY)),
		IsActive:         true,
		FulfillmentType:  constants.FulfillmentTypeAuto,
		IsCostExempt:     false,
		RewardEnabled:    true,
		RewardPoints:     rewardPoints,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := f.db.Create(prod).Error; err != nil {
		t.Fatalf("create auto product: %v", err)
	}
	sku := &productdomain.ProductSKU{
		ProductID:        prod.ID,
		SKUCode:          productdomain.DefaultSKUCode,
		SpecValuesJSON:   jsonmap.JSON{},
		PriceAmount:      money.FromDecimal(mustDec(priceCNY)),
		CostPriceAmount:  money.FromDecimal(mustDec(priceCNY).Mul(decimal.NewFromFloat(0.4))),
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := f.db.Create(sku).Error; err != nil {
		t.Fatalf("create auto sku: %v", err)
	}
	for i := 0; i < secretCount; i++ {
		if err := f.db.Create(&cardsecretdomain.Secret{
			ProductID: prod.ID,
			SKUID:     sku.ID,
			Secret:    fmt.Sprintf("P1SEC-%d-%d-%d", prod.ID, i, time.Now().UnixNano()),
			Status:    cardsecretdomain.StatusAvailable,
			CreatedAt: now,
			UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create secret: %v", err)
		}
	}
	return prod.ID, sku.ID
}

// recharge 给用户钱包充值（USDT）。
func (f *pointsP1Fixture) recharge(t *testing.T, userID uint, amount string) {
	t.Helper()
	if _, _, err := f.walletSvc.Recharge(walletappRechargeInput(userID, amount)); err != nil {
		t.Fatalf("recharge user=%d: %v", userID, err)
	}
}

func walletappRechargeInput(userID uint, amount string) walletcontract.RechargeInput {
	return walletcontract.RechargeInput{
		UserID: userID, Amount: money.FromDecimal(mustDec(amount)), Remark: "p1 recharge",
	}
}

// placeOrder 下单（返回父订单）。
func (f *pointsP1Fixture) placeOrder(t *testing.T, productID, skuID uint, qty int) (*orderdomain.Order, error) {
	t.Helper()
	return f.orderSvc.CreateOrder(CreateOrderInput{
		UserID:          f.user.ID,
		SkipRiskControl: true,
		Items: []CreateOrderItem{{
			ProductID: productID,
			SKUID:     skuID,
			Quantity:  qty,
		}},
	})
}

// childOf 返回父订单的第一个子订单。
func (f *pointsP1Fixture) childOf(t *testing.T, parentID uint) *orderdomain.Order {
	t.Helper()
	var child orderdomain.Order
	if err := f.db.Where("parent_id = ? AND deleted_at IS NULL", parentID).Order("id ASC").First(&child).Error; err != nil {
		t.Fatalf("load child order: %v", err)
	}
	return &child
}

// payWithWallet 用钱包余额支付子订单。
func (f *pointsP1Fixture) payWithWallet(t *testing.T, orderID uint) {
	t.Helper()
	if _, err := f.paySvc.CreatePayment(paymentapp.CreatePaymentInput{
		OrderID:    orderID,
		UseBalance: true,
		Context:    context.Background(),
	}); err != nil {
		t.Fatalf("pay order=%d: %v", orderID, err)
	}
}

// getOrder 读取订单（父单维度断言用）。
func (f *pointsP1Fixture) getOrder(t *testing.T, orderID uint) *orderdomain.Order {
	t.Helper()
	order, err := f.orderStore.GetByID(orderID)
	if err != nil || order == nil {
		t.Fatalf("load order=%d: %v", orderID, err)
	}
	return order
}

// ---- 积分断言辅助 ----

// balanceOf 返回用户当前积分余额（未开户返回 0）。
func (f *pointsP1Fixture) balanceOf(t *testing.T, userID uint) int64 {
	t.Helper()
	acc, err := f.pointsSvc.GetAccount(userID)
	if err != nil {
		t.Fatalf("get account user=%d: %v", userID, err)
	}
	if acc == nil {
		return 0
	}
	return acc.Balance
}

// sumLedger 返回用户积分账本净额（不变量：balance == SUM(amount)）。
func (f *pointsP1Fixture) sumLedger(t *testing.T, userID uint) int64 {
	t.Helper()
	var sum int64
	if err := f.db.Model(&pointsdomain.LedgerEntry{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(amount),0)").
		Scan(&sum).Error; err != nil {
		t.Fatalf("sum ledger: %v", err)
	}
	return sum
}

// ledgerByOrder 返回订单关联的积分流水（按 action_type 过滤）。
func (f *pointsP1Fixture) ledgerByOrder(t *testing.T, orderID uint, actionType string) []pointsdomain.LedgerEntry {
	t.Helper()
	var entries []pointsdomain.LedgerEntry
	if err := f.db.Where("order_id = ? AND action_type = ?", orderID, actionType).
		Order("id ASC").Find(&entries).Error; err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	return entries
}

// rewardOfOrder 返回订单 ORDER_REWARD 净额（0 表示未奖励）。
func (f *pointsP1Fixture) rewardOfOrder(t *testing.T, orderID uint) int64 {
	t.Helper()
	var sum int64
	if err := f.db.Model(&pointsdomain.LedgerEntry{}).
		Where("order_id = ? AND action_type = ?", orderID, pointscontract.ActionOrderReward).
		Select("COALESCE(SUM(amount),0)").
		Scan(&sum).Error; err != nil {
		t.Fatalf("sum order reward: %v", err)
	}
	return sum
}

// reversedOfOrder 返回订单 ORDER_REWARD_REVERSAL 绝对值累计。
func (f *pointsP1Fixture) reversedOfOrder(t *testing.T, orderID uint) int64 {
	t.Helper()
	var sum int64
	if err := f.db.Model(&pointsdomain.LedgerEntry{}).
		Where("order_id = ? AND action_type = ?", orderID, pointscontract.ActionOrderRewardReversal).
		Select("COALESCE(SUM(-amount),0)").
		Scan(&sum).Error; err != nil {
		t.Fatalf("sum order reversal: %v", err)
	}
	return sum
}

// countCommissions 返回订单佣金条数（防漏/防双断言）。
func (f *pointsP1Fixture) countCommissions(t *testing.T, orderID uint) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&affiliatedomain.Commission{}).
		Where("order_id = ? AND deleted_at IS NULL", orderID).Count(&n).Error; err != nil {
		t.Fatalf("count commissions: %v", err)
	}
	return n
}

// assertInvariant 断言 points_accounts.balance == SUM(points_ledger.amount)。
func (f *pointsP1Fixture) assertInvariant(t *testing.T, userID uint) {
	t.Helper()
	balance := f.balanceOf(t, userID)
	sum := f.sumLedger(t, userID)
	if balance != sum {
		t.Fatalf("points invariant broken: account.balance=%d, SUM(ledger)=%d", balance, sum)
	}
}

// ===================== 用例 =====================

// TestP1_Reward_Snapshot 订单创建时快照：下单后改商品配置不影响旧订单。
func TestP1_Reward_Snapshot(t *testing.T) {
	f := newPointsP1Fixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	orderA, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order A: %v", err)
	}
	childA := f.childOf(t, orderA.ID)
	f.payWithWallet(t, orderA.ID)
	if got := f.rewardOfOrder(t, orderA.ID); got != 0 {
		t.Fatalf("early reward on order A before complete: %d", got)
	}

	// 改商品积分配置为 200
	if err := f.db.Model(&productdomain.Product{}).Where("id = ?", prodID).
		Updates(map[string]interface{}{"reward_points": int64(200)}).Error; err != nil {
		t.Fatalf("update product reward: %v", err)
	}

	// 完成 A：必须奖励快照 100，而非当前商品配置 200
	if _, err := f.orderSvc.UpdateOrderStatus(childA.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child A -> processing: %v", err)
	}
	if _, err := f.orderSvc.UpdateOrderStatus(childA.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child A -> completed: %v", err)
	}
	if got := f.rewardOfOrder(t, orderA.ID); got != 100 {
		t.Fatalf("order A reward=%d, want snapshot 100", got)
	}

	// 新订单 B 完成：奖励当前配置 200
	orderB, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order B: %v", err)
	}
	childB := f.childOf(t, orderB.ID)
	f.payWithWallet(t, orderB.ID)
	if _, err := f.orderSvc.UpdateOrderStatus(childB.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child B -> processing: %v", err)
	}
	if _, err := f.orderSvc.UpdateOrderStatus(childB.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child B -> completed: %v", err)
	}
	if got := f.rewardOfOrder(t, orderB.ID); got != 200 {
		t.Fatalf("order B reward=%d, want current config 200", got)
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_Reward_Normal_NoEarly 仅 COMPLETED 发放；流水恰 1 条。
func TestP1_Reward_Normal_NoEarly(t *testing.T) {
	f := newPointsP1Fixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	child := f.childOf(t, order.ID)
	f.payWithWallet(t, order.ID)

	if got := f.balanceOf(t, f.user.ID); got != 0 {
		t.Fatalf("balance after pay=%d, want 0", got)
	}
	if got := f.rewardOfOrder(t, order.ID); got != 0 {
		t.Fatalf("reward after pay=%d, want 0", got)
	}

	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child -> processing: %v", err)
	}
	if got := f.balanceOf(t, f.user.ID); got != 0 {
		t.Fatalf("balance after processing=%d, want 0", got)
	}

	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child -> completed: %v", err)
	}
	if got := f.balanceOf(t, f.user.ID); got != 100 {
		t.Fatalf("balance after completed=%d, want 100", got)
	}
	if entries := f.ledgerByOrder(t, order.ID, pointscontract.ActionOrderReward); len(entries) != 1 {
		t.Fatalf("ORDER_REWARD entries=%d, want exactly 1", len(entries))
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_Reward_DuplicateComplete 并发重复完成只奖励一次（行锁 + reference 唯一）。
func TestP1_Reward_DuplicateComplete(t *testing.T) {
	f := newPointsP1ConcurrentFixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	child := f.childOf(t, order.ID)
	f.payWithWallet(t, order.ID)

	// 先进入 processing（合法迁移）
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child -> processing: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted)
		}()
	}
	wg.Wait()

	if got := f.balanceOf(t, f.user.ID); got != 100 {
		t.Fatalf("balance=%d, want 100 (single reward)", got)
	}
	if entries := f.ledgerByOrder(t, order.ID, pointscontract.ActionOrderReward); len(entries) != 1 {
		t.Fatalf("ORDER_REWARD entries=%d, want exactly 1", len(entries))
	}
	if got := f.countCommissions(t, order.ID); got != 1 {
		t.Fatalf("commissions=%d, want 1", got)
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_ThreeCompletionPaths Admin / Manual / Auto 三条完成路径各发一次奖励 + 一次佣金。
func TestP1_ThreeCompletionPaths(t *testing.T) {
	f := newPointsP1Fixture(t)
	f.recharge(t, f.user.ID, "600")

	type pathResult struct {
		orderID    uint
		rewardPath string
	}
	results := make([]pathResult, 0, 3)

	// A. Admin 状态更新
	{
		prodID, skuID := f.createRewardProduct(t, "100", 100)
		order, err := f.placeOrder(t, prodID, skuID, 1)
		if err != nil {
			t.Fatalf("admin path place: %v", err)
		}
		child := f.childOf(t, order.ID)
		f.payWithWallet(t, order.ID)
		if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
			t.Fatalf("admin path -> processing: %v", err)
		}
		if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted); err != nil {
			t.Fatalf("admin path -> completed: %v", err)
		}
		results = append(results, pathResult{orderID: order.ID, rewardPath: "admin"})
	}

	// B. Manual Fulfillment
	{
		prodID, skuID := f.createRewardProduct(t, "100", 100)
		order, err := f.placeOrder(t, prodID, skuID, 1)
		if err != nil {
			t.Fatalf("manual path place: %v", err)
		}
		child := f.childOf(t, order.ID)
		f.payWithWallet(t, order.ID)
		if _, err := f.fulfillmentSvc.CreateManual(fulfillmentapp.CreateManualInput{
			OrderID: child.ID, AdminID: 1, Payload: "manual ok",
		}); err != nil {
			t.Fatalf("manual fulfillment: %v", err)
		}
		results = append(results, pathResult{orderID: order.ID, rewardPath: "manual"})
	}

	// C. Auto Fulfillment
	{
		prodID, skuID := f.createAutoProduct(t, "100", 100, 2)
		order, err := f.placeOrder(t, prodID, skuID, 1)
		if err != nil {
			t.Fatalf("auto path place: %v", err)
		}
		child := f.childOf(t, order.ID)
		f.payWithWallet(t, order.ID)
		if _, err := f.fulfillmentSvc.CreateAuto(child.ID); err != nil {
			t.Fatalf("auto fulfillment: %v", err)
		}
		results = append(results, pathResult{orderID: order.ID, rewardPath: "auto"})
	}

	for _, r := range results {
		if got := f.rewardOfOrder(t, r.orderID); got != 100 {
			t.Fatalf("path=%s order reward=%d, want 100", r.rewardPath, got)
		}
		if got := f.countCommissions(t, r.orderID); got != 1 {
			t.Fatalf("path=%s commissions=%d, want exactly 1", r.rewardPath, got)
		}
	}
	if got := f.balanceOf(t, f.user.ID); got != 300 {
		t.Fatalf("total balance=%d, want 300", got)
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_Refund_Full 全额退款：+100 ORDER_REWARD，-100 ORDER_REWARD_REVERSAL，净 0。
func TestP1_Refund_Full(t *testing.T) {
	f := newPointsP1Fixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	child := f.childOf(t, order.ID)
	f.payWithWallet(t, order.ID)
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child -> processing: %v", err)
	}
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child -> completed: %v", err)
	}
	if got := f.balanceOf(t, f.user.ID); got != 100 {
		t.Fatalf("balance before refund=%d, want 100", got)
	}

	// 全额退款（父单维度，金额 = WalletPaidAmount USDT）
	parent := f.getOrder(t, order.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}
	if _, _, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID, Amount: parent.WalletPaidAmount,
	}); err != nil {
		t.Fatalf("full refund: %v", err)
	}

	if got := f.rewardOfOrder(t, order.ID); got != 100 {
		t.Fatalf("reward after refund=%d, want 100 (append-only)", got)
	}
	if got := f.reversedOfOrder(t, order.ID); got != 100 {
		t.Fatalf("reversed=%d, want 100", got)
	}
	if got := f.balanceOf(t, f.user.ID); got != 0 {
		t.Fatalf("balance after full refund=%d, want 0", got)
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_Refund_Partial 部分退款按比例冲正（floor 累计算法）。
func TestP1_Refund_Partial(t *testing.T) {
	f := newPointsP1Fixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	child := f.childOf(t, order.ID)
	f.payWithWallet(t, order.ID)
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child -> processing: %v", err)
	}
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child -> completed: %v", err)
	}

	parent := f.getOrder(t, order.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}
	paidBase := parent.WalletPaidAmount.Decimal
	refund30 := money.FromDecimal(paidBase.Mul(decimal.NewFromFloat(0.30)).Round(2))
	if _, _, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID, Amount: refund30,
	}); err != nil {
		t.Fatalf("partial refund 30%%: %v", err)
	}

	// target = floor(100 × cumulative / paid)，USD 下比例一致。
	expect := decimal.NewFromInt(100).Mul(paidBase.Mul(decimal.NewFromFloat(0.30)).Round(2)).Div(paidBase).Floor().IntPart()
	if got := f.reversedOfOrder(t, order.ID); got != expect {
		t.Fatalf("reversed=%d, want floor=%d", got, expect)
	}
	if got := f.balanceOf(t, f.user.ID); got != 100-expect {
		t.Fatalf("balance=%d, want %d", got, 100-expect)
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_Refund_MultiPartial 多次部分退款 30/30/40：累计 floor 精确，最终 100。
func TestP1_Refund_MultiPartial(t *testing.T) {
	f := newPointsP1Fixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	child := f.childOf(t, order.ID)
	f.payWithWallet(t, order.ID)
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child -> processing: %v", err)
	}
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child -> completed: %v", err)
	}

	parent := f.getOrder(t, order.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}
	paidBase := parent.WalletPaidAmount.Decimal

	steps := []float64{0.30, 0.30, 0.40}
	var cumulative decimal.Decimal
	for i, step := range steps {
		stepAmount := paidBase.Mul(decimal.NewFromFloat(step)).Round(2)
		if i == len(steps)-1 {
			// 最后一笔补齐到全额，保证累计退款 = paidBase（确定性满额）。
			stepAmount = paidBase.Sub(cumulative).Round(2)
		}
		cumulative = cumulative.Add(stepAmount).Round(2)
		if _, _, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
			OrderID: order.ID, Amount: money.FromDecimal(stepAmount),
		}); err != nil {
			t.Fatalf("refund step %.0f%%: %v", step*100, err)
		}
		expect := decimal.NewFromInt(100).Mul(cumulative).Div(paidBase).Floor().IntPart()
		if got := f.reversedOfOrder(t, order.ID); got != expect {
			t.Fatalf("step %d cumulative=%s reversed=%d, want floor=%d", i+1, cumulative, got, expect)
		}
	}
	if got := f.reversedOfOrder(t, order.ID); got != 100 {
		t.Fatalf("final reversed=%d, want exactly 100", got)
	}
	if got := f.balanceOf(t, f.user.ID); got != 0 {
		t.Fatalf("balance after full refund=%d, want 0", got)
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_Refund_NegativeBalance 积分已消费后全额退款：允许负余额。
func TestP1_Refund_NegativeBalance(t *testing.T) {
	f := newPointsP1Fixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	child := f.childOf(t, order.ID)
	f.payWithWallet(t, order.ID)
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child -> processing: %v", err)
	}
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child -> completed: %v", err)
	}
	if got := f.balanceOf(t, f.user.ID); got != 100 {
		t.Fatalf("balance=%d, want 100", got)
	}

	// 用户消费 80（Admin 扣减模拟用户已花掉）
	if _, _, err := f.pointsSvc.AdminAdjust(pointscontract.AdjustInput{
		UserID: f.user.ID, OperatorAdminID: 1, Operation: "subtract", Amount: 80,
		Reason: "p1 consumed", Reference: fmt.Sprintf("admin_adjust:p1_neg_%d", order.ID),
	}); err != nil {
		t.Fatalf("admin adjust -80: %v", err)
	}
	if got := f.balanceOf(t, f.user.ID); got != 20 {
		t.Fatalf("balance after spend=%d, want 20", got)
	}

	parent := f.getOrder(t, order.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}
	if _, _, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID, Amount: parent.WalletPaidAmount,
	}); err != nil {
		t.Fatalf("full refund: %v", err)
	}
	if got := f.balanceOf(t, f.user.ID); got != -80 {
		t.Fatalf("balance after refund=%d, want -80 (negative allowed)", got)
	}
	f.assertInvariant(t, f.user.ID)
}

// TestP1_Refund_Concurrent 并发退款：
// (A) 并发多笔部分退款 → 每笔成功退款各自产生一次冲正，累计冲正 = floor(100 × 累计退款 / 实付)，不超额；
// (B) 同一退款事件（同 refund_record / reference）并发重放 → reference 唯一索引保证只冲正一次。
func TestP1_Refund_Concurrent(t *testing.T) {
	f := newPointsP1ConcurrentFixture(t)
	f.recharge(t, f.user.ID, "200")

	prodID, skuID := f.createRewardProduct(t, "100", 100)
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	child := f.childOf(t, order.ID)
	f.payWithWallet(t, order.ID)
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusProcessing); err != nil {
		t.Fatalf("child -> processing: %v", err)
	}
	if _, err := f.orderSvc.UpdateOrderStatus(child.ID, constants.OrderStatusCompleted); err != nil {
		t.Fatalf("child -> completed: %v", err)
	}

	parent := f.getOrder(t, order.ID)
	paidBase := parent.WalletPaidAmount.Decimal
	refund10 := money.FromDecimal(paidBase.Mul(decimal.NewFromFloat(0.10)).Round(2))

	// (A) 10 笔并发部分退款（各 10%）：成功笔数由可退额决定（串行行锁），
	// 关键断言是"每笔成功退款产生一次冲正且累计精确"，而非固定成功数。
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, _, errs[idx] = f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
				OrderID: order.ID, Amount: refund10,
			})
		}(i)
	}
	wg.Wait()

	success := 0
	for _, e := range errs {
		if e == nil {
			success++
		}
	}
	if success < 1 || success > 10 {
		t.Fatalf("successful refunds=%d, want in [1,10]", success)
	}

	parentAfter := f.getOrder(t, order.ID)
	cumulative := parentAfter.RefundedAmount.Decimal
	if cumulative.GreaterThan(paidBase.Round(2)) {
		t.Fatalf("cumulative refunded=%s exceeds paid base=%s", cumulative, paidBase)
	}
	expect := decimal.NewFromInt(100).Mul(cumulative).Div(paidBase).Floor().IntPart()
	if got := f.reversedOfOrder(t, order.ID); got != expect {
		t.Fatalf("cumulative reversed=%d, want floor(100×%s/%s)=%d", got, cumulative, paidBase, expect)
	}
	// 每笔成功退款产生且仅产生一条冲正（reference=points:order_refund:{record_id} 唯一）。
	reversalEntries := f.ledgerByOrder(t, order.ID, pointscontract.ActionOrderRewardReversal)
	if len(reversalEntries) != success {
		t.Fatalf("reversal entries=%d, want %d (one per successful refund)", len(reversalEntries), success)
	}
	f.assertInvariant(t, f.user.ID)

	// (B) 同一退款事件并发重放：直接用同 reference 并发冲正，只允许一条入账。
	var wg2 sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			_ = f.pointsStore.WithinTransaction(func(tx pointscontract.Transaction) error {
				return f.pointsSvc.ReverseOrderReward(tx, pointscontract.OrderReversalInput{
					UserID:         f.user.ID,
					OrderID:        order.ID,
					RefundRecordID: 99999, // 模拟同一退款事件的重放（reference 一致）
					Amount:         5,
					Reason:         "p1_concurrent_replay",
					Reference:      pointscontract.OrderRefundReversalReference(99999),
				})
			})
		}()
	}
	wg2.Wait()
	replayed := f.ledgerByOrder(t, order.ID, pointscontract.ActionOrderRewardReversal)
	replayCount := 0
	for _, e := range replayed {
		if e.Reference == pointscontract.OrderRefundReversalReference(99999) {
			replayCount++
		}
	}
	if replayCount != 1 {
		t.Fatalf("same-event replay entries=%d, want exactly 1 (reference unique)", replayCount)
	}
	f.assertInvariant(t, f.user.ID)
}
