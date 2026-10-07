package httpserver

import (
	"fmt"
	"net"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/Aether-v1/hcz/internal/app/container"
	"github.com/Aether-v1/hcz/internal/app/httpserver/middleware"
	"github.com/Aether-v1/hcz/internal/authz"
	adminauthwiring "github.com/Aether-v1/hcz/internal/bootstrap/adminauth"
	adminauthzwiring "github.com/Aether-v1/hcz/internal/bootstrap/adminauthz"
	adminuserwiring "github.com/Aether-v1/hcz/internal/bootstrap/adminuser"
	affiliatebootstrap "github.com/Aether-v1/hcz/internal/bootstrap/affiliate"
	c2cbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/c2c"
	catalogproductbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/catalogproduct"
	channelwiring "github.com/Aether-v1/hcz/internal/bootstrap/channelapi"
	channeluserwiring "github.com/Aether-v1/hcz/internal/bootstrap/channeluser"
	checkinbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/checkin"
	pointsmallbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/pointsmall"
	fulfillmentwiring "github.com/Aether-v1/hcz/internal/bootstrap/fulfillment"
	orderwiring "github.com/Aether-v1/hcz/internal/bootstrap/order"
	paymentwiring "github.com/Aether-v1/hcz/internal/bootstrap/payment"
	pointsbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/points"
	publicconfigwiring "github.com/Aether-v1/hcz/internal/bootstrap/publicconfig"
	resellerbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/reseller"
	upstreamwiring "github.com/Aether-v1/hcz/internal/bootstrap/upstreamapi"
	userauthwiring "github.com/Aether-v1/hcz/internal/bootstrap/userauth"
	walletbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/wallet"
	walletwithdrawalbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/walletwithdrawal"
	"github.com/Aether-v1/hcz/internal/cache"
	"github.com/Aether-v1/hcz/internal/config"
	"github.com/Aether-v1/hcz/internal/constants"
	"github.com/Aether-v1/hcz/internal/logger"
	apicredentialtransport "github.com/Aether-v1/hcz/internal/modules/apicredential/transport/http"
	auditlogtransport "github.com/Aether-v1/hcz/internal/modules/auditlog/transport/http"
	c2chttp "github.com/Aether-v1/hcz/internal/modules/c2c/transport/http"
	captchahttp "github.com/Aether-v1/hcz/internal/modules/captcha/transport/http"
	cardsecrettransport "github.com/Aether-v1/hcz/internal/modules/cardsecret/transport/http"
	carttransport "github.com/Aether-v1/hcz/internal/modules/cart/transport/http"
	categoryhttp "github.com/Aether-v1/hcz/internal/modules/catalog/category/transport/http"
	mappinghttp "github.com/Aether-v1/hcz/internal/modules/catalog/mapping/transport/http"
	producthttp "github.com/Aether-v1/hcz/internal/modules/catalog/product/transport/http"
	contenttransport "github.com/Aether-v1/hcz/internal/modules/content/transport/http"
	coupontransport "github.com/Aether-v1/hcz/internal/modules/coupon/transport/http"
	dashboardtransport "github.com/Aether-v1/hcz/internal/modules/dashboard/transport/http"
	giftcardtransport "github.com/Aether-v1/hcz/internal/modules/giftcard/transport/http"
	memberleveltransport "github.com/Aether-v1/hcz/internal/modules/memberlevel/transport/http"
	notificationtransport "github.com/Aether-v1/hcz/internal/modules/notification/transport/http"
	procurementtransport "github.com/Aether-v1/hcz/internal/modules/procurement/transport/http"
	promotiontransport "github.com/Aether-v1/hcz/internal/modules/promotion/transport/http"
	settingstransport "github.com/Aether-v1/hcz/internal/modules/settings/transport/http"
	sitemapbrand "github.com/Aether-v1/hcz/internal/modules/sitemap/infrastructure/settingsbrand"
	sitemaptransport "github.com/Aether-v1/hcz/internal/modules/sitemap/transport/http"
	supporttickethttp "github.com/Aether-v1/hcz/internal/modules/supportticket/transport/http"
	telegramchanneltransport "github.com/Aether-v1/hcz/internal/modules/telegram/channelbot/transport/http"
	usernotificationhttp "github.com/Aether-v1/hcz/internal/modules/usernotification/transport/http"
	"github.com/Aether-v1/hcz/internal/web"

	"github.com/gin-gonic/gin"
)

// SetupRouter 初始化路由。
func SetupRouter(cfg *config.Config, c *container.Container) *gin.Engine {
	log := logger.L
	if log == nil {
		log = logger.Init(cfg.Server.Mode, cfg.Log.ToLoggerOptions())
	}
	r := gin.New()
	if err := configureTrustedProxies(r, cfg.Server.TrustedProxies); err != nil {
		panic(fmt.Errorf("server.trusted_proxies 配置错误: %w", err))
	}
	captchaVerifier := captchahttp.NewVerifier(c.CaptchaService)

	// 初始化 Handler（按前台/后台分组）
	adminAuthHandlers := adminauthwiring.New(c)
	adminLoginHandler := adminAuthHandlers.Login
	admin2FAHandler := adminAuthHandlers.TwoFA
	adminUser2FAHandler := adminAuthHandlers.UserTwoFA
	adminUserHandler := adminuserwiring.NewHandler(c)
	adminAuthzHandler := adminauthzwiring.NewHandler(c)
	adminFulfillmentHandler := fulfillmentwiring.NewAdminHandler(c)
	orderHandlers := orderwiring.New(c)
	adminOrderHandler := orderHandlers.Admin
	adminOrderRefundHandler := orderHandlers.AdminRefund
	afterSaleHandler := orderHandlers.AfterSale
	userOrderHandler := orderHandlers.User
	guestOrderHandler := orderHandlers.Guest
	orderPreviewHandler := orderHandlers.Preview
	orderCreateHandler := orderHandlers.Create
	paymentHandlers := paymentwiring.New(c)
	paymentLatestHandler := paymentHandlers.Latest
	paymentWriteHandler := paymentHandlers.Write
	adminPaymentHandler := paymentHandlers.Admin
	adminPaymentChannelHandler := paymentHandlers.AdminChannel
	paymentWebhookHandler := paymentHandlers.Webhook
	paymentCallbackHandler := paymentHandlers.Callback
	publicConfigHandler := publicconfigwiring.NewHandler(c)
	userCartHandler := carttransport.NewUserHandler(c.CartService)
	channelHandler := channelwiring.NewHandler(c)
	upstreamHandler := upstreamwiring.NewHandler(c)
	publicContentHandler := contenttransport.NewPublicHandler(
		c.ContentPostService,
		c.ContentPostCategoryService,
		c.ContentBannerService,
	)
	publicCatalogHandler := catalogproductbootstrap.NewPublicHTTP(catalogproductbootstrap.PublicHTTPDependencies{
		Products:     c.ProductReadService,
		Hidden:       c.ResellerStore,
		Pricer:       c.ResellerPricingResolver,
		Promotions:   c.PromotionRepo,
		MemberLevels: c.MemberLevelService,
		Mappings:     c.ProductMappingRepo,
		SKUMappings:  c.SKUMappingRepo,
		RelatedPosts: c.ContentPostService,
	})
	publicCategoryHandler := categoryhttp.NewPublicHandler(c.CategoryService)
	adminContentHandler := contenttransport.NewAdminHandler(
		c.ContentPostService,
		c.ContentPostCategoryService,
		c.ContentBannerService,
		c.ContentMediaService,
	)
	adminDashboardHandler := dashboardtransport.NewAdminHandler(c.DashboardService)
	adminMemberLevelHandler := memberleveltransport.NewAdminHandler(c.MemberLevelService)
	publicMemberLevelHandler := memberleveltransport.NewPublicHandler(c.MemberLevelService)
	userAuthHandlers := userauthwiring.New(c)
	userProfileHandler := userAuthHandlers.Profile
	userEmailHandler := userAuthHandlers.Email
	userPasswordHandler := userAuthHandlers.Password
	userVerifyHandler := userAuthHandlers.Verify
	userLoginHandler := userAuthHandlers.Login
	user2FAHandler := userAuthHandlers.TwoFA
	userTelegramOIDCHandler := userAuthHandlers.TelegramOIDC
	userTelegramHandler := userAuthHandlers.Telegram
	userGoogleHandler := userAuthHandlers.Google
	walletHandlers := walletbootstrap.New(c)
	userWalletHandler := walletHandlers.User
	adminWalletHandler := walletHandlers.Admin
	channelWalletHandler := walletHandlers.Channel
	pointsHandlers := pointsbootstrap.New(c)
	userPointsHandler := pointsHandlers.User
	adminPointsHandler := pointsHandlers.Admin
	checkinHandlers := checkinbootstrap.New(c)
	userCheckinHandler := checkinHandlers.User
	adminCheckinHandler := checkinHandlers.Admin
	pointsmallHandlers := pointsmallbootstrap.New(c)
	userPointsmallHandler := pointsmallHandlers.User
	adminPointsmallHandler := pointsmallHandlers.Admin
	withdrawalHandlers := walletwithdrawalbootstrap.New(c)
	userWithdrawalHandler := withdrawalHandlers.User
	adminWithdrawalHandler := withdrawalHandlers.Admin
	c2cHandler := c2chttp.NewHandler(c.C2CService)
	adminC2CHandler := c2cbootstrap.NewAdminHandler(c)
	supportUserHandler := supporttickethttp.NewUserHandler(c.SupportTicketService)
	supportAdminHandler := supporttickethttp.NewAdminHandler(c.SupportTicketService)
	channelMemberLevelHandler := memberleveltransport.NewChannelHandler(c.MemberLevelService)
	adminApiCredentialHandler := apicredentialtransport.NewAdminHandler(c.ApiCredentialService)
	userApiCredentialHandler := apicredentialtransport.NewUserHandler(c.ApiCredentialService)
	adminAuditLogHandler := auditlogtransport.NewAdminHandler(c.AuthzAuditService, c.UserLoginLogService)
	adminCardSecretHandler := cardsecrettransport.NewAdminHandler(c.CardSecretService)
	adminCatalogCategoryHandler := categoryhttp.NewAdminCategoryHandler(c.CategoryService)
	adminCatalogProductHandler := producthttp.NewAdminProductHandler(
		c.ProductReadService,
		c.ProductWriteService,
		c.ProductAdminService,
		c.SettingService,
		c.ProductMappingRepo,
		c.SKUMappingRepo,
	)
	adminCatalogProductMappingHandler := mappinghttp.NewAdminHandler(c.ProductMappingService)
	userAuditLogHandler := auditlogtransport.NewUserHandler(c.UserLoginLogService)
	adminCouponHandler := coupontransport.NewAdminHandler(c.CouponAdminService)
	adminGiftCardHandler := giftcardtransport.NewAdminHandler(c.GiftCardService)
	userGiftCardHandler := giftcardtransport.NewUserHandler(c.GiftCardService, captchaVerifier)
	userNotificationHandler := usernotificationhttp.NewUserHandler(c.UserNotificationService)
	channelGiftCardHandler := giftcardtransport.NewChannelHandler(
		c.GiftCardService,
		channeluserwiring.NewSimpleProvisioner(c.UserAuthService),
	)
	channelAffiliateHandler := affiliatebootstrap.NewChannelHandler(c)
	channelTelegramBotHandler := telegramchanneltransport.NewChannelBotHandler(c.SettingService, c.ChannelClientService)
	adminSettingsHandler := settingstransport.NewAdminHandler(c.SettingService)
	adminPromotionHandler := promotiontransport.NewAdminHandler(c.PromotionAdminService)
	adminNotificationHandler := notificationtransport.NewAdminHandler(c.SettingService, c.NotificationLogService, c.NotificationService)
	adminProcurementHandler := procurementtransport.NewAdminHandler(c.ProcurementOrderService)
	resellerHandlers := resellerbootstrap.New(c)
	userResellerHandler := resellerHandlers.User
	userResellerProductSettingHandler := resellerHandlers.UserProductSetting
	userResellerFinanceHandler := resellerHandlers.UserFinance
	userResellerOrderHandler := resellerHandlers.UserOrder
	adminResellerManagementHandler := resellerHandlers.AdminManagement
	adminResellerProfileDetailHandler := resellerHandlers.AdminProfileDetail
	adminResellerSiteConfigHandler := resellerHandlers.AdminSiteConfig
	adminResellerProductSettingHandler := resellerHandlers.AdminProductSetting
	adminResellerOperationsHandler := resellerHandlers.AdminOperations
	adminResellerFinanceHandler := resellerHandlers.AdminFinance

	redisPrefix := strings.TrimSpace(cfg.Redis.Prefix)
	if redisPrefix == "" {
		redisPrefix = constants.RedisPrefixDefault
	}
	redisClient := cache.Client()
	loginRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:login", redisPrefix),
		WindowSeconds: cfg.Security.LoginRateLimit.WindowSeconds,
		MaxRequests:   cfg.Security.LoginRateLimit.MaxAttempts,
		BlockSeconds:  cfg.Security.LoginRateLimit.BlockSeconds,
		MessageKey:    "error.login_too_many",
	}
	adminLoginRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:admin_login", redisPrefix),
		WindowSeconds: cfg.Security.LoginRateLimit.WindowSeconds,
		MaxRequests:   cfg.Security.LoginRateLimit.MaxAttempts,
		BlockSeconds:  cfg.Security.LoginRateLimit.BlockSeconds,
		MessageKey:    "error.login_too_many",
	}
	guestReadRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:guest_orders:read", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   120,
		BlockSeconds:  60,
		MessageKey:    "error.rate_limited",
	}
	guestWriteRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:guest_orders:write", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   20,
		BlockSeconds:  300,
		MessageKey:    "error.rate_limited",
	}
	upstreamAPIRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:upstream_api", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   60,
		BlockSeconds:  30,
		MessageKey:    "error.rate_limited",
	}
	// 渠道 API：按 IP|渠道 Key 计数，Bot 单机高频调用给足余量
	channelAPIRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:channel_api", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   600,
		BlockSeconds:  30,
		MessageKey:    "error.rate_limited",
	}
	// 支付回调 / webhook / 上游回调：网关重试频率远低于此
	callbackRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:callback", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   120,
		BlockSeconds:  60,
		MessageKey:    "error.rate_limited",
	}
	// 礼品卡兑换：按用户+IP 计数
	giftCardRedeemRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:gift_card_redeem", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   10,
		BlockSeconds:  300,
		MessageKey:    "error.rate_limited",
	}
	// 注册：限制批量注册（10 分钟窗口内 5 次，超限封禁 15 分钟）
	registerRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:register", redisPrefix),
		WindowSeconds: 600,
		MaxRequests:   5,
		BlockSeconds:  900,
		MessageKey:    "error.rate_limited",
	}
	// 发送验证码：限制邮件轰炸（1 分钟窗口内 3 次，超限封禁 2 分钟）
	verifyRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:verify", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   3,
		BlockSeconds:  120,
		MessageKey:    "error.rate_limited",
	}
	// 忘记密码/重置提交：限制账号枚举与重置邮件轰炸（15 分钟窗口内 3 次，超限封禁 30 分钟）
	forgotRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:forgot", redisPrefix),
		WindowSeconds: 900,
		MaxRequests:   3,
		BlockSeconds:  1800,
		MessageKey:    "error.rate_limited",
	}
	// 工单：创建 5/小时，回复 3/10s，附件上传 10/分钟。
	supportCreateRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:support_create", redisPrefix),
		WindowSeconds: 3600,
		MaxRequests:   5,
		BlockSeconds:  0,
		MessageKey:    "error.rate_limited",
	}
	supportReplyRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:support_reply", redisPrefix),
		WindowSeconds: 10,
		MaxRequests:   3,
		BlockSeconds:  0,
		MessageKey:    "error.rate_limited",
	}
	supportAttachmentRule := middleware.RateLimitRule{
		Prefix:        fmt.Sprintf("%s:rate:support_attachment", redisPrefix),
		WindowSeconds: 60,
		MaxRequests:   10,
		BlockSeconds:  0,
		MessageKey:    "error.rate_limited",
	}

	// middleware.RequestIDMiddleware 必须前置于 middleware.RecoveryMiddleware：panic 日志与响应都依赖 request_id。
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.LoggerMiddleware(log))
	r.Use(middleware.CORSMiddleware(cfg.CORS))
	r.Use(middleware.CallbackRouteMiddleware(c.SettingService, paymentCallbackHandler, paymentWebhookHandler, upstreamHandler))

	// 静态文件服务（上传的图片）必须放在前面。
	// SVG 强制下载并禁止脚本：即使上传校验被绕过，直接打开 /uploads/x.svg 也不会在站点源下执行脚本；
	// <img src> 引用不受 Content-Disposition 影响，正常显示。
	r.Group("/uploads", func(c *gin.Context) {
		// 私有场景：工单附件属于用户/客服敏感材料，必须经带归属校验的鉴权端点
		// （GET /api/v1/support/attachments/:id）下载，禁止通过公开静态路径直接访问。
		if strings.HasPrefix(c.Request.URL.Path, "/uploads/support_ticket/") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if strings.EqualFold(path.Ext(c.Request.URL.Path), ".svg") {
			c.Header("Content-Disposition", "attachment")
			c.Header("Content-Security-Policy", "sandbox; script-src 'none'")
			c.Header("X-Content-Type-Options", "nosniff")
		}
	}).Static("/", "./uploads")

	// SEO 资源（动态生成）。
	sitemaptransport.RegisterRoutes(r, sitemaptransport.NewHandler(c.SitemapService, sitemapbrand.New(c.SettingService)))

	apiV1 := r.Group("/api/v1")
	registerStorefrontRoutes(apiV1, cfg, c, publicContentHandler, publicCatalogHandler, publicCategoryHandler, userResellerHandler, userResellerProductSettingHandler, userResellerFinanceHandler, userResellerOrderHandler, userApiCredentialHandler, userAuditLogHandler, userGiftCardHandler, publicMemberLevelHandler, userProfileHandler, userEmailHandler, userPasswordHandler, userVerifyHandler, userTelegramOIDCHandler, userTelegramHandler, userGoogleHandler, userLoginHandler, user2FAHandler, publicConfigHandler, userCartHandler, userOrderHandler, afterSaleHandler, guestOrderHandler, orderPreviewHandler, orderCreateHandler, paymentLatestHandler, paymentWriteHandler, userWalletHandler, userPointsHandler, userCheckinHandler, userPointsmallHandler, userWithdrawalHandler, userNotificationHandler, c2cHandler, supportUserHandler, redisClient, loginRule, guestReadRule, guestWriteRule, giftCardRedeemRule, registerRule, verifyRule, forgotRule, supportCreateRule, supportReplyRule, supportAttachmentRule)
	registerUpstreamRoutes(apiV1, c, upstreamHandler, redisClient, upstreamAPIRule, callbackRule)
	registerChannelRoutes(apiV1, c, channelHandler, channelMemberLevelHandler, channelGiftCardHandler, channelAffiliateHandler, channelTelegramBotHandler, channelWalletHandler, redisClient, channelAPIRule)
	registerPaymentCallbackRoutes(apiV1, paymentCallbackHandler, paymentWebhookHandler, redisClient, callbackRule)
	registerAdminRoutes(r, apiV1, cfg, c, adminLoginHandler, admin2FAHandler, adminUser2FAHandler, adminUserHandler, adminAuthzHandler, adminFulfillmentHandler, adminOrderHandler, adminOrderRefundHandler, afterSaleHandler, adminContentHandler, adminDashboardHandler, adminMemberLevelHandler, adminApiCredentialHandler, adminAuditLogHandler, adminCardSecretHandler, adminCatalogCategoryHandler, adminCatalogProductHandler, adminCatalogProductMappingHandler, adminCouponHandler, adminGiftCardHandler, adminPromotionHandler, adminNotificationHandler, adminProcurementHandler, adminResellerManagementHandler, adminResellerProfileDetailHandler, adminResellerSiteConfigHandler, adminResellerProductSettingHandler, adminResellerOperationsHandler, adminResellerFinanceHandler, adminSettingsHandler, adminWalletHandler, adminPointsHandler, adminCheckinHandler, adminPointsmallHandler, adminWithdrawalHandler, adminPaymentHandler, adminPaymentChannelHandler, adminC2CHandler, supportAdminHandler, c.SiteBuilderAdminHandler, redisClient, adminLoginRule)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 嵌入式前端资源（仅在 -tags fullstack 构建时生效）
	if web.Enabled() {
		// cmd/server 已在数据库初始化之前校验过一次；这里保留是为了兜住其它调用方
		// （测试、未来的其它入口）——重复校验没有代价，漏校验会让 RegisterAdmin panic。
		if err := web.ValidateAdminPath(cfg.Web.AdminPath); err != nil {
			log.Sugar().Fatalf("web.admin_path 配置错误: %v", err)
		}
		if err := web.RegisterAdmin(r, cfg.Web.AdminPath, web.AdminFS()); err != nil {
			log.Sugar().Fatalf("注册 admin SPA 失败: %v", err)
		}
		if err := web.RegisterUser(r, web.UserFS()); err != nil {
			log.Sugar().Fatalf("注册 user SPA 失败: %v", err)
		}
	}

	return r
}

func configureTrustedProxies(engine *gin.Engine, trustedProxies []string) error {
	if engine == nil {
		return fmt.Errorf("gin engine is nil")
	}
	for _, rawProxy := range trustedProxies {
		proxy := strings.TrimSpace(rawProxy)
		if proxy == "" {
			return fmt.Errorf("trusted proxy cannot be empty")
		}
		if net.ParseIP(proxy) != nil {
			continue
		}
		_, network, err := net.ParseCIDR(proxy)
		if err != nil {
			return fmt.Errorf("invalid trusted proxy %q: %w", proxy, err)
		}
		ones, bits := network.Mask.Size()
		if ones == 0 && (bits == 32 || bits == 128) {
			return fmt.Errorf("trusted proxy %q would trust every address", proxy)
		}
	}
	return engine.SetTrustedProxies(trustedProxies)
}

type adminPermissionCatalogItem struct {
	Module     string `json:"module"`
	Method     string `json:"method"`
	Object     string `json:"object"`
	Permission string `json:"permission"`
}

func buildAdminPermissionCatalog(engine *gin.Engine) []adminPermissionCatalogItem {
	if engine == nil {
		return []adminPermissionCatalogItem{}
	}

	routes := engine.Routes()
	seen := make(map[string]struct{}, len(routes))
	items := make([]adminPermissionCatalogItem, 0, len(routes))

	for _, item := range routes {
		method := strings.ToUpper(strings.TrimSpace(item.Method))
		if method == "" || method == http.MethodOptions || method == http.MethodHead {
			continue
		}
		if !strings.HasPrefix(item.Path, "/api/v1/admin/") {
			continue
		}
		if item.Path == "/api/v1/admin/login" {
			continue
		}
		if item.Path == "/api/v1/admin/login/verify-2fa" {
			continue
		}
		object := authz.NormalizeObject(item.Path)
		permission := method + ":" + object
		if _, exists := seen[permission]; exists {
			continue
		}
		seen[permission] = struct{}{}
		items = append(items, adminPermissionCatalogItem{
			Module:     deriveAdminPermissionModule(object),
			Method:     method,
			Object:     object,
			Permission: permission,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Module == items[j].Module {
			if items[i].Object == items[j].Object {
				return items[i].Method < items[j].Method
			}
			return items[i].Object < items[j].Object
		}
		return items[i].Module < items[j].Module
	})

	return items
}

func deriveAdminPermissionModule(object string) string {
	normalized := strings.TrimPrefix(strings.TrimSpace(object), "/")
	if normalized == "" {
		return "system"
	}
	segments := strings.Split(normalized, "/")
	if len(segments) <= 1 {
		return segments[0]
	}
	if segments[0] != "admin" {
		return segments[0]
	}
	if segments[1] == "authz" {
		return "authz"
	}
	return segments[1]
}
