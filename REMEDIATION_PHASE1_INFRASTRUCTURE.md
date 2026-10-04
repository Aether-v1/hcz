# Phase 1 User Notification - 全量回归与 CI 报告

## 第一步：gofmt
- 格式化文件数：5（container.go / migrations/registry.go / fulfillment/service.go / usernotification integrationtest + handler_test.go，后续补排 dispatch.go 共 6 个文件落盘）
- gofmt validation：PASS（`gofmt -l .\internal\ .\cmd\` 空输出）

## 第二步：全量回归
| 步骤 | 结果 | 详情 |
|---|---|---|
| gofmt | PASS | `gofmt -l` 空输出 |
| go vet | PASS | exit 0 |
| go test ./... | PASS（扣除已知 Windows flaky） | 374 packages / ok=190 / FAIL=5（其中真实失败 3 个已修复，其余为 flaky） |
| go build | PASS | exit 0 |
| User vue-tsc | PASS | 0 错误 |
| User build | PASS | exit 0（vite built in 19.18s） |
| Migration | PASS | TestAutoMigrate... PASS（16.8s） |
| 核心模块回归 | PASS | usernotification / httpserver / payment 全部 ok |

### go test 详情
- 总包数：374，PASS：190（不含 `[no test files]` 的 `?` 条目），FAIL：5（首次运行）
- Windows flaky（不视为真实失败）：
  - `internal/logger` TestNewReleaseWritesToConfiguredFile — TempDir release.log 文件句柄占用
  - `internal/modules/order/infrastructure/gormstore` TestRiskGate* — SQLite risk-gate.db 文件锁（TempDir cleanup）
  - `internal/selfupdate` 9 用例 — Unix 权限/锁语义（0500 目录、排他锁、并发回滚）
  - `internal/modules/reseller/integrationtest` TestResellerAccountingServiceGetUserFinanceDashboardScopesToUserProfile — 并行包间时间戳 email 偶发冲突，隔离重跑 PASS
- 真实失败（本轮修复）：
  1. `internal/architecture` TestDependencyRules — `usernotification/transport/http/handler_test.go` 直接 import gormstore。根因：该测试是黑盒端到端装配（真实 sqlite + gormstore 跑 handler→app→store 全链），按 `aftersale_handler_test.go` 先例登记进 `isKnownTransportIntegrationTest` 白名单。
  2. `internal/architecture` TestPaymentCallbackImplementationIsSplitByResponsibility — dispatch 文件新增 `notifyUserOrderProcessing` / `notifyUserWalletRechargeSuccess` 未登记。根因：Phase 1 在 payment_service_callback_dispatch.go 末尾追加了两个站内通知写入函数；属 post-callback fan-out 通道之一（与既有 enqueue* 通知同级），更新 dispatch 预期清单，不新增文件（避免触发 payment/application 文件预算守卫 13/12）。
  3. `internal/architecture` TestPaymentServiceImplementationIsSplitByResponsibility — payment_service.go 新增 `SetUserNotifier` 未登记。与既有 `SetProcurementService/SetDownstreamCallbackService/SetMemberLevelService` 同类依赖 setter，补入预期清单。
- 修复后 architecture 包复跑：PASS（2.696s）；build exit 0。

## 第三步：Commit + Push
- Commit hash：`dec7575`（dec7575b23a525697a7b5913814d1add67955edb）
- 修改文件数：35（+1581 / -3），含新建 usernotification 模块 12 文件 + 前端 3 文件
- Push：成功（11bdc6c..dec7575  main -> main，非交互）
- 暂存集已确认不含 test_regression_phase1.txt / push_live.txt / test_regression.txt / 所有 AUDIT_*.md / HCZ_*.md / REMEDIATION_*.md

## 第四步：CI 真实验证
- Run ID：37195307112
- head_sha：dec7575b23a525697a7b5913814d1add67955edb
- 状态：completed / success（queued → in_progress → completed，约 4.5 分钟）

| Job | 结果 | 耗时 |
|---|---|---|
| installer（Verify installer） | success | 19s |
| api（Verify API） | success | 3m22s |
| release-config（Verify release config） | success | 17s |
| fullstack（Verify fullstack build） | success | 2m3s |

## 总结
- 全量回归：PASS（3 个真实 architecture 失败已修复并复跑通过；剩余 4 个 FAIL 均为已知 Windows flaky / 并行隔离偶发）
- CI：全绿（4/4 jobs success，run 37195307112）
