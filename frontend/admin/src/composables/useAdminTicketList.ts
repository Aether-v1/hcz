import { onBeforeUnmount, onMounted, reactive, ref, type Ref } from 'vue'
import { supportAPI } from '@/api/support'
import type { SupportTicketListItem } from '@/types/support'
import { notifySuccess } from '@/utils/notify'
import { useI18n } from 'vue-i18n'

const POLL_INTERVAL = 15000

export interface TicketListFilters {
  status: string
  category_id: string
  priority: string
  assigned: string // '__all__' | 'unassigned' | 'me' | admin_id string
  unread_only: boolean
  search: string
}

/**
 * 工单列表逻辑：筛选 / 搜索 / 分页 / 领取 / 15s 轮询（hidden 暂停）。
 */
export function useAdminTicketList(myAdminId: Ref<number>) {
  const { t } = useI18n()
  const loading = ref(true)
  const list = ref<SupportTicketListItem[]>([])
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)

  const filters = reactive<TicketListFilters>({
    status: '__all__',
    category_id: '__all__',
    priority: '__all__',
    assigned: '__all__',
    unread_only: false,
    search: '',
  })

  let timer: ReturnType<typeof setInterval> | null = null

  const buildParams = () => {
    const params: Record<string, unknown> = {
      page: page.value,
      page_size: pageSize.value,
    }
    if (filters.status && filters.status !== '__all__') params.status = filters.status
    if (filters.category_id && filters.category_id !== '__all__')
      params.category_id = Number(filters.category_id)
    if (filters.priority && filters.priority !== '__all__') params.priority = filters.priority
    if (filters.assigned === 'unassigned') {
      params.assigned_admin_id = 0
    } else if (filters.assigned === 'me') {
      if (myAdminId.value) params.assigned_admin_id = myAdminId.value
    } else if (filters.assigned && filters.assigned !== '__all__') {
      params.assigned_admin_id = Number(filters.assigned)
    }
    if (filters.unread_only) params.unread_only = true
    if (filters.search.trim()) params.search = filters.search.trim()
    return params
  }

  const fetchList = async (preserveRows = false) => {
    if (!preserveRows) loading.value = true
    try {
      const res = await supportAPI.getTickets(buildParams())
      const data = res.data?.data || {}
      list.value = data.items || []
      total.value = Number(data.total || 0)
      page.value = Number(data.page || page.value)
      pageSize.value = Number(data.page_size || pageSize.value)
    } catch {
      if (!preserveRows) list.value = []
    } finally {
      loading.value = false
    }
  }

  const search = () => {
    page.value = 1
    fetchList()
  }

  const changePage = (p: number) => {
    page.value = p
    fetchList()
  }

  const changePageSize = (size: number) => {
    pageSize.value = size
    page.value = 1
    fetchList()
  }

  const claim = async (ticket: SupportTicketListItem) => {
    try {
      await supportAPI.assign(ticket.id, {})
      notifySuccess(t('admin.support.claimSuccess'))
      await fetchList(true)
    } catch {
      /* 错误已统一提示 */
    }
  }

  const poll = () => {
    if (typeof document !== 'undefined' && document.hidden) return
    fetchList(true)
  }

  const onVisibility = () => {
    if (typeof document !== 'undefined' && !document.hidden) poll()
  }

  onMounted(() => {
    fetchList()
    timer = setInterval(poll, POLL_INTERVAL)
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', onVisibility)
    }
  })

  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibility)
    }
  })

  return {
    loading,
    list,
    total,
    page,
    pageSize,
    filters,
    fetchList,
    search,
    changePage,
    changePageSize,
    claim,
  }
}
