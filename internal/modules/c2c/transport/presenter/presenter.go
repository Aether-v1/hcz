// Package presenter 把 C2C 领域对象转换为对外响应 DTO，并对敏感字段脱敏。
package presenter

import (
	"strings"

	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
)

// PaymentMethodResp 收款方式列表响应（敏感字段脱敏）。
type PaymentMethodResp struct {
	ID                 uint   `json:"id"`
	Type               string `json:"type"`
	Currency           string `json:"currency"`
	Network            string `json:"network"`
	Label              string `json:"label"`
	AddressMasked      string `json:"address_masked,omitempty"`       // USDT 地址：前4...后4
	BankName           string `json:"bank_name,omitempty"`
	BankAccountMasked  string `json:"bank_account_masked,omitempty"`  // 银行卡号：**** **** **** 后4
	BranchName         string `json:"branch_name,omitempty"`
	AccountNameMasked  string `json:"account_name_masked,omitempty"`  // 姓名：张**
	AccountIdentMasked string `json:"account_identifier_masked,omitempty"` // 支付宝/微信号：前3****后4
	QRCodeURL          string `json:"qr_code_url,omitempty"`
	IsDefault          bool   `json:"is_default"`
	Status             string `json:"status"`
	Enabled            bool   `json:"enabled"`
	CreatedAt          string `json:"created_at"`
}

// PaymentMethodDetailResp 单条收款方式详情（仅本人，返回解密后的完整数据）。
type PaymentMethodDetailResp struct {
	ID                uint   `json:"id"`
	Type              string `json:"type"`
	Currency          string `json:"currency"`
	Network           string `json:"network"`
	Address           string `json:"address,omitempty"`
	Label             string `json:"label"`
	AccountName       string `json:"account_name"`
	BankName          string `json:"bank_name,omitempty"`
	BankAccount       string `json:"bank_account,omitempty"`
	BranchName        string `json:"branch_name,omitempty"`
	AccountIdentifier string `json:"account_identifier,omitempty"`
	QRCodeFileID      string `json:"qr_code_file_id,omitempty"`
	QRCodeURL         string `json:"qr_code_url,omitempty"`
	Instructions      string `json:"instructions,omitempty"`
	IsDefault         bool   `json:"is_default"`
	Status            string `json:"status"`
	Enabled           bool   `json:"enabled"`
	CreatedAt         string `json:"created_at"`
}

// PaymentMethodListResult 列表结果包装为 {items:[...]}。
type PaymentMethodListResult struct {
	Items []PaymentMethodResp `json:"items"`
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

// MaskUSDTAddress USDT 地址脱敏：前4 ... 后4。
func MaskUSDTAddress(s string) string {
	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		return ""
	}
	if n <= 8 {
		return MaskIdentifier(s)
	}
	return s[:4] + "..." + s[n-4:]
}

// MaskBankAccount 银行卡号脱敏：每4位一组，仅显示后4位。
func MaskBankAccount(s string) string {
	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return strings.Repeat("*", n)
	}
	return "**** **** **** " + s[n-4:]
}

// MaskAccountIdentifier 支付宝/微信号脱敏：前3 + **** + 后4。
func MaskAccountIdentifier(s string) string {
	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		return ""
	}
	if n <= 7 {
		return MaskIdentifier(s)
	}
	return s[:3] + "****" + s[n-4:]
}

// MaskName 姓名脱敏：保留姓，名用 * 替代。
func MaskName(s string) string {
	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		return ""
	}
	// 中文按 rune 处理。
	r := []rune(s)
	if len(r) == 1 {
		return s
	}
	return string(r[0]) + strings.Repeat("*", len(r)-1)
}

// NewPaymentMethodResp 单个收款方式响应（列表/脱敏视图）。
func NewPaymentMethodResp(p *c2cdomain.PaymentMethod) PaymentMethodResp {
	if p == nil {
		return PaymentMethodResp{}
	}
	resp := PaymentMethodResp{
		ID:        p.ID,
		Type:      p.Type,
		Currency:  p.Currency,
		Network:   p.Network,
		Label:     p.Label,
		BankName:  p.BankName,
		BranchName: p.BranchName,
		QRCodeURL: firstNonEmpty(p.QRCodeURL, p.QRImage),
		IsDefault: p.IsDefault,
		Status:    p.Status,
		Enabled:   p.Enabled,
		CreatedAt: p.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	switch p.Type {
	case c2cdomain.PaymentMethodTypeUSDTTRC20:
		resp.AddressMasked = MaskUSDTAddress(p.Address)
	case c2cdomain.PaymentMethodTypeBankCard:
		resp.AccountNameMasked = MaskName(p.AccountName)
		resp.BankAccountMasked = MaskBankAccount(p.BankAccount)
	default: // ALIPAY / WECHAT
		resp.AccountNameMasked = MaskName(p.AccountName)
		resp.AccountIdentMasked = MaskAccountIdentifier(p.AccountIdentifier)
	}
	return resp
}

// NewPaymentMethodRespList 收款方式列表响应（脱敏）。
func NewPaymentMethodRespList(rows []c2cdomain.PaymentMethod) []PaymentMethodResp {
	out := make([]PaymentMethodResp, 0, len(rows))
	for i := range rows {
		out = append(out, NewPaymentMethodResp(&rows[i]))
	}
	return out
}

// NewPaymentMethodListResult 列表响应包装为 {items:[...]}（脱敏）。
func NewPaymentMethodListResult(rows []c2cdomain.PaymentMethod) PaymentMethodListResult {
	return PaymentMethodListResult{Items: NewPaymentMethodRespList(rows)}
}

// NewPaymentMethodDetailResp 单条详情响应（仅本人，完整解密数据）。
func NewPaymentMethodDetailResp(p *c2cdomain.PaymentMethod) PaymentMethodDetailResp {
	if p == nil {
		return PaymentMethodDetailResp{}
	}
	return PaymentMethodDetailResp{
		ID:                p.ID,
		Type:              p.Type,
		Currency:          p.Currency,
		Network:           p.Network,
		Address:           p.Address,
		Label:             p.Label,
		AccountName:       p.AccountName,
		BankName:          p.BankName,
		BankAccount:       p.BankAccount,
		BranchName:        p.BranchName,
		AccountIdentifier: p.AccountIdentifier,
		QRCodeFileID:      p.QRCodeFileID,
		QRCodeURL:         firstNonEmpty(p.QRCodeURL, p.QRImage),
		Instructions:      p.Instructions,
		IsDefault:         p.IsDefault,
		Status:            p.Status,
		Enabled:           p.Enabled,
		CreatedAt:         p.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
	CreatedAt     string `json:"created_at"`
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
		CreatedAt:     l.CreatedAt.Format("2006-01-02 15:04:05"),
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

// NewTradeRespList 交易列表响应（列表不回传收款快照，仅详情按角色/状态返回）。
func NewTradeRespList(rows []c2cdomain.Trade) []TradeResp {
	out := make([]TradeResp, 0, len(rows))
	for i := range rows {
		resp := NewTradeResp(&rows[i])
		resp.PaymentMethodSnapshot = ""
		out = append(out, resp)
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
