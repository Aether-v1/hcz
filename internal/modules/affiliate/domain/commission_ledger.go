package domain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// CommissionLedger 佣金账本（append-only）。
// 核心原则：只 INSERT，永远不 UPDATE/DELETE。佣金净余额 = SUM(ledger.amount)。
// 修正历史只能通过新增 ADJUSTMENT/REVERSAL 记录完成。
type CommissionLedger struct {
	ID                 uint         `gorm:"primarykey" json:"id"`
	CommissionID       uint         `gorm:"not null;index" json:"commission_id"`           // 关联原始 commission（0 表示不关联特定订单的调整）
	AffiliateProfileID uint         `gorm:"not null;index" json:"affiliate_profile_id"`   // 推广档案ID
	BeneficiaryUserID  uint         `gorm:"not null;index" json:"beneficiary_user_id"`    // 真实收益人用户ID
	OrderID            uint         `gorm:"index" json:"order_id"`                        // 关联订单ID（可空=0）
	Type               string       `gorm:"type:varchar(32);not null;index" json:"type"`  // 账本类型：credit/reversal/withdraw_lock/withdraw_settle/withdraw_release/adjustment/debt
	Amount             money.Amount `gorm:"type:decimal(20,2);not null" json:"amount"`    // 有符号金额：credit/adjustment/release 为正，reversal/lock/settle/debt 为负
	BalanceAfter       money.Amount `gorm:"type:decimal(20,2);not null" json:"balance_after"` // 该 profile 累计净佣金余额快照（含所有类型）
	Reference          string       `gorm:"type:varchar(120);uniqueIndex" json:"reference"`   // 幂等唯一键
	WithdrawRequestID  *uint        `gorm:"index" json:"withdraw_request_id,omitempty"`   // 提现相关记录关联
	Remark             string       `gorm:"type:varchar(255)" json:"remark"`              // 备注
	CreatedAt          time.Time    `gorm:"index" json:"created_at"`                       // 创建时间
}

// TableName 指定表名
func (CommissionLedger) TableName() string {
	return "affiliate_commission_ledgers"
}
