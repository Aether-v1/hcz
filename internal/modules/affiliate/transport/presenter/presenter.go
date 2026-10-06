package presenter

import (
	"time"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// Profile 推广用户资料响应
type Profile struct {
	ID            uint      `json:"id"`
	AffiliateCode string    `json:"code"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// NewProfile 从 affiliatedomain.Profile 构造响应
func NewProfile(p *affiliatedomain.Profile) Profile {
	return Profile{
		ID:            p.ID,
		AffiliateCode: p.AffiliateCode,
		Status:        p.Status,
		CreatedAt:     p.CreatedAt,
	}
	// 排除：UserID、User、UpdatedAt
}

// Commission 佣金记录响应
type Commission struct {
	ID               uint         `json:"id"`
	CommissionType   string       `json:"commission_type"`
	Level            int          `json:"level"`
	CommissionAmount money.Amount `json:"commission_amount"`
	Currency         string       `json:"currency"` // P0-2: 固定 USDT
	Status           string       `json:"status"`
	ConfirmAt        *time.Time   `json:"confirm_at,omitempty"`
	AvailableAt      *time.Time   `json:"available_at,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
}

// NewCommission 从 affiliatedomain.Commission 构造响应
func NewCommission(c *affiliatedomain.Commission) Commission {
	return Commission{
		ID:               c.ID,
		CommissionType:   c.CommissionType,
		Level:            c.Level,
		CommissionAmount: c.CommissionAmount,
		Currency:         "USDT",
		Status:           c.Status,
		ConfirmAt:        c.ConfirmAt,
		AvailableAt:      c.AvailableAt,
		CreatedAt:        c.CreatedAt,
	}
	// 排除：AffiliateProfileID、OrderItemID、BaseAmount、RatePercent、
	// WithdrawRequestID、InvalidReason、UpdatedAt、关联 Order/AffiliateProfile/WithdrawRequest
}

// NewCommissionList 批量转换佣金列表
func NewCommissionList(commissions []affiliatedomain.Commission) []Commission {
	result := make([]Commission, 0, len(commissions))
	for i := range commissions {
		result = append(result, NewCommission(&commissions[i]))
	}
	return result
}

// Withdraw 提现记录响应
type Withdraw struct {
	ID           uint         `json:"id"`
	Amount       money.Amount `json:"amount"`
	Channel      string       `json:"channel"`
	Account      string       `json:"account"`
	Status       string       `json:"status"`
	RejectReason string       `json:"reject_reason,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// NewWithdraw 从 affiliatedomain.WithdrawRequest 构造响应
func NewWithdraw(w *affiliatedomain.WithdrawRequest) Withdraw {
	return Withdraw{
		ID:           w.ID,
		Amount:       w.Amount,
		Channel:      w.Channel,
		Account:      w.Account,
		Status:       w.Status,
		RejectReason: w.RejectReason,
		CreatedAt:    w.CreatedAt,
	}
	// 排除：AffiliateProfileID、ProcessedBy、ProcessedAt、UpdatedAt、关联
}

// NewWithdrawList 批量转换提现列表
func NewWithdrawList(withdraws []affiliatedomain.WithdrawRequest) []Withdraw {
	result := make([]Withdraw, 0, len(withdraws))
	for i := range withdraws {
		result = append(result, NewWithdraw(&withdraws[i]))
	}
	return result
}

// TransferRecord 划转记录响应（从 transfer_to_wallet 类型的 CommissionLedger 转换）。
type TransferRecord struct {
	ID        uint         `json:"id"`
	Amount    money.Amount `json:"amount"` // 划转金额（正数展示）
	Type      string       `json:"type"`
	Reference string       `json:"reference"`
	Remark    string       `json:"remark"`
	CreatedAt time.Time    `json:"created_at"`
}

// NewTransferRecord 从 CommissionLedger 构造划转记录响应。
// ledger 中 transfer_to_wallet 为负金额，响应取绝对值展示。
func NewTransferRecord(l *affiliatedomain.CommissionLedger) TransferRecord {
	return TransferRecord{
		ID:        l.ID,
		Amount:    money.FromDecimal(l.Amount.Decimal.Abs()),
		Type:      l.Type,
		Reference: l.Reference,
		Remark:    l.Remark,
		CreatedAt: l.CreatedAt,
	}
}

// NewTransferRecordList 批量转换划转记录列表。
func NewTransferRecordList(ledgers []affiliatedomain.CommissionLedger) []TransferRecord {
	result := make([]TransferRecord, 0, len(ledgers))
	for i := range ledgers {
		result = append(result, NewTransferRecord(&ledgers[i]))
	}
	return result
}

// Application 推广申请响应
type Application struct {
	ID         uint       `json:"id"`
	UserID     uint       `json:"user_id"`
	Status     string     `json:"status"`
	Reason     string     `json:"reason,omitempty"`
	ReviewNote string     `json:"review_note,omitempty"`
	ReviewedBy uint       `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// NewApplication 从 affiliatedomain.Application 构造响应
func NewApplication(a *affiliatedomain.Application) Application {
	if a == nil {
		return Application{}
	}
	return Application{
		ID:         a.ID,
		UserID:     a.UserID,
		Status:     a.Status,
		Reason:     a.Reason,
		ReviewNote: a.ReviewNote,
		ReviewedBy: a.ReviewedBy,
		ReviewedAt: a.ReviewedAt,
		CreatedAt:  a.CreatedAt,
	}
}

// NewApplicationList 批量转换申请列表
func NewApplicationList(apps []affiliatedomain.Application) []Application {
	result := make([]Application, 0, len(apps))
	for i := range apps {
		result = append(result, NewApplication(&apps[i]))
	}
	return result
}
