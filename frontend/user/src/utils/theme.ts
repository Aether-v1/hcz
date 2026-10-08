import { ref, watch } from 'vue'

const THEME_KEY = 'hcz_theme'

export type Theme = 'light' | 'dark' | 'system'

const getSystemTheme = (): 'light' | 'dark' => {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const getSavedTheme = (): Theme => {
    const saved = localStorage.getItem(THEME_KEY)
    if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
    return 'system'
}

const theme = ref<Theme>(getSavedTheme())

// 解析当前实际生效的主题（system 时跟随系统）
const resolveTheme = (t: Theme): 'light' | 'dark' => {
    return t === 'system' ? getSystemTheme() : t
}

// 同步 <meta name="theme-color"> 与当前生效主题，
// 使 iOS Safari / standalone 顶部状态栏跟随浅色/深色（含手动切换，非仅系统偏好）。
const THEME_COLOR_LIGHT = '#eef4fb'
const THEME_COLOR_DARK = '#000000'
const syncThemeColor = (effective: 'light' | 'dark') => {
    const meta = document.querySelector('meta[name="theme-color"]')
    if (meta) meta.setAttribute('content', effective === 'dark' ? THEME_COLOR_DARK : THEME_COLOR_LIGHT)
}

const applyTheme = (t: Theme) => {
    const root = document.documentElement
    const effective = resolveTheme(t)
    if (effective === 'dark') {
        root.classList.add('dark')
    } else {
        root.classList.remove('dark')
    }
    syncThemeColor(effective)
    localStorage.setItem(THEME_KEY, t)
}

// system 模式下监听系统主题变化
let mediaQuery: MediaQueryList | null = null
let mediaHandler: (() => void) | null = null

const startSystemListener = () => {
    if (mediaQuery) return
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaHandler = () => {
        if (theme.value === 'system') {
            applyTheme('system')
        }
    }
    mediaQuery.addEventListener('change', mediaHandler)
}

watch(theme, (newVal) => {
    applyTheme(newVal)
    if (newVal === 'system') {
        startSystemListener()
    }
}, { immediate: true })

export const useTheme = () => {
    const setTheme = (t: Theme) => {
        const root = document.documentElement
        root.classList.add('hcz-theme-switching')
        void root.offsetWidth
        theme.value = t
        requestAnimationFrame(() => {
            requestAnimationFrame(() => {
                root.classList.remove('hcz-theme-switching')
            })
        })
    }

    // 兼容旧调用（Navbar 已移除主题按钮，保留以防其他地方引用）
    const toggleTheme = () => {
        setTheme(theme.value === 'dark' ? 'light' : 'dark')
    }

    return {
        theme,
        setTheme,
        toggleTheme,
    }
}
