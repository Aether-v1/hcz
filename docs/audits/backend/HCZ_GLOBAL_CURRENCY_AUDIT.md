# HCZ Global Site Currency Audit

**审计对象**：Aether-v1/hcz 全站币种能力
**审计日期**：2026-10-03
**审计范围**：Admin 设置页 → 后端 settings schema → 商品/订单/钱包/支付/汇率全链路贯通性
**原则**：不修改业务代码，仅审计现状

---

## 0. 核心结论（先回答问题）

| 问题 | 答案 |
|---|---|
| 当前"全站币种"是否真正贯通后端？ | **部分贯通**。Settings→Public Config→Order 快照→Wallet/Refund 继承 这条链路已通；但 Product 无 currency 字段、Commission 无 currency 字段、前端无货币符号映射、多处 CNY fallback |
| 商品价格是否跟随全站币种？ | **否**。Product 表无 currency 字段，商品价格本身无币种概念；币种在订单创建时从 site_currency 解析并快照到 orders.currency |
| 订单是否已有币种/金额快照？ | **是**。orders.currency 在下单时从 resolveSiteCurrency() 读取并保存；order_items 无独立 currency（继承 order）。历史订单不受后续全站币种修改影响 |
| Wallet 是否可以直接改成 USDT？ | **可以，但需改 3 处**。钱包账户无 currency 字段（单币种隐式），流水有 currency 字段且来自 order.Currency；需改 defaultCurrency 常量、DB 默认值、以及 7 处 CNY fallback。技术上无结构障碍 |
| 现有汇率能力能否复用？ | **不能直接复用**。现有汇率仅为**支付渠道级**（ProviderPayload["exchange_rate"]），用于在线支付网关法币换汇；无全局 USDT/CNY 汇率服务、无订单级汇率快照、无自动获取+手动 fallback 机制 |
| 真正缺失的最小功能是什么？ | **①全局汇率模块（表+service+admin API+前端页）②orders 表加 exchange_rate + usdt_amount 字段 ③下单时汇率快照写入 ④affiliate_commissions 加 currency 字段（或明确隐式 USDT）** |

---

## 1. Admin 前端"全站币种"设置页面

### 1.1 页面位置
- 文件：`frontend/admin/src/views/admin/Settings.vue`
- 区域：品牌设置（Brand）区块，第 927-936 行
- UI 组件：Select 下拉选择器

### 1.2 配置项详情
| 维度 | 值 |
|---|---|
| 配置项名称（前端 form 字段） | `currency` |
| 存储 key | `site_config`（settings 表的一条记录） |
| 存储字段路径 | `site_config` JSON 内的 `currency` 键 |
| 后端常量 | `constants.SettingFieldSiteCurrency = "currency"` |
| 默认值 | `constants.SiteCurrencyDefault = "CNY"` |

### 1.3 支持哪些币种
- **前端**：使用 `Intl.supportedValuesOf('currency')` 动态获取全部 ISO 4217 币种代码（约 170 种），外加 `fallbackCurrencyOptions` 兜底列表
- **后端**：`normalizeSiteCurrency()` 仅校验 `^[A-Z]{3}$` 正则，**不限制具体币种白名单**，任意 3 位大写字母均接受
- **结论**：理论上支持任意 ISO 币种，包括 USDT（虽然 USDT 不是 ISO 4217，但正则只校验 3 位字母，会通过）

### 1.4 API 调用路径
- 不是独立的 currency API，而是**站点配置整体保存**
- 保存：`PUT /api/v1/admin/settings/site`（或等效路径，随 settings 模块统一保存）
- 读取：Admin 设置页加载时从 `GET /api/v1/admin/settings/site` 获取
- 公开下发：`GET /api/v1/public/config` 返回 `currency` 字段

### 1.5 后端对应配置
| 层 | 文件/函数 |
|---|---|
| 读取入口 | `settings/application/general.go:347 GetSiteCurrency(defaultValue string)` |
| 归一化 | `settings/application/scalar_value.go:45 normalizeSiteCurrency(raw)` |
| 存储归一化 | `settings/application/site_normalize.go:45`（写入 site_config 时归一化 currency） |
| 常量 | `constants/constants.go:479 SettingFieldSiteCurrency = "currency"` |
| 默认值常量 | `constants/constants.go:526 SiteCurrencyDefault = "CNY"` |

### 1.6 DB / 缓存存储
- **DB**：`settings` 表，一行记录，`key = "site_config"`，`value` 为 JSON，内含 `currency` 字段
- **缓存**：Public Config 有 Redis 缓存，TTL = 60 秒（`publicConfigCacheTTL = 60 * time.Second`）
- **运行时修改**：**支持**。Admin 保存后立即写 DB，Public Config 最多 60 秒后刷新缓存

### 1.7 Public Site Config 是否下发
- **是**。`settings/transport/http/public/handler.go:130`：
  ```go
  constants.SettingFieldSiteCurrency: constants.SiteCurrencyDefault,  // 默认 CNY
  ```
  然后 `h.settings.GetConfig(defaults)` 合并 DB 中保存的值，最终返回给前端
- 前端用户端通过 `appStore.config.currency` 读取

### 1.8 分类
**已完整实现**（设置→存储→下发→运行时修改）

---

## 2. 商品系统

### 2.1 Product 是否有 currency 字段
- **否**。`internal/modules/catalog/product/domain/product.go` 中 `Product` 结构体：
  - `PriceAmount money.Amount` — 只有金额，无币种
  - `CostPriceAmount money.Amount` — 只有金额，无币种
  - 全结构体无 `Currency` 字段
- `product_skus` 表同样无 currency 字段

### 2.2 是否直接读取全站币种
- **否**。商品 domain / application / store 层均不调用 `GetSiteCurrency`
- 商品价格本身是"裸金额"，币种含义由消费方（订单/前端）解释

### 2.3 是否有写死 CNY/USD
- **商品模块内无写死 CNY/USD**（grep `internal/modules/catalog` 无命中）

### 2.4 前端价格符号/格式是否动态
- **用户端（frontend/user）**：
  - `composables/useProduct.ts:15-18`：`siteCurrency` 从 `appStore.config?.currency` 读取，校验 `^[A-Z]{3}$`，fallback `'CNY'`
  - `formatPrice(amount, currency)`：格式为 `` `${displayAmount} ${cur}` ``（如 "100.00 CNY"）
  - **动态读取全站币种，但无货币符号映射**（不显示 ¥/$/€，只显示币种代码）
- **Admin 端（frontend/admin）**：
  - `utils/format.ts:17-21`：`formatMoney(amount, currency)` = `` `${amount} ${currency}` ``
  - 同样无符号映射，只显示币种代码
- **结论**：价格显示币种是动态的（跟随 public config），但格式为"金额 + 币种代码"，无本地化货币符号

### 2.5 分类
**已有但未贯通**（商品无 currency 字段，币种在订单层解析；前端动态显示但无符号映射）

---

## 3. 订单系统

### 3.1 Order / OrderItem 是否有 currency 字段
- **orders 表：有**。`order/domain/order.go:20`：`Currency string gorm:"not null"`
- **order_items 表：无**。订单项无独立 currency 字段，币种继承自 order

### 3.2 下单后是否保存币种快照
- **是**。完整链路：
  1. `order/application/order_service_validate.go:58`：`currency := resolveSiteCurrency(s.settingService)`
  2. `resolveSiteCurrency`（`order/application/value.go:11`）调用 `settingService.GetSiteCurrency(constants.SiteCurrencyDefault)`
  3. `order_service_validate.go:352`：构建订单时 `Currency: currency`
  4. `order_service.go:417/512/570`：写入 `orders.currency`
- **快照时机**：订单创建时（buildOrderResult 阶段），不是支付时

### 3.3 历史订单是否受全站币种修改影响
- **不受影响**。因为：
  - 订单创建时已将 currency 快照到 `orders.currency`
  - 钱包扣款使用 `order.Currency`（`order_wallet_bridge.go:40`）
  - 退款使用 `order.Currency`（`refund/service.go:486`, `refund/wallet.go:149`）
  - 后续修改全站币种只影响新订单，不影响历史订单
- **这是正确的设计**

### 3.4 订单金额是否有汇率快照
- **否**。orders 表只有 `currency`（站点币种代码），无 `exchange_rate` 字段、无 `usdt_amount` 字段
- 当前架构假设订单币种 = 钱包币种 = 支付币种，无需汇率换算

### 3.5 分类
**已完整实现**（币种快照机制）+ **需要 BUILD**（汇率快照 + USDT 金额字段）

---

## 4. Wallet / Ledger

### 4.1 钱包当前币种模型
| 表 | 有无 currency 字段 | 默认值 |
|---|---|---|
| `wallet_accounts` | **无** | 单币种隐式，账户本身不标记币种 |
| `wallet_transactions` | **有** | `gorm:"default:'CNY'"` |
| `wallet_recharge_orders` | **有** | `gorm:"default:'CNY'"` |

### 4.2 是否固定 CNY
- **不是硬固定，但默认和 fallback 是 CNY**：
  - `wallet/application/service.go:11`：`const defaultCurrency = "CNY"`
  - `normalizeCurrency(currency string)`：空值时返回 `defaultCurrency`（即 CNY）
  - 流水的 currency 来自**调用方输入**，不是钱包服务自己决定
  - 订单支付时传 `order.Currency`（`order_wallet_bridge.go:40`）
  - 管理员加减款时传输入的 currency
- **结论**：钱包不固定 CNY，currency 由调用方传入；但所有 fallback 和 DB 默认值是 CNY

### 4.3 是否支持 USDT
- **技术上可以**，因为：
  - `wallet_transactions.currency` 是 `varchar(16)`，无 CHECK 约束
  - `normalizeCurrency` 只做 `strings.ToUpper(strings.TrimSpace())`，非空即接受
  - 钱包账户无币种字段，不存在"账户币种不匹配"问题
- **但需要修改**：
  1. `defaultCurrency` 常量从 `"CNY"` 改为 `"USDT"`
  2. `wallet_transactions` 表 `currency` 列默认值从 `'CNY'` 改为 `'USDT'`
  3. `wallet_recharge_orders` 表 `currency` 列默认值改为 `'USDT'`
  4. 7 处生产代码中的 CNY fallback（见第 7 节）

### 4.4 Ledger 是否记录 currency
- **是**。`wallet_transactions.currency` 记录每笔流水的币种
- 这就是账本（ledger），无独立 ledger 模块

### 4.5 Refund / Commission 是否继承币种
- **Refund：是**。
  - `refund/service.go:486`：`currency := strings.ToUpper(strings.TrimSpace(order.Currency))`，空则 fallback CNY
  - `refund/wallet.go:149`：同样从 `order.Currency` 读取
  - 退款记录 `order_refund_records.currency` 保存
  - 退款钱包流水使用 `order.Currency`
- **Commission：否（缺口）**。
  - `affiliate/domain/commission.go` 中 `Commission` 结构体**无 currency 字段**
  - 佣金金额 `BaseAmount` / `CommissionAmount` 是 `money.Amount`，无币种列
  - 佣金隐式跟随订单币种，但无显式快照
  - **这是 HCZ 需注意的缺口**：当钱包改为 USDT 时，佣金也应为 USDT，但表结构无法表达

### 4.6 分类
**可直接复用**（钱包流水 currency 机制）+ **需要 MODIFY**（默认值 CNY→USDT、fallback 清理）+ **需要 BUILD**（commission 加 currency 字段或明确隐式约定）

---

## 5. Payment / Exchange Rate

### 5.1 是否已经存在汇率模块
- **否，无全局汇率模块**。grep `internal` 无 `exchange_rate` 表、无 `ExchangeRateService`、无独立汇率 domain

### 5.2 是否只有支付渠道级汇率
- **是**。现有汇率能力仅限于支付渠道级：
  - `payment/application/payment_service_rules.go:39 paymentExchangeRate(payment)`
  - 读取 `payment.ProviderPayload["exchange_rate"]`
  - 这是支付创建时存在渠道配置 JSON 中的汇率，用于在线支付网关（如 Stripe）的法币换汇
  - 仅在支付回调时用于将结算币种金额换算回订单币种（`paymentCoveredOrderAmount`）

### 5.3 是否已有 currency conversion
- **有，但仅在支付网关适配器层面**：
  - Stripe / PayPal 等渠道可配置 `target_currency` + `exchange_rate`
  - 适配器在创建支付时按汇率将订单币种金额换算为渠道结算币种
  - 换算后回写 `payment.Amount` / `payment.Currency`
  - 汇率快照存入 `ProviderPayload["exchange_rate"]`
- **这不是全局汇率服务**，是每个支付渠道各自配置的静态汇率

### 5.4 是否已有 rate snapshot
- **支付级：有**（`ProviderPayload["exchange_rate"]`）
- **订单级：无**（orders 表无 exchange_rate 字段）
- **钱包级：无**（wallet_transactions 无 exchange_rate 字段）

### 5.5 是否存在 USDT 相关能力
- **有，但仅限于加密货币支付网关**：
  - `payment/infrastructure/gateway/bepusdt/` — BEPUSDT（币安 USDT）支付网关
  - `payment/infrastructure/gateway/epusdt/` — EPUSDT 支付网关
  - `payment/infrastructure/gateway/tokenpay/` — TokenPay，`DefaultCurrency = "USDT"`
  - `payment/infrastructure/gateway/okpay/` — OKPay，支持 `case "USDT", "TRX"`
- **这些是支付渠道**，不是钱包币种、不是全局汇率
- HCZ v1 不接在线支付，这些网关全部停用

### 5.6 分类
**需要 BUILD**（全局汇率模块完全缺失）+ **DROP**（渠道级汇率 v1 停用，不可复用为全局汇率）

---

## 6. 全局搜索结果分类

### 6.1 搜索关键词覆盖
`CNY` / `USD` / `USDT` / `currency` / `currency_symbol` / `site_currency` / `default_currency` / `exchange_rate` / `rate` / `money format`

### 6.2 已完整实现
| 能力 | 证据 |
|---|---|
| 全站币种设置页 | `Settings.vue:927-936`，Select + Intl.supportedValuesOf |
| 站点币种存储 | `settings` 表 `site_config.currency`，`GetSiteCurrency()` |
| 站点币种公开下发 | `public/handler.go:130`，`/api/v1/public/config` 返回 currency |
| 运行时修改 | Admin 保存即写 DB，public config 60s 缓存刷新 |
| 订单币种快照 | `order_service_validate.go:58` → `orders.currency` |
| 历史订单币种隔离 | 钱包/退款均读 `order.Currency`，不读全站币种 |
| 钱包流水币种记录 | `wallet_transactions.currency` |
| 前端动态币种显示 | `useProduct.ts:siteCurrency` + `formatPrice` |

### 6.3 已有但未贯通
| 能力 | 现状 | 缺口 |
|---|---|---|
| 商品价格币种 | Product 无 currency 字段 | 币种由订单层解释，商品本身无币种元数据 |
| 钱包币种 | 流水有 currency，账户无 currency | 单币种隐式，改 USDT 需改默认值和 fallback |
| 佣金币种 | Commission 无 currency 字段 | 隐式跟随订单，无显式快照 |
| 前端货币符号 | 动态显示币种代码 | 无 ¥/$/€ 符号映射，无本地化格式 |
| 支付渠道级汇率 | ProviderPayload["exchange_rate"] | 仅支付级，非全局；无自动获取+手动 fallback |

### 6.4 仅前端 UI
| 能力 | 说明 |
|---|---|
| Admin 币种选择器 | UI 完整，但后端只存 3 位字母代码，无币种元数据（符号、小数位、名称） |
| 用户端 formatPrice | 动态读取 currency，但格式固定为 "金额 币种代码" |

### 6.5 写死逻辑（生产代码，非测试）
| # | 文件 | 行 | 代码 | 性质 |
|---|---|---|---|---|
| 1 | `constants/constants.go` | 526 | `SiteCurrencyDefault = "CNY"` | 默认值常量 |
| 2 | `wallet/application/service.go` | 11 | `const defaultCurrency = "CNY"` | 钱包 normalize fallback |
| 3 | `wallet/transport/http/channel_handler.go` | 74 | `"currency": "CNY"` | 渠道 handler 响应 |
| 4 | `order/application/refund/wallet.go` | 151 | `currency = "CNY"` | refund wallet fallback（order.Currency 空时） |
| 5 | `order/application/refund/service.go` | 488 | `currency = "CNY"` | refund service fallback |
| 6 | `payment/application/payment_service_create.go` | 239 | `payment.Currency = "CNY"` | payment create fallback |
| 7 | `payment/application/payment_service_recharge.go` | 101,107 | `currency = "CNY"` | recharge fallback |
| 8 | `payment/infrastructure/gateway/adapters/alipay/adapter.go` | 192 | `Currency: "CNY"` | 支付宝网关（v1 DROP） |
| 9 | `payment/infrastructure/gateway/adapters/epay/adapter.go` | 204 | `Currency: "CNY"` | EPay 网关（v1 DROP） |
| 10 | `payment/infrastructure/gateway/adapters/bepusdt/adapter.go` | 304 | `currency = "CNY"` | BEPUSDT 法币默认（v1 DROP） |
| 11 | `channelapi/transport/http/channel_catalog.go` | 153,156,274,277 | `GetSiteCurrency("CNY")` / `currency = "CNY"` | 渠道 API（v1 DROP） |
| 12 | `upstreamapi/transport/http/` | 38,57,179 | `GetSiteCurrency("CNY")` | 上游 API（v1 DROP） |

**核心业务写死 CNY 共 7 处（#1-#7）**，其中 #1/#2 是默认值常量，#3-#7 是 fallback（当主路径 currency 为空时才触发）。
**支付网关/渠道/上游写死 CNY 共 5 处（#8-#12）**，HCZ v1 全部停用。

### 6.6 可直接复用
| 能力 | 复用方式 |
|---|---|
| `GetSiteCurrency()` | 全局汇率模块可调用获取站点币种（CNY） |
| `normalizeSiteCurrency()` | 币种代码校验 |
| `IsCurrencyCode()` | 币种代码校验 |
| 订单币种快照机制 | 下单时快照 currency 的模式可扩展为同时快照 exchange_rate |
| 钱包流水 currency 字段 | 改为 USDT 后直接记录 USDT |
| Public Config 下发机制 | 汇率可通过 public config 下发给前端（或独立接口） |
| Admin Settings 保存机制 | 汇率设置页可复用 settings 保存框架 |

### 6.7 需要 MODIFY
| 项 | 修改内容 |
|---|---|
| `SiteCurrencyDefault` | 保持 CNY（HCZ 商品定价仍为 CNY），不需改 |
| `wallet defaultCurrency` | CNY → USDT |
| `wallet_transactions.currency` DB 默认值 | CNY → USDT |
| `wallet_recharge_orders.currency` DB 默认值 | CNY → USDT |
| 7 处 CNY fallback | 主路径确保 currency 非空，fallback 改为 USDT 或移除 |
| 前端 formatPrice / formatMoney | 可选：增加货币符号映射（CNY→¥, USDT→₮） |
| `affiliate_commissions` | 加 currency 字段，或在 HCZ 中明确隐式为 USDT |

### 6.8 需要 BUILD
| 项 | 说明 |
|---|---|
| **全局汇率模块** | `exchange_rates` 表（current_rate / manual_fallback_rate / source / updated_at）+ service（自动获取 USDT/CNY + 手动 fallback 优先）+ Admin API |
| **Admin 汇率设置页** | 显示当前汇率、手动 fallback 输入、来源/更新时间 |
| **orders 表加字段** | `exchange_rate`（decimal 10,6）、`usdt_total_amount`（decimal 20,2） |
| **下单时汇率快照** | 创建订单时从汇率服务获取当前汇率，写入 orders.exchange_rate，计算 usdt_total_amount |
| **钱包扣款使用 usdt_total_amount** | 订单钱包支付时扣除 USDT 金额（而非 CNY 金额） |

---

## 7. 全链路贯通性验证

### 7.1 当前链路（CNY 单币种）
```
Admin 设置 currency=CNY
  → settings 表 site_config.currency="CNY"
  → /api/v1/public/config 返回 currency="CNY"
  → 前端 formatPrice 显示 "100.00 CNY"
  → 用户下单
  → order_service_validate.go: resolveSiteCurrency() → "CNY"
  → orders.currency="CNY"（快照）
  → 钱包支付：order_wallet_bridge 传 order.Currency="CNY"
  → wallet_transactions.currency="CNY"
  → 退款：refund/service 读 order.Currency="CNY"
  → order_refund_records.currency="CNY"
  → 钱包退款流水 currency="CNY"
```
**结论：当前 CNY 单币种链路完全贯通。**

### 7.2 HCZ 目标链路（CNY 展示 + USDT 结算）
```
Admin 设置 currency=CNY（商品定价币种）
Admin 设置 exchange_rate=7.25（USDT/CNY，手动 fallback 或自动获取）
  → 前端商品显示 "100.00 ¥"（CNY）
  → 用户下单
  → 后端：商品 CNY 总价 = 100
  → 后端：从汇率服务获取当前汇率 = 7.25（快照）
  → 后端：USDT 扣款 = 100 / 7.25 = 13.79 USDT
  → orders.currency="CNY", orders.exchange_rate=7.25, orders.usdt_total_amount=13.79
  → 钱包支付：扣除 13.79 USDT
  → wallet_transactions.currency="USDT", amount=13.79
  → 退款：退 13.79 USDT（原扣多少退多少，不重新换算）
  → 佣金：13.79 × 比例 = X USDT
```
**缺口：**
1. ❌ 全局汇率服务（无）
2. ❌ orders.exchange_rate / usdt_total_amount 字段（无）
3. ❌ 下单时汇率快照写入（无）
4. ❌ 钱包扣款使用 usdt_total_amount（当前用 order.TotalAmount，即 CNY 金额）
5. ⚠️ affiliate_commissions 无 currency 字段（隐式，需明确）
6. ⚠️ 前端无货币符号映射（可选增强）

---

## 8. HCZ 最小缺失功能清单（按优先级）

### P0（上线必须）
| # | 功能 | 类型 | 工作量 |
|---|---|---|---|
| 1 | 全局汇率模块：`exchange_rates` 表 + service（自动获取+手动 fallback）+ Admin API | BUILD | 中 |
| 2 | Admin 汇率设置页 | BUILD | 小 |
| 3 | `orders` 表加 `exchange_rate` + `usdt_total_amount` 字段 | BUILD | 小 |
| 4 | 下单时汇率快照写入 + USDT 金额计算 | MODIFY | 中 |
| 5 | 钱包扣款使用 `usdt_total_amount`（而非 CNY total） | MODIFY | 小 |
| 6 | Wallet 默认币种 CNY→USDT（常量 + DB 默认值 + 7 处 fallback） | MODIFY | 小 |

### P1（上线前建议）
| # | 功能 | 类型 |
|---|---|---|
| 7 | `affiliate_commissions` 加 `currency` 字段（或代码层明确隐式 USDT 并加注释） | MODIFY |
| 8 | 前端货币符号映射（CNY→¥, USDT→₮）+ 本地化金额格式 | MODIFY |
| 9 | `wallet_transactions` 加 `exchange_rate` 字段（可选，用于对账） | BUILD |

### P2（后续版本）
| # | 功能 | 类型 |
|---|---|---|
| 10 | 商品表加 `currency` 字段（支持多币种商品定价） | BUILD |
| 11 | 钱包多币种账户（每个用户多个币种账户） | BUILD |
| 12 | 币种元数据表（符号、小数位、名称） | BUILD |
| 13 | 在线支付渠道 USDT 充值（BEPUSDT/EPUSDT 等） | MODIFY（复用现有网关） |

---

## 9. 最终回答

### Q1：当前"全站币种"是不是已经真正贯通后端？
**部分贯通。** Settings→Public Config→Order 快照→Wallet/Refund 继承 这条 CNY 单币种链路已完整贯通。但"贯通"仅限于币种代码的传递，不包含汇率换算、双币种结算、商品级币种元数据。HCZ 需要的 CNY 展示 + USDT 结算双轨制，当前架构不支持。

### Q2：商品价格是否已经跟随全站币种？
**否。** Product 表无 currency 字段，商品价格是"裸金额"，币种由订单层在创建时从 site_currency 解析。前端显示时跟随 public config 的 currency，但这只是显示层的解释，不是商品数据层的币种属性。

### Q3：订单是否已有币种/金额快照？
**是（币种），否（汇率/USDT 金额）。** `orders.currency` 在下单时从 `resolveSiteCurrency()` 读取并快照，历史订单不受后续全站币种修改影响。但 orders 表无 `exchange_rate` 字段、无 `usdt_total_amount` 字段，无法表达双币种结算。

### Q4：Wallet 是否可以直接改成 USDT？
**可以，技术上无结构障碍，但需改 6 处。** 钱包账户无 currency 字段（单币种隐式），流水 currency 来自调用方输入。需改：①`defaultCurrency` 常量 ②`wallet_transactions.currency` DB 默认值 ③`wallet_recharge_orders.currency` DB 默认值 ④7 处 CNY fallback ⑤订单钱包扣款使用 usdt_total_amount ⑥充值订单币种。无表结构变更（全新部署），存量数据需折算。

### Q5：现有汇率能力能否复用？
**不能直接复用。** 现有汇率仅为支付渠道级（`ProviderPayload["exchange_rate"]`），用于在线支付网关法币换汇，是每个渠道各自配置的静态汇率。无全局汇率服务、无自动获取、无手动 fallback 优先级、无订单级汇率快照。HCZ 需从零构建全局汇率模块。

### Q6：真正缺失的最小功能是什么？
**4 项 P0：**
1. **全局汇率模块**（表 + service + Admin API + 前端页）—— 完全缺失
2. **orders 表加 `exchange_rate` + `usdt_total_amount` 字段** —— 完全缺失
3. **下单时汇率快照写入 + USDT 金额后端计算** —— 完全缺失
4. **Wallet 默认币种 CNY→USDT + 扣款使用 usdt_total_amount** —— 需 MODIFY

**审计完成，等待下一阶段开发指令。**
