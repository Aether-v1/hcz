/**
 * 用户侧工单系统类型定义（与后端契约对齐）。
 * 后端契约见 docs / 任务说明：
 *   GET    /api/v1/support/categories
 *   GET    /api/v1/support/tickets
 *   POST   /api/v1/support/tickets
 *   GET    /api/v1/support/tickets/:id
 *   POST   /api/v1/support/tickets/:id/replies
 *   POST   /api/v1/support/tickets/:id/close
 *   POST   /api/v1/support/tickets/:id/reopen
 *   POST   /api/v1/support/attachments
 *   GET    /api/v1/support/attachments/:id
 */

export type SupportTicketStatus =
    | 'open'
    | 'waiting_user'
    | 'waiting_support'
    | 'resolved'
    | 'closed'
    | string

export type SupportTicketPriority = 'low' | 'normal' | 'high' | 'urgent' | string

export type SupportSenderType = 'user' | 'admin' | 'system' | string

export interface SupportCategory {
    id: number
    code: string
    name: string
    default_priority: string
}

/** 列表项：GET /support/tickets 返回的 items 元素 */
export interface SupportTicketSummary {
    id: number
    ticket_no: string
    category_id: number
    category_name: string
    subject: string
    status: SupportTicketStatus
    priority: SupportTicketPriority
    user_unread_count: number
    last_reply_by: string | null
    last_replied_at: string | null
    created_at: string
}

/** 详情中的完整工单（在列表项基础上扩展可选字段） */
export interface SupportTicketDetail extends SupportTicketSummary {
    body?: string
    biz_type?: string | null
    biz_id?: number | null
    resolved_at?: string | null
    closed_at?: string | null
}

export interface SupportMessage {
    id: number
    ticket_id: number
    sender_type: SupportSenderType
    sender_user_id: number | null
    sender_admin_id: number | null
    body: string
    message_type: string
    created_at: string
}

export interface SupportAttachment {
    id: number
    file_name: string
    mime_type: string
    file_size: number
    created_at: string
}

/** GET /support/tickets 的 data 载荷 */
export interface SupportTicketListData {
    items: SupportTicketSummary[]
    total: number
    page: number
    page_size: number
}

/** GET /support/tickets/:id 的 data 载荷 */
export interface SupportTicketDetailData {
    ticket: SupportTicketDetail
    category: SupportCategory
    messages: SupportMessage[]
    attachments: SupportAttachment[]
}

/** POST /support/tickets 请求体 */
export interface CreateTicketPayload {
    category_id: number
    subject: string
    body: string
    biz_type?: string
    biz_id?: number
    attachment_ids?: number[]
}

/** POST /support/tickets 的 data 载荷 */
export interface CreateTicketResult {
    id: number
    ticket_no: string
    status: string
}

/** POST /support/attachments 的 data 载荷 */
export interface UploadAttachmentResult {
    id: number
    file_name: string
    mime_type: string
    file_size: number
}

/** POST /support/tickets/:id/replies 请求体 */
export interface CreateReplyPayload {
    body: string
    attachment_ids?: number[]
}
