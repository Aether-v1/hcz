# HCZ 用户前端深度审计报告

- 审计范围：`frontend/user`（用户端 SPA）
- 审计日期：2026-10-08
- 审计方式：全量阅读路由 / API / Store / Composables / Wallet / C2C / 2FA / 上传相关源码（只读审计，未修改任何文件）

---

## 1. 前端技术栈确认

| 项 | 实际情况 | 证据 |
|---|---|---|
| 框架 | Vue 3.5（`<script setup>` + Composition API，`legacy:false`） | `package.json` → `vue: ^3.5.24` |
| 构建 | Vite 7 + `@vitejs/plugin-vue` 6 | `vite.config.ts` |
| 语言 | TypeScript 5.9（`vue-tsc -b` 构建前类型检查） | `package.json` scripts.build |
| 样式 | **Tailwind CSS v4**（PostCSS 插件模式 `@tailwindcss/postcss`，非 v3 配置文件驱动） | `package.json` → `tailwindcss: ^4.1.18`，`postcss.config.js` |
| UI 体系 | **shadcn-vue（new-york 风格）+ reka-ui 2.9**（不是 radix-vue），`cva` + `clsx` + `tailwind-merge` | `components.json`（`style: new-york, baseColor: slate`），`package.json` → `reka-ui` |
| 状态管理 | Pinia 3（setup-store 写法） | `src/stores/*.ts` |
| 路由 | Vue Router 4（`createWebHistory`，全部路由懒加载） | `src/router/index.ts` |
| 国际化 | vue-i18n 9.14（`legacy:false`），构建期预编译 locale JSON | `src/i18n/index.ts`，`vite.config.ts` → `VueI18nPlugin` |
| HTTP 层 | **原生 fetch 封装，不是 axios** | `src/api/client.ts` |
| 其他 | `@unhead/vue`（SEO head）、`qrcode`（2FA 二维码）、`lucide-vue-next`（图标）、`@vueuse/core`、`@tiptap/vue-3`（仅分销商富文本用）、DOMPurify | `package.json` |

> 注意：依赖中**没有 axios**。所有请求走 `src/api/client.ts` 自封装的 fetch 客户端，新增 API 时必须沿用该模式，不要引入 axios。

---

## 2. 路由结构（`src/router/index.ts`）

### 2.1 钱包相关路由（全部 `requiresUserAuth: true`）

| 路径 | name | 组件 | 说明 |
|---|---|---|---|
| `/me/wallet` | `personal-center-wallet` | `views/Wallet.vue` → 实际渲染 `views/personal/WalletPanel.vue` | 钱包主页（余额卡 + 充值/提现入口） |
| `/me/wallet/withdrawal` | `wallet-withdrawal` | `views/WalletAction.vue` | 提现表单页 |
| `/me/wallet/withdrawal-history` | `wallet-withdrawal-history` | `views/WalletAction.vue` | 提现记录页（同一组件按 `route.name` 切换子组件） |
| `/me/wallet/transactions` | `wallet-transactions` | `views/personal/WalletTransactions.vue` | 资金流水 |
| `/me/recharge-orders` | `recharge-orders` | `views/personal/RechargeOrders.vue` | 充值订单列表 |
| `/recharge-orders/:recharge_no` | `recharge-order-detail` | `views/RechargeOrderDetail.vue` | 单笔充值订单详情（轮询支付状态） |

`WalletAction.vue` 是一个壳：`route.name === 'wallet-withdrawal-history'` 时渲染 `WalletWithdrawalHistory`，否则渲染 `WalletWithdrawal`。

### 2.2 C2C 相关路由（全部 `requiresUserAuth: true`）

| 路径 | name | 组件 |
|---|---|---|
| `/c2c` | `c2c-home` | `views/c2c/C2CHome.vue`（钱包卡 + 入口卡片 + 最近交易） |
| `/c2c/buy` | `c2c-buy` | `views/c2c/BuyUSDT.vue` |
| `/c2c/sell` | `c2c-sell` | `views/c2c/SellUSDT.vue` |
| `/c2c/my-listings` | `c2c-my-listings` | `views/c2c/MyListings.vue` |
| `/c2c/listings/:id` | `c2c-listing-detail` | `views/c2c/ListingDetail.vue` |
| `/c2c/trades` | `c2c-my-trades` | `views/c2c/MyTrades.vue` |
| `/c2c/trades/:id` | `c2c-trade-detail` | `views/c2c/TradeDetail.vue` |
| `/c2c/payment-methods` | `c2c-payment-methods` | `views/c2c/PaymentMethods.vue` |

### 2.3 路由守卫要点（`router/index.ts` L465-505）

- `requiresUserAuth`：未登录 → 重定向 `/auth/login?redirect=<原路径>`。
- `resellerConsole` meta：额外校验 `appStore.canAccessResellerConsole`，不通过 → `/me/orders`。
- 每次导航前确保 `appStore.config` 已加载；`afterEach` 里应用 SEO + Telegram MiniApp 返回键同步。
- 路由级预热：首屏空闲时按队列预加载 products/cart/checkout/payment 等 chunk（`warmupCommonRoutes`）。

---

## 3. Wallet 页面完整结构

### 3.1 组件层级

```
/me/wallet
└── views/Wallet.vue                      (薄壳，仅 <WalletPanel />)
    └── views/personal/WalletPanel.vue   (钱包主页，589 行)
        ├── components/wallet/WalletBalanceCard.vue   (三栏余额：total/available/frozen)
        ├── components/wallet/WalletRechargeForm.vue  (充值表单：金额+快捷金额+渠道+备注)
        └── views/personal/WalletWithdrawal.vue      (提现表单，同时被钱包页底部抽屉复用)
```

另有独立组件：`components/wallet/WalletTransactionList.vue`、`views/personal/WalletTransactions.vue`、`views/personal/WalletWithdrawalHistory.vue`。

### 3.2 充值入口位置（`WalletPanel.vue`）

- 入口：页面中「余额充值」按钮（L30-39，Plus 图标），点击 `openRechargeSheet()`。
- 形态：**底部抽屉（Bottom Sheet）**，`<Teleport to="body">` + 自实现拖拽关闭手势（touch/mouse 下拉 >120px 关闭，L376-432），不是 Dialog。
- 抽屉内容：`WalletRechargeForm` —— 金额输入 + 快捷金额 chips + 支付渠道 `Select` + 备注。
- 渠道加载：金额输入后 300ms 防抖调用 `walletAPI.getPaymentChannels(amount)`（POST `/api/v1/wallet/payment-channels`），前端再按 `hide_amount_out_range`、`appStore.config.wallet_recharge_channel_ids`、epay 渠道白名单（wechat/wxpay/alipay/qqpay）过滤（L179-243）。
- 提交：`walletAPI.recharge()` → POST `/api/v1/wallet/recharge` → 成功后 `router.push('/recharge-orders/:recharge_no')`。
- 支付网关回调带 `?recharge_no=` 回到 `/me/wallet` 时，`redirectRechargeReturn()` 自动跳到充值详情页（L505-513）。

### 3.3 提现入口位置

- 入口 1：钱包页「申请提现」按钮（L40-49，ArrowUpRight 图标）→ 打开同款底部抽屉，内嵌 `WalletWithdrawal`。
- 入口 2：独立路由 `/me/wallet/withdrawal`（`WalletAction.vue`）。
- `WalletWithdrawal.vue` 现状：
  - 已保存地址下拉（`walletAPI.listWithdrawalAddresses`）+ 手动地址输入 + 「保存地址」复选框；
  - 金额输入 → 400ms 防抖 `walletAPI.quoteWithdrawal({network:'TRC20', amount})` 预览手续费/到账；
  - **TOTP 6 位输入框直接内联在表单里**（L84-95），`canSubmit` 要求 `totpCode.length === 6`；
  - 提交：先可选保存地址，再 `walletAPI.createWithdrawal(payload, crypto.randomUUID())`，**强制带 `Idempotency-Key` 头**；
  - 网络字段**硬编码 `'TRC20'`**（L210、L251、L261），无网络选择 UI。

### 3.4 钱包 API（`src/api/wallet.ts`）

```
POST   /wallet/payment-channels        按金额查可用充值渠道
GET    /wallet                        钱包账户（available/frozen/total/currency）
GET    /wallet/transactions           流水
POST   /wallet/recharge               创建充值
GET    /wallet/recharges              充值订单列表
GET    /wallet/recharges/stats
GET    /wallet/recharges/:no
POST   /wallet/recharge/payments/:id/capture
POST   /wallet/withdrawals            创建提现（Header: Idempotency-Key）
POST   /wallet/withdrawals/quote       提现报价
GET    /wallet/withdrawals             提现列表
GET    /wallet/withdrawals/:id
POST   /wallet/withdrawals/:id/cancel  取消提现（body: totp_code）
GET/POST/DELETE /wallet/withdrawal-addresses   提现地址簿 CRUD
POST   /wallet/withdrawal-addresses/:id/default
```

类型定义在 `src/api/types.ts` L501-551（`Withdrawal` / `WithdrawalAddress` / `WithdrawalQuote` / `CreateWithdrawalPayload`）。

---

## 4. C2C 页面与收款方式机制

### 4.1 收款方式（PaymentMethods.vue）

- 路由 `/c2c/payment-methods`，独立的收款方式管理页。
- 类型枚举：`bank | alipay | wechat | paynow | custom`（`src/api/c2c.ts` L5-7）。
- 字段：`type`、`account_name`、`account_identifier`（后端已脱敏返回）、`qr_image`（**纯 URL 文本输入框**，L108，无上传组件）、`instructions`、`enabled`。
- 增删改 + 启用/停用 Switch 乐观更新；删除走 `useConfirmDialog()`。
- API：`/c2c/payment-methods` 全套 CRUD + `POST /:id/enabled`。

### 4.2 交易创建流程中的收款方式选择

- **买家侧（BuyUSDT.vue）**：挂单列表 → 点「购买」弹出买币面板（输入法币/USDT 金额联动换算）→ `tradeActions.createTrade(listingId, usdtAmount)` → POST `/c2c/trades`（自动带 `Idempotency-Key`）→ 跳转 `/c2c/trades/:id`。
  - **买家创建交易时不选择收款方式**——收款方式是**卖家维度**的，创建交易时后端从卖家启用的收款方式生成快照（`C2CTrade.payment_method_snapshot`，`api/c2c.ts` L104）。
- **卖家侧（SellUSDT.vue）**：若 `c2cStore.hasEnabledPaymentMethod === false`，显示琥珀色提示条 + 按钮跳 `/c2c/payment-methods`（L27-33）；否则直接创建挂单。挂单本身不绑定具体收款方式，买家下单时后端自动取卖家已启用的方式。
- 风控错误映射在 `composables/useC2C.ts` L206-229：`error.c2c_totp_required` → toast「请先启用谷歌验证器（TOTP）后再使用 C2C 交易」；`error.c2c_no_payment_method` → 提示先添加收款方式。
- **关键缺口**：`createTrade` payload 只有 `{listing_id, usdt_amount}`（`api/c2c.ts` L110-113、L208-209），**不带 totp_code**。即后端目前只校验「是否已启用 2FA」，不要求下单时输入 TOTP；前端也没有 TOTP 二次确认弹窗。

---

## 5. API 层封装模式（`src/api/client.ts`）

### 5.1 客户端实例

- 两个实例：`api`（不注入 Token，公开接口）与 `userApi`（注入 `Bearer <localStorage.user_token>`）。
- Base：`import.meta.env.VITE_API_BASE_URL` + `/api/v1`；超时 10s（AbortController）。
- 请求头：自动带 `X-Lang: <当前locale>`；非 FormData body 自动 `Content-Type: application/json`。
- Token 从 `localStorage.getItem('user_token')` 读取，**未使用 Pinia 持久化插件**。

### 5.2 响应与错误处理

- 统一响应壳：`{ status_code, msg, data, pagination? }`。
- 业务错误判定：`status_code !== 0` → reject（Error，message 为后端 msg）；可通过 `silentBusinessError` 选项静默。
- 401（HTTP 或业务码）→ 清 `user_token` / `user_profile`，`window.location.href = '/auth/login'`（可被 devPreview 策略抑制）。
- Blob 下载：`{ blob: true }` 选项。
- 幂等：业务层自行在 options.headers 里加 `Idempotency-Key`（提现、C2C 下单、积分兑换均如此）。

### 5.3 现有 API 模块清单（`src/api/`）

`auth.ts`（登录/注册/TOTP 状态与绑定/验证码/公开 config）、`wallet.ts`、`c2c.ts`、`order.ts`、`product.ts`、`user.ts`、`points.ts`、`support.ts`、`reseller.ts`、`credential.ts`、`notification.ts`、`invitation.ts`、`affiliate.ts`、`types.ts`、`index.ts`（统一 re-export）。

---

## 6. 现有 2FA（TOTP）能力盘点

| 场景 | 位置 | 形态 |
|---|---|---|
| 绑定/解绑/重生成恢复码 | `components/security/TwoFactorSection.vue` | 个人中心安全设置页内嵌卡片；setup 返回 `secret + otpauth_url`，前端用 `qrcode` 库本地生成二维码 dataURL；启用后返回新 token 直接替换 localStorage（旧 token 失效处理已做） |
| 登录二次验证 | `composables/useLogin.ts` + `views/auth/Login.vue` | 登录返回 `challenge_token` 后切到 `step='totp'`，支持验证码/恢复码两种模式 + challenge 过期倒计时 |
| 提现 | `views/personal/WalletWithdrawal.vue` | 表单内联 6 位 TOTP 输入框（非弹窗） |
| 取消提现 | `walletAPI.cancelWithdrawal(id, totpCode)` | 调 API 时传 totp_code（UI 层入口在 `WalletWithdrawalHistory.vue`） |
| C2C | 仅错误提示 | 后端返回 `error.c2c_totp_required` 时 toast 引导去启用；**无 TOTP 输入弹窗** |

**结论：不存在可复用的「敏感操作 2FA 验证弹窗」组件**（如 `TotpChallengeDialog.vue`）。现有 2FA 都是各自为政的内联实现。`userTotpAPI`（`api/auth.ts` L25-32）只覆盖「管理自己的 2FA」，业务接口需要 TOTP 时由各表单自己收集 6 位码。

---

## 7. 现有文件上传能力

| 项 | 现状 |
|---|---|
| 上传组件 | 仅 `components/support/AttachmentUploader.vue`：拖拽/多选、扩展名白名单（jpg/jpeg/png/webp/pdf/txt/log）、单文件 ≤10MB、进度条为模拟动画（fetch 无上传进度事件） |
| 上传 API | 仅 `supportAPI.uploadAttachment(file)` → `POST /api/v1/support/attachments`（multipart，字段名 `file`），返回 `{id, file_name, mime_type, file_size}`；附件 id 通过 `v-model: attachment_ids: number[]` 挂到工单/回复上 |
| 下载 | `supportAPI.downloadAttachment(id)` → GET blob |
| C2C 收款二维码 | `PaymentMethods.vue` L108：`qr_image` 是**普通 URL 文本框**，无上传 |
| C2C 申诉证据 | `C2CDisputePayload.evidence?: string`（`api/c2c.ts` L122）——**纯文本字段，无证据图片上传** |
| 分销商图片 | `components/reseller/ResellerImageField.vue`（站点配置用，另一套上传链路，与用户端业务无关） |

**结论：没有通用上传 API / 通用上传组件**。要在 C2C（收款二维码、申诉截图）等场景加图片上传，需要新增上传端点复用或扩展 `AttachmentUploader`。

---

## 8. 可用 UI 组件清单（`src/components/ui/`，shadcn-vue primitives）

已安装：`alert`、`badge`、`button`、`card`、`checkbox`、`input`、`label`、`popover`、`select`、`switch`、`table`、`textarea`、`tooltip`。

**未安装（shadcn-vue registry 里有但本项目没有）**：`dialog`、`alert-dialog`、`sheet`、`tabs`、`dropdown-menu`、`radio-group`、`separator`、`skeleton`、`scroll-area`、`form`（项目自封装 `components/FormField.vue`）、`toast`（项目自封装 `components/Toast.vue` + `composables/useToast.ts`）。

业务级通用组件：`ConfirmDialog.vue`（自封装确认弹窗，走 `useConfirmDialog`）、`EmptyState.vue`、`Loading.vue`、`PaginationNav.vue`、`BreadcrumbNav.vue`、`ErrorBoundary.vue`、`FormField.vue`、`PageAlert` 工具（`utils/alerts.ts`）。

> 重要：项目里所有弹层（钱包抽屉、C2C 买币面板、收款方式编辑、2FA 恢复码展示）都是**手写 `<Teleport to="body">` + fixed overlay**，没有统一 Dialog/Sheet primitive。新增弹窗时建议要么补装 shadcn `dialog`/`sheet`，要么沿用现有手写模式保持一致。

---

## 9. 主题系统

- **Tailwind v4 `@theme inline`**（`src/style.css`）：语义色全部映射到 CSS 变量 `--ui-*`（`--ui-accent`、`--ui-bg-page`、`--ui-text-primary`、`--ui-danger`…），同时兼容 shadcn 命名（`--color-background/foreground/primary/muted/border/destructive…`）。
- **暗色模式**：`tailwind.config.js` → `darkMode: 'class'`，`style.css` L5 `@custom-variant dark`；`.dark` 类下翻转 `--ui-*` 变量。
- **品牌色运行时注入**：`composables/useSiteConfig.ts` L35 把 `/public/config` 返回的 `brand.primary_color` 写成 `--brand-primary`（+ 10% 透明衍生色 `--brand-primary-light`），默认值 indigo-600（`style.css` L105-106）。
- 钱包卡背景图：`--wallet-card-light.webp`（CSS 变量 `--wallet-card-bg`，暗色有对应覆盖）。
- 新页面/组件必须使用语义 token（`bg-card`、`text-foreground`、`border-border`、`bg-primary/10` 等），不要写死色值。

---

## 10. i18n 使用方式

- `src/i18n/index.ts`：vue-i18n 9，`legacy:false`；**zh-CN 静态打包**，zh-TW / en-US 路由级懒加载 chunk；`detectLocale()` 按 localStorage → 浏览器语言 → zh-CN 兜底。
- 构建期：`@intlify/unplugin-vue-i18n` 把 locale JSON 预编译为 AST（`dropMessageCompiler: true`），运行时靠 `__INTLIFY_JIT_COMPILATION__` 解释。
- API 联动：每个请求自动带 `X-Lang` 头（`client.ts` L90-93）。
- 用法：组件内 `const { t } = useI18n()`；非组件文件用 `import { t } from '@/api/client'`（client.ts L7-8 导出的薄封装）。
- 现有命名空间：`nav.*`、`personalCenter.wallet.*`、`personalCenter.security.twofa.*`、`c2c.*`（zh-CN.json L2187 起）、`auth.login.totp.*`、`common.api.*` 等。
- **注意**：`composables/useC2C.ts` 里状态标签（`TRADE_STATUS_LABELS` 等）和错误映射是**硬编码中文**，没走 i18n（L16-56、L209-224）——新增文案时若需多语言应改走 locale 文件。

---

## 11. 与目标需求的差距清单（需要新建/补齐）

以下按「若要落地新的钱包/C2C 敏感操作能力」视角列出缺口：

### P0（硬性缺口）

1. **无可复用的 2FA 验证弹窗组件**
   - 现状：2FA 验证散落在登录页 step、提现表单内联输入框、安全设置页。
   - 需要：新建 `components/security/TotpChallengeDialog.vue`（或 shadcn `dialog` 封装），支持 code / recovery_code 双模式 + challenge 过期倒计时，可供提现复核、C2C 下单确认、地址管理确认等场景复用。
   - API 层：业务端如需「先操作、后端返回需要 TOTP 再补码」的两步式（类似登录 challenge_token），后端需先出协议；前端目前无此通用机制。

2. **无通用图片上传能力**
   - 现状：仅工单附件上传（`POST /support/attachments`），C2C `qr_image` 是 URL 文本框、申诉 `evidence` 是纯文本。
   - 需要：后端新增通用上传端点（或复用附件端点并放开类型）；前端把 `AttachmentUploader.vue` 泛化（抽 props：accept/maxSize/endpoint/v-model 返回 url 或 id），供 C2C 收款二维码、申诉证据截图使用。

3. **提现网络硬编码 TRC20**
   - `WalletWithdrawal.vue` L210/L251/L261 三处 `network: 'TRC20'` 写死，无网络选择器。若后端支持多链（TRC20/ERC20 等），需加网络 Select 并透传 quote/address 校验。

### P1（体验/一致性缺口）

4. **无统一 Dialog/Sheet primitive**：所有弹层手写 Teleport。建议补装 shadcn-vue `dialog` + `sheet`，或至少抽出 `components/AppSheet.vue` 收敛现有钱包抽屉模式。
5. **C2C 交易创建无 TOTP 二次确认**：`createTrade` 不带 totp_code，后端只校验 2FA 是否启用。若产品要求「下单时输入 TOTP」，需前端补 TOTP 收集 + 后端协议改造。
6. **i18n 硬编码**：`useC2C.ts` 状态/错误文案硬编码中文，需迁移到 `c2c.*` locale 命名空间。
7. **金额/手续费计算**：前端 `utils/money.ts` 已有 `amountToCents / centsToAmount / calculateFeeCents / formatUsdt`，新页面必须复用，禁止自己 `parseFloat` 算钱。

### P2（可复用资产，无需新建）

- 幂等键：`crypto.randomUUID()`（提现已用），新写操作照抄 `walletAPI.createWithdrawal` 的 `Idempotency-Key` header 模式。
- 轮询：`composables/usePolling.ts`、C2C 交易轮询 `useTradePolling`（12s 间隔 + 页面可见性暂停）。
- 确认弹窗：`useConfirmDialog()` + `ConfirmDialog.vue`。
- Toast：`composables/useToast.ts`。
- 错误提示：`PageAlert`（`utils/alerts.ts`）+ `Alert` UI。
- 路由守卫、登录态、config 加载链路已完备，新页面只需挂 `meta: { requiresUserAuth: true }`。

---

## 12. 关键文件索引速查

| 关注点 | 文件 |
|---|---|
| 路由总表 | `src/router/index.ts` |
| HTTP 客户端 | `src/api/client.ts` |
| 钱包 API | `src/api/wallet.ts`；类型 `src/api/types.ts` L93-147, L501-551 |
| C2C API + 类型 | `src/api/c2c.ts` |
| TOTP 管理 API | `src/api/auth.ts` L25-32 |
| 上传 API | `src/api/support.ts` L34-38 |
| 钱包主页 | `src/views/personal/WalletPanel.vue` |
| 提现表单 | `src/views/personal/WalletWithdrawal.vue` |
| 充值表单组件 | `src/components/wallet/WalletRechargeForm.vue` |
| 余额卡 | `src/components/wallet/WalletBalanceCard.vue` |
| C2C 主页/买/卖/收款方式 | `src/views/c2c/{C2CHome,BuyUSDT,SellUSDT,PaymentMethods}.vue` |
| C2C 状态/错误/轮询 | `src/composables/useC2C.ts` |
| C2C Store | `src/stores/c2c.ts` |
| 2FA 设置 | `src/components/security/TwoFactorSection.vue` |
| 登录 2FA 流程 | `src/composables/useLogin.ts` L54-62, L232-265 |
| 上传组件 | `src/components/support/AttachmentUploader.vue` |
| 主题 | `src/style.css`（@theme inline）；品牌色注入 `src/composables/useSiteConfig.ts` L35 |
| i18n | `src/i18n/index.ts`；文案 `src/i18n/locales/zh-CN.json` |
| shadcn 配置 | `components.json`；Tailwind `tailwind.config.js`；构建 `vite.config.ts` |
