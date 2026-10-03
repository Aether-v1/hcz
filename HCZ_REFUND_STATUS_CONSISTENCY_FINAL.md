# HCZ Refund Status Consistency Closure — Final Report

## Final Verdict: PASS

---

## 一、问题定位

审计确认 canceled / failed 自动退款均走 `ReleaseWalletBalance`（`order_wallet_bridge.go`），全额退 USDT + 写 Wallet Ledger，但**不写 `refund_status` / `refunded_amount`**。导致：
- canceled / failed 订单 DB 中 `refund_status` 仍为 `none`
- 前端/Admin 无法正确显示退款状态
- parent/child 子单 refund_status 与父单不一致

同时发现 P0-3 接线遗留 gap：`UpdateOrderStatus` 非父单路径存在**双重状态校验**——line 190 已用 `ordermachine.Normalize + CanTransition`（新五态），但 line 291 又用旧 `IsTransitionAllowed`（旧 9 态 `allowedTransitions` map），该 map 不含 `pending_recharge` / `processing` / `failed`，导致 `pending_recharge → failed` 被误拒为 "order status invalid"。

---

## 二、修改清单

### 1. `internal/modules/order/application/order_wallet_bridge.go`
- `ReleaseWalletBalance` 的 `UpdateFieldsWhereWalletPaid` 回调中补写：
  - `refund_status = constants.OrderRefundStatusFull`
  - `refunded_amount = order.WalletPaidAmount`（原 USDT 快照）
- 利用 `UpdateFieldsWhereWalletPaid` 仅当 `wallet_paid_amount > 0` 时生效的守卫，**天然幂等**：重复请求不会双退、不会重复推进 refund_status。
- 新增 `constants` import。

### 2. `internal/modules/order/application/order_service_child.go` — cancelOrderWithChildren
- 当父单 `WalletPaidAmount > 0` 时，status update 的 `updates` map 增加 `refund_status = full`。
- 作用于**父单 + 所有子单**，保证 parent/child refund_status 一致。
- 未支付订单（wallet_paid_amount=0）不写 refund_status，保持 `none`。

### 3. `internal/modules/order/application/order_service_child.go` — cancelSingleOrderInTx
- Admin 单订单 canceled / failed 路径，同样条件增加 `refund_status = full`。
- 新增 `decimal` import。

### 4. `internal/modules/order/application/order_service_child.go` — 删除冗余旧校验
- 删除 `UpdateOrderStatus` 非父单路径的 `if !IsTransitionAllowed(order.Status, target)` 检查。
- 理由：line 189-193 已用 `ordermachine.Normalize + CanTransition` 统一校验所有非 legacy 目标；旧 `IsTransitionAllowed`（9 态 map）不含新五态，会误拒 `pending_recharge → failed` 等合法迁移。
- `IsTransitionAllowed` 函数本身**保留**，仍被 legacy parent/child refund 同步（line 255）和 payment callback（`payment_service_callback.go:427`）使用。

### 5. 新增 `internal/modules/order/application/order_refund_status_consistency_test.go`
4 个集成测试（真实 SQLite + Wallet Service + Order Service）：

| 测试 | 断言 |
|------|------|
| `TestCanceledOrderSetsRefundStatusFull` | status=canceled + refund_status=full + refunded_amount=10.00 USDT + wallet +10.00 |
| `TestFailedOrderSetsRefundStatusFull` | status=failed + refund_status=full + refunded_amount=10.00 USDT + wallet +10.00 |
| `TestCanceledOrderIdempotentNoDoubleRefund` | 第二次 cancel 被状态机拒绝，wallet 只 credit 一次 |
| `TestUnpaidCanceledKeepsRefundStatusNone` | 未支付订单 canceled 后 refund_status 保持 none |

---

## 三、测试结果

### 定向测试
```
=== RUN   TestCanceledOrderSetsRefundStatusFull      --- PASS (0.04s)
=== RUN   TestFailedOrderSetsRefundStatusFull        --- PASS (0.01s)
=== RUN   TestCanceledOrderIdempotentNoDoubleRefund  --- PASS (0.01s)
=== RUN   TestUnpaidCanceledKeepsRefundStatusNone    --- PASS (0.01s)
ok  	github.com/Aether-v1/hcz/internal/modules/order/application	6.119s
```

### 回归
| 包 | 结果 |
|----|------|
| `order/application`（含 aftersale + ordermachine） | PASS |
| `order/integrationtest/aftersale` | PASS |
| `order/integrationtest/application` | PASS |
| `order/integrationtest/refund` | PASS |
| `wallet/integrationtest` + `wallet/transport/*` | PASS |
| `affiliate/infrastructure` + `integrationtest` + `presenter` | PASS |
| `go build ./...` | PASS (exit 0) |

---

## 四、四个明确回答

### 1. canceled refund_status 是否正确？
**是。** User Cancel / Admin canceled 后：
- `orders.status = canceled`
- `orders.refund_status = full`
- `orders.refunded_amount = 原 wallet_paid_amount`（USDT）
- Wallet balance 退回对应 USDT
- Wallet Ledger 新增 refund credit
- 重复请求幂等（`UpdateFieldsWhereWalletPaid` 守卫）

### 2. failed 自动退款是否完整？
**是。** 修复了 P0-3 遗留的旧 `IsTransitionAllowed` 误拒问题后：
- `pending_recharge → failed` 合法通过状态机
- 自动调用 `ReleaseWalletBalance` 全额退 USDT
- `refund_status = full` + `refunded_amount` 正确
- Wallet credit + Ledger 正确
- Commission rollback 走现有 `HandleOrderCanceled` 链路

### 3. parent/child refund_status 是否一致？
**是。** `cancelOrderWithChildren` 中，当父单已扣 USDT 时，父单和所有子单的 status update 同步写入 `refund_status = full`。不存在父单已退款但子单 refund_status 仍为 none 的情况。

### 4. 是否可以安全进入 After-Sale Handler + Route？
**是。** canceled / failed / completed 三种主状态的 refund_status 一致性已闭环：
- canceled → full（自动退款）
- failed → full（自动退款）
- completed + 售后 partial/full refund → partial/full（aftersale 链路，已在上一轮验证）
- 未支付 canceled → none
- 重复请求幂等
- 全量回归无新增失败

---

## 五、未修改项（遵守约束）

- 五主状态定义：未改
- USDT 金额模型 / 汇率：未改
- AfterSale 状态模型：未改
- Gateway / Payment：未改
- API Contract：未改
- 未引入 frozen balance
- 未做破坏性 DB migration

---

## 六、剩余 P1/P2

- After-Sale Handler + Route + DTO（后端 HTTP 层）
- After-Sale User/Admin 前端最小接入
- Admin 售后列表/筛选
- 售后并发真实 DB 证明（Linux/PostgreSQL FOR UPDATE，当前 SQLite 架构验证已通过）
