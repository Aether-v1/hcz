// Package exchangerate 实现 HCZ 全局汇率（Global Exchange Rate）。
//
// 固定方向：1 USDT = R SiteCurrency。Business Order 的商品金额以 Site Currency 计价，
// 实际从 USDT 钱包扣款时按 walletAmount = siteCurrencyAmount / R 换算（Round half-up, 2dp）。
//
// 本模块与 Payment Gateway 自有 exchange_rate 完全隔离：Gateway Rate 只用于钱包充值，
// Global Rate 只用于商品订单 Site Currency → USDT。两者互不引用。
package exchangerate

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// USDT 为本系统钱包本位币，固定。
const WalletCurrency = "USDT"

// 来源标记。
const (
	SourceAuto   = "AUTO"
	SourceManual = "MANUAL"
)

// ErrRateUnavailable 在没有任何有效汇率（自动失败/过期 + 无手动兜底）时返回。
// 业务方必须 fail-closed 拒绝下单，禁止任何 1:1 / 0 / Gateway Rate 兜底。
var ErrRateUnavailable = errors.New("exchange_rate_unavailable")

// Rate 是一次解析出的、可用于下单换算的汇率。
type Rate struct {
	// Rate = 1 USDT 折合多少 SiteCurrency（如 CNY 站点 R=7.18 表示 1 USDT = 7.18 CNY）。
	Rate decimal.Decimal
	// Currency 是站点计价币种（Site Currency）。
	Currency string
	// Source = AUTO / MANUAL。
	Source string
	// FetchedAt 为该汇率产生时间。
	FetchedAt time.Time
}

// ToUSDT 把 Site Currency 金额换算为 USDT，结果 Round half-up 到 2 位小数。
// siteAmount 以 Site Currency 计。
func (r Rate) ToUSDT(siteAmount decimal.Decimal) (decimal.Decimal, error) {
	if r.Rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrRateUnavailable
	}
	return siteAmount.Div(r.Rate).Round(2), nil
}
