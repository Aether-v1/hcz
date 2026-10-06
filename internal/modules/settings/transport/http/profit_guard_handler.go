package settingshttp

import (
	"errors"

	ginutil "github.com/Aether-v1/hcz/internal/platform/http/ginutil"

	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ProfitGuardAdminService 是后台 Profit Guard 设置端口。
type ProfitGuardAdminService interface {
	GetProfitGuardSetting() (settingsintegration.ProfitGuardSetting, error)
	UpdateProfitGuardSetting(setting settingsintegration.ProfitGuardSetting) (settingsintegration.ProfitGuardSetting, error)
}

// ProfitGuardHandler 处理后台 Profit Guard 设置请求（/admin/settings/profit-guard）。
type ProfitGuardHandler struct {
	settings ProfitGuardAdminService
}

func NewProfitGuardHandler(settings ProfitGuardAdminService) *ProfitGuardHandler {
	if settings == nil {
		panic("settings profit guard handler: settings is nil")
	}
	return &ProfitGuardHandler{settings: settings}
}

// GetProfitGuard 获取 Profit Guard 设置。
func (h *ProfitGuardHandler) GetProfitGuard(c *gin.Context) {
	setting, err := h.settings.GetProfitGuardSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, setting)
}

// UpdateProfitGuard 更新 Profit Guard 设置。
func (h *ProfitGuardHandler) UpdateProfitGuard(c *gin.Context) {
	var req settingsintegration.ProfitGuardSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.settings.UpdateProfitGuardSetting(req)
	if err != nil {
		if errors.Is(err, settingsintegration.ErrProfitGuardConfigInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}
	response.Success(c, setting)
}
