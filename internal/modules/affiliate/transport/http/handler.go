package affiliatehttp

import (
	"errors"
	"net/http"
	"strings"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"

	ginutil "github.com/Aether-v1/hcz/internal/platform/http/ginutil"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	affiliatepresenter "github.com/Aether-v1/hcz/internal/modules/affiliate/transport/presenter"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// Service 是前台推广返利端口。
type Service interface {
	TrackClick(input affiliateapp.TrackClickInput) error
	OpenAffiliate(userID uint) (*affiliatedomain.Profile, error)
	// ApplyAffiliate 用户提交推广申请（替代旧 OpenAffiliate）。
	ApplyAffiliate(userID uint, reason string) (*affiliatedomain.Application, error)
	// GetUserApplication 返回当前用户最新的申请记录。
	GetUserApplication(userID uint) (*affiliatedomain.Application, error)
	// GetUserProfileForGateway 返回当前用户的 profile（供 /affiliate/profile 端点）。
	GetUserProfileForGateway(userID uint) (*affiliatedomain.Profile, error)
	GetUserDashboard(userID uint) (affiliateapp.Dashboard, error)
	ListUserCommissions(userID uint, page, pageSize int, status string) ([]affiliatedomain.Commission, int64, error)
	ListUserWithdraws(userID uint, page, pageSize int, status string) ([]affiliatedomain.WithdrawRequest, int64, error)
	// TransferToWallet 将佣金划转至主钱包（替代旧独立提现）。
	TransferToWallet(userID uint, input affiliateapp.TransferToWalletInput) (*affiliatedomain.CommissionLedger, *walletdomain.Transaction, error)
	// ListTransferHistory 查询划转历史。
	ListTransferHistory(userID uint, page, pageSize int) ([]affiliatedomain.CommissionLedger, int64, error)
}

type trackClickRequest struct {
	AffiliateCode string `json:"affiliate_code" binding:"required"`
	VisitorKey    string `json:"visitor_key"`
	LandingPath   string `json:"landing_path"`
	Referrer      string `json:"referrer"`
}

type withdrawApplyRequest struct {
	Amount  string `json:"amount" binding:"required"`
	Channel string `json:"channel" binding:"required"`
	Account string `json:"account" binding:"required"`
}

type transferToWalletRequest struct {
	Amount string `json:"amount"`
	All    bool   `json:"all"`
}

// Handler 处理前台推广返利请求。
type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	if svc == nil {
		panic("affiliate handler: service is nil")
	}
	return &Handler{svc: svc}
}

// TrackAffiliateClick 记录推广点击
func (h *Handler) TrackAffiliateClick(c *gin.Context) {
	var req trackClickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.svc.TrackClick(affiliateapp.TrackClickInput{
		AffiliateCode: req.AffiliateCode,
		VisitorKey:    req.VisitorKey,
		LandingPath:   req.LandingPath,
		Referrer:      req.Referrer,
		ClientIP:      c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	}); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// applyAffiliateRequest 推广申请请求体。
type applyAffiliateRequest struct {
	Reason string `json:"reason"`
}

// OpenAffiliate 开通推广返利（已退休，返回 410 Gone）。
// 旧直接开通入口已关闭，请使用 POST /affiliate/apply 提交申请。
func (h *Handler) OpenAffiliate(c *gin.Context) {
	response.ErrorWithHTTPStatus(c, http.StatusGone, http.StatusGone,
		"affiliate open retired, use POST /affiliate/apply instead")
}

// ApplyAffiliate POST /affiliate/apply 用户提交推广申请。
func (h *Handler) ApplyAffiliate(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req applyAffiliateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// reason 可选，允许空 body
		req.Reason = ""
	}

	app, err := h.svc.ApplyAffiliate(uid, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.forbidden", nil)
		case errors.Is(err, affiliateapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		case errors.Is(err, affiliateapp.ErrUserDisabled):
			ginutil.RespondError(c, response.CodeForbidden, "error.user_disabled", nil)
		case errors.Is(err, affiliateapp.ErrAlreadyActive):
			ginutil.RespondError(c, response.CodeBadRequest, "error.affiliate_already_active", nil)
		case errors.Is(err, affiliateapp.ErrApplicationPending):
			ginutil.RespondError(c, response.CodeBadRequest, "error.affiliate_application_pending", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, affiliatepresenter.NewApplication(app))
}

// GetAffiliateApplication GET /affiliate/application 返回当前用户最新申请。
func (h *Handler) GetAffiliateApplication(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	app, err := h.svc.GetUserApplication(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if app == nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, affiliatepresenter.NewApplication(app))
}

// GetAffiliateProfile GET /affiliate/profile 返回当前用户 profile（或 not found）。
func (h *Handler) GetAffiliateProfile(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	profile, err := h.svc.GetUserProfileForGateway(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if profile == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.affiliate_not_opened", nil)
		return
	}
	response.Success(c, affiliatepresenter.NewProfile(profile))
}

// GetAffiliateDashboard 获取推广返利看板
func (h *Handler) GetAffiliateDashboard(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	data, err := h.svc.GetUserDashboard(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, data)
}

// ListAffiliateCommissions 查询我的推广佣金记录
func (h *Handler) ListAffiliateCommissions(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))

	rows, total, err := h.svc.ListUserCommissions(uid, page, pageSize, status)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, affiliatepresenter.NewCommissionList(rows), response.BuildPagination(page, pageSize, total))
}

// ListAffiliateWithdraws 查询我的提现申请记录
func (h *Handler) ListAffiliateWithdraws(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))

	rows, total, err := h.svc.ListUserWithdraws(uid, page, pageSize, status)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, affiliatepresenter.NewWithdrawList(rows), response.BuildPagination(page, pageSize, total))
}

// ApplyAffiliateWithdraw 提交提现申请（已退休，返回 410 Gone）。
// 旧独立提现入口已关闭，请使用 POST /affiliate/transfer-to-wallet 将佣金划转至主钱包。
func (h *Handler) ApplyAffiliateWithdraw(c *gin.Context) {
	response.ErrorWithHTTPStatus(c, http.StatusGone, http.StatusGone,
		"affiliate withdrawal retired, use transfer-to-wallet instead")
}

// TransferToWallet 佣金划转至主钱包。
// 用户身份从 authenticated context 取，禁止请求体传 user_id。
func (h *Handler) TransferToWallet(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req transferToWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	input := affiliateapp.TransferToWalletInput{All: req.All}
	if !req.All {
		amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		input.Amount = amount
	}

	ledger, walletTxn, err := h.svc.TransferToWallet(uid, input)
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrNotOpened), errors.Is(err, affiliateapp.ErrNotActive):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrTransferAmountInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrTransferInsufficient):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrTransferInDebt):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"ledger":      affiliatepresenter.NewTransferRecord(ledger),
		"wallet_txn_id": walletTxn.ID,
	})
}

// ListAffiliateTransfers 查询我的划转历史
func (h *Handler) ListAffiliateTransfers(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)

	rows, total, err := h.svc.ListTransferHistory(uid, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, affiliatepresenter.NewTransferRecordList(rows), response.BuildPagination(page, pageSize, total))
}
