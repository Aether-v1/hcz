package http

import (
	"strings"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cpresenter "github.com/Aether-v1/hcz/internal/modules/c2c/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// parseAmount 解析字符串金额为 money.Amount；空串返回零值。
func parseAmount(raw string) (money.Amount, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return money.Amount{}, true
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return money.Amount{}, false
	}
	return money.FromDecimal(d), true
}

// ListMarketListings 市场挂单列表。
func (h *Handler) ListMarketListings(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.svc.ListMarketListings(c2ccontract.ListingMarketFilter{
		Page:          page,
		PageSize:      pageSize,
		FiatCurrency:  strings.TrimSpace(c.Query("fiat_currency")),
		ExcludeUserID: uid,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.SuccessWithPage(c, c2cpresenter.NewListingRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetListingDetail 挂单详情。
func (h *Handler) GetListingDetail(c *gin.Context) {
	if _, ok := ginutil.GetUserID(c); !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.GetListingDetail(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

// ListListingPaymentMethods 查看挂单卖家的可用收款方式（脱敏，供买家选择）。
// 数据来自卖家自身，不依赖前端传入 user_id；挂在 user 组，买家已登录。
func (h *Handler) ListListingPaymentMethods(c *gin.Context) {
	if _, ok := ginutil.GetUserID(c); !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	rows, err := h.svc.ListEnabledPaymentMethodsByListingID(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewPaymentMethodListResult(rows))
}

type createListingRequest struct {
	FiatCurrency  string `json:"fiat_currency"`
	Price         string `json:"price" binding:"required"`
	MinFiatAmount string `json:"min_fiat_amount" binding:"required"`
	MaxFiatAmount string `json:"max_fiat_amount" binding:"required"`
	TotalUSDT     string `json:"total_usdt" binding:"required"`
	Terms         string `json:"terms"`
}

// CreateListing 发布挂单。
func (h *Handler) CreateListing(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req createListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	price, ok1 := parseAmount(req.Price)
	minFiat, ok2 := parseAmount(req.MinFiatAmount)
	maxFiat, ok3 := parseAmount(req.MaxFiatAmount)
	total, ok4 := parseAmount(req.TotalUSDT)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.CreateListing(c2ccontract.CreateListingInput{
		UserID:        uid,
		FiatCurrency:  strings.TrimSpace(req.FiatCurrency),
		Price:         price,
		MinFiatAmount: minFiat,
		MaxFiatAmount: maxFiat,
		TotalUSDT:     total,
		Terms:         strings.TrimSpace(req.Terms),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

type updateListingRequest struct {
	Price         string `json:"price"`
	MinFiatAmount string `json:"min_fiat_amount"`
	MaxFiatAmount string `json:"max_fiat_amount"`
	Terms         string `json:"terms"`
}

// UpdateListing 修改挂单。
func (h *Handler) UpdateListing(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req updateListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	price, ok1 := parseAmount(req.Price)
	minFiat, ok2 := parseAmount(req.MinFiatAmount)
	maxFiat, ok3 := parseAmount(req.MaxFiatAmount)
	if !ok1 || !ok2 || !ok3 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.UpdateListing(c2ccontract.UpdateListingInput{
		ID:            id,
		UserID:        uid,
		Price:         price,
		MinFiatAmount: minFiat,
		MaxFiatAmount: maxFiat,
		Terms:         strings.TrimSpace(req.Terms),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

// PauseListing 暂停挂单。
func (h *Handler) PauseListing(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.PauseListing(uid, id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

// ResumeListing 恢复挂单。
func (h *Handler) ResumeListing(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.ResumeListing(uid, id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

// CloseListing 关闭挂单。
func (h *Handler) CloseListing(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.CloseListing(uid, id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

// ListMyListings 我的挂单。
func (h *Handler) ListMyListings(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.svc.ListMyListings(c2ccontract.ListingMyFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   uid,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	response.SuccessWithPage(c, c2cpresenter.NewListingRespList(rows), response.BuildPagination(page, pageSize, total))
}
