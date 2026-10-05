import { onBeforeUnmount, onMounted, ref } from 'vue'
import { supportAPI } from '@/api/support'

const POLL_INTERVAL = 15000

// 全局共享：导航栏未读红点。所有客服页面共用同一份轮询，避免重复请求。
const unreadCount = ref(0)
let started = false
let timer: ReturnType<typeof setInterval> | null = null
const subscribers = new Set<number>()

async function refresh() {
  if (typeof document !== 'undefined' && document.hidden) return
  try {
    const res = await supportAPI.getTickets({ page: 1, page_size: 1, unread_only: true })
    const total = Number(res.data?.data?.total ?? 0)
    unreadCount.value = Number.isFinite(total) ? total : 0
  } catch {
    // 静默失败，红点保持上一次值
  }
}

function onVisibilityChange() {
  if (typeof document !== 'undefined' && !document.hidden) {
    refresh()
  }
}

function start() {
  if (started) return
  started = true
  refresh()
  timer = setInterval(refresh, POLL_INTERVAL)
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibilityChange)
  }
}

function stop() {
  started = false
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', onVisibilityChange)
  }
}

/**
 * 导航栏未读工单红点轮询。
 * - 每 15s 拉取一次 unread_only 列表的 total
 * - document.hidden 时暂停，回到前台立即刷新
 * - 全局单例：多个订阅方共享同一定时器
 */
export function useTicketUnread() {
  const subId = Math.random()
  subscribers.add(subId)

  onMounted(() => start())
  onBeforeUnmount(() => {
    subscribers.delete(subId)
    if (subscribers.size === 0) {
      stop()
    }
  })

  return {
    unreadCount,
    refreshUnread: refresh,
  }
}
