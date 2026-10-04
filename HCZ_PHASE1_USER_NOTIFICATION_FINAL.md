# HCZ Phase 1 User Notification System — Final Report

> 实施日期：2026-10-04
> 前置：HCZ_PHASE1_USER_NOTIFICATION_PRE_AUDIT.md（预审计）
> Commit：`dec7575`（已 push origin/main）
> CI Run：#37195307112 — completed / **success**（4/4 jobs 全绿）
> 原则：不修改管理员 notification 模块、不引入 Event Bus/WebSocket/SSE、Phase 1 只接 3 个事件

---

## Final Verdict: PASS

### Phase 1 用户站内通知系统已完整交付，可进入 Phase 2（邀请绑定）。

---

## 一、交付总览

| 层 | 交付物 | 状态 |
|---|---|---|
| 数据模型 | `user_notifications` 表 + AutoMigrate 注册 | ✅ |
| 后端模块 | `internal/modules/usernotification/`（domain/contract/application/gormstore/transport） | ✅ |
| User API | 4 个端点（列表/未读数/标记已读/全部已读） | ✅ |
| 事件接入 | 3 个（充值到账/订单processing/订单completed） | ✅ |
| 幂等 | DB 唯一约束 (user_id, biz_type, biz_id, type) | ✅ |
| 前端 API | `api/notification.ts` | ✅ |
| 前端 Store | `stores/notification.ts`（60s 轮询 + visibility 暂停 + 登录态管理） | ✅ |
| 前端 UI | Navbar bell + 红点（classic+vault）+ 通知中心页 | ✅ |
| i18n | zh-CN / zh-TW / en-US | ✅ |
| 测试 | 后端集成测试 + HTTP handler 测试 + 前端构建验证 | ✅ |

---

## 二、数据模型

### user_notifications 表（独立于 notification_logs）

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| id | BIGINT UNSIGNED | PK, auto_increment | |
| user_id | BIGINT UNSIGNED | NOT NULL, INDEX(user_id, is_read), INDEX(user_id, created_at) | 定向收件人 |
| type | VARCHAR(64) | NOT NULL | 通知类型 |
| title | VARCHAR(255) | NOT NULL | 服务端渲染的标题 |
| body | TEXT | NULL | 正文摘要 |
| data | JSON | NULL | 业务上下文（order_no/amount/recharge_no 等） |
| biz_type | VARCHAR(32) | NOT NULL, 复合唯一索引 | 业务类型 |
| biz_id | BIGINT UNSIGNED | NOT NULL, 复合唯一索引 | 业务主键 |
| is_read | TINYINT(1) | NOT NULL, default 0 | 未读=0 |
| read_at | DATETIME | NULL | 已读时间 |
| created_at / updated_at | DATETIME | INDEX(created_at) | |

**幂等唯一约束**：`UNIQUE(user_id, biz_type, biz_id, type)`

> 关键决策：`biz_type`/`biz_id` 设为 NOT NULL（空串/0 兜底），因为 SQLite/MySQL 对唯一索引中的 NULL 不做等值比较，会击穿 callback 重放去重。

**与管理员 notification_logs 物理隔离**：两表独立，admin notification 模块零修改。

### 通知类型常量

```
order_processing      订单处理中
order_completed       订单已完成
wallet_recharge       钱包充值到账
refund_success        退款到账（定义，Phase 1 未接入）
aftersale_update      售后状态更新（定义，未接入）
commission_confirmed  返利到账（定义，未接入）
order_canceled        订单已取消（定义，未接入）
```

---

## 三、3 个核心事件接入

| 事件 | 触发点（文件:行号） | 通知 type | 幂等键 |
|---|---|---|---|
| Wallet Recharge 成功 | `payment_service_callback_wallet.go:98`（钱包 credit 成功后） | wallet_recharge | (user_id, "wallet_recharge", recharge_id, "wallet_recharge") |
| Order → processing | `payment_service_callback_dispatch.go:52`（五态机 pending_recharge→processing 真实发生处） | order_processing | (user_id, "order", order_id, "order_processing") |
| Order → completed | `order_service_child.go:242,321`（父单/单订单完成收口）+ `fulfillment/service.go:244,427`（顶层单卡密自动交付后） | order_completed | (user_id, "order", order_id, "order_completed") |

**接入原则**：
- 均在业务事务提交后调用，通知创建失败只 `log.Warnw`，不回滚主业务
- 重复触发（如 completed 多路径）由 DB 唯一约束去重，不产生重复通知
- 分组子单不逐单发 completed（避免 spam），由父单完成收口统一通知

---

## 四、User API 契约

| 方法 | 路径 | 功能 | 响应 |
|---|---|---|---|
| GET | `/api/v1/notifications?page=1&page_size=20` | 分页列表（created_at DESC） | `{items: [...], total, page, page_size}` |
| GET | `/api/v1/notifications/unread-count` | 未读数 | `{count: N}` |
| POST | `/api/v1/notifications/:id/read` | 标记单条已读（幂等） | `{ok: true}` |
| POST | `/api/v1/notifications/read-all` | 全部已读 | `{ok: true, marked: N}` |

**DTO**（不直接暴露 DB model）：
```json
{
  "id": 123,
  "type": "wallet_recharge",
  "title": "钱包充值到账",
  "body": "USDT 充值已到账",
  "data": {"recharge_no": "RC...", "amount": "100.00", "currency": "USDT"},
  "biz_type": "wallet_recharge",
  "biz_id": 456,
  "is_read": false,
  "read_at": null,
  "created_at": "2026-10-04T12:00:00Z"
}
```

**IDOR 安全**：
- 所有 handler 强制 `user_id = 当前登录用户`
- MarkRead 带 `WHERE id=? AND user_id=?`，越权返回 404（不暴露存在性）
- 列表/未读数均按 user_id 过滤

---

## 五、前端实现

### 架构
- **共享 Store**：`stores/notification.ts` — 唯一的 notification business logic 来源
- **classic 模板**：`components/Navbar.vue` 加 bell + 红点
- **vault 模板**：`templates/vault/layout/VaultLayout.vue` 加 bell + 红点（vault 不复用 Navbar，自带顶栏）
- 两套模板只写展示代码，数据全部来自同一个 store，**逻辑零复制**

### 轮询机制
- 登录后启动，每 **60 秒**请求 `unread-count`
- `document.visibilityState === 'hidden'` 时暂停，切回时立即请求一次 + 恢复轮询
- logout 后停止轮询并清空状态
- 未登录不启动

### 红点
- 复用现有 `components/ui/badge/Badge.vue`
- 未读数 >99 显示 `99+`，为 0 时隐藏
- bell 仅登录态显示

### 通知中心页
- 路由 `/notifications`（requiresUserAuth）
- 按 type 显示不同图标/颜色（wallet_recharge 绿色、order_processing 蓝色、order_completed 绿色对勾）
- 未读加粗高亮，已读灰色
- 点击单条 → markRead + 本地乐观更新
- 顶部"全部已读"按钮
- 加载更多分页 + 空状态

---

## 六、幂等验证

| 场景 | 验证方式 | 结果 |
|---|---|---|
| Wallet Recharge callback 重放 | 重复触发同一 recharge_id 的通知创建 | ✅ 唯一约束拦截，仅 1 条 |
| Order completed 多路径触发 | fulfillment 交付 + order_service_child 收口同时触发 | ✅ 唯一约束去重，仅 1 条 |
| MarkRead 重复调用 | 同一条通知多次 POST /read | ✅ 幂等，不报错 |
| 重复 CreateNotification | service 层重复调用 | ✅ ErrAlreadyExists 静默返回 |

---

## 七、测试与回归

### 后端测试
- `usernotification` 模块集成测试（sqlite in-memory）：Create 幂等、List、CountUnread、MarkRead、MarkAllRead、唯一约束
- HTTP handler 测试：未登录 401、IDOR 404、已读 200、unread-count 形状、DTO 字段
- payment 模块回归：充值回调不破坏
- order 模块回归：状态机不破坏（仅 RiskGate Windows flaky）

### 前端验证
- `npx vue-tsc --noEmit` → 0 错误
- `npm run build` → exit 0（Notifications chunk 5.36 kB）

### 全量回归
| 检查 | 结果 |
|---|---|
| gofmt | ✅ 空输出 |
| go vet ./... | ✅ exit 0 |
| go test ./... | ✅ 190+ PASS，仅已知 Windows flaky（logger/RiskGate/selfupdate）+ reseller 并行隔离偶发（隔离重跑 PASS） |
| go build ./... | ✅ exit 0 |
| User vue-tsc | ✅ 0 错误 |
| User build | ✅ exit 0 |
| Migration（含新表 AutoMigrate） | ✅ PASS |
| 核心模块（usernotification/httpserver/payment） | ✅ PASS |

### 架构守卫修复（本轮引入，已修复）
基础设施阶段发现 3 个 architecture 测试失败（由 Phase 1 代码引入），已最小化修复：
1. `handler_test.go` 直接 import gormstore → 按 aftersale_handler_test.go 先例登记集成测试白名单
2. dispatch 文件新增 `notifyUser*` 函数 → 更新结构测试预期清单（不新建文件，避免文件预算守卫）
3. `payment_service.go` 新增 `SetUserNotifier` → 补入预期清单

修复后 architecture 测试全绿，CI 验证通过。

---

## 八、Linux CI 真实验证

- **Push**：`11bdc6c..dec7575 main -> main`（非交互，exit 0）
- **CI Run #37195307112**（head_sha=dec7575）：completed / **success**

| Job | 结果 | 耗时 |
|---|---|---|
| Verify installer | ✅ success | ~19s |
| Verify API（gofmt+vet+test+build） | ✅ success | ~3m22s |
| Verify release config | ✅ success | ~17s |
| Verify fullstack build | ✅ success | ~2m3s |

---

## 九、明确回答

| 问题 | 回答 |
|---|---|
| user_notifications 是否独立于 admin notification？ | **是**。新建独立表+模块，admin notification_logs 零修改 |
| 3 个核心事件是否已接入？ | **是**。充值到账、订单→processing、订单→completed，均在事务成功后 |
| 重试是否不会产生重复通知？ | **是**。DB 唯一约束 (user_id, biz_type, biz_id, type)，callback 重放/多路径触发均去重 |
| IDOR 是否安全？ | **是**。所有操作强制 user_id=当前用户，越权返回 404 |
| bell/red-dot/通知中心是否可用？ | **是**。classic+vault 双模板 bell+红点，通知中心页列表/标记已读/全部已读/分页 |
| 是否可以进入 Phase 2 邀请绑定？ | **是**。Phase 1 完整闭环，CI 全绿，无遗留阻断 |

---

## 十、Phase 2 预留（不在本轮范围）

- refund_success / aftersale_update / commission_confirmed / order_canceled 事件接入
- Admin 手动推送 / 站内广播
- SSE / WebSocket 实时推送（当前 60s 轮询足够）
- 通知偏好设置（用户关闭某类通知）
- Telegram Bot 与站内信桥接
- 通知清理/归档策略

---

## 十一、产物索引

| 报告 | 路径 |
|---|---|
| 本报告（最终交付） | `HCZ_PHASE1_USER_NOTIFICATION_FINAL.md` |
| 预审计 | `HCZ_PHASE1_USER_NOTIFICATION_PRE_AUDIT.md` |
| 后端实现详情 | `REMEDIATION_PHASE1_NOTIFICATION_BACKEND.md` |
| 前端实现详情 | `REMEDIATION_PHASE1_NOTIFICATION_FRONTEND.md` |
| 基础设施回归详情 | `REMEDIATION_PHASE1_INFRASTRUCTURE.md` |
