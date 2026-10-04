# HCZ Production Security P1 Final Closure

> 修复日期：2026-10-04
> 修复范围：HCZ_PRODUCTION_READINESS_FULL_AUDIT.md 发现的 2 个生产前安全 P1
> Commit：`11bdc6c`（已 push origin/main）
> CI Run：#37184824780 — completed / **success**（4/4 jobs 全绿）
> 原则：只关闭 2 个安全 P1，禁止新增业务功能

---

## Final Verdict: PASS

### 2 个生产前安全 P1 全部关闭 ✅

| # | P1 | 状态 | 修复文件 |
|---|---|---|---|
| P1-1 | 退款写路由挂 Payment Compliance / Step-Up | ✅ 关闭 | routes_admin.go, route_structure_test.go, refund_compliance_test.go |
| P1-2 | 认证端点速率限制 + 账号枚举防护 | ✅ 关闭 | routes.go, router.go, routes_storefront.go, user_password_handler.go, user_verify_handler.go, p1_2_auth_rate_limit_test.go, router_composition_structure_test.go |

---

## P1-1 退款写路由 Payment Compliance / Step-Up

### 修复内容

**路由分组修正**：`internal/app/httpserver/routes_admin.go:162`
- 修改前：`ordertransport.RegisterAdminRefundWriteRoutes(authorized, ...)`（仅 JWT+RBAC）
- 修改后：`ordertransport.RegisterAdminRefundWriteRoutes(paymentProtected, ...)`（JWT+RBAC+PaymentComplianceRequired）

**受保护的 3 条退款写路由**：
| 方法 | 路径 | 触发 Wallet credit | 保护链 |
|---|---|---|---|
| POST | /admin/orders/:id/refund-to-wallet | 是 | JWT + RBAC + PaymentCompliance |
| POST | /admin/orders/:id/manual-refund | 是 | JWT + RBAC + PaymentCompliance |
| PATCH | /admin/order-refunds/:id/payment-fee | 否（手续费修正） | JWT + RBAC + PaymentCompliance |

### 所有触发 Wallet credit 的 Admin API 审计（全覆盖）

| API | 路径 | 挂载组 | 状态 |
|---|---|---|---|
| 退款到余额 | /admin/orders/:id/refund-to-wallet | paymentProtected | ✅ 本轮修复 |
| 手动退款 | /admin/orders/:id/manual-refund | paymentProtected | ✅ 本轮修复 |
| 退款手续费修正 | /admin/order-refunds/:id/payment-fee | paymentProtected | ✅ 本轮修复 |
| 售后退款（partial/full） | /admin/orders/:id/after-sale/action | paymentProtected | ✅ 已有 |
| 钱包调整 | /admin/users/:id/wallet/adjust 等 | paymentProtected | ✅ 已有 |
| 支付渠道增删改 | /admin/payment-channels* | paymentProtected | ✅ 已有 |
| 支付记录 | /admin/payments* | paymentProtected | ✅ 已有 |
| 对账 | /admin/reconciliation* | paymentProtected | ✅ 已有 |
| 佣金/提现 | /admin/affiliates/withdraws* | paymentProtected | ✅ 已有 |
| 分销财务 | /admin/reseller/*/finance* | paymentProtected | ✅ 已有 |

**结论：无遗漏。所有触发 Wallet credit 的 Admin API 均在 paymentProtected 组。**

### Step-Up 机制说明

当前财务写操作的 Step-Up 机制为 **Payment Compliance 合规声明**：
- `PaymentComplianceRequired` 中间件检查 `compliance.Service.IsAcknowledged()`
- 未确认时返回 403 `compliance_required`（超管）或 `compliance_required_by_super_admin`（非超管）
- 全仓无 per-request 2FA 中间件（搜索 StepUp/Require2FA/step_up 无命中）
- 与其他财务写操作（钱包调整、支付渠道、对账）使用完全一致的保护链

### 幂等性

- **HTTP Idempotency-Key**：当前无 HTTP 级幂等键中间件
- **实际防重机制**：退款服务层使用 `GetByIDForUpdate` 行锁 + refundable 金额上限校验，超额即拒
- **钱包 credit 幂等**：按 reference 幂等，但退款 reference 含纳秒时间戳使重试不命中
- **After-Sale 退款**：由事务+状态机保证幂等（既有测试 `TestFullRefundCallsOnceAndIdempotent` 验证）
- 列为 P2 backlog：引入显式 Idempotency-Key header

### Reason / Audit

- 退款请求 DTO 已有 `remark` 字段并落库到 OrderRefundRecord
- 退款记录无 `admin_id` 字段（P2 backlog）
- 退款操作无独立 auditlog 记录（P2 backlog，RBAC 中间件有结构化日志）

### 新增测试

`internal/app/httpserver/refund_compliance_test.go`（3 个测试）：
1. 未确认合规声明时，3 条退款写路由均返回 403 compliance_required
2. 合规已确认后，退款写路由放行
3. 重复退款不新增退款记录（服务层 refundable 校验）

---

## P1-2 认证端点速率限制 + 账号枚举防护

### 修复内容

**路由函数签名修改**：`internal/modules/identity/userauth/transport/http/routes.go`
- `RegisterUserVerifyAuthRoutes(auth, handler, rateLimit)` — 新增 rateLimit 参数
- `RegisterUserRegisterAuthRoutes(auth, handler, rateLimit)` — 新增 rateLimit 参数
- `RegisterUserPasswordAuthRoutes(auth, handler, rateLimit)` — 新增 rateLimit 参数
- 与已有 `RegisterUserLoginAuthRoutes` 模式完全一致，nil panic 保护

**新增限流规则**：`internal/app/httpserver/router.go`

| 规则 | Window | Max | Block | Prefix | 用途 |
|---|---|---|---|---|---|
| loginRule（已有） | 配置值 | 配置值 | 配置值 | `:rate:login` | 登录暴力破解 |
| registerRule（新增） | 600s (10min) | 5 | 900s (15min) | `:rate:register` | 注册批量 |
| verifyRule（新增） | 60s | 3 | 120s (2min) | `:rate:verify` | 验证码发送冷却 |
| forgotRule（新增） | 900s (15min) | 3 | 1800s (30min) | `:rate:forgot` | 找回密码冷却 |

四种场景使用不同阈值，不统一粗暴全局限制。均复用 `error.rate_limited` i18n key（三语言齐全）。

**限流挂载**：`internal/app/httpserver/routes_storefront.go`
- `POST /auth/send-verify-code` → `RateLimitMiddleware(redisClient, verifyRule, KeyByIPAndJSONField("email"))`
- `POST /auth/register` → `RateLimitMiddleware(redisClient, registerRule, KeyByIPAndJSONField("email"))`
- `POST /auth/forgot-password` → `RateLimitMiddleware(redisClient, forgotRule, KeyByIPAndJSONField("email"))`

**限流维度**：IP + email 组合（`KeyByIPAndJSONField("email")`），避免单 IP 批量撞邮箱、单邮箱被大量轰炸。Redis 优先，本地内存兜底（多实例需 Redis）。

### 账号枚举消除

| 端点 | 修改前 | 修改后 |
|---|---|---|
| `POST /auth/forgot-password` | 用户不存在返回 404 `user_not_found`，验证码错误返回 400 | 统一返回 400，不区分 |
| `POST /auth/send-verify-code`（purpose=reset） | 未注册邮箱可能暴露存在性 | 统一返回 `{sent:true}`，不区分 |
| `POST /auth/register` | 已存在邮箱返回 `email_exists` | 保留（自有邮箱主张的标准 UX，审计未列为枚举项） |

### 所有公开认证端点限流状态

| 端点 | 方法 | 路径 | 限流 | 规则 |
|---|---|---|---|---|
| 登录 | POST | /auth/login | ✅ | loginRule |
| 登录 2FA | POST | /auth/login/verify-2fa | ✅ | loginRule |
| 注册 | POST | /auth/register | ✅ | registerRule（本轮） |
| 发送验证码 | POST | /auth/send-verify-code | ✅ | verifyRule（本轮） |
| 忘记密码 | POST | /auth/forgot-password | ✅ | forgotRule（本轮） |
| Telegram 登录 | POST | /auth/telegram/login | ✅ | loginRule |
| Telegram MiniApp | POST | /auth/telegram/miniapp/login | ✅ | loginRule |
| Telegram OIDC | GET/POST | /auth/telegram/oidc/* | ✅ | loginRule |
| Google 登录 | POST | /auth/google/login | ✅ | loginRule |
| Google redirect intent | POST | /auth/google/redirect/intent | ✅ | loginRule |
| Google redirect callback | POST | /auth/google/redirect/callback | ❌ 有意不加 | 契约测试要求不消耗限流 |
| Google redirect exchange | POST | /auth/google/redirect/exchange | ❌ 有意不加 | 契约测试要求不消耗限流 |

**说明**：Google redirect callback/exchange 不加限流是有意设计——既有测试 `TestGoogleRedirectCallbackAndExchangeDoNotConsumeLoginAttemptRateLimit` 明确要求 OAuth 跳转链中仅 login+intent 消耗限流，共享 loginRule 会误杀同一次正常登录。列为 P2 backlog 评估是否需要独立规则。

### 新增测试

`internal/app/httpserver/p1_2_auth_rate_limit_test.go`（6 个测试）：
1. register 超频 → 429
2. verify send 超频 → 429
3. forgot-password 超频 → 429
4. 正常频率（1 次）不误伤
5. forgot-password 不存在邮箱与存在邮箱响应语义一致
6. send-verify-code reset 场景不泄露邮箱注册状态

---

## 全量回归结果

### 后端

| 检查 | 结果 |
|---|---|
| gofmt validation | ✅ PASS（1 个文件格式化后空输出） |
| go vet ./... | ✅ PASS（exit 0，0 warning） |
| go test ./... | ✅ 190 PASS + 175 no-test + 3 Windows flaky（0 真实失败） |
| go build ./... | ✅ PASS（exit 0） |
| Migration fresh install + 幂等 | ✅ PASS |
| httpserver（含新增 P1 测试） | ✅ PASS |
| userauth 全模块 | ✅ PASS |
| refund application | ✅ PASS |
| aftersale（含幂等测试） | ✅ PASS |
| middleware | ✅ PASS |
| RBAC 覆盖（252 路由） | ✅ PASS |

**Windows flaky（3 包，非真实失败，Linux CI 不受影响）**：
- `internal/logger`：TempDir 日志文件句柄致 cleanup 失败
- `internal/modules/order/infrastructure/gormstore` RiskGate：TempDir SQLite 文件锁
- `internal/selfupdate`：Unix 权限/文件锁语义

### 前端

| 检查 | 结果 |
|---|---|
| User vue-tsc --noEmit | ✅ PASS（0 错误） |
| User npm run build | ✅ PASS（exit 0） |
| Admin vue-tsc --noEmit | ✅ PASS（0 错误） |
| Admin npm run build | ✅ PASS（exit 0） |

---

## Linux CI 真实验证

- **Push**：`d0a4f73..11bdc6c main -> main`（exit 0）
- **CI Run #37184824780**（head_sha=11bdc6c）：status=completed, conclusion=**success**

| Job | 结果 |
|---|---|
| Verify installer | ✅ success |
| Verify API（gofmt + vet + test + build） | ✅ success |
| Verify release config | ✅ success |
| Verify fullstack build | ✅ success |

---

## 修复过程中发现的额外问题（非本轮范围，已记录）

1. **AdminManualRefundOrder 错误映射不匹配**（P2）：handler 使用包内本地哨兵 `ErrWalletRefundExceeded`，服务层返回 `walletcontract.ErrRefundExceeded`，二者非同一 error，超额退款可能落 500 而非 400。未改动既有冻结语义，建议后续修复。
2. **退款记录无 admin_id 字段**（P2）：建议添加以增强审计追踪。
3. **退款操作无独立 auditlog**（P2）：建议接入 auditlog 模块。
4. **无 HTTP 级 Idempotency-Key**（P2）：当前依赖行锁+金额上限，建议引入显式幂等键。
5. **Google redirect callback/exchange 无限流**（P2）：有意设计（契约测试），建议评估独立规则。

---

## 当前是否允许进入 Production Go-Live Final Gate

**是。** 本轮 2 个生产前安全 P1 已全部关闭：
- 退款写入口全部受 Payment Compliance / Step-Up 保护（与其他财务写操作一致）
- 注册/找回密码/发送验证码均有差异化速率限制（IP+email 组合，429 不泄露账号存在性）
- 全量回归通过（0 真实失败）
- Linux CI 真实全绿（4/4 jobs）

结合上一轮 9 个 P0 全部关闭（commit d0a4f73，CI #37154077156 全绿），HCZ 已具备进入 Production Go-Live Final Gate 的条件。剩余 13 个 P1（旧 9 态 UI 迁移、结账分账模型、风控查询口径等）和 20+ 个 P2 backlog 不构成生产阻断，可部署后分迭代修复。

---

## 修复产物索引

| 报告 | 路径 | 内容 |
|---|---|---|
| 本报告（最终交付） | HCZ_PRODUCTION_SECURITY_P1_FINAL.md | 2 P1 关闭确认 + CI 结果 + Go-Live 建议 |
| P1-1 退款合规修复 | REMEDIATION_P1_1_REFUND_COMPLIANCE.md | 路由修正 + 审计 + 测试详情 |
| P1-2 认证限流修复 | REMEDIATION_P1_2_AUTH_RATE_LIMIT.md | 限流规则 + 枚举防护 + 测试详情 |
| 基础设施回归 | REMEDIATION_P1_INFRASTRUCTURE.md | gofmt + 全量回归 + commit/push |
