package http

import (
	"strings"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	c2cpresenter "github.com/Aether-v1/hcz/internal/modules/c2c/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ListPaymentMethods 我的收款方式列表（脱敏）。支持 ?type= 筛选。
func (h *Handler) ListPaymentMethods(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	pmType := strings.TrimSpace(c.Query("type"))
	var rows []c2cdomain.PaymentMethod
	var err error
	if pmType != "" {
		rows, err = h.svc.ListPaymentMethodsByType(uid, pmType)
	} else {
		rows, err = h.svc.ListPaymentMethods(uid)
	}
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodListResult(rows))
}

// GetPaymentMethod 单条收款方式详情（仅本人，返回解密后的完整数据）。
func (h *Handler) GetPaymentMethod(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	pm, err := h.svc.GetPaymentMethod(uid, id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodDetailResp(pm))
}

type createPaymentMethodRequest struct {
	Type              string `json:"type" binding:"required"`
	Currency          string `json:"currency"`
	Network           string `json:"network"`
	Address           string `json:"address"`
	Label             string `json:"label"`
	AccountName       string `json:"account_name"`
	BankName          string `json:"bank_name"`
	BankAccount       string `json:"bank_account"`
	BranchName        string `json:"branch_name"`
	AccountIdentifier string `json:"account_identifier"`
	QRCodeFileID      string `json:"qr_code_file_id"`
	QRCodeURL         string `json:"qr_code_url"`
	Instructions      string `json:"instructions"`
	TOTPCode          string `json:"totp_code"`
	Password          string `json:"password"`
}

// CreatePaymentMethod 新增收款方式（含 Step-Up 安全验证）。
func (h *Handler) CreatePaymentMethod(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req createPaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	pm, err := h.svc.CreatePaymentMethod(c2ccontract.CreatePaymentMethodInput{
		UserID:            uid,
		Type:              strings.TrimSpace(req.Type),
		Currency:          strings.TrimSpace(req.Currency),
		Network:           strings.TrimSpace(req.Network),
		Address:           strings.TrimSpace(req.Address),
		Label:             strings.TrimSpace(req.Label),
		AccountName:       strings.TrimSpace(req.AccountName),
		BankName:          strings.TrimSpace(req.BankName),
		BankAccount:       strings.TrimSpace(req.BankAccount),
		BranchName:        strings.TrimSpace(req.BranchName),
		AccountIdentifier: strings.TrimSpace(req.AccountIdentifier),
		QRCodeFileID:      strings.TrimSpace(req.QRCodeFileID),
		QRCodeURL:         strings.TrimSpace(req.QRCodeURL),
		Instructions:      strings.TrimSpace(req.Instructions),
		TOTPCode:          strings.TrimSpace(req.TOTPCode),
		Password:          req.Password,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodDetailResp(pm))
}

type updatePaymentMethodRequest struct {
	Label             string `json:"label"`
	Address           string `json:"address"`
	AccountName       string `json:"account_name"`
	BankName          string `json:"bank_name"`
	BankAccount       string `json:"bank_account"`
	BranchName        string `json:"branch_name"`
	AccountIdentifier string `json:"account_identifier"`
	QRCodeFileID      string `json:"qr_code_file_id"`
	QRCodeURL         string `json:"qr_code_url"`
	Instructions      string `json:"instructions"`
	TOTPCode          string `json:"totp_code"`
	Password          string `json:"password"`
}

// UpdatePaymentMethod 修改收款方式（含 Step-Up）。
func (h *Handler) UpdatePaymentMethod(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req updatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	pm, err := h.svc.UpdatePaymentMethod(c2ccontract.UpdatePaymentMethodInput{
		ID:                id,
		UserID:            uid,
		Label:             strings.TrimSpace(req.Label),
		Address:           strings.TrimSpace(req.Address),
		AccountName:       strings.TrimSpace(req.AccountName),
		BankName:          strings.TrimSpace(req.BankName),
		BankAccount:       strings.TrimSpace(req.BankAccount),
		BranchName:        strings.TrimSpace(req.BranchName),
		AccountIdentifier: strings.TrimSpace(req.AccountIdentifier),
		QRCodeFileID:      strings.TrimSpace(req.QRCodeFileID),
		QRCodeURL:         strings.TrimSpace(req.QRCodeURL),
		Instructions:      strings.TrimSpace(req.Instructions),
		TOTPCode:          strings.TrimSpace(req.TOTPCode),
		Password:          req.Password,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodDetailResp(pm))
}

type pmStepUpRequest struct {
	TOTPCode string `json:"totp_code"`
	Password string `json:"password"`
}

// DeletePaymentMethod 删除收款方式（含 Step-Up）。
func (h *Handler) DeletePaymentMethod(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req pmStepUpRequest
	_ = c.ShouldBindJSON(&req) // DELETE 允许空 body，绑定失败按空凭证处理
	if err := h.svc.DeletePaymentMethod(uid, id, strings.TrimSpace(req.TOTPCode), req.Password); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// SetDefaultPaymentMethod 设为同类型默认（含 Step-Up）。
func (h *Handler) SetDefaultPaymentMethod(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req pmStepUpRequest
	_ = c.ShouldBindJSON(&req) // 允许空 body
	if err := h.svc.SetDefaultPaymentMethod(uid, id, strings.TrimSpace(req.TOTPCode), req.Password); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, gin.H{"is_default": true, "id": id})
}

type setPaymentMethodEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// SetPaymentMethodEnabled 启用/停用收款方式。
func (h *Handler) SetPaymentMethodEnabled(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req setPaymentMethodEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	pm, err := h.svc.SetPaymentMethodEnabled(uid, id, req.Enabled)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodResp(pm))
}
