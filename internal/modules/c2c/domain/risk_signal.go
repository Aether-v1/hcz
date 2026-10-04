package domain

import (
	"time"
)

// RiskSignal C2C 风控信号。
type RiskSignal struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	UserID     uint      `gorm:"index:idx_c2c_risk_user_type,priority:1;not null" json:"user_id"`
	SignalType string    `gorm:"index:idx_c2c_risk_user_type,priority:2;type:varchar(32);not null" json:"signal_type"` // self_trade_attempt/high_cancel_rate/repeated_counterparty/new_account_large_trade/daily_volume_exceeded
	TradeID    *uint     `gorm:"index" json:"trade_id,omitempty"`
	Metadata   string    `gorm:"type:text;default:''" json:"metadata"` // 信号上下文（JSON）
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// TableName 指定表名。
func (RiskSignal) TableName() string {
	return "c2c_risk_signals"
}
