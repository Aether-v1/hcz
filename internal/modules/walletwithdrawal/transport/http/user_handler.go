package withdrawalhttp

import (
	"errors"
	"strings"

	withdrawalcontract "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	withdrawalpresenter "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	"github.com/Aether-v1/hcz/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// UserService 是用户提现所需的最小端口。
type UserService interface {
	CreateWithdrawal(input withdrawalcontract.CreateWithdrawalInput) (*withdrawaldomain.Withdrawal, error)
	QuoteFee(network string, amount money.Amount) (withdrawalcontract.FeeQuote, error)
	ListUserWithdrawals(userID uint, page, pageSize int, status string) ([]withdrawaldomain.Withdrawal, int64, error)
	GetUserWithdrawalDetail(userID, id uint) (*withdrawaldomain.Withdrawal, error)
	CancelWithdrawal(input withdrawalcontract.CancelWithdrawalInput) (*withdrawaldomain.Withdrawal, error)
	ListAddresses(userID uint) ([]withdrawaldomain.Address, error)
	CreateAddress(input withdrawalcontract.CreateAddressInput) (*withdrawaldomain.Address, error)
	DeleteAddress(userID, id uint) error
	SetDefaultAddress(userID, id uint) (*withdrawaldomain.Address, error)
}

// UserHandler 处理用户提现 HTTP 请求。
type UserHandler struct {
	svc UserService
}

// NewUserHandler 创建用户提现 Handler。
func NewUserHandler(svc UserService) *UserHandler {
	if svc == nil {
		panic("withdrawal user handler: service is nil")
	}
	return &UserHandler{svc: svc}
}

type createWithdrawalRequest struct {
	Network  string `json:"network"`
	Address  string `json:"address" binding:"required"`
	Amount   string `json:"amount" binding:"required"`
	TOTPCode string `json:"totp_code" binding:"required"`
	UserNote string `json:"user_note"`
}

// Create 创建提现。
func (h *UserHandler) Create(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req createWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.idempotency_key_required", nil)
		return
	}
	w, err := h.svc.CreateWithdrawal(withdrawalcontract.CreateWithdrawalInput{
		UserID:         uid,
		Network:        strings.TrimSpace(req.Network),
		Address:        strings.TrimSpace(req.Address),
		Amount:         money.FromDecimal(amount),
		TOTPCode:       strings.TrimSpace(req.TOTPCode),
		IdempotencyKey: idempotencyKey,
		UserNote:       strings.TrimSpace(req.UserNote),
	})
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

type quoteRequest struct {
	Network string `json:"network"`
	Amount  string `json:"amount" binding:"required"`
}

// Quote 手续费预览。
func (h *UserHandler) Quote(c *gin.Context) {
	if _, ok := ginutil.GetUserID(c); !ok {
		return
	}
	var req quoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	quote, err := h.svc.QuoteFee(strings.TrimSpace(req.Network), money.FromDecimal(amount))
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, quote)
}

// List 用户提现单列表。
func (h *UserHandler) List(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.svc.ListUserWithdrawals(uid, page, pageSize, strings.TrimSpace(c.Query("status")))
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.SuccessWithPage(c, withdrawalpresenter.NewWithdrawalRespList(rows), response.BuildPagination(page, pageSize, total))
}

// Detail 用户提现单详情。
func (h *UserHandler) Detail(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	w, err := h.svc.GetUserWithdrawalDetail(uid, id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

type cancelRequest struct {
	TOTPCode string `json:"totp_code" binding:"required"`
}

// Cancel 用户取消提现。
func (h *UserHandler) Cancel(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req cancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	w, err := h.svc.CancelWithdrawal(withdrawalcontract.CancelWithdrawalInput{
		UserID:   uid,
		ID:       id,
		TOTPCode: strings.TrimSpace(req.TOTPCode),
	})
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Success(c, withdrawalpresenter.NewWithdrawalResp(w))
}

func respondUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, withdrawalcontract.ErrWithdrawalDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_disabled", nil)
	case errors.Is(err, withdrawalcontract.ErrWithdrawalNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.withdrawal_not_found", nil)
	case errors.Is(err, withdrawalcontract.ErrInvalidAmount), errors.Is(err, withdrawalcontract.ErrAmountTooSmall), errors.Is(err, withdrawalcontract.ErrAmountTooLarge):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_invalid_amount", nil)
	case errors.Is(err, withdrawalcontract.ErrInsufficientBalance):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_insufficient_balance", nil)
	case errors.Is(err, withdrawalcontract.ErrUnsupportedNetwork):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_unsupported_network", nil)
	case errors.Is(err, withdrawalcontract.ErrInvalidAddress), errors.Is(err, withdrawalcontract.ErrAddressBlacklisted):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_invalid_address", nil)
	case errors.Is(err, withdrawalcontract.ErrTOTPNotEnabled):
		ginutil.RespondError(c, response.CodeForbidden, "error.withdrawal_totp_not_enabled", nil)
	case errors.Is(err, withdrawalcontract.ErrTOTPRequired), errors.Is(err, withdrawalcontract.ErrTOTPInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_totp_invalid", nil)
	case errors.Is(err, withdrawalcontract.ErrIdempotencyKeyRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.idempotency_key_required", nil)
	case errors.Is(err, withdrawalcontract.ErrDailyLimitExceeded), errors.Is(err, withdrawalcontract.ErrDailyCountExceeded), errors.Is(err, withdrawalcontract.ErrFirstWithdrawalLimit):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_limit_exceeded", nil)
	case errors.Is(err, withdrawalcontract.ErrWithdrawalStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_status_invalid", nil)
	case errors.Is(err, withdrawalcontract.ErrAddressNotFound), errors.Is(err, withdrawalcontract.ErrAddressDuplicate):
		ginutil.RespondError(c, response.CodeBadRequest, "error.withdrawal_address_invalid", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
	}
}
