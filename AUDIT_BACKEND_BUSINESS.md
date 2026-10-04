# 后端业务逻辑审计报告

## 审计摘要
- 审计维度数：10
- P0 发现：5
- P1 发现：8
- P2 发现：4

## 各维度详细结果

### 一、订单主链审计
- 结论：FAIL
- 发现列表（按严重度排序）
  - [P0] Payment CreatePayment 校验 `pending_payment` 但新订单状态为 `pending_recharge`，钱包支付链路完全断裂 — `internal/modules/payment/application/payment_service_create.go:98` — 代码 `if lockedOrder.Status != constants.OrderStatusPendingPayment { return orderapp.ErrOrderStatusInvalid }`。而 order_service.go:556 创建订单时写 `Status: constants.OrderStatusPendingRecharge`。用户下单后调 CreatePayment（钱包扣款）必然收到 ErrOrderStatusInvalid，钱包永远不会被扣，订单永远停在 pending_recharge。 — 建议：将 line 98 的校验改为 `OrderStatusPendingRecharge`，或在 order 创建时直接在事务内完成钱包扣款。
  - [P0] CancelExpiredOrder 只处理 `pending_payment` 状态，新订单 `pending_recharge` 过期后永不自动取消 — `internal/modules/order/application/order_service_child.go:441` — `if order.Status != constants.OrderStatusPendingPayment { return order, nil }`。新订单创建时设了 ExpiresAt（order_service.go:549），但超时取消任务只匹配 pending_payment，导致 pending_recharge 订单永久悬挂、库存/优惠券永不释放。 — 建议：改为 `OrderStatusPendingRecharge`。
  - [P0] 履约服务 CreateManual 要求状态为 `paid`/`fulfilling`，新订单 `processing` 被拒绝 — `internal/modules/fulfillment/application/service.go:110` — `if order.Status != constants.OrderStatusPaid && order.Status != constants.OrderStatusFulfilling { return nil, ErrOrderStatusInvalid }`。新模型下 admin 将 pending_recharge → processing 后，交付时因状态不匹配被拒，无法完成履约。 — 建议：改为 `OrderStatusProcessing`。
  - [P0] 履约完成后写 `delivered` 而非 `completed`（ACTIVE_WRITE_BLOCKER） — `internal/modules/fulfillment/application/service.go:143-146` — `tx.Orders().UpdateFields(order.ID, map[string]interface{}{"status": constants.OrderStatusDelivered, ...})`。交付后订单变成 delivered（旧9态），而非五态机的 completed。 — 建议：改为 `OrderStatusCompleted`。
  - [P1] 风控待单查询全部查 `pending_payment`，对新订单 `pending_recharge` 不可见 — `internal/modules/order/infrastructure/gormstore/order_store.go:685,734,748,771` — CountPendingByUserID / CountPendingGuestByRiskIP / CountPendingMemberByRiskIP / SumPendingGuestQuantityByRiskIP 均用 `constants.OrderStatusPendingPayment`。新订单风控限额形同虚设。 — 建议：改为 `OrderStatusPendingRecharge`（或同时兼容两者）。
  - [P1] Dashboard 待支付订单统计查 `pending_payment` — `internal/modules/dashboard/infrastructure/gormstore/overview.go:48,128` — 管理员后台看不到新订单的待处理量。
- 验证证据
  - 未登录不能下单：PASS。routes_storefront.go:109 `user.Use(middleware.UserJWTAuthMiddleware(...))`，order create/pay/cancel/after-sale 均在 user 组内。
  - Guest Order 不可达：PASS。routes_storefront.go:92 注释明确 `/guest/*` 路由组停用，不再注册。CreateGuestOrder 方法仍存在于 service 但无 HTTP 路由暴露。
  - Wallet 余额不足 fail-closed：PARTIAL。order_service.go:492-512 预校验余额；wallet/apply_order_balance.go:63-64 余额不足返回 ErrInsufficientBalance。但预校验在事务外，实际扣款在 payment_service_create.go 的事务内——由于 P0 #1（状态不匹配），扣款根本不会执行。
  - 无有效 Global Rate fail-closed：PASS。order_service.go:478-484 rateResolver.Resolve 出错直接 return nil, rErr；rRate <= 0 返回 ErrInsufficientBalance。exchangerate/service.go:68 三级失败后返回 ErrRateUnavailable。无 1:1 fallback。
  - 商品订单不创建 Gateway Payment：PARTIAL。CreatePayment 仍可被调用（路由在 user 组内），但因 P0 #1 状态不匹配，实际上 gateway payment 创建会被 ErrOrderStatusInvalid 拒绝。代码路径上 payment_service_create.go:222-244 仍有创建 gateway Payment 的逻辑，属于"不可达但未拆除"。
  - Wallet + Online 混合支付不可达：PARTIAL。payment_service_create.go:161-165 有 ReleaseWalletBalance 逻辑（用户改在线支付时退余额），但因 P0 #1，整条混合路径不可达。代码层面仍存在混合支付分支（UseBalance=false + ChannelID>0）。

### 二、五态状态机审计
- 结论：FAIL
- 发现列表
  - [P0] markOrderPaid 写入 `paid` / `fulfilling` 主状态（ACTIVE_WRITE_BLOCKER） — `internal/modules/payment/application/payment_service_callback.go:443,454,456` — `orderRepo.UpdateStatus(order.ID, constants.OrderStatusPaid, ...)` 以及子订单写 `OrderStatusPaid` / `OrderStatusFulfilling`。这是支付回调成功后主动写旧主状态。 — 建议：钱包-only 模式下不应走 markOrderPaid；若保留兼容旧单，应在 wallet-only 开关下短路。
  - [P0] Admin UpdateOrderStatus 允许父订单写 `partially_refunded` / `refunded`（ACTIVE_WRITE_BLOCKER） — `internal/modules/order/application/order_service_child.go:243-286` — `case constants.OrderStatusPartiallyRefunded, constants.OrderStatusRefunded:` 分支直接 `orderStore.UpdateStatus(order.ID, target, ...)` 到父订单和所有子订单。绕过 ordermachine。 — 建议：删除此分支，退款主状态归一由 refund_status 独立列承担。
  - [P0] CalcParentStatus 返回并写入旧状态 — `internal/modules/order/application/order_status.go:76-99` — 返回 `OrderStatusRefunded`/`PartiallyRefunded`/`Delivered`/`PartiallyDelivered`/`Fulfilling`/`Paid`/`PendingPayment`。SyncParentStatus（line 35）把这些旧状态 UpdateStatus 到父订单。 — 建议：重写 CalcParentStatus 基于五态（pending_recharge/processing/completed/failed/canceled）聚合。
  - [P1] order_service.go 内 `allowedTransitions` 旧9态 map 仍存在且被 IsTransitionAllowed 使用 — `internal/modules/order/application/order_service.go:205-241`、`order_service_child.go:481-490` — IsTransitionAllowed 在 order_service_child.go:255 被父订单退款分支调用，使用旧 map 判断子订单迁移合法性。对新五态订单，IsTransitionAllowed("processing", "completed") 查不到 key → false → 误拒。
  - [P1] 履约服务写完 `delivered` 后，父订单 SyncParentStatus 聚合也会产生旧状态链 — `internal/modules/fulfillment/application/service.go:143` → 后续 child 状态变 delivered → SyncParentStatus → CalcParentStatus → 父变 delivered/partially_delivered。
- 验证证据
  - ordermachine/machine.go 定义了正确的五态 allowed map（line 44-55）和 Normalize（line 16-42），但生产代码并未统一走它。UpdateOrderStatus（order_service_child.go:190）虽然调了 `ordermachine.CanTransition`，但 line 189 豁免了 partially_refunded/refunded，line 243-286 又直接写旧状态。
  - 全局搜索 `UpdateStatus` 写订单主状态的位置：order_service_child.go:46,50,248,258,307,372,382,400；payment_service_callback.go:443,458,473；fulfillment/service.go:143；procurement/application/submit.go:170。其中多处写旧状态。

### 三、退款语义审计
- 结论：PARTIAL
- 发现列表
  - [P1] AdminManualRefund 创建退款记录时 currency 取 `order.Currency`（Site Currency），但 USDT 订单的退款金额是 USDT — `internal/modules/order/application/refund/service.go:532-535` — `currency := strings.ToUpper(strings.TrimSpace(order.Currency))`。对比 refund/wallet.go:122-127 正确使用了 `refundCurrency = "USDT"`。adminManualRefundInTx（service.go:373-376）正确用 WalletPaidAmount 作为 paidBase，但 createRefundRecordTx 仍写 order.Currency。导致退款记录币种字段与金额单位不一致。 — 建议：createRefundRecordTx 接收 refundCurrency 参数，USDT 订单传 "USDT"。
  - [P2] ReleaseWalletBalance 退款后把 `online_paid_amount` 写回 `order.TotalAmount`（Site Currency） — `internal/modules/order/application/order_wallet_bridge.go:116` — `"online_paid_amount": money.FromDecimal(order.TotalAmount.Decimal.Round(2))`。USDT 订单的 online_paid_amount 应为 0（wallet-only），此处回填 Site Currency 数值。终态订单影响有限，但语义混淆。
- 验证证据
  - completed + partial refund → status=completed, refund_status=partial：PASS。refund/wallet.go:160-165 只写 refund_status，不写主状态。
  - completed + full refund → status=completed, refund_status=full：PASS。同上。
  - failed 自动退款 → status=failed, refund_status=full：PASS。order_service_child.go:396-399 cancelSingleOrderInTx 写 refund_status=full；order_wallet_bridge.go:114-120 ReleaseWalletBalance 同步写 refund_status=full。
  - canceled 自动退款 → status=canceled, refund_status=full：PASS。order_service_child.go:43-45,104 cancelOrderWithChildren 写 refund_status=full + ReleaseWalletBalance。
  - 新退款链不写 partially_refunded/refunded 到主状态：PARTIAL。refund/wallet.go 和 service.go 的 adminManualRefundInTx 确实不写主状态（line 158-165 注释明确）。但 order_service_child.go:243-286 的 admin 手动设置旧退款主状态分支仍存在。

### 四、USDT 资金语义审计
- 结论：FAIL
- 发现列表
  - [P1] Commission 按比例回退时分母用 `order.TotalAmount`（Site Currency），分子 refundDelta 是 USDT — `internal/modules/affiliate/application/commission.go:167,178,223` — `totalAmount := order.TotalAmount.Decimal`（CNY），但 `delta`（refundDelta）来自 refund/wallet.go:175 传的 USDT amount。`remaining = totalAmount - before` 混币。`deduct = currentCommission * delta / remaining` 比例错误。例：订单 100 CNY = 14.29 USDT，退 5 USDT，正确应扣 5/14.29=35% 佣金，实际扣 5/100=5%。 — 建议：HandleOrderRefunded 对 USDT 订单改用 `order.WalletPaidAmount.Decimal` 作为分母。
  - [P1] 创建子订单时 `OnlinePaidAmount` 被设为 Site Currency 金额 — `internal/modules/order/application/order_service.go:627` — `OnlinePaidAmount: money.FromDecimal(normalizeOrderAmount(plan.TotalAmount.Sub(plan.CouponDiscount)))`。wallet-only 模式下 online_paid_amount 应为 0。这是 Site Currency 数值写入 OnlinePaidAmount 字段。
  - [P2] ReleaseWalletBalance 退款后 online_paid_amount 回填 TotalAmount（Site Currency） — 见维度三。
- 验证证据（不变量成立的部分）
  - Product Price = Site Currency：PASS。catalog product price 来自商品表，order item UnitPrice 快照。
  - Wallet = USDT：PASS。wallet/account.go balance 字段，ApplyOrderBalance 对 USDT 订单传 ledgerCurrency="USDT"（order_wallet_bridge.go:42）。
  - Order total_amount = Site Currency：PASS。order.go:29 TotalAmount 注释"实付金额"，createOrder 传 result.TotalAmount（Site）。
  - Order wallet_paid_amount = USDT：PASS。order_wallet_bridge.go:40-41,67 对有 UsdtTotalAmount 的单用 USDT 扣款额写入 wallet_paid_amount。
  - Refund = USDT（快照）：PASS。refund/wallet.go:124-127 用 WalletPaidAmount(USDT) 作 paidBase，refundCurrency="USDT"。不重新换算。
  - Commission = USDT（基于 wallet_paid_amount）：PARTIAL。calculateCommissionBaseAmount（commission.go:305-307）正确用 ExchangeRate 快照把 Site Currency base 换算为 USDT。但 HandleOrderRefunded 比例回退分母错误（见上）。
  - Wallet Ledger = USDT：PASS。order_balance.go:76 ledger Currency=normalizeCurrency(input.Currency)，USDT 订单传 "USDT"。

### 五、Global Rate 审计
- 结论：PASS
- 验证证据
  - CoinGecko Provider：PASS。`internal/modules/exchangerate/infrastructure/provider/coingecko.go` 存在。
  - API Key 只在 Admin Settings：PASS。UpdateConfig（service.go:130-150）存 state.APIKey；Refresh（line 94）传 state.APIKey 给 provider.Fetch。
  - Public/User API 不泄露 Key：PASS。exchangerate transport 只有 admin_handler.go，无 public/user handler。
  - Admin GET 只返回 masked key：PASS。admin_handler.go:46 `"api_key_masked": maskKey(state.APIKey)`，maskKey（line 117-126）只暴露后4位。
  - 自动/manual fallback 正常：PASS。service.go:53-66 先 AUTO 后 MANUAL，三级失败返回 ErrRateUnavailable。
  - Redis 只是 cache：需进一步确认 rediscache/cached_store.go，但 settingsstore/store.go 是持久化真源。
  - 无 AUTO + Manual → 拒绝下单：PASS。order_service.go:478-481 Resolve 出错直接 return。
  - 无 1:1 fallback：PASS。service.go:101 rate<=0 返回错误；Refresh 失败不写 1.0。
  - Gateway Rate 与 Global Rate 隔离：PASS。Gateway exchange_rate 在 `payment/infrastructure/gateway/common/exchange.go`（渠道配置级），Global Rate 在 `exchangerate/`。两者无交叉引用。
- 发现列表
  - [P2] rateResolver 未注入时静默跳过汇率换算 — order_service.go:477 `if s.rateResolver != nil`。若生产 bootstrap 忘记注入 SetRateResolver，则 orderUsdtTotal=0，后续所有 USDT 逻辑走 legacy 分支（按 Site Currency 扣款）。fail-open 而非 fail-closed。 — 建议：生产 bootstrap 必须注入；或在 NewOrderService 后加启动校验。

### 六、Wallet Recharge 审计
- 结论：PASS
- 验证证据
  - Gateway Payment 只服务 Wallet Recharge：PARTIAL。payment 模块仍有大量 business order gateway payment 代码（payment_service_create.go、callback.go），但因 P0 #1（状态不匹配），business order 走不通。Wallet recharge 走 wallet/application/recharge.go + payment callback。
  - Wallet Recharge 正常创建 Payment：PASS。recharge.go:13-33 ApplyRechargePayment 在支付成功后 CreditInTransaction。
  - callback/webhook 正常：PASS。payment_service_callback.go 处理 gateway 回调。
  - Gateway exchange_rate 不被 Business Order 使用：PASS。同维度五隔离验证。
  - Global Rate 不影响 Wallet Recharge：PASS。recharge.Amount 是 gateway 侧换算后的 USDT 金额，不经过 Global Rate。
- 发现列表
  - [P2] payment 模块仍保留 business order gateway payment 全链路代码（create/callback/markOrderPaid），属于历史遗留死代码/危险代码。当前因状态不匹配不可达，但如果有人修复 P0 #1 时只改状态校验而不拆除 gateway 分支，会导致混合支付复活。

### 七、After-Sale 审计
- 结论：PASS
- 验证证据
  - 售后不产生第六主状态：PASS。aftersale/service.go:73,104,146 只 UpdateFields order.after_sale_status，不动 order.status。
  - partial/full refund 走 AdminRefundToWalletInTx：PASS。aftersale/adapter.go:27 → refund.Service.AdminRefundToWalletInTx。
  - Wallet 真正 credit：PASS。refund/wallet.go:137 CreditInTransaction。
  - Ledger 写入：PASS。CreditInTransaction 内部 CreateTransaction。
  - refund_status 正确：PASS。refund/wallet.go:160-165 partial/full。
  - Commission reverse：PASS。refund/wallet.go:171-181 调 affiliateRefund.HandleOrderRefunded（在 tx 内）。
  - ticket/order/wallet/refund/commission 同一事务：PASS。aftersale/service.go:126-151 WithinTransaction 内锁 ticket → RefundInTx → UpdateAfterSaleTicket → UpdateFields(after_sale_status)。
  - refund 失败整体 rollback：PASS。任一 return err 均回滚整个 tx。
  - 不存在重复退款：PASS。refund/wallet.go:103 GetByIDForUpdate 锁订单；line 131-134 refundable 校验；wallet credit 用 reference 幂等（credit.go:42-58）。
- 发现列表
  - 无新增 P0/P1。

### 十三、事务 / 并发
- 结论：PARTIAL
- 发现列表
  - [P1] Wallet debit row lock：PASS。ensureAccountForUpdate（credit.go:142-146）调 GetAccountByUserIDForUpdate。PostgreSQL 下为 SELECT FOR UPDATE。Windows/SQLite 下单连接池下行锁语义弱化，但生产 PostgreSQL 正确。
  - [P1] AdminRefundToWalletInTx 事务边界：PASS。refund/wallet.go:48 WithinTransaction 包裹整个 InTx；line 103 GetByIDForUpdate 锁订单行。
  - [P1] After-Sale refund 事务：PASS。见维度七。
  - [P2] cancelOrderWithChildren 内 ReleaseWalletBalance 的 claim 回调（order_wallet_bridge.go:111-126）先 claim 再 credit，防双退。PASS。
  - [P2] 未发现 nested independent transaction：order 事务均通过 WithinTransaction 传入 tx，子模块使用 tx.Wallets()/tx.Orders() 同一连接。
- 验证证据（Windows/SQLite 语义差异）
  - SQLite 不支持 SELECT FOR UPDATE，GetAccountByUserIDForUpdate 在 SQLite 下退化为普通读。生产 PostgreSQL 下行锁有效。审计环境为 Windows，无法真实验证并发行锁，标记为已知差异。

### 十四、Commission / Affiliate
- 结论：PARTIAL
- 发现列表
  - [P1] Commission 比例回退分母币种错误 — 见维度四（commission.go:167 用 TotalAmount=Site Currency，但 delta 是 USDT）。
  - [P2] HandleOrderCanceled（commission.go:116-150）把 pending_confirm/available 佣金直接置 rejected，不按比例。对 failed/canceled 全额退是正确的（全额 reverse）。
  - [P2] 已进入提现流程的佣金（WithdrawRequestID != nil）跳过不回退（commission.go:138-141,204-207）。业务规则可接受，但意味着已提现部分无法追回，需运营线下处理。
- 验证证据
  - commission = USDT：PASS（calculateCommissionBaseAmount line 305-307 正确换算）。
  - completed 正常确认：PASS。HandleOrderPaid line 22-103，ConfirmDueCommissions line 107-113。
  - failed/canceled rollback：PASS。HandleOrderCanceled line 116-150。
  - partial refund 按比例 reverse：PARTIAL。比例逻辑存在（line 222-223），但分母币种错误（见上）。
  - full refund 全量 reverse：PASS。refund/wallet.go 全额退 → HandleOrderRefunded 多次累积或一次全量，nextCommission<=0 时置 rejected。

### 十五、Parent / Child Order
- 结论：FAIL
- 发现列表
  - [P0] Admin 可将父订单设为 partially_refunded/refunded 并级联子订单 — order_service_child.go:243-286。父子双写旧退款主状态。
  - [P1] completeParentOrderInTx 要求子订单 status=delivered（旧态） — order_service_child.go:379 `if child.Status != constants.OrderStatusDelivered { return ErrOrderStatusInvalid }`。新模型子订单履约后应是 completed，不是 delivered。
  - [P1] canCompleteParentOrder 要求父 status=delivered — order_service_child.go:470 `if order.Status != constants.OrderStatusDelivered { return false }`。新模型父订单应在 processing 时允许 complete。
  - [P1] SyncParentStatus/CalcParentStatus 不识别 pending_recharge/processing/failed — order_status.go:54-71 switch 无这些 case。新五态子订单状态变化不会正确聚合到父订单。
- 验证证据
  - 父子取消：PASS。cancelOrderWithChildren（order_service_child.go:46-53）父+子都写 canceled，ReleaseWalletBalance 只对父单调一次（line 104），子单不重复退。
  - 父子 failed：PARTIAL。cancelSingleOrderInTx 处理单子；父单 failed 走 UpdateOrderStatus → cancelSingleOrderInTx。
  - 父子退款 refund_status 一致性：PARTIAL。refund/wallet.go:169-170 注释"此处不做父子状态同步"。父单手动退款不联动子单 refund_status。
  - 双退款防护：PASS。ReleaseWalletBalance 的 claim 回调（UpdateFieldsWhereWalletPaid）幂等；refund/wallet.go:131-134 refundable 校验。

## 旧状态词全局搜索分类表
| 状态词 | 出现位置（生产代码） | 分类 | 说明 |
|---|---|---|---|
| pending_payment | payment_service_create.go:98 | ACTIVE_WRITE_BLOCKER（读校验） | 阻止新订单钱包支付 |
| pending_payment | payment_service_callback.go:281,350 | ACTIVE_WRITE_BLOCKER（读校验） | gateway 回调判断订单是否开放 |
| pending_payment | order_store.go:685,734,748,771 | LEGACY_READ_ONLY（风控查询） | 风控查不到新订单 |
| pending_payment | order_service_child.go:441 | ACTIVE_WRITE_BLOCKER（读校验） | 超时取消不处理新订单 |
| pending_payment | dashboard/overview.go:48,128 | LEGACY_READ_ONLY | 仪表盘统计 |
| pending_payment | order_status.go:69,98 | ACTIVE_WRITE_BLOCKER（读+写） | CalcParentStatus 写 pending_payment |
| paid | payment_service_callback.go:443,446,454 | ACTIVE_WRITE_BLOCKER（写） | markOrderPaid 写 paid |
| paid | fulfillment/service.go:110,226 | ACTIVE_WRITE_BLOCKER（读校验） | 履约拒绝新 processing 单 |
| paid | procurement/submit.go:170 | ACTIVE_WRITE_BLOCKER（写） | 采购提交写 paid |
| paid | order_status.go:65,95 | ACTIVE_WRITE_BLOCKER（读+写） | CalcParentStatus |
| paid | order_service_child.go:184 | NORMALIZE_COMPAT | 拦截 admin 直接设 paid（正确） |
| fulfilling | fulfillment/service.go:110 | ACTIVE_WRITE_BLOCKER（读校验） | 同上 |
| fulfilling | payment_service_callback.go:456 | ACTIVE_WRITE_BLOCKER（写） | 子订单写 fulfilling |
| fulfilling | order_status.go:67,91 | ACTIVE_WRITE_BLOCKER（读+写） | CalcParentStatus |
| delivered | fulfillment/service.go:144 | ACTIVE_WRITE_BLOCKER（写） | 履约后写 delivered |
| delivered | order_status.go:63,86 | ACTIVE_WRITE_BLOCKER（读+写） | CalcParentStatus |
| delivered | order_service_child.go:227,379,470,474 | ACTIVE_WRITE_BLOCKER（读校验） | 父子完成判断依赖 delivered |
| partially_delivered | order_status.go:89 | ACTIVE_WRITE_BLOCKER（写） | CalcParentStatus |
| partially_refunded | order_service_child.go:189,243,252,258,273 | ACTIVE_WRITE_BLOCKER（写） | admin 手动设父订单部分退款 |
| partially_refunded | order_status.go:59,80 | ACTIVE_WRITE_BLOCKER（读+写） | CalcParentStatus |
| refunded | order_service_child.go:189,243,248,258,270,273 | ACTIVE_WRITE_BLOCKER（写） | admin 手动设父订单已退款 |
| refunded | order_status.go:57,77 | ACTIVE_WRITE_BLOCKER（读+写） | CalcParentStatus |
| pending_payment/paid | ordermachine/machine.go:28 | NORMALIZE_COMPAT | Normalize 归一（正确） |

## USDT 混淆点清单
| 位置 | 问题 | 严重度 |
|---|---|---|
| affiliate/commission.go:167,178,223 | HandleOrderRefunded 用 TotalAmount(Site Currency) 作比例分母，delta 是 USDT | P1 |
| order_service.go:627 | 子订单 OnlinePaidAmount 初始化为 Site Currency 金额 | P1 |
| order_wallet_bridge.go:116 | ReleaseWalletBalance 把 online_paid_amount 回填为 TotalAmount(Site) | P2 |
| refund/service.go:532 | createRefundRecordTx 用 order.Currency(Site) 作退款记录币种，USDT 订单应为 USDT | P1 |

## ACTIVE_WRITE_BLOCKER 清单（必须为0）
共发现以下生产代码路径主动写旧主状态（非 Normalize 兼容读取）：
1. payment_service_callback.go:443 — `orderRepo.UpdateStatus(order.ID, OrderStatusPaid, ...)`
2. payment_service_callback.go:458 — 子订单 `UpdateStatus(child.ID, OrderStatusPaid/Fulfilling, ...)`
3. payment_service_callback.go:473 — 父订单 `UpdateStatus(order.ID, parentStatus, ...)`（parentStatus 来自 CalcParentStatus，可能是旧态）
4. fulfillment/service.go:144 — `UpdateFields(order.ID, {"status": OrderStatusDelivered})`
5. procurement/submit.go:170 — `orderRepo.UpdateStatus(localOrder.ID, OrderStatusPaid, ...)`
6. order_service_child.go:248 — 父订单 `UpdateStatus(order.ID, target)` target=partially_refunded/refunded
7. order_service_child.go:258 — 子订单 `UpdateStatus(child.ID, target)` target=partially_refunded/refunded
8. order_status.go:35 (SyncParentStatus) → CalcParentStatus 返回旧态后 `orderStore.UpdateStatus(parent.ID, newStatus, ...)`

## 总结

### P0 修复清单
1. **payment_service_create.go:98** — CreatePayment 状态校验从 `OrderStatusPendingPayment` 改为 `OrderStatusPendingRecharge`（或在 wallet-only 模式下直接放行 pending_recharge）。这是当前阻断全部钱包下单的根因。
2. **order_service_child.go:441** — CancelExpiredOrder 状态校验改为 `OrderStatusPendingRecharge`。
3. **fulfillment/service.go:110,144** — 履约准入状态改为 `OrderStatusProcessing`；履约完成写 `OrderStatusCompleted` 而非 `Delivered`。
4. **payment_service_callback.go:443,454,456,471** — markOrderPaid 整条写 paid/fulfilling 的链路在 wallet-only 模式下应短路或拆除；至少不应在新五态订单上写旧态。
5. **order_service_child.go:243-286** — 删除 admin 直接设父子订单 partially_refunded/refunded 的分支，退款主状态由 refund_status 独立列承担。

### P1 修复清单
1. order_store.go:685,734,748,771 — 风控待单查询状态改为 `OrderStatusPendingRecharge`。
2. dashboard/overview.go:48,128 — 仪表盘统计改为 pending_recharge。
3. order_status.go CalcParentStatus — 重写为基于五态（pending_recharge/processing/completed/failed/canceled）聚合。
4. order_service_child.go:379,470 — completeParentOrderInTx / canCompleteParentOrder 接受 processing/completed 而非 delivered。
5. affiliate/commission.go:167 — HandleOrderRefunded 对 USDT 订单用 WalletPaidAmount 作比例分母。
6. refund/service.go:532 — createRefundRecordTx 接收 refundCurrency 参数，USDT 订单写 "USDT"。
7. order_service.go:627 — wallet-only 模式下子订单 OnlinePaidAmount 初始化为 0。
8. order_service.go:205-241 allowedTransitions 旧 map — 确认 IsTransitionAllowed 调用点是否仍需要，迁移到 ordermachine.CanTransition。

### P2 backlog
1. order_wallet_bridge.go:116 — ReleaseWalletBalance 后 online_paid_amount 回填语义修正。
2. payment 模块 business order gateway 死代码清理（markOrderPaid / callback / create gateway payment 分支）。
3. rateResolver 未注入时的启动校验（fail-closed）。
4. procurement lifecycle.go 旧状态返回值对齐五态。
