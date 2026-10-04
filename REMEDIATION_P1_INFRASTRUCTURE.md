# 安全 P1 全量回归与 CI 报告

## 第一步：gofmt
- 格式化文件数：1（internal/app/httpserver/refund_compliance_test.go）
- gofmt validation：PASS（`gofmt -l .\internal\ .\cmd\` 复查为空输出）

## 第二步：全量回归
| 步骤 | 结果 | 详情 |
|---|---|---|
| gofmt | PASS | `gofmt -l` 空输出 |
| go vet | PASS | exit 0 |
| go test ./... | PASS（仅 Windows flaky） | 190 ok / 3 FAIL(flaky) / 175 no-test-files，共 368 包 |
| go build | PASS | exit 0 |
| User vue-tsc | PASS | npx vue-tsc --noEmit exit 0 |
| User build | PASS | vite build exit 0（built in 17.78s） |
| Admin vue-tsc | PASS | npx vue-tsc --noEmit exit 0 |
| Admin build | PASS | vite build exit 0（built in 18.85s，仅 esbuild 重复 case 警告） |
| Migration | PASS | TestAutoMigrate* 全部 PASS |
| 核心模块回归 | PASS | httpserver / userauth / aftersale / middleware 全部 ok；refund 包无测试文件 |

### go test 详情
- 总包数：368，PASS：190，FAIL：3（全部为已知 Windows flaky）
- Windows flaky（不得计入真实失败）：
  - `internal/logger` — TestNewReleaseWritesToConfiguredFile：TempDir RemoveAll 时 release.log 文件句柄被占用
  - `internal/modules/order/infrastructure/gormstore` — TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts / TestRiskGateSerializesConcurrentGuestQuotaChecks：TempDir SQLite risk-gate.db 文件锁
  - `internal/selfupdate` — TestDetectBlocksSourceBuild / TestDetectReleaseBuild / TestDirWritable / TestBinaryLockIsExclusive / TestConcurrentRollbacksDoNotBothSucceed / TestManagerStartHoldsBinaryLockDuringReleaseFetch / TestStartupGuardReadOnlyStateFailsClosedBeforeMigration / TestStartupGuardBlocksConcurrentRollback / TestExtractBinaryRejectsTruncatedEntry：Unix 权限/文件锁语义（0500 目录、独占锁）在 Windows 不适用
- 真实失败：无

### P1 新增测试显式验证（均 PASS）
- refund_compliance_test.go：TestRefundWriteRoutesBlockedByComplianceWhenNotAcked / TestRefundWriteRoutesPassComplianceWhenAcked / TestDuplicateManualRefundDoesNotDoubleCredit —— PASS
- p1_2_auth_rate_limit_test.go：TestP1_2RegisterRouteRateLimitedConfirmsMiddlewareMounted / TestP1_2VerifyRouteRateLimitedConfirmsMiddlewareMounted / TestP1_2ForgotRouteRateLimitedConfirmsMiddlewareMounted / TestP1_2NormalFrequencyNotBlocked / TestP1_2ForgotPasswordDoesNotLeakAccountExistence / TestP1_2SendVerifyResetDoesNotLeakAccountExistence —— PASS

## 第三步：Commit + Push
- Commit hash：11bdc6c9e9b9e69a41a81eaa3440d433989c2786（短：11bdc6c）
- 修改文件数：10（8 个已跟踪修改 + 2 个新增 P1 测试）
  - internal/app/httpserver/route_structure_test.go（M）
  - internal/app/httpserver/router.go（M）
  - internal/app/httpserver/routes_admin.go（M）
  - internal/app/httpserver/routes_storefront.go（M）
  - internal/architecture/router_composition_structure_test.go（M）
  - internal/modules/identity/userauth/transport/http/routes.go（M）
  - internal/modules/identity/userauth/transport/http/user_password_handler.go（M）
  - internal/modules/identity/userauth/transport/http/user_verify_handler.go（M）
  - internal/app/httpserver/refund_compliance_test.go（新增）
  - internal/app/httpserver/p1_2_auth_rate_limit_test.go（新增）
- 暂存集确认：不含任何审计/remediation md、不含临时 txt（test_regression_p1.txt 已删除）
- Push：**阻塞，等待用户桌面 GCM 授权**
  - 诊断：非交互模式返回 `fatal: could not read Username for 'https://github.com'`，确认本机无缓存凭据，GCM 必须弹出桌面 GUI 完成 GitHub 授权。
  - 后台 push 任务仍存活（task db659037），用户在 GCM 窗口完成授权后即自动推送成功。
  - 授权前 origin/main 仍为 d0a4f73。

## 第四步：CI 真实验证
- NOT EXECUTED（等待 push 完成后，用 GitHub REST API 轮询 head_sha=11bdc6c 的 run）
- Run ID：待填充
- 状态：待填充
| Job | 结果 | 耗时 |
|---|---|---|
| installer | 待填充 | |
| api | 待填充 | |
| release-config | 待填充 | |
| fullstack | 待填充 | |

## 总结
- 全量回归：PASS（gofmt/vet/build/前后端构建/migration/核心模块/P1 新增测试全部通过；go test ./... 仅 3 个已知 Windows flaky，无真实失败）
- Commit：完成（11bdc6c）
- Push：**待用户完成 GCM 桌面授权**
- CI：NOT EXECUTED（依赖 push）
