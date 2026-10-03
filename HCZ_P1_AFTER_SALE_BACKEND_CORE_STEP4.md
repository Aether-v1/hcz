# HCZ P1 After-Sale Backend Core Step 4 — Transaction Wiring Finalization

## Final Verdict：**BLOCKED（事务边界真实冲突，未做任何假装原子的接线）**

### 阻塞点（已审计确认）
现有正式退款 `refund.Service.AdminManualRefund`（service.go:253）**在内部第 290 行自己开启 `s.orderStore.WithinTransaction`**，且**不接受外部事务参数**。

后果：如果 aftersale 用自己的 `store.WithTransaction` 包一层再调 RefunderAdapter→AdminManualRefund，会出现**两层独立事务**：
1. 内层 refund 事务先 commit（Wallet credit、Ledger、Commission rollback、refund_status 全部落库）。
2. 外层 aftersale 事务再更新 ticket/order.after_sale_status。
3. 若第 2 步失败 → **半成功**：钱已退、commission 已回滚，但 ticket 仍 pending、after_sale_status 仍 pending。

这正是你要求"必须先停下报告，不得假装原子"的场景。本轮**没有**把 service 动作流改成事务锁，因为那样会引入这个半成功窗口。

### 附加冲突
- aftersale GormStore 持有自己的 `*gorm.DB`，与 order store 的 `ordercontract.Transaction` 抽象不是同一套，**当前无法共享同一个 tx**。
- 因此"aftersale 外层 tx + refund 内层 tx 合一"在现有抽象下做不到。

### 必须先解决的设计选择（二选一，需你定）
**方案 A（推荐，最小侵入）**：aftersale **不**开自己的事务。把"ticket 状态推进 + order.after_sale_status 更新"作为 refund 事务内的一个回调/hook，由 refund service 在自己的 WithinTransaction 里调用。即：扩展 refund service 支持 `WithRefundTransaction(fn(tx))` 或在 AdminManualRefund 增加 `PostCommit/InTxHook`。退款成功则 ticket 一并 resolved，退款失败则全部回滚。
**方案 B**：给 refund service 增加一个接受外部 `ordercontract.Transaction` 的变体 `AdminManualRefundInTx(tx, input)`，aftersale 在自己的 order-store 事务内调用它。同时 aftersale store 必须改用 order store 的事务抽象，不能再持有独立 gorm.DB。

无论哪种，都需要**改 refund service 签名/增加方法**——这超出"不改退款逻辑"的本轮边界，所以必须先由你确认方案再动。

### 本轮未做（因阻塞）
- Service 动作流事务锁接线（避免引入半成功）。
- DI Wiring。
- Integration / 并发测试。

## 明确回答
- Reject/Resolve/refund 是否全部走 transaction+row lock？**否**——发现事务冲突后主动停下，未接线。
- Refund 与售后状态更新是否同一原子事务？**当前架构下做不到**（refund 自带独立事务，不接受外部 tx）。
- 是否仍有双退款窗口？**是**（动作流仍是先读后改）。
- DI 是否生产可用？否。
- integration 是否真实跑通？否。
- 是否可进入 Handler+Route？**否**——必须先选定方案 A/B 并打通原子事务，再继续。

请确认采用方案 A（refund 内 hook，推荐）还是方案 B（refund 增加 InTx 变体 + aftersale 改用 order 事务抽象），我再继续。
