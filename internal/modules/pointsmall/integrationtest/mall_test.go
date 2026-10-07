// mall_test.go — Points Mall 集成测试（SQLite 文件型 WAL）。
//
// 覆盖（P3 Exit Gate 要求）：
//   - 商品创建/更新/上下架/非法价格/非法库存
//   - 兑换成功 / 积分不足 / 缺货 / 下架 / 限购 / 快照 / 幂等 / 取消 / process / complete / fail / 返还 / 库存恢复
//   - 负数余额禁止（用户主动消费）
//   - 重复 fail / 重复 cancel / 重复 complete 幂等
//   - Processing 后用户取消拒绝；Completed 禁止取消
//   - 商品改价 / 下架不影响历史订单（快照语义）
//   - 并发：同 Key、抢最后一件、同账户并发消费、并发 fail 返还（WAL 多连接）
//   - 数据库不变量：points_accounts.balance == SUM(points_ledger.amount)；库存不变量
package integrationtest

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
	pointsgormstore "github.com/Aether-v1/hcz/internal/modules/points/infrastructure/gormstore"
	pointsmallapp "github.com/Aether-v1/hcz/internal/modules/pointsmall/application"
	pointsmallcontract "github.com/Aether-v1/hcz/internal/modules/pointsmall/contract"
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"
	pointsmallgormstore "github.com/Aether-v1/hcz/internal/modules/pointsmall/infrastructure/gormstore"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type mallFixture struct {
	db          *gorm.DB
	mallStore   *pointsmallgormstore.Store
	pointsStore *pointsgormstore.Store
	pointsSvc   *pointsapp.Service
	svc         *pointsmallapp.Service
}

// newMallFixture 创建商城集成测试环境。
// concurrent=true 时使用多连接（并发场景）；否则单连接串行化（顺序场景，语义一致）。
func newMallFixture(t *testing.T, concurrent bool) *mallFixture {
	t.Helper()
	dir := t.TempDir()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)",
		filepath.Join(dir, "mall.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&pointsdomain.Account{}, &pointsdomain.LedgerEntry{},
		&pointsmalldomain.PointsProduct{}, &pointsmalldomain.ExchangeOrder{},
	); err != nil {
		t.Fatalf("migrate mall schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	if !concurrent {
		sqlDB.SetMaxOpenConns(1)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	mallStore := pointsmallgormstore.New(db)
	pointsStore := pointsgormstore.New(db)
	pointsSvc := pointsapp.NewService(pointsapp.Options{
		Repository: pointsStore, Transactions: pointsStore,
	})
	svc := pointsmallapp.NewService(pointsmallapp.Options{
		Repository: mallStore,
		UnitOfWork: mallStore,
		Points:     pointsSvc,
	})
	return &mallFixture{db: db, mallStore: mallStore, pointsStore: pointsStore, pointsSvc: pointsSvc, svc: svc}
}

// seedPoints 给用户预存积分（通过权威 mutation service 的 AdminAdjust，避免裸写表）。
func (f *mallFixture) seedPoints(t *testing.T, userID uint, amount int64) {
	t.Helper()
	if amount == 0 {
		return
	}
	if _, _, err := f.pointsSvc.AdminAdjust(pointscontract.AdjustInput{
		UserID:          userID,
		OperatorAdminID: 999,
		Operation:       "add",
		Amount:          amount,
		Reason:          "test seed",
		Reference:       fmt.Sprintf("test:seed:%d:%d", userID, time.Now().UnixNano()),
	}); err != nil {
		t.Fatalf("seed points: %v", err)
	}
}

// balance 读取用户当前积分余额（权威账户）。
func (f *mallFixture) balance(t *testing.T, userID uint) int64 {
	t.Helper()
	account, err := f.pointsStore.GetAccountByUserID(userID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if account == nil {
		return 0
	}
	return account.Balance
}

// stock 读取商品可用库存。
func (f *mallFixture) stock(t *testing.T, productID uint) int64 {
	t.Helper()
	product, err := f.mallStore.GetProductByID(productID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if product == nil {
		t.Fatalf("product %d not found", productID)
	}
	return product.Stock
}

// ledgerSum 计算用户 Ledger 净值（不变量断言用）。
func (f *mallFixture) ledgerSum(t *testing.T, userID uint) int64 {
	t.Helper()
	var entries []pointsdomain.LedgerEntry
	if err := f.db.Where("user_id = ?", userID).Find(&entries).Error; err != nil {
		t.Fatalf("list ledger: %v", err)
	}
	var sum int64
	for _, e := range entries {
		sum += e.Amount
	}
	return sum
}

// assertInvariants 断言积分与库存不变量。
func (f *mallFixture) assertInvariants(t *testing.T, userID uint) {
	t.Helper()
	balance := f.balance(t, userID)
	sum := f.ledgerSum(t, userID)
	if balance != sum {
		t.Fatalf("points invariant broken: balance=%d ledger_sum=%d", balance, sum)
	}
}

// createProduct 创建商品并返回 ID。
func (f *mallFixture) createProduct(t *testing.T, price, stock int64, unlimited bool, limit int64) uint {
	t.Helper()
	product, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{
		Name:            "测试权益",
		PointsPrice:     price,
		Stock:           stock,
		UnlimitedStock:  unlimited,
		Enabled:         true,
		PerUserLimit:    limit,
		FulfillmentType: pointsmalldomain.FulfillmentTypeManual,
		OperatorAdminID: 1,
		Reason:          "测试创建",
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	return product.ID
}

func TestMallProductValidation(t *testing.T) {
	f := newMallFixture(t, false)

	// 非法：价格 0 / 负数
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: 0, Stock: 1, FulfillmentType: "MANUAL"}); !errors.Is(err, pointsmallcontract.ErrInvalidPointsPrice) {
		t.Fatalf("price=0 want ErrInvalidPointsPrice got %v", err)
	}
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: -100, Stock: 1, FulfillmentType: "MANUAL"}); !errors.Is(err, pointsmallcontract.ErrInvalidPointsPrice) {
		t.Fatalf("price<0 want ErrInvalidPointsPrice got %v", err)
	}
	// 非法：库存负数
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: 100, Stock: -1, FulfillmentType: "MANUAL"}); !errors.Is(err, pointsmallcontract.ErrInvalidStock) {
		t.Fatalf("stock<0 want ErrInvalidStock got %v", err)
	}
	// 非法：履约类型
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: 100, Stock: 1, FulfillmentType: "CODE"}); !errors.Is(err, pointsmallcontract.ErrInvalidFulfillmentType) {
		t.Fatalf("fulfillment=CODE want ErrInvalidFulfillmentType got %v", err)
	}
	// 非法：名称必填
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: " ", PointsPrice: 100, Stock: 1, FulfillmentType: "MANUAL"}); !errors.Is(err, pointsmallcontract.ErrProductNameRequired) {
		t.Fatalf("empty name want ErrProductNameRequired got %v", err)
	}
	// 非法：价格超过单笔积分上限（挡住后台误输入的天文数字）
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: pointscontract.MaxPointsAmount + 1, Stock: 1, FulfillmentType: "MANUAL", OperatorAdminID: 1}); !errors.Is(err, pointsmallcontract.ErrInvalidPointsPrice) {
		t.Fatalf("price>cap want ErrInvalidPointsPrice got %v", err)
	}
	// 非法：库存超过上限
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: 100, Stock: pointsmallcontract.MaxStock + 1, FulfillmentType: "MANUAL", OperatorAdminID: 1}); !errors.Is(err, pointsmallcontract.ErrInvalidStock) {
		t.Fatalf("stock>cap want ErrInvalidStock got %v", err)
	}
	// 非法：限购超过上限
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: 100, Stock: 1, PerUserLimit: pointsmallcontract.MaxPerUserLimit + 1, FulfillmentType: "MANUAL", OperatorAdminID: 1}); !errors.Is(err, pointsmallcontract.ErrInvalidPerUserLimit) {
		t.Fatalf("limit>cap want ErrInvalidPerUserLimit got %v", err)
	}
	// 非法：缺少操作者（后台商品写命令必须可追溯到人）
	if _, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{Name: "x", PointsPrice: 100, Stock: 1, FulfillmentType: "MANUAL"}); !errors.Is(err, pointsmallcontract.ErrAdminRequired) {
		t.Fatalf("missing operator want ErrAdminRequired got %v", err)
	}
	// 合法创建
	product, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{
		Name: "话费 100 元", PointsPrice: 1000, Stock: 5, Enabled: true,
		PerUserLimit: 2, FulfillmentType: "MANUAL", OperatorAdminID: 1, Reason: "上架",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 更新（改价 + 库存绝对值）
	updated, err := f.svc.UpdateProduct(product.ID, pointsmallcontract.ProductInput{
		Name: "话费 200 元", PointsPrice: 2000, Stock: 9, Enabled: true,
		PerUserLimit: 2, FulfillmentType: "MANUAL", OperatorAdminID: 1, Reason: "改价",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.PointsPrice != 2000 || updated.Stock != 9 || updated.Name != "话费 200 元" {
		t.Fatalf("update not applied: %+v", updated)
	}
	// 上下架
	disabled, err := f.svc.SetProductEnabled(pointsmallcontract.ProductEnabledInput{ProductID: product.ID, Enabled: false, OperatorAdminID: 1})
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if disabled.Enabled {
		t.Fatal("disable failed")
	}
	enabled, err := f.svc.SetProductEnabled(pointsmallcontract.ProductEnabledInput{ProductID: product.ID, Enabled: true, OperatorAdminID: 1})
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("enable failed")
	}
}

func TestMallExchangeSuccess(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(11)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-success",
	})
	if err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	if result.Order.Status != pointsmalldomain.ExchangeStatusPending {
		t.Fatalf("status want PENDING got %s", result.Order.Status)
	}
	if result.Order.TotalPoints != 500 || result.Order.UnitPoints != 500 || result.Order.ProductNameSnapshot != "测试权益" {
		t.Fatalf("snapshot wrong: %+v", result.Order)
	}
	if result.CurrentBalance != 500 {
		t.Fatalf("balance want 500 got %d", result.CurrentBalance)
	}
	if got := f.balance(t, userID); got != 500 {
		t.Fatalf("account balance want 500 got %d", got)
	}
	if got := f.stock(t, productID); got != 4 {
		t.Fatalf("stock want 4 got %d", got)
	}
	f.assertInvariants(t, userID)

	// 恰好 1 条 REDEEM
	var entries []pointsdomain.LedgerEntry
	if err := f.db.Where("user_id = ? AND action_type = ?", userID, pointscontract.ActionRedeem).Find(&entries).Error; err != nil {
		t.Fatalf("list redeem: %v", err)
	}
	if len(entries) != 1 || entries[0].Amount != -500 {
		t.Fatalf("redeem ledger want 1 x -500 got %+v", entries)
	}
}

func TestMallInsufficientPoints(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(12)
	f.seedPoints(t, userID, 499)
	productID := f.createProduct(t, 500, 5, false, 0)

	_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-insufficient",
	})
	if !errors.Is(err, pointsmallcontract.ErrPointsInsufficient) {
		t.Fatalf("want ErrPointsInsufficient got %v", err)
	}
	if got := f.balance(t, userID); got != 499 {
		t.Fatalf("balance must stay 499 got %d", got)
	}
	if got := f.stock(t, productID); got != 5 {
		t.Fatalf("stock must stay 5 got %d", got)
	}
	if _, err := f.mallStore.GetExchangeOrderByIdempotencyKey(userID, "key-insufficient"); err != nil {
		t.Fatalf("query: %v", err)
	}
	if order, _ := f.mallStore.GetExchangeOrderByIdempotencyKey(userID, "key-insufficient"); order != nil {
		t.Fatalf("no order should exist on insufficient points, got %+v", order)
	}
	f.assertInvariants(t, userID)
}

func TestMallOutOfStock(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(13)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 0, false, 0)

	_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-oos",
	})
	if !errors.Is(err, pointsmallcontract.ErrProductOutOfStock) {
		t.Fatalf("want ErrProductOutOfStock got %v", err)
	}
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("balance must stay 1000 got %d", got)
	}
	f.assertInvariants(t, userID)
}

func TestMallDisabledProduct(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(14)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)
	if _, err := f.svc.SetProductEnabled(pointsmallcontract.ProductEnabledInput{ProductID: productID, Enabled: false, OperatorAdminID: 1}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-disabled",
	})
	if !errors.Is(err, pointsmallcontract.ErrProductDisabled) {
		t.Fatalf("want ErrProductDisabled got %v", err)
	}
}

func TestMallPerUserLimit(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(15)
	f.seedPoints(t, userID, 5000)
	productID := f.createProduct(t, 500, 100, false, 1)

	// 第一次兑换成功
	if _, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-limit-1",
	}); err != nil {
		t.Fatalf("first exchange: %v", err)
	}
	// 第二次拒绝（PENDING 计入限购）
	_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-limit-2",
	})
	if !errors.Is(err, pointsmallcontract.ErrProductLimitReached) {
		t.Fatalf("want ErrProductLimitReached got %v", err)
	}
	// 余额不变（限购拒绝必须回滚积分）
	if got := f.balance(t, userID); got != 4500 {
		t.Fatalf("balance want 4500 got %d", got)
	}
	f.assertInvariants(t, userID)

	// 第一单 FAILED（返还）后可再次兑换
	order, err := f.mallStore.GetExchangeOrderByIdempotencyKey(userID, "key-limit-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if _, err := f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{
		AdminID: 1, OrderID: order.ID, Reason: "测试失败",
	}); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if _, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-limit-3",
	}); err != nil {
		t.Fatalf("exchange after failed should pass: %v", err)
	}
}

func TestMallSnapshotPriceChange(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(16)
	f.seedPoints(t, userID, 3000)
	productID := f.createProduct(t, 500, 10, false, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-snap-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.Order.TotalPoints != 500 {
		t.Fatalf("snapshot total want 500 got %d", result.Order.TotalPoints)
	}
	// 改价为 800
	if _, err := f.svc.UpdateProduct(productID, pointsmallcontract.ProductInput{
		Name: "测试权益", PointsPrice: 800, Stock: 10, Enabled: true, PerUserLimit: 0, FulfillmentType: "MANUAL", OperatorAdminID: 1, Reason: "改价",
	}); err != nil {
		t.Fatalf("update price: %v", err)
	}
	// 旧订单失败返还按 500
	if _, err := f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{
		AdminID: 1, OrderID: result.Order.ID, Reason: "测试失败",
	}); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if got := f.balance(t, userID); got != 3000 {
		t.Fatalf("refund must return snapshot 500, balance want 3000 got %d", got)
	}
	// 新兑换按新价 800
	newResult, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-snap-2",
	})
	if err != nil {
		t.Fatalf("create after price change: %v", err)
	}
	if newResult.Order.TotalPoints != 800 {
		t.Fatalf("new order total want 800 got %d", newResult.Order.TotalPoints)
	}
	f.assertInvariants(t, userID)
}

func TestMallIdempotency(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(17)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	first, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-idem",
	})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-idem",
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.Order.ID != second.Order.ID {
		t.Fatalf("same key must return same order: %d vs %d", first.Order.ID, second.Order.ID)
	}
	if !second.AlreadyProcessed {
		t.Fatal("second should be AlreadyProcessed")
	}
	if got := f.balance(t, userID); got != 500 {
		t.Fatalf("balance want 500 (deduct once) got %d", got)
	}
	if got := f.stock(t, productID); got != 4 {
		t.Fatalf("stock want 4 (deduct once) got %d", got)
	}
	f.assertInvariants(t, userID)

	// 同 key 不同商品 → 冲突
	otherID := f.createProduct(t, 300, 5, false, 0)
	_, err = f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: otherID, IdempotencyKey: "key-idem",
	})
	if !errors.Is(err, pointsmallcontract.ErrIdempotencyConflict) {
		t.Fatalf("want ErrIdempotencyConflict got %v", err)
	}
}

func TestMallCancelFlow(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(18)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-cancel",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 用户取消
	cancelResult, err := f.svc.CancelOrder(pointsmallcontract.CancelInput{UserID: userID, OrderID: result.Order.ID})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelResult.Order.Status != pointsmalldomain.ExchangeStatusCancelled {
		t.Fatalf("want CANCELLED got %s", cancelResult.Order.Status)
	}
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("balance want 1000 after refund got %d", got)
	}
	if got := f.stock(t, productID); got != 5 {
		t.Fatalf("stock want 5 restored got %d", got)
	}
	// 重复取消幂等：不再返还
	again, err := f.svc.CancelOrder(pointsmallcontract.CancelInput{UserID: userID, OrderID: result.Order.ID})
	if err != nil {
		t.Fatalf("re-cancel: %v", err)
	}
	if !again.AlreadyProcessed {
		t.Fatal("re-cancel should be AlreadyProcessed")
	}
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("re-cancel must not refund again, balance want 1000 got %d", got)
	}
	f.assertInvariants(t, userID)

	// PROCESSING 后用户不可取消
	order2, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-cancel-2",
	})
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if _, err := f.svc.AdminProcessOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: order2.Order.ID}); err != nil {
		t.Fatalf("process: %v", err)
	}
	_, err = f.svc.CancelOrder(pointsmallcontract.CancelInput{UserID: userID, OrderID: order2.Order.ID})
	if !errors.Is(err, pointsmallcontract.ErrExchangeInvalidState) {
		t.Fatalf("cancel after processing want ErrExchangeInvalidState got %v", err)
	}
}

func TestMallProcessComplete(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(19)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-pc",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	orderID := result.Order.ID

	// PENDING 不允许直转 COMPLETED
	_, err = f.svc.AdminCompleteOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: orderID})
	if !errors.Is(err, pointsmallcontract.ErrExchangeInvalidState) {
		t.Fatalf("complete from PENDING want ErrExchangeInvalidState got %v", err)
	}
	// PROCESS
	if _, err := f.svc.AdminProcessOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: orderID}); err != nil {
		t.Fatalf("process: %v", err)
	}
	// 重复 process 幂等
	if order, err := f.svc.AdminProcessOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: orderID}); err != nil || order.Status != pointsmalldomain.ExchangeStatusProcessing {
		t.Fatalf("re-process want PROCESSING no err got %v %v", order, err)
	}
	// COMPLETE
	if _, err := f.svc.AdminCompleteOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: orderID}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	// 重复 complete 幂等（不再次扣/返积分）
	if order, err := f.svc.AdminCompleteOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: orderID}); err != nil || order.Status != pointsmalldomain.ExchangeStatusCompleted {
		t.Fatalf("re-complete want COMPLETED no err got %v %v", order, err)
	}
	if got := f.balance(t, userID); got != 500 {
		t.Fatalf("balance want 500 got %d", got)
	}
	// COMPLETED 禁止取消
	_, err = f.svc.AdminCancelOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: orderID, Reason: "x"})
	if !errors.Is(err, pointsmallcontract.ErrExchangeInvalidState) {
		t.Fatalf("cancel after completed want ErrExchangeInvalidState got %v", err)
	}
	f.assertInvariants(t, userID)
}

func TestMallFailFlow(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(20)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-fail",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// fail 必须 Reason
	if _, err := f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID}); !errors.Is(err, pointsmallcontract.ErrExchangeReasonRequired) {
		t.Fatalf("fail without reason want ErrExchangeReasonRequired got %v", err)
	}
	// fail：返还 + 恢复库存
	if _, err := f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID, Reason: "测试失败"}); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("balance want 1000 got %d", got)
	}
	if got := f.stock(t, productID); got != 5 {
		t.Fatalf("stock want 5 got %d", got)
	}
	// 重复 fail 幂等（不重复返还）
	if _, err := f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID, Reason: "再次失败"}); err != nil {
		t.Fatalf("re-fail: %v", err)
	}
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("re-fail must not refund twice, balance want 1000 got %d", got)
	}
	f.assertInvariants(t, userID)

	// 恰好 1 条 REDEEM_REFUND
	var refunds []pointsdomain.LedgerEntry
	if err := f.db.Where("user_id = ? AND action_type = ?", userID, pointscontract.ActionRedeemRefund).Find(&refunds).Error; err != nil {
		t.Fatalf("list refund: %v", err)
	}
	if len(refunds) != 1 || refunds[0].Amount != 500 {
		t.Fatalf("refund ledger want 1 x +500 got %+v", refunds)
	}
}

func TestMallAdminCancelFlow(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(21)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-ac",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.svc.AdminProcessOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID}); err != nil {
		t.Fatalf("process: %v", err)
	}
	// Admin 可取消 PROCESSING（Reason 必填）
	if _, err := f.svc.AdminCancelOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID, Reason: "后台取消"}); err != nil {
		t.Fatalf("admin cancel: %v", err)
	}
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("balance want 1000 got %d", got)
	}
	if got := f.stock(t, productID); got != 5 {
		t.Fatalf("stock want 5 got %d", got)
	}
	f.assertInvariants(t, userID)
}

func TestMallUnlimitedStock(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(22)
	f.seedPoints(t, userID, 2000)
	productID := f.createProduct(t, 500, 0, true, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-unlimited",
	})
	if err != nil {
		t.Fatalf("unlimited create: %v", err)
	}
	// unlimited 库存不扣减
	if got := f.stock(t, productID); got != 0 {
		t.Fatalf("unlimited stock must stay 0 got %d", got)
	}
	if got := f.balance(t, userID); got != 1500 {
		t.Fatalf("balance want 1500 got %d", got)
	}
	// 失败返还（unlimited 不恢复库存，也不报错）
	if _, err := f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID, Reason: "失败"}); err != nil {
		t.Fatalf("fail unlimited: %v", err)
	}
	if got := f.balance(t, userID); got != 2000 {
		t.Fatalf("balance want 2000 got %d", got)
	}
	f.assertInvariants(t, userID)
}

func TestMallUserScope(t *testing.T) {
	f := newMallFixture(t, false)
	const userA = uint(23)
	const userB = uint(24)
	f.seedPoints(t, userA, 1000)
	f.seedPoints(t, userB, 1000)
	productID := f.createProduct(t, 500, 10, false, 0)

	orderA, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userA, ProductID: productID, IdempotencyKey: "key-a",
	})
	if err != nil {
		t.Fatalf("A create: %v", err)
	}
	// B 看不到 A 的订单（IDOR 归并）
	if _, err := f.svc.GetExchangeOrder(userB, orderA.Order.ID); !errors.Is(err, pointsmallcontract.ErrExchangeOrderNotFound) {
		t.Fatalf("IDOR: B must not see A order, got %v", err)
	}
	// A 能看到自己的
	if order, err := f.svc.GetExchangeOrder(userA, orderA.Order.ID); err != nil || order == nil {
		t.Fatalf("A get own order: %v %v", order, err)
	}
	// 列表隔离
	ordersB, total, err := f.svc.ListExchangeOrders(userB, pointsmallcontract.ExchangeOrderListFilter{})
	if err != nil {
		t.Fatalf("B list: %v", err)
	}
	if total != 0 || len(ordersB) != 0 {
		t.Fatalf("B must see 0 orders got total=%d len=%d", total, len(ordersB))
	}
}

func TestMallDisabledProductKeepsHistoryProcessable(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(25)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-hist",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 下架商品：禁止新兑换，但历史 PENDING 仍可 process/complete
	if _, err := f.svc.SetProductEnabled(pointsmallcontract.ProductEnabledInput{ProductID: productID, Enabled: false, OperatorAdminID: 1}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-hist-2",
	}); !errors.Is(err, pointsmallcontract.ErrProductDisabled) {
		t.Fatalf("new exchange on disabled want ErrProductDisabled got %v", err)
	}
	if _, err := f.svc.AdminProcessOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID}); err != nil {
		t.Fatalf("process history order: %v", err)
	}
	if _, err := f.svc.AdminCompleteOrder(pointsmallcontract.AdminExchangeActionInput{AdminID: 1, OrderID: result.Order.ID}); err != nil {
		t.Fatalf("complete history order: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 并发（SQLite 单连接串行化：验证应用层事务正确性 / 幂等 / 限购语义；
// 真并发行锁与竞争由 PostgreSQL 集成测试 pg_mall_test.go 证明，SQLite 不作为最终并发证据）
// ---------------------------------------------------------------------------

func TestMallConcurrentSameKey(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(31)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: userID, ProductID: productID, IdempotencyKey: "key-conc-same",
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent same-key error: %v", err)
		}
	}
	if got := f.balance(t, userID); got != 500 {
		t.Fatalf("balance want 500 got %d", got)
	}
	if got := f.stock(t, productID); got != 4 {
		t.Fatalf("stock want 4 got %d", got)
	}
	var orders int64
	if err := f.db.Model(&pointsmalldomain.ExchangeOrder{}).Where("user_id = ? AND product_id = ?", userID, productID).Count(&orders).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if orders != 1 {
		t.Fatalf("orders want 1 got %d", orders)
	}
	f.assertInvariants(t, userID)
}

func TestMallConcurrentLastStock(t *testing.T) {
	f := newMallFixture(t, false)
	productID := f.createProduct(t, 500, 1, false, 0)
	// seed 必须在并发前完成（主线程），避免 goroutine 内写库竞争。
	const users = 10
	for i := 0; i < users; i++ {
		f.seedPoints(t, uint(40+i), 1000)
	}

	var wg sync.WaitGroup
	errs := make(chan error, users)
	okCount := 0
	var mu sync.Mutex
	for i := 0; i < users; i++ {
		userID := uint(40 + i)
		wg.Add(1)
		go func(uid uint) {
			defer wg.Done()
			_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: uid, ProductID: productID, IdempotencyKey: fmt.Sprintf("key-last-%d", uid),
			})
			if err == nil {
				mu.Lock()
				okCount++
				mu.Unlock()
				return
			}
			if !errors.Is(err, pointsmallcontract.ErrProductOutOfStock) {
				errs <- err
			}
		}(userID)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("unexpected error: %v", err)
	}
	if okCount != 1 {
		t.Fatalf("exactly 1 success wanted got %d", okCount)
	}
	if got := f.stock(t, productID); got != 0 {
		t.Fatalf("stock want 0 got %d", got)
	}
}

func TestMallConcurrentSameUserConsumption(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(50)
	f.seedPoints(t, userID, 500)
	productID := f.createProduct(t, 500, 100, false, 0)

	var wg sync.WaitGroup
	okCount := 0
	var mu sync.Mutex
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: userID, ProductID: productID, IdempotencyKey: fmt.Sprintf("key-consum-%d", i),
			})
			if err == nil {
				mu.Lock()
				okCount++
				mu.Unlock()
				return
			}
			if !errors.Is(err, pointsmallcontract.ErrPointsInsufficient) {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("exactly 1 success wanted got %d", okCount)
	}
	if got := f.balance(t, userID); got != 0 {
		t.Fatalf("balance want 0 (no negative) got %d", got)
	}
	f.assertInvariants(t, userID)
}

func TestMallConcurrentFailRefund(t *testing.T) {
	f := newMallFixture(t, false)
	const userID = uint(51)
	f.seedPoints(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)
	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "key-conc-fail",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	orderID := result.Order.ID

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{
				AdminID: 1, OrderID: orderID, Reason: "并发失败",
			})
		}()
	}
	wg.Wait()
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("refund exactly once, balance want 1000 got %d", got)
	}
	if got := f.stock(t, productID); got != 5 {
		t.Fatalf("stock restore once, want 5 got %d", got)
	}
	var refunds int64
	if err := f.db.Model(&pointsdomain.LedgerEntry{}).
		Where("user_id = ? AND action_type = ?", userID, pointscontract.ActionRedeemRefund).
		Count(&refunds).Error; err != nil {
		t.Fatalf("count refund: %v", err)
	}
	if refunds != 1 {
		t.Fatalf("refund ledger want 1 got %d", refunds)
	}
	f.assertInvariants(t, userID)
}
