package domain

import "time"

// Message 工单消息。
type Message struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	TicketID      uint      `gorm:"index;not null" json:"ticket_id"`
	SenderType    string    `gorm:"type:varchar(10);not null" json:"sender_type"` // user/admin/system
	SenderUserID  *uint     `json:"sender_user_id"`
	SenderAdminID *uint     `json:"sender_admin_id"`
	Body          string    `gorm:"type:text;not null" json:"body"`
	MessageType   string    `gorm:"type:varchar(20);not null;default:'text'" json:"message_type"` // text/system_notice
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}

// TableName 指定表名。
func (Message) TableName() string {
	return "support_ticket_messages"
}
