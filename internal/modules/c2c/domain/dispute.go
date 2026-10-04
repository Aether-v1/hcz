package domain

import (
	"time"
)

// Dispute C2C 交易申诉。一个 trade 同时只能有一条 dispute（trade_id 唯一索引）。
type Dispute struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	TradeID         uint       `gorm:"uniqueIndex;not null" json:"trade_id"`
	InitiatorUserID uint       `gorm:"index;not null" json:"initiator_user_id"`
	Reason          string     `gorm:"type:varchar(128);not null;default:''" json:"reason"`
	Description     string     `gorm:"type:text;default:''" json:"description"`
	Evidence        string     `gorm:"type:text;default:''" json:"evidence"`                   // 证据材料（JSON）
	Status          string     `gorm:"type:varchar(16);not null;default:'open'" json:"status"` // open/resolved
	AdminResult     string     `gorm:"type:varchar(32);default:''" json:"admin_result"`        // release_to_buyer/return_to_seller/空
	AdminNote       string     `gorm:"type:text;default:''" json:"admin_note"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	CreatedAt       time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"index" json:"updated_at"`
}

// TableName 指定表名。
func (Dispute) TableName() string {
	return "c2c_disputes"
}
