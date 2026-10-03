# HCZ P1 After-Sale / Not Received / Partial Refund — Pre-Audit

## 现状结论
P0 已封板。当前**无** after_sale_status / confirm_status / not_received 字段，无 User 发起退款/售后入口。退款目前只有 **AdminManualRefund**（管理员后台手动退款），写 OrderRefundRecord + USDT snapshot 退款链。

## 现状清单
| 项 | 现状 | 判定 |
|---|---|---|
| after_sale_status / confirm_status 字段 | 不存在 | BUILD |
| User 发起"未收到/售后"入口 | 不存在 | BUILD |
| Admin 售后审核队列 | 不存在（只有手动退款动作） | BUILD |
| 部分/全额退款执行 | 已有 `refund/service.go` AdminManualRefund | KEEP |
| OrderRefundRecord 记录 | 已有 | KEEP |
| refund_status none/partial/full | 已落库，退款流程写回 | KEEP |
| 主状态五态 | 已封板 | KEEP（售后不改主状态） |
| USDT snapshot 退款 | P0-2 已冻结 | KEEP |
| commission rollback | failed/canceled 已 HandleOrderCanceled | MODIFY（售后场景不自动回滚，仅按退款比例另行处理，P1 先不做） |

## 目标流程（不变更主状态）
completed → User 点"未收到" → after_sale_status=pending → Admin 审核 → 驳回 / 部分退款 / 全额退款 / 标记已解决。
全程 `orders.status=completed`，仅 after_sale_status 与 refund_status 变化。

## KEEP / MODIFY / BUILD / DROP
- **KEEP**：AdminManualRefund 退款执行链、OrderRefundRecord、refund_status、USDT snapshot、五主状态。
- **MODIFY**：Admin 退款执行增加"关联售后工单"参数（可选）；refund 后按累计金额刷新 refund_status（已有）。
- **BUILD**：
  1. 新表 `after_sale_ticket`（order_id, user_id, type=not_received, reason, status=pending/approved_partial/approved_full/rejected/resolved, amount, refund_record_id, audit_*）。
  2. order 列 `after_sale_status`（none/pending/resolved/rejected）。
  3. User API：发起未收到工单、查看自己工单状态。
  4. Admin API：工单列表、审核（驳回/部分退款/全额退款/解决）。
  5. 前端：User 订单详情"未收到"按钮（仅 completed 且无 pending 工单）；Admin 售后队列页。
- **DROP**：无（不引入第六主状态，不做 confirm_status 收货确认流）。

## 资金联动
- 部分/全额退款**复用现有 AdminManualRefund**（USDT snapshot，不按当前汇率重算）。
- 退款成功后 refund_status 自动 partial/full。
- 售后驳回/解决不动资金。

## 权限
- User：仅对自己 completed 订单发起一个 pending 工单。
- Admin：审核 + 触发退款（复用现有 RBAC）。

## 测试矩阵
User 发起未收到（completed）→ pending；非 completed 拒绝；重复发起拒绝；Admin 驳回；Admin 部分退款→partial；Admin 全额退款→full；主状态始终 completed；refund 金额按 USDT snapshot；历史订单兼容；权限 401/403。

## 最小实现范围
新增一张 after_sale_ticket 表 + order.after_sale_status 列 + 两组薄 API + 两个最小前端页。退款执行完全复用，不重写。

本轮未改 P0 Contract。
