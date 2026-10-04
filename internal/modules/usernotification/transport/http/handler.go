package usernotificationhttp

import (
	"context"
	"errors"

	"github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	"github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
	"github.com/Aether-v1/hcz/internal/platform/http/ginutil"
	"github.com/Aether-v1/hcz/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// UserService 是用户收件箱用例的最小端口。
type UserService interface {
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]domain.UserNotification, int64, error)
	CountUnread(ctx context.Context, userID uint) (int64, error)
	MarkRead(ctx context.Context, id, userID uint) error
	MarkAllRead(ctx context.Context, userID uint) (int64, error)
}

// UserHandler 用户站内通知 HTTP handler。
type UserHandler struct {
	svc UserService
}

// NewUserHandler 构造用户通知 handler。
func NewUserHandler(svc UserService) *UserHandler {
	if svc == nil {
		panic("usernotification user handler: service is nil")
	}
	return &UserHandler{svc: svc}
}

// List GET /api/v1/notifications?page=&page_size=
func (h *UserHandler) List(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	items, total, err := h.svc.ListByUser(c.Request.Context(), uid, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, gin.H{
		"items":     newNotificationList(items),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// UnreadCount GET /api/v1/notifications/unread-count
func (h *UserHandler) UnreadCount(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	count, err := h.svc.CountUnread(c.Request.Context(), uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, gin.H{"count": count})
}

// MarkRead POST /api/v1/notifications/:id/read
func (h *UserHandler) MarkRead(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), id, uid); err != nil {
		if errors.Is(err, contract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// MarkAllRead POST /api/v1/notifications/read-all
func (h *UserHandler) MarkAllRead(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	marked, err := h.svc.MarkAllRead(c.Request.Context(), uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, gin.H{"ok": true, "marked": marked})
}
