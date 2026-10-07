# Affiliate PostgreSQL 锁顺序审计（AFFILIATE_PG_LOCK_ORDER）

- 审计目标：HCZ Go Affiliate Finance 在真实 PostgreSQL 下的加锁顺序，识别并消除反向锁序（可预测 deadlock）。
- 数据库：PostgreSQL 17.11，隔离级别 **Read Committed**（`default_transaction_isolation=read committed`）。
- 锁原语：GORM `clause.Locking{Strength:"UPDATE"}`（即 `SELECT ... FOR UPDATE`，行级排他锁）。
- 审计日期：2026-10-06。

## 1. 涉及表

| 表 | 说明 |
|---|---|
| `affiliate_withdraw_requests` | 提现申请单 |
| `affiliate_commissions` | 佣金记录 |
| `affiliate_commission_ledgers` | 佣金流水（append-only，reference 唯一） |
| `affiliate_profiles` | 推广档案 |
| `users` | 用户（inviter_id 链） |
| `wallet_accounts` | 用户钱包账户 |
| `wallet_transactions` | 钱包流水（reference 唯一） |
| `orders` / `order_refund_records` | 订单 / 退款单 |

## 2. 各操作加锁顺序（修复前）

锁用 `[表.行]` 表示，`INSERT` 表示插入，无标注为普通读。

### 2.1 ApplyWithdraw（用户申请提现，`affiliate.WithinTransaction`）
```
GetProfileByUserID(读)
→ sum ledger（读）
→ CreateWithdraw(INSERT withdraw_requests)
→ appendLedger(lock)  → [ledger: last row of profile P]   ← 先锁 ledger
→ ListAvailableCommissionsForUpdate(P) → [commissions: profile P, available]  ← 后锁 commissions
→ BatchUpdateCommissions
```
修复前顺序：**last_ledger(P) → commissions(profile)**。

### 2.2 PayWithdraw（管理端出金，`WithinCombinedTransaction`）
```
GetWithdrawByIDForUpdate → [withdraw_request row]
→ GetProfileByID(读), sum ledger(读)
→ wallet CreditInTransaction → [wallet_account(user)]
→ appendLedger(settle) → [ledger: last row of P]          ← 先锁 ledger
→ appendLedger(release)（同 profile，已持锁）
→ UpdateWithdraw
→ ListCommissionsByWithdrawIDForUpdate → [commissions: by withdraw_id]  ← 后锁 commissions
→ BatchUpdateCommissions
```
修复前顺序：**withdraw → wallet_account → last_ledger(P) → commissions(withdraw)**。

### 2.3 Refund（退款冲正，order 模块事务内调用 `HandleOrderRefunded`）
```
orders.GetByIDForUpdate → [order row]
→ orders.UpdateFields / INSERT order_refund_records
→ affiliate.HandleOrderRefunded:
    ListCommissionsByOrderForUpdate(order) → [commissions: by order_id, id asc]  ← 先锁 commissions
    → GetWithdrawByID(读)
    → appendLedger(reversal/debt) → [ledger: last row of P]                     ← 后锁 ledger
    → UpdateCommission
```
顺序：**order → commissions(order) → last_ledger(P)**。

### 2.4 HandleOrderCompleted（订单完成生成佣金）
```
幂等读 GetCommissionByOrderBeneficiaryLevel(读)（事务外）
→ WithinTransaction:
    BatchCreateCommissions(INSERT commissions)
    → appendLedger(credit) → [ledger: last row of P]
```
幂等兜底：`affiliate_commissions(order_id, commission_type, beneficiary_user_id, level)` 唯一索引；`appendLedger` 内 `reference` 唯一索引。

### 2.5 HandleOrderCanceled（订单取消冲正）
```
ListCommissionsByOrder(读)
→ WithinTransaction:
    GetWithdrawByID(读)
    → appendLedger(reversal/debt) → [ledger: last row of P]
    → UpdateCommission
```

## 3. 反向锁序识别（修复前）

| 资源对 | PayWithdraw | Refund | 是否反向 |
|---|---|---|---|
| commissions vs last_ledger(P) | last_ledger **先**，commissions **后** | commissions **先**，last_ledger **后** | **是（反向）** |
| commissions vs last_ledger(P) | ApplyWithdraw: last_ledger **先**，commissions **后** | commissions **先**，last_ledger **后** | **是（反向）** |

**可预测死锁场景（CASE 2：PayWithdraw ∥ Refund）：**
- Tx A(Pay)：持 withdraw、wallet_account、last_ledger(P)，等待 commissions。
- Tx B(Refund)：持 order、commissions，等待 last_ledger(P)。
- A 等 B 的 commissions；B 等 A 的 last_ledger → **循环等待 → PostgreSQL 报 40P01 deadlock**。
这是由反向锁序必然可复现的死锁，**不允许用 retry 掩盖**。

## 4. 修复（最小必要改动，已落地）

规范统一为：**commissions 行先于 last_ledger(P) 加锁**。

### 4.1 `internal/modules/affiliate/application/withdraw.go` — ApplyWithdraw
将 `ListAvailableCommissionsForUpdate(profileID)` 提前到余额计算与 ledger 写入之前：
```
[commissions: profile P, available] → sum ledger(读) → CreateWithdraw(INSERT) → appendLedger(lock)[last_ledger P] → BatchUpdateCommissions
```
附带收益：并发提现时第二笔在佣金行上串行，等待后读到已提交的 WITHDRAW_LOCK，**消除超锁（CASE 3）**。

### 4.2 `internal/modules/affiliate/application/withdraw.go` — payWithdrawInternal
将 `ListCommissionsByWithdrawIDForUpdate(withdrawID)` 提前到钱包入账之前：
```
[withdraw] → [commissions: by withdraw_id] → [wallet_account] → appendLedger[last_ledger P] → UpdateWithdraw → BatchUpdateCommissions
```

## 5. 修复后加锁顺序（canonical）

| 操作 | 加锁顺序 |
|---|---|
| ApplyWithdraw | commissions(profile,available) → withdraw(INSERT) → last_ledger(P) |
| PayWithdraw | withdraw → commissions(withdraw) → wallet_account → last_ledger(P) |
| Refund | order → commissions(order) → last_ledger(P) |
| HandleOrderCompleted | commissions(INSERT) → last_ledger(P) |

**不变量：所有路径均为 commissions 先于 last_ledger(P)。** 任意两条路径对 (commissions, last_ledger) 的相对顺序一致，不再存在循环等待。

## 6. 结论

- 修复前：存在 1 处可预测反向锁序（Pay/Apply vs Refund），CASE 2 必现 40P01。
- 修复后：canonical 顺序 commissions → last_ledger，无反向锁序。
- 幂等兜底（DB 唯一索引）：
  - `affiliate_commissions(order_id, commission_type, beneficiary_user_id, level)` → 防重复佣金（CASE 4）。
  - `affiliate_commission_ledgers.reference` 唯一 → 防重复 reversal/debt/settle（CASE 1/5）。
- 不依赖 retry 掩盖；仅保留对真正瞬时 40P01 的有限重试（≤3 次），且测试中显式计数。
