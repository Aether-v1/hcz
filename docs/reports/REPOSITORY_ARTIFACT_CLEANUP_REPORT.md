# Repository Development Artifacts Cleanup Report

**日期**: 2026-10-03
**范围**: HCZ 仓库根目录及开发产物治理
**原则**: Repository Hygiene — 不修改业务逻辑，仅整理文件结构与引用

---

## 1. Before（整理前）

### 根目录文件（12 个）

| 文件 | 类型 | 决策 |
| --- | --- | --- |
| `.dockerignore` | 核心配置 | KEEP_ROOT |
| `.gitignore` | 核心配置 | KEEP_ROOT（需更新） |
| `.goreleaser.yaml` | 发布配置 | KEEP_ROOT |
| `config.yml.example` | 配置示例 | KEEP_ROOT |
| `Dockerfile` | 构建配置 | KEEP_ROOT |
| `go.mod` | 依赖管理 | KEEP_ROOT |
| `go.sum` | 依赖校验 | KEEP_ROOT |
| `HCZ_FIT_GAP_AUDIT.md` | 审计报告 | MOVE → `docs/audits/backend/` |
| `HCZ_GLOBAL_CURRENCY_AUDIT.md` | 审计报告 | MOVE → `docs/audits/backend/` |
| `HCZ_PAYMENT_BOUNDARY_AUDIT.md` | 审计报告 | MOVE → `docs/audits/backend/` |
| `LICENSE` | 许可证 | KEEP_ROOT |
| `README.md` | 项目说明 | KEEP_ROOT（需更新） |

### 根目录（7 个）

`.git` / `.github` / `assets` / `cmd` / `frontend` / `internal` / `scripts`

### 散落统计

| 类别 | 数量 |
| --- | --- |
| 散落 Audit 文件 | 3 |
| 散落 Report 文件 | 0 |
| 散落 Script 文件 | 0（`scripts/` 已有组织） |
| Log / Test Output / Coverage | 0 |
| 临时 / debug 文件 | 0 |

### 现有 scripts/ 结构（已合规，保留）

```
scripts/
├── hcz-manager.sh          # 安装/运维管理脚本（被 CI + goreleaser 引用）
└── tests/
    └── hcz-manager_test.sh # 安装脚本测试（被 CI 引用）
```

---

## 2. After（整理后）

### 根目录文件（9 个）

全部为项目正式文件，无散落文档：

`.dockerignore` / `.gitignore` / `.goreleaser.yaml` / `config.yml.example` / `Dockerfile` / `go.mod` / `go.sum` / `LICENSE` / `README.md`

### 根目录（9 个）

`.git` / `.github` / `assets` / `cmd` / `docs` / `frontend` / `internal` / `runtime` / `scripts`

### 新建目录结构

```
docs/
├── README.md                 # 文档索引（新建）
├── architecture/             # 架构设计
├── api/                      # API 契约
├── deployment/               # 部署文档
├── runbooks/                 # SOP / 执行教程
├── audits/
│   ├── backend/              # ← 3 份审计报告移入
│   ├── security/
│   ├── database/
│   └── legacy/
├── reports/
│   ├── tests/
│   ├── regression/
│   └── phases/               # ← 本报告所在
├── issues/                   # RCA / 问题调查
├── decisions/                # ADR
└── archive/                  # 历史文档

runtime/
├── logs/                     # gitignore
├── test-results/             # gitignore
├── coverage/                 # gitignore
└── temp/                     # gitignore
```

### 统计

| 指标 | Before | After |
| --- | --- | --- |
| 根目录文件数 | 12 | 9 |
| 根目录散落 Audit | 3 | 0 |
| docs/ 有效文件 | 0 | 4（含索引） |
| scripts/ 文件 | 2 | 2（未改动） |
| runtime/ 受 gitignore 保护 | 无 | 全目录忽略 |

---

## 3. Files Moved

| 原路径 | 目标路径 | 分类 | 引用检查 |
| --- | --- | --- | --- |
| `HCZ_PAYMENT_BOUNDARY_AUDIT.md` | `docs/audits/backend/HCZ_PAYMENT_BOUNDARY_AUDIT.md` | Backend Audit | 无外部引用，安全移动 |
| `HCZ_FIT_GAP_AUDIT.md` | `docs/audits/backend/HCZ_FIT_GAP_AUDIT.md` | Backend Audit | 无外部引用，安全移动 |
| `HCZ_GLOBAL_CURRENCY_AUDIT.md` | `docs/audits/backend/HCZ_GLOBAL_CURRENCY_AUDIT.md` | Backend Audit | 无外部引用，安全移动 |

**移动方式**: 文件系统移动（3 个文件均为 untracked，Git 视为新位置新增）。

---

## 4. Files Deleted

无。本轮遵循「优先 MOVE，不优先 DELETE」原则，未删除任何文件。

---

## 5. Files Archived

无。当前无被新报告取代的历史文档。

---

## 6. References Updated

| 文件 | 变更 | 原因 |
| --- | --- | --- |
| `.gitignore` | 追加 `runtime/`、`coverage.xml`、`junit.xml`、`*.tmp`、`*.cache`、`*.coverage`、`*.out` | 阻止生成文件再次污染 Git |
| `README.md` | Repository Layout 增加 `scripts/`、`docs/`、`runtime/` 说明；新增 docs 索引链接 | 反映新目录结构 |
| `docs/README.md` | 新建文档索引，含目录结构、分类规则、长期规则、当前文档清单 | 建立文档统一入口 |

### 未改动的引用（已验证有效）

| 引用方 | 路径 | 状态 |
| --- | --- | --- |
| `.github/workflows/ci.yml` | `scripts/hcz-manager.sh` | 有效，未改动 |
| `.github/workflows/ci.yml` | `scripts/tests/hcz-manager_test.sh` | 有效，未改动 |
| `.goreleaser.yaml` | `./scripts/hcz-manager.sh` | 有效，未改动 |
| `scripts/hcz-manager.sh` | `raw.githubusercontent.com/.../scripts/hcz-manager.sh` | 有效，未改动 |

---

## 7. 技术栈适配说明

本项目为 **Go + Gin + GORM + Vue3** 技术栈，非 PHP/Laravel。以下为针对目标模板的适配：

| 目标模板项 | Go 项目对应 | 处理方式 |
| --- | --- | --- |
| `tests/` 目录 | Go 惯例：`*_test.go` 与源码同目录（`internal/`、`cmd/`） | 不新建 `tests/`，保留 Go 惯例 |
| `composer.json` / `phpunit.xml` | `go.mod` / `go.sum` | 已在根目录，KEEP_ROOT |
| `database/migrations/` | Go 代码注册迁移（`internal/bootstrap/database/migrations/`） | 不涉及文件移动 |
| `scripts/test/` | 现有 `scripts/tests/` | 复用现有目录，不重命名（CI/goreleaser 引用） |
| PHPUnit 输出 | `go test` 输出 / `coverage.txt` | 已有忽略规则，追加 `coverage.xml`/`junit.xml` |

---

## 8. Verification

### Git

```
git status --short     → M .gitignore / M README.md / ?? docs/
git diff --stat        → 2 files changed, 15 insertions(+), 1 deletion(-)
git diff --name-status → M .gitignore / M README.md
```

仅 2 个已跟踪文件被修改（.gitignore、README.md），docs/ 为新增目录。无 Go 源码被修改，无意外内容改写。

### Go

```
go build ./cmd/server  → exit 0（PASS）
go vet ./...           → exit 0（PASS）
go test ./...          → exit 1（存在预存失败，详见下方）
```

### 路径验证

- CI 引用：`scripts/hcz-manager.sh`、`scripts/tests/hcz-manager_test.sh` — 有效
- Goreleaser 引用：`./scripts/hcz-manager.sh` — 有效
- README 链接：`docs/README.md` — 有效
- docs 内部链接：3 份 audit 相对路径 — 有效
- .gitignore：`runtime/` 及新增模式 — `git check-ignore` 确认生效

---

## 9. Final Verification

> 以下为实际执行结果，非推测。

### Git Status

```
$ git status --short
 M .gitignore
 M README.md
?? docs/

$ git diff --stat
 .gitignore | 9 +++++++++
 README.md  | 7 ++++++-
 2 files changed, 15 insertions(+), 1 deletion(-)

$ git diff --name-status
M       .gitignore
M       README.md
```

- 修改类型：2 个 modification，1 个新目录（docs/）
- 无 deletion、无 rename（移动的 3 个文件原为 untracked，Git 视为新位置新增）
- 无业务代码文件被修改

### Go Build / Vet / Test

```
$ go build ./cmd/server
exit code: 0  ✓ PASS

$ go vet ./...
exit code: 0  ✓ PASS

$ go test ./...
exit code: 1  ✗ 存在预存失败（与本次整理无关）
```

**失败包清单（均为预存问题，非本次引入）：**

| 包 | 失败测试 | 原因分类 |
| --- | --- | --- |
| `internal/app/httpserver` | `TestRouteDomainFilesPreserveTrustBoundaries` | routes_storefront.go 信任边界声明缺失 — 预存业务代码问题 |
| `internal/logger` | `TestNewReleaseWritesToConfiguredFile` | Windows TempDir 文件锁清理失败 — 平台相关 |
| `internal/modules/order/infrastructure/gormstore` | `TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts` / `TestRiskGateSerializesConcurrentGuestQuotaChecks` | Windows TempDir SQLite 文件锁 — 平台相关 |
| `internal/modules/order/integrationtest/application` | 6 个 `TestCreateOrderReseller*` / `TestCreateOrderSerializesCoupon*` | "wallet only payment required" — 预存业务逻辑测试失败 |
| `internal/modules/reseller/integrationtest` | `TestResellerAccountingServiceGetUserFinanceDashboardScopesToUserProfile` / `TestAdminResellerManagementListProfilesFilters` | UNIQUE constraint failed: users.email — 预存测试数据隔离问题 |
| `internal/selfupdate` | 9 个测试（capability/lock/manager/metadata/updater） | Windows 平台不支持 source build / 文件权限 / 文件锁 — 平台相关 |

**判定依据**：本次整理仅移动 3 个 MD 文件、修改 .gitignore 和 README.md、新增 docs/ 目录，未修改任何 `.go` 文件。上述失败均位于 Go 源码/测试中，且失败原因（业务约束、平台文件锁、测试数据）与文件路径整理无因果关系。

### Broken Path Scan

```
搜索范围: 全仓库 *.{md,yml,yaml,json,go,sh,ps1,xml,toml,Dockerfile}
搜索关键词: HCZ_FIT_GAP_AUDIT / HCZ_GLOBAL_CURRENCY_AUDIT / HCZ_PAYMENT_BOUNDARY_AUDIT
命中: 9 处，全部位于 docs/ 内部（cleanup report + docs/README.md 索引）
结论: 无代码/CI/Docker 引用旧根目录路径，无 broken path
```

```
$ git check-ignore runtime/ runtime/logs/ runtime/coverage/
runtime/
runtime/logs/
runtime/
runtime/coverage/
runtime/
结论: runtime/ 全目录已被 gitignore 正确拦截
```

---

## 10. Final Verdict

**PASS WITH CONDITIONS**

**通过项：**
- 根目录散落 Audit 文件已全部清理（3 → 0）
- docs/ 统一目录结构已建立，含文档索引
- runtime/ 已建立并被 .gitignore 拦截
- scripts/ 现有结构保留，CI/goreleaser 引用未破坏
- .gitignore 已追加生成文件规则
- README.md 已更新 Repository Structure
- go build ✓ / go vet ✓
- 无 broken path / 无业务代码修改

**条件项（需后续独立处理，非本轮阻塞）：**
- `go test ./...` 存在 6 个包的预存失败，主要为 Windows 平台文件锁和 order/reseller 业务逻辑测试问题，需单独开 issue 排查，不属于 Repository Hygiene 范围
- 建议在 Linux CI 环境复跑 `go test ./...` 以区分平台相关失败与真实业务失败

---

## 11. 长期规则（自本轮起执行）

| 产物类型 | 归属目录 |
| --- | --- |
| 测试代码（`*_test.go`） | `internal/`、`cmd/`（Go 惯例） |
| 测试执行脚本 | `scripts/test/`（现有 `scripts/tests/`） |
| 审计工具脚本 | `scripts/audit/` |
| 迁移辅助脚本 | `scripts/migration/` |
| 运维 / 维护脚本 | `scripts/maintenance/` |
| 审计报告 | `docs/audits/{backend,security,database,legacy}/` |
| 测试 / 回归 / 阶段报告 | `docs/reports/{tests,regression,phases}/` |
| 执行教程 / SOP | `docs/runbooks/` |
| 问题 / RCA | `docs/issues/` |
| 架构决策（ADR） | `docs/decisions/` |
| 历史文件 | `docs/archive/` |
| 临时 / Log / Coverage / 机器输出 | `runtime/`（gitignore，禁止提交） |

**根目录禁止**继续出现 `AUDIT_*.md`、`REPORT_*.md`、`TEST_*.txt`、`result*.txt`、`debug*.log`、`test-output*.txt`、`temp*.php`、`check*.ps1`、`verify*.ps1`、`*-final.md`、`*-fix.md` 等散落文件，除非有明确项目级理由。
