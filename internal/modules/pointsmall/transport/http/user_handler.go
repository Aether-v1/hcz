package pointsmallphttp

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	pointsmallcontract "github.com/Aether-v1/hcz/internal/modules/pointsmall/contract"
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
)

// UserService 是用户端积分商城所需的最小端口。
type UserService interface {
	ListProducts(filter pointsmallcontract.ProductListFilter) ([]pointsmalldomain.PointsProduct, int64, error)
	GetProductDetail(userID, productID uint) (*pointsmallcontract.ProductDetail, error)
	CreateExchange(input pointsmallcontract.CreateExchangeInput) (*pointsmallcontract.ExchangeResult, error)
	ListExchangeOrders(userID uint, filter pointsmallcontract.ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error)
	GetExchangeOrder(userID, orderID uint) (*pointsmalldomain.ExchangeOrder, error)
	CancelOrder(input pointsmallcontract.CancelInput) (*pointsmallcontract.ExchangeResult, error)
}

// UserHandler 处理用户端积分商城 HTTP 请求。
type UserHandler struct {
	service UserService
}

// NewUserHandler 创建用户 handler。
func NewUserHandler(service UserService) *UserHandler {
	if service == nil {
		panic("points mall user handler: required dependency is nil")
	}
	return &UserHandler{service: service}
}

// ListProducts 用户积分商品列表（仅 enabled，分页，sort ASC）。
func (h *UserHandler) ListProducts(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	enabled := true
	products, total, err := h.service.ListProducts(pointsmallcontract.ProductListFilter{
		Enabled:  &enabled,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_product_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, products, pagination)
}

// GetProductDetail 用户积分商品详情（含 can_redeem 辅助；POST 时后端重新校验）。
func (h *UserHandler) GetProductDetail(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	detail, err := h.service.GetProductDetail(uid, productID)
	if err != nil {
		switch {
		case errors.Is(err, pointsmallcontract.ErrProductNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.points_product_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.points_product_fetch_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"product":             detail.Product,
		"user_redeemed_count": detail.UserRedeemedCount,
		"can_redeem":          detail.CanRedeem,
		"reason_code":         detail.ReasonCode,
	})
}

// CreateExchangeRequest 创建兑换请求体。
// 客户端禁止提交 points_price / total_points / product_name / user_id；
// quantity V1 固定为 1，不接受客户端数量。
type CreateExchangeRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
}

// CreateExchange 创建积分兑换订单（Idempotency-Key 必填，重复 Key 幂等返回原订单）。
func (h *UserHandler) CreateExchange(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.idempotency_key_required", nil)
		return
	}
	if len([]rune(key)) > pointsmallcontract.MaxIdempotencyKeyLength {
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_idempotency_key_too_long", nil)
		return
	}
	var req CreateExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	result, err := h.service.CreateExchange(pointsmallcontract.CreateExchangeInput{
		UserID:         uid,
		ProductID:      req.ProductID,
		IdempotencyKey: key,
	})
	if err != nil {
		switch {
		case errors.Is(err, pointsmallcontract.ErrPointsInsufficient):
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_balance_insufficient", nil)
		case errors.Is(err, pointsmallcontract.ErrProductDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_product_disabled", nil)
		case errors.Is(err, pointsmallcontract.ErrProductOutOfStock):
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_product_out_of_stock", nil)
		case errors.Is(err, pointsmallcontract.ErrProductLimitReached):
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_product_limit_reached", nil)
		case errors.Is(err, pointsmallcontract.ErrProductNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.points_product_not_found", nil)
		case errors.Is(err, pointsmallcontract.ErrIdempotencyConflict):
			ginutil.RespondError(c, response.CodeConflict, "error.idempotency_conflict", nil)
		case errors.Is(err, pointsmallcontract.ErrIdempotencyKeyRequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.idempotency_key_required", nil)
		case errors.Is(err, pointsmallcontract.ErrIdempotencyKeyTooLong):
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_idempotency_key_too_long", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.points_exchange_create_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"order":             result.Order,
		"current_balance":   result.CurrentBalance,
		"already_processed": result.AlreadyProcessed,
	})
}

// ListExchangeOrders 用户兑换订单列表（仅自己的订单；?status= 过滤）。
func (h *UserHandler) ListExchangeOrders(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	if !pointsmalldomain.IsValidExchangeStatus(status) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	orders, total, err := h.service.ListExchangeOrders(uid, pointsmallcontract.ExchangeOrderListFilter{
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_exchange_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, orders, pagination)
}

// GetExchangeOrder 用户兑换订单详情（store 层 id + user_id 限定，IDOR 防御）。
func (h *UserHandler) GetExchangeOrder(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, err := h.service.GetExchangeOrder(uid, orderID)
	if err != nil {
		switch {
		case errors.Is(err, pointsmallcontract.ErrExchangeOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.points_exchange_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.points_exchange_fetch_failed", err)
		}
		return
	}
	response.Success(c, order)
}

// CancelExchangeOrder 用户取消兑换（仅 PENDING；重复取消幂等返回当前状态）。
func (h *UserHandler) CancelExchangeOrder(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	result, err := h.service.CancelOrder(pointsmallcontract.CancelInput{
		UserID:  uid,
		OrderID: orderID,
	})
	if err != nil {
		switch {
		case errors.Is(err, pointsmallcontract.ErrExchangeOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.points_exchange_not_found", nil)
		case errors.Is(err, pointsmallcontract.ErrExchangeInvalidState):
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_exchange_invalid_state", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.points_exchange_action_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"order":             result.Order,
		"current_balance":   result.CurrentBalance,
		"already_processed": result.AlreadyProcessed,
	})
}
