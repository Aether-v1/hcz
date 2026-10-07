package domain

import "time"

// FulfillmentType 是积分商品履约类型（V1 仅支持 MANUAL：后台人工处理数字权益）。
// 禁止在本轮引入兑换码池 / 自动供应商履约。
const (
	FulfillmentTypeManual = "MANUAL"
)

// ValidFulfillmentType 校验履约类型是否受支持。
func ValidFulfillmentType(ft string) bool {
	return ft == FulfillmentTypeManual
}

// PointsProduct 是积分商城商品（独立于 recharge products，禁止复用充值商品模型）。
//
// 资产语义：
//   - PointsPrice 为 BIGINT 整数（禁 float/decimal/负数/0）；
//   - Stock 为可用库存（>=0）；UnlimitedStock=true 时跳过库存校验/扣减/恢复，
//     禁止用 -1 之类的 magic number 表达无限库存；
//   - PerUserLimit=0 表示不限制；>0 时统计 PENDING/PROCESSING/COMPLETED 订单
//     （FAILED/CANCELLED 已返还积分与库存，不计入限购）。
//
// 删除语义：有兑换订单引用的商品禁止物理删除；以下架（Enabled=false）表达。
type PointsProduct struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"column:name;size:120;not null" json:"name"`
	Subtitle        string    `gorm:"column:subtitle;size:255;not null;default:''" json:"subtitle"`
	Description     string    `gorm:"column:description;type:text;not null;default:''" json:"description"`
	Cover           string    `gorm:"column:cover;size:255;not null;default:''" json:"cover"`
	PointsPrice     int64     `gorm:"column:points_price;not null" json:"points_price"`
	Stock           int64     `gorm:"column:stock;not null;default:0" json:"stock"`
	UnlimitedStock  bool      `gorm:"column:unlimited_stock;not null;default:false" json:"unlimited_stock"`
	Enabled         bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	Sort            int       `gorm:"column:sort;not null;default:0" json:"sort"`
	PerUserLimit    int64     `gorm:"column:per_user_limit;not null;default:0" json:"per_user_limit"`
	FulfillmentType string    `gorm:"column:fulfillment_type;size:16;not null;default:'MANUAL'" json:"fulfillment_type"`
	Instructions    string    `gorm:"column:instructions;type:text;not null;default:''" json:"instructions"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (PointsProduct) TableName() string { return "points_products" }
