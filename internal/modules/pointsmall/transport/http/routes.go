package pointsmallphttp

import (
	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户端积分商城端点（挂 storefront user 组，JWT 鉴权）。
//
//	GET  /points/products                         积分商品列表（仅 enabled，分页）
//	GET  /points/products/:id                     积分商品详情（含 can_redeem 辅助）
//	POST /points/exchange-orders                  创建兑换订单（Idempotency-Key 必填）
//	GET  /points/exchange-orders                  我的兑换订单列表（?status= 过滤，分页）
//	GET  /points/exchange-orders/:id              我的兑换订单详情（IDOR 防御）
//	POST /points/exchange-orders/:id/cancel       取消兑换（仅 PENDING，返还积分 + 恢复库存）
func RegisterUserRoutes(r gin.IRoutes, h *UserHandler) {
	if r == nil || h == nil {
		return
	}
	r.GET("/points/products", h.ListProducts)
	r.GET("/points/products/:id", h.GetProductDetail)
	r.POST("/points/exchange-orders", h.CreateExchange)
	r.GET("/points/exchange-orders", h.ListExchangeOrders)
	r.GET("/points/exchange-orders/:id", h.GetExchangeOrder)
	r.POST("/points/exchange-orders/:id/cancel", h.CancelExchangeOrder)
}

// RegisterAdminRoutes 注册后台积分商城端点（挂 authorized 组，JWT + RBAC）。
//
// authorized 组已带 /api/v1/admin 前缀，此处路径一律写相对形式（同其余模块）。
//
//	商品：
//	GET  /points/products                     商品列表（可含下架，分页）
//	POST /points/products                     创建商品
//	PUT  /points/products/:id                 更新商品（含可用库存绝对值）
//	PATCH /points/products/:id/status         上下架
//
//	兑换订单：
//	GET  /points/exchange-orders              订单列表（?status=&user_id= 过滤，分页）
//	GET  /points/exchange-orders/:id          订单详情
//	POST /points/exchange-orders/:id/process  PENDING → PROCESSING
//	POST /points/exchange-orders/:id/complete PROCESSING → COMPLETED
//	POST /points/exchange-orders/:id/fail     → FAILED（返还积分 + 恢复库存，Reason 必填）
//	POST /points/exchange-orders/:id/cancel   → CANCELLED（返还积分 + 恢复库存，Reason 必填）
func RegisterAdminRoutes(authorized gin.IRoutes, h *AdminHandler) {
	if authorized == nil || h == nil {
		return
	}
	authorized.GET("/points/products", h.ListProducts)
	authorized.POST("/points/products", h.CreateProduct)
	authorized.PUT("/points/products/:id", h.UpdateProduct)
	authorized.PATCH("/points/products/:id/status", h.SetProductEnabled)

	authorized.GET("/points/exchange-orders", h.ListExchangeOrders)
	authorized.GET("/points/exchange-orders/:id", h.GetExchangeOrder)
	authorized.POST("/points/exchange-orders/:id/process", h.ProcessOrder)
	authorized.POST("/points/exchange-orders/:id/complete", h.CompleteOrder)
	authorized.POST("/points/exchange-orders/:id/fail", h.FailOrder)
	authorized.POST("/points/exchange-orders/:id/cancel", h.CancelOrder)
}
