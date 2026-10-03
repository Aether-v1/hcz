package container

import (
	"fmt"

	affiliategormstore "github.com/Aether-v1/hcz/internal/modules/affiliate/infrastructure/gormstore"
	apicredentialgormstore "github.com/Aether-v1/hcz/internal/modules/apicredential/infrastructure/gormstore"
	auditloggormstore "github.com/Aether-v1/hcz/internal/modules/auditlog/infrastructure/gormstore"
	cardsecretgormstore "github.com/Aether-v1/hcz/internal/modules/cardsecret/infrastructure/gormstore"
	cartgormstore "github.com/Aether-v1/hcz/internal/modules/cart/infrastructure/gormstore"
	categorygormstore "github.com/Aether-v1/hcz/internal/modules/catalog/category/infrastructure/gormstore"
	mappinggormstore "github.com/Aether-v1/hcz/internal/modules/catalog/mapping/infrastructure/gormstore"
	productgormstore "github.com/Aether-v1/hcz/internal/modules/catalog/product/store/gormstore"
	channelclientstore "github.com/Aether-v1/hcz/internal/modules/channelclient/infrastructure/gormstore"
	coupongormstore "github.com/Aether-v1/hcz/internal/modules/coupon/infrastructure/gormstore"
	dashboardgormstore "github.com/Aether-v1/hcz/internal/modules/dashboard/infrastructure/gormstore"
	downstreamcallbackgormstore "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/infrastructure/gormstore"
	fulfillmentgormstore "github.com/Aether-v1/hcz/internal/modules/fulfillment/infrastructure/gormstore"
	giftcardgormstore "github.com/Aether-v1/hcz/internal/modules/giftcard/infrastructure/gormstore"
	adminstore "github.com/Aether-v1/hcz/internal/modules/identity/admin/infrastructure/gormstore"
	emailverificationstore "github.com/Aether-v1/hcz/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitystore "github.com/Aether-v1/hcz/internal/modules/identity/externalidentity/infrastructure/gormstore"
	userstore "github.com/Aether-v1/hcz/internal/modules/identity/user/infrastructure/gormstore"
	memberlevelgormstore "github.com/Aether-v1/hcz/internal/modules/memberlevel/infrastructure/gormstore"
	notificationgormstore "github.com/Aether-v1/hcz/internal/modules/notification/infrastructure/gormstore"
	ordergormstore "github.com/Aether-v1/hcz/internal/modules/order/infrastructure/gormstore"
	paymentgormstore "github.com/Aether-v1/hcz/internal/modules/payment/infrastructure/gormstore"
	procurementgormstore "github.com/Aether-v1/hcz/internal/modules/procurement/infrastructure/gormstore"
	promotiongormstore "github.com/Aether-v1/hcz/internal/modules/promotion/infrastructure/gormstore"
	reconciliationgormstore "github.com/Aether-v1/hcz/internal/modules/reconciliation/infrastructure/gormstore"
	resellergormstore "github.com/Aether-v1/hcz/internal/modules/reseller/infrastructure/gormstore"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	siteconnectiongormstore "github.com/Aether-v1/hcz/internal/modules/siteconnection/infrastructure/gormstore"
	broadcaststore "github.com/Aether-v1/hcz/internal/modules/telegram/broadcast/infrastructure/gormstore"
	walletgormstore "github.com/Aether-v1/hcz/internal/modules/wallet/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"
)

func (c *Container) initRepositories() error {
	db := gormdb.DB
	c.AdminStore = adminstore.New(db)
	c.UserStore = userstore.New(db)
	c.ExternalIdentityStore = externalidentitystore.New(db)
	c.EmailVerificationStore = emailverificationstore.New(db)
	orderStore := ordergormstore.New(db, c.Config.App.SecretKey)
	if _, err := orderStore.BackfillGuestCredentialHashes(); err != nil {
		return fmt.Errorf("backfill guest order credentials: %w", err)
	}
	c.OrderStore = orderStore
	c.PaymentStore = paymentgormstore.New(db, c.Config.App.SecretKey)
	c.PaymentChannelStore = paymentgormstore.NewChannelStore(db)
	c.CardSecretRepo = cardsecretgormstore.New(db)
	c.CardSecretBatchRepo = cardsecretgormstore.NewBatch(db)
	c.GiftCardRepo = giftcardgormstore.New(db)
	c.FulfillmentStore = fulfillmentgormstore.New(db)
	c.ProductRepo = productgormstore.NewProductStore(db)
	c.ProductSKURepo = productgormstore.NewSKUStore(db)
	c.CartRepo = cartgormstore.New(db)
	c.CouponRepo = coupongormstore.New(db)
	c.CouponUsageRepo = coupongormstore.NewUsageStore(db)
	c.PromotionRepo = promotiongormstore.New(db)
	c.WalletRepo = walletgormstore.New(db)
	c.CategoryRepo = categorygormstore.NewCategoryStore(db)
	c.SettingRepo = settingsstore.New(db)
	c.UserLoginLogRepo = auditloggormstore.NewUserLoginStore(db)
	c.AuthzAuditLogRepo = auditloggormstore.NewAuthzStore(db)
	c.NotificationLogRepo = notificationgormstore.NewLogStore(db)
	c.AdminLoginLogRepo = auditloggormstore.NewAdminLoginStore(db)
	c.DashboardRepo = dashboardgormstore.New(db)
	c.AffiliateRepo = affiliategormstore.New(db)
	c.ResellerStore = resellergormstore.New(db)
	c.ApiCredentialRepo = apicredentialgormstore.New(db)
	c.SiteConnectionRepo = siteconnectiongormstore.New(db)
	c.ProductMappingRepo = mappinggormstore.NewMappingStore(db)
	c.SKUMappingRepo = mappinggormstore.NewSKUMappingStore(db)
	c.ProcurementOrderRepo = procurementgormstore.New(db)
	c.DownstreamOrderRefRepo = downstreamcallbackgormstore.New(db)
	c.ReconciliationJobRepo = reconciliationgormstore.NewJobStore(db)
	c.ReconciliationItemRepo = reconciliationgormstore.NewItemStore(db)
	c.ChannelClientStore = channelclientstore.New(db)
	c.TelegramBroadcastRepo = broadcaststore.New(db)
	c.MemberLevelRepo = memberlevelgormstore.NewLevelStore(db)
	c.MemberLevelPriceRepo = memberlevelgormstore.NewPriceStore(db)
	c.MemberLevelUserRepo = memberlevelgormstore.NewUserStore(db)
	return nil
}

