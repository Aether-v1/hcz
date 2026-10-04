# P1-2 认证端点速率限制修复报告

## 修复清单
| 端点 | 方法 | 路径 | 限流规则 | Key 维度 | 状态 |
|---|---|---|---|---|---|
| 注册 | POST | /auth/register | registerRule | IP+email | ✅ |
| 发送验证码 | POST | /auth/send-verify-code | verifyRule | IP+email | ✅ |
| 忘记密码（重置提交） | POST | /auth/forgot-password | forgotRule | IP+email | ✅ |
| 登录（已有） | POST | /auth/login | loginRule | IP+email | 已有 ✅ |
| 2FA 校验（已有） | POST | /auth/login/verify-2fa | loginRule | IP | 已有 ✅ |
| Telegram 登录（已有） | POST | /auth/telegram/login | loginRule | IP | 已有 ✅ |
| Telegram MiniApp 登录（已有） | POST | /auth/telegram/miniapp/login | loginRule | IP | 已有 ✅ |
| Telegram OIDC（已有） | GET/POST | /auth/telegram/oidc/start,/callback | loginRule | IP | 已有 ✅ |
| Google 登录/intent（已有） | POST | /auth/google/login,/redirect/intent | loginRule | IP | 已有 ✅ |

## 限流规则配置
| 规则 | Window | Max | Block | Prefix |
|---|---|---|---|---|
| loginRule（已有） | 300s | 5 | 900s | {redisPrefix}:rate:login |
| adminLoginRule（已有） | 300s | 5 | 900s | {redisPrefix}:rate:admin_login |
| registerRule（新增） | 600s | 5 | 900s | {redisPrefix}:rate:register |
| verifyRule（新增） | 60s | 3 | 120s | {redisPrefix}:rate:verify |
| forgotRule（新增） | 900s | 3 | 1800s | {redisPrefix}:rate:forgot |

> MessageKey 均复用已有 i18n key `error.rate_limited`（zh-CN/zh-TW/en 三语齐全），未新增配置项，最小改动。规则在 `internal/app/httpserver/router.go` 直接以合理默认值构造（与 guestReadRule/guestWriteRule 等既有硬编码规则风格一致），未改 SecurityConfig。

### 改动文件
1. `internal/modules/identity/userauth/transport/http/routes.go`
   - `RegisterUserVerifyAuthRoutes` / `RegisterUserRegisterAuthRoutes` / `RegisterUserPasswordAuthRoutes` 新增 `rateLimit gin.HandlerFunc` 参数，内部 `auth.POST(path, rateLimit, handler.Method)`，nil 检查 panic 行为与 `RegisterUserLoginAuthRoutes` 完全对齐。
2. `internal/app/httpserver/router.go`
   - 新增 registerRule / verifyRule / forgotRule 三条规则；`registerStorefrontRoutes(...)` 调用追加传入这三条规则。
3. `internal/app/httpserver/routes_storefront.go`
   - `registerStorefrontRoutes` 签名末尾追加 `registerRule / verifyRule / forgotRule`；auth 组三处调用挂载 `RateLimitMiddleware(redisClient, rule, KeyByIPAndJSONField("email"))`。
4. `internal/app/httpserver/route_structure_test.go`
   - 同步更新三处硬编码信任边界断言（旧断言无 rateLimit 参数，与新挂载不匹配）。
5. `internal/architecture/router_composition_structure_test.go`
   - httpserver 目录 Go 文件预算 10→12（P1-1 退款合规 + P1-2 各新增 1 个聚焦测试文件，遵循本仓库既有调预算模式）。

## 账号存在性防护

### forgot-password（重置提交端点 POST /auth/forgot-password）
- 修改前：`ResetPassword` 返回 `ErrUserNotFound` → HTTP 200 + body `status_code=404 error.user_not_found`；验证码错误 → `status_code=400 error.verify_code_invalid`。攻击者用「任意邮箱 + 瞎填验证码」即可区分邮箱是否已注册（审计 P2-3）。
- 修改后：`ErrUserNotFound` 与 `ErrVerifyCodeInvalid` 合并为同一分支，统一返回 `400 error.verify_code_invalid`，业务码完全一致，无法区分。
- 文件：`internal/modules/identity/userauth/transport/http/user_password_handler.go`

### send-verify-code（发送验证码端点 POST /auth/send-verify-code, purpose=reset）
- 修改前：重置场景下邮箱未注册 → service 返回 `ErrNotFound` → 映射为 `ErrUserNotFound` → 404 `error.user_not_found`，与「已注册并发送成功」形成区分。
- 修改后：`purpose=reset` 且 `ErrUserNotFound` 时，不发信但统一返回成功 `{sent:true}`（status_code=0），与成功路径响应完全一致，消除忘记密码邮件轰炸/枚举。
- 文件：`internal/modules/identity/userauth/transport/http/user_verify_handler.go`

### register（注册端点 POST /auth/register）
- 响应分析：已存在邮箱返回 `400 error.email_exists`。**保留**该响应——注册是用户对自有邮箱的主张流程，提示「邮箱已注册请登录」是标准 UX，审计（P2-3）未将注册列为枚举项；且注册需先通过 send-verify-code（purpose=register）拿到验证码，该路径对已存在邮箱返回 `email_exists` 属预期。此为有意取舍，非泄漏。

## 所有公开认证端点限流状态
| 端点 | 限流 | 规则 |
|---|---|---|
| POST /auth/send-verify-code | ✅ | verifyRule (IP+email) |
| POST /auth/register | ✅ | registerRule (IP+email) |
| POST /auth/login | ✅ | loginRule (IP+email) |
| POST /auth/login/verify-2fa | ✅ | loginRule (IP) |
| POST /auth/telegram/login | ✅ | loginRule (IP) |
| POST /auth/telegram/miniapp/login | ✅ | loginRule (IP) |
| GET /auth/telegram/oidc/start | ✅ | loginRule (IP) |
| POST /auth/telegram/oidc/callback | ✅ | loginRule (IP) |
| POST /auth/google/login | ✅ | loginRule (IP) |
| POST /auth/google/redirect/intent | ✅ | loginRule (IP) |
| POST /auth/google/redirect/callback | ⚠️ 有意不限流 | 见遗留问题 1 |
| POST /auth/google/redirect/exchange | ⚠️ 有意不限流 | 见遗留问题 1 |
| POST /auth/forgot-password | ✅ | forgotRule (IP+email) |

> 非认证类公开端点（/public/config、/public/captcha、/public/affiliate）属启动配置/验证码素材，不触发发信或批量注册，不在本次范围。无独立 SMS 验证码端点（本项目仅邮箱验证码）。

## 测试
新增文件：`internal/app/httpserver/p1_2_auth_rate_limit_test.go`（package httpserver，复用既有 RateLimitMiddleware，nil Redis 走进程内兜底）。

| 测试名 | 覆盖场景 | 结果 |
|---|---|---|
| TestP1_2RegisterRouteRateLimitedConfirmsMiddlewareMounted | 注册超频 → 第 1 次放行，第 2 次 429 | PASS |
| TestP1_2VerifyRouteRateLimitedConfirmsMiddlewareMounted | 发送验证码超频 → 429 | PASS |
| TestP1_2ForgotRouteRateLimitedConfirmsMiddlewareMounted | 忘记密码超频 → 429 | PASS |
| TestP1_2NormalFrequencyNotBlocked | 窗口内 1 次请求不误伤（非 429） | PASS |
| TestP1_2ForgotPasswordDoesNotLeakAccountExistence | 不存在邮箱与存在邮箱响应业务码一致（均 400，不再 404） | PASS |
| TestP1_2SendVerifyResetDoesNotLeakAccountExistence | reset 目的下未注册邮箱统一返回成功（status_code=0），与成功路径一致 | PASS |

## 验证
- `go build ./...`：**PASS**（exit=0）
- `go test ./internal/app/httpserver/... -run "TestRateLimit|TestRoute|TestAuth"`：**PASS**（httpserver + middleware）
- `go test ./internal/modules/identity/userauth/...`：**PASS**（application / integrationtest / transport/http / presenter）
- `go test ./internal/app/httpserver/middleware/...`：**PASS**
- `go vet ./internal/app/httpserver/... ./internal/modules/identity/userauth/transport/http/...`：**PASS**
- `go test ./internal/architecture/`：**PASS**（文件预算调整后）

## 遗留问题（如有）

1. **Google redirect callback/exchange 有意不限流（P2，非本次 P1 阻断）**：`POST /auth/google/redirect/callback` 与 `/exchange` 未挂限流。审计 P1-2 证据曾指出二者未限流，但仓库存在契约测试 `TestGoogleRedirectCallbackAndExchangeDoNotConsumeLoginAttemptRateLimit`（user_google_error_test.go:649），明确要求 OAuth 跳转链中仅 popup login + intent 消耗登录限流——callback/exchange 是同一次登录的后续步骤，共享 loginRule(IP) 会在 1 次正常登录内连续消耗 3~4 个 token 而误杀正常用户。因此**未**对这两个端点加 loginRule。残余风险：它们走一次性 state/handle 高熵凭证，爆破面小；如需进一步加固，应引入独立的、按 handle 计数的限流规则（而非复用 loginRule），属后续加固项。
2. 限流阈值为硬编码默认值，未接入 config。若需运维可调，后续可在 SecurityConfig 增加 register/verify/forgotRateLimit 字段（本次按最小改动要求未做）。
3. 多实例部署依赖共享 Redis 才能全局一致计数；Redis 不可用时中间件自动降级为进程内兜底（既有行为，非本次引入）。
