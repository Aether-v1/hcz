package integrationtest

import (
	"testing"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// TestLedger_CreateFreezeEntry 验证创建交易写入 c2c_freeze ledger 且快照字段正确。
func TestLedger_CreateFreezeEntry(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")

	tr := f.createTrade(t, buyerID, listingID, "100.00")

	rows := f.listLedgers(t, sellerID, constants.WalletTxnTypeC2CFreeze)
	if len(rows) != 1 {
		t.Fatalf("want 1 freeze ledger, got %d", len(rows))
	}
	row := rows[0]
	if !row.Amount.Decimal.Round(2).Equal(mustDec("100.00")) {
		t.Fatalf("freeze amount want 100.00, got %s", row.Amount.Decimal)
	}
	if !row.AvailableBefore.Decimal.Round(2).Equal(mustDec("1000.00")) {
		t.Fatalf("freeze available_before want 1000.00, got %s", row.AvailableBefore.Decimal)
	}
	if !row.AvailableAfter.Decimal.Round(2).Equal(mustDec("900.00")) {
		t.Fatalf("freeze available_after want 900.00, got %s", row.AvailableAfter.Decimal)
	}
	if !row.FrozenBefore.Decimal.Round(2).Equal(mustDec("0.00")) {
		t.Fatalf("freeze frozen_before want 0.00, got %s", row.FrozenBefore.Decimal)
	}
	if !row.FrozenAfter.Decimal.Round(2).Equal(mustDec("100.00")) {
		t.Fatalf("freeze frozen_after want 100.00, got %s", row.FrozenAfter.Decimal)
	}
	if row.Reference != "c2c_freeze:trade:"+tr.TradeNo {
		t.Fatalf("freeze reference format want c2c_freeze:trade:%s, got %s", tr.TradeNo, row.Reference)
	}
	if row.Currency != "USDT" {
		t.Fatalf("freeze currency want USDT, got %s", row.Currency)
	}
}

// TestLedger_CancelUnfreezeEntry 验证 cancel 写入 c2c_unfreeze ledger。
func TestLedger_CancelUnfreezeEntry(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")

	if _, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: tr.ID, BuyerUserID: buyerID}); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	rows := f.listLedgers(t, sellerID, constants.WalletTxnTypeC2CUnfreeze)
	if len(rows) != 1 {
		t.Fatalf("want 1 unfreeze ledger, got %d", len(rows))
	}
	row := rows[0]
	if !row.Amount.Decimal.Round(2).Equal(mustDec("100.00")) {
		t.Fatalf("unfreeze amount want 100.00, got %s", row.Amount.Decimal)
	}
	// 冻结时 available=900, frozen=100；解冻后 available=1000, frozen=0
	if !row.AvailableBefore.Decimal.Round(2).Equal(mustDec("900.00")) {
		t.Fatalf("unfreeze available_before want 900.00, got %s", row.AvailableBefore.Decimal)
	}
	if !row.AvailableAfter.Decimal.Round(2).Equal(mustDec("1000.00")) {
		t.Fatalf("unfreeze available_after want 1000.00, got %s", row.AvailableAfter.Decimal)
	}
	if !row.FrozenBefore.Decimal.Round(2).Equal(mustDec("100.00")) {
		t.Fatalf("unfreeze frozen_before want 100.00, got %s", row.FrozenBefore.Decimal)
	}
	if !row.FrozenAfter.Decimal.Round(2).Equal(mustDec("0.00")) {
		t.Fatalf("unfreeze frozen_after want 0.00, got %s", row.FrozenAfter.Decimal)
	}
	if row.Reference != "c2c_unfreeze:trade:"+tr.TradeNo {
		t.Fatalf("unfreeze reference want c2c_unfreeze:trade:%s, got %s", tr.TradeNo, row.Reference)
	}
}

// TestLedger_ConfirmSettleEntry 验证 confirm 给卖家写 c2c_settle ledger（frozen 减，available 不变）。
func TestLedger_ConfirmSettleEntry(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	rows := f.listLedgers(t, sellerID, constants.WalletTxnTypeC2CSettle)
	if len(rows) != 1 {
		t.Fatalf("want 1 settle ledger on seller, got %d", len(rows))
	}
	row := rows[0]
	// settle: source available 不变（900），frozen 从 100 → 0
	if !row.AvailableBefore.Decimal.Round(2).Equal(mustDec("900.00")) {
		t.Fatalf("settle available_before want 900.00, got %s", row.AvailableBefore.Decimal)
	}
	if !row.AvailableAfter.Decimal.Round(2).Equal(mustDec("900.00")) {
		t.Fatalf("settle available_after want 900.00 (unchanged), got %s", row.AvailableAfter.Decimal)
	}
	if !row.FrozenBefore.Decimal.Round(2).Equal(mustDec("100.00")) {
		t.Fatalf("settle frozen_before want 100.00, got %s", row.FrozenBefore.Decimal)
	}
	if !row.FrozenAfter.Decimal.Round(2).Equal(mustDec("0.00")) {
		t.Fatalf("settle frozen_after want 0.00, got %s", row.FrozenAfter.Decimal)
	}
	if row.Reference != "c2c_settle:trade:"+tr.TradeNo {
		t.Fatalf("settle reference want c2c_settle:trade:%s, got %s", tr.TradeNo, row.Reference)
	}
}

// TestLedger_ConfirmReceiveEntry 验证 confirm 给买家写 c2c_receive ledger（available 增，frozen 不变）。
func TestLedger_ConfirmReceiveEntry(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "20.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	rows := f.listLedgers(t, buyerID, constants.WalletTxnTypeC2CReceive)
	if len(rows) != 1 {
		t.Fatalf("want 1 receive ledger on buyer, got %d", len(rows))
	}
	row := rows[0]
	if !row.AvailableBefore.Decimal.Round(2).Equal(mustDec("20.00")) {
		t.Fatalf("receive available_before want 20.00, got %s", row.AvailableBefore.Decimal)
	}
	if !row.AvailableAfter.Decimal.Round(2).Equal(mustDec("120.00")) {
		t.Fatalf("receive available_after want 120.00, got %s", row.AvailableAfter.Decimal)
	}
	if !row.FrozenBefore.Decimal.Round(2).Equal(mustDec("0.00")) {
		t.Fatalf("receive frozen_before want 0.00, got %s", row.FrozenBefore.Decimal)
	}
	if !row.FrozenAfter.Decimal.Round(2).Equal(mustDec("0.00")) {
		t.Fatalf("receive frozen_after want 0.00 (unchanged), got %s", row.FrozenAfter.Decimal)
	}
	if row.Reference != "c2c_receive:trade:"+tr.TradeNo {
		t.Fatalf("receive reference want c2c_receive:trade:%s, got %s", tr.TradeNo, row.Reference)
	}
}

// TestLedger_ReferenceUniqueness 验证同一 trade 的各 ledger reference 唯一且格式正确。
func TestLedger_ReferenceUniqueness(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "0.00")
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// 收集该 trade 相关的所有 ledger reference
	type refRow struct {
		Reference string
		Type      string
		UserID    uint
	}
	var refs []refRow
	if err := f.db.Model(&ledgerRowStub{}).
		Where("reference LIKE ?", "%:"+tr.TradeNo).
		Select("reference, type, user_id").
		Scan(&refs).Error; err != nil {
		t.Fatalf("query ledger refs: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("want 3 ledger rows (freeze+settle+receive) for one trade, got %d: %+v", len(refs), refs)
	}
	seen := map[string]bool{}
	for _, r := range refs {
		if seen[r.Reference] {
			t.Fatalf("duplicate reference: %s", r.Reference)
		}
		seen[r.Reference] = true
	}
	// 3 种 reference 前缀
	wantPrefixes := map[string]bool{
		"c2c_freeze:trade:" + tr.TradeNo:  false,
		"c2c_settle:trade:" + tr.TradeNo:  false,
		"c2c_receive:trade:" + tr.TradeNo: false,
	}
	for _, r := range refs {
		if _, ok := wantPrefixes[r.Reference]; ok {
			wantPrefixes[r.Reference] = true
		}
	}
	for ref, found := range wantPrefixes {
		if !found {
			t.Fatalf("missing ledger reference: %s", ref)
		}
	}
}

// ledgerRowStub 仅用于 reference 查询（避免直接 import walletdomain 字段冲突）。
type ledgerRowStub struct {
	Reference string
	Type      string
	UserID    uint
}

func (ledgerRowStub) TableName() string { return "wallet_transactions" }

// TestLedger_TotalBalanceInvariant 验证资金动作前后总余额守恒。
func TestLedger_TotalBalanceInvariant(t *testing.T) {
	f := newFixture(t)
	listingID, sellerID, buyerID := setupTradeScenario(t, f, "1000.00", "1000.00")
	f.setBalance(t, buyerID, "50.00")

	// freeze 前：seller total = 1000+0 = 1000
	tr := f.createTrade(t, buyerID, listingID, "100.00")
	sellerAvail := f.getAvailable(t, sellerID)
	sellerFrozen := f.getFrozen(t, sellerID)
	sellerTotal := sellerAvail.Add(sellerFrozen)
	if !sellerTotal.Equal(mustDec("1000.00")) {
		t.Fatalf("after freeze seller total want 1000.00, got %s", sellerTotal)
	}

	// settle 前全局总余额 = seller total + buyer total = 1000 + 50 = 1050
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: tr.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: tr.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	sellerAvail2 := f.getAvailable(t, sellerID)
	sellerFrozen2 := f.getFrozen(t, sellerID)
	buyerAvail2 := f.getAvailable(t, buyerID)
	buyerFrozen2 := f.getFrozen(t, buyerID)
	globalTotal := sellerAvail2.Add(sellerFrozen2).Add(buyerAvail2).Add(buyerFrozen2)
	// settle 把 seller.frozen 100 转移到 buyer.available；全局总余额仍为 1050
	if !globalTotal.Equal(mustDec("1050.00")) {
		t.Fatalf("global total after settle want 1050.00, got %s", globalTotal)
	}
}

var _ = decimal.Zero
var _ = money.FromDecimal
