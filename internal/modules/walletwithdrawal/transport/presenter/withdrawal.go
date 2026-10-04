package presenter

import (
	"time"

	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
)

// WithdrawalResp 提现单响应 DTO。
type WithdrawalResp struct {
	ID            uint         `json:"id"`
	WithdrawalNo  string       `json:"withdrawal_no"`
	Network       string       `json:"network"`
	Address       string       `json:"address"`
	RequestAmount money.Amount `json:"request_amount"`
	FeeAmount     money.Amount `json:"fee_amount"`
	NetAmount     money.Amount `json:"net_amount"`
	Currency      string       `json:"currency"`
	Status        string       `json:"status"`
	Txid          string       `json:"txid,omitempty"`
	UserNote      string       `json:"user_note,omitempty"`
	AdminNote     string       `json:"admin_note,omitempty"`
	RejectReason  string       `json:"reject_reason,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	ApprovedAt    *time.Time   `json:"approved_at,omitempty"`
	ProcessingAt  *time.Time   `json:"processing_at,omitempty"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
	RejectedAt    *time.Time   `json:"rejected_at,omitempty"`
	CanceledAt    *time.Time   `json:"canceled_at,omitempty"`
}

// NewWithdrawalResp 从 domain.Withdrawal 构造响应。
func NewWithdrawalResp(w *withdrawaldomain.Withdrawal) WithdrawalResp {
	if w == nil {
		return WithdrawalResp{}
	}
	return WithdrawalResp{
		ID:            w.ID,
		WithdrawalNo:  w.WithdrawalNo,
		Network:       w.Network,
		Address:       w.Address,
		RequestAmount: w.RequestAmount,
		FeeAmount:     w.FeeAmount,
		NetAmount:     w.NetAmount,
		Currency:      "USDT",
		Status:        w.Status,
		Txid:          w.Txid,
		UserNote:      w.UserNote,
		AdminNote:     w.AdminNote,
		RejectReason:  w.RejectReason,
		CreatedAt:     w.CreatedAt,
		ApprovedAt:    w.ApprovedAt,
		ProcessingAt:  w.ProcessingAt,
		CompletedAt:   w.CompletedAt,
		RejectedAt:    w.RejectedAt,
		CanceledAt:    w.CanceledAt,
	}
}

// NewWithdrawalRespList 批量转换。
func NewWithdrawalRespList(rows []withdrawaldomain.Withdrawal) []WithdrawalResp {
	result := make([]WithdrawalResp, 0, len(rows))
	for i := range rows {
		result = append(result, NewWithdrawalResp(&rows[i]))
	}
	return result
}

// AddressResp 提现地址响应 DTO。
type AddressResp struct {
	ID        uint   `json:"id"`
	Network   string `json:"network"`
	Address   string `json:"address"`
	Label     string `json:"label"`
	IsDefault bool   `json:"is_default"`
}

// NewAddressResp 从 domain.Address 构造响应。
func NewAddressResp(a *withdrawaldomain.Address) AddressResp {
	if a == nil {
		return AddressResp{}
	}
	return AddressResp{
		ID:        a.ID,
		Network:   a.Network,
		Address:   a.Address,
		Label:     a.Label,
		IsDefault: a.IsDefault,
	}
}

// NewAddressRespList 批量转换。
func NewAddressRespList(rows []withdrawaldomain.Address) []AddressResp {
	result := make([]AddressResp, 0, len(rows))
	for i := range rows {
		result = append(result, NewAddressResp(&rows[i]))
	}
	return result
}
