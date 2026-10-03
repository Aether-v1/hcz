# HCZ P0-3 Order State Machine — Final Wiring Closure

## Final Verdict：**PASS WITH CONDITIONS（接近封板，剩 1 个 P0 写路径）**

已冻结 direct-debit 资金模型（不引入 frozen/hold/release）。

## 已完成并验证
- 新单初始状态 = pending_recharge（order_service 两处创建点）。
- DTO 对外经 `ordermachine.Normalize` 输出五态 + `refund_status`；历史 9 态兼容。
- 状态机核心 `ordermachine` 单测全 PASS（迁移合法性/User权限/自动退款标记/终态/归一）。
- `go build ./...` EXIT=0；order/refund/wallet 回归 PASS。

## 未完成（阻塞封板的真实 P0）
**Admin 写路径尚未整体切到状态机 + failed 自动退款。**
现有 `UpdateOrderStatus`（order_service_child.go:159）是一条深度耦合父子单、cancel、refund、commission 的老路径：
- 它已对 canceled 走 `cancelOrderWithChildren`（内含钱包退款 credit）+ commission rollback（HandleOrderCanceled）——canceled 链路基本可用。
- 但新增的 `failed` / `processing` 目标尚未接入：没有「processing 开始处理」入口、`failed` 自动退 USDT 还没接 P0-2 退款链、终态回退未统一用 `CanTransition` 拦截。
- refund_status 目前由 Normalize 推断，尚未物理加列。

## 明确回答
- 是否还有状态写入绕过状态机？**是**——老 `UpdateOrderStatus` 仍按旧 9 态逻辑流转，未整体走 ordermachine。
- failed 是否稳定自动退 USDT？**否（未接线）**；canceled 退款已有老链路。
- User Cancel 是否完整接线？**部分**——老 cancel 链存在，但未显式校验 `AllowedByUser`。
- Commission 是否正确回滚？canceled 已有 `HandleOrderCanceled`；failed 未接。
- refund_status 是否独立落库？**否**（当前推断）。
- 历史 9 态是否兼容？**是（DTO Normalize）**。
- P0-3 是否可封板？**暂不可**——需完成 Admin failed/processing 写路径 + 自动退款接线。

## 下一增量（最小）
1. `UpdateOrderStatus` 入口加 `ordermachine.CanTransition` 归一校验，拒绝终态回退。
2. target=failed 时在事务内复用 P0-2 退款链（wallet_paid_amount USDT snapshot），幂等。
3. processing/completed 走现有履约完成逻辑。
4. User Cancel 显式 `AllowedByUser`。
5. refund_status 非破坏性加列 + Normalize 回填读取。

本轮未改资金 Contract / 汇率 / Gateway / 会员 / 商品。
