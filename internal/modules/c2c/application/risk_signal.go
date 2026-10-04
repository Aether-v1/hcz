package application

import (
	"strings"
	"time"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
)

// 风控信号类型常量（第一版只记录，不自动冻结账号）。
const (
	RiskSignalSelfTradeAttempt     = "self_trade_attempt"
	RiskSignalHighCancelRate       = "high_cancel_rate"
	RiskSignalRepeatedCounterparty = "repeated_counterparty"
	RiskSignalNewAccountLargeTrade = "new_account_large_trade"
	RiskSignalDailyVolumeExceeded  = "daily_volume_exceeded"
)

// RecordRiskSignal 记录一条风控信号。第一版只落库，不触发自动冻结/封禁。
func (s *Service) RecordRiskSignal(input c2ccontract.RiskSignalInput) (*c2cdomain.RiskSignal, error) {
	if input.UserID == 0 || strings.TrimSpace(input.SignalType) == "" {
		return nil, c2ccontract.ErrInvalidAmount
	}
	r := &c2cdomain.RiskSignal{
		UserID:     input.UserID,
		SignalType: strings.TrimSpace(input.SignalType),
		TradeID:    input.TradeID,
		Metadata:   strings.TrimSpace(input.Metadata),
		CreatedAt:  time.Now(),
	}
	if err := s.repo.CreateRiskSignal(r); err != nil {
		return nil, err
	}
	return r, nil
}

// ListRiskSignals 管理员分页查询风控信号。
func (s *Service) ListRiskSignals(filter c2ccontract.RiskSignalListFilter) ([]c2cdomain.RiskSignal, int64, error) {
	return s.repo.ListRiskSignals(filter)
}
