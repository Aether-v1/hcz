// Package domain 定义工单系统的持久化模型。
package domain

import "time"

// Category 工单分类。
type Category struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	Code            string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"code"`
	Name            string    `gorm:"type:varchar(100);not null" json:"name"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	SortOrder       int       `gorm:"not null;default:0" json:"sort_order"`
	DefaultPriority string    `gorm:"type:varchar(10);not null;default:'normal'" json:"default_priority"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Category) TableName() string {
	return "support_ticket_categories"
}
