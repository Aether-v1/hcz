package sitebuilderhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册站点装修后台管理路由。
// authorized 为已挂载 JWT + RBAC 中间件的路由组。
func RegisterAdminRoutes(authorized gin.IRoutes, handler *AdminHandler) {
	if handler == nil {
		panic("sitebuilder admin routes: handler is nil")
	}

	// 首页入口
	authorized.GET("/site/home-entries", handler.ListHomeEntries)
	authorized.POST("/site/home-entries", handler.CreateHomeEntry)
	authorized.GET("/site/home-entries/:id", handler.GetHomeEntry)
	authorized.PUT("/site/home-entries/:id", handler.UpdateHomeEntry)
	authorized.DELETE("/site/home-entries/:id", handler.DeleteHomeEntry)
	authorized.PATCH("/site/home-entries/:id/toggle", handler.ToggleHomeEntry)
	authorized.POST("/site/home-entries/reorder", handler.ReorderHomeEntries)

	// 首页热门推荐分类
	authorized.GET("/site/featured-categories", handler.ListFeaturedCategories)
	authorized.POST("/site/featured-categories", handler.CreateFeaturedCategory)
	authorized.PUT("/site/featured-categories/:id", handler.UpdateFeaturedCategory)
	authorized.DELETE("/site/featured-categories/:id", handler.DeleteFeaturedCategory)
	authorized.PATCH("/site/featured-categories/:id/toggle", handler.ToggleFeaturedCategory)
	authorized.POST("/site/featured-categories/reorder", handler.ReorderFeaturedCategories)

	// 发现页区块
	authorized.GET("/site/discovery-blocks", handler.ListDiscoveryBlocks)
	authorized.POST("/site/discovery-blocks", handler.CreateDiscoveryBlock)
	authorized.GET("/site/discovery-blocks/:id", handler.GetDiscoveryBlock)
	authorized.PUT("/site/discovery-blocks/:id", handler.UpdateDiscoveryBlock)
	authorized.DELETE("/site/discovery-blocks/:id", handler.DeleteDiscoveryBlock)
	authorized.PATCH("/site/discovery-blocks/:id/toggle", handler.ToggleDiscoveryBlock)
	authorized.POST("/site/discovery-blocks/reorder", handler.ReorderDiscoveryBlocks)

	// 品牌
	authorized.GET("/site/brand", handler.GetBrand)
	authorized.PUT("/site/brand", handler.UpdateBrand)

	// 审计日志
	authorized.GET("/site/audit-logs", handler.ListAuditLogs)
}
