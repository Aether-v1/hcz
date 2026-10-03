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

## 前端禁止项
- 禁止用 site_config.currency 格式化钱包/退款/返利金额。
- 禁止前端把 CNY 金额当 USDT 显示。
- 禁止按当前汇率重算历史订单实付。
- 禁止前端直连 CoinGecko。
