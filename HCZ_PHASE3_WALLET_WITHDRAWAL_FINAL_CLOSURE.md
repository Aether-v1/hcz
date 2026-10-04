# HCZ Phase 3 — Wallet Withdrawal Final Closure

**日期**: 2026-10-04
**Commit**: `1170b62` (feat(wallet-withdrawal): Phase 3 final closure)
**基线报告**: HCZ_PHASE3_WALLET_WITHDRAWAL_FINAL.md (PASS WITH CONDITIONS)
**本轮目标**: 关闭剩余条件，正式封板

---

## Final Verdict: PASS

HCZ Phase 3 Wallet Withdrawal 正式封板。所有剩余条件已关闭，Linux CI 真实全绿，无资金/安全阻断。

---

## 一、本轮修改文件清单

### 1.1 后端 — new_user_cooldown_hours 真实生效（7 文件）

| 文件 | 操作 | 作用 |
|------|------|------|
| `internal/modules/walletwithdrawal/contract/ports.go` | 修改 | 新增 `UserReader` 端口（`GetByID(uint)(*userdomain.User, error)`） |
| `internal/modules/walletwithdrawal/application/service.go` | 修改 | Options 新增 `Users UserReader`，Service 新增 `users` 字段，NewService 赋值 |
| `internal/modules/walletwithdrawal/contract/errors.go` | 修改 | 新增 `ErrNewUserCooldown`、`ErrUserNotFound` |
| `internal/modules/walletwithdrawal/application/create.go` | 修改 | TOTP 校验之后、事务扣款之前插入冷却校验（duration 比较，非 float） |
| `internal/app/container/services_application.go` | 修改 | WithdrawalService 装配加入 `Users: c.UserStore` |
| `internal/modules/walletwithdrawal/integrationtest/helpers_test.go` | 修改 | 新增 `mockUserReader`、fixture `users` 字段、`setUserCreatedAt`/`countWithdrawals`/`countLedger` helper |
| `internal/modules/walletwithdrawal/integrationtest/cooldown_test.go` | **新增** | 4 个冷却风控测试用例 |

### 1.2 前端 — i18n 三语言补齐 + 地址簿 UI（5 文件）

| 文件 | 操作 | 作用 |
|------|------|------|
| `frontend/user/src/i18n/locales/zh-CN.json` | 修改 | `personalCenter.wallet.withdraw` 下新增 53 个 key（简体中文） |
| `frontend/user/src/i18n/locales/zh-TW.json` | 修改 | 同上，繁體中文 |
| `frontend/user/src/i18n/locales/en-US.json` | 修改 | 同上，English |
| `frontend/admin/src/i18n/index.ts` | 修改 | 三个 locale 段（zh-CN/zh-TW/en）的 `admin.walletWithdrawals` 下新增 61 个 key |
| `frontend/user/src/views/personal/WalletWithdrawal.vue` | 修改 | 新增地址管理区域：地址列表 + 设为默认按钮 + 删除按钮（confirm 确认），复用已有 API |

### 1.3 i18n key 统计

| 端 | 命名空间 | key 数量 | 语言 |
|----|----------|----------|------|
| User | `personalCenter.wallet.withdraw.*` | 53 | zh-CN / zh-TW / en-US |
| Admin | `admin.walletWithdrawals.*` | 61 | zh-CN / zh-TW / en |
| **合计** | | **114** | 三语言结构完全一致 |

User 端覆盖：钱包入口、余额卡、表单、网络、常用地址、地址输入、金额、fee/net、TOTP、提交、历史页、取消提现、7 种状态、错误提示、地址簿管理。
Admin 端覆盖：列表页筛选、详情页、时间线（6 个时间点）、操作区（approve/reject/processing/complete）、反馈消息、status 子对象（6）、table 子对象（9）。

---

## 二、new_user_cooldown_hours 实现细节

### 2.1 校验位置
```
CreateWithdrawal 执行顺序：
1. 基础入参校验
2. 读取 withdrawal_config
3. network 白名单 + TRC20 Base58Check 地址校验
4. min/max 金额校验
5. TOTP 校验（fail-closed）
6. ★ 新用户冷却校验（本轮新增）← 在 TOTP 之后、扣款之前
7. 手续费计算
8. 幂等预检
9. 事务：行锁账户 → 余额校验 → 日限额 → 首提限额 → 扣款 → ledger → 写提现单
```

### 2.2 校验规则
- 读取 `user.CreatedAt`，计算 `time.Since(user.CreatedAt)`
- 若 `< time.Duration(cfg.NewUserCooldownHours) * time.Hour` → 返回 `ErrNewUserCooldown`
- `cfg.NewUserCooldownHours == 0` → 关闭限制，跳过校验
- `s.users == nil` → 跳过（向后兼容）
- 被冷却拒绝时：**Wallet 不扣款、Withdrawal 不创建、Ledger 不写入**

### 2.3 精度
使用 `time.Duration` 比较，未使用 `float64` 小时数，避免精度损失。

---

## 三、全量验证结果汇总

### 3.1 后端

| 检查项 | 命令 | 结果 |
|--------|------|------|
| 代码格式 | `gofmt -l .` | ✅ 空（干净） |
| 静态检查 | `go vet ./...` | ✅ exit 0 |
| 全量编译 | `go build ./...` | ✅ exit 0 |
| 全量测试 | `go test ./...` | ⚠️ 仅 `internal/selfupdate` 9 个预存 Windows 环境失败（见下），其余全部 PASS |
| 提现模块测试 | `go test ./internal/modules/walletwithdrawal/...` | ✅ ok (4.838s) |
| wallet 模块测试 | `go test ./internal/modules/wallet/...` | ✅ ok |
| settings 模块测试 | `go test ./internal/modules/settings/...` | ✅ ok |
| identity 模块测试 | `go test ./internal/modules/identity/...` | ✅ ok |
| notification 模块测试 | `go test ./internal/modules/notification/...` | ✅ ok |
| architecture 测试 | `go test ./internal/architecture/...` | ✅ ok |

**关于 `internal/selfupdate` 失败**：
- 9 个失败用例：`TestDetectBlocksSourceBuild`、`TestDetectReleaseBuild`、`TestDirWritable`、`TestBinaryLockIsExclusive`、`TestConcurrentRollbacksDoNotBothSucceed`、`TestManagerStartHoldsBinaryLockDuringReleaseFetch`、`TestStartupGuardReadOnlyStateFailsClosedBeforeMigration`、`TestStartupGuardBlocksConcurrentRollback`、`TestExtractBinaryRejectsTruncatedEntry`
- 性质：Windows 平台二进制锁/目录可写/发布检测相关测试，**该包未被本轮修改**，为预存环境问题
- Linux CI 中该包测试**全部通过**（见 3.3），确认与提现代码无关

### 3.2 前端

| 端 | vue-tsc --noEmit | npm run build |
|----|-------------------|---------------|
| User | ✅ 0 errors | ✅ 成功 (22.77s) |
| Admin | ✅ 0 errors | ✅ 成功 (24.49s) |

> Admin build 中有一条预存 warning（`status.ts` 重复 case 子句），与本次修改无关，不影响构建成功。

### 3.3 Linux CI（真实结果，非预计）

**Workflow**: ci (run ID 37201909178, commit 1170b62, event=push)
**总体结论**: ✅ success

| Job | 状态 | 耗时 | 覆盖内容 |
|-----|------|------|----------|
| **Verify installer** | ✅ success | 14s | bash 语法、shellcheck、installer 测试、release archive 检查 |
| **Verify release config** | ✅ success | 15s | goreleaser check |
| **Verify API** | ✅ success | 150s | gofmt（git ls-files 范围）、go vet ./...、**go test ./...**、go build ./cmd/server |
| **Verify fullstack build** | ✅ success | 150s | pnpm install、admin/user frontend test、admin/user build（含 vue-tsc）、embed assets、`go build -tags release,fullstack` |

**关键确认**：
- Linux 下 `go test ./...` 全绿（含 `internal/selfupdate`，证明该包失败是 Windows 特有）
- Linux 下两个前端 build 全绿（含 vue-tsc 类型检查）
- fullstack embed 构建通过
- 4 个 pipeline 全部真实 success，无预计/假设

---

## 四、明确回答

### 1. Withdrawal 是否正式封板？
**是。** 核心资金链（申请即扣款、reject/cancel 退款、6 态状态机、TOTP fail-closed、Admin 合规保护、Ledger 完整、通知接入）+ 封板条件（cooldown 生效、i18n 三语言、地址簿 UI）全部完成，Linux CI 全绿。

### 2. new_user_cooldown_hours 是否真实生效？
**是。** `CreateWithdrawal` 在 TOTP 校验之后、事务扣款之前执行冷却校验。配置 > 0 时，注册时间不足的用户返回 `ErrNewUserCooldown`，Wallet 不扣款、Withdrawal 不创建、Ledger 不写入。配置为 0 时关闭。4 个集成测试验证：冷却内拒绝、冷却到期允许、cooldown=0 允许、TOTP 优先于 cooldown。

### 3. i18n 是否完整（三语言 missing key = 0）？
**是。** 扫描 User/Admin 全部 5 个提现相关 Vue 文件，提取 114 个唯一 key（User 53 + Admin 61），全部补齐到 zh-CN、zh-TW、en（Admin）/ en-US（User）三个语言包。三语言 key 结构完全一致。页面不再显示原始 key 字符串。

### 4. User/Admin 是否 build PASS？
**是。** User 端 vue-tsc 0 errors + build 成功 (22.77s)；Admin 端 vue-tsc 0 errors + build 成功 (24.49s)。Linux CI fullstack job 也验证了两个前端的 build + test 全绿。

### 5. Linux CI 是否真实全绿？
**是。** commit `1170b62` 的 ci workflow（run ID 37201909178）4 个 job 全部 success：
- installer ✅ (14s)
- API ✅ (150s) — 含 `go test ./...` 全绿
- release-config ✅ (15s)
- fullstack ✅ (150s) — 含前端 test/build + embed + fullstack binary

通过 GitHub API 真实查询确认，非预计。

### 6. 是否还有提现资金/安全阻断？
**无。**
- 资金：申请即扣款 + reject/cancel 退款，行锁 + 事务 + 唯一索引 + 状态机四重幂等，无重复扣款/退款风险
- 安全：TOTP fail-closed、Admin JWT + RBAC + PaymentCompliance、IDOR fail-closed、TRC20 本地 Base58Check、手续费服务端计算
- 配置：全部走 Admin Settings withdrawal_config，无硬编码
- 测试：20+ 集成用例覆盖正常/异常/并发/幂等/退款/cooldown
- CI：Linux 全绿

### 7. 是否正式允许进入 Phase 4（10-Level Affiliate）？
**是。** HCZ Phase 3 Wallet Withdrawal 正式封板，无资金/安全阻断，Linux CI 全绿，可以进入 Phase 4。

---

## 五、已知 P2 后续项（非阻断）

| # | 项目 | 说明 | 优先级 |
|---|------|------|--------|
| 1 | Admin 写操作 Idempotency-Key 应用层去重 | 当前状态机 + 行锁已防重复状态迁移，但重复 approve 请求返回错误而非幂等成功。可在 handler 层加幂等缓存 | P2 |
| 2 | 并发超扣 MySQL 真实竞争测试 | SQLite in-memory 下串行执行，行锁逻辑复用 wallet 模块已验证的 SELECT FOR UPDATE。可在 MySQL 环境补充真实并发测试 | P2 |
| 3 | `internal/selfupdate` Windows 测试适配 | 9 个测试在 Windows 因二进制锁/目录可写差异失败，Linux 全绿。可加 build tag 或 skip Windows | P2 |
| 4 | Admin 端 i18n 拆分 | Admin i18n/index.ts 为 13000+ 行单文件，可按模块拆分，但不影响功能 | P3 |

---

## 六、Git 信息

- **Commit**: `1170b6295cfc375b95d4c3bf14f93998773faaf8`
- **Branch**: main
- **变更**: 55 files changed, 5313 insertions(+), 9 deletions(-)
- **CI Run**: https://github.com/Aether-v1/hcz/actions/runs/37201909178
- **CI 结论**: ✅ success (4/4 jobs)

---

## 七、结论

HCZ Phase 3 Wallet Withdrawal 从初始实现（PASS WITH CONDITIONS）到本轮封板（PASS），所有剩余条件已关闭：

1. ✅ new_user_cooldown_hours 真实生效（TOTP 后、扣款前，4 测试验证）
2. ✅ i18n 三语言 114 key 补齐（missing key = 0）
3. ✅ User 地址簿删除/设默认 UI 实现
4. ✅ 全量验证：gofmt / vet / build / 关键模块测试全过
5. ✅ Git commit + push
6. ✅ Linux CI 真实全绿（installer / API / release-config / fullstack 4/4）

**Final Verdict: PASS — 正式封板，允许进入 Phase 4（10-Level Affiliate）。**
