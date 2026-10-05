package contract

import (
	"io"
	"time"

	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
)

// ==================== 用户侧用例入参 ====================

// CreateTicketInput 创建工单入参。
type CreateTicketInput struct {
	UserID       uint
	CategoryID   uint
	Subject      string
	Body         string
	BizType      string
	BizID        uint
	AttachmentIDs []uint
}

// ReplyTicketInput 回复工单入参。
type ReplyTicketInput struct {
	UserID       uint
	TicketID     uint
	Body         string
	AttachmentIDs []uint
}

// UploadAttachmentInput 上传附件入参（文件由 handler 读取后传入）。
type UploadAttachmentInput struct {
	UserID   uint
	File     interface{} // *multipart.FileHeader，经 FileUploader 落盘
	Filename string
}

// ==================== 后台用例入参 ====================

// AssignTicketInput 指派/认领入参。AdminID=0 表示认领给自己（OperatorAdminID）。
type AssignTicketInput struct {
	OperatorAdminID uint
	TicketID        uint
	AdminID         uint
}

// ChangePriorityInput 修改优先级入参。
type ChangePriorityInput struct {
	OperatorAdminID uint
	TicketID        uint
	Priority        string
}

// ResolveTicketInput 解决工单入参。
type ResolveTicketInput struct {
	OperatorAdminID uint
	TicketID        uint
	Reason          string
}

// CloseTicketInput 关闭工单入参。
type CloseTicketInput struct {
	OperatorAdminID uint
	TicketID        uint
	Reason          string
}

// ReopenTicketInput 重开工单入参。
type ReopenTicketInput struct {
	OperatorAdminID uint
	TicketID        uint
	Reason          string
}

// AdminReplyInput 客服回复入参。
type AdminReplyInput struct {
	OperatorAdminID uint
	TicketID        uint
	Body            string
	AttachmentIDs   []uint
}

// CreateCategoryInput 创建分类入参。
type CreateCategoryInput struct {
	Code            string
	Name            string
	Enabled         bool
	SortOrder       int
	DefaultPriority string
}

// UpdateCategoryInput 更新分类入参。
type UpdateCategoryInput struct {
	Name            string
	Enabled         bool
	SortOrder       int
	DefaultPriority string
}

// ==================== 视图 DTO ====================

// CategoryView 分类视图。
type CategoryView struct {
	ID              uint   `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	SortOrder       int    `json:"sort_order"`
	DefaultPriority string `json:"default_priority"`
}

// TicketView 工单列表项视图（含 join 的分类名）。
type TicketView struct {
	ID               uint       `json:"id"`
	TicketNo         string     `json:"ticket_no"`
	UserID           uint       `json:"user_id"`
	CategoryID       uint       `json:"category_id"`
	CategoryName     string     `json:"category_name"`
	Subject          string     `json:"subject"`
	Status           string     `json:"status"`
	Priority         string     `json:"priority"`
	AssignedAdminID  *uint      `json:"assigned_admin_id"`
	AssignedAdminName string    `json:"assigned_admin_name"`
	BizType          string     `json:"biz_type"`
	BizID            uint       `json:"biz_id"`
	UserUnreadCount  int        `json:"user_unread_count"`
	AdminUnreadCount int        `json:"admin_unread_count"`
	LastReplyBy      string     `json:"last_reply_by"`
	LastRepliedAt    *time.Time `json:"last_replied_at"`
	CreatedAt        time.Time  `json:"created_at"`
	// 后台额外字段
	UserEmail    string `json:"user_email,omitempty"`
	UserName     string `json:"user_name,omitempty"`
}

// MessageView 消息视图。
type MessageView struct {
	ID            uint      `json:"id"`
	TicketID      uint      `json:"ticket_id"`
	SenderType    string    `json:"sender_type"`
	SenderUserID  *uint     `json:"sender_user_id"`
	SenderAdminID *uint     `json:"sender_admin_id"`
	Body          string    `json:"body"`
	MessageType   string    `json:"message_type"`
	CreatedAt     time.Time `json:"created_at"`
	Attachments   []AttachmentView `json:"attachments"`
}

// AttachmentView 附件视图。
type AttachmentView struct {
	ID           uint      `json:"id"`
	TicketID     uint      `json:"ticket_id"`
	MessageID    *uint     `json:"message_id"`
	UploaderType string    `json:"uploader_type"`
	FileName     string    `json:"file_name"`
	MimeType     string    `json:"mime_type"`
	FileSize     int64     `json:"file_size"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuditView 审计视图。
type AuditView struct {
	ID        uint      `json:"id"`
	TicketID  uint      `json:"ticket_id"`
	AdminID   uint      `json:"admin_id"`
	Action    string    `json:"action"`
	Before    string    `json:"before"`
	After     string    `json:"after"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// TicketDetailView 工单详情视图。
type TicketDetailView struct {
	Ticket    TicketView      `json:"ticket"`
	Messages  []MessageView   `json:"messages"`
	MessagesTotal int64       `json:"messages_total"`
	Attachments []AttachmentView `json:"attachments"`
	Audits    []AuditView     `json:"audits,omitempty"`
}

// AttachmentDownload 附件下载结果。
type AttachmentDownload struct {
	Attachment *supportdomain.Attachment
	Reader     io.ReadCloser
	MimeType   string
	FileName   string
}
