package contract

import (
	"context"
	"errors"

	"github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// 哨兵错误。
var (
	// ErrAlreadyExists 表示违反 (user_id,biz_type,biz_id,type) 唯一约束（回调重放）。
	// Store 在 Create 遇到唯一冲突时返回此错误；Service 层静默吞掉以保证幂等。
	ErrAlreadyExists = errors.New("user notification already exists")
	// ErrNotFound 表示目标通知不存在或不属于当前用户（防越权，不暴露存在性）。
	ErrNotFound = errors.New("user notification not found")
)

// Repository 是用户通知的持久化端口。Handler 不得直接操作 GORM，统一经此端口。
type Repository interface {
	// Create 插入一条通知；违反唯一约束时返回 ErrAlreadyExists（不 panic）。
	Create(ctx context.Context, n *domain.UserNotification) error
	// ListByUser 按 created_at DESC 分页拉取某用户通知。
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]domain.UserNotification, int64, error)
	// CountUnread 统计某用户未读数。
	CountUnread(ctx context.Context, userID uint) (int64, error)
	// MarkRead 带 user_id 条件标记单条已读，返回受影响行数（0=不存在或越权）。
	MarkRead(ctx context.Context, id, userID uint) (int64, error)
	// MarkAllRead 一键全部已读，返回实际标记条数。
	MarkAllRead(ctx context.Context, userID uint) (int64, error)
}

// CreateInput 是业务模块写入用户通知的统一入参。
type CreateInput struct {
	UserID  uint
	Type    string
	Title   string
	Body    string
	Data    jsonmap.JSON
	BizType string
	BizID   uint
}

// Creator 是业务域（支付/订单/履约）写入用户通知的最小端口。
// 实现必须「尽力而为 + 幂等」：唯一冲突静默返回 nil，其余错误由调用方记日志、不阻塞主业务。
type Creator interface {
	CreateNotification(ctx context.Context, in CreateInput) error
}

// UseCase 是用户收件箱查询/标记用例端口（供 HTTP handler 依赖）。
type UseCase interface {
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]domain.UserNotification, int64, error)
	CountUnread(ctx context.Context, userID uint) (int64, error)
	MarkRead(ctx context.Context, id, userID uint) error
	MarkAllRead(ctx context.Context, userID uint) (int64, error)
}
