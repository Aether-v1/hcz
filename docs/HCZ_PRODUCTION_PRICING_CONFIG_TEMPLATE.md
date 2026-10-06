# HCZ 生产环境定价配置模板（Profit Guard + 全局汇率）

> 状态：**模板 / Template**。本文档不预设任何生产最终值。
> 所有生产值一律填写 `ADMIN_CONFIRM_REQUIRED`，由 ADMIN 上线前在 Admin 后台逐项确认。
> 字段名与类型均来自后端真实 Contract，不得自行增删。

- 生成日期：2026-10-06
- 配置存储：settings KV（运行时可热更新，无需重启进程）
- 关联代码：
  - `internal/modules/settings/schema/integration/profit_guard.go`（ProfitGuardSetting）
  - `internal/modules/exchangerate/contract/contract.go`（ExchangeRate State）
  - `internal/modules/exchangerate/infrastructure/settingsstore/store.go`（KV 持久化）
  - `internal/constants/constants.go`（`SettingKeyProfitGuardConfig = "profit_guard_config"`）

---

## 0. 总体状态

| 项目 | 值 |
|---|---|
| Profit Guard 总开关 | ADMIN_CONFIRM_REQUIRED |
| 汇率自动刷新（AutoEnabled） | ADMIN_CONFIRM_REQUIRED |
| 管理员最终确认签字 | ADMIN_CONFIRM_REQUIRED |
| 确认日期 | ADMIN_CONFIRM_REQUIRED |

---

## 1. 配置加载顺序与热更新说明

### 1.1 存储位置

| 配置块 | settings KV Key | 持久化结构 | 备注 |
|---|---|---|---|
| Profit Guard | `profit_guard_config` | JSON：`enabled / require_cost_price / rate_safety_buffer_percent / minimum_profit_amount_cny / minimum_profit_rate_percent` | 注册于 `settings/application/default_registry.go`，写入时经 `NormalizeProfitGuardSettingJSON` 归一化 |
| 全局汇率 | `global_exchange_rate` | JSON：`currency / auto_rate / manual_rate / auto_enabled / provider / api_key / refresh_interval_min / rate_safety_buffer_percent / max_auto_rate_age_minutes / ...` | 注意：此为 **USDT↔站点币种全局汇率**，与 Payment Gateway 自有 `exchange_rate`（钱包充值换算）完全隔离，二者不得混用 |

### 1.2 加载链路

1. Admin 在后台保存配置 → 写入 settings KV（MySQL）。
2. Profit Guard：订单核算链路每次经 `typed_io.go` 读取 `profit_guard_config`，写入即生效，**无需重启**。
3. 全局汇率：除 KV 外另有 Redis 缓存 key `global_exchange_rate:state`（TTL 约 2 分钟，写穿）。KV 更新后缓存最多 2 分钟自然过期；如需立即生效，可主动清缓存：
   ```bash
   redis-cli -n 0 DEL "global_exchange_rate:state"
   ```
4. 归一化兜底：`DecodeProfitGuardSetting` 在字段缺失时用 `DefaultProfitGuardSetting()` 补全；数值越界自动 clamp（buffer 限制在 0~5，负数利润门槛归 0）。

### 1.3 热更新生效范围

- 改配置只影响**保存之后新建的订单核算**；已生成订单的汇率快照（`orders.exchange_rate` / `exchange_rate_source` / `exchange_rate_at`）**永不回算**。
- 关闭 Profit Guard 不影响在途订单（详见第 4 节回滚方案）。

---

## 2. Profit Guard 配置项（KV: `profit_guard_config`）

### 2.1 Profit Guard Enabled

| 项 | 内容 |
|---|---|
| 字段名 | `enabled` |
| 类型 | bool |
| 说明 | Profit Guard 总开关。开启后，下单/核算时执行成本门 + 最低利润门；预计利润不足或成本缺失时拒单（guard_reason：`product_unprofitable` / `product_cost_not_configured` / `exchange_rate_unavailable`） |
| 安全默认值（代码） | `false`（关闭，不改变现有行为） |
| 生产值 | ADMIN_CONFIRM_REQUIRED |
| 生效范围/影响 | 全局：所有走订单核算链路的下单（含 C2C listing 下单）。仅作用于新订单核算，不追溯历史订单 |
| 开启条件/注意事项 | 必须先完成：① 全部 active 商品成本价录入（见清单 COST_MISSING=0）；② RequireCostPrice 已决策；③ 最低利润门槛已测算；④ Pricing Preview 后台预览验证通过。确认无误后再置 true |

### 2.2 Require Cost Price

| 项 | 内容 |
|---|---|
| 字段名 | `require_cost_price` |
| 类型 | bool |
| 说明 | 成本强制门：为 true 时，订单中存在「正式商品 `cost_price_amount <= 0` 且 `is_cost_exempt = false`」→ 直接拒单（`product_cost_not_configured`）。`is_cost_exempt = true` 的真零成本商品（数字权益/赠品/内测）自动豁免 |
| 安全默认值（代码） | `false`（cost=0 商品不拒单） |
| 生产值 | ADMIN_CONFIRM_REQUIRED |
| 生效范围/影响 | 仅当 `enabled = true` 时生效。开启后所有 active 正式商品必须有真实成本或被显式豁免，否则无法成交 |
| 开启条件/注意事项 | 开启前必须确认 `SELECT COUNT(*) FROM products WHERE is_active = true AND deleted_at IS NULL AND cost_price_amount <= 0 AND is_cost_exempt = false` 结果为 0。新接入商品若成本暂未回填，应先录成本，不要靠关闭此门放行 |

### 2.3 Minimum Profit Amount CNY

| 项 | 内容 |
|---|---|
| 字段名 | `minimum_profit_amount_cny` |
| 类型 | float（单位 CNY） |
| 说明 | 单笔订单固定最低利润门槛（CNY）。RequiredProfit = max(本值, TotalAmount × minimum_profit_rate_percent/100)；ExpectedProfit < RequiredProfit → 拒单（`product_unprofitable`）。负值自动归 0 |
| 安全默认值（代码） | `0`（不设固定门槛） |
| 生产值 | ADMIN_CONFIRM_REQUIRED |
| 生效范围/影响 | 全局新订单核算。小金额订单主要受本值约束（比例门槛对小额订单不敏感） |
| 开启条件/注意事项 | 需 ADMIN 结合网关手续费、渠道成本、期望单笔毛利测算后填写。建议先用 Pricing Preview 跑一组真实商品验证拒单边界，确认无误再上线 |

### 2.4 Minimum Profit Rate Percent

| 项 | 内容 |
|---|---|
| 字段名 | `minimum_profit_rate_percent` |
| 类型 | float（单位 %） |
| 说明 | 单笔订单最低利润率门槛（%）。与 2.3 取最大值生效。负值自动归 0 |
| 安全默认值（代码） | `0`（不设比例门槛） |
| 生产值 | ADMIN_CONFIRM_REQUIRED |
| 生效范围/影响 | 全局新订单核算。大金额订单主要受本值约束 |
| 开启条件/注意事项 | 与 2.3 联合测算；建议从小门槛灰度开启，观察 Pricing Preview 拒单率后再收紧 |

---

## 3. 全局汇率配置项（KV: `global_exchange_rate`）

> 语义：1 USDT = X 站点币种（本项目站点币种为 CNY）。

### 3.1 Rate Safety Buffer Percent

| 项 | 内容 |
|---|---|
| 字段名 | `rate_safety_buffer_percent`（State 字段 `RateSafetyBufferPercent`） |
| 类型 | float（单位 %，范围 **0–5**，代码 clamp 到该区间） |
| 说明 | 汇率安全缓冲：effective_rate = market_rate × (1 − buffer/100)。即核算时故意低估 USDT 兑 CNY 的折算，预留汇率波动/滑点缓冲，防止按市价核算为正、实际结算亏损 |
| 安全默认值（代码） | `1.0`（即缓冲 1%；profit_guard.go 注释同步说明 0~5） |
| 生产值 | ADMIN_CONFIRM_REQUIRED |
| 生效范围/影响 | 影响所有走全局汇率的订单核算利润测算。buffer 越高，越容易触发 `product_unprofitable` 拒单；过低则失去缓冲意义 |
| 开启条件/注意事项 | 允许范围 0~5%，超出会被归一化截断。建议 ADMIN 根据 USDT/CNY 波动历史与渠道到账时差决定，默认沿用 1.0 起步并观察 |

### 3.2 Max Auto Rate Age Minutes

| 项 | 内容 |
|---|---|
| 字段名 | `max_auto_rate_age_minutes`（State 字段 `MaxAutoRateAgeMinutes`） |
| 类型 | int（单位：分钟） |
| 说明 | AUTO 汇率允许的最大新鲜度。超过该分钟数未成功刷新，AUTO 视为 stale，Resolver 回退 MANUAL；若 MANUAL 也未设置，则 fail-closed 返回 `exchange_rate_unavailable`（拒单，不按旧汇率强行成交） |
| 安全默认值（代码） | 代码无内置兜底默认，未配置时为 0（**即视为不可用**）→ 上线前必须显式设置 |
| 生产值 | ADMIN_CONFIRM_REQUIRED |
| 生效范围/影响 | 决定 CoinGecko 拉取失败后多久切手动兜底。过短会频繁切 MANUAL，过长则用陈旧汇率核算导致利润失真 |
| 开启条件/注意事项 | 需与 `refresh_interval_min` 联动：建议 > 刷新间隔的 2~3 倍，容忍偶发拉取失败。必须同时配置好 Manual Fallback（3.3），否则 stale 后直接拒单 |

### 3.3 Manual Fallback Rate

| 项 | 内容 |
|---|---|
| 字段名 | `manual_rate`（State 字段 `ManualRate`，decimal，字符串持久化） |
| 类型 | decimal（语义：**1 USDT = X CNY**） |
| 说明 | 手动兜底汇率。AUTO 未开启 / 拉取失败 / 超过 MaxAutoRateAge 时，Resolver 使用本值；`manual_rate <= 0` 表示未设置。写入时同步记录 `manual_rate_updated_at`（Resolver 的 MANUAL 分支回显该时间戳，禁止填 now） |
| 安全默认值（代码） | 未设置（decimal.Zero，即不兜底） |
| 生产值 | ADMIN_CONFIRM_REQUIRED |
| 生效范围/影响 | AUTO 异常期间所有订单核算使用该值。设置偏高会高估 USDT 折算（可能放出实际亏损单），偏低会收紧拒单。该值与 `manual_rate_updated_at` 一起快照进订单 |
| 开启条件/注意事项 | 必须在上线前由 ADMIN 按当时市场汇率显式写入一个正值（如 1 USDT = 7.xx CNY），并定期人工复核更新。注意与 Payment Gateway 自身的 `exchange_rate`（充值换算）区分，两者独立 |

---

## 4. 回滚方案

### 4.1 紧急关闭 Profit Guard（不影响在途订单）

在 Admin 后台将 `profit_guard_config.enabled` 置为 `false`，保存即生效（热更新，秒级）：

- 效果：之后新订单不再执行成本门/利润门，拒单链路关闭；
- **不影响在途订单**：已创建订单持有自己的汇率快照与核算结果，关闭总开关不会回算、不会改动任何已冻结/已结算资金；
- 可选联动：若同时怀疑成本数据不准，可再将 `require_cost_price` 置 `false`。
- 回滚后补刀：事后必须复盘被放过的订单利润，人工对账，确认无亏损单。

### 4.2 汇率异常回滚

- 若 AUTO 源故障导致频繁拒单（`exchange_rate_unavailable`）：确认并调高 `manual_rate`（手动兜底），或临时将 `auto_enabled` 置 false 强制走 MANUAL。
- 若怀疑缓存脏数据：`redis-cli -n 0 DEL "global_exchange_rate:state"` 强制重加载。
- 恢复 AUTO 后观察日志关键字 `exchange_rate_refresh_ok` / `exchange_rate_refresh_failed`（`internal/app/jobs/consumer/consumer_exchangerate.go`）。

### 4.3 配置一致性回退

- settings KV 每次写入前建议导出当前值备份；误操作时按备份 JSON 回填即可。
- Profit Guard 配置被归一化后落库（buffer clamp 0~5、负数门槛归 0），即使误填越界值也会被收敛到安全区间，不会产生非法配置。

---

## 5. 上线前管理员确认清单（签字栏）

- [ ] 已逐项核对上述 7 个配置项，生产值不再是 `ADMIN_CONFIRM_REQUIRED`
- [ ] Pricing Preview 对代表性商品（正利润/临界利润/成本缺失/豁免商品）验证通过
- [ ] Manual Fallback Rate 已写入正值且 `manual_rate_updated_at` 已刷新
- [ ] 回滚方案已演练（关闭 enabled、切换 MANUAL、清 Redis 缓存）

| 角色 | 签字 | 日期 |
|---|---|---|
| ADMIN | ADMIN_CONFIRM_REQUIRED | ADMIN_CONFIRM_REQUIRED |
