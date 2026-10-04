package container

import (
	"context"

	adproxyapp "github.com/Aether-v1/hcz/internal/modules/adproxy/application"
	adproxygateway "github.com/Aether-v1/hcz/internal/modules/adproxy/infrastructure/adgateway"
	categoryapp "github.com/Aether-v1/hcz/internal/modules/catalog/category/application"

	"github.com/Aether-v1/hcz/internal/logger"
	cardsecretapp "github.com/Aether-v1/hcz/internal/modules/cardsecret/application"
	cartapp "github.com/Aether-v1/hcz/internal/modules/cart/application"
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
	promotionapp "github.com/Aether-v1/hcz/internal/modules/promotion/application"
	sitemapapp "github.com/Aether-v1/hcz/internal/modules/sitemap/application"
	sitemapcontract "github.com/Aether-v1/hcz/internal/modules/sitemap/contract"
	sitemapcache "github.com/Aether-v1/hcz/internal/modules/sitemap/infrastructure/cacheadapter"
	sitemapcatalog "github.com/Aether-v1/hcz/internal/modules/sitemap/infrastructure/catalogreader"
	usernotificationapp "github.com/Aether-v1/hcz/internal/modules/usernotification/application"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
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
	c.OrderRefundService = orderrefund.New(
		c.OrderStore,
		c.UserStore,
		c.AffiliateService,
		c.SettingService,
		c.WalletService,
		c.PaymentStore,
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
	c.ContentBannerService = contentapp.NewBannerService(
		gormstore.NewBannerStore(gormdb.DB),
		contentapp.SystemClock{},
	)
}
