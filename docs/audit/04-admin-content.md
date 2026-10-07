# HCZ 管理端 / 内容 / 站点装修 / 文件上传安全审计报告

- 审计日期：2026-10-07
- 代码版本：工作区当前代码（git status 大量已修改，以工作区为准）
- module：`github.com/Aether-v1/hcz`
- 审计方式：只读静态审计（未修改任何源码），追踪到 Service / Repository 层
- 审计范围：dashboard / content / sitebuilder / settings / catalog / upload / auditlog / telegram / compliance / adproxy / reconciliation / reporting / siteconnection / channelclient / downstreamcallback / sitemap 及其 AdminHandler

---

## 执行摘要

管理端路由层 RBAC 覆盖整体**健全**：除 `POST /admin/login`、`POST /admin/login/verify-2fa`（均限流）外，所有 `/api/v1/admin/*` 路由都挂在 `authorized` 组（JWT + Casbin RBAC）下；内置角色策略粒度合理，SMTP/Telegram/Google OAuth 等密钥配置仅授予 `system_admin`，专用读取接口对密钥做了脱敏。

但在**文件上传场景白名单**、**工单附件私有性**、**上游连接 SSRF** 三处发现与项目自身防护约定不一致的缺口。共记录 **P1×1、P2×2、P3×2**，无 P0。

---

### ISSUE-D01
- Severity: P1
- Title: 上传接口 `scene=telegram` 跳过扩展名与 MIME 白名单，运营管理员可上传任意 HTML/JS 并托管在站点同源静态目录
- File: internal/modules/upload/application/service.go
- Function/Method: SaveFileWithMeta
- API: `POST /api/v1/admin/upload`（form 字段 `scene=telegram`）
- Table: content_media（上传记录）；物理落盘 `uploads/telegram/<year>/<month>/<uuid><ext>`
- Root Cause: 上传校验在扩展白名单与 MIME 白名单两处都用 `if normalizedScene != "telegram"` 短路。`telegram` 场景本意为 Bot 渠道接收多种文件类型，但被复用到了**管理端上传端点**（`uploadhttp.AdminHandler.UploadFile` 直接接受前端传入的 `scene`，仅做 normalize 归一化）。于是任何拥有 `/admin/upload` POST 权限的角色（内置 `operations`）都能绕过“仅图片”限制，上传 `.html/.htm` 等可被浏览器渲染的文件。
- Exploit/Trigger:
  1. 使用 operations 管理员登录后台。
  2. `POST /api/v1/admin/upload`，`file` 选择本地 `evil.html`（内容含 `<script>/* 偷 cookie / 伪造请求 */</script>`），form 字段 `scene=telegram`。
  3. 校验链：大小限制仍生效；扩展名白名单被跳过（service.go:74）；`http.DetectContentType` 识别为 `text/html` 但 MIME 白名单同样被跳过（service.go:102）；非 image/ 不做图片解码；非 svg 不做脚本扫描。
  4. 文件以 `uploads/telegram/2026/10/<uuid>.html` 落盘，响应返回公开 URL。
  5. 静态服务 router.go:322-334 仅对 `.svg` 强制 `Content-Disposition: attachment` + CSP，对 `.html` 不做任何处理；`http.FileServer` 按扩展名返回 `text/html`，浏览器直接执行脚本。
- Impact: 同源存储型 HTML/JS 托管。攻击者（恶意 operations 账号或被攻破的运营账号）可在受信任站点源上放置钓鱼/窃取会话页面，诱导用户访问即可劫持前台用户会话。属于管理员权限内但超出“仅图片素材”预期的持久化 XSS/钓鱼托管能力。
- Evidence:
  ```go
  // internal/modules/upload/application/service.go:73-78
  ext := strings.ToLower(filepath.Ext(file.Filename))
  if normalizedScene != "telegram" && len(s.policy.AllowedExtensions) > 0 {
      if ext == "" || !isAllowedExtension(ext, s.policy.AllowedExtensions) {
          return nil, newUploadValidationError("文件扩展名不被允许: %s", ext)
      }
  }
  // :102-113 同样对 telegram 跳过 MIME 白名单
  if normalizedScene != "telegram" && len(s.policy.AllowedTypes) > 0 { ... }
  // :155 文件名 = uuid + 用户传入 ext，不做净化
  filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
  ```
  ```go
  // internal/modules/upload/transport/http/admin_handler.go:48
  scene := c.DefaultPostForm("scene", "common")
  ```
  ```go
  // internal/app/httpserver/router.go:329-333 仅 .svg 被强制下载，.html 不在保护范围
  if strings.EqualFold(path.Ext(c.Request.URL.Path), ".svg") {
      c.Header("Content-Disposition", "attachment")
      c.Header("Content-Security-Policy", "sandbox; script-src 'none'")
      c.Header("X-Content-Type-Options", "nosniff")
  }
  ```
  配置层默认仅允许 jpg/png/gif/webp（config.go:364-376、config.yml:80-90），进一步说明 telegram 旁路是唯一可达的“上传非图片”通道。
- Recommended Fix:
  1. 管理端上传端点不应信任前端传入的 `scene`：按端点固定 scene（如 `/admin/upload` 固定 `common`），或在 admin handler 层拒绝 `scene=telegram`。
  2. 对 `telegram` 场景至少保留 MIME 白名单（image/* + pdf 等 Bot 实际需要的类型），并在静态服务层对 `.html/.htm/.xml` 等危险扩展名统一加 `Content-Disposition: attachment` + `X-Content-Type-Options: nosniff`，或对 `uploads/` 静态目录整体下发 `Content-Security-Policy: default-src 'none'`（图片场景不需要脚本）。

---

### ISSUE-D02
- Severity: P2
- Title: 工单附件场景名 `support_ticket` 不在上传场景白名单，被归一化为 `common`，导致 router 中“禁止公开访问工单附件”的阻断失效
- File: internal/modules/upload/application/service.go; internal/app/container/services_application.go; internal/app/httpserver/router.go
- Function/Method: normalizeUploadScene / localstore.Save
- API: 用户侧 `POST /api/v1/support/...` 附件上传（ticketUploader）；公开静态 `GET /uploads/...`
- Table: support_ticket_attachments（ObjectKey 字段）
- Root Cause: `allowedUploadScenes` 只包含 product/post/banner/editor/common/category/telegram/reseller，**不包含 `support_ticket`**。工单上传器调用 `SaveFileWithMeta(file, "support_ticket")`，`normalizeUploadScene` 对未知场景一律回退 `"common"`。于是附件实际落盘为 `uploads/common/<year>/<month>/<uuid>.<ext>`，而 router.go:325 的私有阻断只匹配 `/uploads/support_ticket/` 前缀——前缀永不出现，阻断成为死代码。
- Exploit/Trigger:
  1. 用户通过工单上传附件（可能含身份证、付款凭证等敏感材料）。
  2. 文件实际 URL 为 `/uploads/common/2026/10/<uuid>.pdf`。
  3. 任何知道该 URL 的人（URL 可能经日志、Referer、工单预览、客服转发泄露）直接 `GET /uploads/common/...` 即可下载，无需登录、无需归属校验。
  4. 鉴权下载端点 `GET /api/v1/support/attachments/:id` 仍正常工作（走 filer.ReadObject），但**同一份文件同时暴露在公开静态路径**，双重暴露。
- Impact: 本应私有、需归属校验的工单附件被公开静态可下载。文件名是 UUID v4，具备“不可猜测性”，但设计意图（router.go 注释明确写“禁止通过公开静态路径直接访问”）被实际存储路径绕过；一旦 URL 泄露即泄露敏感 PII/凭证。
- Evidence:
  ```go
  // internal/modules/upload/application/service.go:25-34
  var allowedUploadScenes = map[string]struct{}{
      "product":{}, "post":{}, "banner":{}, "editor":{},
      "common":{}, "category":{}, "telegram":{}, "reseller":{},
  } // 无 support_ticket
  // :180-189 未知场景回退 common
  func normalizeUploadScene(raw string) string {
      ...
      if _, ok := allowedUploadScenes[value]; ok { return value }
      return "common"
  }
  ```
  ```go
  // internal/app/container/services_application.go:304
  result, err := s.uploader.SaveFileWithMeta(file, supportcontract.SceneSupportTicket) // "support_ticket"
  // :309 objectKey = strings.TrimPrefix(result.URL, "/uploads/") => "common/2026/10/uuid.pdf"
  ```
  ```go
  // internal/app/httpserver/router.go:323-328 阻断前缀永远匹配不到
  if strings.HasPrefix(c.Request.URL.Path, "/uploads/support_ticket/") {
      c.AbortWithStatus(http.StatusNotFound)
      return
  }
  ```
- Recommended Fix: 二选一：
  1. 把 `support_ticket` 加入 `allowedUploadScenes`，让附件真正落到 `uploads/support_ticket/`，使现有 router 阻断生效；
  2. 或更稳妥：工单附件不落公共 `uploads/` 静态目录，改放私有目录（如 `storage/private/`），仅经鉴权 filer 读取，彻底切断公开静态暴露面。

---

### ISSUE-D03
- Severity: P2
- Title: 站点对接 BaseURL 无内网/回环地址校验，上游 HTTP 客户端无 SSRF 防护（与 downstreamcallback/upstream_callback 已有防护不一致）
- File: internal/modules/siteconnection/application/service.go; internal/upstream/dujiao_next.go
- Function/Method: Service.Create / Service.Ping / DujiaoNextAdapter.Ping
- API: `POST /api/v1/admin/site-connections`、`POST /api/v1/admin/site-connections/:id/ping`
- Table: site_connections.base_url
- Root Cause: Create/Update 对 `BaseURL` 仅做 `TrimSpace` + 去尾斜杠，未校验协议、未做私网 IP 阻断。Ping 用 `upstream.NewAdapter` 构造的 `http.Client{Timeout:30s}` 是裸客户端，未设置自定义 Dialer 拦截 `127.0.0.1/10.x/172.16-31.x/192.168.x/169.254.x/::1`。项目在 `downstreamcallback/infrastructure/callbackclient/transport.go:63` 与 `upstreamapi/transport/http/upstream_callback.go:200-221` 已实现私网阻断，但上游适配器这条链路没有复用。
- Exploit/Trigger:
  1. 使用 integration 角色管理员登录。
  2. 创建连接，BaseURL 填 `http://169.254.169.254` 或 `http://127.0.0.1:<内部端口>`。
  3. 调用 ping，服务器向目标发起 `POST {base}/api/v1/upstream/ping`。
  4. 非 200 响应体经 `upstreamHTTPError.Body`（dujiao_next.go:31-36）回传到管理员响应（admin_handler.go:172 `RespondErrorWithMsg(... err.Error() ...)`），构成有限的响应回显；即使不回显也可用于内网端口探测/触发内部端点。
- Impact: 已认证 integration 管理员可借服务器向内网/云元数据地址发起请求。虽为管理员权限内操作，但缺少与项目其它回调链路一致的 SSRF 防护，属于防护不一致导致的内网探测面。
- Evidence:
  ```go
  // internal/modules/siteconnection/application/service.go:81
  BaseURL: strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"),
  // Create 仅校验非空，无 URL/协议/私网校验
  ```
  ```go
  // internal/upstream/dujiao_next.go:63-66 裸客户端，无 Dialer
  client: &http.Client{ Timeout: 30 * time.Second },
  ```
  对比已防护的下游链路：
  ```go
  // internal/modules/downstreamcallback/infrastructure/callbackclient/transport.go:63
  if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ...
  ```
- Recommended Fix: 在 upstream 适配器构造 `http.Client` 时使用自定义 `DialContext`，解析目标 IP 后拒绝 loopback/private/link-local/unspecified 地址（复用 downstreamcallback 已有的判定逻辑）；并在 siteconnection Create/Update 对 BaseURL 做 `http/https` 协议校验。

---

### ISSUE-D04
- Severity: P3
- Title: Banner / 发现页区块 internal 类型链接值未做协议白名单校验（javascript: 依赖前端兜底）
- File: internal/modules/content/application/banner_service.go; internal/modules/sitebuilder/application/discovery_schema.go
- Function/Method: buildBannerEntity / validateLink
- API: `POST/PUT /api/v1/admin/banners`、`POST/PUT /api/v1/site/discovery-blocks`
- Table: content_banners.link_value、site_discovery_blocks.config
- Root Cause: 外链（external）已 fail-closed 仅允许 http/https（banner_service.go:233-243、sitebuilder validation.go:52-69），但 internal 链接类型只校验“非空”，未限制协议。若前端把 internal link_value 直接拼进 `<a href>`，`javascript:alert(1)` 即可在用户点击时执行。是否可利用取决于前端渲染方式（前端由另一审计代理负责）。
- Exploit/Trigger: 管理员创建 Banner，link_type=internal，link_value=`javascript:alert(document.domain)`；用户点击 Banner 触发脚本（前提：前端未对 internal href 做路由跳转隔离）。
- Impact: 潜在存储型 XSS，利用性取决于前端。后端应 fail-closed 而非依赖前端。
- Evidence:
  ```go
  // banner_service.go:156-163
  if linkType != constants.BannerLinkTypeNone && linkValue == "" { return nil, ... }
  if linkType == constants.BannerLinkTypeExternal && !isAllowedExternalHTTPURL(linkValue) { ... }
  // internal 分支无协议校验
  ```
  ```go
  // discovery_schema.go:160-164
  case "internal":
      if linkValue == "" { return errors.New("link_value is required for internal link") }
      return nil // 未限制 javascript:
  ```
- Recommended Fix: internal 链接统一要求以 `/` 开头的站内路径（拒绝 `javascript:/data:/` 及带 scheme 的值），与 home_entry 的路由白名单/ValidateExternalURL 语义对齐。

---

### ISSUE-D05
- Severity: P3
- Title: 通用 `GET /admin/settings?key=` 按原样返回任意配置项，绕过专用脱敏接口
- File: internal/modules/settings/transport/http/admin_handler.go
- Function/Method: AdminHandler.Get
- API: `GET /api/v1/admin/settings?key=<任意key>`
- Table: settings（KV）
- Root Cause: 通用设置读取接口直接 `GetByKey(key)` 后原样返回，未走各专用 schema 的 `Mask*ForAdmin`。专用接口（/settings/smtp、/settings/telegram-bot、/settings/google-auth、/settings/captcha 等）均做了密钥脱敏（已逐一确认 Mask 函数存在且被调用），但通用接口可凭 key 直读原始值（含 SMTP 密码、Telegram Bot token、Google OAuth secret 等）。更新侧对 google_auth_config 做了写入拦截（admin_handler.go:64-72），但读取侧没有对应限制。
- Exploit/Trigger: system_admin 调用 `GET /admin/settings?key=smtp`（或对应配置 key）即获得未脱敏明文密钥。
- Impact: 仅限 system_admin（`/admin/settings` 仅授予 system_admin），不构成越权；但使“密钥不明文回显”的纵深防御在通用接口处被旁路，且密钥可能进入访问日志/浏览器缓存。
- Evidence:
  ```go
  // admin_handler.go:42-54
  key := c.DefaultQuery("key", constants.SettingKeySiteConfig)
  value, err := h.settings.GetByKey(key)
  ...
  response.Success(c, value) // 原样返回，无 Mask
  ```
  对照专用接口均脱敏，如 smtp_handler.go:55 `MaskSMTPSettingForAdmin(setting)`、telegram_bot_handler.go:37 `MaskTelegramBotConfigForAdmin`（后者显式不含 bot_token）。
- Recommended Fix: 通用 Get 对敏感 key 白名单（smtp/telegram-bot/google-auth/captcha 等）拒绝返回或改为走对应 Mask 函数；读取与写入一样对敏感 key 做路由收敛。

---

## 已验证为安全的控制项（正面确认）

1. **Admin 路由 RBAC 覆盖**：`routes_admin.go:102` 创建 `admin` 组，仅 `POST /admin/login`、`POST /admin/login/verify-2fa`（:105-106，均带限流）挂在无认证组；其余全部挂在 `authorized`（:109 = JWT + AdminRBACMiddleware）。全仓搜索 `Group("/admin")` 生产代码仅此一处。
2. **JWT 校验**：`middleware.go:154-237` HS256、强制 access type、校验 TokenVersion / TokenInvalidBefore、管理员存在性，缓存与 DB 双查。
3. **RBAC 判定**：`authz/service.go` Casbin model 为 `some(allow)`（默认拒绝）；`keyMatch2` 匹配路由模板；超管在中间件层短路（middleware.go:250-255，符合预期）。
4. **内置角色粒度**：settings/SMTP/Telegram/Google/audit-logs/system-update/channel-clients/broadcast 仅 `system_admin`；对账 `reconciliation/*` 仅 `integration`；上传 `/admin/upload`、`/admin/media/*` 仅 `operations`。低权限角色无密钥配置权限。
5. **RBAC 覆盖测试**：`rbac_coverage_test.go` 静态扫描所有 routes.go，断言每条 admin 路由都被至少一条内置策略覆盖，防止新增接口漏配。
6. **密钥脱敏**：SMTP / TelegramAuth / GoogleAuth / Captcha / Notification / OrderEmailTemplate / TelegramBot 均有 `Mask*ForAdmin`，且 TelegramBot 管理员视图显式不含 bot_token（telegram_bot.go:224-251）。
7. **SVG 上传防护**：上传时用 XML token 解析器逐元素拦截 `<script>`/`<foreignObject>`/`on*`/`javascript:`/危险 `data:`/处理指令/实体声明（service.go:237-286）；静态服务对 `.svg` 强制下载 + CSP sandbox + nosniff（router.go:329-333）。
8. **文件名无路径穿越**：落盘文件名 = `uuid + ext`（service.go:155），不使用用户原始文件名；scene 归一化白名单、year/month 为时间格式。
9. **图片真实解码**：非 SVG 图片实际 `image.DecodeConfig` 校验真伪并限制宽高（service.go:116-132），双扩展/伪装图片被 magic-byte + 解码双重拦截。
10. **Banner/站点装修 URL 校验**：外链 fail-closed 仅 http/https 且需 Host（banner_service.go:233、validation.go:52）；home_entry internal 路由走白名单、icon 走白名单、primary_color 走 HEX 正则；discovery_block 配置按类型反序列化+校验。
11. **工单附件鉴权下载**：`DownloadAttachment`（user_service.go:325-339）校验工单归属，filer.ReadObject 有路径穿越防护（localfile/store.go:36-53）。
12. **审计日志不含敏感字段**：仅记录 email/status/client_ip/user_agent（response.go:11-19），不记录密码/token。

---

## Admin Route RBAC Matrix

> 权限列引用 `internal/authz/bootstrap.go` 内置角色；Auth=JWT，RBAC=Casbin。所有路由除注明外均挂 `authorized`（JWT+RBAC）。

| Endpoint | Permission(object) | Auth | RBAC | Result |
|---|---|---|---|---|
| POST /admin/login | (公开) | 无 | 无 | OK（限流） |
| POST /admin/login/verify-2fa | (公开) | 无 | 无 | OK（限流） |
| POST /admin/auth/step-up | /admin/auth/step-up | JWT | readonly_auditor 继承 | OK |
| GET /admin/2fa/status | /admin/2fa/status | JWT | readonly_auditor | OK |
| POST /admin/2fa/setup\|enable\|disable | /admin/2fa/* | JWT | readonly_auditor | OK |
| POST /admin/2fa/recovery-codes/regenerate | /admin/2fa/recovery-codes/regenerate | JWT | readonly_auditor | OK |
| PUT /admin/password | /admin/password | JWT | readonly_auditor | OK |
| GET /admin/authz/me | /admin/authz/me | JWT | readonly_auditor + system_admin | OK |
| /admin/authz/roles/* | /admin/authz/roles* | JWT | system_admin | OK |
| /admin/authz/admins/* | /admin/authz/admins* | JWT | system_admin | OK |
| /admin/authz/policies/* | /admin/authz/policies* | JWT | system_admin | OK |
| GET /admin/authz/permissions/catalog | /admin/authz/permissions/catalog | JWT | system_admin | OK |
| GET /admin/authz/audit-logs | /admin/authz/audit-logs | JWT | system_admin | OK |
| POST /admin/authz/admins/:id/2fa/reset | /admin/authz/admins/:id/2fa/reset | JWT | system_admin（handler 二次校验 isSuper） | OK |
| GET /admin/user-login-logs | /admin/user-login-logs | JWT | support | OK |
| GET /admin/compliance/status | /admin/compliance/status | JWT | readonly_auditor | OK |
| POST /admin/compliance/acknowledge | /admin/compliance/acknowledge | JWT | system_admin | OK |
| GET /admin/dashboard/{overview,trends,rankings,inventory-alerts} | /admin/dashboard/* | JWT | readonly_auditor | OK |
| GET /admin/ads/render/:slotCode | /admin/ads/render/:slotCode | JWT | readonly_auditor | OK |
| POST /admin/ads/impression | /admin/ads/impression | JWT | readonly_auditor | OK |
| GET/PUT /admin/settings | /admin/settings | JWT | system_admin | ISSUE-D05（明文读取） |
| GET/PUT /admin/settings/smtp | /admin/settings/smtp | JWT | system_admin | OK（脱敏） |
| POST /admin/settings/smtp/test | /admin/settings/smtp/test | JWT | system_admin | OK |
| GET/PUT /admin/settings/captcha | /admin/settings/captcha | JWT | system_admin | OK（脱敏） |
| GET/PUT /admin/settings/telegram-auth | /admin/settings/telegram-auth | JWT | system_admin | OK（脱敏） |
| GET/PUT /admin/settings/google-auth | /admin/settings/google-auth | JWT | system_admin | OK（脱敏） |
| /admin/settings/notifications/* | /admin/settings/notifications* | JWT | system_admin | OK（脱敏） |
| /admin/settings/order-email-template* | /admin/settings/order-email-template* | JWT | system_admin | OK（脱敏） |
| GET/PUT /admin/settings/affiliate | /admin/settings/affiliate | JWT | system_admin | OK |
| GET/PUT /admin/settings/checkin | /admin/settings/checkin | JWT | system_admin | OK |
| /admin/settings/exchange-rate* | /admin/settings/exchange-rate* | JWT | system_admin | OK |
| GET/PUT /admin/settings/profit-guard | /admin/settings/profit-guard | JWT | system_admin | OK |
| /admin/settings/telegram-bot* | /admin/settings/telegram-bot* | JWT | system_admin | OK（不含 bot_token） |
| POST /admin/pricing/preview | /admin/pricing/preview | JWT | system_admin | OK |
| /admin/system/* (version/update/restart) | /admin/system/* | JWT | system_admin | OK |
| /admin/products* | /admin/products* | JWT | operations | OK |
| /admin/posts* | /admin/posts* | JWT | operations | OK |
| /admin/post-categories* | /admin/post-categories* | JWT | operations | OK |
| /admin/banners* | /admin/banners* | JWT | operations | ISSUE-D04（internal 链接协议） |
| /admin/media* | /admin/media* | JWT | operations | OK |
| POST /admin/upload | /admin/upload | JWT | operations | ISSUE-D01（telegram 旁路） |
| /admin/categories* | /admin/categories* | JWT | operations | OK |
| /admin/coupons* | /admin/coupons* | JWT | operations | OK |
| /admin/promotions* | /admin/promotions* | JWT | operations | OK |
| /admin/card-secrets* | /admin/card-secrets* | JWT | operations | OK |
| /admin/gift-cards* | /admin/gift-cards* | JWT | operations（finance 只读） | OK |
| /admin/member-levels* / member-level-prices* | /admin/member-levels* | JWT | operations | OK |
| /admin/site/home-entries* | /admin/site/home-entries* | JWT | operations + system_admin | OK（白名单校验） |
| /admin/site/featured-categories* | /admin/site/featured-categories* | JWT | operations + system_admin | OK |
| /admin/site/discovery-blocks* | /admin/site/discovery-blocks* | JWT | operations + system_admin | ISSUE-D04（internal 链接协议） |
| GET/PUT /admin/site/brand | /admin/site/brand | JWT | operations + system_admin | OK（HEX 校验） |
| GET /admin/site/audit-logs | /admin/site/audit-logs | JWT | operations + system_admin | OK |
| /admin/points/* | /admin/points/* | JWT | readonly/operations/finance/system_admin | OK |
| /admin/orders* | /admin/orders* | JWT | support GET / finance 写 | OK |
| /admin/order-refunds* | /admin/order-refunds* | JWT | support GET / finance 写 | OK |
| /admin/fulfillments | /admin/fulfillments | JWT | support POST | OK |
| /admin/users* | /admin/users* | JWT | support | OK |
| /admin/wallet/* | /admin/wallet/* | JWT | finance | OK |
| /admin/payments* | /admin/payments* | JWT | support GET / finance | OK |
| /admin/payment-channels* | /admin/payment-channels* | JWT | finance | OK |
| /admin/site-connections* | /admin/site-connections* | JWT | integration | ISSUE-D03（SSRF） |
| /admin/product-mappings* | /admin/product-mappings* | JWT | integration | OK |
| /admin/procurement-orders* | /admin/procurement-orders* | JWT | integration 只读 | OK |
| /admin/reconciliation/* | /admin/reconciliation/* | JWT | integration | OK |
| /admin/api-credentials* | /admin/api-credentials* | JWT | integration | OK |
| /admin/resellers/* | /admin/resellers* | JWT | integration/finance/system_admin | OK |
| /admin/channel-clients* | /admin/channel-clients* | JWT | system_admin | OK |
| /admin/telegram-bot/broadcasts* | /admin/telegram-bot/broadcasts* | JWT | system_admin | OK（system_admin 专属群发） |
| GET /admin/telegram-bot/users | /admin/telegram-bot/users | JWT | system_admin | OK |
| /admin/c2c/* | /admin/c2c* | JWT | authorized（handler 内 Step-Up） | OK |
| /admin/support/* | /admin/support* | JWT | support + system_admin | OK |

未在 routes_admin.go 注册 Admin 路由的模块：`reporting`（仅被 dashboard 复用，无自有 HTTP 路由）、`downstreamcallback`（基础设施回调客户端，无 admin HTTP 路由）、`sitemap`（公开 SEO，RegisterRoutes 挂 r 公开）、`telegram/channelbot`（挂 channel 组，非 admin）。

---

## Stored XSS 攻击面清单

| 管理员输入字段 | 存储位置 | 用户端展示位置 | 后端校验 | 结论 |
|---|---|---|---|---|
| 文章正文 ContentJSON | content_posts.content_json (JSON) | 公开文章详情 API | 无 HTML 过滤（结构化 JSON，富文本渲染在前端） | 攻击面在前端 v-html；后端存原始 JSON。NOT VERIFIED（前端渲染） |
| 文章标题/摘要 TitleJSON/SummaryJSON | content_posts | 列表/详情 | normalizeMultiLangJSON 仅接受字符串并 Trim | 纯文本，低风险 |
| Banner image / mobile_image | content_banners | 前台 Banner <img> | 非外链则须站内路径，外链须 http/https | OK |
| Banner link_value (external) | content_banners.link_value | <a href> | isAllowedExternalHTTPURL 拒绝 javascript: | OK |
| Banner link_value (internal) | content_banners.link_value | <a href>/路由 | 仅非空校验 | ISSUE-D04，依赖前端 |
| home_entry image/action_target | site 配置 | 首页入口 | image http/https 校验；action internal 路由白名单 / external http/https | OK |
| discovery_block banner image/link_value | discovery_blocks.config JSON | 发现页 | image 仅非空；external link http/https；internal 仅非空 | ISSUE-D04（internal link） |
| discovery_card action_target | discovery_blocks.config | 卡片 | internal 路由白名单 / external http/https | OK |
| announcement text | discovery_blocks.config | 公告文案 | 纯文本，仅非空 | 低风险（前端若 v-html 则风险，NOT VERIFIED） |
| brand primary_color / copyright | site_config.brand | 全局 | primary_color HEX 正则；copyright 纯文本 | OK |
| 上传文件（scene=telegram） | uploads/telegram/**.html | 同源静态 HTML 执行 | 无扩展/MIME 校验 | ISSUE-D01（P1） |

---

## 文件上传校验矩阵

| 校验项 | 普通 scene (common/product/post/banner/...) | telegram scene | 工单附件 (ticketUploader) |
|---|---|---|---|
| 文件大小 | MaxSize=10MB | 同左 | 10MB |
| 扩展名白名单 | .jpg/.jpeg/.png/.gif/.webp | **跳过（ISSUE-D01）** | .jpg/.jpeg/.png/.webp/.pdf/.txt/.log |
| MIME magic-byte | DetectContentType + 白名单 | **跳过（ISSUE-D01）** | image/jpeg,png,webp,pdf,text/plain |
| 真实图片解码 | 是（DecodeConfig + 宽高限制） | 非 image 跳过 | 同普通 |
| SVG 脚本扫描 | 是（XML token 解析） | 是（内容为 svg 时仍触发） | svg 不在白名单，不可上传 |
| 文件名净化 | uuid+ext，无穿越 | 同左 | 同左 |
| 路径穿越 | scene 白名单 + uuid | 同左 | 同左 |
| 静态目录强制下载 | .svg 强制 attachment+CSP | **.html 无保护（ISSUE-D01）** | 应落 support_ticket/ 但实际落 common/（ISSUE-D02） |
| 私有访问 | N/A | N/A | 鉴权下载端点存在，但公开静态路径同时可达（ISSUE-D02） |

---

## 验证状态

### 已验证（有代码证据）
- Admin 路由 RBAC 覆盖：routes_admin.go 全文 + 全仓 `Group("/admin")` 搜索 + 各模块 routes.go 逐一核对。
- Casbin model/匹配器/内置角色种子：authz/service.go、bootstrap.go 全文。
- JWT/RBAC 中间件：middleware.go:154-318。
- 上传校验链：upload/service.go 全文 + localstore + admin_handler + config 默认值 + config.yml。
- 工单附件 scene 归一化与静态阻断失效：service.go:25-34/180-189、services_application.go:304、router.go:323-334。
- Banner/sitebuilder URL 校验：banner_service.go、discovery_schema.go、validation.go、home_entry_service.go、brand_service.go。
- Settings 密钥脱敏：smtp/telegram_bot/google_auth/captcha handler + 各 Mask 函数。
- 上游 SSRF：siteconnection service.go + dujiao_next.go + downstreamcallback/upstream_callback 对照。
- Dashboard 数据粒度：types.go 确认聚合指标无个人 PII。
- Telegram 群发权限与入口：broadcast handler + bootstrap 系统管理员策略。

### NOT VERIFIED
- 前端是否对 internal link_value / announcement text / ContentJSON 使用 v-html 或 dangerouslySetInnerHTML（前端由另一审计代理负责）。
- 实际数据库 casbin_rule 表中是否存在被手工插入的、超出内置种子的自定义策略（代码层无法验证 DB 运行时数据）；如存在自定义角色授予了 `/admin/settings*`，需结合 DB 审计。
- Telegram Bot token 在运行时是否通过日志/错误信息意外打印（未逐一审查所有 Bot 调用路径的日志字段）。
- 批量删除（posts/banners/media batch-delete）是否有软删除/确认机制——代码层看到为物理删除（store.Delete），但业务可接受性未深入。
- selfupdate / system update 链路的实际二进制下载源校验（router.go 中 /admin/system/update 仅见路由注册，未深入审计 selfupdate 模块；不在本次重点范围）。
