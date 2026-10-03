# HCZ P1 After-Sale Shared Transaction Wiring Final

## Final Verdict：**PASS WITH CONDITIONS**（共享事务架构 + DI 完成；真实 DB integration/并发注入测试待补）

### 本轮完成
1. **移除 aftersale 独立事务边界**：删除独立 `*gorm.DB` 的 GormStore；after_sale_ticket 方法（Create/Get/Lock...ForUpdate/Update）加进 `ordercontract.Store`，在 gormstore 实现，与订单共享同一事务抽象。
2. **Service 全部走共享事务**：
   - Request：`WithinTransaction` → `LockAfterSalePendingByOrderIDForUpdate` → 校验无 pending → 建 ticket → order.after_sale_status=pending → commit。
   - Reject/Resolve：`WithinTransaction` → 锁 pending ticket → 校验 → 更新 ticket + order.after_sale_status → commit。
   - PartialRefund/FullRefund（doRefund）：`WithinTransaction` → 锁 pending ticket → 校验 → `WalletRefunderAdapter.RefundInTx` → `AdminRefundToWalletInTx`（wallet credit/ledger/refund_status/commission/reseller 同一 tx）→ ticket=resolved → order.after_sale_status=resolved → commit。任一步失败整体 rollback。
3. **WalletRefunderAdapter** 指向 `AdminRefundToWalletInTx`（真正 credit wallet 的 P0-2 链），必须在调用方事务内调用。
4. **DI 生产接线**：container 加 `AfterSaleService`，用 `c.OrderStore` + `NewWalletRefunderAdapter(c.OrderRefundService)` 装配。
5. 验证：`go build ./...` EXIT=0；退款 integrationtest、aftersale 单测、wallet 全部 PASS。

### 仍缺（如实，不假装完成）
1. **真实 DB integration test 未写**：目前 aftersale 测试用 fake（验证事务内调用顺序与幂等），未用真实 Test DB 跑"partial refund → wallet balance 真增加 → ledger 真写入 → refund_status=partial → ticket resolved → order.status 仍 completed"端到端链。
2. **并发/rollback 注入测试未写**：场景 A（refund 成功后 ticket update 失败→全回滚）、场景 B（refund 自身失败）、两个 Admin 并发退款——单元层验证了 pending 锁语义与重复请求不二次退款，但真实 `SELECT ... FOR UPDATE` 并发竞争未在 DB 层验证。
3. aftersale 单测的 fakeTx 用接口嵌入（nil），只覆盖 aftersale 调用的方法，不代表真实事务行为。

## 明确回答
- aftersale 是否真正使用 order transaction abstraction？**是**（已删独立 gorm.DB，全部走 orderStore.WithinTransaction + tx.Orders()）。
- refund/ticket/order/wallet/ledger/commission 是否同一 tx？**架构上是**（AdminRefundToWalletInTx 接受外层 tx，aftersale 在同一 WithinTransaction 内调用）；真实 DB 端到端未跑。
- 是否还存在半成功窗口？**架构上已消除**（同一事务，任一步失败 rollback）；待 integration 注入测试最终证明。
- 是否还存在双退款窗口？**pending ticket 行锁 + 退款层幂等两层**；真实并发待 DB 测试证明。
- DI 是否生产可用？**是**（container 已装配，全量 build 通过）。
- partial/full integration 是否真实跑通？**否**（仅 fake 单测）。
- 是否可进入 Handler+Route？**建议先补一条真实 partial-refund integration（断言 wallet balance 增加）再上路由**；架构已就绪，风险低。

下一增量最小：写一条 aftersale integration（真实 Test DB + wallet service），断言 partial refund 后 wallet balance 增加、ledger 写入、refund_status=partial、ticket resolved、order.status 仍 completed；再补一个 refund-fail rollback 注入。
