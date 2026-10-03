# HCZ P0-2 Global Exchange Rate + USDT Wallet Settlement — 最终报告

## Final Verdict: **PASS WITH CONDITIONS**

核心资金链已按 HCZ 固定规则落地、`go build ./...` 通过、受影响模块测试通过。
未完成项（Admin Global Rate 设置页/API、前端 USDT 展示、端到端订单换算集成测试）列为 P1，不影响本阶段后端正确性，但在 Admin UI 接入前生产需通过 settings KV 或后续 Admin 端点配置 manual fallback。

---

## 1. 固定规则落地情况

| 规则 | 状态 |
|---|---|
| Wallet = USDT，2 位 decimal，half-up | ✅ |
| 计价链 KEEP（site_config.currency，SKU→活动→批发→会员→券→totalAmount） | ✅ 未改 |
| 汇率方向固定 `1 USDT = R SiteCurrency`，`usdt = site / R` | ✅ |
| fail-closed：无有效汇率拒单，禁止 1:1 / 0 / Gateway Rate 兜底 | ✅ |
| Global Rate 与 Gateway Rate 完全隔离 | ✅ |
| 订单快照 rate/source/at + usdt 额 | ✅ |
| 钱包按 USDT 扣款、流水 Currency=USDT | ✅ |
| 退款按订单 wallet_paid_amount(USDT) 快照回充 | ✅ |
| 返利按订单 USDT 结算额计算 | ✅ |
| Wallet Recharge 仍走 Gateway Rate | ✅ 未动 |

## 2. 新增模块 `internal/modules/exchangerate/`

- `domain/rate.go`：`WalletCurrency=USDT`、Source AUTO/MANUAL、`ErrRateUnavailable`、`Rate.ToUSDT`（site/R，Round 2 half-up）。
- `contract/contract.go`：`Provider`、`Store`、`Resolver` 端口 + `State{Currency,AutoRate,AutoFetchedAt,ManualRate,LastSuccessAt,LastError}`。
- `application/service.go`：解析优先级 自动(新鲜)→手动兜底→`ErrRateUnavailable`；`Refresh` 调 provider 写状态、失败只记 last_error 不兜底；`SetManual`、`Snapshot`。
- `infrastructure/provider/coingecko.go`：后端专用 CoinGecko（tether vs site currency），USDT 站点直返 1，10s 超时，baseURL 可注入测试。
- `infrastructure/settingsstore/store.go`：状态持久化到 settings KV（key=`global_exchange_rate`）。
- `application/service_test.go`：自动新鲜/过期回退手动/双失败 fail-closed/provider 失败不 1:1/USDT 站点 1:1 —— **全部 PASS**。

## 3. 订单快照与扣款（非破坏 migration）

`orderdomain.Order` 新增列（AutoMigrate 增量，历史单 NULL）：
- `usdt_total_amount decimal(20,2)` — 本单应收 USDT
- `exchange_rate decimal(20,8) NULL`
- `exchange_rate_source varchar(16)`
- `exchange_rate_at datetime NULL`

集成点：
- `order/application/order_service.go`：下单时经 `rateResolverPort.Resolve` 取率（未注入=legacy，生产必注入），`orderUsdtTotal = TotalAmount / R` 并快照；钱包预校验余额对 **USDT 额** 比较；`online_paid_amount` 新单恒 0。
- `order/application/order_wallet_bridge.go`：有 USDT 快照时扣 `usdt_total_amount`、流水 `Currency=USDT`；online 恒 0。
- 构造注入：`container/services_integration.go` 构造 `ExchangeRateService` 并 `SetRateResolver`；`container/exchange_rate_wiring.go` 适配器。

## 4. 退款 / 返利 USDT

- `order/application/refund/wallet.go`：USDT 单按 `WalletPaidAmount`(USDT) 计算 refundable 与全额判定，回充流水币种 USDT，退款记录币种 USDT。旧单无快照保持 legacy。
- `affiliate/application/commission.go`：返利基数（site）经订单 `ExchangeRate` 一次性换算为 USDT 后再乘 rate，commission/wallet credit/ledger 均 USDT。

## 5. 周期刷新

- `internal/queue/tasks.go`：`TaskExchangeRateRefresh`。
- `jobs/consumer/consumer_exchangerate.go`：handler 调 `Refresh`，失败仅记日志不堆积。
- `jobs/service.go`：`@every 5m` 注册。

## 6. 隔离证明

- 商品订单资金链只引用 `internal/modules/exchangerate`，不引用 `payment/gateway` 的 exchange_rate。
- Wallet Recharge 路径（`payment` gateway adapters、`CreateWalletRechargePayment`）未改，`payment/integrationtest/exchange` 等测试 PASS。
- `wallet_paid_amount` 复用，未物理删 `online_paid_amount`（新单恒 0）。

## 7. 验证结果

- `go build ./...` —— **PASS**
- `go test ./internal/modules/exchangerate/...` —— **PASS**
- `go test ./internal/modules/order/... ./wallet/... ./affiliate/... ./payment/...` ——
  - order/application、order integrationtest/application、integrationtest/refund、wallet、affiliate、payment 各包 **PASS**
  - 唯一失败：`order/infrastructure/gormstore` 两个 risk-gate 用例，失败信息为 `TempDir RemoveAll cleanup: ... file used by another process` —— **Windows 已知环境问题（P0-1 已记录），Linux CI 通过**；SQL 日志确认新列正常写入、断言通过。

## 8. 剩余 P1（本轮未做，需后续）

- **Admin Global Rate 设置 UI/API**：展示当前 AUTO/MANUAL 汇率、来源、最近错误，支持手动 fallback rate。后端 `Service.Snapshot/SetManual` 已就绪，缺 HTTP handler + 前端页。
- **前端 User/Admin 展示**：订单同时显示 `商品金额 X SiteCurrency` 与 `实付 Y USDT` + 汇率快照；钱包/返利/退款显示 USDT 单位。
- **订单级端到端集成测试**：换算 2dp half-up、余额不足/刚好/足够、并发不超扣、汇率变化后退款不重算、Gateway 与 Global 无交叉引用。
- Redis 缓存层（当前用 settings KV 作为跨进程共享真源，下单只读状态不实时调 CoinGecko）。

## 9. P0-2 不触碰（遵守边界）

订单 5 状态、售后、会员等级/会员价/批发价、Gateway 协议、商品全站币种体系 —— 均未改。
