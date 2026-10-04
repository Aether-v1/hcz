package http

import (
	"strings"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cpresenter "github.com/Aether-v1/hcz/internal/modules/c2c/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ListPaymentMethods 我的支付方式列表（脱敏）。
func (h *Handler) ListPaymentMethods(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListPaymentMethods(uid)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodRespList(rows))
}

type createPaymentMethodRequest struct {
	Type              string `json:"type" binding:"required"`
	AccountName       string `json:"account_name"`
	AccountIdentifier string `json:"account_identifier" binding:"required"`
	QRImage           string `json:"qr_image"`
	Instructions      string `json:"instructions"`
}

// CreatePaymentMethod 新增支付方式。
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
		AccountName:       strings.TrimSpace(req.AccountName),
		AccountIdentifier: strings.TrimSpace(req.AccountIdentifier),
		QRImage:           strings.TrimSpace(req.QRImage),
		Instructions:      strings.TrimSpace(req.Instructions),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodResp(pm))
}

type updatePaymentMethodRequest struct {
	AccountName       string `json:"account_name"`
	AccountIdentifier string `json:"account_identifier"`
	QRImage           string `json:"qr_image"`
	Instructions      string `json:"instructions"`
}

// UpdatePaymentMethod 修改支付方式。
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
		AccountName:       strings.TrimSpace(req.AccountName),
		AccountIdentifier: strings.TrimSpace(req.AccountIdentifier),
		QRImage:           strings.TrimSpace(req.QRImage),
		Instructions:      strings.TrimSpace(req.Instructions),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodResp(pm))
}

// DeletePaymentMethod 删除支付方式。
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
	if err := h.svc.DeletePaymentMethod(uid, id); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

type setPaymentMethodEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// SetPaymentMethodEnabled 启用/停用支付方式。
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
