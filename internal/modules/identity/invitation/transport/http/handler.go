package invitationhttp

import (
	"errors"

	invitationapp "github.com/Aether-v1/hcz/internal/modules/identity/invitation/application"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// Service 是邀请查询端点所需的最小端口。
type Service interface {
	GetMyInvitation(userID uint) (invitationapp.MyInvitation, error)
}

// Handler 处理前台邀请绑定查询请求。
type Handler struct {
	svc Service
}

// NewHandler 创建邀请查询 handler。
func NewHandler(svc Service) *Handler {
	if svc == nil {
		panic("invitation handler: service is nil")
	}
	return &Handler{svc: svc}
}

// GetMyInvitation GET /api/v1/invitation/me —— 返回当前登录用户的邀请码与绑定关系。
func (h *Handler) GetMyInvitation(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	data, err := h.svc.GetMyInvitation(userID)
	if err != nil {
		if errors.Is(err, invitationapp.ErrInvitationNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, data)
}
