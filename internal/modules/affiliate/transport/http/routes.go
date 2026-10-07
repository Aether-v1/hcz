package affiliatehttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册公开推广点击路由。
func RegisterPublicRoutes(public gin.IRoutes, handler *Handler) {
	public.POST("/affiliate/click", handler.TrackAffiliateClick)
}

// RegisterUserRoutes 注册需登录的推广返利路由。
func RegisterUserRoutes(user gin.IRoutes, handler *Handler) {
	user.POST("/affiliate/open", handler.OpenAffiliate)                 // 已退休，返回 410
	user.POST("/affiliate/apply", handler.ApplyAffiliate)               // 新：提交推广申请
	user.GET("/affiliate/application", handler.GetAffiliateApplication) // 新：查询我的申请
	user.GET("/affiliate/profile", handler.GetAffiliateProfile)         // 新：查询我的 profile
	user.GET("/affiliate/dashboard", handler.GetAffiliateDashboard)
	user.GET("/affiliate/commissions", handler.ListAffiliateCommissions)
	user.GET("/affiliate/withdraws", handler.ListAffiliateWithdraws)     // 归档只读：历史提现记录
	user.POST("/affiliate/withdraws", handler.ApplyAffiliateWithdraw)    // 已退休，返回 410
	user.POST("/affiliate/transfer-to-wallet", handler.TransferToWallet) // 新：佣金划转至主钱包
	user.GET("/affiliate/transfers", handler.ListAffiliateTransfers)     // 新：划转历史
}

// RegisterAdminRoutes 注册后台推广用户管理路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/affiliates/users", handler.ListAffiliateUsers)
	admin.PATCH("/affiliates/users/:id/status", handler.UpdateAffiliateUserStatus)
	admin.PATCH("/affiliates/users/batch-status", handler.BatchUpdateAffiliateUserStatus)
	// 推广申请审核
	admin.GET("/affiliates/applications", handler.ListAffiliateApplications)
	admin.GET("/affiliates/applications/:id", handler.GetAffiliateApplication)
	admin.POST("/affiliates/applications/:id/approve", handler.ApproveAffiliateApplication)
	admin.POST("/affiliates/applications/:id/reject", handler.RejectAffiliateApplication)
}

// RegisterAdminFinanceRoutes 注册后台推广财务审核路由（需支付合规）。
func RegisterAdminFinanceRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/affiliates/commissions", handler.ListAffiliateCommissions)
	admin.GET("/affiliates/withdraws", handler.ListAffiliateWithdraws)
	admin.POST("/affiliates/withdraws/:id/reject", handler.RejectAffiliateWithdraw)
	admin.POST("/affiliates/withdraws/:id/pay", handler.PayAffiliateWithdraw)
}

// RegisterChannelRoutes 注册渠道推广返利路由。
func RegisterChannelRoutes(channel gin.IRoutes, handler *ChannelHandler) {
	channel.POST("/affiliate/click", handler.TrackAffiliateClick)
	channel.POST("/affiliate/open", handler.OpenAffiliate)
	channel.GET("/affiliate/dashboard", handler.GetAffiliateDashboard)
	channel.GET("/affiliate/commissions", handler.ListAffiliateCommissions)
	channel.GET("/affiliate/withdraws", handler.ListAffiliateWithdraws)     // 归档只读
	channel.POST("/affiliate/withdraws", handler.ApplyAffiliateWithdraw)    // 已退休，返回 410
	channel.POST("/affiliate/transfer-to-wallet", handler.TransferToWallet) // 新：佣金划转至主钱包
	channel.GET("/affiliate/transfers", handler.ListAffiliateTransfers)     // 新：划转历史
}
