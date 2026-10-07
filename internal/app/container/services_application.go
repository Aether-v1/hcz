package container

import (
	"context"

	adproxyapp "github.com/Aether-v1/hcz/internal/modules/adproxy/application"
	adproxygateway "github.com/Aether-v1/hcz/internal/modules/adproxy/infrastructure/adgateway"
	categoryapp "github.com/Aether-v1/hcz/internal/modules/catalog/category/application"

	"github.com/Aether-v1/hcz/internal/crypto"
	"github.com/Aether-v1/hcz/internal/logger"
	c2capp "github.com/Aether-v1/hcz/internal/modules/c2c/application"
	cardsecretapp "github.com/Aether-v1/hcz/internal/modules/cardsecret/application"
	cartapp "github.com/Aether-v1/hcz/internal/modules/cart/application"
	checkinapp "github.com/Aether-v1/hcz/internal/modules/checkin/application"
	checkincontract "github.com/Aether-v1/hcz/internal/modules/checkin/contract"
	contentapp "github.com/Aether-v1/hcz/internal/modules/content/application"
	"github.com/Aether-v1/hcz/internal/modules/content/infrastructure/gormstore"
	couponapp "github.com/Aether-v1/hcz/internal/modules/coupon/application"
	fulfillmentapp "github.com/Aether-v1/hcz/internal/modules/fulfillment/application"
	fulfillmentqueue "github.com/Aether-v1/hcz/internal/modules/fulfillment/infrastructure/queueadapter"
	giftcardapp "github.com/Aether-v1/hcz/internal/modules/giftcard/application"
	giftcardsettingscurrency "github.com/Aether-v1/hcz/internal/modules/giftcard/infrastructure/settingscurrency"
	memberlevelapp "github.com/Aether-v1/hcz/internal/modules/memberlevel/application"
	orderapp "github.com/Aether-v1/hcz/internal/modules/order/application"
	"github.com/Aether-v1/hcz/internal/modules/order/application/aftersale"
	orderrefund "github.com/Aether-v1/hcz/internal/modules/order/application/refund"
	orderqueue "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/queueadapter"
	orderriskapp "github.com/Aether-v1/hcz/internal/modules/orderrisk/application"
	orderrisklimiter "github.com/Aether-v1/hcz/internal/modules/orderrisk/infrastructure/redislimiter"
	pointsapp "github.com/Aether-v1/hcz/internal/modules/points/application"
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsmallapp "github.com/Aether-v1/hcz/internal/modules/pointsmall/application"
	promotionapp "github.com/Aether-v1/hcz/internal/modules/promotion/application"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	sitebuilderapp "github.com/Aether-v1/hcz/internal/modules/sitebuilder/application"
	sitebuilderhttp "github.com/Aether-v1/hcz/internal/modules/sitebuilder/transport/http"
	sitemapapp "github.com/Aether-v1/hcz/internal/modules/sitemap/application"
	sitemapcontract "github.com/Aether-v1/hcz/internal/modules/sitemap/contract"
	sitemapcache "github.com/Aether-v1/hcz/internal/modules/sitemap/infrastructure/cacheadapter"
	sitemapcatalog "github.com/Aether-v1/hcz/internal/modules/sitemap/infrastructure/catalogreader"
	supportapp "github.com/Aether-v1/hcz/internal/modules/supportticket/application"
	supportlocalfile "github.com/Aether-v1/hcz/internal/modules/supportticket/infrastructure/localfile"
	uploadapp "github.com/Aether-v1/hcz/internal/modules/upload/application"
	uploadlocal "github.com/Aether-v1/hcz/internal/modules/upload/infrastructure/localstore"
	usernotificationapp "github.com/Aether-v1/hcz/internal/modules/usernotification/application"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	withdrawalapp "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/application"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
	giftcardredeemgormuow "github.com/Aether-v1/hcz/internal/workflows/giftcardredeem/infrastructure/gormuow"
)

// initApplicationServices 装配内容、购物车、订单、履约和营销用例。
func (c *Container) initApplicationServices() {
	c.AdProxyService = adproxyapp.NewService(adproxygateway.New())
	postStore := gormstore.NewPostStore(gormdb.DB)
	postCategoryStore := gormstore.NewPostCategoryStore(gormdb.DB)
	c.ContentPostService = contentapp.NewPostService(
		postStore,
		postStore,
		postCategoryStore,
		contentapp.SystemClock{},
	)
	c.ContentPostCategoryService = contentapp.NewPostCategoryService(postCategoryStore)
	c.CategoryService = categoryapp.NewService(c.CategoryRepo)
	sitemapService, err := sitemapapp.NewService(
		sitemapcatalog.New(c.ProductRepo, c.CategoryRepo),
		sitemapcontract.PublishedPostReaderFunc(func(ctx context.Context, limit int) ([]sitemapcontract.PublishedPost, error) {
			posts, _, listErr := c.ContentPostService.ListPublic(ctx, contentapp.PublicPostQuery{
				Page:     1,
				PageSize: limit,
			})
			if listErr != nil {
				return nil, listErr
			}
			result := make([]sitemapcontract.PublishedPost, 0, len(posts))
			for _, post := range posts {
				result = append(result, sitemapcontract.PublishedPost{
					Slug:        post.Slug,
					CreatedAt:   post.CreatedAt,
					PublishedAt: post.PublishedAt,
				})
			}
			return result, nil
		}),
		sitemapcache.New(),
	)
	if err != nil {
		logger.Errorw("provider_init_sitemap_failed", "error", err)
		panic(err)
	}
	c.SitemapService = sitemapService
	c.CartService = cartapp.NewService(c.CartRepo, c.ProductRepo, c.ProductSKURepo, c.PromotionRepo, c.SettingService)
	c.WalletService = walletapp.NewService(walletapp.Options{
		Repository: c.WalletRepo, Transactions: c.WalletRepo,
	})
	c.PointsService = pointsapp.NewService(pointsapp.Options{
		Repository: c.PointsRepo, Transactions: c.PointsRepo,
		// 运营统计（P4）直接聚合既有事实表；业务日边界复用签到域唯一时钟实现，
		// 积分侧不新建第二套时区/累计口径。
		Stats: pointscontract.StatsPorts{
			Ledger:           c.PointsRepo,
			Checkins:         c.CheckinRepo,
			Exchanges:        c.PointsmallRepo,
			BusinessDayRange: checkinapp.BusinessDayRange,
		},
	})
	c.CheckinService = checkinapp.NewService(checkinapp.Options{
		Repository: c.CheckinRepo,
		UnitOfWork: c.CheckinRepo,
		Config:     checkinSettingsAdapter{settings: c.SettingService},
		Clock:      checkinapp.SystemClock{},
		Points:     c.PointsService,
	})
	c.PointsmallService = pointsmallapp.NewService(pointsmallapp.Options{
		Repository: c.PointsmallRepo,
		UnitOfWork: c.PointsmallRepo,
		Points:     c.PointsService,
	})
	// Affiliate 提现出金需要调用钱包服务真实入账。
	c.AffiliateService.SetWalletService(c.WalletService, c.WalletRepo)
	c.WithdrawalService = withdrawalapp.NewService(withdrawalapp.Options{
		Repository: c.WithdrawalRepo,
		UnitOfWork: c.WithdrawalRepo,
		TOTP:       c.UserTOTPService,
		Config:     c.SettingService,
		Notifier:   c.NotificationService,
		Users:      c.UserStore,
	})
	c.C2CService = c2capp.NewService(c2capp.Options{
		Repository:    c.C2CRepo,
		UnitOfWork:    c.C2CRepo,
		WalletService: c.WalletService,
		Users:         c.UserStore,
		UserAdmin:     c.UserStore,
		Config:        c.SettingService,
		Scheduler:     c.QueueClient,
		Notifier:      c.NotificationService,
		Audit:         c2cArbitrationAuditWriter{audit: c.AuthzAuditService},
		// 收款方式敏感字段加密密钥（与 TOTP 同一派生密钥）。
		EncKey: crypto.DeriveKey(c.Config.App.SecretKey),
		// 用户端 Step-Up：2FA 走 TOTP，未开 2FA 走密码。
		TOTP:               c.UserTOTPService,
		PaymentMethodAudit: c2cPaymentMethodAuditWriter{audit: c.AuthzAuditService},
	})
	c.OrderRefundService = orderrefund.New(
		c.OrderStore,
		c.UserStore,
		c.AffiliateService,
		c.SettingService,
		c.WalletService,
		c.PaymentStore,
		c.PointsService,
	)
	// P1 after-sale：与订单共享 OrderStore 事务，退款走 WalletRefunderAdapter→AdminRefundToWalletInTx。
	c.AfterSaleService = aftersale.NewService(
		c.OrderStore,
		aftersale.NewWalletRefunderAdapter(c.OrderRefundService),
	)
	c.MemberLevelService = memberlevelapp.NewService(c.MemberLevelRepo, c.MemberLevelPriceRepo, c.MemberLevelUserRepo)
	c.OrderRiskControlService = orderriskapp.NewService(orderriskapp.Options{
		Settings:    c.SettingService,
		RateLimiter: orderrisklimiter.New(),
	})
	orderQueue := orderqueue.New(c.QueueClient)
	c.OrderService = orderapp.NewOrderService(orderapp.OrderServiceOptions{
		OrderStore:              c.OrderStore,
		UserStore:               c.UserStore,
		ProductStore:            c.ProductRepo,
		ProductSKUStore:         c.ProductSKURepo,
		CouponStore:             c.CouponRepo,
		CouponUsageStore:        c.CouponUsageRepo,
		PromotionRepo:           c.PromotionRepo,
		Queue:                   orderQueue,
		SettingService:          c.SettingService,
		DefaultEmailConfig:      c.Config.Email,
		WalletService:           c.WalletService,
		AffiliateService:        c.AffiliateService,
		PointsService:           c.PointsService,
		MemberLevelService:      c.MemberLevelService,
		ResellerPricingResolver: c.ResellerPricingResolver,
		ResellerAccounting:      c.ResellerAccountingLedger,
		RiskControlService:      c.OrderRiskControlService,
		ExpireMinutes:           c.Config.Order.PaymentExpireMinutes,
	})
	c.FulfillmentService = fulfillmentapp.New(fulfillmentapp.Options{
		OrderStore:            c.OrderStore,
		FulfillmentStore:      c.FulfillmentStore,
		OrderQueue:            orderQueue,
		BotNotifier:           fulfillmentqueue.NewBotNotifier(c.QueueClient),
		SettingService:        c.SettingService,
		DefaultEmailConfig:    c.Config.Email,
		ExternalIdentityStore: c.ExternalIdentityStore,
		OrderCompletion:       c.OrderService,
	})
	c.CardSecretService = cardsecretapp.NewService(cardsecretapp.ServiceOptions{
		Secrets:      c.CardSecretRepo,
		Batches:      c.CardSecretBatchRepo,
		Transactions: c.CardSecretRepo,
		Products:     c.ProductRepo,
		ProductSKUs:  c.ProductSKURepo,
	})
	c.GiftCardService = giftcardapp.NewService(giftcardapp.Options{
		Repo:     c.GiftCardRepo,
		Users:    c.UserStore,
		Currency: giftcardsettingscurrency.New(c.SettingService),
		Redeemer: giftcardredeemgormuow.New(c.GiftCardRepo, c.WalletService),
	})
	c.CouponAdminService = couponapp.NewAdminService(c.CouponRepo)
	c.PromotionAdminService = promotionapp.NewAdminService(c.PromotionRepo)
	c.UserNotificationService = usernotificationapp.NewService(c.UserNotificationRepo)

	// 工单系统：独立的严格上传策略（10MB，仅图片/PDF/文本），私有下载。
	ticketUploader := uploadapp.NewService(uploadapp.Policy{
		MaxSize:           10 * 1024 * 1024,
		AllowedTypes:      []string{"image/jpeg", "image/png", "image/webp", "application/pdf", "text/plain"},
		AllowedExtensions: []string{".jpg", ".jpeg", ".png", ".webp", ".pdf", ".txt", ".log"},
	}, uploadlocal.New("uploads"))
	ticketFiler := supportlocalfile.New("uploads")
	c.SupportTicketService = supportapp.NewService(supportapp.Options{
		Repository: c.SupportTicketRepo,
		UnitOfWork: c.SupportTicketRepo,
		Users: &supportUserLookup{getByID: func(userID uint) (uint, string, string, error) {
			u, err := c.UserStore.GetByID(userID)
			if err != nil || u == nil {
				return userID, "", "", err
			}
			return u.ID, u.Email, u.DisplayName, nil
		}},
		Admins: &supportAdminLookup{getAdminByID: func(adminID uint) (uint, string, error) {
			a, err := c.AdminStore.GetByID(adminID)
			if err != nil || a == nil {
				return adminID, "", err
			}
			return a.ID, a.Username, nil
		}},
		Notifier: &supportNotifierAdapter{creator: c.UserNotificationService},
		Uploader: ticketUploader,
		Filer:    ticketFiler,
	})
	c.ContentBannerService = contentapp.NewBannerService(
		gormstore.NewBannerStore(gormdb.DB),
		contentapp.SystemClock{},
	)

	// 站点装修（sitebuilder）：首页入口 / 热门推荐分类 / 发现页区块 / 品牌 / 审计。
	c.SiteHomeEntryService = sitebuilderapp.NewHomeEntryService(c.SiteHomeEntryRepo)
	c.SiteHomeEntryService.SetProductLookup(c.ProductRepo)
	c.SiteHomeFeaturedCategoryService = sitebuilderapp.NewHomeFeaturedCategoryService(c.SiteHomeFeaturedCategoryRepo)
	c.SiteHomeFeaturedCategoryService.SetCategoryLookup(c.CategoryRepo)
	c.SiteDiscoveryBlockService = sitebuilderapp.NewDiscoveryBlockService(c.SiteDiscoveryBlockRepo)
	c.SiteAuditService = sitebuilderapp.NewAuditService(c.SiteAuditRepo, c.SiteAuditRepo)
	c.SiteBrandService = sitebuilderapp.NewBrandService(c.SettingService)
	c.SiteBuilderAdminHandler = sitebuilderhttp.NewAdminHandler(sitebuilderhttp.Services{
		HomeEntries:        c.SiteHomeEntryService,
		FeaturedCategories: c.SiteHomeFeaturedCategoryService,
		Discovery:          c.SiteDiscoveryBlockService,
		Brand:              c.SiteBrandService,
		Audit:              c.SiteAuditService,
	})
}

// checkinSettingsAdapter 把 settings 系统适配为签到配置读取端口。
// 签到业务配置（enabled + 7 天奖励）经 settings 持久化，读取路径宽松（损坏回退默认 + warning）。
type checkinSettingsAdapter struct {
	settings *settingsapp.Service
}

// GetCheckinConfig 读取签到配置并转换为 checkin 域配置。
func (a checkinSettingsAdapter) GetCheckinConfig() (checkincontract.Config, error) {
	setting, err := a.settings.GetCheckinSetting()
	if err != nil {
		return checkincontract.Config{}, err
	}
	return checkincontract.Config{
		Enabled: setting.Enabled,
		Rewards: setting.Rewards,
	}, nil
}
