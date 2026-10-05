package domain

import "time"

// Ticket 工单主表。
type Ticket struct {
	ID               uint       `gorm:"primarykey" json:"id"`
	TicketNo         string     `gorm:"type:varchar(32);uniqueIndex;not null" json:"ticket_no"`
	UserID           uint       `gorm:"index;not null" json:"user_id"`
	CategoryID       uint       `gorm:"index;not null" json:"category_id"`
	Subject          string     `gorm:"type:varchar(255);not null" json:"subject"`
	Status           string     `gorm:"type:varchar(20);index;not null;default:'open'" json:"status"`
	Priority         string     `gorm:"type:varchar(10);index;not null;default:'normal'" json:"priority"`
	AssignedAdminID  *uint      `gorm:"index" json:"assigned_admin_id"`
	BizType          string     `gorm:"type:varchar(30);index" json:"biz_type"`
	BizID            uint       `gorm:"index" json:"biz_id"`
	UserUnreadCount  int        `gorm:"not null;default:0" json:"user_unread_count"`
	AdminUnreadCount int        `gorm:"not null;default:0" json:"admin_unread_count"`
	LastReplyBy      string     `gorm:"type:varchar(10)" json:"last_reply_by"`
	LastRepliedAt    *time.Time `json:"last_replied_at"`
	ResolvedAt       *time.Time `json:"resolved_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	CreatedAt        time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (Ticket) TableName() string {
	return "support_tickets"
}
