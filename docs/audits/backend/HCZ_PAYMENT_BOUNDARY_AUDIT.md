# HCZ Payment Boundary Audit

**审计对象**：Aether-v1/hcz Payment / Wallet / Order 资金边界
**审计日期**：2026-10-03
**审计原则**：不修改业务代码，仅审计现状
**强制业务规则基准**：
- 支付渠道只允许用于给平台钱包充值
- 商品订单只能使用 Wallet 支付，禁止直接调用在线支付渠道
- 禁止商品订单混合支付
- Wallet 统一使用 USDT
- 商品按全站币种定价，下单时用 Global Exchange Rate 换算 USDT 并快照
- Payment Gateway Rate 与 Global Exchange Rate 彻底隔离

---

## 0. 核心结论（先回答问题）

| 问题 | 答案 |
|---|---|
| Order create 是否存在直接 gateway 支付路径？ | **存在，2 条路径**：①`CreateOrderAndPay` 合并接口 ②`POST /payments` 独立创建支付。两者均调用 `PaymentService.CreatePayment()`，可创建 gateway payment |
| 是否存在 wallet + online 混合支付？ | **存在**。`UseBalance=true` + `ChannelID>0` 时，钱包先付，剩余走在线网关；orders 表有 `wallet_paid_amount` / `online_paid_amount` 双字段 |
| 是否存在订单支付 channel_id / payment_id？ | **orders 表无 channel_id/payment_id**（一对多，payments.order_id 反向关联）；但 orders 有 `wallet_paid_amount` / `online_paid_amount` |
| PaymentService 是否会处理商品订单？ | **会**。`PaymentService.CreatePayment(input OrderID)` 专门处理商品订单支付，创建 gateway payment |
| Refund 是否错误依赖 payment gateway？ | **不依赖执行，但依赖手续费核算**。Refund 不调用 gateway 退款 API（`fee.go:28` 明确注释），仅读取 payment 记录计算手续费退还金额；钱包退款直接走 wallet service |
| Wallet recharge 与 business order 是否真正分域？ | **部分分域**。Service 方法分离（`CreateWalletRechargePayment` vs `CreatePayment`）、表分离（`wallet_recharge_orders`），但共用同一 PaymentService、同一 payments 表、同一 gateway 适配器、同一 webhook handler，靠 `payments.order_id=0` 区分 |
| 前端下单页面是否还能选择支付渠道？ | **能**。`useCheckout.ts` 有 `selectedChannelId`、`paymentChannels` 列表、`useBalance` 开关，提交时传 `channel_id` 给 `createAndPay` |
| Admin 是否存在商品订单"重新支付/在线支付"入口？ | **无创建入口**。Admin 支付路由全只读（GET /payments），Payments.vue 仅展示已有 payment 的 pay_url；但 Admin 可管理支付渠道（CRUD），间接影响用户可选渠道 |

---

## 1. 现有资金流全景

### 1.1 两条商品订单支付路径

```
路径 A：CreateOrderAndPay（合并接口）
  POST /api/v1/orders/create-and-pay
  → OrderService.CreateOrder()
  → PaymentService.CreatePayment(OrderID, ChannelID, UseBalance)
  → 若 ChannelID>0：创建 gateway payment（pay_url/qr_code）
  → 若 UseBalance=true：钱包扣款
  → 若两者都有：混合支付

路径 B：CreateOrder + CreatePayment（分步接口）
  POST /api/v1/orders          → 创建订单（pending_payment）
  POST /api/v1/payments        → 传 order_no + channel_id + use_balance
  → PaymentService.CreatePayment()
  → 同上逻辑
```

**两条路径都能为商品订单创建 gateway payment，违反 HCZ 规则。**

### 1.2 钱包充值路径（独立）

```
POST /api/v1/wallet/recharge/payments
  → PaymentService.CreateWalletRechargePayment(UserID, ChannelID, Amount, Currency)
  → 创建 wallet_recharge_orders（充值单）
  → 创建 payments（order_id=0，关联 recharge）
  → 调用 gateway 创建支付
  → webhook 回调成功后：钱包加款 + recharge 标记成功
```

**钱包充值路径是正确的，应保留。**

### 1.3 现有 Wallet-Only 机制（关键发现）

底座**已内置** wallet-only 支付模式：

| 层 | 实现 | 文件 |
|---|---|---|
| Setting | `GetWalletOnlyPayment()` 读取 `wallet_only_payment` 配置 | `settings/application/general.go:392` |
| Public Config | 下发 `wallet_only_payment: true` | `settings/transport/http/public/handler.go:167` |
| Order 创建 | wallet-only 时预校验余额，不足则拒绝下单 | `order/application/order_service.go:452-467` |
| Payment 创建 | wallet-only 时强制 `UseBalance=true`，拒绝 `ChannelID!=0` | `payment/application/payment_service_create.go:78-84` |
| 前端 | `walletOnlyPayment` computed，隐藏渠道选择 | `frontend/user/src/composables/useCheckout.ts:123` |

**这是 HCZ 可直接复用的核心机制。** 但当前是 config toggle（可关闭），HCZ 应将其升级为**硬编码边界**（不可关闭）。

---

## 2. 逐审计点详细分析

### 2.1 Order create 直接 gateway 支付路径

**证据：**
- `order/transport/http/create_handler.go:164-204`：`CreateOrderAndPay` 创建订单后调用 `h.pay.CreatePayment()`
- `create_handler.go:37-58`：`CreatePaymentInput` 含 `OrderID`、`ChannelID`、`UseBalance`
- `payment/transport/http/write_handler.go:75-79`：`CreatePaymentRequest` 含 `OrderNo`、`ChannelID`、`UseBalance`
- `payment/application/payment_service_create.go:55`：`PaymentService.CreatePayment()` 处理商品订单
- `payment_service_create.go:222-237`：创建 gateway payment（含 `ChannelID`、`ProviderType`、`PayURL`、`QRCode`）

**结论：BLOCK — 必须关闭商品订单的 gateway payment 创建路径**

### 2.2 Wallet + Online 混合支付

**证据：**
- `payment_service_create.go:156-166`：`UseBalance=true` 时调用 `ApplyWalletBalance()` 扣钱包
- `payment_service_create.go:168`：`onlineAmount = TotalAmount - WalletPaidAmount`
- `payment_service_create.go:169-202`：若 `onlineAmount <= 0`，纯钱包支付完成
- `payment_service_create.go:203-254`：若 `onlineAmount > 0` 且有 channel，创建在线 gateway payment
- `order/domain/order.go`：`WalletPaidAmount` + `OnlinePaidAmount` 双字段
- `create_handler.go:41`：`CreatePaymentInput.UseBalance` + `ChannelID` 可同时为 true

**结论：BLOCK — 必须禁止商品订单混合支付，只允许纯钱包支付**

### 2.3 订单支付 channel_id / payment_id

**证据：**
- `order/domain/order.go`：无 `ChannelID`、`PaymentID` 字段
- `payment/domain/payment.go`：有 `OrderID` 字段（多对一）
- 一个订单可有多笔 payment（重试、部分支付等）
- orders 有 `WalletPaidAmount`、`OnlinePaidAmount` 跟踪支付构成

**结论：KEEP — 现有设计合理（一对多），HCZ 只需确保商品订单的 payment 只能是 wallet 类型**

### 2.4 PaymentService 处理商品订单

**证据：**
- `PaymentService` 有两个创建方法：
  - `CreatePayment(input CreatePaymentInput{OrderID, ChannelID, UseBalance})` — 商品订单
  - `CreateWalletRechargePayment(input CreateWalletRechargePaymentInput{UserID, ChannelID, Amount})` — 钱包充值
- 两者共用 `PaymentService` 结构体、`paymentRepo`、`channelRepo`、`walletSvc`
- 两者都写入 `payments` 表，靠 `OrderID=0` 区分充值

**结论：MODIFY — PaymentService 需明确分域：商品订单支付只走 wallet 分支，gateway 分支只对 recharge 开放**

### 2.5 Refund 对 payment gateway 的依赖

**证据：**
- `order/application/refund/fee.go:28`：注释明确 "It does not call a payment gateway"
- `refund/fee.go:17-18`：`paymentFeeReader` 接口仅 `ListByOrderID()` — 只读 payment 记录
- `refund/wallet.go`：退款直接调用 `walletService` 加款，不经过 payment gateway
- `refund/service.go`：退款记录 `order_refund_records` 有 `PaymentFeeRefunded` / `PaymentFeeRefundedAmount` 字段，仅用于手续费核算
- 无 `RefundPayment()` / `GatewayRefund()` 方法调用

**结论：KEEP — Refund 不依赖 gateway 执行，仅读 payment 记录算手续费。HCZ 纯钱包模式下手续费为 0，此依赖自然消解**

### 2.6 Wallet recharge 与 business order 分域

**证据：**

| 维度 | Wallet Recharge | Business Order |
|---|---|---|
| Service 方法 | `CreateWalletRechargePayment` | `CreatePayment` |
| 入参 | UserID, ChannelID, Amount | OrderID, ChannelID, UseBalance |
| 关联表 | `wallet_recharge_orders` | `orders` |
| payments.order_id | 0 | >0 |
| 成功后动作 | 钱包加款 | 订单标记 paid + 钱包扣款 |
| 回调处理 | webhook → recharge 成功 → 加款 | webhook → payment 成功 → 订单 paid |

**共用部分：**
- 同一 `PaymentService`
- 同一 `payments` 表
- 同一 gateway 适配器（Alipay/WeChat/Stripe/BEPUSDT 等）
- 同一 webhook handler（需区分 order_id=0 vs >0）
- 同一 `payment_channels` 配置

**结论：MODIFY — 域已部分分离，但需加固边界：gateway 适配器只对 recharge 开放，business order 的 CreatePayment 只允许 wallet 分支**

### 2.7 前端下单页支付渠道选择

**证据：**
- `frontend/user/src/composables/useCheckout.ts:76-81`：`orderPaymentChannels`、`selectedChannelId`、`useBalance`
- `useCheckout.ts:86-121`：`paymentChannels` computed，从全局配置或订单专属接口获取渠道列表
- `useCheckout.ts:123`：`walletOnlyPayment = computed(() => !!appStore.config?.wallet_only_payment)`
- `useCheckout.ts:589`：`if (!walletOnlyPayment.value && requiresOnlineChannel.value && !selectedChannelId.value) return false`
- `useCheckout.ts:822`：提交时 `channel_id: requiresOnlineChannel.value ? (selectedChannelId.value || undefined) : undefined`
- `useCheckout.ts:829`：调用 `userOrderAPI.createAndPay(payload)`

**结论：MODIFY — 前端已有 walletOnlyPayment 开关逻辑，HCZ 开启后渠道选择自动隐藏。但需确保 createAndPay 不再传 channel_id，且后端硬拒绝**

### 2.8 Admin 商品订单支付入口

**证据：**
- `payment/transport/http/routes.go:40-47`：Admin 支付路由全只读：`GET /payments`、`GET /payments/export`、`GET /payments/:id`
- `routes.go:50-60`：Admin 渠道 CRUD：`POST/PUT/DELETE /payment-channels`（管理渠道配置，非创建订单支付）
- `frontend/admin/src/views/admin/Payments.vue:695-698`：展示已有 payment 的 `pay_url`（只读查看）
- 无 `POST /admin/orders/:id/payments` 或"重新支付"按钮
- Admin 订单详情 `OrderDetailDialog.vue` 无支付创建入口

**结论：KEEP — Admin 无商品订单支付创建入口，仅只读查看。Admin 渠道 CRUD 保留给钱包充值渠道管理**

---

## 3. Payment Gateway Rate vs Global Exchange Rate 隔离审计

### 3.1 现有 Payment Gateway Rate

| 维度 | 现状 |
|---|---|
| 存储位置 | `payments.provider_payload["exchange_rate"]`（JSON 字段） |
| 设置方式 | 每个支付渠道各自配置静态汇率（channel config JSON） |
| 用途 | 在线支付网关法币换汇（如 Stripe USD → 订单 CNY） |
| 快照时机 | 支付创建时写入 `ProviderPayload` |
| 回调校验 | `paymentCoveredOrderAmount()` 用快照汇率换算回订单币种 |
| 全局获取 | 无 |
| 手动 fallback | 无（渠道配置即静态值） |
| 关联订单 | 通过 `payments.order_id` 间接关联 |

### 3.2 HCZ 需要的 Global Exchange Rate

| 维度 | 需求 |
|---|---|
| 存储位置 | 独立 `exchange_rates` 表 |
| 设置方式 | 自动获取（外部 API）+ 后台手动 fallback（fallback 优先） |
| 用途 | 商品订单下单时 CNY→USDT 换算 |
| 快照时机 | 订单创建时写入 `orders.exchange_rate` |
| 回调校验 | 不需要（钱包支付无回调） |
| 全局获取 | `ExchangeRateService.GetCurrentRate()` |
| 手动 fallback | Admin 汇率设置页，fallback 优先于自动获取 |
| 关联订单 | 直接写入 `orders.exchange_rate` / `orders.usdt_amount` |

### 3.3 隔离结论

**两者必须彻底隔离，不可复用：**
- Gateway Rate 属于"支付渠道充值钱包"域，存储在 payments JSON，仅用于在线支付回调校验
- Global Rate 属于"商品订单定价换算"域，存储在独立表，用于下单时 USDT 换算
- HCZ v1 不接在线支付，Gateway Rate 代码全部停用（但保留给 recharge）
- Global Exchange Rate 必须从零 BUILD

---

## 4. HCZ Payment Boundary Matrix

| # | 审计点 | 现状 | HCZ 要求 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 1 | CreateOrderAndPay 合并接口 | 创建订单+gateway payment | 商品订单只钱包支付 | **BLOCK** | P0 |
| 2 | POST /payments 独立创建支付 | 可为商品订单创建 gateway payment | 商品订单禁止 | **BLOCK** | P0 |
| 3 | PaymentService.CreatePayment(OrderID) | 处理商品订单，支持 gateway | 只允许 wallet 分支 | **MODIFY** | P0 |
| 4 | Wallet + Online 混合支付 | 支持（UseBalance+ChannelID） | 禁止混合 | **BLOCK** | P0 |
| 5 | orders.wallet_paid_amount / online_paid_amount | 双字段跟踪 | online_paid_amount 恒为 0 | **MODIFY** | P0 |
| 6 | Wallet-only 配置开关 | 已存在（config toggle） | 升级为硬边界 | **MODIFY** | P0 |
| 7 | 前端支付渠道选择 | 可选择渠道+混合 | 只显示钱包余额，隐藏渠道 | **MODIFY** | P0 |
| 8 | PaymentService.CreateWalletRechargePayment | 独立方法，正确 | 保留，仅用于充值 | **KEEP** | P0 |
| 9 | wallet_recharge_orders 表 | 独立表，正确 | 保留 | **KEEP** | P0 |
| 10 | payments 表（order_id=0 区分充值） | 共用表，靠 order_id 区分 | 保留，HCZ 商品订单 payment 只有 wallet 类型 | **KEEP** | P0 |
| 11 | Gateway 适配器（Alipay/WeChat/Stripe/BEPUSDT 等） | 10+ 网关 | 只保留给钱包充值，商品订单不可调用 | **MODIFY**（调用边界） | P0 |
| 12 | Payment webhook handler | 处理 gateway 回调 | 只处理充值回调（order_id=0），商品订单无在线支付 | **MODIFY** | P0 |
| 13 | Refund 执行 | 不调用 gateway，直接钱包退款 | 正确，保留 | **KEEP** | P0 |
| 14 | Refund 手续费核算 | 读 payment 记录算手续费 | 纯钱包模式手续费为 0，自然消解 | **KEEP** | P0 |
| 15 | Admin 支付只读查看 | GET /payments，展示 pay_url | 保留（查看充值支付记录） | **KEEP** | P0 |
| 16 | Admin 渠道 CRUD | POST/PUT/DELETE /payment-channels | 保留（管理充值渠道） | **KEEP** | P0 |
| 17 | Admin 商品订单"重新支付"入口 | 不存在 | 不需要 | **KEEP** | P0 |
| 18 | Payment Gateway Rate（ProviderPayload） | 渠道级静态汇率 | 只用于充值，不可用于商品订单 | **DROP**（商品订单域） | P0 |
| 19 | Global Exchange Rate | 不存在 | 必须独立 BUILD | **BUILD** | P0 |
| 20 | orders.exchange_rate / usdt_amount | 不存在 | 下单时快照 | **BUILD** | P0 |
| 21 | 商品订单 payment 类型约束 | 可创建任意类型 | 只允许 `provider_type=wallet` | **MODIFY** | P0 |
| 22 | 游客下单支付 | 游客无钱包，可在线支付 | HCZ 需登录用户，游客下单禁用 | **DROP** | P1 |
| 23 | Payment fee 计算（渠道手续费） | 支持费率+固定手续费 | 商品订单无手续费，充值可保留 | **MODIFY** | P1 |
| 24 | Payment 过期/迟到回调处理 | 完整 | 只对充值有效，商品订单钱包支付无过期 | **KEEP**（充值域） | P1 |

### 统计

| 结论 | 数量 |
|---|---|
| KEEP | 10 |
| MODIFY | 8 |
| BUILD | 2 |
| DROP | 2 |
| BLOCK | 4（BLOCK 是必须关闭的路径，归入 MODIFY 执行） |

---

## 5. 核心问题回答

### Q1：如何确保商品订单只能 Wallet 支付？

**四层防护（必须全部到位）：**

1. **Order 创建层（硬校验）**：`OrderService.createOrder()` 中，wallet-only 预校验已有（`order_service.go:452-467`），需将 `GetWalletOnlyPayment()` 条件改为**恒 true**（不再读配置），游客下单直接拒绝
2. **Payment 创建层（硬拒绝）**：`PaymentService.CreatePayment()` 中，删除 `ChannelID>0` 分支，`input.ChannelID != 0` 直接返回错误；只保留 `UseBalance=true` 的钱包支付分支
3. **API 路由层（关闭入口）**：不注册 `POST /api/v1/payments`（商品订单独立支付）、`CreateOrderAndPay` 中删除 `respondCreateAndPay` 的 gateway 分支；`POST /api/v1/orders/create-and-pay` 只创建订单+钱包扣款
4. **前端层（隐藏 UI）**：`useCheckout.ts` 中 `walletOnlyPayment` 恒 true，隐藏渠道选择 UI，提交时不传 `channel_id`

**关键：不能只靠前端隐藏，后端必须硬拒绝。**

### Q2：哪些现有 gateway 代码只保留给 Wallet Recharge？

**全部 gateway 适配器只保留给充值域：**

| 网关 | 文件 | 保留用途 |
|---|---|---|
| Alipay | `payment/infrastructure/gateway/adapters/alipay/` | 钱包充值（法币→USDT 钱包） |
| WeChatPay | `payment/infrastructure/gateway/adapters/wechatpay/` | 钱包充值 |
| Stripe | `payment/infrastructure/gateway/adapters/stripe/` | 钱包充值 |
| PayPal | `payment/infrastructure/gateway/adapters/paypal/` | 钱包充值 |
| BEPUSDT | `payment/infrastructure/gateway/adapters/bepusdt/` | 钱包充值（USDT 链上） |
| EPUSDT | `payment/infrastructure/gateway/adapters/epusdt/` | 钱包充值 |
| TokenPay | `payment/infrastructure/gateway/tokenpay/` | 钱包充值 |
| OKPay | `payment/infrastructure/gateway/okpay/` | 钱包充值 |
| EPay | `payment/infrastructure/gateway/adapters/epay/` | 钱包充值 |
| 独角数卡 | `payment/infrastructure/gateway/adapters/dujiaopay/` | 钱包充值 |

**调用边界：** 这些适配器只能被 `CreateWalletRechargePayment()` 调用，`CreatePayment(OrderID)` 中不得调用任何 gateway adapter。

### Q3：哪些订单支付代码必须关闭/删除？

**必须关闭的路径（P0）：**

| # | 代码位置 | 动作 |
|---|---|---|
| 1 | `order/transport/http/create_handler.go:164-204` `CreateOrderAndPay` | 删除 gateway payment 创建分支，只保留订单创建+钱包扣款 |
| 2 | `order/transport/http/create_handler.go:206-245` `CreateGuestOrderAndPay` | 整个关闭（HCZ 无游客下单） |
| 3 | `payment/transport/http/write_handler.go` `CreatePayment` / `CreateGuestPayment` | 关闭商品订单支付创建路由（`POST /api/v1/payments`），或改为只接受 wallet |
| 4 | `payment/application/payment_service_create.go:203-254` gateway payment 创建分支 | 删除或改为返回错误（ChannelID!=0 时拒绝） |
| 5 | `payment/application/payment_service_create.go:156-166` 混合支付逻辑 | 删除，只允许纯钱包 |
| 6 | `order/transport/http/routes.go` `RegisterUserPaymentChannelsRoute` | 关闭订单可用支付渠道查询接口（`POST /api/v1/order/payment-channels`） |
| 7 | `order/transport/http/user_handler.go:205-250` `GetOrderPaymentChannels` | 关闭或返回空列表 |
| 8 | `frontend/user/src/composables/useCheckout.ts` 渠道选择逻辑 | 隐藏渠道 UI，`walletOnlyPayment` 恒 true |
| 9 | `payment/transport/http/routes.go:26-27` `RegisterGuestWriteRoutes` | 关闭游客支付路由 |

**不删除代码（保留给充值）：**
- `PaymentService.CreateWalletRechargePayment()` — 充值核心
- `payment/infrastructure/gateway/` — 全部网关适配器
- `payment/transport/http/routes.go:31-37` 用户支付路由 — 可改为只接受充值支付（需区分）
- `payment/webhook/` — webhook 回调（只处理充值）

### Q4：Global Exchange Rate 与 Payment Gateway Rate 如何彻底隔离？

| 隔离维度 | Global Exchange Rate（新建） | Payment Gateway Rate（现有） |
|---|---|---|
| 表 | `exchange_rates`（独立表） | 无独立表，存 `payments.provider_payload["exchange_rate"]` |
| Service | `ExchangeRateService`（新建模块） | 无独立 service，嵌在 `PaymentService` |
| 调用方 | `OrderService.createOrder()`（下单时换算） | `PaymentService.CreateWalletRechargePayment()`（充值时） |
| 用途 | 商品 CNY 定价 → USDT 扣款换算 | 在线支付网关法币换汇（充值回调校验） |
| 快照存储 | `orders.exchange_rate` / `orders.usdt_amount` | `payments.provider_payload["exchange_rate"]` |
| 获取方式 | 自动外部 API + 手动 fallback（fallback 优先） | 渠道配置静态值 |
| Admin 页面 | 汇率设置页（新建） | 支付渠道配置页（现有） |
| HCZ v1 状态 | **必须 BUILD** | **代码保留，仅充值域使用** |

**代码层面不可交叉引用：**
- `OrderService` 不得 import `PaymentService` 的汇率逻辑
- `ExchangeRateService` 不得读写 `payments` 表
- `PaymentService.CreatePayment(OrderID)` 不得调用任何汇率换算（纯钱包支付不需要）

### Q5：最小修改范围是什么？

**P0 最小修改集（按执行顺序）：**

#### 后端（5 处修改 + 1 处新建）

| # | 文件 | 修改 | 类型 |
|---|---|---|---|
| 1 | `payment/application/payment_service_create.go` | `CreatePayment()` 中：删除 ChannelID>0 分支和混合支付逻辑；ChannelID!=0 直接返回错误；只保留 UseBalance 钱包支付 | MODIFY |
| 2 | `order/application/order_service.go` | `createOrder()` 中：wallet-only 预校验改为恒 true（不读配置）；游客下单直接拒绝 | MODIFY |
| 3 | `order/transport/http/create_handler.go` | `CreateOrderAndPay`：删除 gateway payment 分支，只创建订单+调用钱包支付；关闭 `CreateGuestOrderAndPay` | MODIFY |
| 4 | `payment/transport/http/routes.go` + `write_handler.go` | 关闭 `POST /api/v1/payments` 商品订单支付创建（或改为只接受 wallet+order）；关闭游客支付路由 | MODIFY |
| 5 | `order/transport/http/routes.go` + `user_handler.go` | 关闭 `POST /api/v1/order/payment-channels` 渠道查询接口 | MODIFY |
| 6 | **新建** `exchange_rate` 模块 | `exchange_rates` 表 + service（自动获取+手动 fallback）+ Admin API + 下单时调用写入 orders | BUILD |

#### 前端（2 处修改）

| # | 文件 | 修改 | 类型 |
|---|---|---|---|
| 7 | `frontend/user/src/composables/useCheckout.ts` | `walletOnlyPayment` 恒 true；隐藏渠道选择 UI；提交不传 channel_id | MODIFY |
| 8 | `frontend/user/src/views/Checkout.vue`（及支付步骤组件） | 移除支付渠道选择步骤，只显示钱包余额确认 | MODIFY |

#### 数据库（1 处新建 + 1 处字段）

| # | 变更 | 类型 |
|---|---|---|
| 9 | 新建 `exchange_rates` 表 | BUILD |
| 10 | `orders` 表加 `exchange_rate` + `usdt_total_amount` + `pricing_currency` + `pricing_amount` + `rate_source` + `rate_at` 字段 | BUILD |

**总计：后端 5 MODIFY + 1 BUILD，前端 2 MODIFY，DB 1 BUILD + 1 字段扩展。**

**不需要修改的（KEEP）：**
- Wallet 充值全链路（`CreateWalletRechargePayment` + gateway + webhook）
- Refund 全链路（钱包退款 + 佣金回滚）
- Admin 支付只读查看 + 渠道 CRUD
- orders / payments / wallet_transactions 表结构（仅加字段）
- 全部 gateway 适配器代码

---

## 6. 资金流目标状态（HCZ v1）

```
用户充值（Wallet Recharge）：
  用户 → 选择充值金额(USDT) → 选择支付渠道(Alipay/WeChat/USDT链上等)
  → PaymentService.CreateWalletRechargePayment()
  → 创建 wallet_recharge_orders + payments(order_id=0)
  → 调用 gateway 创建支付
  → 用户付款 → webhook 回调
  → 钱包加 USDT + recharge 标记成功

商品下单（Business Order）：
  用户 → 浏览商品(CNY定价) → 填写动态字段 → 提交订单
  → OrderService.createOrder()：
      ① 预校验钱包 USDT 余额充足
      ② 从 GlobalExchangeRateService 获取当前汇率(自动+手动fallback)
      ③ CNY总价 × 汇率 = USDT扣款金额
      ④ 保存 orders: pricing_currency=CNY, pricing_amount=X,
         exchange_rate=Y, usdt_amount=Z, rate_source, rate_at
      ⑤ 钱包扣 Z USDT（事务+幂等）
      ⑥ 订单状态 → processing（处理中）
  → 管理员人工履约 → completed
  → 失败/取消 → 自动退 Z USDT + 佣金回滚
  → 已完成 → 部分退款 → 退部分 USDT + 佣金按比例回滚

彻底隔离：
  商品订单 → 不接触 PaymentService.CreatePayment(OrderID) 的 gateway 分支
  商品订单 → 不接触任何 gateway adapter
  商品订单 → 不接触 Payment Gateway Rate
  商品订单 → 只接触 WalletService + GlobalExchangeRateService
```

---

## 7. 最终结论

1. **商品订单 gateway 支付路径真实存在且完整**（CreateOrderAndPay + POST /payments + PaymentService.CreatePayment + 混合支付），必须 BLOCK
2. **底座已内置 wallet-only 机制**（配置开关 + 前后端联动），可直接复用并升级为硬边界
3. **钱包充值域已正确分离**（独立方法 + 独立表 + order_id=0 区分），全部 gateway 代码保留给充值
4. **Refund 不依赖 gateway 执行**，仅读 payment 记录算手续费，纯钱包模式下自然消解
5. **Global Exchange Rate 必须从零 BUILD**，与 Payment Gateway Rate 彻底隔离，不可复用
6. **最小修改范围**：后端 5 处 MODIFY + 1 处 BUILD，前端 2 处 MODIFY，DB 1 表新建 + orders 加字段
7. **Admin 无商品订单支付创建入口**，仅只读查看，无需修改

**审计完成，等待下一阶段开发指令。**
