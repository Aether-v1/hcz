# HCZ V1 安全专项审计报告

- 审计日期：2026-10-05
- 项目根：`E:\Users\orang\Downloads\Compressed\hcz_v1`（Go module `github.com/Aether-v1/hcz`）
- 审计方式：逐文件实读 + 全局搜索 + 运行验证（非凭印象）
- 原则：不新增功能，只审计 + 最小必要修复
- 结论速览：**未发现 P0；发现 2 个 P2（已修复）、若干 P3/条件项**。整体安全 posture 良好。

---

## 总体结论

| 维度 | 评级 | 一句话结论 |
|---|---|---|
| XSS / URL 安全 | **PASS WITH CONDITIONS** | 后端外链 fail-closed、SVG 服务端清洗 + 静态层 CSP；已修 Banner 外链漏洞；Blog 富文本渲染缺 DOMPurify（admin-only，前端项） |
| Upload 安全 | **PASS** | magic-byte MIME 嗅探 + 扩展名白名单 + UUID 随机名 + SVG XML 级清洗；已堵私有附件公开静态访问 |
| Auth 安全 | **PASS** | bcrypt、HS256 强制、timing 防护、frozen 用户拦截、TOTP 加密、限流、无账号枚举 |
| Admin / RBAC | **PASS** | casbin fail-closed，CI 测试实证 uncovered routes = 0（已运行通过），财务写操作挂 paymentProtected |
| IDOR | **PASS** | 工单 / C2C 支付方式 / 提现创建与取消均校验资源归属 |
| Secrets | **PASS** | .env/config.yml/logs/uploads 均 gitignore，无生产硬编码 secret，弱密钥启动即 Fatal |
| Logging | **PASS WITH CONDITIONS** | 请求日志不含 body、C2C 不记支付明细；已修 bootstrap 明文记口令 |

---

## 1. XSS / URL 安全 —— PASS WITH CONDITIONS

### 证据（fail-closed 点）
- Site Builder 外链：`internal/modules/sitebuilder/application/validation.go:52-69` `ValidateExternalURL` 仅放行 `http/https` 且要求 `Host != ""`，其余一律拒绝。被 `home_entry_service.go:157`、`discovery_schema.go:134/166/180` 复用。
- Reseller 客服/页脚链接：`internal/modules/reseller/application/site_config.go:130-150` `validateSupportURL` 仅放行 `tg://`、合法 `mailto:`、`https://`；telegram/whatsapp 前缀白名单（:155-161）。
- Google 头像 URL：`internal/modules/identity/googleauth/application/service.go:284-297` `normalizeGooglePictureURL` 仅放行 http/https、拒绝 userinfo。
- SVG 上传：`internal/modules/upload/application/service.go:237-286` 用 `xml.RawToken` 逐 token 拒绝 `<script>`、`<foreignObject>`、所有 `on*` 事件属性、`javascript:`、危险 `data:`、处理指令、实体声明（注释明确说明正则黑名单无法穷举）。
- 静态层兜底：`internal/app/httpserver/router.go:311-321` 对所有 `.svg` 响应强制 `Content-Disposition: attachment` + `Content-Security-Policy: sandbox; script-src 'none'` + `nosniff`。
- 前端富文本：`frontend/user/src/components/AnnouncementModal.vue` 与 `frontend/admin/src/utils/releaseNotes.ts` 均经 DOMPurify 白名单净化（`ALLOWED_URI_REGEXP` 只放 http(s)/mailto/tel/#/相对路径）；`frontend/user/src/utils/customScripts.ts` 已废弃为空实现，不再执行任何下发脚本。

### 发现与修复
- **X-01（P2，已修复）**：Banner `LinkValue` 在 `link_type=external` 时后端未做协议校验，admin 可配置 `javascript:alert(1)` 触发存储型 XSS。
  - 修复前：`internal/modules/content/application/banner_service.go` `buildBannerEntity` 仅判空，不校验协议。
  - 修复后：新增 `isAllowedExternalHTTPURL`（仅 http/https + Host），在 `buildBannerEntity` 对 external 链接 fail-closed 校验；`go build` 与 `content/application` 测试通过。

### 条件项（未改，说明理由）
- **X-02（P3，前端项）**：`frontend/user/src/views/BlogDetail.vue` 对 `post.content` 直接 `v-html="processHtmlForDisplay(...)"`，而 `processHtmlForDisplay`（`frontend/user/src/utils/content.ts`）只重写图片路径、**不做 DOMPurify 净化**。文章为平台 admin 专属内容（非普通用户输入），且 AnnouncementModal 已净化，此处不一致。修复需改前端并重新构建 dist，超出"后端最小修复"范围，建议后续在前端补 DOMPurify。
- **X-03（P3）**：C2C `QRImage`（`internal/modules/c2c/application/payment_method.go:84`）未做协议校验；该字段仅作为用户自己收款码 `<img src>` 使用，img src 中的 `javascript:` 不执行，风险低。

---

## 2. Upload 安全 —— PASS

### 证据
- 入口统一：`internal/modules/upload/application/service.go:64` `SaveFileWithMeta`。
  - 大小限制：:68；扩展名白名单：:73-78；MIME 白名单用 `http.DetectContentType` 嗅探 **512B 魔数**（:88-113），不采信客户端 Content-Type。
  - 随机对象键：:155 `uuid.New().String()+ext`，文件名完全服务端生成，**无 path traversal**。
  - 图片尺寸解码限制：:116-132；WebP 手动解析。
  - 可执行扩展名：白名单仅 `image/*` / `application/pdf` / `text/plain`（`internal/app/container/services_application.go:176-177`），**无 .php/.exe/.html**。
- 存储：`internal/modules/upload/infrastructure/localstore/store.go:28-46`，路径由服务端 scene/year/month + UUID 组成。
- 工单附件：`internal/modules/supportticket/application/user_service.go:297-322` 复用同一 uploader；上传响应只返回 `id/file_name/mime/size`（`user_handler.go:225-230`），不暴露物理 URL。

### 发现与修复
- **U-01（P2，已修复）**：工单附件物理落盘于 `./uploads/support_ticket/...`，而 `/uploads` 是**无鉴权公开静态目录**（原 `router.go:317`）。虽然 UUID v4 不可猜测、且鉴权下载端点 `GET /api/v1/support/attachments/:id`（带 `ticket.UserID != userID` 归属校验，`user_service.go:325-353`）存在，但同一字节仍可被公开静态路径直达，属"private 文件未强制鉴权"。
  - 修复前：`/uploads/support_ticket/**` 公开可下载。
  - 修复后：`router.go:311-321` 新增前缀拦截，`/uploads/support_ticket/` 直接 404；鉴权下载走 `filer.ReadObject` 读本地磁盘（不经 HTTP 静态路由），不受影响。`go build ./internal/app/httpserver/...` 通过。

### 条件项
- `telegram` 场景跳过扩展名/类型白名单（`service.go:74/102`），用于接收 Telegram Bot 入站文件，属可信来源，可接受。

---

## 3. Auth 安全 —— PASS

### 证据
- 口令哈希：`internal/modules/identity/userauth/application/service.go:324` `bcrypt.GenerateFromPassword(DefaultCost)`，登录 `:391` `bcrypt.CompareHashAndPassword`；用户不存在时执行 dummy bcrypt 比较（:382）防 timing 侧信道。
- JWT：`service.go:179` HS256 签发；`internal/modules/identity/jwttoken/token.go:16-18` `NewHS256Parser` 用 `WithValidMethods(["HS256"])` **强制算法**，防 alg-confusion/none；claims 含 `TokenVersion` 用于吊销；2FA challenge token 用独立 `typ=2fa_challenge`（:128-136,488）。
- 冻结/禁用用户：`service.go:385-387` `Status != active → ErrUserDisabled`。
- TOTP：`internal/modules/identity/userauth/totp/application/service.go` 密钥 `crypto.Encrypt` 加密落库（:146）、恢复码一次性消耗（:319）、启用失败 5 次锁定（:354-368）、skew=1/6 位/30s。
- 限流（`internal/app/httpserver/router.go:254-277`）：注册 5/10min、发验证码 3/min、找回密码 3/15min。
- 账号枚举：登录统一 `ErrInvalidCredentials`（:383/392）；找回密码发码对"邮箱未注册"统一返回 `{sent:true}`（`user_verify_handler.go:121-127`）。
- 弱密钥启动即拒：`cmd/server/main.go:88-98` 弱/重复/默认密钥 `Fatalf`；release 模式空管理员口令则跳过初始化（:148）。

---

## 4. Admin / RBAC 安全 —— PASS

### 证据
- 模型：`internal/authz/service.go:26-41` casbin，`e = some(where (p.eft == allow))` **默认拒绝**；`keyMatch2` 路由匹配。
- 中间件 fail-closed：`internal/app/httpserver/middleware/middleware.go:240-302` authzService 不可用即 401、Enforce 出错即 401、仅 `IsSuper==true` 旁路。
- 角色矩阵：`internal/authz/bootstrap.go` 按角色逐路由显式授权（finance 角色才含 refund/withdrawal/wallet adjust）。
- **uncovered routes = 0（实证）**：`internal/app/httpserver/rbac_coverage_test.go` 静态提取所有 admin 路由并断言每条被至少一个内置角色覆盖。实际运行：
  - `go test ./internal/app/httpserver/ -run 'TestAllAdminRoutesCoveredByBuiltinRoles|TestExtractAdminRoutesIncludesPlatformHTTPRoutes'` → **ok 通过**。
- 财务写操作保护：`routes_admin.go:104-107` `authorized`(JWT+RBAC) 之上，`paymentProtected` 子组再叠加 `PaymentComplianceRequired`；refund/withdrawal/wallet adjust/payment-channel/C2C 仲裁/对账全部挂 paymentProtected（:170-211,220）。

---

## 5. IDOR —— PASS

### 证据
- 工单：`supportticket/application/user_service.go:114/150/210/256/337` 每个读/操作都校验 `ticket.UserID != userID → ErrNotOwner`；附件下载 `:337` 校验工单归属。
- C2C 支付方式：`c2c/application/payment_method.go:57-69` `GetPaymentMethodByIDForUser` 校验 `pm.UserID != userID`；更新/删除/启停均复用（:73/96/104）。
- 提现：`walletwithdrawal/application/create.go` 全程用 JWT 推导的 `input.UserID`（:108 行锁账户、:123/132/143 限额均按 UserID）；`cancel.go:31` `w.UserID != input.UserID → ErrWithdrawalNotFound`。
- 未发现 `db.First(&x, id)` 后缺少 `where user_id` 的裸查模式；C2C 交易中买卖双方可见对方支付信息属业务设计（促成付款），非越权。

---

## 6. Secrets —— PASS

### 证据
- `.gitignore` 含 `config.yml` / `.env` / `logs/` / `uploads`；`git ls-files` 仅追踪 `config.yml.example`，**无 .env / 真实 config / .pem / .key 入库**。
- 全局搜索 `SecretKey/Password/ApiKey/PrivateKey = "..."` 的非测试命中仅 `internal/modules/identity/admin/application/bootstrap.go:14` `defaultBootstrapPassword="admin123"`；该常量在 release 模式被 `main.go:93-96` `unsafeBootstrapAdminPassword` 拦截（Fatal），仅 dev 兜底，非生产可达。
- 其余命中全部位于 `*_test.go`（测试夹具）。
- 运行时弱/重复/默认密钥启动即 `Fatalf`（`main.go:88-91`）。
- 前端 `dist/` 0 文件入库；`logs/` 无追踪文件。

---

## 7. Logging —— PASS WITH CONDITIONS

### 证据
- 请求日志（`internal/app/httpserver/middleware/middleware.go` + `logs/app.log` 实证）只记 method/path/status/latency/client_ip/request_id，**不含 request body、不含口令/验证码**。`POST /api/v1/auth/register` 路径被记录，但 body 不记录。
- C2C 应用层（`internal/modules/c2c/application/*`）无任何支付明细日志调用；银行卡号/支付宝账号不入日志。
- 审计/登录日志由 auditlog 模块独立留痕。

### 发现与修复
- **L-01（P2，已修复）**：`internal/modules/identity/admin/application/bootstrap.go:49` 原 `logger.Warnw(..., "password", password)` **把明文初始口令写入日志**。
  - 修复前：`"default_admin_created_with_default_password", "username", ..., "password", password`。
  - 修复后：移除 `password` 字段，仅保留 username 与"需改密"提示；同函数 else 分支本就用 `"password_hidden", true`。`go test ./internal/modules/identity/admin/...` 通过。

---

## 问题清单（按优先级）

### P0（阻断级）
- 无。

### P1（高危）
- 无。

### P2（中危，本次已修复）
1. **X-01** Banner 外链未做协议校验（存储型 XSS，admin-only）→ 已在 `content/application/banner_service.go` 加 `isAllowedExternalHTTPURL` fail-closed。
2. **U-01** 工单附件经公开静态路径可达（private 文件未强制鉴权）→ 已在 `app/httpserver/router.go` 拦截 `/uploads/support_ticket/`。
3. **L-01** bootstrap 明文记录管理员初始口令 → 已在 `identity/admin/application/bootstrap.go:49` 移除 password 字段。

### P3 / 条件项（建议后续，本次未改）
- **X-02** `frontend/user/src/views/BlogDetail.vue` 对文章富文本 `v-html` 未过 DOMPurify（admin-only 内容；需前端重建）。
- **X-03** C2C `QRImage` 未做协议白名单（img src，风险低）。
- 富文本公告/文章为 admin 专属内容，依赖前端 DOMPurify；建议后端长期引入 html sanitizer 做纵深防御。

---

## 本次改动文件清单（最小必要修复）
| 文件 | 改动 |
|---|---|
| `internal/modules/identity/admin/application/bootstrap.go` | 日志移除明文 password 字段 |
| `internal/modules/content/application/banner_service.go` | external 链接新增 http/https fail-closed 校验 + `isAllowedExternalHTTPURL` |
| `internal/app/httpserver/router.go` | `/uploads/support_ticket/` 私有场景静态访问 404 |

验证：`go build ./internal/modules/content/... ./internal/modules/identity/admin/... ./internal/app/httpserver/...` 退出码 0；`content/application`、`identity/admin/...`、`app/httpserver`（含 RBAC 覆盖率测试）测试全部通过。
