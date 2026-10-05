package http

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册客服侧工单路由（挂 authorized 组：JWT + RBAC）。
func RegisterAdminRoutes(authorized gin.IRoutes, handler *AdminHandler) {
	if authorized == nil || handler == nil {
		panic("supportticket admin routes: nil dependency")
	}

	// 概览
	authorized.GET("/support/overview", handler.Overview)

	// 工单
	authorized.GET("/support/tickets", handler.ListTickets)
	authorized.GET("/support/tickets/:id", handler.GetTicket)
	authorized.POST("/support/tickets/:id/replies", handler.ReplyTicket)
	authorized.POST("/support/tickets/:id/assign", handler.AssignTicket)
	authorized.POST("/support/tickets/:id/change-priority", handler.ChangePriority)
	authorized.POST("/support/tickets/:id/resolve", handler.ResolveTicket)
	authorized.POST("/support/tickets/:id/close", handler.CloseTicket)
	authorized.POST("/support/tickets/:id/reopen", handler.ReopenTicket)
	authorized.GET("/support/tickets/:id/audits", handler.ListAudits)

	// 分类管理
	authorized.GET("/support/categories", handler.ListCategories)
	authorized.POST("/support/categories", handler.CreateCategory)
	authorized.PUT("/support/categories/:id", handler.UpdateCategory)
	authorized.DELETE("/support/categories/:id", handler.DeleteCategory)
}
