# HCZ 基础设施 / 数据库 / 全局安全扫描审计报告

- 审计日期：2026-10-07
- 项目：github.com/Aether-v1/hcz（Go 1.26.5 + Gin + GORM 1.31.1 + go-redis v9 + Asynq + shopspring/decimal）
- 路径：`E:\Users\orang\Downloads\Compressed\hcz_v1`
- 审计方式：只读代码审计 + 只读 schema 提取 + 真实执行 `go build` / `go vet` / `go test`（内存 SQLite，未触碰 `db/hcz.db`）
- 硬规则遵守：未修改任何项目源码；测试使用 `mode=memory` SQLite，未创建/修改 `db/hcz.db`

---

## 总体结论

**FAIL — 项目当前工作区无法编译。** `internal/modules/points` 包存在缺失符号，导致 `go build ./...` 与 `go vet ./...` 均以 exit code 1 失败。资金核心模块（wallet / order / c2c / payment / migrations）的单元与集成测试全部 PASS，但 points 模块的断点会阻断整个二进制构建与部署。

数据库 schema 整体健康：金额字段全部为 `decimal(20,2)`（无 float），关键幂等字段（`wallet_transactions.reference`、`orders.order_no`、`orders.user_idempotency_key`、`c2c_trades.idempotency_key`、`affiliate_commission_records`）均有唯一索引；出站回调 SSRF 有 dial 层内网 IP 拦截；密钥（site_connection api_secret、channel_client channel_secret/bot_token）AES-256-GCM 加密落库；未发现 SQL 注入。

---

### ISSUE-I01
- Severity: **P0**
- Title: points 模块编译失败，整个项目无法构建部署
- File:
  - `internal/modules/points/transport/http/routes.go:35`
  - `internal/modules/points/infrastructure/gormstore/store.go:19,37`
- Function/Method: `RegisterAdminRoutes` / `NewStore`
- API: `POST /admin/users/:id/points/compensate`
- Table: 不涉及（编译期）
- Root Cause: `routes.go` 引用了 `(*AdminHandler).CompensateUserPoints`，但 `admin_handler.go` 中并未定义该方法；同时 `*gormstore.Store` 未实现 `contract.Repository` 接口（缺方法 `ListAccounts`）。属于进行中但未完成的重构/开发状态。
- Exploit/Trigger: 任何 `go build ./...` / `go build -o hcz ./cmd/...` / `go test ./...` 全量构建。
- Impact: 生产二进制无法产出；CI 必然红；无法部署。这是阻断级工程问题，不是安全漏洞，但按"生产就绪"标准直接 BLOCK。
- Evidence:
  ```
  internal/modules/points/transport/http/routes.go:35:52: h.CompensateUserPoints undefined
      (type *AdminHandler has no field or method CompensateUserPoints)
  internal/modules/points/infrastructure/gormstore/store.go:19:35:
      *Store does not implement contract.Repository (missing method ListAccounts)
  ```
- Recommended Fix:
  1. 在 `admin_handler.go` 中补全 `CompensateUserPoints` 方法（或临时从 `routes.go:35` 摘除该路由）。
  2. 在 `gormstore/store.go` 中实现 `ListAccounts(ctx, filter)` 以满足 `contract.Repository`。
  3. 修复后重跑 `go build ./...` 与 `go vet ./...`，确认 exit 0。

---

### ISSUE-I02
- Severity: **P1**
- Title: 工作区 config.yml 含真实密钥与默认管理员弱口令
- File: `config.yml`（当前工作区，2722 字节）
- Function/Method: 不涉及
- API: 不涉及
- Table: 不涉及
- Root Cause: `config.yml` 中存在已填好的 `app.secret_key`、`jwt.secret`、`user_jwt.secret`，以及 `bootstrap.default_admin_password: "HczDev@2026"`。
- Exploit/Trigger: 若该文件被随镜像/部署包分发，或服务器被读，攻击者可直接伪造 JWT 并以默认口令登录管理员后台。
- Impact: 凭证泄露 → 完全接管。
- Evidence（仅定性，不在报告中复述具体值）:
  - `config.yml` 命中 `secret_key` / `jwt.secret` / `user_jwt.secret` / `bootstrap.default_admin_password` 均为非占位真实值。
  - `.gitignore:10` 已正确排除 `config.yml`；`git ls-files` 确认 `config.yml` **未被 git 跟踪**，`git log --all -- config.yml` 为空——**历史无泄露**。
- Recommended Fix:
  1. 生产环境必须通过环境变量/secret 管理注入，禁止把含真实 secret 的 config.yml 带入镜像。
  2. 首次启动后立即修改默认管理员口令；或在 bootstrap 检测到默认口令时强制要求修改（代码已在 `bootstrap.go:50-53` 打 warn 日志，建议升级为强制改密）。
  3. 轮换当前 `secret_key` / `jwt.secret`（因工作区已暴露在本地，按"已泄露"处置更稳妥）。

---

### ISSUE-I03
- Severity: **P1**
- Title: CORS 配置 `allowed_origins: ["*"]` 与 `allow_credentials: true` 同时开启
- File: `config.yml`（cors 段）
- Function/Method: Gin CORS 中间件初始化
- API: 全站
- Table: 不涉及
- Root Cause: 通配源与凭证模式共存。浏览器规范会拒绝 `Access-Control-Allow-Origin: *` + `credentials: include` 的响应，因此实际效果是"跨域认证请求被静默阻断"，同时运维误以为开放了跨域。
- Exploit/Trigger: 前端部署在与 API 不同源的环境时，登录态/带 cookie 请求失败；若中间件实现为"反射 Origin"则退化为任意源携带凭证。
- Impact: 功能故障 / 潜在跨域凭证泄露（取决于中间件具体实现）。
- Evidence: `config.yml` 中 `cors.allowed_origins: ["*"]`、`cors.allow_credentials: true`。
- Recommended Fix: 生产环境把 `allowed_origins` 显式枚举为前端实际域名列表；仅在确需凭证时保留 `allow_credentials: true`，且不得使用通配符。

---

### ISSUE-I04
- Severity: **P2**
- Title: Bot 通知出站回调未走 safe dial（SSRF 防护不一致）
- File: `internal/app/jobs/consumer/consumer_bot.go:104`
- Function/Method: `Consumer.notifyBotOrderFulfilled`（及 wallet-recharge-succeeded 分支）
- API: 内部 Asynq worker → channel_clients.callback_url
- Table: `channel_clients.callback_url`
- Root Cause: 下游订单回调 `downstreamcallback` 用了 `newSafeHTTPClient()`（在 dial 层拦截 10/8/127/169.254/::1），但同文件的 Bot 通知路径直接 `&http.Client{Timeout: 10s}`，没有任何内网地址拦截。
- Exploit/Trigger: 一个被授予"渠道客户端"配置权限的账号，把 `callback_url` 指向 `http://169.254.169.254/latest/meta-data/` 或 `http://127.0.0.1:内部管理端口/`，worker 会向其 POST 签名后的订单 JSON。
- Impact: 内网服务探测 / 云元数据泄露（虽有签名，但内网服务可能不校验签名）。
- Evidence:
  ```go
  // consumer_bot.go:104
  httpClient := &http.Client{Timeout: 10 * time.Second}
  resp, err := httpClient.Do(req)
  ```
  对比 `downstreamcallback/infrastructure/callbackclient/transport.go` 中存在的 `safeDialContext` 内网拦截逻辑未在此处复用。
- Recommended Fix: 抽出共享的 `newSafeHTTPClient()`，所有配置型出站 HTTP（bot 通知、upstream adapter、payment gateway）统一复用；至少对 `callback_url` 做 scheme/host 校验 + 内网 IP 拦截。

---

### ISSUE-I05
- Severity: **P2**
- Title: upstream/dujiao_next 适配器与 payment gateway 出站客户端未做内网拦截
- File:
  - `internal/upstream/dujiao_next.go:63-65`
  - `internal/modules/payment/infrastructure/gateway/*`（各 adapter 自建 `&http.Client{...}`）
- Function/Method: `newDujiaoNextClient` / 各 gateway 的 `NewHTTPClient`
- API: 管理员配置的站点对接 base_url / 支付网关 gateway_url
- Table: `site_connections.base_url`、`payment_channels.gateway_url`
- Root Cause: 这些 URL 由管理员（或低权限运营）在后台填写，代码用默认 `http.Client` 直连，未复用 `downstreamcallback` 的 safeDial。
- Exploit/Trigger: 具备渠道/支付通道配置权限的账号填内网地址，触发出站请求（下单/回调/对账），可探测内网或访问云元数据。
- Impact: SSRF（管理员面，非终端用户面）。
- Evidence:
  ```go
  // dujiao_next.go:63-65
  return &http.Client{
      Timeout: 30 * time.Second,
  }
  ```
- Recommended Fix: 与 I04 一并收敛：所有"用户/管理员可配置 URL"的出站 HTTP 统一走 safe dial + 内网网段黑名单 + DNS rebinding 防护（在 dial 时重新解析 IP，本项目 downstreamcallback 已实现，可直接复用）。

---

### ISSUE-I06
- Severity: **P3**
- Title: C2C trade_no 后缀使用 math/rand 4 位数字，可预测
- File: `internal/modules/c2c/application/service.go:97-100`
- Function/Method: `generateTradeNo`
- API: `POST /c2c/trades`
- Table: `c2c_trades.trade_no`（已有 UNIQUE 索引 `idx_c2c_trades_trade_no`）
- Root Cause: `randSuffix := rand.Intn(10000)` 用于订单号后缀，全局未 seed（Go 1.20+ math/rand 自动 seed，但仍非加密安全）。
- Impact: 订单号可被枚举/猜测；但 DB 唯一索引兜底，不会导致重复下单；仅为信息泄露面，非资金漏洞。
- Evidence:
  ```go
  // service.go:97
  rand.Seed(time.Now().UnixNano()) // 已弃用写法
  randSuffix := rand.Intn(10000)
  ```
- Recommended Fix: 改用 `crypto/rand` 生成后缀，或直接用 `google/uuid` / 数据库序列；移除已弃用的 `rand.Seed`。

---

### ISSUE-I07
- Severity: **P3**
- Title: 卡密批次号 batch_no 使用 math/rand
- File: `internal/modules/cardsecret/application/import.go`（同包内引用 math/rand）
- Function/Method: 批次号生成
- API: 管理员导入卡密
- Table: `card_secret_batches.batch_no`（UNIQUE `idx_card_secret_batches_batch_no`）
- Root Cause: 批次号非安全凭证，DB 唯一索引兜底。
- Impact: 低。
- Recommended Fix: 统一改用 crypto/rand 或 UUID，消除 math/rand 在业务代码中的使用面。

---

### ISSUE-I08
- Severity: **P3**
- Title: 上游回调失败日志记录 api_key（标识，非 secret）
- File: `internal/modules/upstreamapi/transport/http/upstream_callback.go:62,95`
- Function/Method: `lookupConnection` / 签名失败日志
- API: 内部回调入口
- Table: 不涉及
- Root Cause: 日志中 `api_key` 是渠道方的公开标识（类似用户名），不是 `api_secret`；`api_secret` 在日志中未出现。
- Impact: 低，但日志聚合后会泄露渠道 key 清单。
- Evidence:
  ```go
  logger.Errorw("upstream_callback_lookup_connection_failed", "api_key", apiKey, ...)
  logger.Warnw("upstream_callback_signature_invalid", "api_key", apiKey)
  ```
- Recommended Fix: 视日志敏感度决定是否截断 api_key（如只记前 4 位 + hash）。

---

### ISSUE-I09
- Severity: **P3**
- Title: 汇率缓存回填错误被静默忽略（设计如此，记录备查）
- File: `internal/modules/exchangerate/infrastructure/rediscache/cached_store.go:40,50`
- Function/Method: `GetState` / `SaveState`
- API: 汇率读取
- Table: 不涉及
- Root Cause: `_ = cache.SetJSON(...)` 忽略 Redis 写入失败。代码注释明确说明"Redis 故障回落 settings 真源，不 1:1"，属读穿/写穿装饰器的有意容错。
- Impact: 无正确性问题；TTL 仅 2 分钟，stale 窗口可控。
- Evidence: `cached_store.go:15 const cacheTTL = 2 * time.Minute`；`SaveState` 写库成功后立即刷新 Redis。
- Recommended Fix: 无需修改；建议在 `SetJSON` 失败时打一条 debug 日志便于排查。

---

### ISSUE-I10
- Severity: **P3**
- Title: safeHTTPClient 仍走 `http.ProxyFromEnvironment`
- File: `internal/modules/downstreamcallback/infrastructure/callbackclient/transport.go`
- Function/Method: `newSafeHTTPClient`
- API: 下游订单回调
- Table: 不涉及
- Root Cause: Transport 同时设置了 `Proxy: http.ProxyFromEnvironment` 与 `DialContext: safeDialContext`。若部署环境设置了 `HTTP_PROXY`，内网地址的请求可能被转发到代理，绕过 dial 层 IP 拦截。
- Impact: 低（需服务器配置了恶意代理才成立）。
- Recommended Fix: 对回调客户端显式 `Proxy: nil`，或仅允许出口代理白名单。

---

## 正面确认（未发现问题的项）

- **金额字段精度**：所有金额列均为 `decimal(20,2)`（shopspring/decimal），未发现 float32/float64 存金额。详见下方"数据库金额字段清单"。
- **幂等唯一约束**：
  - `wallet_transactions.reference` — `uniqueIndex`
  - `orders.order_no` — UNIQUE；`orders(user_id, idempotency_key)` — partial unique
  - `c2c_trades.trade_no` — UNIQUE；`c2c_trades.idempotency_key` — UNIQUE（`idx_c2c_trade_idem`）
  - `downstream_order_refs.order_id` — UNIQUE
  - `card_secret_batches.batch_no` — UNIQUE；`gift_cards.code` — UNIQUE
- **密钥落库加密**：`site_connections.api_secret`、`channel_clients.channel_secret`、`channel_clients.bot_token` 均 AES-256-GCM 加密存储（`internal/crypto/aes.go`），`json:"-"` 不随响应外泄。
- **随机数**：邀请码 `invitecode.go` 使用 `crypto/rand`；未发现把 math/rand 用于 token/session/api_key 生成的位置。
- **SQL 注入**：未发现用户输入拼入 SQL。详见 SQL Injection Matrix。
- **事务锁顺序**：`wallet/infrastructure/gormstore/store.go` 通过 `LockAccountsByUserIDOrder` 按 user_id 升序加锁，避免 wallet↔order 死锁。
- **Recovery**：`middleware/recovery.go` 捕获 panic 并仅写服务端日志（stack 入 zap，不回吐客户端）；客户端断开类 panic 降级为 warn。
- **Self-update**：`internal/selfupdate/updater.go` 强制 https + 主机白名单 + sha256 checksum 校验。
- **Git 历史**：`config.yml` 未被跟踪，历史无真实 secret 泄露。

---

## SQL Injection Matrix

| Input | Endpoint | SQL Path | Bound | Whitelist | Result |
|---|---|---|---|---|---|
| 订单关键词搜索 | 管理后台订单列表 | `order/infrastructure/gormstore/order_store.go` GORM `Where("... LIKE ?", kw)` | 是 | — | SAFE |
| 仪表盘 locale 字段 | dashboard SQL | `dashboard/infrastructure/gormstore/sql.go` `fmt.Sprintf("...->>'$.%s'", locale)` | 参数值绑定 | locale ∈ `SupportedLocales` 常量集 | SAFE |
| 工单搜索 keyword | supportticket | `supportticket/infrastructure/gormstore/ticket_store.go` `.Where("... LIKE ?", kw)` | 是 | — | SAFE |
| 游客密码迁移候选 | order backfill | `order_store.go:164` `guestCredentialBackfillCandidateSQL` | LIKE/GLOB 参数绑定 | 长度/偏移为编译期常量 | SAFE |
| 动态排序 sort/order | 全部 `.Order(...)` 调用 | 全项目 grep | — | 全部硬编码字符串（如 `"orders.created_at DESC"`），无用户输入拼接 | SAFE |
| 动态表名/列名 | GORM 调用 | 全项目 grep | — | 未发现用户可控 table/column | SAFE |
| IN / LIMIT / OFFSET | 分页 | GORM `Limit/Offset` 绑定为整数 | 是 | — | SAFE |

**结论：未发现 SQL 注入。**

---

## 数据库金额字段清单（来自 db/hcz.db 只读提取 + GORM tag 交叉验证）

| Table | Column | Type | Precision |
|---|---|---|---|
| orders | amount / payable_amount | decimal(20,2) | 20,2 |
| orders | discount_amount | decimal(20,2) | 20,2 |
| order_items | unit_price / subtotal | decimal(20,2) | 20,2 |
| wallet_accounts | balance / frozen_balance / available_balance | decimal(20,2) | 20,2 |
| wallet_transactions | amount / balance_before/after / available_* / frozen_* | decimal(20,2) | 20,2 |
| c2c_trades | price / fiat_amount / usdt_amount | decimal(20,2) | 20,2 |
| c2c_listings | price / min_amount / max_amount | decimal(20,2) | 20,2 |
| products | price_amount / cost_price_amount | decimal(20,2) | 20,2 |
| product_skus | price_amount / cost_price_amount | decimal(20,2) | 20,2 |
| coupons | value / min_amount / max_discount | decimal(20,2) | 20,2 |
| promotions | value / min_amount | decimal(20,2) | 20,2 |
| gift_cards | amount | decimal(20,2) | 20,2 |
| member_levels | discount_rate / recharge_threshold / spend_threshold | decimal(6,2)/(20,2) | — |
| member_level_prices | price_amount | decimal(20,2) | 20,2 |
| reseller_balance_accounts | available_amount_cache | decimal(20,2) | 20,2 |
| reseller_ledger_entries | amount | decimal(20,2) | 20,2 |
| reseller_order_snapshots | base_amount / reseller_amount / profit_amount | decimal(20,2) | 20,2 |
| reseller_profiles | default_markup_percent / max_markup_percent | decimal(10,2) | 10,2 |
| reconciliation_items | local_amount / upstream_amount | decimal(20,2) | 20,2 |

未发现 float 类型金额列。

---

## SSRF 攻击面清单

| Endpoint | URL Source | Internal Access | Result |
|---|---|---|---|
| POST /upstream/orders（创建订单带 callback_url） | 调用方提交的 `req.CallbackURL` | dial 层 safeDialContext 拦截 10/8/127/169.254/::1；`validateCallbackURL` 拒绝内网 IP 字面量 | MITIGATED |
| Asynq worker 下游订单回调 | `downstream_order_refs.callback_url` | newSafeHTTPClient + safeDialContext | MITIGATED |
| Asynq worker Bot 通知 | `channel_clients.callback_url`（管理员配置） | **无内网拦截**（I04） | OPEN (P2) |
| upstream/dujiao_next 下单/同步 | `site_connections.base_url`（管理员配置） | **无内网拦截**（I05） | OPEN (P2) |
| payment gateway 各 adapter | `payment_channels.gateway_url`（管理员配置） | **无内网拦截**（I05） | OPEN (P2) |
| selfupdate 下载 | 内置 GitHub release URL | https + 主机白名单 + sha256 | MITIGATED |
| coingecko 汇率 provider | 硬编码公共 API | — | SAFE |

DNS rebinding：downstreamcallback 在 `DialContext` 内重新解析 IP 并校验，已防 rebinding；I04/I05 路径未做，需补。

---

## Tests 执行结果（真实执行）

| 命令 | 结果 | 详情 |
|---|---|---|
| `go build ./...` | **FAIL** | `internal/modules/points/transport/http/routes.go:35:52: h.CompensateUserPoints undefined`；`points/infrastructure/gormstore/store.go:19: *Store missing ListAccounts`。exit 1。 |
| `go vet ./...` | **FAIL** | 同上 points 包编译错误，exit 1。 |
| `go test ./internal/modules/wallet/...` | PASS | 全部 ok（integrationtest / transport/http 18.5s） |
| `go test ./internal/modules/order/...` | PASS | application / aftersale / e2e / refund / gormstore 全部 ok |
| `go test ./internal/modules/c2c/...` | PASS | integrationtest ok 21.6s |
| `go test ./internal/modules/payment/...` | PASS | 全部 gateway adapter / gormstore / integrationtest ok |
| `go test ./internal/bootstrap/...` | **FAIL** | `bootstrap/points [build failed]`（同 I01 编译错误）；migrations 包本身 ok 61.6s；其余 ok |
| `go test -race ./...` | SKIPPED | 因 points 包编译失败，全量 -race 无法运行；资金模块已在单测中通过并发/竞态用例（order/risk_gate_test、wallet integrationtest），但未单独跑 -race。 |
| `govulncheck` | NOT VERIFIED | 环境未确认 govulncheck 可用；依赖扫描不能代替代码审计。 |

说明：测试均使用 `file:<name>?mode=memory&cache=shared` 内存 SQLite，未读写 `db/hcz.db`。

---

## 密钥泄露清单

| 位置 | 内容 | 是否真实可用 | 是否已入库/入历史 | 处置建议 |
|---|---|---|---|---|
| `config.yml` 工作区 | app.secret_key / jwt.secret / user_jwt.secret | 疑似真实可用（非 placeholder） | **未被 git 跟踪**（.gitignore:10，git ls-files 空），git log 无记录 | 轮换；通过环境变量注入 |
| `config.yml` | `bootstrap.default_admin_password: "HczDev@2026"` | 默认口令，若未改则可用 | 未入库 | 首次启动强制改密；上线前确认已改 |
| `config.yml.example` / `config.yml.production` | 全部为 `CHANGE_ME_*` 占位符 | 不可用 | 已入库（仅示例） | 无 |
| Go 源码 | 全项目 grep 硬编码 secret/password/token | **未发现** | — | — |
| `site_connections.api_secret` / `channel_clients.channel_secret` / `bot_token` | 落库为密文 | AES-256-GCM 加密，json:"-" | 已加密 | 无 |

---

## 验证状态汇总

- 数据库 schema（迁移文件 + SQLite 实际 DDL 提取）：**已验证**
- GORM model 金额字段精度：**已验证**
- Redis key 用户隔离（auth_state 按 userID/adminID 命名）：**已验证**
- Redis 汇率缓存 TTL 2 分钟 + 写穿：**已验证**
- Asynq 任务幂等/重试：代码层面看到 retry-max/4xx 不重试/5xx 重试策略；**NOT VERIFIED** 端到端幂等（未跑异步集成测试）
- crypto/rand vs math/rand：**已验证**（仅 c2c/cardsecret 业务号用 math/rand，非安全场景）
- 敏感日志：**已验证**（未发现 password/token/Authorization 入日志；api_key 标识有低风险记录 I08）
- SSRF：**已验证**（downstreamcallback 防护到位；bot/upstream/payment 路径缺失 I04/I05）
- SQL 注入：**已验证**（无注入）
- Panic 面：**已验证**（panic 均在启动期/依赖注入期，运行时由 RecoveryMiddleware 兜底）
- 事务锁顺序：**已验证**（按 user_id 升序）
- go build / vet / test：**已真实执行**，结果见上表
- govulncheck：**NOT VERIFIED**

---

## 阻断项（Deployer Checklist）

1. **必须先修 I01**：points 包编译失败，否则二进制打不出来。
2. 修完 I01 后重跑 `go build ./... && go vet ./... && go test ./...`，要求全绿。
3. 生产部署前：轮换 config.yml 中 secret、确认默认管理员口令已改、CORS 收敛到具体域名。
4. 排期补 I04/I05 的出站 HTTP safe dial 统一封装。
