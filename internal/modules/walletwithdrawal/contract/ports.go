package contract

import (
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	notificationcontract "github.com/Aether-v1/hcz/internal/modules/notification/contract"
	settingssecurity "github.com/Aether-v1/hcz/internal/modules/settings/schema/security"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/shared/money"
	"github.com/shopspring/decimal"
)

// Repository 拥有提现单与地址簿的持久化。事务调用方通过 Transaction 拿到绑定同一事务的实现。
type Repository interface {
	CreateWithdrawal(w *withdrawaldomain.Withdrawal) error
	UpdateWithdrawal(w *withdrawaldomain.Withdrawal) error
	GetWithdrawalByID(id uint) (*withdrawaldomain.Withdrawal, error)
	GetWithdrawalByIDForUpdate(id uint) (*withdrawaldomain.Withdrawal, error)
	GetWithdrawalByReference(reference string) (*withdrawaldomain.Withdrawal, error)
	ListWithdrawals(filter WithdrawalListFilter) ([]withdrawaldomain.Withdrawal, int64, error)
	ListAdminWithdrawals(filter AdminWithdrawalListFilter) ([]withdrawaldomain.Withdrawal, int64, error)
	// SumUserActiveSince 统计用户自 since 起（不含 rejected/canceled）的提现金额合计。
	SumUserActiveSince(userID uint, since time.Time) (decimal.Decimal, error)
	CountUserActiveSince(userID uint, since time.Time) (int64, error)
	// CountUserCompletedBefore 统计用户在 before 之前已完成的提现笔数（用于首提限额）。
	CountUserCompletedBefore(userID uint, before time.Time) (int64, error)
	// NextWithdrawalNoSequence 统计当日已生成的提现单数，用于生成单号。
	CountCreatedOnDay(userID uint, dayStart time.Time) (int64, error)

	CreateAddress(a *withdrawaldomain.Address) error
	UpdateAddress(a *withdrawaldomain.Address) error
	DeleteAddress(id uint) error
	GetAddressByID(id uint) (*withdrawaldomain.Address, error)
	ListAddressesByUserID(userID uint) ([]withdrawaldomain.Address, error)
	GetAddressByUserIDNetworkAddress(userID uint, network, address string) (*withdrawaldomain.Address, error)
	ClearDefaultAddress(userID uint) error
}

// Transaction 是已开启的数据库事务在提现上下文中的视图。
// Wallets() 返回绑定同一事务的钱包仓库，用于行锁账户、写 ledger。
type Transaction interface {
	Withdrawals() Repository
	Wallets() walletcontract.Repository
}

// UnitOfWork 开启提现事务。
type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// TOTPVerifier 校验用户 TOTP 挑战码（fail-closed：未启用返回 ErrNotEnabled）。
type TOTPVerifier interface {
	VerifyChallengeCode(userID uint, code string) error
}

// ConfigReader 读取提现全局配置。
type ConfigReader interface {
	GetWithdrawalConfig() (settingssecurity.WithdrawalConfig, error)
}

// Notifier 事务提交后异步发送通知，失败不阻塞主流程。
type Notifier interface {
	Enqueue(input notificationcontract.EnqueueInput) error
}

// UserReader 读取用户注册时间，用于新用户冷却校验。
type UserReader interface {
	GetByID(userID uint) (*userdomain.User, error)
}

// FeeCalculator 手续费计算（fixed + percentage，2dp 舍入）。
type FeeCalculator interface {
	Calculate(amount money.Amount, fixedFee, percentageFee decimal.Decimal) FeeQuote
}
