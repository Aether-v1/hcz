# HCZ P1 After-Sale / Not Received — Implementation

## Final Verdict：**BLOCKED（部分落地，核心服务/API/前端未接线）**

本轮已落地（编译通过 EXIT=0）：
- order domain 新增 `AfterSaleStatus`（none/pending/resolved/rejected，not null default none）。
- 新领域模型 `after_sale.go`：`AfterSaleTicket` 表（order_id/user_id/type/reason/description/status/admin_note/refund_amount/resolved_at），type 仅 not_received。
- 主状态五态、refund_status、USDT snapshot 退款链全部 KEEP 未改。

## 尚未完成（必须继续，不可假装完成）
1. **Store 层**：after_sale_ticket 的 contract + gormstore CRUD（GetPendingByOrderID/Create/Update）。
2. **AfterSaleService**：
   - RequestAfterSale（校验订单归本人、status=completed、无 pending 工单 → 建 ticket pending + order.after_sale_status=pending）。
   - AdminAction reject/resolve（更新两状态）。
   - partial_refund/full_refund **必须复用** `refund.Service.AdminManualRefund`（USDT snapshot），成功后置 ticket resolved、order.after_sale_status=resolved，主状态保持 completed，refund_status 由现有退款链自动 partial/full。
3. **路由/Handler**：User POST/GET after-sale；Admin GET + action。
4. **幂等/并发**：复用现有 row lock / transaction；防重复工单、防超额退款。
5. **前端**：User 详情"未收到"按钮；Admin 售后区。
6. **Migration/AutoMigrate**：注册 after_sale_ticket。

## 明确回答
- 售后是否独立于主状态？**设计是**（独立 after_sale_status），但服务未接线，未运行验证。
- 部分/全额退款是否复用现有链？**待实现时必须复用** AdminManualRefund。
- refund_status 联动？退款链已有，待接。
- Commission 回退？复用现有退款 commission 逻辑，待验。
- 是否存在重复退款风险？**尚未加防护**，是下一增量 P0。
- P1 是否可封板？**否**。

下一增量：先做 store + service（复用退款）+ 两组薄路由，补幂等测试，再接最小前端。
