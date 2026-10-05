package http

import (
	"fmt"

	"github.com/Aether-v1/hcz/internal/app/httpserver/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RegisterUserRoutes 注册用户侧工单路由（挂已鉴权的 user 组）。
// createRule: 5/小时（按 user_id+IP）；replyRule: 3/10s（按 user_id+ticket_id）；attachmentRule: 10/分钟（按 user_id+IP）。
func RegisterUserRoutes(user *gin.RouterGroup, handler *UserHandler, redisClient *redis.Client, createRule, replyRule, attachmentRule middleware.RateLimitRule) {
	if user == nil || handler == nil {
		panic("supportticket user routes: nil dependency")
	}

	// 分类与工单只读
	user.GET("/support/categories", handler.ListCategories)
	user.GET("/support/tickets", handler.ListTickets)
	user.GET("/support/tickets/:id", handler.GetTicket)
	user.GET("/support/attachments/:id", handler.DownloadAttachment)

	// 创建工单：5/小时（user_id+IP）
	createGroup := user.Group("", middleware.RateLimitMiddleware(redisClient, createRule, middleware.KeyByUserIDAndIP))
	createGroup.POST("/support/tickets", handler.CreateTicket)

	// 回复/关闭/重开：3/10s（user_id+ticket_id）
	replyGroup := user.Group("", middleware.RateLimitMiddleware(redisClient, replyRule, KeyByUserIDAndTicketID))
	replyGroup.POST("/support/tickets/:id/replies", handler.ReplyTicket)
	replyGroup.POST("/support/tickets/:id/close", handler.CloseTicket)
	replyGroup.POST("/support/tickets/:id/reopen", handler.ReopenTicket)

	// 附件上传：10/分钟（user_id+IP）
	attGroup := user.Group("", middleware.RateLimitMiddleware(redisClient, attachmentRule, middleware.KeyByUserIDAndIP))
	attGroup.POST("/support/attachments", handler.UploadAttachment)
}

// KeyByUserIDAndTicketID 按 user_id + 工单 ID 限流（回复类接口，防止单工单刷回复）。
func KeyByUserIDAndTicketID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	userID, exists := c.Get("user_id")
	if !exists {
		return c.ClientIP()
	}
	ticketID := c.Param("id")
	return fmt.Sprintf("%v|ticket:%s", userID, ticketID)
}
