# HCZ Points P3 Mall Report — 积分兑换商城

- 阶段：P3 Points Mall / Redemption
- 日期：2026-10-07
- 项目根：`E:\Users\orang\Downloads\Compressed\hcz_v1`
- Git Baseline：`59c8ae8f94038495c46720d885a5c46e4b22fffd`（branch `main`；工作区 P3 实现未提交，与 P0/P1/P2 同口径）
- 前置：Points Audit（READY WITH CONDITIONS）、P0 Points Core（PASS）、P1 Order Reward（PASS）、P2 Check-in（PASS）

---

## 1. Verdict

**PASS** — 全部 Exit Gate 满足：

| 退出条件 | 结果 |
|---|---|
| points_products / points_exchange_orders 独立表 | PASS（不复用 recharge/orders/wallet/procurement） |
| 用户商品 list/detail | PASS |
| 创建兑换（同事务扣积分 + 扣库存 + 订单 + Ledger） | PASS |
| 积分不足 / 库存不足 / 商品禁用 / 限购 | PASS |
| 快照（价格/名称/履约类型） | PASS |
| Idempotency-Key 幂等（应用层预查 + DB 唯一 + 并发同 Key） | PASS |
| 并发抢最后一件 / 同账户并发消费 / 并发 fail 退款 | PASS（PG 真并发实测） |
| 用户取消（仅 PENDING） | PASS |
| Admin process/complete/fail/cancel（状态机 + Reason） | PASS |
| REDEEM_REFUND 返还 + 库存恢复（幂等，恰一次） | PASS |
| 积分不变量 balance == SUM(ledger) | PASS（全测试断言） |
| 库存不变量（无超卖） | PASS |
| RBAC / Audit / i18n | PASS |
| Fresh migration / Existing upgrade | PASS（`internal/bootstrap/database/migrations` 45s 全绿） |
| P0/P1/P2 回归 | PASS（无新增高优先级 regression） |

**READY FOR P4 POINTS ADMIN / FINALIZATION = YES**

---

## 2. Changed Files

### 新增（Points 核心扩展）
| 文件 | 作用 |
|---|---|
| `internal/modules/points/contract/types.go` | `RedeemReference`/`RedeemRefundReference`、`RedeemInput`/`RedeemRefundInput` |
| `internal/modules/points/contract/ports.go` | UseCase 新增 `Redeem`/`RedeemRefund`（调用方事务内执行） |
| `internal/modules/points/application/redeem.go` | 两个 mutation wrapper（幂等预查 + reference 全局唯一兜底） |

### 新增（Mall 模块，独立顶层）
| 文件 | 作用 |
|---|---|
| `internal/modules/pointsmall/domain/points_product.go` | `points_products`（UnlimitedStock bool；禁 -1 magic number） |
| `internal/modules/pointsmall/domain/exchange_order.go` | `points_exchange_orders`（状态机 + 快照 + 幂等键联合唯一） |
| `internal/modules/pointsmall/contract/errors.go` | 稳定业务错误集 |
| `internal/modules/pointsmall/contract/ports.go` | Repository / Transaction / UnitOfWork / UseCase 端口 |
| `internal/modules/pointsmall/contract/types.go` | ProductInput / CreateExchangeInput / ExchangeResult 等 |
| `internal/modules/pointsmall/application/service.go` | 业务全量（见 §4 锁顺序） |
| `internal/modules/pointsmall/infrastructure/gormstore/store.go` | Store = Repository + UnitOfWork（FOR UPDATE 行锁） |
| `internal/modules/pointsmall/transport/http/user_handler.go` | 用户 6 端点 |
| `internal/modules/pointsmall/transport/http/admin_handler.go` | Admin 10 端点（enabled 用 `*bool`；空 body 容忍 EOF） |
| `internal/modules/pointsmall/transport/http/routes.go` | 路由注册函数 |
| `internal/bootstrap/pointsmall/handlers.go` | Handler 装配 + compile-time guard |
| `internal/modules/pointsmall/integrationtest/mall_test.go` | SQLite 顺序 + 并发（单连接串行，应用语义验证） |
| `internal/modules/pointsmall/integrationtest/pg_mall_test.go` | PG 真并发（`//go:build integration`） |

### 修改（接线 / 治理）
| 文件 | 作用 |
|---|---|
| `internal/app/container/container.go`、`repositories.go`、`services_application.go` | PointsmallRepo / PointsmallService 装配（依赖 PointsService） |
| `internal/app/httpserver/router.go`、`routes_storefront.go`、`routes_admin.go` | user/admin handler 构造与注册 |
| `internal/authz/bootstrap.go` | 4 角色策略（readonly_auditor / operations / finance / system_admin） |
| `internal/authz/points_policy_test.go` | RBAC 覆盖测试（新增） |
| `internal/i18n/messages.go` | 三语 +14 键 |
| `internal/bootstrap/database/migrations/registry.go` | AutoMigrate 追加商品/兑换表 |

---

## 3. Product Schema — points_products

独立商品模型（不复用 recharge/product；业务隔离优先于表数最少化）。V1 字段：

- `id`
- `name` / `subtitle` / `description` / `cover`（仅存资源 URL，本轮不开发上传系统）
- `points_price` BIGINT（>0，禁 float/0/负）
- `stock` BIGINT（可用库存，>=0；Admin 直接设绝对值）
- `unlimited_stock` bool（true=无限库存；禁用 `-1` magic number）
- `enabled` bool（下架禁新建，历史 PENDING 订单仍可履约）
- `sort`
- `per_user_limit` int64（0=不限；统计 PENDING/PROCESSING/COMPLETED，排除自身订单）
- `fulfillment_type`（V1 仅 `MANUAL`）
- `instructions`（人工履约说明）
- `created_at` / `updated_at`

有兑换订单引用的商品禁止物理删除（disable / soft 语义）。

## 4. Exchange Schema — points_exchange_orders

独立兑换订单（不复用 orders/recharge_orders；order_no 用 `serial.Generate("PX")` 独立编号空间）。

- `order_no`（唯一）
- `user_id`
- `product_id`
- `product_name_snapshot` / `unit_points` / `quantity`(恒1) / `total_points` / `fulfillment_type_snapshot`（快照固化，商品改价不影响历史订单）
- `status`（PENDING / PROCESSING / COMPLETED / FAILED / CANCELLED）
- `idempotency_key`（与 user_id 联合唯一：`UNIQUE(user_id, idempotency_key)`）
- `reason` / `last_operator_type` / `last_operator_id`
- `completed_at` / `failed_at` / `cancelled_at`
- `created_at` / `updated_at`

状态机（终态禁回退，本轮无 reopen）：
`PENDING → PROCESSING / FAILED / CANCELLED`；`PROCESSING → COMPLETED / FAILED / CANCELLED`；COMPLETED/FAILED/CANCELLED 为终态。用户取消仅限 PENDING；PROCESSING 后禁止用户反悔（后台可能已开始履约）。

## 5. Points Mutation

- 创建兑换：`REDEEM`（allowNegative=false，trackSpent=true；reference=`points:redeem:{order_id}`，P0 语义保留，同一订单最多 1 个 REDEEM，reference 全局唯一为最终防线）。
- 失败/取消返还：`REDEEM_REFUND`（allowNegative=false，trackEarned=true；reference=`points:redeem_refund:{order_id}`；同一订单最多 1 个正式返还）。**不删除/修改历史 REDEEM 流水**（append-only）。
- `total_spent` 语义（沿用 P0 已锁定）：REDEEM 计入 total_spent；REDEEM_REFUND **不回滚** total_spent（= 历史毛消费），返还计入 total_earned。P3 按 P0 语义实现并在本报告固化，P4 如需"已退款消费"口径应另行设计聚合字段而非改动累计语义。

## 6. Transaction Boundary / Lock Order

创建兑换事务（全部成功 COMMIT / 任一失败 ROLLBACK）：

```
幂等预查（user_id+key）→ 快照读商品 → INSERT 兑换订单
→ REDEEM（SELECT account FOR UPDATE → 校验负余额 → UPDATE account → INSERT ledger）
→ 锁商品 FOR UPDATE（重校验 enabled/库存）→ 扣减库存
→ 限购 count（排除自身订单；账户行锁已串行化同用户并发）
→ buildResult（事务内权威余额）
```

返还（AdminFail / AdminCancel / 用户取消）事务：

```
幂等预查 → 锁订单 FOR UPDATE → 状态校验 → 锁账户 FOR UPDATE
→ REDEEM_REFUND → 恢复库存（限库存商品）→ 状态终态化 → buildResult
```

**统一锁顺序**（防死锁，PG 死锁压力实测无稳定死锁）：
1. Points Account（REDEEM / REDEEM_REFUND 内部加锁）
2. Points Product（FOR UPDATE）

返还路径先锁订单行（ExchangeOrder → Account → Product），创建路径先锁账户（Account → Product）；订单行锁只在返还路径内出现且不与账户/商品行构成环，`lock_timeout=5s` 兜底。禁止用简单 retry 掩盖锁序缺陷。

## 7. Idempotency

- 创建兑换：`Idempotency-Key` 必填（gin 头校验）。应用层 `(user_id, idempotency_key)` 预查返回原订单 + DB `UNIQUE(user_id, idempotency_key)` 最终防线（并发同 Key 撞唯一索引 → 回滚 → 事务外重读原订单，**不重复扣分/扣库存**）。用户可控 Key 不直接进唯一索引——Key 存列（≤120 字符），reference 用订单 ID 派生，无超长索引问题。
- Admin 状态动作：锁订单行 + 状态校验（终态幂等返回当前状态，不重复返还）。
- REDEEM / REDEEM_REFUND：reference 全局唯一索引兜底（同一 order_id 最多各 1 条）。

## 8. Stock Model

- `stock` = 可用库存（创建时同事务扣减，`SELECT ... FOR UPDATE` 后 `stock--`，禁"查>0 再事务外减"）。
- `unlimited_stock=true` 跳过库存扣减与恢复。
- 库存不变量：`initial_stock - 成功扣减 + 返还恢复 = current_stock`（PG 测试断言）。

## 9. User APIs（HCZ V1 风格：恒 200 + `{status_code,msg,data}`）

| 端点 | 说明 |
|---|---|
| `GET /api/v1/points/products` | enabled=true 商品列表（分页） |
| `GET /api/v1/points/products/:id` | 详情 + can_redeem/reason_code UX 辅助（不泄露 admin 内部字段） |
| `POST /api/v1/points/exchange-orders` | 创建兑换；**Idempotency-Key 必填**；客户端禁止提交价格/名称/user_id |
| `GET /api/v1/points/exchange-orders` | 自己的订单列表（?status=，分页） |
| `GET /api/v1/points/exchange-orders/:id` | 详情（store 层 `WHERE id AND user_id`，IDOR 防御） |
| `POST /api/v1/points/exchange-orders/:id/cancel` | 用户取消（仅 PENDING） |

## 10. Admin APIs

| 端点 | 说明 |
|---|---|
| `GET/POST /admin/points/products` | 列表（可含下架）/ 创建 |
| `PUT /admin/points/products/:id` | 更新（库存=绝对可用值 >=0） |
| `PATCH /admin/points/products/:id/status` | 上下架（`{enabled:*bool}`，false 合法） |
| `GET /admin/points/exchange-orders` | 订单列表（?status=） |
| `GET /admin/points/exchange-orders/:id` | 详情 |
| `POST /admin/points/exchange-orders/:id/process` | PENDING→PROCESSING（不改积分/库存） |
| `POST /admin/points/exchange-orders/:id/complete` | PROCESSING→COMPLETED |
| `POST /admin/points/exchange-orders/:id/fail` | →FAILED（Reason 必填；返还+恢复库存） |
| `POST /admin/points/exchange-orders/:id/cancel` | →CANCELLED（Reason 必填；返还+恢复库存） |

Admin Bearer + RBAC + Reason（fail/cancel 必须）+ Audit（日志 operator/order_id/before/after/refunded_points/restored_stock）+ Row Lock。

## 11. RBAC

沿用 Casbin `NormalizeObject`（object 形如 `/admin/points/products`）：

| 角色 | 权限 |
|---|---|
| readonly_auditor | 商品/订单 GET 只读 |
| operations | 商品写（POST/PUT/PATCH status）+ process/complete（不授 fail/cancel，涉资金返还） |
| finance | fail/cancel（与积分调整同资金域） |
| system_admin | 全量通配（先例：`/admin/settings/checkin *`） |

未使用万能 admin 绕过作为正式实现。`internal/authz/points_policy_test.go` 覆盖无 Token→401、无权限→403、有权限→成功（既有鉴权中间件体系）。

## 12. Concurrency（PG 真并发实测，`-tags integration`）

| 测试 | 断言 | 结果 |
|---|---|---|
| `TestPGMallLastStockConcurrent` | 10 用户抢 1 件 → 恰 1 成功、stock=0、无超卖 | PASS |
| `TestPGMallSameKeyConcurrent` | 20 goroutine 同 Key → 1 订单/1 REDEEM/库存减 1 | PASS |
| `TestPGMallAccountConcurrent` | 10 goroutine 同用户消费（余额=价）→ 恰 1 成功、无负余额 | PASS |
| `TestPGMallFailRefundConcurrent` | 10 goroutine 并发 fail → 恰 1 次 REDEEM_REFUND/1 次库存恢复 | PASS |
| `TestPGMallDeadlockFree` | 异用户抢同品 + 同用户换不同品混跑，lock_timeout=5s，无稳定死锁 | PASS |

SQLite（`mall_test.go`）单连接串行验证应用语义与幂等逻辑；SQLite 不证明真并发（多连接 SQLITE_BUSY 已知，非缺陷）。

## 13. Tests

- SQLite 顺序场景 15 个（创建/列表/详情/快照/积分不足/库存不足/禁用/限购/取消（含重复 cancel）/process/complete/fail（含重复 fail）/fail 返还/改价后旧单返还按快照/无限库存/IDOR/禁用商品历史订单可履约）+ 并发 4 个（单连接串行：同 Key 幂等、最后一件、同用户消费、并发 fail 退款）。
- PG 真并发 5 项（§12）。
- 全测试断言积分不变量 `points_accounts.balance == SUM(points_ledger.amount)`（helper `assertInvariants`）。
- 回归：`go build ./...` PASS；`go vet ./...` PASS；points/checkin/pointsmall/order/fulfillment/affiliate/httpserver/authz/migrations 全绿；`go test ./... -count=1` 仅剩既有基线失败（见 §15）。

## 14. Migration

`internal/bootstrap/database/migrations/registry.go` AutoMigrate 追加 `PointsProduct`/`ExchangeOrder`。Fresh install 与 existing DB upgrade 均经 `internal/bootstrap/database/migrations` 45s 测试 PASS（含 registry 一致性验证，无 P0/P1 曾出现的 registry/file 不一致复发）。

## 15. Regression

`go test ./... -count=1` 失败清单与基线对照：

| 失败 | 基线口径 |
|---|---|
| architecture：affiliate/application 12>10、affiliate/domain 7>6、migrations 16>14 | P1/P2 既有基线，错误内容一致 |
| architecture：wallet/transport/http 7>6 | **HEAD 既有**（git ls-tree 证实 7 文件已在 HEAD；P2 基线漏记），P3 未触碰 wallet |
| logger `TestNewReleaseWritesToConfiguredFile` | Windows 文件锁既有 |
| selfupdate 9 项 | Windows 平台语义既有（数量与 P2 一致） |
| supportticket flaky | 本次未出现（flaky 口径，单跑 PASS） |

**无 P3 新增 regression；新失败未归入旧问题。**

## 16. Known Issues / Limitations

- 积分不足时不创建订单、不扣库存、不产生 Ledger（事务回滚保证）。
- PROCESSING 后用户不可取消（V1 决策）。
- COMPLETED 后无售后路径（P4 若需要单独设计 After-Sale，不破坏状态机）。
- 未做 daily_limit（V1 仅 per_user_limit，避免 scope 膨胀）；无冻结积分/秒杀/兑换码池/自动供应商履约/实物物流。
- 商品 disable 仅禁新建，历史 PENDING 订单继续可履约。
- `total_spent` 不回滚退款（P0 锁定语义），已在 §5 固化。

## 17. P4 Readiness

Points Mall / Redemption 已按审计方案完成，独立 Domain、独立表、快照、幂等、行锁、真并发证明、不变量齐全，无阻塞后续 Admin 收尾的结构问题。

**READY FOR P4 POINTS ADMIN / FINALIZATION = YES**
