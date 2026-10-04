package application

import (
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	usercontract "github.com/Aether-v1/hcz/internal/modules/identity/user/contract"
	usernotificationcontract "github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
)

const (
	affiliateCodeLength        = 8
	affiliateSplitTypePrefix   = "sp"
	affiliateAttributionWindow = 30 * 24 * time.Hour
	affiliateClickDedupeWindow = 10 * time.Minute
	affiliateMaxLevel          = 10
)

// Service 推广返利业务服务
type Service struct {
	repo        affiliatecontract.Store
	userRepo    usercontract.Store
	orderRepo   affiliatecontract.OrderReader
	productRepo affiliatecontract.ProductReader
	settings    affiliatecontract.SettingsReader
	// userNotifier 是佣金到账通知写入器（尽力而为+幂等），可空。
	userNotifier usernotificationcontract.Creator
}

// NewService 创建推广返利服务
func NewService(
	repo affiliatecontract.Store,
	userRepo usercontract.Store,
	orderRepo affiliatecontract.OrderReader,
	productRepo affiliatecontract.ProductReader,
	settings affiliatecontract.SettingsReader,
) *Service {
	return &Service{
		repo:        repo,
		userRepo:    userRepo,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		settings:    settings,
	}
}

// SetUserNotifier 注入用户站内通知写入器（佣金到账通知）。
func (s *Service) SetUserNotifier(svc usernotificationcontract.Creator) {
	if s == nil {
		return
	}
	s.userNotifier = svc
}
