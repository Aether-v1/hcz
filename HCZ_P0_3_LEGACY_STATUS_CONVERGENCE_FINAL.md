# HCZ P0-3 Legacy Status Path Convergence — Final

## Final Verdict：**PASS WITH CONDITIONS**

老 `UpdateOrderStatus` 主链已接入状态机合法性校验，保留原有正确副作用，未重写整条链。

## 本轮改动
1. **统一合法性校验**（order_service_child.go UpdateOrderStatus 入口）：
   - 归一当前/目标后调用 `ordermachine.CanTransition`，非法迁移（含终态回退）直接拒。
   - 豁免 legacy 退款主状态 partially_refunded/refunded（它们走独立退款链路，不进五态机）。
2. **failed 复用成熟退款/佣金回滚**：
   - target=failed 与 canceled 共用 `cancelSingleOrderInTx`（内含 P0-2 USDT snapshot 退款 + ledger）。
   - failed/canceled 都触发 `affiliateSvc.HandleOrderCanceled` 回滚佣金。
3. 保留父子单、cancelOrderWithChildren、completed 完成逻辑等全部原有副作用，未新造第二套 cancel/refund。

## 验证
- `go build ./...` EXIT=0。
- `TestUpdateOrderStatus*` 全 PASS（修正对 legacy refund 状态的误拦后）。
- order integrationtest/application、refund、ordermachine 全 PASS。

## 明确回答
- UpdateOrderStatus 是否受状态机保护？**是**（五态写入入口统一 CanTransition）。
- canceled 是否复用原成熟退款链？**是**（未改）。
- failed 是否具备 USDT refund + commission rollback？**是（复用 canceled 底层资金回滚）**。
- parent/child 是否双退款？复用原有级联逻辑，未新增双触发路径。
- 是否仍有可触达写绕过 machine？User Cancel 尚未显式 `AllowedByUser`（走老 cancel 链，建议下一增量补）。
- refund_status 是否落库？否（仍由 Normalize 推断，未加列）。

## 仍待（不阻塞主链，但封板前建议补）
- User Cancel 显式 `AllowedByUser` 后端校验。
- refund_status 物理列 migration。
- parent 分支 failed（目前父单走 canceled 级联；父单 failed 可复用 cancelOrderWithChildren 语义）。
- Admin 前端五态筛选/按钮最小接入。

未引入 frozen balance，未改 P0-2 资金口径/汇率/Gateway/会员/商品。
