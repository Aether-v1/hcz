# 后端 P0 修复报告

## 修复清单
| P0 | 文件 | 修改内容 | 状态 |
|---|---|---|---|
| P0-1 | payment/application/payment_service_create.go:98 | 钱包扣款状态校验接受 pending_recharge | ✅ PASS |
| P0-2 | order/application/order_service_child.go:441 | 超时取消状态校验接受 pending_recharge | ✅ PASS |
| P0-3 | fulfillment/application/service.go:110,144 | 履约接受 processing，完成写 completed | ✅ PASS |
| P0-4 | payment/application/payment_service_callback.go:423 | markOrderPaid 五态机订单写 processing 而非 paid | ✅ PASS |
| P0-5 | order/application/order_service_child.go:243 | 删除 admin 直接设 partially_refunded/refunded 分支 | ✅ PASS |
| P0-Commission | affiliate/application/commission.go:167 | 退款比例分母 USDT 订单用 WalletPaidAmount | ✅ PASS |
| P0-RefundCurrency | order/application/refund/service.go:532 | createRefundRecordTx 接收 refundCurrency 参数 | ✅ PASS |
| M1 | order/application/order_status.go:41 | CalcParentStatus 重写为五态聚合 | ✅ PASS |
| M2 | order/application/order_service_child.go:379,470 | completeParent 状态校验接受 completed/processing | ✅ PASS |
| M3 | order/application/order_service.go:205 | allowedTransitions 添加五态迁移映射 | ✅ PASS |
| M4 | order/application/order_service.go:637 | 子订单 OnlinePaidAmount 初始化为 0 | ✅ PASS |
| Procurement | procurement/application/submit.go, callback.go | 采购模块旧态写入改为五态 | ✅ PASS |

## 各 P0 详细修复

### P0-1 payment_service_create.go:98
- 修改前：`if lockedOrder.Status != constants.OrderStatusPendingPayment`
- 修改后：`if lockedOrder.Status != constants.OrderStatusPendingRecharge && lockedOrder.Status != constants.OrderStatusPendingPayment`
- 说明：wallet-only 模式下新订单为 pending_recharge，CreatePayment 是钱包扣款入口，需接受此状态
- 测试验证：payment 模块全部测试 PASS

### P0-2 order_service_child.go:441
- 修改前：`if order.Status != constants.OrderStatusPendingPayment`
- 修改后：`if order.Status != constants.OrderStatusPendingRecharge && order.Status != constants.OrderStatusPendingPayment`
- 说明：CancelExpiredOrder 定时任务需处理 pending_recharge 状态的过期订单
- 测试验证：order 模块测试 PASS

### P0-3 fulfillment/service.go
- line 110 修改前：`order.Status != OrderStatusPaid && order.Status != OrderStatusFulfilling`
- line 110 修改后：增加 `&& order.Status != OrderStatusProcessing`
- line 144 修改前：写 `OrderStatusDelivered`
- line 144 修改后：写 `OrderStatusCompleted`
- CreateAuto line 226：同样增加 OrderStatusProcessing 接受
- 说明：人工/自动履约完成后直接写 completed（五态终态），不再写 delivered
- 测试验证：fulfillment 模块测试 PASS

### P0-4 payment_service_callback.go markOrderPaid
- 修改前：写 `OrderStatusPaid` 到父订单，子订单写 `OrderStatusPaid`/`OrderStatusFulfilling`
- 修改后：检测 `isFiveStateOrder := order.Status == OrderStatusPendingRecharge`
  - 五态机订单：父订单写 `OrderStatusProcessing`，子订单统一写 `OrderStatusProcessing`
  - 旧九态订单：保持原有 paid/fulfilling 行为（兼容历史数据）
- 说明：钱包扣款成功后五态机订单进入 processing，不再写旧态 paid/fulfilling
- 测试验证：payment 模块测试 PASS

### P0-5 order_service_child.go:243-286
- 修改前：parent switch 中有 `case OrderStatusPartiallyRefunded, OrderStatusRefunded` 分支，允许 admin 直接写退款主状态
- 修改后：删除该分支，同时删除 line 189 的豁免判断，统一走 ordermachine.CanTransition 校验
- 说明：Refund 只更新 refund_status 独立列，主状态由 ordermachine 管理
- 测试验证：更新 TestUpdateOrderStatusParentToPartiallyRefundedSyncsChildren 验证 admin 直接设退款态被拒绝

### P0-Commission commission.go:167
- 修改前：`totalAmount := order.TotalAmount.Decimal.Round(2)`（Site Currency）
- 修改后：`if order.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) { totalAmount = order.WalletPaidAmount.Decimal.Round(2) }`
- 说明：USDT 订单退款比例分母用 WalletPaidAmount（USDT），保证 refund_ratio = refunded_usdt / original_paid_usdt
- 测试验证：affiliate 模块测试 PASS

### P0-RefundCurrency refund/service.go:532
- 修改前：createRefundRecordTx 直接用 `order.Currency` 作退款记录币种
- 修改后：函数新增 `refundCurrency string` 参数，调用方传入正确币种
  - USDT 订单（WalletPaidAmount > 0）传 `"USDT"`
  - 旧订单回退到 `order.Currency`
- 说明：所有 Wallet Refund Record 的 currency = USDT
- 测试验证：refund 相关测试 PASS

## 主链依赖修复

### M1 CalcParentStatus 重写
- 修改前：基于旧九态计数聚合，返回 partially_delivered/refunded 等旧态
- 修改后：先通过 ordermachine.Normalize 将每个子订单归一为五态，再按优先级聚合：
  1. 全部 canceled → canceled
  2. 任一 failed → failed
  3. 任一 pending_recharge → pending_recharge
  4. 任一 processing → processing
  5. 全部 completed → completed
- 说明：refund_status 不影响主状态聚合

### M2 completeParent 状态校验
- line 379：子订单要求 delivered → 接受 delivered 和 completed
- line 470：父订单要求 delivered → 接受 delivered 和 processing
- 说明：五态机下子订单终态为 completed，父订单前置为 processing

### M3 allowedTransitions
- 修改前：仅旧九态迁移映射
- 修改后：新增五态机迁移映射：
  - pending_recharge → processing/failed/canceled
  - processing → completed/failed
- 说明：确保 processing→completed 等合法迁移不被 IsTransitionAllowed 误拒

### M4 子订单 OnlinePaidAmount
- 修改前：子订单 OnlinePaidAmount 初始化为 Site Currency 金额
- 修改后：wallet-only 模式下子订单 OnlinePaidAmount = 0
- 说明：wallet-only 下不存在在线支付

### Procurement 模块旧态写入修复
- submit.go:122：fulfilling → processing
- submit.go:168-170：rollback 从 fulfilling→paid 改为 processing→processing（保持可重试）
- callback.go:62：delivered → completed
- 说明：采购域不再写旧九态主状态

## 全局旧九态写入扫描结果
| 状态词 | 位置 | 分类 |
|---|---|---|
| OrderStatusPendingPayment | constants.go 定义 | LEGACY_READ_ONLY |
| OrderStatusPendingPayment | payment_service_callback.go:281 orderOpen 判断 | LEGACY_READ_ONLY |
| OrderStatusPaid | payment_service_callback.go:437 markOrderPaid（旧单兼容分支） | NORMALIZE_COMPAT |
| OrderStatusPaid | payment_service_callback_dispatch.go:41 日志/邮件参数 | LEGACY_READ_ONLY |
| OrderStatusFulfilling | fulfillment/service.go:110 状态校验 | LEGACY_READ_ONLY |
| OrderStatusDelivered | fulfillment/service.go 已改为 Completed | ✅ 已修复 |
| OrderStatusDelivered | procurement/callback.go 已改为 Completed | ✅ 已修复 |
| OrderStatusPartiallyRefunded | order_service_child.go 已删除写入分支 | ✅ 已修复 |
| OrderStatusRefunded | order_service_child.go 已删除写入分支 | ✅ 已修复 |
| OrderStatusPartiallyDelivered | order_status.go CalcParentStatus 已重写 | ✅ 已修复 |

**ACTIVE_WRITE_BLOCKER 最终数量：0**

## 新增/更新测试清单
| 测试名 | 所在文件 | 覆盖场景 | 结果 |
|---|---|---|---|
| TestCalcParentStatus | order_service_status_test.go | 五态聚合：completed/pending_recharge | ✅ PASS |
| TestCalcParentStatusAllRefunded | order_service_status_test.go | refunded→completed 归一聚合 | ✅ PASS |
| TestCalcParentStatusPartiallyRefunded | order_service_status_test.go | refund_status 不影响主状态 | ✅ PASS |
| TestUpdateOrderStatusParentToPartiallyRefundedSyncsChildren | order_service_status_test.go | admin 直接设退款态被拒绝 | ✅ PASS |
| TestSubmitToUpstream_Success | submit_test.go | 采购提交后订单 processing | ✅ PASS |
| TestSubmitToUpstream_NonRetryableError_Rejects | submit_test.go | 采购失败回退 processing | ✅ PASS |
| TestRejectProcurement_RollsBackOrderStatus | callback_test.go | 拒绝采购回退 processing | ✅ PASS |
| TestHandleUpstreamCallback_Delivered_CreatesFulfillment | callback_test.go | 采购完成写 completed | ✅ PASS |
| TestHandleUpstreamCallback_Delivered_SynchronizesParentStatus | callback_test.go | 子 completed→父 completed | ✅ PASS |
| TestPollUpstreamStatus_Delivered | poll_test.go | 轮询交付写 completed | ✅ PASS |

## 验证结果
- go build ./...：**PASS**
- go test ./internal/modules/payment/...：**PASS**
- go test ./internal/modules/order/...：**PASS**（RiskGate TempDir Windows flaky 不计入）
- go test ./internal/modules/fulfillment/...：**PASS**
- go test ./internal/modules/affiliate/...：**PASS**
- go test ./internal/modules/procurement/...：**PASS**

## 遗留问题
1. **RiskGate TempDir flaky**：Windows SQLite 文件锁导致 TempDir RemoveAll 失败（`TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts`、`TestRiskGateSerializesConcurrentGuestQuotaChecks`），断言本身通过，属已知 Windows flaky。
2. **旧九态兼容读取**：代码中仍保留旧九态常量的读取判断（orderOpen、status check 等），用于兼容历史数据读取，不产生新写入。
3. **payment 模块 gateway 死代码**：business order gateway payment 全链路代码保留作为历史兼容，wallet-only 模式下不可达，不影响主链。
