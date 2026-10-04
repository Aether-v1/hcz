// Package presenter 把 C2C 领域对象转换为对外响应 DTO，并对敏感字段脱敏。
package presenter

import (
	"strings"

	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
)

// PaymentMethodResp 支付方式响应（账号标识已脱敏）。
type PaymentMethodResp struct {
	ID                uint   `json:"id"`
	Type              string `json:"type"`
	AccountName       string `json:"account_name"`
	AccountIdentifier string `json:"account_identifier"` // 脱敏后的卡号/账号/收款标识
	QRImage           string `json:"qr_image"`
	Instructions      string `json:"instructions"`
	Enabled           bool   `json:"enabled"`
}

// MaskIdentifier 脱敏：保留前 4 与后 4，中间打码；短串做最小打码。
func MaskIdentifier(s string) string {
	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		return ""
	}
	if n <= 2 {
		return strings.Repeat("*", n)
	}
	if n <= 8 {
		return s[:1] + strings.Repeat("*", n-2) + s[n-1:]
	}
	return s[:4] + strings.Repeat("*", n-8) + s[n-4:]
}

// NewPaymentMethodResp 单个支付方式响应（脱敏）。
func NewPaymentMethodResp(p *c2cdomain.PaymentMethod) PaymentMethodResp {
	if p == nil {
		return PaymentMethodResp{}
	}
	return PaymentMethodResp{
		ID:                p.ID,
		Type:              p.Type,
		AccountName:       p.AccountName,
		AccountIdentifier: MaskIdentifier(p.AccountIdentifier),
		QRImage:           p.QRImage,
		Instructions:      p.Instructions,
		Enabled:           p.Enabled,
	}
}

// NewPaymentMethodRespList 支付方式列表响应（脱敏）。
func NewPaymentMethodRespList(rows []c2cdomain.PaymentMethod) []PaymentMethodResp {
	out := make([]PaymentMethodResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewPaymentMethodResp(&rows[i]))
	}
	return out
}

// ListingResp 挂单响应。
type ListingResp struct {
	ID            uint   `json:"id"`
	ListingNo     string `json:"listing_no"`
	SellerUserID  uint   `json:"seller_user_id"`
	FiatCurrency  string `json:"fiat_currency"`
	Price         string `json:"price"`
	MinFiatAmount string `json:"min_fiat_amount"`
	MaxFiatAmount string `json:"max_fiat_amount"`
	TotalUSDT     string `json:"total_usdt"`
	AvailableUSDT string `json:"available_usdt"`
	Status        string `json:"status"`
	Terms         string `json:"terms"`
}

// NewListingResp 单个挂单响应。
func NewListingResp(l *c2cdomain.Listing) ListingResp {
	if l == nil {
		return ListingResp{}
	}
	return ListingResp{
		ID:            l.ID,
		ListingNo:     l.ListingNo,
		SellerUserID:  l.SellerUserID,
		FiatCurrency:  l.FiatCurrency,
		Price:         l.Price.String(),
		MinFiatAmount: l.MinFiatAmount.String(),
		MaxFiatAmount: l.MaxFiatAmount.String(),
		TotalUSDT:     l.TotalUSDT.String(),
		AvailableUSDT: l.AvailableUSDT.String(),
		Status:        l.Status,
		Terms:         l.Terms,
	}
}

// NewListingRespList 挂单列表响应。
func NewListingRespList(rows []c2cdomain.Listing) []ListingResp {
	out := make([]ListingResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewListingResp(&rows[i]))
	}
	return out
}

// TradeResp 交易响应。
type TradeResp struct {
	ID                    uint   `json:"id"`
	TradeNo               string `json:"trade_no"`
	ListingID             uint   `json:"listing_id"`
	BuyerUserID           uint   `json:"buyer_user_id"`
	SellerUserID          uint   `json:"seller_user_id"`
	FiatCurrency          string `json:"fiat_currency"`
	Price                 string `json:"price"`
	FiatAmount            string `json:"fiat_amount"`
	USDTAmount            string `json:"usdt_amount"`
	FeeAmount             string `json:"fee_amount"`
	BuyerReceiveUSDT      string `json:"buyer_receive_usdt"`
	Status                string `json:"status"`
	PaymentMethodSnapshot string `json:"payment_method_snapshot,omitempty"`
	PaymentReference      string `json:"payment_reference,omitempty"`
	CreatedAt             string `json:"created_at"`
	ExpiredAt             string `json:"expired_at"`
}

// NewTradeResp 单个交易响应。
func NewTradeResp(t *c2cdomain.Trade) TradeResp {
	if t == nil {
		return TradeResp{}
	}
	return TradeResp{
		ID:                    t.ID,
		TradeNo:               t.TradeNo,
		ListingID:             t.ListingID,
		BuyerUserID:           t.BuyerUserID,
		SellerUserID:          t.SellerUserID,
		FiatCurrency:          t.FiatCurrency,
		Price:                 t.Price.String(),
		FiatAmount:            t.FiatAmount.String(),
		USDTAmount:            t.USDTAmount.String(),
		FeeAmount:             t.FeeAmount.String(),
		BuyerReceiveUSDT:      t.BuyerReceiveUSDT.String(),
		Status:                t.Status,
		PaymentMethodSnapshot: t.PaymentMethodSnapshot,
		PaymentReference:      t.PaymentReference,
		CreatedAt:             t.CreatedAt.Format("2006-01-02 15:04:05"),
		ExpiredAt:             t.ExpiredAt.Format("2006-01-02 15:04:05"),
	}
}

// NewTradeRespList 交易列表响应。
func NewTradeRespList(rows []c2cdomain.Trade) []TradeResp {
	out := make([]TradeResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewTradeResp(&rows[i]))
	}
	return out
}

// DisputeResp 申诉响应。
type DisputeResp struct {
	ID              uint   `json:"id"`
	TradeID         uint   `json:"trade_id"`
	InitiatorUserID uint   `json:"initiator_user_id"`
	Reason          string `json:"reason"`
	Description     string `json:"description"`
	Evidence        string `json:"evidence"`
	Status          string `json:"status"`
	AdminResult     string `json:"admin_result"`
	AdminNote       string `json:"admin_note"`
	CreatedAt       string `json:"created_at"`
}

// NewDisputeResp 单个申诉响应。
func NewDisputeResp(d *c2cdomain.Dispute) DisputeResp {
	if d == nil {
		return DisputeResp{}
	}
	return DisputeResp{
		ID:              d.ID,
		TradeID:         d.TradeID,
		InitiatorUserID: d.InitiatorUserID,
		Reason:          d.Reason,
		Description:     d.Description,
		Evidence:        d.Evidence,
		Status:          d.Status,
		AdminResult:     d.AdminResult,
		AdminNote:       d.AdminNote,
		CreatedAt:       d.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// NewDisputeRespList 申诉列表响应。
func NewDisputeRespList(rows []c2cdomain.Dispute) []DisputeResp {
	out := make([]DisputeResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewDisputeResp(&rows[i]))
	}
	return out
}

// RiskSignalResp 风控信号响应。
type RiskSignalResp struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	SignalType string `json:"signal_type"`
	TradeID    uint   `json:"trade_id,omitempty"`
	Metadata   string `json:"metadata"`
	CreatedAt  string `json:"created_at"`
}

// NewRiskSignalResp 单个风控信号响应。
func NewRiskSignalResp(r *c2cdomain.RiskSignal) RiskSignalResp {
	if r == nil {
		return RiskSignalResp{}
	}
	var tradeID uint
	if r.TradeID != nil {
		tradeID = *r.TradeID
	}
	return RiskSignalResp{
		ID:         r.ID,
		UserID:     r.UserID,
		SignalType: r.SignalType,
		TradeID:    tradeID,
		Metadata:   r.Metadata,
		CreatedAt:  r.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// NewRiskSignalRespList 风控信号列表响应。
func NewRiskSignalRespList(rows []c2cdomain.RiskSignal) []RiskSignalResp {
	out := make([]RiskSignalResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewRiskSignalResp(&rows[i]))
	}
	return out
}
