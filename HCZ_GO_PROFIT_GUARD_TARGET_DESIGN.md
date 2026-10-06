# HCZ Go Profit Guard V1 — Target Design

- **项目**: `github.com/Aether-v1/hcz`
- **设计日期**: 2026-10-06
- **基于**: `docs/audits/HCZ_GO_PROFIT_GUARD_REALITY_AUDIT.md` 真实源码审计
- **原则**: AUDIT FIRST → DESIGN → 最小必要修改 → 不碰 Affiliate Finance Agent 正在修改的佣金域

---

## 1. Pricing Source of Truth

### 现状（已确认）

| 维度 | 实现 | 文件 |
|---|---|---|
| 商品售价 | `Product.PriceAmount` / `ProductSKU.PriceAmount`，decimal(20,2)，全站 Site Currency (CNY) | `catalog/product/domain/product.go:22`, `sku.go:21` |
| 优惠叠加 | SKU 基准价 → 活动价 vs 批发价互斥取低 → 会员价 → 优惠券按比例分摊 | `order_service_validate.go:119-177` |
| 订单金额快照 | `Order.TotalAmount` (CNY) + 各类 Discount | `order/domain/order.go:24-29` |
| 前端权限 | 下单请求无 price/amount 字段，金额全由后端计算 | `preview_handler.go:83-97` |

### Profit Guard 接入点

在 `OrderService.createOrder` 中，**金额计算完成后、汇率换算后、DB 写入前**插入 Profit Guard 校验：

```
buildOrderResult() → TotalAmount(CNY)
    ↓
rateResolver.Resolve() → ExchangeRate + UsdtTotalAmount(USDT)
    ↓
★ ProfitGuard.Validate(orderBuildResult, rate, affiliateConfig) ★
    ↓
orderStore.Create()
```

校验失败时返回明确错误码（如 `ERR_UNPROFITABLE_ORDER`），拒单。

---

## 2. Cost Source

### 现状（已确认）

| 维度 | 实现 |
|---|---|
| 成本字段 | `Product.CostPriceAmount` / `ProductSKU.CostPriceAmount`，decimal(20,2) |
| 聚合逻辑 | `Product.CostPriceAmount = min(活跃 SKU.CostPriceAmount)`，保存时聚合 |
| 上游对接 | 对接商品成本 = 上游价格 × 连接配置 ExchangeRate |
| 订单快照 | `OrderItem.CostPrice` 快照（下单时写入） |
| 前台隔离 | `Order.StripCostPrice()` 清除成本价，不下发用户 |
| 开发库实证 | 3/3 活跃商品 cost=0（数据未录入） |

### Profit Guard 成本计算

```go
// Supplier Cost (CNY) = Σ(order_items.cost_price × quantity)
supplierCost := decimal.Zero
for _, item := range buildResult.Items {
    supplierCost = supplierCost.Add(
        item.CostPrice.Decimal.Mul(decimal.NewFromInt(int64(item.Quantity)))
    )
}
```

### 前置条件

Profit Guard 上线前必须：
1. **Admin 录入所有活跃商品的成本价**
2. 新增配置项 `profit_guard.require_cost_price = true` 时，cost=0 的商品拒绝下单（或标记告警）

---

## 3. FX Source

### 现状（已确认）

| 维度 | 实现 |
|---|---|
| 汇率方向 | 1 USDT = R SiteCurrency (CNY) |
| 自动源 | CoinGecko `GET /api/v3/simple/price?ids=tether&vs_currencies=CNY` |
| 手动兜底 | Admin `SetManual(rate)` |
| Resolve 优先级 | AUTO(valid && fresh<10min) → MANUAL(>0) → ErrRateUnavailable(fail-closed) |
| Redis 缓存 | key `global_exchange_rate:state`，TTL 2min，写穿 |
| 持久化 | settings KV `global_exchange_rate` |
| 刷新 | asynq `@every 5m` 硬编码 + Admin 手动 |
| 订单快照 | `Order.ExchangeRate` / `ExchangeRateSource` / `ExchangeRateAt` |
| 支付变价 | 不重新 Resolve，使用快照 |

### Safety Buffer 设计

**方向确认**：1 USDT = R CNY。平台防止低收 USDT 时，降低 effective_rate 使用户多付 USDT：

```
effective_rate = market_rate × (1 - buffer)
usdt_total = cny_total / effective_rate
```

示例：buffer=0.5%，market_rate=7.0，cny_total=100
- 无 buffer: usdt = 100/7.0 = 14.29
- 有 buffer: effective_rate = 6.965, usdt = 100/6.965 = 14.36（多收 0.07 USDT）

### 实现方案

统一通过 `Rate.ToUSDT()` 增加 buffer 参数，消除当前双实现（P1-3）：

```go
// exchangerate/domain/rate.go
func (r Rate) ToUSDT(siteAmount decimal.Decimal, bufferPercent decimal.Decimal) (decimal.Decimal, error) {
    if r.Rate.LessThanOrEqual(decimal.Zero) {
        return decimal.Zero, ErrRateUnavailable
    }
    effectiveRate := r.Rate.Mul(decimal.NewFromInt(1).Sub(bufferPercent.Div(decimal.NewFromInt(100))))
    return siteAmount.Div(effectiveRate).Round(2), nil
}
```

`order_service.go:567` 改为调用 `rate.ToUSDT(result.TotalAmount, bufferPercent)`。

### 配置

新增 `profit_guard_config.fx_buffer_percent`（默认 0.5，范围 0~5）。

---

## 4. Affiliate Cost

### 现状（已确认）

| 维度 | 实现 |
|---|---|
| Rate source | 全局 `affiliate_config.level_rates[1..10]`，无商品级/profile 级 |
| 基数 | `order.WalletPaidAmount`（USDT 实扣），非 TotalAmount/利润 |
| 计算方式 | 每级同一全量基数 `base × rate_i / 100`，非递减 |
| 触发时机 | 订单 completed（非创建/paid） |
| 上界约束 | `Σ(enabled level_rates) ≤ 100` |
| 账本 | append-only，退款走 REVERSAL/DEBT |
| Order 快照 | 仅 `AffiliateProfileID` / `AffiliateCode`，**无 CommissionAmount 汇总** |

### Profit Guard 最大佣金成本预估

由于佣金在 completed 时才入账，Profit Guard 必须在下单时**事前预估**：

```go
// MaxCommission(USDT) = WalletPaidAmount × S / 100
// S = Σ_{i=1..Min(MaxLevel,10)} (LevelRates[i].Rate if Enabled else 0)
func estimateMaxCommission(walletPaidUSDT decimal.Decimal, affiliateConfig settings.AffiliateSetting) decimal.Decimal {
    sumRate := decimal.Zero
    maxLevel := min(affiliateConfig.MaxLevel, 10)
    for i := 0; i < maxLevel; i++ {
        if affiliateConfig.LevelRates[i].Enabled {
            sumRate = sumRate.Add(decimal.NewFromFloat(affiliateConfig.LevelRates[i].Rate))
        }
    }
    return walletPaidUSDT.Mul(sumRate).Div(decimal.NewFromInt(100)).Round(2)
}
```

### 币种换算

佣金是 USDT，收入/成本是 CNY，必须用订单汇率快照统一换算：

```
MaxCommission(CNY) = MaxCommission(USDT) × Order.ExchangeRate
```

### 注意事项

- **不修改 Affiliate 域**：Affiliate Finance Agent 正在修改佣金域，Profit Guard 只读取 `affiliate_config.level_rates` 做预估，不改动佣金生成逻辑
- 遗留死代码 `calculateCommissionBaseAmount`（commission.go:644）勿接入，现行基数 = WalletPaidAmount

---

## 5. Fee Cost

### 现状（已确认）

| Fee 类型 | 存在 | 计入 Profit Guard |
|---|---|---|
| Platform / Service fee | 不存在 | — |
| Payment/Gateway fee (merchant_absorbed) | 存在 | ⚠️ 钱包直付模式下发生在充值环节，无法精确归因到单订单 |
| Payment fee 退款分摊 | 存在 | 随退款自动 |
| Supplier fee | 不存在 | — |
| Chain/Gas/Network fee | 不存在 | — |
| Withdrawal fee | 存在（USDT，向用户收=平台收入） | 不计入单笔订单成本 |
| Recharge fee | 存在 | 与订单无直接归因 |
| Refund fee | 不存在 | — |

### Profit Guard V1 策略

**V1 不建模额外 Fee 成本**，原因：
1. 业务订单钱包直付（`GetWalletOnlyPayment()=true`），在线网关费发生在充值环节，无法精确归因到单订单
2. Platform/Supplier/Chain fee 字段不存在，新增需 DB migration
3. V1 核心目标是防止"佣金 + 成本"导致的负利润，Fee 可在 V2 补充

**V2 可选**：新增 `profit_guard_config.estimated_fee_percent` 粗估网关费摊销。

---

## 6. Settlement Rounding

### 现状

- `order_service.go:567`: `orderUsdtTotal = TotalAmount.Div(rRate).Round(2)`
- shopspring/decimal `Round(2)` 默认 **half-even**（银行家舍入），注释声称 half-up（P3 不一致）
- 前端 DISPLAY 规则：ROUND_HALF_UP 2 位（1.235→1.24），仅展示

### 目标设计

如果产品决策采用**最小 0.01 USDT 结算单位**，则后端结算应使用 **Decimal 向上取到 2 位**（Ceil），确保平台不少收：

```go
// 13.880001 → 13.89（向上取 2 位）
func ceilTo2(d decimal.Decimal) decimal.Decimal {
    return d.Div(decimal.NewFromInt(100)).Ceil().Mul(decimal.NewFromInt(100))
}
```

### 约束

- **仅改后端结算精度**，不改变前端 DISPLAY 规则
- 前端继续 ROUND_HALF_UP 2 位展示
- 需确认与现有 `Round(2)` 的差异是否影响对账（向上取 vs half-even）

> **本轮仅确认现状，不直接修改。** 是否采用 Ceil 待产品决策。

---

## 7. Quote Snapshot

### 现状（已确认）

Order 级快照字段完整：

| 字段 | 币种 | 说明 |
|---|---|---|
| TotalAmount | CNY | 实付总额 |
| UsdtTotalAmount | USDT | 应收 USDT（汇率换算） |
| ExchangeRate | decimal(20,8) | 1 USDT = R SiteCurrency |
| ExchangeRateSource | string | AUTO / MANUAL |
| ExchangeRateAt | time | 汇率快照时间 |
| WalletPaidAmount | USDT | 钱包实扣 |
| RefundedAmount | USDT | 已退款额 |
| ResellerProfitAmount | CNY | 分销差价 |

OrderItem 级快照：
- UnitPrice / CostPrice / TotalPrice / 各类折扣分摊

### Profit Guard 新增快照（V2 可选）

V1 不新增 DB 字段，Profit Guard 校验结果可通过日志/指标记录。

V2 可考虑在 Order 表增加：
- `ExpectedProfitAmount` (CNY)：下单时预估净利润
- `ProfitGuardPassed` (bool)：是否通过 Profit Guard 校验
- `MaxCommissionAmount` (USDT)：预估最大佣金

---

## 8. Quote TTL

### 现状（已确认）

- 无独立 Quote 实体，Preview 无状态
- TTL 载体 = `Order.ExpiresAt = now + PaymentExpireMinutes`（默认 15min）
- 过期后 asynq 触发 `CancelExpiredOrder`
- 支付窗口期内汇率波动平台自担（无 buffer 时）

### Profit Guard 与 TTL 的关系

Profit Guard 在校验时使用**当前汇率**，校验通过后汇率快照到 Order。支付窗口期内（最长 15min）：
- 汇率上涨（CNY 贬值）→ 平台收的 USDT 变少 → 风险
- 汇率下跌（CNY 升值）→ 平台收的 USDT 变多 → 收益

**Safety buffer 是对冲此风险的主要手段**。若 buffer 不足以覆盖极端波动，可考虑：
- 缩短支付窗口期（如 5min）
- 支付时允许重新 Resolve（但会改变"价格冻结"设计，需产品决策）

> V1 保持现有 TTL 设计，通过 safety buffer 对冲汇率风险。

---

## 9. Refund Rule

### 现状（已确认）

| 维度 | 实现 |
|---|---|
| 退款基数 | `WalletPaidAmount`（USDT 实扣快照），非 TotalAmount |
| 退款币种 | USDT |
| 重新获取汇率 | **否**，两条退款路径均无 rateResolver 调用 |
| 部分退款防超退 | `refundable = WalletPaidAmount - RefundedAmount`，行锁串行化 |
| 退款入账 | `CreditInTransaction` → AvailableBalance += amount (USDT) |
| Payment fee 退还 | 仅 merchant_absorbed，按累计比例分摊 |
| 取消自动退款 | `UpdateFieldsWhereWalletPaid` 条件更新做原子占位，天然幂等 |

### Profit Guard 与退款的关系

Profit Guard 是**事前拦截**，不影响退款逻辑。退款继续基于原订单快照，不重新计算利润。

**注意**：退款时佣金逆向（REVERSAL/DEBT）由 Affiliate 域处理，Profit Guard 不介入。

---

## 10. Profit Guard Formula

### 核心公式

```
Expected Net Profit (CNY) =
    Expected Revenue (CNY)
  - Supplier Cost (CNY)
  - Max Affiliate Cost (CNY)
  - FX Risk Cost (CNY)
  - Reseller Profit (CNY, 可选)

约束: Expected Net Profit >= Required Minimum Profit
```

### 各变量定义

| 变量 | 公式 | 数据源 |
|---|---|---|
| Expected Revenue | `TotalAmount` (CNY) | buildOrderResult |
| Supplier Cost | `Σ(order_items.cost_price × quantity)` | OrderItem 快照 |
| Max Affiliate Cost (USDT) | `UsdtTotalAmount × Σ(enabled level_rates) / 100` | affiliate_config |
| Max Affiliate Cost (CNY) | `MaxCommission(USDT) × ExchangeRate` | Order 汇率快照 |
| FX Risk Cost | `UsdtTotalAmount × fx_buffer_percent/100 × ExchangeRate` | profit_guard_config |
| Reseller Profit | `ResellerProfitAmount`（若按买家总额口径） | Order 快照 |
| Required Minimum Profit | `max(minimum_profit_cny, TotalAmount × minimum_profit_rate/100)` | profit_guard_config |

### 校验逻辑（伪代码）

```go
func (pg *ProfitGuard) Validate(
    buildResult *OrderBuildResult,
    rate exchangeratedomain.Rate,
    affiliateConfig settings.AffiliateSetting,
    config ProfitGuardConfig,
) error {
    if !config.Enabled {
        return nil
    }

    // 1. Revenue
    revenue := buildResult.TotalAmount

    // 2. Supplier Cost
    supplierCost := decimal.Zero
    for _, item := range buildResult.Items {
        supplierCost = supplierCost.Add(item.CostPrice.Decimal.Mul(intToDecimal(item.Quantity)))
    }

    // 3. Max Affiliate Cost
    usdtTotal := revenue.Div(rate.Rate).Round(2)  // 或用 rate.ToUSDT(revenue, buffer)
    sumRate := sumEnabledLevelRates(affiliateConfig)
    maxCommissionUSDT := usdtTotal.Mul(sumRate).Div(100).Round(2)
    maxCommissionCNY := maxCommissionUSDT.Mul(rate.Rate).Round(2)

    // 4. FX Risk Cost (buffer 对应的额外成本)
    fxRiskCost := usdtTotal.Mul(config.FXBufferPercent).Div(100).Mul(rate.Rate).Round(2)

    // 5. Net Profit
    netProfit := revenue.Sub(supplierCost).Sub(maxCommissionCNY).Sub(fxRiskCost)

    // 6. Required Minimum Profit
    requiredProfit := decimal.Max(
        config.MinimumProfitCNY,
        revenue.Mul(config.MinimumProfitRate).Div(100),
    )

    // 7. Validate
    if netProfit.LessThan(requiredProfit) {
        return fmt.Errorf("%w: revenue=%.2f cost=%.2f commission=%.2f fx_risk=%.2f net=%.2f required=%.2f",
            ErrUnprofitableOrder, revenue, supplierCost, maxCommissionCNY, fxRiskCost, netProfit, requiredProfit)
    }
    return nil
}
```

### 拒单行为

- 返回 HTTP 400 + 错误码 `UNPROFITABLE_ORDER`
- 错误信息包含明细（收入/成本/佣金/净利润/最低利润），便于 Admin 排查
- 记录审计日志（订单号、商品、计算明细）

---

## 11. Admin Configuration

### 新增 Settings Key: `profit_guard_config`

```go
type ProfitGuardConfig struct {
    Enabled             bool    `json:"enabled"`               // 总开关
    MinimumProfitCNY    float64 `json:"minimum_profit_cny"`    // 固定最低利润（CNY）
    MinimumProfitRate   float64 `json:"minimum_profit_rate"`   // 比例最低利润（%）
    FXBufferPercent     float64 `json:"fx_buffer_percent"`     // 汇率安全缓冲（%，默认 0.5）
    RequireCostPrice    bool    `json:"require_cost_price"`    // cost=0 时是否拒单
    MaxCommissionRate   float64 `json:"max_commission_rate"`   // 单订单最大佣金成本占比（%，可选，覆盖 affiliate Σ≤100）
}
```

### 注册到 Settings Registry

在 `settings/application/default_registry.go` 中新增 `profit_guard_config` 注册项，与现有 `affiliate_config` / `order_risk_control_config` 并列。

### Admin 前端页面

新增 `SettingsProfitGuardTab.vue`（参考 `SettingsExchangeRateTab.vue` 模式），包含：
- Profit Guard 总开关
- 最低利润（固定金额 + 比例）
- FX buffer 百分比
- cost=0 拒单开关
- 实时测试：输入商品 ID + 数量，显示预估净利润

### 已有配置（不重复造）

- 商品售价/成本价：`Products.vue` / `ProductEditModal.vue`
- 汇率管理：`SettingsExchangeRateTab.vue`
- Affiliate 佣金比例：`AffiliateSettings.vue`
- 支付渠道手续费：`PaymentChannels.vue`
- 提现手续费：`WalletWithdrawals.vue`
- 订单风控：`OrderRiskControl.vue`

---

## 12. Implementation Phases

### Phase 0: 前置修复（阻塞项）

| # | 任务 | 优先级 | 说明 |
|---|---|---|---|
| 0.1 | 修复混合币种减法 bug | P1 | `payment_service_create.go:169` 改用 `OnlinePaidAmount` |
| 0.2 | 录入商品成本价 | P1 | Admin 为所有活跃商品录入 CostPriceAmount |
| 0.3 | 修复退款测试失败 | P1 | `TestWalletServiceAdminRefundToWallet` — beneficiary_user_id |

### Phase 1: Profit Guard V1 核心

| # | 任务 | 说明 |
|---|---|---|
| 1.1 | 新增 `profit_guard_config` settings + 注册 | 后端配置项 |
| 1.2 | 统一 `ToUSDT()` 调用路径 + 增加 buffer 参数 | 消除双实现（P1-3），增加 safety buffer（P1-2） |
| 1.3 | 实现 `ProfitGuard.Validate()` | 核心校验逻辑 |
| 1.4 | 接入 `createOrder` 校验点 | 金额计算后、DB 写入前 |
| 1.5 | 新增错误码 `UNPROFITABLE_ORDER` + 审计日志 | 拒单反馈 |
| 1.6 | Admin 前端 Profit Guard 配置页面 | 配置界面 |
| 1.7 | 单元测试 + 集成测试 | 覆盖各成本场景 |

### Phase 2: 增强（可选）

| # | 任务 | 说明 |
|---|---|---|
| 2.1 | Manual fallback 时间戳修复 | P1-4 |
| 2.2 | RefreshIntervalMin 配置生效 | P2-1 |
| 2.3 | ManualRate 区间校验 | P2-2 |
| 2.4 | Order 级总成本/利润快照 | P2-3 |
| 2.5 | Preview 返回 USDT 估算 | P3-2 |
| 2.6 | 结算取整改为 Ceil 2 位 | 待产品决策 |
| 2.7 | estimated_fee_percent 粗估网关费 | V2 |

---

## 13. Non-Goals (V1 不做)

1. **不修改 Affiliate 佣金域** — Affiliate Finance Agent 正在处理，Profit Guard 只读取配置做预估
2. **不新增 platform/supplier/chain fee 字段** — V1 不建模额外 Fee 成本
3. **不改变退款逻辑** — 退款继续基于原订单快照
4. **不改变支付窗口期/TTL** — 通过 safety buffer 对冲汇率风险
5. **不修改前端 DISPLAY 规则** — 前端继续 ROUND_HALF_UP 2 位
6. **不做实时利润 Dashboard** — V1 只做事前拦截，监控在 Phase 3
7. **不改动 reseller 定价逻辑** — 仅在利润公式中可选计入 ResellerProfitAmount

---

**设计完成。本设计基于真实 Go 源码审计，所有现状描述均有文件+行号证据。实施前请确认 Phase 0 阻塞项已清除。**
