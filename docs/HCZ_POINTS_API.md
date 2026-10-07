# HCZ Points API 契约（P0–P4 收口版）

> 面向 User 前端与 Admin 前端的对接文档。本文档与代码同时代（P4 收口轮生成），
> 字段名、错误码、状态码均以 `internal/modules/{points,checkin,pointsmall}/transport/http`
> 的实际实现为准。若二者不一致，**以代码为准并回来修订本文档**。

## 0. 通用约定

### 0.1 响应信封

所有端点统一返回：

```json
{ "status_code": 0, "msg": "success", "data": { } }
```

分页端点额外带 `page`：

```json
{ "status_code": 0, "msg": "success", "data": [ ],
  "pagination": { "page": 1, "page_size": 20, "total": 137, "total_page": 7 } }
```

- **状态码约定（前端必读）**：业务错误的 **HTTP 状态码恒为 200**，真实语义在信封 `status_code`
  （400/401/403/404/409/500）里；`msg` 是 i18n 文案（三语 zh-CN / zh-TW / en-US 在
  `internal/i18n/messages.go` 中同键齐备）。这是全仓库统一的信封约定（`response.Error`），
  Points 与钱包/订单/C2C 一致；只有网关与基础设施路径（CORS preflight、渠道 API、回调）会用真实 HTTP 状态。
  因此前端拦截器必须判 `status_code`，**不能只判 HTTP 状态**。本文档下表写"400/404/409"均指 `status_code`。
- 分页参数：`?page=1&page_size=20`，非法值归一化而非报错；`page_size` 上限由 `ginutil.ParsePagination` 统一钳制。

### 0.2 鉴权与身份来源

| 端点族 | 鉴权 | 主体来源 |
| --- | --- | --- |
| `/api/v1/points/*`、`/api/v1/checkin*` | User JWT（`Authorization: Bearer`） | `user_id` **只取自 JWT**，任何请求体/查询参数里的 user_id 都被忽略 |
| `/api/v1/admin/**` | Admin JWT + Casbin RBAC | `admin_id` 只取自 JWT |

### 0.3 幂等键

写操作分两类：

| 场景 | 头 | 长度 | 重复提交语义 |
| --- | --- | --- | --- |
| Admin 积分增减 / 人工补偿 | `Idempotency-Key` 必填 | ≤ 64 | 同键同参数 → HTTP 200 回放原流水（不重复入账）；同键不同参数 → HTTP 409 `error.idempotency_conflict` |
| 用户创建兑换单 | `Idempotency-Key` 必填 | ≤ 64 | 同键同商品 → 返回原订单（`already_processed=true`）；同键不同商品 → 409 |

派生 ledger reference：`points:admin_adjust:{key}` / `points:admin_compensation:{key}`（列宽 120，`points_ledger.reference` 唯一索引是最终防线）。

### 0.4 积分与钱包的边界

- 积分是 BIGINT 整数虚拟资产，**不与 USDT 钱包互通**：没有任何端点把积分换成余额，也没有把余额换成积分。
- `points_ledger` 是 append-only：不存在修改/删除流水的端点（`PUT`/`DELETE` 一律 404/405）。
- 余额为负是合法状态（Admin 扣减、订单退款冲正都会产生），前端必须能显示负数，不得钳 0。

---

## 1. User 端

### 1.1 积分账户

`GET /api/v1/points/account`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| balance | int64 | 当前余额，恒等于该用户 `SUM(points_ledger.amount)`；可为负 |
| total_earned | int64 | 生命周期累计获得（含 Admin 加、订单奖励、签到、人工补偿、兑换返还） |
| total_spent | int64 | 生命周期累计消费口径，**不是净消费**：兑换扣减计入后，即使该兑换被全额返还，`total_spent` 也不回滚（返还会另计进 `total_earned`）。运营/前端文案不得把它读成"净花费" |

从未产生积分的用户返回 `{balance:0,total_earned:0,total_spent:0}`（**不返回 404，不创建账户行**）。

### 1.2 积分流水

`GET /api/v1/points/ledger?page=1&page_size=20`

排序固定 `created_at DESC, id DESC`，**不接受客户端排序字段**。用户端只返回本人流水。

条目字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | uint | 流水 ID |
| user_id | uint | |
| action_type | string | 见 §3.1 枚举 |
| source_type | string | 见 §3.2 枚举 |
| source_id | uint | 业务对象 ID（订单/签到/兑换单/用户） |
| amount | int64 | 有符号，正=入账，负=出账 |
| balance_before / balance_after | int64 | 链式连续：`before + amount == after` |
| reference | string | 幂等键，全局唯一 |
| reason | string | 人类可读原因（≤255） |
| operator_type | string | `user` / `admin` / `system` |
| operator_id | uint | Admin 操作时为 admin_id |
| order_id / checkin_date / exchange_order_id | 可空 | 维度关联；空值不序列化 |
| created_at | RFC3339 | |

### 1.3 签到

| 端点 | 说明 |
| --- | --- |
| `GET /api/v1/checkin/status` | 今日签到状态 |
| `POST /api/v1/checkin` | 每日签到（无请求体；一天一次，重复签到不重复发奖） |
| `GET /api/v1/checkin/history?month=YYYY-MM` | 某月签到日历，`month` 缺省为当前业务月 |

`status` 响应：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| enabled | bool | 后台是否开放签到 |
| checked_in_today | bool | 业务日口径为 **Asia/Shanghai** |
| consecutive_days | int | 连续签到天数（断签归 1） |
| cycle_day | int | 7 天周期内第几天（1–7） |
| today_reward | int64 | 今日应得积分（未开放或未签到时的展示值） |
| next_reward | int64 | 下一档奖励 |

`POST /checkin` 响应（`CheckinResult`）：

```json
{ "checkin_date": "2026-10-08", "points_awarded": 5,
  "consecutive_days": 3, "cycle_day": 3, "current_balance": 320,
  "already_checked_in": false }
```

- 同一天重复调用：HTTP 200，`already_checked_in=true`，`points_awarded=0`，**不产生新流水**。
- 奖励为 0 的日子仍算签到成功，只是不产生 `CHECKIN_REWARD` 流水（也不会凭空创建积分账户）。
- 错误：`error.checkin_disabled`（未开放）、`error.checkin_failed`。

`history` 响应：`year` / `month` / `checked_dates`（`["2026-10-01", ...]`）/ `entries`（逐条含 `consecutive_days`、`points_awarded`）/ `total`。
`month` 格式非法 → 400 `error.checkin_invalid_month`。

### 1.4 积分商城商品

| 端点 | 说明 |
| --- | --- |
| `GET /api/v1/points/products` | 已上架商品列表（`enabled=true`，`sort ASC`，分页） |
| `GET /api/v1/points/products/:id` | 商品详情 + 当前用户可兑换判定 |

商品字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id / name / subtitle / description / cover | | 文案与素材 |
| points_price | int64 | 兑换所需积分（>0；>1_000_000 的定价无法通过后台校验创建） |
| stock | int64 | 可用库存（`unlimited_stock=true` 时无意义） |
| unlimited_stock | bool | 无限库存 |
| enabled | bool | 用户端只会看到 true |
| sort | int | 排序 |
| per_user_limit | int64 | 每人累计可兑换上限；0 = 不限 |
| fulfillment_type | string | **V1 只有 `MANUAL`**（后台人工发放数字权益）；其它值一律 400 |
| instructions | string | 兑换说明（如卡密使用方式） |

`detail` 响应：

```json
{ "product": { ... }, "user_redeemed_count": 2,
  "can_redeem": false, "reason_code": "LIMIT_REACHED" }
```

`reason_code` 取值（判定顺序：余额 → 库存 → 限购）：`""`（可兑换）/ `INSUFFICIENT_POINTS` / `OUT_OF_STOCK` / `LIMIT_REACHED`。
已下架商品在详情接口直接 404 `error.points_product_not_found`，**不会**返回 `can_redeem=false` 的下架码。
`can_redeem` 只是 UI 辅助，**下单时后端一定重新校验**，前端不能拿它当授权依据。

### 1.5 兑换订单

| 端点 | 说明 |
| --- | --- |
| `POST /api/v1/points/exchange-orders` | 创建兑换（`Idempotency-Key` 必填） |
| `GET /api/v1/points/exchange-orders?status=` | 我的兑换单列表 |
| `GET /api/v1/points/exchange-orders/:id` | 我的兑换单详情 |
| `POST /api/v1/points/exchange-orders/:id/cancel` | 取消（仅 `PENDING`） |

创建请求体只接受 `product_id`。**禁止**提交 `user_id` / `points_price` / `total_points` / `product_name` / `quantity`（V1 数量固定 1）。

创建/取消响应：

```json
{ "order": { "id": 88, "order_no": "PX20261008...", "status": "PENDING",
             "product_name_snapshot": "10GB 流量包", "unit_points": 100,
             "quantity": 1, "total_points": 100,
             "fulfillment_type_snapshot": "MANUAL", "created_at": "..." },
  "current_balance": 220, "already_processed": false }
```

- 订单固化 `product_name_snapshot` / `unit_points` / `fulfillment_type_snapshot`：商品改价、下架都不影响历史订单。
- `idempotency_key` / `last_operator_*` 是后端内部字段，**不序列化**（JSON 中被剔除）。
- 状态机：`PENDING → PROCESSING → COMPLETED`，或 `PENDING/PROCESSING → FAILED/CANCELLED`。`COMPLETED/FAILED/CANCELLED` 是终态。
- 错误码（创建）：

| `status_code` | msg |
| --- | --- |
| 400 | `error.idempotency_key_required` / `error.points_idempotency_key_too_long` / `error.points_balance_insufficient` / `error.points_product_disabled` / `error.points_product_out_of_stock` / `error.points_product_limit_reached` |
| 404 | `error.points_product_not_found` |
| 409 | `error.idempotency_conflict`（同键不同商品） |
| 404 | `error.points_exchange_not_found`（详情/取消：不是你的订单同样返回 404，不泄露存在性） |
| 400 | `error.points_exchange_invalid_state`（取消非 `PENDING` 订单） |

---

## 2. Admin 端

路径前缀 `/api/v1/admin`。`user_id` 一律来自路径 `:id`，后台会校验该用户真实存在（不存在 → 404 `error.user_not_found`）。

角色速查（完整断言见 `internal/authz/points_rbac_matrix_test.go`）：

| 能力 | readonly_auditor | operations | finance | system_admin |
| --- | --- | --- | --- | --- |
| 读账户 / 流水 / 统计 / 签到历史 / 商城列表与详情 | ✓ | ✓ | ✓ | ✓ |
| `GET /points/accounts`（负余额排查） | — | — | ✓ | — |
| `POST .../points/adjust`、`POST .../points/compensate` | — | — | ✓ | **—** |
| 商品创建 / 更新 / 上下架 | — | ✓ | — | ✓ |
| 兑换单 process / complete | — | ✓ | ✓ | ✓ |
| 兑换单 fail / cancel（含积分返还） | — | — | ✓ | ✓ |
| 签到配置 `PUT /settings/checkin` | — | — | — | ✓ |

注意两处刻意的不对称：**资金型积分写入只属于 finance**（system_admin 也拿不到），
**积分返还型动作（fail/cancel）不给 operations**。见 §4 末尾说明。

### 2.1 积分账户与流水（只读）

| 端点 | RBAC 角色 | 说明 |
| --- | --- | --- |
| `GET /users/:id/points` | readonly_auditor 起 | 返回 `{ "user": {...}, "account": {balance,total_earned,total_spent} }` |
| `GET /users/:id/points/ledger` | readonly_auditor 起 | 过滤白名单：`action_type`、`source_type`、`reference`、`direction`(income/expense)、`created_from`/`created_to`(RFC3339)、`page`/`page_size`；枚举值非法 → 400 |
| `GET /points/accounts` | finance | `?user_id=`、`?negative_only=true` 排查负余额用户；返回带用户摘要的账户分页（**system_admin 也无此权限**，与 §2.0 速查表一致） |
| `GET /points/stats` | readonly_auditor 起 | 运营看板，见 §2.2 |

### 2.2 运营统计

`GET /api/v1/admin/points/stats`

```json
{ "total_balance": 120340, "today_granted": 2300, "today_spent": 900,
  "today_order_reward": 640, "today_checkin_users": 51, "today_exchanges": 7,
  "pending_exchanges": 3, "processing_exchanges": 1, "negative_balance_users": 2,
  "today_start": "2026-10-08T00:00:00+08:00" }
```

全部指标**直接聚合自事实表** `points_ledger` / `user_checkins` / `points_exchange_orders`，不存在"实时累计总表"这一第二事实源。"今日"= 签到域业务日（Asia/Shanghai），`today_start` 用于运营核对口径。`total_balance` 含负值账户。

### 2.3 积分增减

`POST /api/v1/admin/users/:id/points/adjust`（角色：**仅 finance**；system_admin 未授予该权限，见 §4 职责分离说明）

```json
{ "amount": 500, "operation": "subtract", "reason": "误发奖励追回" }
```

- `amount` **恒为正整数**（>0，≤1_000_000）；扣减用 `operation=subtract` 表达，**禁止负数金额**。
- `operation` 只接受 `add` / `subtract`。
- `reason` 必填，≤255。
- `Idempotency-Key` 必填，≤64。
- 响应：`{ "account": {...}, "ledger": {...} }`。
- Admin 扣减**允许把余额扣成负数**（审计结论：负余额是系统内部事实，用户侧消费才会被拒绝）。
- `ADMIN_ADD` 计入 `total_earned`；`ADMIN_DEDUCT` 只影响 `balance`，不计入 `total_spent`。

| `status_code` | msg | 触发 |
| --- | --- | --- |
| 400 | `error.bad_request` | operation 非法 |
| 400 | `error.points_amount_invalid` | ≤0 或 >1_000_000 |
| 400 | `error.points_adjust_remark_required` | reason 缺失 |
| 400 | `error.points_reason_too_long` | reason 超 255 |
| 400 | `error.idempotency_key_required` / `error.points_idempotency_key_too_long` | 幂等键缺失/超长 |
| 404 | `error.user_not_found` | 目标用户不存在 |
| 409 | `error.idempotency_conflict` | 同键不同参数 |

### 2.4 人工补偿

`POST /api/v1/admin/users/:id/points/compensate`（角色：**仅 finance**）

```json
{ "amount": 300, "reason": "订单 #421 漏发积分补发", "order_id": 421 }
```

- 补偿恒为入账，走独立 `action_type = ADMIN_COMPENSATION`（**绝不伪造 `ORDER_REWARD`**，也不重放订单完成生命周期）。
- `order_id` 可选，仅用于把流水关联到发生异常的订单；**不会改变该订单的任何状态、不恢复任何库存**。
- 幂等规则、金额上限、reason 规则与 §2.3 相同；reference 前缀为 `points:admin_compensation:`。

### 2.5 签到

| 端点 | 角色 | 说明 |
| --- | --- | --- |
| `GET /api/v1/admin/users/:id/checkins?month=YYYY-MM` | readonly_auditor 起 | 用户签到历史，**只读**；额外返回 `latest_consecutive_days` |
| `GET /api/v1/admin/settings/checkin` | system_admin | 当前配置 |
| `PUT /api/v1/admin/settings/checkin` | system_admin | 更新配置 |

配置体：

```json
{ "enabled": true, "rewards": [1, 2, 3, 4, 5, 6, 10] }
```

- `rewards` 必须**恰好 7 项**，每项 `0 <= x <= 1_000_000`；违反 → 400 `error.checkin_config_invalid_rewards`（挡住 `100000000000000` 这类误输入，不会静默截断）。
- 每次变更都落审计：`admin_id` + 变更前后的 `enabled` / `rewards`。
- 不存在（也不提供）Admin 补签、删除签到记录、修改签到日期的端点。签到记录只能由用户本人 `POST /checkin` 产生；签到奖励发错时的处理方式是 §2.4 人工补偿。

### 2.6 积分商城商品

| 端点 | 角色 | 说明 |
| --- | --- | --- |
| `GET /api/v1/admin/points/products` | readonly_auditor 起 | 列表（可含下架） |
| `POST /api/v1/admin/points/products` | operations / system_admin | 创建 |
| `PUT /api/v1/admin/points/products/:id` | operations / system_admin | 更新（含**库存绝对值**、积分价） |
| `PATCH /api/v1/admin/points/products/:id/status` | operations / system_admin | 上下架 |

请求体（创建/更新）：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| name | 是 | ≤120 |
| subtitle / description / cover / instructions | 否 | 文案 |
| points_price | 是 | >0 且 ≤1_000_000 |
| stock | 是 | **绝对值**覆盖，非增量；`0 <= stock <= 1_000_000`；`unlimited_stock=true` 时忽略 |
| unlimited_stock | 否 | 无限库存（不使用 `-1` 之类的 magic number） |
| enabled | 否 | 上下架 |
| sort | 否 | 排序 |
| per_user_limit | 否 | 每人上限，`0 = 不限`，`<= 10_000` |
| fulfillment_type | 否 | 只接受 `MANUAL`（V1 唯一合法值；`AUTO` 会被拒绝） |
| reason | 否（建议填） | 写入审计日志 |

- 库存/价格这类资产字段的每次变更都留审计：`operator_id` + `before_stock`/`after_stock` + `before_points_price`/`after_points_price` + `reason` + 时间戳。
- 下架不删除：历史兑换单仍可继续 `process`/`complete`。
- **没有删除商品的端点**：积分商品承载历史兑换订单，物理删除会破坏审计链。非法值 → 400 `error.points_product_invalid`；不存在 → 404 `error.points_product_not_found`。

### 2.7 兑换订单后台操作

| 端点 | 允许迁移 | 副作用 |
| --- | --- | --- |
| `POST /api/v1/admin/points/exchange-orders/:id/process` | `PENDING → PROCESSING` | 仅状态 |
| `POST /api/v1/admin/points/exchange-orders/:id/complete` | `PROCESSING → COMPLETED` | 仅状态（终态，**无自动撤销**） |
| `POST /api/v1/admin/points/exchange-orders/:id/fail` | `PENDING/PROCESSING → FAILED` | 返还积分 + 恢复库存（同事务，**恰一次**） |
| `POST /api/v1/admin/points/exchange-orders/:id/cancel` | `PENDING/PROCESSING → CANCELLED` | 同上 |
| `GET /api/v1/admin/points/exchange-orders?status=&user_id=` | — | 列表（后台可按用户过滤） |
| `GET /api/v1/admin/points/exchange-orders/:id` | — | 详情 |

- 角色：`process` / `complete` 允许 operations、finance、system_admin；`fail` / `cancel`（会退积分、恢复库存）只允许 finance、system_admin。
- `fail` / `cancel` 的 `reason` **必填**（≤255）：`error.points_exchange_reason_required`。
- 幂等：对同一状态重复提交直接返回当前订单，**不重复返还积分、不重复恢复库存**。
- 终态不可回退：`COMPLETED → CANCELLED` 一律 400 `error.points_exchange_invalid_state`。已 `COMPLETED` 的兑换单出现纠纷，处理方式是 §2.4 人工补偿（带 reason 的 `ADMIN_COMPENSATION`），**这是人工补偿，不是兑换退款**：既不恢复原兑换订单库存，也不修改原兑换单状态。
- 操作者写入订单行 `last_operator_type/last_operator_id`（内部字段，不外泄）+ 审计日志（含 `refunded_points`、`restored_stock`）。

---

## 3. 枚举表（唯一事实源：`internal/modules/points/contract/types.go`）

### 3.1 action_type

| 值 | 符号 | 计入 total_earned | 计入 total_spent | 允许负余额 |
| --- | --- | --- | --- | --- |
| `ORDER_REWARD` | 订单完成奖励 | 是 | 否 | 否 |
| `ORDER_REWARD_REVERSAL` | 订单退款冲正 | 否 | 否 | **是** |
| `CHECKIN_REWARD` | 签到奖励 | 是 | 否 | 否 |
| `REDEEM` | 兑换扣减 | 否 | 是（绝对值） | 否 |
| `REDEEM_REFUND` | 兑换返还 | 是 | 否 | 否（恒为入账） |
| `ADMIN_ADD` | 后台加分 | 是 | 否 | 否（恒为入账） |
| `ADMIN_DEDUCT` | 后台扣减 | 否 | 否 | **是** |
| `ADMIN_COMPENSATION` | 后台人工补偿 | 是 | 否 | 否（恒为入账） |

过滤查询使用同一套枚举（`IsValidActionType`），非法值 400，不做模糊匹配。

### 3.2 source_type

`order_reward` / `order_refund` / `checkin` / `redeem` / `redeem_refund` / `admin_adjust` / `admin_compensation`

### 3.3 reference 派生规则

| 业务事件 | reference | 保证 |
| --- | --- | --- |
| 订单奖励 | `points:order_reward:{order_id}` | 同订单至多一条 |
| 订单退款冲正 | `points:order_refund:{refund_record_id}` | 同退款记录至多一条 |
| 签到 | `points:checkin:{user_id}:{yyyy-mm-dd}` | 同用户同日至多一条 |
| 兑换扣减 | `points:redeem:{exchange_order_id}` | 每单恰一条 |
| 兑换返还 | `points:redeem_refund:{exchange_order_id}` | 每单至多一条（失败/取消共用） |
| 后台调整/补偿 | `points:admin_adjust:{key}` / `points:admin_compensation:{key}` | 幂等回放 |

---

## 4. 已知边界与未提供的能力（V1）

- 没有 `GET /api/v1/points/summary`：`account` + `ledger` + `checkin/status` + `products` 已覆盖前端所需，未再合并出一个聚合大端点。
- 没有积分过期、积分转赠、积分换 USDT、积分交易、任务中心、活动中心、等级/VIP、兑换码池、实物物流。
- 没有 OpenAPI/Swagger 生成物：本仓库的接口契约以本文档 + `docs/api/HCZ_FRONTEND_API_CONTRACT.md` 为准。
- 用户端 `POST /checkin`、`POST /points/exchange-orders`、`POST /points/exchange-orders/:id/cancel` **未挂路由级限流**（与钱包/订单/C2C 的用户写操作一致，由签到单日唯一、行锁、幂等键、每人限兑等语义约束兜底）。属于已知 P3 风险，见 `docs/HCZ_POINTS_P4_FINALIZATION_REPORT.md` §风险。
- 高风险资金动作（钱包调整、提现审批、订单退款）走 Step-Up 二次验证（`X-Auth-Challenge`）；**积分调整/补偿当前不要求 Step-Up**，仅靠 Admin JWT + RBAC + 幂等 + append-only 审计。P4 已评估并保留该差异（积分在 V1 不可变现，且 `adjust` 权限被收窄到 finance 单一角色，连 system_admin 都没有），理由见 P4 报告。
- 职责分离：`points/adjust` 与 `points/compensate` **只授予 finance**，`system_admin` 拿不到——超级管理员不能改用户积分，这是显式决策而非遗漏（`internal/authz/points_rbac_matrix_test.go` 有断言）。若将来积分获得任何变现路径（转赠/交易/抵现），必须同步升级为 Step-Up 强制，并重新审视该矩阵。
