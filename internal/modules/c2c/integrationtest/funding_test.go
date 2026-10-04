package integrationtest

import (
	"errors"
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	"github.com/Aether-v1/hcz/internal/modules/c2c/statemachine"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// setupTradeScenario 准备一个可直接发起交易的场景：
// seller(1) 有 balance、支付方式、active listing(totalUSDT)；buyer(2) 已注册。
// 返回 (listing, sellerID, buyerID)。
func setupTradeScenario(t *testing.T, f *fixture, sellerBalance, listingTotal string) (uint, uint, uint) {
	t.Helper()
	const sellerID, buyerID = uint(1), uint(2)
	f.createUser(t, sellerID)
	f.createUser(t, buyerID)
	f.setBalance(t, sellerID, sellerBalance)
	f.createPaymentMethod(t, sellerID)
	listing := f.createListing(t, sellerID, listingTotal)
	return listing.ID, sellerID, buyerID
}

// TestFunding_CreateTradeFreezesSeller 验证创建交易真正冻结卖家 USDT。
func TestFunding_CreateTradeFreezesSeller(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")

	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if tr.Status != statemachine.StatusPendingPayment {
		t.Fatalf("expected pending_payment, got %s", tr.Status)
	}

	avail := f.getAvailable(t, sellerID)
	frozen := f.getFrozen(t, sellerID)
	if !avail.Equal(mustDec("900.00")) {
		t.Fatalf("seller available want 900.00, got %s", avail)
	}
	if !frozen.Equal(mustDec("100.00")) {
		t.Fatalf("seller frozen want 100.00, got %s", frozen)
	}
}

// TestFunding_BuyerWalletUntouched 验证创建交易不影响买家钱包。
func TestFunding_BuyerWalletUntouched(t *testing.T) {
	f := newFixture(t)
	listingID, _, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	// 给买家也设一笔余额，确认不被动
	f.setBalance(t, buyerID, "500.00")

	f.createTrade(t, buyerID, listingID, "100.00")

	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("500.00")) {
		t.Fatalf("buyer available want 500.00, got %s", got)
	}
	if got := f.getFrozen(t, buyerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("buyer frozen want 0.00, got %s", got)
	}
}

// TestFunding_MarkPaidDoesNotTouchWallet 验证 mark-paid 不动钱包。
func TestFunding_MarkPaidDoesNotTouchWallet(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "500.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")

	_, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{
		TradeID:          tr.ID,
		BuyerUserID:      buyerID,
		PaymentReference: "wire-001",
	})
	if err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("900.00")) {
		t.Fatalf("seller available unchanged want 900.00, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("seller frozen unchanged want 100.00, got %s", got)
	}
	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("500.00")) {
		t.Fatalf("buyer available unchanged want 500.00, got %s", got)
	}
	if got := f.getFrozen(t, buyerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("buyer frozen unchanged want 0.00, got %s", got)
	}
}

// TestFunding_ConfirmSettlesToBuyer 验证 confirm 把卖家冻结结算给买家。
func TestFunding_ConfirmSettlesToBuyer(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "50.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")

	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	confirmed, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Status != statemachine.StatusCompleted {
		t.Fatalf("expected completed, got %s", confirmed.Status)
	}

	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("seller frozen want 0.00, got %s", got)
	}
	// seller available 仍为 900（冻结→结算，available 不变）
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("900.00")) {
		t.Fatalf("seller available want 900.00, got %s", got)
	}
	// buyer available 50 + 100 = 150
	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("150.00")) {
		t.Fatalf("buyer available want 150.00, got %s", got)
	}
	if got := f.getFrozen(t, buyerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("buyer frozen want 0.00, got %s", got)
	}
}

// TestFunding_CancelUnfreezesSeller 验证 cancel 解冻卖家并恢复 available。
func TestFunding_CancelUnfreezesSeller(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")

	canceled, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: tr.ID, BuyerUserID: buyerID})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if canceled.Status != statemachine.StatusCanceled {
		t.Fatalf("expected canceled, got %s", canceled.Status)
	}

	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("1000.00")) {
		t.Fatalf("seller available restored want 1000.00, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("seller frozen want 0.00, got %s", got)
	}
}

// TestFunding_ExpireUnfreezesSeller 验证超时解冻卖家。
func TestFunding_ExpireUnfreezesSeller(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")

	if err := f.svc.ExpireTrade(tr.ID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	reloaded := f.getTrade(t, tr.ID)
	if reloaded.Status != statemachine.StatusExpired {
		t.Fatalf("expected expired, got %s", reloaded.Status)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("1000.00")) {
		t.Fatalf("seller available restored want 1000.00, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("seller frozen want 0.00, got %s", got)
	}
}

// TestFunding_ArbitrateReleaseSettlesToBuyer 验证仲裁放行真正结算给买家。
func TestFunding_ArbitrateReleaseSettlesToBuyer(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "10.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: buyerID, TradeID: tr.ID, Reason: "not received"}); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	outTrade, _, err := f.svc.Arbitrate(c2ccontract.ArbitrateInput{
		AdminID: 999, TradeID: tr.ID,
		Result: c2ccontract.ArbitrationResultReleaseToBuyer, Reason: "evidences show buyer paid",
	})
	if err != nil {
		t.Fatalf("arbitrate release: %v", err)
	}
	if outTrade.Status != statemachine.StatusCompleted {
		t.Fatalf("expected completed, got %s", outTrade.Status)
	}

	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("seller frozen want 0.00, got %s", got)
	}
	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("110.00")) {
		t.Fatalf("buyer available want 110.00, got %s", got)
	}
}

// TestFunding_ArbitrateReturnUnfreezesSeller 验证仲裁退回解冻卖家并恢复挂单余量。
func TestFunding_ArbitrateReturnUnfreezesSeller(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: sellerID, TradeID: tr.ID, Reason: "buyer never paid"}); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	outTrade, _, err := f.svc.Arbitrate(c2ccontract.ArbitrateInput{
		AdminID: 999, TradeID: tr.ID,
		Result: c2ccontract.ArbitrationResultReturnToSeller, Reason: "buyer no payment proof",
	})
	if err != nil {
		t.Fatalf("arbitrate return: %v", err)
	}
	if outTrade.Status != statemachine.StatusCanceled {
		t.Fatalf("expected canceled, got %s", outTrade.Status)
	}

	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("1000.00")) {
		t.Fatalf("seller available restored want 1000.00, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("seller frozen want 0.00, got %s", got)
	}
	listing := f.getListing(t, listingID)
	if got := listing.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("1000.00")) {
		t.Fatalf("listing available restored want 1000.00, got %s", got)
	}
}

// TestFunding_DuplicateCreateNoDoubleFreeze 验证同幂等键创建两次只冻结一次。
func TestFunding_DuplicateCreateNoDoubleFreeze(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")

	const key = "same-key-001"
	tr1, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID: buyerID, ListingID: listingID,
		USDTAmount: money.FromDecimal(mustDec("100.00")), IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	tr2, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID: buyerID, ListingID: listingID,
		USDTAmount: money.FromDecimal(mustDec("100.00")), IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("second create (idempotent): %v", err)
	}
	if tr1.ID != tr2.ID {
		t.Fatalf("idempotent create should return same trade, got %d vs %d", tr1.ID, tr2.ID)
	}

	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("seller frozen want 100.00 (single freeze), got %s", got)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("900.00")) {
		t.Fatalf("seller available want 900.00, got %s", got)
	}
	// 只有一条 freeze ledger
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CFreeze); n != 1 {
		t.Fatalf("want 1 freeze ledger, got %d", n)
	}
}

// TestFunding_DuplicateConfirmNoDoubleSettle 验证重复 confirm 不重复结算。
func TestFunding_DuplicateConfirmNoDoubleSettle(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	// 第二次 confirm 应失败（状态已 completed）
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err == nil {
		t.Fatal("second confirm should error, got nil")
	} else if !errors.Is(err, c2ccontract.ErrTradeStatusInvalid) {
		t.Fatalf("second confirm want ErrTradeStatusInvalid, got %v", err)
	}

	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("buyer available want 100.00 (single settle), got %s", got)
	}
	if n := f.countLedger(t, buyerID, constants.WalletTxnTypeC2CReceive); n != 1 {
		t.Fatalf("want 1 receive ledger, got %d", n)
	}
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CSettle); n != 1 {
		t.Fatalf("want 1 settle ledger, got %d", n)
	}
}

// TestFunding_DuplicateCancelNoDoubleUnfreeze 验证重复 cancel 不重复解冻。
func TestFunding_DuplicateCancelNoDoubleUnfreeze(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")

	if _, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: tr.ID, BuyerUserID: buyerID}); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	if _, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: tr.ID, BuyerUserID: buyerID}); err == nil {
		t.Fatal("second cancel should error")
	} else if !errors.Is(err, c2ccontract.ErrTradeStatusInvalid) {
		t.Fatalf("second cancel want ErrTradeStatusInvalid, got %v", err)
	}

	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("1000.00")) {
		t.Fatalf("seller available want 1000.00 (single unfreeze), got %s", got)
	}
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CUnfreeze); n != 1 {
		t.Fatalf("want 1 unfreeze ledger, got %d", n)
	}
}

// TestFunding_DuplicateArbitrateNoDoubleMoney 验证同 dispute 仲裁两次不重复资金动作。
func TestFunding_DuplicateArbitrateNoDoubleMoney(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: buyerID, TradeID: tr.ID, Reason: "r"}); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	// 第一次：release_to_buyer
	if _, _, err := f.svc.Arbitrate(c2ccontract.ArbitrateInput{
		AdminID: 1, TradeID: tr.ID, Result: c2ccontract.ArbitrationResultReleaseToBuyer, Reason: "r1",
	}); err != nil {
		t.Fatalf("first arbitrate: %v", err)
	}
	// 第二次：同 result，应幂等返回（dispute 已 resolved）
	outTrade, _, err := f.svc.Arbitrate(c2ccontract.ArbitrateInput{
		AdminID: 2, TradeID: tr.ID, Result: c2ccontract.ArbitrationResultReleaseToBuyer, Reason: "r2",
	})
	if err != nil {
		t.Fatalf("second arbitrate should be idempotent, got %v", err)
	}
	if outTrade.Status != statemachine.StatusCompleted {
		t.Fatalf("expected completed, got %s", outTrade.Status)
	}

	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("buyer available want 100.00 (single receive), got %s", got)
	}
	if n := f.countLedger(t, buyerID, constants.WalletTxnTypeC2CReceive); n != 1 {
		t.Fatalf("want 1 receive ledger, got %d", n)
	}
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CSettle); n != 1 {
		t.Fatalf("want 1 settle ledger, got %d", n)
	}
}

// TestFunding_ExpireAfterPaidDoesNotTouchMoney 验证 paid 后调用 ExpireTrade 不动资金。
func TestFunding_ExpireAfterPaidDoesNotTouchMoney(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	// 直接调用 ExpireTrade：paid 状态下应幂等返回，不动资金
	if err := f.svc.ExpireTrade(tr.ID); err != nil {
		t.Fatalf("expire after paid: %v", err)
	}
	reloaded := f.getTrade(t, tr.ID)
	if reloaded.Status != statemachine.StatusPaid {
		t.Fatalf("trade should stay paid, got %s", reloaded.Status)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("900.00")) {
		t.Fatalf("seller available unchanged want 900.00, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("seller frozen unchanged want 100.00, got %s", got)
	}
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CUnfreeze); n != 0 {
		t.Fatalf("want 0 unfreeze ledger, got %d", n)
	}
}

// TestFunding_ListingAvailableDecrementAndRestore 验证挂单 available_usdt 扣减与恢复。
func TestFunding_ListingAvailableDecrementAndRestore(t *testing.T) {
	f := newFixture(t)
	listingID, _, buyerID := setupTradeScenario(t, f, "1000.00", "500.00")

	// 创建两笔交易：扣减 100 + 200
	tr1 := f.createTrade(t, buyerID, listingID, "100.00")
	// 第二笔需要不同幂等键（fixture.createTrade 自动根据 amount 生成，但 buyer+listing 相同会撞键）
	// 手动构造第二笔
	tr2, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID: buyerID, ListingID: listingID,
		USDTAmount: money.FromDecimal(mustDec("200.00")), IdempotencyKey: "manual-key-200",
	})
	if err != nil {
		t.Fatalf("second trade: %v", err)
	}

	listing := f.getListing(t, listingID)
	if got := listing.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("200.00")) {
		t.Fatalf("after 100+200 trades, listing available want 200.00, got %s", got)
	}

	// cancel 第一笔 → 恢复 100
	if _, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: tr1.ID, BuyerUserID: buyerID}); err != nil {
		t.Fatalf("cancel tr1: %v", err)
	}
	listing = f.getListing(t, listingID)
	if got := listing.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("300.00")) {
		t.Fatalf("after cancel tr1, listing available want 300.00, got %s", got)
	}

	// expire 第二笔 → 恢复 200
	if err := f.svc.ExpireTrade(tr2.ID); err != nil {
		t.Fatalf("expire tr2: %v", err)
	}
	listing = f.getListing(t, listingID)
	if got := listing.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("500.00")) {
		t.Fatalf("after expire tr2, listing available restored to 500.00, got %s", got)
	}
}

// 确保 decimal 包被使用（避免 import 未用）
var _ = decimal.Zero
