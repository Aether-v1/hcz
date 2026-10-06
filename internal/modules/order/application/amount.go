package application

import (
	"github.com/Aether-v1/hcz/internal/modules/order/domain"
	"github.com/shopspring/decimal"
)

func normalizeOrderAmount(amount decimal.Decimal) decimal.Decimal {
	normalized := amount.Round(2)
	if normalized.LessThan(decimal.Zero) {
		return decimal.Zero
	}
	return normalized
}

// RemainingOnlineAmountCNY 计算订单在钱包扣款之后、仍需由在线网关支付的
// Site Currency（CNY）应付额。
//
// 金额语义（详见项目根目录 MONEY_SEMANTICS_TABLE.md）：
//   - Order.TotalAmount      = Site Currency（CNY）订单实付总额
//   - Order.WalletPaidAmount = USDT 钱包实扣（钱包本位币为 USDT）
//   - Order.UsdtTotalAmount  = 本单应收 USDT 快照（P0-2 全局汇率上线后的订单 >0）
//   - Order.ExchangeRate     = 下单时冻结的 1 USDT = R Site Currency
//
// P0 修复：USDT 结算单上钱包扣的是 USDT，必须先用订单冻结汇率把钱包已扣 USDT
// 折回 CNY，再从 CNY 总额里减；直接 TotalAmount(CNY) - WalletPaidAmount(USDT)
// 是跨币种减法，会凭空得到一个正的"残余在线应付"，导致 wallet-only 订单在钱包
// 全额扣款后仍被误判需要在线支付并返回 ErrOnlyPaymentRequired。
//
// 无 USDT 快照的历史订单钱包按 Site Currency 扣款，保留原同币种直减逻辑。
func RemainingOnlineAmountCNY(order *domain.Order) decimal.Decimal {
	if order == nil {
		return decimal.Zero
	}
	walletPaid := order.WalletPaidAmount.Decimal
	if order.UsdtTotalAmount.Decimal.GreaterThan(decimal.Zero) &&
		order.ExchangeRate.Valid && order.ExchangeRate.Decimal.GreaterThan(decimal.Zero) {
		walletPaidCNY := walletPaid.Mul(order.ExchangeRate.Decimal)
		return normalizeOrderAmount(order.TotalAmount.Decimal.Sub(walletPaidCNY))
	}
	return normalizeOrderAmount(order.TotalAmount.Decimal.Sub(walletPaid))
}
