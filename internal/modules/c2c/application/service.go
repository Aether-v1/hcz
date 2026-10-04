// Package application 实现 C2C 用例编排：支付方式、挂单、交易资金操作。
// 所有资金操作（Freeze/Unfreeze/SettleFrozen）都在本层、同一事务内完成，
// Handler 只做参数解析与鉴权，不触碰钱包。
package application

import (
	"math/rand"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	c2ccontract "github.com/Aether-v1/hcz/internal/modules/c2c/contract"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// Options 装配 C2C 用例所需依赖。
type Options struct {
	Repository    c2ccontract.Repository
	UnitOfWork    c2ccontract.UnitOfWork
	WalletService *walletapp.Service
	Users         c2ccontract.UserReader
	UserAdmin     c2ccontract.UserAdminStore
	Config        c2ccontract.ConfigReader
	Scheduler     c2ccontract.ExpireTaskScheduler
	Notifier      c2ccontract.Notifier
	Audit         c2ccontract.ArbitrationAuditWriter
}

// Service C2C 用例入口。
type Service struct {
	repo      c2ccontract.Repository
	uow       c2ccontract.UnitOfWork
	wallet    *walletapp.Service
	users     c2ccontract.UserReader
	userAdmin c2ccontract.UserAdminStore
	config    c2ccontract.ConfigReader
	scheduler c2ccontract.ExpireTaskScheduler
	notifier  c2ccontract.Notifier
	audit     c2ccontract.ArbitrationAuditWriter
}

// NewService 创建 C2C Service。
func NewService(opts Options) *Service {
	return &Service{
		repo:      opts.Repository,
		uow:       opts.UnitOfWork,
		wallet:    opts.WalletService,
		users:     opts.Users,
		userAdmin: opts.UserAdmin,
		config:    opts.Config,
		scheduler: opts.Scheduler,
		notifier:  opts.Notifier,
		audit:     opts.Audit,
	}
}

// notifySafely 事务提交后异步通知，失败不阻塞资金主流程。
func (s *Service) notifySafely(eventType string, bizID uint, data map[string]interface{}) {
	if s == nil || s.notifier == nil {
		return
	}
	defer func() { _ = recover() }()
	_ = s.notifier.Enqueue(notificationcontract.EnqueueInput{
		EventType: eventType,
		BizType:   constants.NotificationBizTypeC2CTrade,
		BizID:     bizID,
		Force:     false,
		Data:      jsonmap.JSON(data),
	})
}

// loadConfig 读取并归一化 C2C 配置；读取失败回退默认（disabled）。
func (s *Service) loadConfig() settingsintegration.C2CSetting {
	if s.config == nil {
		return settingsintegration.DefaultC2CSetting()
	}
	cfg, err := s.config.GetC2CConfig()
	if err != nil {
		return settingsintegration.DefaultC2CSetting()
	}
	return cfg
}

// dayStart 返回今日零点（按 now 的本地日界截断）。
func dayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// genNo 生成单号：前缀 + 时间戳 + 随机数，唯一索引兜底冲突。
func genNo(prefix string, now time.Time) string {
	return prefix + now.Format("20060102150405") + randSuffix()
}

func randSuffix() string {
	const digits = "0123456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = digits[rand.Intn(len(digits))]
	}
	return string(b)
}
