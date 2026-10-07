package settingshttp

import (
	"errors"

	settingspoints "github.com/Aether-v1/hcz/internal/modules/settings/schema/points"
	ginutil "github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// CheckinService 是后台签到配置端口。
type CheckinService interface {
	GetCheckinSetting() (settingspoints.CheckinConfig, error)
	UpdateCheckinSetting(setting settingspoints.CheckinConfig, operatorAdminID uint) (settingspoints.CheckinConfig, error)
}

// CheckinHandler 处理 /admin/settings/checkin。
type CheckinHandler struct {
	settings CheckinService
}

func NewCheckinHandler(settings CheckinService) *CheckinHandler {
	if settings == nil {
		panic("settings checkin handler: settings is nil")
	}
	return &CheckinHandler{settings: settings}
}

// GetCheckin 返回当前签到配置（enabled + rewards）。
func (h *CheckinHandler) GetCheckin(c *gin.Context) {
	cfg, err := h.settings.GetCheckinSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, cfg)
}

// UpdateCheckin 更新签到配置。
//
// 要求：
//   - Admin JWT + RBAC（路由层：/admin/settings/checkin，system_admin）；
//   - rewards 必须恰为 7 项、每项 0 <= x <= 1_000_000，否则拒绝入库；
//   - 允许 0 积分日（仍算签到成功，不产生 0 amount Points Ledger）；
//   - 变更必须携带管理员身份并落审计（操作者 + 变更前后配置）。
func (h *CheckinHandler) UpdateCheckin(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req settingspoints.CheckinConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	cfg, err := h.settings.UpdateCheckinSetting(req, adminID)
	if err != nil {
		switch {
		case errors.Is(err, settingspoints.ErrCheckinConfigInvalidRewards):
			ginutil.RespondError(c, response.CodeBadRequest, "error.checkin_config_invalid_rewards", nil)
		case errors.Is(err, settingspoints.ErrCheckinOperatorRequired):
			ginutil.RespondError(c, response.CodeForbidden, "error.admin_id_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}
	response.Success(c, cfg)
}
