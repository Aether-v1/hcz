package application

import (
	"errors"

	"github.com/Aether-v1/hcz/internal/config"
	"github.com/Aether-v1/hcz/internal/logger"
	productcontract "github.com/Aether-v1/hcz/internal/modules/catalog/product/contract"
	externalidentitycontract "github.com/Aether-v1/hcz/internal/modules/identity/externalidentity/contract"
	usercontract "github.com/Aether-v1/hcz/internal/modules/identity/user/contract"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	ordercontract "github.com/Aether-v1/hcz/internal/modules/order/contract"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	paymentcontract "github.com/Aether-v1/hcz/internal/modules/payment/contract"
	paymentdomain "github.com/Aether-v1/hcz/internal/modules/payment/domain"
	resellercontract "github.com/Aether-v1/hcz/internal/modules/reseller/contract"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
	usernotificationcontract "github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	walletapp "github.com/Aether-v1/hcz/internal/modules/wallet/application"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var (
	ErrPaymentInvalid                      = errors.New("payment invalid")
	ErrPaymentNotFound                     = errors.New("payment not found")
	ErrPaymentCreateFailed                 = errors.New("payment create failed")
	ErrPaymentUpdateFailed                 = errors.New("payment update failed")
	ErrPaymentStatusInvalid                = errors.New("payment status invalid")
	ErrPaymentAmountMismatch               = errors.New("payment amount mismatch")
	ErrPaymentCurrencyMismatch             = errors.New("payment currency mismatch")
	ErrPaymentChannelNotFound              = errors.New("payment channel not found")
	ErrPaymentChannelInactive              = errors.New("payment channel inactive")
	ErrPaymentProviderNotSupported         = errors.New("payment provider not supported")
	ErrPaymentChannelConfigInvalid         = errors.New("payment channel config invalid")
	ErrPaymentAmountTooSmall               = errors.New("payment amount too small")
	ErrPaymentAmountTooLarge               = errors.New("payment amount too large")
	ErrPaymentGatewayRequestFailed         = errors.New("payment gateway request failed")
	ErrPaymentGatewayResponseInvalid       = errors.New("payment gateway response invalid")
	ErrPaymentChannelNotAllowedForProduct  = errors.New("payment channel not allowed for product")
	ErrPaymentChannelNotAllowedForRecharge = errors.New("payment channel not allowed for wallet recharge")
	ErrProductFetchFailed                  = errors.New("product fetch failed")
	ErrQueueUnavailable                    = errors.New("queue unavailable")
	// ErrUSDTAddressNotBound 加密货币充值前，用户未绑定 USDT TRC20 收款地址。
	ErrUSDTAddressNotBound = errors.New("usdt trc20 address not bound")
)

// PaymentMethodChecker 收款方式查询端口（由 c2c 模块实现，避免循环依赖）。
type PaymentMethodChecker interface {
	HasUSDTTRC20Address(userID uint) bool
}

// cryptoRechargeProviders 需要前置绑定 USDT TRC20 地址的加密货币充值渠道。
var cryptoRechargeProviders = map[string]struct{}{
	"epusdt":    {},
	"bepusdt":   {},
	"dujiaopay": {},
	"okpay":     {},
	"tokenpay":  {},
}

// PaymentService 支付服务
type PaymentService struct {
	orderRepo               ordercontract.Store
	productRepo             productcontract.Repository
	productSKURepo          productcontract.SKURepository
	paymentRepo             paymentcontract.Store
	channelRepo             paymentcontract.ChannelStore
	walletRepo              walletcontract.Repository
	userRepo                usercontract.Store
	userOAuthIdentityRepo   externalidentitycontract.Store
	queue                   paymentcontract.Queue
	walletSvc               *walletapp.Service
	settingService          *settingsapp.Service
	defaultEmailConfig      config.EmailConfig
	expireMinutes           int
	affiliateSvc            AffiliatePaymentLifecycle
	notificationSvc         notificationcontract.NotificationEnqueuer
	procurementSvc          ProcurementCreator
	downstreamCallbackSvc   DownstreamCallbackEnqueuer
	memberLevelSvc          MemberLevelProgressor
	paymentProviderRegistry paymentcontract.GatewayRegistry
	resellerAccounting      resellerAccountingTransactions
	// userNotifier 写入用户站内通知（尽力而为+幂等，Phase 1）。
	userNotifier usernotificationcontract.Creator
	// pmChecker 收款方式查询（充值前置校验）；通过 setter 注入以解耦 c2c 模块。
	pmChecker PaymentMethodChecker
}

type MemberLevelProgressor interface {
	OnOrderPaid(userID uint, amount decimal.Decimal) error
	OnRechargeCompleted(userID uint, amount decimal.Decimal) error
}

type ProcurementCreator interface {
	CreateForOrder(orderID uint) error
}

// DownstreamCallbackEnqueuer 是支付与交付上下文触发下游回调所需的最小端口。
type DownstreamCallbackEnqueuer interface {
	EnqueueCallback(orderID uint)
}

// AffiliatePaymentLifecycle 是支付成功回调所需的推广返利用例端口。
type AffiliatePaymentLifecycle interface {
	HandleOrderPaid(orderID uint) error
}

type resellerAccountingTransactions interface {
	PostOrderProfit(store resellercontract.AccountingLedgerStore, order *orderdomain.Order, payment *paymentdomain.Payment) error
}

// SetProcurementService 设置采购单服务（解决循环依赖）
func (s *PaymentService) SetProcurementService(svc ProcurementCreator) {
	s.procurementSvc = svc
}

// SetDownstreamCallbackService 设置下游回调服务（解决循环依赖）
func (s *PaymentService) SetDownstreamCallbackService(svc DownstreamCallbackEnqueuer) {
	s.downstreamCallbackSvc = svc
}

// SetMemberLevelService 设置会员等级服务
func (s *PaymentService) SetMemberLevelService(svc MemberLevelProgressor) {
	s.memberLevelSvc = svc
}

// SetUserNotifier 设置用户站内通知写入器（Phase 1，尽力而为+幂等）。
func (s *PaymentService) SetUserNotifier(svc usernotificationcontract.Creator) {
	s.userNotifier = svc
}

// SetPaymentMethodChecker 设置收款方式查询端口（加密货币充值前置校验，避免循环依赖）。
func (s *PaymentService) SetPaymentMethodChecker(checker PaymentMethodChecker) {
	s.pmChecker = checker
}

// PaymentServiceOptions 支付服务构造参数
type PaymentServiceOptions struct {
	OrderStore              ordercontract.Store
	ProductRepo             productcontract.Repository
	ProductSKURepo          productcontract.SKURepository
	PaymentStore            paymentcontract.Store
	ChannelStore            paymentcontract.ChannelStore
	WalletRepo              walletcontract.Repository
	UserStore               usercontract.Store
	ExternalIdentityStore   externalidentitycontract.Store
	Queue                   paymentcontract.Queue
	WalletService           *walletapp.Service
	SettingService          *settingsapp.Service
	DefaultEmailConfig      config.EmailConfig
	ExpireMinutes           int
	AffiliateService        AffiliatePaymentLifecycle
	NotificationService     notificationcontract.NotificationEnqueuer
	PaymentProviderRegistry paymentcontract.GatewayRegistry
	ResellerAccounting      resellerAccountingTransactions
}

// NewPaymentService 创建支付服务
func NewPaymentService(opts PaymentServiceOptions) *PaymentService {
	return &PaymentService{
		orderRepo:               opts.OrderStore,
		productRepo:             opts.ProductRepo,
		productSKURepo:          opts.ProductSKURepo,
		paymentRepo:             opts.PaymentStore,
		channelRepo:             opts.ChannelStore,
		walletRepo:              opts.WalletRepo,
		userRepo:                opts.UserStore,
		userOAuthIdentityRepo:   opts.ExternalIdentityStore,
		queue:                   opts.Queue,
		walletSvc:               opts.WalletService,
		settingService:          opts.SettingService,
		defaultEmailConfig:      opts.DefaultEmailConfig,
		expireMinutes:           opts.ExpireMinutes,
		affiliateSvc:            opts.AffiliateService,
		notificationSvc:         opts.NotificationService,
		paymentProviderRegistry: opts.PaymentProviderRegistry,
		resellerAccounting:      opts.ResellerAccounting,
	}
}

// ListPayments 管理端支付列表。
func (s *PaymentService) ListPayments(filter paymentcontract.ListFilter) ([]paymentdomain.Payment, int64, error) {
	return s.paymentRepo.ListAdmin(filter)
}

// GetPayment 获取支付记录。
func (s *PaymentService) GetPayment(id uint) (*paymentdomain.Payment, error) {
	if id == 0 {
		return nil, ErrPaymentInvalid
	}
	payment, err := s.paymentRepo.GetByID(id)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if payment == nil {
		return nil, ErrPaymentNotFound
	}
	return payment, nil
}

// ListChannels 支付渠道列表。
func (s *PaymentService) ListChannels(filter paymentcontract.ChannelListFilter) ([]paymentdomain.PaymentChannel, int64, error) {
	return s.channelRepo.List(filter)
}

// GetChannel 获取支付渠道。
func (s *PaymentService) GetChannel(id uint) (*paymentdomain.PaymentChannel, error) {
	if id == 0 {
		return nil, ErrPaymentInvalid
	}
	channel, err := s.channelRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, ErrPaymentChannelNotFound
	}
	return channel, nil
}

func paymentLogger(kv ...interface{}) *zap.SugaredLogger {
	if len(kv) == 0 {
		return logger.S()
	}
	return logger.SW(kv...)
}
