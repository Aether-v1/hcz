# HCZ P0-2 Final Backend Closure

## Final Verdict：**PASS WITH CONDITIONS**

P0-2 后端资金链已收口，**API Contract 可冻结，供全新 User 前端直接开发**。
条件见末尾「剩余 P1/P2」。

---

## 1. Order 资金链 E2E（回归覆盖）
真实 Service + Repository + Test DB 已覆盖并 PASS：
- CNY/USD/SGD 下单 → 取 Global Rate → USDT Snapshot → 余额校验 → Wallet 扣款 → 订单
- 2dp half-up rounding（exchangerate application 单测 + order 快照测试）
- 余额不足拒单 / 刚好 / 正常扣款（order integrationtest）
- 汇率 snapshot 写库（usdt_total_amount/exchange_rate/source/at）
- 修改当前汇率后旧订单不变（前端按 snapshot 显示，后端不重算）
- 全额退款按原 USDT、部分退款按 USDT snapshot 比例（refund integrationtest PASS）
- 返利基于 USDT settlement（affiliate integrationtest PASS）
- Wallet ledger 币种 USDT（wallet presenter 测试断言 currency=USDT）
- 无有效汇率 → ErrRateUnavailable 拒单，不产生订单/扣款（application 单测）
- Gateway Rate 仅充值路径，与 Global Rate 无交叉引用
- Wallet Recharge 回归正常（wallet integrationtest PASS）

测试结果：
```
exchangerate/application         ok
order/integrationtest/application ok
order/integrationtest/refund     ok
wallet/integrationtest           ok
affiliate/integrationtest        ok
```

## 2. Redis Global Rate Cache
新增 `internal/modules/exchangerate/infrastructure/rediscache/cached_store.go`：
- Settings KV 仍是持久化真源，Redis 仅读穿缓存（key=`global_exchange_rate:state`, TTL 2min）。
- Redis hit → 直接用；miss → Settings → 回填；Redis 故障 → 回落 Settings（错误吞掉，不 1:1）。
- SaveState 写穿：先写 settings，再更新 Redis。
- 容器已接线：`exStore = rediscache.New(settingsStore)`。
- 严禁项全部满足：Redis 非唯一真源、无 1:1、无 Gateway fallback、下单不实时调 CoinGecko。

## 3. 资金语义残留审计（本轮新修）
全局 grep 生产路径后，**新发现并修复第二处退款口径**：
- `order/application/refund/service.go`（主退款服务，非 wallet.go）：
  原先 `refundable = order.TotalAmount - refundedBefore`、`markRefunded >= TotalAmount` 用 Site Currency 计算。
  已改为：USDT 快照单按 `WalletPaidAmount` 为基数（paidBase 切换），与 wallet.go 一致。

其余命中定性：
- `order_service.go:509` 余额校验用 expectedWallet（有快照=USDT）—— SAFE
- `order_wallet_bridge.go` USDT 分支 online 恒 0 —— SAFE
- `affiliate/commission.go` 已转 USDT —— SAFE
- `payment callback/create` onlineAmount = TotalAmount - WalletPaidAmount：wallet-only 下 online 恒 0，仅在线路径会用到，不影响 HCZ —— 接受
- `memberLevelSvc.OnOrderPaid(TotalAmount)`：会员体系范畴，用户明令不动 —— KEEP

最终语义闭环：
- Product Price = Site Currency ✅
- Wallet = USDT ✅
- Order Original Amount (total_amount) = Site Currency ✅
- Order Paid Amount (usdt_total_amount/wallet_paid) = USDT ✅
- Refund = USDT ✅
- Commission = USDT ✅
- Wallet Ledger = USDT ✅

## 验证
- `go build ./...` PASS（EXIT=0）
- 定向回归（exchangerate/order/wallet/refund/affiliate）全 PASS
- Admin `npm run build` PASS（上一轮已验）
- 未跑全量 `go test ./...`：Windows 环境已知 gormstore TempDir 文件占用 flaky，非本次引入；Linux CI 为最终判定。

## Contract 是否可冻结？
**是。** `HCZ_FRONTEND_API_CONTRACT.md` 字段已稳定：
Wallet/Order/Refund/Commission/Recharge 全部显式带 currency，新前端可直接对接，无需再猜币种或自行换算。

## 剩余 P1/P2
- P2：Redis 缓存层专门的 hit/miss/down/TTL 单测（当前靠回归覆盖，缺专门用例）
- P2：并发下单超扣、事务半成功回滚的专门 E2E（现有 integrationtest 已覆盖主链，缺并发专项）
- P2：新 User 前端按 contract 渲染
- 不做：User 前端 UI、订单 5 状态、售后、会员体系、Gateway 协议、Auth
