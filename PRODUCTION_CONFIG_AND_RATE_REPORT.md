# HCZ 生产配置清单审计 + Global Rate fail-closed 验证报告

- 审计日期：2026-10-05
- 项目根：`E:\Users\orang\Downloads\Compressed\hcz_v1`
- Go module：`github.com/Aether-v1/hcz`
- 配置真源：YAML 启动配置 `config.yml`（结构体见 `internal/config/config.go`）+ 数据库 `settings` KV 运行时配置（后台可改，不在 YAML）

> 关键架构事实：`internal/config/config.go:16-36` 的 `Config` 结构体**只包含** App/Server/Log/Database/JWT/UserJWT/Bootstrap/TelegramAuth/GoogleAuth/Redis/Queue/Upload/CORS/Security/Email/Order/Captcha/Web/Reseller 这 19 段。
> **CoinGecko API Key、手动汇率、提现配置、C2C 配置、Site Builder、工单限额均不在 YAML**，而是运行时写入数据库 `settings` KV（后台管理员配置）。本报告分别列出两类。

---

## 一、YAML 生产配置清单（config.yml）

分类说明：
- **REQUIRED**：生产必须显式设置，不能用默认值/示例值
- **RECOMMENDED**：建议生产调整，不调不会崩但有风险
- **OPTIONAL**：按业务需要开启

### 1.1 安全密钥类（最高优先级）

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `app.secret_key` | AES-256 加密敏感数据（`config.go:40`） | `your-secret-key-change-in-production-please` | `openssl rand -hex 32` 强随机串，3 个密钥彼此不同 | REQUIRED | 是：example 占位串 |
| `jwt.secret` | 后台管理 JWT 签名（`config.go:91`） | `your-secret-key-change-in-production-please` | 独立强随机串（≠ app.secret_key） | REQUIRED | 是：与 app.secret_key 同串即违规 |
| `user_jwt.secret` | C 端用户 JWT 签名（`config.go:91`） | `user-secret-key-change-in-production-please` | 独立强随机串（≠ 上面两个） | REQUIRED | 是：三者必须不同 |

> 代码注释 `config.yml.example:2-4` 明确："首次启动前必须把下面三个占位密钥替换为彼此不同的强随机值，否则服务会拒绝启动"。

### 1.2 服务器/日志类

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `server.host` | 监听地址 | `0.0.0.0` | `0.0.0.0`（容器）或内网网卡 IP | RECOMMENDED | 无 |
| `server.port` | 监听端口 | `8080` | 按部署，反代后保持 8080 | OPTIONAL | 无 |
| `server.mode` | Gin 运行模式 | `debug` | **`release`** | REQUIRED | 是：debug 泄露路由/栈 |
| `server.trusted_proxies` | 信任的反代网段 | `127.0.0.1/32, ::1/128` | **真实反代网段**（Docker/CDN 必须改），禁止 `0.0.0.0/0` | REQUIRED | 是：见 example 注释 |
| `log.dir` | 日志目录 | `""`（运行目录/logs） | 挂载持久化卷路径 | RECOMMENDED | 无 |
| `web.admin_path` | 后台路由前缀 | `/admin` | 改成不可猜测串如 `/dj-mgmt-7x9k2` | RECOMMENDED | 弱：`/admin` 易被扫描 |

### 1.3 数据库

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `database.driver` | 驱动 | `sqlite` | **`postgres`** | REQUIRED | 是：sqlite 不适合生产并发 |
| `database.dsn` | 连接串 | `./db/hcz.db` | Postgres DSN，含 sslmode=require、生产账号、强密码 | REQUIRED | 是：文件路径/sqlite |
| `database.pool.max_open_conns` | 最大连接 | `1` | 10~50（按 PG 规格） | REQUIRED | 是：sqlite 默认 1 连接 |
| `database.pool.max_idle_conns` | 空闲连接 | `1` | 与 max_open 匹配 | RECOMMENDED | 是 |
| `database.pool.conn_max_lifetime_seconds` | 连接最大存活 | `0`（不限制） | 1800~3600 | RECOMMENDED | 无 |
| `database.pool.conn_max_idle_time_seconds` | 空闲回收 | `0` | 600~900 | RECOMMENDED | 无 |

> 注意：config.go 与 example 中**没有** `database.timezone` / `database.sslmode` 独立字段——sslmode 必须写在 DSN 串里（如 `... sslmode=require&TimeZone=Asia/Shanghai`）。

### 1.4 Redis / Queue

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `redis.host` | 缓存 Redis | `127.0.0.1` | 生产 Redis 内网地址 | REQUIRED | 是：localhost |
| `redis.port` | 端口 | `6379` | 生产端口 | REQUIRED | 无 |
| `redis.password` | Redis 密码 | `""` | **必填强密码** | REQUIRED | 是：无密码裸奔 |
| `redis.db` | DB 号 | `0` | 与其他服务隔离的 db | RECOMMENDED | 无 |
| `redis.prefix` | key 前缀 | `dj` | 保持默认或改站点专属 | OPTIONAL | 无 |
| `queue.host` | 队列 Redis（asynq） | `127.0.0.1` | 生产 Redis（可与 redis.* 同实例不同 db） | REQUIRED | 是：localhost |
| `queue.port` | 队列端口 | `6379` | 生产端口 | REQUIRED | 无 |
| `queue.password` | 队列 Redis 密码 | `""` | **必填** | REQUIRED | 是：无密码 |
| `queue.db` | 队列 db | `1` | 与 redis.db 不同 | RECOMMENDED | 无 |
| `queue.concurrency` | worker 并发 | `10` | 按 CPU/上游能力调 | RECOMMENDED | 无 |
| `queue.upstream_sync_interval` | 上游库存同步间隔 | `5m` | 保持或按上游限流调 | OPTIONAL | 无 |

### 1.5 管理员引导 / 登录

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `bootstrap.default_admin_username` | 首次初始化管理员 | `""` | **留空**，首次启动后手动建号；或填一次性强用户名 | REQUIRED | 是：dev `admin` |
| `bootstrap.default_admin_password` | 首次初始化管理员密码 | `""` | **留空**或一次性强密码（用后立即删配置/改密） | REQUIRED | 是：dev `HczDev@2026` |
| `telegram_auth.*` | Telegram 登录 | `enabled:false` | 按业务开启，填 bot_token/client_secret | OPTIONAL | token 是敏感值 |
| `google_auth.*` | Google 登录 | `enabled:false` | 按业务开启，登记正确 client_id | OPTIONAL | 无 |

### 1.6 邮件 SMTP

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `email.enabled` | 邮件开关 | `true`(example) / `false`(本地 config.yml) | 生产必须 `true`（验证码/通知依赖） | REQUIRED | 是：关邮件则注册/找回不可用 |
| `email.host` | SMTP 主机 | `smtp.xxx.com` | 真实 SMTP（如 smtp.exmail.qq.com） | REQUIRED | 是：占位 |
| `email.port` | SMTP 端口 | `465` | 465(SSL) / 587(TLS) | REQUIRED | 无 |
| `email.username` | SMTP 账号 | `your-username` | 真实账号 | REQUIRED | 是：占位 |
| `email.password` | SMTP 密码/授权码 | `your-password` | 真实授权码 | REQUIRED | 是：占位 |
| `email.from` | 发件地址 | `your-email` | 与 username 一致的已验证发件箱 | REQUIRED | 是：占位 |
| `email.use_ssl` / `use_tls` | 加密方式 | example: ssl=true/tls=false | 与端口匹配（465→ssl true，587→tls true） | RECOMMENDED | 无 |

> 注意：`internal/modules/settings/schema/messaging/smtp.go` 还存在一份后台可改的 SMTP 配置（settings KV）。若后台已配置 SMTP，运行时会以后台值为准；YAML `email.*` 是首次引导/默认值。生产部署后应在后台复核一次。

### 1.7 CORS

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `cors.allowed_origins` | 允许的前端来源 | `["*"]` | **精确域名列表**，如 `https://shop.example.com` | REQUIRED | 是：`*` + `allow_credentials:true` 是危险组合 |
| `cors.allow_credentials` | 携带凭证 | `true` | 保持 true，但 origins 绝不能用 `*` | REQUIRED | 是 |
| `cors.allowed_methods/headers` | 方法/头白名单 | 全套默认 | 保持默认即可 | OPTIONAL | 无 |

### 1.8 上传 / 订单 / 安全策略

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `upload.max_size` | 上传上限 | 10MB | 保持或调小 | OPTIONAL | 无 |
| `upload.allowed_types/extensions` | 类型白名单 | jpg/png/gif/webp | **不要开启 svg**（example 已注释掉） | RECOMMENDED | svg 可携脚本 |
| `order.payment_expire_minutes` | 支付超时 | 15 | 10~30 按业务 | OPTIONAL | 无 |
| `order.max_refund_days` | 退款窗口 | 30 | 按业务 | OPTIONAL | 无 |
| `security.login_rate_limit.*` | 登录限流 | 5 次/5min 封 15min | 保持默认，可更严 | RECOMMENDED | 无 |
| `security.password_policy.*` | 密码强度 | 8 位+大小写+数字 | 建议 min_length≥10，require_special=true | RECOMMENDED | 弱 |

### 1.9 分销（Reseller，可选开启）

| 配置路径 | 用途 | example 默认值 | 生产要求值 | 分类 | 禁止值风险 |
|---|---|---|---|---|---|
| `reseller.enabled` | 分销总开关 | `false` | 按需开 | OPTIONAL | 无 |
| `reseller.main_hosts` | 主站域名 | `localhost,127.0.0.1,::1` | **真实主站域名** | REQUIRED（若开分销） | 是：localhost |
| `reseller.subdomain_base` | 二级域基础域 | `""` | 配好 wildcard DNS/TLS 的域名 | REQUIRED（若开分销） | 无 |
| `reseller.settlement_confirm_days` | 利润确认期 | 7 | 按风控调 | RECOMMENDED | 无 |

---

## 二、不在 YAML、必须在后台（settings KV）配置的生产项

以下配置**没有 YAML 入口**，首次部署后必须登录后台逐项核对，否则对应功能要么 fail-closed 拒单、要么默认关闭。

### 2.1 全局汇率（Global Rate）—— settings key `global_exchange_rate`
- 持久化位置：`internal/modules/exchangerate/infrastructure/settingsstore/store.go:11`（key=`global_exchange_rate`）
- 字段（`contract/contract.go:20-33`）：`currency` / `auto_rate` / `manual_rate` / `api_key`(CoinGecko) / `auto_enabled` / `refresh_interval_min` / `provider`
- **生产动作**：
  1. 后台填入 CoinGecko Demo API Key（`coingecko.go:47` 用 `x_cg_demo_api_key` 头）；
  2. 开启 `auto_enabled=true`；
  3. 预置一个 `manual_rate` 作为冷启动兜底（否则首次拉取失败期间全部拒单）。

### 2.2 提现配置（Withdrawal）—— `schema/security/withdrawal_config.go:13`
- 默认 `enabled=false`（`:32`，"避免静默开放提现"）
- 生产动作：上线提现功能前，后台设置 network/currency/min/max/daily_limit/fixed_fee/require_2fa/new_user_cooldown_hours/first_withdrawal_max。

### 2.3 C2C 配置 —— `schema/integration/c2c.go:25`
- 默认 `enabled=true`（`:39`），trade_timeout 30min，min/max/daily 限额，fee_rate=0
- 生产动作：按风控调 min_trade_usdt / daily_trade_limit_usdt / new_user_cooldown_hours。

### 2.4 Site Builder（站点装修）
- 模块：`internal/modules/sitebuilder/`，DB 驱动（banners / home entries / discovery blocks）
- 无 YAML，首次部署后在后台装修。

### 2.5 工单（Support Ticket）
- 模块：`internal/modules/supportticket/`（ticket/message/category/attachment/audit 五张表）
- 工单分类与处理流程在 DB，无 YAML 限额项；附件上传复用全局 `upload.*` 限制（路由 `/uploads/support_ticket/` 见 `router.go:314`）。

---

## 三、当前 `config.yml`（本地开发）禁止值检查结果

逐项核对（基于本会话实际读取的 `config.yml`）：

| # | 检查项 | 当前值 | 是否违规 | 生产必须覆盖 |
|---|---|---|---|---|
| 1 | `server.mode` | `debug` | **违规** | 改 `release` |
| 2 | `database.driver` | `sqlite` | **违规** | 改 `postgres` |
| 3 | `database.dsn` | `./db/hcz.db` | **违规** | 改 PG DSN |
| 4 | `database.pool.max_open_conns` | `1` | **违规** | 改 10~50 |
| 5 | `redis.host` | `127.0.0.1` | **违规** | 改生产 Redis 地址 |
| 6 | `redis.password` | `""` | **违规** | 必须填强密码 |
| 7 | `queue.host` | `127.0.0.1` | **违规** | 改生产 Redis 地址 |
| 8 | `queue.password` | `""` | **违规** | 必须填强密码 |
| 9 | `cors.allowed_origins` | `["*"]`（且 `allow_credentials:true`） | **违规** | 改精确域名 |
| 10 | `bootstrap.default_admin_username` | `admin` | **违规（dev 凭证）** | 改强用户名或留空 |
| 11 | `bootstrap.default_admin_password` | `HczDev@2026` | **违规（dev 凭证）** | 改一次性强密码或留空 |
| 12 | `email.enabled` | `false` | **违规** | 改 `true` |
| 13 | `email.host/username/password/from` | `smtp.xxx.com` / `your-*` 占位 | **违规** | 填真实 SMTP |
| 14 | `reseller.main_hosts` | `localhost/127.0.0.1/::1` | 若启用分销则违规 | 改真实域名 |
| 15 | `app.secret_key` | 64-hex（本机生成，非 example 占位） | 非占位串，但**是开发机值** | 生产必须重新生成，不得复用 |
| 16 | `jwt.secret` | 64-hex（与上面不同） | 同上 | 重新生成 |
| 17 | `user_jwt.secret` | 64-hex（三者互不相同） | 同上 | 重新生成 |
| 18 | `server.trusted_proxies` | `127.0.0.1/32, ::1/128` | 同机反代可用；Docker/CDN 需改 | 按实际反代网段 |
| 19 | `web.admin_path` | `/admin` | 弱违规 | 建议改不可猜测串 |

**结论：当前 `config.yml` 不可用于生产，共 14 项硬性违规 + 3 项密钥需重新生成。**

---

## 四、Global Rate / exchangerate 模块 5 项逻辑验证

### 4.1 CoinGecko auto rate 定时拉取 —— ✅ 存在

- 定时任务注册：`internal/app/jobs/service.go:109-117`
  - `:111` `scheduler.Register("@every 5m", task, ...)` —— **硬编码每 5 分钟**刷新一次。
- 任务消费者：`internal/app/jobs/consumer/consumer_exchangerate.go:13-23`
  - `:17` 调 `ExchangeRateService.Refresh(ctx)`；失败仅 Warn（`:18-19`），不重试堆积，下一周期再拉。
- CoinGecko 请求：`internal/modules/exchangerate/infrastructure/provider/coingecko.go:45`
  - `GET {base}/simple/price?ids=tether&vs_currencies={siteCurrency}`，API Key 通过 `x_cg_demo_api_key` 传（`:46-48`）。
- Redis 缓存 key：`internal/modules/exchangerate/infrastructure/rediscache/cached_store.go:12` = `"global_exchange_rate:state"`。
- ⚠️ **发现**：`State.RefreshIntervalMin`（`contract.go:32`）虽可由后台配置（`service.go:146`），但 scheduler 注册间隔在 `jobs/service.go:111` 写死 `@every 5m`，**未读取该后台配置**。当前后台改"刷新间隔"不生效。

### 4.2 Redis 缓存 —— ✅ 存在，读穿+写穿

- 文件：`internal/modules/exchangerate/infrastructure/rediscache/cached_store.go`
- `:12` cacheKey = `global_exchange_rate:state`
- `:15` cacheTTL = **2 分钟**
- `:27-42` GetState：先读 Redis（`:31`），miss/故障回落 settings KV（`:34-35`，注释明确"绝不 1:1"），再回填 Redis（`:40`）。
- `:44-52` SaveState：写穿 settings 真源后立即刷新 Redis。

### 4.3 manual fallback（手动兜底汇率） —— ✅ 支持

- 优先级：`internal/modules/exchangerate/application/service.go:52-66`
  - `:53-59` AUTO 有效且未过期 → 用 AUTO
  - `:60-66` 否则 `ManualRate > 0` → 用 MANUAL（Source=`SourceManual`）
- 后台设置入口：`service.go:114-127` `SetManual(rate)`，rate<=0 表示清除。
- 持久化：`settingsstore/store.go:40`（`manual_rate` 字段）。
- 单测证据：`service_test.go:57-71` `TestResolveFallsBackToManualWhenAutoStale`。

### 4.4 Provider 失败时的 fallback —— ✅ 回退手动率/上次成功自动率，不抛 1:1

- `service.go:94-100`：CoinGecko Fetch 失败时，只写 `LastError="provider_fetch_failed"`（`:97`），**不覆盖 AutoRate/AutoFetchedAt**，保留上一次成功的自动率。
- 自动率新鲜度窗口：`internal/app/container/exchange_rate_wiring.go:13` `exchangerateStaleDuration = 10 * time.Minute`。
- 即：CoinGecko 挂了 ≤10min 内继续用上次成功的自动率；超过 10min 自动率视为失效，再走手动兜底；都没有才拒单。
- 单测证据：`service_test.go:81-94` `TestRefreshRecordsProviderErrorWithout1to1`（Refresh 失败后 Resolve 仍 fail-closed，绝不补 1:1）。

### 4.5 fail-closed（最关键） —— ✅ PASS，无静默 1:1 兜底

下单链路证据链：

1. **生产容器无条件注入 Resolver**：`internal/app/container/services_integration.go:99-105`
   - `:99-104` 构造 `ExchangeRateService`（CoinGecko + CachedStore + siteCurrency + 10min staleness）
   - `:105` `c.OrderService.SetRateResolver(exchangerateResolverAdapter{svc: c.ExchangeRateService})`
2. **下单时调用并拒单**：`internal/modules/order/application/order_service.go:530-537`
   - `:531` `rRate, rSrc, rAt, rErr := s.rateResolver.Resolve(...)`
   - `:532-534` `if rErr != nil { return nil, rErr }` —— **无有效汇率直接拒单**
   - `:535-537` `if rRate <= 0 { return nil, ErrInsufficientBalance }` —— 非正汇率也拒单
   - `:538` 才用有效 rate 做 `TotalAmount.Div(rRate).Round(2)` 换算，快照写入订单（`:621-623` ExchangeRate/Source/At）。
3. **错误定义**：`internal/modules/exchangerate/domain/rate.go:26-28`
   - `ErrRateUnavailable`，注释明确："业务方必须 fail-closed 拒绝下单，禁止任何 1:1 / 0 / Gateway Rate 兜底"。
4. **用例层 fail-closed**：`service.go:67-68` 自动失效 + 无手动率 → 返回 `ErrRateUnavailable`。
5. **单测证据**：`service_test.go:73-79` `TestResolveFailClosedWhenNothingAvailable` 断言空状态下 Resolve 必返回 `ErrRateUnavailable`。

⚠️ **唯一需要留意的旁路**：`order_service.go:530` 是 `if s.rateResolver != nil`。当 Resolver 为 nil（老测试/未走生产 bootstrap）时会跳过汇率换算、`orderUsdtTotal=0`。但生产容器 `services_integration.go:105` 无条件注入，且 `order_service.go:60` 注释明确"生产 bootstrap 必须注入"——**生产路径不存在该旁路**。

> 关于 `internal/modules/payment/integrationtest/exchange/exchange_test.go`：该文件测的是 **Payment Gateway 回调金额/币种校验**（USD→CNY 转换后回调金额必须等于转换后金额，否则 `ErrPaymentAmountMismatch`），**与 Global Rate 下单 fail-closed 无直接关系**。Global Rate 的 fail-closed 单测位于 `internal/modules/exchangerate/application/service_test.go`。

### fail-closed 最终判定：**PASS**

- 自动率失败/过期 + 手动兜底存在 → 用手动率；
- 两者都无 → `ErrRateUnavailable` → 下单接口直接返回错误拒单；
- 全代码路径**未发现**任何静默 1:1 / 0 / Gateway Rate 兜底。

---

## 五、生产 config.yml 模板

> 基于 `config.yml.example`，所有占位符用 `__CHANGE_ME__`，driver=postgres，mode=release，CORS 改具体域名，移除 dev 密码。**本模板不覆盖当前 `config.yml`，仅作交付参考。**

```yaml
# HCZ 生产配置模板（所有 __CHANGE_ME__ 必须替换）
# 三个密钥必须彼此不同：openssl rand -hex 32 执行三次

app:
  secret_key: __CHANGE_ME_OPENSLL_HEX32_1__
  totp_issuer: HCZ

server:
  host: 0.0.0.0
  port: 8080
  mode: release
  trusted_proxies:
    - __CHANGE_ME_REAL_PROXY_CIDR__/32   # 例如 Nginx 同机: 127.0.0.1/32；Docker: 172.16.0.0/12；禁止 0.0.0.0/0

log:
  dir: "/var/log/hcz"
  filename: app.log
  max_size_mb: 100
  max_backups: 7
  max_age_days: 30
  compress: true

database:
  driver: postgres
  dsn: "host=__CHANGE_ME_PG_HOST__ port=5432 user=__CHANGE_ME_PG_USER__ password=__CHANGE_ME_PG_PASSWORD__ dbname=hcz sslmode=require TimeZone=Asia/Shanghai"
  pool:
    max_open_conns: 20
    max_idle_conns: 10
    conn_max_lifetime_seconds: 1800
    conn_max_idle_time_seconds: 600

jwt:
  secret: __CHANGE_ME_OPENSLL_HEX32_2__
  expire_hours: 24

user_jwt:
  secret: __CHANGE_ME_OPENSLL_HEX32_3__
  expire_hours: 24
  remember_me_expire_hours: 168

bootstrap:
  # 生产建议留空，首次启动后手动建管理员；或填一次性强密码，建号后立即删除本段并改密
  default_admin_username: ""
  default_admin_password: ""

telegram_auth:
  enabled: false
  bot_username: ""
  bot_token: ""
  client_secret: ""
  oidc_redirect_uri: ""
  mini_app_url: ""
  login_expire_seconds: 300
  replay_ttl_seconds: 300

google_auth:
  enabled: false
  client_id: ""

redis:
  enabled: true
  host: __CHANGE_ME_REDIS_HOST__
  port: 6379
  password: __CHANGE_ME_REDIS_PASSWORD__
  db: 0
  prefix: "dj"

queue:
  enabled: true
  host: __CHANGE_ME_REDIS_HOST__
  port: 6379
  password: __CHANGE_ME_REDIS_PASSWORD__
  db: 1
  concurrency: 10
  queues:
    default: 10
    critical: 5
  upstream_sync_interval: "5m"

upload:
  max_size: 10485760
  allowed_types:
    - image/jpeg
    - image/png
    - image/gif
    - image/webp
  allowed_extensions:
    - .jpg
    - .jpeg
    - .png
    - .gif
    - .webp
  max_width: 4096
  max_height: 4096

cors:
  allowed_origins:
    - "https://__CHANGE_ME_SHOP_DOMAIN__"
    # 若启用分销白标，把每个白标域名也列在这里
  allowed_methods:
    - GET
    - POST
    - PUT
    - PATCH
    - DELETE
    - OPTIONS
  allowed_headers:
    - Content-Type
    - Content-Length
    - Accept-Encoding
    - Authorization
    - Cache-Control
    - X-Requested-With
    - X-CSRF-Token
  allow_credentials: true
  max_age: 600

security:
  login_rate_limit:
    window_seconds: 300
    max_attempts: 5
    block_seconds: 900
  password_policy:
    min_length: 10
    require_upper: true
    require_lower: true
    require_number: true
    require_special: true

email:
  enabled: true
  host: __CHANGE_ME_SMTP_HOST__
  port: 465
  username: __CHANGE_ME_SMTP_USER__
  password: __CHANGE_ME_SMTP_PASSWORD__
  from: __CHANGE_ME_SMTP_FROM__
  from_name: HCZ
  use_tls: false
  use_ssl: true
  verify_code:
    expire_minutes: 10
    send_interval_seconds: 60
    max_attempts: 5
    length: 6

order:
  payment_expire_minutes: 15
  max_refund_days: 30

reseller:
  enabled: false
  main_hosts:
    - "__CHANGE_ME_MAIN_DOMAIN__"
  trusted_forwarded_host: false
  subdomain_base: ""
  self_apply_enabled: true
  settlement_confirm_days: 7

web:
  admin_path: "/__CHANGE_ME_UNGUESSABLE_ADMIN_PATH__"
```

---

## 六、生产上线前 Checklist（YAML 之外别忘了）

1. 后台 → 全局汇率：填 CoinGecko API Key、开 `auto_enabled`、**预置 manual_rate 兜底**。
2. 后台 → 提现：默认 `enabled=false`，按需开启并配限额/费率/2FA。
3. 后台 → C2C：按风控调 min/max/daily/cooldown。
4. 后台 → SMTP：核对一次（与 YAML 双写，以后台为准）。
5. `uploads/` 与 `db/`（切 PG 后无 db 文件，但 uploads 目录）必须挂持久化卷。
6. 三个密钥确认彼此不同；bootstrap 建号后立即清空配置里的明文密码。
7. 观察日志 `exchange_rate_refresh_ok` / `exchange_rate_refresh_failed`，确认 5 分钟一次的拉取在跑；断网时订单应报 `exchange_rate_unavailable` 而不是按 1:1 放行。
