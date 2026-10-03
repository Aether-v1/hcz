package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// P1 售后工单：独立于订单主状态。type 第一版仅 not_received。
const (
	AfterSaleTypeNotReceived = "not_received"

	AfterSaleStatusNone     = "none"
	AfterSaleStatusPending  = "pending"
	AfterSaleStatusResolved = "resolved"
	AfterSaleStatusRejected = "rejected"
)

type AfterSaleTicket struct {
	ID           uint            `gorm:"primaryKey"`
	OrderID      uint            `gorm:"index;not null"`
	UserID       uint            `gorm:"index;not null"`
	Type         string          `gorm:"not null;default:'not_received'"`
	Reason       string          `gorm:"type:varchar(255)"`
	Description  string          `gorm:"type:text"`
	Status       string          `gorm:"index;not null;default:'pending'"`
	AdminNote    string          `gorm:"type:varchar(255)"`
	RefundAmount decimal.NullDecimal `gorm:"type:decimal(20,2)"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ResolvedAt   *time.Time
}

func (AfterSaleTicket) TableName() string { return "after_sale_tickets" }
