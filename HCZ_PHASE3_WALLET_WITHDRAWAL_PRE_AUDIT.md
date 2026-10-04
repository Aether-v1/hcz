# HCZ Phase 3 — Wallet Withdrawal Pre-Audit

> 本轮仅审计，不修改代码。所有结论基于当前代码库实际实现。
> 审计日期：2026-10-04
> 代码基线：`internal/modules/wallet/` 全模块 + 关联模块

---

## 0. 结论速览

| 审计项 | 结论 |
|---|---|
| 当前 Wallet 是否适合直接支持提现 | **适合，无需重构**。现有 `changeBalance()` + row lock + transaction + reference 唯一索引已覆盖提现扣款核心需求 |
| 是否需要 frozen balance | **第一版不需要**。采用方案 A（申请即扣款 + reject 退款），对现有 Wallet 改动最小 |
| 第一版扣款时机 | **申请时直接扣除** `request_amount`，reject/cancel 时全额退回 |
| 手续费模型 | `request_amount` − `fee_amount` = `net_amount`，服务端计算，前端仅展示 |
| 2FA 方案 | **复用现有 User TOTP**，提现提交时强制验证；不引入独立提现密码；fail-closed |
| Admin 审批流程 | pending → approved → processing → completed，rejected/canceled 为终态；全部财务写操作走 Admin JWT + RBAC + Compliance + Reason + Audit |
| 第一版链上打款 | **推荐手动打款**。Admin 人工转 USDT 后回填 txid + mark completed；不引入 hot wallet / Tron API |
| 与 C2C 双余额是否解耦 | **完全解耦**。C2C 是独立 Phase，提现不依赖双余额；未来 C2C 上线后再适配扣款来源 |
| 最小实施范围 | 新增 `wallet_withdrawals` 表 + withdrawal 模块（domain/application/transport）+ 3 个 User API + 5 个 Admin API + Settings 配置 + 2FA 校验 + Ledger 类型扩展 |

---

## 1. 现有 Wallet 审计

### 1.1 Account Model

**文件**: `internal/modules/wallet/domain/account.go`

```go
type Account struct {
    ID        uint
    UserID    uint         // uniqueIndex
    Balance   money.Amount // decimal(20,2), default 0
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt *time.Time
}
```

- 单余额模型，无 `frozen_balance`、无 `currency` 字段（币种隐含在站点配置中）
- `UserID` 唯一索引，一用户一账户
- `money.Amount` 封装 shopspring decimal，强制 2dp 舍入

**审计结论**: 结构简洁，支持提现扣款。若引入 frozen balance 需加列，但第一版不需要。

### 1.2 Transaction (Ledger) Model

**文件**: `internal/modules/wallet/domain/transaction.go`

```go
type Transaction struct {
    ID              uint
    UserID          uint         // index
    OperatorAdminID *uint        // index
    OrderID         *uint        // index
    Type            string       // varchar(40), index
    Direction       string       // varchar(16), index: in / out
    Amount          money.Amount // decimal(20,2)
    BalanceBefore   money.Amount // decimal(20,2)
    BalanceAfter    money.Amount // decimal(20,2)
    Currency        string       // varchar(16), default 'CNY'
    Reference       string       // varchar(120), **uniqueIndex**
    Remark          string       // varchar(255)
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       *time.Time
}
```

**关键能力**:
- `Reference` 唯一索引 → 天然幂等键
- `BalanceBefore` / `BalanceAfter` → 每笔流水带余额快照，可审计可对账
- `Direction` in/out → 明确资金方向
- `OperatorAdminID` → 可追溯管理员操作
- `Type` varchar(40) → 可扩展新类型

**现有 Type 常量** (`internal/constants/constants.go`):
```
recharge, order_pay, order_refund, admin_adjust, admin_refund,
gift_card_redeem, order_underpaid_credit
```

**审计结论**: Ledger 设计完善，提现只需新增 2 个 type：`withdrawal_debit`、`withdrawal_refund`。无需改表结构。

### 1.3 Debit/Credit Service

**文件**: `internal/modules/wallet/application/credit.go`

核心方法 `changeBalance()`:
1. 开启事务 `WithinTransaction`
2. `ensureAccountForUpdate` → `SELECT ... FOR UPDATE` 行锁
3. 校验 `after >= 0`（不足额返回 `ErrInsufficientBalance`）
4. 更新 balance
5. 创建 Transaction 流水（带 before/after 快照）
6. 事务提交

其他方法:
- `CreditInTransaction(tx, input)` — 在调用方事务内入账，先查 reference 幂等
- `ApplyOrderBalance(tx, input)` — 订单扣款，在调用方事务内，reference 幂等 + 轮次键
- `ReleaseOrderBalance(tx, input, claim)` — 订单退款，claim 原子占位防重复

**审计结论**: `changeBalance()` 已完整覆盖提现扣款所需的 row lock + 余额校验 + 流水记录。提现可直接复用或新增 `DebitInTransaction()` 方法（对称于 `CreditInTransaction`）。

### 1.4 Row Lock

**文件**: `internal/modules/wallet/infrastructure/gormstore/store.go`

```go
func (s *Store) GetAccountByUserIDForUpdate(userID uint) (*Account, error) {
    s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
        Where("user_id = ? AND deleted_at IS NULL", userID).
        First(&account)
}
```

- 标准 `SELECT ... FOR UPDATE`
- 所有写操作（recharge, admin_adjust, order_pay）均通过此方法获取行锁
- 并发测试 `concurrency_test.go` 验证了 MaxOpenConns=1 下不死锁

**审计结论**: 行锁机制健全。提现扣款必须使用同一行锁，确保与订单扣款、管理员调整等操作串行化。

### 1.5 Transaction Helper

**文件**: `internal/modules/wallet/contract/ports.go` + `gormstore/store.go`

```go
type UnitOfWork interface {
    WithinTransaction(fn func(Transaction) error) error
}
type Transaction interface {
    Wallets() Repository
}
```

- `WithinTransaction` 封装 GORM 事务
- `UseTransaction(tx)` 可将外部事务绑定为 Wallet Transaction
- 支持跨模块事务（订单模块已通过此机制与 Wallet 协作）

**审计结论**: 提现创建 + 扣款必须在同一个 `WithinTransaction` 内完成。若需跨模块（如通知），通知应在事务提交后异步发出。

### 1.6 Idempotency

现有三层幂等:
1. **`wallet_transactions.reference` 唯一索引** — 数据库层面最终兜底
2. **`GetTransactionByReference()` 预检查** — 应用层提前拦截重复
3. **`orderAllocationReference()` 轮次键** — 订单场景多轮操作的幂等设计

**缺失**: HTTP 层无 `Idempotency-Key` 中间件。提现提交需新增。

**审计结论**: 提现的幂等设计:
- User 提交: 客户端传 `Idempotency-Key` header，服务端组合 `user_id + idempotency_key` 作为 reference 前缀
- Admin approve/reject: `Idempotency-Key` + 状态机校验（仅 pending 可操作）
- 数据库唯一索引兜底

### 1.7 Admin Adjustment

**文件**: `internal/modules/wallet/application/admin.go` + `transport/http/admin_handler.go`

- `AdminAdjustBalance(input)` — 要求 `OperatorAdminID != 0`，remark 必填
- HTTP 层: `POST /admin/v1/users/:id/wallet/adjust`，挂在 `paymentProtected` 分组
- 操作记录在 `Transaction.OperatorAdminID`

**审计结论**: 管理员调整已有完整链路。提现审批的 admin 操作应复用相同的审计模式（记录操作管理员、原因、时间戳），但提现审批是状态流转而非直接余额调整。

### 1.8 现有 Wallet 能否直接支持提现扣款

**结论: 可以。** 现有能力清单:

| 提现所需能力 | 现有实现 | 状态 |
|---|---|---|
| 余额扣减 | `changeBalance()` / `ApplyOrderBalance()` | 已有 |
| 行锁 | `GetAccountByUserIDForUpdate()` | 已有 |
| 事务 | `WithinTransaction()` | 已有 |
| 余额不足校验 | `after < 0 → ErrInsufficientBalance` | 已有 |
| 流水记录 | `Transaction` 表 + before/after 快照 | 已有 |
| 幂等 | `reference` 唯一索引 + 预检查 | 已有 |
| 管理员审计 | `OperatorAdminID` + remark | 已有 |
| 提现单状态机 | 无 | **需新增** |
| 提现地址管理 | 无 | **需新增** |
| HTTP Idempotency-Key | 无 | **需新增** |
| 2FA step-up 校验 | 无（仅有登录 2FA） | **需新增** |
| 提现限额配置 | 无 | **需新增** |

---

## 2. 提现业务模型（状态机）

### 2.1 推荐状态

```
                    ┌──────────┐
                    │  pending  │ ← 用户提交，余额已扣
                    └────┬─────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
          ▼              ▼              ▼
    ┌──────────┐   ┌──────────┐   ┌──────────┐
    │ rejected │   │ canceled │   │ approved │ ← Admin 审批通过
    └──────────┘   └──────────┘   └────┬─────┘
     (退款)          (退款)              │
                                        ▼
                                  ┌──────────┐
                                  │processing│ ← 打款中（手动/自动）
                                  └────┬─────┘
                                       │
                                       ▼
                                  ┌──────────┐
                                  │completed │ ← 已打款，记录 txid
                                  └──────────┘
```

### 2.2 状态定义

| 状态 | 含义 | 余额影响 | 可由谁触发 |
|---|---|---|---|
| `pending` | 已申请，待审核 | 已扣 `request_amount` | 用户提交 |
| `approved` | 审核通过，待打款 | 不变（已扣） | Admin |
| `processing` | 打款中 | 不变（已扣） | Admin（手动打款标记） |
| `completed` | 打款完成 | 不变（已扣） | Admin（回填 txid） |
| `rejected` | 审核拒绝 | 全额退回 `request_amount` | Admin |
| `canceled` | 用户撤销 | 全额退回 `request_amount` | 用户（仅 pending 状态） |

### 2.3 是否需要更简化

**审计结论: 6 状态是合理的最小集，不建议简化。**

理由:
- `approved` 与 `processing` 分离: 手动打款场景下，审批通过和实际打款是两个动作，可能由不同人执行或跨时间。合并会丢失"已审批未打款"的可观测性。
- `canceled` 与 `rejected` 分离: 主动撤销和被动拒绝的审计语义不同，退款原因不同。
- 不复用 Business Order 5-State（pending/paid/shipped/completed/cancelled），因为提现是资金单向流出，无履约/物流维度。

### 2.4 状态转换规则（fail-closed）

```
pending → approved    : Admin, 需 Reason + Idempotency-Key
pending → rejected    : Admin, 需 reject_reason + Idempotency-Key
pending → canceled    : User, 仅本人, 需 2FA
approved → processing : Admin, 需 Idempotency-Key
processing → completed: Admin, 需 txid + Idempotency-Key
```

**非法转换一律返回 409 Conflict**，不做隐式降级。

---

## 3. 资金语义

### 3.1 方案对比

| 维度 | A. 申请即扣款 + reject 退款 | B. 引入 frozen balance |
|---|---|---|
| Wallet 改动 | 无需改 Account 表 | 需加 `frozen_balance` 列 + 改所有余额查询 |
| 扣款实现 | 复用 `changeBalance()` debit | 需新增 freeze/unfreeze 操作 |
| 可用余额展示 | `balance`（已扣后） | `balance - frozen_balance` |
| reject 退款 | credit 一笔 `withdrawal_refund` | unfreeze，无流水 |
| 并发安全 | row lock + transaction | row lock + transaction |
| 审计轨迹 | debit + refund 两笔流水，完整 | freeze 无流水，审计较弱 |
| 余额为负风险 | 无（扣款时校验） | 无（freeze 时校验） |
| C2C 适配 | 未来需指定扣款来源 | 未来需指定 freeze 来源 |
| 实现复杂度 | 低 | 中高 |

### 3.2 推荐方案 A：申请即扣款

**审计结论: 第一版采用方案 A。**

理由:
1. **对现有 Wallet 零结构改动** — 不加列、不改查询、不改展示
2. **审计轨迹更完整** — 每笔资金变动都有 ledger entry，reject 退款也有明确记录
3. **复用成熟路径** — `changeBalance()` 已在订单扣款、管理员调整中验证
4. **用户体验可接受** — 提现申请后余额立即减少，用户明确知道"这笔钱已被锁定"，reject 后退回有流水可查
5. **与 C2C 解耦** — 未来 C2C 双余额上线后，只需在扣款前选择余额来源，不影响冻结模型

### 3.3 资金流转明细

```
用户申请提现 100 USDT，手续费 1 USDT:

1. pending (申请时):
   - Wallet debit: -100.00 (type=withdrawal_debit, reference=wd:<user_id>:<idempotency_key>)
   - wallet_withdrawals 插入: amount=100, fee_amount=1, net_amount=99, status=pending

2a. rejected (Admin 拒绝):
   - Wallet credit: +100.00 (type=withdrawal_refund, reference=wd_refund:<withdrawal_id>)
   - wallet_withdrawals 更新: status=rejected, reject_reason=..., rejected_at=...

2b. canceled (用户撤销):
   - Wallet credit: +100.00 (type=withdrawal_refund, reference=wd_refund:<withdrawal_id>)
   - wallet_withdrawals 更新: status=canceled, canceled_at=...

2c. approved → processing → completed (正常打款):
   - approved: status=approved, approved_at=..., approved_by=admin_id
   - processing: status=processing, processing_at=...
   - completed: status=completed, completed_at=..., txid=...
   - 无额外 Wallet 操作（申请时已扣）
```

### 3.4 关键约束

- **申请扣款和提现单创建必须在同一数据库事务内** — 防止"扣了钱但没创建提现单"或"创建了提现单但没扣钱"
- **reject/cancel 退款和状态更新必须在同一事务内** — 防止"状态改了但钱没退"
- **退款金额 = request_amount（全额）** — 手续费在 completed 时才真正实现，未完成的提现不产生手续费
- **completed 后不可退款** — 链上转账不可逆，若需退款走管理员人工调整 + 线下追回

---

## 4. 提现地址

### 4.1 USDT-TRC20 地址格式

| 属性 | 值 |
|---|---|
| 编码 | Base58Check |
| 首字符 | `T` |
| 长度 | 34 字符（固定） |
| 字符集 | `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz`（Base58） |
| 校验 | 4 字节 checksum（双 SHA256 前 4 字节） |

### 4.2 地址校验实现要求

```go
// 校验规则:
// 1. 长度 == 34
// 2. 首字符 == 'T'
// 3. 全部字符在 Base58 字符集内
// 4. Base58Check 解码 + checksum 校验通过
```

- **不依赖第三方链 API 做存在性验证**（按用户要求）
- Base58Check 校验可在本地完成，无需网络
- 建议引入或自实现 Base58 编解码（项目当前无此依赖，需新增轻量库或自实现）

### 4.3 地址黑名单

- **存储**: Admin Settings 配置 `withdrawal_address_blacklist`，JSON 数组
- **校验时机**: 用户提交提现时检查地址是否在黑名单
- **管理**: Admin 可增删黑名单地址
- **第一版**: 简单精确匹配即可，不需要模糊匹配

### 4.4 常用地址保存

- **新增表**: `wallet_withdrawal_addresses`
- 字段: `id, user_id, network, address, label, created_at, updated_at, deleted_at`
- 唯一索引: `(user_id, network, address)` — 同一用户同一网络不重复保存
- 用户可在提现页面选择已保存地址或输入新地址
- 保存地址不跳过校验（每次使用仍需格式校验 + 黑名单检查）

### 4.5 审计结论

| 项 | 第一版 |
|---|---|
| 格式校验 | Base58Check 本地校验，必须 |
| 链上存在性 | 不做（不依赖第三方 API） |
| 地址黑名单 | Admin Settings 配置，必须 |
| 常用地址保存 | 新增表，推荐但非阻塞 |

---

## 5. 2FA

### 5.1 现有 2FA 能力

**文件**: `internal/modules/identity/userauth/`

- User TOTP 完整实现: setup → enable → verify → disable → recovery codes
- `User2FATOTPService` 接口: `GetStatus, Setup, Enable, Disable, VerifyChallengeCode, VerifyChallengeRecoveryCode`
- 用户表字段: `TOTPSecret, TOTPEnabledAt, TOTPPendingSecret, RecoveryCodes`
- 2FA 仅用于登录流程，无 step-up 中间件

### 5.2 提现 2FA 方案

**审计结论: 复用现有 User TOTP，提现提交时强制验证。**

| 项 | 方案 |
|---|---|
| 验证方式 | User TOTP 6 位验证码 |
| 验证时机 | `POST /api/v1/wallet/withdrawals` 请求体携带 `totp_code` |
| 未启用 TOTP | **fail-closed**: 拒绝提现，返回错误提示用户先启用 2FA |
| 恢复码 | 不接受恢复码用于提现（恢复码仅用于登录找回） |
| 提现密码 | **不引入**。复用 TOTP 即可，避免多套密码管理 |
| 邮箱验证码 | 第一版不做。TOTP 已足够；未来可作为额外因子叠加 |
| Admin 操作 | Admin 审批走 Admin 2FA（现有 `admin_2fa_handler`），Step-Up |

### 5.3 实现要点

- 提现 handler 内调用 `totp.VerifyChallengeCode(userID, code)`
- 验证失败计数: 复用现有 `User2FAChallengeStore` 的失败计数机制，或独立实现提现 2FA 失败计数（5 次锁定 15 分钟）
- 2FA 验证和扣款在同一事务内？**不**。2FA 验证是无状态的密码学校验，应在事务前完成。事务内只做扣款 + 创建提现单。
- 高风险写操作（提现、cancel）必须 2FA；查询类 API 不需要

---

## 6. 提现限制配置

### 6.1 配置项

所有配置通过 **Admin Settings** 管理，key 为 `withdrawal_config`，禁止硬编码。

```json
{
  "enabled": true,
  "network": "TRC20",
  "currency": "USDT",
  "minimum_withdrawal": "10.00",
  "maximum_withdrawal": "10000.00",
  "daily_limit": "50000.00",
  "daily_count_limit": 10,
  "fee": {
    "type": "fixed",
    "fixed_amount": "1.00",
    "percentage_rate": "0.00",
    "minimum_fee": "0.00"
  },
  "address_blacklist": [],
  "new_user_withdrawal_delay_hours": 24,
  "first_withdrawal_maximum": "1000.00"
}
```

### 6.2 配置说明

| 配置项 | 含义 | 校验 |
|---|---|---|
| `enabled` | 提现总开关 | false 时所有提现 API 返回 503 |
| `network` | 网络类型，第一版固定 `TRC20` | 仅接受 TRC20 |
| `currency` | 币种，固定 `USDT` | 仅接受 USDT |
| `minimum_withdrawal` | 单笔最低 | `amount >= min` |
| `maximum_withdrawal` | 单笔最高 | `amount <= max` |
| `daily_limit` | 单日累计金额上限 | 当日 completed + pending 之和 |
| `daily_count_limit` | 单日笔数上限 | 当日提现单数 |
| `fee.type` | `fixed` / `percentage` / `mixed` | 决定手续费计算方式 |
| `fee.fixed_amount` | 固定手续费 | `>= 0` |
| `fee.percentage_rate` | 百分比费率（如 0.005 = 0.5%） | `0 ~ 0.1` |
| `fee.minimum_fee` | 最低手续费 | percentage 模式下兜底 |
| `address_blacklist` | 地址黑名单 | 精确匹配 |
| `new_user_withdrawal_delay_hours` | 新账号提现冷却（注册后 N 小时） | 风控 |
| `first_withdrawal_maximum` | 首次提现限额 | 风控 |

### 6.3 Settings 注册模式

参考现有 `OrderRiskControlConfig` 的实现:
- `internal/modules/settings/schema/security/withdrawal.go` — 定义配置结构体 + Normalize + Decode/Encode
- `internal/modules/settings/application/default_registry.go` — 注册 `NormalizeWithdrawalConfigJSON`
- 遵循现有 Registry 模式，禁止 key switch dispatch

---

## 7. 手续费

### 7.1 金额模型

所有金额使用 `money.Amount`（USDT, decimal, 2dp）。

| 字段 | 含义 | 示例 |
|---|---|---|
| `request_amount` | 用户申请提现金额（从钱包扣除的金额） | 100.00 |
| `fee_amount` | 手续费 | 1.00 |
| `net_amount` | 用户实际到账金额 | 99.00 |

**公式**: `net_amount = request_amount - fee_amount`

### 7.2 手续费计算（服务端）

```
fixed:      fee_amount = fixed_amount
percentage: fee_amount = max(request_amount * percentage_rate, minimum_fee)
mixed:      fee_amount = fixed_amount + request_amount * percentage_rate
```

- **前端不得自行计算** `fee_amount` 和 `net_amount`
- 前端可调用预览接口 `POST /api/v1/wallet/withdrawals/quote` 获取手续费预览（不扣款、不创建单）
- 实际提交时服务端重新计算，以服务端为准
- 计算结果 2dp 舍入（`money.Amount` 自动处理）

### 7.3 手续费的资金归属

- 申请时扣除 `request_amount`（含手续费）
- 手续费在 `completed` 时才真正实现为平台收入
- rejected/canceled 时全额退回 `request_amount`，不产生手续费
- 第一版**不在用户 Wallet Ledger 中拆分手续费** — 单笔 `withdrawal_debit` 记录 `request_amount`
- 平台手续费收入统计通过 `wallet_withdrawals.fee_amount` 聚合，未来可引入平台收入台账

---

## 8. Admin 审批

### 8.1 API 设计

所有 Admin API 挂在 `/api/admin/v1/wallet/withdrawals`，需通过:
- Admin JWT 鉴权
- Admin RBAC（resource: `/api/admin/v1/wallet/withdrawals*`）
- Payment Compliance 中间件
- 写操作需 `Idempotency-Key` header
- 写操作需 `reason` 字段（approve 可选，reject 必填）

| 方法 | 路径 | 说明 | 写操作 |
|---|---|---|---|
| GET | `/api/admin/v1/wallet/withdrawals` | 列表（分页、筛选: status/user_id/withdrawal_no/date_range） | 否 |
| GET | `/api/admin/v1/wallet/withdrawals/:id` | 详情（含用户信息、钱包流水关联） | 否 |
| POST | `/api/admin/v1/wallet/withdrawals/:id/approve` | 审批通过 | 是 |
| POST | `/api/admin/v1/wallet/withdrawals/:id/reject` | 审批拒绝（需 reject_reason） | 是 |
| POST | `/api/admin/v1/wallet/withdrawals/:id/processing` | 标记打款中 | 是 |
| POST | `/api/admin/v1/wallet/withdrawals/:id/complete` | 标记完成（需 txid） | 是 |

### 8.2 审批流程安全要求

| 要求 | 实现 |
|---|---|
| Admin JWT | 现有 `JWTAuthMiddleware` |
| RBAC | 现有 `AdminRBACMiddleware`，需注册提现相关 resource/action |
| Payment Compliance | 现有 `PaymentComplianceRequired` 中间件 |
| Step-Up | Admin 2FA 验证（参考 `admin_2fa_handler`），高风险操作需重新验证 |
| Reason | reject 必填 `reject_reason`；approve 可选 `admin_note` |
| Audit | 记录 `approved_by/rejected_by/processed_by/completed_by` + 时间戳；建议接入 auditlog |
| Idempotency-Key | 写操作 header 必填，重复请求返回同一结果 |
| 状态机校验 | 仅允许合法状态转换，非法返回 409 |
| 行锁 | 审批操作 `SELECT ... FOR UPDATE` 锁定提现单行 |

### 8.3 审批不触碰余额

- approve/reject/processing/complete **均不直接修改 wallet balance**
- 余额在用户申请时已扣除，reject 时才触发退款
- approve 仅更新提现单状态 + 记录审批人
- complete 仅更新状态 + txid

---

## 9. 链上转账

### 9.1 方案对比

| 维度 | A. 手动打款 | B. 自动链上打款 |
|---|---|---|
| 实现复杂度 | 低 | 高 |
| hot wallet 管理 | 不需要 | 需要（私钥存储、签名安全） |
| Tron API 集成 | 不需要 | 需要（节点/RPC、gas 管理、nonce 管理） |
| 到账确认 | 人工确认 | 需监听链上确认 |
| 出错回滚 | 人工处理 | 需复杂重试/补偿机制 |
| 上线速度 | 快 | 慢 |
| 安全风险 | 人工操作失误 | 私钥泄露、智能合约风险 |
| 可审计性 | 回填 txid，可链上验证 | 自动记录 txid |
| 适合阶段 | 第一版 | 后续 Phase |

### 9.2 推荐方案 A：手动打款

**审计结论: 第一版必须采用手动打款。**

理由:
1. **尽快上线** — 无需 hot wallet、Tron API、gas 管理等基础设施
2. **安全可控** — 私钥不在系统中，杜绝私钥泄露风险
3. **审计简单** — Admin 人工转账后回填 txid，可在 TronScan 验证
4. **当前体量可接受** — 业务初期提现量有限，人工处理可行
5. **用户偏好明确** — 用户已指出"如果当前目标是尽快上线，优先考虑手动打款"

### 9.3 手动打款操作流程

```
1. Admin 在后台看到 status=approved 的提现单
2. Admin 使用外部钱包（如 TronLink）向用户地址转 net_amount USDT
3. 转账成功后，Admin 在后台:
   - 调用 POST /api/admin/v1/wallet/withdrawals/:id/processing（可选，标记开始处理）
   - 调用 POST /api/admin/v1/wallet/withdrawals/:id/complete，填入 txid
4. 系统校验 txid 格式（64 位 hex），更新 status=completed
5. 触发 withdrawal_completed 通知
```

### 9.4 未来自动化路径

- 后续 Phase 引入 hot wallet 模块（独立 bounded context）
- 通过 webhook/worker 自动发起转账 + 监听确认
- `processing` 状态为自动化预留（自动打款中）
- 当前数据模型已兼容未来自动化（txid 字段、processing 状态）

---

## 10. 数据模型

### 10.1 wallet_withdrawals 表

```go
type Withdrawal struct {
    ID            uint         `gorm:"primarykey"`
    WithdrawalNo  string       `gorm:"type:varchar(40);uniqueIndex;not null"` // WD + yyyyMMdd + 序号
    UserID        uint         `gorm:"index;not null"`
    Network       string       `gorm:"type:varchar(16);not null;default:'TRC20'"`
    Address       string       `gorm:"type:varchar(64);not null"` // Base58Check 地址
    Amount        money.Amount `gorm:"type:decimal(20,2);not null"` // request_amount
    FeeAmount     money.Amount `gorm:"type:decimal(20,2);not null;default:0"`
    NetAmount     money.Amount `gorm:"type:decimal(20,2);not null"` // amount - fee
    Status        string       `gorm:"type:varchar(20);index;not null"`
    TxID          string       `gorm:"type:varchar(128);index"` // 链上交易哈希
    UserNote      string       `gorm:"type:varchar(255)"` // 用户备注
    AdminNote     string       `gorm:"type:varchar(255)"` // 管理员备注
    RejectReason  string       `gorm:"type:varchar(500)"` // 拒绝原因
    ApprovedBy    *uint        `gorm:"index"`
    RejectedBy    *uint        `gorm:"index"`
    ProcessedBy   *uint        `gorm:"index"`
    CompletedBy   *uint        `gorm:"index"`
    CanceledBy    *uint        `gorm:"index"` // 用户自身撤销，存 user_id
    ApprovedAt    *time.Time   `gorm:"index"`
    RejectedAt    *time.Time   `gorm:"index"`
    ProcessingAt  *time.Time   `gorm:"index"`
    CompletedAt   *time.Time   `gorm:"index"`
    CanceledAt    *time.Time   `gorm:"index"`
    CreatedAt     time.Time    `gorm:"index"`
    UpdatedAt     time.Time    `gorm:"index"`
    DeletedAt     *time.Time   `gorm:"index"`
}
```

### 10.2 索引设计

| 索引 | 字段 | 用途 |
|---|---|---|
| unique | `withdrawal_no` | 业务单号查询 |
| index | `user_id` | 用户提现列表 |
| index | `status` | 按状态筛选（Admin 列表） |
| index | `(user_id, status)` | 用户按状态筛选 |
| index | `txid` | 链上交易溯源 |
| index | `created_at` | 时间范围筛选 |
| index | `(user_id, created_at)` | 用户日限额统计 |

### 10.3 wallet_withdrawal_addresses 表（常用地址）

```go
type WithdrawalAddress struct {
    ID        uint       `gorm:"primarykey"`
    UserID    uint       `gorm:"index;not null"`
    Network   string     `gorm:"type:varchar(16);not null;default:'TRC20'"`
    Address   string     `gorm:"type:varchar(64);not null"`
    Label     string     `gorm:"type:varchar(64)"` // 地址标签，如"我的钱包"
    CreatedAt time.Time  `gorm:"index"`
    UpdatedAt time.Time  `gorm:"index"`
    DeletedAt *time.Time `gorm:"index"`
}
// uniqueIndex: (user_id, network, address)
```

### 10.4 不需要新增的表

- **不需要** `wallet_withdrawal_fees` — 手续费内联在 `wallet_withdrawals`
- **不需要** `wallet_withdrawal_audit_logs` — 复用 `auditlog` 模块或提现单字段已足够
- **不需要** 修改 `wallet_accounts` — 不加 frozen_balance

---

## 11. Ledger 设计

### 11.1 新增 Transaction Type

在 `internal/constants/constants.go` 新增:

```go
WalletTxnTypeWithdrawalDebit  = "withdrawal_debit"
WalletTxnTypeWithdrawalRefund = "withdrawal_refund"
```

### 11.2 Ledger Entry 明细

#### 提现申请扣款 (withdrawal_debit)

| 字段 | 值 |
|---|---|
| `type` | `withdrawal_debit` |
| `direction` | `out` |
| `amount` | `request_amount` |
| `balance_before` | 扣款前余额 |
| `balance_after` | 扣款后余额 |
| `currency` | `USDT` |
| `reference` | `wd:<user_id>:<idempotency_key>` 或 `wd:<withdrawal_id>` |
| `remark` | `提现申请 <withdrawal_no>` |
| `order_id` | nil |
| `operator_admin_id` | nil |

#### 提现退款 (withdrawal_refund)

| 字段 | 值 |
|---|---|
| `type` | `withdrawal_refund` |
| `direction` | `in` |
| `amount` | `request_amount`（全额） |
| `balance_before` | 退款前余额 |
| `balance_after` | 退款后余额 |
| `currency` | `USDT` |
| `reference` | `wd_refund:<withdrawal_id>` |
| `remark` | `提现拒绝退款 <withdrawal_no>` 或 `提现撤销退款 <withdrawal_no>` |
| `order_id` | nil |
| `operator_admin_id` | reject 时为 admin_id；cancel 时为 nil |

### 11.3 禁止事项

- **禁止**只有 `wallet_withdrawals` 状态变更但无对应 Wallet Ledger 记录
- **禁止**completed 状态产生额外 ledger entry（资金已在申请时扣减）
- **禁止**手续费单独产生用户侧 ledger entry（第一版不拆分）
- **禁止**直接修改 `wallet_accounts.balance` 而不创建 `wallet_transactions` 流水

### 11.4 对账校验

每日对账可验证:
```
Σ(withdrawal_debit) - Σ(withdrawal_refund for completed) = 
    Σ(wallet_withdrawals where status in [pending, approved, processing, completed]).amount
```

---

## 12. 幂等 / 并发

### 12.1 并发场景分析

| 场景 | 风险 | 防护措施 |
|---|---|---|
| 用户双击提交 | 重复扣款 | `Idempotency-Key` + reference 唯一索引 |
| 用户多标签页同时提交 | 重复扣款 | 同上 + row lock 串行化 |
| Admin 重复 approve | 状态重复更新 | 状态机校验（仅 pending→approved）+ Idempotency-Key |
| Admin 重复 reject | 重复退款 | 状态机校验 + reference 唯一索引（退款流水） |
| approve 与 reject 并发 | 一个成功一个失败 | 行锁 `SELECT ... FOR UPDATE` + 状态校验 |
| 提现扣款与订单扣款并发 | 余额超扣 | 两者均使用 `GetAccountByUserIDForUpdate`，行锁串行化 |
| 提现扣款与管理员调整并发 | 余额不一致 | 同上，行锁串行化 |
| 用户 cancel 与 Admin approve 并发 | 状态竞争 | 行锁 + 状态校验，先到先得 |
| 退款与新提现扣款并发 | 余额计算错误 | 行锁串行化，退款在事务内完成 |

### 12.2 关键实现约束

1. **提现创建事务**:
   ```
   BEGIN
     SELECT account FOR UPDATE
     校验余额 >= request_amount
     校验日限额
     UPDATE account SET balance = balance - request_amount
     INSERT wallet_transactions (withdrawal_debit)
     INSERT wallet_withdrawals (status=pending)
   COMMIT
   ```

2. **Admin 审批事务**:
   ```
   BEGIN
     SELECT withdrawal FOR UPDATE
     校验 status == pending
     UPDATE withdrawal SET status=approved/rejected, ...
     IF rejected:
       SELECT account FOR UPDATE
       UPDATE account SET balance = balance + request_amount
       INSERT wallet_transactions (withdrawal_refund)
   COMMIT
   ```

3. **用户 cancel 事务**: 同 reject，但触发者是用户本人

4. **行锁顺序**: 若事务内同时锁 `wallet_withdrawals` 和 `wallet_accounts`，固定顺序为 `withdrawals → accounts`，避免死锁

### 12.3 Idempotency-Key 规范

- User 提交: `Idempotency-Key` header，客户端生成 UUID
  - 服务端 reference = `wd:<user_id>:<idempotency_key>`
  - 重复请求返回已创建的提现单（200，非 409）
- Admin 写操作: `Idempotency-Key` header
  - 重复请求返回同一操作结果
  - 若 key 对应不同操作参数，返回 409

---

## 13. User API 设计

### 13.1 端点清单

| 方法 | 路径 | 说明 | 2FA |
|---|---|---|---|
| POST | `/api/v1/wallet/withdrawals` | 提交提现申请 | 是（totp_code） |
| GET | `/api/v1/wallet/withdrawals` | 提现历史列表（分页） | 否 |
| GET | `/api/v1/wallet/withdrawals/:id` | 提现详情 | 否 |
| POST | `/api/v1/wallet/withdrawals/:id/cancel` | 撤销提现（仅 pending） | 是（totp_code） |
| POST | `/api/v1/wallet/withdrawals/quote` | 手续费预览（不扣款） | 否 |

### 13.2 请求/响应示例

**POST /api/v1/wallet/withdrawals**
```json
// Request
{
  "amount": "100.00",
  "network": "TRC20",
  "address": "TXYZ...",
  "address_id": 0,
  "user_note": "",
  "totp_code": "123456"
}
// Headers: Idempotency-Key: <uuid>

// Response 201
{
  "id": 1,
  "withdrawal_no": "WD202610040001",
  "amount": "100.00",
  "fee_amount": "1.00",
  "net_amount": "99.00",
  "status": "pending",
  "address": "TXYZ...",
  "created_at": "2026-10-04T10:00:00Z"
}
```

**POST /api/v1/wallet/withdrawals/quote**
```json
// Request
{ "amount": "100.00", "network": "TRC20" }

// Response 200
{
  "request_amount": "100.00",
  "fee_amount": "1.00",
  "net_amount": "99.00",
  "currency": "USDT"
}
```

### 13.3 错误码

| HTTP | 业务码 | 场景 |
|---|---|---|
| 400 | `withdrawal_disabled` | 提现功能未开启 |
| 400 | `withdrawal_amount_invalid` | 金额低于最低/高于最高 |
| 400 | `withdrawal_address_invalid` | 地址格式错误 |
| 400 | `withdrawal_address_blacklisted` | 地址在黑名单 |
| 400 | `withdrawal_daily_limit_exceeded` | 超日限额 |
| 400 | `withdrawal_new_user_cooldown` | 新账号冷却期 |
| 403 | `withdrawal_2fa_required` | 未启用 TOTP |
| 403 | `withdrawal_2fa_invalid` | TOTP 验证码错误 |
| 409 | `withdrawal_status_invalid` | 状态不允许此操作 |
| 422 | `insufficient_balance` | 余额不足 |

---

## 14. User 前端（预审计）

### 14.1 最小页面

**提现申请页** (`/wallet/withdraw`):
- 当前 Wallet Balance（实时，可用余额）
- 网络选择（第一版固定 TRC20，下拉禁用或隐藏）
- 地址输入框 + 常用地址下拉选择
- 金额输入框 + 「全部」按钮
- 手续费展示（调用 quote 接口实时预览）
- 实际到账展示（net_amount，服务端返回）
- TOTP 验证码输入框
- 用户备注（可选）
- 提交按钮（防双击，提交后 disable）

**提现历史页** (`/wallet/withdrawals`):
- 列表: withdrawal_no, 创建时间, 金额, 手续费, 到账金额, 状态, 地址（脱敏）, txid（completed 时显示）
- 状态筛选: 全部/pending/approved/processing/completed/rejected/canceled
- 分页
- 点击行查看详情
- pending 状态行显示「撤销」按钮

### 14.2 前端约束

- 手续费和到账金额**必须从接口获取**，禁止前端计算
- 地址脱敏展示: 前 6 位 + ... + 后 4 位
- 提交按钮防双击: 点击后立即 disable + loading
- 2FA 输入框仅在用户已启用 TOTP 时显示；未启用时引导至 2FA 设置页
- 余额低于最低提现额时提交按钮 disable 并提示

---

## 15. 通知

### 15.1 通知事件设计

与 Phase 1 User Notification 对接，通过 `NotificationEnqueuer.Enqueue()` 异步发送。

| 事件类型 | 触发时机 | 接收者 | 渠道 |
|---|---|---|---|
| `withdrawal_submitted` | 用户提交提现成功 | 用户 | 邮件 + 站内 |
| `withdrawal_approved` | Admin 审批通过 | 用户 | 邮件 + 站内 |
| `withdrawal_completed` | Admin 标记完成（含 txid） | 用户 | 邮件 + 站内 |
| `withdrawal_rejected` | Admin 拒绝（含原因） | 用户 | 邮件 + 站内 |
| `withdrawal_canceled` | 用户撤销 | 用户 | 站内（可选） |

### 15.2 实现约束

- 通知在**数据库事务提交后**发出（避免回滚后误通知）
- 通过现有 `notification` 模块的 async queue 投递
- 邮件模板需新增 4 套（submitted/approved/completed/rejected）
- 本轮只设计，不实施；后续 Phase 决定是否实现

---

## 16. 风控

### 16.1 第一版风控规则（人工审核为主）

| 规则 | 实现方式 | 动作 |
|---|---|---|
| 新账号提现限制 | `new_user_withdrawal_delay_hours` 配置 | 注册后 N 小时内禁止提现 |
| 首次提现限额 | `first_withdrawal_maximum` 配置 | 首笔提现金额上限 |
| 大额提现 | `maximum_withdrawal` + Admin 人工审核 | 所有提现均需 Admin 审批，大额重点关注 |
| 高频提现 | `daily_limit` + `daily_count_limit` | 超限额拒绝 |
| 2FA 强制 | 未启用 TOTP 禁止提现 | fail-closed |
| 同地址多账号 | Admin 后台可查询 `address` 关联的所有 user_id | 人工判断是否洗钱/欺诈 |
| 地址黑名单 | `address_blacklist` 配置 | 提交时拦截 |

### 16.2 第一版不做

- 自动风控引擎（规则引擎/机器学习）
- 设备指纹/IP 风控
- 行为分析
- 自动冻结账户

**所有提现均需 Admin 人工审批**，这是第一版最核心的风控手段。

### 16.3 Admin 风控辅助信息

提现详情页应展示:
- 用户注册时间、首次充值时间
- 用户历史提现记录（笔数、金额、成功率）
- 该地址关联的其他用户
- 用户当前余额、累计充值、累计消费
- 用户登录 IP/设备信息（如有）

---

## 17. 与 C2C 双余额改造的解耦

### 17.1 结论

**提现与 C2C 双余额完全解耦，第一版不需要为 C2C 做任何预留。**

### 17.2 解耦分析

| 维度 | 当前（单余额） | 未来 C2C（双余额） |
|---|---|---|
| 扣款来源 | `wallet_accounts.balance` | 需区分 `available_balance` / `in_order_balance` |
| 提现扣款 | 直接扣 balance | 从 available_balance 扣 |
| 退款路径 | 直接加 balance | 加回 available_balance |
| 提现单结构 | 无来源字段 | 可能需加 `balance_source` 字段 |
| 改动范围 | 无 | C2C Phase 统一改造 |

### 17.3 原则

- **不为 Withdrawal 提前引入 C2C 双余额** — 用户明确要求
- C2C 上线时，提现模块的扣款逻辑需适配新的余额来源，但这是 C2C Phase 的工作
- 当前提现设计保持单余额简单模型，避免过度设计

---

## 18. 最小实施范围

### 18.1 后端（Go）

| 模块 | 文件 | 改动类型 |
|---|---|---|
| constants | `constants.go` | 新增 2 个 wallet txn type + 6 个 withdrawal status |
| wallet/domain | `withdrawal.go`, `withdrawal_address.go` | 新增 domain |
| wallet/contract | `types.go`, `ports.go`, `errors.go` | 新增 withdrawal 相关类型/接口/错误 |
| wallet/application | `withdrawal.go` | 新增用例（提交/撤销/审批/完成） |
| wallet/application | `credit.go` | 新增 `DebitInTransaction()`（对称于 CreditInTransaction） |
| wallet/infrastructure | `store.go` | 新增 withdrawal CRUD + ForUpdate |
| wallet/transport/http | `withdrawal_user_handler.go`, `withdrawal_admin_handler.go`, `routes.go` | 新增 handler + 路由注册 |
| wallet/transport/presenter | `withdrawal.go` | 新增响应 DTO |
| settings/schema | `withdrawal.go` | 新增配置结构体 + Normalize |
| settings/application | `default_registry.go` | 注册 withdrawal config normalizer |
| authz | `bootstrap.go` | 注册提现 RBAC resource/action |
| migrations | `migrations.go` | 新增 2 张表的 AutoMigrate |
| architecture | `wallet_module_structure_test.go` | 更新结构测试断言 |

### 18.2 前端（User）

| 文件 | 改动 |
|---|---|
| `api/wallet.ts` | 新增 withdrawal API 方法 |
| `components/wallet/WalletWithdrawForm.vue` | 新增提现表单组件 |
| `components/wallet/WalletWithdrawalList.vue` | 新增提现历史组件 |
| `views/personal/WalletWithdraw.vue` | 新增提现页面 |
| `views/personal/WalletPanel.vue` | 加入口跳转 |
| router | 新增提现路由 |

### 18.3 前端（Admin）

| 文件 | 改动 |
|---|---|
| `api/admin.ts` | 新增 withdrawal API 方法 |
| `views/admin/WalletWithdrawals.vue` | 新增提现管理列表页 |
| `views/admin/WalletWithdrawalDetail.vue` | 新增提现详情/审批页 |

### 18.4 测试

- 单元测试: 手续费计算、地址校验、状态机转换
- 集成测试: 提扣款事务、并发扣款、幂等、reject 退款
- 架构测试: 更新 `wallet_module_structure_test.go`

### 18.5 本轮禁止

- 自动 TRON 转账 / hot wallet
- C2C 双余额改造
- 10 级返利
- 自动风控引擎
- 提现密码（独立于登录密码）
- 链上地址存在性 API 调用

---

## 19. 主要风险与缓解

| 风险 | 等级 | 缓解措施 |
|---|---|---|
| 扣款后提现单创建失败（部分提交） | 高 | 同一事务，原子性保证 |
| reject 退款重复（多次调用） | 高 | 状态机校验 + reference 唯一索引 + row lock |
| 余额超扣（提现与订单并发） | 高 | 统一行锁 `GetAccountByUserIDForUpdate` |
| 2FA 绕过 | 高 | fail-closed，handler 内强制校验，未启用直接拒绝 |
| 地址错误导致资金丢失 | 高 | Base58Check 校验 + 黑名单 + 用户确认 + 人工打款二次确认 |
| Admin 误操作批准 | 中 | 审批需 reason + audit log + 双人审批可选（未来） |
| 手续费配置错误 | 中 | Settings Normalize 校验 + 管理员操作审计 |
| 日限额计算不准 | 中 | 事务内聚合查询 pending+completed，行锁保护 |
| Idempotency-Key 客户端不生成 | 中 | 服务端检测缺失则拒绝（写操作必填） |
| 手动打款忘记回填 txid | 低 | Admin 后台 pending 打款提醒 + processing 状态超时告警（未来） |
| 通知发送失败 | 低 | async queue 重试 + 通知日志 |

---

## 20. 附录：现有代码引用索引

| 能力 | 文件路径 |
|---|---|
| Wallet Account domain | `internal/modules/wallet/domain/account.go` |
| Wallet Transaction domain | `internal/modules/wallet/domain/transaction.go` |
| Wallet RechargeOrder domain | `internal/modules/wallet/domain/recharge_order.go` |
| Debit/Credit core | `internal/modules/wallet/application/credit.go` |
| Admin adjust | `internal/modules/wallet/application/admin.go` |
| Order balance debit/release | `internal/modules/wallet/application/order_balance.go` |
| Repository + UoW + row lock | `internal/modules/wallet/infrastructure/gormstore/store.go` |
| Contract ports/types/errors | `internal/modules/wallet/contract/` |
| User handler | `internal/modules/wallet/transport/http/user_handler.go` |
| Admin handler | `internal/modules/wallet/transport/http/admin_handler.go` |
| Routes | `internal/modules/wallet/transport/http/routes.go` |
| Concurrency test | `internal/modules/wallet/integrationtest/concurrency_test.go` |
| Wallet txn constants | `internal/constants/constants.go` (L140-153) |
| Money Amount | `internal/shared/money/amount.go` |
| User TOTP 2FA | `internal/modules/identity/userauth/transport/http/user_2fa_handler.go` |
| Admin 2FA | `internal/modules/identity/adminauth/transport/http/admin_2fa_handler.go` |
| User domain (TOTP fields) | `internal/modules/identity/user/domain/user.go` (L24-28) |
| Settings core + Registry | `internal/modules/settings/application/core.go`, `registry.go` |
| Settings risk config 参考 | `internal/modules/settings/schema/security/order_risk_control.go` |
| Notification enqueuer | `internal/modules/notification/contract/ports.go` |
| Audit log | `internal/modules/auditlog/` |
| RBAC middleware | `internal/app/httpserver/middleware/middleware.go` (AdminRBACMiddleware) |
| Compliance middleware | `internal/app/httpserver/middleware/compliance_middleware.go` |
| JWT middleware | `internal/app/httpserver/middleware/middleware.go` (JWTAuthMiddleware) |
| Reseller withdraw 参考 | `internal/modules/reseller/application/accounting_withdraw.go` |
| Reseller withdraw domain | `internal/modules/reseller/domain/accounting.go` (WithdrawRequest) |
| Wallet architecture test | `internal/architecture/wallet_module_structure_test.go` |
| Migrations | `internal/bootstrap/database/migrations/migrations.go` |
| User frontend wallet API | `frontend/user/src/api/wallet.ts` |
| User frontend wallet panel | `frontend/user/src/views/personal/WalletPanel.vue` |
| Admin frontend wallet config | `frontend/admin/src/views/admin/Wallet.vue` |

---

*审计完成。本文件为 Phase 3 开发的输入基线，实施前需据此拆解任务。*
