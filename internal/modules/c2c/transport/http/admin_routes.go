package http

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册 C2C 后台管理路由。
//   - authorized：普通读写（JWT + RBAC）
//   - paymentProtected：仲裁（JWT + RBAC + PaymentCompliance，Handler 内再做 Step-Up）
//
// authorized/paymentProtected 为已挂载中间件的路由组，直接在其上注册 /c2c 前缀路径。
func RegisterAdminRoutes(authorized gin.IRoutes, paymentProtected gin.IRoutes, handler *AdminHandler) {
	if handler == nil {
		panic("c2c admin routes: handler is nil")
	}

	authorized.GET("/c2c/overview", handler.Overview)
	authorized.GET("/c2c/listings", handler.ListListings)
	authorized.GET("/c2c/listings/:id", handler.GetListing)
	authorized.PUT("/c2c/listings/:id/close", handler.CloseListing)

	authorized.GET("/c2c/trades", handler.ListTrades)
	authorized.GET("/c2c/trades/:id", handler.GetTrade)

	authorized.GET("/c2c/disputes", handler.ListDisputes)
	authorized.GET("/c2c/disputes/:id", handler.GetDispute)

	authorized.GET("/c2c/users/:id", handler.GetUserC2CStatus)
	authorized.POST("/c2c/users/:id/disable", handler.DisableUserC2C)
	authorized.POST("/c2c/users/:id/enable", handler.EnableUserC2C)

	authorized.GET("/c2c/risk-signals", handler.ListRiskSignals)

	authorized.GET("/c2c/settings", handler.GetSettings)
	authorized.PUT("/c2c/settings", handler.UpdateSettings)

	// 仲裁涉及资金放行/退回，挂 paymentProtected 组。
	paymentProtected.POST("/c2c/arbitration", handler.Arbitrate)
}
