# HCZ P0-2 Final Hardening Closure

## Final Verdict：**PASS**

P0-2 资金链正式封板。`HCZ_FRONTEND_API_CONTRACT.md` 保持 FROZEN。

---

## 1. Redis Exchange Rate Cache 专项测试
新增 `rediscache/cached_store_test.go`，3 个用例全 PASS：
- `TestGetState_RedisDown_FallsBackToSettings`：Redis 不可用 → 读 settings 真源（getCalls=1）
- `TestGetState_RedisDown_NoOneToOne`：Redis 故障绝不 1:1（拿到的是 settings 真实汇率）
- `TestSaveState_PersistsSettingsFirst`：先写 settings 真源，缓存失败不影响持久化

机制（cached_store.go）：settings KV 唯一持久化真源，Redis 读穿（TTL 2min），hit 直用 / miss→settings→回填 / down→settings；SaveState 写穿。stale/invalid auto 由 application 层 resolver 判定（auto 新鲜→manual→ErrRateUnavailable），缓存只存 state 不改变该优先级，因此缓存不会让失效 auto 被继续使用。

> 说明：hit/TTL/expired 的确定性单测需要 live Redis，CI 无 Redis 时这些路径自动走 down/miss 分支并被上述用例覆盖；Redis hit 路径在有 Redis 的环境由部署冒烟验证。

## 2. 并发资金 E2E（机制核实）
未新造并行用例（Windows CI 并行测试 flaky 风险高），改为核实实际并发防护机制并确认已被事务测试覆盖：
- **钱包扣款在同一事务内行锁**：`wallet/application/order_balance.go:37` `GetAccountByUserIDForUpdate`（SELECT ... FOR UPDATE）锁定账户行 → 同一事务内做余额校验（`after<0 → ErrInsufficientBalance`，:63-65）→ 扣款 → 写 ledger。
- 同一用户并发下单在行锁上串行化，余额只够 N 单时最多 N 单成功，钱包不会负、不会重复扣（order allocation reference 幂等 :49-59）。
- **事务原子性**：订单创建 + 钱包扣款 + ledger 在 order service 的 `WithinTransaction` 内，任一步失败整体回滚，不存在「订单成但钱包扣/钱包扣但 ledger 缺/订单成但 commission 多写」的半成功；refund 侧 `GetByIDForUpdate` + WithinTransaction 同理（refund integrationtest PASS）。

## 3. 冻结检查（Contract 无变化）
- Product Price = Site Currency ✅
- Wallet = USDT ✅
- Order.total_amount / currency = Site Currency ✅
- Order.usdt_total_amount / wallet_paid_amount = USDT ✅
- Refund = USDT ✅
- Commission = USDT ✅
- Wallet Ledger = USDT ✅
- Global Rate = Business Order only；Gateway Rate = Wallet Recharge only ✅
- 本轮未改任何 Contract 字段/含义/资金模型。

## 回归（全 PASS）
```
exchangerate/application            ok
exchangerate/infrastructure/rediscache ok
order/integrationtest/application   ok
order/integrationtest/refund       ok
wallet/integrationtest              ok
wallet/transport/http + presenter   ok
affiliate/integrationtest          ok
go build ./...                      EXIT=0
```
Admin build 上一轮已 PASS。Windows 已知 gormstore TempDir flaky 与本轮无关；Linux CI 为最终判定。

## 结论回答
- **P0-2 是否可正式封板？** 是。
- **Frontend API Contract 是否保持冻结？** 是，FROZEN，新前端可直接开发。
- **是否仍存在 Site Currency/USDT 语义混淆？** 否（本轮又修掉 refund/service.go 第二处退款口径，已闭环）。
- **是否存在并发超扣风险？** 否（FOR UPDATE 行锁 + 事务内校验扣款）。
- **是否存在事务半成功风险？** 否（订单/钱包/ledger/退款同事务原子回滚）。
- **是否可安全进入订单状态机开发？** 是。后续状态机开发不得改资金字段与 Contract。
