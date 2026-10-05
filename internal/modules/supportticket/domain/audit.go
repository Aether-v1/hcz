package domain

import "time"

// Audit 工单操作审计日志。
type Audit struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TicketID  uint      `gorm:"index;not null" json:"ticket_id"`
	AdminID   uint      `gorm:"index;not null" json:"admin_id"`
	Action    string    `gorm:"type:varchar(30);not null" json:"action"` // assign/status_change/priority_change/resolve/close/reopen
	Before    string    `gorm:"type:text" json:"before"`
	After     string    `gorm:"type:text" json:"after"`
	Reason    string    `gorm:"type:varchar(255)" json:"reason"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// TableName 指定表名。
func (Audit) TableName() string {
	return "support_ticket_audits"
}
