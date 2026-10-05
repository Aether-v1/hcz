package contract

import "errors"

// 领域哨兵错误，由 handler 映射为 HTTP 响应。
var (
	ErrTicketNotFound      = errors.New("support ticket not found")
	ErrCategoryNotFound   = errors.New("support ticket category not found")
	ErrCategoryDisabled    = errors.New("support ticket category disabled")
	ErrTicketClosed        = errors.New("support ticket is closed")
	ErrTicketStatusInvalid = errors.New("support ticket status transition invalid")
	ErrReopenExpired       = errors.New("support ticket reopen window expired")
	ErrAlreadyAssigned     = errors.New("support ticket already assigned")
	ErrInvalidBizOwnership = errors.New("support ticket biz ownership verification failed")
	ErrInvalidPriority     = errors.New("invalid support ticket priority")
	ErrInvalidSubject      = errors.New("invalid support ticket subject")
	ErrInvalidBody         = errors.New("invalid support ticket body")
	ErrAttachmentNotFound  = errors.New("support ticket attachment not found")
	ErrNotOwner            = errors.New("support ticket not owned by current user")
	ErrAdminNotFound       = errors.New("admin not found")
	ErrDuplicateCategoryCode = errors.New("support ticket category code already exists")
)
