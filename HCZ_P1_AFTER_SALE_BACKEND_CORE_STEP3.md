# HCZ P1 After-Sale Backend Core Step 3 — Concurrency Closure

## Final Verdict：**PASS WITH CONDITIONS**（行锁能力已落地，动作流/DI/integration 未完成）

### 本轮完成
- Store 新增 `WithTransaction(fn)` 与 `LockPendingByOrderIDForUpdate(tx, orderID)`：事务内 `SELECT ... FOR UPDATE` 锁定 pending 工单，真实 DB 行锁能力就绪（GormStore 实现，编译通过）。
- fakeStore 补齐接口方法，aftersale 单测继续全 PASS。
- 包 `go build` EXIT=0、`go test` PASS。

### 仍缺（如实，不可封板）
1. **Service 动作流未切到事务内锁**：Reject/Resolve/PartialRefund/FullRefund 目前仍是"先读 ticket → 校验 pending → 调 refund → 更新"，未走 `WithTransaction + LockPendingByOrderIDForUpdate`。行锁能力存在但未被业务流使用——**真实并发双退款窗口仍在**。
2. **DI Wiring 未做**：GormStore / Service / RefunderAdapter 未装进 bootstrap 容器，生产拿不到实例。
3. **Integration Test 未写**：partial/full 真实链（wallet credit→refund_status→after_sale_status→主状态 completed）未跑；并发双退款未在真实 DB 验证。

## 明确回答
- 是否真实使用 DB 行锁？**Store 层有 FOR UPDATE 能力**，但 service 动作流尚未调用——端到端未生效。
- 是否仍存在并发双退款窗口？**是**（动作流未切事务锁）。
- RefunderAdapter 是否已生产 DI 接线？**否**。
- Partial/Full refund 是否端到端跑通？**否**（仅单测 fake）。
- 是否可安全进入 Handler+Route？**否**——先把 service 动作流切到事务锁 + DI 装配 + 一条 integration，再上路由。

下一增量最小：把 Reject/Resolve/doRefund 三个动作统一改成 `store.WithTransaction` + `LockPendingByOrderIDForUpdate` + reload 校验，再做 DI。
