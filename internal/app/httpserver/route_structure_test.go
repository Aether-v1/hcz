package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Aether-v1/hcz/internal/app/httpserver/middleware"
	"github.com/gin-gonic/gin"
)

func TestSetupRouterDelegatesRouteDomains(t *testing.T) {
	routerDirectory := currentRouterDirectory(t)
	source := readRouterSource(t, filepath.Join(routerDirectory, "router.go"))

	for _, registration := range []string{
		"registerStorefrontRoutes(",
		"registerUpstreamRoutes(",
		"registerChannelRoutes(",
		"registerPaymentCallbackRoutes(",
		"registerAdminRoutes(",
	} {
		if !strings.Contains(source, registration) {
			t.Errorf("SetupRouter must delegate through %s", registration)
		}
	}

	for _, inlineGroup := range []string{
		`apiV1.Group("/upstream")`,
		`apiV1.Group("/channel")`,
		`apiV1.Group("/admin")`,
	} {
		if strings.Contains(source, inlineGroup) {
			t.Errorf("router.go must not retain inline domain group %s", inlineGroup)
		}
	}
}

func TestRouteDomainFilesPreserveTrustBoundaries(t *testing.T) {
	routerDirectory := currentRouterDirectory(t)
	tests := []struct {
		file     string
		required []string
	}{
		{
			file: "routes_storefront.go",
			required: []string{
				`storefront.Use(middleware.ResellerTenantMiddleware(`,
				`publicconfigtransport.RegisterPublicRoutes(public, publicConfigHandler)`,
				`authedPublic := storefront.Group("/public", middleware.UserJWTAuthMiddleware(`,
				`producthttp.RegisterPublicRoutes(authedPublic, publicCatalogHandler)`,
				`categoryhttp.RegisterPublicRoutes(authedPublic, publicCategoryHandler)`,
				`contenttransport.RegisterPublicRoutes(authedPublic, publicContentHandler)`,
				`captchatransport.RegisterPublicRoutes(public,`,
				`affiliatetransport.RegisterPublicRoutes(public, affiliateHandler)`,
				`affiliatetransport.RegisterUserRoutes(user, affiliateHandler)`,
				`user.Use(middleware.UserJWTAuthMiddleware(`,
				`resellerConsole.Use(middleware.RequireMainTenantForResellerConsole())`,
				`resellertransport.RegisterUserConsoleRoutes(resellerConsole, userResellerHandler)`,
				`resellertransport.RegisterUserProductSettingRoutes(resellerConsole, userResellerProductSettingHandler)`,
				`resellertransport.RegisterUserFinanceRoutes(resellerConsole, userResellerFinanceHandler)`,
				`resellertransport.RegisterUserOrderRoutes(resellerConsole, userResellerOrderHandler)`,
				`apicredentialtransport.RegisterUserRoutes(user, userApiCredentialHandler)`,
				`auditlogtransport.RegisterUserRoutes(user, userAuditLogHandler)`,
				`giftcardtransport.RegisterUserRoutes(giftCardRedeem, userGiftCardHandler)`,
				`wallettransport.RegisterUserRoutes(user, userWalletHandler)`,
				`userauthtransport.RegisterUserProfileRoutes(user, userProfileHandler)`,
				`userauthtransport.RegisterUserEmailRoutes(user, userEmailHandler)`,
				`userauthtransport.RegisterUserVerifyAuthRoutes(auth, userVerifyHandler)`,
				`userauthtransport.RegisterUserRegisterAuthRoutes(auth, userLoginHandler)`,
				`userauthtransport.RegisterUserLoginAuthRoutes(auth, userLoginHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIPAndJSONField("email")))`,
				`userauthtransport.RegisterUser2FAAuthRoutes(auth, user2FAHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))`,
				`userauthtransport.RegisterUser2FARoutes(user, user2FAHandler)`,
				`userauthtransport.RegisterUserTelegramAuthRoutes(auth, userTelegramHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))`,
				`userauthtransport.RegisterUserTelegramRoutes(user, userTelegramHandler)`,
				`userauthtransport.RegisterUserTelegramOIDCAuthRoutes(auth, userTelegramOIDCHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))`,
				`userauthtransport.RegisterUserTelegramOIDCRoutes(user, userTelegramOIDCHandler)`,
				`userauthtransport.RegisterUserGoogleAuthRoutes(auth, userGoogleHandler, middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByIP))`,
				`userauthtransport.RegisterUserGoogleRoutes(`,
				`middleware.RateLimitMiddleware(redisClient, loginRule, middleware.KeyByUserIDAndIP)`,
				`userauthtransport.RegisterUserPasswordAuthRoutes(auth, userPasswordHandler)`,
				`userauthtransport.RegisterUserPasswordRoutes(user, userPasswordHandler)`,
				`carttransport.RegisterUserRoutes(user, userCartHandler)`,
				`ordertransport.RegisterUserReadRoutes(user, userOrderHandler)`,
				`ordertransport.RegisterUserCancelRoute(user, userOrderHandler)`,
				`ordertransport.RegisterUserPreviewRoute(user, orderPreviewHandler)`,
				`ordertransport.RegisterUserCreateRoute(user, orderCreateHandler)`,
				`ordertransport.RegisterUserCreateAndPayRoute(user, orderCreateHandler)`,
				`ordertransport.RegisterUserPaymentChannelsRoute(user, userOrderHandler)`,
				`paymenttransport.RegisterUserWriteRoutes(user, paymentWriteHandler)`,
				`paymenttransport.RegisterUserLatestRoute(user, paymentLatestHandler)`,
				`paymenttransport.RegisterWebhookRoutes(callbacks, webhookHandler)`,
			},
		},
		{
			file: "routes_upstream.go",
			required: []string{
				`upstreamAPI.Use(middleware.RateLimitMiddleware(`,
				`upstreamAPI.Use(middleware.UpstreamAPIAuthMiddleware(`,
				`apiV1.POST("/upstream/callback",`,
			},
		},
		{
			file: "routes_channel.go",
			required: []string{
				`channelAPI.Use(middleware.ChannelAPIAuthMiddleware(`,
				`telegramtransport.RegisterChannelBotRoutes(channelAPI, channelTelegramBotHandler)`,
				`affiliatetransport.RegisterChannelRoutes(channelAPI, channelAffiliateHandler)`,
				`memberleveltransport.RegisterChannelRoutes(channelAPI, channelMemberLevelHandler)`,
				`wallettransport.RegisterChannelRoutes(channelAPI, channelWalletHandler)`,
				`giftcardtransport.RegisterChannelRoutes(channelAPI, channelGiftCardHandler)`,
			},
		},
		{
			file: "routes_admin.go",
			required: []string{
				`adminauthtransport.RegisterAdminLoginAuthRoutes(admin, adminLoginHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))`,
				`adminauthtransport.RegisterAdmin2FAAuthRoutes(admin, admin2FAHandler, middleware.RateLimitMiddleware(redisClient, adminLoginRule, middleware.KeyByIP))`,
				`adminauthtransport.RegisterAdminPasswordRoutes(authorized, adminLoginHandler)`,
				`adminauthtransport.RegisterAdmin2FARoutes(authorized, admin2FAHandler)`,
				`adminauthtransport.RegisterAdminUser2FARoutes(authorized, adminUser2FAHandler)`,
				`adminusertransport.RegisterAdminRoutes(authorized, adminUserHandler)`,
				`adminauthztransport.RegisterAdminRoutes(authorized, adminAuthzHandler)`,
				`ordertransport.RegisterAdminRoutes(authorized, adminOrderHandler)`,
				`ordertransport.RegisterAdminRefundWriteRoutes(authorized, adminOrderRefundHandler)`,
				`ordertransport.RegisterAdminRefundRoutes(authorized, adminOrderRefundHandler)`,
				`fulfillmenttransport.RegisterAdminRoutes(authorized, adminFulfillmentHandler)`,
				`authorized := admin.Use(middleware.JWTAuthMiddleware(`,
				`paymentProtected := admin.Group("", middleware.PaymentComplianceRequired(`,
				`compliancetransport.RegisterAdminRoutes(authorized,`,
				`systemtransport.RegisterAdminRoutes(authorized,`,
				`adproxytransport.RegisterAdminRoutes(authorized,`,
				`contenttransport.RegisterAdminRoutes(authorized, adminContentHandler)`,
				`dashboardtransport.RegisterAdminRoutes(authorized, adminDashboardHandler)`,
				`memberleveltransport.RegisterAdminRoutes(authorized, adminMemberLevelHandler)`,
				`apicredentialtransport.RegisterAdminRoutes(authorized, adminApiCredentialHandler)`,
				`auditlogtransport.RegisterAdminRoutes(authorized, adminAuditLogHandler)`,
				`cardsecrettransport.RegisterAdminRoutes(authorized, adminCardSecretHandler)`,
				`giftcardtransport.RegisterAdminRoutes(authorized, adminGiftCardHandler)`,
				`settingstransport.RegisterAdminRoutes(authorized, adminSettingsHandler)`,
				`settingstransport.RegisterAdminSMTPRoutes(authorized,`,
				`settingstransport.RegisterAdminCaptchaRoutes(authorized,`,
				`settingstransport.RegisterAdminTelegramAuthRoutes(authorized,`,
				`settingstransport.RegisterAdminGoogleAuthRoutes(authorized,`,
				`settingstransport.RegisterAdminOrderEmailTemplateRoutes(authorized,`,
				`settingstransport.RegisterAdminAffiliateRoutes(authorized,`,
				`settingstransport.RegisterAdminTelegramBotRoutes(authorized,`,
				`uploadtransport.RegisterAdminRoutes(authorized,`,
				`broadcasthttp.RegisterAdminRoutes(authorized,`,
				`channelclienthttp.RegisterAdminRoutes(authorized,`,
				`siteconnectiontransport.RegisterAdminRoutes(authorized,`,
				`affiliatetransport.RegisterAdminRoutes(authorized, adminAffiliateHandler)`,
				`affiliatetransport.RegisterAdminFinanceRoutes(paymentProtected, adminAffiliateHandler)`,
				`producthttp.RegisterAdminRoutes(authorized, adminCatalogProductHandler)`,
				`categoryhttp.RegisterAdminRoutes(authorized, adminCatalogCategoryHandler)`,
				`mappinghttp.RegisterAdminRoutes(authorized, adminCatalogProductMappingHandler)`,
				`coupontransport.RegisterAdminRoutes(authorized, adminCouponHandler)`,
				`promotiontransport.RegisterAdminRoutes(authorized, adminPromotionHandler)`,
				`notificationtransport.RegisterAdminRoutes(authorized, adminNotificationHandler)`,
				`procurementtransport.RegisterAdminRoutes(authorized, adminProcurementHandler)`,
				`resellertransport.RegisterOperationsOverviewRoutes(authorized, adminResellerOperationsHandler)`,
				`resellertransport.RegisterManagementRoutes(authorized, adminResellerManagementHandler)`,
				`resellertransport.RegisterProfileDetailRoutes(authorized, adminResellerProfileDetailHandler)`,
				`resellertransport.RegisterSiteConfigRoutes(authorized, adminResellerSiteConfigHandler)`,
				`resellertransport.RegisterProductSettingRoutes(authorized, adminResellerProductSettingHandler)`,
				`resellertransport.RegisterOperationsFinanceRoutes(paymentProtected, adminResellerOperationsHandler)`,
				`resellertransport.RegisterFinanceRoutes(paymentProtected, adminResellerFinanceHandler)`,
				`paymenttransport.RegisterAdminChannelRoutes(paymentProtected, adminPaymentChannelHandler)`,
				`paymenttransport.RegisterAdminRoutes(paymentProtected, adminPaymentHandler)`,
				`wallettransport.RegisterAdminRoutes(paymentProtected, adminWalletHandler)`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			source := readRouterSource(t, filepath.Join(routerDirectory, test.file))
			for _, required := range test.required {
				if !strings.Contains(source, required) {
					t.Errorf("%s must preserve trust-boundary statement %q", test.file, required)
				}
			}
			if test.file == "routes_storefront.go" {
				// HCZ No-Guest-Purchase: 游客下单/查单/支付/下载路由组不得再注册
				for _, forbidden := range []string{
					`Group("/guest")`,
					`RegisterGuestPreviewRoute(`,
					`RegisterGuestReadRoutes(`,
					`RegisterGuestCreateRoute(`,
					`RegisterGuestCreateAndPayRoute(`,
					`RegisterGuestWriteRoutes(`,
					`RegisterGuestLatestRoute(`,
				} {
					if strings.Contains(source, forbidden) {
						t.Errorf("%s must NOT register guest route %q (HCZ no-guest-purchase)", test.file, forbidden)
					}
				}
			}
		})
	}
}

func currentRouterDirectory(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve router test filename")
	}
	return filepath.Dir(thisFile)
}

func readRouterSource(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read router source %s: %v", path, err)
	}
	return string(raw)
}

// TestAuthenticatedOnlyBoundaryBehavior 校验 HCZ 登录收口的运行时行为：
//   - /public/config 等认证链启动接口：无 token 仍可访问（200）
//   - /public/products 等业务读接口：已挂 UserJWTAuthMiddleware，无 token 必须 401（fail-closed）
//   - /guest/* 游客下单/查单路由不再注册：访问即 404
//
// 与上面的源码结构断言配合，闭合「路由确实挂在 JWT 组」+「JWT 组确实拒绝无 token」。
func TestAuthenticatedOnlyBoundaryBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()

	// 无 auth 的公开组（等价 routes_storefront.go 里的 public：config/captcha/affiliate）
	public := r.Group("/public")
	public.GET("/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status_code": 0, "data": gin.H{"languages": []string{"zh-CN"}}})
	})

	// 挂 JWT 的业务读组（等价 authedPublic：products/categories/content/member-levels）
	authedPublic := r.Group("/public", middleware.UserJWTAuthMiddleware("test-secret-key", nil))
	authedPublic.GET("/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status_code": 0, "data": "products"})
	})

	do := func(method, path string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		r.ServeHTTP(w, req)
		// UserJWTAuthMiddleware 走统一响应包：HTTP 200 包体里 status_code=401
		var body struct {
			StatusCode int `json:"status_code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code == http.StatusOK && body.StatusCode != 0 {
			return body.StatusCode
		}
		return w.Code
	}

	// 1) 认证链启动配置：无 token 必须可用
	if got := do(http.MethodGet, "/public/config"); got != http.StatusOK {
		t.Fatalf("/public/config must stay public (200) for auth boot, got %d", got)
	}

	// 2) 业务读接口：无 token 必须被拒（401），不能只靠前端隐藏
	if got := do(http.MethodGet, "/public/products"); got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET /public/products must be rejected with 401, got %d", got)
	}

	// 3) 游客路由组已停用：直接访问 /guest/* 必须无路由（404）
	if got := do(http.MethodPost, "/guest/orders"); got != http.StatusNotFound {
		t.Fatalf("POST /guest/orders must be unregistered (404), got %d", got)
	}
	if got := do(http.MethodGet, "/guest/orders/whatever"); got != http.StatusNotFound {
		t.Fatalf("GET /guest/orders/:order_no must be unregistered (404), got %d", got)
	}
}
