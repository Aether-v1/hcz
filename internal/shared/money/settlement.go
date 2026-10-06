package money

import "github.com/shopspring/decimal"

// CeilTo2 rounds a monetary amount UP to 2 decimal places (ceiling),
// used for forward settlement receivables so the platform never under-collects.
//
// Examples:
//
//	13.880000 -> 13.88
//	13.880001 -> 13.89
//	13.889    -> 13.89
//	13.890001 -> 13.90
func CeilTo2(d decimal.Decimal) decimal.Decimal {
	return d.RoundCeil(2)
}

// RoundHalfUpTo2 rounds a monetary amount to 2 decimal places using
// round-half-up (四舍五入), used ONLY for UI display, never for settlement.
//
// shopspring/decimal's default Round(2) is half-even; this helper implements
// true half-up for the non-negative amounts the money package deals with.
func RoundHalfUpTo2(d decimal.Decimal) decimal.Decimal {
	scaled := d.Mul(decimal.NewFromInt(100))
	return scaled.Add(decimal.NewFromFloat(0.5)).Floor().Div(decimal.NewFromInt(100))
}
