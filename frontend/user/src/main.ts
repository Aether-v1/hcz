import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createHead } from '@unhead/vue/client'
import './style.css'
// 主题模块的 immediate watcher 在启动时解析浅色/深色并同步 html.dark 与 <meta name="theme-color">，
// 否则未打开主题设置页时 theme-color 会停留在 index.html 的静态浅色值。
import './utils/theme'
import App from './App.vue'
import router, { warmupCommonRoutes } from './router'
import i18n, { detectLocale, setI18nLocale, warmupLocaleMessages } from './i18n'
import { useTelegramMiniAppStore } from './stores/telegramMiniApp'

const brandLog = (globalThis as any).console?.log?.bind(console)
brandLog?.(
  '%c HCZ %c Digital Commerce Platform ',
  'background:#0071e3;color:#fff;padding:4px 8px;border-radius:4px 0 0 4px;font-weight:bold;',
  'background:#1d1d1f;color:#f5f5f7;padding:4px 8px;border-radius:0 4px 4px 0;',
)
brandLog?.('%cGitHub → https://github.com/Aether-v1/hcz', 'color:#6e6e73;')

const app = createApp(App)
const head = createHead()
const pinia = createPinia()

app.use(pinia)
app.use(head)
app.use(router)
app.use(i18n)

// 非默认语言的语言包为懒加载 chunk，挂载前并行加载，避免首屏文案闪现兜底语言
Promise.all([
  useTelegramMiniAppStore(pinia).init(),
  setI18nLocale(detectLocale()),
]).then(() => {
  app.mount('#app')
})

void router.isReady().then(() => {
    warmupCommonRoutes()
    warmupLocaleMessages()
})

