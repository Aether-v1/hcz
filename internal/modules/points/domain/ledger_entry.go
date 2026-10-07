package domain

import "time"

// LedgerEntry 是积分账本条目（append-only：只 INSERT，禁止 UPDATE / DELETE）。
//
// 余额变化只能通过新增一条 LedgerEntry 表达；净余额 = SUM(amount)。
// 修正历史（退款冲正 / 兑换返还）一律新增 REVERSAL/REFUND 类条目，绝不改写原条目。
//
// 幂等设计：
//   - reference 全局唯一（最终防线，数据库唯一索引）；
//   - source_type / source_id / action_type 表达"一次业务事件的唯一入账"，
//     供未来订单奖励（order_reward:order_id:reward）、兑换扣减（redeem:exchange_order_id:deduct）
//     等按事件去重；Admin 调整不受该三元组约束（同用户多次调整合法），其幂等完全依赖 reference。
//
// 语义约定：
//   - Amount 为有符号整数，正=入账，负=出账；
//   - OperatorType 为 admin / system / user；Admin 操作必须同时写 OperatorID（admin_id）；
//   - CheckinDate / OrderID / ExchangeOrderID 为后续 Phase 预留的维度列，本轮不产生相关逻辑。
type LedgerEntry struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"column:user_id;not null;index:idx_points_ledger_user_created,priority:1" json:"user_id"`
	ActionType      string     `gorm:"column:action_type;size:32;not null" json:"action_type"`
	SourceType      string     `gorm:"column:source_type;size:32;not null" json:"source_type"`
	SourceID        uint       `gorm:"column:source_id;not null" json:"source_id"`
	Amount          int64      `gorm:"column:amount;not null" json:"amount"`
	BalanceBefore   int64      `gorm:"column:balance_before;not null" json:"balance_before"`
	BalanceAfter    int64      `gorm:"column:balance_after;not null" json:"balance_after"`
	Reference       string     `gorm:"column:reference;size:120;not null;uniqueIndex" json:"reference"`
	Reason          string     `gorm:"column:reason;size:255;not null" json:"reason"`
	OperatorType    string     `gorm:"column:operator_type;size:16;not null" json:"operator_type"`
	OperatorID      uint       `gorm:"column:operator_id;not null" json:"operator_id"`
	OrderID         *uint      `gorm:"column:order_id;index" json:"order_id,omitempty"`
	CheckinDate     *time.Time `gorm:"column:checkin_date;type:date" json:"checkin_date,omitempty"`
	ExchangeOrderID *uint      `gorm:"column:exchange_order_id;index" json:"exchange_order_id,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;index:idx_points_ledger_user_created,priority:2" json:"created_at"`
}

func (LedgerEntry) TableName() string { return "points_ledger" }
