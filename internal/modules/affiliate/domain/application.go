package domain

import (
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
)

// Application 推广申请（用户提交 → 管理员审核 → 通过后开通 active Profile）。
// 与 Profile 状态分离：Application 有 pending/approved/rejected，Profile 只有 active/disabled。
type Application struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	Status     string     `gorm:"type:varchar(20);not null;index" json:"status"` // pending/approved/rejected
	Reason     string     `gorm:"type:varchar(500)" json:"reason"`               // 申请原因（可选）
	ReviewNote string     `gorm:"type:varchar(500)" json:"review_note"`          // 审核备注
	ReviewedBy uint       `gorm:"index" json:"reviewed_by"`                      // 审核管理员ID（0=未审核/migration）
	ReviewedAt *time.Time `gorm:"index" json:"reviewed_at,omitempty"`            // 审核时间
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"index" json:"updated_at"`

	User userdomain.User `gorm:"foreignKey:UserID" json:"user,omitempty"` // 用户信息
}

// TableName 指定表名
func (Application) TableName() string {
	return "affiliate_applications"
}
