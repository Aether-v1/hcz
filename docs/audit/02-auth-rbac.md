# 02 — 认证 / RBAC / 中间件安全审计

- 项目：github.com/Aether-v1/hcz（Go + Gin + GORM）
- 审计日期：2026-10-07
- 审计方式：只读静态审计（未修改任何源码），所有结论附文件 + 行号证据
- 范围：identity(userauth/adminauth/invitation/user/adminauthorization)、authz(Casbin)、apicredential、httpserver/middleware、platform/http/stepup、captcha、bootstrap/*、crypto、配置文件

> 总览：JWT 实现本身较规范（HS256 白名单、TokenVersion 撤销、冻结即拒），RBAC 模型为默认拒绝且角色矩阵经过设计；但存在 **2 个 P0 默认配置级致命问题**（硬编码默认 JWT 密钥 + 默认 admin/admin123）、**1 个 P1 越权**（system_admin 可任命超管）、以及若干 P2 纵深防御缺口。

---

### ISSUE-A01
- Severity: **P0**
- Title: 硬编码默认 JWT 签名密钥，配置缺失时无启动校验，可伪造任意 admin/user 令牌
- File: internal/config/config.go
- Function/Method: Load()
- API: 任意需认证端点（含 /api/v1/admin/**）
- Table: admins / users（通过伪造 token 直接命中）
- Root Cause: viper 给 jwt.secret / user_jwt.secret / app.secret_key 设置了公开的、写在源码里的默认值；当 config.yml 缺失或未配置这些键时，ReadInConfig 失败仅打 Warn（line 437-441），进程照常启动并使用这些公开密钥签发/校验 token，且全程没有任何“生产环境拒绝默认密钥”的校验。
- Exploit/Trigger:
  1. 攻击者知道源码中的默认密钥 `change-me-in-production`（admin JWT）。
  2. 用该密钥 HS256 自签一个 `admin_id=1, typ=access, tv=<0 或猜测值>, iat=now` 的 JWT。
  3. 带上 `Authorization: Bearer <forged>` 访问 /api/v1/admin/**。中间件只验签名（默认密钥已知 = 可伪造），随后从 DB 取 admin id=1（首启即 IsSuper=true）。TokenVersion 为“相等比较”（middleware.go:224），全新部署 TokenVersion=0，伪造 `tv=0` 即通过；`TokenInvalidBefore` 为空直接放行。
  4. 直接获得超管会话，绕过登录/2FA/step-up。
- Impact: 未改配置即上线的实例 = 任何人可远程获得超级管理员权限，完全失守（资金、用户、密钥全暴露）。
- Evidence:
  ```go
  // internal/config/config.go:315,333-335
  viper.SetDefault("app.secret_key", "change-me-32-byte-secret-key!!")
  viper.SetDefault("jwt.secret", "change-me-in-production")
  viper.SetDefault("user_jwt.secret", "user-change-me-in-production")
  ...
  // config.go:437-441 —— 配置文件缺失仅告警，不退出
  if err := viper.ReadInConfig(); err != nil {
      logger.Warnw("config_file_read_failed", "error", err, "fallback", "env_or_defaults")
  }
  // internal/app/httpserver/middleware/middleware.go:185-189 —— 仅用配置密钥验签
  parser := newHS256JWTParser()
  token, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
      return []byte(secretKey), nil   // secretKey 若=源码默认值，等于公开
  })
  // middleware.go:436-438 —— 算法白名单正确（仅 HS256），但挡不住“密钥本身已知”
  func newHS256JWTParser() *jwt.Parser {
      return jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
  }
  ```
- Recommended Fix:
  1. 生产模式（server.mode=release）下启动时强制校验：jwt.secret / user_jwt.secret / app.secret_key 不得为空、不得等于已知默认值、长度 ≥ 32 字节，否则 panic 拒绝启动。
  2. 把 SetDefault 里的敏感密钥默认值删掉（给空值 + 启动校验），不要给“可直接用”的默认。
  3. config.yml.example 注释与实际行为对齐（注释声称“否则拒绝启动”，实际并未拒绝）。

### ISSUE-A02
- Severity: **P0**
- Title: 首启默认超级管理员硬编码凭据 admin / admin123
- File: internal/modules/identity/admin/application/bootstrap.go
- Function/Method: InitDefaultAdmin
- API: POST /api/v1/admin/login（+ 2FA 若未绑定）
- Table: admins
- Root Cause: 当 bootstrap.default_admin_password 未配置时，首启创建的第一名管理员使用写死的口令 `admin123`，且 IsSuper=true。
- Exploit/Trigger: 未显式配置 `bootstrap.default_admin_password` 即上线的实例，攻击者用 `admin / admin123` 直接登录后台；若该管理员未绑 2FA 则一步到位；即使绑了 2FA，配合 A01 的伪造 token 亦可绕过。
- Impact: 公开默认超管口令，批量扫站可直接接管。
- Evidence:
  ```go
  // bootstrap.go:13-14
  const defaultBootstrapUsername = "admin"
  const defaultBootstrapPassword = "admin123"
  // bootstrap.go:37-48
  if password == "" {
      password = defaultBootstrapPassword   // 未配置即用公开口令
  }
  ...
  admin := &admindomain.Admin{Username: bootstrapUsername, PasswordHash: string(hash), IsSuper: true}
  ```
- Recommended Fix:
  1. 首启不允许弱默认口令：未配置 bootstrap 口令时，随机生成一次性强密码并仅打印一次（或强制走安装向导）。
  2. 登录后强制改密流程（首次登录强制修改默认口令）。
  3. 部署文档把“必须配置 bootstrap.default_admin_password”列为硬性上线检查项。

### ISSUE-A03
- Severity: **P1**
- Title: system_admin 角色可自行任命/提拔超级管理员（垂直越权）
- File: internal/modules/identity/adminauthorization/transport/http/admin_account_handler.go
- Function/Method: CreateAuthzAdmin / UpdateAuthzAdmin
- API: POST /api/v1/admin/authz/admins ; PUT /api/v1/admin/authz/admins/:id
- Table: admins
- Root Cause: 创建/更新管理员时直接采信请求体里的 `is_super`，没有像 ResetTargetAdmin2FA 那样二次校验调用方是否本身为 IsSuper。而 `/admin/authz/admins*` 策略在种子里授予了 `system_admin` 角色（非超管）。
- Exploit/Trigger: 任何持有 system_admin 角色的管理员（非超管）：
  1. PUT /admin/authz/admins/{自己的id}  body `{"is_super": true}` → 自己变成超管（middleware.go:212/234 缓存随即生效）。
  2. 或 POST /admin/authz/admins 创建一个 `is_super:true` 的新超管后门账号。
  3. 超管身份直接绕过全部 Casbin（middleware.go:250-255）。
- Impact: system_admin 本应是“管 RBAC 的系统管理员”，但可自我提权为超管，突破了 RBAC 分层设计；一旦 system_admin 账号被盗即全量失守。
- Evidence:
  ```go
  // admin_account_handler.go:78-87（CreateAuthzAdmin）
  isSuper := req.IsSuper != nil && *req.IsSuper
  if strings.EqualFold(username, protectedSuperAdminUsername) { isSuper = true }
  admin := &admindomain.Admin{Username: username, PasswordHash: hash, IsSuper: isSuper} // 直接采信请求体
  // admin_account_handler.go:165-174（UpdateAuthzAdmin）
  if req.IsSuper != nil {
      nextIsSuper := *req.IsSuper
      ...
      admin.IsSuper = nextIsSuper   // 无 IsSuperAdmin() 二次校验
  }
  // internal/authz/bootstrap.go:320-321 —— 该能力仅授予 system_admin（非超管角色）
  {Object: "/admin/authz/admins", Action: "*"},
  {Object: "/admin/authz/admins/:id", Action: "*"},
  // 对比：line 323 明确注释 reset 2fa 仍需二次校验
  {Object: "/admin/authz/admins/:id/2fa/reset", Action: "POST"}, // handler 仍二次校验 isSuper
  ```
- Recommended Fix:
  1. Create/UpdateAuthzAdmin 中设置 is_super 前必须 `ginutil.IsSuperAdmin(c)` 为真，否则忽略/拒绝。
  2. 把“任命超管”动作收归超管专属（RBAC 层不向任何内置角色开放，仅 IsSuper 放行）。
  3. 对 is_super 变更单独记审计并告警。

### ISSUE-A04
- Severity: **P1**
- Title: Step-Up 端点无速率限制、无失败计数，可在线暴力破解 6 位 TOTP
- File: internal/modules/identity/adminauth/transport/http/admin_2fa_handler.go（StepUp）；路由 internal/modules/identity/adminauth/transport/http/routes.go:35
- Function/Method: StepUp
- API: POST /api/v1/admin/auth/step-up
- Table: admins（TOTPSecret）
- Root Cause: StepUp 直接调用 `totp.VerifyChallengeCode`，既没有登录 2FA 那条链路的 `BumpFails/5 次吊销 challenge`，路由也未挂 RateLimitMiddleware。TOTP 为 6 位数字、skew=1（前后各放宽 1 个 30s 窗口）。
- Exploit/Trigger: 攻击者已取得一个超管/财务的 access token（如 XSS、内鬼、终端失陷），但高风险操作（钱包调账/提现审批/退款）要求 step-up 第二因子。此时对 /auth/step-up 无限速地枚举 6 位 TOTP 即可绕过第二因子。
- Impact: 2FA/step-up 这道“第二道锁”可被在线爆破，削弱资金操作的二次验证强度。
- Evidence:
  ```go
  // adminauth/transport/http/routes.go:35 —— 无 RateLimit 包裹
  authorized.POST("/auth/step-up", handler.StepUp)
  // admin_2fa_handler.go StepUp：失败仅 401，未见 BumpFails / challenge 吊销计数
  if err := h.totp.VerifyChallengeCode(adminID, code); err != nil { ... 401 ... }
  // 对比：登录 2FA Verify2FA 才有 challengeMaxFailures=5 的 BumpFails/Revoke
  ```
- Recommended Fix:
  1. 给 /admin/auth/step-up 挂 RateLimitMiddleware（KeyByAdminID，比如 10 次/5 分钟）。
  2. 在 StepUp 内引入与登录一致的失败计数：连续失败 N 次吊销 challenge 并要求重新走 step-up 签发。
  3. TOTP 校验增加“该 challenge 周期内失败即递增冷却”。

### ISSUE-A05
- Severity: **P2**
- Title: CORS 默认配置 `allowed_origins: ["*"]` 且 `allow_credentials: true`，回显任意 Origin
- File: internal/app/httpserver/middleware/middleware.go:80-101；internal/config/config.go:379-383；config.yml.example:113-132
- Function/Method: resolveAllowedOrigin
- API: 全部 API（OPTIONS 预检 + 实际响应头）
- Table: 无
- Root Cause: 当 allowedOrigins 含 `*` 且 allowCredentials=true 时，中间件把请求方 Origin 原样回显为 Access-Control-Allow-Origin，并同时下发 Access-Control-Allow-Credentials: true。默认配置（config.go:382）allow_credentials 即为 true。
- Exploit/Trigger: 任意恶意站点可在用户已登录状态下跨域发起带凭证请求并读取响应。当前认证走 Authorization: Bearer（localStorage），浏览器不会自动携带，故实际可利用性下降；但系统存在 OAuth/支付回调等 Cookie 链路，且一旦某处改用 Cookie 会话即立刻变成完整 CSRF/跨域读。
- Impact: 跨域策略过宽，是潜在 CSRF/数据读取面；默认值不安全。
- Evidence:
  ```go
  // middleware.go:84-90
  for _, allowed := range allowedOrigins {
      if allowed == "*" {
          if allowCredentials && origin != "" { return origin } // 回显任意 Origin
          return "*"
      }
  }
  // middleware.go:62-63
  if cfg.AllowCredentials { c.Writer.Header().Set("Access-Control-Allow-Credentials", "true") }
  // config.go:382  viper.SetDefault("cors.allow_credentials", true)
  // config.yml.example:114-115  allowed_origins: ["*"]
  ```
- Recommended Fix:
  1. 默认不允许 `*` + credentials 组合；生产必须显式列出可信 Origin 白名单。
  2. allow_credentials=true 时拒绝 `*`，强制精确 Origin 匹配。
  3. 若无 Cookie 会话，考虑把 allow_credentials 默认关小或改为精确白名单。

### ISSUE-A06
- Severity: **P2**
- Title: 登录接口通过差异化错误码造成用户枚举
- File: internal/modules/identity/userauth/transport/http/user_login_handler.go；internal/modules/identity/userauth/application/service.go:LoginStep1
- Function/Method: Login / LoginStep1
- API: POST /api/v1/auth/login
- Table: users
- Root Cause: 服务层对“账号不存在”“账号未验证”“账号被冻结”“密码错”返回不同错误，handler 映射为不同文案，攻击者可据此区分邮箱是否注册、账号当前状态。
- Exploit/Trigger: 用脚本对登录接口批量探测邮箱：返回 email_not_verified / user_disabled 即证明该邮箱已注册；返回 login_invalid 则为未注册或密码错。可结合撞库/钓鱼。
- Impact: 用户枚举，辅助撞库与定向钓鱼。
- Evidence:
  ```go
  // service.go LoginStep1 分支：
  //   user==nil            -> ErrInvalidCredentials   (login_invalid)
  //   status != active     -> ErrUserDisabled         (user_disabled)
  //   !EmailVerified       -> ErrEmailNotVerified    (email_not_verified)
  //   bcrypt 不符          -> ErrInvalidCredentials
  // user_login_handler.go 据此返回 error.email_not_verified / error.user_disabled / error.login_invalid
  ```
- Recommended Fix:
  1. 对外统一返回“邮箱或密码错误”，把 disabled/unverified 仅在登录成功后按需提示（或仅在验证码链路区分）。
  2. 保留内部区分用于日志，不映射到不同对外文案。

### ISSUE-A07
- Severity: **P2**
- Title: 登录限流键为 email|IP，单 IP 可对大量账号做密码喷洒
- File: internal/app/httpserver/middleware/rate_limit.go:271-277；router 中 loginKey = KeyByIPAndJSONField("email")
- Function/Method: KeyByIPAndJSONField
- API: POST /api/v1/auth/login
- Table: users
- Root Cause: 登录限流桶键 = `邮箱|IP`。同一 IP 尝试 N 个不同邮箱时，每个邮箱独立成桶，互不影响，单机密码喷洒不受总次数约束。
- Exploit/Trigger: 攻击者从一个 IP 对 10 万个邮箱各试 1-2 个常见密码，每个 (email,IP) 桶都未触发 5 次上限，完成大规模喷洒。
- Impact: 弱密码账号批量被撞。
- Evidence:
  ```go
  // rate_limit.go:271-277
  func KeyByIPAndJSONField(field string) RateLimitKeyFunc {
      ... return fmt.Sprintf("%s|%s", value, c.ClientIP())  // email|IP，每邮箱独立桶
  }
  ```
- Recommended Fix:
  1. 增加纯 IP 维度的全局登录次数上限（如 1 分钟 60 次）作为第二道闸。
  2. 服务层增加按账号的失败锁定（N 次失败锁 15 分钟），与 IP 限流互补。

### ISSUE-A08
- Severity: **P2**
- Title: C2C 仲裁的 Step-Up 校验弱于平台标准（不绑 scope、不消费 challenge、可重放）
- File: internal/modules/c2c/transport/http/admin_handler.go:~253；internal/bootstrap/c2c/wiring.go
- Function/Method: Arbitrate（C2C admin）
- API: POST /api/v1/admin/c2c/orders/:id/arbitrate（推测路径）
- Table: c2c_orders
- Root Cause: 平台 stepup.Verifier 会校验 scope 绑定并用 SETNX 单次消费 challenge（bootstrap/stepup/adapter.go ConsumeChallenge fail-closed）；但 C2C 仲裁改用 `adminChallengeVerifier.ParseChallengeToken`，只校验 token 里的 admin_id 与 scope 无关，且从不调用 ConsumeChallenge。
- Exploit/Trigger:
  1. challenge 有效期 5 分钟（challenge/challenge.go ChallengeTTL），期间同一 challenge token 可多次用于不同仲裁。
  2. 为别的 scope（如 wallet.adjust）签发的合法 challenge，也能用于 C2C 仲裁。
- Impact: step-up 在 C2C 仲裁场景失去“单次 + 绑定操作”的强度，纵深防御下降（前提是已持有登录态，故定 P2）。
- Evidence:
  ```go
  // bootstrap/c2c/wiring.go: c2c 使用 adminChallengeVerifier.ParseChallengeToken（仅解 token），
  // 未接入 stepup.Verifier / ConsumeChallenge
  // bootstrap/stepup/adapter.go: ConsumeChallenge 使用 SETNX，失败即拒绝（fail-closed）——C2C 未走这条路径
  ```
- Recommended Fix:
  1. C2C 仲裁改用统一 stepup.Verifier，显式声明 scope（如 "c2c.arbitrate"）并强制消费 challenge。

### ISSUE-A09
- Severity: **P2**
- Title: 无服务端登出，令牌在有效期内（admin 24h / user 24-168h）被盗后无法吊销
- File: internal/modules/identity/adminauth/transport/http/routes.go；userauth 路由
- Function/Method: （无 logout 端点）
- API: 无 POST /auth/logout
- Table: admins / users（TokenVersion）
- Root Cause: 没有 logout 接口，也无令牌黑名单。吊销只能依赖 TokenVersion++（仅改密/重置密码/管理员冻结用户时触发）。退出登录仅前端删 token。
- Exploit/Trigger: XSS/机器失陷拿到 token 后，用户即使“退出登录”也无法使其失效；被盗 token 最长可用 7 天（remember_me）。
- Impact: 令牌失窃后撤销不及时。
- Evidence:
  ```go
  // adminauth/transport/http/routes.go 全文：仅 login / login/verify-2fa / step-up / 2fa/*，无 logout
  // 中间件仅在 claims.TokenVersion != DB.TokenVersion 时拒绝（middleware.go:224,377,402），
  // 日常退出不 bump TokenVersion。
  ```
- Recommended Fix:
  1. 增加 logout 端点：服务端把当前 jti 写入短期黑名单（或 bump TokenVersion），使 token 立即失效。
  2. 缩短 access token 有效期并引入 refresh token + 可撤销 refresh 轮换。

### ISSUE-A10
- Severity: **P2**
- Title: API 凭证 secret 明文落库
- File: internal/modules/apicredential/application/service.go:112；domain/credential.go:14
- Function/Method: Approve / Regenerate
- API: POST /api/v1/admin/api-credentials/:id/approve（secret 一次性返回）
- Table: api_credentials
- Root Cause: api_secret 以明文存入 DB（HMAC 校验需要可逆明文）。API 响应 json:"-" 不回显（良好），但 DB 一旦泄露（备份拖库/SQLi），全部对接方密钥直接暴露。
- Exploit/Trigger: DB 泄露 → 攻击者直接用明文 api_secret 以对端身份调上游/渠道接口。
- Impact: 数据库泄露放大为 API 身份冒用。
- Evidence:
  ```go
  // service.go:111-112
  cred.ApiKey = apiKey; cred.ApiSecret = apiSecret   // 明文持久化
  // domain/credential.go:14  ApiSecret string `... json:"-"`  // 不回显，但不加密存储
  ```
- Recommended Fix: 与 channel/upstream secret 统一加密存储（复用 crypto.Encrypt AES-256-GCM，用 app.secret_key 派生），使用时解密。

### ISSUE-A11
- Severity: **P3**
- Title: TOTP 未做“用过即废”的 code 级防重放（仅靠 challenge jti 单次消费）
- File: internal/modules/identity/adminauth/totp/application/service.go（Skew=1）
- Function/Method: VerifyChallengeCode
- API: 所有 TOTP 校验点
- Table: admins
- Root Cause: pquerna TOTP skew=1（±30s），同一 6 位码在约 90s 窗口内有效；本身不记录已用 code。登录/2FA 靠 challenge jti 单次消费防重放（良好），但 step-up（A04）与 C2C（A08）弱化了这层保护。
- Impact: 窗口内 TOTP 重放可能性存在，但被 challenge 单次消费部分缓解。
- Evidence:
  ```go
  // totp/application/service.go：ValidateCustom(skew=1)，无 used-code 存储
  ```
- Recommended Fix: 在 challenge 消费层统一单次化（见 A04/A08）；如需更严可记录最近用过的 code 哈希。

### ISSUE-A12
- Severity: **P3**
- Title: 默认 server.mode=debug，且默认开启 CORS credentials；captcha 默认全关
- File: internal/config/config.go:319,407-412
- Function/Method: Load
- API: 全局
- Table: 无
- Root Cause: 默认值 server.mode=debug（Gin debug 模式），captcha.provider=none 且 login/register/reset 场景默认 false，依赖限流兜底。
- Impact: 部署者忘记改 mode 时 debug 输出更详细；登录无验证码时撞库面更大（有 A07 限流部分缓解）。
- Evidence:
  ```go
  // config.go:319  viper.SetDefault("server.mode", "debug")
  // config.go:407-412 captcha.provider="none"; scenes.login=false ...
  ```
- Recommended Fix: release 构建强制 mode=release；登录/注册默认开启 turnstile 或图形验证码。

### ISSUE-A13
- Severity: **P3**
- Title: 管理员改用户邮箱不吊销该用户现有 token
- File: internal/modules/identity/user/transport/http/admin/admin_handler.go:402-421,487-490
- Function/Method: UpdateAdminUser
- API: PUT /api/v1/admin/users/:id
- Table: users
- Root Cause: 仅在改密码或禁用时 revokeToken=true；改邮箱（user.Email）不触发 TokenVersion++。JWT 里带 email claim 但中间件仅以 user_id 鉴权，影响有限。
- Impact: 改邮箱后旧 token 仍有效至过期（邮箱 claim 与 DB 不一致）。
- Evidence:
  ```go
  // admin_handler.go:417-419 改 email 不置 revokeToken；487-490 仅 revokeToken 时才 bump
  ```
- Recommended Fix: 改邮箱属于敏感变更，一并 bump TokenVersion。

### ISSUE-A14
- Severity: **P3**
- Title: 用户端提现创建接口无 HTTP 层速率限制
- File: internal/app/httpserver/routes_storefront.go（withdrawalhttp.RegisterUserRoutes(user, ...) 无 RateLimit 包裹）
- Function/Method: 提现创建
- API: POST /api/v1/user/wallet/withdrawals（推测）
- Table: wallet_withdrawals
- Root Cause: 下单有 orderrisk 业务级限流，但提现创建路由未挂 HTTP 限流；依赖业务层余额/风控校验。
- Impact: 持票会话可高频刷提现申请；NOT VERIFIED 业务层是否已有防重/频控。
- Evidence:
  ```go
  // routes_storefront.go: withdrawalhttp.RegisterUserRoutes(user, userWithdrawalHandler) —— 未包 RateLimitMiddleware
  ```
- Recommended Fix: 确认提现 Service 层频控；缺则补 KeyByUserID 限流。

---

## Authorization Matrix（重点 admin 端点）

| Endpoint | Auth | Ownership | RBAC | Step-Up | Result |
|---|---|---|---|---|---|
| POST /admin/login | 否（公网） | - | - | 否 | 限流+可选验证码，区分失败原因 OK |
| POST /admin/login/verify-2fa | challenge token | - | - | challenge 单次消费 | 5 次失败吊销 challenge |
| POST /admin/auth/step-up | Admin JWT | - | 无策略命中(仅超管可用) | 是(TOTP) | **无限流/失败计数 (A04)** |
| PUT /admin/password | Admin JWT | 本人 | readonly_auditor 起 | 否 | 改密 bump TokenVersion，旧 token 全失效 |
| POST /admin/authz/admins | Admin JWT | - | system_admin | 否 | **可指定 is_super (A03)** |
| PUT /admin/authz/admins/:id | Admin JWT | - | system_admin | 否 | **可自提为超管 (A03)** |
| PUT /admin/authz/admins/:id/roles | Admin JWT | - | system_admin | 否 | 改角色，不碰 IsSuper |
| POST /admin/authz/policies | Admin JWT | - | system_admin | 否 | 仅能改非内置角色策略 |
| GET /admin/authz/permissions/catalog | Admin JWT | - | system_admin | 否 | - |
| POST /admin/users/:id/wallet/adjust | Admin JWT | - | finance | 是(scope wallet.adjust) | 资金操作有 step-up |
| POST /admin/wallet/withdrawals/:id/approve | Admin JWT | - | finance | 是 | 有 step-up |
| POST /admin/wallet/withdrawals/:id/complete | Admin JWT | - | finance | 是 | 有 step-up |
| POST /admin/orders/:id/manual-refund | Admin JWT | - | finance | 是 | 有 step-up |
| POST /admin/c2c/orders/:id/arbitrate | Admin JWT | - | (authorized) | 弱(无scope/不消费) | **(A08)** |
| PUT /admin/users/:id | Admin JWT | - | support | 否 | 可改任意用户密码/邮箱/冻结；改密即吊销其 token |
| PUT /admin/users/batch-status | Admin JWT | - | support | 否 | 批量冻结用户（冻结即吊销 token） |
| DELETE /admin/users/:id/2fa | Admin JWT | - | support | 否 | 协助重置用户 2FA |
| POST /admin/authz/admins/:id/2fa/reset | Admin JWT | - | system_admin | handler 二次校验 IsSuper | 超管重置他人 2FA |
| POST /admin/system/update/start / restart | Admin JWT | - | system_admin | 否 | 一键升级/重启进程 |

用户端重点：user group 全部挂 UserJWTAuthMiddleware；冻结用户 status!=active 一律拒（middleware.go:396-400）；改密/重置 bump TokenVersion。

---

## 认证流程链路及各环节缺陷

1. **登录(admin)**：用户名+密码 → bcrypt 校验（含 dummy hash 防时序）→ 失败统一 ErrInvalidCredentials（不枚举，良好）。
   - 缺陷：默认口令 admin/admin123（A02）。
2. **登录后 2FA**：issue challenge JTI(5min) → TOTP 校验，5 次失败吊销 challenge → 签发 access JWT(HS256, typ=access, TV, iat)。
   - 缺陷：`CompleteLoginAfter2FA` 不再复查 admin 是否仍 active（窗口内被冻结仍可登录，P3）。
3. **请求鉴权**：Bearer token → HS256 验签（仅 HS256，无 alg=none 面）→ 缓存/DB 查管理员 → 校验 TokenVersion 相等 + iat>=TokenInvalidBefore → 超管直过 Casbin，否则 EnforceAdmin 默认拒绝。
   - 缺陷：默认 JWT 密钥公开（A01）使整条验签失去意义。
4. **Refresh**：**无 refresh token 机制**，remember_me 仅延长用户 access 有效期至 168h。
5. **Logout**：**无服务端登出**（A09）。
6. **冻结**：用户 status!=active → 中间件每请求拒绝；管理员冻结用户时 bump TokenVersion。✅ 冻结后旧 token 不可用。
7. **改密码**：bump TokenVersion + 设 TokenInvalidBefore → 全部旧 token 失效。✅
8. **撤销机制**：仅 TokenVersion/TokenInvalidBefore 两个版本号（Redis 缓存 + DB 双读），无 jti 黑名单。
   - 缺陷：无 logout 即无即时撤销。

---

## Rate Limit 覆盖表

| Endpoint | Rate Limited | Key | Window | Max |
|---|---|---|---|---|
| POST /admin/login | 是 | IP | 300s | 5 / 封 900s |
| POST /admin/login/verify-2fa | 是 | IP | 300s | 5 / 封 900s |
| POST /auth/login (用户) | 是 | email\|IP | 300s | 5 / 封 900s |
| POST /auth/register (send-code) | 是 | email\|IP | - | registerRule |
| POST /auth/password/forgot (send-code) | 是 | email\|IP | - | forgotRule |
| POST /auth/step-up | **否** | - | - | **0 (A04)** |
| POST /user/orders (下单) | 业务级风控 | 风控 key | - | orderrisk limiter |
| POST /user/wallet/withdrawals | **否(HTTP层)** | - | - | NOT VERIFIED 业务层(A14) |
| POST /user/gift-cards/redeem | 是 | - | - | giftCardRule |
| POST /user/support/tickets | 是 | userID\|IP | - | createRule |
| POST /user/support/tickets/:id/replies | 是 | userID\|ticketID | - | replyRule |
| Channel API 下单 | 是 | IP\|ChannelKey | - | channelErrorRule |
| Upstream API | 是 | UpstreamApiKey | - | upstreamRule |
| Payment callback(配置路径) | **绕过 callbackRule** | - | - | 自定义回调路径不经 callbackRule（A-附注） |

附注：CallbackRouteMiddleware 拦截的自定义支付回调路径不走已注册路由的 callbackRule 限流（靠签名校验兜底）。

---

## 已验证 / NOT VERIFIED 清单

**已验证（有代码证据）：**
- JWT 仅 HS256，无 alg=none 绕过面 ✅
- Admin/User JWT 使用不同密钥（cfg.JWT vs cfg.UserJWT）✅
- 用户/管理员冻结后 access token 即拒（status 检查）✅
- 改密/重置密码后 TokenVersion++ 使旧 token 失效 ✅
- bcrypt(DefaultCost)，无 MD5/SHA1 密码，dummy hash 防时序 ✅
- TOTP 登录 challenge 单次消费 + 5 次失败吊销 ✅
- 恢复码 bcrypt 存储、单次使用 ✅
- Casbin 默认拒绝 + 角色矩阵 + 内置角色不可改 ✅
- Recovery 中间件不回显 stack（通用 500）✅
- TrustedProxies 拒绝 0.0.0.0/0、::/0 ✅
- API secret json:"-" 不回显 ✅
- AES-256-GCM + crypto/rand nonce ✅

**NOT VERIFIED（受限于静态审计未运行）：**
- 提现创建 Service 层是否有业务频控（A14）
- 生产 config.yml.production 是否已替换默认密钥/口令（建议上线前核对）
- OAuth(Telegram/Google) state 一次性与 replay 是否完整（未在本次深读）
- upstream.Verify 的 HMAC 是否 constant-time 比较（upstream_auth 调用链未逐行）
- challenge Redis 不可用时 ConsumeChallenge fail-closed 已读代码确认 fail-closed ✅（已验证）

---

## 修复优先级建议
1. **先堵 A01/A02**（上线配置红线：拒绝默认密钥 + 拒绝默认口令）——这是“未配置即失守”的部署级致命项。
2. **A03**：is_super 变更收归超管。
3. **A04/A08**：统一 step-up 限流、失败计数、scope 绑定与单次消费。
4. 其余 P2/P3 按迭代排期。
