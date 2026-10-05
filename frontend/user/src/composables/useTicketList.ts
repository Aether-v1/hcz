import { computed, ref } from 'vue'
import { supportAPI } from '../api/support'
import type { SupportTicketListData, SupportTicketSummary } from '../types/support'
import { usePolling } from './usePolling'

/** 状态过滤页签取值；all 表示不传 status 参数 */
export type TicketFilter = 'all' | 'waiting_support' | 'waiting_user' | 'resolved' | 'closed'

export const TICKET_FILTERS: TicketFilter[] = [
    'all',
    'waiting_support',
    'waiting_user',
    'resolved',
    'closed',
]

const PAGE_SIZE = 20
const POLL_INTERVAL_MS = 20_000

/**
 * 工单列表逻辑：过滤页签、分页加载、轮询刷新未读数。
 * 轮询间隔 20s（hint 要求 15-30s），页面切后台自动暂停。
 */
export function useTicketList() {
    const items = ref<SupportTicketSummary[]>([])
    const total = ref(0)
    const page = ref(1)
    const loading = ref(false)
    const loadingMore = ref(false)
    const filter = ref<TicketFilter>('all')
    const selectedId = ref<number | null>(null)

    const hasMore = computed(() => items.value.length < total.value)

    const fetchList = async (targetPage = 1, append = false) => {
        if (append) {
            loadingMore.value = true
        } else {
            loading.value = true
        }
        try {
            const params: { page: number; page_size: number; status?: string } = {
                page: targetPage,
                page_size: PAGE_SIZE,
            }
            if (filter.value !== 'all') {
                params.status = filter.value
            }
            const res = await supportAPI.tickets(params)
            const payload = (res.data.data || {}) as SupportTicketListData
            const list = payload.items || []
            items.value = append ? [...items.value, ...list] : list
            total.value = Number(payload.total || 0)
            page.value = Number(payload.page || targetPage)
        } finally {
            loading.value = false
            loadingMore.value = false
        }
    }

    /** 切换页签后回到第一页 */
    const setFilter = (next: TicketFilter) => {
        if (filter.value === next) return
        filter.value = next
        selectedId.value = null
        return fetchList(1)
    }

    const loadMore = () => {
        if (!hasMore.value || loading.value || loadingMore.value) return
        return fetchList(page.value + 1, true)
    }

    const selectTicket = (id: number) => {
        selectedId.value = id
    }

    const deselect = () => {
        selectedId.value = null
    }

    // 每 20s 静默刷新第一页，更新未读红点/状态；切后台自动暂停
    const polling = usePolling(() => fetchList(1), POLL_INTERVAL_MS)

    return {
        items,
        total,
        page,
        loading,
        loadingMore,
        filter,
        selectedId,
        hasMore,
        fetchList,
        setFilter,
        loadMore,
        selectTicket,
        deselect,
        stopListPolling: polling.stopForever,
    }
}
