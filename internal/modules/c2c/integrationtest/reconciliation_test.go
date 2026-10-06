// Package integrationtest — Phase 10 只读对账（Wallet + C2C）。
//
// 所有函数只读：只 SELECT，不写库。返回 issues 列表，空列表 = PASS。
// 对应 RECONCILIATION_REPORT.md。
package integrationtest

import (
	"strconv"
	"testing"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// Wallet 对账
// ---------------------------------------------------------------------------

// reconcileWalletAccounts 逐账户校验：
//  1. available + frozen == 最新一笔 ledger 的 balance_after
//  2. frozen == SUM(c2c_freeze) - SUM(c2c_unfreeze) - SUM(c2c_settle)
//  3. 无负余额
func reconcileWalletAccounts(f *fixture) []string {
	var issues []string

	var accounts []walletdomain.Account
	if err := f.db.Find(&accounts).Error; err != nil {
		return []string{"wallet_accounts query: " + err.Error()}
	}
	for _, acc := range accounts {
		total := acc.AvailableBalance.Decimal.Add(acc.FrozenBalance.Decimal).Round(2)
		// 最新 ledger
		var latest walletdomain.Transaction
		res := f.db.Where("user_id = ?", acc.UserID).Order("id DESC").Limit(1).Find(&latest)
		if res.Error != nil {
			issues = append(issues, "user "+strconv.Itoa(int(acc.UserID))+" latest tx: "+res.Error.Error())
			continue
		}
		if res.RowsAffected > 0 {
			if !total.Equal(latest.BalanceAfter.Decimal.Round(2)) {
				issues = append(issues,
					"user "+strconv.Itoa(int(acc.UserID))+" TOTAL MISMATCH: available("+
						acc.AvailableBalance.Decimal.String()+")+frozen("+
						acc.FrozenBalance.Decimal.String()+")="+total.String()+
						" != ledger balance_after="+latest.BalanceAfter.Decimal.String())
			}
			if !acc.FrozenBalance.Decimal.Round(2).Equal(latest.FrozenAfter.Decimal.Round(2)) {
				issues = append(issues,
					"user "+strconv.Itoa(int(acc.UserID))+" FROZEN vs ledger mismatch: account="+
						acc.FrozenBalance.Decimal.String()+" latest.frozen_after="+
						latest.FrozenAfter.Decimal.String())
			}
		}
		// frozen == freeze - unfreeze - settle
		var freezeSum, unfreezeSum, settleSum decimal.Decimal
		var txns []walletdomain.Transaction
		f.db.Where("user_id = ?", acc.UserID).Find(&txns)
		for _, tx := range txns {
			switch tx.Type {
			case "c2c_freeze":
				freezeSum = freezeSum.Add(tx.Amount.Decimal)
			case "c2c_unfreeze":
				unfreezeSum = unfreezeSum.Add(tx.Amount.Decimal)
			case "c2c_settle":
				settleSum = settleSum.Add(tx.Amount.Decimal)
			}
		}
		computed := freezeSum.Sub(unfreezeSum).Sub(settleSum).Round(2)
		if !computed.Equal(acc.FrozenBalance.Decimal.Round(2)) {
			issues = append(issues,
				"user "+strconv.Itoa(int(acc.UserID))+" FROZEN INCONSISTENT: account.frozen="+
					acc.FrozenBalance.Decimal.String()+" computed(freeze"+
					freezeSum.String()+"-unfreeze"+unfreezeSum.String()+"-settle"+
					settleSum.String()+")="+computed.String())
		}
		// 无负余额
		if acc.AvailableBalance.Decimal.Sign() < 0 {
			issues = append(issues, "user "+strconv.Itoa(int(acc.UserID))+" NEGATIVE available: "+acc.AvailableBalance.Decimal.String())
		}
		if acc.FrozenBalance.Decimal.Sign() < 0 {
			issues = append(issues, "user "+strconv.Itoa(int(acc.UserID))+" NEGATIVE frozen: "+acc.FrozenBalance.Decimal.String())
		}
	}
	return issues
}

// ---------------------------------------------------------------------------
// C2C 对账
// ---------------------------------------------------------------------------

// listingNetFrozen 计算一个挂单当前实际占用的冻结本金：
//
//	= c2c_freeze:listing:{no}.amount
//	  - Σ(c2c_settle:trade:{tradeNo} for 已成交 trade under this listing)
//	  - c2c_unfreeze:listing:{no}.amount
func listingNetFrozen(f *fixture, l *c2cdomain.Listing) decimal.Decimal {
	net := decimal.Zero
	if txn, _ := f.walletDB.GetTransactionByReference("c2c_freeze:listing:" + l.ListingNo); txn != nil {
		net = net.Add(txn.Amount.Decimal)
	}
	if txn, _ := f.walletDB.GetTransactionByReference("c2c_unfreeze:listing:" + l.ListingNo); txn != nil {
		net = net.Sub(txn.Amount.Decimal)
	}
	// 该 listing 下所有 trade 的 settle
	var trades []c2cdomain.Trade
	f.db.Where("listing_id = ?", l.ID).Find(&trades)
	for _, tr := range trades {
		if txn, _ := f.walletDB.GetTransactionByReference("c2c_settle:trade:" + tr.TradeNo); txn != nil {
			net = net.Sub(txn.Amount.Decimal)
		}
	}
	return net.Round(2)
}

// activeTradeCommittedUSDT 一个 listing 下 active（pending_payment/paid/disputed）trade 的 USDT 合计。
func activeTradeCommittedUSDT(f *fixture, listingID uint) decimal.Decimal {
	sum := decimal.Zero
	var trades []c2cdomain.Trade
	f.db.Where("listing_id = ? AND status IN ?", listingID,
		[]string{"pending_payment", "paid", "disputed"}).Find(&trades)
	for _, tr := range trades {
		sum = sum.Add(tr.USDTAmount.Decimal)
	}
	return sum.Round(2)
}

// reconcileC2CListings 校验：
//  1. 每个 active/paused listing：net_frozen == available_usdt + active_trade_committed
//  2. 每个 closed listing：net_frozen == 0（无残留冻结引用）
//  3. 每个 suspended listing：net_frozen == 0（迁移挂起未冻结）
func reconcileC2CListings(f *fixture) []string {
	var issues []string
	var listings []c2cdomain.Listing
	if err := f.db.Find(&listings).Error; err != nil {
		return []string{"c2c_listings query: " + err.Error()}
	}
	for i := range listings {
		l := &listings[i]
		net := listingNetFrozen(f, l)
		switch l.Status {
		case "active", "paused":
			want := l.AvailableUSDT.Decimal.Round(2).Add(activeTradeCommittedUSDT(f, l.ID))
			if !net.Equal(want) {
				issues = append(issues,
					"listing "+l.ListingNo+" net_frozen="+net.String()+
						" want available_usdt("+l.AvailableUSDT.Decimal.String()+
						")+active_committed("+activeTradeCommittedUSDT(f, l.ID).String()+")="+want.String())
			}
		case "closed", "suspended":
			if !net.Equal(decimal.Zero) {
				issues = append(issues,
					"listing "+l.ListingNo+" status="+l.Status+
						" but net_frozen="+net.String()+" (should be 0)")
			}
		}
	}
	return issues
}

// ---------------------------------------------------------------------------
// 测试：用真实服务跑完一轮生命周期后对账，应 0 issues
// ---------------------------------------------------------------------------

// TestReconciliation_WalletAndC2C_AfterLifecycle 完整跑一轮：
// 创建挂单 → 两笔 trade（一笔成交、一笔取消）→ close → 全量对账。
func TestReconciliation_WalletAndC2C_AfterLifecycle(t *testing.T) {
	f := newFixture(t)
	sellerID, buyerID := uint(901), uint(902)
	f.createUser(t, sellerID)
	f.createUser(t, buyerID)
	f.setBalance(t, sellerID, "1000.00")
	f.setBalance(t, buyerID, "0.00")
	f.createPaymentMethod(t, sellerID)

	l := mustCreateListing(t, f, sellerID, "100.00")

	// trade A=30 成交
	trA := f.createTrade(t, buyerID, l.ID, "30.00")
	if _, err := f.svc.MarkPaid(c2ccontract.MarkPaidInput{TradeID: trA.ID, BuyerUserID: buyerID, PaymentReference: "x"}); err != nil {
		t.Fatalf("mark paid A: %v", err)
	}
	if _, err := f.svc.Confirm(c2ccontract.ConfirmTradeInput{TradeID: trA.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("confirm A: %v", err)
	}
	// trade B=20 取消（恢复 listing available）
	trB := f.createTrade(t, buyerID, l.ID, "20.00")
	if _, err := f.svc.Cancel(c2ccontract.CancelTradeInput{TradeID: trB.ID, BuyerUserID: buyerID}); err != nil {
		t.Fatalf("cancel B: %v", err)
	}
	// close listing（剩余 70 解冻）
	if _, err := f.svc.CloseListing(sellerID, l.ID); err != nil {
		t.Fatalf("close: %v", err)
	}

	if issues := reconcileWalletAccounts(f); len(issues) > 0 {
		t.Fatalf("WALLET RECONCILE ISSUES:\n  %v", issues)
	}
	if issues := reconcileC2CListings(f); len(issues) > 0 {
		t.Fatalf("C2C RECONCILE ISSUES:\n  %v", issues)
	}
}

// TestReconciliation_WalletConservation 对账函数本身能检出不一致（负面用例）。
func TestReconciliation_DetectsTampering(t *testing.T) {
	f := newFixture(t)
	seller := uint(911)
	f.createUser(t, seller)
	f.setBalance(t, seller, "1000.00")
	f.createPaymentMethod(t, seller)
	l := mustCreateListing(t, f, seller, "100.00")
	_ = l

	// 先确认干净状态
	if issues := reconcileWalletAccounts(f); len(issues) > 0 {
		t.Fatalf("precondition should be clean, got: %v", issues)
	}
	// 人为篡改账户 frozen（绕过 ledger），对账应检出
	f.db.Model(&walletdomain.Account{}).Where("user_id = ?", seller).
		UpdateColumn("frozen_balance", "999.00")
	issues := reconcileWalletAccounts(f)
	if len(issues) == 0 {
		t.Fatalf("expected reconciliation to detect tampering, got 0 issues")
	}
}
