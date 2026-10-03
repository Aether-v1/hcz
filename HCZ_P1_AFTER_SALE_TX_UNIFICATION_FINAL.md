# HCZ P1 After-Sale Tx Unification

## Final Verdict：**PASS WITH CONDITIONS**（退款核心抽取完成且回归通过；aftersale 共享事务/钱包退款路径/DI/integration 未完成）

### 本轮完成
1. **退款唯一核心抽取**（refund/service.go）：
   - 新增 `AdminManualRefundInTx(tx, input)`：接受外部 `ordercontract.Transaction`，不自己开事务，执行完整退款核心（order 状态/refunded_amount/refund_status、退款记录、affiliate、reseller）。
   - 私有 `adminManualRefundInTx` 是唯一实现。
   - 旧 `AdminManualRefund` 保留兼容：内部 `WithinTransaction` → 调 `adminManualRefundInTx`，**无重复逻辑**。
2. 编译 EXIT=0；退款 integrationtest 全 PASS（6.5s），旧调用方未破坏。

### 关键发现（必须修正之前的错误）
现有退款有**两个入口**：
- `AdminManualRefund`（service.go）：**只记账，不 credit wallet**（不写 ledger、不动钱包余额）。
- `AdminRefundToWallet`（wallet.go:33）：**真正 credit wallet + ledger + affiliate**，才是 P0-2 USDT 退款链。

我之前 aftersale 的 `RefunderAdapter` 接的是 `AdminManualRefund`——**这是错的，售后退款不会真的退钱到钱包**。aftersale 必须接 `AdminRefundToWallet`。

### 仍缺（不可封板）
1. **`AdminRefundToWallet` 也需要 InTx 抽取**（它目前在 wallet.go 里自己开事务，含 wallet credit）。这是 aftersale 真正要复用的路径，必须先抽成 `AdminRefundToWalletInTx(tx, ...)`。
2. **aftersale Store 必须改用 order store 事务抽象**，不能再持独立 `*gorm.DB`，否则无法与 refund 共享同一 tx。
3. aftersale Reject/Resolve/doRefund 动作流切到 `WithinTransaction + LockPendingByOrderIDForUpdate + AdminRefundToWalletInTx`。
4. DI Wiring、integration/并发测试。

## 明确回答
- Refund 是否只有一套核心逻辑？**AdminManualRefund 是**（已抽取）；AdminRefundToWallet 仍是独立实现，待统一。
- AfterSale/Refund 是否真正共享同一事务？**否**（aftersale 还持独立 gorm.DB，且未接对退款入口）。
- 是否还存在半成功窗口？**是**（动作流未切共享事务）。
- 是否还存在双退款窗口？**是**。
- 旧 AdminManualRefund 是否保持兼容？**是**（回归通过）。
- 是否可进入 Handler+Route？**否**——先抽 AdminRefundToWalletInTx + aftersale 改用 order 事务抽象 + 动作流接线。

下一增量最小：对 `AdminRefundToWallet` 做同样的 InTx 抽取（这才是 aftersale 要用的），然后 aftersale store 改 order tx 抽象、动作流接共享事务。
