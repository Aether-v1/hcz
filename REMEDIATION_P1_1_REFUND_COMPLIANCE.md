# P1-1 退款写路由 Payment Compliance 修复报告

## 修复清单
| 项目 | 文件 | 修改 | 状态 |
|---|---|---|---|
| 路由分组修正 | internal/app/httpserver/routes_admin.go:162 | `RegisterAdminRefundWriteRoutes(authorized,...)` → `paymentProtected` | DONE |
| 路由结构断言同步 | internal/app/httpserver/route_structure_test.go:130 | 断言由 `authorized` 改为 `paymentProtected`（原断言固化了不安全挂载，随修复更新） | DONE |
| 合规拦截运行时测试 | internal/app/httpserver/refund_compliance_test.go（新增） | 3 条退款写路由未 ack → 403 compliance_required；ack 后放行 | DONE |
| 重复退款防护测试 | internal/app/httpserver/refund_compliance_test.go（新增） | 超额二次退款不新增退款记录、不增加 refunded_amount | DONE |

## 1. 路由分组修正
- 修改前：`routes_admin.go:162 RegisterAdminRefundWriteRoutes(authorized, adminOrderRefundHandler)`（仅 JWT+RBAC）
- 修改后：`RegisterAdminRefundWriteRoutes(paymentProtected, adminOrderRefundHandler)`（JWT+RBAC+PaymentComplianceRequired）
- `paymentProtected` 定义于 `routes_admin.go:99`：`admin.Group("", middleware.PaymentComplianceRequired(c.ComplianceService))`，继承 `authorized` 的 JWT+RBAC。

受保护路由列表（internal/modules/order/transport/http/routes.go:29-31，全部随注册函数挂入 paymentProtected）：
- `POST /admin/orders/:id/refund-to-wallet` → `AdminRefundOrderToWallet`
- `POST /admin/orders/:id/manual-refund` → `AdminManualRefundOrder`
- `PATCH /admin/order-refunds/:id/payment-fee` → `UpdateAdminOrderRefundPaymentFee`

## 2. 所有 Wallet credit Admin API 审计
| API | 方法 | 路径 | 挂载组 | 触发 credit | 状态 |
|---|---|---|---|---|---|
| 退款到余额 | POST | /admin/orders/:id/refund-to-wallet | paymentProtected（本次修复） | 是（wallets.CreditInTransaction） | 已修复 |
| 手动退款 | POST | /admin/orders/:id/manual-refund | paymentProtected（本次修复） | 否（仅写退款记录/refunded_amount/refund_status + affiliate 冲销 + reseller 账务） | 已修复 |
| 退款手续费更正 | PATCH | /admin/order-refunds/:id/payment-fee | paymentProtected（本次修复） | 否（更正 payment_fee_refunded 账务标记） | 已修复 |
| 售后退款动作 | POST | /admin/orders/:id/after-sale/action | paymentProtected（routes_admin.go:165 原有） | 是（partial_refund/full_refund） | 已合规 |
| 钱包人工调整 | POST | /admin/users/:id/wallet/adjust | paymentProtected（routes_admin.go:183 原有） | 是 | 已合规 |
| 支付渠道管理 | * | /admin/payment-channels/* | paymentProtected（routes_admin.go:178 原有） | 间接（动账配置） | 已合规 |
| 支付记录管理 | * | /admin/payments/* | paymentProtected（routes_admin.go:179 原有） | 间接 | 已合规 |
| 对账管理 | * | /admin/reconciliations/* | paymentProtected（routes_admin.go:202 原有） | 间接 | 已合规 |
| 推广/分销财务 | * | affiliate finance / reseller finance | paymentProtected（:141/:147/:148 原有） | 是 | 已合规 |

结论：除本次修复的 3 条外，其余动账/财务写入口均已在 paymentProtected，无遗漏。

## 3. Idempotency
- 当前机制：
  - 退款写 handler **不读取** `Idempotency-Key` 请求头；项目内无 HTTP 级幂等中间件。
  - 钱包 `CreditInTransaction` 本身按 `Reference` 幂等（`wallet/application/credit.go:42`：同 reference 已存在则直接返回原交易、不再入账）。
  - 但退款到余额的 reference 为 `fmt.Sprintf("order:%d:admin_refund:%d", orderID, time.Now().UnixNano())`（`refund/wallet.go:91`），含纳秒时间戳 → 每次重试 reference 不同，钱包层幂等对管理员重试不生效。
  - **真正生效的防重兜底**：服务层事务内 `orders.GetByIDForUpdate(orderID)` 行锁 + `refundable = paidBase - refundedBefore` 上限校验，超额即返回 `walletcontract.ErrRefundExceeded`（`refund/wallet.go:131-135`、`refund/service.go:380-384`）。并发/重复请求在行锁上串行，第二次看到已更新的 refunded_amount，超额即拒。
  - after-sale 退款幂等由事务 + 状态机保证（工单必须 PENDING 才能退款，退款后转 RESOLVED，再次退款返回 `ErrInvalidAction`），已有 `TestFullRefundCallsOnceAndIdempotent` 覆盖。
- 修改内容：本次 P1 不新增 idempotency_key 列（需 DB migration + domain/store/contract 改动，超出最小修复范围且触及已冻结退款表）。按任务允许的降级方案，保留并验证服务层 refundable 上限校验作为防重复 credit 兜底。
- 重复退款防护验证：新增 `TestDuplicateManualRefundDoesNotDoubleCredit`——首笔全额退款 100 成功并产生 1 条记录；再退 1 被拒绝，记录数仍为 1，refunded_amount 仍为 100。PASS。

## 4. Reason / Audit
- reason 字段：请求 DTO 已具备 `remark` 字段（`AdminRefundOrderToWalletRequest.Remark` / `AdminManualRefundOrderRequest.Remark`），并落入 `OrderRefundRecord.Remark`。未重复新增 `reason` 字段（remark 即用途一致，避免重复字段）。
- 退款记录字段：`OrderRefundRecord` 已存 `UserID`、`OrderID`、`Type`、`Amount`、`Currency`、`Remark`、`CreatedAt`、`UpdatedAt`。
- admin_id：`OrderRefundRecord` **无 AdminID 列**，退款记录不直接记录"是哪个管理员操作"。本次未加列（DB migration，超出最小修复范围）。
- auditlog：退款流程**无显式 auditlog 写入**；`auditlog` 模块仅为只读查询（`auditlogtransport.RegisterAdminRoutes`），记录的是 AuthzAuditLog（权限/登录类）。RBAC 中间件对拒绝只写结构化日志（logger.Warnw），不对成功退款落审计。管理员身份可从 JWT 上下文（`ginutil.GetAdminID`）获取，但未串入退款记录。
- 遗留：退款记录缺 admin_id、缺独立操作审计事件，建议后续作为 P2 增补（加列 + 审计事件）。

## 5. Step-Up 机制说明
- 当前 step-up 机制：**Payment Compliance 合规声明确认**（`PaymentComplianceRequired`，`middleware/compliance_middleware.go`）。`cs.IsAcknowledged()` 为 false 时拦截：超管返回 403 `compliance_required`，非超管返回 403 `compliance_required_by_super_admin`。
- 是否有 per-request 2FA：**没有**。全仓搜索 `StepUp/Require2FA/step_up` 无结果；中间件目录无逐请求 2FA 闸门。现有 2FA 仅用于管理员登录（`RegisterAdmin2FAAuthRoutes`）与 2FA 管理（`RegisterAdmin2FARoutes`），财务写操作不触发二次验证。
- 结论：当前规范即以"合规声明确认"作为财务写操作的 step-up 闸门；本次已把全部退款写入口挂到该闸门下。是否引入 per-request 2FA 属产品安全策略升级，非本次 P1 修复范围，建议后续评估。

## 测试
| 测试名 | 覆盖场景 | 结果 |
|---|---|---|
| TestRefundWriteRoutesBlockedByComplianceWhenNotAcked（3 子用例） | 未确认合规 → refund-to-wallet/manual-refund/payment-fee 均 403 compliance_required，handler 不被调用 | PASS |
| TestRefundWriteRoutesPassComplianceWhenAcked | 合规已确认 → 闸门放行，请求到达 handler（响应不含 compliance_required） | PASS |
| TestDuplicateManualRefundDoesNotDoubleCredit | 第二次超额退款不新增记录、refunded_amount 不重复增加 | PASS |
| TestRouteDomainFilesPreserveTrustBoundaries / routes_admin.go | 结构断言：退款写路由挂 paymentProtected；after-sale 写路由挂 paymentProtected | PASS |
| TestAllAdminRoutesCoveredByBuiltinRoles | 252 条 admin 路由均被内置角色策略覆盖（组改动不影响 object 路径） | PASS |
| TestFullRefundCallsOnceAndIdempotent（aftersale 既有） | after-sale 状态机幂等确认 | PASS |
| TestPaymentComplianceRequired_*（middleware 既有） | 合规中间件 ack/未 ack/nil 行为 | PASS |

## 验证
- go build ./...：PASS（exit 0）
- go test ./internal/app/httpserver/...：PASS（含 middleware 子包）
- go test ./internal/modules/order/transport/http/...：PASS
- go test ./internal/modules/order/application/refund/...：无测试文件（该包本就无单测），编译通过
- go test ./internal/modules/order/application/aftersale/...：PASS

## 遗留问题（如有）
1. [P2] 退款记录缺 `admin_id`，无法直接追溯"哪个管理员发起的退款"；建议后续加列 + 迁移。
2. [P2] 退款写操作无独立 auditlog 事件；当前仅靠 JWT 上下文 + 结构化日志，建议补审计事件。
3. [P2] 无 HTTP 级 `Idempotency-Key` 幂等回放；当前防重复 credit 依赖行锁 + refundable 上限（可防超额，但同额重复退款会产生两条记录）。建议后续为 `order_refund_records` 增 `idempotency_key` 唯一列，前端 types.ts 已有该字段可对齐。
4. [观察，非本次范围] `AdminManualRefundOrder` handler 错误映射中，`ErrWalletRefundExceeded` 为包内本地哨兵，而服务层返回的是 `walletcontract.ErrRefundExceeded`，二者非同一 error 值，超额退款可能落入 default（500）而非 400。本次不改动既有错误映射（冻结语义），仅在此标注。
5. 未发现新的 P0 阻断。
