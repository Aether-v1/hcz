import { userApi } from './client'

/**
 * 用户站内通知 API（Phase 1）
 * 后端契约：
 *   GET  /api/v1/notifications?page=1&page_size=20
 *        -> { items: [...], total, page, page_size }
 *   GET  /api/v1/notifications/unread-count -> { count: N }
 *   POST /api/v1/notifications/:id/read     -> { ok: true }
 *   POST /api/v1/notifications/read-all     -> { ok: true, marked: N }
 */
export const notificationAPI = {
    list: (page = 1, pageSize = 20) =>
        userApi.get('/notifications', { params: { page, page_size: pageSize } }),
    unreadCount: () => userApi.get('/notifications/unread-count'),
    markRead: (id: number) => userApi.post(`/notifications/${id}/read`),
    markAllRead: () => userApi.post('/notifications/read-all'),
}
