package pointsphttp

import (
	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户端积分端点（挂 storefront user 组，JWT 鉴权）。
//
//	GET /points/account  积分账户（余额 / 累计获得 / 累计消费）
//	GET /points/ledger   积分流水（分页）
func RegisterUserRoutes(r gin.IRoutes, h *UserHandler) {
	if r == nil || h == nil {
		return
	}
	r.GET("/points/account", h.GetAccount)
	r.GET("/points/ledger", h.ListLedgerEntries)
}

// RegisterAdminRoutes 注册后台积分端点（挂 authorized 组，JWT + RBAC）。
//
// authorized 组已带 /api/v1/admin 前缀，因此此处路径一律写相对形式，
// 与仓库内其余 RegisterAdminRoutes 保持一致；Casbin object 为 /admin/...。
//
//	GET  /users/:id/points        用户积分账户
//	GET  /users/:id/points/ledger 用户积分流水（分页 + 过滤）
//	POST /users/:id/points/adjust 增减积分（Idempotency-Key + Reason）
//	POST /users/:id/points/compensate 人工补偿（独立 action，Idempotency-Key + Reason）
//	GET  /points/accounts         积分账户列表（negative_only=true 排查欠额用户）
//	GET  /points/stats            运营统计基础指标（直接聚合事实表）
func RegisterAdminRoutes(authorized gin.IRoutes, h *AdminHandler) {
	if authorized == nil || h == nil {
		return
	}
	authorized.GET("/users/:id/points", h.GetUserPoints)
	authorized.GET("/users/:id/points/ledger", h.GetUserLedgerEntries)
	authorized.POST("/users/:id/points/adjust", h.AdjustUserPoints)
	authorized.POST("/users/:id/points/compensate", h.CompensateUserPoints)
	authorized.GET("/points/accounts", h.ListAccounts)
	authorized.GET("/points/stats", h.GetStats)
}
