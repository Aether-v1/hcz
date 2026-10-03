# HCZ P1 After-Sale Core Wiring

## Final Verdict：**BLOCKED（仍未跑通业务，本轮只完成 schema 注册）**

### 本轮新增
- AutoMigrate 注册表（bootstrap/database/migrations/registry.go）已加入 `&orderdomain.AfterSaleTicket{}`，空库/升级库会自动建 after_sale_tickets 表；orders.after_sale_status 列随 Order 自动迁移。非破坏性，不动历史主状态。
- 编译 EXIT=0。

### 仍未完成（不可封板）
1. Store：AfterSaleTicket CRUD（Create/GetByOrderID/GetPendingByOrderID/Update）+ 事务内 row lock。
2. AfterSaleService：User 发起（本人/completed/无 pending 工单）、Admin reject/resolve/partial_refund/full_refund。
3. partial/full refund 复用 `refund.Service.AdminManualRefund`（USDT snapshot）——尚未接线。
4. User/Admin HTTP Handler + 路由（JWT / Admin RBAC）。
5. 幂等/并发：防重复工单、防超额退款、两 Admin 竞争。
6. DTO（refund_currency=USDT）。
7. 测试矩阵。

## 明确回答
- User 发起是否可跑？否（无 service/route）。
- Admin 处理是否可跑？否。
- partial/full 是否复用现有链？待接（必须复用 AdminManualRefund）。
- 重复退款风险？尚未加防护。
- AutoMigrate 是否完成？是（表+列注册）。
- 后端核心是否可封板？**否**。

下一增量：store → service（refund 复用）→ 薄路由 → 幂等测试。
