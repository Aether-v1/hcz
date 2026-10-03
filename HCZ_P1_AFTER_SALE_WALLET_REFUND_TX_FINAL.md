# HCZ P1 After-Sale Wallet Refund Transaction Unification

## Final Verdict：**PASS WITH CONDITIONS**（退款链 InTx 化完成且回归通过；aftersale 共享事务动作流/DI/integration 未完成）

### 本轮完成
1. **AdminRefundToWalletInTx 抽取**（refund/wallet.go）：
   - 新增 `AdminRefundToWalletInTx(tx, input)`：接受外部 `ordercontract.Transaction`，不自己开事务，唯一实现 wallet credit + ledger + refunded_amount + status + **refund_status（原函数漏了，本轮补上）** + refund record + affiliate reversal + reseller。
   - 旧 `AdminRefundToWallet` 保留兼容：内部 `WithinTransaction` → 调 InTx，无重复逻辑。
2. **AdminManualRefundInTx**（上一轮已完成，本轮回归确认）。
3. **修正 aftersale RefunderAdapter**：删除错误的 AdminManualRefund 适配，新增 `WalletRefunderAdapter` 指向 `AdminRefundToWalletInTx`（真正 credit wallet 的 P0-2 链），必须在调用方事务内调用。
4. 验证：`go build` EXIT=0；退款 integrationtest 全 PASS（6.18s），旧调用方未破坏。

### 仍缺（不可封板）
1. **aftersale Store 仍持独立 `*gorm.DB`**，未改用 order store 事务抽象——ticket 更新与 refund 目前无法真正共享同一 DB tx。
2. **aftersale 动作流（Reject/Resolve/doRefund）未切到 `orderStore.WithinTransaction + LockPendingByOrderIDForUpdate + AdminRefundToWalletInTx`**。当前 doRefund 还是先读后改、调退款（且 service 层 Refunder 端口还是旧签名，未接新 WalletRefunderAdapter）。
3. **DI Wiring 未做**：生产容器拿不到 aftersale service / adapter。
4. **Integration / 并发 / rollback 注入测试未写**：wallet credit、ledger、commission 同一事务、半成功回滚、双 Admin 竞争均未真实验证。

## 明确回答
- AfterSale 是否已改用 AdminRefundToWalletInTx？**适配器已修正指向它，但 service 动作流尚未调用**。
- 售后退款是否真的 credit Wallet？**退款链本身会（已验证），但 aftersale 端到端未接，未跑通**。
- Wallet/Refund/Ticket/Commission 是否同一事务？**否**（aftersale store 独立 gorm.DB，动作流未切共享 tx）。
- 是否还有半成功窗口？**是**。
- 是否还有并发双退款窗口？**是**。
- DI 是否已生产接线？否。
- 是否可进入 Handler+Route？**否**——必须先完成 aftersale store 改 order tx 抽象 + 动作流接共享事务 + DI + 至少一条 integration（含 wallet credit 验证）。

下一增量最小：把 aftersale GormStore 改成依赖 `ordercontract.Store`/事务（或在 order store 上加 after_sale_ticket 方法），然后 Reject/Resolve/doRefund 统一走 `orderStore.WithinTransaction` + 行锁 + `WalletRefunderAdapter.RefundInTx`，再做 DI 与一条 partial-refund integration（断言 wallet balance 增加）。
