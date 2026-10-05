package domain

import "time"

// HomeEntry 首页入口（金刚区/快捷入口）配置项。
type HomeEntry struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Key          string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"key"`
	Title        string    `gorm:"type:varchar(100);not null" json:"title"`
	Subtitle     string    `gorm:"type:varchar(200)" json:"subtitle"`
	Icon         string    `gorm:"type:varchar(100)" json:"icon"`
	ActionType   string    `gorm:"type:varchar(20);not null" json:"action_type"` // internal/external
	ActionTarget string    `gorm:"type:varchar(500);not null" json:"action_target"`
	Badge        string    `gorm:"type:varchar(50)" json:"badge"`
	Recommended  bool      `gorm:"default:false" json:"recommended"`
	Enabled      bool      `gorm:"default:true;index" json:"enabled"`
	SortOrder    int       `gorm:"default:0;index" json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (HomeEntry) TableName() string { return "home_entries" }
