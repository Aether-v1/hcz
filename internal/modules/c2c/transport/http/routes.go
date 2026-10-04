package http

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册 C2C 用户侧端点（挂 user 组，前缀 /api/v1）。
func RegisterUserRoutes(user gin.IRoutes, handler *Handler) {
	if user == nil || handler == nil {
		panic("c2c user routes: required dependency is nil")
	}
	// 支付方式
	user.GET("/c2c/payment-methods", handler.ListPaymentMethods)
	user.POST("/c2c/payment-methods", handler.CreatePaymentMethod)
	user.PUT("/c2c/payment-methods/:id", handler.UpdatePaymentMethod)
	user.DELETE("/c2c/payment-methods/:id", handler.DeletePaymentMethod)
	user.POST("/c2c/payment-methods/:id/enabled", handler.SetPaymentMethodEnabled)

	// 挂单
	user.GET("/c2c/listings/market", handler.ListMarketListings)
	user.GET("/c2c/listings/my", handler.ListMyListings)
	user.GET("/c2c/listings/:id", handler.GetListingDetail)
	user.POST("/c2c/listings", handler.CreateListing)
	user.PUT("/c2c/listings/:id", handler.UpdateListing)
	user.POST("/c2c/listings/:id/pause", handler.PauseListing)
	user.POST("/c2c/listings/:id/resume", handler.ResumeListing)
	user.POST("/c2c/listings/:id/close", handler.CloseListing)

	// 交易
	user.POST("/c2c/trades", handler.CreateTrade)
	user.GET("/c2c/trades/my", handler.ListMyTrades)
	user.GET("/c2c/trades/:id", handler.GetTradeDetail)
	user.POST("/c2c/trades/:id/mark-paid", handler.MarkPaid)
	user.POST("/c2c/trades/:id/cancel", handler.Cancel)
	user.POST("/c2c/trades/:id/confirm", handler.Confirm)
	user.POST("/c2c/trades/:id/dispute", handler.Dispute)
}
