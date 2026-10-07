package container

import (
	catalogmappingbootstrap "github.com/Aether-v1/hcz/internal/bootstrap/catalogmapping"
	telegrambroadcast "github.com/Aether-v1/hcz/internal/bootstrap/telegrambroadcast"
	"github.com/Aether-v1/hcz/internal/logger"
	apicredentialapp "github.com/Aether-v1/hcz/internal/modules/apicredential/application"
	auditlogapp "github.com/Aether-v1/hcz/internal/modules/auditlog/application"
	channelclientapp "github.com/Aether-v1/hcz/internal/modules/channelclient/application"
	contentapp "github.com/Aether-v1/hcz/internal/modules/content/application"
	localfilestore "github.com/Aether-v1/hcz/internal/modules/content/infrastructure/filestore/local"
	contentgormstore "github.com/Aether-v1/hcz/internal/modules/content/infrastructure/gormstore"
	dashboardapp "github.com/Aether-v1/hcz/internal/modules/dashboard/application"
	downstreamcallbackapp "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/application"
	downstreamcallbackcontract "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/contract"
	downstreamcallbackclient "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/infrastructure/callbackclient"
	downstreamcallbackcredentialreader "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/infrastructure/credentialreader"
	downstreamcallbackorderreader "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/infrastructure/orderreader"
	downstreamcallbackqueue "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/infrastructure/queueadapter"
	exchangerateapp "github.com/Aether-v1/hcz/internal/modules/exchangerate/application"
	exchangeratecoingecko "github.com/Aether-v1/hcz/internal/modules/exchangerate/infrastructure/provider"
	exchangeraterediscache "github.com/Aether-v1/hcz/internal/modules/exchangerate/infrastructure/rediscache"
	exchangeratesettingsstore "github.com/Aether-v1/hcz/internal/modules/exchangerate/infrastructure/settingsstore"
	notificationapp "github.com/Aether-v1/hcz/internal/modules/notification/application"
	notificationasyncqueue "github.com/Aether-v1/hcz/internal/modules/notification/infrastructure/asyncqueue"
	notificationfeishu "github.com/Aether-v1/hcz/internal/modules/notification/infrastructure/feishu"
	orderapp "github.com/Aether-v1/hcz/internal/modules/order/application"
	paymentapp "github.com/Aether-v1/hcz/internal/modules/payment/application"
	paymentqueue "github.com/Aether-v1/hcz/internal/modules/payment/infrastructure/queueadapter"
	procurementapp "github.com/Aether-v1/hcz/internal/modules/procurement/application"
	procurementcontract "github.com/Aether-v1/hcz/internal/modules/procurement/contract"
	procurementmapping "github.com/Aether-v1/hcz/internal/modules/procurement/infrastructure/mappingreader"
	procurementnotification "github.com/Aether-v1/hcz/internal/modules/procurement/infrastructure/notificationadapter"
	procurementorder "github.com/Aether-v1/hcz/internal/modules/procurement/infrastructure/orderreader"
	procurementqueue "github.com/Aether-v1/hcz/internal/modules/procurement/infrastructure/queueadapter"
	procurementupstream "github.com/Aether-v1/hcz/internal/modules/procurement/infrastructure/upstreamgateway"
	reconciliationapp "github.com/Aether-v1/hcz/internal/modules/reconciliation/application"
	reconciliationnotification "github.com/Aether-v1/hcz/internal/modules/reconciliation/infrastructure/notificationadapter"
	reconciliationprocurement "github.com/Aether-v1/hcz/internal/modules/reconciliation/infrastructure/procurementreader"
	reconciliationqueue "github.com/Aether-v1/hcz/internal/modules/reconciliation/infrastructure/queueadapter"
	reconciliationupstream "github.com/Aether-v1/hcz/internal/modules/reconciliation/infrastructure/upstreamreader"
	siteconnectionapp "github.com/Aether-v1/hcz/internal/modules/siteconnection/application"
	broadcastapp "github.com/Aether-v1/hcz/internal/modules/telegram/broadcast/application"
	notifyapp "github.com/Aether-v1/hcz/internal/modules/telegram/notify/application"
	notifybotapi "github.com/Aether-v1/hcz/internal/modules/telegram/notify/infrastructure/botapi"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
)

var _ procurementcontract.OrderCompletion = (*orderCompletionAdapter)(nil)

// orderCompletionAdapter 把订单域 OrderService 适配为采购侧统一完成生命周期端口（P1）。
// 依赖方向：procurement/container → order application；order 不反向依赖 procurement。
type orderCompletionAdapter struct {
	svc *orderapp.OrderService
}

// CompleteOrder 本地订单进入 completed 的统一入口（自开事务，幂等）。
func (a orderCompletionAdapter) CompleteOrder(orderID uint) error {
	if a.svc == nil {
		return nil
	}
	return a.svc.CompleteOrderByProcurement(orderID)
}

// CompleteParentSideEffects 父订单 completed 的统一副作用入口（自开事务，幂等）。
func (a orderCompletionAdapter) CompleteParentSideEffects(parentID uint) error {
	if a.svc == nil {
		return nil
	}
	return a.svc.CompleteParentSideEffects(parentID)
}

// initIntegrationServices 装配通知、站点对接、支付、采购、渠道与 Telegram 集成。
func (c *Container) initIntegrationServices() {
	c.UserLoginLogService = auditlogapp.NewUserLoginService(c.UserLoginLogRepo)
	c.AuthzAuditService = auditlogapp.NewAuthzService(c.AuthzAuditLogRepo)
	c.AffiliateService.SetAuditRecorder(affiliateAuditRecorder{audit: c.AuthzAuditService})
	c.AdminLoginLogService = auditlogapp.NewAdminLoginService(c.AdminLoginLogRepo)
	c.NotificationLogService = notificationapp.NewLogService(c.NotificationLogRepo)
	c.DashboardService = dashboardapp.NewService(c.DashboardRepo, c.SettingService)
	telegramNotifyService := notifyapp.NewService(c.SettingService, c.Config.TelegramAuth, notifybotapi.New())
	c.NotificationService = notificationapp.NewService(
		c.SettingService,
		c.EmailSender,
		notificationasyncqueue.New(c.QueueClient),
		c.DashboardService,
		c.NotificationLogService,
		telegramNotifyService,
		notificationfeishu.New(),
	)
	c.ApiCredentialService = apicredentialapp.NewService(c.ApiCredentialRepo)
	c.SiteConnectionService = siteconnectionapp.NewService(c.SiteConnectionRepo, c.Config.App.SecretKey, "uploads")
	mediaCore := contentapp.NewMediaService(
		contentgormstore.NewMediaStore(gormdb.DB),
		localfilestore.New(),
		contentapp.WarningLoggerFunc(logger.Warnw),
	)
	c.ContentMediaService = mediaCore
	productMappingService, err := catalogmappingbootstrap.New(catalogmappingbootstrap.Dependencies{
		Mappings:    c.ProductMappingRepo,
		SKUMappings: c.SKUMappingRepo,
		Products:    c.ProductRepo,
		SKUs:        c.ProductSKURepo,
		Categories:  c.CategoryRepo,
		Connections: c.SiteConnectionService,
		Media:       mediaCore,
	})
	if err != nil {
		logger.Errorw("provider_init_product_mapping_failed", "error", err)
		panic(err)
	}
	c.ProductMappingService = productMappingService
	c.ProductMappingService.SetCategoryCreator(c.CategoryService)
	c.ProductMappingService.SetSettings(c.SettingService)
	c.SiteConnectionService.SetMarkupReapplier(c.ProductMappingService)
	c.OrderService.SetProductMappingService(c.ProductMappingService)

	// HCZ P0-2: Global Exchange Rate。独立于 Payment Gateway 充值汇率，只用于商品订单
	// Site Currency → USDT 换算。无有效汇率时 OrderService 拒单（fail-closed）。
	exStore := exchangeratesettingsstore.New(c.SettingRepo)
	cachedStore := exchangeraterediscache.New(exStore)
	siteCurrency := "CNY"
	if cur, err := c.SettingService.GetSiteCurrency("CNY"); err == nil && cur != "" {
		siteCurrency = cur
	}
	c.ExchangeRateService = exchangerateapp.NewService(
		exchangeratecoingecko.NewCoinGecko(""),
		cachedStore,
		siteCurrency,
		exchangerateStaleDuration,
	)
	c.OrderService.SetRateResolver(exchangerateResolverAdapter{svc: c.ExchangeRateService})
	var downstreamQueue downstreamcallbackcontract.CallbackQueue
	if c.QueueClient != nil {
		downstreamQueue = downstreamcallbackqueue.New(c.QueueClient)
	}
	c.DownstreamCallbackService = downstreamcallbackapp.NewService(downstreamcallbackapp.Options{
		References:  c.DownstreamOrderRefRepo,
		Orders:      downstreamcallbackorderreader.New(c.OrderStore),
		Credentials: downstreamcallbackcredentialreader.New(c.ApiCredentialRepo),
		Queue:       downstreamQueue,
		Deliverer:   downstreamcallbackclient.New(),
	})
	c.PaymentService = paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:              c.OrderStore,
		ProductRepo:             c.ProductRepo,
		ProductSKURepo:          c.ProductSKURepo,
		PaymentStore:            c.PaymentStore,
		ChannelStore:            c.PaymentChannelStore,
		WalletRepo:              c.WalletRepo,
		UserStore:               c.UserStore,
		ExternalIdentityStore:   c.ExternalIdentityStore,
		Queue:                   paymentqueue.New(c.QueueClient),
		WalletService:           c.WalletService,
		SettingService:          c.SettingService,
		DefaultEmailConfig:      c.Config.Email,
		ExpireMinutes:           c.Config.Order.PaymentExpireMinutes,
		AffiliateService:        c.AffiliateService,
		NotificationService:     c.NotificationService,
		PaymentProviderRegistry: c.PaymentProviderRegistry,
		ResellerAccounting:      c.ResellerAccountingLedger,
	})
	// 充值（加密货币渠道）前置校验：用户须已绑定 USDT TRC20 地址。
	c.PaymentService.SetPaymentMethodChecker(c.C2CService)
	// P1：采购回调完成本地订单走统一完成生命周期（Affiliate + Points 副作用收口）。
	// 先持有 Lifecycle 实例（container 层适配），构造后注入 OrderCompletion。
	procurementLifecycle := c.ProcurementOrderRepo.NewLifecycle(c.QueueClient, c.SettingService, c.Config.Email)
	c.ProcurementOrderService = procurementapp.NewService(procurementapp.Options{
		Repository:         c.ProcurementOrderRepo,
		Orders:             procurementorder.New(c.OrderStore),
		ProductMappings:    procurementmapping.NewProducts(c.ProductMappingRepo),
		SKUMappings:        procurementmapping.NewSKUs(c.SKUMappingRepo),
		Connections:        procurementupstream.New(c.SiteConnectionService),
		Queue:              procurementqueue.New(c.QueueClient),
		OrderLifecycle:     procurementLifecycle,
		DownstreamCallback: c.DownstreamCallbackService,
		BotNotifier:        c.FulfillmentService,
		Notifications:      procurementnotification.New(c.NotificationService),
	})
	// P1：采购回调完成本地订单走统一完成生命周期（Affiliate + Points 副作用收口）。
	procurementLifecycle.SetOrderCompletion(orderCompletionAdapter{svc: c.OrderService})
	c.ReconciliationService = reconciliationapp.NewService(reconciliationapp.Options{
		Jobs: c.ReconciliationJobRepo, Items: c.ReconciliationItemRepo,
		Procurements:  reconciliationprocurement.New(c.ProcurementOrderRepo),
		Upstream:      reconciliationupstream.New(c.SiteConnectionService),
		Queue:         reconciliationqueue.New(c.QueueClient),
		Notifications: reconciliationnotification.New(c.NotificationService),
	})
	c.ChannelClientService = channelclientapp.NewService(c.ChannelClientStore, c.Config.App.SecretKey)
	c.TelegramBroadcastService = broadcastapp.NewService(
		c.TelegramBroadcastRepo,
		telegrambroadcast.NewUserDirectory(c.ExternalIdentityStore),
		telegrambroadcast.NewBotTokenResolver(c.ChannelClientService),
		telegrambroadcast.NewDispatcher(c.QueueClient),
		telegramNotifyService,
	)
}
