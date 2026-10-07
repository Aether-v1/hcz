package checkinphttp

import (
	"errors"
	"strings"

	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"
	ginutil "github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户端签到端点（挂 storefront user 组，JWT 鉴权）。
//
//	GET  /checkin/status   今日签到状态
//	POST /checkin          每日签到
//	GET  /checkin/history  本月签到日历（?month=YYYY-MM，默认当前月）
func RegisterUserRoutes(r gin.IRoutes, h *UserHandler) {
	if r == nil || h == nil {
		return
	}
	r.GET("/checkin/status", h.GetStatus)
	r.POST("/checkin", h.CheckIn)
	r.GET("/checkin/history", h.GetHistory)
}

// UserCheckinService 是用户端签到所需的最小端口。
type UserCheckinService interface {
	CheckIn(userID uint) (*checkincontract.CheckinResult, error)
	Status(userID uint) (*checkincontract.StatusResult, error)
	History(userID uint, month string) (*checkincontract.HistoryResult, error)
}

// UserHandler 处理用户端签到 HTTP 请求。
type UserHandler struct {
	checkins UserCheckinService
}

// NewUserHandler 创建签到用户 handler。
func NewUserHandler(checkins UserCheckinService) *UserHandler {
	if checkins == nil {
		panic("checkin user handler: required dependency is nil")
	}
	return &UserHandler{checkins: checkins}
}

// GetStatus 返回今日签到状态。
// user_id 仅取自 JWT 上下文，不接受客户端传入，杜绝 IDOR。
func (h *UserHandler) GetStatus(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	status, err := h.checkins.Status(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.checkin_fetch_failed", err)
		return
	}
	response.Success(c, gin.H{
		"enabled":          status.Enabled,
		"checked_in_today": status.CheckedInToday,
		"consecutive_days": status.ConsecutiveDays,
		"cycle_day":        status.CycleDay,
		"today_reward":     status.TodayReward,
		"next_reward":      status.NextReward,
	})
}

// CheckIn 每日签到。
//
// 幂等语义：同一天重复签到（含并发双击）返回 HTTP 200 + already_checked_in=true
// 与当天原奖励数据，不视为服务器错误（不 409 / 不 500）。
func (h *UserHandler) CheckIn(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	result, err := h.checkins.CheckIn(uid)
	if err != nil {
		switch {
		case errors.Is(err, checkincontract.ErrCheckinDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.checkin_disabled", nil)
		case errors.Is(err, checkincontract.ErrUserRequired):
			ginutil.RespondError(c, response.CodeUnauthorized, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.checkin_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"checkin_date":       result.CheckinDate,
		"points_awarded":     result.PointsAwarded,
		"consecutive_days":   result.ConsecutiveDays,
		"cycle_day":          result.CycleDay,
		"current_balance":    result.CurrentBalance,
		"already_checked_in": result.AlreadyCheckedIn,
	})
}

// GetHistory 返回本月签到日历数据（?month=YYYY-MM，默认当前 Asia/Shanghai 月份）。
// user_id 仅取自 JWT 上下文，杜绝 IDOR。
func (h *UserHandler) GetHistory(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	month := strings.TrimSpace(c.DefaultQuery("month", ""))
	history, err := h.checkins.History(uid, month)
	if err != nil {
		switch {
		case errors.Is(err, checkincontract.ErrInvalidMonth):
			ginutil.RespondError(c, response.CodeBadRequest, "error.checkin_invalid_month", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.checkin_fetch_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"year":          history.Year,
		"month":         history.Month,
		"checked_dates": history.CheckedDates,
		"entries":       history.Entries,
		"total":         history.Total,
	})
}
