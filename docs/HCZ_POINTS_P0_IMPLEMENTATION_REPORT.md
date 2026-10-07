# HCZ Points P0 Implementation Report

> 积分核心账本（Points Core）——第一阶段实施报告
> 项目：`E:\Users\orang\Downloads\Compressed\hcz_v1`（branch `main`，HEAD `35496c4`）
> 前置审计：`docs/audits/HCZ_POINTS_BACKEND_AUDIT.md`（READY WITH CONDITIONS，C1–C5 已全部满足）
> 实施日期：2026-10-07

---

## 0. HCZ Points P0 Final

| 项目 | 结论 |
|---|---|
| **Verdict** | **PASS** |
| Git Baseline | branch `main` @ `35496c4`；工作区含用户既有未提交改动（未触碰） |
| Points Account 建立 | ✅ `points_accounts`（AutoMigrate 注册，Fresh/Existing 均走统一 registry） |
| Append-only Ledger | ✅ `points_ledger`（只 INSERT；reference 唯一索引） |
| User account API | ✅ PASS |
| User ledger API | ✅ PASS |
| Admin adjustment | ✅ PASS |
| Admin RBAC | ✅ PASS（finance 读写+调整；readonly_auditor 只读） |
| Reason required | ✅ PASS（handler + application 双校验） |
| Idempotency | ✅ PASS（reference 预查 + 唯一索引兜底 + 409 冲突） |
| Row Lock | ✅ PASS（`SELECT ... FOR UPDATE`） |
| Concurrent mutation | ✅ PASS（PostgreSQL 真并发 20 goroutine） |
| First-create race | ✅ PASS（PostgreSQL 并发首建：1 account / N ledger） |
| Fresh install | ✅ PASS（AutoMigrate 全量跑通，含 bootstrap/migrations 测试 50s PASS） |
| Existing migration | ✅ PASS（注册式 AutoMigrate，与 HCZ 现状一致；PG 测试 DROP+重建验证） |
| 全量现有测试无回归 | ✅ 积分相关全绿；4 个既有失败包与 P0 无依赖（见 Regression） |
| **READY FOR P1 ORDER REWARD** | **YES** |

---

## 1. Changed Files（P0 实际改动）

### 1.1 新建（模块本体）

| 文件 | 作用 |
|---|---|
| `internal/modules/points/domain/account.go` | `points_accounts`：user_id 唯一、balance/total_earned/total_spent（int64 BIGINT） |
| `internal/modules/points/domain/ledger_entry.go` | `points_ledger`：append-only，`reference` 唯一索引，预留 order_id/checkin_date/exchange_order_id 维度列 |
| `internal/modules/points/contract/types.go` | ActionType / SourceType / OperatorType 常量；`AdminAdjustReference`；预留 P1/P2/P3 reference 规则函数 |
| `internal/modules/points/contract/errors.go` | 错误集（ErrInvalidAmount / ErrInvalidOperation / ErrReasonRequired / ErrReferenceRequired / ErrIdempotencyConflict / ErrNegativeNotAllowed / ErrAccountCreateFailed） |
| `internal/modules/points/contract/ports.go` | Repository / Transaction / UnitOfWork / AdjustInput / LedgerListFilter 端口 |
| `internal/modules/points/application/mutation.go` | 核心 mutation service：`applyMutation`（行锁→policy→同事务账户+Ledger）+ `ensureAccountForUpdate`（首建冲突重查）+ actionMetas 权威表 |
| `internal/modules/points/application/admin.go` | `AdminAdjust`（幂等预查/参数比对/唯一索引兜底） |
| `internal/modules/points/application/service.go` | `GetAccount`（零值语义）、`ListLedgerEntries` |
| `internal/modules/points/infrastructure/gormstore/store.go` | GORM 实现：`GetAccountByUserIDForUpdate`（FOR UPDATE）、`UseTransaction` 事务视图、分页过滤 |
| `internal/modules/points/transport/http/user_handler.go` | 用户端 account/ledger handler |
| `internal/modules/points/transport/http/admin_handler.go` | 后台查询/调整 handler（Idempotency-Key + Reason + operation 校验） |
| `internal/modules/points/transport/http/routes.go` | 用户 + 后台路由注册 |
| `internal/bootstrap/points/adapters.go` | 应用用例 → HTTP 端口适配 |
| `internal/bootstrap/points/handlers.go` | handler 装配（user reader 注入） |

### 1.2 新建（测试）

| 文件 | 作用 |
|---|---|
| `internal/modules/points/integrationtest/points_test.go` | SQLite 全语义测试：Credit/Debit/负余额/幂等/非法输入/并发（单连接串行，见 8.2） |
| `internal/modules/points/integrationtest/pg_concurrency_test.go` | PostgreSQL 真并发（`//go:build integration`，`TEST_POSTGRES_DSN` 为空则 Skip）：20 并发 credit/debit、并发首建、同键并发幂等 |
| `internal/modules/points/transport/http/user_handler_test.go` | 用户端点：无上下文 401、零值账户、IDOR 作用域断言 |
| `internal/modules/points/transport/http/admin_handler_test.go` | 后台端点：401/400/404/409 业务码、reference 派生、operation 透传 |
| `internal/authz/points_policy_test.go` | RBAC：finance 可读写+调整、readonly_auditor 只读、未授权拒绝 |

### 1.3 修改（装配现有文件）

| 文件 | 改动 |
|---|---|
| `internal/bootstrap/database/migrations/registry.go` | AutoMigrate 注册 `&pointsdomain.Account{}`、`&pointsdomain.LedgerEntry{}`（+7 行） |
| `internal/app/container/container.go` | 增加 `PointsRepo` / `PointsService` 字段（+4 行，与本任务相关部分） |
| `internal/app/container/repositories.go` | 初始化 `PointsRepo`（+2 行） |
| `internal/app/container/services_application.go` | 初始化 `PointsService`（+4 行） |
| `internal/app/httpserver/router.go` | 引入 `pointsbootstrap` 装配 |
| `internal/app/httpserver/routes_storefront.go` | 用户端点挂载（user JWT 组） |
| `internal/app/httpserver/routes_admin.go` | 后台端点挂载（admin JWT + RBAC 组） |
| `internal/authz/bootstrap.go` | finance 角色 +3（GET points、GET points/ledger、POST points/adjust）；readonly_auditor 角色 +2（只读两点） |
| `internal/i18n/messages.go` | 三语文案 +4（points_adjust_remark_required / points_account_fetch_failed / points_ledger_fetch_failed / points_adjust_failed） |

> 注：工作区另有大量用户既有未提交改动（sitebuilder、stepup、wallet、affiliate 等），P0 未触碰。

---

## 2. Database Schema

### 2.1 `points_accounts`（用户积分账户）

| 列 | 类型 | 约束 | 说明 |
|---|---|---|---|
| id | BIGSERIAL / INTEGER PK | primaryKey | |
| user_id | BIGINT | **UNIQUE** | 一人一账户 |
| balance | BIGINT | NOT NULL DEFAULT 0 | 当前余额（恒等于 ledger 净累计） |
| total_earned | BIGINT | NOT NULL DEFAULT 0 | 生命周期正向入账（Admin 加/签到/订单奖励/返还） |
| total_spent | BIGINT | NOT NULL DEFAULT 0 | 用户消费（兑换扣减） |
| created_at / updated_at | timestamptz | NOT NULL | |

**语义决策**（落实审计 14 节）：Admin 扣减与系统冲正**只影响 balance**，不计入 total_earned / total_spent 任一侧——避免 total_spent 语义污染；未来兑换扣减计入 total_spent。该语义由 `actionMetas` 权威表驱动（见 4 节）。

### 2.2 `points_ledger`（append-only 积分流水）

| 列 | 类型 | 约束 | 说明 |
|---|---|---|---|
| id | BIGSERIAL / INTEGER PK | primaryKey | |
| user_id | BIGINT | NOT NULL，复合索引 (user_id, created_at) | |
| action_type | VARCHAR(32) | NOT NULL | ADMIN_ADD / ADMIN_DEDUCT（+预留） |
| source_type | VARCHAR(32) | NOT NULL | admin_adjust（+预留） |
| source_id | BIGINT | NOT NULL | 来源对象 ID |
| amount | BIGINT | NOT NULL | **有符号**：正=入账，负=出账 |
| balance_before / balance_after | BIGINT | NOT NULL | 变更前后余额 |
| reference | VARCHAR(120) | NOT NULL **UNIQUE** | 幂等最终防线 |
| reason | VARCHAR(255) | NOT NULL | 必填 |
| operator_type | VARCHAR(16) | NOT NULL | admin / system / user |
| operator_id | BIGINT | NOT NULL | admin 时 = admin_id |
| order_id / checkin_date / exchange_order_id | BIGINT / DATE / BIGINT | 可空 + index | P1/P2/P3 预留维度列 |
| created_at | timestamptz | NOT NULL | |

**不变量**：`points_accounts.balance == SUM(points_ledger.amount)`（按 user_id），测试 helper `assertPGInvariant` 强制。

**迁移策略**：跟随 HCZ 现状（统一 `AutoMigrate` registry，无版本化 migration 文件），因此无独立 Up/Down 文件；"Down"语义由 `DROP TABLE` 表达（PG 集成测试 drop+recreate 已验证）。Fresh install 与 Existing upgrade 均走同一 registry。

---

## 3. API Proposal（已实现）

HCZ 响应约定：**HTTP 恒 200，业务码在 `body.status_code`（0=成功）**；分页用 `response.SuccessWithPage`。

### 3.1 用户端（storefront，JWT）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/points/account` | `{balance, total_earned, total_spent}`；**从未产生积分 → 零值，非 404，不创建账户** |
| GET | `/api/v1/points/ledger?page=&page_size=` | 本人流水（分页，按 created_at DESC, id DESC） |

IDOR 防护：user_id 只取自 JWT 上下文（`ginutil.GetUserID`），API 不接受任意 user_id。

### 3.2 后台（authorized，JWT + RBAC）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/admin/users/:id/points` | 用户积分账户（含 user 摘要） |
| GET | `/api/v1/admin/users/:id/points/ledger?page=&page_size=` | 用户积分流水 |
| POST | `/api/v1/admin/users/:id/points/adjust` | 增减积分 |

POST body：`{"amount": 100, "operation": "add"|"subtract", "reason": "..."}`
请求头：`Idempotency-Key` 必填；`X-Admin-Token`（HCZ admin JWT）

**金额边界**：`amount` 恒为正数（binding `required` + application 层 `>0`）；扣减由 `operation=subtract` 表达，禁止负数金额（避免双重负数）。handler 显式校验 `operation ∈ {add, subtract}`。

**业务码**：400=参数/幂等键缺失，404=目标用户不存在，409=同幂等键不同参数，500=内部。

---

## 4. Transaction Model（核心记账流水线）

所有积分变更经统一 mutation service（`applyMutation`），**禁止复制第二套扣分逻辑**：

```
WithinTransaction(tx)
  ├─ SELECT points_accounts FOR UPDATE   ← 行锁（GetAccountByUserIDForUpdate）
  ├─ 不存在 → ensureAccountForUpdate：
  │     Create → 撞 UNIQUE(user_id) → isDuplicateKeyError → 重新 FOR UPDATE 读取已存在账户
  ├─ balance_before = account.balance
  ├─ actionMeta 校验（见下）
  ├─ 溢出防御（int64 有符号加减方向校验）
  ├─ 负余额 policy（allowNegative）
  ├─ balance_after = before ± amount
  ├─ UPDATE points_accounts（balance / total_earned / total_spent / updated_at）
  ├─ INSERT points_ledger（before/after/action/source/reference/reason/operator）
  └─ Commit（账户 + 流水同事务，原子）
```

**actionMetas 权威表**（唯一记账语义来源）：

| action_type | allowNegative | trackEarned | trackSpent |
|---|---|---|---|
| ADMIN_ADD | false | true | false |
| ADMIN_DEDUCT | **true** | false | false |
| ORDER_REWARD（预留） | false | true | false |
| ORDER_REWARD_REVERSAL（预留） | true | false | false |
| CHECKIN_REWARD（预留） | false | true | false |
| REDEEM（预留） | false | false | true |
| REDEEM_REFUND（预留） | false | true | false |

**负积分规则**（落实审计 6 节）：Admin 扣减允许余额为负（`allowNegative=true`）；用户消费类（REDEEM）禁止负余额（`ErrNegativeNotAllowed`）。不同 mutation policy 由 actionMeta 单点驱动，不复制逻辑。

---

## 5. Idempotency（幂等）

```
AdminAdjust(input)
  ├─ 校验（user/op/amount>0/reason/reference）
  ├─ 事务外预查 GetLedgerEntryByReference(ref)
  │    ├─ 命中且参数一致 → 返回原流水（200，不重复入账）
  │    └─ 命中且参数不一致 → ErrIdempotencyConflict（409）
  ├─ 事务内 applyMutation
  └─ 撞 reference 唯一索引（并发兜底）→ isDuplicateKeyError
       └─ 重查 + 参数比对 → 幂等返回 / 409
```

- 幂等键派生：`reference = "admin_adjust:" + Idempotency-Key`（与钱包 `admin_adjust:` 前缀对齐，跨资产不混淆）。
- **数据库最终防线**：`points_ledger.reference` UNIQUE 索引——不依赖应用层 `if exists`。
- 为 P1 预留的 reference 规则（函数已落码，未产生业务实现）：`points:order_reward:{order_id}`、`points:order_refund:{refund_record_id}`、`points:checkin:{user_id}:{YYYY-MM-DD}`、`points:redeem:{exchange_order_id}`，天然支持未来 `UNIQUE(source_type, source_id, action_type)` 事件级去重；Admin 调整不适用该三元组（同用户多次调整合法），其幂等完全依赖 reference。

---

## 6. RBAC

Casbin 模型沿用 `defaultRBACModel`（`keyMatch2`，`g` 角色继承），政策注册于 `internal/authz/bootstrap.go` 的 `BuiltinRoleSeeds`：

| 角色 | 积分权限 |
|---|---|
| `finance` | GET `/admin/users/:id/points`、GET `/admin/users/:id/points/ledger`、POST `/admin/users/:id/points/adjust`（+3） |
| `readonly_auditor` | GET 两点（+2），**不可调整** |

- 路由层：Admin 端点挂载于 RBAC 中间件组（admin JWT + `EnforceAdmin`），**非万能 admin 绕过**。
- 测试：`TestPointsAdminPolicies` 断言 finance/auditor/未授权三类行为（全部 PASS）。

---

## 7. Test Evidence

### 7.1 单元 + SQLite 集成（`go test ./internal/modules/points/... ./internal/authz/ -count=1`）

```
--- PASS: TestGetAccountReturnsZeroValueWhenNeverCreated
--- PASS: TestAdminAdjustCreditCreatesAccountAndLedger
--- PASS: TestAdminAdjustDebitCanProduceNegativeBalance
--- PASS: TestAdminAdjustRejectsInvalidInputs
--- PASS: TestAdminAdjustIdempotencySameKeyReplays
--- PASS: TestAdminAdjustIdempotencyConflictOnDifferentPayload
--- PASS: TestConcurrentFirstCreateCreatesSingleAccount
--- PASS: TestConcurrentCreditNoLostUpdate
--- PASS: TestConcurrentDebitAllowsNegative
--- PASS: TestListLedgerEntriesScopedByUser
ok  github.com/Aether-v1/hcz/internal/modules/points/integrationtest
--- PASS: TestAdminAdjustRequiresAdminContext
--- PASS: TestAdminAdjustValidations
--- PASS: TestAdminAdjustIdempotencyConflictReturns409
--- PASS: TestAdminAdjustSuccessPassesReferenceAndOperation
--- PASS: TestAdminGetUserPointsNotFound
--- PASS: TestAdminGetUserPointsUsesPathParam
--- PASS: TestUserGetAccountRequiresAuthContext
--- PASS: TestUserGetAccountReturnsZeroValueWhenNoAccount
--- PASS: TestUserLedgerScopedToAuthContextUserID
ok  github.com/Aether-v1/hcz/internal/modules/points/transport/http
--- PASS: TestPointsAdminPolicies
--- PASS: TestEnforceAdminWithRolePolicy / TestSetAdminRolesOverride / TestNormalizeObject
--- PASS: TestBootstrapBuiltinRoles / TestBootstrapBuiltinRolesRemovesStaleImmutableWildcard
--- PASS: TestImmutableBuiltinRoleRejectsPolicyMutationAndDeletion
ok  github.com/Aether-v1/hcz/internal/authz
```

### 7.2 PostgreSQL 真并发（`go test -tags integration -run TestPGPoints -count=1`，本地 PG `hcz_test`）

```
--- PASS: TestPGPointsConcurrentCreditNoLostUpdate    (1.50s)  20 goroutine × +10 → balance=200、20 条 ledger、SUM 不变量成立
--- PASS: TestPGPointsConcurrentDebitAllowsNegative   (1.42s)  基础 1000 → 20 × -10 → balance=800（精确，允许负语义）
--- PASS: TestPGPointsConcurrentFirstCreateSingleAccount (1.28s)  20 并发首建 → 恰好 1 account、20 ledger
--- PASS: TestPGPointsIdempotencySameKeyConcurrent    (0.78s)  10 goroutine 同键同参 → 恰好 1 条 ledger、balance 只变一次
PASS  ok  github.com/Aether-v1/hcz/internal/modules/points/integrationtest  5.982s
```

### 7.3 验证命令

```
go build ./...                                    → PASS
go vet ./internal/modules/points/... ./internal/authz/... ./internal/app/httpserver/... → PASS
go test ./internal/modules/points/... ./internal/authz/ -count=1                       → PASS
go test -tags integration -run TestPGPoints ./internal/modules/points/integrationtest/ → PASS（需 TEST_POSTGRES_DSN）
go test ./... -count=1                            → 见 Regression
```

---

## 8. Regression（全量回归）

`go test ./... -count=1`：除以下 4 个包外全部 PASS（含 `internal/bootstrap/database/migrations` 50.3s PASS、`internal/app/httpserver` PASS、全部既有模块 PASS）。

| 失败包 | 失败原因 | 与 P0 关系 |
|---|---|---|
| `internal/architecture` | 架构守卫文件数预算：affiliate/application 12>10、affiliate/domain 7>6、migrations 16>14（用户既有改动引入新文件，非 P0 新增） | **无依赖** |
| `internal/logger` | Windows 下 TempDir cleanup 文件锁（`release.log` 被占用） | **无依赖** |
| `internal/modules/procurement/integrationtest` | 测试端口 `127.0.0.1:56926` 并发占用（Only one usage of each socket address） | **无依赖** |
| `internal/selfupdate` | Windows 平台 `unsupported_os` / 文件锁语义（平台相关既有失败） | **无依赖** |

结论：P0 未引入回归；4 个失败均为既有工作区状态 / 平台问题。如需严格归零可后续单独处理（超出 P0 范围，未擅自动手）。

---

## 9. Known Limitations（如实声明）

1. **SQLite 并发测试为单连接串行**：SQLite 是单写者数据库，并发写必然 BUSY/LOCKED（与业务无关）。跟随 `wallet/concurrency_test.go` 先例 `SetMaxOpenConns(1)`，验证**应用层事务正确性**（首建唯一、幂等、余额精确）；**真并发行锁语义以 PostgreSQL 测试为准**（已 PASS）。
2. **migration 无独立 Down 文件**：HCZ 现状为统一 `AutoMigrate` registry（无版本化 migration 机制），故遵循现有规范；Down 语义由 DROP TABLE 表达。
3. **total_earned/total_spent 语义已定但仅有 Admin 数据**：P0 只有 ADMIN_ADD/ADMIN_DEDUCT 产生数据，P1+ 接入订单奖励/兑换后累计字段由 actionMetas 自动驱动，无需改账本。
4. **GET 账户不创建记录**：读取无账户返回零值（避免 GET 写库）；首次 mutation 时才创建账户。
5. **未实现订单奖励/签到/商城/兑换**：按本轮范围严格排除，reference 规则与 action 常量已预留（P1 Ready）。
6. **Admin 扣减不限制 reason 最小长度**：仅要求非空（与钱包一致），无强制最小字数。

---

## 10. P1 Readiness（订单奖励接入点）

**READY FOR P1 ORDER REWARD = YES**。P1 无需改动 Points Core 账本，只需：

1. 在订单完成链路（`internal/modules/order/application/...`，具体位置由 P1 审计确认）调用 `PointsService` 新增的 OrderReward 用例，`reference = OrderRewardReference(orderID)`；
2. 新 action 走既有 `applyMutation`（actionMetas 已有 ORDER_REWARD 条目），同事务保证；
3. 退款冲正走 `ORDER_REWARD_REVERSAL`（allowNegative=true，新增流水而非修改原流水），reference 规则已定义；
4. 数据库无需新增表；`points_ledger.order_id` 维度列已就位。

---

*本报告基于实际代码与可复现测试结果编写；未完成的验证项均已如实标注，无"应该可以"式结论。*
