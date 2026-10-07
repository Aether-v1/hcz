# HCZ 安全审计报告：2FA/TOTP · 密码验证 · 文件上传 · 审计日志 · 加密机制

- 审计日期：2026-10-08
- 审计范围：用户端 2FA/TOTP、密码校验、文件上传、审计日志、字段加密与脱敏
- 审计方式：只读静态代码走查，未运行时验证
- 结论：**BLOCKED（存在 P1 级缺口需补齐后再上生产）**

---

## 一、总览结论

| 能力域 | 现状 | 评级 | 关键差距 |
|---|---|---|---|
| 用户 2FA/TOTP | 完整可用（setup/enable/disable/恢复码/登录挑战） | ✅ PASS | 无 Step-Up、改密/改邮箱不要求 2FA |
| Step-Up 二次验证 | 仅 admin 侧有成熟实现，用户侧**完全缺失** | ⚠️ PARTIAL | 用户高风险操作无二次验证 |
| 密码验证 | bcrypt + 策略 + 改密强制下线 | ✅ PASS | 无独立密码重验端点 |
| 文件上传 | admin 端 MIME 嗅探 + SVG 净化 | ⚠️ PARTIAL | telegram scene 跳过校验、无 DB 记录 |
| 审计日志 | 仅登录 + 权限变更 | ❌ GAP | 无业务操作审计（提现/调账/改密/2FA 启停） |
| 字段级加密 | AES-256-GCM 已封装，覆盖 TOTP/渠道密钥 | ⚠️ PARTIAL | C2C 银行卡号/户名明文存储 |
| 敏感数据脱敏 | C2C 账号掩码；邮箱/手机号无脱敏 | ⚠️ PARTIAL | 用户列表/后台展示邮箱明文 |

---

## 二、2FA / TOTP（用户端）

### 2.1 启用状态字段

`internal/modules/identity/user/domain/user.go:25-29`

| 字段 | 类型 | 含义 |
|---|---|---|
| `TOTPSecret` | varchar(512) | AES-GCM 加密后的 hex 密文，未启用为空串 |
| `TOTPEnabledAt` | *time.Time（带索引） | **启用状态主键**：NULL=未启用，非空=启用时间 |
| `TOTPPendingSecret` | varchar(512) | 绑定流程中待首次验证的 secret（加密） |
| `TOTPPendingExpiresAt` | *time.Time | 待绑定 secret 过期时间（10 分钟） |
| `RecoveryCodes` | text (JSON) | `[{"hash":"...","used_at":null|"..."}]`，存 hash 不存明文 |

判定逻辑：`TOTPEnabledAt != nil` 即视为已启用（`totp/application/service.go:102,126,188,220,249,294`）。

### 2.2 API 清单

路由注册：`internal/modules/identity/userauth/transport/http/routes.go:56-65`

| 方法 | 路径 | 鉴权 | Handler | 作用 |
|---|---|---|---|---|
| GET | `/me/2fa/status` | 用户 JWT | `GetUser2FAStatus` | 查询启用状态+恢复码余量 |
| POST | `/me/2fa/setup` | 用户 JWT | `SetupUser2FA` | 生成 pending secret + otpauth URL（10 分钟有效） |
| POST | `/me/2fa/enable` | 用户 JWT | `EnableUser2FA` | 校验首个 code，落库 secret，签发 10 个恢复码，bump TokenVersion |
| POST | `/me/2fa/disable` | 用户 JWT | `DisableUser2FA` | 用 TOTP code 或恢复码关闭 2FA |
| POST | `/me/2fa/recovery-codes/regenerate` | 用户 JWT | `RegenerateUser2FARecoveryCodes` | 必须当前 TOTP code 才能重生成 |
| POST | `/login/verify-2fa` | 公开（限流） | `VerifyUser2FA` | 登录第二步，校验 challenge token + code |

### 2.3 实现要点

- **算法参数**：`totp/application/service.go:23-31` — 6 位数字、30s 周期、skew=1（前后各放宽 1 个窗口，实际约 90s 有效窗口）
- **secret 加密**：`crypto.Encrypt(s.encKey, key.Secret())`（service.go:146），`encKey = DeriveKey(cfg.App.SecretKey)`（service.go:56）
- **失败保护（enable 阶段）**：Redis 计数 `2fa:user:enable:{id}:fails`，5 次失败清空 pending secret（service.go:26,354-368）
- **失败保护（登录挑战）**：challenge JWT 带 jti，失败 `BumpFails`，5 次 `Revoke`（`user_2fa_handler.go:17,312-318`）
- **恢复码**：10 个，一次性消费，存 bcrypt/sha hash（`totpapplication.MatchAndConsumeRecoveryCode`，service.go:320）
- **管理员强制解绑**：`AdminResetUser2FA(operatorID, targetID)`（service.go:283），operatorID 必须非零以留痕
- **challenge token 防串用**：`Purpose=two_factor`、`Typ=2fa_challenge`（`application/service.go:439-462,488`），防止挑战 token 被当作 access JWT

### 2.4 评级：✅ PASS

实现完整，与主流 2FA 方案对齐。唯一弱点是 6 位 TOTP + skew=1 本身爆破空间大，但有挑战级失败计数兜底。

---

## 三、Step-Up 二次验证机制

### 3.1 现状：admin 侧已有成熟实现

核心包：`internal/platform/http/stepup/verify.go`

- `RequireFor(c, verifier, expectedScope)`（verify.go:80）在业务 handler 开头调用
- 安全模型（注释见 verify.go:6-12）：
  1. 当前 JWT 管理员已登录
  2. verifier 已配置（否则 fail-closed）
  3. 请求头 `X-Auth-Challenge` 携带 challenge token
  4. token 签名/用途/有效期合法
  5. token 内 `admin_id` 与当前登录管理员一致
  6. token 的 `scope` claim 与本次动作 scope **完全一致**（格式 `wallet.adjust:user:123`）
  7. 原子消费 jti（Redis SETNX），禁止重放

签发入口：`POST /admin/auth/step-up`（`adminauth/transport/http/admin_2fa_handler.go:430`），要求 body `{code, scope}`，scope 必须匹配正则 `^[a-z0-9_]+(\.[a-z0-9_]{0,2})?:[a-z0-9_]+:[1-9][0-9]*$`（line 423）。

已接入的高风险动作（见 `docs/audit/02-auth-rbac.md` 路由表）：
- `POST /admin/users/:id/wallet/adjust`
- `POST /admin/wallet/withdrawals/:id/approve`
- `POST /admin/wallet/withdrawals/:id/complete`
- `POST /admin/orders/:id/manual-refund`
- C2C 仲裁相关

### 3.2 用户侧：完全缺失

在用户端路由（`userauth/transport/http/routes.go`）与 handler 中**未发现任何 step-up / reauth / fresh-auth 机制**。以下敏感操作仅靠登录态 JWT：

| 操作 | 路由 | 现有保护 | 缺口 |
|---|---|---|---|
| 修改密码 | `PUT /me/password` | 需旧密码 | 未要求 2FA（若用户已启用 2FA） |
| 更换邮箱 | `POST /me/email/change` | 旧邮箱+新邮箱双验证码 | 未要求 2FA / 密码重验 |
| 关闭 2FA | `POST /me/2fa/disable` | TOTP code 或恢复码本身 | 可接受，但未记录审计日志 |
| 绑定/解绑 Telegram、Google | `/me/telegram/*`, `/me/google/*` | 仅 JWT | 未要求密码/2FA 重验 |
| C2C 收款方式绑定 | c2c 模块 | 仅 JWT | 未要求重验 |

### 3.3 已知遗留问题（项目自带审计已标记）

- **A04 / ISSUE-005**：`POST /admin/auth/step-up` 本身未挂 RateLimitMiddleware，且 StepUp 内部直接调 `totp.VerifyChallengeCode`，没有登录链路的 `BumpFails/5 次吊销 challenge` 计数。攻击者持有 admin access token 后可在线爆破 6 位 TOTP。证据：`adminauth/transport/http/routes.go:35`、`admin_2fa_handler.go:447`。
- 详见 `HCZ_FULL_SECURITY_AUDIT_REPORT.md:336-351`。

### 3.4 评级：⚠️ PARTIAL

**建议**：将 `stepup` 包泛化为 `userstepup`，复用 `UserChallengeClaims` 机制，为用户端 `PUT /me/password`、`POST /me/email/change`、`POST /me/2fa/disable`、绑定/解绑第三方身份强制 step-up（已启用 2FA 的用户必须过 2FA；未启用者退化为密码重验）。

---

## 四、密码验证

### 4.1 现有接口

| 方法 | 路径 | Handler | Service |
|---|---|---|---|
| POST | `/forgot-password`（公开，限流） | `UserPasswordHandler.UserForgotPassword`（user_password_handler.go:66） | `Service.ResetPassword`（profile.go:19） |
| PUT | `/me/password`（登录态） | `UserPasswordHandler.ChangeUserPassword`（user_password_handler.go:113） | `Service.ChangePassword`（profile.go:57） |

路由注册：`userauth/transport/http/routes.go:68-81`。

### 4.2 实现方式

- **哈希算法**：`bcrypt.GenerateFromPassword(..., bcrypt.DefaultCost)`（service.go:324、profile.go:39,83）— DefaultCost=10
- **登录校验**：`bcrypt.CompareHashAndPassword`（service.go:391）
- **时序侧信道防护**：用户不存在时仍执行一次 dummy bcrypt 比较（service.go:382）
- **密码策略**：`passwordpolicy.Validate(cfg.Security.PasswordPolicy.ValidationPolicy(), pwd)`（service.go:293、profile.go:24,79），可配置
- **改密模式**：
  - `PasswordChangeModeChangeWithOld`：必须旧密码（profile.go:73-77）
  - `PasswordChangeModeSetWithoutOld`：Telegram 占位邮箱账号首次设置，免旧密码（profile.go:279）
- **改密后强制下线**：`user.TokenVersion++` + `TokenInvalidBefore = &now`（profile.go:47-48,92-93），同步更新缓存
- **忘记密码**：邮箱验证码（purpose=reset），成功后同样 bump TokenVersion
- **账号枚举防护**：忘记密码接口对"邮箱未注册"与"验证码错误"统一返回 `error.verify_code_invalid`（user_password_handler.go:87-90）

### 4.3 缺口

- **无独立的"密码重验"端点**：业务侧无法在不实际改密的情况下验证用户当前密码（即没有 `/me/reauth` 之类的 Step-Up 密码因子端点）。
- **改密未联动 2FA**：已启用 2FA 的用户改密时，不要求 TOTP code（profile.go:73-77 只校验旧密码）。

### 4.4 评级：✅ PASS（核心流程），⚠️ 缺 Step-Up 复用端点

---

## 五、文件上传

### 5.1 API

路由：`internal/modules/upload/transport/http/routes.go:5-7`

```
POST /admin/upload     (AdminHandler.UploadFile)
```

**仅 admin 端可用**，用户端无上传路由。

### 5.2 策略配置

注入点：`internal/app/container/services_foundation.go:117-123`

```go
uploadapp.NewService(uploadapp.Policy{
    MaxSize:           c.Config.Upload.MaxSize,
    AllowedTypes:      c.Config.Upload.AllowedTypes,
    AllowedExtensions: c.Config.Upload.AllowedExtensions,
    MaxWidth:          c.Config.Upload.MaxWidth,
    MaxHeight:         c.Config.Upload.MaxHeight,
}, uploadlocal.New("uploads"))
```

测试默认值：`MaxSize: 10 * 1024 * 1024`（services_application.go:209）。

### 5.3 校验链路（`upload/application/service.go:64-178`）

1. **大小**：`file.Size > policy.MaxSize` 拒绝（line 68）
2. **扩展名白名单**：小写化后比对 `AllowedExtensions`（line 73-78）
3. **MIME 嗅探**：读取前 512 字节，`http.DetectContentType`（line 88-97）
4. **SVG 补充识别**：`.svg` 且内容含 `<svg` → `image/svg+xml`（line 99-101）
5. **MIME 白名单**：`AllowedTypes` 大小写不敏感比对（line 102-113）
6. **图片尺寸**：非 SVG 的 image/* 解码宽高，校验 MaxWidth/MaxHeight（line 116-132）
7. **SVG 安全净化**：XML token 流式解析，拒绝 `<script>`、`<foreignObject>`、`on*` 事件属性、`javascript:`、`data:text/html|application|image/svg`、处理指令、实体声明（line 135-149,237-286）— 实现质量高，优于正则黑名单

### 5.4 存储与返回格式

- **存储路径**：`uploads/{scene}/{yyyy}/{mm}/{uuid}{ext}`（localstore/store.go:29-33）
- **文件名**：UUID v4 + 原扩展名（service.go:155），不可预测
- **返回结构**（contract/ports.go:6-13）：

```go
type Result struct {
    URL      string  // /uploads/{scene}/{yyyy}/{mm}/{uuid}{ext}
    Filename string  // 用户原始文件名
    MimeType string
    Size     int64
    Width    int
    Height   int
}
```

**注意**：返回的是公开 URL，**没有 file ID，也没有数据库记录**。文件一经写入即无元数据索引，无法按 ID 删除/审计。

### 5.5 风险点

| 级别 | 问题 | 位置 |
|---|---|---|
| **P1** | `telegram` scene **跳过扩展名白名单和 MIME 白名单**（`if normalizedScene != "telegram" && ...`） | service.go:74,102 |
| P2 | 本地磁盘存储，无对象存储/CDN 切换抽象；`Store` 接口仅一个 `Save`，无 Delete/List | contract/ports.go:25-27 |
| P2 | 无数据库记录，无法审计"谁上传了什么"，也无法批量清理 | — |
| P3 | 原始 `file.Filename` 透传到返回值，未做长度/字符清洗（风险低，因落盘名是 UUID） | service.go:172 |

### 5.6 评级：⚠️ PARTIAL

---

## 六、审计日志

### 6.1 存储结构（三张表）

| 表 | Domain 文件 | 记录内容 |
|---|---|---|
| `user_login_logs` | `auditlog/domain/user_login.go:7-18` | 用户登录：user_id/email/status/fail_reason/client_ip/user_agent/login_source/request_id |
| `admin_login_logs` | `auditlog/domain/admin_login.go:6-18` | 管理员登录+2FA 操作：admin_id/username/event_type/status/fail_reason/client_ip/user_agent/request_id/operator_id |
| `authz_audit_logs` | `auditlog/domain/authz.go:11-24` | 权限变更：operator_admin_id/target_admin_id/action/role/object/method/detail(JSON) |

### 6.2 记录方式

- 三个应用服务：`UserLoginService.Record`（user_login_service.go:36）、`AdminLoginService.Record`（admin_login_service.go:33）、`AuthzService.Record`（authz_service.go:37）
- 直接 `repo.Create` 写 GORM，无异步队列、无签名/链式哈希防篡改
- 字段规范化：email 小写化、status 强制 success/failed、source 默认 web（user_login_service.go:41-61）

### 6.3 查询接口

- admin：`GET /admin/authz/audit-logs`、`GET /admin/user-login-logs`（auditlog/transport/http/routes.go:5-8）
- user：`GET /me/login-logs`（routes.go:11，user_handler.go:24），仅能查自己

### 6.4 覆盖范围（关键缺口）

**已覆盖**：登录成功/失败、管理员 2FA 操作、权限/角色变更。

**未覆盖（P1 缺口）**：
- 用户改密 / 重置密码
- 用户改邮箱 / 绑定解绑第三方身份
- 用户 2FA 启用/关闭/重生成恢复码
- 管理员强制重置用户 2FA（虽有 operatorID 参数，但需确认 handler 是否写日志 — `totp/service.go:283` 注释称"返回 targetUser 供 handler 写审计日志"）
- 资金类操作：提现申请/审批/打款、钱包调账、退款、C2C 仲裁
- 文件上传

### 6.5 评级：❌ GAP

现有日志仅覆盖"身份认证"维度，未覆盖"业务操作"维度，不满足金融级审计要求。

---

## 七、加密机制

### 7.1 原语

`internal/crypto/aes.go`：

- `DeriveKey(secret)` = SHA256(secret) → 32 字节（aes.go:14-17）
- `Encrypt(key, plaintext)` = AES-256-GCM，随机 nonce 前置，返回 hex（aes.go:20-38）
- `Decrypt(key, ciphertextHex)` = 反向（aes.go:41-69）

GCM 带认证标签，不存在 padding oracle；nonce 用 `crypto/rand` 生成，正确。

### 7.2 实际加密覆盖范围（grep 结果）

| 字段 | 位置 | 是否加密 |
|---|---|---|
| users.TOTPSecret / TOTPPendingSecret | `userauth/totp/application/service.go:146` | ✅ AES-GCM |
| admins.TOTPSecret | `adminauth/totp/application/service.go:133` | ✅ AES-GCM |
| channel_clients.ChannelSecret | `channelclient/application/service.go:57` | ✅ AES-GCM |
| channel_clients.BotToken | `channelclient/application/service.go:74` | ✅ AES-GCM |
| site_connections.ApiSecret | `siteconnection/application/service.go:60` | ✅ AES-GCM |
| **c2c_payment_methods.AccountName（户名）** | `c2c/domain/payment_method.go:12` | ❌ **明文** |
| **c2c_payment_methods.AccountIdentifier（卡号/账号）** | `c2c/domain/payment_method.go:13` | ❌ **明文** |
| users.PasswordHash | — | bcrypt 单向（非加密） |

### 7.3 密钥管理弱点

- 所有加密共用 `cfg.App.SecretKey` 经 SHA256 派生（aes.go:15），**无 KEK/DEK 分层、无密钥轮换、无 salt**。SecretKey 一旦泄露，所有 TOTP secret、渠道 bot token、C2C 卡号均可解密。
- 未发现 HSM / KMS 集成。

### 7.4 评级：⚠️ PARTIAL

**P1**：C2C 银行卡号/户名必须加密存储（复用 `crypto.Encrypt`，key 派生方式同 TOTP）。

---

## 八、敏感数据脱敏

### 8.1 已实现

- `c2c/transport/presenter/presenter.go:22` `MaskIdentifier(s)`：保留前 4 后 4，中间打码；line 46 用于 `AccountIdentifier` 返回
- `users.PasswordHash` / `TokenVersion` / `TokenInvalidBefore` / `TOTPSecret` / `TOTPPendingSecret` / `RecoveryCodes` 均 `json:"-"`（user.go:13,23-29）

### 8.2 未脱敏

- **用户邮箱在所有 user presenter 中明文返回**：`presenter/user.go:15,33,110,119`。管理员后台用户列表、用户自己的 `/me` 都返回完整邮箱。
- 手机号：代码中未发现手机号字段。
- IP 地址：登录日志 `client_ip` 明文存储并返回（user_login.go:13）。

---

## 九、与目标需求的差距清单（按优先级）

### P0（资金/账号安全红线）

| # | 差距 | 建议 |
|---|---|---|
| P0-1 | C2C 银行卡号/户名明文存储 | 用 `crypto.Encrypt` 加密 AccountName/AccountIdentifier；DB 迁移脚本批量重加密；查询时按需解密 |
| P0-2 | 用户端无 Step-Up 机制 | 复用 `stepup` 包模式，新增用户挑战签发端点（如 `POST /me/auth/step-up`），对改密/改邮箱/2FA 关闭/绑解第三方身份强制二次验证 |

### P1（审计完整性与爆破防护）

| # | 差距 | 建议 |
|---|---|---|
| P1-1 | admin `/admin/auth/step-up` 无限流、无失败计数（项目自带审计 A04） | 挂 RateLimitMiddleware（按 adminID，10 次/5 分钟）；引入与登录一致的 BumpFails |
| P1-2 | 审计日志不覆盖业务操作 | 新增 `operation_audit_logs` 表或在 authz_audit_logs 扩展 action 枚举，记录：改密/改邮箱/2FA 启停/提现实审批/钱包调账/C2C 仲裁/文件上传 |
| P1-3 | upload `telegram` scene 跳过扩展名+MIME 白名单 | 收回该豁免，或对 telegram scene 单独配置更窄的白名单（仅图片） |

### P2（纵深防御）

| # | 差距 | 建议 |
|---|---|---|
| P2-1 | 改密不要求 2FA | 已启用 2FA 的用户改密时必须附带 TOTP code |
| P2-2 | 上传无 DB 记录 | 新增 `upload_files` 表存 file_id/scene/uploader/url/mime/size，便于删除与审计 |
| P2-3 | 邮箱明文返回 | 管理员后台用户列表对非本人邮箱脱敏（`a***@b.com`）；本人 `/me` 保留明文 |
| P2-4 | 加密密钥无分层 | 引入 DEK/KEK 概念，或至少将 TOTP 加密 key 与渠道 token 加密 key 拆分为不同配置项 |

### P3（优化项）

| # | 差距 | 建议 |
|---|---|---|
| P3-1 | 审计日志无防篡改（无链式哈希/只追加） | 高合规要求场景下，对 authz_audit_logs 做 prev_hash 链 |
| P3-2 | 登录日志 client_ip 明文 | 按合规要求做 IP 截断或哈希化 |
| P3-3 | 上传返回原始 filename | 清洗长度与控制字符 |

---

## 十、关键代码索引

| 主题 | 文件:行号 |
|---|---|
| TOTP 状态字段 | `internal/modules/identity/user/domain/user.go:25-29` |
| TOTP 服务核心 | `internal/modules/identity/userauth/totp/application/service.go:53,118,165,180,212,241,263,283` |
| 2FA HTTP handler | `internal/modules/identity/userauth/transport/http/user_2fa_handler.go:117,131,157,198,239,272` |
| 2FA 路由 | `internal/modules/identity/userauth/transport/http/routes.go:56-65` |
| 登录 challenge 签发 | `internal/modules/identity/userauth/application/service.go:439-462` |
| 密码改/重置 | `internal/modules/identity/userauth/application/profile.go:19,57` |
| 密码 handler | `internal/modules/identity/userauth/transport/http/user_password_handler.go:66,113` |
| Step-Up 中间件 | `internal/platform/http/stepup/verify.go:80` |
| admin Step-Up 入口 | `internal/modules/identity/adminauth/transport/http/admin_2fa_handler.go:430` |
| 上传服务 | `internal/modules/upload/application/service.go:64` |
| 上传路由 | `internal/modules/upload/transport/http/routes.go:6` |
| 上传策略注入 | `internal/app/container/services_foundation.go:117-123` |
| 本地存储 | `internal/modules/upload/infrastructure/localstore/store.go:28` |
| 审计 domain | `internal/modules/auditlog/domain/{user_login,admin_login,authz}.go` |
| 审计路由 | `internal/modules/auditlog/transport/http/routes.go:5-12` |
| AES 原语 | `internal/crypto/aes.go:14,20,41` |
| C2C 支付方式（明文） | `internal/modules/c2c/domain/payment_method.go:12-13` |
| C2C 脱敏 | `internal/modules/c2c/transport/presenter/presenter.go:22,46` |

---

## 十一、验证说明

- 本报告基于静态代码走查，未运行单元测试或动态验证。
- 项目根目录已存在 `HCZ_FULL_SECURITY_AUDIT_REPORT.md` 与 `docs/audit/02-auth-rbac.md`，其中 A04（step-up 无限流）等结论与本次走查一致，已交叉引用。
- 未验证项：
  - 实际运行时 `cfg.Upload.AllowedTypes/AllowedExtensions` 的配置值（需看 config 或 .env）
  - `AdminResetUser2FA` 的 handler 是否真的写了审计日志（service 层仅返回 targetUser 供 handler 写，未在本次审计中读 handler）
  - Redis challenge store 的具体实现（仅见接口 `User2FAChallengeStore`）
