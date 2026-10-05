import { computed, ref } from 'vue'
import { supportAPI } from '../api/support'
import type {
    SupportAttachment,
    SupportCategory,
    SupportMessage,
    SupportTicketDetail,
    SupportTicketDetailData,
} from '../types/support'
import { usePolling } from './usePolling'

const POLL_INTERVAL_MS = 15_000
/** resolved 状态允许重新打开的期限：7 天 */
const REOPEN_EXPIRE_MS = 7 * 24 * 60 * 60 * 1000

/**
 * 单个工单详情逻辑：加载详情、15s 轮询（页面切后台暂停、closed 后停止）、
 * 发送回复、关闭/重新打开工单。classic / vault 共用。
 *
 * @param idGetter 返回当前工单 id 的 getter（路由参数变化时也能响应）
 */
export function useSupportTicket(idGetter: () => number | string) {
    const ticket = ref<SupportTicketDetail | null>(null)
    const category = ref<SupportCategory | null>(null)
    const messages = ref<SupportMessage[]>([])
    const attachments = ref<SupportAttachment[]>([])

    const loading = ref(false)
    const loadingFailed = ref(false)
    const sending = ref(false)
    const acting = ref(false)

    /** resolved_at 距今是否已超过 7 天（超过则不允许重新打开） */
    const reopenExpired = computed(() => {
        const resolvedAt = ticket.value?.resolved_at
        if (!resolvedAt) return false
        return Date.now() - new Date(resolvedAt).getTime() > REOPEN_EXPIRE_MS
    })

    const isClosed = computed(() => ticket.value?.status === 'closed')

    let polling: ReturnType<typeof usePolling> | null = null

    const fetchDetail = async (isPoll = false) => {
        const id = idGetter()
        if (!id) return
        if (!isPoll) loading.value = true
        try {
            const res = await supportAPI.ticketDetail(id)
            const data = (res.data.data || {}) as SupportTicketDetailData
            ticket.value = data.ticket || null
            category.value = data.category || null
            messages.value = data.messages || []
            attachments.value = data.attachments || []
            loadingFailed.value = false
            // 工单关闭后停止轮询
            if (data.ticket?.status === 'closed') {
                polling?.stop()
            }
        } catch (err) {
            if (!isPoll) {
                loadingFailed.value = true
                throw err
            }
        } finally {
            if (!isPoll) loading.value = false
        }
    }

    const sendReply = async (body: string, attachmentIds: number[] = []) => {
        const id = idGetter()
        if (!id || sending.value) return
        sending.value = true
        try {
            await supportAPI.reply(id, {
                body,
                ...(attachmentIds.length ? { attachment_ids: attachmentIds } : {}),
            })
            await fetchDetail(true)
        } finally {
            sending.value = false
        }
    }

    const closeTicket = async () => {
        const id = idGetter()
        if (!id || acting.value) return
        acting.value = true
        try {
            await supportAPI.close(id)
            await fetchDetail(true)
        } finally {
            acting.value = false
        }
    }

    const reopenTicket = async () => {
        const id = idGetter()
        if (!id || acting.value) return
        acting.value = true
        try {
            await supportAPI.reopen(id)
            await fetchDetail(true)
        } finally {
            acting.value = false
        }
    }

    // 15s 轮询刷新详情；closed 后自动停止（见 fetchDetail）
    polling = usePolling(() => fetchDetail(true), POLL_INTERVAL_MS)

    return {
        ticket,
        category,
        messages,
        attachments,
        loading,
        loadingFailed,
        sending,
        acting,
        reopenExpired,
        isClosed,
        fetchDetail,
        sendReply,
        closeTicket,
        reopenTicket,
        stopPolling: polling.stopForever,
    }
}
