// Package profitguard implements the PURE, side-effect-free pricing/profit math
// for order pre-validation. It operates on plain numeric inputs so it can be
// unit-tested without a database. Wiring it into createOrder is a separate step.
package profitguard

import (
	"errors"

	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/shopspring/decimal"
)

// ErrUnprofitableOrder is returned when expected net profit is below the required minimum.
var ErrUnprofitableOrder = errors.New("order_unprofitable")

// ErrProductCostNotConfigured is returned when a real product has cost<=0 while
// cost enforcement is enabled.
var ErrProductCostNotConfigured = errors.New("product_cost_not_configured")

// Config is the admin-configurable profit guard knobs.
type Config struct {
	Enabled          bool
	MinimumProfitCNY decimal.Decimal // fixed floor
	MinimumProfitRate decimal.Decimal // percentage floor
	FXBufferPercent  decimal.Decimal // safety buffer %
	RequireCostPrice bool            // reject formal products with cost<=0
}

// LineCost is one order item's supplier cost (already CNY).
type LineCost struct {
	CostPrice decimal.Decimal
	Quantity  int
	// ZeroCostAllowed marks a genuine zero-cost digital entitlement that must
	// not be rejected by RequireCostPrice.
	ZeroCostAllowed bool
}

// Inputs are all CNY unless suffixed USDT.
type Inputs struct {
	RevenueCNY    decimal.Decimal // TotalAmount
	Lines         []LineCost
	FeeCostCNY    decimal.Decimal // platform-absorbed fee (counted exactly once)
	UsdtTotal     decimal.Decimal //应收 USDT
	ExchangeRate  decimal.Decimal // 1 USDT = R CNY
	Affiliate     settingsintegration.AffiliateSetting
	ResellerProfitCNY decimal.Decimal // optional
}

// SumEnabledLevelRates returns Σ(enabled level rates) within MaxLevel.
func SumEnabledLevelRates(a settingsintegration.AffiliateSetting) decimal.Decimal {
	sum := decimal.Zero
	maxLevel := a.MaxLevel
	if maxLevel < 1 {
		maxLevel = 1
	}
	if maxLevel > 10 {
		maxLevel = 10
	}
	for i := 0; i < maxLevel && i < len(a.LevelRates); i++ {
		item := a.LevelRates[i]
		if !item.Enabled {
			continue
		}
		sum = sum.Add(decimal.NewFromFloat(item.Rate))
	}
	return sum
}

// EstimateMaxAffiliateCostUSDT is the worst-case commission the platform may pay.
func EstimateMaxAffiliateCostUSDT(usdtTotal decimal.Decimal, a settingsintegration.AffiliateSetting) decimal.Decimal {
	sumRate := SumEnabledLevelRates(a)
	return usdtTotal.Mul(sumRate).Div(decimal.NewFromInt(100)).Round(2)
}

// supplierCostCNY sums cost_price × qty.
func supplierCostCNY(lines []LineCost) decimal.Decimal {
	total := decimal.Zero
	for _, l := range lines {
		total = total.Add(l.CostPrice.Mul(decimal.NewFromInt(int64(l.Quantity))))
	}
	return total
}

// CheckCostConfigured rejects formal products with cost<=0 when enforcement is on.
func CheckCostConfigured(lines []LineCost, require bool) error {
	if !require {
		return nil
	}
	for _, l := range lines {
		if l.ZeroCostAllowed {
			continue
		}
		if l.CostPrice.LessThanOrEqual(decimal.Zero) {
			return ErrProductCostNotConfigured
		}
	}
	return nil
}

// Evaluate computes expected net profit and required minimum. Returns net, required.
func Evaluate(in Inputs, cfg Config) (netCNY decimal.Decimal, requiredCNY decimal.Decimal) {
	revenue := in.RevenueCNY
	cost := supplierCostCNY(in.Lines)

	// Max affiliate cost converted to CNY.
	maxAffiliateUSDT := EstimateMaxAffiliateCostUSDT(in.UsdtTotal, in.Affiliate)
	maxAffiliateCNY := maxAffiliateUSDT.Mul(in.ExchangeRate).Round(2)

	// FX risk cost = buffered fraction, CNY. Only counted once (buffer already
	// discounts the charged USDT; here we keep it explicit for the guard math).
	fxRiskCNY := decimal.Zero
	if cfg.FXBufferPercent.GreaterThan(decimal.Zero) && in.ExchangeRate.GreaterThan(decimal.Zero) {
		fxRiskCNY = in.UsdtTotal.Mul(cfg.FXBufferPercent.Div(decimal.NewFromInt(100))).Mul(in.ExchangeRate).Round(2)
	}

	net := revenue.Sub(cost).Sub(in.FeeCostCNY).Sub(maxAffiliateCNY).Sub(fxRiskCNY).Sub(in.ResellerProfitCNY)

	required := cfg.MinimumProfitCNY
	rateFloor := revenue.Mul(cfg.MinimumProfitRate).Div(decimal.NewFromInt(100))
	if rateFloor.GreaterThan(required) {
		required = rateFloor
	}
	return net, required
}

// Validate runs cost-enforcement then profit guard.
func Validate(in Inputs, cfg Config) error {
	if err := CheckCostConfigured(in.Lines, cfg.RequireCostPrice); err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	net, required := Evaluate(in, cfg)
	if net.LessThan(required) {
		return ErrUnprofitableOrder
	}
	return nil
}
