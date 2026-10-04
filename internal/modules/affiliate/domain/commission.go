package domain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// Commission 推广返利佣金记录
type Commission struct {
	ID                 uint  `gorm:"primarykey" json:"id"`                                                                   // 主键
	AffiliateProfileID uint  `gorm:"not null;index" json:"affiliate_profile_id"`                                             // 推广用户ID（提现关联与历史兼容，多级别下不再构成唯一约束）
	OrderID            uint  `gorm:"not null;index;index:idx_affiliate_commission_multilevel_unique,unique" json:"order_id"` // 订单ID
	OrderItemID        *uint `gorm:"index" json:"order_item_id,omitempty"`                                                   // 订单项ID
	// CommissionType 纳入唯一索引：正常订单佣金为 "order"，提现拆分产生的余留行使用 "sp..." 前缀，
	// 二者共享同一 (order, beneficiary, level) 但靠 commission_type 区分，避免唯一约束冲突。
	CommissionType string `gorm:"type:varchar(20);not null;default:'order';index;index:idx_affiliate_commission_multilevel_unique,unique" json:"commission_type"` // 佣金类型
	// BeneficiaryUserID 是该笔佣金的收益人用户ID（多级别返利核心维度）。
	// 历史数据由迁移脚本从 affiliate_profiles.user_id 回填，默认 1 级。
	BeneficiaryUserID uint `gorm:"not null;default:0;index;index:idx_affiliate_commission_multilevel_unique,unique" json:"beneficiary_user_id"`
	// SourceUserID 是下单用户ID（佣金来源），可空；历史数据可能无法回填。
	SourceUserID *uint `gorm:"index" json:"source_user_id,omitempty"`
	// Level 是真实邀请链层级（1-10），中间用户无推广资格时不压缩层级。
	Level             int          `gorm:"not null;default:1;index;index:idx_affiliate_commission_multilevel_unique" json:"level"`
	BaseAmount        money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"base_amount"`       // 佣金基数金额（USDT）
	RatePercent       money.Amount `gorm:"type:decimal(10,2);not null;default:0" json:"rate_percent"`      // 佣金比例（百分比）
	CommissionAmount  money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"commission_amount"` // 佣金金额（USDT）
	Status            string       `gorm:"type:varchar(32);not null;index" json:"status"`                  // 佣金状态
	ConfirmAt         *time.Time   `gorm:"index" json:"confirm_at,omitempty"`                              // 待确认到期时间
	AvailableAt       *time.Time   `gorm:"index" json:"available_at,omitempty"`                            // 转可提现时间
	WithdrawRequestID *uint        `gorm:"index" json:"withdraw_request_id,omitempty"`                     // 关联提现申请
	InvalidReason     string       `gorm:"type:varchar(255)" json:"invalid_reason"`                        // 失效原因
	CreatedAt         time.Time    `gorm:"index" json:"created_at"`                                        // 创建时间
	UpdatedAt         time.Time    `gorm:"index" json:"updated_at"`                                        // 更新时间
	DeletedAt         *time.Time   `gorm:"index" json:"-"`                                                 // 软删除时间

	AffiliateProfile Profile          `gorm:"foreignKey:AffiliateProfileID" json:"affiliate_profile,omitempty"` // 推广用户
	Order            OrderReference   `gorm:"foreignKey:OrderID" json:"order,omitempty"`                        // 关联订单
	WithdrawRequest  *WithdrawRequest `gorm:"foreignKey:WithdrawRequestID" json:"withdraw_request,omitempty"`   // 提现申请
}

// TableName 指定表名
func (Commission) TableName() string {
	return "affiliate_commissions"
}
