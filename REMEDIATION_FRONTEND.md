# 前端 P0 修复报告

> 范围：frontend/user、frontend/admin（Vue 3 + TS + vite）
> 目标：修复 USDT 金额（wallet_paid_amount / refunded_amount / 钱包余额）被误按站点币（site_config.currency / order.currency）格式化的问题。
> 原则：最小改动，只改币种格式化参数，不重构组件、不动已冻结 Contract。

## 修复清单

| P0 | 文件 | 修改内容 | 状态 |
|---|---|---|---|
| P0-6 | frontend/user/src/templates/vault/components/VaultOrderBody.vue | line 78 wallet_paid_amount、line 86 refunded_amount 改用 `order.wallet_currency \|\| 'USDT'` | ✅ 已修复 |
| P0-7a | frontend/admin/src/api/types.ts | AdminOrder 增加 `usdt_total_amount?: number`、`wallet_currency?: string` | ✅ 已修复 |
| P0-7b | frontend/admin/src/views/admin/components/OrderDetailDialog.vue | line 844 wallet_paid_amount、line 856 refunded_amount、line 995 refunded_amount 汇总条 改用 `selectedOrder.wallet_currency \|\| 'USDT'` | ✅ 已修复 |
| P0-8a | frontend/user/src/composables/usePayment.ts | paymentWalletPaidDisplay（line ~506）改用 `paymentResult.wallet_currency \|\| order.wallet_currency \|\| 'USDT'` | ✅ 已修复 |
| P0-8b | frontend/user/src/composables/usePayment.ts | walletBalanceDisplay（line ~423）、expectedWalletPaidDisplay（line ~436）改用 `'USDT'` | ✅ 已修复 |

## 后端字段核对（修复依据）

读取 `internal/modules/order/transport/presenter/order.go` 与 `internal/modules/order/domain/order.go`：

- `total_amount` / `currency` = **站点币**（site_config.currency）。
- `wallet_paid_amount` = **USDT**（domain 注释「钱包支付金额（USDT 实扣）」）。
- `refunded_amount` = **USDT**（domain 注释「已退款金额（退回钱包）」）。
- `online_paid_amount` = 在线支付金额，按**站点币**展示（与参考实现 OrderDetail.vue:367 一致，未改动）。
- `usdt_total_amount` = USDT 应收（domain Order 直接序列化，admin 原始 DTO 会下发）。
- `wallet_currency`：user 侧 presenter `walletCurrencyForOrder()` 计算返回 `"USDT"` 或 `""`；admin 侧 `AdminOrderDetail` 内嵌原始 `orderdomain.Order`，**不下发** `wallet_currency`（该字段为 presenter 计算值）。因此前端统一用 `wallet_currency || 'USDT'` 回退：admin 下 wallet_currency 为 undefined 时回退 `'USDT'`，语义正确（钱包本位币恒为 USDT，见 `exchangerate/domain/rate.go: WalletCurrency = "USDT"`、`wallet presenter: WalletCurrency:"USDT"`）。

参考正确实现：`frontend/user/src/views/OrderDetail.vue:362-373`（wallet_paid_amount / refunded_amount 均用 `order.wallet_currency || 'USDT'`，online_paid_amount 用 `order.currency`）。

## 各 P0 详细修复

### P0-6 VaultOrderBody.vue

- 修改前 line 78：
  `{{ formatMoney(order.wallet_paid_amount, order.currency) }}`
- 修改后 line 78：
  `{{ formatMoney(order.wallet_paid_amount, order.wallet_currency || 'USDT') }}`
- 修改前 line 86：
  `{{ formatMoney(order.refunded_amount, order.currency) }}`
- 修改后 line 86：
  `{{ formatMoney(order.refunded_amount, order.wallet_currency || 'USDT') }}`
- 未改动：
  - line 82 `online_paid_amount` 仍用 `order.currency`（在线支付=站点币，正确）。
  - line 111 退款记录 `record.amount` 用 `record.currency || order.currency`（退款记录自带 currency 字段，正确）。
- 说明：`order` prop 类型为 `any`，无类型错误风险。

### P0-7 Admin OrderDetailDialog.vue + types.ts

types.ts（AdminOrder，refunded_amount 之后新增）：
```ts
  refunded_amount: number
  // USDT 结算字段：wallet_paid_amount/refunded_amount 本位币为 USDT。
  usdt_total_amount?: number
  wallet_currency?: string
```

OrderDetailDialog.vue：
- 修改前 line 844：`formatMoney(selectedOrder.wallet_paid_amount, selectedOrder.currency)`
- 修改后 line 844：`formatMoney(selectedOrder.wallet_paid_amount, selectedOrder.wallet_currency || 'USDT')`
- 修改前 line 856：`formatMoney(selectedOrder.refunded_amount, selectedOrder.currency)`
- 修改后 line 856：`formatMoney(selectedOrder.refunded_amount, selectedOrder.wallet_currency || 'USDT')`
- 修改前 line 995（退款汇总条，同一原始字段的第二次展示）：`formatMoney(selectedOrder.refunded_amount, selectedOrder.currency)`
- 修改后 line 995：`formatMoney(selectedOrder.refunded_amount, selectedOrder.wallet_currency || 'USDT')`
- 未改动：
  - line 850 `online_paid_amount` 仍用 `selectedOrder.currency`（正确）。
  - 商品单价/成本/各项折扣（line 962-975、1055-1065）均站点币，正确。
  - line 984/1074 `itemRefundAmount()`、line 481 `refundableAmountDisplay`、line 987/1001 利润行：属 admin 内部「按站点币收入权重分摊 USDT 退款 + 利润」的跨币种会计视图，其数值本身就是混合币种运算结果。按「最小改动、不重构」原则，仅修正原始字段展示的币种标注，不改派生分摊算法（见遗留问题）。

### P0-8 usePayment.ts

- paymentWalletPaidDisplay（原 line 506 用 `order.value?.currency`）：
```ts
// 修改前
return formatMoney(String(paymentResult.value.wallet_paid_amount), order.value?.currency)
// 修改后
const walletCcy = String(paymentResult.value?.wallet_currency || order.value?.wallet_currency || 'USDT')
return formatMoney(String(paymentResult.value.wallet_paid_amount), walletCcy)
```
- walletBalanceDisplay（原 line 423）：钱包余额来自 `walletAPI.account()`，本位币恒为 USDT
```ts
// 修改前
const walletBalanceDisplay = computed(() => formatMoney(walletBalance.value, order.value?.currency))
// 修改后
const walletBalanceDisplay = computed(() => formatMoney(walletBalance.value, 'USDT'))
```
- expectedWalletPaidDisplay（原 line 436）：钱包实扣预期，USDT
```ts
// 修改前
const expectedWalletPaidDisplay = computed(() => formatMoney(centsToAmount(expectedWalletPaidCents.value), order.value?.currency))
// 修改后
const expectedWalletPaidDisplay = computed(() => formatMoney(centsToAmount(expectedWalletPaidCents.value), 'USDT'))
```
- 未改动：expectedOnlinePayDisplay（line 437，在线支付部分=站点币，仍用 `order.value?.currency`，正确）；payableAmountDisplay / fee_amount 逻辑（已有正确注释，结算币种用 paymentResult.currency、手续费仍用 order.currency）。

## 全局搜索确认

- 搜索 `formatMoney(...)` 且第二参数绑定 `.currency`、作用于 `wallet_paid_amount` / `refunded_amount`：
  - 修复后 user 侧（vault 模板、订单详情、支付结果页）**0 处**用站点币格式化 USDT 金额。
  - admin 订单详情（OrderDetailDialog）原始 USDT 字段 **0 处**用站点币。
  - 仅余 `admin/ProcurementOrders.vue:764`（采购对账域，见遗留问题）。
- `wallet_paid_amount` 使用点（12 文件）：
  - user：types.ts（类型）、PaymentAmountBreakdown.vue（仅透传 `walletPaidDisplay` prop，不自算币种）、VaultOrderBody.vue（已修）、OrdersPanel.vue:489-490（已正确：`order.wallet_currency || 'USDT'` + `wallet_paid_amount ?? usdt_total_amount`）、OrderDetail.vue（参考正确）、usePayment.ts（已修）、useOrderDisplayHelpers.ts（纯 formatMoney 工具，币种由调用方传入）。
  - admin：types.ts（已补字段）、OrderDetailDialog.vue（已修）、OrderRefunds.vue / OrderRefundsDialog.vue（退款记录用记录自带 `currency`，正确）、ProcurementOrders.vue（见遗留）。
- `refunded_amount` 使用点：均已核对，原始字段展示已改 `wallet_currency || 'USDT'`；退款记录列表用记录自带 `currency`。
- `commission_amount` 使用点：
  - admin `AffiliateCommissions.vue:200` 仅渲染纯数字 `{{ item.commission_amount || '0.00' }}`，未套用 `.currency`。
  - user `AffiliatePanel.vue:134` 已正确 `{{ item.currency || 'USDT' }}`。
  - 结论：无站点币格式化问题。

## 构建验证

（均在 Windows PowerShell 下真实执行）

| 步骤 | 结果 | 说明 |
|---|---|---|
| `cd frontend/user; npx vue-tsc --noEmit` | **PASS**（0 错误，exit 0） | |
| `cd frontend/user; npm run build` | **PASS**（exit 0） | `vue-tsc -b && vite build`；vite 构建 16.08s，整命令约 34.3s，2988 modules transformed |
| `cd frontend/admin; npx vue-tsc --noEmit` | **PASS**（0 错误，exit 0） | |
| `cd frontend/admin; npm run build` | **PASS**（exit 0） | `vue-tsc -b && vite build`；vite 构建 23.37s，整命令约 44.2s，2890 modules transformed |

> admin build 期间 esbuild 向 stderr 打印 `src/utils/status.ts` 一个「duplicate case clause」提示（PowerShell 以 NativeCommandError 呈现）。该文件本次未触碰，属既有告警，不影响构建产物，exit code = 0。

## 遗留问题（如有）

1. **admin 利润/退款分摊视图的跨币种运算**（OrderDetailDialog.vue：`buildRefundAllocation` / `itemRefundAmount` / `itemProfit` / `orderProfit` / `refundableAmountValue`，line 147-213、471-481、模板 984/987/995/1001/1074/1077）：退款按站点币收入权重把 USDT 退款额分摊到订单项，再与站点币成本/收入相减求利润，本质是跨币种混合运算。本次仅修正「原始 USDT 字段」展示的币种标注，未动分摊算法。如需正确的利润/可退金额，应以后端结算口径为准，另立任务处理，不宜在前端临时换算（Contract 禁止前端自行算汇率）。
2. **usePayment.ts `expectedWalletPaidCents`**（line 424-430）：`Math.min(钱包USDT余额cents, 订单站点币total cents)` 直接比较两种币别的 cents，是既有跨币种逻辑。本次仅把展示币种标注改为 USDT，未改比较逻辑；正确做法应由后端返回「本单钱包可抵扣 USDT 额」，另立任务。
3. **admin/ProcurementOrders.vue:764** `formatMoney(detailOrder.local_order?.refunded_amount, detailOrder.currency)`：采购/对账域，`local_order` 为内嵌订单快照、`detailOrder.currency` 为该页模型币种（与订单详情币种模型不同）。不在本次 P0-6/7/8 明确范围内，且其 local_order 退款额币种语义需结合采购对账 DTO 确认，未贸然改动。建议后续核对 `AdminProcurementOrder.local_order` 的 refunded_amount 币种口径后统一。
4. admin 原始 Order DTO 不下发 `wallet_currency`：目前靠 `|| 'USDT'` 回退，语义正确。若日后 admin 需要展示真实钱包币种（恒为 USDT），可在 `admin_handler.go AdminOrderDetail` 显式补 `wallet_currency` 字段，属后端增强，非前端缺陷。
