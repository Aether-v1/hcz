package usernotificationhttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册用户站内通知端点（须挂在已登录 user 路由组下）。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("usernotification user routes: required dependency is nil")
	}
	user.GET("/notifications", handler.List)
	user.GET("/notifications/unread-count", handler.UnreadCount)
	user.POST("/notifications/read-all", handler.MarkAllRead)
	user.POST("/notifications/:id/read", handler.MarkRead)
}
