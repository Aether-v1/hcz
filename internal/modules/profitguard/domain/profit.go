// Package profitguard 实现 HCZ Profit Guard V1：下单创建前的资金安全校验。
//
// 全部计算使用 shopspring/decimal，禁止 float64 参与（配置里的百分比 float64
// 在进入本包前必须已 decimal.NewFromFloat 转换）。
//
// 统一口径：一切换算为 CNY 后比较。汇率方向 1 USDT = R CNY。
package profitguard

import (
	"errors"

	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/shopspring/decimal"
)

// ErrProductCostNotConfigured 商品成本价未配置（正式充值商品 cost<=0）。
var ErrProductCostNotConfigured = errors.New("product_cost_not_configured")

// ErrUnprofitableOrder 预计利润低于门槛，拒单。
var ErrUnprofitableOrder = errors.New("product_unprofitable")

// OrderLine 是 Profit Guard 视角下的一行订单项。
type OrderLine struct {
	CostPriceCNY decimal.Decimal // OrderItem.CostPrice 快照
	Quantity     int
	CostExempt   bool            // Admin 标记的真零成本商品（数字权益/赠品/内部测试），不计成本门
}

// Input 是一次下单前的利润核算输入。
type Input struct {
	RevenueCNY decimal.Decimal // Order.TotalAmount
	Lines      []OrderLine

	// 汇率与应收 USDT。
	MarketRate decimal.Decimal // 1 USDT = R CNY（订单快照汇率）
	BufferPct  decimal.Decimal // 生效的安全缓冲（%），仅审计展示，不重复扣减

	// Affiliate 最坏成本预估。
	AffiliateEnabled bool
	AffiliateSetting settingsintegration.AffiliateSetting

	// 平台承担费用（V1 多为 0；见 Fee Cost Resolver）。
	FeeCostCNY decimal.Decimal

	// 门槛。
	MinProfitAmountCNY decimal.Decimal
	MinProfitRatePct   decimal.Decimal // %
}

// Result 是核算明细（仅管理员可见，用户端不得暴露）。
type Result struct {
	RevenueCNY        decimal.Decimal
	SupplierCostCNY   decimal.Decimal
	FeeCostCNY        decimal.Decimal
	AffiliateCostCNY  decimal.Decimal
	ExpectedProfitCNY decimal.Decimal
	RequiredProfitCNY decimal.Decimal
	UsdtTotal         decimal.Decimal
	BufferPct         decimal.Decimal
}

// SupplierCostCNY 汇总行成本。
func SupplierCostCNY(lines []OrderLine) decimal.Decimal {
	total := decimal.Zero
	for _, ln := range lines {
		total = total.Add(ln.CostPriceCNY.Mul(decimal.NewFromInt(int64(ln.Quantity))))
	}
	return total.Round(2)
}

// MissingCostLines 返回未配置成本且未豁免的行（用于 PRODUCT_COST_BACKFILL_REQUIRED 审计）。
func MissingCostLines(lines []OrderLine) []OrderLine {
	var missing []OrderLine
	for _, ln := range lines {
		if ln.CostExempt {
			continue
		}
		if ln.CostPriceCNY.LessThanOrEqual(decimal.Zero) {
			missing = append(missing, ln)
		}
	}
	return missing
}

// MaxAffiliateRatePct 按真实层级配置汇总"最坏情况下可能被分掉的佣金比例"。
// 不能简单 10 级全 sum：只统计 enabled 且不超过 MaxLevel 的层级。
func MaxAffiliateRatePct(setting settingsintegration.AffiliateSetting) decimal.Decimal {
	sum := decimal.Zero
	if !setting.Enabled {
		return decimal.Zero
	}
	maxLevel := setting.MaxLevel
	if maxLevel < 1 {
		maxLevel = 1
	}
	if maxLevel > len(setting.LevelRates) {
		maxLevel = len(setting.LevelRates)
	}
	for i := 0; i < maxLevel; i++ {
		item := setting.LevelRates[i]
		if !item.Enabled {
			continue
		}
		sum = sum.Add(decimal.NewFromFloat(item.Rate))
	}
	return sum.Round(2)
}

// Evaluate 执行完整利润核算（不拒单，只出明细）。
// usdtTotal 为订单已算好的应收 USDT（ToUSDTWithBuffer 结果）。
func Evaluate(in Input, usdtTotal decimal.Decimal) Result {
	supplier := SupplierCostCNY(in.Lines)
	fee := decimal.Zero
	if in.FeeCostCNY.IsPositive() {
		fee = in.FeeCostCNY.Round(2)
	}

	// 最坏 Affiliate 成本（USDT → CNY，用订单快照市场汇率折回）。
	affiliateCNY := decimal.Zero
	if in.AffiliateEnabled && in.MarketRate.IsPositive() {
		sumRatePct := MaxAffiliateRatePct(in.AffiliateSetting)
		if sumRatePct.IsPositive() {
			maxAffiliateUSDT := usdtTotal.Mul(sumRatePct).Div(decimal.NewFromInt(100)).Round(2)
			affiliateCNY = maxAffiliateUSDT.Mul(in.MarketRate).Round(2)
		}
	}

	// 注意：effective_rate 已含 buffer（用户已多付 USDT），这里不得再把 buffer 重复计为 FXRiskCost。
	expected := in.RevenueCNY.Sub(supplier).Sub(fee).Sub(affiliateCNY).Round(2)

	required := in.MinProfitAmountCNY.Round(2)
	if in.MinProfitRatePct.IsPositive() {
		ratePart := in.RevenueCNY.Mul(in.MinProfitRatePct).Div(decimal.NewFromInt(100)).Round(2)
		if ratePart.GreaterThan(required) {
			required = ratePart
		}
	}

	return Result{
		RevenueCNY:        in.RevenueCNY.Round(2),
		SupplierCostCNY:   supplier,
		FeeCostCNY:        fee,
		AffiliateCostCNY:  affiliateCNY,
		ExpectedProfitCNY: expected,
		RequiredProfitCNY: required,
		UsdtTotal:         usdtTotal,
		BufferPct:         in.BufferPct,
	}
}

// Guard 成本门 + 利润门。
//
//	requireCost=true 时，存在未配置成本且未豁免的正式商品 → ErrProductCostNotConfigured。
//	ExpectedProfit < RequiredProfit → ErrUnprofitableOrder。
func (r Result) Guard(requireCost bool, lines []OrderLine) error {
	if requireCost && len(MissingCostLines(lines)) > 0 {
		return ErrProductCostNotConfigured
	}
	if r.ExpectedProfitCNY.LessThan(r.RequiredProfitCNY) {
		return ErrUnprofitableOrder
	}
	return nil
}
