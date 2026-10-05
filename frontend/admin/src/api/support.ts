import { api } from './client'
import type {
  SupportAttachment,
  SupportAudit,
  SupportCategory,
  SupportCategoryPayload,
  SupportOverview,
  SupportTicketDetailResponse,
  SupportTicketListResponse,
} from '@/types/support'

// ============ 客服支持 - API 封装 ============
// 后端路径前缀：/admin/support/*（client 已带 /api/v1）

export interface SupportTicketListParams {
  page?: number
  page_size?: number
  status?: string
  category_id?: number
  priority?: string
  assigned_admin_id?: number
  unread_only?: boolean
  search?: string
}

export const supportAPI = {
  // 概览统计
  getOverview: () => api.get('/admin/support/overview'),

  // 工单列表
  getTickets: (params?: SupportTicketListParams) =>
    api.get('/admin/support/tickets', { params: params as Record<string, unknown> }),

  // 工单详情（打开会重置 admin_unread_count）
  getTicket: (id: number) => api.get(`/admin/support/tickets/${id}`),

  // 回复
  reply: (id: number, data: { body: string; attachment_ids?: number[] }) =>
    api.post(`/admin/support/tickets/${id}/replies`, data),

  // 分配（omit / admin_id=0 表示领取给自己）
  assign: (id: number, data: { admin_id?: number }) =>
    api.post(`/admin/support/tickets/${id}/assign`, data),

  // 修改优先级
  changePriority: (id: number, data: { priority: string }) =>
    api.post(`/admin/support/tickets/${id}/change-priority`, data),

  // 解决 / 关闭 / 重新打开
  resolve: (id: number, data: { reason?: string }) =>
    api.post(`/admin/support/tickets/${id}/resolve`, data),
  close: (id: number, data: { reason?: string }) =>
    api.post(`/admin/support/tickets/${id}/close`, data),
  reopen: (id: number, data: { reason?: string }) =>
    api.post(`/admin/support/tickets/${id}/reopen`, data),

  // 分类
  getCategories: () => api.get('/admin/support/categories'),
  createCategory: (data: SupportCategoryPayload) =>
    api.post('/admin/support/categories', data),
  updateCategory: (id: number, data: SupportCategoryPayload) =>
    api.put(`/admin/support/categories/${id}`, data),
  deleteCategory: (id: number) => api.delete(`/admin/support/categories/${id}`),

  // 审计日志
  getAudits: (id: number) => api.get(`/admin/support/tickets/${id}/audits`),

  // 附件上传（multipart，字段 file）
  uploadAttachment: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post('/admin/support/attachments', form)
  },

  // 附件下载（返回 blob）
  downloadAttachment: (id: number) =>
    api.get(`/admin/support/attachments/${id}`, { blob: true }),
}

export type {
  SupportAttachment,
  SupportAudit,
  SupportCategory,
  SupportOverview,
  SupportTicketDetailResponse,
  SupportTicketListResponse,
}
