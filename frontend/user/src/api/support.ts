import { userApi } from './client'
import type { CreateReplyPayload, CreateTicketPayload } from '../types/support'

/**
 * 用户侧工单 API。
 * 所有接口均需登录态（userApi 自动注入 Bearer Token）。
 * 返回值统一为 { data: ApiResponse }，业务数据在 res.data.data。
 */
export const supportAPI = {
    /** 工单分类（仅启用项） */
    categories: () => userApi.get('/support/categories'),

    /** 工单列表：page / page_size / status 可选过滤 */
    tickets: (params: { page?: number; page_size?: number; status?: string }) =>
        userApi.get('/support/tickets', { params }),

    /** 工单详情（同时会由服务端清零 user_unread_count） */
    ticketDetail: (id: number | string) => userApi.get(`/support/tickets/${id}`),

    /** 创建工单，成功后返回 { id, ticket_no, status } */
    createTicket: (payload: CreateTicketPayload) => userApi.post('/support/tickets', payload),

    /** 回复工单 */
    reply: (id: number | string, payload: CreateReplyPayload) =>
        userApi.post(`/support/tickets/${id}/replies`, payload),

    /** 关闭工单 */
    close: (id: number | string) => userApi.post(`/support/tickets/${id}/close`),

    /** 重新打开工单（resolved 超过 7 天后端会报错） */
    reopen: (id: number | string) => userApi.post(`/support/tickets/${id}/reopen`),

    /** 上传附件（multipart/form-data，字段名 file），返回附件元信息 */
    uploadAttachment: (file: File) => {
        const form = new FormData()
        form.append('file', file)
        return userApi.post('/support/attachments', form)
    },

    /** 下载附件，返回文件 Blob */
    downloadAttachment: (id: number | string) =>
        userApi.get(`/support/attachments/${id}`, { blob: true }),
}
