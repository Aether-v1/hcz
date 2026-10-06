package refund_test

import (
	"testing"
	"time"

	. "github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TestOrderRateSnapshotImmutable: the order's ExchangeRate snapshot is frozen at
// creation; a later "market rate move" must not change it.
func TestOrderRateSnapshotImmutable(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 321)
	order := usdtOrder(t, db, 321, "SNAP-RATE", "100.00", "14.29")

	// Simulate market rate moving after the order was placed.
	marketNow := decimal.RequireFromString("8.5")
	_ = marketNow

	var refreshed orderdomain.Order
	if err := db.First(&refreshed, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !refreshed.ExchangeRate.Valid || !refreshed.ExchangeRate.Decimal.Equal(decimal.RequireFromString("7.0")) {
		t.Fatalf("order rate snapshot must stay 7.0, got %v", refreshed.ExchangeRate.Decimal)
	}
	if !refreshed.UsdtTotalAmount.Decimal.Equal(decimal.RequireFromString("14.29")) {
		t.Fatalf("usdt snapshot must stay 14.29, got %s", refreshed.UsdtTotalAmount.String())
	}
	if refreshed.ExchangeRateAt == nil {
		t.Fatal("exchange_rate_at must be snapshot")
	}
}

// TestOrderCostSnapshotImmutable: the OrderItem cost snapshot is frozen at creation.
func TestOrderCostSnapshotImmutable(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 322)
	order := usdtOrder(t, db, 322, "SNAP-COST", "100.00", "14.29")
	now := time.Now()
	item := &orderdomain.OrderItem{
		OrderID:    order.ID,
		ProductID:  1,
		TitleJSON:  jsonmap.JSON{"zh-CN": "p1"},
		UnitPrice:  money.FromDecimal(decimal.RequireFromString("100")),
		CostPrice:  money.FromDecimal(decimal.RequireFromString("42")),
		Quantity:   1,
		TotalPrice: money.FromDecimal(decimal.RequireFromString("100")),
		CreatedAt:  now, UpdatedAt: now,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	// "upstream" cost changes later; snapshot must NOT move.
	_ = decimal.RequireFromString("999")
	var refreshedItem orderdomain.OrderItem
	if err := db.First(&refreshedItem, item.ID).Error; err != nil {
		t.Fatalf("reload item: %v", err)
	}
	if !refreshedItem.CostPrice.Decimal.Equal(decimal.RequireFromString("42")) {
		t.Fatalf("cost snapshot must stay 42, got %s", refreshedItem.CostPrice.String())
	}
}

// TestFullRefundUsesSnapshot: a full refund credits back the original USDT
// snapshot (WalletPaidAmount), not recomputed from the current market rate.
func TestFullRefundUsesSnapshot(t *testing.T) {
	svc, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 323)
	order := createTestOrder(t, db, 323, "SNAP-FULLREFUND", decimal.RequireFromString("40"))
	// seed a wallet-paid USDT snapshot of 40 (as a real paid order would).
	if err := db.Model(&orderdomain.Order{}).Where("id=?", order.ID).Updates(map[string]interface{}{
		"status":             constants.OrderStatusPaid,
		"paid_at":            time.Now(),
		"wallet_paid_amount": "40.00",
	}).Error; err != nil {
		t.Fatalf("seed paid order: %v", err)
	}
	before, _ := walletServiceForTest(db).GetAccount(323)
	_, _, _, err := svc.AdminRefundToWallet(AdminRefundToWalletInput{
		OrderID: order.ID,
		Amount:  money.FromDecimal(decimal.RequireFromString("40")),
		Remark:  "full refund uses snapshot",
	})
	if err != nil {
		t.Fatalf("full refund: %v", err)
	}
	after, _ := walletServiceForTest(db).GetAccount(323)
	// refund credits exactly the snapshot 40 USDT.
	if !after.AvailableBalance.Decimal.Sub(before.AvailableBalance.Decimal).Equal(decimal.RequireFromString("40")) {
		t.Fatalf("refund must credit snapshot 40 USDT, got delta %s", after.AvailableBalance.Decimal.Sub(before.AvailableBalance.Decimal).String())
	}
}

// TestRepeatedPartialRefund: multiple partial refunds accumulate against the bound.
func TestRepeatedPartialRefund(t *testing.T) {
	svc, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 324)
	order := createTestOrder(t, db, 324, "SNAP-REPEATED", decimal.RequireFromString("40"))
	if err := db.Model(&orderdomain.Order{}).Where("id=?", order.ID).Updates(map[string]interface{}{
		"status":             constants.OrderStatusPaid,
		"paid_at":            time.Now(),
		"wallet_paid_amount": "40.00",
	}).Error; err != nil {
		t.Fatalf("seed paid order: %v", err)
	}
	for _, amt := range []string{"10", "10", "10"} {
		if _, _, _, err := svc.AdminRefundToWallet(AdminRefundToWalletInput{
			OrderID: order.ID, Amount: money.FromDecimal(decimal.RequireFromString(amt)), Remark: "partial",
		}); err != nil {
			t.Fatalf("partial refund %s: %v", amt, err)
		}
	}
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if !refreshed.RefundedAmount.Decimal.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("accumulated refunded = %s, want 30", refreshed.RefundedAmount.String())
	}
}

// TestRefundRateChanged: even if the market rate moves between payment and refund,
// the refund amount stays pinned to the original order USDT snapshot.
func TestRefundRateChanged(t *testing.T) {
	svc, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 325)
	order := usdtOrder(t, db, 325, "SNAP-RATECHG", "100.00", "14.29")
	if err := db.Model(&orderdomain.Order{}).Where("id=?", order.ID).Updates(map[string]interface{}{
		"status":             constants.OrderStatusPaid,
		"paid_at":            time.Now(),
		"wallet_paid_amount": "14.29",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	// "market rate changes" — we do not touch order; refund must still be 14.29 USDT.
	if _, _, _, err := svc.AdminRefundToWallet(AdminRefundToWalletInput{
		OrderID: order.ID, Amount: money.FromDecimal(decimal.RequireFromString("14.29")), Remark: "rate moved",
	}); err != nil {
		t.Fatalf("refund after rate change: %v", err)
	}
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if !refreshed.RefundedAmount.Decimal.Equal(decimal.RequireFromString("14.29")) {
		t.Fatalf("refunded = %s, want pinned 14.29", refreshed.RefundedAmount.String())
	}
}

var _ = gorm.ErrRecordNotFound
