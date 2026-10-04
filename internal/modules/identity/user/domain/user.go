package userdomain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// User 用户表
type User struct {
	ID                    uint         `gorm:"primarykey" json:"id"`                                         // 主键
	Email                 string       `gorm:"uniqueIndex;not null" json:"email"`                            // 邮箱
	PasswordHash          string       `gorm:"not null" json:"-"`                                            // 密码哈希（不返回给前端）
	PasswordSetupRequired bool         `gorm:"not null;default:false" json:"-"`                              // 是否需要首次设置密码（Telegram 自动建号场景）
	DisplayName           string       `gorm:"default:''" json:"display_name"`                               // 昵称
	Locale                string       `gorm:"default:'zh-CN'" json:"locale"`                                // 语言偏好
	Status                string       `gorm:"default:'active'" json:"status"`                               // 账号状态
	C2CBanned             bool         `gorm:"not null;default:false" json:"c2c_banned"`                     // C2C 交易禁用标记
	MemberLevelID         uint         `gorm:"not null;default:0" json:"member_level_id"`                    // 当前会员等级ID
	TotalRecharged        money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"total_recharged"` // 充值累计
	TotalSpent            money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"total_spent"`     // 消费累计
	AdminNote             string       `gorm:"type:text;default:''" json:"admin_note,omitempty"`             // 管理员备注（仅后台可见）
	TokenVersion          uint64       `gorm:"not null;default:0" json:"-"`                                  // Token 版本（用于全量失效）
	TokenInvalidBefore    *time.Time   `gorm:"index" json:"-"`                                               // 该时间点前签发的 Token 失效
	TOTPSecret            string       `gorm:"type:varchar(512);default:''" json:"-"`                        // AES-GCM 加密后的 hex 密文，未启用为空
	TOTPEnabledAt         *time.Time   `gorm:"index" json:"totp_enabled_at,omitempty"`                       // 启用时间，NULL 表示未启用
	TOTPPendingSecret     string       `gorm:"type:varchar(512);default:''" json:"-"`                        // 绑定流程中尚未首次验证的 secret（加密）
	TOTPPendingExpiresAt  *time.Time   `json:"-"`                                                            // 待绑定 secret 过期时间（10 分钟）
	RecoveryCodes         string       `gorm:"type:text;default:''" json:"-"`                                // jsonmap.JSON 数组：[{"hash":"...","used_at":null|"..."}]
	EmailVerifiedAt       *time.Time   `json:"email_verified_at"`                                            // 邮箱验证时间
	LastLoginAt           *time.Time   `json:"last_login_at"`                                                // 最后登录时间
	// 邀请绑定（Phase 2）：仅记录直接上下级关系，不涉及多级返利计算。
	InviterID     *uint      `gorm:"index" json:"inviter_id,omitempty"`                       // 直接上级用户ID，可空（无上级）
	InviteCode    string     `gorm:"type:varchar(12);not null;default:''" json:"invite_code"` // 个人邀请码（全局唯一，唯一索引由 migration 在 backfill 后创建）
	InviteBoundAt *time.Time `gorm:"index" json:"invite_bound_at,omitempty"`                  // 绑定上级的时间，未绑定为 NULL
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`                                 // 创建时间
	UpdatedAt     time.Time  `gorm:"index" json:"updated_at"`                                 // 更新时间
	DeletedAt     *time.Time `gorm:"index" json:"-"`                                          // 软删除时间
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
