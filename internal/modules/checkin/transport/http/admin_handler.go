package checkinphttp

import (
	"errors"
	"strings"

	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	ginutil "github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册后台签到端点（挂 authorized 组，JWT + RBAC）。
//
// authorized 组已带 /api/v1/admin 前缀，此处路径一律写相对形式。
//
//	GET /users/:id/checkins  用户签到历史（?month=YYYY-MM，默认当前月）
//
// 后台对签到记录是**只读**的：V1 不提供 Admin 补签、删除签到或修改签到日期。
// 签到记录只能由用户本人签到这一条路径产生，这是"签到奖励不可伪造"的前提。
// 若签到奖励发放有误，处理方式是积分模块的人工补偿（ADMIN_COMPENSATION），
// 而不是改写 user_checkins 历史。
func RegisterAdminRoutes(authorized gin.IRoutes, h *AdminHandler) {
	if authorized == nil || h == nil {
		return
	}
	authorized.GET("/users/:id/checkins", h.GetUserCheckins)
}

// AdminCheckinService 是后台签到查看所需的最小端口（复用用户侧同一查询用例）。
type AdminCheckinService interface {
	History(userID uint, month string) (*checkincontract.HistoryResult, error)
}

// AdminUserReader 用于确认 :id 指向真实用户。
type AdminUserReader interface {
	GetByID(id uint) (*userdomain.User, error)
}

// AdminHandler 处理后台签到 HTTP 请求（只读）。
type AdminHandler struct {
	checkins AdminCheckinService
	users    AdminUserReader
}

// NewAdminHandler 创建签到后台 handler。
func NewAdminHandler(checkins AdminCheckinService, users AdminUserReader) *AdminHandler {
	if checkins == nil || users == nil {
		panic("checkin admin handler: required dependency is nil")
	}
	return &AdminHandler{checkins: checkins, users: users}
}

// GetUserCheckins 后台查看某用户指定月份的签到记录（只读）。
//
// 返回字段：user_id / year / month / checked_dates / entries / total /
// latest_consecutive_days（本月最后一次签到时的连签天数，无记录为 0）。
func (h *AdminHandler) GetUserCheckins(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}
	if _, ok := ginutil.GetAdminID(c); !ok {
		return
	}
	user, err := h.users.GetByID(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if user == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return
	}

	month := strings.TrimSpace(c.DefaultQuery("month", ""))
	history, err := h.checkins.History(user.ID, month)
	if err != nil {
		switch {
		case errors.Is(err, checkincontract.ErrInvalidMonth):
			ginutil.RespondError(c, response.CodeBadRequest, "error.checkin_invalid_month", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.checkin_fetch_failed", err)
		}
		return
	}
	latestConsecutive := 0
	if n := len(history.Entries); n > 0 {
		latestConsecutive = history.Entries[n-1].ConsecutiveDays
	}
	response.Success(c, gin.H{
		"user_id":                 user.ID,
		"year":                    history.Year,
		"month":                   history.Month,
		"checked_dates":           history.CheckedDates,
		"entries":                 history.Entries,
		"total":                   history.Total,
		"latest_consecutive_days": latestConsecutive,
	})
}
