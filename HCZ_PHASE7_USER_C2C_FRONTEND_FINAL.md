# HCZ Phase 7 — User C2C Frontend Implementation — Final Report

**日期**: 2026-10-05
**项目**: hcz_user (frontend/user)
**范围**: C2C 用户端完整 UI + API 接入（不改后端资金逻辑）

---

## Final Verdict: PASS

---

## 一、交付物清单

### 基础层（API / 类型 / 状态 / 组合式函数）

| 文件 | 作用 |
|---|---|
| `src/api/c2c.ts` | 统一 C2C API 层：支付方式 / 挂单 / 交易 / 钱包，全部 TypeScript 类型集中管理，含风控错误码常量、幂等键自动生成 |
| `src/stores/c2c.ts` | 共享 Pinia Store：wallet / paymentMethods / marketListings / myListings / myTrades / currentTrade，含 currentUserRole、isCurrentTradeTerminal computed |
| `src/composables/useC2C.ts` | 共享组合式函数：useCountdown（后端 expired_at 倒计时）、useTradePolling（12s 轮询 / 页面 hidden 暂停 / terminal 停止）、useC2CTradeActions（markPaid / cancel / confirm / dispute / createTrade，内置 14 种风控错误中文映射） |
| `src/i18n/locales/zh-CN.json` | 完整 c2c.* 中文翻译（首页 / 买 / 卖 / 挂单 / 交易 / 收款方式 / 申诉 / 错误） |
| `src/i18n/locales/en-US.json` | 对应英文翻译 |
| `src/router/index.ts` | 新增 9 条 C2C 路由（/c2c, /c2c/buy, /c2c/sell, /c2c/listings/:id, /c2c/my-listings, /c2c/trades, /c2c/trades/:id, /c2c/payment-methods） |
| `src/api/index.ts` | 导出 c2cAPI 及 C2C 类型 |
| `src/views/Notifications.vue` | C2C 通知类型（7 种）点击跳转 /c2c/trades/:id |

### 页面层（8 个页面）

| 文件 | 功能 |
|---|---|
| `src/views/c2c/C2CHome.vue` | C2C 首页：Wallet 三卡（available/frozen/total）+ 2×2 入口（买/卖/我的交易/我的挂单）+ 最近交易摘要 |
| `src/views/c2c/BuyUSDT.vue` | 买 USDT：市场挂单列表（PC 表格 / Mobile 卡片）、fiat+金额筛选、购买面板（fiat↔USDT 联动预览）、自己挂单标记禁用、分页加载 |
| `src/views/c2c/SellUSDT.vue` | 卖 USDT / 发布挂单：Wallet 展示、无启用收款方式时引导跳转、挂单表单（fiat/price/total/min/max/terms）、前端校验 |
| `src/views/c2c/ListingDetail.vue` | 挂单详情：完整信息展示、自己挂单操作（edit/pause/resume/close）、他人 active 挂单购买按钮 |
| `src/views/c2c/MyListings.vue` | 我的挂单：状态筛选（全部/active/paused/closed）、PC 表格+Mobile 卡片、edit/pause/resume/close、编辑弹窗、分页 |
| `src/views/c2c/MyTrades.vue` | 我的交易：7 状态筛选、自动判断买家/卖家角色+对手方、PC 表格+Mobile 卡片、分页、Buyer/Seller 共用 |
| `src/views/c2c/TradeDetail.vue` | 交易详情（最重要）：PC 双栏（交易信息 / 操作收款面板）、六状态动态交互、倒计时、轮询、收款信息快照解析、申诉表单 |
| `src/views/c2c/PaymentMethods.vue` | 收款方式：CRUD + enable/disable、类型（Bank/Alipay/WeChat/PayNow/Custom）、添加/编辑弹窗、后端脱敏字段直接展示 |

### 后端微调（非资金逻辑）

| 文件 | 变更 |
|---|---|
| `internal/modules/c2c/transport/presenter/presenter.go` | ListingResp 新增 `created_at` 字段（前端我的挂单需要展示创建时间，原 presenter 遗漏） |

### 测试

| 文件 | 覆盖 |
|---|---|
| `tests/c2cFrontend.test.ts` | 32 个单元测试：交易状态机、操作可用性、轮询启停、倒计时格式化、金额换算、表单校验、风控错误映射、通知深链、Wallet 展示、自交易防护、幂等键、共用架构验证 |

---

## 二、Trade Detail 六状态交互验证

| 状态 | Buyer 视角 | Seller 视角 |
|---|---|---|
| **pending_payment** | 卖家收款信息（payment_method_snapshot JSON 解析）+ 倒计时 + payment_reference 输入 + "我已付款"（确认弹窗）+ "取消交易"（确认弹窗） | "等待买家付款" + 倒计时，无操作 |
| **paid** | "等待卖家确认" + "发起申诉" | "确认已收到法币"（高风险二次确认，确认后冻结 USDT 转入买家 Wallet，不可撤销）+ "发起申诉" |
| **disputed** | "申诉中，等待平台仲裁" + 申诉原因 | 同左 |
| **completed** | "已收到平台 Wallet USDT" | "交易完成" |
| **canceled** | "已取消，卖家冻结余额已退回" | 同左 |
| **expired** | "已超时，卖家冻结余额已退回" | 同左 |

**倒计时**: 使用后端 `expired_at`，前端仅显示，到期触发 `fetchCurrentTrade` 刷新，不自行设置 expired。

**Polling**: pending_payment / paid / disputed 状态每 12 秒刷新；页面 `visibilityState === 'hidden'` 时暂停；切回前台立即刷新并恢复；terminal 状态（completed/canceled/expired）自动停止。

---

## 三、风控错误 UX（14 种，不统一显示"操作失败"）

| 后端错误码 | 前端中文提示 |
|---|---|
| `error.c2c_self_trade` | 不能购买自己的挂单 |
| `error.c2c_listing_not_active` | 挂单当前不可交易（已暂停或已关闭） |
| `error.c2c_invalid_amount` | 交易金额不在挂单的最小/最大范围内，或数量无效 |
| `error.c2c_status_invalid` | 当前交易状态不允许此操作 |
| `error.c2c_totp_required` | 请先启用谷歌验证器（TOTP）后再使用 C2C 交易 |
| `error.c2c_new_user_cooldown` | 新账号需等待冷却期结束后才能使用 C2C 交易 |
| `error.c2c_no_payment_method` | 请先添加并启用至少一个收款方式 |
| `error.c2c_daily_limit` | 今日交易额度已达上限，请明日再试 |
| `error.c2c_insufficient_balance` | 可用余额不足，无法完成此操作 |
| `error.c2c_permission_denied` | 您没有权限执行此操作（账号可能已被限制 C2C 交易） |
| `error.c2c_user_inactive` | 账号状态异常，无法使用 C2C 交易 |
| `error.c2c_disabled` | C2C 交易功能暂未开放 |
| `error.c2c_not_found` | 挂单或交易不存在 |
| `error.idempotency_key_required` | 请求缺少幂等键，请重试 |

---

## 四、Classic + Vault 共用架构

- **API 共用**: `src/api/c2c.ts` 唯一一份
- **Types 共用**: 所有 C2C 类型定义在 `src/api/c2c.ts`
- **Store 共用**: `src/stores/c2c.ts` 唯一一份 Pinia store
- **Composable 共用**: `src/composables/useC2C.ts` 唯一一份（倒计时/轮询/交易操作/状态标签）
- **Trade status/action logic 共用**: 六状态判断、操作可用性、风控错误映射全部在 `useC2C.ts`
- **classic/vault 只负责 layout/spacing/visual theme**: C2C 页面通过 `templateView()` 注册，vault 模板下自动回退到 classic 页面（业务逻辑零复制）

---

## 五、响应式布局

| 断点 | 布局 |
|---|---|
| **PC (md+)** | C2C Market 表格 + 筛选区；Trade Detail 双栏（左交易信息 3/5，右操作面板 2/5）；我的挂单/交易表格 |
| **Tablet (sm-md)** | 2 列网格，不强行套 Desktop table |
| **Mobile (<sm)** | 单列 Card，关键信息优先，操作按钮内嵌卡片 |

视觉方向：HCZ 现代生活服务 + 数字钱包风格，强调安全/清晰/易操作。**未使用**大面积黑金、K 线、高饱和红绿盘口。

---

## 六、构建验证结果

| 验证项 | 结果 |
|---|---|
| `vue-tsc --noEmit` | **PASS** — 0 错误 |
| `npm test`（全量 94 测试） | **PASS** — 94/94（含 32 个新增 C2C 测试） |
| `npm run build`（vue-tsc + vite build） | **PASS** — exit 0，3023 modules，所有 C2C 页面生成为独立 chunk |
| 后端 `go build ./internal/modules/c2c/...` | **PASS** — presenter 新增字段不影响编译 |
| classic 模板 | **PASS** — 直接使用 views/c2c/*.vue |
| vault 模板 | **PASS** — templateView 自动回退 classic C2C 页面，业务逻辑共用 |

---

## 七、12 项验收问题回答

| # | 验收项 | 结论 |
|---|---|---|
| 1 | **买 USDT 是否可用** | ✅ 是。市场挂单列表、筛选、购买面板、fiat↔USDT 预览、创建交易跳转详情，全部实现 |
| 2 | **卖 USDT / 发布挂单是否可用** | ✅ 是。表单校验、收款方式前置检查、发布挂单、跳转我的挂单，全部实现 |
| 3 | **我的挂单是否可用** | ✅ 是。状态筛选、列表、edit/pause/resume/close、编辑弹窗、分页，全部实现 |
| 4 | **我的交易是否可用** | ✅ 是。7 状态筛选、Buyer/Seller 角色自动判断、对手方展示、分页，全部实现 |
| 5 | **Trade Detail 六状态交互是否正确** | ✅ 是。pending_payment/paid/disputed/completed/canceled/expired 六状态 × Buyer/Seller 双视角动态操作，倒计时+轮询+申诉+确认+取消全部正确 |
| 6 | **Wallet available/frozen 是否正确展示** | ✅ 是。首页/买/卖页面均展示 available_balance/frozen_balance/total_balance，币种固定 USDT，不读取旧 balance 字段 |
| 7 | **Payment Methods 是否可用** | ✅ 是。list/create/edit/delete/enable-disable，5 种类型（Bank/Alipay/WeChat/PayNow/Custom），后端脱敏字段直接展示 |
| 8 | **Notification 是否能跳 Trade** | ✅ 是。7 种 C2C 通知类型（c2c_trade_created/c2c_buyer_paid/c2c_trade_completed/c2c_trade_canceled/c2c_trade_expired/c2c_disputed/c2c_arbitrated）点击后跳转 /c2c/trades/:id |
| 9 | **classic/vault 是否共用业务逻辑** | ✅ 是。API/types/store/composable/Trade status action logic 全部单文件共用，classic/vault 仅负责 layout/theme，零复制 |
| 10 | **PC/Mobile responsive 是否完成** | ✅ 是。PC 表格/双栏，Tablet 2 列，Mobile 单列卡片，全部实现 |
| 11 | **User build 是否全绿** | ✅ 是。vue-tsc 0 错误、94/94 测试通过、production build 成功、后端编译通过 |
| 12 | **User C2C 是否可以正式封板** | ✅ **是，可以正式封板** |

---

## 八、未做事项（按禁止范围）

- ❌ 未修改 C2C 后端资金逻辑
- ❌ 未加 BUY listing
- ❌ 未加 Merchant
- ❌ 未加 fee（前端固定展示 fee=0）
- ❌ 未加链上 C2C
- ❌ 未加 WebSocket（使用 12s 轮询）
- ❌ 未重构整套 HCZ 前端

---

## 九、已知说明

1. **后端 presenter 补字段**: `ListingResp` 原遗漏 `created_at`，本次补充（仅 DTO 字段，不涉及资金逻辑），使"我的挂单"能展示创建时间。
2. **BuyUSDT 加载更多**: 因 store 的 `fetchMarketListings` 为整体替换语义，加载更多直接调 `c2cAPI.listMarketListings` 后追加进 store 状态并同步分页，仍走统一 API 层与 store。
3. **vault C2C 页面**: 第一版未创建 vault 专属 C2C 页面，vault 模板通过 `templateView()` 自动回退到 classic 页面，业务逻辑完全共用。后续如需 vault 专属视觉可在 `templates/vault/c2c/` 下增量添加。
