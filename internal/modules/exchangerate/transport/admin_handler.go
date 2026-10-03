package transport

import (
	"context"
	"strings"
	"time"

	exchangerateapp "github.com/Aether-v1/hcz/internal/modules/exchangerate/application"
	ginutil "github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AdminHandler 是 Global Exchange Rate 的管理后台 HTTP 入口。
// 复用现有 admin JWT/RBAC（挂在 /admin/settings/exchange-rate 下），不新增旁路认证。
type AdminHandler struct {
	svc *exchangerateapp.Service
}

func NewAdminHandler(svc *exchangerateapp.Service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// GET /admin/settings/exchange-rate
func (h *AdminHandler) Get(c *gin.Context) {
	if h == nil || h.svc == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.exchangerate_not_configured", nil)
		return
	}
	state, effective, err := h.svc.Snapshot()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.exchangerate_fetch_failed", err)
		return
	}
	resp := gin.H{
		"site_currency":        state.Currency,
		"provider":             state.Provider,
		"auto_enabled":         state.AutoEnabled,
		"refresh_interval":      state.RefreshIntervalMin,
		"auto_rate":             state.AutoRate.String(),
		"manual_fallback_rate":  state.ManualRate.String(),
		"fetched_at":           fmtTime(state.AutoFetchedAt),
		"last_success_at":       fmtTime(state.LastSuccessAt),
		"last_error":            state.LastError,
		"api_key_masked":        maskKey(state.APIKey),
		"status":               "unknown",
		"effective_rate":       "",
		"effective_source":     "",
	}
	if effective.Rate.GreaterThan(decimal.Zero) {
		resp["effective_rate"] = effective.Rate.String()
		resp["effective_source"] = effective.Source
		resp["status"] = "healthy"
	} else if state.ManualRate.GreaterThan(decimal.Zero) {
		resp["effective_source"] = "MANUAL"
		resp["status"] = "manual_fallback"
	} else {
		resp["status"] = "unavailable"
	}
	response.Success(c, resp)
}

type updateReq struct {
	APIKey             string `json:"api_key"`
	AutoEnabled        *bool  `json:"auto_enabled"`
	RefreshIntervalMin int    `json:"refresh_interval_min"`
	ManualFallbackRate string `json:"manual_fallback_rate"`
}

// PUT /admin/settings/exchange-rate
func (h *AdminHandler) Update(c *gin.Context) {
	if h == nil || h.svc == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.exchangerate_not_configured", nil)
		return
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		return
	}
	autoEnabled := false
	if req.AutoEnabled != nil {
		autoEnabled = *req.AutoEnabled
	}
	if err := h.svc.UpdateConfig(req.APIKey, autoEnabled, req.RefreshIntervalMin); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.exchangerate_update_failed", err)
		return
	}
	if strings.TrimSpace(req.ManualFallbackRate) != "" {
		rate, err := decimal.NewFromString(strings.TrimSpace(req.ManualFallbackRate))
		if err != nil {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, "bad_manual_rate", nil)
			return
		}
		if err := h.svc.SetManual(rate); err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.exchangerate_update_failed", err)
			return
		}
	}
	response.Success(c, gin.H{"updated": true})
}

// POST /admin/settings/exchange-rate/refresh
func (h *AdminHandler) Refresh(c *gin.Context) {
	if h == nil || h.svc == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.exchangerate_not_configured", nil)
		return
	}
	if err := h.svc.Refresh(context.Background()); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.exchangerate_refresh_failed", err)
		return
	}
	response.Success(c, gin.H{"refreshed": true})
}

func maskKey(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return ""
	}
	if len(k) <= 4 {
		return "••••"
	}
	return "••••••" + k[len(k)-4:]
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
