package pointsmallphttp

import (
	"errors"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	pointsmallcontract "github.com/Aether-v1/hcz/internal/modules/pointsmall/contract"
	pointsmalldomain "github.com/Aether-v1/hcz/internal/modules/pointsmall/domain"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
)

// AdminService 是后台积分商城所需的最小端口。
type AdminService interface {
	ListProducts(filter pointsmallcontract.ProductListFilter) ([]pointsmalldomain.PointsProduct, int64, error)
	CreateProduct(input pointsmallcontract.ProductInput) (*pointsmalldomain.PointsProduct, error)
	UpdateProduct(id uint, input pointsmallcontract.ProductInput) (*pointsmalldomain.PointsProduct, error)
	SetProductEnabled(input pointsmallcontract.ProductEnabledInput) (*pointsmalldomain.PointsProduct, error)
	AdminListExchangeOrders(filter pointsmallcontract.ExchangeOrderListFilter) ([]pointsmalldomain.ExchangeOrder, int64, error)
	AdminGetExchangeOrder(orderID uint) (*pointsmalldomain.ExchangeOrder, error)
	AdminProcessOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
	AdminCompleteOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
	AdminFailOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
	AdminCancelOrder(input pointsmallcontract.AdminExchangeActionInput) (*pointsmalldomain.ExchangeOrder, error)
}

// AdminHandler 处理后台积分商城 HTTP 请求。
type AdminHandler struct {
	service AdminService
}

// NewAdminHandler 创建后台 handler。
func NewAdminHandler(service AdminService) *AdminHandler {
	if service == nil {
		panic("points mall admin handler: required dependency is nil")
	}
	return &AdminHandler{service: service}
}

// ProductRequest 商品创建/更新请求体。
// PointsPrice 为 BIGINT 整数（>0，且不超单笔积分上限）；Stock 为可用库存（绝对值）；
// Reason 是后台变更原因（库存/价格属资产字段，必须可追溯）；FulfillmentType V1 仅 MANUAL。
// 客户端禁止提交 id / created_at / updated_at / operator。
type ProductRequest struct {
	Name            string `json:"name" binding:"required"`
	Subtitle        string `json:"subtitle"`
	Description     string `json:"description"`
	Cover           string `json:"cover"`
	PointsPrice     int64  `json:"points_price"`
	Stock           int64  `json:"stock"`
	UnlimitedStock  bool   `json:"unlimited_stock"`
	Enabled         bool   `json:"enabled"`
	Sort            int    `json:"sort"`
	PerUserLimit    int64  `json:"per_user_limit"`
	FulfillmentType string `json:"fulfillment_type"`
	Instructions    string `json:"instructions"`
	Reason          string `json:"reason"`
}

func (r *ProductRequest) toInput(adminID uint) pointsmallcontract.ProductInput {
	return pointsmallcontract.ProductInput{
		Name:            r.Name,
		Subtitle:        r.Subtitle,
		Description:     r.Description,
		Cover:           r.Cover,
		PointsPrice:     r.PointsPrice,
		Stock:           r.Stock,
		UnlimitedStock:  r.UnlimitedStock,
		Enabled:         r.Enabled,
		Sort:            r.Sort,
		PerUserLimit:    r.PerUserLimit,
		FulfillmentType: strings.ToUpper(strings.TrimSpace(r.FulfillmentType)),
		Instructions:    r.Instructions,
		Reason:          strings.TrimSpace(r.Reason),
		OperatorAdminID: adminID,
	}
}

// ListProducts 后台商品列表（可含下架商品）。
func (h *AdminHandler) ListProducts(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	filter := pointsmallcontract.ProductListFilter{Page: page, PageSize: pageSize}
	enabled, err := ginutil.ParseQueryBoolPtr(c, "enabled")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	filter.Enabled = enabled
	products, total, err := h.service.ListProducts(filter)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_product_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, products, response.BuildPagination(page, pageSize, total))
}

// CreateProduct 创建积分商品。
func (h *AdminHandler) CreateProduct(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	product, err := h.service.CreateProduct(req.toInput(adminID))
	if err != nil {
		h.respondProductError(c, err)
		return
	}
	response.Success(c, product)
}

// UpdateProduct 更新积分商品（含可用库存绝对值设置；Admin Bearer + RBAC + 前后值审计）。
func (h *AdminHandler) UpdateProduct(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	product, err := h.service.UpdateProduct(id, req.toInput(adminID))
	if err != nil {
		h.respondProductError(c, err)
		return
	}
	response.Success(c, product)
}

// SetProductEnabledRequest 上下架请求体。
// Enabled 用 *bool + required：{enabled:false} 合法（下架），缺字段则绑定失败。
type SetProductEnabledRequest struct {
	Enabled *bool  `json:"enabled" binding:"required"`
	Reason  string `json:"reason"`
}

// SetProductEnabled 上下架商品（下架不删除；历史 PENDING 订单仍可继续履约）。
// 有兑换记录的商品一律以下架表达"不再可售"，本模块不提供物理删除入口。
func (h *AdminHandler) SetProductEnabled(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req SetProductEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	product, err := h.service.SetProductEnabled(pointsmallcontract.ProductEnabledInput{
		ProductID:       id,
		Enabled:         *req.Enabled,
		OperatorAdminID: adminID,
		Reason:          strings.TrimSpace(req.Reason),
	})
	if err != nil {
		h.respondProductError(c, err)
		return
	}
	response.Success(c, product)
}

func (h *AdminHandler) respondProductError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pointsmallcontract.ErrProductNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.points_product_not_found", nil)
	case errors.Is(err, pointsmallcontract.ErrInvalidPointsPrice),
		errors.Is(err, pointsmallcontract.ErrInvalidStock),
		errors.Is(err, pointsmallcontract.ErrInvalidPerUserLimit),
		errors.Is(err, pointsmallcontract.ErrInvalidName),
		errors.Is(err, pointsmallcontract.ErrInvalidFulfillmentType),
		errors.Is(err, pointsmallcontract.ErrProductNameRequired),
		errors.Is(err, pointsmallcontract.ErrExchangeInvalidReason):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_product_invalid", err)
	case errors.Is(err, pointsmallcontract.ErrAdminRequired):
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.points_product_update_failed", err)
	}
}

// ListExchangeOrders 后台兑换订单列表（?status= / ?user_id= 过滤，均为白名单参数）。
func (h *AdminHandler) ListExchangeOrders(c *gin.Context) {
	if _, ok := ginutil.GetAdminID(c); !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	if !pointsmalldomain.IsValidExchangeStatus(status) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	userID, err := ginutil.ParseQueryUint(strings.TrimSpace(c.Query("user_id")), true)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", err)
		return
	}
	orders, total, err := h.service.AdminListExchangeOrders(pointsmallcontract.ExchangeOrderListFilter{
		UserID:   userID,
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.points_exchange_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, orders, response.BuildPagination(page, pageSize, total))
}

// GetExchangeOrder 后台兑换订单详情。
func (h *AdminHandler) GetExchangeOrder(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, err := h.service.AdminGetExchangeOrder(id)
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

// ExchangeActionRequest 后台兑换操作请求体。
// Process/Complete 无需 reason；Fail/Cancel 必须 reason（影响积分返还与库存恢复）。
type ExchangeActionRequest struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// exchangeActionInput 组装后台操作输入（AdminID 来自 JWT 上下文）。
// Process/Complete 常不带 body（空 body EOF 属正常）；Fail/Cancel 的 reason 在 service 层校验。
func exchangeActionInput(c *gin.Context, orderID uint) (pointsmallcontract.AdminExchangeActionInput, bool) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return pointsmallcontract.AdminExchangeActionInput{}, false
	}
	var req ExchangeActionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		ginutil.RespondBindError(c, err)
		return pointsmallcontract.AdminExchangeActionInput{}, false
	}
	return pointsmallcontract.AdminExchangeActionInput{
		AdminID: adminID,
		OrderID: orderID,
		Reason:  req.Reason,
		Note:    req.Note,
	}, true
}

// respondExchangeActionError 统一后台兑换操作错误映射。
func respondExchangeActionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pointsmallcontract.ErrExchangeOrderNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.points_exchange_not_found", nil)
	case errors.Is(err, pointsmallcontract.ErrExchangeInvalidState):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_exchange_invalid_state", nil)
	case errors.Is(err, pointsmallcontract.ErrExchangeReasonRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_exchange_reason_required", nil)
	case errors.Is(err, pointsmallcontract.ErrExchangeInvalidReason):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_reason_too_long", nil)
	case errors.Is(err, pointsmallcontract.ErrAdminRequired):
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.points_exchange_action_failed", err)
	}
}

// ProcessOrder 后台开始处理：PENDING → PROCESSING。
func (h *AdminHandler) ProcessOrder(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	input, ok := exchangeActionInput(c, id)
	if !ok {
		return
	}
	order, err := h.service.AdminProcessOrder(input)
	if err != nil {
		respondExchangeActionError(c, err)
		return
	}
	response.Success(c, order)
}

// CompleteOrder 后台完成履约：PROCESSING → COMPLETED。
func (h *AdminHandler) CompleteOrder(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	input, ok := exchangeActionInput(c, id)
	if !ok {
		return
	}
	order, err := h.service.AdminCompleteOrder(input)
	if err != nil {
		respondExchangeActionError(c, err)
		return
	}
	response.Success(c, order)
}

// FailOrder 后台失败：PENDING/PROCESSING → FAILED（返还积分 + 恢复库存；Reason 必填）。
func (h *AdminHandler) FailOrder(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	input, ok := exchangeActionInput(c, id)
	if !ok {
		return
	}
	order, err := h.service.AdminFailOrder(input)
	if err != nil {
		respondExchangeActionError(c, err)
		return
	}
	response.Success(c, order)
}

// CancelOrder 后台取消：PENDING/PROCESSING → CANCELLED（返还积分 + 恢复库存；Reason 必填）。
func (h *AdminHandler) CancelOrder(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	input, ok := exchangeActionInput(c, id)
	if !ok {
		return
	}
	order, err := h.service.AdminCancelOrder(input)
	if err != nil {
		respondExchangeActionError(c, err)
		return
	}
	response.Success(c, order)
}
