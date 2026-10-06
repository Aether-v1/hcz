package e2e_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	exchangeratedomain "github.com/Aether-v1/hcz/internal/modules/exchangerate/domain"
	exchangeratecontract "github.com/Aether-v1/hcz/internal/modules/exchangerate/contract"
	. "github.com/Aether-v1/hcz/internal/modules/order/application"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	refundapp "github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// assertWalletUnchanged 校验钱包 total(available+frozen) 前后守恒。
func assertWalletUnchanged(t *testing.T, f *e2eFixture, userID uint, beforeAvail, beforeFrozen decimal.Decimal) {
	t.Helper()
	avail, frozen, total := f.walletTotal(t, userID)
	if !avail.Equal(beforeAvail) {
		t.Fatalf("wallet available changed: before=%s after=%s", beforeAvail, avail)
	}
	if !frozen.Equal(beforeFrozen) {
		t.Fatalf("wallet frozen changed: before=%s after=%s", beforeFrozen, frozen)
	}
	if !total.Equal(beforeAvail.Add(beforeFrozen)) {
		t.Fatalf("wallet total not conserved: before=%s after=%s", beforeAvail.Add(beforeFrozen), total)
	}
}

// ===================== 5.2 正利润订单 =====================

func TestE2E_PositiveProfitOrder(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 10, 5) // min profit 10 CNY 或 5%
	f.recharge(t, f.user.ID, "100")

	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "40"})
	beforeAvail, beforeFrozen, _ := f.walletTotal(t, f.user.ID)

	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("positive-profit order should succeed, got: %v", err)
	}

	// 定价快照：ExpectedProfit 已保存且 >= Required。
	if !order.ExpectedProfitCNY.Valid {
		t.Fatalf("ExpectedProfitCNY should be saved (Pricing Snapshot), got invalid")
	}
	// expected = 100(revenue) - 40(cost) - 0(fee) - 0(affiliate) = 60 CNY。
	exp := order.ExpectedProfitCNY.Decimal
	if exp.LessThan(decimal.NewFromInt(10)) {
		t.Fatalf("ExpectedProfitCNY=%s, want >= required 10", exp)
	}
	t.Logf("ExpectedProfitCNY=%s (>= required 10) ✓", exp)

	if order.PricingVersion != "profit_guard_v1" {
		t.Fatalf("PricingVersion=%q, want profit_guard_v1", order.PricingVersion)
	}
	if order.ExchangeRateSource == "" || !order.UsdtTotalAmount.Decimal.IsPositive() {
		t.Fatalf("FX snapshot missing: rate_src=%q usdt=%s", order.ExchangeRateSource, order.UsdtTotalAmount.Decimal)
	}
	t.Logf("UsdtTotalAmount=%s, ExchangeRate=%s src=%s", order.UsdtTotalAmount.Decimal, order.ExchangeRate.Decimal, order.ExchangeRateSource)

	// 下单（仅创建，未支付）不得扣款。
	assertWalletUnchanged(t, f, f.user.ID, beforeAvail, beforeFrozen)
	if got := f.countOrders(t); got != 1 {
		t.Fatalf("order count=%d, want 1", got)
	}
	t.Logf("订单创建成功，状态=%s，钱包未扣款，资金守恒 ✓", order.Status)
}

// ===================== 5.3 负利润订单被拒（无副作用）=====================

func TestE2E_NegativeProfitRejected(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 10, 5)
	f.recharge(t, f.user.ID, "100")

	// cost=95 接近售价 100 → expected=5 < required=10。
	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "95"})
	beforeAvail, beforeFrozen, _ := f.walletTotal(t, f.user.ID)

	_, err := f.placeOrder(t, prodID, skuID, 1)
	if err == nil {
		t.Fatal("negative-profit order must be rejected, got nil error")
	}
	if !errors.Is(err, ErrProductUnprofitable) {
		t.Fatalf("want ErrProductUnprofitable, got: %v", err)
	}
	t.Logf("拒单错误=%q ✓", err)

	// 关键：无任何副作用——钱包不变、无订单记录。
	assertWalletUnchanged(t, f, f.user.ID, beforeAvail, beforeFrozen)
	if got := f.countOrders(t); got != 0 {
		t.Fatalf("rejected order must not create DB rows, order count=%d", got)
	}
}

// ===================== 5.4 Cost Missing =====================

func TestE2E_CostMissingRejected(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 0, 0) // RequireCostPrice=true
	f.recharge(t, f.user.ID, "100")

	// cost=0 且非豁免商品。
	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "0", costExempt: false})
	beforeAvail, beforeFrozen, _ := f.walletTotal(t, f.user.ID)

	_, err := f.placeOrder(t, prodID, skuID, 1)
	if err == nil {
		t.Fatal("cost-missing order must be rejected")
	}
	if !errors.Is(err, ErrProductCostNotConfigured) {
		t.Fatalf("want ErrProductCostNotConfigured, got: %v", err)
	}
	t.Logf("拒单错误=%q ✓", err)

	assertWalletUnchanged(t, f, f.user.ID, beforeAvail, beforeFrozen)
	if got := f.countOrders(t); got != 0 {
		t.Fatalf("cost-missing order must not create DB rows, order count=%d", got)
	}
}

// ===================== 5.5 FX 异常（fail-closed）=====================

func TestE2E_FXUnavailableFailClosed(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 10, 5)
	f.recharge(t, f.user.ID, "100")
	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "40"})

	// 场景 A：AUTO stale（超过 30min）且 MANUAL 未配置。
	f.setRateState(exchangeratecontract.State{
		Currency:        "CNY",
		AutoRate:        mustDec("7.18"),
		AutoFetchedAt:   time.Now().Add(-2 * time.Hour), // 远超 30min staleness
		ManualRate:      decimal.Zero,                   // 无手动兜底
	})
	beforeAvail, beforeFrozen, _ := f.walletTotal(t, f.user.ID)
	_, errA := f.placeOrder(t, prodID, skuID, 1)
	if !errors.Is(errA, exchangeratedomain.ErrRateUnavailable) {
		t.Fatalf("scenario A (stale auto, no manual): want ErrRateUnavailable, got: %v", errA)
	}
	assertWalletUnchanged(t, f, f.user.ID, beforeAvail, beforeFrozen)
	if got := f.countOrders(t); got != 0 {
		t.Fatalf("FX-stale order must not create rows, count=%d", got)
	}
	t.Logf("场景A stale FX 拒单=%q，钱包/订单无副作用 ✓", errA)

	// 场景 B：完全无汇率配置。
	f.setRateState(exchangeratecontract.State{Currency: "CNY"})
	_, errB := f.placeOrder(t, prodID, skuID, 1)
	if !errors.Is(errB, exchangeratedomain.ErrRateUnavailable) {
		t.Fatalf("scenario B (no rate config): want ErrRateUnavailable, got: %v", errB)
	}
	assertWalletUnchanged(t, f, f.user.ID, beforeAvail, beforeFrozen)
	if got := f.countOrders(t); got != 0 {
		t.Fatalf("FX-missing order must not create rows, count=%d", got)
	}
	t.Logf("场景B 无汇率配置拒单=%q ✓", errB)
}

// ===================== 5.6 Wallet-only 支付原子性 =====================

func TestE2E_WalletOnlyPaymentAtomic(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 10, 5)
	f.recharge(t, f.user.ID, "100")
	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "40"})

	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	usdtDue := order.UsdtTotalAmount.Decimal // 应扣款 USDT
	if !usdtDue.IsPositive() {
		t.Fatal("expected positive UsdtTotalAmount")
	}

	// 支付前余额。
	beforeAvail, beforeFrozen, _ := f.walletTotal(t, f.user.ID)
	if !beforeAvail.Equal(mustDec("100")) {
		t.Fatalf("prepay available=%s, want 100", beforeAvail)
	}

	// 钱包支付（原子：扣款 + 置已支付在同一事务）。
	result, err := f.payWithWallet(t, order.ID)
	if err != nil {
		t.Fatalf("wallet pay failed: %v", err)
	}
	if !result.OrderPaid {
		t.Fatal("expected OrderPaid=true")
	}

	// 支付后：available 精确减少 usdtDue；订单 status=paid。
	afterAvail, afterFrozen, _ := f.walletTotal(t, f.user.ID)
	wantAvail := beforeAvail.Sub(usdtDue)
	if !afterAvail.Equal(wantAvail) {
		t.Fatalf("after-pay available=%s, want %s (100 - %s)", afterAvail, wantAvail, usdtDue)
	}
	// total 守恒：available 减少的部分就是资金流出（支付给订单），frozen 不变。
	if !afterFrozen.Equal(beforeFrozen) {
		t.Fatalf("frozen should not change on direct wallet pay: before=%s after=%s", beforeFrozen, afterFrozen)
	}
	var refreshed orderdomain.Order
	if err := f.db.First(&refreshed, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	// 五态机（pending_recharge）钱包支付成功后由 markOrderPaid 直接派单到 processing
	// （人工履约），并置 PaidAt；旧九态才停在 paid。二者都代表“已支付”。
	if refreshed.PaidAt == nil {
		t.Fatal("order PaidAt must be set after wallet pay")
	}
	if refreshed.Status == constants.OrderStatusPendingRecharge {
		t.Fatalf("order still %s after wallet pay", refreshed.Status)
	}
	if refreshed.Status != constants.OrderStatusPaid && refreshed.Status != constants.OrderStatusProcessing {
		t.Fatalf("order status=%s, want paid/processing (paid)", refreshed.Status)
	}
	t.Logf("钱包支付成功：扣款 %s USDT，余额 100→%s，订单状态=%s（已支付）✓", usdtDue, afterAvail, refreshed.Status)

	// ---- 原子性：支付时余额不足 → 必须整体回滚（不出现“钱扣了订单未支付”）----
	// 制造竞态：再下一单，但先把余额扣到不足，再支付。
	order2, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("create order2: %v", err)
	}
	// 把余额人工扣到远低于 order2 应付（模拟 create 后余额被其它事务挪走）。
	if _, _, err := f.walletSvc.AdminAdjustBalance(walletcontract.AdjustBalanceInput{
		UserID: f.user.ID, OperatorAdminID: 1,
		Delta:  money.FromDecimal(afterAvail.Neg()), Remark: "drain for atomicity test",
	}); err != nil {
		t.Fatalf("drain wallet: %v", err)
	}
	drainedAvail, drainedFrozen, drainedTotal := f.walletTotal(t, f.user.ID)
	if !drainedAvail.IsZero() {
		t.Fatalf("drained available=%s, want 0", drainedAvail)
	}

	_, payErr := f.payWithWallet(t, order2.ID)
	if payErr == nil {
		t.Fatal("insufficient-balance pay must fail, got nil")
	}
	// 回滚后：钱包仍为 0，订单2 仍未支付。
	rollAvail, rollFrozen, _ := f.walletTotal(t, f.user.ID)
	if !rollAvail.Equal(drainedAvail) || !rollFrozen.Equal(drainedFrozen) {
		t.Fatalf("wallet not rolled back: avail %s→%s (want %s)", drainedAvail, rollAvail, drainedAvail)
	}
	var refreshed2 orderdomain.Order
	if err := f.db.First(&refreshed2, order2.ID).Error; err != nil {
		t.Fatalf("reload order2: %v", err)
	}
	if refreshed2.Status != constants.OrderStatusPendingRecharge {
		t.Fatalf("order2 status=%s, want still pending_recharge after failed wallet debit (atomic rollback broken)", refreshed2.Status)
	}
	// total 守恒（回滚）。
	if !drainedTotal.Equal(rollAvail.Add(rollFrozen)) {
		t.Fatalf("wallet total not conserved after rollback")
	}
	t.Logf("原子回滚验证：短款支付被拒(%q)，钱包=%s 不变，订单2状态=%s ✓", payErr, rollAvail, refreshed2.Status)
}

// ===================== 5.1 商品订单资金链（全链路编排）=====================

func TestE2E_FullOrderFundChain(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 10, 5)
	f.recharge(t, f.user.ID, "100")
	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "40"})

	// 1) 下单（PG 通过 + FX 快照 + Pricing Snapshot）。
	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	usdtDue := order.UsdtTotalAmount.Decimal

	// 2) 钱包支付：available 减少 usdtDue，订单 → paid。
	beforePay, _, _ := f.walletTotal(t, f.user.ID)
	if _, err := f.payWithWallet(t, order.ID); err != nil {
		t.Fatalf("wallet pay: %v", err)
	}
	afterPay, _, _ := f.walletTotal(t, f.user.ID)
	if !afterPay.Equal(beforePay.Sub(usdtDue)) {
		t.Fatalf("after pay wallet=%s, want %s", afterPay, beforePay.Sub(usdtDue))
	}

	// 3) 履约完成。
	f.markCompleted(t, order.ID)

	// 4) 全额退款：钱包收回 usdtDue。
	walletBeforeRefund, _, _ := f.walletTotal(t, f.user.ID)
	updated, txn, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID,
		Amount:  money.FromDecimal(usdtDue), // USDT 结算单按 WalletPaidAmount 退
		Remark:  "E2E 全额退款",
	})
	if err != nil {
		t.Fatalf("full refund: %v", err)
	}
	if txn == nil || txn.Type != constants.WalletTxnTypeAdminRefund {
		t.Fatalf("unexpected refund txn: %+v", txn)
	}
	walletAfterRefund, _, _ := f.walletTotal(t, f.user.ID)
	if !walletAfterRefund.Equal(walletBeforeRefund.Add(usdtDue)) {
		t.Fatalf("after refund wallet=%s, want %s", walletAfterRefund, walletBeforeRefund.Add(usdtDue))
	}
	// 资金守恒闭环：充值100 → 扣 usdtDue → 退 usdtDue → 回到 100。
	if !walletAfterRefund.Equal(mustDec("100")) {
		t.Fatalf("fund conservation broken: final wallet=%s, want 100", walletAfterRefund)
	}
	if updated.RefundStatus != constants.OrderRefundStatusFull {
		t.Fatalf("refund_status=%s, want full", updated.RefundStatus)
	}
	t.Logf("全链路资金守恒闭环：100 → -%s(支付) → +%s(退款) → %s ✓", usdtDue, usdtDue, walletAfterRefund)
}

// ===================== 5.7 全额退款 + Affiliate 冲正 =====================

// seedCommission 为已支付订单 seed 一笔可用佣金 + credit ledger。
func (f *e2eFixture) seedCommission(t *testing.T, orderID uint, baseAmount, commissionAmount string) affiliatedomain.Commission {
	t.Helper()
	now := time.Now()
	profile := &affiliatedomain.Profile{
		UserID:        f.user.ID + 1000,
		AffiliateCode: "E2EAFF",
		Status:        constants.AffiliateProfileStatusActive,
		CreatedAt:     now, UpdatedAt: now,
	}
	if err := f.db.Create(profile).Error; err != nil {
		t.Fatalf("create affiliate profile: %v", err)
	}
	comm := affiliatedomain.Commission{
		AffiliateProfileID: profile.ID,
		OrderID:            orderID,
		CommissionType:     constants.AffiliateCommissionTypeOrder,
		BeneficiaryUserID:  profile.UserID,
		BaseAmount:         money.FromDecimal(mustDec(baseAmount)),
		RatePercent:        money.FromDecimal(mustDec("10")),
		CommissionAmount:   money.FromDecimal(mustDec(commissionAmount)),
		Status:             constants.AffiliateCommissionStatusAvailable,
		CreatedAt:          now, UpdatedAt: now,
	}
	if err := f.db.Create(&comm).Error; err != nil {
		t.Fatalf("create commission: %v", err)
	}
	ledger := affiliatedomain.CommissionLedger{
		CommissionID:       comm.ID,
		AffiliateProfileID: profile.ID,
		BeneficiaryUserID:  profile.UserID,
		OrderID:            orderID,
		Type:               constants.AffiliateLedgerTypeCredit,
		Amount:             money.FromDecimal(mustDec(commissionAmount)),
		BalanceAfter:       money.FromDecimal(mustDec(commissionAmount)),
		Reference:          "e2e:seed:credit",
		Remark:             "seed",
		CreatedAt:          now,
	}
	if err := f.db.Create(&ledger).Error; err != nil {
		t.Fatalf("create credit ledger: %v", err)
	}
	return comm
}

func TestE2E_FullRefundWithAffiliateReversal(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 10, 5)
	f.recharge(t, f.user.ID, "100")
	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "40"})

	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	usdtDue := order.UsdtTotalAmount.Decimal
	if _, err := f.payWithWallet(t, order.ID); err != nil {
		t.Fatalf("wallet pay: %v", err)
	}
	f.markCompleted(t, order.ID)

	// seed 佣金：基数 = 实付 USDT，佣金 = 10%。
	comm := f.seedCommission(t, order.ID, usdtDue.StringFixed(2), usdtDue.Mul(mustDec("0.1")).StringFixed(2))

	walletBefore, _, _ := f.walletTotal(t, f.user.ID)
	updated, txn, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID,
		Amount:  money.FromDecimal(usdtDue),
		Remark:  "全额退款",
	})
	if err != nil {
		t.Fatalf("full refund: %v", err)
	}
	if txn.Type != constants.WalletTxnTypeAdminRefund {
		t.Fatalf("refund txn type=%s", txn.Type)
	}
	walletAfter, _, _ := f.walletTotal(t, f.user.ID)
	if !walletAfter.Equal(walletBefore.Add(usdtDue)) {
		t.Fatalf("refund wallet credit=%s, want +%s", walletAfter.Sub(walletBefore), usdtDue)
	}
	if updated.RefundStatus != constants.OrderRefundStatusFull {
		t.Fatalf("refund_status=%s, want full", updated.RefundStatus)
	}

	// Affiliate 冲正：全额退款 → reversal = -commissionAmount。
	var ledgers []affiliatedomain.CommissionLedger
	if err := f.db.Where("commission_id = ? AND type = ?", comm.ID, constants.AffiliateLedgerTypeReversal).Find(&ledgers).Error; err != nil {
		t.Fatalf("query reversal: %v", err)
	}
	if len(ledgers) != 1 {
		t.Fatalf("expected 1 reversal ledger, got %d", len(ledgers))
	}
	wantReversal := comm.CommissionAmount.Decimal.Neg()
	if !ledgers[0].Amount.Decimal.Equal(wantReversal) {
		t.Fatalf("reversal amount=%s, want %s", ledgers[0].Amount.Decimal, wantReversal)
	}
	t.Logf("全额退款：钱包 +%s，Affiliate 冲正 %s（佣金原值 %s 不可变）✓",
		usdtDue, ledgers[0].Amount.Decimal, comm.CommissionAmount.Decimal)
}

// ===================== 5.8 多次部分退款 =====================

func TestE2E_PartialRefundsCumulativeAndAffiliateProration(t *testing.T) {
	f := newE2EFixture(t)
	f.enableProfitGuard(t, true, 10, 5)
	f.recharge(t, f.user.ID, "100")
	prodID, skuID := f.createProduct(t, productSpec{priceCNY: "100", costCNY: "40"})

	order, err := f.placeOrder(t, prodID, skuID, 1)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	usdtDue := order.UsdtTotalAmount.Decimal
	if _, err := f.payWithWallet(t, order.ID); err != nil {
		t.Fatalf("wallet pay: %v", err)
	}
	f.markCompleted(t, order.ID)

	// seed 佣金 C = usdtDue * 0.1。
	commissionAmt := usdtDue.Mul(mustDec("0.1"))
	comm := f.seedCommission(t, order.ID, usdtDue.StringFixed(2), commissionAmt.StringFixed(2))

	// 分两次部分退款：每次 30%、再 30%。累计 60% <= 100%。
	r1 := usdtDue.Mul(mustDec("0.3")).Round(2)
	r2 := usdtDue.Mul(mustDec("0.3")).Round(2)

	walletBefore, _, _ := f.walletTotal(t, f.user.ID)

	if _, txn1, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID, Amount: money.FromDecimal(r1), Remark: "部分退款1",
	}); err != nil || txn1 == nil {
		t.Fatalf("partial refund1: err=%v txn=%v", err, txn1)
	}
	updated, _, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID, Amount: money.FromDecimal(r2), Remark: "部分退款2",
	})
	if err != nil {
		t.Fatalf("partial refund2: %v", err)
	}

	// 累计退款 = r1+r2，钱包增加恰好累计额。
	totalRefunded := r1.Add(r2)
	walletAfter, _, _ := f.walletTotal(t, f.user.ID)
	if !walletAfter.Sub(walletBefore).Equal(totalRefunded) {
		t.Fatalf("wallet credited=%s, want %s", walletAfter.Sub(walletBefore), totalRefunded)
	}
	if !updated.RefundedAmount.Decimal.Equal(totalRefunded) {
		t.Fatalf("refunded_amount=%s, want %s", updated.RefundedAmount.Decimal, totalRefunded)
	}
	if updated.RefundStatus != constants.OrderRefundStatusPartial {
		t.Fatalf("refund_status=%s, want partial (cumulative < full)", updated.RefundStatus)
	}

	// Affiliate 冲正（镜像模块既定公式，commission.go:480）：
	//   deduct_i = originalCommission * delta_i / remaining_i, remaining_i = usdtDue - refundedBefore_i, round2
	// 这是 append-only 设计：每次按“本次退款/剩余未退款额”比例从原始佣金额冲正；
	// 全额退款时由 full-refund 兜底把净余额清零，累计冲正永不超过原始佣金。
	originalCommission := comm.CommissionAmount.Decimal
	d1 := originalCommission.Mul(r1).Div(usdtDue).Round(2) // before=0, remaining=usdtDue
	remaining2 := usdtDue.Sub(r1)
	d2 := originalCommission.Mul(r2).Div(remaining2).Round(2)
	wantReversalSum := d1.Add(d2).Neg()

	var ledgers []affiliatedomain.CommissionLedger
	if err := f.db.Where("commission_id = ? AND type = ?", comm.ID, constants.AffiliateLedgerTypeReversal).
		Order("id asc").Find(&ledgers).Error; err != nil {
		t.Fatalf("query reversal ledgers: %v", err)
	}
	if len(ledgers) != 2 {
		t.Fatalf("expected 2 reversal ledgers, got %d", len(ledgers))
	}
	reversalSum := decimal.Zero
	for _, l := range ledgers {
		reversalSum = reversalSum.Add(l.Amount.Decimal)
	}
	if !reversalSum.Equal(wantReversalSum) {
		t.Fatalf("affiliate reversal sum=%s, want %s", reversalSum, wantReversalSum)
	}
	// 资金守恒：累计冲正（绝对值）不得超过原始佣金。
	if reversalSum.Neg().GreaterThan(originalCommission) {
		t.Fatalf("reversal sum %s exceeds original commission %s", reversalSum.Neg(), originalCommission)
	}
	t.Logf("两次部分退款累计 %s USDT，钱包 +%s，Affiliate 累计冲正 %s（原始佣金 %s，未超额）✓",
		totalRefunded, totalRefunded, reversalSum, originalCommission)

	// 超额退款必须被拒，累计不得超过实付。
	over := usdtDue.Sub(totalRefunded).Add(mustDec("1"))
	if _, _, _, err := f.refundSvc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{
		OrderID: order.ID, Amount: money.FromDecimal(over), Remark: "超额",
	}); !errors.Is(err, walletcontract.ErrRefundExceeded) {
		t.Fatalf("over-refund: want ErrRefundExceeded, got %v", err)
	}
	t.Logf("超额退款被拒(%q)，累计退款不超过实付 ✓", walletcontract.ErrRefundExceeded)
}
