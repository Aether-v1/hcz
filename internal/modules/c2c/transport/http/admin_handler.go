package http

import (
	"errors"
	"strings"

	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	c2cpresenter "github.com/Aether-v1/hcz/internal/modules/c2c/transport/presenter"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminService 是 C2C 后台管理所需的最小服务端口。
type AdminService interface {
	Overview() (c2ccontract.OverviewStats, error)

	ListAdminListings(filter c2ccontract.AdminListingFilter) ([]c2cdomain.Listing, int64, error)
	GetListingByIDAdmin(id uint) (*c2cdomain.Listing, error)
	AdminCloseListing(adminID, id uint) (*c2cdomain.Listing, error)

	ListAdminTrades(filter c2ccontract.AdminTradeFilter) ([]c2cdomain.Trade, int64, error)
	GetTradeByIDAdmin(id uint) (*c2cdomain.Trade, error)

	ListDisputes(filter c2ccontract.DisputeListFilter) ([]c2cdomain.Dispute, int64, error)
	GetDisputeByID(id uint) (*c2cdomain.Dispute, error)
	Arbitrate(input c2ccontract.ArbitrateInput) (*c2cdomain.Trade, *c2cdomain.Dispute, error)

	ListRiskSignals(filter c2ccontract.RiskSignalListFilter) ([]c2cdomain.RiskSignal, int64, error)

	AdminDisableUserC2C(adminID, userID uint, reason string) (*c2ccontract.UserC2CStatusView, error)
	AdminEnableUserC2C(adminID, userID uint) (*c2ccontract.UserC2CStatusView, error)
	GetUserC2CStatus(userID uint) (*c2ccontract.UserC2CStatusView, error)
}

// ChallengeVerifier 校验管理员 Step-Up 2FA challenge token。
type ChallengeVerifier interface {
	ParseChallengeToken(token string) (adminID uint, jti string, err error)
}

// SettingsProvider C2C 设置读写。
type SettingsProvider interface {
	GetC2CConfig() (settingsintegration.C2CSetting, error)
	UpdateC2CConfig(setting settingsintegration.C2CSetting) (settingsintegration.C2CSetting, error)
}

// AdminHandler C2C 后台管理 Handler（薄层）。
type AdminHandler struct {
	svc       AdminService
	challenge ChallengeVerifier
	settings  SettingsProvider
}

// NewAdminHandler 创建 C2C 后台 Handler。challenge/settings 可为 nil（对应端点禁用）。
func NewAdminHandler(svc AdminService, challenge ChallengeVerifier, settings SettingsProvider) *AdminHandler {
	if svc == nil {
		panic("c2c admin handler: service is nil")
	}
	return &AdminHandler{svc: svc, challenge: challenge, settings: settings}
}

func respondAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, c2ccontract.ErrListingNotFound), errors.Is(err, c2ccontract.ErrTradeNotFound), errors.Is(err, c2ccontract.ErrDisputeNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.c2c_not_found", nil)
	case errors.Is(err, c2ccontract.ErrTradeStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_status_invalid", nil)
	case errors.Is(err, c2ccontract.ErrDisputeAlreadyOpen), errors.Is(err, c2ccontract.ErrDisputeAlreadyResolved):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_status_invalid", nil)
	case errors.Is(err, c2ccontract.ErrInvalidArbitrationResult):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_invalid_arbitration_result", nil)
	case errors.Is(err, c2ccontract.ErrArbitrationReasonRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_reason_required", nil)
	case errors.Is(err, c2ccontract.ErrChallengeRequired), errors.Is(err, c2ccontract.ErrChallengeInvalid):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.c2c_challenge_invalid", nil)
	case errors.Is(err, c2ccontract.ErrPermissionDenied), errors.Is(err, c2ccontract.ErrUserBanned):
		ginutil.RespondError(c, response.CodeForbidden, "error.c2c_permission_denied", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
	}
}

// Overview GET /overview
func (h *AdminHandler) Overview(c *gin.Context) {
	stats, err := h.svc.Overview()
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{
		"active_listings":   stats.ActiveListings,
		"active_trades":     stats.ActiveTrades,
		"open_disputes":     stats.OpenDisputes,
		"today_volume_usdt": stats.TodayVolumeUSDT.String(),
	})
}

// ListListings GET /listings
func (h *AdminHandler) ListListings(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	sellerID, _ := ginutil.ParseQueryUint(c.Query("seller_user_id"), false)
	rows, total, err := h.svc.ListAdminListings(c2ccontract.AdminListingFilter{
		Page:         page,
		PageSize:     pageSize,
		SellerUserID: sellerID,
		Status:       strings.TrimSpace(c.Query("status")),
		FiatCurrency: strings.TrimSpace(c.Query("fiat_currency")),
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.SuccessWithPage(c, c2cpresenter.NewListingRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetListing GET /listings/:id
func (h *AdminHandler) GetListing(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.GetListingByIDAdmin(id)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

// CloseListing PUT /listings/:id/close
func (h *AdminHandler) CloseListing(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	l, err := h.svc.AdminCloseListing(adminID, id)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewListingResp(l))
}

// ListTrades GET /trades
func (h *AdminHandler) ListTrades(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	buyerID, _ := ginutil.ParseQueryUint(c.Query("buyer_user_id"), false)
	sellerID, _ := ginutil.ParseQueryUint(c.Query("seller_user_id"), false)
	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	rows, total, err := h.svc.ListAdminTrades(c2ccontract.AdminTradeFilter{
		Page:         page,
		PageSize:     pageSize,
		Status:       strings.TrimSpace(c.Query("status")),
		BuyerUserID:  buyerID,
		SellerUserID: sellerID,
		CreatedFrom:  createdFrom,
		CreatedTo:    createdTo,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.SuccessWithPage(c, c2cpresenter.NewTradeRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetTrade GET /trades/:id
func (h *AdminHandler) GetTrade(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	t, err := h.svc.GetTradeByIDAdmin(id)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewTradeResp(t))
}

// ListDisputes GET /disputes
func (h *AdminHandler) ListDisputes(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.svc.ListDisputes(c2ccontract.DisputeListFilter{
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.SuccessWithPage(c, c2cpresenter.NewDisputeRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetDispute GET /disputes/:id
func (h *AdminHandler) GetDispute(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	d, err := h.svc.GetDisputeByID(id)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, c2cpresenter.NewDisputeResp(d))
}

type arbitrateRequest struct {
	TradeID   uint   `json:"trade_id" binding:"required"`
	Result    string `json:"result" binding:"required"`
	Reason    string `json:"reason" binding:"required"`
	AdminNote string `json:"admin_note"`
}

// Arbitrate POST /arbitration（挂在 paymentProtected 组；Step-Up + Idempotency-Key + Reason）。
func (h *AdminHandler) Arbitrate(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.idempotency_key_required", nil)
		return
	}

	var req arbitrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_reason_required", nil)
		return
	}

	// Step-Up：从 header（X-Auth-Challenge）或 body（challenge_token）取 challenge token。
	challengeToken := strings.TrimSpace(c.GetHeader("X-Auth-Challenge"))
	if challengeToken == "" {
		challengeToken = strings.TrimSpace(c.Request.Header.Get("X-Auth-Challenge-Token"))
	}
	if challengeToken == "" {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.c2c_challenge_required", nil)
		return
	}
	if h.challenge == nil {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.c2c_challenge_invalid", nil)
		return
	}
	claimsAdminID, _, err := h.challenge.ParseChallengeToken(challengeToken)
	if err != nil || claimsAdminID != adminID {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.c2c_challenge_invalid", nil)
		return
	}

	t, d, err := h.svc.Arbitrate(c2ccontract.ArbitrateInput{
		AdminID:        adminID,
		TradeID:        req.TradeID,
		Result:         strings.TrimSpace(req.Result),
		Reason:         strings.TrimSpace(req.Reason),
		AdminNote:      strings.TrimSpace(req.AdminNote),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, gin.H{
		"trade":   c2cpresenter.NewTradeResp(t),
		"dispute": c2cpresenter.NewDisputeResp(d),
	})
}

type disableUserC2CRequest struct {
	Reason string `json:"reason"`
}

// DisableUserC2C POST /users/:id/disable
func (h *AdminHandler) DisableUserC2C(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req disableUserC2CRequest
	_ = c.ShouldBindJSON(&req)
	st, err := h.svc.AdminDisableUserC2C(adminID, userID, strings.TrimSpace(req.Reason))
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, st)
}

// EnableUserC2C POST /users/:id/enable
func (h *AdminHandler) EnableUserC2C(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	st, err := h.svc.AdminEnableUserC2C(adminID, userID)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, st)
}

// GetUserC2CStatus GET /users/:id
func (h *AdminHandler) GetUserC2CStatus(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	st, err := h.svc.GetUserC2CStatus(userID)
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.Success(c, st)
}

// ListRiskSignals GET /risk-signals
func (h *AdminHandler) ListRiskSignals(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	userID, _ := ginutil.ParseQueryUint(c.Query("user_id"), false)
	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	rows, total, err := h.svc.ListRiskSignals(c2ccontract.RiskSignalListFilter{
		Page:        page,
		PageSize:    pageSize,
		UserID:      userID,
		SignalType:  strings.TrimSpace(c.Query("signal_type")),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	})
	if err != nil {
		respondAdminError(c, err)
		return
	}
	response.SuccessWithPage(c, c2cpresenter.NewRiskSignalRespList(rows), response.BuildPagination(page, pageSize, total))
}

// GetSettings GET /settings
func (h *AdminHandler) GetSettings(c *gin.Context) {
	if h.settings == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", nil)
		return
	}
	cfg, err := h.settings.GetC2CConfig()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, cfg)
}

// UpdateSettings PUT /settings
func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	if h.settings == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", nil)
		return
	}
	var cfg settingsintegration.C2CSetting
	if err := c.ShouldBindJSON(&cfg); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	updated, err := h.settings.UpdateC2CConfig(cfg)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.c2c_config_invalid", err)
		return
	}
	response.Success(c, updated)
}
