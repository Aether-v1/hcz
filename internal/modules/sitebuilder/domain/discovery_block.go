package domain

import (
	"time"

	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// DiscoveryBlock 发现页/首页装修区块。
type DiscoveryBlock struct {
	ID        uint         `gorm:"primarykey" json:"id"`
	Type      string       `gorm:"type:varchar(30);not null;index" json:"type"`
	Title     string       `gorm:"type:varchar(200)" json:"title"`
	Config    jsonmap.JSON `gorm:"type:json" json:"config"`
	Enabled   bool         `gorm:"default:true;index" json:"enabled"`
	SortOrder int          `gorm:"default:0;index" json:"sort_order"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// TableName 指定表名。
func (DiscoveryBlock) TableName() string { return "discovery_blocks" }
