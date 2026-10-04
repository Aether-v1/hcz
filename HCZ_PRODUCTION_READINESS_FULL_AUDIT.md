# HCZ Production Readiness Full Audit

> 审计日期：2026-10-04
> 审计范围：全项目 22 个维度
> 审计方式：四域并行深度代码审计 + 真实命令执行（go test/build、vue-tsc/build、Migration 测试）
> 已封板能力：P0-1 Wallet-Only / Authenticated-Only / P0-2 Global Rate + USDT / P0-3 5-State Machine / P1 After-Sale
> 原则：先审计后修复；禁止降低标准；禁止 skip/删测试；禁止修改冻结 Contract 除非真实 P0 缺陷

---

## Final Verdict: BLOCKED

| 指标 | 结果 |
|---|---|
| P0 未闭环项 | **9**（后端 5 + 前端 3 + 基础设施 1） |
| P1 上线阻断项 | **15**（后端 8 + 前端 4 + 安全 2 + 基础设施 1） |
| P2 backlog | **20** |
| 当前可否部署生产 | **否** |
| Linux CI | **NOT EXECUTED**（静态分析：push 必红） |

**核心阻断原因：订单主链断裂。** 新订单创建为 `pending_recharge`，但支付创建、超时取消、履约准入、支付回调仍按旧 `pending_payment`/`paid`/`delivered` 九态运转——用户下单后钱包扣款被 `ErrOrderStatusInvalid` 拒绝，订单永久悬挂，无法履约交付。同时存在 8 处 ACTIVE_WRITE_BLOCKER 主动写旧九态主状态，3 处前端把 USDT 金额错标为站点币，架构守卫测试真实失败。

---

## 关键问题速答

| 问题 | 回答 | 证据 |
|---|---|---|
| P0 是否还有未闭环项 | **是，9 项** | 见 P0 修复清单 |
| P1 是否还有上线阻断项 | **是，15 项**（含 2 项安全） | 见 P1 修复清单 |
| 当前 HCZ 是否可以部署到生产 | **否** | 订单主链断裂 + 8 处旧态主动写入 + CI 必红 |
| User/Admin/Backend Contract 是否一致 | **否** | 前端 3 处 USDT 金额用站点币格式化；AdminOrder 类型缺 wallet_currency 字段；两端订单筛选仍为旧 9 态 |
| 是否还存在 Site Currency / USDT 混淆 | **是** | 佣金退款比例分母用 Site Currency（commission.go:167）；退款记录币种写 order.Currency（refund/service.go:532）；子订单 OnlinePaidAmount 初始化为 Site Currency（order_service.go:627）；前端 3 处格式化错误 |
| 是否还存在旧 9 态主动写入 | **是，8 处 ACTIVE_WRITE_BLOCKER** | payment callback 写 paid/fulfilling；履约写 delivered；admin 直接设 partially_refunded/refunded；CalcParentStatus 写旧态；procurement 写 paid |
| 是否还存在 Guest 商品购买路径 | **否** | ACTIVE_BLOCKER=0；/guest/* 路由组已停用（routes_storefront.go:92），实测 404；guest handler 未被任何路由组引用 |
| 是否还存在 Business Order Gateway 支付路径 | **代码存在但当前不可达**（因 P0 #1 状态不匹配被拒）；属危险死代码，修复 P0 #1 时若不拆除 gateway 分支会导致混合支付复活 | payment_service_create.go:222-244 仍有创建 gateway Payment 逻辑 |
| After-Sale 是否完整可用 | **后端 PASS，前端部分受旧态依赖影响** | 后端事务/退款/佣金回滚/幂等全部到位；前端售后工单 UI 符合 Contract，但订单列表筛选缺新状态 |
| 是否存在重复退款/双扣/半成功风险 | **后端退款链事务安全**（PASS）；但父子订单 refund_status 不同步（PARTIAL），admin 手动退款不联动子单 | refund/wallet.go 行锁+金额上限+幂等；order_service_child.go:169-170 注释"不做父子状态同步" |
| Migration/Fresh Install 是否通过 | **是** | 空库 in-memory SQLite 连跑两遍 AutoMigrate PASS；53 个 model 全量注册；汇率快照三字段/refund_status/after_sale_status/after_sale_tickets/exchangerate settings 全部就位 |
| Linux CI 是否真实全绿 | **CI NOT EXECUTED**；静态分析：api job 会因 gofmt（26 文件）+ architecture 测试双红 | ci.yml api job 含 gofmt 硬门禁 + go test ./... |
| Git Worktree 是否达到可发布状态 | **否** | 26 个 Go 文件未格式化；architecture 测试失败；审计产物未跟踪（不影响但需清理） |

---

## 一、订单主链审计 — FAIL

完整链路：User 登录 → 浏览商品 → 下单 → Global Rate → Site Currency 换算 USDT → Wallet 扣 USDT → pending_recharge → Admin processing → completed

### P0 阻断

1. **钱包支付被状态校验阻断** — `internal/modules/payment/application/payment_service_create.go:98`
   - 代码：`if lockedOrder.Status != constants.OrderStatusPendingPayment { return orderapp.ErrOrderStatusInvalid }`
   - 而 `order_service.go:556` 创建订单时写 `Status: constants.OrderStatusPendingRecharge`
   - 后果：用户下单后调 CreatePayment（钱包扣款）必然收到 ErrOrderStatusInvalid，钱包永远不被扣，订单永远停在 pending_recharge
   - 修复：line 98 校验改为 `OrderStatusPendingRecharge`

2. **超时取消不处理新订单** — `internal/modules/order/application/order_service_child.go:441`
   - 代码：`if order.Status != constants.OrderStatusPendingPayment { return order, nil }`
   - 后果：pending_recharge 订单过期后永久悬挂，库存/优惠券永不释放
   - 修复：改为 `OrderStatusPendingRecharge`

3. **履约服务拒绝新订单 + 写旧态** — `internal/modules/fulfillment/application/service.go:110,144`
   - line 110：要求 `paid`/`fulfilling`，新订单 `processing` 被拒
   - line 144：履约完成后写 `delivered` 而非 `completed`（ACTIVE_WRITE_BLOCKER）
   - 修复：准入改为 `OrderStatusProcessing`；完成写 `OrderStatusCompleted`

### 通过项

- 未登录不能下单：PASS（routes_storefront.go:109 UserJWTAuthMiddleware）
- Guest Order 不可达：PASS（/guest/* 路由组停用）
- 无有效 Global Rate fail-closed：PASS（order_service.go:478-484 Resolve 出错直接 return）
- Wallet 余额不足 fail-closed：PARTIAL（预校验在事务外，实际扣款因 P0 #1 未执行）

---

## 二、五态状态机审计 — FAIL

正式主状态：pending_recharge / processing / completed / failed / canceled

### ACTIVE_WRITE_BLOCKER 清单（必须为 0，当前 8 处）

| # | 位置 | 写入的旧状态 | 说明 |
|---|---|---|---|
| 1 | `payment_service_callback.go:443` | `paid` | markOrderPaid 写父订单 |
| 2 | `payment_service_callback.go:458` | `paid`/`fulfilling` | markOrderPaid 写子订单 |
| 3 | `payment_service_callback.go:473` | CalcParentStatus 返回旧态 | 父订单同步 |
| 4 | `fulfillment/service.go:144` | `delivered` | 履约完成 |
| 5 | `procurement/submit.go:170` | `paid` | 采购提交 |
| 6 | `order_service_child.go:248` | `partially_refunded`/`refunded` | admin 直接设父订单 |
| 7 | `order_service_child.go:258` | `partially_refunded`/`refunded` | admin 直接设子订单 |
| 8 | `order_status.go:35` (SyncParentStatus) | CalcParentStatus 返回旧态 | 父订单状态聚合 |

### P0 阻断

4. **支付回调写旧主状态** — `payment_service_callback.go:443,454,456,471`
   - markOrderPaid 主动写 `paid`/`fulfilling`
   - 修复：wallet-only 模式下短路 markOrderPaid；新五态订单不应走此路径

5. **Admin 可绕过状态机写旧退款态** — `order_service_child.go:243-286`
   - 允许 admin 直接把父子订单设为 `partially_refunded`/`refunded`
   - 修复：删除此分支，退款主状态由 refund_status 独立列承担

### P1

- `order_status.go:76-99` CalcParentStatus 返回并写入旧 9 态，需重写为五态聚合
- `order_service.go:205-241` allowedTransitions 旧 9 态 map 仍被 IsTransitionAllowed 使用，对新五态迁移误拒
- `ordermachine/machine.go` 定义了正确的五态 allowed map 和 Normalize，但生产代码未统一走它

---

## 三、退款语义审计 — PARTIAL

固定规则：主状态 != 退款状态（refund_status 独立列）

### 通过项

- completed + partial refund → status=completed, refund_status=partial：PASS（refund/wallet.go:160-165 只写 refund_status）
- completed + full refund → status=completed, refund_status=full：PASS
- failed 自动退款 → status=failed, refund_status=full：PASS（order_service_child.go:396-399）
- canceled 自动退款 → status=canceled, refund_status=full：PASS（order_service_child.go:43-45,104）

### P1

- **退款记录币种错误** — `refund/service.go:532-535`
  - `currency := strings.ToUpper(strings.TrimSpace(order.Currency))` 取 Site Currency
  - 但 USDT 订单的退款金额是 USDT
  - 对比 refund/wallet.go:122-127 正确使用了 `refundCurrency = "USDT"`
  - 修复：createRefundRecordTx 接收 refundCurrency 参数

### P2

- ReleaseWalletBalance 退款后把 online_paid_amount 写回 order.TotalAmount（Site Currency）— order_wallet_bridge.go:116

---

## 四、USDT 资金语义审计 — FAIL

不变量：Product Price=Site Currency / Wallet=USDT / Order total_amount=Site Currency / Order wallet_paid_amount=USDT / Refund=USDT / Commission=USDT / Ledger=USDT

### P1 混淆点

| 位置 | 问题 | 严重度 |
|---|---|---|
| `affiliate/commission.go:167,178,223` | HandleOrderRefunded 用 TotalAmount(Site Currency) 作比例分母，delta 是 USDT。例：100 CNY=14.29 USDT，退 5 USDT，正确扣 35% 佣金，实际扣 5% | P1 |
| `order_service.go:627` | 子订单 OnlinePaidAmount 初始化为 Site Currency 金额，wallet-only 应为 0 | P1 |
| `refund/service.go:532` | createRefundRecordTx 用 order.Currency(Site) 作退款记录币种 | P1 |
| `order_wallet_bridge.go:116` | ReleaseWalletBalance 后 online_paid_amount 回填 TotalAmount(Site) | P2 |

### 通过项

- Product Price = Site Currency：PASS
- Wallet = USDT：PASS（wallet/account.go；ApplyOrderBalance 传 ledgerCurrency="USDT"）
- Order total_amount = Site Currency：PASS
- Order wallet_paid_amount = USDT：PASS（order_wallet_bridge.go:40-41,67）
- Refund = USDT（快照，不重算）：PASS（refund/wallet.go:124-127）
- Commission = USDT（计算正确）：PASS（calculateCommissionBaseAmount line 305-307 正确换算）；但退款比例回退分母错误（见上）
- Wallet Ledger = USDT：PASS

---

## 五、Global Rate 审计 — PASS

- CoinGecko Provider：PASS（exchangerate/infrastructure/provider/coingecko.go）
- API Key 只在 Admin Settings：PASS（service.go:130-150 存 state.APIKey）
- Public/User API 不泄露 Key：PASS（exchangerate 无 public/user handler）
- Admin GET 只返回 masked key：PASS（admin_handler.go:46 `api_key_masked`，只暴露后 4 位）
- 自动/manual fallback：PASS（service.go:53-66 先 AUTO 后 MANUAL，三级失败返回 ErrRateUnavailable）
- 无 AUTO + Manual → 拒绝下单：PASS（order_service.go:478-481）
- 无 1:1 fallback：PASS（service.go:101 rate<=0 返回错误）
- Gateway Rate 与 Global Rate 完全隔离：PASS（Gateway exchange_rate 在 payment/infrastructure/gateway/common/exchange.go，与 exchangerate/ 无交叉引用）

### P2

- rateResolver 未注入时静默跳过（order_service.go:477 `if s.rateResolver != nil`），fail-open 而非 fail-closed。建议加启动校验。

---

## 六、Wallet Recharge 审计 — PASS

- Gateway Payment 只服务 Wallet Recharge：PARTIAL（business order gateway 代码仍存在但因 P0 #1 不可达）
- Wallet Recharge 正常创建 Payment：PASS（wallet/application/recharge.go:13-33）
- callback/webhook 正常：PASS（payment_service_callback.go）
- Gateway exchange_rate 不被 Business Order 使用：PASS
- Global Rate 不影响 Wallet Recharge：PASS（recharge.Amount 是 gateway 侧换算后 USDT）

### P2

- payment 模块仍保留 business order gateway payment 全链路代码（create/callback/markOrderPaid），属危险死代码。修复 P0 #1 时若只改状态校验不拆除 gateway 分支，会导致混合支付复活。

---

## 七、After-Sale 审计 — PASS（后端）

完整链：completed order → User 未收到 → after_sale_status=pending → Admin reject/resolve/partial_refund/full_refund

- 售后不产生第六主状态：PASS（aftersale/service.go:73,104,146 只写 after_sale_status）
- partial/full refund 走 AdminRefundToWalletInTx：PASS（aftersale/adapter.go:27）
- Wallet 真正 credit：PASS（refund/wallet.go:137 CreditInTransaction）
- Ledger 写入：PASS
- refund_status 正确：PASS（refund/wallet.go:160-165）
- Commission 正确 reverse：PASS（refund/wallet.go:171-181 在 tx 内）
- ticket/order/wallet/refund/commission 同一事务：PASS（aftersale/service.go:126-151 WithinTransaction）
- refund 失败整体 rollback：PASS
- 不存在重复退款：PASS（refund/wallet.go:103 GetByIDForUpdate 行锁；line 131-134 refundable 校验；wallet credit 用 reference 幂等）

---

## 八、权限 / IDOR / RBAC — PARTIAL

### Admin RBAC

- **252 条 admin 路由全部被 RBAC 覆盖**（自动化测试 `TestAllAdminRoutesCoveredByBuiltinRoles` PASS）
- Exchange Rate Settings 已纳入 RBAC（system_admin 角色）
- 未覆盖 Admin Route：**无**

### User 侧 IDOR（全部 PASS）

| 资源 | 归属校验 | 失败状态码 |
|---|---|---|
| 订单列表/详情/取消 | UserID 来自 JWT，GetOrderByUserOrderNoForTenant | 404 |
| 交付下载 | GetAnyOrderByUserOrderNoForTenant(uid) | 404 |
| 发起/查询售后 | svc.Request(uid) / GetByIDAndUser(orderID, uid) | 404 |
| 创建/捕获支付 | GetOrderByUserOrderNoForTenant(uid) | 404 |
| 钱包/交易/充值 | GetAccount(uid) / ListTransactions(uid) | 404 |

非本人/不存在资源统一返回 404，不暴露存在性。

### P1 — 财务写操作 Step-Up 缺口

3 条退款写路由挂在 `authorized`（仅 JWT+RBAC）而非 `paymentProtected`（JWT+RBAC+PaymentComplianceRequired）：

| 路由 | 挂载分组 | 应挂载 |
|---|---|---|
| `POST /admin/orders/:id/refund-to-wallet` | authorized | paymentProtected |
| `POST /admin/orders/:id/manual-refund` | authorized | paymentProtected |
| `PATCH /admin/order-refunds/:id/payment-fee` | authorized | paymentProtected |

而同样动账的 after-sale/action 和 wallet/adjust 均挂在 paymentProtected。修复：routes_admin.go:162 将 RegisterAdminRefundWriteRoutes 接收者改为 paymentProtected。

---

## 九、Authenticated-Only 审计 — PARTIAL

### 通过项

- Products/Categories/Content/MemberLevels/Orders/Wallet/Recharge/Affiliate/After-Sale 全部在 JWT 组
- 未登录直打业务 API 被拒绝（自动化测试 TestAuthenticatedOnlyBoundaryBehavior PASS）
- 公开例外与 Contract 一致：public config / captcha / affiliate click / auth 链 / password reset / payment callback

### 偏差（P2）

未认证请求返回 **HTTP 200 + body `status_code:401`**（统一响应包 response/response.go:88,106-108），而非 HTTP 401 状态行。enforcement 正确（fail-closed），但 WAF/网关无法按 HTTP 401 识别。

---

## 十、Guest / Legacy Payment 残留 — PASS

- **ACTIVE_BLOCKER = 0**
- `/guest/*` 路由组已停用（routes_storefront.go:92 注释明确），实测 404
- 所有 RegisterGuest* 路由函数均为 LEGACY_UNUSED，无生产路由调用
- GuestHandler 已实例化但未挂到任何路由组
- CreateGuestOrder / CreateGuestPayment 仅被未注册路由引用，不可达
- CreateOrderAndPay（用户版）挂在 user 组，强制登录，调用 CreateOrder 非 guest 分支

---

## 十一、前端 Contract 审计 — FAIL

以 HCZ_FRONTEND_API_CONTRACT.md 为冻结 Contract。

### P0 — USDT 金额被错标为站点币（3 处）

1. **`user/templates/vault/components/VaultOrderBody.vue:78,86`**
   - `formatMoney(order.wallet_paid_amount, order.currency)` 和 `formatMoney(refunded_amount, order.currency)`
   - 经典 OrderDetail.vue:362/372 已正确，仅 vault 模板漏改
   - 修复：改用 `order.wallet_currency || 'USDT'`

2. **`admin/views/admin/components/OrderDetailDialog.vue:844,856`**
   - `formatMoney(wallet_paid_amount, selectedOrder.currency)` 和 `formatMoney(refunded_amount, selectedOrder.currency)`
   - 且 `admin/api/types.ts:154 AdminOrder` 类型**根本没有 wallet_currency 字段**
   - 修复：补类型字段 + 改用 USDT

3. **`user/composables/usePayment.ts:502-507`**
   - `paymentWalletPaidDisplay` 用 `order.value?.currency` 格式化 wallet_paid_amount
   - 修复：改用 `paymentResult.wallet_currency || 'USDT'`

### P1 — 旧 9 态未迁移到新 5 态

1. User 订单筛选下拉仍是旧 9 态（OrdersPanel.vue:294-306），缺 pending_recharge/processing/failed；立即支付按钮绑 pending_payment（:130）；usePayment.ts 轮询/倒计时全部绑 pending_payment（10+ 处）
2. Admin markCompleted 绑 delivered（Orders.vue:169）；canCreateFulfillment 绑 paid/fulfilling（:181, OrderDetailDialog.vue:270）；筛选下拉旧 9 态（:320-328）；Dashboard KPI pending_payment_orders（:40）
3. 结账分账模型（useCheckout.ts:128-139）把 USDT 钱包余额与站点币 total 当同币种做 cents 分账
4. 退款卡片显隐由旧主状态 refunded/partially_refunded 驱动（useOrderDisplayHelpers.ts:33-36），应改为 refund_status

### 通过项

- 售后工单 UI 完全符合 Contract（四 action / USDT 退款 / pending·resolved·rejected）
- 佣金 User 端 USDT 显示正确
- 历史订单 USDT 实付不自行重算（直接读后端快照）
- 无前端直连 CoinGecko（仅 Admin 汇率配置页显示 provider 名）

---

## 十二、数据库 / Migration — PASS

- AutoMigrate 入口：internal/bootstrap/database/migrations/registry.go:43，53 个 model 全量注册
- 关键字段全部就位：

| 表/字段 | 状态 |
|---|---|
| orders.exchange_rate (decimal(20,8)) | ✓ |
| orders.exchange_rate_source (varchar(16)) | ✓ |
| orders.exchange_rate_at (带 index) | ✓ |
| orders.refund_status (not null default 'none') | ✓ |
| orders.after_sale_status (not null default 'none') | ✓ |
| after_sale_tickets 表 | ✓ |
| exchangerate settings (KV global_exchange_rate) | ✓ |

- Fresh install 测试：PASS（全新 in-memory SQLite 连跑两遍 AutoMigrate，TestAutoMigrateOwnsResellerSchemaAndCrossModuleConstraints PASS）
- 非破坏性：仅两处有守卫的一次性 DropColumn/DropIndex（price_currency、旧 cart 索引），无 DropTable
- 历史数据可读：ordermachine.Normalize() 覆盖旧 9 态映射，不批量改写

---

## 十三、事务 / 并发 — PARTIAL

- Wallet debit row lock：PASS（ensureAccountForUpdate credit.go:142-146 → GetAccountByUserIDForUpdate，PostgreSQL 下 SELECT FOR UPDATE）
- AdminRefundToWalletInTx 事务边界：PASS（refund/wallet.go:48 WithinTransaction + line 103 行锁）
- After-Sale refund 事务：PASS（见维度七）
- 未发现 nested independent transaction
- 未发现 transaction escape

### Windows/SQLite 语义差异

SQLite 不支持 SELECT FOR UPDATE，GetAccountByUserIDForUpdate 在 SQLite 下退化为普通读。生产 PostgreSQL 下行锁有效。审计环境为 Windows，无法真实验证并发行锁，标记为已知差异。

---

## 十四、Commission / Affiliate — PARTIAL

- commission = USDT：PASS（calculateCommissionBaseAmount 正确换算）
- completed 正常确认：PASS（HandleOrderPaid + ConfirmDueCommissions）
- failed/canceled rollback：PASS（HandleOrderCanceled 全额 reverse）
- full refund 全量 reverse：PASS
- 无重复 rollback：PASS（状态机 + 金额上限）

### P1

- **partial refund 按比例 reverse 分母币种错误** — commission.go:167,178,223
  - 用 order.TotalAmount（Site Currency）作分母，delta 是 USDT
  - 例：100 CNY = 14.29 USDT，退 5 USDT，正确扣 35% 佣金，实际扣 5%
  - 修复：对 USDT 订单改用 order.WalletPaidAmount.Decimal 作分母

### P2

- 已进入提现流程的佣金跳过不回退（commission.go:138-141,204-207），业务规则可接受，需运营线下处理

---

## 十五、Parent / Child Order — FAIL

### P0

- Admin 可将父订单设为 partially_refunded/refunded 并级联子订单（order_service_child.go:243-286）— 同维度二 P0 #5

### P1

- completeParentOrderInTx 要求子订单 status=delivered（order_service_child.go:379），新模型应为 completed
- canCompleteParentOrder 要求父 status=delivered（:470），新模型应在 processing 时允许 complete
- SyncParentStatus/CalcParentStatus 不识别 pending_recharge/processing/failed（order_status.go:54-71）
- 父子退款 refund_status 不同步（refund/wallet.go:169-170 注释"不做父子状态同步"），父单手动退款不联动子单

### 通过项

- 父子取消：PASS（cancelOrderWithChildren 父+子都写 canceled，ReleaseWalletBalance 只对父单调一次）
- 双退款防护：PASS（ReleaseWalletBalance claim 回调幂等 + refundable 校验）

---

## 十六、前端 Build / Type — PASS（构建层面）

所有命令真实执行：

| 检查 | 结果 | 耗时 |
|---|---|---|
| User `npx vue-tsc --noEmit` | **PASS**（exit 0，0 错误） | — |
| User `npm run build` | **PASS**（vue-tsc -b && vite build，exit 0） | 31.20s |
| Admin `npx vue-tsc --noEmit` | **PASS**（exit 0，0 错误） | — |
| Admin `npm run build` | **PASS**（exit 0；1 条 esbuild 警告：重复 case 'completed'，非阻断） | 18.93s |

### UI 残留

- old 9-state UI：存在（两端订单筛选/动作仍绑旧态，见维度十一 P1）
- guest purchase UI：存在（功能完整，由 wallet_only_payment 配置门控）
- Gateway payment UI in checkout：存在（由 wallet_only_payment=true 时强制 useBalance 并阻断 online 渠道）

---

## 十七、后端全量验证 — FAIL

所有命令真实执行：

| 检查 | 结果 |
|---|---|
| go fmt | **FAIL**（26 个文件未格式化） |
| go vet | **PASS**（0 warning / 0 error） |
| go test ./... | **FAIL**（189/368 包通过；4 包 FAIL，其中 1 包真实失败、3 包 Windows flaky） |
| go build ./... | **PASS** |

### go test 详情

- 总包数：368
- PASS（含测试且通过）：189
- FAIL：4
- 无测试文件：175
- SKIP：0

**真实失败（非环境问题，Linux CI 同样会红）：**

- `internal/architecture` — 2 个测试全部失败：
  - TestDependencyRules：exchangerate/transport/admin_handler.go 直接 import gin（违反分层）；aftersale_handler_test.go 直接 import 6 个具体 gormstore（应依赖 contract）
  - TestOrderAdminHTTPLivesInTransport：P1 after-sale 新增文件超预算（order/domain 6>5、gormstore 9>8、transport/http 9>7）

**已知 Windows flaky（不计入真实失败）：**

- `internal/modules/order/infrastructure/gormstore` RiskGate 2 例（TempDir SQLite 文件锁，断言通过）
- `internal/selfupdate` 9 例（Unix permission / binary lock 语义）
- `internal/logger` 1 例（TempDir 日志文件句柄，本次新观测，同类文件锁）

---

## 十八、Linux CI — NOT EXECUTED

本轮未 push 远程，未触发任何 workflow。以下为静态分析：

### ci.yml（4 个 job，配置完整）

| Job | 验证内容 | 静态评价 |
|---|---|---|
| installer | bash -n + shellcheck + hcz-manager_test.sh + goreleaser archives 校验 | 有，路径真实存在 |
| api | gofmt 门禁 → go vet → go test → go build | **当前 push 必红**：gofmt 26 文件 + architecture 测试失败 |
| release-config | goreleaser check | 配置合理 |
| fullstack | pnpm install → 双前端单测 → build → 嵌入 → fullstack 标签编译 | 有，覆盖完整链路 |

### release.yml

- 触发：push tag v*，permissions: contents: write
- 步骤：goreleaser release --clean，配置正确

**静态结论：当前 worktree 一旦 push，api job 会因 gofmt + architecture 测试双红失败。**

---

## 十九、死代码 / Legacy 清理审计 — PARTIAL

| 发现 | 分类 | 说明 |
|---|---|---|
| guest HTTP 传输处理器与路由注册函数 | RETIRE_SAFE | 当前不可达，被测试钉住；产品决策性停用 |
| guest 领域/存储逻辑（UserID=0 订单、guest 风控） | KEEP | 仍被 order_service/risk_gate 引用，历史 guest 订单可读 |
| 旧 9 态常量与 Normalize 映射 | KEEP | 历史订单读取依赖，迁移后才可能退役 |
| 旧 legacy HTTP handler 目录 | 已删除 | 架构测试强制保持删除 |
| scripts/hcz-manager.sh + 测试 | KEEP | 活跃安装器，被 CI 验证 |
| payment 模块 business order gateway 全链路 | RETIRE_AFTER_MIGRATION | 当前因 P0 #1 不可达，但修复时必须拆除否则混合支付复活 |

- BLOCKER 清单：**无**

---

## 二十、Git / Repository Closure — PARTIAL

- git status：dirty（仅 2 个未跟踪文件：AUDIT_BACKEND_BUSINESS.md 遗留 + test_output.txt 临时），无已跟踪修改
- 分支：main，跟踪文件 1802 个

### 不应提交的文件检查（全部 PASS）

| 类型 | 状态 |
|---|---|
| 已跟踪 .exe / 二进制 | 无 |
| 已跟踪 .db / .sqlite | 无 |
| 已跟踪 dist/ | 无 |
| 已跟踪 node_modules | 无 |
| 已跟踪 coverage | 无 |
| 已跟踪 runtime/ | 无 |

### .gitignore

- 完备：覆盖 .vscode/、config.yml、db/、uploads/、*.db、*.log、.env、node_modules/、二进制、*.test、coverage、logs/、dist/、runtime/、*.tmp、.agents/ 等
- 建议：test_output.txt 等审计日志可加入（*.log 不匹配 .txt）

### P2

- 根目录堆积 30 份 HCZ_*.md 阶段性报告，建议归档至 docs/archive/

---

## 二十一、安全审计 — PARTIAL

### P0：无

### P1

1. **退款写路由未挂 Payment Compliance Step-Up** — routes_admin.go:162（同维度八）
2. **注册/找回密码/发送验证码端点无速率限制** — routes_storefront.go:97-98,104
   - 登录有限流（KeyByIPAndJSONField("email")）
   - 但 send-verify-code / register / forgot-password 均未挂限流
   - 风险：邮件轰炸 / SMTP 配额耗尽 / 批量注册
   - 修复：挂 middleware.RateLimitMiddleware（按 IP+email）

### P2

1. 硬编码默认超管密码 admin123（bootstrap.go:14）— release 模式已兜底（空密码跳过 + 弱密码 Fatal），dev 模式仍有风险
2. 默认管理员密码明文写入日志（bootstrap.go:49）
3. 密码重置存在账号枚举（user_password_handler.go:87-88，用户不存在返回 404 vs 验证码错误返回 400）
4. CORS 默认允许任意来源（config.go:225 defaultCORSAllowedOrigins=["*"]）— allow_credentials 默认 false，Bearer token 无 cookie，CSRF 风险低；但若误开 credentials 则危险
5. 未认证返回 HTTP 200 + body 业务码 401（同维度九）
6. 退款/钱包调整无显式幂等键（依赖状态机+金额上限兜底）
7. JWT 无独立 refresh token 轮换（24h + TokenVersion 吊销，可接受）

### 已通过的安全项

- API Key 脱敏：支付渠道 12 个敏感键统一脱敏为 ••••••••
- Webhook 签名：DujiaoPay HMAC、WeChat 签名校验、Stripe/PayPal 委托服务层
- SQL 注入：全部 GORM 参数化，无字符串拼接
- Mass assignment：handler 一律使用显式 Request DTO
- 登录验证码：强制 captcha.Verify
- 密码策略：passwordpolicy.Validate，弱密码拒绝
- JWT：HS256 强制（防 alg=none），24h 过期，TokenVersion 吊销
- Trusted Proxies：禁止 0.0.0.0/0

---

## 二十二、最终交付

### P0 修复清单（9 项，全部必须修复后方可考虑上线）

| # | 模块 | 文件:行号 | 问题 | 修复 |
|---|---|---|---|---|
| 1 | 订单主链 | payment_service_create.go:98 | CreatePayment 校验 pending_payment，新订单为 pending_recharge，钱包扣款被拒 | 改为 OrderStatusPendingRecharge |
| 2 | 订单主链 | order_service_child.go:441 | CancelExpiredOrder 只处理 pending_payment，新订单永久悬挂 | 改为 OrderStatusPendingRecharge |
| 3 | 订单主链 | fulfillment/service.go:110,144 | 履约准入要求 paid/fulfilling；完成写 delivered 而非 completed | 准入改 processing；完成写 completed |
| 4 | 状态机 | payment_service_callback.go:443,454,456,471 | markOrderPaid 写 paid/fulfilling（ACTIVE_WRITE_BLOCKER） | wallet-only 模式短路或拆除 |
| 5 | 状态机 | order_service_child.go:243-286 | Admin 直接设父子订单 partially_refunded/refunded（ACTIVE_WRITE_BLOCKER） | 删除此分支，退款由 refund_status 承担 |
| 6 | 前端 | VaultOrderBody.vue:78,86 | USDT 金额用 order.currency 格式化 | 改用 wallet_currency \|\| 'USDT' |
| 7 | 前端 | OrderDetailDialog.vue:844,856 + types.ts:154 | Admin USDT 金额用 selectedOrder.currency 格式化；AdminOrder 缺 wallet_currency 字段 | 补类型字段 + 改用 USDT |
| 8 | 前端 | usePayment.ts:502-507 | paymentWalletPaidDisplay 用 order.currency 格式化 USDT | 改用 paymentResult.wallet_currency \|\| 'USDT' |
| 9 | 基础设施 | internal/architecture | 架构守卫测试真实失败（after-sale 文件超预算 + exchangerate transport import gin + aftersale test 依赖具体 store） | 同步更新架构守卫预算/白名单，或调整文件位置 |

### P1 修复清单（15 项，上线前必须修复安全相关 2 项，其余建议本轮修复）

| # | 模块 | 文件:行号 | 问题 |
|---|---|---|---|
| 1 | 风控 | order_store.go:685,734,748,771 | 风控待单查询查 pending_payment，看不到新订单 |
| 2 | Dashboard | overview.go:48,128 | 仪表盘统计查 pending_payment |
| 3 | 状态机 | order_status.go:76-99 | CalcParentStatus 返回旧 9 态，需重写为五态聚合 |
| 4 | 父子订单 | order_service_child.go:379,470 | completeParent/canCompleteParent 要求 delivered，应为 completed/processing |
| 5 | 佣金 | commission.go:167,178,223 | 退款比例回退分母用 Site Currency，应为 USDT (WalletPaidAmount) |
| 6 | 退款 | refund/service.go:532 | 退款记录币种写 order.Currency，USDT 订单应为 USDT |
| 7 | 订单 | order_service.go:627 | 子订单 OnlinePaidAmount 初始化为 Site Currency，wallet-only 应为 0 |
| 8 | 状态机 | order_service.go:205-241 | allowedTransitions 旧 9 态 map 仍被 IsTransitionAllowed 使用，对新五态误拒 |
| 9 | 前端 | OrdersPanel.vue:294-306 + usePayment.ts 10+ 处 | User 订单筛选/动作/轮询仍绑旧 9 态 |
| 10 | 前端 | Admin Orders.vue:169,181,320-328 + Dashboard.vue:40 | Admin 订单动作/筛选/KPI 仍绑旧 9 态 |
| 11 | 前端 | useCheckout.ts:128-139 | 结账分账把 USDT 钱包余额与站点币 total 当同币种 |
| 12 | 前端 | useOrderDisplayHelpers.ts:33-36 | 退款卡片显隐由旧主状态驱动，应改为 refund_status |
| 13 | 安全 | routes_admin.go:162 | 3 条退款写路由未挂 PaymentCompliance Step-Up |
| 14 | 安全 | routes_storefront.go:97-98,104 | register/forgot-password/send-verify-code 无速率限制 |
| 15 | 基础设施 | 26 个 Go 文件 | gofmt 未格式化，CI 硬门禁必失败 |

### P2 backlog（20 项，不阻断上线，排入后续迭代）

1. order_wallet_bridge.go:116 — ReleaseWalletBalance 后 online_paid_amount 回填语义修正
2. payment 模块 business order gateway 死代码清理（markOrderPaid/callback/create gateway 分支）
3. order_service.go:477 — rateResolver 未注入时启动校验（fail-closed）
4. procurement lifecycle.go — 旧状态返回值对齐五态
5. 前端 status.ts — 清理旧 9 态 label/variant 死映射
6. 前端 admin status.ts — 重复 case 'completed'（esbuild 警告）
7. 前端 Admin AffiliateCommissions.vue:200 — 佣金金额补 USDT 单位
8. 前端 Admin ProcurementOrders.vue — 上游采购前端汇率乘法确认域归属
9. 前端 i18n — 旧 9 态文案死键清理
10. 前端 guest purchase UI / checkout 网关支付 UI — 产品决策是否保留
11. 安全 bootstrap.go:14 — 移除 admin123 硬编码回退
12. 安全 bootstrap.go:49 — 删除明文密码日志
13. 安全 user_password_handler.go:87-88 — 密码重置消除账号枚举
14. 安全 config.go:225 — 生产强制显式配置 CORS allowed_origins
15. 安全 response.go:88 — 评估未认证响应改为真实 HTTP 401
16. 安全 admin_refund_handler.go — 退款/钱包调整引入幂等键
17. 安全 JWT — 评估 access token refresh 轮换
18. 基础设施 — guest 传输层死代码退役（待产品确认永久放弃游客购买）
19. 基础设施 — 根目录 30 份 HCZ_*.md 归档至 docs/archive/
20. 基础设施 — test_output.txt 等审计日志加入 .gitignore

---

## GO LIVE CHECKLIST

### 必须全部完成后方可进入生产部署阶段

- [ ] **P0-1** 修复 payment_service_create.go:98 状态校验（pending_payment → pending_recharge）
- [ ] **P0-2** 修复 order_service_child.go:441 超时取消状态校验
- [ ] **P0-3** 修复 fulfillment/service.go:110 准入状态 + :144 完成写 completed
- [ ] **P0-4** 拆除/短路 payment_service_callback.go markOrderPaid 写旧态链路
- [ ] **P0-5** 删除 order_service_child.go:243-286 admin 直接设旧退款态分支
- [ ] **P0-6** 修复 VaultOrderBody.vue USDT 金额格式化
- [ ] **P0-7** 修复 Admin OrderDetailDialog.vue USDT 金额格式化 + 补类型字段
- [ ] **P0-8** 修复 usePayment.ts paymentWalletPaidDisplay 币种
- [ ] **P0-9** 修复 internal/architecture 架构守卫测试（预算/白名单/分层）
- [ ] **P1-安全-1** 退款写路由挂 PaymentCompliance Step-Up
- [ ] **P1-安全-2** register/forgot-password/send-verify-code 补速率限制
- [ ] **P1-基础** gofmt -w 26 个未格式化文件
- [ ] **回归验证** 修复后重新执行 go test ./...（确认 architecture 测试通过，无新增真实失败）
- [ ] **回归验证** 修复后重新执行 User/Admin vue-tsc + build
- [ ] **回归验证** 端到端验证：下单 → 钱包扣款 → pending_recharge → admin processing → 履约 → completed 全链路
- [ ] **回归验证** 退款链：partial/full refund + failed/canceled 自动退款 + after-sale refund
- [ ] **回归验证** 确认 ACTIVE_WRITE_BLOCKER = 0（全局搜索旧状态词写入路径）
- [ ] **CI** push 后确认 Linux CI 全绿（gofmt + vet + test + build + fullstack + installer）
- [ ] **Git** 清理审计临时文件（test_output.txt、AUDIT_*.md），确认 worktree 干净

---

## 是否允许进入生产部署阶段

**否。** 当前存在 9 项 P0 阻断（含订单主链完全断裂），不具备生产部署条件。必须完成上述 GO LIVE CHECKLIST 全部项目后，重新执行全量审计并获得 PASS / PASS WITH CONDITIONS 结论，方可进入生产部署阶段。

---

## 审计产物索引

| 报告 | 路径 | 覆盖维度 |
|---|---|---|
| 后端业务逻辑审计 | AUDIT_BACKEND_BUSINESS.md | 1-7, 13-15 |
| 安全与权限审计 | AUDIT_SECURITY.md | 8-10, 21 |
| 前端 Contract 与 Build 审计 | AUDIT_FRONTEND.md | 11, 16 |
| 基础设施与后端验证审计 | AUDIT_INFRASTRUCTURE.md | 12, 17-20 |
| 本报告（最终交付） | HCZ_PRODUCTION_READINESS_FULL_AUDIT.md | 1-22 汇总 |
