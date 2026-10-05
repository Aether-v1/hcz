# HCZ Phase 8 — General Ticket System Pre-Audit

> 本轮只审计，不修改代码。
> 审计日期：2026-10-05
> 代码基线：Phase 7 User C2C 前端已封板

---

## 0. 结论速览

| 审计项 | 结论 |
|---|---|
| 现有代码可复用 | Upload FileUploader（文件校验+落盘）、Redis 限流中间件、Notification 发送系统、Admin JWT/RBAC、i18n。**无**现有通用 Ticket 模型/页面/对话基础设施 |
| Ticket 与 After-Sale | **彻底分域**。After-Sale 仅限 Recharge Order 退款（after_sale_tickets 表在 order 模块），Ticket 是通用客服支持，不执行资金操作 |
| 新表数量 | **5 张**：support_ticket_categories / support_tickets / support_ticket_messages / support_ticket_attachments / support_ticket_audits |
| 最终状态机 | 5 态：open / waiting_user / waiting_support / resolved / closed。合法流转见第六节 |
| unread/read | **Ticket 层计数器**（user_unread_count / admin_unread_count），比 message read cursor 简单稳定。回复时原子更新对方计数，查看时清零本方计数 |
| Attachment 安全 | 复用现有 FileUploader（MIME/extension/size 校验），新增 User 上传端点，random object key，不使用原始 filename 作路径，私有访问（现有存储为本地文件系统，通过受控 API 读取而非公开 CDN） |
| User Notification | 复用 Phase 1 Notification 系统，新增 4 种 event_type（ticket_admin_replied / ticket_resolved / ticket_closed / ticket_created 可选），点击跳 /support/tickets/:id |
| Admin Notification | **无独立 admin notification 系统**。第一版用 admin_unread_count + Admin 前端 15-30s polling 实现红点。不混入 user_notifications |
| User API 最小范围 | POST tickets / GET tickets / GET ticket/:id / POST ticket/:id/replies / POST ticket/:id/close / POST ticket/:id/reopen（可选）/ GET categories / POST attachments（上传） |
| Admin API 最小范围 | GET tickets / GET ticket/:id / POST reply / PATCH status / PATCH priority / POST assign / GET categories CRUD / GET audits / overview |
| User/Admin UI 页面 | User：帮助入口、我的工单、创建工单、工单详情（对话时间线+回复+附件+未读红点）。Admin：Ticket Dashboard、Ticket List、Ticket Detail（Conversation+Reply+Assign+Priority+Status）、Categories |
| 跨业务关联 | Ticket 可选 biz_type + biz_id（order/withdrawal/c2c_trade），**只查看/跳转，不修改业务状态**。资金操作必须走对应正式业务流程 |
| MVP 最小实施范围 | 5 表 + category + create/list/detail + User/Admin reply + assignment + status 5 态 + unread 计数 + user notification + attachments + basic rate limit + User/Admin UI + IDOR + RBAC |
| 是否可进入实施 | **可以**。所有依赖基础设施已确认，边界清晰，MVP 范围明确 |

---

## 一、Ticket 与 After-Sale 边界

### 1.1 After-Sale（已有，不改动）

- **范围**：仅针对 Recharge Order
- **类型**：未收到（not_received）、部分退款、全额退款
- **表**：`after_sale_tickets`（在 `internal/modules/order/domain/after_sale.go`）
- **能力**：可执行退款（RefundAmount 字段），直接影响 Wallet
- **状态**：none / pending / resolved / rejected

### 1.2 Ticket（新建，通用客服支持）

- **范围**：登录问题、Wallet 问题、Withdrawal 问题、C2C 问题、Recharge 咨询、Affiliate 问题、账号问题、其他客服支持
- **表**：`support_tickets` 等 5 张新表（独立模块 `internal/modules/support/`）
- **能力**：**不允许**直接执行退款、Wallet 调账、C2C 仲裁、Withdrawal approve/reject
- **如涉及资金操作**：Ticket 只能跳转对应正式业务流程（After-Sale / Withdrawal Admin / C2C Arbitration），客服在 Ticket 中提供指导，实际操作在对应业务域执行

### 1.3 边界总结

| 维度 | After-Sale | Ticket |
|---|---|---|
| 触发场景 | Recharge Order 退款申请 | 通用客服咨询/问题 |
| 资金操作 | 可执行退款 | 不可执行任何资金操作 |
| 表位置 | order 模块 | 独立 support 模块 |
| 状态机 | 4 态（none/pending/resolved/rejected） | 5 态（open/waiting_user/waiting_support/resolved/closed） |
| 对话 | 无持续对话（单条 reason+description） | 持续多轮对话（messages 表） |
| 附件 | 无 | 有 |

**User 从 Recharge Order 发起退款问题时，UI 应优先引导 After-Sale 流程。** Ticket 主要处理咨询、账号、技术、非标准问题。

---

## 二、现有代码审计

### 2.1 全局搜索结果

| 关键词 | 命中 | 分类 |
|---|---|---|
| ticket | `AfterSaleTicket`（order 模块，售后专用）、`support` 角色名（authz） | KEEP（After-Sale 不动）、BUILD（通用 Ticket） |
| support | `support` admin 角色（authz/bootstrap.go）、reseller support URL（无关） | MODIFY（support 角色可复用为客服角色，需细化权限） |
| help / customer_service / helpdesk | 无命中 | BUILD |
| message / conversation / reply / comment | 无通用对话基础设施 | BUILD |
| attachment / upload | `internal/modules/upload/`（Admin 上传端点 + FileUploader 接口）、`content` 模块 MediaService + FileStore | KEEP（FileUploader 可复用）、MODIFY（需新增 User 上传端点） |
| notification | `internal/modules/notification/`（完整发送系统 + dedupe + async queue） | KEEP（复用，新增 event_type） |
| audit | `internal/modules/auditlog/`（admin_login / user_login / authz 变更） | MODIFY（当前仅限认证事件，Ticket Admin 操作可复用模式或独立 audit 表） |
| rate limit | `internal/app/httpserver/middleware/rate_limit.go`（Redis + local fallback） | KEEP（复用，新增 Ticket 限流规则） |

### 2.2 可复用资产详情

| 资产 | 位置 | 复用方式 |
|---|---|---|
| FileUploader 接口 | `internal/modules/upload/transport/http/admin_handler.go`（`SaveFileWithMeta(file, scene)`） | Ticket 附件上传直接调用，复用 MIME/extension/size 校验和文件落盘 |
| UploadConfig | `internal/config/config.go`（MaxSize / AllowedTypes / AllowedExtensions） | 复用全局配置，Ticket 附件可独立配置更严格的 whitelist |
| Redis 限流中间件 | `internal/app/httpserver/middleware/rate_limit.go`（RateLimitRule：Prefix/WindowSeconds/MaxRequests/BlockSeconds） | 新增 Ticket 创建/回复/附件上传限流规则 |
| Notification 发送 | `internal/modules/notification/application/send.go` + async queue | 新增 ticket event_type，复用发送/去重/异步分发 |
| Admin JWT + RBAC | authz 模块 | 新增 support.ticket.* 权限，support 角色可预绑定 |
| i18n | `internal/i18n/` | 新增 Ticket 相关多语言 key |
| AutoMigrate registry | `internal/bootstrap/database/migrations/registry.go` | 注册 5 张新表 |

### 2.3 需要新建

- `internal/modules/support/` 完整模块（domain / contract / application / infrastructure / transport / integrationtest）
- 5 张新表
- User 上传端点（现有 upload 只有 Admin 端点）
- Ticket 专属 Admin audit（或扩展 auditlog 模块）
- User/Admin 前端页面

### 2.4 DROP（不使用）

- AfterSaleTicket 模型（不通用，保留在 order 模块不动）
- content 模块 MediaService（与 CMS 素材绑定，不适合 Ticket 附件）

---

## 三、第一版 Ticket 模型

### 3.1 support_tickets 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| ticket_no | varchar(32) | uniqueIndex，工单号（如 TKT-20261005-000001） |
| user_id | uint | index，提交用户 ID |
| category_id | uint | index，分类 ID（关联 support_ticket_categories） |
| subject | varchar(255) | 工单标题 |
| status | varchar(20) | index，状态（open/waiting_user/waiting_support/resolved/closed） |
| priority | varchar(10) | index，优先级（low/normal/high/urgent） |
| assigned_admin_id | uint | index，分配的客服 Admin ID（nullable，未分配为 null） |
| biz_type | varchar(30) | 可选关联业务类型（order/withdrawal/c2c_trade，nullable） |
| biz_id | uint | 可选关联业务 ID（nullable） |
| last_reply_by | varchar(10) | 最后回复方（user/admin/system） |
| last_replied_at | *time.Time | 最后回复时间 |
| user_unread_count | int | default 0，用户未读数（Admin 回复时 +1，用户查看时清零） |
| admin_unread_count | int | default 0，客服未读数（用户回复时 +1，Admin 查看时清零） |
| closed_at | *time.Time | 关闭时间 |
| created_at / updated_at | | |

### 3.2 设计说明

- `assigned_admin_id` 第一版需要（手动领取/分配），数据模型预留
- `biz_type` + `biz_id` 可选关联，只用于查看/跳转，不修改业务状态
- `user_unread_count` / `admin_unread_count` 在 Ticket 层维护，比 message read cursor 简单（见第十四节）
- `priority` 用户不能自己选 urgent，由 category default + Admin 调整决定（见第八节）

---

## 四、Ticket Reply

### 4.1 support_ticket_messages 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| ticket_id | uint | index，关联工单 |
| sender_type | varchar(10) | 发送方类型（user/admin/system） |
| sender_user_id | uint | 发送用户 ID（sender_type=user 时必填，admin 时可为 null 或填 admin 对应的 user_id） |
| admin_id | uint | 发送 Admin ID（sender_type=admin 时必填） |
| body | text | 消息正文（纯文本 + 安全换行，不允许 raw HTML） |
| message_type | varchar(20) | 消息类型（text/system_notice，第一版主要 text） |
| created_at | | |

### 4.2 设计说明

- **不建议**把整个对话塞进 ticket 单表 JSON。独立 messages 表支持分页、索引、附件关联
- `sender_type` 三值：user（用户回复）、admin（客服回复）、system（系统消息，如"工单已关闭"、"工单已分配给 XXX"）
- system 消息的 body 可以是 i18n key 或预定义文本，sender_user_id/admin_id 可为 null
- 消息按 `created_at, id` 稳定排序（见第十八节）

---

## 五、附件

### 5.1 support_ticket_attachments 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| ticket_id | uint | index，关联工单 |
| message_id | uint | index，关联消息（nullable，附件可在创建工单时上传，不绑定具体 message） |
| uploader_type | varchar(10) | 上传方类型（user/admin） |
| uploader_user_id | uint | 上传用户 ID |
| file_name | varchar(255) | 原始文件名（仅展示用） |
| file_path | varchar(500) | 存储路径（random object key，不使用原始 filename） |
| mime_type | varchar(100) | MIME 类型 |
| file_size | int64 | 文件大小（字节） |
| created_at | | |

### 5.2 现有 Upload 基础设施复用

- **FileUploader 接口**：`SaveFileWithMeta(file, scene)` 已实现 MIME 校验、extension 校验、size 校验、文件落盘
- **UploadConfig**：全局 `upload.max_size`（默认 10MB）、`upload.allowed_types`、`upload.allowed_extensions`
- **当前限制**：只有 Admin 上传端点（`/admin/upload`），无 User 上传端点

### 5.3 Ticket 附件方案

**第一版方案：新增 User 上传端点，复用 FileUploader**

- 新增 `POST /api/v1/support/attachments`（User 认证 + 限流）
- 调用现有 FileUploader，scene = "support_ticket"
- 返回 file_path / file_name / mime_type / file_size
- 创建工单或回复时，将 attachment ID 关联到 ticket/message
- 附件读取通过受控 API（`GET /api/v1/support/attachments/:id`），校验用户是 ticket 参与者或 Admin，不暴露公开 CDN URL

### 5.4 安全配置

| 项 | 第一版配置 |
|---|---|
| 允许类型 | jpg / jpeg / png / webp / pdf / txt / log |
| MIME whitelist | image/jpeg, image/png, image/webp, application/pdf, text/plain |
| extension whitelist | .jpg, .jpeg, .png, .webp, .pdf, .txt, .log |
| 最大文件大小 | 10MB（复用全局 upload.max_size，可独立配置） |
| 每消息最大附件数 | 5 |
| 每工单最大附件数 | 20 |
| 禁止 | 可执行文件（.exe, .bat, .sh, .js, .html, .php 等）、压缩包（.zip, .rar，第一版不支持） |
| 存储路径 | random object key（如 `support/2026/10/05/{uuid}.{ext}`），不使用原始 filename |
| 访问控制 | 私有访问，通过受控 API 校验权限后读取，不成为公开 CDN 资源 |

### 5.5 内容安全

- 图片文件：不做内容审核（第一版），但 MIME + extension 双重校验防止伪装
- PDF/txt/log：纯文本展示或下载，不渲染为 HTML
- **防 XSS**：消息 body 纯文本 + 安全换行，前端渲染时转义，不允许用户提交 raw HTML 或 Markdown（第一版）

---

## 六、分类

### 6.1 support_ticket_categories 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| name | varchar(100) | 分类名称（多语言 key 或直接中文） |
| code | varchar(50) | uniqueIndex，分类代码（account / recharge / wallet / withdrawal / c2c / affiliate / technical / other） |
| default_priority | varchar(10) | 默认优先级（low/normal/high） |
| enabled | bool | 是否启用 |
| sort_order | int | 排序 |
| created_at / updated_at | | |

### 6.2 第一版默认分类

| code | name | default_priority | 说明 |
|---|---|---|---|
| account | 账号问题 | normal | 登录、注册、密码、2FA |
| recharge | 充值咨询 | normal | 充值流程咨询（退款走 After-Sale） |
| wallet | 钱包问题 | high | 余额异常、ledger 疑问 |
| withdrawal | 提现问题 | high | 提现流程、状态咨询（审批走 Withdrawal Admin） |
| c2c | C2C 问题 | high | C2C 交易咨询（资金争议走 C2C Dispute） |
| affiliate | 返利问题 | normal | 邀请、佣金咨询 |
| technical | 技术问题 | normal | 页面报错、功能异常 |
| other | 其他 | normal | 未分类问题 |

### 6.3 Admin 可配置

Admin 可以新增/编辑/启用/禁用/排序分类。第一版至少上述 8 个默认分类，通过 migration seed 初始化。

---

## 七、Ticket 状态机

### 7.1 最终状态机（5 态）

```
                    ┌──────────────┐
                    │    open      │
                    │ (用户创建)    │
                    └──────┬───────┘
                           │ Admin 回复
                           ▼
                    ┌──────────────┐
              ┌────▶│ waiting_user │────┐
              │     │ (等待用户回复)  │    │ 用户回复
              │     └──────────────┘    ▼
              │                    ┌──────────────┐
              │     Admin 回复     │waiting_support│
              │     ◀────────────│ (等待客服回复)  │
              │                    └──────┬───────┘
              │                           │ Admin 标记解决
              │                           ▼
              │                    ┌──────────────┐
              │                    │   resolved   │
              │                    │  (已解决)     │
              │                    └──────┬───────┘
              │                           │ 用户确认 / 超时自动
              │                           ▼
              │                    ┌──────────────┐
              └────────────────────│   closed     │
                  用户重新打开        │  (已关闭)     │
                                   └──────────────┘
```

### 7.2 合法流转表

| 当前状态 | 操作 | 目标状态 | 操作方 |
|---|---|---|---|
| - | 创建工单 | open | User |
| open | Admin 回复 | waiting_user | Admin |
| open | Admin 标记解决 | resolved | Admin |
| open | Admin 关闭 | closed | Admin |
| waiting_user | User 回复 | waiting_support | User |
| waiting_user | Admin 标记解决 | resolved | Admin |
| waiting_user | Admin 关闭 | closed | Admin |
| waiting_support | Admin 回复 | waiting_user | Admin |
| waiting_support | Admin 标记解决 | resolved | Admin |
| waiting_support | Admin 关闭 | closed | Admin |
| resolved | User 重新打开（reopen window 内） | waiting_support | User |
| resolved | 超时自动关闭 | closed | System |
| resolved | Admin 关闭 | closed | Admin |
| closed | - | 终态（不可重新打开，需创建新工单） | - |

### 7.3 状态机规则

- **禁止 Controller 直接写 status**，必须建立单一 transition guard（`support_ticket` 状态机模块）
- **completed/canceled/expired 不适用**（这是 C2C 状态机，Ticket 用自己的 5 态）
- **resolved 后 reopen**：用户在 reopen window（建议 7 天）内可以重新打开，状态变为 waiting_support。超过 window 后只能创建新工单
- **closed 为终态**：不可重新打开，需创建新工单
- **每次状态变更记录 audit**（见第二十九节）

---

## 八、Priority

### 8.1 优先级等级

| 等级 | 说明 | 用户可选 |
|---|---|---|
| low | 低优先级（一般咨询） | ✅ |
| normal | 普通（默认） | ✅ |
| high | 高优先级（资金相关问题） | ❌ 由 category default 或 Admin 调整 |
| urgent | 紧急（严重影响使用/资金安全） | ❌ 仅 Admin 可设置 |

### 8.2 规则

- 用户创建工单时只能选 low / normal（默认 normal）
- high / urgent 由 category 的 `default_priority` 自动设置（如 wallet/withdrawal/c2c 默认 high），或 Admin 手动调整
- Admin 可以随时修改 priority，记录 audit

---

## 九、User API

### 9.1 端点清单

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| GET | /api/v1/support/categories | 获取启用的分类列表 | User JWT |
| GET | /api/v1/support/tickets | 我的工单列表（分页+状态筛选） | User JWT |
| POST | /api/v1/support/tickets | 创建工单（含首条消息+附件） | User JWT + 限流 |
| GET | /api/v1/support/tickets/:id | 工单详情（含消息分页+附件） | User JWT + 本人校验 |
| POST | /api/v1/support/tickets/:id/replies | 回复工单（含附件） | User JWT + 本人校验 + 限流 |
| POST | /api/v1/support/tickets/:id/close | 用户关闭工单（resolved 状态下确认关闭） | User JWT + 本人校验 |
| POST | /api/v1/support/tickets/:id/reopen | 重新打开（resolved 后 reopen window 内） | User JWT + 本人校验 |
| POST | /api/v1/support/attachments | 上传附件 | User JWT + 限流 |
| GET | /api/v1/support/attachments/:id | 读取附件（受控，校验权限） | User JWT + 本人校验 |

### 9.2 创建工单请求

```json
{
  "category_id": 3,
  "subject": "提现一直显示 processing",
  "body": "我昨天提交了提现，到现在还是 processing 状态，麻烦帮忙看看。",
  "priority": "normal",
  "biz_type": "withdrawal",
  "biz_id": 12345,
  "attachment_ids": [1, 2]
}
```

### 9.3 回复请求

```json
{
  "body": "好的，我已经按照要求操作了，请看截图。",
  "attachment_ids": [3]
}
```

### 9.4 IDOR 防护

- 所有 ticket/:id 操作必须校验 `ticket.user_id == current_user_id`
- 非本人返回 404（不泄露 ticket 存在性）或 403（按现有 HCZ 规范 fail-closed）
- attachment 读取同样校验用户是 ticket 参与者

---

## 十、Admin API

### 10.1 端点清单

| 方法 | 路径 | 说明 | 权限 |
|---|---|---|---|
| GET | /api/admin/v1/support/overview | 概览统计（待处理/处理中/已解决/今日新增） | support.ticket.read |
| GET | /api/admin/v1/support/tickets | 工单列表（分页+状态/分类/优先级/分配/未读筛选） | support.ticket.read |
| GET | /api/admin/v1/support/tickets/:id | 工单详情（含消息+附件+audit） | support.ticket.read |
| POST | /api/admin/v1/support/tickets/:id/replies | 客服回复 | support.ticket.reply |
| PATCH | /api/admin/v1/support/tickets/:id/status | 修改状态（resolved/closed） | support.ticket.close |
| PATCH | /api/admin/v1/support/tickets/:id/priority | 修改优先级 | support.ticket.assign |
| POST | /api/admin/v1/support/tickets/:id/assign | 分配/领取工单 | support.ticket.assign |
| GET | /api/admin/v1/support/categories | 分类列表 | support.category.manage |
| POST | /api/admin/v1/support/categories | 新建分类 | support.category.manage |
| PUT | /api/admin/v1/support/categories/:id | 编辑分类 | support.category.manage |
| DELETE | /api/admin/v1/support/categories/:id | 删除分类（软删除/禁用） | support.category.manage |
| GET | /api/admin/v1/support/tickets/:id/audits | 工单操作审计日志 | support.ticket.read |
| GET | /api/admin/v1/support/attachments/:id | 读取附件 | support.ticket.read |

### 10.2 assigned_admin_id

第一版需要。Admin 可以：
- 领取工单（assign 给自己）
- 分配给其他 Admin（assign 给指定 admin_id）
- 未分配的工单任何客服可领取

**暂不做**：自动轮询分配、技能组路由、SLA 自动升级。

### 10.3 Admin 回复不需要 Payment Compliance

Ticket 回复本身不涉及资金操作，不需要 Payment Compliance 中间件。
但如果 Ticket UI 跳转到退款/提现/C2C 仲裁页面，**真正的资金操作继续走对应业务域的 Payment Compliance**。

---

## 十一、分配机制

### 11.1 第一版：手动领取/分配

- 新工单 `assigned_admin_id = null`
- Admin 在 Ticket List 看到未分配工单，可以点击"领取"（assign 给自己）
- Admin 可以将工单分配给其他客服
- 已分配工单只有分配人或管理员可操作（或所有客服可查看，仅分配人可回复——第一版建议所有客服可回复，assigned 仅作标识）

### 11.2 数据模型预留

`assigned_admin_id` 字段已预留，未来可扩展：
- 自动轮询分配（Round Robin）
- 技能组路由（按 category 分配给对应技能组）
- SLA 自动升级（超时未响应自动升级给高级客服）

### 11.3 并发分配防护

- Admin A/B 同时领取同一工单：使用 ticket row lock + 状态机校验（assigned_admin_id 为 null 才允许 assign）
- 分配操作记录 audit（谁在什么时候分配给谁）

---

## 十二、User IDOR

### 12.1 规则

用户必须只能：
- 查看自己的 ticket（`ticket.user_id == current_user_id`）
- 回复自己的 ticket
- 关闭自己的 ticket
- 读取自己 ticket 的附件和消息

### 12.2 失败行为

- User A 访问 User B 的 ticket：返回 404（不泄露存在性）或 403（按现有 HCZ 规范 fail-closed）
- User A 回复 User B 的 ticket：同上
- Payment Method IDOR：本次不涉及（Ticket 无 Payment Method）
- 附件读取：校验 attachment 所属 ticket 的 user_id == current_user_id

### 12.3 实现方式

- 在 service 层统一校验 ownership，不依赖 Handler 层
- 封装 `RequireTicketOwnership(ctx, ticketID, userID)` helper
- 所有 ticket 操作（detail/reply/close/reopen/attachment）经过同一校验

---

## 十三、Admin RBAC

### 13.1 独立权限

| 权限 | 说明 |
|---|---|
| support.ticket.read | 查看工单列表/详情 |
| support.ticket.reply | 回复工单 |
| support.ticket.assign | 分配/领取工单、修改优先级 |
| support.ticket.close | 标记解决/关闭工单 |
| support.category.manage | 分类管理（CRUD） |

### 13.2 不复用财务权限

- Ticket 权限独立于 payment.* / wallet.* / withdrawal.* 等财务权限
- `support` admin 角色可以预绑定全部 support.* 权限
- 财务管理员不一定有 Ticket 权限，客服不一定有财务权限

### 13.3 Ticket 回复不需要 Payment Compliance

- Ticket 回复是文本操作，不涉及资金，不需要 Payment Compliance 中间件
- Admin 仲裁（C2C）、提现审批等资金操作仍在各自业务域走 Payment Compliance
- Ticket 中如果涉及资金操作，客服只能指导用户去对应流程，不能在 Ticket 中执行

---

## 十四、未读/红点

### 14.1 两种方案比较

| 方案 | 实现 | 优点 | 缺点 |
|---|---|---|---|
| **A. Ticket 层计数器** | support_tickets 表加 user_unread_count / admin_unread_count，回复时原子 +1，查看时清零 | 简单、查询快、不需要额外表、稳定 | 计数器可能与实际消息数有偏差（需保证原子更新） |
| B. Message read cursor | 每张 ticket 每个用户维护 last_read_message_id，未读数 = 新消息数 | 精确、可定位到具体消息 | 复杂、需要额外表或字段、并发更新风险高 |

### 14.2 第一版推荐：方案 A（Ticket 层计数器）

**理由**：
1. 简单稳定，第一版优先可靠
2. 查询 Ticket List 时直接返回 unread_count，不需要额外计算
3. 原子更新：在同一事务中创建 message + 更新对方 unread_count + 更新 last_reply_by/at
4. 查看 Ticket Detail 时清零本方 unread_count（`UPDATE ... SET user_unread_count = 0 WHERE id = ? AND user_id = ?`）

### 14.3 计数器更新规则

| 事件 | user_unread_count | admin_unread_count |
|---|---|---|
| User 创建工单 | 0 | +1 |
| User 回复 | 0（本方不变） | +1 |
| Admin 回复 | +1 | 0（本方不变） |
| Admin 标记解决 | +1（通知用户） | 0 |
| System 消息（关闭/分配） | +1 | +1（或按业务规则） |
| User 查看详情 | 清零 | 不变 |
| Admin 查看详情 | 不变 | 清零 |

### 14.4 Admin 未读总览

Admin 前端 Ticket List 显示 `admin_unread_count > 0` 的工单标记红点，顶部显示总未读数。
第一版用 15-30s polling 刷新未读数，不引入 WebSocket。

---

## 十五、User Notification

### 15.1 接入现有 Phase 1 Notification

复用 `internal/modules/notification/application/send.go`，新增 event_type：

| event_type | 触发时机 | 通知对象 | 重要性 |
|---|---|---|---|
| ticket_created | 用户创建工单成功（可选，仅确认） | User | 低（第一版可不发） |
| ticket_admin_replied | Admin 回复工单 | User | **高（必须）** |
| ticket_resolved | Admin 标记工单已解决 | User | 高 |
| ticket_closed | 工单关闭（用户确认/超时/Admin 关闭） | User | 中 |

### 15.2 通知内容

- title：如"客服回复了您的工单"、"您的工单已解决"
- body：包含 ticket_no + subject 摘要
- biz_type："support_ticket"
- biz_id：ticket.id
- 点击 Notification 跳转：`/support/tickets/:id`（User 前端路由）

### 15.3 通知规则

- 通知失败不阻塞 Ticket 主事务（异步发送，复用现有 async queue）
- 同一 ticket 同一事件不重复通知（复用现有 notification dedupe 机制）
- 用户已在 Ticket Detail 页面时仍发送通知（红点 + 站内信），不做"在线则不发"的复杂逻辑

---

## 十六、Admin Notification

### 16.1 现有系统审计

- **无独立 Admin Notification 系统**。现有 notification 模块是面向 User 的（user_notifications 表 + notification_logs）
- Admin 端没有类似的站内通知中心

### 16.2 第一版方案：admin_unread_count + Polling

- 不新建 Admin Notification 系统
- 不混入 user_notifications 表
- 使用 `support_tickets.admin_unread_count` 实现红点
- Admin 前端：
  - Ticket List 每 15-30s polling 刷新列表和未读数
  - 顶部导航显示"客服"红点（总未读数）
  - 新工单提示（admin_unread_count > 0 且 status=open）

### 16.3 未来扩展（Full V1）

- 独立 Admin Notification 系统
- 新工单实时通知（WebSocket 或 SSE）
- 工单分配通知
- SLA 超时告警

---

## 十七、轮询

### 17.1 第一版不引入 WebSocket

使用 HTTP polling，简单稳定。

### 17.2 Polling 策略

| 页面 | 轮询内容 | 间隔 | 条件 |
|---|---|---|---|
| Ticket Detail（User） | 刷新 ticket + 最新消息 | 15s | status in (open, waiting_user, waiting_support, resolved)，页面 visible |
| Ticket Detail（Admin） | 同上 | 15s | 同上 |
| Ticket List（User） | 刷新列表 + 未读数 | 30s | 页面 visible |
| Ticket List（Admin） | 刷新列表 + 未读数 | 15s | 页面 visible（客服需要更快响应） |
| terminal 状态 | 停止 polling | - | status = closed |

### 17.3 页面可见性

- 使用 `document.visibilityState` 监听
- 页面 hidden 时暂停 polling（`clearInterval`）
- 页面重新 visible 时立即刷新一次 + 恢复 polling

### 17.4 消息分页加载

- Ticket Detail 初始加载最近 50 条消息
- 向上滚动加载历史（cursor pagination，按 id 倒序）
- 新消息通过 polling 追加（只拉取 created_at > 最后一条的消息）

---

## 十八、消息顺序与分页

### 18.1 排序

- 消息按 `created_at ASC, id ASC` 稳定排序
- 同一秒内的消息按 id 排序保证确定性
- 不使用 `created_at DESC` 后前端 reverse（避免分页边界问题）

### 18.2 分页策略

| 场景 | 策略 |
|---|---|
| 初始加载 | 最近 50 条（`ORDER BY id DESC LIMIT 50`，结果前端 reverse 为正序） |
| 向上加载历史 | cursor pagination：`WHERE id < :oldest_id ORDER BY id DESC LIMIT 50` |
| 轮询新消息 | `WHERE id > :newest_id ORDER BY id ASC LIMIT 50` |
| 不使用 offset | 避免深分页性能问题 |

### 18.3 第一版简化

- 如果工单消息量不大（通常 < 100 条），可以一次加载全部消息
- 但 API 设计仍支持分页，前端实现"加载更多"按钮
- 超长历史（> 200 条）才必须分页

---

## 十九、并发

### 19.1 并发场景分析

| 场景 | 风险 | 防护 |
|---|---|---|
| User/Admin 同时回复 | last_reply_by 状态覆盖错误、unread count 丢失 | ticket row lock + 原子更新 unread_count + message 独立插入（不依赖 last_reply 做乐观锁） |
| Admin A/B 同时领取 | 双分配 | ticket row lock + 校验 assigned_admin_id IS NULL |
| close 与 reply 并发 | 已关闭工单仍被回复 | 状态机校验（closed 为终态，不允许回复）+ row lock |
| resolved 后 User 回复（reopen） | 状态竞争 | 状态机校验（仅 resolved 且在 reopen window 内允许 reopen）+ row lock |
| User 快速双击回复 | 重复消息 | 前端禁用按钮 + 后端 Idempotency-Key + rate limit |
| Admin 快速双击回复 | 重复消息 | 同上 |

### 19.2 防护机制

1. **Ticket row lock**：所有写操作（reply/status/assign/priority）先 `SELECT ... FOR UPDATE` 锁定 ticket 行
2. **状态机 guard**：每次操作前校验当前状态是否允许该操作
3. **原子 unread 更新**：在同一事务中 `UPDATE support_tickets SET admin_unread_count = admin_unread_count + 1 WHERE id = ?`
4. **Idempotency-Key**：回复操作支持 Idempotency-Key，防止双击/网络重试
5. **Rate limit**：用户回复限流（如 10 秒内最多 3 条），防止刷消息

### 19.3 不使用乐观锁

- Ticket 不使用 version 字段做乐观锁
- 因为 ticket 行的更新频率不高（每次回复更新一次），row lock 足够
- message 插入是独立 insert，不需要锁
- unread_count 使用原子 `+1` / `= 0`，不需要读-改-写

---

## 二十、防滥用

### 20.1 Rate Limit

复用现有 Redis 限流中间件，新增规则：

| 操作 | 规则 | 限流 |
|---|---|---|
| 创建工单 | user_id + "ticket_create" | 每小时最多 5 单 |
| 回复工单 | user_id + ticket_id + "ticket_reply" | 每 10 秒最多 3 条 |
| 上传附件 | user_id + "ticket_attach" | 每分钟最多 10 个文件 |
| Admin 回复 | admin_id + ticket_id | 每 10 秒最多 5 条 |

### 20.2 内容长度限制

| 字段 | 最大长度 |
|---|---|
| subject | 255 字符 |
| body（单条消息） | 5000 字符 |
| description（分类） | 1000 字符 |

### 20.3 防 spam

- 同一用户短时间内创建大量工单：rate limit 拦截
- 同一工单高频回复：rate limit 拦截
- 重复内容：第一版不做语义去重，rate limit 足够
- 超大附件：UploadConfig max_size 拦截（10MB）
- 附件数量限制：每消息 5 个，每工单 20 个

### 20.4 账号级防护

- 被 C2C banned 的用户不影响 Ticket（Ticket 是客服通道，不应被 C2C 禁用影响）
- 被全站封禁的用户不能创建工单（user.status != active 时拒绝）
- 新注册用户可以创建 Ticket（不需要 cooldown，客服通道应保持开放）

---

## 二十一、内容安全

### 21.1 第一版：纯文本 + 安全换行

- **不允许**用户提交 raw HTML
- **不允许** Markdown（第一版）
- 消息 body 存储为纯文本
- 前端渲染时：
  - HTML 转义（防止 XSS）
  - 换行符转换为 `<br>`（安全换行）
  - URL 自动识别为链接（可选，第一版可不做）

### 21.2 防 XSS

- 后端：不做 HTML 过滤（因为不允许 HTML），但存储纯文本
- 前端：使用框架自带的文本插值（Vue `{{ }}`），不使用 `v-html`
- 附件：图片通过 `<img src>` 展示，PDF/txt/log 通过下载链接，不渲染为 HTML

### 21.3 未来扩展（Full V1）

- 支持 Markdown（安全子集，不允许原始 HTML）
- 代码块
- 表情
- 富文本编辑器

---

## 二十二、附件安全

### 22.1 安全措施清单

| 措施 | 实现 |
|---|---|
| MIME whitelist | 复用 FileUploader，仅允许 image/jpeg, image/png, image/webp, application/pdf, text/plain |
| extension whitelist | .jpg, .jpeg, .png, .webp, .pdf, .txt, .log |
| MIME + extension 双重校验 | 防止扩展名伪装（如 .exe 改名为 .jpg） |
| 文件大小限制 | 10MB（复用 upload.max_size） |
| random object key | 存储路径使用 UUID，如 `support/2026/10/05/{uuid}.{ext}` |
| 不使用原始 filename 作路径 | 原始 filename 仅存 file_name 字段用于展示 |
| 私有访问 | 附件不放在公开 CDN 目录，通过受控 API 读取 |
| 读取权限校验 | GET /api/v1/support/attachments/:id 校验用户是 ticket 参与者或 Admin |
| 禁止可执行文件 | MIME + extension 双重拦截 .exe/.bat/.sh/.js/.html/.php 等 |
| 禁止压缩包 | 第一版不支持 .zip/.rar（防止恶意文件打包） |
| 图片不自动执行 | 通过 `<img>` 标签展示，SVG 第一版不支持（防止 SVG XSS） |

### 22.2 存储方式

- 现有存储为**本地文件系统**（FileStore 接口 + local 实现）
- Ticket 附件存储在本地文件系统的 `uploads/support/` 目录
- 不使用原始 filename，使用 UUID + 原扩展名
- 文件路径存 `support_ticket_attachments.file_path`
- 读取时通过受控 API（校验权限后 `Open(file_path)` 返回文件流）

### 22.3 未来扩展（Full V1）

- S3/OSS 等对象存储
- 私有 bucket + signed URL（限时访问）
- 图片内容审核（NSFW 检测）
- 病毒扫描
- 支持更多格式（.docx, .xlsx, .zip 密码保护）

---

## 二十三、User 前端

### 23.1 页面清单

| 页面 | 路由 | 功能 |
|---|---|---|
| 帮助与支持入口 | /support | 入口页：创建工单按钮 + 我的工单入口 + 常见问题（可选） |
| 我的工单 | /support/tickets | 工单列表（状态筛选 + 未读红点 + 分页） |
| 创建工单 | /support/tickets/new | 表单：分类、subject、body、优先级（low/normal）、关联业务（可选）、附件上传 |
| 工单详情 | /support/tickets/:id | 对话时间线 + 回复框 + 附件 + 状态展示 + 操作按钮（关闭/重新打开） |

### 23.2 工单详情布局

**PC（双栏或单栏宽页）**：
- 左侧：工单信息（ticket_no、状态、优先级、分类、创建时间、关联业务）
- 右侧：对话时间线（消息气泡，user/admin/system 不同样式）+ 底部回复框（textarea + 附件上传 + 发送按钮）

**Mobile（单列）**：
- 顶部：工单状态 + 优先级
- 中部：对话时间线（滚动）
- 底部：sticky 回复框

### 23.3 关键交互

- 未读红点：Ticket List 中 `user_unread_count > 0` 的工单标记红点
- 进入详情后自动清零未读
- 状态为 waiting_user 时高亮提示"等待您的回复"
- 状态为 resolved 时显示"问题已解决"+ 确认关闭按钮 + 重新打开按钮（7 天内）
- 状态为 closed 时显示"工单已关闭"+ 创建新工单按钮
- 附件上传：拖拽或点击上传，显示上传进度，支持图片预览
- Polling：非终态 15s 刷新，页面 hidden 暂停

### 23.4 视觉方向

- 保持 HCZ 现代生活服务 + 数字钱包风格
- 不做传统客服系统的厚重风格
- 强调：清晰、易操作、安全感
- 对话气泡简洁，Admin 消息可标记"客服"标签

---

## 二十四、Admin 前端

### 24.1 页面清单

| 页面 | 功能 |
|---|---|
| C2C... 不对，是 Support Dashboard | 概览：待处理数、处理中数、已解决数、今日新增、未读总数、平均响应时间（可选） |
| Ticket List | 工单列表：筛选（status/category/priority/assigned/unread）、搜索（ticket_no/subject）、批量领取（可选） |
| Ticket Detail | 对话时间线 + 回复框 + 侧栏（状态/优先级/分配/分类/关联业务操作）+ audit 日志 |
| Categories | 分类管理：列表 + 新增/编辑/启用禁用/排序 |

### 24.2 Ticket Detail 侧栏操作

- **状态**：下拉切换（resolved/closed），需 confirm
- **优先级**：下拉切换（low/normal/high/urgent）
- **分配**：领取按钮 / 分配给其他客服（下拉选择 Admin）
- **分类**：可修改分类
- **关联业务**：显示 biz_type + biz_id，点击跳转对应业务详情页（只读）
- **Audit**：展开查看操作历史（谁在什么时候做了什么）

### 24.3 关键交互

- 未读红点：`admin_unread_count > 0` 的工单标记
- 进入详情后自动清零未读
- 新工单（status=open, assigned=null）高亮提示"待领取"
- 回复框支持快捷回复（Full V1，第一版可不做）
- 状态为 waiting_support 时高亮提示"等待您回复"
- Polling：15s 刷新列表和详情

### 24.4 筛选

- status：全部 / open / waiting_user / waiting_support / resolved / closed
- category：下拉
- priority：全部 / low / normal / high / urgent
- assigned：全部 / 未分配 / 我处理的 / 指定客服
- unread：仅显示未读

---

## 二十五、跨业务关联

### 25.1 设计

Ticket 可选关联业务，使用通用字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| biz_type | varchar(30) | 业务类型：order / withdrawal / c2c_trade / recharge（nullable） |
| biz_id | uint | 业务 ID（nullable） |

### 25.2 规则

- **只查看/跳转，不修改业务状态**
- Ticket 详情页显示关联业务摘要，点击跳转到对应业务详情页
- 客服不能在 Ticket 中修改 order/withdrawal/c2c_trade 的任何状态
- 如涉及资金操作，客服在 Ticket 中指导用户去对应流程，或 Admin 切换到对应业务域执行操作

### 25.3 关联类型

| biz_type | 跳转目标 | 说明 |
|---|---|---|
| order | Admin Order Detail | 订单咨询 |
| withdrawal | Admin Withdrawal Detail | 提现咨询（审批走 Withdrawal Admin） |
| c2c_trade | Admin C2C Trade Detail | C2C 咨询（争议走 C2C Dispute） |
| recharge | Admin Recharge Detail | 充值咨询（退款走 After-Sale） |

### 25.4 创建工单时关联

- 用户创建工单时可选择关联业务（如从 Withdrawal Detail 页"联系客服"按钮跳转，自动带入 biz_type=withdrawal, biz_id=xxx）
- 也可以不关联（biz_type=null）
- 关联后不可修改（第一版），如需修改需 Admin 操作

---

## 二十六、与 After-Sale 的关系

### 26.1 明确边界

| 场景 | 应使用 | 不应使用 |
|---|---|---|
| Recharge Order 未收到款 | After-Sale（not_received） | Ticket |
| Recharge Order 全额退款 | After-Sale | Ticket |
| Recharge Order 部分退款 | After-Sale | Ticket |
| Recharge 流程咨询（如何充值、支持哪些渠道） | Ticket | After-Sale |
| Recharge 到账延迟咨询 | Ticket（可关联 recharge） | After-Sale（除非用户要退款） |
| 充值失败原因咨询 | Ticket | After-Sale |

### 26.2 UI 引导

- User 从 Recharge Order 详情页点击"有问题"时：
  - 如果订单状态允许 After-Sale，优先显示"申请售后/退款"按钮
  - 同时提供"联系客服"入口（创建 Ticket，自动关联 biz_type=recharge）
- Ticket 分类中"充值咨询"的描述应提示：退款请使用售后流程

### 26.3 数据隔离

- After-Sale 表 `after_sale_tickets` 在 order 模块，不动
- Ticket 表 `support_tickets` 在独立 support 模块
- 两者不共享表、不共享状态机、不共享 API
- After-Sale 不需要持续对话（单条 reason+description），Ticket 需要多轮对话

---

## 二十七、与 C2C Dispute 的关系

### 27.1 明确边界

| 场景 | 应使用 | 不应使用 |
|---|---|---|
| C2C paid 后买家未收到法币/卖家未收到款 | C2C Dispute → Admin Arbitration | Ticket |
| C2C 交易资金争议 | C2C Dispute → Admin Arbitration | Ticket |
| C2C 交易流程咨询（如何买、如何卖） | Ticket | C2C Dispute |
| C2C 挂单发布问题 | Ticket | C2C Dispute |
| C2C 收款方式配置问题 | Ticket | C2C Dispute |
| C2C 交易状态异常（非资金争议） | Ticket（可关联 c2c_trade） | C2C Dispute |

### 27.2 规则

- **C2C paid 后资金争议必须走正式 C2C Dispute/Arbitration**
- **不能通过 Ticket 让客服手工修改交易资金**（不能在 Ticket 中执行 Settle/Unfreeze）
- Ticket 可以关联 c2c_trade（biz_type=c2c_trade, biz_id=trade_id），客服点击跳转到 C2C Trade Detail
- 如果 Ticket 中用户描述的是资金争议，客服应引导用户去 C2C Trade 页发起 Dispute，或 Admin 在 C2C 域执行操作

### 27.3 数据隔离

- C2C Dispute 表 `c2c_disputes` 在 c2c 模块，不动
- Ticket 表 `support_tickets` 在独立 support 模块
- C2C Arbitration 执行资金操作（Settle/Unfreeze），Ticket 不执行任何资金操作

---

## 二十八、与 Withdrawal 的关系

### 28.1 明确边界

| 场景 | 应使用 | 不应使用 |
|---|---|---|
| 提现审批（approve/reject） | Withdrawal Admin API | Ticket |
| 提现资金状态变更 | Withdrawal Domain | Ticket |
| 提现流程咨询（如何提现、手续费、到账时间） | Ticket | Withdrawal Admin |
| 提现状态异常（一直 processing） | Ticket（可关联 withdrawal） | Ticket 中直接 approve/reject |
| 提现被拒申诉 | Ticket（可关联 withdrawal），Admin 可在 Withdrawal 域复核 | Ticket 中直接改状态 |

### 28.2 规则

- **提现资金状态由 Withdrawal Domain 管理**
- **不能通过 Ticket Reply 执行**：approve withdrawal、reject withdrawal、Wallet credit、任何资金操作
- Ticket 可以关联 withdrawal（biz_type=withdrawal, biz_id=withdrawal_id），客服点击跳转到 Withdrawal Detail
- 如 Ticket 中发现提现确实有问题，Admin 切换到 Withdrawal 域执行操作（走 Payment Compliance），Ticket 中记录处理结果

---

## 二十九、审计日志

### 29.1 support_ticket_audits 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| ticket_id | uint | index，关联工单 |
| admin_id | uint | 操作 Admin ID |
| action | varchar(30) | 操作类型（assign / status_change / priority_change / category_change / reply / close / reopen） |
| old_value | text | 旧值（JSON 或文本，可选） |
| new_value | text | 新值（JSON 或文本，可选） |
| reason | varchar(255) | 操作原因（可选，关闭/重新打开时必填） |
| created_at | | |

### 29.2 是否复用现有 auditlog 模块

**第一版：独立 support_ticket_audits 表，不复用 auditlog 模块**

理由：
1. 现有 auditlog 模块专注于认证事件（admin_login / user_login / authz 变更），domain 模型是 auth-specific
2. Ticket audit 需要记录 ticket_id / action / old_value / new_value，与 auth audit 结构不同
3. 独立表查询更高效（按 ticket_id 索引）
4. 未来如果需要统一审计视图，可以做聚合查询，不影响独立存储

### 29.3 记录范围

| 操作 | 是否记录 audit | 说明 |
|---|---|---|
| Admin 回复 | 是（action=reply） | 消息内容本身存在 messages 表，audit 只记录"谁在什么时候回复了" |
| 分配/领取 | 是（action=assign） | 记录 old assigned_admin_id → new assigned_admin_id |
| 状态变更 | 是（action=status_change） | 记录 old status → new status + reason |
| 优先级变更 | 是（action=priority_change） | 记录 old priority → new priority |
| 分类变更 | 是（action=category_change） | 记录 old category_id → new category_id |
| 关闭 | 是（action=close） | 记录 reason |
| 重新打开 | 是（action=reopen） | 记录 reason |
| User 回复 | 否 | User 操作不需要 admin audit（消息本身有记录） |
| User 创建工单 | 否 | 工单创建本身有记录 |
| User 查看工单 | 否 | 不记录查看行为 |

### 29.4 Admin 可查看

Ticket Detail 页展示 audit 日志时间线（与对话消息分开显示，或混合显示 system 消息）。
第一版建议在侧栏单独区域展示 audit 日志，不混入对话。

---

## 三十、MVP 与 Full V1

### 30.1 Ticket MVP（第一版）

**必须包含**：
- 5 张新表（categories / tickets / messages / attachments / audits）
- Category 管理（Admin CRUD + 默认 8 分类 seed）
- User 创建工单（含首条消息 + 附件）
- User 工单列表（分页 + 状态筛选 + 未读红点）
- User 工单详情（对话时间线 + 回复 + 附件 + 状态展示）
- User 关闭/重新打开工单
- Admin 工单列表（筛选 + 搜索 + 未读）
- Admin 工单详情（对话 + 回复 + 状态/优先级/分配/分类操作 + audit）
- Admin 回复（纯文本 + 附件）
- 状态机 5 态（open / waiting_user / waiting_support / resolved / closed）
- unread 计数器（user_unread_count / admin_unread_count）
- User Notification（ticket_admin_replied / ticket_resolved / ticket_closed）
- 附件上传（复用 FileUploader，User 上传端点，MIME whitelist，random object key，私有访问）
- Basic rate limit（创建/回复/上传限流）
- User 前端（帮助入口 / 我的工单 / 创建工单 / 工单详情）
- Admin 前端（Dashboard / Ticket List / Ticket Detail / Categories）
- IDOR 防护（用户只能操作自己的工单）
- Admin RBAC（support.ticket.* 独立权限）
- 跨业务关联（biz_type + biz_id，只读跳转）
- Polling（15-30s，页面 hidden 暂停，终态停止）

### 30.2 Ticket Full V1（后续扩展）

- SLA 管理（响应时间/解决时间 SLA，超时自动升级）
- 自动分配（Round Robin / 技能组路由）
- 快捷回复（canned replies，Admin 可保存常用回复模板）
- CSAT（用户满意度评价，工单关闭后弹出评价）
- Internal notes（Admin 内部备注，用户不可见）
- Tags（工单标签，灵活分类）
- Merge ticket（合并重复工单）
- 高级报表（工单量趋势、响应时间、解决率、客服绩效）
- Realtime WebSocket（实时消息推送，替代 polling）
- Admin Notification 系统（新工单/分配/SLA 超时实时通知）
- Markdown 支持（安全子集）
- 更多附件格式（.docx, .xlsx, 密码保护 .zip）
- 图片内容审核（NSFW 检测）
- 工单模板（常见问题预填）
- 客服工作时间（自动回复"非工作时间"）

---

## 三十一、最终明确回答

### 31.1 核心问题回答

| # | 问题 | 回答 |
|---|---|---|
| 1 | 现有代码有多少可复用 | **高复用率**：Upload FileUploader（文件校验+落盘）、Redis 限流中间件、Notification 发送系统（新增 event_type 即可）、Admin JWT/RBAC、i18n、AutoMigrate registry。**无**现有通用 Ticket 模型/对话基础设施，需新建 support 模块 |
| 2 | Ticket 与 After-Sale 是否彻底分域 | **是**。After-Sale 仅限 Recharge Order 退款（after_sale_tickets 在 order 模块，可执行退款），Ticket 是通用客服支持（独立 support 模块，不执行任何资金操作）。两者不共享表/状态机/API。Recharge 退款问题 UI 优先引导 After-Sale |
| 3 | 需要几张新表 | **5 张**：support_ticket_categories / support_tickets / support_ticket_messages / support_ticket_attachments / support_ticket_audits |
| 4 | Ticket 最终状态机 | **5 态**：open → waiting_user（Admin 回复）→ waiting_support（User 回复）→ resolved（Admin 标记解决）→ closed（用户确认/超时/Admin 关闭）。resolved 后 7 天内可 reopen（→ waiting_support），closed 为终态。禁止 Controller 直接写 status |
| 5 | unread/read 如何实现 | **Ticket 层计数器**：support_tickets 表加 user_unread_count / admin_unread_count。回复时原子 +1 对方计数，查看详情时清零本方计数。比 message read cursor 简单稳定，查询快 |
| 6 | Attachment 如何安全存储 | 复用现有 FileUploader（MIME/extension/size 校验），新增 User 上传端点，random object key（UUID），不使用原始 filename 作路径，私有访问（通过受控 API 校验权限后读取，不暴露公开 CDN）。MIME whitelist：jpg/jpeg/png/webp/pdf/txt/log，最大 10MB，每消息 5 个，禁止可执行文件和压缩包 |
| 7 | User Notification 如何接 | 复用 Phase 1 Notification 系统，新增 4 种 event_type：ticket_admin_replied（必须）、ticket_resolved、ticket_closed、ticket_created（可选）。biz_type=support_ticket，biz_id=ticket.id，点击跳 /support/tickets/:id。异步发送不阻塞主事务，复用 dedupe 机制 |
| 8 | Admin Notification 如何接 | **无独立 Admin Notification 系统**。第一版用 admin_unread_count + Admin 前端 15s polling 实现红点和新工单提示。不混入 user_notifications。Full V1 再建独立 Admin Notification |
| 9 | User/Admin API 最小范围 | User：GET categories / GET tickets / POST tickets / GET ticket/:id / POST reply / POST close / POST reopen / POST attachments / GET attachment/:id。Admin：GET overview / GET tickets / GET ticket/:id / POST reply / PATCH status / PATCH priority / POST assign / categories CRUD / GET audits / GET attachment |
| 10 | User/Admin UI 页面范围 | User：帮助入口 / 我的工单 / 创建工单 / 工单详情（对话时间线+回复+附件+未读红点）。Admin：Dashboard / Ticket List（筛选+搜索+未读）/ Ticket Detail（对话+回复+状态/优先级/分配/分类+audit）/ Categories |
| 11 | Ticket 与 Order/Withdrawal/C2C 如何关联但不越权 | Ticket 可选 biz_type + biz_id（order/withdrawal/c2c_trade/recharge），**只查看/跳转，不修改业务状态**。资金操作必须走对应正式业务流程（After-Sale / Withdrawal Admin / C2C Arbitration）。客服在 Ticket 中提供指导，实际操作在对应业务域执行。不能通过 Ticket 执行退款/调账/Settle/Unfreeze/approve/reject |
| 12 | MVP 最小实施范围 | 5 表 + category + create/list/detail + User/Admin reply + assignment + 5 态状态机 + unread 计数 + user notification（3 事件）+ attachments（复用 FileUploader）+ basic rate limit + User/Admin UI（4+4 页面）+ IDOR + RBAC + 跨业务关联（只读）+ Polling |
| 13 | 是否可以进入正式实施 | **可以**。所有依赖基础设施已确认（Upload/限流/Notification/RBAC/i18n/AutoMigrate），边界清晰（After-Sale/C2C/Withdrawal 彻底分域），MVP 范围明确，5 表设计完成，状态机和 unread 方案已定 |

### 31.2 主要风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| 附件安全（恶意文件/XSS） | 中高 | MIME+extension 双重校验、random object key、私有访问、纯文本渲染、禁止可执行文件/压缩包/SVG |
| 客服误操作（在 Ticket 中执行资金操作） | 中 | 架构隔离：Ticket 模块不注入 Wallet/Withdrawal/C2C service，只能通过 biz_type/biz_id 跳转。资金操作必须在对应业务域执行 |
| unread 计数偏差 | 低 | 原子更新 + 查看清零 + row lock，第一版可接受微小偏差（用户可手动刷新） |
| 并发回复状态覆盖 | 低 | ticket row lock + 状态机 guard + message 独立插入 |
| 用户滥用（spam/刷消息） | 中 | rate limit（创建/回复/上传）+ 内容长度限制 + 附件数量限制 |
| Admin 通知不及时 | 低 | 15s polling，第一版可接受。Full V1 引入 WebSocket |
| 附件存储本地文件系统 | 低 | 第一版可接受，Full V1 迁移到对象存储 + signed URL |

### 31.3 实施建议

- 新建独立模块 `internal/modules/support/`（vertical slice 模式，参考 walletwithdrawal / c2c）
- 分阶段：①Foundation（5 表 model + migration + category seed + 状态机）②User 端（create/list/detail/reply/close/reopen/attachments）③Admin 端（list/detail/reply/status/priority/assign/categories/audits/overview）④Notification + unread + rate limit + IDOR + RBAC ⑤User/Admin 前端 ⑥测试 + 回归 + CI
- 资金安全红线：Ticket 模块**绝对不注入** Wallet Service / Withdrawal Service / C2C Service。只能通过 biz_type/biz_id 做只读关联和跳转

---

*预审计完成。所有依赖基础设施已确认，边界清晰，MVP 范围明确，可以进入正式实施。*
