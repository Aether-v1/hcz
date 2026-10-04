package contract

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// WithdrawalListFilter 用户侧提现单分页过滤。
type WithdrawalListFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
}

// AdminWithdrawalListFilter 后台提现单分页过滤。
type AdminWithdrawalListFilter struct {
	Page         int
	PageSize     int
	Status       string
	UserID       uint
	WithdrawalNo string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
}

// CreateWithdrawalInput 用户发起提现输入。
type CreateWithdrawalInput struct {
	UserID         uint
	Network        string
	Address        string
	Amount         money.Amount
	TOTPCode       string
	IdempotencyKey string
	UserNote       string
}

// CancelWithdrawalInput 用户取消提现输入。
type CancelWithdrawalInput struct {
	UserID   uint
	ID       uint
	TOTPCode string
}

// AdminReviewInput 后台审批（approve/reject）输入。
type AdminReviewInput struct {
	ID             uint
	AdminID        uint
	AdminNote      string
	RejectReason   string
	IdempotencyKey string
}

// AdminCompleteInput 后台打款完成输入。
type AdminCompleteInput struct {
	ID             uint
	AdminID        uint
	Txid           string
	AdminNote      string
	IdempotencyKey string
}

// AdminProcessingInput 后台标记处理中输入。
type AdminProcessingInput struct {
	ID             uint
	AdminID        uint
	IdempotencyKey string
}

// FeeQuote 手续费预览（不扣款）。
type FeeQuote struct {
	RequestAmount money.Amount `json:"request_amount"`
	FeeAmount     money.Amount `json:"fee_amount"`
	NetAmount     money.Amount `json:"net_amount"`
	Currency      string       `json:"currency"`
}

// CreateAddressInput 新增提现地址输入。
type CreateAddressInput struct {
	UserID  uint
	Network string
	Address string
	Label   string
}

// SetDefaultAddressInput 设置默认地址输入。
type SetDefaultAddressInput struct {
	UserID uint
	ID     uint
}
