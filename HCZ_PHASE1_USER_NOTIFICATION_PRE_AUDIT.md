# HCZ Phase 1 User Notification System Pre-Audit

> 审计范围：仅审计、不修改代码。所有结论均附 `文件:行号` 证据，证据来自本轮实际 Grep/Read。
> 项目根：`E:\Users\orang\Downloads\Compressed\hcz_v1`，Go module：`github.com/Aether-v1/hcz`

---

## 一、现有资产盘点

### 1.1 notification 模块（管理员告警中心）

**结论：这是「管理员告警中心」，不是用户站内消息中心，不能直接复用为用户收件箱。**

- 表名 `notification_logs`，定义在 `internal/modules/notification/domain/log.go:11-31`：
  - 字段：`ID / EventType / BizType / BizID / Channel / Recipient / Locale / Title / Body / Status / ErrorMessage / IsTest / VariablesJSON(json) / CreatedAt`
  - **无 `user_id` 字段**（`Recipient` 是 string，存的是管理员邮箱 / Telegram chat_id / Feishu receive_id，见 `send.go:153/188/223`）
  - **无 `is_read` / `read_at` / 已读未读字段**
  - 有 `EventType`（varchar(100)，带索引）与 `VariablesJSON`（json，模板变量快照）
- AutoMigrate 注册：`internal/bootstrap/database/migrations/registry.go:58`（`&notificationdomain.NotificationLog{}`）
- 事件类型常量：`internal/constants/constants.go:373-378`
  - `wallet_recharge_success`、`order_paid_success`、`manual_fulfillment_pending`、`exception_alert`、`exception_alert_check`
- 渠道常量：`constants.go:382-385` —— `email` / `telegram` / `feishu`
- **收件人是管理员**：分发逻辑 `application/send.go:152-262` 遍历的是 `setting.Channels.Email.Recipients / Telegram.Recipients / Feishu.Recipients`（后台配置的管理员名单），**不是用户**。
- BizType 常量：`constants.go:563-568`（order / wallet_recharge / dashboard_alert / payment_callback / procurement / reconciliation）
- 后台能力：路由 `internal/modules/notification/transport/http/routes.go:9-16`，全部挂在 `/api/v1/admin/settings/notification-center*`（设置读取/更新、日志列表、测试发送）。前端页面 `frontend/admin/src/views/admin/Notifications.vue`（模板本地化编辑 + 渠道配置 + 日志列表 + 测试发送）。

**可复用部分**：
- 模板渲染能力 `application/format/template.go`（`RenderTemplate` 变量插值）可借鉴，但它是给管理员告警用的多语言模板，与用户站内消息不是一回事。
- `VariablesJSON jsonmap.JSON` 的 payload 结构思路可复用。
- 异步分发管道（asynq）可复用——用户通知同样走队列落库，不阻塞主事务。

**不可复用部分**：表结构（无 user_id、无 read 状态）、收件人模型（配置式管理员名单 vs 定向 user_id）、路由前缀（admin-only）。

### 1.2 订单邮件通知（SMTP 发给用户）

这是目前**真正面向用户**的通道（注册用户发 `user.Email`，游客发 `order.GuestEmail`）：

- 触发入队：`internal/modules/order/application/order_status_email_queue.go:19` `EnqueueStatusEmailTaskIfEligible`
- 消费 worker：`internal/app/jobs/consumer/consumer_order.go:32-205` `handleOrderStatusEmail`
  - 收件人解析：`consumer_order.go:95-108`（注册用户取 `user.Email`+`user.Locale`，游客取 `GuestEmail`+`GuestLocale`）
  - 跳过规则：`consumer_order.go:49-52` **canceled 状态不发邮件**；`consumer_order.go:113-116` Telegram 占位邮箱跳过
- 邮件模板入参结构 `OrderStatusEmailInput`：`internal/modules/notification/contract/ports.go:27-42`
  - 字段：`OrderNo / Status / Amount / RefundAmount / RefundReason / Currency / SiteName / SiteURL / FulfillmentInfo / Instructions / IsGuest / AttachmentName / AttachmentContent / MailBrand`
  - 这就是可复用为站内消息模板的变量结构。
- 现有业务事件 payload 变量（可直接复用）：`internal/modules/payment/application/payment_service_notification_payload.go:22-98`
  - 订单：`order_id / order_no / user_id / guest_email / amount / currency / order_status / customer_email / customer_label / items_summary / payment_channel ...`
  - 充值：`user_id / recharge_id / recharge_no / amount / currency / provider_type / channel_type ...`

### 1.3 前端现有通知相关 UI

**用户端 `frontend/user`：基本为空，无通知中心。**

- `api/` 目录文件清单（`frontend/user/src/api/`）：`affiliate / auth / client / credential / index / order / product / reseller / types / user / wallet` —— **无 `notification.ts`**
- `stores/` 目录：`app / buyNow / cart / telegramMiniApp / userAuth / userProfile` —— **无 notification store**
- `router/index.ts`：路由清单见 `:116-325`，**无 `/notifications`、`/messages`、`/inbox` 路由**。唯一相关的是 `/notice`（`router/index.ts:271-273`），但它是**内容营销公告（content Post）**，不是用户站内消息。
- `Navbar.vue`：仅购物车角标动画（`components/Navbar.vue:276`），**无 bell / 通知入口 / 红点**。
- 仅有的 UI 原语：`components/ui/badge/Badge.vue`（通用角标组件，可直接复用做红点）。
- Admin 端：`frontend/admin/src/views/admin/Notifications.vue` 是管理员告警中心配置页，与用户侧无关。

### 1.4 实时通信基础设施

**结论：无 WebSocket、无 SSE、无通用轮询框架。只有两处业务级一次性轮询。**

- `go.mod:47` 仅有 `github.com/gin-contrib/sse v1.1.0 // indirect`（间接依赖，未使用）。**无 gorilla/websocket、无 melody、无 nhooyr/coder websocket**。
- 前端全局搜索 `EventSource | new WebSocket | WebSocket`：**0 匹配**。
- 前端 `setInterval` 用途（全部是业务倒计时/支付轮询，非通知）：
  - `composables/usePayment.ts:705` —— 支付结果轮询
  - `composables/useRechargeOrderDetail.ts:205` —— 充值单详情轮询
  - 其余为验证码倒计时 / banner 轮播 / 登录 challenge tick

**这是唯一可借鉴的现成模式**：`usePayment.ts` / `useRechargeOrderDetail.ts` 已经演示了「登录态 + 定时 GET + 状态到达即停」的轻量轮询写法，可直接套用到未读数轮询。

---

## 二、事件触发点审计

> 当前系统**没有统一 event bus**（搜索 `eventbus/pubsub/publisher/subscriber/dispatch/emit/domain event` 全仓库 0 命中）。模块间联动方式 = **直接函数调用 + asynq 异步队列**。
> 下表「是否有 hook 出口」指：该位置当前是否已有「可插入用户通知写入」的明确调用点。

| 业务事件 | 触发点（文件:行号） | 当前对用户的触达 | 有无 hook 出口 |
|---|---|---|---|
| 订单 paid（已支付） | `payment/application/payment_service_callback_dispatch.go:22` `enqueueOrderPaidAsync` | ① 状态邮件(`:37`) ② 管理员告警(`:128-145`) ③ Telegram Bot(`:166-196`) | **有**（同一函数内，已注入 `notificationSvc`） |
| 订单 completed（已完成） | `order/application/order_service_child.go:232` | 仅状态邮件 | **有**（`EnqueueStatusEmailTaskIfEligible` 旁） |
| 订单 delivered / 状态机通用跃迁 | `order/application/order_service_child.go:297`(父单同步) / `:308`(单订单) | 状态邮件 | **有**（两处统一收口点） |
| 订单 canceled / failed | `order/application/order_service_child.go:257-274` | **无邮件**（worker 主动跳过 canceled，`consumer_order.go:49-52`）；仅 affiliate 逆向 | **有**（`:267` 旁，当前无任何用户触达） |
| 订单 pending_recharge / processing / failed（五态机） | `order/application/order_service_child.go`（`UpdateStatus` 统一收口 `:262`） | 状态邮件（按状态） | **有**（统一收口 `:307-316`） |
| 钱包充值到账 success | `payment/application/payment_service_callback_wallet.go:96` `enqueueWalletRechargeSuccessAsync` | ① 管理员告警(`dispatch.go:147-164`) ② Telegram Bot(`:198-230`) | **有**（同函数内） |
| 退款成功（后台手动退款） | `order/transport/http/admin_refund_handler.go:340-372` `enqueueOrderRefundStatusEmail` | 仅状态邮件 | **有**（`:365` 旁） |
| 售后状态变化（resolve/reject） | `order/application/aftersale/service.go:88-106` `resolveAction` | **完全无** | **有**（事务提交后即可写；当前 0 触达） |
| 售后退款成功（partial/full） | `order/application/aftersale/service.go:121-155` `doRefund` | **完全无**（钱包已入账，但用户不知） | **有**（`doRefund` 事务后） |
| 返利到账（commission confirmed） | `affiliate/application/commission.go:107-113` `ConfirmDueCommissions` → `affiliate/infrastructure/gormstore/store.go:347-349` | **完全无** | **有**（定时任务批处理出口，需逐条收集 affected user） |

补充：
- Telegram Bot 已是**面向用户**的第二通道（`payment_service_callback_dispatch.go:184` `EnqueueBotNotification`，事件 `BotNotificationOrderPaid` / `BotNotificationWalletRechargeSucceeded`），Phase 1 用户通知落库后可考虑后续桥接到 Bot。
- `notificationSvc.Enqueue` 当前只发管理员告警；用户通知需要**新增一条并行的写入路径**（写 `user_notifications` 表），不要复用 `Enqueue`（它的收件人是配置式管理员）。

---

## 三、推荐数据模型

### 3.1 `user_notifications` 表字段设计（新建表，不动 `notification_logs`）

| 字段 | 类型 | 约束 / 索引 | 说明 |
|---|---|---|---|
| `id` | BIGINT UNSIGNED | PK, auto_increment | |
| `user_id` | BIGINT UNSIGNED | **NOT NULL, INDEX(`user_id`,`is_read`), INDEX(`user_id`,`created_at`)** | 定向收件人；游客（UserID=0）不发站内信 |
| `type` | VARCHAR(64) | NOT NULL, INDEX | 见 3.2 枚举 |
| `title` | VARCHAR(255) | NOT NULL | 前端直接展示的标题（服务端按 locale 渲染好，不存模板） |
| `body` | TEXT | NULL | 正文摘要/纯文本；复杂内容靠 `data` + 前端渲染 |
| `data` | JSON | NULL | 业务上下文（order_no / amount / recharge_no / after_sale_id 等），见 3.3 |
| `biz_type` | VARCHAR(32) | NULL, INDEX | order / wallet_recharge / refund / after_sale / commission |
| `biz_id` | BIGINT UNSIGNED | NULL, INDEX | 业务主键（订单/充值单/售后单/佣金单 ID） |
| `is_read` | TINYINT(1) | NOT NULL, default 0 | 未读=0 |
| `read_at` | DATETIME | NULL | 标记已读时间 |
| `created_at` | DATETIME | INDEX | |
| `updated_at` | DATETIME | | |

索引设计理由：红点计数与列表都按 `(user_id, is_read)` 和 `(user_id, created_at DESC)` 走；`biz_type+biz_id` 用于「同业务去重/点跳转」。
**不设 `deleted_at` 软删除**（消息量增长快，MVP 用「清空已读」或定期清理即可，避免红点计数需额外过滤软删）。

### 3.2 `notification_type` 枚举（建议新增常量组，对齐 `constants.go` 风格）

```
order_paid            订单已支付
order_delivered       订单已交付/完成
order_canceled        订单已取消/失败
wallet_recharge       钱包充值到账
refund_success        退款到账（含售后退款）
aftersale_update      售后状态更新
commission_confirmed  返利到账（可提现）
```

### 3.3 `data` payload 结构（JSON，前端据此跳路由）

```jsonc
{
  "order_no": "R20261004...",        // order* 类型必填
  "recharge_no": "RC...",            // wallet_recharge 类型必填
  "amount": "100.00", "currency": "USDT",
  "refund_amount": "20.00",          // refund 类型
  "after_sale_status": "resolved",   // aftersale_update 类型: pending/resolved/rejected
  "commission_amount": "5.00",       // commission_confirmed 类型
  "deep_link": "/me/orders"          // 前端点击跳转，可由前端按 type 推导，不必服务端存
}
```

---

## 四、KEEP / MODIFY / BUILD 分类

| 组件 | 判断 | 说明 |
|---|---|---|
| `notification_logs` 表（管理员告警日志） | **KEEP** | 职责是管理员告警发送留痕，保持原样，与用户消息中心物理隔离 |
| 管理员告警中心（email/telegram/feishu → 管理员） | **KEEP** | 继续作为运维告警，不动 |
| 订单状态邮件（SMTP → 用户） | **KEEP** | 现有通道保留，不被站内信替代 |
| Telegram Bot 通知用户 | **KEEP** | 已存在，Phase 1 不动 |
| asynq 异步队列基础设施 | **KEEP** | 用户通知写入走同一套队列，复用 `internal/queue` |
| `payment_service_notification_payload.go` payload 变量组装 | **MODIFY/复用** | 其字段结构可直接作为 `user_notifications.data` 的来源 |
| `EnqueueStatusEmailTaskIfEligible` 统一收口点 | **MODIFY** | 在这些收口点旁新增「写用户通知」调用，不新建事件总线 |
| `NotificationEnqueuer` 接口 | **BUILD（新接口）** | 新增 `UserNotificationWriter` 端口，与管理员 `Enqueue` 分离 |
| `user_notifications` 表 + domain/repo | **BUILD** | 全新模块（建议放 `internal/modules/usernotification/` 或挂在 notification 下新增 user 子域） |
| 用户端 API（列表/未读数/标记已读） | **BUILD** | 全新 |
| 前端 bell + 红点 + 通知中心页 | **BUILD** | 全新（可复用 `ui/badge/Badge.vue`） |
| WebSocket / SSE 网关 | **BUILD（延后）** | Phase 1 不做 |

---

## 五、第一版业务事件接入优先级

| 优先级 | 事件 | 触发点（文件:行号） | 说明 |
|---|---|---|---|
| P0 | 钱包充值到账 | `payment/application/payment_service_callback_wallet.go:96` | 资金类，用户最敏感；已有 `recharge.UserID` 直接可用 |
| P0 | 订单已支付 paid | `payment/application/payment_service_callback_dispatch.go:22` | 已有 `order.UserID`，且该函数已注入通知依赖 |
| P0 | 订单已交付/完成 completed | `order/application/order_service_child.go:232` | 用户最关心的「买到了」 |
| P1 | 退款到账（后台手动退款） | `order/transport/http/admin_refund_handler.go:365` | 资金类 |
| P1 | 售后退款到账（partial/full） | `order/application/aftersale/service.go:154`（doRefund 事务后） | 资金类，当前 0 触达 |
| P1 | 售后状态更新（resolve/reject） | `order/application/aftersale/service.go:106`（resolveAction 事务后） | 当前 0 触达 |
| P2 | 返利到账 confirmed | `affiliate/application/commission.go:111`（ConfirmDueCommissions） | 批处理，需在 `MarkPendingCommissionsAvailable` 后收集 affected 行 |
| P2 | 订单 canceled / failed | `order/application/order_service_child.go:267` | 低频，邮件已刻意不发，站内信补位 |
| P3 | pending_recharge / processing 中间态 | `order/application/order_service_child.go:308` | 中间态抖动大，建议 Phase 1 不做，避免刷屏 |

---

## 六、User API 设计

> 统一挂在现有 `/api/v1` 登录态 `user` 组下（参考 `order/transport/http/routes.go:39`、`wallet/transport/http/routes.go:10` 的注册方式）。

| 方法 | 路径 | 功能 |
|---|---|---|
| GET | `/api/v1/user/notifications` | 分页拉通知列表（`?page=&page_size=&type=&is_read=`），默认按 created_at DESC |
| GET | `/api/v1/user/notifications/unread-count` | 红点计数（返回 `{count: n}`，>99 前端显示 99+） |
| POST | `/api/v1/user/notifications/:id/read` | 标记单条已读（幂等） |
| POST | `/api/v1/user/notifications/read-all` | 一键全部已读 |
| GET | `/api/v1/user/notifications/:id` | （可选）单条详情；MVP 可用列表字段直出，省略 |

权限：全部要求登录中间件；handler 内强制 `WHERE user_id = 当前登录用户`，杜绝越权读他人消息。

---

## 七、Admin 手动推送

**建议 Phase 1 不做**，理由：
- 现有 `notification_logs` 是「发送尝试日志」，没有用户收件箱语义；管理员手动群发需要新增「全量/按标签推送给 user_notifications」的运营能力，属于运营功能，不是用户感知的必需。
- Phase 1 目标是「业务事件自动触达」，手动群发可作为 Phase 2。
- 若一定要做，推荐方案：复用现有 admin 告警后台的模板编辑器（`Notifications.vue`），新增 `POST /admin/user-notifications/broadcast`（选 type + 标题 + 正文 + 受众：全部/按 user_id），异步遍历写入 `user_notifications`。**本轮仅记录，不实现。**

---

## 八、unread/read 设计

- 字段：`is_read TINYINT(1) default 0` + `read_at DATETIME NULL`（见 3.1）。
- 标记已读：`UPDATE user_notifications SET is_read=1, read_at=NOW() WHERE id=? AND user_id=?`（带 user_id 条件，防越权）。
- 红点计数：`SELECT COUNT(*) FROM user_notifications WHERE user_id=? AND is_read=0`，命中 `(user_id,is_read)` 复合索引。
- 一键已读：`UPDATE ... SET is_read=1, read_at=NOW() WHERE user_id=? AND is_read=0`。
- 不做「会话级/已读时间戳」复杂模型，MVP 二元状态足够。

---

## 九、红点实现方式

- 组件：复用现有 `components/ui/badge/Badge.vue`（用户端已有该 UI 原语），在 `Navbar.vue` 右上角新增 bell 图标 + Badge 未读数。
- 未读数 API：`GET /api/v1/user/notifications/unread-count`（第六章）。
- 轮询频率：登录态页面每 **60s** 轮询一次 unread-count（仅计数，轻量）；进入通知中心页时再拉列表。参考现有 `useRechargeOrderDetail.ts:205` 的 `setInterval` + 组件卸载 `clearInterval` 写法。
- 进入通知中心页 / 点击 bell → 列表加载后可静默调用「标记已读」或逐条点击时标记。
- 未读数为 0 时隐藏红点（不显示 0）。

---

## 十、第一版通信方式：Polling vs SSE vs WebSocket

**推荐：HTTP 短轮询（Polling），Phase 1 不引入 SSE/WebSocket。**

理由（基于现有基础设施的实证）：
1. 后端**无任何实时网关**：go.mod 仅 `gin-contrib/sse // indirect`（`go.mod:47`），无 gorilla/melody；前端**无 EventSource/WebSocket 引用**。引入 WebSocket 需要新增连接管理、鉴权、心跳、多实例广播（Redis pub/sub），工程量直接跳到 XL，与 Phase 1「先让用户感知到」的目标不匹配。
2. 已有轮询先例：`usePayment.ts:705`、`useRechargeOrderDetail.ts:205` 已落地「登录态定时 GET」模式，团队熟悉，风险低。
3. 用户通知不是高频即时场景（订单到账/退款/返利，秒级延迟可接受），60s 轮询未读数 + 进页拉列表完全够用。
4. SSE 比 WebSocket 轻，但仍需长连接 + 反向代理（Nginx）缓冲/超时配置 + 鉴权改造，对当前架构是额外负担，**留作 Phase 2 演进**（当消息量大、要做实时推送时再上）。

---

## 十一、最小实现范围（MVP）

**第一版必须做（Must）：**
1. 新建 `user_notifications` 表 + AutoMigrate 注册（`registry.go`）。
2. 新建用户通知 domain / repo / application（`CreateForUser`、`ListByUser`、`CountUnread`、`MarkRead`、`MarkAllRead`）。
3. 接 P0 三个事件写入：充值到账、订单已支付、订单已完成（第五章）。
4. 4 个 User API（列表 / 未读数 / 单条已读 / 全部已读）。
5. 前端：`api/notification.ts` + `stores/notification.ts` + Navbar bell 红点（60s 轮询）+ `/notifications` 通知中心页 + 路由。
6. 写入失败不阻塞主业务（仿现有 `log.Warnw` 降级，通知失败只记日志）。

**第一版可以延后（Later）：**
- 手动群发 / Admin 推送（第七章）。
- SSE / WebSocket 实时推送。
- 售后/退款/返利事件接入（P1/P2，随 MVP 验证后第二批接）。
- canceled/failed 中间态通知。
- 通知偏好设置（用户关闭某类通知）、通知清理策略、多语言模板后台化。
- Telegram Bot 与站内信桥接。
- 未读数 >99 之外的富交互（下拉预览、 toast 实时弹出）。

---

## 十二、工作量估算

| 模块 | 范围 | 规模 |
|---|---|---|
| 后端 - 数据模型+repo | `user_notifications` 表、AutoMigrate、CRUD repo | S |
| 后端 - application+User API | 列表/未读数/标记已读/全部已读 + 登录态鉴权 | M |
| 后端 - 事件接入 | P0 三事件写入（充值/paid/completed），含降级日志 | M |
| 后端 - 测试 | repo 单测 + handler 接口测试（越权 user_id 校验） | M |
| 前端 - API+store | `api/notification.ts` + `stores/notification.ts`（轮询） | S |
| 前端 - Navbar 红点 | bell + Badge + 60s 轮询未读数 | S |
| 前端 - 通知中心页 | 列表页 + 分页 + 标记已读交互 + 路由 | M |
| 前端 - i18n | zh-CN/zh-TW/en-US 文案 | S |
| **合计（端到端 MVP）** | | **M（约 1 个迭代）** |

> 说明：因无需引入 WebSocket/SSE、无需事件总线、复用现成 asynq 与轮询模式，整体控制在 M；若把 P1/P2 事件（售后/退款/返利）一次性接入，则后端事件接入升级为 M~L。
