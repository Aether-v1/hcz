package contract

import (
	"time"

	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
)

// Repository 拥有积分聚合的持久化。
// 事务调用方通过 Transaction 拿到绑定到同一事务的同一端口。
type Repository interface {
	GetAccountByUserID(userID uint) (*pointsdomain.Account, error)
	// GetAccountByUserIDForUpdate 以 SELECT ... FOR UPDATE 行锁读取账户（并发安全）。
	GetAccountByUserIDForUpdate(userID uint) (*pointsdomain.Account, error)
	CreateAccount(account *pointsdomain.Account) error
	UpdateAccount(account *pointsdomain.Account) error

	// CreateLedgerEntry 仅 INSERT（append-only），不允许 UPDATE/DELETE。
	CreateLedgerEntry(entry *pointsdomain.LedgerEntry) error
	// GetLedgerEntryByReference 幂等查重（reference 全局唯一）。
	GetLedgerEntryByReference(reference string) (*pointsdomain.LedgerEntry, error)
	// SumOrderRewards 聚合某订单的奖励与已冲正积分（P1 部分退款累计算法用）。
	// reward  = Σ amount WHERE order_id=? AND action_type=ORDER_REWARD（正数）
	// reversed = Σ |amount| WHERE order_id=? AND action_type=ORDER_REWARD_REVERSAL（正数）
	SumOrderRewards(orderID uint) (reward int64, reversed int64, err error)
	ListLedgerEntries(filter LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error)

	// ListAccounts 分页查询积分账户（P4 负余额排查；UserID=0 表示不限用户）。
	ListAccounts(filter AccountListFilter) ([]AccountWithUser, int64, error)
}

// LedgerStatsReader 是运营统计所需的流水聚合只读端口（由积分 store 实现）。
type LedgerStatsReader interface {
	// AggregateLedgerRange 聚合 [from,to) 区间内流水的入账/出账/订单奖励合计与条数。
	AggregateLedgerRange(from, to time.Time) (LedgerAggregate, error)
	// SumBalances 返回全平台余额总量与负余额账户数。
	SumBalances() (total int64, negativeAccounts int64, err error)
}

// CheckinStatsReader 是统计所需的签到只读端口（由签到 store 实现，按业务日边界）。
type CheckinStatsReader interface {
	CountCheckinUsers(from, to time.Time) (int64, error)
}

// ExchangeStatsReader 是统计所需的商城兑换只读端口（由商城 store 实现）。
type ExchangeStatsReader interface {
	CountExchangeOrdersCreated(from, to time.Time) (int64, error)
	CountPendingExchangeOrders() (int64, error)
	CountProcessingExchangeOrders() (int64, error)
}

// StatsPorts 聚合统计相关的外部只读端口（装配期注入，缺失即统计不可用）。
type StatsPorts struct {
	Ledger    LedgerStatsReader
	Checkins  CheckinStatsReader
	Exchanges ExchangeStatsReader
	// BusinessDayRange 返回给定时刻所属业务日的 [from,to) UTC 边界。
	// 单一权威实现来自签到域时钟（Asia/Shanghai），积分不自建第二套时区口径。
	BusinessDayRange func(now time.Time) (time.Time, time.Time)
}

// Transaction 是已打开数据库事务的积分域视图，不暴露 ORM 原语。
type Transaction interface {
	Points() Repository
}

// UnitOfWork 暴露开启事务的能力。
type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// UseCase 是积分模块对外的应用端口。
type UseCase interface {
	// GetAccount 返回用户积分账户；从未产生积分的用户返回零值账户（nil 语义下返回 nil, nil，由调用方归一化为 0）。
	GetAccount(userID uint) (*pointsdomain.Account, error)
	ListLedgerEntries(filter LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error)

	// AdminAdjust 管理员增减积分（统一 mutation 入口）。
	AdminAdjust(input AdjustInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error)
	// AdminCompensate 管理员人工补偿（P4）：独立 action，仍走积分核心，
	// 不复用 ORDER_REWARD，保证审计可辨认"人工补发"。
	AdminCompensate(input CompensateInput) (*pointsdomain.Account, *pointsdomain.LedgerEntry, error)

	// ListAccounts 分页查询积分账户（P4，支持 balance<0 排查）。
	ListAccounts(filter AccountListFilter) ([]AccountWithUser, int64, error)
	// GetStats 运营统计基础指标（P4，直接聚合事实表，不引入累计总表）。
	GetStats(now time.Time) (*StatsResult, error)

	// RewardOrderCompleted 订单进入 COMPLETED 后发放积分（P1）。
	// 必须在调用方事务内执行（tx 为订单事务的 points 视图）；reference 唯一保证幂等。
	RewardOrderCompleted(tx Transaction, input OrderRewardInput) error
	// ReverseOrderReward 订单退款时按比例冲正积分（P1）。必须在调用方事务内执行。
	ReverseOrderReward(tx Transaction, input OrderReversalInput) error
	// OrderRewardSummary 返回订单奖励与已冲正累计（P1 部分退款累计算法用）。必须在调用方事务内执行。
	OrderRewardSummary(tx Transaction, orderID uint) (reward int64, reversed int64, err error)

	// CheckinReward 每日签到发放积分（P2）。
	// 必须在调用方（签到事务）事务内执行；reference（points:checkin:{user_id}:{YYYY-MM-DD}）
	// 全局唯一保证重复签到只奖励一次。
	CheckinReward(tx Transaction, input CheckinRewardInput) error

	// Redeem 积分商城兑换扣减（P3）。
	// 必须在调用方（兑换事务）事务内执行；reference（points:redeem:{exchange_order_id}）
	// 全局唯一保证同一兑换订单最多扣减一次。用户主动消费，禁止余额变负（ErrNegativeNotAllowed）。
	Redeem(tx Transaction, input RedeemInput) (*pointsdomain.LedgerEntry, error)

	// RedeemRefund 积分商城兑换返还（P3）。
	// 必须在调用方（兑换失败/取消事务）事务内执行；reference（points:redeem_refund:{exchange_order_id}）
	// 全局唯一保证同一兑换订单最多返还一次（数据库唯一索引是最终防线）。
	// 幂等语义：若该订单已有 REDEEM_REFUND，直接返回已存在流水（不重复返还）。
	RedeemRefund(tx Transaction, input RedeemRefundInput) (*pointsdomain.LedgerEntry, error)
}
