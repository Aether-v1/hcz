package aftersale_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"
	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/modules/order/application/aftersale"
	"github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	paymentgormstore "github.com/Aether-v1/hcz/internal/modules/payment/infrastructure/gormstore"
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

func setupAfterSaleTest(t *testing.T) (*aftersale.Service, *refund.Service, *gorm.DB, uint) {
	t.Helper()
	dsn := fmt.Sprintf("file:aftersale_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&fulfillmentdomain.Fulfillment{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Commission{},
		&affiliatedomain.WithdrawRequest{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&orderdomain.AfterSaleTicket{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&settingsstore.SettingRecord{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	orderStore := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	walletStore := walletgormstore.New(db)
	walletSvc := walletapp.NewService(walletapp.Options{Repository: walletStore, Transactions: walletStore})
	affiliateSvc := application.NewService(affiliategormstore.New(db), nil, nil, nil, nil)
	settingSvc := settingsapp.NewService(settingsstore.New(db))
	paymentStore := paymentgormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	refundSvc := refund.New(orderStore, userstore.New(db), affiliateSvc, settingSvc, walletSvc, paymentStore, nil)
	svc := aftersale.NewService(orderStore, aftersale.NewWalletRefunderAdapter(refundSvc))

	// user + wallet account
	user := &userdomain.User{Email: "u@test.com", Status: "active"}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	acct := &walletdomain.Account{UserID: user.ID, AvailableBalance: money.FromDecimal(decimal.RequireFromString("100.00"))}
	if err := db.Create(acct).Error; err != nil {
		t.Fatal(err)
	}
	// completed order with USDT snapshot
	now := time.Now()
	order := &orderdomain.Order{
		UserID: user.ID, OrderNo: fmt.Sprintf("T%d", now.UnixNano()),
		Status: constants.OrderStatusCompleted, Currency: "CNY",
		TotalAmount:      money.FromDecimal(decimal.RequireFromString("71.80")),
		UsdtTotalAmount:  money.FromDecimal(decimal.RequireFromString("10.00")),
		WalletPaidAmount: money.FromDecimal(decimal.RequireFromString("10.00")),
		RefundStatus:     "none", AfterSaleStatus: "none",
		PaidAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	return svc, refundSvc, db, order.ID
}

func walletBalance(t *testing.T, db *gorm.DB, userID uint) decimal.Decimal {
	t.Helper()
	var acct walletdomain.Account
	if err := db.Where("user_id = ?", userID).First(&acct).Error; err != nil {
		t.Fatal(err)
	}
	return acct.AvailableBalance.Decimal
}

func TestPartialRefundRealDB(t *testing.T) {
	svc, _, db, orderID := setupAfterSaleTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	userID := order.UserID
	before := walletBalance(t, db, userID)

	// 发起售后
	if _, err := svc.Request(userID, orderID, "not_received", "desc"); err != nil {
		t.Fatalf("request: %v", err)
	}
	// 部分退款 4 USDT
	if _, err := svc.PartialRefund(orderID, decimal.RequireFromString("4.00")); err != nil {
		t.Fatalf("partial refund: %v", err)
	}
	// 断言
	db.First(&order, orderID)
	if order.Status != constants.OrderStatusCompleted {
		t.Fatalf("order.status must stay completed, got %s", order.Status)
	}
	if order.AfterSaleStatus != "resolved" {
		t.Fatalf("after_sale_status must resolved, got %s", order.AfterSaleStatus)
	}
	if order.RefundStatus != "partial" {
		t.Fatalf("refund_status must partial, got %s", order.RefundStatus)
	}
	if !order.RefundedAmount.Decimal.Equal(decimal.RequireFromString("4.00")) {
		t.Fatalf("refunded_amount want 4.00 got %s", order.RefundedAmount.Decimal)
	}
	after := walletBalance(t, db, userID)
	if !after.Equal(before.Add(decimal.RequireFromString("4.00"))) {
		t.Fatalf("wallet balance want +4.00, before=%s after=%s", before, after)
	}
	// ledger
	var cnt int64
	db.Model(&walletdomain.Transaction{}).Where("user_id = ? AND type = ?", userID, constants.WalletTxnTypeAdminRefund).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("want 1 refund ledger, got %d", cnt)
	}
	// refund record
	var rc int64
	db.Model(&orderdomain.OrderRefundRecord{}).Where("order_id = ?", orderID).Count(&rc)
	if rc != 1 {
		t.Fatalf("want 1 refund record, got %d", rc)
	}
	// ticket
	var ticket orderdomain.AfterSaleTicket
	db.Where("order_id = ?", orderID).First(&ticket)
	if ticket.Status != "resolved" {
		t.Fatalf("ticket must resolved, got %s", ticket.Status)
	}
}

func TestFullRefundRealDB(t *testing.T) {
	svc, _, db, orderID := setupAfterSaleTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	userID := order.UserID
	before := walletBalance(t, db, userID)

	if _, err := svc.Request(userID, orderID, "not_received", "d"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FullRefund(orderID); err != nil {
		t.Fatalf("full refund: %v", err)
	}
	db.First(&order, orderID)
	if order.RefundStatus != "full" {
		t.Fatalf("refund_status must full, got %s", order.RefundStatus)
	}
	if !order.RefundedAmount.Decimal.Equal(decimal.RequireFromString("10.00")) {
		t.Fatalf("refunded_amount want 10.00 got %s", order.RefundedAmount.Decimal)
	}
	if order.Status != constants.OrderStatusCompleted {
		t.Fatalf("status must stay completed")
	}
	after := walletBalance(t, db, userID)
	if !after.Equal(before.Add(decimal.RequireFromString("10.00"))) {
		t.Fatalf("wallet want +10.00, before=%s after=%s", before, after)
	}
}

func TestRefundFailureRollsBack(t *testing.T) {
	svc, _, db, orderID := setupAfterSaleTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	userID := order.UserID
	before := walletBalance(t, db, userID)

	if _, err := svc.Request(userID, orderID, "not_received", "d"); err != nil {
		t.Fatal(err)
	}
	// 超额退款 → refund 层 ErrRefundExceeded，整个 tx 必须回滚
	if _, err := svc.PartialRefund(orderID, decimal.RequireFromString("999.00")); err == nil {
		t.Fatal("want refund exceeded error")
	}
	db.First(&order, orderID)
	after := walletBalance(t, db, userID)
	if !after.Equal(before) {
		t.Fatalf("wallet must not change on refund failure, before=%s after=%s", before, after)
	}
	if order.RefundStatus != "none" {
		t.Fatalf("refund_status must stay none, got %s", order.RefundStatus)
	}
	if order.AfterSaleStatus != "pending" {
		t.Fatalf("after_sale_status must stay pending, got %s", order.AfterSaleStatus)
	}
	var rc int64
	db.Model(&orderdomain.OrderRefundRecord{}).Where("order_id = ?", orderID).Count(&rc)
	if rc != 0 {
		t.Fatalf("no refund record should be created, got %d", rc)
	}
}

func TestDuplicateRefundIdempotent(t *testing.T) {
	svc, _, db, orderID := setupAfterSaleTest(t)
	var order orderdomain.Order
	db.First(&order, orderID)
	userID := order.UserID
	before := walletBalance(t, db, userID)

	if _, err := svc.Request(userID, orderID, "not_received", "d"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PartialRefund(orderID, decimal.RequireFromString("3.00")); err != nil {
		t.Fatal(err)
	}
	// 第一次成功后 ticket 已 resolved（pending 被清空），第二次必须拒绝且不二次退款
	if _, err := svc.PartialRefund(orderID, decimal.RequireFromString("3.00")); err == nil {
		t.Fatal("second refund must be rejected")
	}
	after := walletBalance(t, db, userID)
	if !after.Equal(before.Add(decimal.RequireFromString("3.00"))) {
		t.Fatalf("wallet must credit only once, before=%s after=%s", before, after)
	}
}
