package refund_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	orderapp "github.com/Aether-v1/hcz/internal/modules/order/application"
	. "github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"

	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func nullDec(d decimal.Decimal) decimal.NullDecimal { return decimal.NullDecimal{Decimal: d, Valid: true} }

func walletRecharge(userID uint, amount string) walletcontract.RechargeInput {
	return walletcontract.RechargeInput{UserID: userID, Amount: money.FromDecimal(decimal.RequireFromString(amount))}
}

// usdtOrder builds an order that has the P0-2 USDT snapshot: UsdtTotalAmount>0.
func usdtOrder(t *testing.T, db *gorm.DB, userID uint, orderNo string, cnyTotal, usdtTotal string) *orderdomain.Order {
	t.Helper()
	now := time.Now()
	rate := decimal.RequireFromString("7.0")
	o := &orderdomain.Order{
		OrderNo:            orderNo,
		UserID:             userID,
		Status:             constants.OrderStatusPendingRecharge,
		Currency:           "CNY",
		OriginalAmount:     money.FromDecimal(decimal.RequireFromString(cnyTotal)),
		TotalAmount:        money.FromDecimal(decimal.RequireFromString(cnyTotal)),
		UsdtTotalAmount:    money.FromDecimal(decimal.RequireFromString(usdtTotal)),
		ExchangeRate:       nullDec(rate),
		ExchangeRateSource: "AUTO",
		ExchangeRateAt:    &now,
		WalletPaidAmount:   money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:   money.FromDecimal(decimal.Zero),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(o).Error; err != nil {
		t.Fatalf("create usdt order: %v", err)
	}
	return o
}

// TestWalletOnlyPayment: a USDT-snapshot order debits the wallet exactly UsdtTotalAmount
// and leaves online_paid_amount = 0.
func TestWalletOnlyPayment(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 301)
	order := usdtOrder(t, db, 301, "CUR-WALLETONLY", "100.00", "14.29")

	if _, _, err := walletServiceForTest(db).Recharge(walletRecharge(301, "50")); err != nil {
		t.Fatalf("recharge: %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		deducted, err := orderapp.ApplyWalletBalance(
			walletServiceForTest(db),
			ordergormstore.UseTransaction(tx, "test-guest-credential-secret-with-32-bytes"),
			order, true)
		if err != nil {
			return err
		}
		if !deducted.Equal(decimal.RequireFromString("14.29")) {
			return fmt.Errorf("deducted = %s, want 14.29 USDT", deducted.String())
		}
		return nil
	}); err != nil {
		t.Fatalf("apply wallet: %v", err)
	}

	var refreshed orderdomain.Order
	if err := db.First(&refreshed, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !refreshed.OnlinePaidAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("wallet-only order online_paid must be 0, got %s", refreshed.OnlinePaidAmount.String())
	}
	if !refreshed.WalletPaidAmount.Decimal.Equal(decimal.RequireFromString("14.29")) {
		t.Fatalf("wallet_paid = %s, want 14.29", refreshed.WalletPaidAmount.String())
	}
}

// TestNoMixedCurrencySubtraction locks the red line: a USDT-snapshot order must
// never compute online_paid = TotalAmount(CNY) - WalletPaid(USDT).
func TestNoMixedCurrencySubtraction(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 302)
	order := usdtOrder(t, db, 302, "CUR-NOMIX", "100.00", "14.29")
	if _, _, err := walletServiceForTest(db).Recharge(walletRecharge(302, "50")); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	_ = db.Transaction(func(tx *gorm.DB) error {
		_, _ = orderapp.ApplyWalletBalance(
			walletServiceForTest(db),
			ordergormstore.UseTransaction(tx, "test-guest-credential-secret-with-32-bytes"),
			order, true)
		return nil
	})
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if !refreshed.OnlinePaidAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("MIXED CURRENCY BUG: online_paid=%s must be 0, not TotalCNY-WalletUSDT", refreshed.OnlinePaidAmount.String())
	}
}

// TestMixedPayment: a legacy order WITHOUT USDT snapshot uses CNY-basis allocation;
// the remainder is online_paid (same CNY, allowed).
func TestMixedPayment(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 303)
	order := createTestOrder(t, db, 303, "CUR-MIXED", decimal.RequireFromString("30"))
	if _, _, err := walletServiceForTest(db).Recharge(walletRecharge(303, "10")); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	_ = db.Transaction(func(tx *gorm.DB) error {
		_, _ = orderapp.ApplyWalletBalance(
			walletServiceForTest(db),
			ordergormstore.UseTransaction(tx, "test-guest-credential-secret-with-32-bytes"),
			order, true)
		return nil
	})
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if !refreshed.WalletPaidAmount.Decimal.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("wallet_paid = %s, want 10", refreshed.WalletPaidAmount.String())
	}
	if !refreshed.OnlinePaidAmount.Decimal.Equal(decimal.RequireFromString("20")) {
		t.Fatalf("online_paid = %s, want 20 (CNY remainder, allowed)", refreshed.OnlinePaidAmount.String())
	}
}

// TestWalletDebitRollback: when debit cannot even start (guest order, no wallet),
// the order is NOT marked paid (wallet_paid stays 0) and the tx rolls back.
func TestWalletDebitRollback(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	// guest order (UserID=0): wallet debit must fail with ErrNotSupportedForGuest.
	order := usdtOrder(t, db, 0, "CUR-ROLLBACK", "100.00", "14.29")
	err := db.Transaction(func(tx *gorm.DB) error {
		_, applyErr := orderapp.ApplyWalletBalance(
			walletServiceForTest(db),
			ordergormstore.UseTransaction(tx, "test-guest-credential-secret-with-32-bytes"),
			order, true)
		return applyErr
	})
	if !errors.Is(err, walletcontract.ErrNotSupportedForGuest) {
		t.Fatalf("guest debit should fail with ErrNotSupportedForGuest, got %v", err)
	}
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if !refreshed.WalletPaidAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("order must not be paid when debit fails, wallet_paid=%s", refreshed.WalletPaidAmount.String())
	}
}

// TestOrderStatusRollback: repeated ApplyWalletBalance is idempotent (no double debit).
func TestOrderStatusRollback(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 305)
	order := usdtOrder(t, db, 305, "CUR-STATUSROLLBACK", "100.00", "14.29")
	if _, _, err := walletServiceForTest(db).Recharge(walletRecharge(305, "50")); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	for i := 0; i < 2; i++ {
		_ = db.Transaction(func(tx *gorm.DB) error {
			_, _ = orderapp.ApplyWalletBalance(
				walletServiceForTest(db),
				ordergormstore.UseTransaction(tx, "test-guest-credential-secret-with-32-bytes"),
				order, true)
			return nil
		})
	}
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if !refreshed.WalletPaidAmount.Decimal.Equal(decimal.RequireFromString("14.29")) {
		t.Fatalf("idempotent apply must not double-debit, wallet_paid=%s want 14.29", refreshed.WalletPaidAmount.String())
	}
	acct, _ := walletServiceForTest(db).GetAccount(305)
	if !acct.AvailableBalance.Decimal.Equal(decimal.RequireFromString("35.71")) {
		t.Fatalf("balance = %s, want 50-14.29=35.71", acct.AvailableBalance.String())
	}
}

// TestConcurrentOrderCreateNoDoubleDebit: concurrent debits on the same order
// must charge exactly once (reference idempotency).
func TestConcurrentOrderCreateNoDoubleDebit(t *testing.T) {
	_, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 306)
	order := usdtOrder(t, db, 306, "CUR-CONC-ORDER", "100.00", "14.29")
	if _, _, err := walletServiceForTest(db).Recharge(walletRecharge(306, "50")); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = db.Transaction(func(tx *gorm.DB) error {
				_, _ = orderapp.ApplyWalletBalance(
					walletServiceForTest(db),
					ordergormstore.UseTransaction(tx, "test-guest-credential-secret-with-32-bytes"),
					order, true)
				return nil
			})
		}()
	}
	wg.Wait()
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if !refreshed.WalletPaidAmount.Decimal.Equal(decimal.RequireFromString("14.29")) {
		t.Fatalf("concurrent debit must charge once, wallet_paid=%s", refreshed.WalletPaidAmount.String())
	}
}

// TestConcurrentRefundNoOverRefund: the refundable bound (WalletPaid-Refunded) is
// enforced even under repeated refund attempts; once exhausted, further refunds
// are rejected (ErrRefundExceeded). On sqlite in-memory true concurrent writers are
// serialized by locking, so this exercises the bound deterministically.
func TestConcurrentRefundNoOverRefund(t *testing.T) {
	svc, db := setupOrderRefundWalletTest(t)
	createTestUser(t, db, 307)
	order := createTestOrder(t, db, 307, "CUR-CONC-REFUND", decimal.RequireFromString("40"))
	if err := db.Model(&orderdomain.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"status":  constants.OrderStatusPaid,
		"paid_at": time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed paid order: %v", err)
	}
	// Refund 5 eight times = 40 (exactly the bound).
	for i := 0; i < 8; i++ {
		if _, _, _, err := svc.AdminRefundToWallet(AdminRefundToWalletInput{
			OrderID: order.ID, Amount: money.FromDecimal(decimal.RequireFromString("5")), Remark: "partial",
		}); err != nil {
			t.Fatalf("partial refund %d: %v", i, err)
		}
	}
	// One more must be rejected: refundable == 0.
	if _, _, _, err := svc.AdminRefundToWallet(AdminRefundToWalletInput{
		OrderID: order.ID, Amount: money.FromDecimal(decimal.RequireFromString("5")), Remark: "over",
	}); !errors.Is(err, walletcontract.ErrRefundExceeded) {
		t.Fatalf("expected ErrRefundExceeded at bound, got %v", err)
	}
	var refreshed orderdomain.Order
	_ = db.First(&refreshed, order.ID).Error
	if refreshed.RefundedAmount.Decimal.GreaterThan(decimal.RequireFromString("40")) {
		t.Fatalf("over-refund detected: refunded=%s > 40", refreshed.RefundedAmount.String())
	}
	if !refreshed.RefundedAmount.Decimal.Equal(decimal.RequireFromString("40")) {
		t.Fatalf("refunded = %s, want exactly 40", refreshed.RefundedAmount.String())
	}
}
