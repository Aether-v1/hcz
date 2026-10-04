package integrationtest

import (
	"errors"
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"
	totpapplication "github.com/Aether-v1/hcz/internal/modules/identity/totp/application"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

var testAddress = validTRC20Address("410000000000000000000000000000000000000001")

func TestCreateWithdrawal_Success(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")

	w, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "123456",
		IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatalf("create withdrawal: %v", err)
	}
	if w.Status != withdrawaldomain.StatusPending {
		t.Fatalf("expected pending, got %s", w.Status)
	}
	// fee = 1.00 fixed, net = 49.00
	if !w.FeeAmount.Decimal.Equal(mustDec("1.00")) {
		t.Fatalf("expected fee 1.00, got %s", w.FeeAmount.Decimal)
	}
	if !w.NetAmount.Decimal.Equal(mustDec("49.00")) {
		t.Fatalf("expected net 49.00, got %s", w.NetAmount.Decimal)
	}
	// balance = 100 - 50 = 50
	bal := f.getBalance(t, 1)
	if !bal.Equal(mustDec("50.00")) {
		t.Fatalf("expected balance 50.00, got %s", bal)
	}
	// ledger check
	var txns []walletdomain.Transaction
	f.db.Model(&walletdomain.Transaction{}).Where("user_id = ?", 1).Find(&txns)
	if len(txns) != 1 {
		t.Fatalf("expected 1 ledger, got %d", len(txns))
	}
	if txns[0].Type != constants.WalletTxnTypeWithdrawalDebit {
		t.Fatalf("expected withdrawal_debit, got %s", txns[0].Type)
	}
	if !txns[0].BalanceBefore.Decimal.Equal(mustDec("100.00")) {
		t.Fatalf("expected balance_before 100, got %s", txns[0].BalanceBefore.Decimal)
	}
	if !txns[0].BalanceAfter.Decimal.Equal(mustDec("50.00")) {
		t.Fatalf("expected balance_after 50, got %s", txns[0].BalanceAfter.Decimal)
	}
	if txns[0].Currency != "USDT" {
		t.Fatalf("expected USDT, got %s", txns[0].Currency)
	}
	// notification enqueued
	found := false
	for _, e := range f.notifier.events {
		if e == constants.NotificationEventWithdrawalSubmitted {
			found = true
		}
	}
	if !found {
		t.Fatal("expected withdrawal_submitted notification")
	}
}

func TestCreateWithdrawal_TOTPInvalid(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	_, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "wrong",
		IdempotencyKey: "key-totp",
	})
	if !errors.Is(err, withdrawalcontract.ErrTOTPInvalid) {
		t.Fatalf("expected ErrTOTPInvalid, got %v", err)
	}
	// balance unchanged
	if !f.getBalance(t, 1).Equal(mustDec("100.00")) {
		t.Fatal("balance should be unchanged")
	}
}

func TestCreateWithdrawal_TOTPNotEnabled_FailClosed(t *testing.T) {
	f := newFixture(t)
	f.totp.enabled = false
	f.setBalance(t, 1, "100.00")
	_, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "123456",
		IdempotencyKey: "key-disabled",
	})
	if !errors.Is(err, withdrawalcontract.ErrTOTPNotEnabled) {
		t.Fatalf("expected ErrTOTPNotEnabled, got %v", err)
	}
}

func TestCreateWithdrawal_InsufficientBalance(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "10.00")
	_, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         1,
		Network:        "TRC20",
		Address:        testAddress,
		Amount:         money.FromDecimal(mustDec("50.00")),
		TOTPCode:       "123456",
		IdempotencyKey: "key-insuff",
	})
	if !errors.Is(err, withdrawalcontract.ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestCreateWithdrawal_MinMax(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "1000.00")
	// too small
	_, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("5.00")), TOTPCode: "123456", IdempotencyKey: "key-min",
	})
	if !errors.Is(err, withdrawalcontract.ErrAmountTooSmall) {
		t.Fatalf("expected ErrAmountTooSmall, got %v", err)
	}
	// too large
	_, err = f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50000.00")), TOTPCode: "123456", IdempotencyKey: "key-max",
	})
	if !errors.Is(err, withdrawalcontract.ErrAmountTooLarge) {
		t.Fatalf("expected ErrAmountTooLarge, got %v", err)
	}
}

func TestCreateWithdrawal_InvalidAddress(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "1000.00")
	_, err := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: "not-a-valid-address",
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "key-badaddr",
	})
	if !errors.Is(err, withdrawalcontract.ErrInvalidAddress) {
		t.Fatalf("expected ErrInvalidAddress, got %v", err)
	}
}

func TestCreateWithdrawal_DuplicateIdempotency(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	in := withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "idem-1",
	}
	w1, err := f.svc.CreateWithdrawal(in)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	w2, err := f.svc.CreateWithdrawal(in)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if w1.ID != w2.ID {
		t.Fatal("expected same withdrawal returned on idempotent replay")
	}
	// balance still 50 (only debited once)
	if !f.getBalance(t, 1).Equal(mustDec("50.00")) {
		t.Fatalf("expected balance 50, got %s", f.getBalance(t, 1))
	}
}

func TestCancelWithdrawal_Refund(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	w, _ := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "key-cancel",
	})
	// balance = 50
	cw, err := f.svc.CancelWithdrawal(withdrawalcontract.CancelWithdrawalInput{
		UserID: 1, ID: w.ID, TOTPCode: "123456",
	})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cw.Status != withdrawaldomain.StatusCanceled {
		t.Fatalf("expected canceled, got %s", cw.Status)
	}
	// balance restored to 100 (refund request_amount = 50)
	if !f.getBalance(t, 1).Equal(mustDec("100.00")) {
		t.Fatalf("expected balance 100 after refund, got %s", f.getBalance(t, 1))
	}
}

func TestRejectWithdrawal_Refund(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	w, _ := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "key-reject",
	})
	rw, err := f.svc.Reject(withdrawalcontract.AdminReviewInput{
		ID: w.ID, AdminID: 99, RejectReason: "fraud",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rw.Status != withdrawaldomain.StatusRejected {
		t.Fatalf("expected rejected, got %s", rw.Status)
	}
	if !f.getBalance(t, 1).Equal(mustDec("100.00")) {
		t.Fatalf("expected balance 100 after reject refund, got %s", f.getBalance(t, 1))
	}
}

func TestReject_DuplicateNoDoubleRefund(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	w, _ := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "key-dbl",
	})
	_, _ = f.svc.Reject(withdrawalcontract.AdminReviewInput{ID: w.ID, AdminID: 99, RejectReason: "fraud"})
	// second reject should fail state machine (already rejected terminal)
	_, err := f.svc.Reject(withdrawalcontract.AdminReviewInput{ID: w.ID, AdminID: 99, RejectReason: "again"})
	if !errors.Is(err, withdrawalcontract.ErrWithdrawalStatusInvalid) {
		t.Fatalf("expected status invalid on duplicate reject, got %v", err)
	}
	// balance = 100 (refunded once)
	if !f.getBalance(t, 1).Equal(mustDec("100.00")) {
		t.Fatalf("expected balance 100, got %s", f.getBalance(t, 1))
	}
}

func TestAdminLifecycle(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	w, _ := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "key-lifecycle",
	})
	// approve
	aw, err := f.svc.Approve(withdrawalcontract.AdminReviewInput{ID: w.ID, AdminID: 99})
	if err != nil || aw.Status != withdrawaldomain.StatusApproved {
		t.Fatalf("approve: %v status=%s", err, aw.Status)
	}
	// processing
	pw, err := f.svc.MarkProcessing(withdrawalcontract.AdminProcessingInput{ID: w.ID, AdminID: 99})
	if err != nil || pw.Status != withdrawaldomain.StatusProcessing {
		t.Fatalf("processing: %v", err)
	}
	// complete without txid should fail
	_, err = f.svc.Complete(withdrawalcontract.AdminCompleteInput{ID: w.ID, AdminID: 99})
	if !errors.Is(err, withdrawalcontract.ErrTxidRequired) {
		t.Fatalf("expected ErrTxidRequired, got %v", err)
	}
	// complete with txid
	cw, err := f.svc.Complete(withdrawalcontract.AdminCompleteInput{ID: w.ID, AdminID: 99, Txid: "0xabc"})
	if err != nil || cw.Status != withdrawaldomain.StatusCompleted {
		t.Fatalf("complete: %v", err)
	}
	if cw.Txid != "0xabc" {
		t.Fatalf("expected txid 0xabc, got %s", cw.Txid)
	}
}

func TestIllegalTransition(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	w, _ := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "key-illegal",
	})
	// cannot complete directly from pending
	_, err := f.svc.Complete(withdrawalcontract.AdminCompleteInput{ID: w.ID, AdminID: 99, Txid: "x"})
	if !errors.Is(err, withdrawalcontract.ErrWithdrawalStatusInvalid) {
		t.Fatalf("expected illegal transition, got %v", err)
	}
}

func TestUserIDOR(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	f.setBalance(t, 2, "100.00")
	w, _ := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("50.00")), TOTPCode: "123456", IdempotencyKey: "key-idor",
	})
	// user 2 cannot see user 1's withdrawal
	_, err := f.svc.GetUserWithdrawalDetail(2, w.ID)
	if !errors.Is(err, withdrawalcontract.ErrWithdrawalNotFound) {
		t.Fatalf("expected ErrWithdrawalNotFound for IDOR, got %v", err)
	}
	// user 2 cannot cancel user 1's withdrawal
	_, err = f.svc.CancelWithdrawal(withdrawalcontract.CancelWithdrawalInput{UserID: 2, ID: w.ID, TOTPCode: "123456"})
	if !errors.Is(err, withdrawalcontract.ErrWithdrawalNotFound) {
		t.Fatalf("expected ErrWithdrawalNotFound for IDOR cancel, got %v", err)
	}
}

func TestFeePercentage(t *testing.T) {
	f := newFixture(t)
	f.config.cfg.FixedFee = "1.00"
	f.config.cfg.PercentageFee = "0.05" // 5%
	f.setBalance(t, 1, "1000.00")
	w, _ := f.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID: 1, Network: "TRC20", Address: testAddress,
		Amount: money.FromDecimal(mustDec("100.00")), TOTPCode: "123456", IdempotencyKey: "key-fee",
	})
	// fee = 1 + 100*0.05 = 6.00, net = 94.00
	if !w.FeeAmount.Decimal.Equal(mustDec("6.00")) {
		t.Fatalf("expected fee 6.00, got %s", w.FeeAmount.Decimal)
	}
	if !w.NetAmount.Decimal.Equal(mustDec("94.00")) {
		t.Fatalf("expected net 94.00, got %s", w.NetAmount.Decimal)
	}
}

func TestQuoteFee_NoDebit(t *testing.T) {
	f := newFixture(t)
	f.setBalance(t, 1, "100.00")
	quote, err := f.svc.QuoteFee("TRC20", money.FromDecimal(mustDec("50.00")))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if !quote.FeeAmount.Decimal.Equal(mustDec("1.00")) {
		t.Fatalf("expected fee 1.00, got %s", quote.FeeAmount.Decimal)
	}
	// balance unchanged
	if !f.getBalance(t, 1).Equal(mustDec("100.00")) {
		t.Fatal("quote must not debit balance")
	}
}

var _ = totpapplication.ErrCodeInvalid
var _ = decimal.Zero
