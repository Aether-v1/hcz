// Package http 实现 C2C 用户侧薄层 Handler：只做参数解析、鉴权、调用 service、返回响应。
package http

import (
	"errors"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// Service 是 C2C Handler 所需的最小服务端口。
type Service interface {
	// 支付方式
	ListPaymentMethods(userID uint) ([]c2cdomain.PaymentMethod, error)
	CreatePaymentMethod(input c2ccontract.CreatePaymentMethodInput) (*c2cdomain.PaymentMethod, error)
	UpdatePaymentMethod(input c2ccontract.UpdatePaymentMethodInput) (*c2cdomain.PaymentMethod, error)
	DeletePaymentMethod(userID, id uint) error
	SetPaymentMethodEnabled(userID, id uint, enabled bool) (*c2cdomain.PaymentMethod, error)

	// 挂单
	CreateListing(input c2ccontract.CreateListingInput) (*c2cdomain.Listing, error)
	UpdateListing(input c2ccontract.UpdateListingInput) (*c2cdomain.Listing, error)
	PauseListing(userID, id uint) (*c2cdomain.Listing, error)
	ResumeListing(userID, id uint) (*c2cdomain.Listing, error)
	CloseListing(userID, id uint) (*c2cdomain.Listing, error)
	GetListingDetail(id uint) (*c2cdomain.Listing, error)
	ListMarketListings(filter c2ccontract.ListingMarketFilter) ([]c2cdomain.Listing, int64, error)
	ListMyListings(filter c2ccontract.ListingMyFilter) ([]c2cdomain.Listing, int64, error)

	// 交易
	CreateTrade(input c2ccontract.CreateTradeInput) (*c2cdomain.Trade, error)
	MarkPaid(input c2ccontract.MarkPaidInput) (*c2cdomain.Trade, error)
	Confirm(input c2ccontract.ConfirmTradeInput) (*c2cdomain.Trade, error)
	Cancel(input c2ccontract.CancelTradeInput) (*c2cdomain.Trade, error)
	InitiateDispute(input c2ccontract.InitiateDisputeInput) (*c2cdomain.Dispute, error)
	GetTradeDetail(userID, tradeID uint) (*c2cdomain.Trade, error)
	ListMyTrades(filter c2ccontract.TradeListFilter) ([]c2cdomain.Trade, int64, error)
}

// Handler C2C 用户侧 Handler。
type Handler struct {
	svc Service
}

// NewHandler 创建 C2C Handler。
func NewHandler(svc Service) *Handler {
	if svc == nil {
		panic("c2c handler: service is nil")
	}
	return &Handler{svc: svc}
}

// respondError 把领域错误映射为 HTTP 响应。
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, c2ccontract.ErrListingNotFound), errors.Is(err, c2ccontract.ErrTradeNotFound), errors.Is(err, c2ccontract.ErrPaymentMethodNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.c2c_not_found", nil)
	case errors.Is(err, c2ccontract.ErrSelfTrade):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_self_trade", nil)
	case errors.Is(err, c2ccontract.ErrListingNotActive):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_listing_not_active", nil)
	case errors.Is(err, c2ccontract.ErrAmountOutOfRange), errors.Is(err, c2ccontract.ErrInvalidAmount), errors.Is(err, c2ccontract.ErrInvalidPaymentType):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_invalid_amount", nil)
	case errors.Is(err, c2ccontract.ErrTradeStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_status_invalid", nil)
	case errors.Is(err, c2ccontract.ErrIdempotencyConflict):
		ginutil.RespondError(c, response.CodeBadRequest, "error.idempotency_key_required", nil)
	case errors.Is(err, c2ccontract.ErrC2CDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_disabled", nil)
	case errors.Is(err, c2ccontract.ErrTOTPRequired):
		ginutil.RespondError(c, response.CodeForbidden, "error.c2c_totp_required", nil)
	case errors.Is(err, c2ccontract.ErrNewUserCooldown):
		ginutil.RespondError(c, response.CodeForbidden, "error.c2c_new_user_cooldown", nil)
	case errors.Is(err, c2ccontract.ErrNoPaymentMethod):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_no_payment_method", nil)
	case errors.Is(err, c2ccontract.ErrDailyLimitExceeded):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_daily_limit", nil)
	case errors.Is(err, c2ccontract.ErrInsufficientBalance):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_insufficient_balance", nil)
	case errors.Is(err, c2ccontract.ErrPermissionDenied), errors.Is(err, c2ccontract.ErrUserBanned):
		ginutil.RespondError(c, response.CodeForbidden, "error.c2c_permission_denied", nil)
	case errors.Is(err, c2ccontract.ErrUserInactive):
		ginutil.RespondError(c, response.CodeForbidden, "error.c2c_user_inactive", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
	}
}
