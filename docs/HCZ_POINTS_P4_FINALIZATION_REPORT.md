# HCZ Points System — P4 Admin / Operations / Finalization Report

日期：2026-10-08 · 范围：P0 Points Core / P1 Order Reward / P2 Check-in / P3 Points Mall 的**收口**（Admin 运营、审计、RBAC、契约、迁移、并发、门禁）
本轮定位：**不扩产品范围**。P0–P3 报告一律按"不可盲信"处理：先审计代码，后修改，改完重跑。

---

## 0. 结论速览

| 项 | 结论 |
| --- | --- |
| 本轮 Verdict | **PASS WITH CONDITIONS** |
| POINTS SYSTEM READY FOR FRONTEND INTEGRATION | **YES** |
| POINTS SYSTEM READY FOR PRODUCTION | **NO** |
| Production 阻塞点归属 | **仓库全局问题 + 前端未接线**；Points 自身无 blocking defect（详见 §15） |
| 本轮代码改动 | 3 处（1 个死常量、1 段过期注释、2 条 RBAC 断言）；文档 4 份 |
| 本轮明确"不改"的项 | Step-Up 强制、新建限流框架、CHECK/触发器约束、扩大 file budget、P4 期内接线前端 |

---

## 1. 真实能力矩阵（逐条对着路由与 Casbin 策略核）

`RegisterAdminRoutes` 挂在已带 `/api/v1/admin` 前缀的 authorized 组（`internal/app/httpserver/routes_admin.go:205-207`），
用户端挂在 storefront 已鉴权 `user` 组（`routes_storefront.go:156-158`）。表内 RBAC 列是 `internal/authz/bootstrap.go` 的实际策略，不是设计意图。

| Domain | Capability | Route | RBAC | Audit | Idempotency | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Points | 查自己的账户 | `GET /api/v1/points/account` | user JWT | — | 只读 | DONE |
| Points | 查自己的流水 | `GET /api/v1/points/ledger` | user JWT | — | 只读 | DONE |
| Points | 后台查用户账户 | `GET /api/v1/admin/users/:id/points` | auditor↑（全角色可读） | — | 只读 | DONE |
| Points | 后台查流水（过滤+分页） | `GET /api/v1/admin/users/:id/points/ledger` | auditor↑ | — | 只读 | DONE |
| Points | 后台增减积分 | `POST /api/v1/admin/users/:id/points/adjust` | **仅 finance** | `points_admin_mutation` | `Idempotency-Key`≤64 + DB `reference` UNIQUE(120) | DONE |
| Points | 人工补偿 | `POST /api/v1/admin/users/:id/points/compensate` | **仅 finance** | `points_admin_mutation`（action=ADMIN_COMPENSATION） | 独立 reference 前缀 + 同上 | DONE |
| Points | 账户列表 / 负余额排查 | `GET /api/v1/admin/points/accounts` | **仅 finance**（auditor/operations/system_admin 均无） | — | 只读 | DONE |
| Points | 运营统计 | `GET /api/v1/admin/points/stats` | auditor↑ | — | 只读 | DONE |
| Checkin | 签到状态 / 签到 / 月历史 | `GET /api/v1/checkin/status`、`POST /api/v1/checkin`、`GET /api/v1/checkin/history` | user JWT | Ledger `CHECKIN_REWARD` | `uniqueIndex(user_id,checkin_date)` + 同日重复=成功 | DONE |
| Checkin | 后台签到历史（只读） | `GET /api/v1/admin/users/:id/checkins` | auditor↑ | — | 只读；**无补签/删签/改签路由** | DONE |
| Checkin | 签到配置 | `GET/PUT /api/v1/admin/settings/checkin` | **仅 system_admin** | `checkin_config_updated`（before/after + 操作者） | 配置整体覆盖 | DONE |
| Mall | 商品列表 / 详情 | `GET /api/v1/points/products`、`/points/products/:id` | user JWT | — | 只读 | DONE |
| Mall | 创建兑换 | `POST /api/v1/points/exchange-orders` | user JWT | Ledger `REDEEM` + 订单行 | `uniqueIndex(user_id,idempotency_key)` + key≤64 | DONE |
| Mall | 我的兑换单 | `GET /api/v1/points/exchange-orders`、`/:id` | user JWT（owner scoped） | — | 只读 | DONE |
| Mall | 用户取消 | `POST /api/v1/points/exchange-orders/:id/cancel` | user JWT（仅 PENDING） | Ledger `REDEEM_REFUND` + `cancelled_at/last_operator_id` | reference 唯一 ⇒ 返还恰一次 | DONE |
| Mall | 商品维护 | `POST /points/products`、`PUT /:id`、`PATCH /:id/status` | operations / system_admin（finance 无） | `points_mall_product_created/updated/enabled_changed` | 非幂等（绝对值覆盖） | DONE |
| Mall | 订单列表 / 详情 | `GET /admin/points/exchange-orders`、`/:id` | auditor↑ | — | 只读 | DONE |
| Mall | 开始处理 | `POST /admin/points/exchange-orders/:id/process` | operations / finance / system_admin | `points_mall_order_processed` | 状态机 ⇒ 重复调用无副作用 | DONE |
| Mall | 完成履约 | `POST /admin/points/exchange-orders/:id/complete` | operations / finance / system_admin | `points_mall_order_completed` | 同上；不二次扣分/扣库存 | DONE |
| Mall | 失败 | `POST /admin/points/exchange-orders/:id/fail` | **finance / system_admin**（operations 无） | `points_mall_order_failed`（含 refunded_points/restored_stock） | 同单重复 fail 不重复返还 | DONE |
| Mall | 取消 | `POST /admin/points/exchange-orders/:id/cancel` | **finance / system_admin** | `points_mall_order_cancelled` | 同上 | DONE |

矩阵口径说明：
- `auditor↑` = `readonly_auditor` 显式列举 + 其余角色全部 `Inherits: readonly_auditor`，因此 operations/finance/support/integration/system_admin 都能读。**这是继承面，不是通配**：`internal/authz/bootstrap.go` 里 Points 相关策略 0 条 `Object:"*"`（全仓 74 处 `Action:"*"` 均绑定具体 object）。
- 副作用：`support` 与 `integration` 也继承到积分只读（账户/流水/统计/商城/签到历史）。这是仓库既有继承设计，不是 Points 引入；若判定积分流水对 integration（对接角色）过宽，应在 authz 层收窄继承，而不是在 Points 加白名单——记录为后续项，本轮不改。
- **不存在** `GET /api/v1/points/summary`（§16：用户聚合仅复用 account/ledger，未做商品/订单/流水大端点）。

---

## 2. 本轮改动清单（全部为收口，无新增能力）

| 文件 | 改动 | 理由（审计发现，非风格偏好） |
| --- | --- | --- |
| `internal/modules/pointsmall/contract/types.go` | 删除 `ReasonDisabled = "DISABLED"` | 全仓零引用；下架商品在详情接口直接 404（`GetProductDetail` 返回 `ErrProductNotFound`），不存在 DISABLED reason_code。留着会让前端按文档实现一个永远不会出现的分支 |
| `internal/modules/points/application/mutation.go` | 修正注释 | 原注释称"本轮实际产生数据的只有 ADMIN_ADD/ADMIN_DEDUCT，其余为后续 Phase 预留"——P0 时代残留，实际八种 action 在 P0–P4 全部产生流水。注释错误会误导下一个新增 action 的人跳过 `actionMetas` 登记（未登记 ⇒ `ErrInvalidOperation` 直接拒绝，mutation.go:70-73） |
| `internal/authz/points_rbac_matrix_test.go` | +2 断言：system_admin / operations 不能 `GET /admin/points/accounts` | 文档 §1 与 §2.1 的角色归属此前不一致（一处写 finance/system_admin）。以代码为准修正文档，并把"仅 finance"这条职责分离锁进测试，防止将来有人"顺手"给 system_admin 加回 |
| `docs/HCZ_POINTS_API.md` | 新建 382 行（§61） | 逐端点字段表、错误码、幂等、枚举、reference 规则、角色速查、V1 边界 |
| `docs/api/HCZ_FRONTEND_API_CONTRACT.md` | +§8 Points 索引与前端必读约束、+3 条禁止项 | §25：此前该契约对 Points **零覆盖** |
| `docs/README.md` | +Points 文档区块（P0–P4 + API） | §25：文档入口可发现性 |
| `docs/HCZ_POINTS_P4_FINALIZATION_REPORT.md` | 本文件 | §60 |

根目录卫生（§60）：`AFFILIATE_PG_LOCK_ORDER.md`、`HCZ_GO_PROFIT_GUARD_TARGET_DESIGN.md`、`MONEY_SEMANTICS_TABLE.md` 已从仓库根移入 `docs/`；测试 stdout、PG DSN 试验、临时 SQLite 全部留在 `%TEMP%`，未进仓库，未进文档。

---

## 3. 事务边界（§35，逐条读代码得出，非设计稿）

| 流程 | 事务 | 边界内动作 | 失败后果 |
| --- | --- | --- | --- |
| 订单完成发奖 | 复用**订单完成事务**（`order/application/order_service_child.go:593` 调 `RewardOrderCompleted(tx.Points(), …)`） | 订单状态迁移 + `ORDER_REWARD` 入账 + ledger INSERT | 整体回滚：不存在"已完成但无积分"或"有积分但订单未完成" |
| 退款冲正 | 复用**退款事务**（`order/application/refund/service.go:536`） | 退款记录 + `ORDER_REWARD_REVERSAL` 负向入账 | 整体回滚；`reference=points:order_refund:{refund_record_id}` ⇒ 同一退款最多冲正一次 |
| 签到 | `checkin/application/service.go:91` `unitOfWork.WithinTransaction` | 账户行锁 → `user_checkins` INSERT → `CHECKIN_REWARD` 入账 | 撞 `uniqueIndex(user_id,checkin_date)` ⇒ 视为已成功，不重复发积分；0 积分日仍算签到成功但不产 Ledger |
| 创建兑换 | Mall service 单事务 | 幂等预查 → 商品快照读 → 订单 INSERT → `REDEEM`（账户 `FOR UPDATE`）→ 商品 `FOR UPDATE` 扣库存 → 限购校验 | 任一步失败全回滚：积分不足 ⇒ 不建单、不扣库存、无流水 |
| 兑换失败/取消返还 | 单事务（Admin `adminTransitionWithRefund` / 用户 `CancelOrder`） | 兑换单 `FOR UPDATE` → 状态迁移 → `REDEEM_REFUND` → 账户 → 商品 `FOR UPDATE` 恢复库存 | 返还与恢复库存原子；`refundAndRestore` 以 reference 唯一性为闸门，库存恢复只在真实发生返还时执行 |

---

## 4. 锁顺序（§36）

全局固定顺序：**业务行（兑换单 / 订单 / 签到账户行）→ PointsAccount → PointsProduct**，绝不反向。

- 创建兑换：订单（新行，无竞争）→ Account → Product。
- 返还路径：ExchangeOrder `FOR UPDATE` 先于 Account/Product，因此两个后台操作同一订单只会有一个进入返还分支。
- 不存在 Product→Account 的持锁路径（商品维护只锁商品行，不触碰账户）。
- 结论：**无 ABBA 环**。证据：PG 真并发 `TestPGMallDeadlockFree` + 5 项跨域行锁证明全绿（§8）。

---

## 5. DB 约束与不变量（§37–§42）

DB 级事实约束（`TestPointsFreshInstallSchema` 在全新空库上逐条断言，SQLite/PG 通用）：

| 表 | 约束 | 保护的不变量 |
| --- | --- | --- |
| `points_accounts` | `uniqueIndex(user_id)` | 一人一户（并发首建靠撞索引后重查） |
| `points_ledger` | `reference` `not null` + `uniqueIndex`，size 120 | 幂等最终防线；`§5` 无第二条写路径 |
| `user_checkins` | `uniqueIndex(user_id,checkin_date)` | 单日单人一次签到 |
| `points_exchange_orders` | `uniqueIndex(order_no)`、`uniqueIndex(user_id,idempotency_key)`（key size 64） | 同用户同幂等键只建一单 |
| `orders` / `products` | `reward_enabled` / `reward_points` 快照列 | 历史奖励不随配置漂移 |

**未加的约束及决策（§37 要求逐条回答，不沉默）**：`CHECK (stock >= 0)`、`CHECK (amount <> 0)`、`balance == SUM(ledger)` 触发器**本轮不加**。理由：
1. SQLite 不支持 `ALTER TABLE ADD CHECK`（需重建表），PG 侧 ADD CONSTRAINT 会在任何历史违规行上直接失败并阻断启动 —— 升级路径风险高于收益；
2. `stock<0` 不可达已由 PG 真并发证明（`TestPGMallLastStockConcurrent`/`TestPGMallAccountConcurrent`），实现靠 `FOR UPDATE` + 应用校验；
3. 余额一致性靠 append-only + 单一 `applyMutation` 保证，全局不变量测试覆盖（下条）。
若积分将来获得变现路径，PG 侧 `stock>=0` CHECK 与 `balance==SUM(ledger)` 触发器应作为强制升级项——已写入 §12 风险与 §13 后续。

不变量测试（§38–§42）：
- `TestPointsGlobalInvariants`：全部账户 `balance == SUM(ledger.amount)`，且 `total_earned`/`total_spent` 与各 action 归集口径一致；
- 签到唯一 ↔ `CHECKIN_REWARD` 恰一条（`checkin/integrationtest`）；
- 订单奖励与冲正上界：一单最多 1 条 `ORDER_REWARD`、1 条冲正，冲正绝对值 ≤ 原奖励（`order/integrationtest/e2e/points_order_reward_test.go`）；
- `REDEEM` / `REDEEM_REFUND` 恰一次（含并发同 key、并发同末件库存）；
- PG 并发下 `stock<0` 不可能、负余额只允许出现在 `allowNegative=true` 的两条 action（`ADMIN_DEDUCT`、`ORDER_REWARD_REVERSAL`）。

`actionMetas` 精确值（`points/application/mutation.go`）——`§22` 的实现真源：

| action | allowNegative | trackEarned | trackSpent |
| --- | --- | --- | --- |
| ADMIN_ADD | false | true | false |
| ADMIN_DEDUCT | **true** | false | false |
| ADMIN_COMPENSATION | false | true | false |
| ORDER_REWARD | false | true | false |
| ORDER_REWARD_REVERSAL | **true** | false | false |
| CHECKIN_REWARD | false | true | false |
| REDEEM | false | false | **true** |
| REDEEM_REFUND | false | true | false |

由此锁定的语义（前端/运营不得误解，§21）：`total_spent` 是**历史累计消费**，兑换返还不回滚它（返还计入 `total_earned`）；`ADMIN_DEDUCT` 可把余额打成负数且**不 clamp**，负余额用户不能 REDEEM（`allowNegative=false` ⇒ `ErrNegativeNotAllowed` ⇒ 商城映射为 `ErrPointsInsufficient`）。

---

## 6. RBAC 终局矩阵（§43）

权威断言文件：`internal/authz/points_rbac_matrix_test.go`（本轮 +2 条）。要点：

- 读：`readonly_auditor` 显式列举 7 条 Points GET，其余角色全部继承；**无 `/admin/*` 通配**。
- 资金型写入：`adjust` / `compensate` ⇒ **仅 finance**；`system_admin cannot adjust/compensate` 是断言，不是遗漏（职责分离：超级管理员不能改用户积分）。
- 账户列表：仅 finance（本轮补断言 auditor/operations/system_admin 均 false）。
- 返还型动作：`fail` / `cancel` ⇒ finance + system_admin；`operations fails order=false`、`operations cancels order=false`。
- 商品维护：operations + system_admin；`finance creates product=false`。
- 签到配置：`PUT /admin/settings/checkin` ⇒ 仅 system_admin；`finance updates checkin config=false`；auditor 连该配置 GET 都没有。
- 签到记录：后台只有 GET，POST/DELETE 全 false（`TestCheckinAdmin_noWriteRoutes` 另从路由层断言不存在写路由）。
- 无角色 admin（有 JWT 无角色）：Points 读写全 false。

---

## 7. 十个后台写动作审计矩阵（§44）

| # | 动作 | 审计事件 | 字段（实测） |
| --- | --- | --- | --- |
| 1 | `POST .../points/adjust` | `points_admin_mutation` | admin_id, user_id, action, amount, balance_before, balance_after, reason, reference |
| 2 | `POST .../points/compensate` | `points_admin_mutation` | 同上（action=ADMIN_COMPENSATION，独立 reference） |
| 3 | `POST /admin/points/products` | `points_mall_product_created` | operator_id, product_id, points_price, stock, unlimited_stock, enabled, reason |
| 4 | `PUT /admin/points/products/:id` | `points_mall_product_updated` | operator_id, product_id, before_stock/after_stock, before_points_price/after_points_price, unlimited_stock, enabled, reason（满足 §12 全部要素） |
| 5 | `PATCH /admin/points/products/:id/status` | `points_mall_product_enabled_changed` | operator_id, product_id, enabled, reason |
| 6 | `POST .../exchange-orders/:id/process` | `points_mall_order_processed` | operator_id, exchange_order_id, before_status, after_status |
| 7 | `POST .../exchange-orders/:id/complete` | `points_mall_order_completed` | 同上 |
| 8 | `POST .../exchange-orders/:id/fail` | `points_mall_order_failed` | operator_id, exchange_order_id, reason, refunded_points, restored_stock（§15 齐备） |
| 9 | `POST .../exchange-orders/:id/cancel` | `points_mall_order_cancelled` | 同上 |
| 10 | `PUT /admin/settings/checkin` | `checkin_config_updated` | 操作者 + 变更前后完整 rewards 配置 |

补充：`points_mall_product_missing_during_refund`（Warn）是防御性观测事件，不是第 11 个动作。
口径说明：**用户自助取消**不产独立 logger 事件，其审计链是 `REDEEM_REFUND` 流水（含 reason/reference）+ 订单行 `cancelled_at`/`last_operator_id`；`§44` 只约束后台写动作，此项判为符合。
日志卫生（§57）：Points/Checkin/Mall/Settings 四包内无 token、Authorization、密码、密钥或完整敏感请求体入日志。

---

## 8. 测试与验证证据（§52–§56）

### 8.1 全量回归
`go test ./... -count=1`：**211 包 ok，3 包 FAIL，14 个顶层测试失败**，全部为既有项（归属见 §9）：
`internal/architecture`（4）· `internal/logger`（1）· `internal/selfupdate`（9）。
本轮改动后复跑：`go build ./...` 通过；`points`/`pointsmall`/`checkin`/`settings`/`authz`/`app`/`i18n` 21 包 `-count=1` **全绿（EXIT=0）**。

测试规模（顶层 Test 函数）：`internal/modules/points` 34 · `pointsmall` 24 · `checkin` 29 · `internal/authz` 8 · `migrations` 39，另有 `internal/app/integrationtest` HTTP 冒烟与 `order/integrationtest/e2e` 奖励链路。

### 8.2 PG 真并发（本轮重跑，非引用旧结论）
命令：`TEST_POSTGRES_DSN=… go test -tags integration -p 1 -count=1 -run 'TestPG(Points|Mall|Checkin)|TestRowLock'`
结果：**15 项全部 PASS，0 skip**（本机 PG:5432，2026-10-08）：
- Points 4：ConcurrentCreditNoLostUpdate / ConcurrentDebitAllowsNegative / ConcurrentFirstCreateSingleAccount / IdempotencySameKeyConcurrent
- Mall 5：LastStockConcurrent / SameKeyConcurrent / AccountConcurrent / FailRefundConcurrent / DeadlockFree
- Checkin 1：ConcurrentSingleRecord
- 跨域行锁证明 5：WalletConcurrentDebit / WithdrawalVsOrder / C2CDoubleFreeze / C2CCancelVsSettle / BidirectionalSettleNoDeadlock

**测试工装缺陷（本轮发现，必须写进运行说明）**：三个集成包共享同一个 `hcz_test` 库并各自 `DropTable + AutoMigrate`，`go test` 默认并行会互相拆表，首跑因此出现 `关系 "points_accounts" 不存在 (SQLSTATE 42P01)`。这不是产品缺陷，是工装竞态 ⇒ **Points 集成测试必须 `-p 1` 串行**。已在上表命令中固化。

### 8.3 迁移（§53 / §54）
`TestPointsFreshInstallSchema`：空库走**生产同一入口** `AutoMigrate()`，断言五张表就位 + 4 条唯一索引 + 4 个快照列。
`TestPointsExistingUpgradeKeepsLegacyRowsUnrewarded`：既有库升级后五表就位、历史订单/商品/钱包行数与金额**逐条不变**、历史订单默认 `reward_enabled=false / reward_points=0`（不补发、不回填流水）。
`internal/bootstrap/database/migrations` 整包（39 个顶层测试）此前全量跑绿（约 124s）；本轮 `-p 1` 子集复跑 PASS。

### 8.4 真实 HTTP 冒烟 + 安全冒烟（§55 / §56）
`internal/app/integrationtest/points_http_smoke_test.go`：走**真实 composition root** + `httptest`（不是 service 单测的替身），`TestPointsHTTPSmoke` 15 个命名子测试覆盖账户/流水/签到/商城/兑换/后台读写。
安全子测试 7 例：A 不能读/取消 B 的兑换单（IDOR）、普通用户 token 进 `/api/v1/admin` ⇒ 401/403、只读审计员 adjust/compensate/fail ⇒ 拒绝、operations cancel ⇒ 拒绝、finance 建商品 ⇒ 拒绝、无角色 admin ⇒ 403。
注入验证：`sort=amount;DROP TABLE users` 被白名单忽略（排序固定 `created_at DESC, id DESC`），§4 达成。
口径说明：`pointsmall/transport/http` 包内**没有**包级单测，其 HTTP 行为由 app 级冒烟覆盖；两者合并计为完整覆盖，本报告如实标注。

### 8.5 i18n / 错误码（§26）
Points/Checkin/Mall/Settings 四个 transport 包使用的 42 个 `error.*` 键在 `internal/i18n/messages.go` 三语计数均为 3；`TestAllLocalesExposeSameKeys` 通过；前端 9 个 points 相关键三语齐备。
全仓信封约定（本轮核对并写进契约文档）：业务错误 **HTTP 恒 200**，语义在 body `status_code`（400/401/403/404/409）+ `message_key`；分页键为 `data.pagination`。Points 未自创第二套响应形状。

### 8.6 限流（§34）
复用既有限流：已鉴权 `user` 组对 points/checkin/mall 写路由**未挂路由级限流**，与钱包、订单、C2C 用户写一致（仓库只对 login/giftcard/support/register 等挂规则）。判定：不新建 Points 专属限流框架，作为 **P3 风险**记录（§12）。

---

## 9. 既有失败再评估（§47–§51，逐条独立判定，不复制 Known Issues）

对每个失败回答：**A** 是否由 Points（P0–P4）引入 · **B** 影响面 · **C** 是否阻断 Points 正确性/资金安全 · **D** 修复动作与成本 · **E** 判定。

### 9.1 `internal/architecture` — affiliate/application 12>10、affiliate/domain 7>6
A：**否**。`git ls-tree HEAD` 证明两目录在 HEAD 已分别是 12/7 个 .go 文件（Points 五张表与两模块均在 `internal/modules/points|pointsmall|checkin`，未动 affiliate）。
B：仅架构棘轮指标；affiliate 编译与测试全绿。C：否。
D：成本 = 拆分 affiliate application/domain，属独立重构任务。
E：**KEEP AS KNOWN**（Points 无关，发布不阻断，需 affiliate 责任人排期）。

### 9.2 `internal/architecture` — wallet/transport/http 7>6
A：**否**（HEAD 即 7 文件，P2 基线漏记，P3 已用 `git ls-tree` 更正，本轮复核未变）。B/C：同 9.1。
E：**KEEP AS KNOWN**。

### 9.3 `internal/architecture` — migrations 目录 17>14（本轮含 Points 贡献）
A：**部分**。HEAD 已是 15 个文件（预算 14，`TestDatabaseBootstrapIsSeparatedFromPlatformConnection` 在 HEAD 即 FAIL）；工作区 17 = HEAD 15 + `nav_config_dedup.go`（sitebuilder 工作，非 Points）+ `points_schema_verify_test.go`（**Points P4 新增**）。⇒ Points 把一个已存在的超标项从 15 推到 16（同口径下 17）。
B：单包 17 文件横跨 6 个业务域，已经是可维护性问题，不只是数字。
C：否（约束校验/行锁证明都在这些文件里，功能正常）。
D：**本轮已评估三个选项**：① 把预算从 14 调到 17 —— 违反 §51"不得盲目扩大 file budget"，拒绝；② 把 P4 的 255 行验证测试塞进 `idempotency_verify_test.go` —— 主题错配且掩盖根因（根因是一包多域），拒绝；③ 按域拆分 migrations 包（`migrations/points`、`migrations/affiliate`…）—— 正确解法，但要改生产启动装配路径与 6 个域的既有测试文件，属独立收口任务，**不在本轮做**。
E：**FIX NOW（下一轮，Points 责任人）**，本轮不阻断发布；要求：拆分 migrations 包而非上调预算，并在拆分 PR 里恢复该测试为绿。P4 报告显式记录了这一净增，不允许下一轮把它当"基线既有"洗掉。

### 9.4 `internal/logger` — `TestNewReleaseWritesToConfiguredFile`
A：**否**。失败信息：`TempDir RemoveAll cleanup: unlinkat …release.log: The process cannot access the file because it is being used by another process.`（Windows 句柄占用，lumberjack 仍持有）。
B：仅 Windows 本地工装；Linux 构建不受影响。C：否。
E：**KEEP AS KNOWN**（记录：CI 平台若为 Linux，则该测试非发布风险；Windows 开发者需知悉这是清理噪音而非日志功能缺陷）。

### 9.5 `internal/selfupdate` — 9 项失败
A：**否**。模式：`unsupported_os` 判定、0500 目录在 NTFS 仍可写、`os.Rename` 对目标存在的覆盖语义差异、exe 锁占用。数量与 P2/P3 基线一致（本轮未变）。
B：自更新在 Windows 的测试可信度。C：否（与 Points 无调用关系）。
D：需在 Linux runner 或加平台门禁。E：**KEEP AS KNOWN for Windows local；BLOCK RELEASE only if self-update ships to Windows prod**——该判断属仓库全局发布门禁，不属 Points。

### 9.6 Points 相关新增失败
**0 项**。211/214 与 HEAD 基线同集合，未出现新失败包，也未把新失败并入旧 Known Issues。

---

## 10. 遗留与死代码扫描（§45–§46）

- 全仓**非测试** Go 代码 `TODO`/`FIXME`/`HACK`/`XXX`/`temporary`/`workaround` 计数：**0**。
- 八个 action、七个 reference helper 均有生产调用方；无"预留但从未使用"的 action 类型。
- 唯一死代码：`pointsmall/contract.ReasonDisabled`（本轮删除，见 §2）。
- 复核未误删：`giftcard` 的 `Redeem`（卡密兑换，`internal/modules/giftcard/application/redeem.go` + `internal/workflows/giftcardredeem`）与 Points 的 `REDEEM`（积分兑换）是**两条不同业务链，同名但必须同时保留**（§45）；本轮未改动 giftcard 任何文件，全量回归该模块 4 个含测试包（application / infrastructure/gormstore / integrationtest / transport/http）均 `ok`；`dashboard` 模块里的 "Points" 是趋势图数据点，与积分无关（假阳性，未动）。
- `identity` 模块无残留积分字段（§3：无第二套积分查询实现，用户端积分只由 Points 模块提供）。
- `OrderRewardReversalReference` 不存在——实际名为 `OrderRefundReversalReference`，被 `refund/service.go:536` 使用（核对引用，非死代码）。

---

## 11. 调用链文本（§62）

```
Earn   : 订单完成 → order_service_child.completeTx → pointsSvc.RewardOrderCompleted(tx.Points()) → applyMutation(ORDER_REWARD)
         → account FOR UPDATE → balance+=reward → ledger(reference=points:order_reward:{order_id}) → total_earned
       签到   → POST /api/v1/checkin → checkin.UnderTransaction → account FOR UPDATE → CHECKIN_REWARD
       补偿   → POST /admin/users/:id/points/compensate → adminMutation → replayIdempotent → applyMutation(ADMIN_COMPENSATION)
       后台加 → POST /admin/users/:id/points/adjust(amount>0) → applyMutation(ADMIN_ADD) → trackEarned
Spend  : 兑换   → POST /api/v1/points/exchange-orders → [预查幂等] → 订单 INSERT → Redeem(-amount, allowNegative=false)
         → applyMutation(REDEEM) → total_spent+=|amount| → 商品 FOR UPDATE → stock 扣减 → 限购校验
       后台减 → adjust(amount<0) → applyMutation(ADMIN_DEDUCT, allowNegative=true) → 余额可为负
Reverse: 退款冲正 → refund/service → applyMutation(ORDER_REWARD_REVERSAL, allowNegative=true)
         → reference=points:order_refund:{refund_record_id}（上界=原奖励）
       兑换返还 → fail/cancel(Admin) 或 cancel(User) → ExchangeOrder FOR UPDATE → RedeemRefund(+amount)
         → applyMutation(REDEEM_REFUND) → total_earned → 商品 FOR UPDATE → stock 恢复（仅在实际发生返还时）
Manual : adminMutation = 校验(admin_id/key≤64/amount/reason/reference) → 事务外 reference 预查 → replayIdempotent(同载荷 200 回放/异载荷 409)
         → 事务内 applyMutation → 唯一索引兜底 → logger.Infow("points_admin_mutation")
```

---

## 12. 风险与已知限制清单

| # | 风险 | 等级 | 本轮处置 |
| --- | --- | --- | --- |
| 1 | 用户端积分/签到/兑换写路由无路由级限流（与钱包/订单/C2C 同口径） | P3 | 记录，不新建框架（§34）。上线前若积分成为刷量目标，应在全局限流层补 |
| 2 | `adjust`/`compensate` 不要求 Step-Up，而钱包调整/提现审批/订单退款要求 | 已评估保留 | 不改代码（§6 未把 Step-Up 列为 adjust 必要条件；V1 积分不可变现且权限仅 finance）。文档 §4 已写明：**积分一旦获得变现路径，Step-Up 强制 + 矩阵复审是前置条件** |
| 3 | `frontend/user/.../PointsPanel.vue` 是静态占位（`ref(0)` + 空数组）却已挂 `/me/points` | **P2，发布可见** | 不在 P4 后端轮接线（属前端集成工作）。已在契约文档标注"不得把占位 0 当真值"；上线前必须以 `GET /points/account` + `/points/ledger` 替换，否则积分用户看到假 0 |
| 4 | 业务错误 HTTP 恒 200 + body `status_code` | 既有全局约定 | 契约文档 §0/§8 显式写明，防止前端拦截器只看 HTTP 状态 |
| 5 | `pointsmall/transport/http` 无包级单测 | 口径说明 | 由 app 级真实 HTTP 冒烟覆盖，已标注 |
| 6 | Points 集成测试必须 `-p 1`（三包共享 `hcz_test` 库竞态） | 工装 | 已在 §8.2 固化命令；建议后续给集成包加串行标记 |
| 7 | `migrations` 包文件数 17>14（Points 净增 1） | P3 | §9.3，FIX NOW 下一轮按域拆分，禁止上调预算 |
| 8 | 未加 `CHECK`/触发器级 DB 约束 | 决策 | §5 给出理由与升级条件 |
| 9 | 无指标采集（Prometheus） | Later | 按 §58 本轮**不引入**新监控栈 |
| 10 | `support`/`integration` 继承只读积分数据 | 观察项 | 属仓库继承设计；如需收窄应在 authz 层处理，记录为后续 |

---

## 13. Git 收口与提交拆分建议（§59）

工作区现状：`git status --short` 共 162 条（已跟踪改动 120 条 + 未跟踪 42 条）。必须区分三类，**不得混提交**：

1. **Points P0–P4 交付**（建议单独 PR 链）：`internal/modules/{points,pointsmall,checkin}/`、`internal/bootstrap/{points,pointsmall,checkin,stepup}/`、`internal/modules/settings/schema/points/` + `settings/transport/http/checkin_handler*`、`internal/authz/{bootstrap.go,points_policy_test.go,points_rbac_matrix_test.go}`、`internal/app/{container,httpserver}/*` 中的 Points 接线、`internal/app/integrationtest/`、`internal/bootstrap/database/migrations/{registry.go,points_schema_verify_test.go,rowlock_proof_test.go}`、`internal/i18n/messages*.go`、`docs/HCZ_POINTS_*`、`docs/HCZ_POINTS_API.md`、`docs/api/HCZ_FRONTEND_API_CONTRACT.md`、`docs/README.md`。
2. **他人/其他业务在途改动**（不要打包进 Points 提交）：`frontend/**`（site-builder、wallet UI、i18n 文案等 40+ 文件）、`internal/modules/sitebuilder/**` 与 `internal/bootstrap/database/migrations/nav_config_dedup.go`、`internal/modules/catalog|order|wallet|procurement|content|fulfillment|identity/**` 中与 Points 无关的行、根目录 `HCZ_FULL_SECURITY_AUDIT_REPORT.md`、`bj.png`、`scripts_qa/`。
3. **文件位置迁移**：根目录三份 .md → `docs/`（建议独立一个 `docs:` 提交，便于 review 判定"无内容变更"）。

建议拆分顺序（每步都可独立回滚）：
`feat(points): P0 core + admin adjust/compensate` → `feat(order): P1 reward & reversal wiring` → `feat(checkin): P2 check-in + config audit` → `feat(pointsmall): P3 mall + exchange lifecycle` → `feat(authz)+refactor(routes): P4 RBAC 终局矩阵与路由前缀收口` → `docs(points): API 契约 + P4 收口报告` → `chore(docs): 根文档迁入 docs/`。

本轮执行边界（已遵守）：未 `git add`、未 commit、未 push；未使用 `git reset --hard` / `git clean -fd`；未覆盖 `bj.png`、`scripts_qa/`、`HCZ_FULL_SECURITY_AUDIT_REPORT.md` 等用户自有文件；根目录未落任何测试输出或临时 SQL。

---

## 14. 退出门禁（§64，24 项）

| # | 门禁 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | 无第二套积分查询实现 | PASS | §10；用户端仅 `/points/account`+`/points/ledger` |
| 2 | 流水过滤白名单 + 禁任意 SQL sort | PASS | `ledgerFilterFromRequest`，注入用例被忽略（§8.4） |
| 3 | 禁 PUT/DELETE ledger、禁改历史 amount/balance_after | PASS | 路由清单实测：Points 侧无任何 PUT/DELETE（唯一 PUT 是 `PUT /admin/points/products/:id` 商品维护，属库存资产而非流水）；`internal/modules/points/transport/http/routes.go` 只有 GET/POST adjust·compensate；签到后台另有 `TestCheckinAdmin_noWriteRoutes` 从路由层断言无写路由 |
| 4 | adjust 六要素（JWT+RBAC+key+reason+行锁+前后余额） | PASS | §7 第 1 行 |
| 5 | 只读审计员仅查询 | PASS | §6 |
| 6 | 人工补偿走独立 action、不重放订单生命周期 | PASS | `ADMIN_COMPENSATION` + §2 说明 |
| 7 | 后台签到只读、无补签/删签/改签 | PASS | §1/§6 |
| 8 | 签到配置变更审计 + 上限 1_000_000 | PASS | `checkin_config_updated` + `MaxCheckinReward` |
| 9 | 有历史订单的商品不可物理删除 | PASS | 无 DELETE 商品路由；仅 `PATCH /status` |
| 10 | 库存调整审计六要素 | PASS | `points_mall_product_updated`（§7 第 4 行） |
| 11 | 终态不可被普通 API 逆转 | PASS | 状态机 + `adminTransition` 前置态校验 |
| 12 | COMPLETED 后无自动撤销（补偿为人力，不改原单/不恢复库存） | PASS | `ADMIN_COMPENSATION` 语义 + 文档口径 |
| 13 | FAILED/CANCELLED ⇒ 返还恰一次 + 恢复恰一次 | PASS | reference 唯一 + PG 并发（§8.2） |
| 14 | 统计只聚合三张事实表，未新增总表 | PASS | `GetStats` 只读 `points_ledger`/`user_checkins`/`points_exchange_orders` |
| 15 | `total_spent` 语义对前端/运营无歧义 | PASS | §5 + 契约文档禁止项 |
| 16 | 负余额可见、不 clamp、不能兑换 | PASS | `actionMetas` + `TestAdminAdjustDebitCanProduceNegativeBalance` + `negative_only=true` |
| 17 | IDOR：user_id 只取 JWT、兑换详情 owner scoped | PASS | §8.4 安全子测试 |
| 18 | 每个 `:id` 解析/存在/域校验 | PASS | `ParseParamUint` + 404 分支 + 非法参数用例 |
| 19 | 溢出与上界（MaxInt64 / 商品积分价 / 快照 BIGINT） | PASS | `MaxPointsAmount`、`TestAdminAmountTooLargeMapsToBadRequest` |
| 20 | 三语 i18n 完整 + 稳定错误码 | PASS | §8.5 |
| 21 | 事务边界与锁顺序成文且无 ABBA | PASS | §3/§4 |
| 22 | 迁移：全新 + 升级双路径验证 | PASS | §8.3 |
| 23 | 真实 HTTP 冒烟 + 安全冒烟 | PASS | §8.4 |
| 24 | 文档/API 契约/索引同步 + 报告落地 | PASS | §2、§13 |

---

## 15. 最终结论（§65–§66）

**Verdict：PASS WITH CONDITIONS**

条件（Points 侧）：
1. 前端接线前必须替换 `PointsPanel.vue` 占位实现，并按 `docs/HCZ_POINTS_API.md` 的 `status_code` 信封与 `pagination` 键实现（不是"顺手加个页面"，是发布可见缺陷）；
2. 集成测试运行说明固化 `-tags integration -p 1`；
3. 下一轮按域拆分 `internal/bootstrap/database/migrations`，恢复该架构测试为绿，禁止上调预算数字；
4. 若积分将来出现任何变现路径（转赠/抵现/交易），Step-Up 强制 + RBAC 矩阵复审 + PG `stock>=0` CHECK 是上线前置条件。

**POINTS SYSTEM READY FOR FRONTEND INTEGRATION = YES**
后端能力、契约、错误码、幂等、RBAC、真实 HTTP 冒烟全部就位；三份文档（API 契约 / 前端契约 §8 / 本报告的矩阵）足以支撑前端独立开发。未接线是前端的待办，不是 Points 的就绪缺口。

**POINTS SYSTEM READY FOR PRODUCTION = NO**
阻塞点**全部不在 Points 自身**，分两类，不与积分体系混为一谈：
- 仓库全局：`internal/architecture` 4 项文件预算 FAIL（其中 3 项 HEAD 即失败、与 Points 无关；1 项见条件 3）；`internal/selfupdate` 9 项 + `internal/logger` 1 项 Windows 工装失败未澄清；`HCZ_PRODUCTION_GATE_CHECKLIST.md` 的全仓门禁未由本轮重新签署。
- 交付面：User 前端 Points 面板未接线（条件 1）；用户写路由限流策略需要全局限流层统一决策（§12-1）。

Points 侧无 blocking defect：资金一致性、幂等、并发、状态机、审计、权限、迁移、i18n 八项均有本轮重跑的一手证据（§5–§8）。

**READY FOR P5 / NEW POINTS FEATURES = NO（按 §67 本轮到此停止，不启动任何新功能）**
