import { onMounted, onUnmounted } from 'vue'

export type PollingCallback = () => Promise<unknown> | unknown

export interface UsePollingReturn {
    start: () => void
    stop: () => void
    /** 永久停止（组件卸载仍会自动清理，无需再调） */
    stopForever: () => void
}

/**
 * 通用轮询封装（classic / vault 共用，纯逻辑无 UI）。
 *
 * - 每 intervalMs 执行一次 callback，组件挂载即启动；
 * - 页面切到后台（document.visibilityState === 'hidden'）暂停定时器，
 *   切回前台立即执行一次并恢复轮询；
 * - callback 抛出的异常被静默吞掉，避免单次失败打挂定时器；
 * - 组件卸载自动移除监听并清理定时器。
 */
export function usePolling(callback: PollingCallback, intervalMs: number): UsePollingReturn {
    let timer: number | null = null
    let permanentlyStopped = false

    const run = async () => {
        if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
        try {
            await callback()
        } catch {
            // 轮询失败静默处理，避免打扰用户
        }
    }

    const stop = () => {
        if (timer !== null) {
            window.clearInterval(timer)
            timer = null
        }
    }

    const start = () => {
        if (permanentlyStopped) return
        stop()
        if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
        void run()
        timer = window.setInterval(() => void run(), intervalMs)
    }

    const stopForever = () => {
        permanentlyStopped = true
        stop()
    }

    const onVisibilityChange = () => {
        if (typeof document === 'undefined') return
        if (document.visibilityState === 'hidden') {
            stop()
        } else {
            start()
        }
    }

    onMounted(() => {
        start()
        if (typeof document !== 'undefined') {
            document.addEventListener('visibilitychange', onVisibilityChange)
        }
    })

    onUnmounted(() => {
        stop()
        if (typeof document !== 'undefined') {
            document.removeEventListener('visibilitychange', onVisibilityChange)
        }
    })

    return { start, stop, stopForever }
}
