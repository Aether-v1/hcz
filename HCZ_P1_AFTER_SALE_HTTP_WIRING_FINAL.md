# HCZ P1 After-Sale HTTP Wiring — Final Report

## Final Verdict: PASS

---

## 一、实施范围

将已完成的 aftersale domain/store/service（含共享事务 + WalletRefunderAdapter → AdminRefundToWalletInTx）正式暴露为 User/Admin HTTP API。不修改资金、退款、状态机、数据库模型。

---

## 二、新增文件

| 文件 | 作用 |
|------|------|
| `internal/modules/order/transport/http/aftersale_handler.go` | User + Admin After-Sale Handler + DTO + HTTP error mapping |
| `internal/modules/order/transport/http/aftersale_handler_test.go` | 10 个 Handler 集成测试（真实 DB + 真实退款链） |

## 三、修改文件

| 文件 | 修改 |
|------|------|
| `internal/modules/order/transport/http/routes.go` | 新增 RegisterUserAfterSaleRoutes / RegisterAdminAfterSaleRoutes / RegisterAdminAfterSaleWriteRoutes |
| `internal/bootstrap/order/wiring.go` | Handlers 新增 AfterSale 字段，装配 NewAfterSaleHandler(afterSaleService, orderStore) |
| `internal/bootstrap/order/adapters.go` | orderAdminOrderLookupAdapter 补 GetByIDAndUser 方法 |
| `internal/app/httpserver/router.go` | 构造 afterSaleHandler，传入 registerStorefrontRoutes / registerAdminRoutes |
| `internal/app/httpserver/routes_storefront.go` | 注册 User after-sale 路由（JWT 组） |
| `internal/app/httpserver/routes_admin.go` | 注册 Admin GET（authorized）+ action（paymentProtected） |
| `internal/authz/bootstrap.go` | support 角色补 GET after-sale；finance 角色补 GET + POST action；system_admin 补 exchange-rate 路由（修复 P0-2 遗留 RBAC 覆盖缺口） |
| `HCZ_FRONTEND_API_CONTRACT.md` | 新增第 7 章 After-Sale API Contract |

---

## 四、API 清单

### User API（需 JWT）
| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/v1/orders/:id/after-sale` | 用户发起未收到 |
| GET | `/api/v1/orders/:id/after-sale` | 用户查询售后工单 |

### Admin API（需 JWT + RBAC）
| Method | Path | 中间件组 | 说明 |
|--------|------|----------|------|
| GET | `/api/admin/v1/orders/:id/after-sale` | authorized (JWT+RBAC) | 查询售后 |
| POST | `/api/admin/v1/orders/:id/after-sale/action` | paymentProtected (JWT+RBAC+合规) | reject/resolve/partial_refund/full_refund |

### DTO（AfterSaleDTO）
所有金额显式返回 `refund_currency: "USDT"`，前端不得猜币种。
- `status`: pending / resolved / rejected
- `refund_amount`: 退款金额（USDT 2dp），无退款时为空字符串
- `order_status`: 始终 completed
- `refund_status`: none / partial / full

---

## 五、权限与安全

- **User**：JWT 中间件 + Handler 内 `GetByIDAndUser` 校验归属（IDOR：非本人返回 404，不暴露存在性）
- **Admin GET**：JWT + RBAC，support / finance / system_admin 角色可访问
- **Admin action**：JWT + RBAC + Payment Compliance（paymentProtected 组），finance / system_admin 可执行
- **无公开入口**：after-sale 路由不挂 /public/*
- **Handler 薄**：只负责 parse / auth / validate / DTO / error mapping，资金逻辑全在 aftersale.Service

---

## 六、资金链复用证明

partial_refund / full_refund 调用链：
```
AdminAfterSaleAction (Handler)
  → aftersale.Service.PartialRefund/FullRefund
    → doRefund (共享事务 WithinTransaction)
      → LockAfterSalePendingByOrderIDForUpdate (行锁)
      → WalletRefunderAdapter.RefundInTx
        → AdminRefundToWalletInTx (P0-2 已冻结退款链)
          → Wallet credit USDT
          → Ledger refund credit
          → refund_status partial/full
          → Commission reversal
      → ticket = resolved
      → order.after_sale_status = resolved
    → COMMIT
```

测试验证：
- partial_refund 3.00 USDT → Wallet 100→103，refund_status=partial，order.status=completed
- full_refund → Wallet 100→110，refund_status=full，order.status=completed

---

## 七、测试结果

### Handler 测试（10/10 PASS）
| 测试 | 结果 |
|------|------|
| UserCreateSuccess | PASS |
| UserCreateInvalidTypeRejected | PASS |
| UserCreateDuplicateRejected | PASS |
| UserGetSuccess | PASS |
| UserUnauthorized (401) | PASS |
| AdminReject | PASS |
| AdminPartialRefund (真实退款链) | PASS |
| AdminFullRefund (真实退款链) | PASS |
| AdminInvalidAction | PASS |
| AdminGet | PASS |

### 回归
| 包 | 结果 |
|----|------|
| order/application（含 aftersale + ordermachine） | PASS |
| order/integrationtest/aftersale | PASS |
| order/integrationtest/application | PASS |
| order/integrationtest/refund | PASS |
| order/transport/http | PASS |
| order/transport/presenter | PASS |
| order/domain | PASS |
| authz | PASS |
| httpserver（含 RBAC coverage） | PASS |
| go build ./... | PASS |

### 已知环境问题（非本轮引入）
- `order/infrastructure/gormstore` RiskGate 测试：Windows TempDir SQLite 文件锁导致 cleanup 失败（测试断言本身通过）。Linux CI 正常。

---

## 八、四个明确回答

1. **User API 是否可跑** → 是。POST 发起 + GET 查询，JWT 鉴权 + 归属校验，10 个 Handler 测试覆盖。
2. **Admin API 是否可跑** → 是。GET 查询 + POST action（reject/resolve/partial_refund/full_refund），RBAC + Payment Compliance。
3. **IDOR/RBAC 是否正确** → 是。User 非本人返回 404；Admin 路由挂 RBAC；action 挂 paymentProtected；无公开入口。
4. **partial/full refund 是否仍走原核心链** → 是。WalletRefunderAdapter → AdminRefundToWalletInTx，共享事务 + 行锁，测试验证 Wallet credit + refund_status + order.status=completed。

---

## 九、API Contract 冻结状态

`HCZ_FRONTEND_API_CONTRACT.md` 已新增第 7 章 After-Sale，包含：
- User POST/GET API
- Admin GET/POST API
- AfterSaleDTO 完整字段表（类型 / nullable / 币种含义）
- action 说明表
- 资金规则
- 错误码

P0-2 已冻结资金字段（Wallet/Order/Refund/Commission/Recharge/ExchangeRate）未做任何修改。

---

## 十、是否可以进入 User/Admin 前端最小接入

**是。** 后端 API 已完整可用，Contract 已冻结。前端可直接对接：
- User：completed 订单详情页增加"未收到"按钮 + 售后状态展示
- Admin：订单详情增加售后区域 + 操作按钮（reject/resolve/partial_refund/full_refund）
