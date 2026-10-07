package domain

import (
	"time"

	"gorm.io/gorm"
)

// 支付方式类型白名单（统一大写）。
const (
	PaymentMethodTypeUSDTTRC20 = "USDT_TRC20"
	PaymentMethodTypeBankCard  = "BANK_CARD"
	PaymentMethodTypeAlipay    = "ALIPAY"
	PaymentMethodTypeWechat    = "WECHAT"
)

// 支付方式状态。
const (
	PaymentMethodStatusActive   = "active"
	PaymentMethodStatusDisabled = "disabled"
)

// PaymentMethod C2C / 钱包统一收款方式（扩展自 c2c_payment_methods，不新建表）。
//
// 敏感字段（Address / AccountName / BankAccount / AccountIdentifier）在 service 层
// 用 AES-256-GCM 加密后存储；读取时解密。列表接口由 presenter 脱敏返回。
type PaymentMethod struct {
	ID       uint `gorm:"primarykey" json:"id"`
	UserID   uint `gorm:"index;index:idx_c2c_pm_user_enabled,priority:1;not null" json:"user_id"`
	Type     string `gorm:"type:varchar(32);not null" json:"type"` // USDT_TRC20/BANK_CARD/ALIPAY/WECHAT
	Currency string `gorm:"type:varchar(16);not null;default:''" json:"currency"` // USDT（仅数字资产）
	Network  string `gorm:"type:varchar(32);not null;default:''" json:"network"`  // TRC20（仅数字资产）
	Address  string `gorm:"type:varchar(256);not null;default:''" json:"address,omitempty"` // 加密后的 USDT 地址
	Label    string `gorm:"type:varchar(64);not null;default:''" json:"label"`               // 标签，如"我的钱包"

	// 法币收款信息
	AccountName       string `gorm:"type:varchar(256);not null;default:''" json:"account_name"`        // 收款人姓名（加密存储）
	BankName          string `gorm:"type:varchar(128);not null;default:''" json:"bank_name"`          // 银行名称（仅 BANK_CARD）
	BankAccount       string `gorm:"type:varchar(128);not null;default:''" json:"bank_account"`         // 银行卡号（加密存储，仅 BANK_CARD）
	BranchName        string `gorm:"type:varchar(256);not null;default:''" json:"branch_name"`         // 开户行（可选，仅 BANK_CARD）
	AccountIdentifier string `gorm:"type:varchar(256);not null;default:''" json:"account_identifier"` // 支付宝/微信号（加密存储）

	// 二维码：统一走文件上传服务，禁止 base64 存库。
	QRCodeFileID string `gorm:"type:varchar(128);not null;default:''" json:"qr_code_file_id"` // 上传服务返回的文件 ID
	QRCodeURL    string `gorm:"type:varchar(512);not null;default:''" json:"qr_code_url"`      // 上传服务返回的访问 URL

	IsDefault bool   `gorm:"not null;default:false" json:"is_default"` // 同类型内互斥默认
	Status    string `gorm:"type:varchar(16);not null;default:'active'" json:"status"`          // active/disabled

	// 旧字段保留向后兼容；新代码使用 QRCodeFileID + QRCodeURL。
	QRImage      string `gorm:"type:varchar(512);default:''" json:"qr_image,omitempty"` // Deprecated
	Instructions string `gorm:"type:text;default:''" json:"instructions"`                // 转账备注说明

	Enabled   bool           `gorm:"index:idx_c2c_pm_user_enabled,priority:2;not null;default:true" json:"enabled"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 软删除
}

// TableName 指定表名。
func (PaymentMethod) TableName() string {
	return "c2c_payment_methods"
}
