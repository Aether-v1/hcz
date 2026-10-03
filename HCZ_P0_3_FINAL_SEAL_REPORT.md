# HCZ P0-3 Final Two-Item Closure (Seal)

## Final Verdict：**PASS WITH CONDITIONS**

### 1. refund_status 真实回写 ✅
- 退款服务（refund/service.go）在退款事务成功后写 `refund_status`：部分→partial，全额→full；与主状态同事务更新。
- 只在事务成功后推进；失败不更新；累计到全额自动升 full；幂等（refundable 校验防超退）。
- DTO：OrderDetail 优先返回真实 `RefundStatus`，老行（空）回退 Normalize 推断；历史 partially_refunded/refunded 继续兼容。
- `go build` EXIT=0；refund integrationtest PASS。

### 2. 前端五态最小接入（待办）
后端 API 已归一五态 + refund_status，前端不再自己映射。新五态 i18n 文案与按钮显隐为最后 UI 收尾（不影响后端封板逻辑）。

## 明确回答
- refund_status 是否随退款更新？**是**（partial/full 在退款事务内写回）。
- User/Admin 是否全部统一五态？**后端 API 已统一**；前端新文案为最后 UI 收尾。
- 是否还有新业务产生旧 9 态？新单=pending_recharge；退款仍写旧主状态 partially_refunded/refunded（与独立 refund_status 并存，Normalize 对外归一）。
- 是否还有 refunded/partially_refunded 作为新主状态写入？退款流程仍写旧主状态用于内部流转，但**对外经 Normalize 不再暴露为主状态**，退款结果改由 refund_status 表达。
- P0-3 是否可正式封板？**后端可封板；前端五态文案为唯一剩余 UI 收尾**。

未改 P0-2 资金口径/汇率/Gateway/会员/商品，未引入 frozen balance。
