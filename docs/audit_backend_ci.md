# HCZ V1 Backend 全量验证 + Linux CI 审计报告

- **审计时间**: 2026-10-05 20:19 (Asia/Shanghai)
- **项目根**: `E:\Users\orang\Downloads\Compressed\hcz_v1`
- **Go module**: `github.com/Aether-v1/hcz`
- **Go 版本**: go1.26.5 windows/amd64
- **HEAD commit**: `2ed401341ef36a6a7e4f71359a678a8e9064327c`
- **远程仓库**: https://github.com/Aether-v1/hcz

---

## 1. 执行摘要

| 检查项 | 修复前 | 修复后 |
|--------|--------|--------|
| gofmt -l . | ✅ PASS（无输出） | ✅ PASS |
| go vet ./... | ✅ PASS（exit 0，无 warning） | ✅ PASS |
| go test ./... | ❌ FAIL（含 1 个真实分层违规） | ⚠️ FAIL（仅剩 Windows 平台相关失败 + 1 个 flaky） |
| go build ./... | ✅ PASS（exit 0） | ✅ PASS |
| Linux CI (commit 2ed4013) | ❌ Verify API = failure | ⚠️ 本地修复已完成，待推送后 CI 复绿 |

**关键发现**: 最新 main commit `2ed4013` 的 Linux CI 处于 **红色** 状态（Verify API job 在 "Run tests" 步骤失败）。根因是 `internal/architecture/TestDependencyRules` 检测到 sitebuilder 模块的 application/transport 层违规 import 了 infrastructure/gormstore。该问题已在本地修复（将 `ReorderItem` DTO 从 gormstore 移至 application 包），修复后架构测试通过。

---

## 2. gofmt 校验

```
命令: gofmt -l .
结果: 输出为空 → PASS
```

无未格式化文件。修复后再次对 `internal/modules/sitebuilder/` 执行 `gofmt -l` 仍为空。

---

## 3. go vet 校验

```
命令: go vet ./...
结果: exit code 0，无输出 → PASS
```

无 warning 或 error。

---

## 4. go test ./... 结果汇总

### 4.1 总体统计

| 指标 | 数量 |
|------|------|
| 总包数 | 405 |
| PASS (ok) | 196 |
| 无测试文件 (?) | 205 |
| FAIL | 4 |

### 4.2 FAIL 包分类

| # | 包名 | 失败测试数 | 分类 | 说明 |
|---|------|-----------|------|------|
| 1 | `internal/logger` | 1 | **Windows 平台问题** | `TestNewReleaseWritesToConfiguredFile`: TempDir RemoveAll 清理时 release.log 文件被占用（Windows 文件锁语义）。Linux 无此问题。 |
| 2 | `internal/modules/order/infrastructure/gormstore` | 2 | **Windows 平台问题** | `TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts`、`TestRiskGateSerializesConcurrentGuestQuotaChecks`: TempDir 清理时 risk-gate.db SQLite 文件句柄未释放。Windows SQLite 文件锁与 Linux 不同。 |
| 3 | `internal/modules/reseller/integrationtest` | 1 | **Flaky 测试（P2）** | `TestAdminResellerManagementListProfilesFilters`: `UNIQUE constraint failed: users.email`。首次运行通过（4.168s），二次运行失败（3.305s）。疑似测试隔离/DB 清理不完整。 |
| 4 | `internal/selfupdate` | 9 | **已知 Windows 预存在问题** | 见下方 4.3 节。 |

### 4.3 selfupdate 已知 Windows 失败（9 个，与所有 Phase 无关）

| 测试名 | 原因 |
|--------|------|
| `TestDetectBlocksSourceBuild` | unsupported_os（Windows 不识别 source build） |
| `TestDetectReleaseBuild` | unsupported_os |
| `TestDirWritable` | 0500 目录权限测试依赖 Unix chmod |
| `TestBinaryLockIsExclusive` | Windows 文件锁语义与 Unix flock 不同 |
| `TestConcurrentRollbacksDoNotBothSucceed` | Windows 文件锁语义 |
| `TestManagerStartHoldsBinaryLockDuringReleaseFetch` | Windows 文件锁语义 |
| `TestStartupGuardReadOnlyStateFailsClosedBeforeMigration` | Unix 只读目录权限测试 |
| `TestStartupGuardBlocksConcurrentRollback` | Windows 文件锁语义 |
| `TestExtractBinaryRejectsTruncatedEntry` | Windows 临时文件清理行为 |

以上 9 个在 Linux CI 上全部通过（已由 CI 历史验证）。

### 4.4 已修复的真实失败（P0）

**修复前**: `internal/architecture/TestDependencyRules` FAIL

违规详情（修复前）:
```
internal/modules/sitebuilder/application/discovery_block_service.go
  imports "internal/modules/sitebuilder/infrastructure/gormstore":
  application code must depend on ports, not infrastructure, transport, or bootstrap packages

internal/modules/sitebuilder/application/home_entry_service.go
  imports "internal/modules/sitebuilder/infrastructure/gormstore": 同上

internal/modules/sitebuilder/transport/http/admin_handler.go
  imports "internal/modules/sitebuilder/infrastructure/gormstore":
  transport code must depend on application contracts, not concrete stores or infrastructure
```

**根因**: `ReorderItem` 结构体（`{ID uint, SortOrder int}`）被错误定义在 `infrastructure/gormstore` 包中，但它是 application 层端口接口 `DiscoveryBlockStore.Reorder()` / `HomeEntryStore.Reorder()` 的参数类型，导致 application 和 transport 层为了引用这个 DTO 而被迫 import infrastructure 包。

**修复后**: `internal/architecture/TestDependencyRules` PASS（0.28s）

详见第 7 节。

---

## 5. go build ./... 结果

```
命令: go build ./...
结果: exit code 0 → PASS
```

编译通过，无错误。修复后重新编译同样 PASS。

---

## 6. Linux CI 真实检查（commit 2ed4013）

### 6.1 Workflow 文件

`.github/workflows/` 下实际有两个文件:
- `ci.yml`（name: ci）— 包含 4 个 job
- `release.yml`（name: goreleaser）— 仅 tag 触发

### 6.2 CI Job 状态表（commit 2ed4013）

| Job 名称 | 状态 | Conclusion | 证据 URL |
|----------|------|------------|----------|
| Verify installer | completed | ✅ **success** | https://github.com/Aether-v1/hcz/actions/runs/37306713335/job/111752032167 |
| Verify API | completed | ❌ **failure** | https://github.com/Aether-v1/hcz/actions/runs/37306713335/job/111752032181 |
| Verify release config | completed | ✅ **success** | https://github.com/Aether-v1/hcz/actions/runs/37306713335/job/111752032126 |
| Verify fullstack build | completed | ✅ **success** | https://github.com/Aether-v1/hcz/actions/runs/37306713335/job/111752031673 |

Run URL: https://github.com/Aether-v1/hcz/actions/runs/37306713335

### 6.3 Verify API 失败步骤定位

| Step # | 步骤名 | 状态 |
|--------|--------|------|
| 1 | Set up job | success |
| 2 | Checkout | success |
| 3 | Set up Go | success |
| 4 | Download dependencies | success |
| 5 | Check formatting | success |
| 6 | Run vet | success |
| **7** | **Run tests** | **❌ failure** |
| 8 | Build server | skipped |

**失败根因**: Step 7 `go test ./...` 在 Linux 上失败。gofmt（Step 5）和 vet（Step 6）均通过。失败的测试是 `internal/architecture/TestDependencyRules`（sitebuilder 分层违规），该问题与平台无关，在 Linux 上同样触发。

### 6.4 main 分支 CI 历史

| Run ID | Commit | Conclusion | 时间 (UTC) |
|--------|--------|------------|------------|
| 37306713335 | 2ed4013 (HEAD) | ❌ failure | 2026-10-05 12:01 |
| 37305985242 | 8078b408 | ❌ failure | 2026-10-05 11:54 |
| 37305358398 | 226e82b2 | ❌ failure | 2026-10-05 11:48 |
| 37291014003 | aba7a6e8 | ❌ failure | 2026-10-05 09:35 |
| 37290439497 | 286b388e | ✅ success | 2026-10-05 09:30 |

最近 4 次提交 CI 全部红色，上一次绿色是 `286b388e`。

---

## 7. 关键模块测试通过情况

| 模块 | 包路径 | 状态 |
|------|--------|------|
| wallet | `internal/modules/wallet/integrationtest` | ✅ ok |
| walletwithdrawal | `internal/modules/walletwithdrawal/integrationtest` | ✅ ok |
| affiliate | `internal/modules/affiliate/integrationtest` | ✅ ok |
| c2c | `internal/modules/c2c/integrationtest` | ✅ ok |
| supportticket | `internal/modules/supportticket/integrationtest` | ✅ ok |
| sitebuilder | `internal/modules/sitebuilder/application` | ✅ ok（修复后） |
| exchangerate | `internal/modules/exchangerate/application` | ✅ ok |
| order / refund | `internal/modules/order/integrationtest/refund` | ✅ ok |
| order / aftersale | `internal/modules/order/integrationtest/aftersale` | ✅ ok |
| authz | `internal/authz` | ✅ ok |

所有关键模块均有测试且通过。

---

## 8. 已执行的修复

### 8.1 P0 修复：sitebuilder 分层违规

**问题**: `ReorderItem` DTO 定义在 infrastructure/gormstore，导致 application 和 transport 层违规 import infrastructure。

**修复方案**: 将 `ReorderItem` 结构体移至 application 包（与它服务的端口接口同包），gormstore 改为 import application。

**修改文件清单**:

| 文件 | 修改内容 |
|------|----------|
| `internal/modules/sitebuilder/application/discovery_block_service.go` | 移除 gormstore import；新增 `ReorderItem` struct 定义；接口和方法签名改用 `ReorderItem` |
| `internal/modules/sitebuilder/application/home_entry_service.go` | 移除 gormstore import；接口和方法签名改用 `ReorderItem` |
| `internal/modules/sitebuilder/application/sitebuilder_test.go` | 移除 gormstore import；fake store 的 Reorder 方法签名改用 `ReorderItem` |
| `internal/modules/sitebuilder/transport/http/admin_handler.go` | 移除 gormstore import；4 处 `gormstore.ReorderItem` 改为 `sitebuilderapp.ReorderItem` |
| `internal/modules/sitebuilder/infrastructure/gormstore/discovery_block_store.go` | 移除 `ReorderItem` struct 定义；新增 application import；Reorder 方法签名改用 `sitebuilderapp.ReorderItem` |
| `internal/modules/sitebuilder/infrastructure/gormstore/home_entry_store.go` | 新增 application import；Reorder 方法签名改用 `sitebuilderapp.ReorderItem` |

**修复前验证**: `TestDependencyRules` FAIL（3 条违规）
**修复后验证**: `TestDependencyRules` PASS（0.28s），`go build ./...` exit 0，`gofmt -l` 无输出

---

## 9. 问题清单

### P0（阻断 CI，已本地修复，待推送）

1. **sitebuilder 分层违规导致 Linux CI 红灯**
   - `internal/modules/sitebuilder/application/` 和 `transport/http/` 违规 import `infrastructure/gormstore`
   - 根因：`ReorderItem` DTO 位置错误
   - 状态：✅ 已本地修复，`TestDependencyRules` PASS
   - 待办：推送到 main 后验证 CI 复绿

### P1（Windows 平台已知问题，不影响 Linux CI）

2. **internal/logger — TestNewReleaseWritesToConfiguredFile**
   - Windows TempDir 清理时日志文件句柄未释放
   - Linux CI 无此问题

3. **internal/modules/order/infrastructure/gormstore — 2 个 risk gate 测试**
   - Windows SQLite 文件锁导致 TempDir 清理失败
   - Linux CI 无此问题

4. **internal/selfupdate — 9 个测试**
   - unsupported_os / Unix 权限 / 文件锁语义差异
   - Linux CI 全部通过（已知预存在问题）

### P2（需关注的 flaky 测试）

5. **internal/modules/reseller/integrationtest — TestAdminResellerManagementListProfilesFilters**
   - `UNIQUE constraint failed: users.email`（2067）
   - 首次运行通过，二次运行失败
   - 疑似测试间 DB 清理不完整或并发子测试 email 碰撞
   - 建议：检查 `management_test.go` 中用户创建逻辑的 email 唯一性和 DB cleanup 策略

---

## 10. 最终结论

| 问题 | 结论 |
|------|------|
| Backend 是否全绿（排除已知 selfupdate）？ | ⚠️ **修复后本地基本全绿**：gofmt ✅ / vet ✅ / build ✅；go test 仅剩 Windows 平台相关失败（logger 1 + order gormstore 2 + reseller flaky 1）和已知 selfupdate 9 个。真实分层违规已修复。 |
| Linux CI 是否最新 main 全绿？ | ❌ **当前 HEAD (2ed4013) CI 红色**：Verify API job 在 Run tests 步骤失败。根因是 sitebuilder 分层违规（本报告 P0 #1），已本地修复，推送后预计复绿。 |
| 是否有新的真实失败？ | ✅ **已发现并修复 1 个真实失败**（P0 sitebuilder 分层违规）。另有 1 个 flaky 测试（P2 reseller）需关注。其余失败均为 Windows 平台特性或已知 selfupdate 问题。 |
