package publicconfigwiring

import (
	"github.com/Aether-v1/hcz/internal/app/container"
	publicconfigtransport "github.com/Aether-v1/hcz/internal/modules/settings/transport/http/public"
)

func NewHandler(c *container.Container) *publicconfigtransport.Handler {
	var captcha publicconfigtransport.CaptchaPublic
	if c.CaptchaService != nil {
		captcha = publicConfigCaptchaAdapter{svc: c.CaptchaService}
	}
	var telegram publicconfigtransport.TelegramAuthPublic
	if c.TelegramAuthService != nil {
		telegram = publicConfigTelegramAdapter{svc: c.TelegramAuthService}
	}
	var google publicconfigtransport.GoogleAuthPublic
	if c.GoogleAuthService != nil {
		google = publicConfigGoogleAdapter{svc: c.GoogleAuthService}
	}
	var overlay publicconfigtransport.ResellerOverlay
	if c.ResellerSiteConfigService != nil {
		overlay = publicConfigResellerOverlayAdapter{svc: c.ResellerSiteConfigService}
	}
	fallback := publicconfigtransport.TelegramAuthFallback{}
	googleFallback := publicconfigtransport.GoogleAuthFallback{}
	if c.Config != nil {
		fallback = publicconfigtransport.TelegramAuthFallback{
			Enabled:     c.Config.TelegramAuth.Enabled,
			BotUsername: c.Config.TelegramAuth.BotUsername,
			MiniAppURL:  c.Config.TelegramAuth.MiniAppURL,
		}
		googleFallback = publicconfigtransport.GoogleAuthFallback{
			Enabled:  c.Config.GoogleAuth.Enabled,
			ClientID: c.Config.GoogleAuth.ClientID,
		}
	}
	handler := publicconfigtransport.NewHandler(
		publicConfigCacheAdapter{},
		publicConfigSettingsAdapter{settings: c.SettingService, cfg: c.Config},
		publicConfigPaymentAdapter{payments: c.PaymentService},
		captcha,
		telegram,
		fallback,
		google,
		googleFallback,
		overlay,
	)
	if sb := initSiteBuilderPublic(c); sb != nil {
		handler.SetSiteBuilder(sb)
	}
	return handler
}

// initSiteBuilderPublic 注入公开装修数据端口。
func initSiteBuilderPublic(c *container.Container) publicconfigtransport.SiteBuilderPublic {
	if c.SiteHomeEntryService == nil || c.SiteDiscoveryBlockService == nil || c.ContentBannerService == nil {
		return nil
	}
	return publicConfigSiteBuilderAdapter{
		homeEntries: c.SiteHomeEntryService,
		discovery:   c.SiteDiscoveryBlockService,
		banners:     c.ContentBannerService,
	}
}
