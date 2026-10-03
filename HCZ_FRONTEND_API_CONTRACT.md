# HCZ Frontend API Contract（P0-2 USDT 结算）

> 供后续新 User 前端直接对接。所有金额为 `money.Amount` 结构（含 `.decimal`/展示用字符串，
> 由后端序列化），前端**不得**自行按 site_config.currency 猜测币种、不得自行做汇率换算、
> 不得调用 CoinGecko、不得用当前汇率重算历史订单。

## 币种规则（固定）
- **Site Currency**：`site_config.currency`（如 CNY）。商品定价、订单 `total_amount` 用它。
- **Wallet Currency**：固定 **USDT**。钱包余额、扣款、退款、返利、流水、充值到账全部 USDT。
- **汇率方向**：`1 USDT = R SiteCurrency`。订单创建时快照，历史订单永不变。

---

## 1. Wallet 钱包

### GET 用户钱包账户
字段：
| 字段 | 类型 | 说明 |
|---|---|---|
| balance | money.Amount | 余额（USDT） |
| currency | string | 固定 `"USDT"` |

```json
{ "balance": "123.45", "currency": "USDT" }
```

### GET 钱包流水（ledger）
| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | |
| type | string | recharge/order_pay/order_refund/commission... |
| direction | string | in/out |
| amount | money.Amount | USDT |
| balance_before | money.Amount | USDT |
| balance_after | money.Amount | USDT |
| currency | string | 固定 `"USDT"` |
| remark | string | |
| created_at | RFC3339 | |

```json
{ "type":"order_pay","direction":"out","amount":"13.93",
  "balance_before":"100.00","balance_after":"86.07","currency":"USDT" }
```

---

## 2. Order 订单

### 订单详情 OrderDetail
| 字段 | 类型 | nullable | 说明 |
|---|---|---|---|
| order_no | string | | |
| status | string | | |
| total_amount | money.Amount | | **Site Currency** 商品应付 |
| currency | string | | **Site Currency**（如 CNY） |
| usdt_total_amount | money.Amount | | **USDT 应收**（= total/rate, 2dp half-up） |
| wallet_paid_amount | money.Amount | | 实际 USDT 扣款 |
| online_paid_amount | money.Amount | | wallet-only 恒 0 |
| refunded_amount | money.Amount | | 已退 USDT |
| wallet_currency | string | | 有结算快照时 `"USDT"`，旧单可能 `""` |
| exchange_rate | decimal string | nullable | `1 USDT = R SiteCurrency`，8dp |
| exchange_rate_source | string | nullable | `AUTO` / `MANUAL` |
| exchange_rate_at | RFC3339 | nullable | 快照时间 |

历史订单新快照字段允许 null（旧单无快照，前端按 legacy 展示）。

```json
{
  "order_no": "HCZ20261003001",
  "status": "paid",
  "total_amount": "100.00",
  "currency": "CNY",
  "usdt_total_amount": "13.93",
  "wallet_paid_amount": "13.93",
  "online_paid_amount": "0.00",
  "refunded_amount": "0.00",
  "wallet_currency": "USDT",
  "exchange_rate": "7.18000000",
  "exchange_rate_source": "AUTO",
  "exchange_rate_at": "2026-10-03T23:20:00+08:00"
}
```

---

## 3. Refund 退款

### 订单退款记录 OrderRefundResp
| 字段 | 类型 | 说明 |
|---|---|---|
| type | string | |
| amount | money.Amount | **USDT** |
| currency | string | **`"USDT"`** |
| remark | string | |
| created_at | RFC3339 | |

退款额来自订单 USDT 快照，不随当前汇率变化。

---

## 4. Affiliate / Commission 返利

### 佣金记录
| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | |
| commission_type | string | |
| commission_amount | money.Amount | **USDT** |
| currency | string | 固定 `"USDT"` |
| status | string | |

```json
{ "commission_amount": "1.39", "currency": "USDT", "status": "available" }
```

---

## 5. Wallet Recharge 充值

### 充值单 / 充值支付响应
| 字段 | 类型 | 说明 |
|---|---|---|
| amount | money.Amount | 到账 USDT |
| payable_amount | money.Amount | 链上应付（Gateway 币种，看 currency） |
| currency | string | Gateway 收币币种 |
| wallet_currency | string | 固定 `"USDT"`（到账本位币） |
| status | string | |
| paid_at | RFC3339 nullable | |

Gateway 自己的 exchange_rate 仅用于充值换算，**与 Global Rate 完全隔离**，前端不要混用。

---

## 6. Exchange Rate（仅 Admin）

### GET /admin/settings/exchange-rate
```json
{
  "site_currency": "CNY",
  "provider": "coingecko",
  "auto_enabled": true,
  "refresh_interval": 5,
  "auto_rate": "7.18234567",
  "manual_fallback_rate": "7.20000000",
  "effective_rate": "7.18234567",
  "effective_source": "AUTO",
  "fetched_at": "2026-10-03T23:20:00+08:00",
  "last_success_at": "2026-10-03T23:20:00+08:00",
  "last_error": "",
  "status": "healthy",
  "api_key_masked": "••••••1234"
}
```
- `PUT /admin/settings/exchange-rate`：body `{api_key?, auto_enabled, refresh_interval_min?, manual_fallback_rate?}`。api_key 留空保留现有 key。
- `POST /admin/settings/exchange-rate/refresh`：立即刷新。
- **User API 与 /public/config 永不返回 api_key 或任何汇率配置。**

---

## 7. After-Sale 售后（P1）

售后独立于订单五主状态。售后期间 `orders.status` 始终保持 `completed`，使用独立 `after_sale_status`（none/pending/resolved/rejected）和 `refund_status`（none/partial/full）。

### POST /api/v1/orders/{order_id}/after-sale
用户发起"未收到"。需 User JWT，仅本人 completed 订单。

**Request**
```json
{ "type": "not_received", "reason": "未收到充值", "description": "可选详细说明" }
```

**Response**（AfterSaleDTO）
```json
{
  "id": 1,
  "order_id": 123,
  "type": "not_received",
  "status": "pending",
  "reason": "未收到充值",
  "description": "",
  "admin_note": "",
  "refund_amount": "",
  "refund_currency": "USDT",
  "order_status": "completed",
  "refund_status": "none",
  "created_at": "2026-10-04T03:00:00Z",
  "updated_at": "2026-10-04T03:00:00Z",
  "resolved_at": null
}
```

**错误**
- 非本人 / 订单不存在 → business code 404
- 非 completed → business code 400 (`error.after_sale_order_not_completed`)
- 已有 pending 售后 → business code 400 (`error.after_sale_pending_exists`)
- type 非 not_received → business code 400

### GET /api/v1/orders/{order_id}/after-sale
用户查询自己的售后工单。需 User JWT。返回 AfterSaleDTO（同上）。无工单时 business code 404。

### GET /api/admin/v1/orders/{order_id}/after-sale
Admin 查询售后工单。需 Admin JWT + RBAC（support / finance 角色）。返回 AfterSaleDTO。

### POST /api/admin/v1/orders/{order_id}/after-sale/action
Admin 处理售后。需 Admin JWT + RBAC + Payment Compliance（paymentProtected 组）。

**Request**
```json
{ "action": "reject", "admin_note": "已核实正常到账" }
```
```json
{ "action": "resolve", "admin_note": "已解决" }
```
```json
{ "action": "partial_refund", "refund_amount": "3.00", "admin_note": "部分退款" }
```
```json
{ "action": "full_refund", "admin_note": "全额退款" }
```

**action 说明**
| action | 效果 | refund_amount |
|--------|------|---------------|
| reject | ticket=rejected, after_sale_status=rejected | 不退款 |
| resolve | ticket=resolved, after_sale_status=resolved | 不退款 |
| partial_refund | 按指定 USDT 金额退款，ticket=resolved, refund_status=partial | 必填，>0，USDT |
| full_refund | 按剩余可退款 USDT 全额退款，ticket=resolved, refund_status=full | 后端计算，前端不传 |

**Response**：AfterSaleDTO（含更新后 status / refund_amount / refund_status）。

**资金规则**
- partial/full_refund 复用 P0-2 已冻结 USDT 退款链（AdminRefundToWalletInTx）
- 退款金额 = USDT，使用订单 wallet_paid_amount snapshot
- 不读取当前汇率，不走 Gateway Refund
- 退款成功后 Wallet credit USDT + Ledger + refund_status 联动
- 订单主状态始终保持 completed

**错误**
- 无 pending ticket → business code 400 (`error.after_sale_invalid_action`)
- partial_refund 金额非法/超额 → business code 400
- action 非法 → business code 400

### AfterSaleDTO 字段说明
| 字段 | 类型 | nullable | 说明 |
|------|------|----------|------|
| id | uint | 否 | 工单 ID |
| order_id | uint | 否 | 订单 ID |
| type | string | 否 | 固定 not_received |
| status | string | 否 | pending / resolved / rejected |
| reason | string | 否 | 用户原因 |
| description | string | 否 | 用户详细说明 |
| admin_note | string | 否 | Admin 备注 |
| refund_amount | string | 是 | 退款金额（USDT，2dp），无退款时为空字符串 |
| refund_currency | string | 否 | 固定 "USDT" |
| order_status | string | 否 | 订单主状态（始终 completed） |
| refund_status | string | 否 | none / partial / full |
| created_at | time | 否 | 创建时间 |
| updated_at | time | 否 | 更新时间 |
| resolved_at | time | 是 | 处理时间，pending 时为 null |

---

## 前端禁止项
- 禁止用 site_config.currency 格式化钱包/退款/返利金额。
- 禁止前端把 CNY 金额当 USDT 显示。
- 禁止按当前汇率重算历史订单实付。
- 禁止前端直连 CoinGecko。
