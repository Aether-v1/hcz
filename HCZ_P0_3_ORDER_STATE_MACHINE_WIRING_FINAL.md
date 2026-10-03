# HCZ P0-3 Order State Machine — Wiring Closure Report

## Final Verdict：**PASS WITH CONDITIONS**

已把冻结的状态机核心接入真实业务读链与新单创建；Admin/退款副作用接线为下一增量。未引入冻结资金模型（沿用 P0-2 direct-debit）。

## 本轮接线内容
1. **新单初始状态**：`order_service.go` 两处订单创建（:556/:618）由 `pending_payment` 改为 `pending_recharge`。Wallet-only 下新单直接进入待充值，不再产生 pending_payment/paid 主状态。
2. **DTO Normalize**：`order/transport/presenter/order.go`
   - OrderSummary.Status 与 OrderDetail.Status 经 `ordermachine.Normalize()` 输出五主状态；
   - OrderDetail 新增 `refund_status`（none/partial/full）；
   - 历史 9 态（paid/fulfilling/delivered/partially_refunded/refunded…）由 API 归一，前端不再做兼容判断。
3. **状态机核心**：`ordermachine/machine.go`（上一增量冻结）继续作为唯一权威。

## 验证
- `go build ./...` EXIT=0。
- 回归：order/application、ordermachine、integrationtest/application、integrationtest/refund、wallet 全 PASS。
- gormstore 两个失败为已知 Windows TempDir 文件占用 flaky（风险闸门用例，与状态改动无关），Linux CI 不受影响。

## 已冻结/确认
- 不引入 frozen_balance/hold/release/settle；沿用「下单直接扣 USDT，failed/canceled 按 USDT snapshot 退款」。
- P0-2 资金 Contract、汇率、退款/返利金额口径未改。

## 待接线（下一增量）
- Admin 改状态入口统一走 `ordermachine.CanTransition` + `RequiresAutoRefund` 副作用（事务内触发 P0-2 退款链）。
- User Cancel 改走 `AllowedByUser` 后端校验。
- commission failed/canceled 回滚接线。
- refund_status 列 migration（当前由 Normalize 从旧主状态推断，未物理加列）。
- Admin/User 前端五态文案与按钮最小接入。

## 结论
- 新单已只产生五态之一（pending_recharge）。
- 对外 DTO 已归一五态 + refund_status。
- 仍有部分 Admin/写状态点未走状态机（见上），故为 PASS WITH CONDITIONS；不建议在完成 Admin 写路径接线前封板。
