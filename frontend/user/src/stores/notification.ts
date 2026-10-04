import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { notificationAPI } from '../api/notification'
import { useUserAuthStore } from './userAuth'

/** 与后端契约对齐的单条通知结构 */
export interface NotificationItem {
    id: number
    type: string
    title: string
    body: string
    data: Record<string, any> | null
    biz_type: string | null
    biz_id: number | null
    is_read: boolean
    read_at: string | null
    created_at: string
}

const POLL_INTERVAL_MS = 60_000

/**
 * 用户站内通知 store（classic / vault 两套模板共用，业务逻辑只此一份）。
 *
 * 轮询策略：
 * - 仅登录态启动，每 60s 静默拉一次 unread-count；
 * - 页面切到后台（visibilityState === 'hidden'）暂停定时器；
 * - 切回前台时立即拉一次并恢复轮询；
 * - 登出后停止轮询并清空本地状态。
 */
export const useNotificationStore = defineStore('notification', () => {
    const auth = useUserAuthStore()

    const unreadCount = ref(0)
    const notifications = ref<NotificationItem[]>([])
    const page = ref(1)
    const pageSize = ref(20)
    const total = ref(0)
    const loadingList = ref(false)

    let pollTimer: number | null = null

    const hasMore = computed(() => notifications.value.length < total.value)

    /** 红点角标文本：>99 显示 99+ */
    const badgeText = computed(() => {
        const n = unreadCount.value
        if (n <= 0) return ''
        return n > 99 ? '99+' : String(n)
    })

    const fetchUnreadCount = async () => {
        if (!auth.isAuthenticated) return
        try {
            const res = await notificationAPI.unreadCount()
            unreadCount.value = Number(res.data?.data?.count ?? 0) || 0
        } catch {
            // 轮询失败静默处理，避免打扰用户
        }
    }

    const fetchList = async (targetPage = 1, append = false) => {
        if (!auth.isAuthenticated) return
        loadingList.value = true
        try {
            const res = await notificationAPI.list(targetPage, pageSize.value)
            const payload = res.data?.data || {}
            const items = (payload.items || []) as NotificationItem[]
            notifications.value = append ? [...notifications.value, ...items] : items
            page.value = Number(payload.page || targetPage)
            pageSize.value = Number(payload.page_size || pageSize.value)
            total.value = Number(payload.total || 0)
        } finally {
            loadingList.value = false
        }
    }

    const loadMore = async () => {
        if (!hasMore.value || loadingList.value) return
        await fetchList(page.value + 1, true)
    }

    const markRead = async (id: number) => {
        const item = notifications.value.find((n) => n.id === id)
        const wasUnread = Boolean(item && !item.is_read)
        if (item) {
            item.is_read = true
            item.read_at = new Date().toISOString()
        }
        if (wasUnread) {
            unreadCount.value = Math.max(0, unreadCount.value - 1)
        }
        try {
            await notificationAPI.markRead(id)
        } catch (err) {
            // 回滚本地状态
            if (item && wasUnread) {
                item.is_read = false
                item.read_at = null
            }
            if (wasUnread) {
                unreadCount.value += 1
            }
            throw err
        }
    }

    const markAllRead = async () => {
        const prevUnread = unreadCount.value
        unreadCount.value = 0
        const now = new Date().toISOString()
        notifications.value.forEach((n) => {
            if (!n.is_read) {
                n.is_read = true
                n.read_at = now
            }
        })
        try {
            await notificationAPI.markAllRead()
        } catch (err) {
            unreadCount.value = prevUnread
            throw err
        }
    }

    const stopPolling = () => {
        if (pollTimer !== null) {
            window.clearInterval(pollTimer)
            pollTimer = null
        }
    }

    const startPolling = () => {
        if (!auth.isAuthenticated) return
        if (pollTimer !== null) return
        if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
        // 启动时立即拉一次，保证登录后红点不滞后
        void fetchUnreadCount()
        pollTimer = window.setInterval(() => {
            void fetchUnreadCount()
        }, POLL_INTERVAL_MS)
    }

    const reset = () => {
        stopPolling()
        unreadCount.value = 0
        notifications.value = []
        page.value = 1
        total.value = 0
    }

    const onVisibilityChange = () => {
        if (!auth.isAuthenticated) return
        if (typeof document === 'undefined') return
        if (document.visibilityState === 'hidden') {
            stopPolling()
        } else {
            void fetchUnreadCount()
            startPolling()
        }
    }

    // 登录态联动：登录即启动轮询，登出即停止并清空
    watch(
        () => auth.isAuthenticated,
        (loggedIn) => {
            if (loggedIn) {
                startPolling()
            } else {
                reset()
            }
        },
        { immediate: true },
    )

    if (typeof document !== 'undefined') {
        document.addEventListener('visibilitychange', onVisibilityChange)
    }

    return {
        unreadCount,
        notifications,
        page,
        pageSize,
        total,
        loadingList,
        hasMore,
        badgeText,
        fetchUnreadCount,
        fetchList,
        loadMore,
        markRead,
        markAllRead,
        startPolling,
        stopPolling,
        reset,
    }
})
