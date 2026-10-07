package contract

import (
	"time"

	procurementdomain "github.com/Aether-v1/hcz/internal/modules/procurement/domain"
)

type Repository interface {
	GetByID(id uint) (*procurementdomain.Order, error)
	GetByLocalOrderID(localOrderID uint) (*procurementdomain.Order, error)
	GetByLocalOrderNo(localOrderNo string) (*procurementdomain.Order, error)
	Create(order *procurementdomain.Order) error
	UpdateStatus(id uint, status string, updates map[string]interface{}) error
	List(filter ListFilter) ([]procurementdomain.Order, int64, error)
	StatsByStatus(filter ListFilter) (map[string]int64, error)
	ListByConnectionAndTimeRange(connectionID uint, start, end time.Time) ([]procurementdomain.Order, error)
}

type OrderRepository interface {
	GetByID(id uint) (*procurementdomain.LocalOrder, error)
	GetByIDs(ids []uint) ([]procurementdomain.LocalOrder, error)
	UpdateStatus(id uint, status string, updates map[string]interface{}) error
}

type ProductMappingReader interface {
	FindConnectionID(productID uint) (connectionID uint, found bool, err error)
}

type SKUMappingReader interface {
	FindUpstreamSKUID(skuID uint) (upstreamSKUID uint, found bool, err error)
}

type ConnectionProvider interface {
	Open(connectionID uint) (UpstreamConnection, error)
}

type Enqueuer interface {
	EnqueueSubmit(procurementOrderID uint) error
	EnqueuePoll(procurementOrderID uint, delay time.Duration) error
}

type OrderLifecycle interface {
	CreateUpstreamFulfillment(orderID uint, fulfillment *Fulfillment, now time.Time) error
	SyncParentStatus(parentID uint, now time.Time) (string, error)
	EnqueueStatusEmail(orderID uint, status string) (skipped bool, err error)
	// CompleteOrder 本地订单进入 completed 的统一入口（自开事务，行锁 + 状态校验 + Affiliate/Points 副作用，幂等）。
	// 替代"直接 UpdateStatus(completed) 绕过完成生命周期"的旧做法。
	CompleteOrder(orderID uint) error
	// CompleteParentSideEffects 父订单 completed 的统一副作用入口（自开事务，幂等）。
	CompleteParentSideEffects(parentID uint) error
}

// OrderCompletion 是采购侧委托订单域统一完成生命周期的端口（P1）。
// 由 container 注入 order application 的 OrderService 适配器；实现侧负责
// Affiliate Commission + Points Reward 与订单状态更新的同事务原子性。
type OrderCompletion interface {
	CompleteOrder(orderID uint) error
	CompleteParentSideEffects(parentID uint) error
}

type DownstreamCallbackEnqueuer interface {
	EnqueueCallback(orderID uint)
}

type BotFulfillmentNotifier interface {
	NotifyBotOrderFulfilled(userID, orderID uint)
}

type FailureNotifier interface {
	NotifyFailure(order *procurementdomain.Order, message string) error
}
