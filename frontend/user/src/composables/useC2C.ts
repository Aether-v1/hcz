import { ref, computed, onUnmounted, watch } from 'vue'
import {
    c2cAPI,
    type C2CTrade,
    type C2CTradeStatus,
    C2C_TRADE_POLLING_STATUSES,
    C2C_TRADE_TERMINAL_STATUSES,
} from '../api/c2c'
import { useC2CStore } from '../stores/c2c'
import { toast } from './useToast'

const POLL_INTERVAL_MS = 12_000 // 10~15 秒区间取 12s

// ─── 交易状态 → 中文标签 ────────────────────────────────────────

export const TRADE_STATUS_LABELS: Record<C2CTradeStatus, string> = {
    pending_payment: '待付款',
    paid: '待确认',
    disputed: '申诉中',
    completed: '已完成',
    canceled: '已取消',
    expired: '已超时',
}

export const TRADE_STATUS_VARIANTS: Record<C2CTradeStatus, string> = {
    pending_payment: 'bg-amber-500/10 text-amber-600',
    paid: 'bg-blue-500/10 text-blue-600',
    disputed: 'bg-rose-500/10 text-rose-600',
    completed: 'bg-emerald-500/10 text-emerald-600',
    canceled: 'bg-zinc-500/10 text-zinc-500',
    expired: 'bg-zinc-500/10 text-zinc-500',
}

// ─── 挂单状态 → 中文标签 ────────────────────────────────────────

export const LISTING_STATUS_LABELS: Record<string, string> = {
    active: '进行中',
    paused: '已暂停',
    closed: '已关闭',
}

export const LISTING_STATUS_VARIANTS: Record<string, string> = {
    active: 'bg-emerald-500/10 text-emerald-600',
    paused: 'bg-amber-500/10 text-amber-600',
    closed: 'bg-zinc-500/10 text-zinc-500',
}

// ─── 支付方式类型 → 中文标签 ─────────────────────────────────────

export const PAYMENT_METHOD_TYPE_LABELS: Record<string, string> = {
    bank: '银行卡',
    alipay: '支付宝',
    wechat: '微信支付',
    paynow: 'PayNow',
    custom: '自定义',
}

// ─── 倒计时 composable ──────────────────────────────────────────

export function useCountdown(expiredAt: () => string | undefined) {
    const remainingMs = ref(0)
    const isExpired = ref(false)
    let timer: number | null = null

    const update = () => {
        const exp = expiredAt()
        if (!exp) {
            remainingMs.value = 0
            isExpired.value = true
            return
        }
        const target = new Date(exp.replace(' ', 'T')).getTime()
        const now = Date.now()
        const diff = target - now
        if (diff <= 0) {
            remainingMs.value = 0
            isExpired.value = true
            stop()
        } else {
            remainingMs.value = diff
            isExpired.value = false
        }
    }

    const start = () => {
        stop()
        update()
        timer = window.setInterval(update, 1000)
    }

    const stop = () => {
        if (timer !== null) {
            window.clearInterval(timer)
            timer = null
        }
    }

    onUnmounted(() => stop())

    const formatted = computed(() => {
        const total = Math.max(0, Math.floor(remainingMs.value / 1000))
        const h = Math.floor(total / 3600)
        const m = Math.floor((total % 3600) / 60)
        const s = total % 60
        const pad = (n: number) => String(n).padStart(2, '0')
        return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
    })

    return { remainingMs, isExpired, formatted, start, stop, update }
}

// ─── Trade Detail 轮询 composable ───────────────────────────────

export function useTradePolling(tradeId: number, onExpired?: () => void) {
    const c2cStore = useC2CStore()
    let pollTimer: number | null = null
    let visibilityHandler: (() => void) | null = null

    const shouldPoll = (status: C2CTradeStatus | undefined): boolean => {
        if (!status) return false
        return (C2C_TRADE_POLLING_STATUSES as string[]).includes(status)
    }

    const isTerminal = (status: C2CTradeStatus | undefined): boolean => {
        if (!status) return false
        return (C2C_TRADE_TERMINAL_STATUSES as string[]).includes(status)
    }

    const stopPolling = () => {
        if (pollTimer !== null) {
            window.clearInterval(pollTimer)
            pollTimer = null
        }
    }

    const startPolling = () => {
        stopPolling()
        if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
        pollTimer = window.setInterval(() => {
            void c2cStore.fetchCurrentTrade(tradeId)
        }, POLL_INTERVAL_MS)
    }

    const onVisibilityChange = () => {
        if (typeof document === 'undefined') return
        if (document.visibilityState === 'hidden') {
            stopPolling()
        } else {
            // 切回前台立即刷新一次
            void c2cStore.fetchCurrentTrade(tradeId)
            if (shouldPoll(c2cStore.currentTrade?.status as C2CTradeStatus)) {
                startPolling()
            }
        }
    }

    // 监听 trade 状态变化：terminal 停止轮询，非 terminal 且在 polling 列表中则启动
    watch(
        () => c2cStore.currentTrade?.status,
        (status) => {
            if (isTerminal(status as C2CTradeStatus)) {
                stopPolling()
            } else if (shouldPoll(status as C2CTradeStatus)) {
                startPolling()
            }
        },
    )

    // 倒计时结束回调：刷新 trade
    const handleCountdownExpired = () => {
        void c2cStore.fetchCurrentTrade(tradeId)
        onExpired?.()
    }

    const init = () => {
        if (typeof document !== 'undefined') {
            visibilityHandler = onVisibilityChange
            document.addEventListener('visibilitychange', visibilityHandler)
        }
    }

    const cleanup = () => {
        stopPolling()
        if (visibilityHandler && typeof document !== 'undefined') {
            document.removeEventListener('visibilitychange', visibilityHandler)
        }
    }

    onUnmounted(() => cleanup())

    return {
        startPolling,
        stopPolling,
        handleCountdownExpired,
        init,
        cleanup,
        isPolling: computed(() => pollTimer !== null),
    }
}

// ─── Trade 操作 composable（含错误映射） ─────────────────────────

export function useC2CTradeActions() {
    const c2cStore = useC2CStore()

    const mapError = (err: unknown): string => {
        const msg = err instanceof Error ? err.message : String(err)
        // 后端错误码映射为中文友好提示
        const mapping: Record<string, string> = {
            'error.c2c_not_found': '挂单或交易不存在',
            'error.c2c_self_trade': '不能购买自己的挂单',
            'error.c2c_listing_not_active': '挂单当前不可交易（已暂停或已关闭）',
            'error.c2c_invalid_amount': '交易金额不在挂单的最小/最大范围内，或数量无效',
            'error.c2c_status_invalid': '当前交易状态不允许此操作',
            'error.idempotency_key_required': '请求缺少幂等键，请重试',
            'error.c2c_disabled': 'C2C 交易功能暂未开放',
            'error.c2c_totp_required': '请先启用谷歌验证器（TOTP）后再使用 C2C 交易',
            'error.c2c_new_user_cooldown': '新账号需等待冷却期结束后才能使用 C2C 交易',
            'error.c2c_no_payment_method': '请先添加并启用至少一个收款方式',
            'error.c2c_daily_limit': '今日交易额度已达上限，请明日再试',
            'error.c2c_insufficient_balance': '可用余额不足，无法完成此操作',
            'error.c2c_permission_denied': '您没有权限执行此操作（账号可能已被限制 C2C 交易）',
            'error.c2c_user_inactive': '账号状态异常，无法使用 C2C 交易',
        }
        for (const [key, label] of Object.entries(mapping)) {
            if (msg.includes(key)) return label
        }
        return msg || '操作失败，请稍后重试'
    }

    const wrapAction = async <T>(fn: () => Promise<T>, successMsg?: string): Promise<T> => {
        try {
            const result = await fn()
            if (successMsg) toast.success(successMsg)
            return result
        } catch (err) {
            const friendly = mapError(err)
            toast.error(friendly)
            throw err
        }
    }

    const markPaid = async (tradeId: number, paymentReference?: string) =>
        wrapAction(async () => {
            const res = await c2cAPI.markPaid(tradeId, { payment_reference: paymentReference })
            c2cStore.currentTrade = (res.data?.data || null) as C2CTrade | null
            return res
        }, '已标记付款，等待卖家确认')

    const cancelTrade = async (tradeId: number) =>
        wrapAction(async () => {
            const res = await c2cAPI.cancelTrade(tradeId)
            c2cStore.currentTrade = (res.data?.data || null) as C2CTrade | null
            return res
        }, '交易已取消')

    const confirmTrade = async (tradeId: number) =>
        wrapAction(async () => {
            const res = await c2cAPI.confirmTrade(tradeId)
            c2cStore.currentTrade = (res.data?.data || null) as C2CTrade | null
            // 卖家确认后刷新钱包和通知
            await c2cStore.fetchWallet()
            return res
        }, '已确认收款，USDT 已转入买家钱包')

    const disputeTrade = async (tradeId: number, reason: string, description?: string, evidence?: string) =>
        wrapAction(async () => {
            const res = await c2cAPI.disputeTrade(tradeId, { reason, description, evidence })
            // 刷新 trade 状态
            await c2cStore.fetchCurrentTrade(tradeId)
            return res
        }, '申诉已提交，等待平台仲裁')

    const createTrade = async (listingId: number, usdtAmount: string, paymentMethodId?: number) =>
        wrapAction(async () => {
            const res = await c2cAPI.createTrade({
                listing_id: listingId,
                usdt_amount: usdtAmount,
                payment_method_id: paymentMethodId,
            })
            return (res.data?.data || null) as C2CTrade | null
        })

    return {
        mapError,
        markPaid,
        cancelTrade,
        confirmTrade,
        disputeTrade,
        createTrade,
    }
}
