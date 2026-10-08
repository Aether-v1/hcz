import { fileURLToPath, URL } from 'node:url'
import { dirname, resolve } from 'node:path'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import VueI18nPlugin from '@intlify/unplugin-vue-i18n/vite'
import { VitePWA } from 'vite-plugin-pwa'

const cfAsyncModuleScriptPlugin = () => ({
  name: 'cfasync-module-script',
  transformIndexHtml(html: string) {
    return html.replace(
      /<script\s+type="module"(?![^>]*data-cfasync)/g,
      '<script data-cfasync="false" type="module"',
    )
  },
})

// https://vite.dev/config/
export default defineConfig(({ mode }) => ({
  plugins: [
    vue(),
    // 语言包 JSON 构建期预编译为 AST，运行时无需 message compiler（vue-i18n 走 runtime-only 构建）
    VueI18nPlugin({
      include: [resolve(dirname(fileURLToPath(import.meta.url)), 'src/i18n/locales/**')],
      dropMessageCompiler: true,
    }),
    cfAsyncModuleScriptPlugin(),
    // PWA：可安装 App Shell + 保守缓存策略。
    // - 静态资源(hash 后 JS/CSS/图片)：generateSW precache，cache-first（内容寻址，天然安全）
    // - HTML 导航：navigateFallback 到 precached index.html，保证离线可打开页面壳
    // - /api：NetworkOnly —— 余额/订单/佣金/C2C/钱包/积分等实时数据绝不以旧缓存冒充
    // - /uploads：NetworkFirst —— 仅图片类静态资源，短超时回退缓存
    // registerType: 'prompt' —— 新版本不静默替换，交由 UI 提示用户主动刷新
    VitePWA({
      registerType: 'prompt',
      includeAssets: ['favicon-16.png', 'favicon-32.png', 'apple-touch-icon.png'],
      manifest: {
        name: 'HCZ · Digital Commerce Platform',
        short_name: 'HCZ',
        description: '充值、钱包、积分、分销与 C2C 交易的一站式数字商业平台。',
        lang: 'zh-CN',
        dir: 'ltr',
        display: 'standalone',
        orientation: 'portrait',
        start_url: '/',
        scope: '/',
        background_color: '#f5f8fd',
        theme_color: '#eef4fb',
        icons: [
          { src: '/pwa-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/pwa-512.png', sizes: '512x512', type: 'image/png' },
          { src: '/maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,webp,woff,woff2}'],
        navigateFallback: 'index.html',
        navigateFallbackDenylist: [/^\/api/, /^\/uploads/],
        cleanupOutdatedCaches: true,
        maximumFileSizeToCacheInBytes: 4 * 1024 * 1024,
        runtimeCaching: [
          {
            urlPattern: ({ url }) => url.pathname.startsWith('/api'),
            handler: 'NetworkOnly',
          },
          {
            urlPattern: ({ url }) => url.pathname.startsWith('/uploads'),
            handler: 'NetworkFirst',
            options: {
              cacheName: 'hcz-uploads',
              networkTimeoutSeconds: 10,
              expiration: { maxEntries: 200, maxAgeSeconds: 60 * 60 * 24 * 7 },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  // vue-i18n 9.x 需显式开启 JIT，runtime 才能解释预编译的 AST 消息（v10+ 默认开启，届时可移除）
  define: {
    __INTLIFY_JIT_COMPILATION__: true,
  },
  esbuild: mode === 'production' ? { drop: ['console', 'debugger'] } : {},
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-qrcode': ['qrcode'],
          'vendor-vue-i18n': ['vue-i18n'],
        },
      },
    },
  },
  server: {
    host: '0.0.0.0', // 监听所有网络接口
    port: 5173,
    strictPort: true,
    // 允许通过分销商子域名(*.hcz.test)访问 dev server，否则 Vite 5.4+ 会拦截非 localhost 的 Host
    allowedHosts: ['.hcz.test'],
    proxy: {
      // changeOrigin 必须为 false：保留原始子域名 Host，后端才能据此解析分销商租户
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
      '/sitemap.xml': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
      '/robots.txt': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
    }
  },
}))

