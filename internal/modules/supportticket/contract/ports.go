package contract

import (
	"context"
	"io"
	"mime/multipart"

	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	uploadcontract "github.com/Aether-v1/hcz/internal/modules/upload/contract"
)

// ==================== 查询过滤条件 ====================

// TicketListFilter 用户侧工单列表过滤。
type TicketListFilter struct {
	UserID    uint
	Status    string
	Page      int
	PageSize  int
}

// AdminTicketListFilter 后台工单列表过滤。
type AdminTicketListFilter struct {
	Status         string
	CategoryID     uint
	Priority       string
	AssignedAdminID uint // 0=unassigned, math.MaxUint 特殊值保留；"me" 由 service 层展开
	UnassignedOnly bool
	MyAssignedOnly uint // 0=不过滤；否则只看该 admin 负责的
	UnreadOnly     bool
	Search         string
	Page           int
	PageSize       int
}

// MessageListFilter 消息分页过滤。
type MessageListFilter struct {
	TicketID uint
	Page     int
	PageSize int
}

// ==================== 概览统计 ====================

// OverviewStats 后台概览统计。
type OverviewStats struct {
	Open           int64 `json:"open"`
	WaitingUser    int64 `json:"waiting_user"`
	WaitingSupport int64 `json:"waiting_support"`
	Resolved       int64 `json:"resolved"`
	Closed         int64 `json:"closed"`
	Unassigned     int64 `json:"unassigned"`
	MyAssigned     int64 `json:"my_assigned"`
}

// ==================== Repository 持久化端口 ====================

// Repository 拥有工单系统全部聚合的持久化。事务调用方通过 Transaction 拿到绑定同一事务的实现。
type Repository interface {
	// ---- 分类 ----
	CreateCategory(c *supportdomain.Category) error
	UpdateCategory(c *supportdomain.Category) error
	GetCategoryByID(id uint) (*supportdomain.Category, error)
	GetCategoryByCode(code string) (*supportdomain.Category, error)
	ListEnabledCategories() ([]supportdomain.Category, error)
	ListAllCategories() ([]supportdomain.Category, error)

	// ---- 工单 ----
	CreateTicket(t *supportdomain.Ticket) error
	UpdateTicket(t *supportdomain.Ticket) error
	GetTicketByID(id uint) (*supportdomain.Ticket, error)
	GetTicketByIDForUpdate(id uint) (*supportdomain.Ticket, error)
	ListMyTickets(filter TicketListFilter) ([]supportdomain.Ticket, int64, error)
	ListAdminTickets(filter AdminTicketListFilter) ([]supportdomain.Ticket, int64, error)
	OverviewStats(myAdminID uint) (OverviewStats, error)

	// 原子未读计数（UPDATE ... SET col = col + ?）。
	IncrAdminUnread(ticketID uint, delta int) error
	IncrUserUnread(ticketID uint, delta int) error
	ResetUserUnread(ticketID uint) error
	ResetAdminUnread(ticketID uint) error

	// CompareAndSetAssign 认领：仅当 assigned_admin_id IS NULL 时更新为新 admin。返回受影响行数。
	ClaimTicket(ticketID, adminID uint) (int64, error)
	// AssignTicket 直接指派（覆盖现有 assigned_admin_id）。
	AssignTicket(ticketID, adminID uint) error

	// ---- 消息 ----
	CreateMessage(m *supportdomain.Message) error
	ListMessagesByTicket(ticketID uint, page, pageSize int) ([]supportdomain.Message, int64, error)

	// ---- 附件 ----
	CreateAttachment(a *supportdomain.Attachment) error
	GetAttachmentByID(id uint) (*supportdomain.Attachment, error)
	ListAttachmentsByTicket(ticketID uint) ([]supportdomain.Attachment, error)
	LinkAttachmentToTicket(attachmentID, ticketID uint) error
	LinkAttachmentToMessage(attachmentID, messageID uint) error

	// ---- 审计 ----
	CreateAudit(a *supportdomain.Audit) error
	ListAuditsByTicket(ticketID uint) ([]supportdomain.Audit, error)

	// ---- Biz 归属校验（只读原始 SQL，不注入任何资金服务）----
	VerifyBizOwnership(bizType string, bizID uint, userID uint) (bool, error)
}

// Transaction 是已开启的数据库事务在工单上下文中的视图。
type Transaction interface {
	Tickets() Repository
}

// UnitOfWork 开启工单事务，回调内 Tickets() 共享同一 *gorm.DB。
type UnitOfWork interface {
	WithinTransaction(fn func(Transaction) error) error
}

// ==================== 外部依赖端口 ====================

// UserReader 读取用户（用于列表 join 展示 email/display_name）。
type UserReader interface {
	GetByID(userID uint) (id uint, email string, displayName string, err error)
}

// AdminReader 读取管理员（用于校验指派目标是否存在、展示指派人昵称）。
type AdminReader interface {
	GetAdminByID(adminID uint) (id uint, username string, err error)
}

// NotificationCreator 写入用户站内通知（尽力而为、幂等）。
type NotificationCreator interface {
	CreateNotification(ctx context.Context, in NotificationInput) error
}

// NotificationInput 业务模块写入用户通知的入参。
type NotificationInput struct {
	UserID  uint
	Type    string
	Title   string
	Body    string
	BizType string
	BizID   uint
}

// FileUploader 文件落盘端口（复用 upload application.Service 的接口形状）。
type FileUploader interface {
	SaveFileWithMeta(file *multipart.FileHeader, scene string) (*uploadcontract.Result, error)
}

// FileStreamer 从私有存储读取附件内容（用于下载流回）。
type FileStreamer interface {
	ReadObject(objectKey string) (io.ReadCloser, error)
}
