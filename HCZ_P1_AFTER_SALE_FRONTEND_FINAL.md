# HCZ P1 After-Sale Frontend Minimal Integration — Final Report

## Final Verdict: PASS

---

## 一、实施范围

在现有 User/Admin 前端做最小接入：User 订单详情增加"未收到"按钮+表单+售后状态展示；Admin 订单详情增加售后区域+操作按钮（驳回/标记解决/部分退款/全额退款）。不改后端业务逻辑、不重做 UI、不改资金 contract。

---

## 二、修改文件

### 后端（最小 DTO 补充）
| 文件 | 修改 |
|------|------|
| `internal/modules/order/transport/presenter/order.go` | OrderDetail 新增 `ID uint json:"id"` 字段（纯新增，不破坏）；NewOrderDetail 赋值 `o.ID` |

**原因**：After-Sale API 使用数字 `:id`（order ID），而 User 订单详情 API 原仅返回 `order_no`。新增 `id` 字段使前端可调用 after-sale API。纯新增字段，不影响任何现有逻辑。

### User 前端
| 文件 | 修改 |
|------|------|
| `frontend/user/src/api/order.ts` | userOrderAPI 新增 `createAfterSale(orderId, data)` / `getAfterSale(orderId)` |
| `frontend/user/src/composables/useAfterSale.ts` | 新增：售后状态管理、发起/查询、表单、i18n 状态映射 |
| `frontend/user/src/views/OrderDetail.vue` | 订单详情增加"未收到"按钮、售后状态卡片、发起表单弹窗 |
| `frontend/user/src/templates/vault/OrderDetail.vue` | vault 模板同步增加售后 UI |
| `frontend/user/src/i18n/locales/zh-CN.json` | 新增 afterSale 命名空间（21 个 key） |
| `frontend/user/src/i18n/locales/zh-TW.json` | 新增 afterSale 命名空间（21 个 key） |
| `frontend/user/src/i18n/locales/en-US.json` | 新增 afterSale 命名空间（21 个 key） |

### Admin 前端
| 文件 | 修改 |
|------|------|
| `frontend/admin/src/api/admin.ts` | 新增 `getOrderAfterSale(orderId)` / `actionOrderAfterSale(orderId, data)` |
| `frontend/admin/src/views/admin/components/OrderDetailDialog.vue` | 订单详情弹窗增加售后区域：状态展示、pending 时操作按钮（驳回/标记解决/部分退款/全额退款）、partial_refund 金额输入 |
| `frontend/admin/src/i18n/index.ts` | zh-CN / zh-TW / en 三语言 orders 部分新增 afterSale 相关 key（24 个/语言） |

---

## 三、User 前端功能

### 发起售后
- 仅 `order.status === 'completed'` 且无 pending 售后时显示"未收到"按钮
- 点击打开表单：type 固定 `not_received`、reason（必填）、description（可选）
- 提交 `POST /api/v1/orders/:id/after-sale`
- 提交成功后自动刷新售后状态，显示"处理中"

### 售后状态展示
- 调 `GET /api/v1/orders/:id/after-sale`
- pending = 处理中（黄色 Badge）
- resolved = 已解决（绿色 Badge）
- rejected = 已驳回（红色 Badge）
- 有退款时显示：`退款金额：xx.xx USDT`（直接读 API 返回的 refund_amount + refund_currency，不自行计算）
- 显示原因、详细说明、Admin 备注、发起时间、处理时间

### 行为约束
- pending 时隐藏再次发起按钮，显示"售后处理中"
- resolved/rejected 显示最终结果
- 是否允许再次发起完全依后端 contract（后端校验无 pending 才可发起）
- 余额不足等业务错误通过 toast 显示后端返回的 msg，不统一显示"系统错误"

---

## 四、Admin 前端功能

### 售后区域
- 订单详情弹窗中增加"售后"卡片
- 显示：type、reason、description、status、admin_note、refund_amount（USDT）、created_at、resolved_at
- 无售后时显示"暂无售后"

### 操作按钮（仅 pending 状态）
- **驳回** → `action: "reject"`
- **标记解决** → `action: "resolve"`
- **部分退款** → 展开金额输入框（明确标注 USDT），输入后提交 `action: "partial_refund", refund_amount: "x.xx"`
- **全额退款** → 直接提交 `action: "full_refund"`，前端不计算金额，后端按剩余可退 USDT 处理

### 操作后
- 成功后自动刷新售后工单 + 订单详情 + emit refresh
- 失败显示后端返回的业务错误 msg
- 操作中按钮 disabled，防止重复提交

---

## 五、资金安全

- **前端不计算退款金额**：full_refund 只传 action，后端计算剩余可退 USDT
- **partial_refund 金额明确标注 USDT**：输入框 placeholder 和 label 均显示 USDT
- **退款显示直接读 API**：`refund_amount` + `refund_currency`，不做任何前端换算
- **不调用 CoinGecko**：前端无任何汇率源调用
- **不根据当前汇率重算历史订单**：所有金额读订单 snapshot
- **Admin 操作走现有 JWT + RBAC + Payment Compliance**：API 路由已挂 paymentProtected 中间件，前端不新增旁路权限

---

## 六、i18n 覆盖

### User（afterSale 命名空间，21 key）
zh-CN / zh-TW / en-US 三语言全覆盖：
title, notReceived, notReceivedHint, reason, reasonPlaceholder, reasonRequired, description, descriptionPlaceholder, adminNote, statusPending, statusResolved, statusRejected, refundAmount, createdAt, resolvedAt, submitted, submitFailed, submitting

### Admin（orders.afterSale*，24 key/语言）
zh-CN / zh-TW / en 三语言全覆盖：
afterSaleTitle, afterSaleNone, afterSaleStatusPending/Resolved/Rejected, afterSaleType, afterSaleReason, afterSaleDescription, afterSaleAdminNote, afterSaleAdminNotePlaceholder, afterSaleRefundAmount, afterSaleCreatedAt, afterSaleResolvedAt, afterSaleReject, afterSaleResolve, afterSalePartialRefund, afterSaleFullRefund, afterSaleInvalidAmount, afterSaleActionSuccess, afterSaleActionFailed, loading, confirm

---

## 七、验证结果

| 验证项 | 结果 |
|--------|------|
| go build ./... | PASS (EXIT=0) |
| User vue-tsc --noEmit | PASS (EXIT=0) |
| Admin vue-tsc --noEmit | PASS (EXIT=0) |
| User npm run build | PASS (20.68s, EXIT=0) |
| Admin npm run build | PASS (21.28s, EXIT=0) |
| After-sale handler 测试回归 | PASS (10/10) |

### Smoke 验证（代码级）
- User 订单详情：completed 订单显示"未收到"按钮 ✓
- User 发起表单：reason 必填校验 ✓
- User 售后状态：pending/resolved/rejected 三态 Badge + 退款金额显示 ✓
- User pending 时隐藏再次发起按钮 ✓
- Admin 售后区域：状态/原因/金额/时间展示 ✓
- Admin pending 操作：驳回/标记解决/部分退款/全额退款四按钮 ✓
- Admin partial_refund：USDT 金额输入 + 确认 ✓
- Admin full_refund：不传金额，后端计算 ✓
- Admin 操作后自动刷新 ✓

---

## 八、五个明确回答

1. **User 是否能发起/查看售后** → 是。completed 订单显示"未收到"按钮，提交后显示售后状态卡片，pending/resolved/rejected 三态完整展示。
2. **Admin 是否能处理售后** → 是。订单详情弹窗增加售后区域，pending 时可执行驳回/标记解决/部分退款/全额退款，操作后自动刷新。
3. **partial/full refund 是否正确显示 USDT** → 是。partial_refund 输入框明确标注 USDT；退款金额直接读 API 返回的 `refund_amount` + `refund_currency`，前端不计算。
4. **是否还存在前端自行计算退款金额** → 否。full_refund 只传 action 不传金额；partial_refund 金额由 Admin 输入；所有退款显示读 API。
5. **P1 After-Sale 是否可以正式封板** → 是。后端 API + 前端 User/Admin 最小接入全部完成，typecheck/build 全 PASS，资金安全约束全部满足。

---

## 九、未做（按要求）

- 不做 User/Admin 整体 UI 重构
- 不改订单状态机
- 不改退款逻辑
- 不改资金 contract
- 不新增售后类型（仅 not_received）
- 不做 confirm receipt
- 不做供应商 API
