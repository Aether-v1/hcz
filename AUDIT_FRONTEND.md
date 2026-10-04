# 前端 Contract 与 Build 审计报告

> 审计对象：`frontend/user`（Vue3+TS+vite）、`frontend/admin`（Vue3+TS+vite）
> 冻结 Contract：`HCZ_FRONTEND_API_CONTRACT.md`（P0-2 USDT 结算）
> 环境：Windows + PowerShell 5.1，node v22.23.2 / npm 10.9.8，两端 `node_modules` 均已存在
> 所有 vue-tsc / build 命令均真实执行，输出见下文。

## 审计摘要
- P0 发现：3
- P1 发现：4
- P2 发现：6
- User vue-tsc：PASS（`npx vue-tsc --noEmit` exit 0，0 错误）
- User build：PASS（`npm run build` = `vue-tsc -b && vite build`，exit 0，31.20s）
- Admin vue-tsc：PASS（`npx vue-tsc --noEmit` exit 0，0 错误）
- Admin build：PASS（`npm run build` exit 0，18.93s；含 1 条 esbuild 警告：`admin/src/utils/status.ts` 重复 case `completed`，非阻断）

---

## 十一、前端 Contract 审计

### Contract 一致性检查

| 字段 | User前端 | Admin前端 | Contract | 状态 |
|---|---|---|---|---|
| Wallet currency | `WalletPanel.vue:275-278` balanceDisplay 读 `wallet.currency\|\|'USDT'` ✓；`WalletTransactionList.vue:95-99` 用流水记录自带 `item.currency`（USDT）✓；**但结账侧 `usePayment.ts:423` `walletBalanceDisplay = formatMoney(walletBalance, order.currency)` 把 USDT 钱包余额按站点币格式化 ✗** | `views/admin/Wallet.vue` 实为钱包**配置**页（recharge_channel_ids / wallet_only_payment），无余额展示 | Wallet=USDT | 部分违规（结账页） |
| Order total_amount（Site Currency） | `OrdersPanel.vue:108` `formatMoney(total_amount, order.currency)` ✓；`OrderDetail.vue:357` ✓；`VaultOrderBody.vue:73` ✓；`PaymentAmountBreakdown.vue:8` ✓ | `OrderDetailDialog.vue:838` `formatMoney(total_amount, selectedOrder.currency)` ✓ | Site Currency（site_config.currency） | 通过 |
| Order wallet_paid_amount（USDT 实付） | `OrdersPanel.vue:486-493` `usdtPaidDisplay` 用 `wallet_currency\|\|'USDT'`，优先 `wallet_paid_amount ?? usdt_total_amount`，注释明确"不自行重算" ✓；`OrderDetail.vue:362-363` 用 `wallet_currency\|\|'USDT'` ✓；**但 `VaultOrderBody.vue:78` `formatMoney(wallet_paid_amount, order.currency)` ✗；`usePayment.ts:502-507` `paymentWalletPaidDisplay` 用 `order.value?.currency` ✗** | **`OrderDetailDialog.vue:844` `formatMoney(wallet_paid_amount, selectedOrder.currency)` ✗**（且 `api/types.ts:154-189` `AdminOrder` 类型根本没有 `wallet_currency` 字段） | USDT | 违规（vault 模板 + 支付结果页 + Admin 弹窗） |
| Order refunded_amount（USDT 已退） | `OrderDetail.vue:372-373` 用 `wallet_currency\|\|'USDT'` ✓；**但 `VaultOrderBody.vue:86` 用 `order.currency` ✗** | **`OrderDetailDialog.vue:856` `formatMoney(refunded_amount, selectedOrder.currency)` ✗** | USDT | 违规（同 wallet_paid） |
| refund_status（none/partial/full） | `useOrderDisplayHelpers.ts:33-36` 仍用 `order.status==='refunded'\|\|'partially_refunded'` 决定是否展示退款卡片（旧9态）✗；未按 `refund_status` 枚举驱动 | `OrderDetailDialog` 售后工单内显示 `refund_amount + refund_currency`（USDT）✓；订单级 `refund_status` 枚举未上屏 | none/partial/full | 部分违规 |
| after_sale_status（none/pending/resolved/rejected） | `composables/useAfterSale.ts` 类型与逻辑正确：`status:'pending'\|'resolved'\|'rejected'`、`refund_status:'none'\|'partial'\|'full'`、`canInitiate` 要求 `orderStatus==='completed'`、`type:'not_received'` ✓ | `OrderDetailDialog.vue:530-576,1359-1412` 售后工单 UI 完整：pending/resolved/rejected 三色徽标、reject/resolve/partial_refund/full_refund 四 action、partial 金额标注 "(USDT)"、i18n "请输入有效的退款金额（USDT）" ✓；`api/admin.ts:523-524` 两个 after-sale 端点齐全 | none/pending/resolved/rejected | 通过 |
| Commission currency | `AffiliatePanel.vue:39/43/47` 三个佣金卡硬编码 "USDT" ✓；`:134` `commission_amount + (currency\|\|'USDT')` ✓ | **`AffiliateCommissions.vue:200` `{{ item.commission_amount \|\| '0.00' }}` 裸数字，无 USDT 单位 ⚠** | USDT | User 通过；Admin 缺单位 |
| 五态 status（pending_recharge/processing/completed/failed/canceled） | `utils/status.ts:5-20` 虽已加入新5态映射，**但仍完整保留旧9态**（pending_payment/paid/fulfilling/partially_delivered/partially_refunded/delivered/expired/refunded）；`OrdersPanel.vue:294-306` 状态筛选下拉=旧9态，**缺 pending_recharge/processing/failed**；`:130` 立即支付按钮 `v-if="order.status==='pending_payment'"`；`usePayment.ts:389/390/631...` 轮询/倒计时全部绑定 `pending_payment`；`usePayment.ts:1041` `['paid','fulfilling','partially_delivered','delivered','completed'].includes(status)` | `utils/status.ts` 同 User（新5态+旧9态并存）；`Orders.vue:320-328/430-434` 筛选下拉=旧9态；`Orders.vue:169` `markCompleted` 要求 `status==='delivered'`；`:181`、`OrderDetailDialog.vue:270` `canCreateFulfillment` 要求 `paid\|\|'fulfilling'`；`:157` `canUpdateStatus` 排除 `partially_refunded/refunded`；`Dashboard.vue:40/473` KPI `pending_payment_orders` | 仅5态 | 违规（两端均为旧9态兼容，新态未接管） |

### 禁止项检查

- **前端自行算汇率：发现（需分类）**
  - 【用户订单域 - 旧模型残留，P1】`composables/useCheckout.ts:128-139` `expectedWalletPaidCents = min(balance_cents, previewTotal_cents)`：把 USDT 钱包余额与站点币 total_amount 当**同币种**做 cents 取小/相减（`expectedOnlinePayCents = total - expectedWalletPaid`），再用 `previewCurrency`（站点币）展示。这是 P0-2 之前"钱包=站点币"时代的结账分账模型，未迁移到 USDT / `usdt_total_amount`。`usePayment.ts:424-437` 同构。
  - 【上游采购域 - Admin 工具，P2】`admin/.../ProcurementOrders.vue:338-349` `getExchangeRate = order.connection?.exchange_rate ?? 1`，并 `upstreamRefundInLocal = upstream_refunded_amount * rate`（前端做乘法换汇）；`admin/.../ProductMappings.vue:195` `return conn?.exchange_rate || 1`；`SiteConnections.vue:41-426` 配置上游连接汇率。此为**上游采购**定价换算，非用户订单 USDT 重算，但确属"前端做汇率乘法"。
  - 【合规】历史订单 USDT 实付**未**被前端按当前汇率重算：`OrdersPanel.vue:486-490` 明确注释"历史单无快照则显示 --，不自行重算"，直接读后端 `wallet_paid_amount/usdt_total_amount/exchange_rate` 快照。`OrderDetail.vue:375-380` 汇率快照仅只读展示 `1 USDT = R {currency}`，无计算。

- **前端调用 CoinGecko：未发现直连**。仅 `admin/.../SettingsExchangeRateTab.vue:16/43/136/141` 出现 provider 名 `'coingecko'` 与 "CoinGecko API Key（已设置掩码）" 输入框——这是 Contract §6 允许的 **Admin 汇率配置页**（展示后端 provider 名与掩码 key），前端没有任何对 coingecko.com 的 HTTP 请求。合规。

- **前端兼容旧9态：发现（大量，详见 P0/P1/P2）**。关键活跃逻辑位置：
  - User：`utils/status.ts:11-18,35-47`；`OrdersPanel.vue:130,296-305,313-319`；`OrderDetail.vue:72,75`；`GuestOrders.vue:70`；`GuestOrderDetail.vue:61`；`PersonalCenter.vue:255`；`usePayment.ts:389,390,631,648,663,774,858,895,1041,1234,1291`；`useOrderDisplayHelpers.ts:35,74,76`；`templates/vault/OrderDetail.vue:42,45`、`vault/GuestOrderDetail.vue:51`、`vault/GuestOrders.vue:54`、`vault/PersonalCenter.vue:162`；`utils/resellerConsole.ts:126-153`（分销控制台，独立域）。
  - Admin：`utils/status.ts:11-17,32-46`；`Orders.vue:157,169,181,320-328,430-434`；`OrderDetailDialog.vue:270,1153`；`Dashboard.vue:40,473`。
  - 说明：`i18n/locales/*.json` 与 `admin/i18n/index.ts` 中的旧态文案键属字典残留（死键），真正驱动业务的是上述 `status.ts` 映射与各 `v-if/filter`。

- **前端猜 currency：发现**
  - `useCheckout.ts:233` `totalCurrency = siteCurrency || 'CNY'`；`useCart.ts:26`、`useProductDetail.ts:606`、`useProduct.ts:17` 站点币兜底 `'CNY'`（站点币域，可接受）。
  - `WalletPanel.vue:257` `selectedChannelCurrency = appStore.config?.currency || 'CNY'`，用于充值渠道手续费/限额提示（网关侧币种，非钱包本位币）。
  - 真正违规是把 **USDT 金额**套用 `order.currency`（见上表 wallet_paid_amount / refunded_amount 行），即"用 site_config.currency 格式化钱包/退款金额"。

- **hardcoded CNY：发现（多为配置默认值，非钱包误标）**
  - 支付渠道配置默认法币：`admin/.../PaymentChannelModal.vue:144/167/187/416/439/459/562/585/606`（bepusdt fiat='CNY'、tokenpay base_currency='CNY'、dujiaopay fiat_currency='CNY' 等）——网关渠道法币默认值。
  - Admin 站点币兜底：`Settings.vue:92-108,191,399-400,614`（站点币下拉/默认 CNY）、`Products.vue:112,283-286`、`Users.vue:73,119-122`、`UserDetail.vue:86,103-106`、`WholesalePrices.vue:34,231-234`——均为站点币展示兜底，合规。
  - 无在钱包/退款/返利金额上硬编码 `¥`/`$` 符号的情况（formatMoney 统一用 `金额 + 币种码`，不渲染货币符号）。

---

## 十六、前端 Build / Type

### User vue-tsc 结果
命令：`cd frontend/user; npx vue-tsc --noEmit`
```
（无任何输出）
VUE-TSC-USER-EXIT=0
```
结论：**PASS，0 错误**。

### User build 结果
命令：`cd frontend/user; npm run build`（= `vue-tsc -b && vite build`）
```
> web@0.0.0 build
> vue-tsc -b && vite build
vite v7.3.6 building client environment for production...
transforming...
✓ 2988 modules transformed.
rendering chunks...
computing gzip size...
...
dist/assets/index-BFWi0aX_.js   326.29 kB │ gzip:  98.59 kB
✓ built in 31.20s
USER-BUILD-EXIT=0
```
结论：**PASS**。

### Admin vue-tsc 结果
命令：`cd frontend/admin; npx vue-tsc --noEmit`
```
（无任何输出）
VUE-TSC-ADMIN-EXIT=0
```
结论：**PASS，0 错误**。

### Admin build 结果
命令：`cd frontend/admin; npm run build`
```
> admin-web@0.0.0 build
> vue-tsc -b && vite build
vite v7.3.6 building client environment for production...
transforming...
[plugin vite:esbuild] src/utils/status.ts: This case clause will never be evaluated because it duplicates an earlier case clause
  39 |        return 'text-orange-700 border-orange-200 bg-orange-50'
  40 |      case 'delivered':
  41 |      case 'completed':
     |           ^
  42 |        return 'text-slate-800 border-slate-200 bg-slate-50'
✓ 2890 modules transformed.
...
dist/assets/index-BXa7tC0Z.js   482.18 kB │ gzip: 148.94 kB
✓ built in 18.93s
ADMIN-BUILD-EXIT=0
```
结论：**PASS**。唯一警告：`admin/src/utils/status.ts` `orderStatusClass` 中 `case 'completed'` 与上方第 29 行 `case 'completed'` 重复（死分支），esbuild 警告，非阻断。

### UI 残留检查

- **old 9-state UI：存在**
  - User 订单筛选下拉 `OrdersPanel.vue:294-306`（pending_payment/paid/fulfilling/partially_delivered/partially_refunded/delivered/expired/refunded，无 pending_recharge/processing/failed）；立即支付按钮绑定 `pending_payment`（`:130`、`OrderDetail.vue:72`、`GuestOrders.vue:70` 等）。
  - Admin 订单筛选下拉 `Orders.vue:320-328`（同旧9态）；`markCompleted` 绑 `delivered`、`canCreateFulfillment` 绑 `paid/fulfilling`。
  - 状态徽标映射 `user/utils/status.ts`、`admin/utils/status.ts` 同时含新旧两套。

- **guest purchase UI：存在（功能完整，未被移除）**
  - 页面：`views/GuestOrders.vue`、`views/GuestOrderDetail.vue`、`templates/vault/GuestOrders.vue`、`templates/vault/GuestOrderDetail.vue`。
  - 逻辑：`composables/useGuestOrders.ts`、`useGuestOrderDetail.ts`、`utils/guestOrderAuth.ts`、`utils/guestOrderDetailState.ts`。
  - 结账：`useCheckout.ts:273-281,545-573,726-765,832-841`（checkoutMode='guest'，guest_email/guest_password/图形或 Turnstile 验证码，`guestOrderAPI.preview/createAndPay`）。
  - 说明：访客下单是 Contract 未禁止的独立功能域，列出供决策是否保留。

- **Gateway payment UI in checkout：存在（条件渲染）**
  - 组件：`components/payment/PaymentChannelSelector.vue`、`components/payment/PaymentAmountBreakdown.vue`、`components/checkout/CheckoutManualForm.vue`。
  - 逻辑：`useCheckout.ts:86-146,443-493,669-696`（paymentChannels、requiresOnlineChannel、expectedOnlinePayCents、按渠道限额置灰）；`usePayment.ts:515+`；页面 `views/Checkout.vue`、`views/Payment.vue`、`templates/vault/Checkout.vue`。
  - 门控：`useCheckout.ts:123,588,618,878-880` 当 `appStore.config.wallet_only_payment=true` 时强制 `useBalance=true` 并阻断 online 渠道；网关 UI 代码路径仍在，是否展示取决于 `wallet_only_payment` 配置。

---

## 总结

### P0 修复清单（金额币种错标，直接违反 Contract 禁止项"用 site_config.currency 格式化钱包/退款金额"）
1. **`user/templates/vault/components/VaultOrderBody.vue:78`**：`formatMoney(order.wallet_paid_amount, order.currency)` → 改为 `order.wallet_currency || 'USDT'`；同文件 **`:86`** `refunded_amount` 同样错用 `order.currency` → 改 USDT。（vault 模板的订单金额明细块；经典 `OrderDetail.vue:362/372` 已正确，仅 vault 漏改。）
2. **`admin/views/admin/components/OrderDetailDialog.vue:844`**：`formatMoney(wallet_paid_amount, selectedOrder.currency)` → 用 USDT；**`:856`** `refunded_amount` 同改。需先在 `admin/api/types.ts:154 AdminOrder` 补 `wallet_currency?`/`usdt_total_amount?` 字段（当前类型缺失，导致无法正确取币）。
3. **`user/composables/usePayment.ts:502-507`**：`paymentWalletPaidDisplay` 用 `order.value?.currency` 格式化 `wallet_paid_amount` → 改用 USDT（`paymentResult.wallet_currency || 'USDT'`）。

### P1 修复清单（旧9态业务逻辑未迁移到新5态 / 结账模型）
1. 订单状态筛选与动作仍绑旧9态：User `OrdersPanel.vue:294-306`（筛选下拉补 pending_recharge/processing/failed，去旧态）、`:130` 立即支付按钮 `pending_payment`→`pending_recharge`；`usePayment.ts` 轮询/倒计时 `pending_payment`（389/390/631/648/663/774/858/895/1234/1291）整体迁移到新5态。
2. Admin `Orders.vue:169/181`、`OrderDetailDialog.vue:270`：`markCompleted`/`canCreateFulfillment` 由 `delivered/paid/fulfilling` 改为新5态对应处理点；`Orders.vue:320-328` 筛选下拉换5态；`Dashboard.vue:40` KPI `pending_payment_orders` 对齐后端新口径。
3. 结账分账模型：`useCheckout.ts:128-139`、`usePayment.ts:424-437` 仍把 USDT 钱包余额与站点币 total 当同币种做 cents 分账，需改为以返回的 `usdt_total_amount`/钱包 USDT 余额为准，不前端算 USDT↔Site 换算。
4. 退款卡片显隐由订单主状态 `refunded/partially_refunded`（`useOrderDisplayHelpers.ts:33-36,66-80`）改为按 `refund_status`（none/partial/full）驱动。

### P2 backlog
1. `user/utils/status.ts:11-18,35-47` 与 `admin/utils/status.ts:11-17,32-46`：清理旧9态 label/variant 死映射（或确认仅作历史单兼容后加注释）。
2. `admin/utils/status.ts` `orderStatusClass` 重复 `case 'completed'`（esbuild 警告，死分支）。
3. `admin/views/admin/AffiliateCommissions.vue:200` 佣金金额补 "USDT" 单位（User 端已正确）。
4. `admin ProcurementOrders.vue:338-349` / `ProductMappings.vue:195` 上游采购前端汇率乘法——确认属上游采购域后保留，否则改由后端返回换算值。
5. i18n locale 旧9态文案键（`user/i18n/locales/*.json`、`admin/i18n/index.ts`）死键清理。
6. guest purchase UI / checkout 网关支付 UI 是否保留，需产品决策（当前由 `wallet_only_payment` 配置门控）。

---
**Build 状态结论：两端 `vue-tsc --noEmit` 与 `npm run build` 全部真实通过（exit 0），可构建；问题集中在 Contract 币种/状态一致性，不在类型或构建层面。**
