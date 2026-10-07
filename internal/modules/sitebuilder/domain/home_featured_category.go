package domain

import "time"

// HomeFeaturedCategory 首页热门推荐分类配置。
// 只存 category_id + 展示元数据 + sort + enabled，不复制分类数据。
type HomeFeaturedCategory struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	CategoryID   uint      `gorm:"not null;uniqueIndex;index" json:"category_id"`
	Alias        string    `gorm:"type:varchar(100)" json:"alias"`
	IconOverride string    `gorm:"type:varchar(500)" json:"icon_override"`
	Enabled      bool      `gorm:"default:true;index" json:"enabled"`
	SortOrder    int       `gorm:"default:0;index" json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (HomeFeaturedCategory) TableName() string { return "home_featured_categories" }
