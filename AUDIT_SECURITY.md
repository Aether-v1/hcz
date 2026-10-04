# 安全与权限审计报告

> 审计对象：HCZ（github.com/Aether-v1/hcz）
> 审计范围：维度八（权限/IDOR/RBAC）、维度九（Authenticated-Only）、维度十（Guest/Legacy Payment 残留）、维度二十一（安全）
> 审计方式：静态代码审计 + 运行既有测试（`go test`）。只审计，未修改任何业务代码。
> 关键证据文件均标注 `文件:行号`。

## 审计摘要
- P0 发现：0
- P1 发现：2
- P2 发现：8

---

## 八、权限 / IDOR / RBAC

- 结论：**PARTIAL**（RBAC 全量覆盖、User 侧 IDOR 到位；但 3 条财务写路由未挂 Payment Compliance Step-Up）

### Admin Route 覆盖清单（抽样 + 自动测试全量校验）

项目内置静态测试 `TestAllAdminRoutesCoveredByBuiltinRoles`（`internal/app/httpserver/rbac_coverage_test.go:32`）AST 扫描全部 admin 路由注册调用，并与 `authz.BuiltinRoleSeeds()` 用 Casbin `keyMatch2` 比对。本次运行结果：

```
=== RUN   TestAllAdminRoutesCoveredByBuiltinRoles
    rbac_coverage_test.go:40: validating RBAC coverage for 252 admin routes
--- PASS: ... (0.12s)
```

即 **252 条 admin 路由全部被至少一个内置角色策略覆盖**。

路由分组中间件装配（`internal/app/httpserver/routes_admin.go`）：

| 分组 | 中间件链 | 覆盖范围 |
|---|---|---|
| `admin`（裸组，仅注册于 line 92-93 之前） | 无 | 仅 `/admin/login`、`/admin/login/verify-2fa`（公开登录） |
| `authorized = admin.Use(JWTAuth, AdminRBAC)`（line 96） | JWT + RBAC | 绝大多数管理路由 |
| `paymentProtected = admin.Group("", PaymentComplianceRequired)`（line 99） | JWT + RBAC + 合规声明 | 财务/支付类写路由 |

注意：`admin.Use(...)` 在 Gin 中 mutate 了 `admin` 自身（line 98 注释明确说明），因此后建的 `paymentProtected` 子组正确继承 JWT+RBAC。

### 未覆盖 Admin Route 清单
**无。** 自动测试对 252 条路由零未覆盖。Exchange Rate Settings 已纳入 RBAC：
- `GET/PUT /admin/settings/exchange-rate`、`POST /admin/settings/exchange-rate/refresh`（`routes_admin.go:128-130`）
- 策略：`{Object:"/admin/settings/exchange-rate", Action:"*"}` 与 `/refresh POST` 归属 `system_admin` 角色（`internal/authz/bootstrap.go:244-245`）。

### User 侧 IDOR 检查结果（全部 PASS）

| 资源 | Handler | 归属校验 | 失败状态码 |
|---|---|---|---|
| 订单列表 | `ordertransport.UserHandler.ListOrders` (`user_handler.go:96`) | `UserID: uid` 来自 JWT 上下文 (line 109) | — |
| 订单详情 | `GetOrderByOrderNo` (`user_handler.go:150`) | `GetOrderByUserOrderNoForTenant(tenant, orderNo, uid)` (line 162) | 404 `error.order_not_found` |
| 交付下载 | `DownloadFulfillment` (`user_handler.go:179`) | `GetAnyOrderByUserOrderNoForTenant(..., uid)` (line 189) | 404 |
| 取消订单 | `CancelOrder` (`user_handler.go:298`) | 先 `GetOrderByUserOrderNoForTenant(...,uid)` 再 `CancelOrder(found.ID, uid)` (line 311,321) | 404 |
| 发起售后 | `UserCreateAfterSale` (`aftersale_handler.go:86`) | `svc.Request(uid, orderID, ...)` (line 106)，服务层 `ErrNotOwner` → 404 (line 223-224) | 404 |
| 查询售后 | `UserGetAfterSale` (`aftersale_handler.go:116`) | `orders.GetByIDAndUser(orderID, uid)` (line 128)，注释明确「非本人返回 404，不暴露存在性」 | 404 |
| 创建支付 | `paymenthttp.WriteHandler.CreatePayment` (`write_handler.go:102`) | `GetOrderByUserOrderNoForTenant(..., uid)` (line 114) | 404 |
| 捕获支付 | `CapturePayment` (`write_handler.go:147`) | `GetOrderByUserForTenant(..., payment.OrderID, uid)` (line 166) | 404 |
| 钱包/交易 | `wallethttp.UserHandler.*` (`user_handler.go`) | 全部 `GetAccount(uid)` / `ListTransactions(uid,...)` / `GetRechargeOrderByRechargeNo(uid,...)` (line 153,167,229,292) | 404 |

**IDOR 状态码一致性：PASS。** 非本人/不存在资源统一返回 404（`ErrOrderNotFound` / `ErrNotOwner` / `ErrRechargeNotFound`），不区分「不存在」与「无权限」，不暴露资源存在性。

### 财务写操作 Step-Up 检查结果（PARTIAL — 见 P1-1）

| 财务写操作 | 路由 | 挂载分组 | RBAC | PaymentCompliance Step-Up |
|---|---|---|---|---|
| 售后退款（reject/resolve/partial/full_refund） | `POST /admin/orders/:id/after-sale/action` | `paymentProtected` (routes_admin.go:165) | finance | ✅ 有 |
| 支付渠道增删改 | `/admin/payment-channels*` | `paymentProtected` (line 178) | finance | ✅ 有 |
| 支付记录 | `/admin/payments*` | `paymentProtected` (line 179) | finance | ✅ 有 |
| 钱包调整 | `POST /admin/users/:id/wallet/adjust` 等 | `paymentProtected` (line 183) | finance | ✅ 有 |
| 对账 | `/admin/reconciliation*` | `paymentProtected` (line 202) | integration | ✅ 有 |
| 佣金/提现 | `/admin/affiliates/withdraws*` | `paymentProtected` (line 141) | finance | ✅ 有 |
| **退款到余额** | `POST /admin/orders/:id/refund-to-wallet` | **`authorized`** (routes_admin.go:162 → routes.go:29) | finance | ❌ **缺失** |
| **手动退款** | `POST /admin/orders/:id/manual-refund` | **`authorized`** (routes_admin.go:162 → routes.go:30) | finance | ❌ **缺失** |
| **退款手续费修正** | `PATCH /admin/order-refunds/:id/payment-fee` | **`authorized`** (routes_admin.go:162 → routes.go:31) | finance | ❌ **缺失** |

---

## 九、Authenticated-Only 审计

- 结论：**PARTIAL**（业务路由全部 fail-closed 挂 JWT；例外清单与 Contract 一致；但未认证返回 HTTP 200 而非 HTTP 401）

### 路由认证覆盖表

`internal/app/httpserver/routes_storefront.go` 路由分组结构：

| 模块 | 路由 | 方法 | 是否需要登录 | 例外原因 / 证据 |
|---|---|---|---|---|
| Public Config | `/api/v1/public/config` | GET | 否 | 启动最小配置（contract 例外），`routes_storefront.go:77` |
| Captcha | `/api/v1/public/captcha/image` | GET | 否 | 验证码（例外），`routes_storefront.go:78` |
| Affiliate pre-login | `/api/v1/public/affiliate/click` | POST | 否 | 返利点击追踪（例外），`affiliate/routes.go` |
| Login | `/api/v1/auth/login` | POST | 否 | 登录（例外） |
| Register | `/api/v1/auth/register` | POST | 否 | 注册（例外） |
| 2FA | `/api/v1/auth/login/verify-2fa` | POST | 否 | 登录链（例外） |
| Telegram/Google OIDC | `/api/v1/auth/telegram/*`、`/auth/google/*` | GET/POST | 否 | Auth Callback（例外） |
| Password Reset | `/api/v1/auth/forgot-password`、`/auth/send-verify-code` | POST | 否 | 找回密码（例外） |
| Payment Callback/Webhook | `/api/v1/payments/webhook/*`、callback 路由 | POST | 否 | 支付回调（例外），`routes_storefront.go:152-156` |
| **Products** | `/api/v1/public/products*` | GET | **是** | `authedPublic` 组 + UserJWT (line 84,86) |
| **Categories** | `/api/v1/public/categories*` | GET | **是** | line 87 |
| **Content** | `/api/v1/public/posts*`、banners | GET | **是** | line 88 |
| **Member Levels** | `/api/v1/public/member-levels*` | GET | **是** | line 89 |
| **Orders** | `/api/v1/orders*`（create/preview/read/cancel/after-sale） | * | **是** | `user` 组 + UserJWT (line 108-130) |
| **Wallet** | `/api/v1/wallet*` | * | **是** | line 133 |
| **Recharge** | `/api/v1/wallet/recharge*` | * | **是** | line 133（含于 wallet） |
| **Affiliate（登录态）** | `/api/v1/affiliate/dashboard`、commissions、withdraws | * | **是** | `user` 组 (line 136) |
| **After-Sale** | `/api/v1/orders/:id/after-sale` | POST/GET | **是** | line 130 |
| Cart | `/api/v1/cart*` | * | **是** | line 123 |
| 用户中心 | `/api/v1/me/*` | * | **是** | line 111-122 |

### 未登录可访问的业务 API 清单
**无（业务面）。** 所有 Products/Categories/Content/MemberLevels/Orders/Wallet/Recharge/Affiliate/After-Sale 均在 `authedPublic` 或 `user` 组，叠加 `middleware.UserJWTAuthMiddleware`。

自动测试 `TestAuthenticatedOnlyBoundaryBehavior`（`route_structure_test.go:232`，本次运行 PASS）断言：
- `GET /public/config` 无 token → 放行（认证链启动需要）
- `GET /public/products` 无 token → 拒绝（401）
- `POST /guest/orders`、`GET /guest/orders/x` → 404

### 偏差（见 P2-5）
未认证请求经 `response.Unauthorized` → `Error()` 返回 **HTTP 200**，body 内 `{"status_code":401}`（`response/response.go:88,106-108`）。鉴权 enforcement 本身 fail-closed 正确（测试通过），但严格意义上「返回 401」体现在业务码而非 HTTP 状态行。

---

## 十、Guest / Legacy Payment 残留

- 结论：**PASS（ACTIVE_BLOCKER = 0）**

### 关键词搜索分类表

| 关键词 | 文件:行号 | 分类 | 说明 |
|---|---|---|---|
| `/guest/*` 路由组 | `routes_storefront.go:92`（注释停用） | DEAD | 原组不再注册；`/guest/*` 实测 404（测试 line 275-279） |
| `RegisterGuestReadRoutes` | `order/transport/http/routes.go:87` | LEGACY_UNUSED | 已定义，生产路由无调用方 |
| `RegisterGuestPreviewRoute` | `order/transport/http/routes.go:105` | LEGACY_UNUSED | 同上 |
| `RegisterGuestCreateRoute` | `order/transport/http/routes.go:129` | LEGACY_UNUSED | 同上 |
| `RegisterGuestCreateAndPayRoute` | `order/transport/http/routes.go:137` | LEGACY_UNUSED | 同上 |
| `RegisterGuestLatestRoute` | `payment/transport/http/routes.go:6` | LEGACY_UNUSED | 已定义，无调用 |
| `RegisterGuestWriteRoutes` | `payment/transport/http/routes.go:22` | LEGACY_UNUSED | 已定义，无调用 |
| `GuestHandler` 结构 | `order/transport/http/guest_handler.go:25` | LEGACY_UNUSED | 在 `bootstrap/order/wiring.go:41` 实例化并作为参数传入 `registerStorefrontRoutes`（`routes_storefront.go:58`），但**未挂到任何路由组** |
| `CreateGuestOrder` / `CreateGuestOrderAndPay` | `create_handler.go:133,207` | DEAD | 仅被上述未注册路由函数引用，不可达 |
| `CreateGuestPayment` / `CaptureGuestPayment` | `write_handler.go:189,235` | DEAD | 仅被 `RegisterGuestWriteRoutes` 引用，不可达 |
| `GetGuestLatestPayment` | `latest_handler.go` | DEAD | 仅被 `RegisterGuestLatestRoute` 引用 |
| `purchase_type=guest` 校验 | `order/application/order_service_validate.go:106-112` | LEGACY_UNUSED | 防御性代码：`input.IsGuest && purchaseType==member` 拒绝；但 HTTP 层已无任何入口置 `IsGuest=true` |
| `purchase_type` 字段定义 | `product/domain/product.go:27` | LEGACY_UNUSED | DB 字段保留 guest/member，guest 分支不可达 |
| `/api/v1/guest/` 保留前缀 | `settings/schema/integration/callback_routes.go:60` | DEAD（配置校验） | 仅用于禁止自定义回调路由与 guest 前缀冲突，非路由 |
| sitemap `Disallow: /guest/` | `sitemap/application/service.go:83` | DEAD（SEO） | 仅 robots.txt |
| `CreateOrderAndPay`（用户版） | `create_handler.go:165` | **ACTIVE（合规）** | 挂在 `user` 组（`routes_storefront.go:125`），首行 `ginutil.GetUserID(c)` 强制登录（line 166-169），调用 `CreateOrder` 而非 `CreateGuestOrder`。非 guest 分支。 |

### ACTIVE_BLOCKER 清单（必须为 0）
**空。** 无任何可触达的游客下单/查单/支付/下载路径。`/guest/*` 实测 404。

---

## 二十一、安全审计

- 结论：**PARTIAL**（无 P0；核心防护到位；存在 2 项 P1 与若干 P2 加固项）

### 发现列表（按严重度排序）

#### [P1-1] 退款写路由未挂 Payment Compliance Step-Up — 财务写操作 — `routes_admin.go:162` + `order/transport/http/routes.go:29-31`
- **证据**：`ordertransport.RegisterAdminRefundWriteRoutes(authorized, adminOrderRefundHandler)` 挂在 `authorized`（仅 JWT+RBAC），而非 `paymentProtected`（JWT+RBAC+`PaymentComplianceRequired`）。注册的 3 条路由：
  - `POST /admin/orders/:id/refund-to-wallet`（退款到钱包，直接动账）
  - `POST /admin/orders/:id/manual-refund`（手动退款）
  - `PATCH /admin/order-refunds/:id/payment-fee`（退款手续费修正）
- **问题**：同为退款动账的 `after-sale/action`（`routes_admin.go:165`）与 `wallet/adjust`（line 183）均挂在 `paymentProtected`，唯独直接退款 3 条绕过合规声明闸门。持有 `finance` 角色的管理员在超管未确认合规声明时仍可直接退款到余额。
- **修复建议**：将 `RegisterAdminRefundWriteRoutes` 的接收者由 `authorized` 改为 `paymentProtected`（与 after-sale 写路由对齐）。

#### [P1-2] 注册 / 找回密码 / 发送验证码端点无速率限制 — 速率限制 — `routes_storefront.go:97-98,104` + `userauth/transport/http/routes.go:28,36,72`
- **证据**：
  - 登录 `POST /auth/login` 有限流（`KeyByIPAndJSONField("email")`，`routes_storefront.go:99`）。
  - 但 `POST /auth/send-verify-code`（`routes.go:28`）、`POST /auth/register`（`routes.go:36`）、`POST /auth/forgot-password`（`routes.go:72`）三个 Register 函数**均不接收 rateLimit 参数**，`routes_storefront.go:97-98,104` 调用时也未挂限流。
  - 另：`POST /auth/google/redirect/callback`、`/exchange`（`routes.go:128-129`）同样未挂限流。
- **问题**：`send-verify-code` 无频率限制 → 可被滥用来对任意邮箱批量触发邮件（邮件轰炸 / SMTP 配额耗尽 / 被标记为垃圾邮件源）。`register`、`forgot-password` 无 IP 限流 → 可枚举/批量注册。
- **缓解（部分）**：注册与重置均需邮箱验证码，验证码本身有尝试次数上限（`ErrVerifyCodeAttemptsExceeded`，`user_login_handler.go:121`、`user_password_handler.go:93`），但发送端本身未限流。
- **修复建议**：为 `/auth/send-verify-code`、`/auth/register`、`/auth/forgot-password` 挂 `middleware.RateLimitMiddleware`（按 IP + email），复用 `loginRule` 或新增更严规则。

#### [P2-1] 硬编码默认超管密码 `admin123` — hardcoded secret — `identity/admin/application/bootstrap.go:14,38`
- **证据**：`const defaultBootstrapPassword = "admin123"`；当未配置 bootstrap 密码时回退使用（line 37-39）。
- **缓解（已做）**：
  - release 模式下若未配置密码则**跳过**默认管理员创建（`cmd/server/main.go:148-149`）。
  - `unsafeBootstrapAdminPassword`（`main.go:295-305`）将 `admin123`/`password` 等列入弱密码黑名单，release 模式下 Fatal 拒绝启动。
- **残余风险**：非 release（debug/dev）模式下会以 `admin/admin123` 建超管。建议直接移除默认回退常量，未配置即报错。

#### [P2-2] 默认管理员密码明文写入日志 — sensitive logs — `identity/admin/application/bootstrap.go:49`
- **证据**：`logger.Warnw("default_admin_created_with_default_password", "username", ..., "password", password)` — 把明文密码打进日志。同函数 line 52 对非默认密码已用 `password_hidden:true`，默认密码分支却打印了明文。
- **修复建议**：删除 `"password", password` 字段，仅保留 username 与「请立即修改」提示。

#### [P2-3] 密码重置存在账号枚举 — IDOR / 信息泄漏 — `user_password_handler.go:87-88`
- **证据**：`ResetPassword` 对 `ErrUserNotFound` 返回 404 `error.user_not_found`，而验证码错误返回 400 `error.verify_code_invalid`。攻击者用任意邮箱+瞎填验证码即可区分「邮箱是否已注册」。
- **修复建议**：对用户不存在与验证码错误返回统一文案/状态码（如一律 400 `error.reset_failed`），不区分。

#### [P2-4] CORS 默认允许任意来源 — CORS — `config/config.go:225,379` + `middleware.go:80-101`
- **证据**：`defaultCORSAllowedOrigins = ["*"]`；`cors.allowed_origins` 默认 `["*"]`。
- **缓解**：`allow_credentials` 默认为 false（未在 viper 设置默认，Go 零值 false），因此默认响应 `Access-Control-Allow-Origin: *` 且不带 credentials。鉴权走 `Authorization: Bearer` 头而非 Cookie，浏览器不会自动携带，CSRF 实际风险低。
- **残余风险**：生产部署若误开 `allow_credentials=true` 而仍用 `*`，中间件会反射任意 Origin（`middleware.go:86-88`），形成任意站携带凭据跨域。建议生产强制显式配置 allowed_origins。

#### [P2-5] 未认证/未授权返回 HTTP 200 + body 业务码 — HTTP 语义 — `platform/http/response/response.go:88,106-113`
- **证据**：`Error()` 恒 `c.JSON(http.StatusOK, ...)`；`Unauthorized`/`Forbidden` 仅把 `status_code` 置 401/403，HTTP 状态行仍为 200（`route_structure_test.go:253-260` 注释与 `do()` helper 也据此从 body 取 401）。
- **影响**：鉴权 enforcement 正确（fail-closed，测试通过），但 WAF/监控/网关无法按 HTTP 401 识别未授权扫描，且不符合 REST 语义。属设计取舍，记录为加固项。

#### [P2-6] 退款 / 钱包调整无显式幂等键 — idempotency — `admin_refund_handler.go:217,266`
- **证据**：退款/手动退款/钱包调整依赖状态机（`ErrOrderStatusInvalid`）与金额上限（`ErrWalletRefundExceeded`）防重，未提供幂等键（Idempotency-Key）。管理员重复点击/网络重试时，依赖服务层订单状态校验兜底。
- **修复建议**：对财务写接口引入幂等键或基于退款记录唯一约束，避免双击重复退款。

#### [P2-7] JWT 无独立 refresh token 轮换机制 — JWT — `config/config.go:334-337`
- **现状（合规项）**：HS256 强制（`middleware.go:436-438` `WithValidMethods(["HS256"])]`，防 alg=none/RS256 混淆）；默认过期 24h（remember-me 168h）；有 `TokenVersion` + `TokenInvalidBefore` 吊销机制（`middleware.go:203-235`），改密/改邮箱可使旧 token 失效。
- **残余**：access token 24h 内无 refresh 轮换，丢失后只能等到期或改密吊销。可接受，记录为 backlog。

### 已核查通过的安全项（无发现）

- **API Key 泄漏**：支付渠道敏感配置键（`api_secret`/`webhook_secret`/`private_key`/`client_secret` 等 12 个）在 admin 响应中统一脱敏为 `••••••••`（`admin_channel_handler.go:21-35`，测试 `admin_handler_test.go:244-251` 验证）。
- **Webhook 签名**：DujiaoPay 校验 HMAC（`dujiaopay ParseWebhook`，测试 `dujiaopay_test.go:313`）；WeChat 校验 `Wechatpay-Signature/Timestamp/Nonce/Serial`（`wechat_callback.go:63`）；Stripe/PayPal 委托服务层 `Handle*Webhook`。
- **SQL 注入**：全部 GORM 查询参数化；`gorm.Expr` 均用 `?` 占位（`manual_stock.go:18-53`、`coupon/store.go:148` 等）；无 `Where(fmt.Sprintf(...))` 或用户输入拼接（Grep 零命中）。migrations 中的 Raw 为静态 DDL。
- **Mass assignment**：handler 一律使用显式 Request DTO（如 `AdminRefundOrderToWalletRequest`、`CreateOrderAndPayRequest`），未将 request body 直接绑定到 GORM model。
- **登录验证码**：`UserLogin` 强制 `captcha.Verify(CaptchaSceneLogin,...)`（`user_login_handler.go:150`），ErrRequired/ErrInvalid 均拒绝。
- **密码策略**：有 `passwordpolicy.Validate`，弱密码拒绝（`respondWeakPassword`）。
- **Trusted Proxies**：禁止 `0.0.0.0/0` 信任（`router.go:310-312`）。

---

## 总结

### P0 修复清单
无。

### P1 修复清单
1. **退款写路由挂 PaymentCompliance**：将 `ordertransport.RegisterAdminRefundWriteRoutes`（`routes_admin.go:162`）的接收者由 `authorized` 改为 `paymentProtected`，使 `refund-to-wallet` / `manual-refund` / `payment-fee` 与其他财务写操作一致受合规声明闸门保护。
2. **敏感端点补限流**：为 `POST /auth/send-verify-code`、`POST /auth/register`、`POST /auth/forgot-password`（及 google redirect callback/exchange）挂 `middleware.RateLimitMiddleware`（按 IP+email），封堵邮件轰炸与批量注册。

### P2 backlog
1. 移除 `defaultBootstrapPassword="admin123"` 硬编码回退（或强制 release 报错）。
2. 删除 `bootstrap.go:49` 明文密码日志。
3. 密码重置对「用户不存在」与「验证码错误」返回统一响应，消除账号枚举。
4. 生产强制显式配置 CORS allowed_origins，禁止 `*` + credentials 组合。
5. 评估将未认证响应改为真实 HTTP 401/403 状态码（利于网关监控）。
6. 退款/钱包调整引入幂等键防双击。
7. 评估 access token refresh 轮换机制（当前 24h + TokenVersion 吊销，可接受）。

### 总体结论
HCZ 的安全基线整体健康：Admin RBAC 对 252 条路由全覆盖并有自动化回归测试；User 侧订单/售后/支付/钱包 IDOR 全部 fail-closed 且非本人统一 404 不暴露存在性；Guest/Legacy Payment 残留 ACTIVE_BLOCKER=0；JWT 算法固定 HS256、Webhook 验签、SQL 参数化、敏感 key 脱敏均到位。**上线前必须处理 2 项 P1**（退款 Step-Up 缺口 + 敏感端点限流缺失）。
