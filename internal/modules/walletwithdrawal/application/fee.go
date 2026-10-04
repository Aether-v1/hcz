package application

import (
	"github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// calculateFee 计算手续费：fee = fixed_fee + amount * percentage_fee（2dp 舍入），net = amount - fee。
// decimal only，禁止 float。
func calculateFee(amount money.Amount, fixedFee, percentageFee decimal.Decimal) contract.FeeQuote {
	amt := amount.Decimal.Round(2)
	fixed := fixedFee.Round(2)
	percentage := percentageFee.Round(4)
	fee := amt.Mul(percentage).Add(fixed).Round(2)
	if fee.LessThan(decimal.Zero) {
		fee = decimal.Zero
	}
	net := amt.Sub(fee).Round(2)
	if net.LessThan(decimal.Zero) {
		net = decimal.Zero
	}
	return contract.FeeQuote{
		RequestAmount: money.FromDecimal(amt),
		FeeAmount:     money.FromDecimal(fee),
		NetAmount:     money.FromDecimal(net),
	}
}
