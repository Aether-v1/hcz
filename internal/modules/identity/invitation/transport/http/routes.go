package invitationhttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 在已登录的用户路由组下注册邀请相关路由。
func RegisterUserRoutes(user gin.IRoutes, handler *Handler) {
	user.GET("/invitation/me", handler.GetMyInvitation)
}
