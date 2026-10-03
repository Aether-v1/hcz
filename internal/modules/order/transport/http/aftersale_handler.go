package orderhttp

import (
	"errors"
	"time"

	"github.com/Aether-v1/hcz/internal/modules/order/application/aftersale"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

// AfterSaleOrderLookup 售后 DTO 所需的订单查询端口（由 order store 实现）。
type AfterSaleOrderLookup interface {
	GetByID(id uint) (*orderdomain.Order, error)
	GetByIDAndUser(id, userID uint) (*orderdomain.Order, error)
}

// AfterSaleHandler 处理用户端和管理端售后 HTTP。
// Handler 只负责 parse / auth / validate / DTO / error mapping，资金逻辑全部在 aftersale.Service。
type AfterSaleHandler struct {
	svc    *aftersale.Service
	orders AfterSaleOrderLookup
}

func NewAfterSaleHandler(svc *aftersale.Service, orders AfterSaleOrderLookup) *AfterSaleHandler {
	if svc == nil || orders == nil {
		panic("after-sale handler: required dependency is nil")
	}
	return &AfterSaleHandler{svc: svc, orders: orders}
}

// AfterSaleDTO 售后工单统一返回（User / Admin 共用）。
type AfterSaleDTO struct {
	ID             uint       `json:"id"`
	OrderID        uint       `json:"order_id"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	Reason         string     `json:"reason"`
	Description    string     `json:"description"`
	AdminNote      string     `json:"admin_note"`
	RefundAmount   string     `json:"refund_amount"`
	RefundCurrency string     `json:"refund_currency"`
	OrderStatus    string     `json:"order_status"`
	RefundStatus   string     `json:"refund_status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
}

func toAfterSaleDTO(t *orderdomain.AfterSaleTicket, order *orderdomain.Order) AfterSaleDTO {
	dto := AfterSaleDTO{
		ID:             t.ID,
		OrderID:        t.OrderID,
		Type:           t.Type,
		Status:         t.Status,
		Reason:         t.Reason,
		Description:    t.Description,
		AdminNote:      t.AdminNote,
		RefundCurrency: "USDT",
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
		ResolvedAt:     t.ResolvedAt,
	}
	if t.RefundAmount.Valid {
		dto.RefundAmount = t.RefundAmount.Decimal.StringFixed(2)
	}
	if order != nil {
		dto.OrderStatus = order.Status
		dto.RefundStatus = order.RefundStatus
	}
	return dto
}

// CreateAfterSaleRequest 用户发起售后请求。
type CreateAfterSaleRequest struct {
	Type        string `json:"type" binding:"required"`
	Reason      string `json:"reason" binding:"required"`
	Description string `json:"description"`
}

// UserCreateAfterSale POST /api/v1/orders/:id/after-sale
func (h *AfterSaleHandler) UserCreateAfterSale(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.unauthorized", nil)
		return
	}
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	var req CreateAfterSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if req.Type != orderdomain.AfterSaleTypeNotReceived {
		ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_invalid_type", nil)
		return
	}
	ticket, err := h.svc.Request(uid, orderID, req.Reason, req.Description)
	if err != nil {
		h.mapServiceError(c, err)
		return
	}
	order, _ := h.orders.GetByID(orderID)
	response.Success(c, toAfterSaleDTO(ticket, order))
}

// UserGetAfterSale GET /api/v1/orders/:id/after-sale
func (h *AfterSaleHandler) UserGetAfterSale(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.unauthorized", nil)
		return
	}
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	// 先校验订单归属（IDOR：非本人返回 404，不暴露存在性）
	order, err := h.orders.GetByIDAndUser(orderID, uid)
	if err != nil || order == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		return
	}
	ticket, err := h.svc.Get(orderID)
	if err != nil || ticket == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.after_sale_not_found", nil)
		return
	}
	response.Success(c, toAfterSaleDTO(ticket, order))
}

// AdminAfterSaleActionRequest 管理端售后操作请求。
type AdminAfterSaleActionRequest struct {
	Action       string `json:"action" binding:"required"` // reject / resolve / partial_refund / full_refund
	RefundAmount string `json:"refund_amount"`             // partial_refund 时必填
	AdminNote    string `json:"admin_note"`
}

// AdminGetAfterSale GET /api/admin/v1/orders/:id/after-sale
func (h *AfterSaleHandler) AdminGetAfterSale(c *gin.Context) {
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.orders.GetByID(orderID)
	if err != nil || order == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		return
	}
	ticket, err := h.svc.Get(orderID)
	if err != nil || ticket == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.after_sale_not_found", nil)
		return
	}
	response.Success(c, toAfterSaleDTO(ticket, order))
}

// AdminAfterSaleAction POST /api/admin/v1/orders/:id/after-sale/action
func (h *AfterSaleHandler) AdminAfterSaleAction(c *gin.Context) {
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	var req AdminAfterSaleActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	var ticket *orderdomain.AfterSaleTicket
	var svcErr error

	switch req.Action {
	case "reject":
		svcErr = h.svc.Reject(orderID, req.AdminNote)
	case "resolve":
		svcErr = h.svc.Resolve(orderID, req.AdminNote)
	case "partial_refund":
		if req.RefundAmount == "" {
			ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_refund_amount_required", nil)
			return
		}
		amount, err := decimal.NewFromString(req.RefundAmount)
		if err != nil || amount.LessThanOrEqual(decimal.Zero) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_invalid_refund_amount", nil)
			return
		}
		ticket, svcErr = h.svc.PartialRefund(orderID, amount)
	case "full_refund":
		ticket, svcErr = h.svc.FullRefund(orderID)
	default:
		ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_invalid_action", nil)
		return
	}

	if svcErr != nil {
		h.mapServiceError(c, svcErr)
		return
	}

	// reject/resolve 不返回 ticket，重新查询一次以返回最新状态
	if ticket == nil {
		ticket, _ = h.svc.Get(orderID)
	}
	order, _ := h.orders.GetByID(orderID)
	response.Success(c, toAfterSaleDTO(ticket, order))
}

// mapServiceError 将 aftersale.Service 错误映射为 HTTP 状态码。
func (h *AfterSaleHandler) mapServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, aftersale.ErrNotOwner):
		ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
	case errors.Is(err, aftersale.ErrNotCompleted):
		ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_order_not_completed", nil)
	case errors.Is(err, aftersale.ErrPendingExists):
		ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_pending_exists", nil)
	case errors.Is(err, aftersale.ErrTicketNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.after_sale_not_found", nil)
	case errors.Is(err, aftersale.ErrInvalidAction):
		ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_invalid_action", nil)
	default:
		// 退款超额、金额非法等资金层错误
		ginutil.RespondError(c, response.CodeBadRequest, "error.after_sale_action_failed", err)
	}
}
