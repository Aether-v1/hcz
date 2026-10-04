package domain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// Listing C2C 挂单。side 固定为 SELL（第一版仅支持卖家挂单卖出 USDT）。
type Listing struct {
	ID            uint         `gorm:"primarykey" json:"id"`
	ListingNo     string       `gorm:"type:varchar(64);uniqueIndex;not null" json:"listing_no"` // 全局唯一单号
	SellerUserID  uint         `gorm:"index;not null" json:"seller_user_id"`
	FiatCurrency  string       `gorm:"index:idx_c2c_list_status_fiat,priority:2;type:varchar(8);not null;default:'CNY'" json:"fiat_currency"`    // 法币币种，如 CNY
	Price         money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price"`                                                       // 单价（法币/USDT）
	MinFiatAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"min_fiat_amount"`                                             // 最小法币成交额
	MaxFiatAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"max_fiat_amount"`                                             // 最大法币成交额
	TotalUSDT     money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"total_usdt"`                                                  // 挂单总量
	AvailableUSDT money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"available_usdt"`                                              // 可售余量
	Status        string       `gorm:"index;index:idx_c2c_list_status_fiat,priority:1;type:varchar(16);not null;default:'active'" json:"status"` // active/paused/closed
	Terms         string       `gorm:"type:text;default:''" json:"terms"`                                                                        // 交易条款说明
	CreatedAt     time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time    `gorm:"index" json:"updated_at"`
}

// TableName 指定表名。
func (Listing) TableName() string {
	return "c2c_listings"
}
