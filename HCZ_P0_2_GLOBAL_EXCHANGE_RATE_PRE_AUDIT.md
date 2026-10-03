# HCZ P0-2 Global Exchange Rate — Implementation Pre-Audit

> 状态：**纯审计 + 设计，未改任何业务代码**。
> 固定规则：Wallet=USDT；商品计价=site_config.currency；Business Order 只能 Wallet；Gateway Rate 只用于 Wallet Recharge；Global Rate 只用于 Business Order 的 Site Currency→USDT；两套汇率严禁互相引用。
> 本轮不改：订单状态机、退款执行、售后、会员体系、Payment Gateway Rate。

---

## 0. 结论速览

| 项 | 判断 |
|---|---|
| 商品价格→Site Currency 最终金额链路 | **KEEP**（已完备，不改计算逻辑） |
| Site Currency→USDT 换算 | **BUILD**（当前完全缺失，存在 1:1 直扣的隐含假设） |
| 汇率 Provider / 缓存 / 存储 / 周期刷新 | **BUILD** |
| 订单汇率快照字段 | **BUILD（新增列，可空，向后兼容）** |
| Gateway 充值汇率 | **KEEP 不动**（与本特性物理隔离） |
| 退款按订单快照 USDT 回充 | **MODIFY**（读快照，不再用 site 金额 1:1） |
| fail-closed | **MUST**：无有效汇率直接拒单，禁止 1:1 |

**当前最大风险（BLOCK 级事实）**：`order_service.go:464` 与 `order_wallet_bridge.go` 把 **USDT 钱包余额** 直接和 **Site Currency 的 TotalAmount** 比较/扣款；`refund/wallet.go:91` 把 Site Currency 金额直接回充 USDT 钱包。在多币种站点这等于按 1:1 结算，必须由 P0-2 修正。

---

## 1. 精确定位（带文件:行号）

### 1.1 商品基础价格在哪确定
- `internal/modules/order/application/order_service_validate.go:119`
  `basePrice := sku.PriceAmount.Decimal.Round(2)` —— 基础单价来自 SKU 价格，币种 = site currency。

### 1.2 quantity / wholesale / member price / reseller / 优惠 → 最终应付 Site Currency 金额
全部在 `order_service_validate.go` 内、`buildOrderResult` 中按固定顺序，全程 site currency，每步 `.Round(2)`：
1. **活动价** `:126-133` `promotionService.ApplyPromotion` → promoUnitPriceAmount。
2. **批发价** `:144-164` `ResolveWholesaleUnitPriceForSKU`，与活动价取更优（不叠加）。
3. **会员价** `:166-177` `memberLevelService.ResolveMemberPrice`，再取更优。
4. **优惠券（订单级）** `:286-323` `couponService.ApplyCoupon`，按比例分摊到订单项。
5. **合计** `:325-338` `totalAmount = Σ(itemTotal - couponDiscount)`，`:339` 校验 >0。
- 分销价重写在 `reseller_pricing.go:95`（plan.TotalAmount 被改写），最终仍汇入 `result.TotalAmount`（`:132`）。
- **最终应付 site currency 金额 = `orderBuildResult.TotalAmount`（decimal，2dp）**，币种 = `currency`（= site_config.currency）。

### 1.3 Wallet 扣款发生在哪
- 扣款桥：`internal/modules/order/application/order_wallet_bridge.go`
  - `ApplyWalletBalance(...)` `:19` → `wallets.ApplyOrderBalance(tx.Wallets(), OrderBalanceInput{TotalAmount: order.TotalAmount, WalletPaidAmount, UseBalance})`。
  - 写回 `wallet_paid_amount` / `online_paid_amount`（`:54-60`）。
- 余额前置校验：`order_service.go:460-465`
  `account.Balance.Decimal.LessThan(result.TotalAmount) → ErrInsufficientBalance`。
  **注意：account.Balance 是 USDT，result.TotalAmount 是 site currency —— 当前直接比较。**

### 1.4 Order 创建事务边界
- `order_service.go:544` `s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {...})`。
- 钱包动作通过 `tx.Wallets()` 进入同一事务（`order_wallet_bridge.go:35`）；分销账务走 `tx.ResellerAccounting()`；退款同理（`refund/wallet.go:67`、`refund/service.go:290`）。
- **汇率读取与换算必须发生在事务内、扣款前**，与扣款共用同一快照。

### 1.5 订单现有 amount / currency 快照字段
表 `orders`（`domain/order.go`），全部 `decimal(20,2)`，币种 = `Currency`（`:20`）：
- `OriginalAmount :21`、`DiscountAmount :22`、`MemberDiscountAmount :23`、`PromotionDiscountAmount :24`、`WholesaleDiscountAmount :25`
- `TotalAmount :26`（实付，site currency）
- `WalletPaidAmount :27`、`OnlinePaidAmount :28`（新单 Online 恒 0）、`RefundedAmount :29`
- `ResellerProfitAmount :37`
订单项 `order_item.go`：OriginalUnitPrice/UnitPrice/CostPrice/OriginalTotalPrice/TotalPrice + 各 discount。
退款单 `order_refund_record.go:17-20`：Amount、Currency。

### 1.6 refund / commission 当前读哪个金额
- 退款：`refund/wallet.go:91` `refundable := order.TotalAmount - refundedBefore`，`:96` `wallets.CreditInTransaction(...)` 直接以该 decimal 回充钱包（**当前按 site=USDT 1:1**）。
- 分销/返利：`refund/wallet.go:170` `resellerAccounting.HandleRefundDeduct(tx.ResellerAccounting(), &order, record, ...)`；下单时返利/分润基数来自 order 金额快照。
- **含义**：退款与分润都必须复用订单**快照汇率换算出的 USDT**，不能用实时汇率重算，也不能用 site 金额当 USDT。

### 1.7 settings 存储结构 & Admin API
- 存储：`settings(key PRIMARY KEY, value JSON)`（`settings/infrastructure/gormstore/store.go:11-19`）。
- 上层是 typed registry / schema 包（`settings/schema/**`），按 key 注册读写器；Admin API 在 `transport/http/admin_handler.go`，统一挂 `settings/transport/http/routes.go`。
- **新增 Global Rate 配置 = 新增一个 settings key，无需改表结构**（JSON 值即可）。

### 1.8 Redis / cache 现状
- `internal/cache/`（如 `auth_state.go`）：服务端 Redis 缓存，带 TTL。
- 队列：`internal/queue` + `hibiken/asynq`（Redis 后端）。
- **结论：Redis 现成，可直接做汇率缓存（短 TTL）。**

### 1.9 已有定时任务 / worker
- `internal/app/jobs/service.go`：asynq `Scheduler`，已用 `@every Xm` 注册：
  affiliate 确认佣金、reseller 台账确认、上游库存同步（间隔可后台配置）、库存告警、采购同步。
- **新增「周期拉取 Global Rate」任务直接在 `registerPeriodicTasks` 注册一个 `@every Nm` 即可，基础设施零新增。**

### 1.10 money / decimal 精度
- `internal/shared/money/amount.go`：`money.Amount` = `shopspring/decimal`，构造与读写一律 `Round(2)`，DB `decimal(20,2)`。
- 钱包余额同为 USDT、2dp。
- **汇率本身需要更高精度**（建议 `decimal(20,8)`），换算中间过程用 decimal 全精度，最终落金额再 Round(2)。

---

## 2. 方向与换算规则（必须全系统统一，二选一）

**选定方向：`1 USDT = R SiteCurrency`（R = site currency 每 1 USDT 的价格）。**
- 例：site currency = CNY，R = 7.20 → 1 USDT = 7.20 CNY。
- 与 Gateway 充值渠道 `exchange_rate`（payment channel 配置，callback 测试里 `"exchange_rate":"7"`）语义一致，运维心智统一。
- **Site Currency → USDT**：
  `usdtAmount = siteCurrencyAmount / R`，中间全精度，结果 `Round(2)`（round-half-up，与 money.Amount 一致）。
- **USDT → Site Currency（展示/对账）**：`siteAmount = usdtAmount * R`。

> 为什么不是 1 SiteCurrency = X USDT：钱包本位是 USDT，渠道充值已用「1 USDT = X 本币」；保持同一方向可避免充值/下单两套口径。

### rounding / fail-closed
- 换算只在**下单事务内做一次**，把 R、R 来源、取数时间、换算后的 USDT 应付额一并快照到订单。
- **退款/分润一律读订单快照的 USDT 金额**，绝不实时重算。
- 无有效汇率（Provider 失败 且 无可用缓存/无手动兜底）→ **拒单**，返回 `ErrExchangeRateUnavailable`；**禁止任何 1:1 兜底**。

---

## 3. 设计（只设计，不实施）

### 3.1 GlobalExchangeRateService（接口）
```go
type GlobalRate struct {
    Rate        decimal.Decimal // 1 USDT = Rate SiteCurrency
    Source      string          // provider_manual / provider_auto_<name>
    FetchedAt   time.Time
    Valid       bool
}

type Service interface {
    // ResolveForOrder 返回当前可用于下单的有效汇率；无有效汇率时返回 error（fail-closed）
    ResolveForOrder(ctx context.Context) (GlobalRate, error)
    // Refresh 由周期任务/手动触发，拉取最新汇率并写入缓存+存储
    Refresh(ctx context.Context) error
}
```

### 3.2 Auto Provider 接口
```go
type RateProvider interface {
    Name() string
    // Fetch 拉取 1 USDT = R SiteCurrency；siteCurrency 由调用方传入
    Fetch(ctx context.Context, siteCurrency string) (rate decimal.Decimal, fetchedAt time.Time, err error)
}
```
- 至少一个可插拔实现（如公开汇率源）；网络失败/超时/非 200/数值非法 → 视为不可用，走 fallback。

### 3.3 Manual fallback
- settings key `exchange_rate.global` JSON：`{ rate, source:"manual", enabled }`。
- Provider 失败时：① 先用 Redis 缓存的「最近一次有效 auto rate」（受新鲜度上限约束，如 ≤24h）；② 再用后台手动设置 rate；③ 都无 → fail-closed 拒单。

### 3.4 Redis cache
- key：`exchange:global:{site_currency}`，value JSON `{rate,source,fetchedAt}`，TTL = 刷新周期 × ~2。
- Provider 成功后写缓存；下单读优先缓存（本地内存 + Redis 二级可后续加）。

### 3.5 DB / config 存储
- 配置：复用 `settings` 键值表（新增 key，无 migration）。
- 历史汇率：可选新表 `exchange_rate_snapshots(id, site_currency, rate(decimal 20,8), source, fetched_at, created_at)`，供审计与回溯。

### 3.6 Order 快照字段（新增列，可空，向后兼容）
`orders` 表新增：
- `exchange_rate decimal(20,8) NULL` —— 下单时 R
- `exchange_rate_source varchar(20) NULL` —— auto/manual/cached
- `exchange_rate_snapshot_at datetime NULL`
- `usdt_total_amount decimal(20,2) NULL` —— 实际要扣的 USDT
- 现有 site currency 字段全部保留（展示、对账、退款基数仍以 site 金额为准，退款时按快照 R 换算成 USDT）。

### 3.7 字段约定
rate=decimal(20,8)；source∈{auto, manual, cached}；time=fetched_at(unix)；status=valid/stale/unavailable（驱动 fail-closed）。

---

## 4. 标签清单

- **KEEP**：SKU/批发/会员/活动/优惠券计价链（`order_service_validate.go`）；Gateway 充值汇率；settings 键值结构；asynq 调度；money.Amount=2dp；orders 现有 site 金额字段。
- **MODIFY**：
  - `order_service.go:460-465` 余额校验改为「USDT 应付额」对比。
  - `order_wallet_bridge.go ApplyWalletBalance` 扣款金额用快照 USDT 额。
  - `refund/wallet.go`（及 service.go）回充钱包金额用订单快照 USDT，不再 site 1:1。
  - 分润/返利基数以 site 金额记录、USDT 钱包动作按快照换算（不双转）。
- **BUILD**：exchangerate 模块（domain/application/infrastructure: redis + gormstore/settings、provider、http admin handler）；周期刷新任务；订单快照列与写入；Admin 汇率设置/查看页。
- **BLOCK**：无有效汇率时禁止下单（fail-closed）；严禁 Gateway Rate 与 Global Rate 互相引用。

---

## 5. 最小实现文件清单（新增）

```
internal/modules/exchangerate/
  domain/rate.go                 // GlobalRate 值对象、方向常量、错误
  application/service.go         // ResolveForOrder / Refresh（含 fallback 决策）
  application/service_test.go
  contract/contract.go           // RateProvider / Store / Cache 端口
  infrastructure/provider/...    // Auto provider 实现
  infrastructure/redisstore/...  // Redis 缓存
  infrastructure/settingsstore/... // 手动兜底（读 settings）
  transport/http/admin_handler.go
  transport/http/routes.go
internal/queue/task_exchange_rate_refresh.go   // asynq 任务
internal/app/jobs/service.go                   // 注册周期任务（改）
internal/modules/order/application/order_service_validate.go  // 取汇率+算USDT（改）
internal/modules/order/application/order_wallet_bridge.go      // 用 USDT 扣款（改）
internal/modules/order/application/refund/wallet.go            // 用快照 USDT 回充（改）
internal/modules/order/domain/order.go                        // 新增快照字段（改）
frontend/admin/.../ExchangeRate.vue + i18n                     // 后台查看/手动兜底
```

## 6. Migration 设计
- **可重复、非破坏**：仅给 `orders` `ADD COLUMN` 4 个可空列；`exchange_rate_snapshots` 新建表。
- 老订单列 NULL：退款逻辑对 NULL 列回退现状（或标记为历史单不换算），不回填、不重算。
- 不删列、不改旧列类型、不改现有索引。

## 7. 测试矩阵
| 场景 | 期望 |
|---|---|
| 正常 auto 汇率，下单 | site 金额按 R 换算 USDT，扣款=usdt_total_amount，余额校验正确 |
| 余额（USDT）不足 | 拒单 ErrInsufficientBalance |
| Provider 失败 + 缓存有效(≤24h) | 用缓存 R，下单成功 |
| Provider 失败 + 缓存过期 + 无手动 | **拒单 ErrExchangeRateUnavailable（fail-closed）** |
| 手动兜底 rate | 用 manual rate，source=manual |
| 退款 | 按订单快照 R 换算 USDT 回充，不读实时汇率 |
| 分润/返利 | site 金额记账、USDT 钱包动作按快照，不双转 |
| 充值链路 | Gateway 渠道 exchange_rate 不受影响，回归通过 |
| 换算 rounding | 70 CNY / 7.20 → 9.72 USDT（half-up 2dp），边界用例覆盖 |
| R 精度 | rate 20,8；中间全精度；落金额 2dp |

---

## 8. 不做（本轮硬边界）
- 不动订单 5 状态机、退款执行流程、售后、会员等级/会员价/批发价。
- 不动 Payment Gateway 自有 exchange_rate（仅充值用）。
- 不回填历史订单、不做破坏性 migration。

## 9. 下一步
P0-2 实施顺序建议：① 建 exchangerate 模块 + provider/cache/settings 端口与单测 → ② 订单快照列 + 下单事务内取率换算 → ③ 退款/分润改读快照 → ④ asynq 周期刷新 + Admin 手动兜底页 → ⑤ 上述测试矩阵全绿 + 充值回归。
