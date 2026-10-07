package http

import (
	"strings"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cpresenter "github.com/Aether-v1/hcz/internal/modules/c2c/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type createTradeRequest struct {
	ListingID       uint   `json:"listing_id" binding:"required"`
	USDTAmount      string `json:"usdt_amount" binding:"required"`
	PaymentMethodID uint   `json:"payment_method_id"`
}

// CreateTrade 发起交易（支持 Idempotency-Key header）。
func (h *Handler) CreateTrade(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req createTradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, ok := parseAmount(req.USDTAmount)
	if !ok {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.idempotency_key_required", nil)
		return
	}
	t, err := h.svc.CreateTrade(c2ccontract.CreateTradeInput{
		BuyerUserID:    uid,
		ListingID:      req.ListingID,
		USDTAmount:     amount,
		IdempotencyKey: idempotencyKey,
		PaymentMethodID: req.PaymentMethodID,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewTradeResp(t))
}

// ListMyTrades 我的交易列表。
func (h *Handler) ListMyTrades(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.svc.ListMyTrades(c2ccontract.TradeListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   uid,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.SuccessWithPage(c, c2cpresenter.NewTradeRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetTradeDetail 交易详情。
func (h *Handler) GetTradeDetail(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	t, err := h.svc.GetTradeDetail(uid, id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewTradeResp(t))
}

type markPaidRequest struct {
	PaymentReference string `json:"payment_reference"`
}

// MarkPaid 买家标记已付款。
func (h *Handler) MarkPaid(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req markPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	t, err := h.svc.MarkPaid(c2ccontract.MarkPaidInput{
		TradeID:          id,
		BuyerUserID:      uid,
		PaymentReference: strings.TrimSpace(req.PaymentReference),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewTradeResp(t))
}

// Cancel 买家取消待付款交易。
func (h *Handler) Cancel(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	t, err := h.svc.Cancel(c2ccontract.CancelTradeInput{
		TradeID:     id,
		BuyerUserID: uid,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewTradeResp(t))
}

// Confirm 卖家确认放行。
func (h *Handler) Confirm(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	t, err := h.svc.Confirm(c2ccontract.ConfirmTradeInput{
		TradeID:  id,
		SellerID: uid,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewTradeResp(t))
}

type disputeRequest struct {
	Reason      string `json:"reason" binding:"required"`
	Description string `json:"description"`
	Evidence    string `json:"evidence"`
}

// Dispute 发起争议（仅 buyer/seller，且 trade 处于 paid 状态）。
func (h *Handler) Dispute(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req disputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	d, err := h.svc.InitiateDispute(c2ccontract.InitiateDisputeInput{
		UserID:      uid,
		TradeID:     id,
		Reason:      strings.TrimSpace(req.Reason),
		Description: strings.TrimSpace(req.Description),
		Evidence:    strings.TrimSpace(req.Evidence),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewDisputeResp(d))
}
