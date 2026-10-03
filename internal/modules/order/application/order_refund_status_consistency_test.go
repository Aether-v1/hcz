package application

import (
	"fmt"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	cardsecretdomain "github.com/Aether-v1/hcz/internal/modules/cardsecret/domain"
	coupondomain "github.com/Aether-v1/hcz/internal/modules/coupon/domain"
	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	paymentdomain "github.com/Aether-v1/hcz/internal/modules/payment/domain"
	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupRefundStatusConsistencyTest(t *testing.T) (*OrderService, *gorm.DB, uint) {
	t.Helper()
	dsn := fmt.Sprintf("file:refund_status_consistency_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&fulfillmentdomain.Fulfillment{},
		&cardsecretdomain.Secret{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&paymentdomain.Payment{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&settingsstore.SettingRecord{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	user := &userdomain.User{Email: "rs@test.com", Status: "active"}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	acct := &walletdomain.Account{UserID: user.ID, Balance: money.FromDecimal(decimal.RequireFromString("100.00"))}
	if err := db.Create(acct).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	order := &orderdomain.Order{
		UserID: user.ID, OrderNo: fmt.Sprintf("RSC%d", now.UnixNano()),
		Status: constants.OrderStatusPendingRecharge, Currency: "CNY",
		TotalAmount:      money.FromDecimal(decimal.RequireFromString("71.80")),
		UsdtTotalAmount:  money.FromDecimal(decimal.RequireFromString("10.00")),
		WalletPaidAmount: money.FromDecimal(decimal.RequireFromString("10.00")),
		RefundStatus:     "none",
		PaidAt:           &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	orderStore := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	walletStore := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{Repository: walletStore, Transactions: walletStore})
	settingSvc := settingsapp.NewService(settingsstore.New(db))
	svc := NewOrderService(OrderServiceOptions{
		OrderStore:    orderStore,
		UserStore:     userstore.New(db),
		WalletService: walletSvc,
		SettingService: settingSvc,
	})
	return svc, db, order.ID
}

func walletBalanceRS(t *testing.T, db *gorm.DB, userID uint) decimal.Decimal {
	t.Helper()
	var acct walletdomain.Account
	if err := db.Where("user_id = ?", userID).First(&acct).Error; err != nil {
		t.Fatal(err)
	}
	return acct.Balance.Decimal
}

// canceled: status=canceled + refund_status=full + wallet 退回 + refunded_amount 正确
func TestCanceledOrderSetsRefundStatusFull(t *testing.T) {
	svc, db, orderID := setupRefundStatusConsistencyTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	userID := order.UserID
	before := walletBalanceRS(t, db, userID)

	if _, err := svc.CancelOrder(orderID, userID); err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	db.First(&order, orderID)
	if order.Status != constants.OrderStatusCanceled {
		t.Fatalf("status must be canceled, got %s", order.Status)
	}
	if order.RefundStatus != constants.OrderRefundStatusFull {
		t.Fatalf("refund_status must be full, got %s", order.RefundStatus)
	}
	if !order.RefundedAmount.Decimal.Equal(decimal.RequireFromString("10.00")) {
		t.Fatalf("refunded_amount want 10.00, got %s", order.RefundedAmount.Decimal)
	}
	after := walletBalanceRS(t, db, userID)
	if !after.Equal(before.Add(decimal.RequireFromString("10.00"))) {
		t.Fatalf("wallet must +10.00, before=%s after=%s", before, after)
	}
}

// failed: status=failed + refund_status=full + wallet 退回
func TestFailedOrderSetsRefundStatusFull(t *testing.T) {
	svc, db, orderID := setupRefundStatusConsistencyTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	userID := order.UserID
	before := walletBalanceRS(t, db, userID)

	if _, err := svc.UpdateOrderStatus(orderID, constants.OrderStatusFailed); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	db.First(&order, orderID)
	if order.Status != constants.OrderStatusFailed {
		t.Fatalf("status must be failed, got %s", order.Status)
	}
	if order.RefundStatus != constants.OrderRefundStatusFull {
		t.Fatalf("refund_status must be full, got %s", order.RefundStatus)
	}
	if !order.RefundedAmount.Decimal.Equal(decimal.RequireFromString("10.00")) {
		t.Fatalf("refunded_amount want 10.00, got %s", order.RefundedAmount.Decimal)
	}
	after := walletBalanceRS(t, db, userID)
	if !after.Equal(before.Add(decimal.RequireFromString("10.00"))) {
		t.Fatalf("wallet must +10.00, before=%s after=%s", before, after)
	}
}

// 重复 cancel 不双退款
func TestCanceledOrderIdempotentNoDoubleRefund(t *testing.T) {
	svc, db, orderID := setupRefundStatusConsistencyTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	userID := order.UserID
	before := walletBalanceRS(t, db, userID)

	if _, err := svc.CancelOrder(orderID, userID); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	// 第二次 cancel 应被状态机拒绝（canceled 是终态）
	if _, err := svc.CancelOrder(orderID, userID); err == nil {
		t.Fatal("second cancel must be rejected")
	}
	after := walletBalanceRS(t, db, userID)
	if !after.Equal(before.Add(decimal.RequireFromString("10.00"))) {
		t.Fatalf("wallet must credit only once, before=%s after=%s", before, after)
	}
}

// 未支付订单 canceled 时 refund_status 保持 none（无钱可退）
func TestUnpaidCanceledKeepsRefundStatusNone(t *testing.T) {
	svc, db, orderID := setupRefundStatusConsistencyTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	// 把订单改为未支付
	db.Model(&orderdomain.Order{}).Where("id = ?", orderID).Updates(map[string]interface{}{
		"wallet_paid_amount": money.FromDecimal(decimal.Zero),
		"paid_at":            nil,
	})
	if _, err := svc.CancelOrder(orderID, order.UserID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	db.First(&order, orderID)
	if order.Status != constants.OrderStatusCanceled {
		t.Fatalf("status must canceled, got %s", order.Status)
	}
	if order.RefundStatus != "none" {
		t.Fatalf("unpaid canceled refund_status must stay none, got %s", order.RefundStatus)
	}
}
