package application

import (
	"strings"

	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/shopspring/decimal"
)

// QuoteFee 手续费预览：不扣款，仅返回 fee/net。
func (s *Service) QuoteFee(network string, amount money.Amount) (withdrawalcontract.FeeQuote, error) {
	cfg := s.loadConfig()
	if !cfg.Enabled {
		return withdrawalcontract.FeeQuote{}, withdrawalcontract.ErrWithdrawalDisabled
	}
	net := strings.ToUpper(strings.TrimSpace(network))
	if net == "" {
		net = cfg.Network
	}
	if net != cfg.Network {
		return withdrawalcontract.FeeQuote{}, withdrawalcontract.ErrUnsupportedNetwork
	}
	amt := amount.Decimal.Round(2)
	if amt.LessThanOrEqual(decimal.Zero) {
		return withdrawalcontract.FeeQuote{}, withdrawalcontract.ErrInvalidAmount
	}
	minAmount := parseConfigDecimal(cfg.MinAmount)
	if amt.LessThan(minAmount) {
		return withdrawalcontract.FeeQuote{}, withdrawalcontract.ErrAmountTooSmall
	}
	fixedFee := parseConfigDecimal(cfg.FixedFee)
	percentageFee := parseConfigDecimal(cfg.PercentageFee)
	quote := calculateFee(money.FromDecimal(amt), fixedFee, percentageFee)
	quote.Currency = cfg.Currency
	return quote, nil
}
