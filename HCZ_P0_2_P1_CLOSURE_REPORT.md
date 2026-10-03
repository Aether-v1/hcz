# HCZ P0-2 P1 Closure 报告

## Final Verdict: **PASS WITH CONDITIONS**

P0-2 核心资金模型在 P0-2 阶段已完成；本阶段 P1 收口补齐了**可配置、可管理、可验证**的后端闭环。
前端展示页（Admin 汇率设置页、User/Admin USDT 渲染）、Redis 读穿缓存、完整 E2E 矩阵列为 P2，已在末尾列明。

---

## 一、已完成

### 1. 汇率配置 DB 化（不写死源码/环境变量）
- `contract.State` 扩展：`Provider / APIKey / AutoEnabled / RefreshIntervalMin`。
- `settingsstore` 持久化到 settings KV（key=`global_exchange_rate`），非破坏。
- CoinGecko provider 从 settings 读 key；`Refresh` 遵守 `AutoEnabled`，关闭时不拉源、不覆盖合法自动率。
- 空 api_key 更新时**保留现有 key**（不误清空）；错误信息只写 `provider_fetch_failed`，**不含 key**。

### 2. Admin Global Rate HTTP API
挂在现有 admin JWT/RBAC 组下（`routes_admin.go`），不新增旁路认证：
- `GET  /admin/settings/exchange-rate` → site_currency / provider / auto_enabled / refresh_interval /
  auto_rate / manual_fallback_rate / effective_rate / effective_source / fetched_at / last_success_at /
  last_error / status / **api_key_masked（••••••后4位）**。
- `PUT  /admin/settings/exchange-rate` → 更新 api_key（空则保留）、auto_enabled、refresh_interval_min、manual_fallback_rate。
- `POST /admin/settings/exchange-rate/refresh` → 立即触发 CoinGecko；失败更新 last_error、不覆盖最后合法率。

### 3. API Key 安全
- 仅后端持有，`/public/config` 不暴露，User API 不返回；Admin GET 只返回脱敏。
- 日志/错误不打印完整 key。

### 4. 刷新机制
- 复用现有 asynq `@every 5m` 任务（P0-2 已建）。动态间隔：第一版固定周期检查，真正是否请求 provider 由 `AutoEnabled` + 状态判断决定，未重构 Scheduler。

### 5. 核心资金链（P0-2 已交付，本阶段未改坏）
- 商品 Site Currency 定价 KEEP；订单按 `1 USDT = R SiteCurrency` 换算 USDT 扣款；订单快照 rate/source/at；退款读 USDT 快照；返利基于 USDT；Gateway Rate 隔离未动。

## 二、验证
- `go build ./...` —— **PASS**
- `go test ./internal/modules/exchangerate/...` —— **PASS**（自动新鲜/过期回退手动/双失败 fail-closed/provider 失败不 1:1/USDT 站点 1:1）
- 受影响模块（order/wallet/affiliate/payment）上一轮已回归 PASS；唯一 Windows 环境性失败（gormstore TempDir 文件占用）非本次引入。

## 三、安全/隔离证明
- 商品订单资金链只依赖 `internal/modules/exchangerate`，不引用 Gateway exchange_rate。
- Wallet Recharge / gateway adapters 零改动。
- fail-closed：无有效汇率 → `ErrRateUnavailable` → 拒单；无 1:1 / 0 / Gateway 兜底。

## 四、剩余 P2（明确未做，不冒充完成）
1. **Admin 汇率设置前端页**：后端 API 已就绪，需在现有 Settings 页附近加「USDT 结算汇率」面板（展示当前率/来源/错误/立即刷新/手动 fallback）。
2. **前端 User/Admin USDT 展示**：订单同时显示 `商品金额 X CNY` 与 `实付 Y USDT` + 汇率快照；钱包/退款/返利统一 USDT 单位。需在 DTO 补 `wallet_currency/wallet_amount/exchange_rate*` 字段后渲染。
3. **Redis 读穿缓存**：当前以 settings KV 为跨进程共享真源（下单只读状态、不实时调 CoinGecko）；Redis miss→settings→回填的优化层待加，且 Redis down 必须回落 settings、禁止 1:1。
4. **完整 E2E 矩阵**：provider timeout/500/malformed/0/负数、stale、manual fallback、双失败 fail-closed、CNY/USD/SGD、余额边界、并发、汇率变化后历史单不变、退款快照、返利 USDT、Gateway 隔离。exchangerate 单元层已覆盖核心分支，端到端 order 层待补。

## 五、边界遵守
未改：商品 Site Currency 模块、订单 5 状态、售后、Guest/Auth、Wallet-Only、会员等级/价/批发价、Gateway 协议与 Rate、Webhook、计价顺序。
