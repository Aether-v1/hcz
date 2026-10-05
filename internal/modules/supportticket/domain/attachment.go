package domain

import "time"

// Attachment 工单附件。
type Attachment struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	TicketID     uint      `gorm:"index;not null" json:"ticket_id"`
	MessageID    *uint     `gorm:"index" json:"message_id"`
	UploaderType string    `gorm:"type:varchar(10);not null" json:"uploader_type"` // user/admin
	FileName     string    `gorm:"type:varchar(255);not null" json:"file_name"`
	ObjectKey    string    `gorm:"type:varchar(500);not null" json:"object_key"` // random UUID path
	MimeType     string    `gorm:"type:varchar(100);not null" json:"mime_type"`
	FileSize     int64     `gorm:"not null" json:"file_size"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

// TableName 指定表名。
func (Attachment) TableName() string {
	return "support_ticket_attachments"
}
