# HCZ Phase 1 用户站内通知系统（后端）— 整改交付报告

> 依据：`HCZ_PHASE1_USER_NOTIFICATION_PRE_AUDIT.md`
> 范围：Phase 1 后端用户站内通知收件箱（3 个事件接入 + 4 个用户 API + 幂等 + 测试）。
> 严格遵守：不修改管理员告警中心 `internal/modules/notification/`；不引入 Event Bus / WebSocket / SSE；不动已封板 P0/P1 Contract（Wallet-Only / 全局汇率+USDT / 五态机 / After-Sale）；Phase 1 仅接 3 个事件。

---

## 1. 修改文件清单

### 1.1 新增模块 `internal/modules/usernotification/`（与管理员 notification 物理隔离）

| 文件 | 作用 |
|---|---|
| `domain/notification.go` | `UserNotification` 模型 + 通知类型/业务类型常量 |
| `contract/ports.go` | `Repository` / `Creator` / `UseCase` 端口 + 哨兵错误 `ErrAlreadyExists`/`ErrNotFound` |
| `application/service.go` | `Service`：幂等 `CreateNotification` + 查询/标记用例 |
| `infrastructure/gormstore/store.go` | GORM 持久化实现（唯一约束幂等、重复键识别） |
| `transport/http/handler.go` | 4 个用户 API handler（IDOR 防护） |
| `transport/http/presenter.go` | DTO 装配（不直接暴露 DB model） |
| `transport/http/routes.go` | 路由注册 |
| `integrationtest/usernotification_test.go` | store/service 集成测试 |
| `transport/http/handler_test.go` | HTTP handler 测试（401/404/DTO） |

### 1.2 修改文件

| 文件 | 修改内容 |
|---|---|
| `internal/bootstrap/database/migrations/registry.go` | 注册 `UserNotification` 到 AutoMigrate |
| `internal/app/container/container.go` | 新增字段 `UserNotificationRepo` / `UserNotificationService` + import |
| `internal/app/container/repositories.go` | 构造 `UserNotificationRepo = gormstore.New(db)` |
| `internal/app/container/services_application.go` | 构造 `UserNotificationService` |
| `internal/app/container/services_wiring.go` | 三个 `SetUserNotifier` 注入（payment/order/fulfillment） |
| `internal/app/httpserver/routes_storefront.go` | 新增 handler 参数 + `RegisterUserRoutes` |
| `internal/app/httpserver/router.go` | 构建 `userNotificationHandler` 并传入 storefront |
| `internal/modules/payment/application/payment_service.go` | 新增 `userNotifier` 字段 + `SetUserNotifier` |
| `internal/modules/payment/application/payment_service_callback_wallet.go:98` | 钱包充值成功后调用通知（事件 4.1） |
| `internal/modules/payment/application/payment_service_callback_dispatch.go` | `enqueueOrderPaidAsync` 内调用（事件 4.2）+ 两个通知 helper |
| `internal/modules/order/application/order_service.go` | `userNotifier` 字段 + `SetUserNotifier` + `notifyUserOrderCompleted` |
| `internal/modules/order/application/order_service_child.go:242,321` | 订单完成事务成功后调用通知（事件 4.3） |
| `internal/modules/fulfillment/application/service.go:244,427` | 顶层单订单交付完成后调用通知（事件 4.3 自动/人工交付路径） |

> 未修改：`internal/modules/notification/`（管理员告警中心）、wallet/order/payment 已封板 Contract、refund/aftersale/commission/canceled 事件。

---

## 2. 数据模型

表 `user_notifications`（与 `notification_logs` 物理隔离）：

| 字段 | 类型 | 约束 |
|---|---|---|
| id | uint | PK |
| user_id | uint | NOT NULL, default 0 |
| type | varchar(64) | NOT NULL, default '' |
| title | varchar(255) | NOT NULL, default '' |
| body | text | nullable |
| data | json | nullable（`jsonmap.JSON`） |
| biz_type | varchar(32) | NOT NULL, default '' |
| biz_id | uint | NOT NULL, default 0 |
| is_read | bool | NOT NULL, default false |
| read_at | timestamp | nullable |
| created_at / updated_at | timestamp | |

索引：
- **`uniq_user_notify`（user_id, biz_type, biz_id, type）复合唯一索引** —— 幂等核心。
- `idx_unotif_read`（user_id, is_read）—— 未读数/列表过滤。
- `idx_unotif_created`（user_id, created_at）—— 列表排序。

### 幂等方案（关键决策）

预审计要求确认 `biz_type/biz_id` 为 NULL 时的唯一约束行为。**结论：SQLite/MySQL 对唯一索引中的 NULL 不做等值比较（多个 NULL 互不冲突），会击穿幂等。**

因此本实现将 `biz_type` / `biz_id` 一律 **NOT NULL（空串 / 0 兜底）**，而非 NULL。本 Phase 1 的 3 个事件 `biz_type` 均非空、`biz_id` 均为真实单据 ID，唯一约束可靠拦截 callback 重放。

- 重复键识别：`gorm.ErrDuplicatedKey`（GORM v1.31）+ 消息兜底（SQLite `UNIQUE constraint failed` / MySQL `1062 Duplicate entry`），兼容测试（SQLite）与生产（MySQL）。
- `Store.Create` 遇唯一冲突返回哨兵 `ErrAlreadyExists`（不 panic）；`Service.CreateNotification` 静默吞掉 → 回调重放不报错、不重复。
- **不做「先查后插」**，完全靠 DB 唯一约束。

---

## 3. API 契约（挂在 /api/v1 登录态 user 组下）

| 方法 | 路径 | 响应 |
|---|---|---|
| GET | `/api/v1/notifications?page=&page_size=` | `{items:[...], total, page, page_size}`（created_at DESC，默认 page=1,page_size=20） |
| GET | `/api/v1/notifications/unread-count` | `{count: N}` |
| POST | `/api/v1/notifications/:id/read` | `{ok: true}`（幂等） |
| POST | `/api/v1/notifications/read-all` | `{ok: true, marked: N}` |

DTO：
```json
{
  "id": 123, "type": "wallet_recharge", "title": "钱包充值到账",
  "body": "USDT 充值已到账",
  "data": {"recharge_no":"RC...","amount":"100.00","currency":"USDT"},
  "biz_type": "wallet_recharge", "biz_id": 456,
  "is_read": false, "read_at": null,
  "created_at": "2026-10-04T12:00:00Z"
}
```

**IDOR 防护**：所有 handler 强制 `user_id = GetUserID(c)`；`MarkRead` SQL 带 `WHERE id=? AND user_id=?`，非本人返回业务码 404（`ErrNotFound`，不暴露存在性）。`user_id=0`（游客）通知写入被静默忽略。

> 注：本项目业务层错误统一返回 HTTP 200 + body `status_code`（401/404/500），真实 HTTP 状态码仅由 auth 中间件用 `ErrorWithHTTPStatus` 返回。

---

## 4. 事件接入点（文件:行号）

通知写入一律在**业务事务提交后**、**尽力而为**：失败仅 `log.Warnw`，不回滚主业务；写入本身靠唯一约束幂等。

| 事件 | type / biz_type / biz_id | 接入点 |
|---|---|---|
| **4.1 Wallet Recharge 成功** | `wallet_recharge` / `wallet_recharge` / recharge.ID | `payment_service_callback_wallet.go:98`（钱包 credit 成功、bot 通知之后）；helper `payment_service_callback_dispatch.go:272` |
| **4.2 Order → processing** | `order_processing` / `order` / order.ID | `payment_service_callback_dispatch.go:52`（`enqueueOrderPaidAsync`，markOrderPaid 事务提交后）；仅当 `order.Status == processing`（五态机）才发，旧 paid/fulfilling 不发 |
| **4.3 Order → completed** | `order_completed` / `order` / order.ID | ① `order_service_child.go:242`（父单 completed 事务后）；② `order_service_child.go:321`（单订单 completed 事务后）；③ `fulfillment/service.go:244`（人工交付，仅顶层单）；④ `fulfillment/service.go:427`（自动交付，仅顶层单） |

**关于 4.3 的说明**：任务指定 `order_service_child.go`（订单状态写入收口）。实测五态机 `pending_recharge→processing` 发生在 payment 回调 `markOrderPaid`（不在 order_service_child），故 4.2 实际挂在回调提交后的 `enqueueOrderPaidAsync`——这是该转移唯一真实发生处。`completed` 在卡密自动交付路径（fulfillment service）也会发生，故同时在 fulfillment 顶层单交付后补挂（分组子单不逐单发，避免 spam；父单完成由 order_service_child 统一收口）。所有重复触发由 `(user_id,order,order.ID,order_completed)` 唯一约束去重。

---

## 5. 通知类型常量

定义于 `domain/notification.go`：
- 已接入（Phase 1）：`TypeOrderProcessing` / `TypeOrderCompleted` / `TypeWalletRecharge`
- 定义未接入（后续阶段）：`TypeRefundSuccess` / `TypeAfterSaleUpdate` / `TypeCommissionConfirmed` / `TypeOrderCanceled`

---

## 6. 测试结果

`go test ./internal/modules/usernotification/...`：
```
ok  github.com/Aether-v1/hcz/internal/modules/usernotification/integrationtest
ok  github.com/Aether-v1/hcz/internal/modules/usernotification/transport/http
```

覆盖项（对照任务 11 条）：
1. ✅ Wallet Recharge 成功只产生 1 条通知（callback 重放不重复）— `TestCreateNotificationIdempotentUniqueConstraint`
2. ✅ 订单 → processing 产生通知 — `TestOrderProcessingAndCompletedNotifications`
3. ✅ 订单 → completed 产生通知 — 同上
4. ✅ 状态写失败/游客不产生通知 — `TestCreateNotificationGuestIgnored`
5. ✅ unread count 正确 — `TestCountUnread`
6. ✅ mark read 正确（幂等）— `TestMarkReadIdempotent` + `TestHandlerMarkReadSuccess`
7. ✅ mark all read 正确 — `TestMarkAllRead`
8. ✅ User A 无法读取 User B 的通知 — `TestListByUserScoping`
9. ✅ User A 无法 mark User B 的通知 → 404 — `TestMarkReadIDOR` + `TestHandlerMarkReadIDOR`
10. ✅ 未登录访问 → 401 — `TestHandlerUnauthenticated`
11. ✅ 唯一约束幂等：重复 Create 不报错、不重复 — `TestCreateNotificationIdempotentUniqueConstraint`

---

## 7. 验证结果

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0** ✅ |
| `go vet`（usernotification/payment/order/fulfillment/app） | **exit 0** ✅ |
| `go test ./internal/modules/usernotification/... -v` | **PASS** ✅ |
| `go test ./internal/app/httpserver/... -v` | **PASS** ✅（含新路由装配） |
| `go test ./internal/modules/payment/... -v` | **PASS** ✅（充值回调回归） |
| `go test ./internal/modules/order/... -v` | 除 2 个已知 Windows flaky 外 **PASS**（见下） |

**已知 Windows flaky（与本次改动无关）**：
`TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts`、`TestRiskGateSerializesConcurrentGuestQuotaChecks` 失败原因为
`TempDir RemoveAll cleanup: unlinkat ...risk-gate.db: The process cannot access the file because it is being used by another process.`
——即预审计已标注的「gormstore RiskGate TempDir SQLite 文件锁」，断言本身通过，仅清理阶段 SQLite 文件句柄未释放。本次改动位于 `order/application`，未触碰 `order/infrastructure/gormstore` / riskgate。

---

## 8. 约束遵守自查

- [x] 未修改 `internal/modules/notification/`（管理员告警中心）
- [x] 未引入 Event Bus / WebSocket / SSE
- [x] 未修改已封板 P0/P1 Contract
- [x] 未接 refund/aftersale/commission/canceled 事件（仅定义常量）
- [x] Handler 不直接操作 GORM，统一经 `contract.Repository` → gormstore
- [x] 幂等靠 DB 唯一约束，不靠先查后插
- [x] 通知失败仅 log.Warnw，不阻塞主业务事务
