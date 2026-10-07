//go:build integration
// +build integration

// pg_mall_test.go — Points Mall 在真实 PostgreSQL 上的并发验证（最终并发证明）。
//
// 运行（PowerShell）：
//
//	$env:TEST_POSTGRES_DSN="host=127.0.0.1 port=5432 user=postgres password=postgres dbname=hcz_test sslmode=disable TimeZone=UTC"
//	go test -tags integration -run TestPGMall -v -count=1 -timeout 240s ./internal/modules/pointsmall/integrationtest/
//
// 验证矩阵：
//  1. 10 用户抢最后 1 件库存 → 恰好 1 成功，stock=0（商品行 FOR UPDATE 串行）。
//  2. 20 goroutine 同用户同 Idempotency-Key → 1 订单、1 REDEEM、库存只减 1、
//     UNIQUE(user_id, idempotency_key) 最终防线。
//  3. 10 goroutine 同用户不同 Key 消费（余额恰好=价）→ 恰好 1 成功、无负余额
//     （账户行锁串行化）。
//  4. 10 goroutine 并发 fail 同一订单 → 只返还一次、库存只恢复一次
//     （订单行锁 + REDEEM_REFUND reference 唯一）。
//  5. 死锁探测：不同用户兑换同商品 + 同用户兑换不同商品混跑（统一锁顺序）。
//  6. 数据库不变量：balance == SUM(ledger)；库存不变量。
package integrationtest

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

type pgMallFixture struct {
	db   *gorm.DB
	svc  *pointsmallapp.Service
	psvc *pointsapp.Service
}

func openPGMall(t *testing.T) *pgMallFixture {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("skip pg mall test: TEST_POSTGRES_DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: glogger.Default.LogMode(glogger.Silent)})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlDB: %v", err)
	}
	sqlDB.SetMaxOpenConns(30)
	db.Exec("SET lock_timeout = '5s'")

	models := []interface{}{
		&pointsmalldomain.ExchangeOrder{}, &pointsmalldomain.PointsProduct{},
		&pointsdomain.LedgerEntry{}, &pointsdomain.Account{},
	}
	if err := db.Migrator().DropTable(models...); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	mallStore := pointsmallgormstore.New(db)
	pointsStore := pointsgormstore.New(db)
	psvc := pointsapp.NewService(pointsapp.Options{Repository: pointsStore, Transactions: pointsStore})
	svc := pointsmallapp.NewService(pointsmallapp.Options{
		Repository: mallStore,
		UnitOfWork: mallStore,
		Points:     psvc,
	})
	return &pgMallFixture{db: db, svc: svc, psvc: psvc}
}

func (f *pgMallFixture) seed(t *testing.T, userID uint, amount int64) {
	t.Helper()
	if amount == 0 {
		return
	}
	if _, _, err := f.psvc.AdminAdjust(pointscontract.AdjustInput{
		UserID:          userID,
		OperatorAdminID: 999,
		Operation:       "add",
		Amount:          amount,
		Reason:          "pg test seed",
		Reference:       fmt.Sprintf("pg:seed:%d:%d", userID, time.Now().UnixNano()),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func (f *pgMallFixture) createProduct(t *testing.T, price, stock int64, unlimited bool, limit int64) uint {
	t.Helper()
	p, err := f.svc.CreateProduct(pointsmallcontract.ProductInput{
		Name:            "PG 测试权益",
		PointsPrice:     price,
		Stock:           stock,
		UnlimitedStock:  unlimited,
		Enabled:         true,
		PerUserLimit:    limit,
		FulfillmentType: pointsmalldomain.FulfillmentTypeManual,
		OperatorAdminID: 1,
		Reason:          "PG 测试创建",
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	return p.ID
}

func (f *pgMallFixture) balance(t *testing.T, userID uint) int64 {
	t.Helper()
	acc, err := f.psvc.GetAccount(userID)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	if acc == nil {
		return 0
	}
	return acc.Balance
}

func (f *pgMallFixture) stock(t *testing.T, productID uint) int64 {
	t.Helper()
	var product pointsmalldomain.PointsProduct
	if err := f.db.Where("id = ?", productID).First(&product).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	return product.Stock
}

// TestPGMallLastStockConcurrent 10 用户抢最后 1 件库存。
func TestPGMallLastStockConcurrent(t *testing.T) {
	f := openPGMall(t)
	const users = 10
	productID := f.createProduct(t, 500, 1, false, 0)
	for i := 0; i < users; i++ {
		f.seed(t, uint(2000+i), 1000)
	}

	var wg sync.WaitGroup
	okCount := 0
	var mu sync.Mutex
	errCh := make(chan error, users)
	for i := 0; i < users; i++ {
		uid := uint(2000 + i)
		wg.Add(1)
		go func(uid uint) {
			defer wg.Done()
			_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: uid, ProductID: productID, IdempotencyKey: fmt.Sprintf("pg-last-%d", uid),
			})
			if err == nil {
				mu.Lock()
				okCount++
				mu.Unlock()
				return
			}
			if !errors.Is(err, pointsmallcontract.ErrProductOutOfStock) {
				errCh <- err
			}
		}(uid)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("unexpected: %v", err)
	}
	if okCount != 1 {
		t.Fatalf("exactly 1 success wanted got %d", okCount)
	}
	if got := f.stock(t, productID); got != 0 {
		t.Fatalf("stock want 0 got %d", got)
	}
	// 库存不变量：初始1 - 成功1 = 0（无超卖）
	var sum int64
	f.db.Model(&pointsdomain.LedgerEntry{}).Where("action_type = ?", pointscontract.ActionRedeem).
		Select("COALESCE(SUM(amount),0)").Scan(&sum)
	if sum != -500 {
		t.Fatalf("redeem sum want -500 got %d", sum)
	}
}

// TestPGMallSameKeyConcurrent 20 goroutine 同用户同 Key。
func TestPGMallSameKeyConcurrent(t *testing.T) {
	f := openPGMall(t)
	const userID = uint(2100)
	const workers = 20
	f.seed(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)

	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: userID, ProductID: productID, IdempotencyKey: "pg-key-same",
			}); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent same key: %v", err)
	}

	var orders int64
	f.db.Model(&pointsmalldomain.ExchangeOrder{}).Where("user_id = ?", userID).Count(&orders)
	if orders != 1 {
		t.Fatalf("orders want 1 got %d", orders)
	}
	var redeems int64
	f.db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ? AND action_type = ?", userID, pointscontract.ActionRedeem).
		Count(&redeems)
	if redeems != 1 {
		t.Fatalf("redeem ledger want 1 got %d", redeems)
	}
	if got := f.balance(t, userID); got != 500 {
		t.Fatalf("balance want 500 got %d", got)
	}
	if got := f.stock(t, productID); got != 4 {
		t.Fatalf("stock want 4 got %d", got)
	}
	// 不变量
	var sum int64
	f.db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ?", userID).Select("COALESCE(SUM(amount),0)").Scan(&sum)
	if f.balance(t, userID) != sum {
		t.Fatalf("invariant: balance=%d sum=%d", f.balance(t, userID), sum)
	}
}

// TestPGMallAccountConcurrent 10 goroutine 同用户不同 Key 消费（余额=价）。
func TestPGMallAccountConcurrent(t *testing.T) {
	f := openPGMall(t)
	const userID = uint(2200)
	const workers = 10
	f.seed(t, userID, 500)
	productID := f.createProduct(t, 500, 100, false, 0)

	var wg sync.WaitGroup
	okCount := 0
	var mu sync.Mutex
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: userID, ProductID: productID, IdempotencyKey: fmt.Sprintf("pg-consum-%d", i),
			})
			if err == nil {
				mu.Lock()
				okCount++
				mu.Unlock()
				return
			}
			if !errors.Is(err, pointsmallcontract.ErrPointsInsufficient) {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("unexpected: %v", err)
	}
	if okCount != 1 {
		t.Fatalf("exactly 1 success wanted got %d", okCount)
	}
	if got := f.balance(t, userID); got != 0 {
		t.Fatalf("balance want 0 (no negative) got %d", got)
	}
}

// TestPGMallFailRefundConcurrent 10 goroutine 并发 fail 同一订单。
func TestPGMallFailRefundConcurrent(t *testing.T) {
	f := openPGMall(t)
	const userID = uint(2300)
	const workers = 10
	f.seed(t, userID, 1000)
	productID := f.createProduct(t, 500, 5, false, 0)
	result, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID: userID, ProductID: productID, IdempotencyKey: "pg-key-fail",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	orderID := result.Order.ID

	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.AdminFailOrder(pointsmallcontract.AdminExchangeActionInput{
				AdminID: 1, OrderID: orderID, Reason: "pg 并发失败",
			}); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent fail: %v", err)
	}

	var refunds int64
	f.db.Model(&pointsdomain.LedgerEntry{}).Where("user_id = ? AND action_type = ?", userID, pointscontract.ActionRedeemRefund).
		Count(&refunds)
	if refunds != 1 {
		t.Fatalf("refund ledger want 1 got %d", refunds)
	}
	if got := f.balance(t, userID); got != 1000 {
		t.Fatalf("balance want 1000 got %d", got)
	}
	if got := f.stock(t, productID); got != 5 {
		t.Fatalf("stock want 5 got %d", got)
	}
}

// TestPGMallDeadlockFree 死锁探测：不同用户抢同商品 + 同用户换不同商品混跑。
// 锁顺序（PointsAccount → PointsProduct，返还路径 ExchangeOrder → Account → Product）统一，
// 不应出现稳定死锁（lock_timeout=5s 兜底，deadlock 视为失败而非 retry 掩盖）。
func TestPGMallDeadlockFree(t *testing.T) {
	f := openPGMall(t)
	// 商品 A/B：同一用户 U 兑换 A 与 B 并发（账户行锁热点）
	const userU = uint(2400)
	f.seed(t, userU, 20000)
	productA := f.createProduct(t, 500, 100, false, 0)
	productB := f.createProduct(t, 600, 100, false, 0)
	// 商品 C：10 个不同用户抢（商品行锁热点）
	productC := f.createProduct(t, 700, 10, false, 0)
	for i := 0; i < 10; i++ {
		f.seed(t, uint(2500+i), 5000)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 32)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pid := productA
			if i%2 == 1 {
				pid = productB
			}
			_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: userU, ProductID: pid, IdempotencyKey: fmt.Sprintf("pg-deadlock-u-%d", i),
			})
			if err != nil && !errors.Is(err, pointsmallcontract.ErrProductLimitReached) {
				errCh <- err
			}
		}(i)
	}
	for i := 0; i < 10; i++ {
		uid := uint(2500 + i)
		wg.Add(1)
		go func(uid uint) {
			defer wg.Done()
			_, err := f.svc.CreateExchange(pointsmallcontract.CreateExchangeInput{
				UserID: uid, ProductID: productC, IdempotencyKey: fmt.Sprintf("pg-deadlock-c-%d", uid),
			})
			if err != nil && !errors.Is(err, pointsmallcontract.ErrProductOutOfStock) {
				errCh <- err
			}
		}(uid)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("deadlock or unexpected error: %v", err)
	}
}
