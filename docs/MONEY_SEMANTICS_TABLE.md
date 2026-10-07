# HCZ Money Semantics Table — 金额字段币种/单位/含义总表

> 本文档是 HCZ（`github.com/Aether-v1/hcz`）资金计算的唯一权威语义参考。
> 任何涉及金额的新代码必须先确认本表的币种与单位，禁止跨币种直接运算。
> 所有资金计算一律使用 `github.com/shopspring/decimal`，禁止 `float64` 参与。

## 0. 全局约定

| 项 | 值 |
|---|---|
| 站点计价币种（Site Currency） | CNY（`constants.SiteCurrencyDefault`） |
| 钱包本位币（Wallet Currency） | USDT（`exchangeratedomain.WalletCurrency`，固定） |
| 汇率方向 | `1 USDT = R CNY`（R ≈ 7.0~7.3），存为 `decimal(20,8)` |
| 金额精度 | 金额一律 `decimal(20,2)`；汇率 `decimal(20,8)` |
| 前端展示舍入 | `ROUND_HALF_UP` 2 位（仅展示，不参与结算） |
| 后端结算舍入 | 应收 USDT 向上 CEILING 到 2 位（仅正向应收，见 P8） |
| 数据类型 | `money.Amount`（封装 `decimal.Decimal`）；汇率用 `decimal.NullDecimal` |

---

## 1. Order（订单表 `orders`）

| 字段 | 币种 | 单位/精度 | 含义 |
|---|---|---|---|
| `OriginalAmount` | CNY | 元，2dp | 下单时商品原始总额（优惠前） |
| `DiscountAmount` | CNY | 元，2dp | 总优惠金额 |
| `MemberDiscountAmount` | CNY | 元，2dp | 会员优惠分摊 |
| `PromotionDiscountAmount` | CNY | 元，2dp | 活动价优惠分摊 |
| `WholesaleDiscountAmount` | CNY | 元，2dp | 批发价优惠分摊 |
| `TotalAmount` | **CNY** | 元，2dp | 订单实付总额（Site Currency 口径，Profit Guard Revenue） |
| `UsdtTotalAmount` | **USDT** | USDT，2dp | 本单应收 USDT 快照（= TotalAmount / effective_rate，CEILING 2dp） |
| `ExchangeRate` | — | 1 USDT = R CNY，8dp | 下单时冻结的汇率快照（`decimal.NullDecimal`） |
| `ExchangeRateSource` | — | — | `AUTO` / `MANUAL` |
| `ExchangeRateAt` | — | timestamp | 汇率快照时间 |
| `WalletPaidAmount` | **USDT** | USDT，2dp | 钱包实扣金额（钱包本位币，订单完成时为 `UsdtTotalAmount`） |
| `OnlinePaidAmount` | **CNY** | 元，2dp | 在线网关实付（Site Currency 口径；USDT 结算单 wallet-only 恒为 0） |
| `RefundedAmount` | **USDT** | USDT，2dp | 已退回钱包金额（按原单 `WalletPaidAmount` 快照退款，不重新换算） |
| `ResellerProfitAmount` | CNY | 元，2dp | 分销差价快照 |
| `Currency` | CNY | — | 订单币种标记（= Site Currency） |

**关键守恒式（USDT 结算单）：**

```
RemainingOnlineCNY = TotalAmount(CNY) − WalletPaidAmount(USDT) × ExchangeRate
```

- 严禁 `TotalAmount(CNY) − WalletPaidAmount(USDT)` 直接相减（P0 已修复）。
- wallet-only 全额扣款后 `WalletPaidAmount ≈ UsdtTotalAmount`，上式结果 ≈ 0 → 订单由钱包支付成功。

## 2. OrderItem（订单项 `order_items`）

| 字段 | 币种 | 单位/精度 | 含义 |
|---|---|---|---|
| `UnitPrice` | CNY | 元，2dp | 单价快照 |
| `TotalPrice` | CNY | 元，2dp | 行总价快照 |
| `CostPrice` | **CNY** | 元，2dp | 成本价快照（Profit Guard Supplier Cost 来源；前台 `StripCostPrice()` 不下发） |
| `Quantity` | — | 整数 | 数量 |

```
SupplierCost(CNY) = Σ OrderItem.CostPrice × Quantity
```

## 3. Product / ProductSKU

| 字段 | 币种 | 含义 |
|---|---|---|
| `Product.PriceAmount` | CNY | 商品售价（取 SKU 价计算后快照） |
| `Product.CostPriceAmount` | CNY | 商品成本价（= min(活跃 SKU.CostPriceAmount)） |
| `ProductSKU.PriceAmount` | CNY | SKU 售价 |
| `ProductSKU.CostPriceAmount` | CNY | SKU 成本价 |

## 4. Wallet（钱包）

| 字段 | 币种 | 含义 |
|---|---|---|
| `WalletAccount.AvailableBalance` | **USDT** | 可用余额 |
| `WalletAccount.FrozenBalance` | USDT | 冻结余额 |
| `WalletTransaction.Amount` | USDT | 流水金额（订单支付/退款流水均 USDT） |
| `WalletTransaction.Currency` | USDT | 流水币种标记 |

> 充值环节的网关手续费发生在充值时，无法精确归因到单笔订单（见 P6）。

## 5. ExchangeRate（全局汇率）

| 字段 | 含义 |
|---|---|
| `Rate.Rate` | `1 USDT = R CNY` |
| `Rate.Source` | AUTO（CoinGecko 自动）/ MANUAL（人工兜底） |
| `Rate.FetchedAt` | 该汇率值的**产生时间**（AUTO = 拉取时间；MANUAL = Admin 设置时间，禁止填 now） |
| `State.AutoRate / AutoFetchedAt` | 自动汇率及其产生时间 |
| `State.ManualRate / ManualRateUpdatedAt` | 手动兜底值及其写入时间（P4 新增） |

换算方向：

```
USDT(siteAmount) = siteAmount(CNY) / effective_rate      // CEILING 2dp 结算
effective_rate   = market_rate × (1 − buffer_percent/100)
```

## 6. Affiliate（推广佣金）

| 项 | 币种 | 含义 |
|---|---|---|
| 佣金基数 | **USDT** = `order.WalletPaidAmount` | 订单 completed 时沿 inviter_id 向上分佣 |
| 每级费率 `LevelRates[i].Rate` | %（float64 配置存储） | **计算时必须 `decimal.NewFromFloat` 转 decimal，禁止 float64 直接运算** |
| 单级佣金 | USDT = base × rate / 100 | Round 2dp |
| `MaxLevel` | 1~10 | 实际生效层数上限 |
| 约束 | — | Σ(已启用层级费率，不超过 MaxLevel) ≤ 100% |

Profit Guard 事前预估（P5）：

```
max_affiliate_cost(USDT) = UsdtTotalAmount × Σ(enabled rates within MaxLevel) / 100
max_affiliate_cost(CNY)   = max_affiliate_cost(USDT) × ExchangeRate
```

## 7. Payment（支付单 `payments`）

| 字段 | 币种 | 含义 |
|---|---|---|
| `Payment.Amount` | 订单币种或换汇后结算币种 | 应付金额（创建时快照） |
| `Payment.FeeAmount` | 同上 | 渠道手续费快照 |
| `Payment.FeePolicy` | — | `none / customer_surcharge / merchant_absorbed` |
| `Payment.Currency` | CNY / 结算币种 | 发生渠道换汇时为结算币种（`ProviderPayload["exchange_rate"]` 记录快照） |

## 8. Profit Guard 统一口径（P7）

全部换算为 **CNY** 后计算：

```
ExpectedProfitCNY = TotalAmount(CNY)        // Revenue
                  − Σ CostPrice×Qty (CNY)  // Supplier Cost
                  − FeeCostCNY             // 平台承担费（V1 多为 0/标注无法归因）
                  − MaxAffiliateCostCNY    // 见 §6
                  − FXRiskCostCNY          // 见下
RequiredProfitCNY = max(min_profit_amount_cny, TotalAmount × min_profit_rate/100)

ExpectedProfitCNY < RequiredProfitCNY → 拒单 PRODUCT_UNPROFITABLE
```

> **双重计提警告**：`effective_rate` 已含 safety buffer（用户多付 USDT），
> 利润公式中不得再把 buffer 对应的 USDT 差额重复计为 FXRiskCostCNY。
> V1 公式里 `FXRiskCostCNY = 0`（buffer 本身即风险对冲，已体现在 charged_usdt 上）。

---

## 9. 禁止运算（跨币种红线）

| 禁止运算 | 原因 |
|---|---|
| `CNY − USDT` 直接相减 | 币种不同，无意义（P0 已修复 `payment_service_create.go`） |
| `USDT − CNY` 直接相减 | 同上 |
| `TotalAmount(CNY) − WalletPaidAmount(USDT)` | 跨币种直减；wallet-only 单 `online_paid_amount` 恒为 0 |
| 用充值环节 Gateway Rate 反算商品订单 USDT | Gateway Rate 与 Global Rate 隔离 |
| 退款时按当前汇率重新换算退款额 | 必须用原订单快照 |
| 把 `ExchangeRate` / `FeeRate` / `RatePercent` / `buffer` 当金额加减 | 它们是**比率**，不是钱 |

跨币种换算公式（唯一允许路径）：

```
CNY → USDT :  usdt = cny / R        （R = ExchangeRate，1 USDT = R CNY）
USDT → CNY :  cny  = usdt × R
```

## 10. 舍入精度对照（Settlement vs UI）

| 场景 | 取整模式 | 13.880000 | 13.880001 | 13.889 | 13.890001 |
|---|---|---|---|---|---|
| 结算应收（正向 USDT） | **CEIL 2dp** | 13.88 | **13.89** | 13.89 | **13.90** |
| UI 展示 | **ROUND_HALF_UP 2dp** | 13.88 | 13.88 | 13.89 | 13.89 |

> 两者在"非零尾差"处会分叉：CEIL 宁可多收用户 0.01 USDT，UI 仍按四舍五入展示，二者互不影响。

## 11. 退款规则（要点复述）

1. 退款币种恒为 **USDT**，基数 = 原单 `WalletPaidAmount` 快照，**不**用 `TotalAmount(CNY)`。
2. 退款前市场汇率再变动，退款额**不重新换算**，锁定原快照。
3. 防超退：`refundable = WalletPaidAmount − RefundedAmount`，行锁串行化，超额 `ErrRefundExceeded`。
4. 部分退款可多次累计；退款入账 `AvailableBalance(USDT) += amount`。

---

**维护规则**：新增任何金额字段前，必须先在本表登记币种/单位/含义，再写代码。
