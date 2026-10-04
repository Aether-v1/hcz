package contract

import (
	"time"

	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"

	"github.com/shopspring/decimal"
)

// Repository 拥有 C2C 全部聚合（支付方式/挂单/交易/争议/风控信号）的持久化。
// 事务调用方通过 Transaction 拿到绑定同一事务的实现。
type Repository interface {
	// ---- 支付方式 ----
	CreatePaymentMethod(p *c2cdomain.PaymentMethod) error
	UpdatePaymentMethod(p *c2cdomain.PaymentMethod) error
	SoftDeletePaymentMethod(id uint) error
	GetPaymentMethodByID(id uint) (*c2cdomain.PaymentMethod, error)
	ListPaymentMethodsByUserID(userID uint) ([]c2cdomain.PaymentMethod, error)
	ListEnabledPaymentMethodsByUserID(userID uint) ([]c2cdomain.PaymentMethod, error)

	// ---- 挂单 ----
	CreateListing(l *c2cdomain.Listing) error
	UpdateListing(l *c2cdomain.Listing) error
	GetListingByID(id uint) (*c2cdomain.Listing, error)
	GetListingByIDForUpdate(id uint) (*c2cdomain.Listing, error)
	ListMarketListings(filter ListingMarketFilter) ([]c2cdomain.Listing, int64, error)
	ListMyListings(filter ListingMyFilter) ([]c2cdomain.Listing, int64, error)
	// DecrementListingAvailableUSDT 原子扣减可售余量：
	// UPDATE c2c_listings SET available_usdt = available_usdt - ? WHERE id = ? AND available_usdt >= ?。
	// 返回受影响行数；0 表示余量不足（并发被抢先扣完）。
	DecrementListingAvailableUSDT(id uint, amount decimal.Decimal) (int64, error)
	// IncrementListingAvailableUSDT 原子恢复可售余量（取消/超时退回）。
	IncrementListingAvailableUSDT(id uint, amount decimal.Decimal) error

	// ---- 交易 ----
	CreateTrade(t *c2cdomain.Trade) error
	UpdateTrade(t *c2cdomain.Trade) error
	GetTradeByID(id uint) (*c2cdomain.Trade, error)
	GetTradeByIDForUpdate(id uint) (*c2cdomain.Trade, error)
	GetTradeByTradeNo(tradeNo string) (*c2cdomain.Trade, error)
	// GetTradeByBuyerIdempotency 用买家 + 幂等键查重（命中则直接返回已有交易）。
	GetTradeByBuyerIdempotency(buyerID uint, idempotencyKey string) (*c2cdomain.Trade, error)
	ListMyTrades(filter TradeListFilter) ([]c2cdomain.Trade, int64, error)
	// SumUserTradedUSDTSince 统计用户自 since 起已成交（completed）的 USDT 合计，用于日限额。
	SumUserTradedUSDTSince(userID uint, since time.Time) (decimal.Decimal, error)

	// ---- 争议 / 风控 ----
	CreateDispute(d *c2cdomain.Dispute) error
	UpdateDispute(d *c2cdomain.Dispute) error
	GetDisputeByID(id uint) (*c2cdomain.Dispute, error)
	GetDisputeByTradeIDForUpdate(tradeID uint) (*c2cdomain.Dispute, error)
	ListDisputes(filter DisputeListFilter) ([]c2cdomain.Dispute, int64, error)
	CreateRiskSignal(r *c2cdomain.RiskSignal) error
	ListRiskSignals(filter RiskSignalListFilter) ([]c2cdomain.RiskSignal, int64, error)

	// ---- 后台管理查询 ----
	ListAdminListings(filter AdminListingFilter) ([]c2cdomain.Listing, int64, error)
	ListAdminTrades(filter AdminTradeFilter) ([]c2cdomain.Trade, int64, error)
	OverviewStats() (OverviewStats, error)
}

// Transaction 是已开启的数据库事务在 C2C 上下文中的视图。
// Wallets() 返回绑定同一事务的钱包仓库，使 C2C Transaction 自动满足
// walletcontract.Transaction，可直接传给 walletService.Freeze/Unfreeze/SettleFrozen。
type Transaction interface {
	C2C() Repository
	Wallets() walletcontract.Repository
}

// UnitOfWork 开启 C2C 事务，回调内 C2C() 与 Wallets() 共享同一 *gorm.DB。
type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// ConfigReader 读取 C2C 全局配置。
type ConfigReader interface {
	GetC2CConfig() (settingsintegration.C2CSetting, error)
}

// AdminSettingsConfig C2C 设置读写（管理员用）。
type AdminSettingsConfig interface {
	GetC2CConfig() (settingsintegration.C2CSetting, error)
	UpdateC2CSetting(setting settingsintegration.C2CSetting) (settingsintegration.C2CSetting, error)
}

// UserReader 读取用户（状态/封禁/TOTP/注册时间），用于资格校验。
type UserReader interface {
	GetByID(userID uint) (*userdomain.User, error)
}

// UserAdminStore 用户读取 + 更新（用于管理员禁用/启用 C2C）。
type UserAdminStore interface {
	UserReader
	Update(*userdomain.User) error
}

// ExpireTaskScheduler 事务提交后异步排期交易超时任务，失败不阻塞主流程。
type ExpireTaskScheduler interface {
	EnqueueC2CTradeExpire(tradeID uint, delay time.Duration) error
}

// Notifier 事务提交后异步发送 C2C 通知，失败不阻塞资金主流程。
type Notifier interface {
	Enqueue(input notificationcontract.EnqueueInput) error
}

// ArbitrationAuditWriter 记录管理员仲裁审计日志（事务提交后调用）。
type ArbitrationAuditWriter interface {
	WriteC2CArbitration(entry ArbitrationAuditEntry) error
}
