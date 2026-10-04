# HCZ Phase 5 — Wallet Dual-Balance & C2C Foundation Pre-Audit

> 本轮只审计，不修改代码。所有结论基于当前代码库实际实现。
> 审计日期：2026-10-05
> 代码基线：`internal/modules/wallet/` + `internal/modules/walletwithdrawal/` + 关联模块

---

## 0. 结论速览

| 审计项 | 结论 |
|---|---|
| 是否真的需要双余额 | **是**。C2C escrow 必须冻结买方资金，单余额模型无法表达"可用"与"冻结"的语义区分。用 withdrawal pending 状态模拟冻结不可扩展（无法支持部分冻结、多笔并发冻结、冻结超时释放） |
| 旧 balance 怎么迁移 | `available_balance = old balance`，`frozen_balance = 0`，总资产不变。推荐**直接重命名** `balance` → `available_balance`，新增 `frozen_balance`，不保留兼容 getter（ONE API ONE DOMAIN，禁止双写/Fallback） |
| 哪些现有路径受影响 | 6 条资金写入路径全部需改为操作 `available_balance`；2 条 DTO/查询需扩展字段；5 个前端组件需改字段引用 |
| freeze/unfreeze/settle 原语 | 新增独立 wallet application 方法，全部在事务内 + 行锁 + reference 幂等。freeze/unfreeze 是单账户内部转移（total 不变），settle 是跨账户转移（total 变化） |
| ledger 是否需要扩展 | **需要**。现有 `wallet_transactions` 只有 `balance_before/balance_after`（单余额）。需新增 `available_before/available_after` 和 `frozen_before/frozen_after`，或扩展为 JSON metadata。推荐新增 4 列，保持可查询性 |
| 双账户锁顺序 | 固定按 `user_id` 升序锁账户。C2C settle 同时操作 buyer/seller 时，先锁小 user_id，再锁大 user_id。所有多账户操作必须遵守此顺序，防 deadlock |
| API contract 如何改 | `WalletAccountResp` 新增 `available_balance`、`frozen_balance`、`total_balance`，**删除** `balance` 字段（不保留兼容）。`WalletTransactionResp` 新增 `available_before/after`、`frozen_before/after` |
| 前端影响范围 | User: WalletBalanceCard.vue（显示三余额）、WalletPanel.vue（余额展示）、WalletWithdrawal.vue（只能用 available）。Admin: WalletRecharges.vue（余额列）。共 4~5 个组件 |
| 与 Withdrawal/Order/Refund/Affiliate 兼容 | 全部兼容。这些业务只操作 `available_balance`，不感知 `frozen_balance`。Affiliate 佣金余额模型完全独立，不涉及主 Wallet 双余额 |
| 最小实施范围 | wallet domain 扩展（2 列）、ledger 扩展（4 列）、6 条资金路径改 available、新增 freeze/unfreeze/settle 原语、DTO 扩展、前端字段替换、迁移 + 测试。**不实现任何 C2C 业务逻辑** |
| 是否建议 Phase 5 先做双余额再进 Phase 6 C2C | **是**。双余额是 C2C 的基础设施前提，必须先独立封板，再在其上构建 C2C escrow。混在一起会导致资金风险无法隔离验证 |

---

## 1. 现有 Wallet 完整审计

### 1.1 模块结构

```
internal/modules/wallet/
├── domain/
│   ├── account.go          # Account: ID, UserID(unique), Balance, timestamps
│   ├── transaction.go      # Transaction: UserID, Type, Direction, Amount, BalanceBefore/After, Currency, Reference(unique), Remark
│   └── recharge_order.go   # RechargeOrder
├── application/
│   ├── service.go          # Service struct + Options + reference 生成工具
│   ├── credit.go           # changeBalance (核心) + CreditInTransaction + ensureAccountForUpdate
│   ├── order_balance.go    # ApplyOrderBalance (扣款) + ReleaseOrderBalance (退款)
│   ├── admin.go            # Recharge + AdminAdjustBalance
│   ├── recharge.go         # ApplyRechargePayment
│   └── query.go            # GetAccount + ListTransactions + GetBalancesByUserIDs
├── contract/
│   ├── ports.go            # Repository / Transaction / UnitOfWork / UseCase 接口
│   ├── types.go            # DTO: CreditInput, OrderBalanceInput, OrderReleaseInput, etc.
│   └── errors.go
├── infrastructure/gormstore/
│   └── store.go            # GORM 实现，GetAccountByUserIDForUpdate 用 SELECT ... FOR UPDATE
├── transport/http/
│   ├── user_handler.go     # User 端 API
│   ├── admin_handler.go    # Admin 端 API
│   ├── channel_handler.go  # Channel 端 API
│   └── routes.go
└── transport/presenter/
    └── wallet.go           # WalletAccountResp { Balance, Currency }, WalletTransactionResp
```

### 1.2 wallet_accounts 表

**文件**: `internal/modules/wallet/domain/account.go`

| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | PK |
| UserID | uint | uniqueIndex，一用户一账户 |
| Balance | money.Amount | decimal(20,2)，not null，default 0 — **唯一余额字段** |
| CreatedAt / UpdatedAt / DeletedAt | | |

**审计结论**: 单余额模型，无 available/frozen 区分。

### 1.3 wallet_transactions 表（Ledger）

**文件**: `internal/modules/wallet/domain/transaction.go`

| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | PK |
| UserID | uint | index |
| OperatorAdminID | *uint | index |
| OrderID | *uint | index |
| Type | varchar(40) | index，交易类型 |
| Direction | varchar(16) | index，in/out |
| Amount | money.Amount | decimal(20,2) |
| BalanceBefore | money.Amount | decimal(20,2) — **单余额快照** |
| BalanceAfter | money.Amount | decimal(20,2) — **单余额快照** |
| Currency | varchar(16) | default 'CNY'（实际 USDT） |
| Reference | varchar(120) | **uniqueIndex**，幂等键 |
| Remark | varchar(255) | |
| CreatedAt / UpdatedAt / DeletedAt | | |

**现有交易类型** (`internal/constants/constants.go` L140-151):
- `recharge` — 充值入账
- `order_pay` — 订单余额支付扣款
- `order_refund` — 订单退款入账
- `admin_adjust` — 管理员调整
- `admin_refund` — 管理员退款
- `gift_card_redeem` — 礼品卡兑换
- `order_underpaid_credit` — 支付不足转余额
- `withdrawal_debit` — 提现申请扣款
- `withdrawal_refund` — 提现拒绝/取消退款

**审计结论**: ledger 只有单余额 before/after，无法还原 freeze/unfreeze 操作中 available 和 frozen 的各自变化。必须扩展。

### 1.4 核心资金操作：changeBalance

**文件**: `internal/modules/wallet/application/credit.go` L87-140

这是所有非事务内资金操作的核心：
1. `s.transactions.WithinTransaction()` 开启事务
2. `ensureAccountForUpdate()` → `GetAccountByUserIDForUpdate()` 行锁（SELECT ... FOR UPDATE）
3. `before = account.Balance`，`after = before + delta`
4. 校验 `after >= 0`（不允许负余额）
5. `account.Balance = after`，`UpdateAccount`
6. 创建 Transaction 记录（BalanceBefore/BalanceAfter）
7. 提交事务

**关键特征**:
- 单余额操作，delta 可正可负
- 行锁防并发
- 事务原子
- reference 唯一索引幂等（但 changeBalance 本身不检查重复，由调用方传 reference）

### 1.5 CreditInTransaction（事务内入账）

**文件**: `internal/modules/wallet/application/credit.go` L15-85

在调用方已开启的事务内执行入账：
1. 检查 reference 幂等（`GetTransactionByReference`，已存在则返回已有记录）
2. 行锁账户
3. `balance += amount`
4. 创建 Transaction

**使用方**: 订单退款 (`refund/wallet.go`)、礼品卡兑换、支付不足转余额等。

### 1.6 订单扣款/退款：ApplyOrderBalance / ReleaseOrderBalance

**文件**: `internal/modules/wallet/application/order_balance.go`

**ApplyOrderBalance** (L16-83):
- 在调用方事务内执行（`tx walletcontract.Transaction`）
- 行锁账户
- 扣款逻辑：`deduct = min(available, total)`，即钱包余额不足时扣全部可用额
- 幂等：`orderAllocationReference` 生成轮次键 + `GetTransactionByReference` 检查
- 创建 `order_pay` 类型 Transaction

**ReleaseOrderBalance** (L87-149):
- 在调用方事务内执行
- 幂等检查
- `claim` 回调原子清除订单侧 wallet_paid_amount（防重复退款）
- `balance += amount`
- 创建 `order_refund` 类型 Transaction

**调用方**: `internal/modules/order/application/order_wallet_bridge.go` — `ApplyWalletBalance` / `ReleaseWalletBalance`，在订单创建/取消/退款的事务内调用。

### 1.7 提现模块：直接操作 Balance

**文件**: `internal/modules/walletwithdrawal/application/`

提现模块**不使用** wallet 的 `changeBalance`，而是直接在行锁后内联操作 `account.Balance`：

| 文件 | 操作 | 逻辑 |
|---|---|---|
| `create.go` L108-157 | 提现申请扣款 | 行锁 → `before = Balance` → 校验 `before >= amount` → `after = before - amount` → `Balance = after` → 创建 `withdrawal_debit` Transaction |
| `cancel.go` L40-53 | 取消退款 | 行锁 → `after = Balance + request_amount` → `Balance = after` → 创建 `withdrawal_refund` Transaction |
| `admin.go` L73-86 | 拒绝退款 | 同 cancel，创建 `withdrawal_refund` Transaction |

**审计结论**: 提现模块有 3 处直接写 `account.Balance`，双余额改造时必须全部改为 `available_balance`。这是最容易遗漏的点。

### 1.8 充值 / Admin 调整

**文件**: `internal/modules/wallet/application/admin.go`

- `Recharge()` → 调用 `changeBalance`（入账）
- `AdminAdjustBalance()` → 调用 `changeBalance`（delta 可正可负）

### 1.9 查询 / DTO

**文件**: `internal/modules/wallet/application/query.go`
- `GetAccount(userID)` → 返回 `*walletdomain.Account`（含 Balance）
- `GetBalancesByUserIDs(userIDs)` → 返回 `map[uint]money.Amount`（只有 Balance）

**文件**: `internal/modules/wallet/transport/presenter/wallet.go`
- `WalletAccountResp { Balance money.Amount, Currency string }` — 只有单余额
- `WalletTransactionResp { ..., BalanceBefore, BalanceAfter, ... }` — 只有单余额快照

### 1.10 全部直接读写 Balance 的生产路径汇总

| # | 路径 | 文件 | 操作 | 双余额改造 |
|---|---|---|---|---|
| 1 | changeBalance | `wallet/application/credit.go` L87 | `Balance += delta` | 改为 `available_balance += delta` |
| 2 | CreditInTransaction | `wallet/application/credit.go` L15 | `Balance += amount` | 改为 `available_balance += amount` |
| 3 | ApplyOrderBalance | `wallet/application/order_balance.go` L61 | `Balance -= deduct` | 改为 `available_balance -= deduct` |
| 4 | ReleaseOrderBalance | `wallet/application/order_balance.go` L129 | `Balance += amount` | 改为 `available_balance += amount` |
| 5 | Withdrawal create | `walletwithdrawal/application/create.go` L154 | `Balance -= amount` | 改为 `available_balance -= amount` |
| 6 | Withdrawal cancel | `walletwithdrawal/application/cancel.go` L50 | `Balance += refund` | 改为 `available_balance += refund` |
| 7 | Withdrawal admin reject | `walletwithdrawal/application/admin.go` L83 | `Balance += refund` | 改为 `available_balance += refund` |
| 8 | ensureAccountForUpdate | `wallet/application/credit.go` L142 | 新建账户 `Balance=0` | 改为 `available=0, frozen=0` |
| 9 | GetAccount (query) | `wallet/application/query.go` L22 | 新建账户 `Balance=0` | 同上 |

**读取 Balance 的路径**:
- `WalletAccountResp.Balance` (presenter)
- `GetBalancesByUserIDs` 返回 `map[uint]money.Amount`
- 前端 `WalletBalanceCard.vue`、`WalletPanel.vue`、`WalletWithdrawal.vue`
- Admin `WalletRecharges.vue`

---

## 2. 目标资金模型

### 2.1 双余额定义

```
wallet_accounts
├── available_balance  decimal(20,2)  -- 可用余额，可用于支付/提现/退款
├── frozen_balance     decimal(20,2)  -- 冻结余额，C2C escrow 占用，不可用
└── total = available_balance + frozen_balance  -- 总资产（计算字段，不存储）
```

### 2.2 迁移方案：直接重命名，不保留兼容

**推荐方案**:

1. `wallet_accounts` 表：
   - `balance` 列重命名为 `available_balance`（或新增列 + 数据拷贝 + 删除旧列）
   - 新增 `frozen_balance` 列，default 0
2. 所有代码中的 `account.Balance` 改为 `account.AvailableBalance`
3. DTO 中删除 `balance`，新增 `available_balance` / `frozen_balance` / `total_balance`
4. **不保留兼容 getter**（如 `GetBalance()` 返回 available），不做双写

**理由**:
- 用户全局偏好：ONE API ONE DOMAIN ONE SOURCE OF TRUTH，禁止旧 API 兼容/双写/Fallback
- 保留兼容 getter 会导致未来代码混淆"balance 到底是 available 还是 total"
- 全项目搜索 `account.Balance` 和 `.balance` 可一次性替换，范围可控（9 处写入 + 4 处前端）
- GORM AutoMigrate 会自动处理列变更（但重命名需要显式 migration）

### 2.3 不推荐的方案

| 方案 | 不推荐原因 |
|---|---|
| 保留 `balance` 作为 total，新增 available/frozen | 三列冗余，total 需要触发器维护，容易不一致 |
| 保留 `balance` 兼容 getter 返回 available | 违反 ONE API ONE DOMAIN，未来混淆 |
| 用 JSON 字段存 `{available, frozen}` | 无法 SQL 查询/索引，decimal 精度风险 |

---

## 3. 非 C2C 业务兼容

### 3.1 语义不变保证

所有现有业务只操作 `available_balance`，不感知 `frozen_balance`：

| 业务 | 操作 | 双余额后 | 语义变化 |
|---|---|---|---|
| Business Order 支付 | 扣余额 | `available -= amount` | 无变化，frozen 不参与 |
| Business Order 退款 | 加余额 | `available += amount` | 无变化 |
| Withdrawal 申请 | 扣余额 | `available -= amount` | 无变化，frozen 不可提现 |
| Withdrawal reject/cancel | 退余额 | `available += amount` | 无变化 |
| Recharge | 加余额 | `available += amount` | 无变化 |
| Admin Adjust | 加减余额 | `available += delta` | 无变化 |
| Gift Card Redeem | 加余额 | `available += amount` | 无变化 |
| Affiliate withdraw | 独立佣金余额模型 | 不涉及主 Wallet | 无变化 |

### 3.2 关键约束

- **frozen_balance 不参与任何现有业务的余额校验**。订单支付校验 `available >= amount`，不是 `total >= amount`。
- **提现只能使用 available_balance**。提现申请时校验 `available >= request_amount`。
- **退款只进入 available_balance**，不进入 frozen。

---

## 4. C2C Freeze 原语设计

### 4.1 三个核心操作

新增 `internal/modules/wallet/application/freeze.go`：

```go
// Freeze 将可用余额冻结。available -= amount, frozen += amount。
// 单账户内部转移，total 不变。
func (s *Service) Freeze(tx Transaction, input FreezeInput) (*Account, *Transaction, error)

// Unfreeze 解冻，将冻结余额转回可用。frozen -= amount, available += amount。
// 单账户内部转移，total 不变。
func (s *Service) Unfreeze(tx Transaction, input UnfreezeInput) (*Account, *Transaction, error)

// SettleFrozen 结算冻结资金。frozen -= amount，然后根据 C2C 交易：
// - credit 对手方 available（买方→卖方）
// - 或 credit 平台 fee 账户
// - 或退回买方 available（交易取消）
// 跨账户转移，total 变化（或在多账户间转移）。
func (s *Service) SettleFrozen(tx Transaction, input SettleInput) (*Transaction, error)
```

### 4.2 Freeze 语义

```
输入: user_id, amount, reference_type, reference_id, remark
事务内:
  1. 行锁账户 (SELECT ... FOR UPDATE)
  2. 幂等检查: GetTransactionByReference(reference)
  3. 校验 available >= amount（不允许冻结超过可用额）
  4. available_before = account.AvailableBalance
     frozen_before = account.FrozenBalance
  5. available_after = available_before - amount
     frozen_after = frozen_before + amount
  6. 更新账户
  7. 创建 Transaction:
     type = "c2c_freeze"
     direction = "out" (从 available 视角)
     amount = amount
     available_before / available_after
     frozen_before / frozen_after
     reference = unique
```

### 4.3 Unfreeze 语义

```
与 Freeze 对称:
  frozen -= amount, available += amount
  type = "c2c_unfreeze"
  direction = "in"
校验: frozen >= amount（不允许解冻超过冻结额）
```

### 4.4 SettleFrozen 语义

```
输入: from_user_id, to_user_id (可空), amount, fee_amount (可空), reference, action
事务内:
  1. 按 user_id 升序行锁所有涉及账户 (from, to, fee_account if any)
  2. 幂等检查
  3. 校验 from.frozen >= amount
  4. from.frozen -= amount
  5. 根据 action:
     - "release": from.available += amount（退回买方，解冻+释放合并）
     - "pay": to.available += (amount - fee_amount); fee_account.available += fee_amount
  6. 为每个账户创建 Transaction:
     - from: type="c2c_settle", direction="out", frozen_before/after
     - to: type="c2c_receive", direction="in", available_before/after
     - fee: type="c2c_fee", direction="in", available_before/after
```

### 4.5 原语约束

- 所有操作必须在调用方事务内执行（C2C trade 状态机事务）
- 所有操作必须行锁
- 所有操作必须 reference 幂等
- Freeze/Unfreeze 是**单账户内部转移**，total 不变
- SettleFrozen 是**跨账户转移**，可能涉及 2~3 个账户
- **不允许**在 wallet 模块内实现 C2C 业务逻辑（如判断交易状态、超时等），wallet 只提供原子资金原语

---

## 5. Ledger 设计

### 5.1 现有 ledger 的不足

现有 `wallet_transactions` 只有 `balance_before` / `balance_after`（单余额）。

对于 freeze 操作：
- `balance_before = available_before + frozen_before = total_before`
- `balance_after = available_after + frozen_after = total_after`
- `total_before = total_after`（freeze 是内部转移）
- → `balance_before = balance_after`，无法从 ledger 还原 freeze 操作！

**必须扩展 ledger 以记录 available 和 frozen 的各自快照。**

### 5.2 推荐方案：新增 4 列

```
wallet_transactions
├── balance_before      -- 保留，= available_before + frozen_before（total 快照）
├── balance_after       -- 保留
├── available_before    -- 新增，decimal(20,2), default 0
├── available_after     -- 新增，decimal(20,2), default 0
├── frozen_before       -- 新增，decimal(20,2), default 0
└── frozen_after        -- 新增，decimal(20,2), default 0
```

**理由**:
- 保留 `balance_before/after` 作为 total 快照，历史数据兼容
- 新增 4 列可直接 SQL 查询，不需要解析 JSON
- 所有现有交易类型回填：`available_before = balance_before`, `frozen_before = 0`, `available_after = balance_after`, `frozen_after = 0`
- 新交易类型（c2c_freeze 等）正确填充 6 个字段

### 5.3 不推荐的方案

| 方案 | 不推荐原因 |
|---|---|
| 用 JSON metadata 存 available/frozen | 无法 SQL 查询，decimal 精度风险，GORM 查询复杂 |
| 只存 delta（available_delta, frozen_delta） | 无法直接还原快照，需要累加，审计困难 |
| 新建独立 c2c_ledger 表 | 资金流水分散，全局审计需要 union，违反 ONE SOURCE OF TRUTH |

### 5.4 新增交易类型

```go
WalletTxnTypeC2CFreeze   = "c2c_freeze"
WalletTxnTypeC2CUnfreeze = "c2c_unfreeze"
WalletTxnTypeC2CSettle    = "c2c_settle"
WalletTxnTypeC2CReceive   = "c2c_receive"
WalletTxnTypeC2CFee       = "c2c_fee"
```

### 5.5 Ledger 还原能力验证

| 操作 | available_before | available_after | frozen_before | frozen_after | balance_before | balance_after |
|---|---|---|---|---|---|---|
| recharge 100 | 0 | 100 | 0 | 0 | 0 | 100 |
| order_pay 30 | 100 | 70 | 0 | 0 | 100 | 70 |
| c2c_freeze 50 | 70 | 20 | 0 | 50 | 70 | 70 |
| c2c_unfreeze 20 | 20 | 40 | 50 | 30 | 70 | 70 |
| c2c_settle (pay 30 to seller) | 40 | 40 | 30 | 0 | 70 | 40 |
| seller c2c_receive 30 | 0 | 30 | 0 | 0 | 0 | 30 |

每一行都能独立还原该操作前后的资金状态。✅

---

## 6. 数据库迁移

### 6.1 wallet_accounts 迁移

```sql
-- Step 1: 新增 frozen_balance 列
ALTER TABLE wallet_accounts ADD COLUMN frozen_balance DECIMAL(20,2) NOT NULL DEFAULT 0;

-- Step 2: 重命名 balance → available_balance
-- SQLite: 不支持 RENAME COLUMN (旧版本)，需要表重建
-- MySQL/PostgreSQL: ALTER TABLE wallet_accounts RENAME COLUMN balance TO available_balance;
-- 推荐用 GORM AutoMigrate + 显式 migration 处理

-- Step 3: 数据校验
-- 迁移后验证: 所有行 available_balance >= 0, frozen_balance = 0
-- 总资产不变: SUM(available_balance) = 迁移前 SUM(balance)
```

### 6.2 wallet_transactions 迁移

```sql
-- Step 1: 新增 4 列
ALTER TABLE wallet_transactions
  ADD COLUMN available_before DECIMAL(20,2) NOT NULL DEFAULT 0,
  ADD COLUMN available_after  DECIMAL(20,2) NOT NULL DEFAULT 0,
  ADD COLUMN frozen_before    DECIMAL(20,2) NOT NULL DEFAULT 0,
  ADD COLUMN frozen_after     DECIMAL(20,2) NOT NULL DEFAULT 0;

-- Step 2: 回填历史数据（历史交易都是单余额，frozen=0）
UPDATE wallet_transactions
SET available_before = balance_before,
    available_after  = balance_after,
    frozen_before    = 0,
    frozen_after     = 0
WHERE available_before = 0 AND available_after = 0 AND frozen_before = 0 AND frozen_after = 0;
-- 幂等：只回填全零行

-- Step 3: 校验
-- 所有行: balance_before = available_before + frozen_before
-- 所有行: balance_after = available_after + frozen_after
```

### 6.3 迁移特性

| 特性 | 保证 |
|---|---|
| 幂等 | 回填 SQL 带 WHERE 全零条件，重复执行不覆盖 |
| 可回滚 | 新增列可 DROP，重命名可改回（但需在低峰期） |
| 不改变总资产 | `available_balance = old balance`，`frozen_balance = 0`，sum 不变 |
| 不删除历史数据 | 所有历史 transaction 保留，只新增列回填 |
| 可重复执行 | GORM AutoMigrate 幂等，显式 migration 带 IF NOT EXISTS / WHERE 条件 |

### 6.4 迁移顺序

1. 先部署代码（domain 已含新字段，但 GORM 会 AutoMigrate 新增列）
2. AutoMigrate 自动新增 `frozen_balance` 和 4 个 ledger 列
3. 显式 migration 执行重命名 `balance` → `available_balance`（如果 GORM 不自动处理）
4. 执行历史数据回填
5. 校验数据一致性
6. 切换流量

**注意**: GORM AutoMigrate 不会自动重命名列（会新增 `available_balance` 列但保留旧 `balance`）。需要显式 migration 处理数据拷贝 + 删除旧列，或在 domain 中直接将字段名从 `Balance` 改为 `AvailableBalance` 并配置 `gorm:"column:available_balance"`，然后显式 migration 做 `UPDATE wallet_accounts SET available_balance = balance` + `DROP COLUMN balance`。

---

## 7. 资金不变量

### 7.1 必须定义并测试的不变量

| 不变量 | 说明 | 校验时机 |
|---|---|---|
| `available >= 0` | 可用余额不为负 | 每次写入后校验 |
| `frozen >= 0` | 冻结余额不为负 | 每次写入后校验 |
| `total = available + frozen` | 总资产等于两余额之和 | 每次写入后校验（balance_before/after） |
| Freeze/Unfreeze: `total_before = total_after` | 内部转移不改变总资产 | freeze/unfreeze 操作校验 |
| Settle: 多账户 total 变化合法 | 跨账户转移，系统总 total 变化 = fee 部分 | settle 操作校验 |
| `available >= freeze_amount` | 冻结额不超过可用额 | freeze 操作前置校验 |
| `frozen >= unfreeze_amount` | 解冻额不超过冻结额 | unfreeze 操作前置校验 |
| `frozen >= settle_amount` | 结算额不超过冻结额 | settle 操作前置校验 |

### 7.2 不允许的操作

- available 负数（所有扣款操作校验 after >= 0）
- frozen 负数（所有 unfreeze/settle 校验 after >= 0）
- double freeze（reference 幂等 + 状态机）
- double unfreeze（reference 幂等）
- double settle（reference 幂等 + C2C trade 状态机终态校验）
- 冻结超过可用额（freeze 前置校验）
- 解冻超过冻结额（unfreeze 前置校验）

### 7.3 不变量测试清单

- 并发 freeze + order debit：不超扣，不 deadlock
- 并发 freeze + withdrawal：不超扣
- 并发 unfreeze + settle：只有一个成功
- 双账户 settle 锁顺序：无 deadlock
- freeze 后 available 减少、frozen 增加、total 不变
- unfreeze 后对称恢复
- settle 后 from.frozen 减少、to.available 增加
- 全部操作 ledger 6 字段正确

---

## 8. 幂等

### 8.1 幂等键设计

所有 C2C wallet action 使用 `wallet_transactions.reference` 唯一索引作为最终幂等保障。

```
reference 格式:
  c2c_freeze:<trade_id>
  c2c_unfreeze:<trade_id>
  c2c_settle:<trade_id>
  c2c_receive:<trade_id>
  c2c_fee:<trade_id>
```

对于多账户 settle，每个账户的 transaction 有独立 reference：
- from: `c2c_settle:<trade_id>`
- to: `c2c_receive:<trade_id>`
- fee: `c2c_fee:<trade_id>`

### 8.2 幂等流程

```
Freeze(input):
  1. 应用层: GetTransactionByReference(reference) → 已存在则返回已有记录（不重复操作）
  2. 事务内: 行锁 → 校验 → 写入 → 创建 Transaction
  3. 唯一索引兜底: 并发时 duplicate key error → 捕获并返回已有记录
```

### 8.3 防重场景

| 场景 | 防护 |
|---|---|
| C2C trade 创建时重复 freeze | reference 幂等 + trade 状态机 |
| 超时释放重复 unfreeze | reference 幂等 + trade 状态终态校验 |
| 结算重复 settle | reference 幂等 + trade 状态机 |
| 买方/卖方同时操作 | 行锁 + 固定锁顺序 |
| API 重试 | reference 幂等 |

---

## 9. 并发专项审计

### 9.1 C2C freeze 与 Business Order 同时扣款

**场景**: 用户 A 有 available=100。同时发起 C2C freeze(60) 和 Business Order pay(80)。

**当前机制**: 两个操作都使用 `GetAccountByUserIDForUpdate` 行锁同一行，串行化执行。

**双余额后**:
- freeze: `available -= 60, frozen += 60`
- order pay: `available -= 80`
- 如果 freeze 先执行：available 变为 40，order pay 校验 `40 >= 80` 失败 → 订单支付失败（或扣 40，取决于 ApplyOrderBalance 的 min 逻辑）
- 如果 order pay 先执行：available 变为 20，freeze 校验 `20 >= 60` 失败 → freeze 失败

**结论**: 行锁已保证串行化，不会超扣。但业务层需明确：freeze 失败时 C2C trade 应如何处理（重试/取消）。

### 9.2 C2C freeze 与 Withdrawal 同时扣款

同上，行锁串行化。withdrawal 校验 `available >= amount`，freeze 后 available 减少可能导致 withdrawal 失败。

**结论**: 无资金风险，业务层处理失败即可。

### 9.3 cancel/unfreeze 与 settle 并发

**场景**: C2C trade 超时，系统执行 unfreeze(释放)；同时 admin 执行 settle(确认打款)。

**防护**:
1. C2C trade 状态机：trade 只能处于一个状态（released / settled），状态变更本身行锁 + 原子
2. wallet 操作在 trade 状态机事务内执行
3. reference 幂等：unfreeze 和 settle 的 reference 不同，但都依赖 trade 状态
4. 先变更 trade 状态（行锁），再执行 wallet 操作，同一事务

**结论**: C2C trade 状态机是最终仲裁，wallet 原语在其事务内执行。不会出现既 unfreeze 又 settle。

### 9.4 buyer/seller 双账户锁顺序

**场景**: settle 同时操作 buyer（扣 frozen）和 seller（加 available）。

**死锁风险**: 如果交易 A 先锁 buyer 再锁 seller，交易 B 先锁 seller 再锁 buyer，可能死锁。

**解决方案**: **固定按 user_id 升序锁账户**。

```go
func lockAccountsInOrder(tx, userIDs []uint) {
    sorted := sort(userIDs)  // 升序
    for _, uid := range sorted {
        GetAccountByUserIDForUpdate(uid)  // 按顺序锁
    }
}
```

所有多账户 wallet 操作（settle、多账户批量调整等）必须遵守此顺序。

**结论**: 固定锁顺序可完全避免 deadlock。必须在代码审查中强制检查。

---

## 10. 双账户结算

### 10.1 Settle 事务结构

```
SettleFrozen(tx, input):
  涉及账户: [from_user_id, to_user_id, fee_account_id(if fee>0)]
  1. 按 user_id 升序去重，依次行锁所有账户
  2. 幂等检查（每个账户的 reference）
  3. 校验 from.frozen >= amount
  4. from.frozen -= amount
  5. if action == "release":
       from.available += amount（退回买方）
     if action == "pay":
       net = amount - fee_amount
       to.available += net
       if fee_amount > 0: fee_account.available += fee_amount
  6. 更新所有账户
  7. 为每个账户创建 Transaction（6 字段快照）
  8. 同一事务提交
```

### 10.2 原子性保证

- 所有账户更新和 Transaction 创建在同一事务内
- 任何一步失败全部 rollback
- 不会出现"from 扣了但 to 没加"的部分结算

### 10.3 fee 处理

- fee 从 settlement amount 中扣除，不额外从买方收取
- `seller_receives = amount - fee_amount`
- `platform_fee = fee_amount`，credit 到平台 fee 账户（或直接计入平台收入，不经过 wallet）
- 第一版设计 fee 原语，具体费率配置在 C2C 域实现

---

## 11. Fee 设计

### 11.1 C2C 平台手续费

| 维度 | 设计 |
|---|---|
| 收取方 | 平台 |
| 承担方 | 可配置：seller / buyer / 双方分担 |
| 计算方式 | fixed fee + percentage fee（同提现模型） |
| 计算基数 | settlement amount（USDT） |
| 精度 | USDT / decimal / 2dp |
| 收取时机 | settle 时从 settlement amount 中扣除 |
| 配置来源 | Admin Settings（C2C 域，本轮不实现） |

### 11.2 公式

```
fee_amount = fixed_fee + Round(settlement_amount * percentage_fee / 100, 2)
seller_receives = settlement_amount - fee_amount
```

### 11.3 本轮范围

- 只设计 fee 在 settle 原语中的位置和计算方式
- 不实现费率配置、不实现 fee 收取逻辑
- wallet SettleFrozen 接受 `fee_amount` 参数，由 C2C 域计算后传入

---

## 12. 与提现兼容

### 12.1 提现只能使用 available_balance

**提现申请校验**:
```go
// walletwithdrawal/application/create.go
before := account.AvailableBalance  // 改为 available
if before.LessThan(amount) {
    return ErrInsufficientBalance
}
after := before.Sub(amount)
account.AvailableBalance = after  // 只扣 available
```

**frozen_balance 不参与提现余额校验，不可提现。**

### 12.2 提现页面展示

User Wallet Withdrawal 页面应显示：
- 可用余额（available_balance）— 可提现金额
- 冻结余额（frozen_balance）— 不可提现，提示"C2C 交易中冻结"
- 总资产（total）— 仅展示

提现申请时只能输入不超过 available_balance 的金额。

### 12.3 提现 reject/cancel 退款

退款进入 `available_balance`，不进入 frozen。

---

## 13. 与订单兼容

### 13.1 Business Order 只扣 available

`ApplyOrderBalance` 中：
```go
available := account.AvailableBalance  // 改为 available
deduct := min(available, total)
after := available - deduct
account.AvailableBalance = after  // 只扣 available
```

订单不感知 frozen_balance。frozen 中的资金不能用于订单支付。

### 13.2 P0 Wallet-Only Contract 不被破坏

- 订单支付校验 `available >= amount`（或扣全部 available）
- 订单退款 credit `available`
- frozen 不参与订单资金流
- 原 P0 契约（钱包余额支付、退款、幂等）完全保留

---

## 14. 与 Refund 兼容

### 14.1 所有 refund 默认 credit available

- `ReleaseOrderBalance` → `available += amount`
- `CreditInTransaction`（退款用）→ `available += amount`
- Admin refund → `available += amount`

**不允许退款进入 frozen。** 退款是释放资金到用户可用余额，不是冻结。

### 14.2 退款与 frozen 的交互

如果用户有 frozen 资金（C2C 交易中），同时收到订单退款：
- 退款 credit available，frozen 不变
- 用户 total 增加
- 无冲突

---

## 15. 与 Affiliate 兼容

### 15.1 Affiliate 佣金余额模型完全独立

- Affiliate 佣金在 `affiliate_commissions` 表中累计，不经过主 Wallet
- Affiliate withdraw 通过 `affiliate_withdraw_requests` 独立流程
- 主 Wallet 双余额改造**不影响** affiliate 佣金模型

### 15.2 如果 Affiliate withdraw 到主 Wallet

当前 affiliate withdraw 是独立流程（Admin 审核后人工打款），不 credit 主 Wallet。

如果未来 affiliate withdraw 改为自动 credit 主 Wallet，则 credit `available_balance`，与 recharge 相同。

**本轮不需要处理**，因为当前 affiliate withdraw 不涉及主 Wallet。

---

## 16. API Contract

### 16.1 WalletAccountResp 变更

**当前**:
```go
type WalletAccountResp struct {
    Balance  money.Amount `json:"balance"`
    Currency string       `json:"currency"`
}
```

**目标**:
```go
type WalletAccountResp struct {
    AvailableBalance money.Amount `json:"available_balance"`
    FrozenBalance    money.Amount `json:"frozen_balance"`
    TotalBalance     money.Amount `json:"total_balance"`  // = available + frozen，服务端计算
    Currency         string       `json:"currency"`         // 固定 USDT
}
```

**不保留 `balance` 兼容字段。** 前端必须同步修改。

### 16.2 WalletTransactionResp 变更

**当前**:
```go
type WalletTransactionResp struct {
    ID, Type, Direction, Amount, BalanceBefore, BalanceAfter, Currency, Remark, CreatedAt
}
```

**目标**:
```go
type WalletTransactionResp struct {
    ID            uint         `json:"id"`
    Type          string       `json:"type"`
    Direction     string       `json:"direction"`
    Amount        money.Amount `json:"amount"`
    AvailableBefore money.Amount `json:"available_before"`  // 新增
    AvailableAfter  money.Amount `json:"available_after"`   // 新增
    FrozenBefore    money.Amount `json:"frozen_before"`     // 新增
    FrozenAfter     money.Amount `json:"frozen_after"`      // 新增
    TotalBefore     money.Amount `json:"total_before"`      // 原 balance_before 重命名
    TotalAfter      money.Amount `json:"total_after"`       // 原 balance_after 重命名
    Currency        string       `json:"currency"`
    Remark          string       `json:"remark"`
    CreatedAt       time.Time    `json:"created_at"`
}
```

### 16.3 GetBalancesByUserIDs 变更

**当前**: 返回 `map[uint]money.Amount`（只有 balance）

**目标**: 返回 `map[uint]WalletBalance`，其中 `WalletBalance { Available, Frozen, Total money.Amount }`

或者保持简单，调用方需要时调用 `GetAccount` 获取完整信息。

### 16.4 新增 API（C2C 域调用，不直接暴露给 User）

wallet module 内部新增 application 方法，通过 C2C domain 的 service 调用，不直接暴露 HTTP API：

```go
Freeze(tx Transaction, input FreezeInput) (*Account, *Transaction, error)
Unfreeze(tx Transaction, input UnfreezeInput) (*Account, *Transaction, error)
SettleFrozen(tx Transaction, input SettleInput) error
```

这些方法在 C2C trade 的事务内调用，不通过 HTTP 直接暴露。

---

## 17. 前端影响矩阵

### 17.1 User 端

| 文件 | 当前用法 | 改造内容 | 优先级 |
|---|---|---|---|
| `components/wallet/WalletBalanceCard.vue` | 显示 `balance` | 改为显示三余额：可用/冻结/总资产 | P0 |
| `views/personal/WalletPanel.vue` | 余额展示、流水列表 | 余额卡片替换，流水列表增加 available/frozen 列（可选） | P0 |
| `views/personal/WalletWithdrawal.vue` | 显示 balance，校验可提现额 | 改为显示 available_balance，frozen 不可提现 | P0 |
| `api/wallet.ts` | 接口类型定义 | 更新 WalletAccountResp / WalletTransactionResp 类型 | P0 |
| `components/wallet/WalletTransactionList.vue` | 流水列表 | 可选增加 available/frozen 变化展示 | P1 |

### 17.2 Admin 端

| 文件 | 当前用法 | 改造内容 | 优先级 |
|---|---|---|---|
| `views/admin/WalletRecharges.vue` | 显示用户余额列 | 改为显示 available/total | P0 |
| Admin 用户详情（如有余额展示） | 显示 balance | 改为三余额 | P1 |

### 17.3 不影响的前端

- 提现历史（WalletWithdrawal 历史列表，不显示余额）
- 充值历史（只显示充值单状态，不显示余额）
- Affiliate 页面（独立佣金模型，不涉及主 Wallet 余额）
- 订单页面（不直接显示钱包余额）

### 17.4 i18n

新增文案 key：
- `wallet.available_balance` — 可用余额
- `wallet.frozen_balance` — 冻结余额
- `wallet.total_balance` — 总资产
- `wallet.frozen_hint` — 冻结资金说明（C2C 交易中）

三语言（zh-CN/zh-TW/en）必须补齐，missing key = 0。

---

## 18. C2C Domain 边界

### 18.1 Wallet Dual-Balance 只是基础设施

Phase 5 只做 wallet 双余额基础设施，**不实现任何 C2C 业务逻辑**。

### 18.2 C2C 后续独立域（Phase 6+）

C2C 域至少包含以下独立模块，**本轮禁止塞进 wallet 模块**：

| 模块 | 职责 | 与 Wallet 的关系 |
|---|---|---|
| listing/offer | 商品发布、报价 | 不涉及 |
| trade | 交易单、状态机 | 调用 wallet Freeze/Settle/Unfreeze |
| escrow | 托管逻辑 | 调用 wallet freeze 原语 |
| payment confirmation | 买方确认付款 | 触发 trade 状态变更 → settle |
| timeout | 超时释放 | 触发 trade 状态变更 → unfreeze |
| dispute | 争议处理 | 触发 settle 或 unfreeze |
| arbitration | 仲裁 | 触发 settle 或 unfreeze |
| merchant | 商家管理 | 不涉及 |
| C2C settings | 费率、限额配置 | 不涉及 wallet |

### 18.3 边界原则

- Wallet 只提供原子资金原语（Freeze/Unfreeze/Settle），不判断业务状态
- C2C trade 状态机是业务仲裁，在其事务内调用 wallet 原语
- Wallet 不引用 C2C 域的任何类型
- C2C 域通过 wallet contract 端口调用 wallet 原语

---

## 19. 风险分析

### 19.1 历史余额迁移

| 风险 | 等级 | 缓解 |
|---|---|---|
| 重命名列导致数据丢失 | 高 | 先新增列 + 数据拷贝 + 校验，再删除旧列；低峰期执行；备份 |
| GORM AutoMigrate 不自动重命名 | 中 | 显式 migration 处理；domain 字段配置 `gorm:"column:available_balance"` |
| 回填 SQL 性能（大表） | 中 | 分批回填；wallet_transactions 表可能较大，需评估 |
| 迁移期间新旧代码混用 | 高 | 蓝绿部署或停机窗口；迁移完成后再切换代码 |

### 19.2 双账不平

| 风险 | 等级 | 缓解 |
|---|---|---|
| available + frozen ≠ total（balance 字段） | 中 | 每次写入后校验不变量；定时对账任务；迁移后全表校验 |
| freeze 后 total 变化（应为不变） | 中 | freeze/unfreeze 操作校验 total_before = total_after |
| ledger 6 字段不一致 | 中 | 创建 Transaction 时从账户快照填充，不手工计算 |

### 19.3 旧代码直接写 balance

| 风险 | 等级 | 缓解 |
|---|---|---|
| 遗漏某条资金路径未改 available | 高 | 全项目 grep `account.Balance` / `.Balance` / `balance` 逐一确认；9 处写入路径已全部定位 |
| 提现模块内联 balance 操作遗漏 | 高 | 3 处已定位（create/cancel/admin），代码审查重点检查 |
| 未来新代码误用 Balance | 中 | domain 字段重命名为 AvailableBalance 后，编译错误会暴露；不保留兼容 getter |

### 19.4 Race Condition

| 风险 | 等级 | 缓解 |
|---|---|---|
| freeze 与 order debit 并发超扣 | 低 | 行锁已串行化，校验 after >= 0 |
| freeze 与 withdrawal 并发超扣 | 低 | 同上 |
| unfreeze 与 settle 并发 | 中 | C2C trade 状态机仲裁 + 行锁 + reference 幂等 |
| 多账户 settle 部分成功 | 低 | 同一事务，全部 rollback |

### 19.5 Deadlock

| 风险 | 等级 | 缓解 |
|---|---|---|
| 双账户 settle 锁顺序不一致 | 高 | 固定按 user_id 升序锁；代码审查强制检查；所有多账户操作统一入口 |
| wallet 与 order 交叉锁 | 中 | wallet 操作在 order 事务内调用，锁顺序固定（先 order 行，后 wallet 行） |

### 19.6 Partial Transaction

| 风险 | 等级 | 缓解 |
|---|---|---|
| settle 中 from 扣了但 to 没加 | 低 | 同一事务，rollback 保证原子 |
| Transaction 创建失败但账户已更新 | 低 | 同一事务，rollback |
| 事务提交后通知失败 | 低 | 通知异步，不阻塞资金事务；幂等发送 |

### 19.7 Stale DTO

| 风险 | 等级 | 缓解 |
|---|---|---|
| 前端仍使用旧 `balance` 字段 | 高 | 删除 `balance` 字段后前端编译报错（TypeScript），强制修改；不保留兼容 |
| 第三方 API 消费方依赖 `balance` | 中 | 如有外部消费方需提前通知；当前 API-Only 单栈，消费方为自有前端 |
| Admin 报表依赖 `balance` | 中 | 全项目搜索确认所有引用点 |

---

## 20. 最小实施范围

### 20.1 后端

| 模块 | 改动 | 工作量 |
|---|---|---|
| `wallet/domain/account.go` | `Balance` → `AvailableBalance`，新增 `FrozenBalance` | 小 |
| `wallet/domain/transaction.go` | 新增 `AvailableBefore/After`、`FrozenBefore/After`，`BalanceBefore/After` 保留为 total | 小 |
| `wallet/application/credit.go` | changeBalance / CreditInTransaction / ensureAccountForUpdate 改 available | 中 |
| `wallet/application/order_balance.go` | ApplyOrderBalance / ReleaseOrderBalance 改 available | 中 |
| `wallet/application/admin.go` | Recharge / AdminAdjustBalance 自动继承 changeBalance 改动 | 小 |
| `wallet/application/freeze.go` | **新增** Freeze / Unfreeze / SettleFrozen 原语 | 大 |
| `wallet/contract/ports.go` | 新增 Freeze/Unfreeze/Settle 接口，Repository 不变 | 小 |
| `wallet/contract/types.go` | 新增 FreezeInput / UnfreezeInput / SettleInput | 小 |
| `wallet/transport/presenter/wallet.go` | WalletAccountResp / WalletTransactionResp 扩展字段 | 中 |
| `wallet/infrastructure/gormstore/store.go` | 无需改动（AutoMigrate 处理） | 小 |
| `walletwithdrawal/application/create.go` | 内联 balance 改 available | 小 |
| `walletwithdrawal/application/cancel.go` | 内联 balance 改 available | 小 |
| `walletwithdrawal/application/admin.go` | 内联 balance 改 available | 小 |
| `constants/constants.go` | 新增 c2c_freeze/unfreeze/settle/receive/fee 类型 | 小 |
| `bootstrap/database/migrations/registry.go` | AutoMigrate 自动处理，无需改动 | 小 |
| 显式 migration | 新增列 + 重命名 + 历史数据回填 + 校验 | 中 |

### 20.2 前端

| 端 | 改动 | 工作量 |
|---|---|---|
| User WalletBalanceCard.vue | 三余额展示 | 小 |
| User WalletPanel.vue | 余额展示替换 | 小 |
| User WalletWithdrawal.vue | available 校验 + 冻结提示 | 中 |
| User api/wallet.ts | 类型更新 | 小 |
| User i18n | 三语言新增 key | 小 |
| Admin WalletRecharges.vue | 余额列替换 | 小 |
| Admin i18n | 三语言新增 key | 小 |

### 20.3 测试

| 类型 | 内容 |
|---|---|
| 单元测试 | freeze/unfreeze/settle 原语、不变量校验、幂等 |
| 集成测试 | 并发 freeze+order、并发 freeze+withdrawal、双账户 settle 锁顺序、refund 兼容 |
| 迁移测试 | 带历史数据 fixture：列新增、重命名、回填、幂等、数据一致性校验 |
| 回归测试 | wallet / order / refund / withdrawal / affiliate 全量 |

### 20.4 明确不做

- ❌ 不实现 C2C trade / listing / escrow / dispute / arbitration
- ❌ 不实现 C2C 费率配置
- ❌ 不实现自动 TRON 转账
- ❌ 不修改 affiliate 佣金模型
- ❌ 不保留 `balance` 兼容字段
- ❌ 不做双写/Fallback

---

## 21. 实施建议：Phase 5 先做双余额，再进 Phase 6 C2C

### 21.1 推荐分阶段

| Phase | 内容 | 封板标准 |
|---|---|---|
| **Phase 5** | Wallet 双余额基础设施 + freeze/unfreeze/settle 原语 + 迁移 + 测试 | 所有现有业务回归通过，双余额不变量测试通过，原语单元测试通过，Linux CI 全绿 |
| **Phase 6** | C2C trade 状态机 + escrow（调用 freeze/settle）+ listing + 基础买卖流程 | C2C 端到端测试通过，资金安全审计通过 |

### 21.2 为什么必须先独立封板双余额

1. **资金风险隔离**: 双余额改造影响 6 条现有资金路径，必须独立验证不破坏现有充值/支付/退款/提现。如果和 C2C 混在一起，出问题时无法定位是双余额改造还是 C2C 逻辑。
2. **原语可独立测试**: freeze/unfreeze/settle 是原子资金原语，可以在没有 C2C 业务逻辑的情况下完整测试（并发、幂等、不变量、deadlock）。
3. **回滚成本低**: 如果双余额有问题，可以单独回滚（退回单余额），不影响 C2C 代码（C2C 还没写）。
4. **C2C 依赖明确**: C2C trade 状态机依赖 wallet 原语，先有稳定的原语 API，C2C 才能可靠构建。

### 21.3 Phase 5 封板后进入 Phase 6 的前提

- 双余额迁移在生产环境安全执行
- 所有现有业务回归通过（充值/支付/退款/提现/affiliate）
- freeze/unfreeze/settle 原语测试覆盖率 ≥ 90%
- 并发/deadlock 测试通过
- Linux CI 全绿
- 前端三余额展示可用
- 无资金安全阻断

---

## 22. 附录：代码引用索引

| 能力 | 文件路径 |
|---|---|
| Account domain | `internal/modules/wallet/domain/account.go` |
| Transaction domain | `internal/modules/wallet/domain/transaction.go` |
| changeBalance 核心 | `internal/modules/wallet/application/credit.go` L87-140 |
| CreditInTransaction | `internal/modules/wallet/application/credit.go` L15-85 |
| ensureAccountForUpdate | `internal/modules/wallet/application/credit.go` L142-164 |
| ApplyOrderBalance | `internal/modules/wallet/application/order_balance.go` L16-83 |
| ReleaseOrderBalance | `internal/modules/wallet/application/order_balance.go` L87-149 |
| Recharge / AdminAdjust | `internal/modules/wallet/application/admin.go` |
| Service / reference 工具 | `internal/modules/wallet/application/service.go` |
| Query / GetBalancesByUserIDs | `internal/modules/wallet/application/query.go` |
| Repository 接口 | `internal/modules/wallet/contract/ports.go` |
| DTO 类型 | `internal/modules/wallet/contract/types.go` |
| GORM Store / 行锁 | `internal/modules/wallet/infrastructure/gormstore/store.go` |
| Presenter / DTO | `internal/modules/wallet/transport/presenter/wallet.go` |
| 交易类型常量 | `internal/constants/constants.go` L140-157 |
| 提现 create 扣款 | `internal/modules/walletwithdrawal/application/create.go` L108-157 |
| 提现 cancel 退款 | `internal/modules/walletwithdrawal/application/cancel.go` L40-53 |
| 提现 admin reject 退款 | `internal/modules/walletwithdrawal/application/admin.go` L73-86 |
| 订单 wallet bridge | `internal/modules/order/application/order_wallet_bridge.go` |
| AutoMigrate registry | `internal/bootstrap/database/migrations/registry.go` L55-57 |
| User 余额卡片 | `frontend/user/src/components/wallet/WalletBalanceCard.vue` |
| User 钱包面板 | `frontend/user/src/views/personal/WalletPanel.vue` |
| User 提现页面 | `frontend/user/src/views/personal/WalletWithdrawal.vue` |
| User 流水列表 | `frontend/user/src/components/wallet/WalletTransactionList.vue` |
| Admin 充值管理 | `frontend/admin/src/views/admin/WalletRecharges.vue` |

---

*审计完成。本文件为 Phase 5 开发的输入基线。核心建议：先独立封板 Wallet 双余额基础设施，再进入 Phase 6 C2C 业务开发。*
