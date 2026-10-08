import { computed, ref } from 'vue'
import { registerSW } from 'virtual:pwa-register'

/**
 * PWA App Shell 运行时状态（模块级单例，全局共享一份）：
 * - standalone / iOS 检测
 * - 在线状态
 * - Android/Chromium 安装提示（beforeinstallprompt，不自动弹）
 * - iOS Safari 手动“添加到主屏幕”提示（localStorage 记录关闭时间，30 天冷却）
 * - Service Worker 新版本提示（registerType: 'prompt'，不静默替换）
 */

const IOS_HINT_KEY = 'hcz_pwa_ios_hint_dismissed_at'
const IOS_HINT_COOLDOWN_MS = 30 * 24 * 60 * 60 * 1000

const readIosHintDismissed = (): boolean => {
    try {
        const ts = Number(localStorage.getItem(IOS_HINT_KEY) || 0)
        return Boolean(ts) && Date.now() - ts < IOS_HINT_COOLDOWN_MS
    } catch {
        return false
    }
}

const detectStandalone = (): boolean => {
    if (typeof window === 'undefined') return false
    const displayMode = window.matchMedia?.('(display-mode: standalone)').matches === true
    const iosStandalone = (navigator as unknown as { standalone?: boolean }).standalone === true
    return displayMode || iosStandalone
}

const detectIOS = (): boolean => {
    if (typeof navigator === 'undefined') return false
    const ua = navigator.userAgent || ''
    return /iPad|iPhone|iPod/.test(ua)
        || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
}

type BeforeInstallPromptEvent = Event & {
    prompt: () => Promise<void>
    userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const online = ref(typeof navigator === 'undefined' ? true : navigator.onLine !== false)
const standalone = ref(detectStandalone())
const isIOS = detectIOS()
const canInstall = ref(false)
const needRefresh = ref(false)
const offlineReady = ref(false)
const iosHintDismissed = ref(readIosHintDismissed())

let deferredPrompt: BeforeInstallPromptEvent | null = null

if (typeof window !== 'undefined') {
    window.addEventListener('online', () => { online.value = true })
    window.addEventListener('offline', () => { online.value = false })
    window.addEventListener('beforeinstallprompt', (event) => {
        // 阻止浏览器自动安装气泡，改由 UI 在合适位置提示。
        event.preventDefault()
        deferredPrompt = event as BeforeInstallPromptEvent
        canInstall.value = true
    })
    window.addEventListener('appinstalled', () => {
        deferredPrompt = null
        canInstall.value = false
    })
}

const updateSW = registerSW({
    immediate: true,
    onNeedRefresh() {
        needRefresh.value = true
    },
    onOfflineReady() {
        offlineReady.value = true
    },
    onRegisteredSW(_url, registration) {
        // 每小时检查一次是否有新版本；仅提示，不静默 reload 用户正在填写的表单。
        if (registration) {
            window.setInterval(() => {
                void registration.update().catch(() => undefined)
            }, 60 * 60 * 1000)
        }
    },
})

const promptInstall = async (): Promise<boolean> => {
    if (!deferredPrompt) return false
    await deferredPrompt.prompt()
    const { outcome } = await deferredPrompt.userChoice
    deferredPrompt = null
    canInstall.value = false
    return outcome === 'accepted'
}

const dismissIosHint = () => {
    iosHintDismissed.value = true
    try {
        localStorage.setItem(IOS_HINT_KEY, String(Date.now()))
    } catch {
        /* localStorage 不可用时仅本次会话内隐藏 */
    }
}

const showIosInstallHint = computed(
    () => isIOS && !standalone.value && !canInstall.value && !iosHintDismissed.value,
)

export function usePwa() {
    return {
        online,
        standalone,
        isIOS,
        canInstall,
        needRefresh,
        offlineReady,
        showIosInstallHint,
        promptInstall,
        dismissIosHint,
        updateSW,
    }
}
