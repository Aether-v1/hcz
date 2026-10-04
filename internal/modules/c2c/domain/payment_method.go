package domain

import (
	"time"
)

// PaymentMethod C2C 支付方式。
type PaymentMethod struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	UserID            uint      `gorm:"index;index:idx_c2c_pm_user_enabled,priority:1;not null" json:"user_id"`
	Type              string    `gorm:"type:varchar(32);not null" json:"type"`                           // bank_card/alipay/wechat
	AccountName       string    `gorm:"type:varchar(128);not null;default:''" json:"account_name"`       // 账户户名
	AccountIdentifier string    `gorm:"type:varchar(256);not null;default:''" json:"account_identifier"` // 卡号/账号/收款码标识
	QRImage           string    `gorm:"type:varchar(512);default:''" json:"qr_image"`                    // 收款二维码图片 URL
	Instructions      string    `gorm:"type:text;default:''" json:"instructions"`                        // 转账备注说明
	Enabled           bool      `gorm:"index:idx_c2c_pm_user_enabled,priority:2;not null;default:true" json:"enabled"`
	CreatedAt         time.Time `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time `gorm:"index" json:"updated_at"`
}

// TableName 指定表名。
func (PaymentMethod) TableName() string {
	return "c2c_payment_methods"
}
