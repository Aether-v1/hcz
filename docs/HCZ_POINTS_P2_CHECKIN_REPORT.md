# HCZ Points P2 — Daily Check-in Implementation Report

- **Verdict**: PASS
- **阶段**: Points System P2 Daily Check-in（每日签到）
- **前置**: Points P0 (PASS) → Points P1 Order Reward (PASS)
- **审计依据**: `docs/audits/HCZ_POINTS_BACKEND_AUDIT.md`、`docs/HCZ_POINTS_P0_IMPLEMENTATION_REPORT.md`、`docs/HCZ_POINTS_P1_ORDER_REWARD_REPORT.md`
- **时间**: 2026-10-07（Asia/Shanghai）

---

## 1. Git Baseline

- Branch: `main`
- HEAD: `35496c40f226d770a1c8d910cf7a4364d24e181a`（`feat(user): 浅色背景体系修复与视觉优化`）
- 工作区存在大量既有未提交改动（含 P0/P1/P2 全部实现），本轮未提交、未触碰用户既有文件。
- 本轮禁止事项确认：未实现补签、签到卡、VIP/活动倍率、Task/Mission、积分商城、积分兑换、积分过期。

## 2. Changed Files

### 新建（checkin 模块，独立 Domain）
| 文件 | 作用 |
|---|---|
| `internal/modules/checkin/domain/checkin.go` | `UserCheckin` 实体，表 `user_checkins`，`UNIQUE(user_id, checkin_date)`（priority1+2） |
| `internal/modules/checkin/contract/ports.go` | 端口与错误：`ErrCheckinDisabled/ErrInvalidMonth/ErrUserRequired`；`Config/ConfigReader/Clock/Repository/Transaction/UnitOfWork/CheckinResult/StatusResult/HistoryResult` |
| `internal/modules/checkin/application/clock.go` | Asia/Shanghai 业务时钟：`BusinessDate`/`FormatBusinessDate`/`SystemClock`/`FixedClock` |
| `internal/modules/checkin/application/service.go` | `CheckIn/Status/History` 用例；`RewardForCycleDay` 唯一 resolver；断签/未来日期防御 |
| `internal/modules/checkin/infrastructure/gormstore/store.go` | GORM 仓储（业务日序列化约定见 §4） |
| `internal/modules/checkin/transport/http/user_handler.go` | 用户 API：`GET /checkin/status`、`POST /checkin`、`GET /checkin/history` |
| `internal/modules/checkin/transport/http/user_handler_test.go` | HTTP 测试（含 IDOR 防御、重复签到非错误、禁用、历史月份） |
| `internal/modules/checkin/integrationtest/checkin_test.go` | 15 个 SQLite 集成用例（含 fixture/不变式/20 并发串行） |
| `internal/modules/checkin/integrationtest/pg_checkin_test.go` | PostgreSQL 真并发测试（`//go:build integration`） |

### 新建（settings 集成）
| 文件 | 作用 |
|---|---|
| `internal/modules/settings/schema/points/checkin.go` | `CheckinConfig{Enabled, Rewards:[7]int64}`；默认 `{true,[1,2,3,4,5,6,10]}`；Normalize/Validate/Decode/Encode |
| `internal/modules/settings/schema/points/errors.go` | `ErrCheckinConfigInvalidRewards` |
| `internal/modules/settings/schema/points/checkin_test.go` | 配置 schema 单测 |
| `internal/modules/settings/transport/http/checkin_handler.go` | Admin `GET/PUT /admin/settings/checkin` |
| `internal/modules/settings/transport/http/checkin_handler_test.go` | Admin handler 测试（非法配置拒绝） |
| `internal/bootstrap/checkin/handlers.go` | bootstrap 装配（`New(c *container.Container)`） |

### 修改
| 文件 | 作用 |
|---|---|
| `internal/constants/constants.go` | `SettingKeyCheckinConfig = "checkin_config"` |
| `internal/i18n/messages.go` | 5 key × 3 语：`error.checkin_disabled/invalid_month/fetch_failed/failed/config_invalid_rewards` |
| `internal/modules/settings/application/default_registry.go` | registry 注册 `checkin_config`（Normalize 兜底） |
| `internal/modules/settings/application/core.go` | `GetCheckinSetting`（宽松读+损坏回退默认+warning）、`UpdateCheckinSetting`（严格校验写） |
| `internal/modules/settings/application/default_registry_test.go` | registry keys 期望加入 `checkin_config` |
| `internal/modules/settings/transport/http/routes.go` | `RegisterAdminCheckinRoutes` |
| `internal/modules/points/contract/{ports.go,types.go}` | `CheckinReward` 用例 + `CheckinRewardInput`（含 `CheckinReference` 维度） |
| `internal/modules/points/application/checkin.go` | 双层幂等：reference 预查 + applyMutation 撞唯一索引静默跳过；`Action=CHECKIN_REWARD`、`Source=checkin` |
| `internal/app/container/{container.go,repositories.go,services_application.go}` | 容器装配：`CheckinRepo/CheckinService` + `checkinSettingsAdapter` |
| `internal/app/httpserver/{router.go,routes_storefront.go,routes_admin.go}` | 路由注册（user group + admin settings group） |
| `internal/authz/bootstrap.go` | builtin admin policy：`/admin/settings/checkin` `*` |
| `internal/bootstrap/database/migrations/registry.go` | AutoMigrate 加入 `&checkindomain.UserCheckin{}`（points 之后） |

> 注：`internal/modules/points/` 整体为 P0 创建、当前工作区未提交；本轮 points 侧只新增/修改了上述 3 个文件。

## 3. Check-in Schema

表 `user_checkins`：

| 字段 | 类型 | 说明 |
|---|---|---|
| id | BIGSERIAL PK | |
| user_id | BIGINT NOT NULL | 唯一索引 (user_id, checkin_date) 联合 |
| checkin_date | DATE NOT NULL | 业务日（Asia/Shanghai 日期，UTC 零点存储），`type:date` + `uniqueIndex:idx_user_checkin_date` |
| consecutive_days | INT NOT NULL | 连续签到总天数（断签重算；第 8 天继续增长，不重置） |
| cycle_day | INT NOT NULL | `((consecutive-1)%7)+1`，7 天循环档位 |
| points_awarded | BIGINT NOT NULL | 当日奖励快照（配置变更不重算历史） |
| points_ledger_id | BIGINT NOT NULL DEFAULT 0 | 关联 points_ledger 流水（零积分日 = 0） |
| created_at | TIMESTAMPTZ NOT NULL | |

- 最终防线：`UNIQUE(user_id, checkin_date)`（数据库层，非应用层 if）。
- 事实来源：签到记录表本身；不维护 `last_checkin_at/checkin_days` 脆缓存字段。

## 4. Business Timezone 与业务日

- 统一业务时区：`time.LoadLocation("Asia/Shanghai")`（tzdata 缺失时 FixedZone 仅作启动兜底，非长期实现；正常路径恒为 Location）。
- `BusinessDate(now)` = now 在上海时区的日期取 UTC 零点（`time.Date(y,m,d,0,0,0,0,time.UTC)`），作为存储/查询键。
- **存储序列化约定**（store.go 注释已文档化）：`checkin_date` 列查询/写入一律直接传 `time.Time`，由驱动序列化——SQLite 存 RFC3339 `2026-10-01T00:00:00Z`；PostgreSQL `date` 列隐式截断为日期。**禁止手工拼接 `YYYY-MM-DD` 字符串**（曾实测与驱动存储值不匹配）。
- 时区边界测试：UTC 15:59 ↔ 上海 23:59 = 10-01；UTC 16:01 ↔ 上海 00:01 = 10-02，两个不同签到日 ✓。

## 5. Reward Rule

- 首版固定 7 天循环：`[1,2,3,4,5,6,10]`，Day 8 起 cycle 回 1 但 streak 继续增长。
- 唯一 resolver：`RewardForCycleDay(rewards, cycleDay)`（禁止规则散落 Handler/Repo/DTO）。
- `cycleDay = ((consecutiveDays-1)%7)+1`。
- 零积分日（配置允许 0）：签到成功、streak 正常、**不插入 0 amount Points Ledger**、`points_ledger_id=0`。
- 配置损坏（长度≠7/负数/超上限 `1_000_000`）：读取路径回退默认 `[1,2,3,4,5,6,10]` + `logger.Warnw`；Admin 保存路径严格拒绝（`ErrCheckinConfigInvalidRewards`）。
- 历史快照：`points_awarded` 固化在签到记录，配置变更不重算历史（测试 `TestCheckin_RewardConfigChange` 验证）。

## 6. Streak Algorithm

- 签到时查询 `LatestBefore(user_id, today)`（`checkin_date < today` 最近一条）：
  - 最近日期 = 昨天 → `consecutive = prev.ConsecutiveDays + 1`
  - 否则 → `consecutive = 1`（断签重算）
- 未来日期异常数据：防御处理（`logger.Warnw` 留痕，不 panic、不污染 streak）。
- 月跨界/年跨界连续（9/30→10/1、12/31→1/1 = 2）已测；History 按月展示与 streak 是独立概念。
- 补签：首版不支持（用户确认选择，评价合理——V1 用户量下维护简单、规则可审计；如需补签属 V1.1 能力）。

## 7. Points Integration

- 签到 + 积分同事务（`UnitOfWork.WithinTransaction`）：
  `确定业务日 → 查当日（幂等）→ 计算连签 → Create checkin → Points.CheckinReward(tx, ...) → 回填 ledger_id → COMMIT`；任一失败 ROLLBACK。
- 经 Points Core 权威 mutation（`pointsapp.Service.CheckinReward`），禁止签到模块直写账户/Ledger。
- Ledger：`Action=CHECKIN_REWARD`、`Source=checkin`、`SourceID=checkinID`、`Amount>0`、`checkinDate` 透传。
- 幂等 reference：`points:checkin:{user_id}:{YYYY-MM-DD}`（日期为 Asia/Shanghai 业务日）。
- 双层幂等：reference 预查（`GetLedgerEntryByReference`）+ `applyMutation` 撞唯一索引 `isDuplicateKeyError` 静默跳过（数据库最终防线，非应用层 if）。
- 并发双击：撞 `UNIQUE(user_id,checkin_date)` → 事务回滚 → **事务外重读**权威记录（避免事务内快照读不到未提交数据）→ 返回 `AlreadyCheckedIn=true`（HTTP 200 业务幂等，非 409/500）。
- 零积分日不触发 Points 调用（无 0 mutation）。

## 8. API（用户 + Admin）

用户（`/api/v1` user group，user_id 仅取自 JWT ctx，禁客户端传入）：

| 方法 | 路径 | 语义 |
|---|---|---|
| GET | `/api/v1/checkin/status` | `{enabled, checked_in_today, consecutive_days, cycle_day, today_reward, next_reward}`（未签到：consecutive=截至昨天，today_reward=今天将得） |
| POST | `/api/v1/checkin` | `{checkin_date, points_awarded, consecutive_days, cycle_day, current_balance, already_checked_in}`；重复签到 200 + already_checked_in=true |
| GET | `/api/v1/checkin/history?month=YYYY-MM` | `{year, month, checked_dates[], entries[], total}` 日历结构；严格 `^\d{4}-(0[1-9]|1[0-2])$`，空=当前上海月；仅业务数据（无 CSS/文案） |

Admin（`/admin/settings` group，Bearer + RBAC）：

| 方法 | 路径 | 语义 |
|---|---|---|
| GET | `/admin/settings/checkin` | 返回 `{enabled, rewards:[7]}` |
| PUT | `/admin/settings/checkin` | 更新 `enabled` + `rewards[7]`；校验：长度=7、每元素 ≥0 且 ≤1,000,000；非法拒绝入库 |

响应惯例沿用 HCZ：HTTP 恒 200 + `{status_code, msg(本地化), data}`；业务错误 `status_code=400`；未登录 401（既有中间件）。

## 9. Admin Settings & RBAC

- 复用现有 settings 体系（typed_io Get/Update + Registry Normalize + 声明副作用），未新建第二套设置系统。
- 配置 key：`checkin_config`（`constants.SettingKeyCheckinConfig`）。
- RBAC：builtin admin policy 登记 `{Object: "/admin/settings/checkin", Action: "*"}`，沿用 `/admin/settings/*` 全量权限模型；**未使用万能 admin 绕过作为正式实现**。readonly auditor 只读、无修改权限（未改变既有模型）。
- Admin 操作经既有 settings Update 副作用/审计链路。

## 10. Concurrency

- 数据库唯一索引是最终防线；应用层幂等（事务内当日查询 + 撞索引静默处理 + 事务外重读）。
- SQLite 集成：`TestCheckin_Concurrent` 20 goroutine（单连接串行，与 points P0 先例一致）→ 恰 1 checkin + 1 CHECKIN_REWARD ledger + balance 恰好 +1 + 19 个 already=true。
- **PostgreSQL 真并发**：`TestPGCheckinConcurrentSingleRecord`（`-tags integration`，本机 PG 实测 PASS）→ 20 goroutine 并发同天签到：恰 1 checkin、1 CHECKIN_REWARD ledger、balance 恰好 +1、`assertCheckinInvariant` 通过。
- 事务模型：签到与积分同事务；任一失败回滚；不允许"签到成积分败"或"积分发签到败"。

## 11. Tests（证据）

- checkin 集成（SQLite，15 用例全 PASS）：FirstDay / SevenDayCycle(累计31) / Day8(streak8,cycle1,reward1) / BreakStreak / DuplicateSameDay / Concurrent(20) / MonthBoundary / YearBoundary / TimezoneBoundary / Disabled / RewardConfigChange / ZeroRewardDay / Status / History。
- PG 集成：TestPGCheckinConcurrentSingleRecord PASS。
- HTTP：status/check-in/duplicate-not-error/disabled/history-month/401/IDOR(query 注入 user_id 无效) 全 PASS。
- settings：schema/application/transport（含 default_registry keys 期望、非法配置拒绝）全 PASS。
- Points 不变量：`assertCheckinInvariant(balance == SUM(ledger))` 贯穿全部用例。

## 12. Regression

- `go build ./...` exit 0。
- `go vet`（checkin/points/settings/app/authz/bootstrap/constants/i18n）exit 0。
- 影响模块：checkin / settings / points（SQLite+PG）/ order(application,domain,e2e,refund,aftersale,ordermachine) / app / httpserver / authz / migrations 全 PASS。
- 全量 `go test ./... -count=1` 失败项与 P1 基线逐项对照：
  - `internal/architecture`：P2 新触发的 `TestGoPackagesStayWithinFileBudgets`（settings/application total 21>20）**已修复**（按 P1 先例将 checkin.go 合并入 core.go，净增 0 文件）；剩余 3 个既有失败（affiliate/application 12>10、affiliate/domain 7>6、migrations 16>14）与 P1 基线一致，未受本轮影响。
  - `internal/logger` `TestNewReleaseWritesToConfiguredFile`：Windows 文件锁既有问题（P1 基线）。
  - `internal/selfupdate`（9 项）：Windows 平台语义既有问题（P1 基线）。
  - `internal/modules/supportticket/integrationtest` `TestCloseVsReplyRace`：既有 flaky，单跑 PASS（P1 基线口径）。

## 13. Known Issues / Limitations

- **settings/application 文件预算守卫曾被本轮触发**：新增 checkin.go 后 total 21>20，已合并进 core.go 消除（P2 无净增文件）。后续 Phase 若再往该包加文件需先确认预算。
- SQLite 无真正 DATE 类型：`checkin_date` 存 RFC3339；查询统一直接传 time.Time（驱动序列化），PG `date` 列行为一致。已在 store.go 注释固化约定，后续 Phase 禁止改为字符串拼接。
- 并发签到在 SQLite 侧用单连接串行验证（SQLite 单写者限制），真并发以 PG 集成测试为准。
- 未实现：补签、签到卡、活动/VIP 倍率、连签大奖、Task/Mission、商城兑换、过期（均为后续 Phase）。
- PG 集成测试依赖本机 `127.0.0.1:5432` `hcz_test`（DSN 见 pg_checkin_test.go 顶部），CI/其他环境需提供 `TEST_POSTGRES_DSN`。

## 14. P3 Readiness

- Points Core（P0）、Order Reward/Refund（P1）无回归破坏；签到独立 Domain、Asia/Shanghai 业务时钟、7 天循环规则、DB 唯一约束、同事务 Points 发放均已就绪。
- **READY FOR P3 POINTS MALL = YES**
- 完成 P2 后停止，未自动进入 P3。
