# DOMPurify 安全门审计报告

**审计基准目录**: `E:\Users\orang\Downloads\Compressed\hcz_v1\frontend\user\src`（生产源码，git-tracked）
**对比目录**: `E:\Users\orang\Downloads\Compressed\hcz_user\src`（外部开发副本，非生产）
**审计日期**: 2026-10-05
**最终更新**: 2026-10-05（同步修复后重建复验）
**构建产物**: 从同步后源码重新构建的 `dist/`（已删除旧 dist 后重建）

---

## 〇、同步修复记录

### 同步内容（从 hcz_user → hcz_v1\frontend\user\src）

经逐文件 `Compare-Object` 对比，确认以下 6 个文件差异**仅为 DOMPurify 安全修复**，无其他业务逻辑变更，已执行同步覆盖：

| # | 文件路径 | 修复内容 |
|---|---------|---------|
| 1 | `src/utils/content.ts` | 新增 `import DOMPurify` 和 `sanitizeRichHtml(html)` 函数（先 processHtmlForDisplay 图片路径改写，再过 DOMPurify.sanitize） |
| 2 | `src/composables/useLegal.ts` | content computed 中对 legal.terms/privacy 原始 HTML 增加 `DOMPurify.sanitize(raw)` 处理 |
| 3 | `src/views/ProductDetail.vue` | `processHtmlForDisplay` → `sanitizeRichHtml`（v-html 调用 + import 替换） |
| 4 | `src/views/BlogDetail.vue` | 同上 |
| 5 | `src/templates/vault/ProductDetail.vue` | 同上（vault 模板） |
| 6 | `src/templates/vault/BlogDetail.vue` | 同上（vault 模板） |

---

## 一、全量 v-html 审计表（同步后复验）

生产源码 `hcz_v1/frontend/user/src` 共 **13 处实际 v-html**（另有 4 处注释提及 v-html，非使用）。

| # | 文件路径 | 行号 | v-html 表达式 | 是否 sanitized | sanitizer 来源 | 判定 |
|---|---------|------|--------------|---------------|---------------|------|
| 1 | `components/AnnouncementModal.vue` | 166 | `sanitizedContent` | ✅ 是 | 组件内 computed: `DOMPurify.sanitize(processHtmlForDisplay(...))` | **PASS** |
| 2 | `views/BlogDetail.vue` | 58 | `sanitizeRichHtml(getLocalizedText(post.content))` | ✅ 是 | `utils/content.ts` 的 `sanitizeRichHtml()` = `DOMPurify.sanitize(processHtmlForDisplay(html))` | **PASS** |
| 3 | `views/Legal.vue` | 19 | `content` | ✅ 是 | `useLegal.ts` content computed 内 `DOMPurify.sanitize(raw)` | **PASS** |
| 4 | `views/GuestOrderDetail.vue` | 200 | `block.html` | ✅ 是 | `instructionBlocks()` → `sanitizeInstructionsHtml()` → `DOMPurify.sanitize()` | **PASS** |
| 5 | `views/GuestOrderDetail.vue` | 259 | `block.html` | ✅ 是 | 同上 | **PASS** |
| 6 | `views/ProductDetail.vue` | 328 | `sanitizeRichHtml(getLocalizedText(product.content))` | ✅ 是 | `sanitizeRichHtml()` = DOMPurify + processHtmlForDisplay | **PASS** |
| 7 | `components/reseller/ResellerSiteConfigPanel.vue` | 252 | `announcementPreviewHtml` | ✅ 是 | 组件内 computed: `DOMPurify.sanitize(processHtmlForDisplay(...))` | **PASS** |
| 8 | `views/OrderDetail.vue` | 211 | `block.html` | ✅ 是 | instructionBlocks() → sanitizeInstructionsHtml() → DOMPurify.sanitize() | **PASS** |
| 9 | `views/OrderDetail.vue` | 270 | `block.html` | ✅ 是 | 同上 | **PASS** |
| 10 | `templates/vault/ProductDetail.vue` | 195 | `sanitizeRichHtml(getLocalizedText(product.content))` | ✅ 是 | `sanitizeRichHtml()` = DOMPurify + processHtmlForDisplay | **PASS** |
| 11 | `templates/vault/Legal.vue` | 12 | `content` | ✅ 是 | useLegal() 内 DOMPurify.sanitize(raw) | **PASS** |
| 12 | `templates/vault/BlogDetail.vue` | 36 | `sanitizeRichHtml(getLocalizedText(post.content))` | ✅ 是 | `sanitizeRichHtml()` = DOMPurify + processHtmlForDisplay | **PASS** |
| 13 | `templates/vault/components/VaultOrderFulfillment.vue` | 47 | `block.html` | ✅ 是 | instructionBlocks() → sanitizeInstructionsHtml() → DOMPurify.sanitize() | **PASS** |

**汇总**: PASS = 13 处，FAIL = 0 处。

---

## 二、6 个审计声称 sink 逐一确认

### Sink 1: Recharge 商品详情描述（ProductDetail / templates/vault/ProductDetail）

- **文件**: `views/ProductDetail.vue:328`、`templates/vault/ProductDetail.vue:195`
- **修复方式**: ✅ **已修复 — DOMPurify 包裹**
- **代码证据**: 两处均为 `v-html="sanitizeRichHtml(getLocalizedText(product.content))"`；`sanitizeRichHtml()` 定义于 `utils/content.ts:27-30`，内部执行 `DOMPurify.sanitize(processHtmlForDisplay(html))`。
- **判定**: **PASS**

### Sink 2: Order 详情商品描述（OrderDetail / GuestOrderDetail / VaultOrderFulfillment block.html）

- **文件**: `views/OrderDetail.vue:211,270`、`views/GuestOrderDetail.vue:200,259`、`templates/vault/components/VaultOrderFulfillment.vue:47`
- **修复方式**: ✅ **已修复 — DOMPurify 包裹**
- **代码证据**: block.html 来自 `useOrderDisplayHelpers.ts` 的 `instructionBlocks()` → `sanitizeInstructionsHtml()` → `DOMPurify.sanitize(raw, { ALLOWED_TAGS: [...], FORBID_ATTR: ['style','class','id'], ... })`。
- **判定**: **PASS**

### Sink 3: Notification 内容渲染

- **文件**: `views/Notifications.vue:60`
- **修复方式**: ✅ **已修复 — 改用纯文本插值**
- **代码证据**: `<p v-if="item.body" class="...">{{ item.body }}</p>` — 纯文本插值，无 v-html。
- **判定**: **PASS**

### Sink 4: Ticket 消息 body 渲染

- **文件**: `components/support/TicketConversation.vue:7,32`
- **修复方式**: ✅ **已修复 — 改用纯文本插值**
- **代码证据**: 两处均为 `{{ msg.body }}` 纯文本插值，无 v-html。
- **判定**: **PASS**

### Sink 5: C2C 挂单 terms 渲染

- **文件**: `views/c2c/ListingDetail.vue:75`、`views/c2c/BuyUSDT.vue:98`
- **修复方式**: ✅ **已修复 — 改用纯文本插值**
- **代码证据**: ListingDetail: `<p class="...">{{ listing.terms }}</p>`；BuyUSDT: `{{ listing.terms || '-' }}`。均为纯文本插值。
- **判定**: **PASS**

### Sink 6: Site Builder discovery block 文本渲染

- **文件**: `components/discovery/BlockRenderer.vue`、`views/Discovery.vue`、`utils/customScripts.ts`
- **修复方式**: ✅ **已修复 — 纯文本渲染 + 脚本注入已停用**
- **代码证据**: BlockRenderer.vue 和 Discovery.vue 中均无 v-html；`customScripts.ts` 已废弃为 no-op。
- **判定**: **PASS**

---

## 三、外部 hcz_user 对比差异（同步前 vs 同步后）

### 同步前差异（已消除）

以下 6 处修复在外部 hcz_user 中存在但在 hcz_v1 缺失，**现已全部同步到 hcz_v1**：

| 文件 | 同步前 hcz_v1 状态 | 同步后 hcz_v1 状态 |
|------|------------------|------------------|
| `utils/content.ts` | 无 sanitizeRichHtml | ✅ 已新增 sanitizeRichHtml() |
| `composables/useLegal.ts` | content 未 sanitize | ✅ content 经过 DOMPurify.sanitize |
| `views/ProductDetail.vue` | processHtmlForDisplay（无 sanitize） | ✅ sanitizeRichHtml |
| `views/BlogDetail.vue` | processHtmlForDisplay（无 sanitize） | ✅ sanitizeRichHtml |
| `templates/vault/ProductDetail.vue` | processHtmlForDisplay（无 sanitize） | ✅ sanitizeRichHtml |
| `templates/vault/BlogDetail.vue` | processHtmlForDisplay（无 sanitize） | ✅ sanitizeRichHtml |

### 同步后状态

hcz_v1 与 hcz_user 在 DOMPurify 安全修复层面已对齐。

---

## 四、vue-tsc 类型检查结果

```
命令: npx vue-tsc -b
目录: E:\Users\orang\Downloads\Compressed\hcz_v1\frontend\user
退出码: 0
结果: ✅ 通过（同步后复验，无类型错误）
```

---

## 五、Tests 结果

```
命令: npm test（node --experimental-strip-types --test tests/*.test.ts）
目录: E:\Users\orang\Downloads\Compressed\hcz_v1\frontend\user
退出码: 0
结果: ✅ 全部通过（同步后复验）
  - tests: 104
  - pass: 104
  - fail: 0
  - duration_ms: 1970.54
```

---

## 六、构建结果（同步后重建）

```
构建命令: npm run build（vue-tsc -b && vite build）
构建开始时间: 2026-10-05 21:06:36
构建结束时间: 2026-10-05 21:07:14
构建耗时: ~38.2 秒（vite build 自身 18.09s）
构建是否成功: ✅ 成功（exit code 0）
dist 目录: E:\Users\orang\Downloads\Compressed\hcz_v1\frontend\user\dist
dist 总大小: 2.87 MB（3,013,188 bytes 级别）
旧 dist: 已删除后重建（非复用）
```

---

## 七、dist 中 DOMPurify 代码验证证据（同步后复验）

### 独立 DOMPurify chunk

- **`purify.es-IRQXsms6.js`**（28.98 KB / gzip 11.06 KB）— DOMPurify 运行时独立 chunk，包含 `"dompurify"` 标识字符串。

### 引用 purify chunk 的业务 chunk（同步后新增引用）

以下 chunk 均引用 `purify.es-IRQXsms6.js`，证明 DOMPurify 被真实打包并参与运行时：

| chunk 文件 | 说明 |
|-----------|------|
| `purify.es-IRQXsms6.js` | DOMPurify 本体 |
| `useOrderDisplayHelpers-*.js` | 订单 instructions sanitize（原有） |
| `ResellerSiteConfig-*.js` | 分销商公告预览 sanitize（原有） |
| `HomeExperience-*.js` / `Home-*.js` ×2 | 首页体验组件（原有） |
| `OrderDetail-*.js` ×2 / `GuestOrderDetail-*.js` | 订单详情（原有） |
| `VaultOrderBody/VaultOrderFulfillment/VaultOrderItem-*.js` | vault 订单组件（原有） |
| `ResellerRichText-*.js` | 分销商富文本编辑器（原有） |
| **`content-*.js`** | **utils/content.ts 的 sanitizeRichHtml（同步后新增）** |
| **`useLegal-*.js`** | **useLegal composable 的 DOMPurify sanitize（同步后新增）** |
| **`Legal-*.js` ×2** | **views/Legal + vault/Legal 页面（同步后新增引用）** |
| **`ProductDetail-*.js` ×2** | **views/ProductDetail + vault/ProductDetail 页面（同步后新增引用）** |
| **`BlogDetail-*.js` ×2** | **views/BlogDetail + vault/BlogDetail 页面（同步后新增引用）** |

**结论**: DOMPurify 运行时已确认打包进生产 dist，且 ProductDetail、BlogDetail、Legal 相关 chunk 在同步后新增了对 purify chunk 的引用，证明 sanitizeRichHtml 和 useLegal sanitize 逻辑真实进入了生产产物。

---

## 八、最终判定

### ✅ **PASS**

**判定依据**: 生产源码 `hcz_v1/frontend/user/src` 中全部 **13 处 v-html** 均经过 DOMPurify sanitization（或等价纯文本插值），无任何 raw untrusted HTML 直接进入 v-html。

### 同步修复总结

- 从外部 hcz_user 同步了 6 个文件的 DOMPurify 安全修复
- 经逐文件 diff 确认：差异仅为 DOMPurify 相关代码，无其他业务逻辑变更
- 同步后重新执行：vue-tsc ✅ 通过、104 tests ✅ 全过、生产 dist ✅ 从源码重建成功
- dist 中 DOMPurify chunk 被所有相关业务 chunk（ProductDetail、BlogDetail、Legal、OrderDetail、AnnouncementModal、ResellerSiteConfig 等）正确引用
