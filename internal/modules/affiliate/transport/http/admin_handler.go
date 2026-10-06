package affiliatehttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	affiliateapp "github.com/Aether-v1/hcz/internal/modules/affiliate/application"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台推广返利管理端口。
type AdminService interface {
	ListAdminUsers(filter affiliateapp.AdminProfileListFilter) ([]affiliateapp.AdminUserItem, int64, error)
	ListAdminCommissions(filter affiliateapp.AdminCommissionListFilter) ([]affiliatedomain.Commission, int64, error)
	ListAdminWithdraws(filter affiliateapp.AdminWithdrawListFilter) ([]affiliatedomain.WithdrawRequest, int64, error)
	UpdateAffiliateProfileStatus(profileID uint, status string) (*affiliatedomain.Profile, error)
	BatchUpdateAffiliateProfileStatus(profileIDs []uint, status string) (int64, error)
	// 申请审核相关
	ListAdminApplications(filter affiliateapp.AffiliateApplicationListFilter) ([]affiliatedomain.Application, int64, error)
	GetApplicationDetail(applicationID uint) (*affiliatedomain.Application, error)
	ApproveApplication(applicationID uint, adminID uint) (*affiliatedomain.Profile, error)
	RejectApplication(applicationID uint, adminID uint, reason string) error
}

type profileStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type batchProfileStatusRequest struct {
	ProfileIDs []uint `json:"profile_ids" binding:"required"`
	Status     string `json:"status" binding:"required"`
}

type reviewWithdrawRequest struct {
	Reason string `json:"reason"`
}

// AdminHandler 处理后台推广返利管理请求。
type AdminHandler struct {
	svc AdminService
}

func NewAdminHandler(svc AdminService) *AdminHandler {
	if svc == nil {
		panic("affiliate admin handler: service is nil")
	}
	return &AdminHandler{svc: svc}
}

// ListAffiliateUsers 管理端推广用户列表
func (h *AdminHandler) ListAffiliateUsers(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	userID, _ := ginutil.ParseQueryUint(c.Query("user_id"), false)

	rows, total, err := h.svc.ListAdminUsers(affiliateapp.AdminProfileListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
		Status:   strings.TrimSpace(c.Query("status")),
		Code:     strings.TrimSpace(c.Query("code")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

// ListAffiliateCommissions 管理端佣金列表
func (h *AdminHandler) ListAffiliateCommissions(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	profileID, _ := ginutil.ParseQueryUint(c.Query("affiliate_profile_id"), false)
	level, _ := strconv.Atoi(strings.TrimSpace(c.Query("level")))

	rows, total, err := h.svc.ListAdminCommissions(affiliateapp.AdminCommissionListFilter{
		Page:               page,
		PageSize:           pageSize,
		AffiliateProfileID: profileID,
		OrderNo:            strings.TrimSpace(c.Query("order_no")),
		Status:             strings.TrimSpace(c.Query("status")),
		Keyword:            strings.TrimSpace(c.Query("keyword")),
		Level:              level,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

// ListAffiliateWithdraws 管理端提现审核列表
func (h *AdminHandler) ListAffiliateWithdraws(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	profileID, _ := ginutil.ParseQueryUint(c.Query("affiliate_profile_id"), false)

	rows, total, err := h.svc.ListAdminWithdraws(affiliateapp.AdminWithdrawListFilter{
		Page:               page,
		PageSize:           pageSize,
		AffiliateProfileID: profileID,
		Status:             strings.TrimSpace(c.Query("status")),
		Keyword:            strings.TrimSpace(c.Query("keyword")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

// UpdateAffiliateUserStatus 管理端更新返利用户状态
func (h *AdminHandler) UpdateAffiliateUserStatus(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	var req profileStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	row, err := h.svc.UpdateAffiliateProfileStatus(id, strings.TrimSpace(req.Status))
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrProfileStatusInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, row)
}

// BatchUpdateAffiliateUserStatus 管理端批量更新返利用户状态
func (h *AdminHandler) BatchUpdateAffiliateUserStatus(c *gin.Context) {
	var req batchProfileStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if len(req.ProfileIDs) == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	updated, err := h.svc.BatchUpdateAffiliateProfileStatus(req.ProfileIDs, strings.TrimSpace(req.Status))
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrProfileStatusInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"updated": updated})
}

// RejectAffiliateWithdraw 拒绝提现申请（已退休，返回 410 Gone）。
func (h *AdminHandler) RejectAffiliateWithdraw(c *gin.Context) {
	response.ErrorWithHTTPStatus(c, http.StatusGone, http.StatusGone,
		"affiliate withdrawal retired, use transfer-to-wallet instead")
}

// PayAffiliateWithdraw 标记提现已支付（已退休，返回 410 Gone）。
func (h *AdminHandler) PayAffiliateWithdraw(c *gin.Context) {
	response.ErrorWithHTTPStatus(c, http.StatusGone, http.StatusGone,
		"affiliate withdrawal retired, use transfer-to-wallet instead")
}

// ---- 推广申请审核 ----

type rejectApplicationRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// ListAffiliateApplications GET /admin/affiliates/applications
func (h *AdminHandler) ListAffiliateApplications(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	userID, _ := ginutil.ParseQueryUint(c.Query("user_id"), false)

	rows, total, err := h.svc.ListAdminApplications(affiliateapp.AffiliateApplicationListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
		Status:   strings.TrimSpace(c.Query("status")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

// GetAffiliateApplication GET /admin/affiliates/applications/:id
func (h *AdminHandler) GetAffiliateApplication(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	app, err := h.svc.GetApplicationDetail(id)
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrApplicationNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		}
		return
	}
	response.Success(c, app)
}

// ApproveAffiliateApplication POST /admin/affiliates/applications/:id/approve
func (h *AdminHandler) ApproveAffiliateApplication(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}

	profile, err := h.svc.ApproveApplication(id, adminID)
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrApplicationNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
		case errors.Is(err, affiliateapp.ErrApplicationAlreadyReviewed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.affiliate_application_already_reviewed", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"profile": profile,
	})
}

// RejectAffiliateApplication POST /admin/affiliates/applications/:id/reject
func (h *AdminHandler) RejectAffiliateApplication(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}

	var req rejectApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.svc.RejectApplication(id, adminID, strings.TrimSpace(req.Reason)); err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrApplicationNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
		case errors.Is(err, affiliateapp.ErrApplicationAlreadyReviewed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.affiliate_application_already_reviewed", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"ok": true})
}
