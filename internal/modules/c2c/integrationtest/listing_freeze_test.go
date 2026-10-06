package integrationtest

import (
	"errors"
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	"github.com/Aether-v1/hcz/internal/modules/c2c/statemachine"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// helper: 直接用 service 创建挂单（失败 fatal）。
func mustCreateListing(t *testing.T, f *fixture, sellerID uint, total string) *c2cdomain.Listing {
	t.Helper()
	l, err := f.svc.CreateListing(c2ccontract.CreateListingInput{
		UserID: sellerID, FiatCurrency: "CNY",
		Price:         money.FromDecimal(mustDec("7.00")),
		MinFiatAmount: money.FromDecimal(mustDec("1.00")),
		MaxFiatAmount: money.FromDecimal(mustDec("100000.00")),
		TotalUSDT:     money.FromDecimal(mustDec(total)),
	})
	if err != nil {
		t.Fatalf("create listing seller=%d total=%s: %v", sellerID, total, err)
	}
	return l
}

// 1. available=100, create listing=60 → available=40, frozen=60
func TestListingFreeze_CreateListingFreezesFunds(t *testing.T) {
	f := newFixture(t)
	sellerID := uint(1)
	f.createUser(t, sellerID)
	f.setBalance(t, sellerID, "100.00")
	f.createPaymentMethod(t, sellerID)

	l := mustCreateListing(t, f, sellerID, "60.00")
	if l.ID == 0 {
		t.Fatal("listing id should be set")
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("40.00")) {
		t.Fatalf("available want 40, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("60.00")) {
		t.Fatalf("frozen want 60, got %s", got)
	}
}

// 2. available=50, create listing=60 → ErrInsufficientBalance
func TestListingFreeze_InsufficientAvailableRejects(t *testing.T) {
	f := newFixture(t)
	sellerID := uint(1)
	f.createUser(t, sellerID)
	f.setBalance(t, sellerID, "50.00")
	f.createPaymentMethod(t, sellerID)

	_, err := f.svc.CreateListing(c2ccontract.CreateListingInput{
		UserID: sellerID, FiatCurrency: "CNY",
		Price:         money.FromDecimal(mustDec("7.00")),
		MinFiatAmount: money.FromDecimal(mustDec("1.00")),
		MaxFiatAmount: money.FromDecimal(mustDec("100000.00")),
		TotalUSDT:     money.FromDecimal(mustDec("60.00")),
	})
	if !errors.Is(err, c2ccontract.ErrInsufficientBalance) {
		t.Fatalf("want ErrInsufficientBalance, got %v", err)
	}
	// wallet 不变
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("50.00")) {
		t.Fatalf("available unchanged want 50, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("frozen unchanged want 0, got %s", got)
	}
}

// 3. available=100, listing A=60 成功, listing B=50 失败
func TestListingFreeze_SecondListingAfterFirst(t *testing.T) {
	f := newFixture(t)
	sellerID := uint(1)
	f.createUser(t, sellerID)
	f.setBalance(t, sellerID, "100.00")
	f.createPaymentMethod(t, sellerID)

	mustCreateListing(t, f, sellerID, "60.00")
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("40.00")) {
		t.Fatalf("after A available want 40, got %s", got)
	}

	_, err := f.svc.CreateListing(c2ccontract.CreateListingInput{
		UserID: sellerID, FiatCurrency: "CNY",
		Price:         money.FromDecimal(mustDec("7.00")),
		MinFiatAmount: money.FromDecimal(mustDec("1.00")),
		MaxFiatAmount: money.FromDecimal(mustDec("100000.00")),
		TotalUSDT:     money.FromDecimal(mustDec("50.00")),
	})
	if !errors.Is(err, c2ccontract.ErrInsufficientBalance) {
		t.Fatalf("second listing want ErrInsufficientBalance, got %v", err)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("40.00")) {
		t.Fatalf("available unchanged want 40, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("60.00")) {
		t.Fatalf("frozen unchanged want 60, got %s", got)
	}
}

// 4. listing=100 frozen=100, create trade=30 → frozen 仍=100
func TestListingFreeze_CreateTradeNoDoubleFreeze(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.createTrade(t, buyerID, listingID, "30.00")
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("frozen want 100, got %s", got)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("available want 0, got %s", got)
	}
}

// 5. trade cancel 后 frozen 不变, listing available 恢复
func TestListingFreeze_CancelTradeDoesNotUnfreeze(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	tr := f.createTrade(t, buyerID, listingID, "30.00")
	if _, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: tr.ID, BuyerUserID: buyerID}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("frozen unchanged want 100, got %s", got)
	}
	l := f.getListing(t, listingID)
	if got := l.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("100.00")) {
		t.Fatalf("listing available restored want 100, got %s", got)
	}
}

// 6. expire trade 后 frozen 不变, listing available 恢复
func TestListingFreeze_ExpireTradeDoesNotUnfreeze(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	tr := f.createTrade(t, buyerID, listingID, "30.00")
	if err := f.svc.ExpireTrade(tr.ID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("frozen unchanged want 100, got %s", got)
	}
	l := f.getListing(t, listingID)
	if got := l.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("100.00")) {
		t.Fatalf("listing available restored want 100, got %s", got)
	}
}

// 7. confirm 后 frozen=100-30=70, buyer +=30
func TestListingFreeze_SettleDeductsFrozen(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "30.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("70.00")) {
		t.Fatalf("seller frozen want 70, got %s", got)
	}
	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("30.00")) {
		t.Fatalf("buyer available want 30, got %s", got)
	}
}

// 8. partial settlement: listing=100, trade A=30 completed, trade B=20 completed → frozen=50
func TestListingFreeze_PartialSettlement(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.setBalance(t, buyerID, "0.00")

	trA := f.createTrade(t, buyerID, listingID, "30.00")
	trB, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID: buyerID, ListingID: listingID,
		USDTAmount: money.FromDecimal(mustDec("20.00")), IdempotencyKey: "partial-b",
	})
	if err != nil {
		t.Fatalf("create trade B: %v", err)
	}

	for _, tr := range []*c2cdomain.Trade{trA, trB} {
		if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
			t.Fatalf("mark paid: %v", err)
		}
		if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
			t.Fatalf("confirm: %v", err)
		}
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("50.00")) {
		t.Fatalf("seller frozen want 50, got %s", got)
	}
	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("50.00")) {
		t.Fatalf("buyer available want 50, got %s", got)
	}
}

// 9. multiple trades: listing=100, trade A=30 pending, trade B=20 pending → listing available=50, frozen=100
func TestListingFreeze_MultipleTrades(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.createTrade(t, buyerID, listingID, "30.00")
	_, err := f.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID: buyerID, ListingID: listingID,
		USDTAmount: money.FromDecimal(mustDec("20.00")), IdempotencyKey: "multi-b",
	})
	if err != nil {
		t.Fatalf("create trade B: %v", err)
	}
	l := f.getListing(t, listingID)
	if got := l.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("50.00")) {
		t.Fatalf("listing available want 50, got %s", got)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("seller frozen want 100, got %s", got)
	}
}

// 10. close listing (no trade) → frozen=0, available 恢复
func TestListingFreeze_CloseUnfreezesRemaining(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, _ := setupTradeScenario(t, f, "100.00", "100.00")
	if _, err := f.svc.CloseListing(sellerID, listingID); err != nil {
		t.Fatalf("close: %v", err)
	}
	l := f.getListing(t, listingID)
	if l.Status != "closed" {
		t.Fatalf("status want closed, got %s", l.Status)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("frozen want 0 after close, got %s", got)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("available restored want 100, got %s", got)
	}
}

// 11. close after partial settlement: listing=100, 30 settled, close → unfreeze 70
func TestListingFreeze_CloseAfterPartialSettlement(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "30.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	// listing available = 70 (100-30), frozen = 70
	if _, err := f.svc.CloseListing(sellerID, listingID); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("frozen want 0 after close, got %s", got)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("70.00")) {
		t.Fatalf("available restored want 70, got %s", got)
	}
}

// 12. duplicate close: 两次 close，第二次幂等，只解冻一次
func TestListingFreeze_DuplicateClose(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, _ := setupTradeScenario(t, f, "100.00", "100.00")
	if _, err := f.svc.CloseListing(sellerID, listingID); err != nil {
		t.Fatalf("first close: %v", err)
	}
	// 第二次 close 应幂等返回（不报错，不重复解冻）
	if _, err := f.svc.CloseListing(sellerID, listingID); err != nil {
		t.Fatalf("second close should be idempotent, got %v", err)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("0.00")) {
		t.Fatalf("frozen want 0, got %s", got)
	}
	if got := f.getAvailable(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("available want 100, got %s", got)
	}
	// 只应有一条 unfreeze ledger
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CUnfreeze); n != 1 {
		t.Fatalf("want 1 unfreeze ledger, got %d", n)
	}
}

// 13. active trade blocks close → ErrListingNotClosable
func TestListingFreeze_ActiveTradeBlocksClose(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.createTrade(t, buyerID, listingID, "30.00")
	_, err := f.svc.CloseListing(sellerID, listingID)
	if !errors.Is(err, c2ccontract.ErrListingNotClosable) {
		t.Fatalf("want ErrListingNotClosable, got %v", err)
	}
	// wallet 不变
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("frozen unchanged want 100, got %s", got)
	}
	l := f.getListing(t, listingID)
	if l.Status != "active" {
		t.Fatalf("listing should stay active, got %s", l.Status)
	}
}

// 14. disputed 状态 listing frozen 不变
func TestListingFreeze_ArbitrationKeepsFrozen(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	tr := f.createTrade(t, buyerID, listingID, "30.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: buyerID, TradeID: tr.ID, Reason: "r"}); err != nil {
		t.Fatalf("dispute: %v", err)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("frozen unchanged want 100, got %s", got)
	}
}

// 15. seller wins arbitration (return_to_seller) → frozen 不变, listing available 恢复
func TestListingFreeze_SellerWinsArbitration(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	tr := f.createTrade(t, buyerID, listingID, "30.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: sellerID, TradeID: tr.ID, Reason: "r"}); err != nil {
		t.Fatalf("dispute: %v", err)
	}
	outTrade, _, err := f.svc.Arbitrate(c2ccontract.ArbitrateInput{
		AdminID: 999, TradeID: tr.ID, Result: c2ccontract.ArbitrationResultReturnToSeller, Reason: "r",
	})
	if err != nil {
		t.Fatalf("arbitrate return: %v", err)
	}
	if outTrade.Status != statemachine.StatusCanceled {
		t.Fatalf("want canceled, got %s", outTrade.Status)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("100.00")) {
		t.Fatalf("frozen unchanged want 100, got %s", got)
	}
	l := f.getListing(t, listingID)
	if got := l.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("100.00")) {
		t.Fatalf("listing available restored want 100, got %s", got)
	}
}

// 16. buyer wins arbitration (release_to_buyer) → frozen -= tradeAmount, buyer += amount
func TestListingFreeze_BuyerWinsArbitration(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "30.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{UserID: buyerID, TradeID: tr.ID, Reason: "r"}); err != nil {
		t.Fatalf("dispute: %v", err)
	}
	outTrade, _, err := f.svc.Arbitrate(c2ccontract.ArbitrateInput{
		AdminID: 999, TradeID: tr.ID, Result: c2ccontract.ArbitrationResultReleaseToBuyer, Reason: "r",
	})
	if err != nil {
		t.Fatalf("arbitrate release: %v", err)
	}
	if outTrade.Status != statemachine.StatusCompleted {
		t.Fatalf("want completed, got %s", outTrade.Status)
	}
	if got := f.getFrozen(t, sellerID); !got.Equal(mustDec("70.00")) {
		t.Fatalf("seller frozen want 70, got %s", got)
	}
	if got := f.getAvailable(t, buyerID); !got.Equal(mustDec("30.00")) {
		t.Fatalf("buyer available want 30, got %s", got)
	}
}

// 17. wallet total invariant: create/close/settle 各阶段 total=available+frozen
func TestListingFreeze_WalletTotalInvariant(t *testing.T) {
	f := newFixture(t)
	sellerID := uint(1)
	f.createUser(t, sellerID)
	f.setBalance(t, sellerID, "100.00")
	f.createPaymentMethod(t, sellerID)

	// initial: total=100
	if got := f.getAvailable(t, sellerID).Add(f.getFrozen(t, sellerID)); !got.Equal(mustDec("100.00")) {
		t.Fatalf("initial total want 100, got %s", got)
	}
	l := mustCreateListing(t, f, sellerID, "60.00")
	// after listing: total=100
	if got := f.getAvailable(t, sellerID).Add(f.getFrozen(t, sellerID)); !got.Equal(mustDec("100.00")) {
		t.Fatalf("after listing total want 100, got %s", got)
	}
	// close listing: total=100
	if _, err := f.svc.CloseListing(sellerID, l.ID); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := f.getAvailable(t, sellerID).Add(f.getFrozen(t, sellerID)); !got.Equal(mustDec("100.00")) {
		t.Fatalf("after close total want 100, got %s", got)
	}
}

// 18. ledger reference unique: 重复 CreateListing 同 listingNo 不重复冻结（测试 reference 幂等）
// 注：listingNo 由 genNo 生成，外部无法直接指定；这里验证 wallet.Freeze 的 reference 幂等行为
// 通过重复调用 MigrateListingsToListingFreeze 验证（它对已有 reference 跳过/幂等）。
func TestListingFreeze_LedgerReferenceUnique(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, _ := setupTradeScenario(t, f, "100.00", "100.00")
	_ = listingID

	// 第一次迁移：listing 已是新模型（已 freeze），应幂等跳过
	stats, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	// 第二次迁移：仍应幂等
	stats2, err := f.svc.MigrateListingsToListingFreeze()
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	// 两次后 freeze ledger 仍只有一条
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CFreeze); n != 1 {
		t.Fatalf("want 1 freeze ledger after double migrate, got %d", n)
	}
	_ = stats
	_ = stats2
}

// 19. rollback on freeze failure: 模拟 freeze 失败时 listing 不残留
// 通过创建一个余额不足的 listing 来验证（Freeze 返回 ErrInsufficientBalance，事务回滚，listing 不残留）
func TestListingFreeze_RollbackOnFreezeFailure(t *testing.T) {
	f := newFixture(t)
	sellerID := uint(1)
	f.createUser(t, sellerID)
	f.setBalance(t, sellerID, "50.00")
	f.createPaymentMethod(t, sellerID)

	_, err := f.svc.CreateListing(c2ccontract.CreateListingInput{
		UserID: sellerID, FiatCurrency: "CNY",
		Price:         money.FromDecimal(mustDec("7.00")),
		MinFiatAmount: money.FromDecimal(mustDec("1.00")),
		MaxFiatAmount: money.FromDecimal(mustDec("100000.00")),
		TotalUSDT:     money.FromDecimal(mustDec("60.00")),
	})
	if !errors.Is(err, c2ccontract.ErrInsufficientBalance) {
		t.Fatalf("want ErrInsufficientBalance, got %v", err)
	}
	// listing 表应无残留
	var count int64
	f.db.Model(&c2cdomain.Listing{}).Count(&count)
	if count != 0 {
		t.Fatalf("want 0 listings after rollback, got %d", count)
	}
	// wallet 无 freeze ledger
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CFreeze); n != 0 {
		t.Fatalf("want 0 freeze ledger, got %d", n)
	}
}

// 20. close with zero remaining: listing 全部成交后 close, unfreeze 0, 不报错
func TestListingFreeze_CloseWithZeroRemaining(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "100.00", "100.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	// listing available = 0, frozen = 0
	l := f.getListing(t, listingID)
	if got := l.AvailableUSDT.Decimal.Round(2); !got.Equal(mustDec("0.00")) {
		t.Fatalf("listing available want 0, got %s", got)
	}
	// close: unfreeze 0, 不应报错
	if _, err := f.svc.CloseListing(sellerID, listingID); err != nil {
		t.Fatalf("close with zero remaining should not error, got %v", err)
	}
	l = f.getListing(t, listingID)
	if l.Status != "closed" {
		t.Fatalf("status want closed, got %s", l.Status)
	}
	// 不应有 unfreeze ledger（remaining=0 时跳过）
	if n := f.countLedger(t, sellerID, constants.WalletTxnTypeC2CUnfreeze); n != 0 {
		t.Fatalf("want 0 unfreeze ledger (zero remaining), got %d", n)
	}
}

var _ = decimal.Zero
