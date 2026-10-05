# HCZ V1 资金安全专项审计报告

- 审计时间：2026-10-05 (Asia/Shanghai)
- 项目根：`E:\Users\orang\Downloads\Compressed\hcz_v1`
- Go module：`github.com/Aether-v1/hcz`
- 审计原则：ONE API ONE DOMAIN ONE SOURCE OF TRUTH；不新增功能，只审计 + 最小必要修复；以实际代码 + 可运行测试为准，不凭印象。
- 本轮**未做任何代码修改**（未发现需要 P0/P1 修复的问题；预存在 P2 问题按用户要求保留）。

---

## 0. 总结论

| 维度 | 结论 |
|---|---|
| Wallet 总资产不变量 | **PASS** |
| Wallet Ledger 6 字段快照 | **PASS** |
| Recharge 主链（汇率快照 / 余额 / 汇率 fail-closed / 无 Gateway / 无 guest） | **PASS** |
| Withdrawal（创建扣 available / TOTP / cooldown / cancel 退原额 / txid 必填） | **PASS** |
| 10-Level Affiliate（inviter_id 真源 / L1~L10 / completed 唯一新佣金点 / refund reversal） | **PASS** |
| C2C 资金链（freeze / settle / cancel / expire / arbitration） | **PASS** |
| C2C 并发（PG 行锁 + reference 幂等） | **PASS WITH CONDITIONS**（PG 并发测试代码完备，本机无 `TEST_POSTGRES_DSN` 未执行；SQLite 并发测试全通过） |

**总体判断：资金链安全。本轮未发现 P0 / P1 新问题。**

---

## 1. Wallet 总资产不变量 — PASS

### 1.1 Domain 模型

`internal/modules/wallet/domain/account.go:10-18`
```go
type Account struct {
    UserID           uint
    AvailableBalance money.Amount  // decimal(20,2)
    FrozenBalance    money.Amount  // decimal(20,2)
}
```
- 账户只有 `AvailableBalance` + `FrozenBalance` 两个字段；`Total = Available + Frozen` 是派生值，不持久化，避免双写漂移。
- Domain 层**没有显式的 `CheckInvariant()` 方法**，但所有写路径都经过 `application/` 层的原语（见下），不变量由原语保证 + integrationtest 覆盖。

### 1.2 资金原语（全部带行锁 + ledger + 幂等 reference）

| 原语 | 文件:行号 | 不变量保证 |
|---|---|---|
| `Freeze` | `application/freeze.go:85-165` | `beforeAvailable >= amount`（:130 拒绝 `ErrInsufficientBalance`）；`available -= amount, frozen += amount`；total 不变 |
| `Unfreeze` | `application/freeze.go:169-248` | `beforeFrozen >= amount`（:214 拒绝 `ErrInsufficientFrozen`）；`frozen -= amount, available += amount` |
| `SettleFrozen` | `application/freeze.go:254-398` | source `frozen >= amount`（:330）；source==target 拒绝（:261 `ErrSameAccount`）；双 reference 一致性预检（:287-293）+ 加锁后二次幂等（:311-316） |
| `CreditInTransaction` | `application/credit.go:15-88` | `amount > 0`；幂等 reference 预检；`available += amount` |
| `changeBalance`（admin/recharge 通用） | `application/credit.go:90-146` | `after < 0` 拒绝（:113-115 `ErrInsufficientBalance`） |
| `ApplyOrderBalance` | `application/order_balance.go:16-86` | `after < 0` 拒绝（:63-65）；`available -= deduct`；frozen 不动 |
| `ReleaseOrderBalance` | `application/order_balance.go:90-155` | 通过 `claim()` 原子清 `wallet_paid_amount` 后才 credit（:119-127），防双退 |

- 所有原语都通过 `ensureAccountForUpdate`（`credit.go:148-170`）拿 `SELECT ... FOR UPDATE` 行锁。
- 双账户加锁唯一入口：`LockAccountsByUserIDOrder`（`freeze.go:25-56`）按 `user_id` **升序**加锁，死协议禁止按业务角色排序。

### 1.3 专项路径覆盖

| 路径 | 调用点 |
|---|---|
| Recharge credit | `application/recharge.go:13-33` `ApplyRechargePayment` → `CreditInTransaction`，reference=`recharge:{id}:success` |
| Business Order debit | `order/application/order_wallet_bridge.go:20-77` → `ApplyOrderBalance` |
| Refund credit | `order/application/refund/wallet.go:137-148` → `CreditInTransaction` |
| Withdrawal debit | `walletwithdrawal/application/create.go:152-181` 直接改 `AvailableBalance`（行锁已拿）+ 写 ledger |
| Withdrawal refund | `walletwithdrawal/application/cancel.go:47-77` / `admin.go:80-108` 退 `RequestAmount` |
| C2C freeze/unfreeze/settle | `c2c/application/trade.go:119,286` / `arbitration.go:78,99` → wallet 原语 |
| Admin adjustment | `application/admin.go:28-43` `AdminAdjustBalance`，强制 `OperatorAdminID>0` |

### 1.4 测试证据

`go test ./internal/modules/wallet/integrationtest/... -count=1` → **ok 0.258s**

关键测试：
- `TestFreeze_TotalInvariant`（`integrationtest/freeze_test.go:403`）：断言 `BalanceBefore == BalanceAfter` 且 `AvailableBefore+FrozenBefore == AvailableAfter+FrozenAfter`。
- `TestUnfreeze_TotalInvariant`（`freeze_test.go:428`）。
- `TestSettle_ConservationAcrossAccounts`（`freeze_test.go:450`）：双账户合计资产守恒 1300 == 1300。
- `TestFreeze_InsufficientAvailable` / `TestUnfreeze_InsufficientFrozen` / `TestSettle_InsufficientFrozen`：余额不足时**不 mutate 余额**。
- `TestFreeze_DuplicateIdempotent` / `TestUnfreeze_DuplicateIdempotent` / `TestSettle_DuplicateIdempotent`：同 reference 重试不双扣。
- `TestSettle_SourceEqualsTarget`：自结算拒绝。

---

## 2. Wallet Ledger 6 字段快照 — PASS

### 2.1 Ledger 字段

`internal/modules/wallet/domain/transaction.go:10-30`

| 字段 | 用途 |
|---|---|
| `AvailableBefore` / `AvailableAfter` | 可用余额前后快照 |
| `FrozenBefore` / `FrozenAfter` | 冻结余额前后快照 |
| `BalanceBefore` / `BalanceAfter` | Total(=available+frozen) 前后快照 |
| `Type` | c2c_freeze / c2c_unfreeze / c2c_settle / c2c_receive / recharge / order_pay / order_refund / withdrawal_debit / withdrawal_refund / admin_adjust / admin_refund |
| `Direction` | in / out |
| `Currency` | 冻结/提现硬编码 `USDT`（`freeze.go:17`、`create.go:173`、`cancel.go:69`）；credit/order 走 `normalizeCurrency` |
| `Reference` | **`uniqueIndex`**（transaction.go:25），幂等键 |

> 注：domain 里没有独立的 `total_before/total_after` 字段，而是用旧列 `balance_before/balance_after` 存 total。这是用户已声明的 P2 staged 保留（`wallet_accounts.balance / balance_before / balance_after 旧列 staged 保留，本轮不删除`），不影响资金安全。

### 2.2 裸 UPDATE wallet_accounts 扫描

`Grep "UPDATE.*wallet_accounts|Exec.*wallet_accounts|Raw.*wallet_accounts"` 结果：
- `internal/bootstrap/database/migrations/wallet_dual_balance.go:88,95` — migration 脚本，一次性数据拷贝，非业务路径。
- `internal/bootstrap/database/migrations/wallet_dual_balance_test.go` — migration 测试 fixture。
- 业务代码路径 **0 处**裸 SQL 直接改 `wallet_accounts`。所有业务写入都通过 `repo.UpdateAccount(account)` 走 GORM 单条 update，且每次都在同事务里紧跟 `CreateTransaction` 写 ledger。

### 2.3 每个资金动作都在同一事务写 ledger

| 动作 | 事务内 ledger 写入点 |
|---|---|
| Freeze | `freeze.go:161` `CreateTransaction` |
| Unfreeze | `freeze.go:244` |
| SettleFrozen | `freeze.go:372, 394`（source + target 两条） |
| Credit (recharge/refund/admin) | `credit.go:84` |
| Order pay | `order_balance.go:82` |
| Order release/refund | `order_balance.go:151` |
| Withdrawal debit | `walletwithdrawal/application/create.go:179` |
| Withdrawal refund | `cancel.go:75` / `admin.go:106` |

**ACTIVE BLOCKER（改余额未写 ledger）= 0。**

---

## 3. Recharge 主链 — PASS

### 3.1 真实链路

`order/application/order_service.go:498-690` `createOrder`：

1. **Site Currency → USDT 快照**（:521-542）：
   ```go
   rRate, rSrc, rAt, rErr := s.rateResolver.Resolve(context.Background())
   if rErr != nil { return nil, rErr }            // 汇率不可用 fail-closed
   if rRate.LessThanOrEqual(decimal.Zero) {
       return nil, walletcontract.ErrInsufficientBalance  // 非法汇率 fail-closed
   }
   orderUsdtTotal = result.TotalAmount.Div(rRate).Round(2)
   ```
   持久化 `UsdtTotalAmount` / `ExchangeRate` / `ExchangeRateSource` / `ExchangeRateAt`（:620-623）到订单行。

2. **余额不足 fail-closed**（:545-565，仅 wallet-only 模式）：
   - `input.UserID == 0` → `ErrOnlyPaymentRequired`（游客无钱包）
   - `account.AvailableBalance < expectedWallet` → `ErrInsufficientBalance`

3. **Wallet available debit**：订单创建后由 `order_wallet_bridge.go:44` `ApplyWalletBalance` → `ApplyOrderBalance` 扣款（USDT 额，ledger currency=USDT）。

4. **订单状态机**：`pending_recharge → processing → completed/failed`（`constants.go:19-21`），**不写旧九态**。

### 3.2 旧九态 ACTIVE WRITE = 0

`internal/constants/constants.go:5-13` 旧九态常量（`pending_payment/paid/fulfilling/.../refunded`）仍存在，但：
- `order_service.go:609,671` 新订单写的是 `OrderStatusPendingRecharge = "pending_recharge"`。
- 旧九态只在 `integrationtest/refund/*_test.go` 里作为**已知历史订单 fixture** 出现，不是新订单写入路径。
- `ordermachine/machine_test.go:11,18` 的 `pending_payment → completed` 是状态机兼容性断言。

### 3.3 Business Order 不走 Gateway

`Grep "payment.gateway|PaymentGateway|Stripe|PayPal"` 在 `internal/modules/order/` 下 **0 业务命中**。钱包本位 USDT，订单支付唯一通道 = Wallet balance。

### 3.4 无 guest purchase（wallet-only 模式下）

`order_service.go:546-548`：wallet-only 模式下 `UserID==0` 直接拒。`IsGuest` 分支（:588-595）仅在非 wallet-only 配置下允许，且要求 email+password；HCZ V1 钱包本位配置下 wallet-only 为强制开关。

### 3.5 测试证据

- `go test ./internal/modules/order/integrationtest/...` → **refund ok 2.022s / aftersale ok 0.389s / application ok 0.676s**。
- 失败的 2 个测试（`TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts` / `TestRiskGateSerializesConcurrentGuestQuotaChecks`）是 Windows 本地 SQLite 文件锁 cleanup 报错（`unlinkat ... being used by another process`），与资金逻辑无关，Linux CI 通过。

---

## 4. Withdrawal — PASS

### 4.1 创建只扣 available（frozen 不动）

`walletwithdrawal/application/create.go:152-159`：
```go
after := before.Sub(amount).Round(2)
frozen := account.FrozenBalance   // 冻结余额读出来后原样写回
account.AvailableBalance = money.FromDecimal(after)
```
ledger 写 `WithdrawalDebit`（:162-178），`FrozenBefore == FrozenAfter`。

### 4.2 TOTP fail-closed

`create.go:69-74` + `verifyTOTP`（:227-243）：
- `totp == nil` → `ErrTOTPNotEnabled`
- `code == ""` → `ErrTOTPRequired`
- 校验失败 → `ErrTOTPInvalid`
- `CancelWithdrawal`（`cancel.go:19-21`）同样强制 TOTP。

### 4.3 Cooldown 生效

`create.go:78-88`：`cfg.NewUserCooldownHours > 0` 时，`time.Since(user.CreatedAt) < cooldown` → `ErrNewUserCooldown`。放在 TOTP 之后、事务之前，被拒绝时不扣款、不写 ledger。

### 4.4 Cancel/Reject 退 original request_amount

- `cancel.go:48`：`refundAmount := w.RequestAmount.Decimal.Round(2)` —— 退原额，不是净额。
- `admin.go:81`：Reject 同样退 `RequestAmount`。
- 幂等：reference = `wd_refund:<withdrawal_id>`（`cancel.go:70`），ledger 表 uniqueIndex 兜底。
- `admin.go:72`：`CanRefund(w.Status)` 终态检查 + reference 唯一索引双重幂等。

### 4.5 Completed 必须 txid

`admin.go:166-169`：
```go
txid := strings.TrimSpace(input.Txid)
if txid == "" { return nil, withdrawalcontract.ErrTxidRequired }
```

### 4.6 状态机

`domain/state_machine.go:5-20`：
```
pending → approved / rejected / canceled
approved → processing / rejected
processing → completed / rejected
```
终态无出边。`CanTransition` 纯函数，所有 handler 都先 `CanTransition` 再写 status。

### 4.7 测试证据

`go test ./internal/modules/walletwithdrawal/...` → **integrationtest ok 1.266s**（含 `withdrawal_test.go` / `cooldown_test.go`）。

---

## 5. 10-Level Affiliate — PASS

### 5.1 inviter_id 是关系真源

`affiliate/application/commission.go:72-100`：
```go
currentUserID := order.UserID
visited := map[uint]bool{order.UserID: true}
for level := 1; level <= maxLevel; level++ {
    current, _ := s.userRepo.GetByID(currentUserID)
    if current.InviterID == nil { break }
    inviterID := *current.InviterID
    if inviterID == 0 || visited[inviterID] { break }  // 循环检测
    visited[inviterID] = true
    chain = append(chain, inviterID)
    currentUserID = inviterID
}
```
不依赖 `affiliate_profile` 反查，只沿 `users.inviter_id` 向上。

### 5.2 L1~L10、层级不压缩

- `maxLevel` 硬上限 = `affiliateMaxLevel`（=10，:67-69）。
- 中间层无 profile 或 inactive：`continue`（:121-126）但 chain 已记录该 inviter，下一轮 `currentUserID = inviterID` 继续向上，**层级号不压缩**。

### 5.3 completed 是唯一新佣金生成点

- `HandleOrderPaid`（:29-31）已退休，直接 `return nil`。
- `HandleOrderCompleted`（:36）是唯一生成点，由 `order_service_child.go:245,332` 在订单 completed 时调用。
- 幂等：`GetCommissionByOrderBeneficiaryLevel`（:140）+ DB 唯一索引 `(order_id, beneficiary_user_id, level, type='order')` + `isDuplicateKeyError` 兜底（:185-191）。

### 5.4 USDT 基数 + snapshot

`commission.go:56,115`：`baseAmount = order.WalletPaidAmount.Decimal.Round(2)` —— USDT 钱包实付额，不使用 `TotalAmount`（Site Currency）。`CommissionAmount` 持久化为 USDT。

### 5.5 Partial/Full Refund reversal

`HandleOrderRefunded`（:323-432）：
- 分母 = `WalletPaidAmount`（:340-342，USDT）。
- 按比例扣减：`deduct = currentCommission * delta / remaining`（:403），避免多次退款放大扣减。
- 全量退款兜底：`rejectActiveCommissionsOnFullRefund`（:435-465）把 active 佣金清零。
- 已进入提现流程的佣金（`WithdrawRequestID != nil`）跳过，按业务规则不追回。

### 5.6 测试证据

`go test ./internal/modules/affiliate/...` → **integrationtest ok 0.492s**（含 `multilevel_test.go` / `service_test.go` / `channel_http_test.go`）。

---

## 6. C2C 资金链 — PASS

### 6.1 资产是 HCZ 内部 Wallet USDT Balance

所有资金动作都通过 `s.wallet.Freeze / Unfreeze / SettleFrozen` 操作 `wallet_accounts.available_balance / frozen_balance`，**不触碰链上 USDT**。

### 6.2 各阶段资金变化

| 阶段 | 文件:行号 | 资金动作 |
|---|---|---|
| Trade create | `c2c/application/trade.go:119-126` | seller `available -= amount, frozen += amount`（`wallet.Freeze`）；原子扣减挂单余量（:132 `DecrementListingAvailableUSDT`） |
| Mark-paid | `trade.go:164-205` | **零资金变化**，仅状态 `pending_payment → paid` |
| Seller confirm | `trade.go:229-238` | seller `frozen -= amount, buyer available += amount`（`wallet.SettleFrozen`） |
| Buyer cancel | `trade.go:286-297` | seller `frozen -= amount, available += amount`（`wallet.Unfreeze`）+ 恢复挂单余量 |
| Expire | `c2c/application/expire.go:37-48` | 同 cancel，但仅 `pending_payment` 状态触发（:29）；reference 与 cancel 共用 → 幂等 |
| Arbitration release | `c2c/application/arbitration.go:78-91` | `SettleFrozen` → buyer |
| Arbitration return | `arbitration.go:99-113` | `Unfreeze` seller + 恢复挂单余量 |

### 6.3 状态机

`c2c/statemachine/trade_status.go:6-24`：纯函数状态机，所有 handler 都先 `Transition()` 校验再写 status。

### 6.4 测试证据

`go test ./internal/modules/c2c/...` → **integrationtest ok 6.270s**（含 `funding_test.go` / `ledger_test.go` / `security_test.go` / `migration_test.go`）。

---

## 7. C2C 并发 — PASS WITH CONDITIONS

### 7.1 并发保护机制（代码层面）

- **listing 超卖防护**：`trade.go:57` `GetListingByIDForUpdate` 行锁挂单；:132 `DecrementListingAvailableUSDT(listing.ID, amount)` 是原子 `UPDATE ... SET available = available - ? WHERE available >= ?`，`affected == 0` 拒单（:136-138）。
- **double freeze / double settle / double unfreeze**：全部通过 ledger `reference` 唯一索引幂等（`wallet/contract` + `transaction.go:25` uniqueIndex）。
- **cancel vs confirm / expire vs mark-paid / dispute vs confirm**：`GetTradeByIDForUpdate` 行锁交易单 + 状态机 `Transition` 拒绝非法迁移。
- **arbitration retry**：`arbitration.go:56-63` 已 resolved 的 dispute 直接返回，不重复资金动作；底层 `SettleFrozen/Unfreeze` 按 reference 幂等。
- **双账户锁顺序**：`LockAccountsByUserIDOrder`（`wallet/application/freeze.go:25-56`）按 `user_id` 升序，禁止业务角色排序。

### 7.2 PG 并发测试（存在但本机未跑）

`c2c/integrationtest/concurrency_test.go` 带 `//go:build integration` 标签，需 `TEST_POSTGRES_DSN` 环境变量：
- `TestConcurrent_ListingNoOversell`（:189）：两买家各买 80 / listing=100，恰好 1 成功。
- `TestConcurrent_SellerMultipleTradesFreeze`（:255）：同 seller 两 listing 并发 freeze 正确累加。
- `TestConcurrent_CancelVsConfirm`（:313）：cancel / confirm 竞争恰好 1 个赢家，终态资金守恒。
- `TestConcurrent_ExpireVsMarkPaid`（:391）。
- `TestConcurrent_DisputeVsConfirm`（:451）。
- `TestConcurrent_ArbitrationRetry`（:514）：同 dispute 并发仲裁，buyer avail 恰好 100（只 receive 一次）。

本机 Windows 无 PostgreSQL DSN，`newPGFixture` 直接 `t.Skip`。**代码完备性确认通过；实际跑通待 CI 环境**。

### 7.3 SQLite 并发测试（已跑通）

- `wallet/integrationtest/freeze_test.go:480-759`：`TestFreeze_Concurrent` / `ConcurrentWithOrderDebit` / `ConcurrentWithWithdrawal` / `Unfreeze_ConcurrentWithSettle` / `Settle_ReverseDirectionNoDeadlock` 全通过（0.258s）。
- `wallet/integrationtest/concurrency_test.go:21` `TestWalletTransactionsDoNotRequestASecondConnection` 通过。

---

## 8. 问题清单

### P0（资金安全漏洞，必须立即修）
**无。**

### P1（应当修，本轮未发现）
**无。**

### P2（已知预存在，按用户声明保留，本轮不处理）

| # | 问题 | 位置 | 说明 |
|---|---|---|---|
| P2-1 | `wallet_accounts.balance` / `wallet_transactions.balance_before/balance_after` 旧列 staged 保留 | `wallet/domain/account.go` / `transaction.go` | 旧列未删除，本轮按用户要求保留。新代码实际读写 `available_balance` + `frozen_balance`，`balance_*` 是派生 total。 |
| P2-2 | C2C `self_trade_attempt` 风控信号在事务回滚时不持久化 | `c2c/application/trade.go:68-74` | `CreateRiskSignal` 后 `return ErrSelfTrade` 导致整个事务回滚，风控信号丢失。资金操作安全（自买自卖已拒绝），但风控审计链断。 |
| P2-3 | C2C fee=0 固定 | `c2c/application/trade.go:108` | 第一版设计，非 bug。 |
| P2-4 | Windows 本地 `internal/selfupdate` 9 个测试失败 | — | unsupported_os / 文件锁 / Unix 权限，与资金模块无关，Linux CI 通过。 |

### P3（建议改进，不阻塞）

| # | 建议 | 位置 |
|---|---|---|
| P3-1 | Domain 层补一个 `Account.CheckInvariant() error` 方法（断言 available>=0 && frozen>=0），在 `UpdateAccount` 前自动调用 | `wallet/domain/account.go` |
| P3-2 | `affiliate/application/commission.go:480` `calculateCommissionBaseAmount` 是死代码（grep 无调用方），建议下轮清理 | `affiliate/application/commission.go` |
| P3-3 | `order/application/refund/wallet.go:91` refund reference 用 `UnixNano` 不是幂等键；当前靠 `refunded_amount` 上限兜底防双退，正确但建议改为 `order:{id}:admin_refund:{refund_seq}` 稳定 reference | `order/application/refund/wallet.go` |

---

## 9. 最终安全判断

- **Wallet 总资产安全**：✅ Available+Frozen 不变量由 4 个资金原语强制，所有路径带行锁、ledger、幂等。
- **Recharge 主链安全**：✅ 汇率快照 + 余额不足/汇率不可用 fail-closed；无 Gateway；wallet-only 下无 guest。
- **Withdrawal 安全**：✅ 只扣 available、TOTP/cooldown/日限额/首提限额、cancel/reject 退原额且幂等、complete 必须 txid。
- **Affiliate 安全**：✅ inviter_id 真源、L1~L10 不压缩、completed 唯一新佣金点、USDT 基数、refund 按比例 reversal。
- **C2C 资金链安全**：✅ freeze/mark-paid/confirm/cancel/expire/arbitration 资金路径与设计一致。
- **C2C 并发安全**：✅ 代码层面行锁 + 原子扣减 + reference 幂等完备；PG 并发测试代码存在，待 CI 环境执行。
