package domain

import "time"

// Account 是用户积分账户（余额缓存）。
//
// 积分是 BIGINT 整数虚拟资产，与 USDT 钱包（decimal 资金）严格隔离：
//   - balance 恒等于 points_ledger 的净累计（SUM(amount)），由 mutation service 单点维护；
//   - total_earned / total_spent 是生命周期累计统计，语义由 action 元数据驱动，
//     Admin 扣减 / 系统冲正不计入任何一侧（它们只影响 balance）。
//
// 账户在首次积分 mutation 时创建；查询接口对"从未产生积分"的用户返回零值而非 404。
type Account struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"column:user_id;not null;uniqueIndex" json:"user_id"`
	Balance     int64     `gorm:"column:balance;not null;default:0" json:"balance"`
	TotalEarned int64     `gorm:"column:total_earned;not null;default:0" json:"total_earned"`
	TotalSpent  int64     `gorm:"column:total_spent;not null;default:0" json:"total_spent"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (Account) TableName() string { return "points_accounts" }
