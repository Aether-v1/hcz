package domain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// 用户站内通知类型枚举（Phase 1 仅接入 order_processing / order_completed / wallet_recharge，
// 其余常量先行定义，后续阶段再接线）。
const (
	// TypeOrderProcessing 订单进入处理中（pending_recharge -> processing）。
	TypeOrderProcessing = "order_processing"
	// TypeOrderCompleted 订单已完成（processing -> completed）。
	TypeOrderCompleted = "order_completed"
	// TypeWalletRecharge 钱包充值到账。
	TypeWalletRecharge = "wallet_recharge"
	// TypeRefundSuccess 退款到账（Phase 1 不接入）。
	TypeRefundSuccess = "refund_success"
	// TypeAfterSaleUpdate 售后状态更新（Phase 1 不接入）。
	TypeAfterSaleUpdate = "aftersale_update"
	// TypeCommissionConfirmed 返利到账可提现（Phase 1 不接入）。
	TypeCommissionConfirmed = "commission_confirmed"
	// TypeOrderCanceled 订单已取消/失败（Phase 1 不接入）。
	TypeOrderCanceled = "order_canceled"
)

// 业务关联类型（biz_type）枚举，配合 biz_id 定位业务单据。
const (
	BizTypeOrder          = "order"
	BizTypeWalletRecharge = "wallet_recharge"
	BizTypeRefund         = "refund"
	BizTypeAfterSale      = "after_sale"
	BizTypeCommission     = "commission"
)

// UserNotification 用户站内通知（用户收件箱）。
//
// 与管理员告警中心 notification_logs 物理隔离：本表带 user_id 定向收件人与
// is_read/read_at 已读状态，面向 C 端用户；notification_logs 面向配置式管理员名单。
//
// 幂等核心：(user_id, biz_type, biz_id, type) 复合唯一索引。
// 注意：SQLite/MySQL 对唯一索引中的 NULL 不做等值比较（多个 NULL 互不冲突），
// 因此 biz_type/biz_id 一律 NOT NULL（空串/0 兜底），确保回调重放被唯一索引拦截、不产生重复通知。
type UserNotification struct {
	ID        uint         `gorm:"primarykey" json:"id"`
	UserID    uint         `gorm:"not null;default:0;uniqueIndex:uniq_user_notify,priority:1;index:idx_unotif_read,priority:1;index:idx_unotif_created,priority:1" json:"user_id"`
	Type      string       `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uniq_user_notify,priority:4" json:"type"`
	Title     string       `gorm:"type:varchar(255);not null;default:''" json:"title"`
	Body      string       `gorm:"type:text" json:"body"`
	Data      jsonmap.JSON `gorm:"type:json" json:"data"`
	BizType   string       `gorm:"type:varchar(32);not null;default:'';uniqueIndex:uniq_user_notify,priority:2" json:"biz_type"`
	BizID     uint         `gorm:"not null;default:0;uniqueIndex:uniq_user_notify,priority:3" json:"biz_id"`
	IsRead    bool         `gorm:"not null;default:false;index:idx_unotif_read,priority:2" json:"is_read"`
	ReadAt    *time.Time   `json:"read_at"`
	CreatedAt time.Time    `gorm:"index:idx_unotif_created,priority:2" json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// TableName 指定表名。
func (UserNotification) TableName() string {
	return "user_notifications"
}
