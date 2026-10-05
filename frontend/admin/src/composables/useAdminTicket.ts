import { onBeforeUnmount, onMounted, ref } from 'vue'
import { supportAPI } from '@/api/support'
import type { SupportTicketDetailResponse } from '@/types/support'
import { notifySuccess } from '@/utils/notify'
import { useI18n } from 'vue-i18n'

const POLL_INTERVAL = 15000

/**
 * 工单详情逻辑：拉取详情 / 回复 / 分配 / 改优先级 / 解决关闭重开 / 15s 轮询（hidden 暂停）。
 */
export function useAdminTicket(ticketId: number) {
  const { t } = useI18n()
  const loading = ref(true)
  const detail = ref<SupportTicketDetailResponse | null>(null)
  const acting = ref(false)

  let timer: ReturnType<typeof setInterval> | null = null

  const fetchDetail = async (preserve = false) => {
    if (!preserve) loading.value = true
    try {
      const res = await supportAPI.getTicket(ticketId)
      detail.value = (res.data?.data as SupportTicketDetailResponse) || null
    } catch {
      if (!preserve) detail.value = null
    } finally {
      loading.value = false
    }
  }

  const reply = async (body: string, attachment_ids: number[] = []) => {
    acting.value = true
    try {
      await supportAPI.reply(ticketId, { body, attachment_ids })
      notifySuccess(t('admin.support.replySuccess'))
      await fetchDetail(true)
      return true
    } catch {
      return false
    } finally {
      acting.value = false
    }
  }

  const assign = async (admin_id?: number) => {
    acting.value = true
    try {
      await supportAPI.assign(ticketId, admin_id ? { admin_id } : {})
      notifySuccess(t('admin.support.assignSuccess'))
      await fetchDetail(true)
      return true
    } catch {
      return false
    } finally {
      acting.value = false
    }
  }

  const changePriority = async (priority: string) => {
    acting.value = true
    try {
      await supportAPI.changePriority(ticketId, { priority })
      notifySuccess(t('admin.support.priorityChangedSuccess'))
      await fetchDetail(true)
      return true
    } catch {
      return false
    } finally {
      acting.value = false
    }
  }

  const resolve = async (reason?: string) => {
    acting.value = true
    try {
      await supportAPI.resolve(ticketId, reason ? { reason } : {})
      notifySuccess(t('admin.support.resolveSuccess'))
      await fetchDetail(true)
      return true
    } catch {
      return false
    } finally {
      acting.value = false
    }
  }

  const close = async (reason?: string) => {
    acting.value = true
    try {
      await supportAPI.close(ticketId, reason ? { reason } : {})
      notifySuccess(t('admin.support.closeSuccess'))
      await fetchDetail(true)
      return true
    } catch {
      return false
    } finally {
      acting.value = false
    }
  }

  const reopen = async (reason?: string) => {
    acting.value = true
    try {
      await supportAPI.reopen(ticketId, reason ? { reason } : {})
      notifySuccess(t('admin.support.reopenSuccess'))
      await fetchDetail(true)
      return true
    } catch {
      return false
    } finally {
      acting.value = false
    }
  }

  const poll = () => {
    if (typeof document !== 'undefined' && document.hidden) return
    fetchDetail(true)
  }

  const onVisibility = () => {
    if (typeof document !== 'undefined' && !document.hidden) poll()
  }

  onMounted(() => {
    fetchDetail()
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
    detail,
    acting,
    fetchDetail,
    reply,
    assign,
    changePriority,
    resolve,
    close,
    reopen,
  }
}
