package domain

import "time"

// SiteAuditLog 站点装修操作审计日志。
type SiteAuditLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	AdminID   uint      `gorm:"index" json:"admin_id"`
	Section   string    `gorm:"type:varchar(30);index" json:"section"` // brand/template/home_entries/discovery/navigation/footer
	Action    string    `gorm:"type:varchar(30)" json:"action"`        // create/update/delete/reorder/enable/disable/switch
	Before    string    `gorm:"type:text" json:"before"`
	After     string    `gorm:"type:text" json:"after"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名。
func (SiteAuditLog) TableName() string { return "site_audit_logs" }
