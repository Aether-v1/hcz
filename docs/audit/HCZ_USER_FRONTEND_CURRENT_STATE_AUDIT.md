# HCZ User Frontend — Current State Audit

> 审计日期：2026-10-03
> 审计范围：`frontend/user/`（用户前端）
> 审计方式：只读源码扫描 + TypeCheck + Build + Tests
> 约束：未修改任何业务代码、依赖、配置

---

## 1. Final Verdict

**NEEDS PARTIAL REBUILD**

当前用户前端工程质量高于平均水平（TypeScript 全量通过、62 项单元测试全绿、生产构建成功、设计 token 体系完整），核心交易链路（商品→购物车→结算→支付→订单）功能完整且可运行。

但存在以下结构性问题，无法通过纯样式重写解决：

1. **CNY/USDT 双币种模型缺失**（P0）：前端将钱包余额与订单金额（CNY）直接比较和扣减，无汇率快照、无 USDT 展示、前端自行计算扣款金额，与 HCZ 正式业务规则冲突。
2. **消息通知系统完全缺失**（P0）：无 Notification Store、无 WebSocket/SSE、无未读计数、无消息中心、无客服消息通道。
3. **底部导航与目标 IA 不一致**（P1）：当前为 Home/Products/Cart/Me，目标为 Home/Orders/Discovery/Me。
4. **双模板系统（classic + vault）造成全量页面重复**（P1）：28 个 vault 模板文件与 classic 视图并行维护，增加一倍维护成本。
5. **无 Local First / 离线缓存 / 自动重试机制**（P1）：每次进入页面重新 Skeleton，错误即清空数据，无网络状态监听。

**结论**：保留现有工程基础设施（Vite/Pinia/TS/Tailwind token 体系/reka-ui 组件库/API 层/i18n），对页面层进行**部分重构**——先建立统一 App Shell + Design System，再逐页重构首页/订单/发现/我的，同时补齐 CNY/USDT 模型和消息通知系统。不建议推倒重来。

---

## 2. Current Stack

| 维度 | 技术选型 | 版本 | 备注 |
|------|----------|------|------|
| 框架 | Vue | ^3.5.24 | Composition API + `<script setup>` |
| 语言 | TypeScript | ~5.9.3 | 全量 TS，vue-tsc 构建 |
| 构建工具 | Vite | ^7.2.4 | 含 route warmup、manualChunks |
| 包管理器 | pnpm | 10.34.3 | 但同时存在 package-lock.json（混合） |
| 状态管理 | Pinia | ^3.0.4 | 6 个 store |
| 路由 | Vue Router | ^4.6.4 | History 模式，路由级懒加载 |
| CSS | Tailwind CSS | ^4.1.18 | 自定义 CSS 变量 token 体系 |
| UI 组件库 | reka-ui | ^2.9.10 | 无头组件（Radix Vue 等价），shadcn-vue 风格封装 |
| 图标 | lucide-vue-next | ^0.563.0 | |
| 样式工具 | class-variance-authority + clsx + tailwind-merge | latest | shadcn 风格组件变体 |
| 国际化 | vue-i18n | ^9.14.5 | 3 语言（zh-CN/zh-TW/en-US），JIT 编译 |
| SEO | @unhead/vue | ^2.1.4 | 响应式 head 管理 |
| 工具库 | @vueuse/core | ^14.3.0 | |
| 富文本 | Tiptap | ^2.27.2 | 仅分销商后台用 |
| HTTP | 原生 fetch（自封装） | — | 无 Axios，`/api/v1` 前缀 |
| 二维码 | qrcode | ^1.5.4 | |
| HTML 净化 | dompurify | ^3.4.0 | |
| 字体 | @fontsource/rubik + @fontsource/nunito-sans | latest | 自托管，vault 模板专用 |
| 测试 | node --experimental-strip-types | — | 62 项测试，无 Vitest/Jest |
| Lint | **未配置** | — | 无 ESLint/Prettier |
| 环境变量 | **无 .env 文件** | — | `VITE_API_BASE_URL` 可选，默认空（走 Vite proxy） |

### API 请求层细节

- 基础路径：`/api/v1`（硬编码在 `api/client.ts`）
- BaseURL：`import.meta.env.VITE_API_BASE_URL || ''`
- 双客户端：`api`（无鉴权）+ `userApi`（Bearer Token，从 `localStorage.user_token` 读取）
- 超时：10s（AbortController）
- 401 处理：清除 token + `window.location.href = '/auth/login'`
- 业务错误：`status_code !== 0` 即 reject
- 无请求/响应缓存、无重试、无并发去重

---

## 3. Current Page Map

### 3.1 认证 (Auth)

| 页面 | 路由 | 状态 | 建议 |
|------|------|------|------|
| 登录 | `/auth/login` | COMPLETE | RESTYLE |
| 注册 | `/auth/register` | COMPLETE | RESTYLE |
| 忘记密码 | `/auth/forgot` | COMPLETE | RESTYLE |
| Telegram 回调 | `/auth/telegram/callback` | COMPLETE | KEEP |
| Google 回调 | `/auth/google/callback`（动态路径） | COMPLETE | KEEP |
| 2FA 验证 | 内嵌于 Login 流程 | COMPLETE | KEEP |

### 3.2 首页 (Home)

| 页面 | 路由 | 状态 | 建议 |
|------|------|------|------|
| 首页（Card 模式） | `/` | COMPLETE | PARTIAL REBUILD |
| 首页（List 模式） | `/` | COMPLETE | PARTIAL REBUILD |

### 3.3 充值业务 (Recharge / Products)

| 页面 | 路由 | 状态 | 建议 |
|------|------|------|------|
| 商品列表 | `/products` | COMPLETE | RESTYLE |
| 分类商品 | `/categories/:slug` | COMPLETE | RESTYLE |
| 商品详情 | `/products/:slug` | COMPLETE | RESTYLE |
| 购物车 | `/cart` | COMPLETE | RESTYLE |
| 结算 | `/checkout` | COMPLETE | REBUILD（CNY/USDT） |
| 支付 | `/pay` | COMPLETE | REBUILD（CNY/USDT） |
| 钱包充值 | `/me/wallet`（内嵌） | COMPLETE | REBUILD（USDT 模型） |

### 3.4 订单 (Orders)

| 页面 | 路由 | 状态 | 建议 |
|------|------|------|------|
| 订单列表 | `/me/orders`（内嵌 PersonalCenter） | COMPLETE | REBUILD（状态映射+IA） |
| 订单详情 | `/orders/:order_no` | COMPLETE | RESTYLE |
| 充值订单详情 | `/recharge-orders/:recharge_no` | COMPLETE | RESTYLE |
| 游客订单列表 | `/guest/orders` | COMPLETE | KEEP |
| 游客订单详情 | `/guest/orders/:order_no` | COMPLETE | KEEP |

### 3.5 USDT / C2C

| 页面 | 路由 | 状态 | 建议 |
|------|------|------|------|
| USDT 交易 | — | **MISSING** | 需新建 |
| C2C 交易 | — | **MISSING** | 需新建 |
| 钱包（USDT） | `/me/wallet` | PARTIAL（当前按 CNY 展示） | REBUILD |

### 3.6 发现 (Discovery)

| 页面 | 路由 | 状态 | 建议 |
|------|------|------|------|
| 博客列表 | `/blog` | COMPLETE | 可复用为发现页内容源 |
| 博客详情 | `/blog/:slug` | COMPLETE | KEEP |
| 公告列表 | `/notice` | COMPLETE | 可复用 |
| 关于页 | `/about` | COMPLETE | KEEP |
| 发现首页 | — | **MISSING** | 需新建（Schema Driven） |

### 3.7 我的 (Profile / Personal Center)

| 模块 | 路由 | 状态 | 建议 |
|------|------|------|------|
| 概览 | `/me` | COMPLETE | RESTYLE |
| 个人资料 | `/me/profile` | COMPLETE | RESTYLE |
| 安全中心 | `/me/security` | COMPLETE（含 2FA/登录设备/Telegram/Google 绑定） | KEEP |
| 订单 | `/me/orders` | COMPLETE | REBUILD |
| 钱包 | `/me/wallet` | PARTIAL | REBUILD（USDT） |
| 礼品卡 | `/me/gift-cards` | COMPLETE | KEEP |
| API 凭证 | `/me/api` | COMPLETE | KEEP |
| 邀请好友（Affiliate） | `/me/affiliate` | COMPLETE | RESTYLE |
| 分销商控制台 | `/reseller/*` | COMPLETE（11 个子页面） | KEEP |
| 消息通知 | — | **MISSING** | 需新建 |
| 帮助中心 | — | **MISSING**（仅有 About/Legal） | 需新建 |
| 在线客服 | — | **MISSING** | 需新建 |
| 设置 | 内嵌于安全中心 | PARTIAL | 可拆分 |

### 3.8 消息 (Notifications)

| 模块 | 状态 | 建议 |
|------|------|------|
| 系统通知 | **MISSING** | 需新建 |
| 订单通知 | **MISSING** | 需新建 |
| 客服消息 | **MISSING** | 需新建 |
| 未读红点 | **MISSING** | 需新建 |
| WebSocket/SSE | **MISSING** | 需新建 |
| Push 通知 | **MISSING** | 需新建 |

### 3.9 其他

| 页面 | 路由 | 状态 | 建议 |
|------|------|------|------|
| 服务条款 | `/terms` | COMPLETE | KEEP |
| 隐私政策 | `/privacy` | COMPLETE | KEEP |
| 404 | `/:pathMatch(.*)*` | COMPLETE | KEEP |

---

## 4. Route Map

### 路由统计

- **总路由数**：38 条（含 reseller 子路由 11 条）
- **需要登录的路由**：26 条（`requiresUserAuth: true`）
- **游客路由**：3 条（`userGuest: true`）
- **公开路由**：9 条（terms/privacy/callback/404 等）

### 路由结构

```
/                          home (Card/List 双模式)
/products                  products
/categories/:slug          category-products (复用 Home/Products)
/products/:slug            product-detail
/cart                      cart
/checkout                  checkout
/pay                       payment
/me                        personal-center (overview)
/me/profile                personal-center (profile)
/me/security               personal-center (security)
/me/orders                 personal-center (orders)
/me/wallet                 personal-center (wallet)
/me/gift-cards             personal-center (giftCard)
/me/api                    personal-center (api)
/me/affiliate              personal-center (affiliate)
/me/reseller               → redirect /reseller
/reseller                  reseller console (layout + 11 children)
  /reseller/apply
  /reseller/domains
  /reseller/site
  /reseller/products
  /reseller/orders
  /reseller/orders/:order_no
  /reseller/finance
  /reseller/ledger
  /reseller/withdraws
/orders/:order_no          order-detail
/recharge-orders/:recharge_no  recharge-order-detail
/blog                      blog
/blog/:slug                blog-detail
/notice                    notice
/about                     about
/terms                     legal (terms)
/privacy                   legal (privacy)
/auth/login                user-login
/auth/register             user-register
/auth/forgot               user-forgot
/auth/telegram/callback    user-telegram-callback
/auth/google/callback      user-google-callback (动态路径常量)
/*                         not-found
```

### 路由守卫

- `beforeEach`：加载全局 config → 检查 `requiresUserAuth` → 检查 `resellerConsole` 权限 → 捕获 affiliate 参数
- `afterEach`：应用 SEO + Telegram MiniApp 返回按钮同步
- 路由预热（warmup）：页面加载后空闲时预加载常用路由 chunk

---

## 5. API Matrix

### 5.1 统计

- **API 模块数**：10 个文件
- **API 端点总数**：约 55 个
- **Legacy API**：**0 个**（全部使用 `/api/v1/*`）
- **Mock 数据**：**0 处**
- **写死接口路径**：无（全部通过 API 模块封装）

### 5.2 端点清单

| 模块 | Method | Endpoint | 用途 | 新/旧 |
|------|--------|----------|------|-------|
| **auth** | POST | `/api/v1/auth/send-verify-code` | 发送验证码 | 新 |
| | POST | `/api/v1/auth/register` | 注册 | 新 |
| | POST | `/api/v1/auth/login` | 登录 | 新 |
| | POST | `/api/v1/auth/login/verify-2fa` | 2FA 验证 | 新 |
| | POST | `/api/v1/auth/telegram/login` | Telegram 登录 | 新 |
| | POST | `/api/v1/auth/telegram/miniapp/login` | Telegram MiniApp 登录 | 新 |
| | GET | `/api/v1/auth/telegram/oidc/start` | Telegram OIDC 开始 | 新 |
| | POST | `/api/v1/auth/telegram/oidc/callback` | Telegram OIDC 回调 | 新 |
| | POST | `/api/v1/auth/google/login` | Google 登录 | 新 |
| | POST | `/api/v1/auth/google/redirect-intent` | Google 重定向意图 | 新 |
| | POST | `/api/v1/auth/google/redirect-exchange` | Google 重定向交换 | 新 |
| | POST | `/api/v1/auth/forgot-password` | 忘记密码 | 新 |
| | GET | `/api/v1/public/captcha/image` | 图形验证码 | 新 |
| | GET | `/api/v1/public/config` | 全局站点配置 | 新 |
| **user** | GET | `/api/v1/me` | 当前用户信息 | 新 |
| | GET | `/api/v1/me/login-logs` | 登录日志 | 新 |
| | PUT | `/api/v1/me/profile` | 更新资料 | 新 |
| | POST | `/api/v1/me/email/send-verify-code` | 改邮箱验证码 | 新 |
| | POST | `/api/v1/me/email/change` | 改邮箱 | 新 |
| | PUT | `/api/v1/me/password` | 改密码 | 新 |
| | GET/POST/DELETE | `/api/v1/me/telegram/*` | Telegram 绑定管理 | 新 |
| | GET/POST/DELETE | `/api/v1/me/google/*` | Google 绑定管理 | 新 |
| | GET | `/api/v1/me/2fa/status` | 2FA 状态 | 新 |
| | POST | `/api/v1/me/2fa/setup` | 2FA 设置 | 新 |
| | POST | `/api/v1/me/2fa/enable` | 2FA 启用 | 新 |
| | POST | `/api/v1/me/2fa/disable` | 2FA 禁用 | 新 |
| | POST | `/api/v1/me/2fa/recovery-codes/regenerate` | 恢复码重生成 | 新 |
| **product** | GET | `/api/v1/public/products` | 商品列表 | 新 |
| | GET | `/api/v1/public/products/:slug` | 商品详情 | 新 |
| | GET | `/api/v1/public/posts` | 文章列表（博客/公告） | 新 |
| | GET | `/api/v1/public/posts/:slug` | 文章详情 | 新 |
| | GET | `/api/v1/public/banners` | Banner 列表 | 新 |
| | GET | `/api/v1/public/categories` | 分类列表 | 新 |
| | GET | `/api/v1/public/member-levels` | 会员等级 | 新 |
| **order** | POST | `/api/v1/orders/preview` | 订单预览 | 新 |
| | POST | `/api/v1/orders` | 创建订单 | 新 |
| | POST | `/api/v1/orders/create-and-pay` | 创建并支付 | 新 |
| | GET | `/api/v1/orders` | 订单列表 | 新 |
| | GET | `/api/v1/orders/stats` | 订单统计 | 新 |
| | GET | `/api/v1/orders/:order_no` | 订单详情 | 新 |
| | POST | `/api/v1/orders/:order_no/cancel` | 取消订单 | 新 |
| | GET | `/api/v1/orders/:order_no/fulfillment/download` | 下载发货内容 | 新 |
| | POST | `/api/v1/order/payment-channels` | 获取支付渠道 | 新 |
| | POST | `/api/v1/payments` | 创建支付 | 新 |
| | POST | `/api/v1/payments/:id/capture` | 捕获支付 | 新 |
| | GET | `/api/v1/payments/latest` | 最新支付 | 新 |
| | POST/GET | `/api/v1/guest/orders/*` | 游客订单（6 端点） | 新 |
| | POST | `/api/v1/guest/payments` | 游客创建支付 | 新 |
| | POST | `/api/v1/guest/payments/:id/capture` | 游客捕获支付 | 新 |
| | GET | `/api/v1/guest/payments/latest` | 游客最新支付 | 新 |
| **wallet** | GET | `/api/v1/wallet` | 钱包账户 | 新 |
| | GET | `/api/v1/wallet/transactions` | 钱包交易记录 | 新 |
| | POST | `/api/v1/wallet/recharge` | 钱包充值 | 新 |
| | GET | `/api/v1/wallet/recharges` | 充值订单列表 | 新 |
| | GET | `/api/v1/wallet/recharges/stats` | 充值统计 | 新 |
| | GET | `/api/v1/wallet/recharges/:recharge_no` | 充值详情 | 新 |
| | POST | `/api/v1/wallet/recharge/payments/:id/capture` | 捕获充值支付 | 新 |
| | POST | `/api/v1/wallet/payment-channels` | 钱包充值渠道 | 新 |
| | POST | `/api/v1/gift-cards/redeem` | 礼品卡兑换 | 新 |
| **affiliate** | POST | `/api/v1/public/affiliate/click` | 联盟点击追踪 | 新 |
| | POST | `/api/v1/affiliate/open` | 开通联盟 | 新 |
| | GET | `/api/v1/affiliate/dashboard` | 联盟概览 | 新 |
| | GET | `/api/v1/affiliate/commissions` | 佣金列表 | 新 |
| | GET | `/api/v1/affiliate/withdraws` | 提现列表 | 新 |
| | POST | `/api/v1/affiliate/withdraws` | 申请提现 | 新 |
| **reseller** | 15 端点 | `/api/v1/reseller/*` | 分销商管理 | 新 |
| **credential** | 4 端点 | `/api/v1/api-credential/*` | API 凭证 | 新 |

### 5.3 API 接入评价

- **与 HCZ V1 目标一致性**：✅ 全部端点使用 `/api/v1/*`，无 Legacy fallback、无新旧双调用、无旧 Controller 接口。
- **鉴权模型**：✅ JWT Bearer Token，公开端点（public/*）无需鉴权但商品读接口已移入 JWT 保护组。
- **缺失端点**：通知/消息相关 API 完全缺失（无 `/api/v1/notifications`、`/api/v1/messages`、`/api/v1/notifications/unread-count`）。
- **问题**：`/api/v1/order/payment-channels` 路径不一致（单数 `order` vs 复数 `orders`），建议统一。

---

## 6. UI/UX Findings

### 6.1 视觉系统

| 维度 | 现状 | 评价 | KEEP/RESTYLE/REBUILD/REMOVE |
|------|------|------|------|
| 色彩系统 | CSS 变量 token，Apple 风格（#0071e3 主色），完整亮/暗双模式 | 设计良好，但主色为蓝色，与目标"不要大量蓝色"冲突 | RESTYLE（换主色） |
| 字体系统 | Inter/system-ui 兜底；vault 模板用 Rubik + Nunito Sans；价格用等宽字体 | 基础可用，但无明确字号层级规范 | RESTYLE |
| 字号层级 | h1-h4 有 letter-spacing 和 line-height 定义；价格用 `theme-price-lg/sm` | 部分规范，但页面中大量直接用 Tailwind 文字类 | RESTYLE |
| 圆角 | 4 级 token（sm 8px / md 12px / lg 16px / xl 24px），通过 CSS 变量映射 Tailwind | 设计良好 | KEEP |
| 阴影 | 3 级 + soft/card，暗色模式自动调整 | 设计良好 | KEEP |
| Border | hairline 系统（rgba 透明度），亮/暗自动翻转 | 设计良好 | KEEP |
| Icon | lucide-vue-next，统一线性图标 | 良好 | KEEP |
| 按钮 | shadcn 风格 Button 组件（CVA 变体），含 default/secondary/outline/ghost/destructive | 良好 | KEEP |
| 输入框 | shadcn 风格 Input/Textarea/Select | 良好 | KEEP |
| Card | shadcn 风格 Card 组件 | 良好，但页面存在大量直接用 `bg-card border rounded-2xl` 的内联卡片 | RESTYLE |
| Modal | ConfirmDialog 组件 + AnnouncementModal；无通用 Modal 组件 | 部分缺失 | RESTYLE |
| Toast | 全局 Toast 组件（useToast composable） | 可用 | KEEP |
| Empty State | EmptyState 组件（多 variant） | 良好 | KEEP |
| Skeleton | `theme-skeleton` CSS 类 + 内联骨架 | 可用但分散 | RESTYLE |
| Loading | 全局 Loading 组件（appStore.loading）+ 页面级骨架 | 可用 | KEEP |

### 6.2 布局

| 维度 | 现状 | 评价 | 判定 |
|------|------|------|------|
| Header | classic 模板：Navbar.vue（顶部导航栏）；vault 模板：VaultLayout.vue 内嵌 Header | 两套实现，classic 偏桌面导航，vault 偏 App 化 | REBUILD（统一 Shell） |
| Bottom Tab | MobileBottomNav.vue，4 Tab（Home/Products/Cart/Me），`lg:hidden`，safe-area 支持 | 功能完整但 IA 与目标不符 | REBUILD |
| 页面边距 | `container mx-auto px-4`，最大宽度无显式约束（默认 Tailwind container） | 可用但偏宽 | RESTYLE |
| Safe Area | `theme-safe-bottom` 工具类（env(safe-area-inset-bottom)），BottomNav 已应用 | 良好 | KEEP |
| 手机宽度适配 | 响应式断点（sm/md/lg），移动端单列 | 可用 | RESTYLE |
| 横向溢出 | 未发现明显横向溢出 | 良好 | KEEP |
| 组件密度 | 偏桌面端密度，移动端卡片间距较大 | 可优化 | RESTYLE |
| 卡片堆叠 | 页面存在大量卡片堆叠（PersonalCenter 尤其明显） | 与目标"不要页面堆满卡片"冲突 | REBUILD |
| 视觉层级 | 主色强调 + 灰阶层级，基本清晰 | 可用 | RESTYLE |

### 6.3 交互

| 维度 | 现状 | 评价 | 判定 |
|------|------|------|------|
| 页面切换 | `page-fade` 过渡（opacity 200ms） | 过于简单，App 化需要更丰富的转场 | RESTYLE |
| 返回 | 浏览器原生返回 + Telegram MiniApp BackButton 同步 | 可用 | KEEP |
| Loading | 全局 loading + 页面骨架 | 可用但每次进入页面都重新骨架 | REBUILD（Local First） |
| 错误处理 | API 错误 reject → 页面 catch → Toast/Alert；无统一错误边界（ErrorBoundary 组件存在但仅包裹 RouterView） | 部分可用 | RESTYLE |
| 空状态 | EmptyState 组件统一 | 良好 | KEEP |
| 下拉刷新 | **未实现** | 缺失 | REBUILD |
| 自动刷新 | 支付页 5s 轮询；其他页面无自动刷新 | 仅支付场景 | REBUILD |
| 表单校验 | useFormValidation composable + 手动校验；Checkout 有完整手动表单校验 | 可用但分散 | RESTYLE |
| 重复提交 | 提交按钮 `submitting` 状态禁用；支付有 debounce | 良好 | KEEP |
| Toast 提示 | 全局 Toast，支持多类型 | 良好 | KEEP |
| Dialog 确认 | ConfirmDialog 全局组件 | 良好 | KEEP |

### 6.4 KEEP / RESTYLE / REBUILD / REMOVE 总矩阵

| 类别 | KEEP | RESTYLE | REBUILD | REMOVE |
|------|------|---------|---------|--------|
| 色彩 token 体系 | ✅ 圆角/阴影/Border token | 主色换色 | | |
| 字体 | | ✅ 建立字号层级 | | |
| 基础组件（Button/Input/Card/Badge/Alert/Select/Switch/Checkbox/Table/Tooltip/Popover/Label/Textarea） | ✅ 全部保留 | | | |
| Icon (lucide) | ✅ | | | |
| Toast/ConfirmDialog/EmptyState | ✅ | | | |
| Skeleton | | ✅ 统一封装 | | |
| Header/Navbar | | | ✅ 统一 App Shell | classic Navbar 桌面导航模式 |
| Bottom Tab | | | ✅ 重写 IA | |
| 页面布局 | | ✅ 减少卡片堆叠 | | |
| 页面转场 | | ✅ App 化转场 | | |
| Loading 策略 | | | ✅ Local First + SWR | |
| 下拉刷新 | | | ✅ 新增 | |
| 双模板系统 | vault 模板的 App 化思路 | | ✅ 合并为单一模板 | classic 模板的桌面导航模式 |
| 全局 CSS 覆盖层（style.css 中的 utility override） | | ✅ 清理冗余 | | 部分 Tailwind 颜色强制覆盖（如 `.bg-gray-900 → --ui-accent`） |

---

## 7. Home Findings

### 7.1 当前模块

**Card 模式（默认）：**
1. Hero Banner 轮播（API 驱动：`/api/v1/public/banners`）
2. 推荐商品网格（API 驱动：`/api/v1/public/products?page=1&page_size=15`，**写死 15 条**）
3. 最新文章/公告（API 驱动：`/api/v1/public/posts`，博客+公告合并展示）

**List 模式：**
1. Hero Banner 轮播
2. 左侧分类侧栏 + 右侧商品列表（含搜索、分页）

### 7.2 专项检查

| 检查项 | 结果 |
|--------|------|
| 四宫格业务入口 | **不存在**。当前无核心业务快捷入口区域 |
| 四宫格是否写死 | N/A（不存在） |
| 四宫格是否 API 驱动 | N/A |
| Banner 是否 API 驱动 | ✅ 是（`/api/v1/public/banners`，useBannerCarousel composable） |
| 公告是否 API 驱动 | ✅ 是（`appStore.config.announcement` 站点配置 + `/api/v1/public/posts?type=notice`） |
| 首页布局是否可由管理后台控制 | ⚠️ 部分。`template_mode`（card/list）由后端 config 控制；nav_config 控制博客/公告显隐；但模块顺序、模块内容、四宫格均不可配置 |
| 是否存在大量重复代码 | ⚠️ 是。Card 模式和 List 模式的 Banner 轮播代码完全重复（两份几乎相同的 template）；vault 模板又有一份 Home.vue |
| 推荐商品数量 | 写死 `page_size: 15` |

### 7.3 结论

**PARTIAL REBUILD**

保留 Banner 轮播机制和 API 驱动模式，重构首页布局：
- 新增顶部用户/钱包区域（USDT 资产展示）
- 新增四宫格核心业务入口（API/Site Config 驱动）
- 保留 Banner 区域
- 推荐业务改为可配置模块
- 消除 Card/List 双模式代码重复（统一为组件化渲染）
- 消除 vault 模板重复

---

## 8. Order Findings

### 8.1 Frontend Order Contract Matrix

| 前端状态枚举 | 后端对应 | 文案来源 | 目标状态映射 | 一致性 |
|-------------|----------|----------|-------------|--------|
| `pending_payment` | pending_payment | `order.status.pending_payment` | 待充值 | ✅ |
| `paid` | paid | `order.status.paid` | 处理中 | ✅（可合并） |
| `fulfilling` | fulfilling | `order.status.fulfilling` | 处理中 | ✅（可合并） |
| `partially_delivered` | partially_delivered | `order.status.partially_delivered` | 处理中 | ✅（可合并） |
| `partially_refunded` | partially_refunded | `order.status.partially_refunded` | 已完成（部分退款） | ⚠️ 需定义 |
| `delivered` | delivered | `order.status.delivered` | 已完成 | ✅（可合并） |
| `completed` | completed | `order.status.completed` | 已完成 | ✅（可合并） |
| `expired` | expired | `order.status.expired` | 已取消/失败 | ⚠️ 需定义 |
| `canceled` | canceled | `order.status.canceled` | 已取消 | ✅ |
| `refunded` | refunded | `order.status.refunded` | 已完成（已退款） | ⚠️ 需定义 |

### 8.2 订单操作

| 操作 | 前端实现 | API | 状态 |
|------|----------|-----|------|
| 订单详情 | ✅ OrderDetail.vue | GET `/orders/:order_no` | 可用 |
| 取消订单 | ✅ OrderDetail 内 | POST `/orders/:order_no/cancel` | 可用 |
| 未收到 | ❌ **未实现** | — | 缺失 |
| 售后 | ❌ **未实现** | — | 缺失 |
| 下载发货内容 | ✅ | GET `/orders/:order_no/fulfillment/download` | 可用 |
| 立即支付 | ✅ | 跳转 `/pay?order_no=` | 可用 |

### 8.3 订单页结构

- 订单列表内嵌于 PersonalCenter（`/me/orders`），非独立页面
- 双 Tab：商品订单 + 充值订单
- 商品订单有 10 种状态筛选 + 关键词搜索 + 分页
- 统计卡片：总数/当前页/待支付/已完成（前端从 stats API 聚合）
- 充值订单有 4 种状态（pending/success/failed/expired）

### 8.4 问题

1. **状态粒度过细**：10 种状态对用户不友好，目标 5 种（待充值/处理中/已完成/失败/已取消）需要前端做映射聚合。
2. **"未收到"和"售后"操作缺失**：HCZ 业务需要。
3. **订单列表非独立页面**：内嵌于 PersonalCenter，与目标 Bottom Tab "订单" 独立 Tab 不符。
4. **`expired` 状态归类模糊**：是失败还是已取消？需明确定义。
5. **统计卡片"已完成"聚合逻辑**：前端自行聚合 delivered+completed+partially_refunded+refunded，应以后端 stats 为准。

---

## 9. Recharge / CNY-USDT Findings

### 9.1 HCZ 正式业务规则对照

| 规则 | 前端现状 | 合规性 |
|------|----------|--------|
| 商品展示价格：CNY | ✅ `siteCurrency \|\| 'CNY'`，商品价格按 CNY 展示 | 合规 |
| 用户钱包资产：USDT | ❌ 钱包余额按 `appStore.config.currency \|\| 'CNY'` 展示，**未区分 USDT** | **违规** |
| 扣款：USDT | ❌ 前端 `expectedWalletPaidCents` 直接用 `Math.min(balance, total)` 比较，**假设钱包与订单同币种** | **违规** |
| 汇率由后端计算 | ❌ 前端无汇率概念，无汇率快照展示 | **违规** |
| 前端不能自行决定最终扣款金额 | ⚠️ 前端计算 `expectedWalletPaidCents`/`expectedOnlinePayCents` 用于展示和渠道筛选，**实际扣款由后端 createAndPay 处理**，但前端展示金额可能误导 | 部分违规 |
| 订单必须使用后端返回的价格/汇率快照 | ✅ 订单预览（`/orders/preview`）返回 total_amount，前端使用 preview 数据 | 合规 |

### 9.2 具体问题清单

#### P0 — 钱包币种模型错误

1. **`WalletPanel.vue:274`**：`balanceDisplay = formatMoney(wallet.value?.balance, String(appStore.config?.currency || 'CNY'))`
   - 问题：如果钱包资产是 USDT，这里会显示为 "XXX CNY"，币种错误。
   - 应从 `wallet.value.coin_type` 或 `wallet.value.currency` 读取实际币种。

2. **`useCheckout.ts:128-134`**：`expectedWalletPaidCents = Math.min(balance, total)`
   - 问题：直接将钱包余额（USDT）与订单总额（CNY）做数值比较和扣减计算，无汇率转换。
   - 影响：`walletOnlyPayment` 模式下，如果用户有 10 USDT 但订单是 100 CNY，前端会认为余额不足（10 < 100），但实际 10 USDT ≈ 72 CNY 可能仍不足——但判断逻辑本身就是错的。
   - 更严重：如果用户有 100 USDT，订单 100 CNY，前端会认为刚好够，但实际 100 USDT ≈ 720 CNY，远超订单金额——前端展示的"钱包扣除 100"是错误的。

3. **`usePayment.ts:424-430`**：同样的 `expectedWalletPaidCents` 逻辑。

4. **`usePayment.ts:423`**：`walletBalanceDisplay = formatMoney(walletBalance.value, order.value?.currency)`
   - 问题：支付页钱包余额用订单币种（CNY）展示，而非钱包实际币种。

#### P1 — 汇率展示缺失

5. **无"预计扣除 ≈ X USDT"展示**：商品详情、购物车、结算、支付页均无汇率换算展示。
6. **无汇率快照**：订单确认时未展示后端返回的汇率快照。
7. **`PaymentAmountBreakdown.vue`**：金额明细全部用 `order.currency`（CNY），`wallet_paid_amount` 也用 CNY 展示——如果后端返回的是 USDT 扣款金额，这里币种标注错误。

#### P2 — 精度处理

8. **`utils/money.ts`**：`amountToCents` 使用 2 位小数精度（分），对于 USDT（通常 6-8 位小数）不够。
9. **前端金额计算**：`useCheckout.ts` 中 `totalAmount` 用 cents 计算，对 CNY 合理，但如果涉及 USDT 则精度不足。
10. **无 float 精度问题**：前端统一用整数 cents 计算，避免了 float 精度问题，这是好的。

### 9.3 结论

**需要在重构结算/支付/钱包模块时，建立完整的 CNY/USDT 双币种模型：**
- 钱包余额从后端读取 `coin_type`/`currency` 字段，按实际币种展示
- 结算页展示"¥XXX（CNY）" + "预计扣除 ≈ Y.YY USDT（汇率由后端返回）"
- 前端不自行计算 USDT 扣款金额，仅展示后端 preview/payment 接口返回的 `wallet_paid_amount` + 对应币种
- `walletOnlyPayment` 判断逻辑改为基于后端返回的可用余额（USDT）和汇率
- USDT 金额精度需支持 6-8 位小数

---

## 10. Discovery Findings

### 10.1 现状

- **无独立"发现"页面**。当前最接近的是 Blog（博客）+ Notice（公告），均为简单文章列表。
- Blog 页面：文章卡片网格（3 列），从 `/api/v1/public/posts?type=blog` 获取。
- Notice 页面：文章列表，从 `/api/v1/public/posts?type=notice` 获取。
- 首页有"最新文章"区域，合并展示博客+公告。

### 10.2 专项检查

| 检查项 | 结果 |
|--------|------|
| 是否已有发现页 | ❌ 无 |
| 模块是否写死 | N/A |
| Banner/Grid/Card/Activity 是否组件化 | ⚠️ Blog 用内联 article 卡片，无通用内容卡片组件 |
| 数据是否来自 API | ✅ 是（`/api/v1/public/posts`） |
| 是否适合 Schema Driven / Config Driven UI | ✅ 适合。后端已有 `appStore.config` 站点配置体系，可扩展为页面装修配置 |

### 10.3 结构建议（不实现）

未来发现页建议采用 **Config Driven UI** 架构：

1. **后端扩展**：Site Config 增加 `discovery_page` 配置，包含模块数组（`modules: [{type: 'banner', config: {...}}, {type: 'grid', config: {...}}, {type: 'post_list', config: {...}}, {type: 'activity', config: {...}}]`）
2. **前端模块渲染器**：建立 `DynamicModuleRenderer`，根据 module.type 渲染对应组件
3. **可复用模块组件**：
   - `DiscoveryBanner`（复用现有 Banner 轮播）
   - `DiscoveryGrid`（四宫格/六宫格入口）
   - `DiscoveryPostList`（复用 Blog 文章卡片）
   - `DiscoveryActivity`（活动卡片）
   - `DiscoveryHTML`（富文本/自定义 HTML 模块）
4. **管理后台**：增加发现页装修器（拖拽排序、模块配置）

---

## 11. Profile Findings

### 11.1 已有模块

| 模块 | 位置 | 状态 |
|------|------|------|
| 用户信息（头像/昵称/邮箱） | ProfilePanel | COMPLETE |
| USDT 钱包 | WalletPanel | PARTIAL（币种错误） |
| 账单（钱包交易记录） | WalletPanel 内嵌 | COMPLETE |
| USDT 交易入口 | ❌ 缺失 | MISSING |
| 邀请好友（Affiliate） | AffiliatePanel | COMPLETE |
| 消息通知 | ❌ 缺失 | MISSING |
| 帮助与支持 | ❌ 缺失（仅有 About） | MISSING |
| 在线客服 | ❌ 缺失 | MISSING |
| 安全中心 | SecurityPanel | COMPLETE（2FA/登录设备/改密/改邮箱/Telegram/Google 绑定） |
| 2FA | SecurityPanel 内嵌 | COMPLETE |
| 设置 | 内嵌于安全中心 | PARTIAL（无独立设置页） |
| 礼品卡 | GiftCardPanel | COMPLETE |
| API 凭证 | ApiPanel | COMPLETE |
| 分销商入口 | PersonalCenter + /reseller | COMPLETE |

### 11.2 问题

1. **PersonalCenter 是单页多 Section 模式**：通过 `props.section` 切换内容，非独立路由页面。这导致 URL 虽然不同（`/me/profile`、`/me/security` 等），但实际是同一个组件的 props 切换，页面切换无转场效果。
2. **侧边栏导航在移动端变为横向滚动 Tab**：可用但不 App 化。
3. **概览页卡片堆叠严重**：4 个 StatCard + 会员等级卡片 + 最近订单卡片，信息密度高。
4. **缺少消息/帮助/客服入口**：目标 IA 需要。

### 11.3 重复/Legacy

- 无明显重复组件。
- `userProfileStore`（13KB）承载了用户资料、会员等级、订单统计、最近订单等多个职责，可拆分。

---

## 12. Notification Findings

### 12.1 现状全面检查

| 检查项 | 结果 |
|--------|------|
| Notification Store | ❌ **MISSING** |
| unread_count | ❌ **MISSING** |
| Badge / 红点 | ❌ **MISSING**（仅购物车有 Badge） |
| WebSocket | ❌ **MISSING** |
| SSE (Server-Sent Events) | ❌ **MISSING** |
| 轮询 (Polling) | ⚠️ 仅支付页有 5s 轮询（支付状态），非通知系统 |
| Push 通知 | ❌ **MISSING** |
| 消息 API | ❌ **MISSING**（无 `/api/v1/notifications` 或 `/api/v1/messages`） |
| 客服消息 API | ❌ **MISSING** |
| 通知列表页 | ❌ **MISSING** |
| 客服会话页 | ❌ **MISSING** |

### 12.2 结论

**消息通知系统完全缺失，需要从零建设。**

建议架构：
1. **后端 API**：`/api/v1/notifications`（列表/已读/全部已读/未读计数）、`/api/v1/notifications/settings`
2. **实时通道**：SSE（`/api/v1/notifications/stream`）或 WebSocket，优先 SSE（更简单、自动重连）
3. **前端 Store**：`useNotificationStore`（Pinia），管理未读计数、通知列表、已读状态
4. **UI 组件**：Bottom Tab 红点、通知铃铛、通知列表页、通知详情
5. **客服消息**：可独立为 `/api/v1/conversations` + `/api/v1/messages`，支持 WebSocket 实时消息
6. **降级方案**：无 SSE 时降级为 30s 轮询未读计数

---

## 13. Network UX Findings

### 13.1 现状检查

| 检查项 | 结果 |
|--------|------|
| API 请求错误机制 | ⚠️ fetch 封装有基本错误处理（HTTP 状态码 + 业务 status_code），但错误即 reject，无统一错误恢复 |
| Axios interceptor | N/A（使用原生 fetch） |
| 网络状态监听 | ❌ **未实现**（无 `navigator.onLine` 监听） |
| Retry（重试） | ❌ **未实现**（无自动重试机制） |
| Cache（缓存） | ❌ **未实现**（无请求级缓存，无 SWR） |
| Store 持久化 | ⚠️ 仅 userAuth（token/profile）持久化到 localStorage；app config 不持久化；其他 store 不持久化 |
| stale-while-revalidate | ❌ **未实现** |
| 重连 (online 重试) | ❌ **未实现** |
| 页面 onMounted 是否每次清空 | ⚠️ 是。多数页面 onMounted 时 `loading = true` + 数据清空，然后重新请求。OrdersPanel catch 错误时 `orders.value = []` 会清空已有数据 |
| Skeleton 使用 | ⚠️ 每次进入页面都显示 Skeleton，而非仅首次加载。返回上一页时重新 Skeleton |
| 离线体验 | ❌ 无离线缓存，断网即白屏/错误 |

### 13.2 问题列表

1. **无 Local First 策略**：每次进入页面重新加载，无缓存数据先展示。
2. **错误即清空**：`OrdersPanel.loadOrders` catch 时 `orders.value = []`，网络抖动会导致已有订单消失。
3. **无自动重试**：网络瞬时错误不会自动重试，需要用户手动刷新。
4. **无网络状态监听**：断网无提示，恢复后无自动重试。
5. **Skeleton 滥用**：每次页面进入都 Skeleton，包括从详情页返回列表页时。
6. **无请求去重**：同一 API 可能被多次调用（如 watch 触发的 debounce 仍可能重复）。
7. **支付页轮询无退避**：固定 5s 轮询，无指数退避。

### 13.3 建议架构

1. **请求层增强**：在 `api/client.ts` 增加请求缓存（Map + TTL）、自动重试（指数退避，最多 3 次，仅对 GET 和幂等请求）、请求去重（in-flight promise 共享）。
2. **SWR Composable**：建立 `useSWR(key, fetcher)` composable，返回 `{ data, error, isLoading, isValidating, mutate }`，实现"先展示缓存 → 后台刷新 → 更新展示"。
3. **网络状态**：`useOnline()` composable（`@vueuse/core` 已有 `useOnline`），监听 online/offline，恢复时自动 mutate 所有 SWR key。
4. **Store 持久化**：app config、用户资料等可持久化到 localStorage（Pinia `persist` 插件或手动）。
5. **错误边界**：ErrorBoundary 组件增强，支持"保留上次数据 + 错误提示条"模式，而非清空数据。

---

## 14. Hardcoded Configuration Matrix

### 14.1 硬编码清单

| 配置项 | 位置 | 当前值 | 建议归属 |
|--------|------|--------|----------|
| GitHub 仓库链接 | `main.ts:20`, `Footer.vue:84`, `VaultLayout.vue:127` | `https://github.com/Aether-v1/hcz` | 前端常量（开源项目标识） |
| Google Identity Script | `utils/googleIdentity.ts:9` | `https://accounts.google.com/gsi/client` | 前端常量（第三方 SDK） |
| Telegram Widget Script | `SecurityPanel.vue:420`, `useLogin.ts:415` | `https://telegram.org/js/telegram-widget.js?22` | 前端常量 |
| Telegram WebApp Script | `utils/telegramMiniApp.ts:3` | `https://telegram.org/js/telegram-web-app.js?61` | 前端常量 |
| Cloudflare Turnstile Script | `TurnstileCaptcha.vue:49` | `https://challenges.cloudflare.com/turnstile/v0/api.js` | 前端常量 |
| Telegram Bot 链接模板 | `utils/telegramMiniApp.ts:213` | `https://telegram.me/${botUsername}/webapp` | 前端常量 |
| 默认品牌名 | `VaultLayout.vue:177` | `'D&J Studio'` | 后端 Site Config（已有 `brand.site_name`，仅 fallback） |
| 默认货币 | `WalletPanel.vue:257`, `useCheckout.ts:233` 等多处 | `'CNY'` | 后端 Site Config（已有 `config.currency`，仅 fallback） |
| API 前缀 | `api/client.ts:20` | `'/api/v1'` | 前端常量（HCZ 标准） |
| API BaseURL | `api/client.ts:19` | `import.meta.env.VITE_API_BASE_URL \|\| ''` | .env（已有 env 变量，无 .env 文件） |
| Vite Dev Proxy | `vite.config.ts:57` | `http://localhost:8080` | 前端常量（开发环境） |
| 首页推荐商品数量 | `Home.vue:481` | `page_size: 15` | 后端 Site Config 或前端常量 |
| 首页最新文章数量 | `Home.vue:491` | `page_size: 3` | 后端 Site Config 或前端常量 |
| 订单列表每页数量 | `OrdersPanel.vue:287` | `page_size: 20` | 前端常量 |
| 支付轮询间隔 | `usePayment.ts:701` | `5000ms` | 前端常量 |
| 请求超时 | `api/client.ts:62` | `10000ms` | 前端常量 |
| 语言列表 | `VaultLayout.vue:220`, `i18n/index.ts` | `['zh-CN', 'zh-TW', 'en-US']` | 前端常量 |
| 客服联系方式 | 无（分销商 Site Config 有 telegram/whatsapp/support_url 字段） | — | 后端 Site Config（已有字段，用户端未展示） |
| 下载地址 | 无 | — | 后端 Site Config |
| App 名称 | 后端 `config.brand.site_name` | — | 后端 Site Config（已有） |
| Logo | 后端 `config.brand.site_logo` / `site_icon` | — | 后端 Site Config（已有） |
| Theme Color | CSS 变量 `--ui-accent: #0071e3` | — | 前端常量（设计 token） |
| 汇率 | ❌ 前端无汇率概念 | — | 后端计算（HCZ 规则） |
| 商品分类 | API 驱动 `/api/v1/public/categories` | — | 后端管理（已有） |
| 首页模块 | ⚠️ 部分（template_mode/nav_config） | — | 后端 Site Config（需扩展） |
| Footer 链接 | 后端 `config.footer_links` | — | 后端 Site Config（已有） |

### 14.2 评价

- **后端 Site Config 体系完善**：brand/seo/nav_config/contact/footer_links/payment_channels/currency/template_mode 等均已由后端配置驱动。
- **无 .env 文件**：`VITE_API_BASE_URL` 未使用（默认空，走 Vite proxy），生产环境需通过环境变量或 Nginx 反代配置。
- **第三方 SDK URL 硬编码合理**：Google/Telegram/Cloudflare 的 SDK URL 是标准常量，无需配置化。
- **GitHub 链接**：3 处硬编码，建议提取为常量。

---

## 15. Dead / Legacy / Duplicate Code

### 15.1 双模板系统（最大重复源）

| 类别 | classic 模板 | vault 模板 | 判定 |
|------|-------------|-----------|------|
| 首页 | `views/Home.vue` (537 行) | `templates/vault/Home.vue` | **DUPLICATE** |
| 商品列表 | `views/Products.vue` | `templates/vault/Products.vue` | **DUPLICATE** |
| 商品详情 | `views/ProductDetail.vue` | `templates/vault/ProductDetail.vue` | **DUPLICATE** |
| 购物车 | `views/Cart.vue` | `templates/vault/Cart.vue` | **DUPLICATE** |
| 结算 | `views/Checkout.vue` | `templates/vault/Checkout.vue` | **DUPLICATE** |
| 支付 | `views/Payment.vue` | `templates/vault/Payment.vue` | **DUPLICATE** |
| 个人中心 | `views/PersonalCenter.vue` | `templates/vault/PersonalCenter.vue` | **DUPLICATE** |
| 订单详情 | `views/OrderDetail.vue` | `templates/vault/OrderDetail.vue` | **DUPLICATE** |
| 充值订单详情 | `views/RechargeOrderDetail.vue` | `templates/vault/RechargeOrderDetail.vue` | **DUPLICATE** |
| 博客 | `views/Blog.vue` | `templates/vault/Blog.vue` | **DUPLICATE** |
| 博客详情 | `views/BlogDetail.vue` | `templates/vault/BlogDetail.vue` | **DUPLICATE** |
| 公告 | `views/Notice.vue` | `templates/vault/Notice.vue` | **DUPLICATE** |
| 关于 | `views/About.vue` | `templates/vault/About.vue` | **DUPLICATE** |
| 法律 | `views/Legal.vue` | `templates/vault/Legal.vue` | **DUPLICATE** |
| 404 | `views/NotFound.vue` | `templates/vault/NotFound.vue` | **DUPLICATE** |
| 登录 | `views/auth/Login.vue` | `templates/vault/auth/Login.vue` | **DUPLICATE** |
| 注册 | `views/auth/Register.vue` | `templates/vault/auth/Register.vue` | **DUPLICATE** |
| 忘记密码 | `views/auth/Forgot.vue` | `templates/vault/auth/Forgot.vue` | **DUPLICATE** |
| 游客订单 | `views/GuestOrders.vue` | `templates/vault/GuestOrders.vue` | **DUPLICATE** |
| 游客订单详情 | `views/GuestOrderDetail.vue` | `templates/vault/GuestOrderDetail.vue` | **DUPLICATE** |
| Layout | `App.vue` + `Navbar.vue` + `Footer.vue` | `templates/vault/layout/VaultLayout.vue` | **DUPLICATE** |
| 组件 | `components/ProductCard.vue` 等 | `templates/vault/components/VaultProductCard.vue` 等 (10 个) | **DUPLICATE** |

**vault 模板总计**：28 个页面/组件文件 + 1 个 CSS + 1 个 Layout，约占用户前端代码量的 40%。

**判定**：vault 模板是更 App 化的设计方向，但与 classic 模板全量并行维护造成巨大重复。建议合并为单一模板，取 vault 的 App 化布局 + classic 的成熟交易逻辑。

### 15.2 其他重复/未引用

| 项目 | 位置 | 判定 |
|------|------|------|
| `buyNow.ts` store | `stores/buyNow.ts` (532 bytes) | KEEP（结算页 Buy Now 模式使用） |
| `telegramMiniApp.ts` store | `stores/telegramMiniApp.ts` (6587 bytes) | KEEP（Telegram MiniApp 支持） |
| `Cart.vue` 页面 | `views/Cart.vue` | KEEP（购物车功能完整） |
| 注释掉的大段代码 | 未发现明显大段注释代码 | — |
| `.bak` / `.old` / `.tmp` 文件 | 未发现 | — |
| `dist/` 目录 | 存在构建产物 | SAFE TO REMOVE（已在 .gitignore） |
| `node_modules/` | 存在 | KEEP（依赖） |
| `package-lock.json` + `pnpm-lock.yaml` | 同时存在 | REVIEW（应统一为 pnpm-lock.yaml，删除 package-lock.json） |
| 测试文件散落 | `tests/` 目录统一管理 15 个测试文件 | KEEP |
| 日志文件 | 未发现 | — |
| 临时截图 | 未发现 | — |

### 15.3 未引用组件检查（抽样）

- `components/ui/*`：全部通过 `index.ts` 导出，被页面引用。
- `components/shared/PanelHeading.vue`、`StatCard.vue`：被 PersonalCenter 系列引用。
- `components/wallet/*`：被 WalletPanel 引用。
- `components/checkout/*`：被 Checkout 引用。
- `components/payment/*`：被 Payment 引用。
- `components/security/*`：被 SecurityPanel 引用。
- `components/reseller/*`：被分销商页面引用。
- `components/reseller-console/*`：被分销商控制台引用。
- `BreadcrumbNav.vue`：需确认是否被引用（可能未使用）。
- `BackToTop.vue`：在 App.vue 中引用。

### 15.4 清理候选清单

| 项目 | 分类 | 说明 |
|------|------|------|
| `frontend/user/package-lock.json` | SAFE TO REMOVE | 项目使用 pnpm，package-lock.json 是误提交 |
| `frontend/user/dist/` | SAFE TO REMOVE | 构建产物，已在 .gitignore |
| vault 模板（28 文件） | REVIEW | 合并模板后可删除，但当前是活跃功能，需先完成模板合并 |
| `BreadcrumbNav.vue` | REVIEW | 需确认引用情况，可能未使用 |
| classic `Navbar.vue` 桌面导航模式 | REVIEW | 统一 App Shell 后可简化 |

---

## 16. Build / Lint / TypeCheck Result

### 16.1 执行结果

| 命令 | 结果 | 耗时 | 说明 |
|------|------|------|------|
| `vue-tsc -b --noEmit` (TypeCheck) | ✅ **PASS** | ~15s | 无类型错误 |
| `npm run build` (vue-tsc -b && vite build) | ✅ **PASS** | 17.20s | 2987 modules transformed，构建成功 |
| `npm test` (node --test) | ✅ **PASS** | 1.8s | 62 tests, 62 pass, 0 fail |
| ESLint | ⚠️ **NOT CONFIGURED** | — | 无 ESLint 配置和脚本 |
| Prettier | ⚠️ **NOT CONFIGURED** | — | 无 Prettier 配置 |

### 16.2 构建产物分析

- **主 JS chunk**：`index-CYcvqAkx.js` 325.09 KB (gzip 98.39 KB)
- **Tiptap 富文本**：`ResellerRichText-CjnU4BnO.js` 333.06 KB (gzip 106.17 KB) — 仅分销商后台用，已独立 chunk
- **vue-i18n**：`vendor-vue-i18n` 118.87 KB (gzip 45.37 KB)
- **usePersonalCenter**：107.69 KB (gzip 23.72 KB) — 偏大，因包含所有 PersonalCenter 子面板逻辑
- **语言包**：en-US 97.94 KB / zh-TW 95.52 KB / zh-CN（内联）
- **CSS**：主 CSS 164.30 KB (gzip 26.05 KB)
- **字体**：Rubik + Nunito Sans 各 5 字重，woff2 + woff 双格式

### 16.3 现有缺陷

1. **无 ESLint/Prettier**：代码风格无强制检查，可能存在不一致。
2. **`usePersonalCenter` chunk 偏大**（107 KB）：PersonalCenter 所有子面板逻辑打包在一起，未按 section 拆分。
3. **Tiptap 体积大**（333 KB）：但已独立 chunk，仅分销商后台加载，不影响用户端首屏。
4. **主 JS chunk 325 KB**（gzip 98 KB）：可接受，但有优化空间（如进一步拆分 composable）。
5. **无打包分析**：无 `rollup-plugin-visualizer` 等分析工具。

---

## 17. KEEP / RESTYLE / REBUILD / REMOVE Matrix

### 17.1 工程基础设施

| 项目 | 判定 | 理由 |
|------|------|------|
| Vue 3 + TS + Vite | KEEP | 现代、成熟、构建快 |
| Pinia | KEEP | 官方推荐，当前 6 个 store 结构清晰 |
| Vue Router | KEEP | 路由预热、守卫完善 |
| Tailwind CSS 4 + CSS 变量 token | KEEP | 设计 token 体系完整，亮/暗模式良好 |
| reka-ui + shadcn 风格组件 | KEEP | 无头组件灵活，基础组件齐全 |
| lucide-vue-next | KEEP | 图标统一、轻量 |
| vue-i18n | KEEP | 3 语言完整，JIT 编译优化 |
| @unhead/vue | KEEP | SEO 管理完善 |
| 原生 fetch 封装 | KEEP（增强） | 基础可用，需增加缓存/重试/SWR |
| API 层（/api/v1） | KEEP | 与 HCZ V1 目标一致，无 Legacy |
| 测试框架（node --test） | RESTYLE | 建议迁移到 Vitest（更好的 Vue 组件测试支持、watch 模式、覆盖率） |
| ESLint/Prettier | REBUILD（新增） | 当前未配置，需新增 |

### 17.2 页面层

| 页面/模块 | 判定 | 理由 |
|-----------|------|------|
| 登录/注册/忘记密码 | RESTYLE | 功能完整，需 App 化视觉重设计 |
| 首页 | PARTIAL REBUILD | 保留 Banner API 机制，重构布局（四宫格/钱包区域/模块化） |
| 商品列表/详情 | RESTYLE | 功能完整，视觉重设计 |
| 购物车 | RESTYLE | 功能完整，视觉重设计 |
| 结算 (Checkout) | REBUILD | CNY/USDT 模型需重建，逻辑可复用 composable |
| 支付 (Payment) | REBUILD | CNY/USDT 模型需重建，支付流程逻辑可复用 |
| 订单列表 | REBUILD | 独立页面（从 PersonalCenter 拆出），状态映射为 5 种，新增未收到/售后 |
| 订单详情 | RESTYLE | 功能完整，视觉重设计 |
| 钱包 (Wallet) | REBUILD | USDT 币种模型重建，充值流程保留 |
| 个人中心概览 | RESTYLE | 减少卡片堆叠，App 化 |
| 安全中心 | KEEP | 功能完整（2FA/登录设备/绑定），视觉微调 |
| 邀请好友 (Affiliate) | RESTYLE | 功能完整，视觉重设计 |
| 礼品卡 | KEEP | 功能完整 |
| API 凭证 | KEEP | 功能完整 |
| 分销商控制台 | KEEP | 功能完整，非用户端核心 |
| 博客/公告 | KEEP（复用） | 可作为发现页内容源 |
| 发现页 | REBUILD（新建） | 当前缺失，需 Schema Driven 新建 |
| 消息通知 | REBUILD（新建） | 当前完全缺失 |
| 帮助中心 | REBUILD（新建） | 当前缺失 |
| 在线客服 | REBUILD（新建） | 当前缺失 |
| USDT 交易/C2C | REBUILD（新建） | 当前缺失 |

### 17.3 组件/布局

| 项目 | 判定 | 理由 |
|------|------|------|
| 基础 UI 组件（Button/Input/Card 等 13 类） | KEEP | shadcn 风格，质量良好 |
| Toast / ConfirmDialog / EmptyState / Loading | KEEP | 全局组件完善 |
| MobileBottomNav | REBUILD | IA 不匹配（需 Home/Orders/Discovery/Me） |
| Navbar (classic) | REMOVE（替换） | 统一 App Shell 后替代 |
| VaultLayout | RESTYLE（合并） | App 化布局方向正确，合并为唯一 Shell |
| 双模板系统 (classic + vault) | REBUILD（合并） | 全量重复，合并为单一模板 |
| ProductCard / ProductListItem | RESTYLE | 视觉重设计 |
| WalletBalanceCard / WalletRechargeForm / WalletTransactionList | REBUILD | USDT 模型重建 |
| PaymentAmountBreakdown | REBUILD | CNY/USDT 双币种展示 |
| Skeleton | RESTYLE | 统一封装为组件 |
| ErrorBoundary | RESTYLE | 增强为"保留数据+错误提示"模式 |

---

## 18. Recommended New Information Architecture

### 18.1 底部导航（4 Tab）

```
┌─────────────────────────────────────┐
│  首页    订单    发现    我的        │
│  🏠     📋    ✨    👤              │
└─────────────────────────────────────┘
```

### 18.2 页面结构

```
/ (首页)
  ├── 顶部：用户问候 + USDT 钱包余额（快捷入口）
  ├── 四宫格核心业务入口（API 驱动）
  ├── Banner 轮播（API 驱动）
  ├── 公告滚动条（API 驱动）
  ├── 推荐业务（API 驱动，可配置）
  └── 运营活动模块（Config Driven）

/orders (订单)
  ├── 顶部：状态 Tab（待充值 / 处理中 / 已完成 / 失败 / 已取消）
  ├── 订单列表（卡片式，支持下拉刷新）
  └── 订单详情 → /orders/:order_no
       ├── 订单状态 + 金额明细（CNY + USDT 扣款）
       ├── 商品信息
       ├── 操作按钮：取消订单 / 未收到 / 售后 / 联系客服
       └── 发货内容下载

/discovery (发现)
  ├── Config Driven 模块渲染
  ├── Banner / Grid / 文章列表 / 活动卡片
  └── 文章详情 → /blog/:slug（复用）

/me (我的)
  ├── 顶部：用户信息 + USDT 钱包卡片
  ├── 功能网格：
  │   ├── 我的订单 → /orders
  │   ├── USDT 钱包 → /me/wallet
  │   ├── 账单 → /me/wallet#transactions
  │   ├── USDT 交易 → /me/usdt（新建）
  │   ├── 邀请好友 → /me/affiliate
  │   ├── 消息通知 → /me/notifications（新建）
  │   ├── 帮助中心 → /me/help（新建）
  │   ├── 在线客服 → /me/support（新建）
  │   ├── 安全中心 → /me/security
  │   └── 设置 → /me/settings（新建）
  └── 分销商入口（条件显示）

/auth/login, /auth/register, /auth/forgot（保留）
/pay（支付页，保留但重构 CNY/USDT）
/checkout（结算页，保留但重构 CNY/USDT）
/cart（购物车，保留）
/products, /products/:slug（保留）
/reseller/*（分销商控制台，保留）
```

### 18.3 统一 App Shell

```
AppShell
├── StatusBar（安全区域适配，PWA 风格）
├── Header（动态：首页显示钱包，列表页显示标题+搜索，详情页显示返回）
├── Main Content（页面内容，页面转场动画）
├── BottomTabNav（4 Tab，未读红点）
└── Global Overlays（Toast / ConfirmDialog / Loading / NetworkStatus）
```

---

## 19. Recommended UI Refactor Order

### Phase 0：基础设施（必须最先做）

1. **Design System 建立**
   - 定义新的色彩 token（替换蓝色主色，建立 Fintech 风格配色）
   - 定义字号层级（Display/Title/Body/Caption 等规范）
   - 定义间距系统、圆角、阴影规范
   - 封装基础组件变体（Button/Input/Card 等按新设计调整）
   - 建立 Skeleton 组件、EmptyState 组件统一封装

2. **App Shell 建立**
   - 统一 Layout 组件（Header + Main + BottomTab + SafeArea）
   - 页面转场动画（App 化滑动/淡入）
   - 合并 classic + vault 模板为单一模板
   - 新 BottomTabNav（Home/Orders/Discovery/Me + 未读红点）

3. **CNY/USDT 双币种模型**
   - 建立 `useCurrency` composable（汇率快照、币种格式化）
   - 钱包余额按实际币种展示
   - 结算/支付页展示"¥XXX CNY + 预计扣除 ≈ Y.YY USDT"
   - 前端不自行计算 USDT 扣款，仅展示后端返回值

4. **Network UX 增强**
   - `useSWR` composable（Local First + 后台刷新）
   - 请求缓存 + 自动重试 + 网络状态监听
   - 错误边界增强（保留数据 + 错误提示）

### Phase 1：核心页面重构

5. **首页重构**（四宫格 + 钱包区域 + 模块化 + Banner 保留）
6. **订单页重构**（独立页面 + 5 状态 Tab + 下拉刷新 + 未收到/售后）
7. **我的页面重构**（功能网格 + USDT 钱包卡片 + 入口整理）
8. **结算/支付页重构**（CNY/USDT 展示 + 流程优化）

### Phase 2：新增页面

9. **发现页**（Config Driven UI + 模块渲染器）
10. **消息通知页**（通知列表 + 未读计数 + SSE/轮询）
11. **帮助中心**（FAQ + 文章）
12. **在线客服**（会话列表 + 实时消息）

### Phase 3：优化

13. **登录/注册/商品/购物车视觉重设计**
14. **性能优化**（chunk 拆分、usePersonalCenter 拆分）
15. **ESLint/Prettier 配置 + Vitest 迁移**
16. **PWA 支持**（离线缓存、Push 通知）

---

## 20. Risk List

### P0（阻塞性，必须立即解决）

| # | 风险 | 影响 | 证据 |
|---|------|------|------|
| 1 | **CNY/USDT 双币种模型缺失** | 钱包余额币种错误展示；前端直接比较 USDT 余额与 CNY 订单金额；`walletOnlyPayment` 判断逻辑错误；用户可能被错误地阻止支付或看到错误的扣款金额 | `WalletPanel.vue:274`, `useCheckout.ts:128-134`, `usePayment.ts:424-430` |
| 2 | **消息通知系统完全缺失** | 用户无法接收订单状态变更、系统公告、客服消息；无未读提醒；与 HCZ 运营目标冲突 | 无 notification store、无 `/api/v1/notifications` API、无 WebSocket/SSE |

### P1（高优先级，重构前必须规划）

| # | 风险 | 影响 | 证据 |
|---|------|------|------|
| 3 | **底部导航 IA 与目标不符** | 当前为 Home/Products/Cart/Me，目标为 Home/Orders/Discovery/Me；需要新增订单独立页和发现页 | `MobileBottomNav.vue:49-57` |
| 4 | **双模板系统全量重复** | 28 个 vault 文件与 classic 并行维护，维护成本翻倍，bug 需修两处；vault CSS 与 classic CSS 可能冲突 | `templates/vault/` 目录 28 文件 |
| 5 | **无 Local First / 离线体验** | 网络抖动即清空数据；每次进入页面重新 Skeleton；无自动重试；用户体验差 | `OrdersPanel.vue:331` catch 时 `orders.value = []`；无 SWR/cache/retry |
| 6 | **订单状态粒度过细 + 操作缺失** | 10 种状态对用户不友好；"未收到""售后"操作未实现；与目标 5 状态不符 | `utils/status.ts:5-16`；OrderDetail 无未收到/售后按钮 |
| 7 | **无 ESLint/Prettier** | 代码风格无强制检查，长期维护质量风险 | `package.json` 无 lint 脚本，无 ESLint 配置 |

### P2（中优先级，重构过程中解决）

| # | 风险 | 影响 | 证据 |
|---|------|------|------|
| 8 | **API 路径不一致** | `/api/v1/order/payment-channels`（单数）vs `/api/v1/orders/*`（复数），可能导致后端路由混淆 | `api/order.ts:46` |
| 9 | **`usePersonalCenter` chunk 过大** | 107 KB（gzip 23 KB），个人中心首次加载慢 | 构建产物分析 |
| 10 | **package-lock.json 与 pnpm-lock.yaml 并存** | 包管理器不一致，可能导致依赖版本差异 | 两个 lockfile 同时存在 |
| 11 | **无打包体积分析工具** | 无法持续监控 bundle 体积 | 无 rollup-plugin-visualizer |
| 12 | **测试框架为 node --test** | 不支持 Vue 组件测试、无 watch 模式、无覆盖率报告 | `package.json:9` |
| 13 | **首页推荐商品数量写死** | `page_size: 15` 不可配置 | `Home.vue:481` |
| 14 | **支付轮询无指数退避** | 固定 5s 轮询，长时间未支付时持续请求 | `usePayment.ts:701` |

### P3（低优先级，后续优化）

| # | 风险 | 影响 | 证据 |
|---|------|------|------|
| 15 | **GitHub 链接 3 处硬编码** | 维护不便，应提取常量 | `main.ts`, `Footer.vue`, `VaultLayout.vue` |
| 16 | **classic Navbar 桌面导航模式** | 与 App 化目标冲突，统一 Shell 后可移除 | `components/Navbar.vue` |
| 17 | **无 PWA 支持** | 无离线缓存、无 Add to Home Screen、无 Push 通知 | 无 service worker、无 manifest |
| 18 | **语言包体积较大** | en-US 98 KB / zh-TW 96 KB，可考虑按语言拆分更细 | 构建产物分析 |
| 19 | **`BreadcrumbNav.vue` 可能未引用** | 死代码风险 | 需确认引用 |
| 20 | **无错误监控（Sentry 等）** | 生产环境错误无法追踪 | 无错误监控集成 |

---

## 附录：审计命令记录

| 命令 | 结果 |
|------|------|
| `npx vue-tsc -b --noEmit` | exit 0，无错误 |
| `npm run build` | exit 0，17.20s，2987 modules |
| `npm test` | exit 0，62 tests / 62 pass / 0 fail |
| `git status` | 有未提交修改（前后端均有），审计未触碰 |
| 源码文件扫描 | `src/` 下 200+ 文件，全部人工抽查关键文件 |

---

*审计完成。等待确认下一阶段。*
