package domain

import "time"

// ExchangeOrderStatus 是积分兑换订单状态。
// V1 状态机（终态不可转移；本轮不做 COMPLETED 售后回滚，售后属未来独立设计）：
//
//	PENDING    → PROCESSING / CANCELLED / FAILED
//	PROCESSING → COMPLETED / FAILED / CANCELLED（Admin）
//	COMPLETED / FAILED / CANCELLED：终态
//
// 语义：
//   - PENDING：兑换成功，积分已扣除，等待后台处理；
//   - PROCESSING：管理员开始处理；
//   - COMPLETED：履约完成（终态，本轮不允许自动退款）；
//   - FAILED：履约失败，必须返还积分（REDEEM_REFUND）并恢复库存；
//   - CANCELLED：取消（用户仅可取消 PENDING；Admin 可取消 PENDING/PROCESSING），
//     必须返还积分并恢复库存。
const (
	ExchangeStatusPending    = "PENDING"
	ExchangeStatusProcessing = "PROCESSING"
	ExchangeStatusCompleted  = "COMPLETED"
	ExchangeStatusFailed     = "FAILED"
	ExchangeStatusCancelled  = "CANCELLED"
)

// terminalStatuses 是终态集合。
var terminalStatuses = map[string]bool{
	ExchangeStatusCompleted: true,
	ExchangeStatusFailed:    true,
	ExchangeStatusCancelled: true,
}

// IsTerminalExchangeStatus 判断状态是否为终态。
func IsTerminalExchangeStatus(status string) bool {
	return terminalStatuses[status]
}

// allStatuses 是兑换订单状态全集（查询过滤白名单）。
var allStatuses = map[string]bool{
	ExchangeStatusPending:    true,
	ExchangeStatusProcessing: true,
	ExchangeStatusCompleted:  true,
	ExchangeStatusFailed:     true,
	ExchangeStatusCancelled:  true,
}

// IsValidExchangeStatus 判断状态过滤值是否属于登记枚举（空值表示不限）。
func IsValidExchangeStatus(status string) bool {
	return status == "" || allStatuses[status]
}

// CanTransition 校验状态转换合法性（禁止 FAILED→PROCESSING / CANCELLED→COMPLETED 等回退）。
// 允许转换集（V1 锁定）：
//
//	PENDING    → PROCESSING / FAILED / CANCELLED
//	PROCESSING → COMPLETED / FAILED / CANCELLED（Admin）
//
// PENDING 不允许直转 COMPLETED：履约完成前必须先经管理员开始处理。
func CanTransition(from, to string) bool {
	switch from {
	case ExchangeStatusPending:
		return to == ExchangeStatusProcessing ||
			to == ExchangeStatusFailed ||
			to == ExchangeStatusCancelled
	case ExchangeStatusProcessing:
		return to == ExchangeStatusCompleted ||
			to == ExchangeStatusFailed ||
			to == ExchangeStatusCancelled
	default:
		return false
	}
}

// ExchangeOrder 是积分兑换订单（独立于 recharge orders，禁止复用订单表）。
//
// 快照语义：创建时固化 product_name / unit_points / fulfillment_type，
// 商品改价/改名/下架不影响历史订单；失败返还按订单 total_points，而非当前商品价格。
// 幂等：idempotency_key 与 user_id 联合唯一（同一用户同 Key 只创建一单）。
// 审计：reason + last_operator_* 记录最近一次状态变更的操作者与原因；
// 积分返还事实以 points_ledger 的 REDEEM_REFUND（reference 全局唯一）为准。
type ExchangeOrder struct {
	ID                      uint       `gorm:"primaryKey" json:"id"`
	OrderNo                 string     `gorm:"column:order_no;size:32;not null;uniqueIndex" json:"order_no"`
	UserID                  uint       `gorm:"column:user_id;not null;index:idx_exchange_user_status,priority:1;uniqueIndex:idx_exchange_user_idem,priority:1" json:"user_id"`
	ProductID               uint       `gorm:"column:product_id;not null;index" json:"product_id"`
	ProductNameSnapshot     string     `gorm:"column:product_name_snapshot;size:120;not null" json:"product_name_snapshot"`
	UnitPoints              int64      `gorm:"column:unit_points;not null" json:"unit_points"`
	Quantity                int64      `gorm:"column:quantity;not null;default:1" json:"quantity"`
	TotalPoints             int64      `gorm:"column:total_points;not null" json:"total_points"`
	Status                  string     `gorm:"column:status;size:16;not null;index:idx_exchange_user_status,priority:2" json:"status"`
	FulfillmentTypeSnapshot string     `gorm:"column:fulfillment_type_snapshot;size:16;not null;default:'MANUAL'" json:"fulfillment_type_snapshot"`
	IdempotencyKey          string     `gorm:"column:idempotency_key;size:64;not null;uniqueIndex:idx_exchange_user_idem,priority:2" json:"-"`
	Reason                  string     `gorm:"column:reason;size:255;not null;default:''" json:"reason"`
	LastOperatorType        string     `gorm:"column:last_operator_type;size:16;not null;default:''" json:"-"`
	LastOperatorID          uint       `gorm:"column:last_operator_id;not null;default:0" json:"-"`
	CompletedAt             *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	FailedAt                *time.Time `gorm:"column:failed_at" json:"failed_at,omitempty"`
	CancelledAt             *time.Time `gorm:"column:cancelled_at" json:"cancelled_at,omitempty"`
	CreatedAt               time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt               time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (ExchangeOrder) TableName() string { return "points_exchange_orders" }
