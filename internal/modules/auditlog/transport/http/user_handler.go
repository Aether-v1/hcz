package auditloghttp

import (
	"github.com/Aether-v1/hcz/internal/modules/auditlog/domain"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type UserLoginHistoryReader interface {
	ListByUser(userID uint, page, pageSize int) ([]domain.UserLoginLog, int64, error)
}

type UserHandler struct {
	userLoginLogs UserLoginHistoryReader
}

func NewUserHandler(userLoginLogs UserLoginHistoryReader) *UserHandler {
	return &UserHandler{userLoginLogs: userLoginLogs}
}

// GetMyLoginLogs 获取当前用户登录日志
func (h *UserHandler) GetMyLoginLogs(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	page, pageSize := ginutil.ParsePagination(c)

	logs, total, err := h.userLoginLogs.ListByUser(uid, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_login_log_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, newUserLoginResponseList(logs), pagination)
}
