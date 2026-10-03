# HCZ P0-3 Order State Machine — Final Report

## Final Verdict：**PASS WITH CONDITIONS**

本轮交付了**统一状态机核心（ONE SOURCE OF TRUTH）**并全量单测通过；与现有下单/Admin/退款链路的接线为下一增量。未触碰 P0-2 冻结资金 Contract。

## 已完成（本增量）
### 1. 五主状态常量（internal/constants/constants.go）
新增：`pending_recharge` / `processing` / `failed`（completed/canceled 已有）。
新增独立退款子状态：`none` / `partial` / `full`。
旧 9 态常量保留，不删除、不破坏性 UPDATE。

### 2. 统一状态机核心（order/application/ordermachine/machine.go）
纯逻辑包，不碰 DB/资金：
- `Normalize(old)` → 对外五主状态 + 独立 refund_status
  - pending_payment/paid→pending_recharge；fulfilling/partially_delivered→processing；delivered/completed→completed；canceled→canceled；partially_refunded→completed+partial；refunded→completed+full
- `CanTransition(from,to)`：pending_recharge→processing/failed/canceled；processing→completed/failed；终态无出边
- `AllowedByUser`：仅 pending_recharge→canceled
- `RequiresAutoRefund`：to=failed/canceled 触发 USDT 自动退款
- `IsTerminal`：completed/failed/canceled

### 3. 测试（全 PASS，6 用例）
历史归一映射、合法迁移、非法迁移拒、User 取消权限、自动退款标记、终态判定。

### 4. 验证
- `go build ./...` EXIT=0
- `go test ./ordermachine` PASS

## 待接线（下一增量，P0-3 剩余范围）
为不破坏当前线上可用链路，本轮未强行改写所有写状态点：
- 新单初始状态：现仍写 pending_payment/paid，需改为 pending_recharge（order_service.go:556/618）。
- Admin 改状态入口：需改走 `ordermachine.CanTransition` 校验 + `RequiresAutoRefund` 副作用（复用 P0-2 退款链，事务+幂等）。
- User 取消：改为 `AllowedByUser` 后端校验（现 order_service_child.go:136 校验 pending_payment）。
- DTO/Presenter：输出前经 `Normalize()`，统一五态 + refund_status。
- Migration：新增 `refund_status` 列（nullable，默认 none），非破坏性。
- Commission：failed/canceled 回滚已有返利（不改 USDT 金额口径）。
- Admin/User 前端筛选与按钮收敛到五态。

## 冻结确认
P0-2 资金 Contract、汇率、退款/返利金额口径、Wallet、Ledger 全部未改。

## 结论
- 状态机规则已冻结并通过单测，可作为后续接线的唯一权威。
- 建议下一增量：order_service/Admin handler 接线 → refund_status 列 migration → presenter normalize → 前端五态文案 → 端到端迁移/自动退款/幂等测试。
- 不建议在未完成接线前直接切流量。
