# 基础设施与后端验证审计报告

> 审计范围：维度十二（数据库/Migration）、十七（后端全量验证）、十八（Linux CI）、十九（死代码/Legacy）、二十（Git/仓库收尾）。
> 环境：Windows + PowerShell 5.1，Go module `github.com/Aether-v1/hcz`（go 1.26.5），分支 `main`。
> 所有命令均真实执行，输出存于 `test_output.txt`。

## 审计摘要

- P0 发现：1（`internal/architecture` 架构守卫测试真实失败，CI 必红）
- P1 发现：1（gofmt 26 个文件未格式化，CI 硬门禁必失败）
- P2 发现：2（guest 传输层死代码、根目录 30 份 HCZ_*.md 报告堆积）
- go fmt：**FAIL**（26 个文件未格式化）
- go vet：**PASS**（0 warning / 0 error）
- go test：**FAIL**（189/368 包通过；4 包 FAIL，其中 1 包为真实失败、3 包为 Windows 环境 flaky）
- go build：**PASS**
- CI：**NOT EXECUTED**（本轮未 push 远程；仅静态分析配置）

---

## 十二、数据库 / Migration

- 结论：**PASS**
- AutoMigrate 入口：`internal/bootstrap/database/migrations/registry.go:43 AutoMigrate()`，由 `cmd/server/main.go:143` 调用（位于 `MarkMigrationStarted` 之后、版本回滚保护之内）。

### AutoMigrate model 清单（registry.go:45-98，共 53 个）

Admin、User、externalidentity.Identity、affiliate.Profile/Click/Commission/WithdrawRequest、wallet.Account/Transaction/RechargeOrder、auditlog.UserLoginLog/AuthzAuditLog/AdminLoginLog、notification.NotificationLog、emailverification.Code、**order.Order/OrderItem/OrderRefundRecord/AfterSaleTicket**、orderrisk.LockKey、cart.Item、payment.PaymentChannel/Payment、cardsecret.Secret/Batch、giftcard.GiftCard/GiftCardBatch、fulfillment.Fulfillment、coupon.Coupon/CouponUsage、promotion.Promotion、category.Category、product.Product/ProductSKU、content.Post/PostProduct/PostCategory/Banner/Media、settings.SettingRecord、apicredential.ApiCredential、siteconnection.Connection、mapping.Mapping/SKUMapping、procurement.Order、downstreamcallback.OrderRef、reconciliation.Job/Item、channelclient.Client、telegram broadcast.Broadcast、memberlevel.MemberLevel/MemberLevelPrice。
随后顺序执行：OAuth 唯一索引预检、risk_ip 回填、`resellerstore.Migrate`、cart SKU 唯一索引迁移、ProductSKU/manual_stock/category_parent、payment provider 重命名、payment fee policy、order refund fee、order item original price、cart/procurement 外键约束收敛、废弃列清理。

### 关键字段检查表

| 表/字段 | 是否存在 | 位置 |
|---|---|---|
| orders.exchange_rate | 是 | `internal/modules/order/domain/order.go:33`（decimal(20,8)） |
| orders.exchange_rate_source | 是 | `order.go:34`（varchar(16)，AUTO/MANUAL） |
| orders.exchange_rate_at | 是 | `order.go:35`（带 index） |
| orders.refund_status | 是 | `order.go:21`（not null default 'none'） |
| orders.after_sale_status | 是 | `order.go:22`（not null default 'none'） |
| after_sale_tickets 表 | 是 | `after_sale.go:19`，TableName()=after_sale_tickets，已注册进 registry.go:64 |
| exchangerate settings | 是 | `exchangerate/infrastructure/settingsstore/store.go`，KV key=`global_exchange_rate`，复用 settings.SettingRecord 表 |

### Fresh install 测试结果

复用项目既有空库测试，真实执行：

```
go test ./internal/bootstrap/database/migrations/ -run "TestAutoMigrateOwnsResellerSchemaAndCrossModuleConstraints|TestAutoMigrateReplacesSupersededProcurementConstraintName" -v
--- PASS: TestAutoMigrateOwnsResellerSchemaAndCrossModuleConstraints (0.20s)
--- PASS: TestAutoMigrateReplacesSupersededProcurementConstraintName (0.12s)
ok  github.com/Aether-v1/hcz/internal/bootstrap/database/migrations  0.387s
```

该测试在**全新 in-memory SQLite（`_pragma=foreign_keys(1)`）上连续跑两遍 `AutoMigrate()`**（空库 + 幂等），并断言 9 张 reseller 表、5 个索引、cart/procurement 外键约束全部建立、旧约束被替换。空库安装与升级幂等均通过。日志中的 `record not found` 为各一次性迁移用 settings key 做的「是否已执行」预检，属正常。

### 非破坏性确认

- AutoMigrate 本身只加列/加表/加索引，不删表、不删活跃列。
- 全生产迁移代码仅两处破坏性操作，且均有守卫、属一次性 schema 收敛：
  - `registry.go:144-148` `DropColumn(Product,"price_currency")`：先 `HasColumn` 守卫，清理已被新 schema 取代的废弃列。
  - `migrations.go:437-441` `DropIndex(cart.Item,"idx_cart_user_product")`：先 `HasIndex` 守卫，随后立即建新的 `idx_cart_user_product_sku`。
- 未发现任何 `DropTable` / 无守卫删除。

### 历史数据可读性确认

- `internal/modules/order/application/ordermachine/machine.go:16 Normalize()` 覆盖旧 9 态（pending_payment/paid→pending_recharge；fulfilling/partially_delivered→processing；delivered/completed→completed；partially_refunded→completed+partial；refunded→completed+full；canceled/failed→终态）。
- 旧状态常量仍保留在 `constants/constants.go:5-13`，仅供 Normalize 映射与测试使用，**不批量改写历史数据**，符合预期。

---

## 十七、后端全量验证

### go fmt 结果

**FAIL** — `gofmt -l .\internal\ .\cmd\` 输出 26 个未格式化文件：

```
internal\app\container\container.go
internal\app\container\services_integration.go
internal\app\httpserver\routes_admin.go
internal\app\httpserver\routes_storefront.go
internal\constants\constants.go
internal\modules\exchangerate\application\service.go
internal\modules\exchangerate\infrastructure\rediscache\cached_store.go
internal\modules\exchangerate\infrastructure\rediscache\cached_store_test.go
internal\modules\exchangerate\infrastructure\settingsstore\store.go
internal\modules\exchangerate\transport\admin_handler.go
internal\modules\order\application\aftersale\adapter.go
internal\modules\order\application\aftersale\service_test.go
internal\modules\order\application\order_refund_status_consistency_test.go
internal\modules\order\application\order_service.go
internal\modules\order\application\order_service_child.go
internal\modules\order\application\ordermachine\machine.go
internal\modules\order\application\ordermachine\machine_test.go
internal\modules\order\domain\after_sale.go
internal\modules\order\domain\order.go
internal\modules\order\integrationtest\aftersale\service_test.go
internal\modules\order\transport\http\aftersale_handler.go
internal\modules\order\transport\http\aftersale_handler_test.go
internal\modules\order\transport\http\create_handler.go
internal\modules\order\transport\presenter\order.go
internal\modules\payment\application\payment_service_create.go
internal\modules\wallet\transport\presenter\wallet.go
```

### go vet 结果

**PASS** — `go vet ./...` 退出码 0，无任何 warning/error 输出。

### go test ./... 结果

命令：`go test ./... 2>&1 | Tee-Object test_output.txt`（退出码 1）

- 总包数：**368**
- PASS（含测试且通过）：**189**
- FAIL：**4**
- 无测试文件（不计为 PASS/FAIL）：**175**
- SKIP：0（无显式 skip 包）

#### FAIL 包详情

1. **`internal/architecture`（真实失败，Linux CI 同样会红）**
   - `TestDependencyRules`（dependencies_test.go:51）：
     - `exchangerate/transport/admin_handler.go` import `github.com/gin-gonic/gin`：HTTP 传输层不应落在 domain module 内。
     - `order/transport/http/aftersale_handler_test.go` 直接 import 6 个具体 `infrastructure/gormstore`（affiliate / identity user / order / payment / settings / wallet）：transport 应依赖 application contract，而非具体 store。
   - `TestOrderAdminHTTPLivesInTransport`（order_handler_structure_test.go）：文件数超预算
     - `order/domain`：预算 5，实际 6（多出 `after_sale.go` / `refund_fee_test.go`）
     - `order/infrastructure/gormstore`：预算 8，实际 9（多出 `aftersale_store.go`）
     - `order/transport/http`：预算 7，实际 9（多出 `aftersale_handler.go` + 其 test）
   - 根因：P1 after-sale 特性新增了文件，但架构守卫的文件预算与依赖白名单未同步更新。

2. **`internal/logger`（Windows 环境 flaky，本次新观测）**
   - `TestNewReleaseWritesToConfiguredFile`：断言本身通过（release.log 已写入），仅 `t.TempDir()` 的 `RemoveAll` 清理失败——`unlinkat ... release.log: The process cannot access the file because it is being used by another process`（lumberjack 句柄未释放）。与 RiskGate 同类 Windows 文件锁问题，但**不在预已知 flaky 清单内**，单列标注。

3. **`internal/modules/order/infrastructure/gormstore`（已知 Windows flaky）**
   - `TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts`、`TestRiskGateSerializesConcurrentGuestQuotaChecks`：均为 `TempDir RemoveAll ... risk-gate.db: being used by another process`。断言通过，仅 SQLite 文件锁导致 cleanup 失败——与 agent-hint 标注的 RiskGate TempDir flaky 完全一致。

4. **`internal/selfupdate`（已知 Windows flaky）**
   - 9 个测试失败，全部为 Unix 语义在 Windows 不成立：
     - `TestDetectBlocksSourceBuild` / `TestDetectReleaseBuild`：`unsupported_os`（Windows 不允许自更新）。
     - `TestDirWritable`：`0500 dir should not be writable`（Windows 不识别 Unix 权限位）。
     - `TestBinaryLockIsExclusive` / `TestConcurrentRollbacksDoNotBothSucceed` / `TestManagerStartHoldsBinaryLockDuringReleaseFetch` / `TestStartupGuardReadOnlyStateFailsClosedBeforeMigration` / `TestStartupGuardBlocksConcurrentRollback` / `TestExtractBinaryRejectsTruncatedEntry`：文件排他锁 / 只读状态语义仅在 Unix 成立。

#### 已知 Windows flaky（环境问题，不计入真实失败）
- order/infrastructure/gormstore RiskGate 2 例（TempDir SQLite 文件锁）。
- selfupdate 9 例（Unix permission / binary lock）。
- logger 1 例（TempDir 日志文件句柄）——本次新观测，同类文件锁，非业务逻辑失败。

#### 真实失败（非环境问题）
- **`internal/architecture` 全部 2 个测试**——在 Linux CI 上必然失败，必须修复（要么调整 after-sale 相关文件位置以符合分层预算，要么同步更新架构守卫的预算/白名单）。

### go build ./... 结果

**PASS** — `go build ./...` 退出码 0，无编译错误。

---

## 十八、Linux CI

- 状态：**CI NOT EXECUTED**（本轮未 push 远程，未触发任何 workflow；以下为静态分析）。

### ci.yml 静态分析

4 个 job，配置完整：

| Job | 步骤 | 评价 |
|---|---|---|
| **installer** | checkout → apt 装 python3-yaml/shellcheck → `bash -n` 语法检查 + shellcheck → 跑 `scripts/tests/hcz-manager_test.sh` → python 校验 `.goreleaser.yaml` archives 含 `./scripts/hcz-manager.sh` | Verify installer：**有**，且为独立 job。路径与脚本真实存在（`scripts/hcz-manager.sh`、`scripts/tests/hcz-manager_test.sh` 均在库中）。 |
| **api** | setup-go(go.mod) → `go mod download` → **gofmt 门禁**（`gofmt -l $(git ls-files '*.go')`，非空即 exit 1）→ `go vet ./...` → `go test ./...` → `go build ./cmd/server` | Verify API：**有**。注意：gofmt 门禁会被本次 26 个未格式化文件直接打红；`go test ./...` 会被 `internal/architecture` 真实失败打红。 |
| **release-config** | goreleaser-action `check` | 提前校验 `.goreleaser.yaml`，配置合理。 |
| **fullstack** | pnpm 10.34.3 + node 24.11.1 → `pnpm install --frozen-lockfile`（admin/user 双前端）→ 双前端单测 → `pnpm run build` → 拷贝 dist 到 `internal/web/dist/{admin,user}` → `go build -tags release,fullstack ./cmd/server` | Verify fullstack build：**有**，覆盖前端构建 + 嵌入 + fullstack 标签编译链路。 |

- 配置问题：未发现无效路径/缺失依赖。`gofmt -l $(git ls-files '*.go')` 用 git ls-files 限定范围、规避 node_modules，写法正确。
- **静态结论：当前 worktree 一旦 push，api job 会因 gofmt + architecture 测试双红失败。**

### release.yml 静态分析

- 触发条件：push tag `v*`，`permissions: contents: write`。
- 步骤：checkout(fetch-depth:0) → pnpm + node + go 工具链 → `goreleaser release --clean`，`GITHUB_TOKEN: secrets.GITHUB_TOKEN`。
- 评价：发布配置正确；前端构建/嵌入交由 `.goreleaser.yaml` 的 before hooks 完成（与 ci.yml 注释一致）。未发现配置错误。

---

## 十九、死代码 / Legacy 清理审计

- 结论：**PARTIAL（存在一处有意停用的 guest 传输层死代码，无阻断性代码；旧 legacy 已清理）**

| 发现 | 文件:位置 | 分类 | 说明 |
|---|---|---|---|
| guest HTTP 传输处理器与路由注册函数 | `order/transport/http/guest_handler.go`；`order/transport/http/routes.go` 的 `RegisterGuestReadRoutes/Preview/Create/CreateAndPay` | **RETIRE_SAFE（当前不可达，但被测试钉住）** | `routes_storefront.go:92` 注释明确「HCZ 不支持游客购买：原 /guest/* 下单/查单/支付/下载路由组停用，不再注册」。生产装配中无任何 `RegisterGuest*` 调用（仅 `route_structure_test.go:192-197` 断言其存在）。属产品决策性停用，非漏接。 |
| guest 领域/存储逻辑（UserID=0 订单、guest 密码哈希、guest 风控配额） | `order/infrastructure/gormstore/guest_credential*`、`order_service`、`risk_gate_test` | **KEEP** | 领域层仍被 order_service / risk_gate 引用，历史 guest 订单仍可读，不能随传输层一起删。 |
| 旧 9 态状态常量与 Normalize 映射 | `constants/constants.go:5-13`；`ordermachine/machine.go:16` | **KEEP** | 历史订单 status 字段读取依赖 Normalize 归一，迁移后才可能退役 → 严格说为 RETIRE_AFTER_MIGRATION，但当前必须保留。 |
| 旧 legacy HTTP handler 目录 `internal/http/handlers/{admin,public}` | — | **已删除** | `Test-Path internal\http\handlers` = False；架构测试 order_handler_structure_test.go:93-100 强制其保持删除。 |
| `scripts/hcz-manager.sh` + 其 shell 测试 | `scripts/hcz-manager.sh`、`scripts/tests/hcz-manager_test.sh` | **KEEP** | 活跃安装器，被 CI installer job 与 shellcheck 验证，且被 `.goreleaser.yaml` 打入发布归档。 |
| 旧支付订单路径 / 旧退款 helper | 未发现独立死路径 | **KEEP** | 退款逻辑已统一到 `order_refund_record` + wallet 退款链；go vet 无 unused 告警。 |

### BLOCKER 清单
- 无。无活跃的阻断性死代码。guest 传输层虽不可达但被架构/路由测试显式钉住，删除会先破坏测试，需与产品确认是否永久放弃游客购买后再退役。

---

## 二十、Git / Repository Closure

- git status：**dirty（仅 2 个未跟踪文件，无已跟踪修改）**，分支 `main`，跟踪文件 1802 个。
- untracked files（`git ls-files --others --exclude-standard`）：
  - `AUDIT_BACKEND_BUSINESS.md`（上一轮审计产物，遗留）
  - `test_output.txt`（本轮 go test 输出，审计临时产物）

### .gitignore 检查
- 已忽略：`.vscode/`、`config.yml`、`db/`、`uploads/`、`*.db`、`*.log`、`.env`、`node_modules/`、二进制（`*.exe/*.dll/*.so/*.dylib`、`/hcz` 等）、`*.test`、coverage（coverage.txt/html/xml、*.out、*.coverage）、`logs/`、`dist/`、`frontend/*/dist/`、`frontend/*/node_modules/`、`internal/web/dist/`、`runtime/`、`*.tmp`、`*.cache`、`.agents/`、`skills-lock.json`。
- 缺失项：**未发现明显应忽略而未忽略的类型**。
  - 唯一建议：`test_output.txt` 这类审计/测试日志未被显式覆盖（`*.log` 不匹配 `.txt`），可后续加入，但不影响收尾。

### 不应提交的文件检查

| 类型 | 是否存在 | 状态 |
|---|---|---|
| 已跟踪 .exe / 二进制 | 否 | PASS |
| 已跟踪 .db / .sqlite | 否（worktree 内亦无游离 db，排除 node_modules/runtime 后计数 0） | PASS |
| 已跟踪 dist/（含 internal/web/dist/） | 否 | PASS |
| 已跟踪 node_modules | 否 | PASS |
| 已跟踪 coverage.out / coverage.txt | 否（唯一含 coverage 字样的是 `internal/app/httpserver/rbac_coverage_test.go`，为合法测试文件） | PASS |
| 已跟踪 runtime/（logs/temp/test-results/coverage） | 否（git ls-files runtime/ = 0，且在 .gitignore） | PASS |

### docs 整理情况
- `docs/` 目录结构合理（api/architecture/archive/audit/audits/decisions/deployment/issues/reports/runbooks），但大量子目录仅 `.gitkeep` 占位。
- **根目录堆积 30 份 `HCZ_*.md` 阶段性收尾报告**（如 HCZ_P0_*.md、HCZ_P1_*.md），属过程性产物，建议归档进 `docs/archive/` 以保持根目录整洁（P2，非阻断）。

---

## 总结

### P0 修复清单
1. **`internal/architecture` 测试真实失败**：P1 after-sale 特性新增文件导致
   - `order/domain`(6>5)、`order/infrastructure/gormstore`(9>8)、`order/transport/http`(9>7) 文件预算超限；
   - `exchangerate/transport/admin_handler.go` 直接 import gin（违反「HTTP 传输不落在 domain module」）；
   - `aftersale_handler_test.go` 直接 import 6 个具体 gormstore（应改用 application contract/mock）。
   → 不修复则 Linux CI api job 必然失败。

### P1 修复清单
2. **gofmt 26 个文件未格式化**：执行 `gofmt -w` 这 26 个文件即可，当前会让 CI gofmt 硬门禁直接 exit 1。

### P2 backlog
3. guest 传输层（guest_handler.go + RegisterGuest*）为产品决策性停用的死代码，被测试钉住；待确认永久放弃游客购买后，连同 guest 领域逻辑与 Normalize 旧 9 态映射一并 RETIRE。
4. 根目录 30 份 `HCZ_*.md` 过程报告建议归档至 `docs/archive/`；`test_output.txt` 等审计日志可考虑加入 .gitignore。

### 通过项（无需处理）
- Migration 空库安装 + 幂等：PASS；关键字段齐全；非破坏性；历史 9 态可读。
- go vet：PASS；go build：PASS。
- CI/release 配置静态正确（installer / API / release-config / fullstack 四 job 齐全，release.yml 正确）。
- 仓库收尾干净：无二进制/db/dist/node_modules/coverage/runtime 被跟踪，.gitignore 完备。
