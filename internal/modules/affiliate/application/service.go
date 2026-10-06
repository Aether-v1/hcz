package application

import (
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
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
	// walletSvc 是钱包服务，用于提现出金时真实入账到用户钱包。
	// 通过 SetWalletService 注入，测试时可 mock。
	walletSvc *walletapp.Service
	// walletUoW 是钱包工作单元，用于跨模块事务。
	walletUoW walletcontract.UnitOfWork
	// auditRecorder 是推广申请审计记录器（可空，尽力而为）。
	auditRecorder AuditRecorder
}

// AuditRecorder 推广业务审计记录端口（可空，尽力而为）。
type AuditRecorder interface {
	RecordAffiliateAudit(action string, actorID uint, targetUserID uint, metadata map[string]interface{}) error
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

// SetWalletService 注入钱包服务和工作单元（提现出金用）。
func (s *Service) SetWalletService(svc *walletapp.Service, uow walletcontract.UnitOfWork) {
	if s == nil {
		return
	}
	s.walletSvc = svc
	s.walletUoW = uow
}

// SetAuditRecorder 注入推广业务审计记录器（可空）。
func (s *Service) SetAuditRecorder(rec AuditRecorder) {
	if s == nil {
		return
	}
	s.auditRecorder = rec
}

// recordAudit 尽力而为写入审计日志，失败不影响主流程。
func (s *Service) recordAudit(action string, actorID uint, targetUserID uint, metadata map[string]interface{}) {
	if s == nil || s.auditRecorder == nil {
		return
	}
	_ = s.auditRecorder.RecordAffiliateAudit(action, actorID, targetUserID, metadata)
}
