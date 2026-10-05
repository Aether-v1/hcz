// ============ 客服支持 - 类型定义 ============

export type SupportTicketStatus =
  | 'open'
  | 'waiting_user'
  | 'waiting_support'
  | 'resolved'
  | 'closed'

export type SupportPriority = 'low' | 'normal' | 'high' | 'urgent'

export type SupportMessageSender = 'user' | 'admin' | 'system'

export interface SupportOverview {
  total: number
  open: number
  waiting_user: number
  waiting_support: number
  resolved: number
  closed: number
  unassigned: number
  my_assigned: number
}

export interface SupportTicketListItem {
  id: number
  ticket_no: string
  user_id: number
  user_email: string
  user_name: string
  category_id: number
  category_name: string
  subject: string
  status: SupportTicketStatus | string
  priority: SupportPriority | string
  assigned_admin_id: number | null
  assigned_admin_name: string
  admin_unread_count: number
  last_reply_by: SupportMessageSender | string
  last_replied_at: string
  created_at: string
}

export interface SupportTicketListResponse {
  items: SupportTicketListItem[]
  total: number
  page: number
  page_size: number
}

export interface SupportCategory {
  id: number
  code: string
  name: string
  enabled: boolean
  sort_order: number
  default_priority: SupportPriority | string
  created_at: string
}

export interface SupportMessage {
  id: number
  ticket_id: number
  sender_type: SupportMessageSender | string
  sender_id: number
  sender_name: string
  body: string
  attachment_ids: number[]
  created_at: string
}

export interface SupportAttachment {
  id: number
  ticket_id?: number
  message_id?: number
  file_name: string
  file_size: number
  mime_type: string
  file_path?: string
  created_at: string
}

export interface SupportAudit {
  id: number
  ticket_id: number
  admin_id: number
  admin_name: string
  action: string
  before?: unknown
  after?: unknown
  reason?: string
  created_at: string
}

export interface SupportTicketDetail {
  id: number
  ticket_no: string
  user_id: number
  user_email?: string
  user_name?: string
  category_id: number
  subject: string
  status: SupportTicketStatus | string
  priority: SupportPriority | string
  assigned_admin_id: number | null
  assigned_admin_name?: string
  admin_unread_count: number
  last_reply_by?: SupportMessageSender | string
  last_replied_at?: string
  biz_type?: string
  biz_id?: number
  created_at: string
  updated_at?: string
}

export interface SupportTicketDetailResponse {
  ticket: SupportTicketDetail
  category: SupportCategory | null
  user: { id: number; email: string; username?: string } | null
  messages: SupportMessage[]
  attachments: SupportAttachment[]
  audits: SupportAudit[]
}

export interface SupportCategoryPayload {
  code?: string
  name: string
  enabled?: boolean
  sort_order?: number
  default_priority?: SupportPriority | string
}

export interface SupportAdminUser {
  id: number
  username?: string
  email?: string
}
