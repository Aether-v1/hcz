package withdrawalhttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册用户提现端点（挂 user 组，前缀 /api/v1）。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("withdrawal user routes: required dependency is nil")
	}
	user.POST("/wallet/withdrawals", handler.Create)
	user.POST("/wallet/withdrawals/quote", handler.Quote)
	user.GET("/wallet/withdrawals", handler.List)
	user.GET("/wallet/withdrawals/:id", handler.Detail)
	user.POST("/wallet/withdrawals/:id/cancel", handler.Cancel)

	user.GET("/wallet/withdrawal-addresses", handler.ListAddresses)
	user.POST("/wallet/withdrawal-addresses", handler.CreateAddress)
	user.DELETE("/wallet/withdrawal-addresses/:id", handler.DeleteAddress)
	user.POST("/wallet/withdrawal-addresses/:id/default", handler.SetDefaultAddress)
}

// RegisterAdminRoutes 注册后台提现端点（挂 paymentProtected 组，前缀 /api/admin/v1/wallet/withdrawals）。
func RegisterAdminRoutes(paymentProtected gin.IRoutes, handler *AdminHandler) {
	if paymentProtected == nil || handler == nil {
		panic("withdrawal admin routes: required dependency is nil")
	}
	paymentProtected.GET("/wallet/withdrawals", handler.List)
	paymentProtected.GET("/wallet/withdrawals/:id", handler.Detail)
	paymentProtected.POST("/wallet/withdrawals/:id/approve", handler.Approve)
	paymentProtected.POST("/wallet/withdrawals/:id/reject", handler.Reject)
	paymentProtected.POST("/wallet/withdrawals/:id/processing", handler.Processing)
	paymentProtected.POST("/wallet/withdrawals/:id/complete", handler.Complete)
}
