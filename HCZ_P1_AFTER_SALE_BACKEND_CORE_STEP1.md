# HCZ P1 After-Sale Backend Core Step 1

## Final Verdict：**PASS WITH CONDITIONS**（Store+Service 包已建并编译通过，尚未接 DI/退款适配器/测试）

### 本轮完成
- `application/aftersale/store.go`：GormStore 实现 Create/GetByID/GetByOrderID/GetPendingByOrderID/UpdateStatus/UpdateResolution。
- `application/aftersale/service.go`：唯一业务入口
  - Request：本人订单 + status=completed + 无 pending 工单 → 建 ticket pending + order.after_sale_status=pending。
  - Reject/Resolve/PartialRefund/FullRefund，状态联动，主状态保持 completed。
  - FullRefund 取 order.WalletPaidAmount。
  - Refunder 端口抽象——Partial/FullRefund 通过 `Refunder.AdminManualRefund(orderID, amount, remark)` 复用正式退款链，售后层不算钱。
- 包 `go build` EXIT=0。

### 仍缺（下一轮）
1. **退款适配器**：现有 `refund.Service.AdminManualRefund` 返回 `(*Order,*Record,error)`，需写一个薄 adapter 实现 `aftersale.Refunder` 适配签名（不能直接复用，要包一层）。
2. **DI 装配**：把 GormStore/Service 接入 bootstrap。
3. Handler/Route/DTO（本轮明确不做）。
4. **测试**：尚未写 service/integration 测试——幂等/并发重复退款防护目前只是逻辑判断（pending 检查），未用 row lock 强约束。
5. partial 金额上限（不得超 wallet_paid_amount）由下游退款链 ErrRefundExceeded 兜底，售后层未前置校验。

## 明确回答
- Store 是否完整？方法齐全（编译通过）。
- Service 是否可跑？包可编译，但未接 DI/退款适配器，端到端未跑。
- Partial/Full 是否真正复用现有退款链？**设计是**（Refunder 端口指向 AdminManualRefund），适配器未写。
- 是否还有重复退款风险？**有**——仅靠 pending 状态判断，未加 row lock/幂等键，需测试验证。
- 是否可进下一轮 Handler+Route？建议先补退款适配器 + 一条 service 单测，再上路由。
