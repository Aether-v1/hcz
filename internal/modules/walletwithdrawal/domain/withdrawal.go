package domain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// 提现单状态机独立 6 态。终态：rejected / canceled / completed。
const (
	StatusPending    = "pending"
	StatusApproved   = "approved"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusRejected   = "rejected"
	StatusCanceled   = "canceled"
)

// Withdrawal 用户提现单。申请即扣款，reject/cancel 退款。
type Withdrawal struct {
	ID             uint         `gorm:"primarykey" json:"id"`
	WithdrawalNo   string       `gorm:"size:64;uniqueIndex;not null" json:"withdrawal_no"`
	UserID         uint         `gorm:"index;not null" json:"user_id"`
	Network        string       `gorm:"size:16;not null;default:'TRC20'" json:"network"`
	Address        string       `gorm:"size:128;not null" json:"address"`
	RequestAmount  money.Amount `gorm:"type:decimal(20,2);not null" json:"request_amount"`
	FeeAmount      money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"fee_amount"`
	NetAmount      money.Amount `gorm:"type:decimal(20,2);not null" json:"net_amount"`
	Status         string       `gorm:"size:16;index;not null;default:'pending'" json:"status"`
	Txid           string       `gorm:"size:128;index" json:"txid"`
	UserNote       string       `gorm:"size:255" json:"user_note"`
	AdminNote      string       `gorm:"size:255" json:"admin_note"`
	RejectReason   string       `gorm:"size:255" json:"reject_reason"`
	IdempotencyKey string       `gorm:"size:128;not null" json:"idempotency_key"`
	// Reference 全局唯一：wd:<user_id>:<idempotency_key>。
	Reference string `gorm:"size:128;uniqueIndex;not null" json:"reference"`

	ApprovedBy  *uint `gorm:"index" json:"approved_by,omitempty"`
	RejectedBy  *uint `gorm:"index" json:"rejected_by,omitempty"`
	ProcessedBy *uint `gorm:"index" json:"processed_by,omitempty"`
	CompletedBy *uint `gorm:"index" json:"completed_by,omitempty"`

	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	ProcessingAt *time.Time `json:"processing_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	RejectedAt   *time.Time `json:"rejected_at,omitempty"`
	CanceledAt   *time.Time `json:"canceled_at,omitempty"`

	CreatedAt time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt time.Time  `gorm:"index" json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (Withdrawal) TableName() string { return "wallet_withdrawals" }

// Address 用户提现地址簿。
type Address struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	Network   string     `gorm:"size:16" json:"network"`
	Address   string     `gorm:"size:128" json:"address"`
	Label     string     `gorm:"size:64" json:"label"`
	IsDefault bool       `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt time.Time  `gorm:"index" json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (Address) TableName() string { return "wallet_withdrawal_addresses" }

// IsTerminal 返回状态是否为终态（不可再流转）。
func IsTerminal(status string) bool {
	switch status {
	case StatusRejected, StatusCanceled, StatusCompleted:
		return true
	default:
		return false
	}
}
