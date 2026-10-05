package application_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	categorydomain "github.com/Aether-v1/hcz/internal/modules/catalog/category/domain"
	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	productgormstore "github.com/Aether-v1/hcz/internal/modules/catalog/product/store/gormstore"
	coupondomain "github.com/Aether-v1/hcz/internal/modules/coupon/domain"
	coupongormstore "github.com/Aether-v1/hcz/internal/modules/coupon/infrastructure/gormstore"
	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	usergormstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	. "github.com/Aether-v1/hcz/internal/modules/order/application"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	promotiondomain "github.com/Aether-v1/hcz/internal/modules/promotion/domain"
	promotiongormstore "github.com/Aether-v1/hcz/internal/modules/promotion/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ---- idempotency test fixture ----

type idempotencyFixture struct {
	db      *gorm.DB
	product productdomain.Product
	sku     productdomain.ProductSKU
	user    userdomain.User
	queue   *idempotencyQueue
}

type idempotencyQueue struct {
	enqueued int
	mu       sync.Mutex
}

func (q *idempotencyQueue) Enabled() bool { return true }
func (q *idempotencyQueue) EnqueueTimeoutCancel(_ uint, _ time.Duration) error {
	q.mu.Lock()
	q.enqueued++
	q.mu.Unlock()
	return nil
}
func (q *idempotencyQueue) EnqueueStatusEmail(_ uint, _ string) error { return nil }

func newIdempotencyFixture(t *testing.T, name string) *idempotencyFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", name, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(
		&userdomain.User{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// P1：手动创建部分唯一索引（SQLite 支持 WHERE 子句的唯一索引）。
	if err := db.Exec(
		"CREATE UNIQUE INDEX idx_orders_user_idempotency ON orders (user_id, idempotency_key) WHERE idempotency_key <> ''",
	).Error; err != nil {
		t.Fatalf("create idempotency unique index: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{
		Slug:     name + "-cat",
		NameJSON: jsonmap.JSON{"zh-CN": "幂等测试分类"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	product := productdomain.Product{
		CategoryID:         category.ID,
		Slug:               name + "-prod",
		TitleJSON:          jsonmap.JSON{"zh-CN": "幂等测试商品"},
		PriceAmount:        money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:           true,
		FulfillmentType:    constants.FulfillmentTypeManual,
		ManualStockTotal:   100,
		ManualStockLocked:  0,
		ManualStockSold:    0,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           productdomain.DefaultSKUCode,
		SpecValuesJSON:    jsonmap.JSON{},
		PriceAmount:       product.PriceAmount,
		ManualStockTotal:  100,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		IsActive:          true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}

	user := userdomain.User{
		Email:       name + "@test.com",
		DisplayName: name + "-user",
		Status:      "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return &idempotencyFixture{
		db:      db,
		product: product,
		sku:     sku,
		user:    user,
		queue:   &idempotencyQueue{},
	}
}

func (f *idempotencyFixture) service() *OrderService {
	return NewOrderService(OrderServiceOptions{
		OrderStore:       ordergormstore.New(f.db, "test-guest-credential-secret-with-32-bytes"),
		UserStore:        usergormstore.New(f.db),
		ProductStore:     productgormstore.NewProductStore(f.db),
		ProductSKUStore:  productgormstore.NewSKUStore(f.db),
		CouponStore:      coupongormstore.New(f.db),
		CouponUsageStore: coupongormstore.NewUsageStore(f.db),
		PromotionRepo:    promotiongormstore.New(f.db),
		Queue:            f.queue,
		ExpireMinutes:    15,
	})
}

func (f *idempotencyFixture) input(idemKey string) CreateOrderInput {
	return CreateOrderInput{
		UserID:         f.user.ID,
		IdempotencyKey: idemKey,
		Items: []CreateOrderItem{{
			ProductID: f.product.ID,
			SKUID:     f.sku.ID,
			Quantity:  1,
		}},
	}
}

func (f *idempotencyFixture) inputWithProduct(idemKey string, productID, skuID uint, qty int) CreateOrderInput {
	return CreateOrderInput{
		UserID:         f.user.ID,
		IdempotencyKey: idemKey,
		Items: []CreateOrderItem{{
			ProductID: productID,
			SKUID:     skuID,
			Quantity:  qty,
		}},
	}
}

func (f *idempotencyFixture) countOrders() int64 {
	var count int64
	// 只统计父订单（parent_id IS NULL），因为每个商品会生成一个子订单。
	f.db.Model(&orderdomain.Order{}).Where("deleted_at IS NULL AND parent_id IS NULL").Count(&count)
	return count
}

// ---- tests ----

// Test 1: 正常 create order with idempotency key
func TestOrderIdempotency_NormalCreate(t *testing.T) {
	f := newIdempotencyFixture(t, "normal_create")
	svc := f.service()

	order, err := svc.CreateOrder(f.input("key-normal-1"))
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	if order == nil || order.OrderNo == "" {
		t.Fatal("expected non-nil order with order_no")
	}
	if order.IdempotencyKey != "key-normal-1" {
		t.Fatalf("idempotency_key = %q, want %q", order.IdempotencyKey, "key-normal-1")
	}
	if order.IdempotencyFingerprint == "" {
		t.Fatal("expected non-empty idempotency_fingerprint")
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1", f.countOrders())
	}
}

// Test 2: 相同 key 顺序请求两次 → 返回同一订单，不创建第二单
func TestOrderIdempotency_SequentialDuplicate(t *testing.T) {
	f := newIdempotencyFixture(t, "sequential_dup")
	svc := f.service()

	order1, err := svc.CreateOrder(f.input("key-seq-1"))
	if err != nil {
		t.Fatalf("first CreateOrder failed: %v", err)
	}

	order2, err := svc.CreateOrder(f.input("key-seq-1"))
	if err != nil {
		t.Fatalf("second CreateOrder failed: %v", err)
	}

	if order1.ID != order2.ID {
		t.Fatalf("order IDs differ: first=%d second=%d", order1.ID, order2.ID)
	}
	if order1.OrderNo != order2.OrderNo {
		t.Fatalf("order_nos differ: first=%s second=%s", order1.OrderNo, order2.OrderNo)
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1", f.countOrders())
	}
}

// Test 3: 相同 key 并发请求两次 → 只有一个订单创建
func TestOrderIdempotency_ConcurrentDuplicate(t *testing.T) {
	f := newIdempotencyFixture(t, "concurrent_dup")
	svc := f.service()

	const goroutines = 2
	results := make(chan *orderdomain.Order, goroutines)
	errs := make(chan error, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			order, err := svc.CreateOrder(f.input("key-concurrent-1"))
			if err != nil {
				errs <- err
				return
			}
			results <- order
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent CreateOrder error: %v", err)
	}

	var orders []*orderdomain.Order
	for order := range results {
		orders = append(orders, order)
	}
	if len(orders) != goroutines {
		t.Fatalf("got %d results, want %d", len(orders), goroutines)
	}

	firstID := orders[0].ID
	for _, order := range orders {
		if order.ID != firstID {
			t.Fatalf("order IDs differ: %d vs %d", firstID, order.ID)
		}
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1", f.countOrders())
	}
}

// Test 4: 相同 key + 相同 payload → 返回同一订单
func TestOrderIdempotency_SameKeySamePayload(t *testing.T) {
	f := newIdempotencyFixture(t, "same_payload")
	svc := f.service()

	order1, err := svc.CreateOrder(f.input("key-payload-1"))
	if err != nil {
		t.Fatalf("first CreateOrder failed: %v", err)
	}
	order2, err := svc.CreateOrder(f.input("key-payload-1"))
	if err != nil {
		t.Fatalf("second CreateOrder failed: %v", err)
	}
	if order1.ID != order2.ID {
		t.Fatalf("order IDs differ: %d vs %d", order1.ID, order2.ID)
	}
}

// Test 5: 相同 key + 不同 payload → 返回 ErrIdempotencyPayloadConflict
func TestOrderIdempotency_SameKeyDifferentPayload(t *testing.T) {
	f := newIdempotencyFixture(t, "diff_payload")
	svc := f.service()

	// First request: quantity 1
	_, err := svc.CreateOrder(f.inputWithProduct("key-conflict-1", f.product.ID, f.sku.ID, 1))
	if err != nil {
		t.Fatalf("first CreateOrder failed: %v", err)
	}

	// Second request: same key, quantity 2 (different payload)
	_, err = svc.CreateOrder(f.inputWithProduct("key-conflict-1", f.product.ID, f.sku.ID, 2))
	if err == nil {
		t.Fatal("expected ErrIdempotencyPayloadConflict, got nil")
	}
	if !errors.Is(err, ErrIdempotencyPayloadConflict) {
		t.Fatalf("expected ErrIdempotencyPayloadConflict, got %v", err)
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1 (conflict should not create second order)", f.countOrders())
	}
}

// Test 6: 第一次成功但模拟客户端重试 → 返回原订单（replay 语义）
func TestOrderIdempotency_ReplayAfterSuccess(t *testing.T) {
	f := newIdempotencyFixture(t, "replay")
	svc := f.service()

	original, err := svc.CreateOrder(f.input("key-replay-1"))
	if err != nil {
		t.Fatalf("first CreateOrder failed: %v", err)
	}

	// Simulate client not receiving response and retrying
	replayed, err := svc.CreateOrder(f.input("key-replay-1"))
	if err != nil {
		t.Fatalf("retry CreateOrder failed: %v", err)
	}

	if original.ID != replayed.ID {
		t.Fatalf("replay returned different order: original=%d replayed=%d", original.ID, replayed.ID)
	}
	if original.OrderNo != replayed.OrderNo {
		t.Fatalf("replay returned different order_no: original=%s replayed=%s", original.OrderNo, replayed.OrderNo)
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1", f.countOrders())
	}
}

// Test 7 & 8: order 只产生一个（幂等保证不重复建单）
func TestOrderIdempotency_OnlyOneOrderCreated(t *testing.T) {
	f := newIdempotencyFixture(t, "only_one")
	svc := f.service()

	// Create 5 times with same key
	for i := 0; i < 5; i++ {
		_, err := svc.CreateOrder(f.input("key-only-one-1"))
		if err != nil {
			t.Fatalf("CreateOrder #%d failed: %v", i, err)
		}
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1 after 5 duplicate requests", f.countOrders())
	}
}

// Test 9: 不同 key 可创建两笔不同订单
func TestOrderIdempotency_DifferentKeysCreateDifferentOrders(t *testing.T) {
	f := newIdempotencyFixture(t, "diff_keys")
	svc := f.service()

	order1, err := svc.CreateOrder(f.input("key-diff-1"))
	if err != nil {
		t.Fatalf("first CreateOrder failed: %v", err)
	}
	order2, err := svc.CreateOrder(f.input("key-diff-2"))
	if err != nil {
		t.Fatalf("second CreateOrder failed: %v", err)
	}

	if order1.ID == order2.ID {
		t.Fatal("different keys should create different orders")
	}
	if order1.OrderNo == order2.OrderNo {
		t.Fatal("different keys should create different order_nos")
	}
	if f.countOrders() != 2 {
		t.Fatalf("order count = %d, want 2", f.countOrders())
	}
}

// Test 10: 无 Idempotency-Key 时 service 层正常创建（handler 层会返回 400）
func TestOrderIdempotency_NoKeyCreatesOrderNormally(t *testing.T) {
	f := newIdempotencyFixture(t, "no_key")
	svc := f.service()

	order, err := svc.CreateOrder(f.input(""))
	if err != nil {
		t.Fatalf("CreateOrder without key failed: %v", err)
	}
	if order == nil {
		t.Fatal("expected non-nil order")
	}
	if order.IdempotencyKey != "" {
		t.Fatalf("expected empty idempotency_key, got %q", order.IdempotencyKey)
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1", f.countOrders())
	}
}

// Test 11: transaction rollback 后同 key 可重试成功
// 模拟：第一次请求因库存不足导致事务 rollback（订单未持久化），同 key 重试应成功。
func TestOrderIdempotency_RetryAfterRollback(t *testing.T) {
	f := newIdempotencyFixture(t, "rollback_retry")
	svc := f.service()

	// Set stock to 0 to force rollback during order creation
	if err := f.db.Model(&productdomain.ProductSKU{}).
		Where("id = ?", f.sku.ID).
		Update("manual_stock_total", 0).Error; err != nil {
		t.Fatalf("set stock to 0: %v", err)
	}

	// First request should fail (insufficient stock) → transaction rollback → no order persisted
	_, err := svc.CreateOrder(f.input("key-rollback-1"))
	if err == nil {
		t.Fatal("expected error due to insufficient stock")
	}
	if f.countOrders() != 0 {
		t.Fatalf("order count = %d, want 0 after rollback", f.countOrders())
	}

	// Restore stock
	if err := f.db.Model(&productdomain.ProductSKU{}).
		Where("id = ?", f.sku.ID).
		Update("manual_stock_total", 100).Error; err != nil {
		t.Fatalf("restore stock: %v", err)
	}

	// Retry with same key should succeed (key was not persisted due to rollback)
	order, err := svc.CreateOrder(f.input("key-rollback-1"))
	if err != nil {
		t.Fatalf("retry after rollback failed: %v", err)
	}
	if order == nil || order.OrderNo == "" {
		t.Fatal("expected non-nil order after retry")
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1", f.countOrders())
	}
}

// Test 12: 15min checkout expiry 不影响幂等语义
// 订单的 expires_at 是支付过期时间，与幂等 key 无关。同 key 重试即使超过 15 分钟也应返回原订单。
func TestOrderIdempotency_ExpiryDoesNotAffectIdempotency(t *testing.T) {
	f := newIdempotencyFixture(t, "expiry_idem")
	svc := f.service()

	order1, err := svc.CreateOrder(f.input("key-expiry-1"))
	if err != nil {
		t.Fatalf("first CreateOrder failed: %v", err)
	}

	// Simulate time passing beyond 15min expiry by directly updating expires_at
	if err := f.db.Model(&orderdomain.Order{}).
		Where("id = ?", order1.ID).
		Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("backdate expires_at: %v", err)
	}

	// Retry with same key should still return the original order (idempotency is independent of expiry)
	order2, err := svc.CreateOrder(f.input("key-expiry-1"))
	if err != nil {
		t.Fatalf("retry after expiry failed: %v", err)
	}
	if order1.ID != order2.ID {
		t.Fatalf("expiry should not affect idempotency: original=%d replayed=%d", order1.ID, order2.ID)
	}
	if f.countOrders() != 1 {
		t.Fatalf("order count = %d, want 1", f.countOrders())
	}
}
