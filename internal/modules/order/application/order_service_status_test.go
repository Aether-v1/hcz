package application

import (
	"fmt"
	"testing"
	"time"

	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestCalcParentStatus(t *testing.T) {
	// HCZ M1: 五态机下 CalcParentStatus 先归一子订单状态再聚合
	// delivered → completed, paid → pending_recharge
	// 一个 completed + 一个 pending_recharge → 父 pending_recharge（优先级更高）
	children := []orderdomain.Order{
		{Status: constants.OrderStatusDelivered},
		{Status: constants.OrderStatusPaid},
	}
	status := CalcParentStatus(children, constants.OrderStatusProcessing)
	if status != constants.OrderStatusPendingRecharge {
		t.Fatalf("expected pending_recharge, got %s", status)
	}

	children = []orderdomain.Order{
		{Status: constants.OrderStatusCompleted},
		{Status: constants.OrderStatusCompleted},
	}
	status = CalcParentStatus(children, constants.OrderStatusProcessing)
	if status != constants.OrderStatusCompleted {
		t.Fatalf("expected completed, got %s", status)
	}
}

func TestCalcParentStatusAllRefunded(t *testing.T) {
	// HCZ M1: refunded 归一为 completed + refund_status=full，不影响主状态聚合
	// 所有子 refunded → 归一为 completed → 父 completed
	children := []orderdomain.Order{
		{Status: constants.OrderStatusRefunded},
		{Status: constants.OrderStatusRefunded},
	}
	status := CalcParentStatus(children, constants.OrderStatusDelivered)
	if status != constants.OrderStatusCompleted {
		t.Fatalf("expected completed, got %s", status)
	}
}

func TestCalcParentStatusPartiallyRefunded(t *testing.T) {
	// HCZ M1: refund_status 不影响主状态聚合
	// refunded → completed, delivered → completed → 父 completed
	children := []orderdomain.Order{
		{Status: constants.OrderStatusRefunded},
		{Status: constants.OrderStatusDelivered},
	}
	status := CalcParentStatus(children, constants.OrderStatusDelivered)
	if status != constants.OrderStatusCompleted {
		t.Fatalf("expected completed, got %s", status)
	}
}

func TestExpectedRefundStatus(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		order  orderdomain.Order
		expect string
	}{
		{
			name: "partial refund",
			order: orderdomain.Order{
				Status:         constants.OrderStatusCompleted,
				PaidAt:         &now,
				TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: money.FromDecimal(decimal.NewFromInt(30)),
			},
			expect: constants.OrderStatusPartiallyRefunded,
		},
		{
			name: "full refund",
			order: orderdomain.Order{
				Status:         constants.OrderStatusCompleted,
				PaidAt:         &now,
				TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: money.FromDecimal(decimal.NewFromInt(100)),
			},
			expect: constants.OrderStatusRefunded,
		},
		{
			name: "canceled should keep",
			order: orderdomain.Order{
				Status:         constants.OrderStatusCanceled,
				PaidAt:         &now,
				TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: money.FromDecimal(decimal.NewFromInt(100)),
			},
			expect: "",
		},
	}

	for _, tc := range tests {
		got := expectedRefundStatus(&tc.order)
		if got != tc.expect {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.expect, got)
		}
	}
}

func TestResolvedParentStatusPrefersOwnRefund(t *testing.T) {
	now := time.Now()
	order := &orderdomain.Order{
		Status:         constants.OrderStatusCompleted,
		PaidAt:         &now,
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(40)),
		RefundedAmount: money.FromDecimal(decimal.NewFromInt(10)),
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusCompleted},
			{Status: constants.OrderStatusCompleted},
		},
	}
	if got := resolvedParentStatus(order); got != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected partially_refunded, got %s", got)
	}
}

func TestUpdateOrderStatusRejectsManualPaidTransition(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_reject_manual_paid_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	now := time.Now()
	order := &orderdomain.Order{
		OrderNo:        "MANUAL-PAID-MUST-FAIL",
		Status:         constants.OrderStatusPendingPayment,
		Currency:       "CNY",
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	svc := NewOrderService(OrderServiceOptions{OrderStore: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")})

	if _, err := svc.UpdateOrderStatus(order.ID, constants.OrderStatusPaid); err != ErrOrderStatusInvalid {
		t.Fatalf("manual paid transition error = %v, want %v", err, ErrOrderStatusInvalid)
	}
	var stored orderdomain.Order
	if err := db.First(&stored, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if stored.Status != constants.OrderStatusPendingPayment || stored.PaidAt != nil {
		t.Fatalf("manual paid transition mutated order: %+v", stored)
	}
}

func TestIsTransitionAllowedRefunded(t *testing.T) {
	if !IsTransitionAllowed(constants.OrderStatusDelivered, constants.OrderStatusPartiallyRefunded) {
		t.Fatalf("expected delivered to partially_refunded transition to be allowed")
	}
	if !IsTransitionAllowed(constants.OrderStatusPartiallyRefunded, constants.OrderStatusRefunded) {
		t.Fatalf("expected partially_refunded to refunded transition to be allowed")
	}
	if !IsTransitionAllowed(constants.OrderStatusDelivered, constants.OrderStatusRefunded) {
		t.Fatalf("expected delivered to refunded transition to be allowed")
	}
	if !IsTransitionAllowed(constants.OrderStatusCompleted, constants.OrderStatusRefunded) {
		t.Fatalf("expected completed to refunded transition to be allowed")
	}
	if IsTransitionAllowed(constants.OrderStatusCanceled, constants.OrderStatusRefunded) {
		t.Fatalf("expected canceled to refunded transition to be rejected")
	}
}

func TestUpdateOrderStatusParentToPartiallyRefundedSyncsChildren(t *testing.T) {
	// HCZ P0-5: admin 不能直接设 partially_refunded/refunded 主状态
	// Refund 只更新 refund_status 独立列，主状态由 ordermachine 管理
	dsn := fmt.Sprintf("file:order_service_parent_partial_refund_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	paidAt := now
	parent := &orderdomain.Order{
		OrderNo:          "PARENT-PARTIAL-REFUND-001",
		UserID:           0,
		Status:           constants.OrderStatusCompleted,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   money.FromDecimal(decimal.NewFromInt(30)),
		PaidAt:           &paidAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent order failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderStore: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
	})
	// HCZ P0-5: admin 直接设 partially_refunded 应被拒绝
	_, err = svc.UpdateOrderStatus(parent.ID, constants.OrderStatusPartiallyRefunded)
	if err == nil {
		t.Fatalf("expected admin direct partially_refunded to be rejected, but got nil error")
	}
}

func TestCanCompleteParentOrder(t *testing.T) {
	order := &orderdomain.Order{
		Status: constants.OrderStatusDelivered,
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusDelivered},
			{Status: constants.OrderStatusCompleted},
		},
	}
	if !canCompleteParentOrder(order) {
		t.Fatalf("expected delivered parent order to be completable")
	}
}

func TestCanCompleteParentOrderRejectInvalidStatus(t *testing.T) {
	order := &orderdomain.Order{
		Status: constants.OrderStatusPartiallyDelivered,
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusDelivered},
		},
	}
	if canCompleteParentOrder(order) {
		t.Fatalf("expected partially_delivered parent order to be rejected")
	}
}

func TestCanCompleteParentOrderRejectInvalidChild(t *testing.T) {
	order := &orderdomain.Order{
		Status: constants.OrderStatusDelivered,
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusPaid},
		},
	}
	if canCompleteParentOrder(order) {
		t.Fatalf("expected parent order with paid child to be rejected")
	}
}
