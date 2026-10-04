package domain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/money"
)

// Trade C2C 交易单。
type Trade struct {
	ID                    uint         `gorm:"primarykey" json:"id"`
	TradeNo               string       `gorm:"type:varchar(64);uniqueIndex;not null" json:"trade_no"` // 全局唯一单号
	ListingID             uint         `gorm:"index;not null" json:"listing_id"`
	BuyerUserID           uint         `gorm:"index;uniqueIndex:idx_c2c_trade_idem,priority:1;not null" json:"buyer_user_id"`
	SellerUserID          uint         `gorm:"index;not null" json:"seller_user_id"`
	IdempotencyKey        string       `gorm:"uniqueIndex:idx_c2c_trade_idem,priority:2;type:varchar(128);not null;default:''" json:"idempotency_key,omitempty"` // 买家幂等键，同一买家重复提交不重复建单
	FiatCurrency          string       `gorm:"type:varchar(8);not null;default:'CNY'" json:"fiat_currency"`
	Price                 money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price"`              // 成交单价
	FiatAmount            money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"fiat_amount"`        // 法币成交额
	USDTAmount            money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"usdt_amount"`        // USDT 成交总量
	FeeAmount             money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"fee_amount"`         // 手续费（第一版固定 0）
	BuyerReceiveUSDT      money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"buyer_receive_usdt"` // 买家实际到账 USDT
	Status                string       `gorm:"index;type:varchar(32);not null;default:'pending_payment'" json:"status"`
	PaymentMethodSnapshot string       `gorm:"type:text;default:''" json:"payment_method_snapshot"`   // 支付方式快照（JSON）
	PaymentReference      string       `gorm:"type:varchar(256);default:''" json:"payment_reference"` // 买家转账备注/凭证
	BuyerPaidAt           *time.Time   `json:"buyer_paid_at"`
	SellerConfirmedAt     *time.Time   `json:"seller_confirmed_at"`
	ExpiredAt             time.Time    `gorm:"index" json:"expired_at"` // 支付超时时间
	CanceledAt            *time.Time   `json:"canceled_at"`
	DisputedAt            *time.Time   `json:"disputed_at"`
	CompletedAt           *time.Time   `json:"completed_at"`
	CreatedAt             time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt             time.Time    `gorm:"index" json:"updated_at"`
}

// TableName 指定表名。
func (Trade) TableName() string {
	return "c2c_trades"
}
