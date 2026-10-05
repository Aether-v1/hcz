package http

// 用户侧请求 DTO。

type createTicketRequest struct {
	CategoryID    uint   `json:"category_id" binding:"required"`
	Subject       string `json:"subject" binding:"required"`
	Body          string `json:"body" binding:"required"`
	BizType       string `json:"biz_type"`
	BizID         uint   `json:"biz_id"`
	AttachmentIDs []uint `json:"attachment_ids"`
}

type replyTicketRequest struct {
	Body          string `json:"body" binding:"required"`
	AttachmentIDs []uint `json:"attachment_ids"`
}

// 客服侧请求 DTO。

type assignTicketRequest struct {
	AdminID uint `json:"admin_id"`
}

type changePriorityRequest struct {
	Priority string `json:"priority" binding:"required"`
}

type resolveTicketRequest struct {
	Reason string `json:"reason"`
}

type closeTicketRequest struct {
	Reason string `json:"reason"`
}

type reopenTicketRequest struct {
	Reason string `json:"reason"`
}

// 分类请求 DTO。

type createCategoryRequest struct {
	Code            string `json:"code" binding:"required"`
	Name            string `json:"name" binding:"required"`
	Enabled         bool   `json:"enabled"`
	SortOrder       int    `json:"sort_order"`
	DefaultPriority string `json:"default_priority"`
}

type updateCategoryRequest struct {
	Name            string `json:"name" binding:"required"`
	Enabled         bool   `json:"enabled"`
	SortOrder       int    `json:"sort_order"`
	DefaultPriority string `json:"default_priority" binding:"required"`
}
