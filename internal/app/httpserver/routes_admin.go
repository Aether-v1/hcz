package httpserver

import (
	"github.com/Aether-v1/hcz/internal/app/container"
	"github.com/Aether-v1/hcz/internal/app/httpserver/middleware"
	affiliatebootstrap "github.com/Aether-v1/hcz/internal/bootstrap/affiliate"
	settingsbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/settingshttp"
	"github.com/Aether-v1/hcz/internal/config"
	adproxytransport "github.com/Aether-v1/hcz/internal/modules/adproxy/transport/http"
	affiliatetransport "github.com/Aether-v1/hcz/internal/modules/affiliate/transport/http"
	apicredentialtransport "github.com/Aether-v1/hcz/internal/modules/apicredential/transport/http"
	auditlogtransport "github.com/Aether-v1/hcz/internal/modules/auditlog/transport/http"
	c2ctransport "github.com/Aether-v1/hcz/internal/modules/c2c/transport/http"
	cardsecrettransport "github.com/Aether-v1/hcz/internal/modules/cardsecret/transport/http"
	categoryhttp "github.com/Aether-v1/hcz/internal/modules/catalog/category/transport/http"
	mappinghttp "github.com/Aether-v1/hcz/internal/modules/catalog/mapping/transport/http"
	producthttp "github.com/Aether-v1/hcz/internal/modules/catalog/product/transport/http"
	channelclienthttp "github.com/Aether-v1/hcz/internal/modules/channelclient/transport/http"
	checkinphttp "github.com/Aether-v1/hcz/internal/modules/checkin/transport/http"
	compliancetransport "github.com/Aether-v1/hcz/internal/modules/compliance/transport/http"
	contenttransport "github.com/Aether-v1/hcz/internal/modules/content/transport/http"
	coupontransport "github.com/Aether-v1/hcz/internal/modules/coupon/transport/http"
	dashboardtransport "github.com/Aether-v1/hcz/internal/modules/dashboard/transport/http"
	exchangeratetransport "github.com/Aether-v1/hcz/internal/modules/exchangerate/transport"
	fulfillmenttransport "github.com/Aether-v1/hcz/internal/modules/fulfillment/transport/http"
	giftcardtransport "github.com/Aether-v1/hcz/internal/modules/giftcard/transport/http"
	adminauthtransport "github.com/Aether-v1/hcz/internal/modules/identity/adminauth/transport/http"
	adminauthztransport "github.com/Aether-v1/hcz/internal/modules/identity/adminauthorization/transport/http"
	adminusertransport "github.com/Aether-v1/hcz/internal/modules/identity/user/transport/http/admin"
	memberleveltransport "github.com/Aether-v1/hcz/internal/modules/memberlevel/transport/http"
	notificationtransport "github.com/Aether-v1/hcz/internal/modules/notification/transport/http"
	ordertransport "github.com/Aether-v1/hcz/internal/modules/order/transport/http"
	paymenttransport "github.com/Aether-v1/hcz/internal/modules/payment/transport/http"
	procurementtransport "github.com/Aether-v1/hcz/internal/modules/procurement/transport/http"
	pricinghttp "github.com/Aether-v1/hcz/internal/modules/pricing/transport/http"
	promotiontransport "github.com/Aether-v1/hcz/internal/modules/promotion/transport/http"
	pointsphttp "github.com/Aether-v1/hcz/internal/modules/points/transport/http"
	pointsmallphttp "github.com/Aether-v1/hcz/internal/modules/pointsmall/transport/http"
	reconciliationtransport "github.com/Aether-v1/hcz/internal/modules/reconciliation/transport/http"
	resellertransport "github.com/Aether-v1/hcz/internal/modules/reseller/transport/http/admin"
	settingstransport "github.com/Aether-v1/hcz/internal/modules/settings/transport/http"
	sitebuilderttp "github.com/Aether-v1/hcz/internal/modules/sitebuilder/transport/http"
	siteconnectiontransport "github.com/Aether-v1/hcz/internal/modules/siteconnection/transport/http"
	supporttickethttp "github.com/Aether-v1/hcz/internal/modules/supportticket/transport/http"
	broadcasthttp "github.com/Aether-v1/hcz/internal/modules/telegram/broadcast/transport/http"
	uploadtransport "github.com/Aether-v1/hcz/internal/modules/upload/transport/http"
	wallettransport "github.com/Aether-v1/hcz/internal/modules/wallet/transport/http"
	withdrawalhttp "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/transport/http"
	"github.com/Aether-v1/hcz/internal/platform/http/response"
	systemtransport "github.com/Aether-v1/hcz/internal/platform/http/system"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func registerAdminRoutes(
	engine *gin.Engine,
	apiV1 *gin.RouterGroup,
	cfg *config.Config,
	c *container.Container,
	adminLoginHandler *adminauthtransport.AdminLoginHandler,
	admin2FAHandler *adminauthtransport.Admin2FAHandler,
	adminUser2FAHandler *adminauthtransport.AdminUser2FAHandler,
	adminUserHandler *adminusertransport.AdminHandler,
	adminAuthzHandler *adminauthztransport.AdminHandler,
	adminFulfillmentHandler *fulfillmenttransport.AdminHandler,
	adminOrderHandler *ordertransport.AdminHandler,
	adminOrderRefundHandler *ordertransport.AdminRefundHandler,
	afterSaleHandler *ordertransport.AfterSaleHandler,
	adminContentHandler *contenttransport.AdminHandler,
	adminDashboardHandler *dashboardtransport.AdminHandler,
	adminMemberLevelHandler *memberleveltransport.AdminHandler,
	adminApiCredentialHandler *apicredentialtransport.AdminHandler,
	adminAuditLogHandler *auditlogtransport.AdminHandler,
	adminCardSecretHandler *cardsecrettransport.AdminHandler,
	adminCatalogCategoryHandler *categoryhttp.AdminCategoryHandler,
	adminCatalogProductHandler *producthttp.AdminProductHandler,
	adminCatalogProductMappingHandler *mappinghttp.AdminHandler,
	adminCouponHandler *coupontransport.AdminHandler,
	adminGiftCardHandler *giftcardtransport.AdminHandler,
	adminPromotionHandler *promotiontransport.AdminHandler,
	adminNotificationHandler *notificationtransport.AdminHandler,
	adminProcurementHandler *procurementtransport.AdminHandler,
	adminResellerManagementHandler *resellertransport.AdminManagementHandler,
	adminResellerProfileDetailHandler *resellertransport.AdminProfileDetailHandler,
	adminResellerSiteConfigHandler *resellertransport.AdminSiteConfigHandler,
	adminResellerProductSettingHandler *resellertransport.AdminProductSettingHandler,
	adminResellerOperationsHandler *resellertransport.AdminOperationsHandler,
	adminResellerFinanceHandler *resellertransport.AdminFinanceHandler,
	adminSettingsHandler *settingstransport.AdminHandler,
	adminWalletHandler *wallettransport.AdminHandler,
	adminPointsHandler *pointsphttp.AdminHandler,
	adminCheckinHandler *checkinphttp.AdminHandler,
	adminPointsmallHandler *pointsmallphttp.AdminHandler,
	adminWithdrawalHandler *withdrawalhttp.AdminHandler,
	adminPaymentHandler *paymenttransport.AdminHandler,
	adminPaymentChannelHandler *paymenttransport.AdminChannelHandler,
	adminC2CHandler *c2ctransport.AdminHandler,
	supportAdminHandler *supporttickethttp.AdminHandler,
	siteBuilderAdminHandler *sitebuilderttp.AdminHandler,
	redisClient *redis.Client,
	adminLoginRule middleware.RateLimitRule,
) {
	admin := apiV1.Group("/admin")

	// 登录接口（无需鉴权）
	adminauthtransport.RegisterAdminLoginAuthRoutes(admin, adminLoginHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))
	adminauthtransport.RegisterAdmin2FAAuthRoutes(admin, admin2FAHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))

	// 需要鉴权的接口
	authorized := admin.Use(middleware.JWTAuthMiddleware(cfg.JWT.SecretKey, c.AdminStore), middleware.AdminRBACMiddleware(c.AuthzService))
	// 注：历史上支付/财务路由另挂独立子组（合规声明闸门），该阻断已移除，
	// 所有原受保护财务路由统一挂在 authorized（JWT + RBAC）下。

	// 合规声明
	compliancetransport.RegisterAdminRoutes(authorized, compliancetransport.NewAdminHandler(c.ComplianceService))

	// 仪表盘
	dashboardtransport.RegisterAdminRoutes(authorized, adminDashboardHandler)

	// 广告代理
	adproxytransport.RegisterAdminRoutes(authorized, adproxytransport.NewAdminHandler(c.AdProxyService))

	// 商品 / 分类管理
	producthttp.RegisterAdminRoutes(authorized, adminCatalogProductHandler)
	contenttransport.RegisterAdminRoutes(authorized, adminContentHandler)
	categoryhttp.RegisterAdminRoutes(authorized, adminCatalogCategoryHandler)

	// 设置管理
	settingstransport.RegisterAdminRoutes(authorized, adminSettingsHandler)
	settingstransport.RegisterAdminSMTPRoutes(authorized, settingsbootstrap.NewSMTPHandler(c, cfg))
	settingstransport.RegisterAdminCaptchaRoutes(authorized, settingsbootstrap.NewCaptchaHandler(c, cfg))
	settingstransport.RegisterAdminTelegramAuthRoutes(authorized, settingsbootstrap.NewTelegramAuthHandler(c, cfg))
	settingstransport.RegisterAdminGoogleAuthRoutes(authorized, settingsbootstrap.NewGoogleAuthHandler(c, cfg))
	notificationtransport.RegisterAdminRoutes(authorized, adminNotificationHandler)
	settingstransport.RegisterAdminOrderEmailTemplateRoutes(authorized, settingstransport.NewOrderEmailTemplateHandler(c.SettingService))
	settingstransport.RegisterAdminAffiliateRoutes(authorized, settingstransport.NewAffiliateHandler(c.SettingService))
	settingstransport.RegisterAdminProfitGuardRoutes(authorized, settingstransport.NewProfitGuardHandler(c.SettingService))
	settingstransport.RegisterAdminTelegramBotRoutes(authorized, settingstransport.NewTelegramBotHandler(c.SettingService))
	settingstransport.RegisterAdminCheckinRoutes(authorized, settingstransport.NewCheckinHandler(c.SettingService))

	// HCZ P0-2: 全局汇率（USDT 结算）管理，挂在 /admin/settings 下复用现有 JWT/RBAC。
	exchRateHandler := exchangeratetransport.NewAdminHandler(c.ExchangeRateService)
	authorized.GET("/settings/exchange-rate", exchRateHandler.Get)
	authorized.PUT("/settings/exchange-rate", exchRateHandler.Update)
	authorized.POST("/settings/exchange-rate/refresh", exchRateHandler.Refresh)

	// HCZ Profit Guard：管理员定价预览（只读核算明细，敏感成本字段不进 user DTO）。
	pricingPreviewHandler := pricinghttp.NewAdminHandler(c.ProductReadService, c.SettingService, c.ExchangeRateService)
	authorized.POST("/pricing/preview", pricingPreviewHandler.Preview)

	adminauthtransport.RegisterAdminPasswordRoutes(authorized, adminLoginHandler)

	// 系统信息与版本检测
	systemtransport.RegisterAdminRoutes(authorized, systemtransport.NewAdminHandler(nil))

	adminauthtransport.RegisterAdmin2FARoutes(authorized, admin2FAHandler)

	// 推广返利
	adminAffiliateHandler := affiliatebootstrap.NewAdminHandler(c)
	affiliatetransport.RegisterAdminRoutes(authorized, adminAffiliateHandler)
	affiliatetransport.RegisterAdminFinanceRoutes(authorized, adminAffiliateHandler)
	resellertransport.RegisterOperationsOverviewRoutes(authorized, adminResellerOperationsHandler)
	resellertransport.RegisterManagementRoutes(authorized, adminResellerManagementHandler)
	resellertransport.RegisterProfileDetailRoutes(authorized, adminResellerProfileDetailHandler)
	resellertransport.RegisterSiteConfigRoutes(authorized, adminResellerSiteConfigHandler)
	resellertransport.RegisterProductSettingRoutes(authorized, adminResellerProductSettingHandler)
	resellertransport.RegisterOperationsFinanceRoutes(authorized, adminResellerOperationsHandler)
	resellertransport.RegisterFinanceRoutes(authorized, adminResellerFinanceHandler)

	// 权限管理
	adminauthztransport.RegisterAdminRoutes(authorized, adminAuthzHandler)
	auditlogtransport.RegisterAdminRoutes(authorized, adminAuditLogHandler)
	authorized.GET("/authz/permissions/catalog", func(ctx *gin.Context) {
		response.Success(ctx, buildAdminPermissionCatalog(engine))
	})

	// 文件上传
	uploadtransport.RegisterAdminRoutes(authorized, uploadtransport.NewAdminHandler(c.UploadService, c.ContentMediaService))

	// 订单管理
	ordertransport.RegisterAdminRoutes(authorized, adminOrderHandler)
	ordertransport.RegisterAdminRefundWriteRoutes(authorized, adminOrderRefundHandler)
	ordertransport.RegisterAdminRefundRoutes(authorized, adminOrderRefundHandler)
	ordertransport.RegisterAdminAfterSaleRoutes(authorized, afterSaleHandler)
	ordertransport.RegisterAdminAfterSaleWriteRoutes(authorized, afterSaleHandler)
	fulfillmenttransport.RegisterAdminRoutes(authorized, adminFulfillmentHandler)
	cardsecrettransport.RegisterAdminRoutes(authorized, adminCardSecretHandler)
	giftcardtransport.RegisterAdminRoutes(authorized, adminGiftCardHandler)

	// 优惠券与活动价
	coupontransport.RegisterAdminRoutes(authorized, adminCouponHandler)
	promotiontransport.RegisterAdminRoutes(authorized, adminPromotionHandler)

	// 会员等级
	memberleveltransport.RegisterAdminRoutes(authorized, adminMemberLevelHandler)

	// 支付渠道与支付记录
	paymenttransport.RegisterAdminChannelRoutes(authorized, adminPaymentChannelHandler)
	paymenttransport.RegisterAdminRoutes(authorized, adminPaymentHandler)

	// 用户管理
	adminusertransport.RegisterAdminRoutes(authorized, adminUserHandler)
	wallettransport.RegisterAdminRoutes(authorized, adminWalletHandler)
	pointsphttp.RegisterAdminRoutes(authorized, adminPointsHandler)
	checkinphttp.RegisterAdminRoutes(authorized, adminCheckinHandler)
	pointsmallphttp.RegisterAdminRoutes(authorized, adminPointsmallHandler)
	withdrawalhttp.RegisterAdminRoutes(authorized, adminWithdrawalHandler)
	adminauthtransport.RegisterAdminUser2FARoutes(authorized, adminUser2FAHandler)

	// API 凭证审核管理
	apicredentialtransport.RegisterAdminRoutes(authorized, adminApiCredentialHandler)

	// 站点对接连接管理
	siteconnectiontransport.RegisterAdminRoutes(authorized, siteconnectiontransport.NewAdminHandler(
		c.SiteConnectionService,
		c.ProductMappingService,
	))

	// 商品映射管理
	mappinghttp.RegisterAdminRoutes(authorized, adminCatalogProductMappingHandler)

	// 采购单管理
	procurementtransport.RegisterAdminRoutes(authorized, adminProcurementHandler)

	// 对账管理
	reconciliationtransport.RegisterAdminRoutes(authorized, reconciliationtransport.NewAdminHandler(c.ReconciliationService))

	// 渠道客户端管理
	channelclienthttp.RegisterAdminRoutes(authorized, channelclienthttp.NewAdminHandler(c.ChannelClientService))

	// Telegram Bot 群发
	broadcasthttp.RegisterAdminRoutes(authorized, broadcasthttp.NewAdminHandler(c.TelegramBroadcastService))

	// C2C 后台管理：普通读写与仲裁均挂 authorized（Handler 内做 Step-Up），合规闸门已移除
	c2ctransport.RegisterAdminRoutes(authorized, authorized, adminC2CHandler)

	// 客服工单管理
	supporttickethttp.RegisterAdminRoutes(authorized, supportAdminHandler)

	// 站点装修（首页入口 / 发现页区块 / 品牌 / 模板 / 审计）
	sitebuilderttp.RegisterAdminRoutes(authorized, siteBuilderAdminHandler)
}
