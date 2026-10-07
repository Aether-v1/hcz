# HCZ Points — P1 Order Reward 实施报告

- 日期：2026-10-07
- Git Baseline：branch `main`，HEAD `35496c4`（P1 全部改动在工作区，未提交；未动用户既有未提交改动）
- 前置：`docs/audits/HCZ_POINTS_BACKEND_AUDIT.md`（READY WITH CONDITIONS）；P0 `docs/HCZ_POINTS_P0_IMPLEMENTATION_REPORT.md`（PASS）
- Verdict：**PASS**
- P2 Readiness：**READY FOR P2 CHECK-IN = YES**

---

## 1. 本轮范围（已完成）

| 项 | 状态 |
|---|---|
| 商品固定积分配置（reward_enabled / reward_points） | ✅ |
| 订单创建时积分快照（父单/子单） | ✅ |
| 统一订单完成 Lifecycle（收口 4 条完成路径） | ✅ |
| ORDER_REWARD 发放（仅 COMPLETED；DB 幂等） | ✅ |
| 全额/部分退款 ORDER_REWARD_REVERSAL 冲正（floor 累计算法） | ✅ |
| 商品/订单 Admin、用户 API reward 字段 | ✅ |
| 并发、幂等、DB 不变量测试 | ✅ |
| 佣金回归（Manual/Auto/Procurement 漏触发修复） | ✅ |
| 全量回归 | ✅（仅既有失败，无新增） |

禁止项（签到/商城/兑换/VIP/任务）未实施。

## 2. 商品奖励 Schema（固定积分模式）

商品实体 `internal/modules/catalog/product/domain/product.go`：

```go
RewardEnabled bool  `gorm:"column:reward_enabled;not null;default:false" json:"reward_enabled"`
RewardPoints  int64 `gorm:"column:reward_points;not null;default:0" json:"reward_points"`
```

- 校验（`application/write/{create.go,update.go}`、`transport/http/admin_handler.go`）：
  - `reward_enabled=false` → `reward_points` 可为 0（写入时归 0）；
  - `reward_enabled=true` → 必须 `reward_points > 0`，否则 `productcontract.ErrRewardPointsInvalid`（HTTP key `error.reward_points_invalid`，`contract/errors.go`）；
  - 禁止负数；BIGINT 边界由 `int64` 保证。
- 用户商品 API 返回 `reward_enabled` / `reward_points`（`transport/presenter/product.go`、`transport/http/public_view.go`），供前端「完成订单可得 X 积分」；不泄露内部配置元数据。

## 3. Order Snapshot（订单创建时固化）

`internal/modules/order/application/order_service.go` `CreateOrder`：

- 父单：`RewardEnabled / RewardPoints = Σ 子商品固定积分`（`parentRewardEnabled / parentRewardPoints` 累计）；
- 子单：单商品固定积分（helper `orderRewardPoints(p)`：reward_enabled=false 或 <=0 → 0）；
- 订单完成/退款一律**只读订单快照字段**，禁止回读商品当前配置（测试 TestP1_Reward_Snapshot 验证：下单 100 → 改商品 200 → 旧单完成 +100 / 新单完成 +200）。
- 历史订单（无快照）：不补发、不凭当前商品配置推测。

## 4. Unified Completion Lifecycle（统一完成生命周期）

### 4.1 权威入口（`internal/modules/order/application/order_service_child.go`）

```go
CompleteOrderInTx(tx, orderID, now)          // 事务内：行锁 → completedTransitionAllowed(processing/paid/fulfilling) → UpdateFields(completed) → 子单早退 → 父单副作用
CompleteParentSideEffects(parentID)          // 自开事务：父单 completed 后统一副作用（Affiliate → Points），幂等
CompleteOrderByProcurement(orderID)          // 采购回调入口（自开事务包装 CompleteOrderInTx）
completeOrderSideEffectsInTx(tx, order)      // Affiliate.HandleOrderCompletedInTx → Points.RewardOrderCompleted，同事务
completedTransitionAllowed(status)           // processing / paid / fulfilling → completed；completed → completed 幂等
```

关键约束（用户决策落地）：
- 任一副作用失败 → **整个 completed 事务回滚**（不允许 Order=completed 而 Reward=missing）；
- 已 completed → 直接返回 nil（行锁幂等第一层）；
- 子订单完成不触发子单奖励/佣金（副作用归父单）；
- **Points 模块不反向依赖 Order**（依赖方向：Order Application → Points Contract）。

### 4.2 四条完成路径收口

| 路径 | 原行为 | 现行为 |
|---|---|---|
| ① Admin `UpdateOrderStatus(→completed)` | 父单经 `completeParentOrderInTx` + 事务外 `HandleOrderCompleted` | 事务内 `completeParentSideEffectsInTx`（Affiliate+Points 同事务）；子单走 `CompleteOrderInTx`；`SyncParentStatus` 检测父单 completed → `CompleteParentSideEffects` |
| ② Manual Fulfillment `CreateManual` | 直接 `UpdateFields(completed)`（绕过 Affiliate/Points） | `CompleteOrderInTx(tx, order.ID, now)`（nil 退化旧行为，测试兼容） |
| ③ Auto Fulfillment `CreateAuto` | 同上绕过 | 同上 |
| ④ Procurement `HandleUpstreamCallback` delivered | 直接 `UpdateStatus(completed)`（绕过；父单只 SyncParentStatus） | `s.orderLifecycle.CompleteOrder(localOrderID)`（container 注入 `orderCompletionAdapter` → `CompleteOrderByProcurement`）；父单 completed → `CompleteParentSideEffects` |

Procurement 接线（`internal/modules/procurement/`）：
- `contract/ports.go`：`OrderLifecycle` 增加 `CompleteOrder / CompleteParentSideEffects`；新增独立端口 `OrderCompletion`；
- `infrastructure/gormstore/lifecycle.go`：`SetOrderCompletion / CompleteOrder / CompleteParentSideEffects`（nil 退化旧直接写 completed）；
- `application/callback.go`：delivered 分支调用 `CompleteOrder`；
- container（`internal/app/container/services_integration.go`）持 `orderCompletionAdapter{svc: c.OrderService}` 注入（adapter 置于 container 层，满足 procurement 包文件数/函数所有权架构守卫）。

Fulfillment（`internal/modules/fulfillment/application/service.go`）：本地端口 `OrderCompletionService`（`CompleteOrderInTx` + `CompleteParentSideEffects`），`CreateManual/CreateAuto` 收口；`SyncParentStatus` 返回 completed 时调 `CompleteParentSideEffects`。

## 5. Points Reward（ORDER_REWARD）

- `internal/modules/points/contract/types.go`：`OrderRewardInput{UserID, OrderID, Amount>0, Reason, Reference}`；`OrderRewardReference(orderID) = "points:order_reward:{order_id}"`；常量 `ActionOrderReward / SourceOrderReward`。
- `internal/modules/points/application/order.go` `RewardOrderCompleted`：
  - 幂等双层：reference 预查 `GetLedgerEntryByReference`（先查后写）+ **DB 唯一索引兜底**（`isDuplicateKeyError` 静默跳过）；
  - 走 P0 权威 mutation `applyMutation`（ActionMeta：`ORDER_REWARD {allowNegative:false, trackEarned:true}`）；
  - Amount<=0 / OrderID=0 / UserID=0 → `ErrInvalidAmount`；不发无意义 0 积分流水。
- 仅当 `order.RewardEnabled && order.RewardPoints > 0` 且父单进入 completed 时发放；奖励绑定 **parent_order_id**（父单快照）。

## 6. Affiliate Regression（佣金收口）

`internal/modules/affiliate/application/commission.go`：
- 新增 `HandleOrderCompletedInTx(repoTx, order)`：在调用方事务内生成佣金（幂等检查 + 批量插入 + ledger 全走 repoTx 视图）；
- 唯一约束 `(order_id, beneficiary_user_id, level, commission_type)` 兜底防双佣金；
- `handleOrderCompletedCore` 抽取公共逻辑；原 `HandleOrderCompleted(orderID)` 保留独立事务兼容旧调用方（unique 冲突静默）。
- 修复：Manual/Auto/Procurement 此前绕过 Affiliate（漏佣金），现全部经统一 Lifecycle → 三路径各恰好一次佣金（测试断言 count==1）。

## 7. Refund Reversal（ORDER_REWARD_REVERSAL）

`internal/modules/order/application/refund/{service.go,wallet.go}`：

- `adminManualRefundInTx` / `AdminRefundToWalletInTx` 创建退款记录后、reseller 之前调用共享 `reverseOrderRewardInTx(tx, order, amount, refundedBefore, record.ID)`；
- 触发条件：仅父单（`ParentID==nil`）；仅 `RewardEnabled && RewardPoints>0`；仅已发放过 `ORDER_REWARD`（`SumOrderRewards>0`）；无快照/历史订单不冲正；
- 全额退款保证最终累计冲正 == 原始奖励（测试 30%+30%+40% 与 33/33/34 示例均精确 100）。

### 7.1 部分退款公式（floor 累计算法，防累计 rounding error）

```text
target = floor(original_reward × cumulative_refunded / original_paid)
if target <= reversed           → return（幂等，含重复退款调用）
delta  = target - reversed
if delta > (reward - reversed)  → delta = reward - reversed（安全阀，防并发超额）
if delta <= 0                   → return
ReverseOrderReward(Amount=delta, Reference=points:order_refund:{record_id})
```

- `cumulative_refunded = refundedBefore + amount`（订单累计退款）；`original_paid = TotalAmount`，USDT 结算单（`UsdtTotalAmount>0`）取 `WalletPaidAmount`；
- 金额 Decimal、积分 BIGINT int64；禁止 float；
- 本轮修复一个实现 bug：早期版本把 cap 误作用于**累计目标 target**（`target=min(target,remaining)` 后与 reversed 比较），导致最后一步部分退款冲正被跳过（MultiPartial 测试暴露，已改为 **delta cap**）。

## 8. Idempotency（幂等）

| 场景 | 第一层 | DB 最终防线 |
|---|---|---|
| 重复 completed（含并发/重放） | 行锁后 `Status==completed` 短路 | Affiliate `(order_id,beneficiary,level,type)` 唯一 + Points `points_ledger.reference` 唯一索引 |
| ORDER_REWARD 同单重复 | reference 预查 | `UNIQUE(reference)`（`points_ledger`） |
| 退款冲正同 record 重放 | `target<=reversed` 短路 | `UNIQUE(reference)`（`points:order_refund:{record_id}`） |
| Admin 调整（P0） | reference 预查 | `UNIQUE(reference)` |

测试：TestP1_Reward_DuplicateComplete（10 goroutines 同单完成 → 1 条奖励）、TestP1_Refund_Concurrent Part B（同 reference 并发重放 → 恰 1 条）。

## 9. Concurrency（并发）

- `CompleteOrderInTx`：`GetByIDForUpdate` 行锁 → 状态校验 → 副作用同事务；
- 退款：`GetByIDForUpdate`（refund 既有）+ reference 唯一索引；
- 积分 mutation（P0）：`SELECT ... FOR UPDATE` 账户行锁 + `applyMutation` 单点；
- 测试基础设施：SQLite **shared-cache 内存库多连接并发写会触发读锁升级死锁**（busy_timeout 无效，已复现 607s 挂起）→ 并发测试改用**文件型 WAL**（`newPointsP1ConcurrentFixture`，writer 唯一 + busy_timeout 排队）；真并发/PostgreSQL 语义由 PG 集成测试承担（见 Known Issues 8.4）。

## 10. Database Invariants

所有 P1 测试末尾断言（`assertInvariant`）：

```text
points_accounts.balance == SUM(points_ledger.amount)   // 单一 user 维度
```

另：累计冲正 `SUM(ORDER_REWARD_REVERSAL)` ≤ `SUM(ORDER_REWARD)`；全额退款后 == 原奖励；`refunded_amount ≤ paidBase`。

## 11. Tests（证据）

### 11.1 P1 专项集成测试（`internal/modules/order/integrationtest/e2e/points_order_reward_test.go`，9/9 PASS）

| 测试 | 验证点 |
|---|---|
| TestP1_Reward_Snapshot | 快照：旧单 +100 / 新单 +200；下单未完成 0 奖励 |
| TestP1_Reward_Normal_NoEarly | created/processing=0，completed 恰 1 条 ORDER_REWARD |
| TestP1_Reward_DuplicateComplete | 10 goroutines 同单完成 → 余额只 +100、1 条流水、1 条佣金 |
| TestP1_ThreeCompletionPaths | Admin/Manual/Auto 三路径 × affiliate+points 各恰一次；总余额 300 |
| TestP1_Refund_Full | +100/-100，净 0，原始流水保留 |
| TestP1_Refund_Partial | 30% 退款 → floor(100×cumulative/paid) 精确 |
| TestP1_Refund_MultiPartial | 30%+30%+40% → 每步 floor 累计精确，最终恰 100 |
| TestP1_Refund_NegativeBalance | +100、消费 80、全额退款 → -80（允许负余额） |
| TestP1_Refund_Concurrent | Part A：10 并发退款 → 累计冲正 == floor(累计) 且每笔成功退款恰 1 条；Part B：同 reference 并发重放恰 1 条 |

### 11.2 回归证据

- `go build ./...` → exit 0
- `go vet ./internal/modules/{points,order,affiliate,fulfillment,catalog,procurement}/... ./internal/app/container/...` → exit 0
- 影响模块：points、affiliate、fulfillment、catalog/product、order（application / integrationtest / transport / e2e / refund / aftersale）→ 全绿
- `go test ./... -count=1` 全量 → **仅既有失败**（见下），P1 无新增失败

### 11.3 既有失败清单（全量回归重核，均与 P1 无关）

| 包 | 失败 | 归类 |
|---|---|---|
| internal/architecture | TestAffiliateApplicationOwnsUseCasesAndContracts（application 12>10 文件）、TestAffiliateModuleOwnsDomainAndPersistence（domain 7>6）、TestDatabaseBootstrapIsSeparatedFromPlatformConnection（migrations 16>14） | 既有：无 P1 新增文件（P1 曾引入的 FileBudgets/OrderAdmin/Procurement 守卫失败均已修复并回归 PASS） |
| internal/logger | TestNewReleaseWritesToConfiguredFile | 既有：文件锁（Windows） |
| internal/modules/supportticket/integrationtest | TestCloseVsReplyRace / TestClaimTicketRace（两次 run 交替失败） | 既有 flaky：SQLite 竞态测试，单跑 PASS；git diff 无 supportticket 改动 |
| internal/selfupdate | 8 个（Detect/Release/DirWritable/BinaryLock/ConcurrentRollback/…） | 既有：Windows 平台语义（Release 目录不存在） |

## 12. Changed Files（P1）

### 新增
- `internal/modules/points/application/order.go` — RewardOrderCompleted / ReverseOrderReward / OrderRewardSummary（幂等 + 冲正 append-only）
- `internal/modules/order/integrationtest/e2e/points_order_reward_test.go` — P1 专项集成测试（含文件 WAL 并发 fixture）

### 修改
- `internal/modules/points/contract/{ports.go,types.go}` — P1 用例端口、OrderReward/OrderReversal Input、reference 规则、Action 常量
- `internal/modules/points/infrastructure/gormstore/store.go` — `SumOrderRewards`（COALESCE）
- `internal/modules/order/contract/store.go` + `infrastructure/gormstore/transaction.go` — Transaction.Points()
- `internal/modules/order/domain/order.go` — RewardEnabled/RewardPoints 快照字段（P1 前置）
- `internal/modules/order/application/order_service.go` — PointsService 端口 + CreateOrder 快照
- `internal/modules/order/application/order_service_child.go` — 完成路径收口 + 统一 Lifecycle（CompleteOrderInTx/CompleteParentSideEffects/completeOrderSideEffectsInTx 等，原 order_completion.go 合并至此以满足文件预算守卫）
- `internal/modules/order/application/refund/{service.go,wallet.go}` — pointsSvc 端口、reverseOrderRewardInTx（floor 累计算法 + delta cap）、New 第 7 参
- `internal/modules/fulfillment/application/service.go` — OrderCompletionService 端口、CreateManual/CreateAuto 收口
- `internal/modules/procurement/contract/ports.go` — OrderLifecycle.CompleteOrder/CompleteParentSideEffects + OrderCompletion 端口
- `internal/modules/procurement/application/callback.go` — delivered 分支收口（adapter 与访问器移出以满足函数所有权守卫）
- `internal/modules/procurement/infrastructure/gormstore/lifecycle.go` — SetOrderCompletion/CompleteOrder/CompleteParentSideEffects
- `internal/modules/affiliate/application/commission.go` — HandleOrderCompletedInTx / handleOrderCompletedCore 重构
- `internal/modules/catalog/product/**` — reward 字段、校验、公开 API（domain / write/{service,create,update} / contract/errors / transport/http/{admin_handler,public_view} / presenter）
- `internal/app/container/{services_application.go,services_integration.go}` — PointsService/OrderCompletion 注入 + container 层 adapter
- 测试调用点：`internal/app/httpserver/order_refund_handler_test.go`、`internal/modules/order/integrationtest/{refund,e2e,aftersale}`、`transport/http/aftersale_handler_test.go`（refund.New 第 7 参 + mock 端口）

## 13. Known Issues

1. **子单退款不联动积分冲正**：`reverseOrderRewardInTx` 仅对父单执行（ORDER_REWARD 只发父单）。若未来出现"部分子单退款 + 父单未全额"的中间态，父单累计退款口径已包含子单退款金额（refunded_amount 聚合在父单），比例正确；无需改动。纯子单维度独立冲正不支持（当前业务无此需求）。
2. **SQLite 测试并发限制**：并发语义（重复完成/并发退款/并发积分 mutation）以文件 WAL 近似；PostgreSQL 真并发集成测试建议在 P2 前补充（P0 已有 `TEST_POSTGRES_DSN` + `-tags integration` 通道，points 包已有 PG 用例可参照）。
3. **Procurement 父单路径**：`CompleteParentSideEffects` 由 `SyncParentStatus` 返回 completed 时触发；若采购回调直接交付父单本身（无子单），`CompleteOrderInTx` 覆盖父单副作用 ✓。
4. **补偿/回算**：历史订单无快照不补发积分；未来如需 Backfill 需单独设计（本轮不做）。
5. **既有失败**：见 11.3（architecture 3 项 / logger 1 项 / supportticket flaky / selfupdate 8 项），均非 P1 引入。

## 14. P1 Exit Gate 核对

| 门槛 | 结果 |
|---|---|
| 商品固定奖励积分 | ✅ |
| Order Snapshot 正确 | ✅ TestP1_Reward_Snapshot |
| Parent Order only | ✅ 副作用绑定父单 |
| Completed only | ✅ TestP1_Reward_Normal_NoEarly |
| Unified completion lifecycle | ✅ 4 路径收口 |
| Admin/Manual/Auto 三路径 reward PASS | ✅ TestP1_ThreeCompletionPaths |
| Affiliate 三路径 PASS | ✅ 每路径 count==1 |
| Duplicate completed 不重复奖励 | ✅ 并发 10 → 1 条 |
| DB 幂等 PASS | ✅ reference 唯一索引 |
| Full refund reversal PASS | ✅ |
| Partial refund PASS | ✅ floor 公式 |
| Multi-partial rounding PASS | ✅ 30/30/40 精确 100 |
| Negative balance refund PASS | ✅ -80 |
| Concurrent refund PASS | ✅ 累计正确 + 同事件幂等 |
| Points invariant PASS | ✅ balance == SUM(ledger) |
| Fresh/Existing migration | ✅（无新增 migration；AutoMigrate 模型未变） |
| 无新增 regression | ✅（全量仅既有失败） |

## 15. Final

```
HCZ Points P1 Final
Verdict:               PASS
Git Baseline:          main @ 35496c4（改动未提交）
Changed Files:         §12（新增 2、修改 31）
Product Reward Schema: §2（reward_enabled / reward_points，fixed 模式）
Order Snapshot:        §3（父单 Σ / 子单单品，创建时固化）
Unified Lifecycle:     §4（CompleteOrderInTx / CompleteParentSideEffects / 4 路径）
Points Reward:         §5（ORDER_REWARD，reference=points:order_reward:{parent_order_id}）
Affiliate Regression:  §6（HandleOrderCompletedInTx，三路径各一次）
Refund Reversal:       §7（ORDER_REWARD_REVERSAL，append-only）
Partial Refund Formula:§7.1（target=floor(reward×cumulative/paid)；delta cap；精确全额）
Idempotency:           §8（reference 预查 + UNIQUE(reference) DB 兜底）
Concurrency:           §9（行锁 + WAL 测试；PG 通道就绪）
Database Invariants:   §10（balance == SUM(ledger)）
Tests:                 §11（9/9 P1 + 影响模块全绿 + 全量回归）
Regression:            §11.3（仅既有失败，无新增）
Known Issues:          §13
P2 Readiness:          READY FOR P2 CHECK-IN = YES
```
