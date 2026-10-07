# HCZ 资金链路核心安全审计报告 (01-money-flow)

- 审计对象：`github.com/Aether-v1/hcz`（Go + Gin + GORM）
- 审计范围：wallet / walletwithdrawal / order(+refund) / payment / procurement / reconciliation / exchangerate / pricing / profitguard / cardsecret / giftcard / coupon+promotion 及其 bootstrap
- 审计方式：只读代码审计（Read/Grep），未修改任何源码
- 审计日期：2026-10-07
- 总体结论：**资金主链路设计相当稳健，未发现 P0/P1 级别可被普通用户直接利用的资金漏洞。** 金额全程使用 `shopspring/decimal`；下单价格、汇率、折扣均服务端计算；余额扣减/退款/提现均在事务内行锁；支付回调先验签再落库并强比对金额；退款累计上限在服务端强制。发现的问题集中在 P3（次要错误码、管理员操作无金额上限、状态校验不完整等）。

---

## 发现列表

### ISSUE-M01
- Severity: P3
- Title: 钱包管理员调账的幂等冲突错误被误判为 500（错误 sentinel 不一致）
- File: internal/modules/wallet/transport/http/admin_handler.go
- Function/Method: AdminAdjustBalance 错误映射 switch
- API: POST /api/v1/admin/wallet/adjust（推测，以 routes_admin.go 为准）
- Table: wallet_transactions
- Root Cause: handler 在文件内自定义了一个全新的 sentinel `ErrIdempotencyConflictTx = errors.New("wallet idempotency conflict")`（第 27 行），但 service 层实际返回的是 `walletcontract.ErrIdempotencyConflict`。`errors.Is(err, ErrIdempotencyConflictTx)`（第 398 行）永远为 false，导致命中 default 分支返回 500，而非 409 Conflict。
- Exploit/Trigger: 同一 Idempotency-Key 重放时，第二次请求本应幂等返回首调结果；当底层抛出唯一索引冲突时，客户端收到 500 而非 409/重放结果。资金不会重复入账（DB unique 兜底），仅为语义错误。
- Impact: 错误码/语义错误，不造成资金损失。可能让客户端/运维误判为系统故障。
- Evidence:
  ```go
  // admin_handler.go:27
  ErrIdempotencyConflictTx = errors.New("wallet idempotency conflict")
  // admin_handler.go:398 —— 比较的是本文件私有 sentinel，非 walletcontract.ErrIdempotencyConflict
  case errors.Is(err, ErrIdempotencyConflictTx):
      c.JSON(http.StatusConflict, ...)
  ```
- Recommended Fix: 将第 398 行改为 `case errors.Is(err, walletcontract.ErrIdempotencyConflict):`，并删除/对齐私有 sentinel；或在 handler 顶层直接 `errors.Is(err, walletcontract.ErrIdempotencyConflict)`。

### ISSUE-M02
- Severity: P3
- Title: money.Amount 解析 JSON 数字时经 float64 中间态
- File: internal/shared/money/amount.go
- Function/Method: UnmarshalJSON
- API: 所有接收 money.Amount 作为 body 字段的接口
- Table: N/A（解析层）
- Root Cause: 当客户端以 JSON number（非字符串）提交金额时，先反序列化到 `float64` 再 `decimal.NewFromFloat(value)`。字符串路径走 `decimal.NewFromString`（精确），数字路径走 float64。
- Exploit/Trigger: 客户端提交 `{"amount": 0.1}` 这类数字字面量。`decimal.NewFromFloat` 会恢复该 float64 实际表示的精确十进制值，且末尾 `.Round(2)` 收敛到分，因此不会产生 0.1+0.2 类的累积误差被放大。仅当某金额字段被允许超过 2 位小数时，精度在 Round(2) 处被截断（而非四舍五入到分以外）。
- Impact: 低。核心资金计算全部在 decimal 域内完成，未发现 float64 参与余额加减/汇率乘除。
- Evidence:
  ```go
  // amount.go:44-48
  var value float64
  if err := json.Unmarshal(data, &value); err != nil { return err }
  m.Decimal = decimal.NewFromFloat(value).Round(2)
  ```
- Recommended Fix: 保持现状可接受；如需彻底消除 float 路径，可要求金额一律以 JSON string 提交（字符串路径已精确），或在 bind 层拒绝非字符串金额。

### ISSUE-M03
- Severity: P3
- Title: 管理员人工调账/退款入金无金额上限与双人复核
- File: internal/modules/wallet/application/admin.go (AdminAdjustBalance)；internal/modules/order/application/refund/wallet.go (AdminRefundToWalletInTx)
- Function/Method: AdminAdjustBalance / AdminRefundToWalletInTx
- API: 管理员侧（/api/v1/admin/*，JWT + Casbin RBAC + Step-Up）
- Table: wallet_transactions / orders
- Root Cause: 调账与退款入金均校验金额 >0、必填 remark、记录 operator_admin_id、要求 Idempotency-Key（调账）与 Step-Up，但没有单日累计上限、单笔上限或二级审批。
- Exploit/Trigger: 一个被攻陷或恶意的管理员账号（已通过 Step-Up）可任意加余额或任意金额退款入金。属内部威胁模型。
- Impact: 内部运营风险，非外部用户可触达。
- Evidence:
  ```go
  // wallet/application/admin.go —— AdminAdjustBalance 仅做 sign/amount>0，无上限
  if input.Delta.Equal(money.Amount{}) { return ...ErrInvalidAmount }
  ```
- Recommended Fix: 增加单笔/单日调账上限与超限二次审批；审计日志落独立表并留存 operator、IP、前后余额（现有 BalanceBefore/After 已具备，建议加对账日审）。

### ISSUE-M04
- Severity: P3
- Title: 管理员退款未校验订单主状态，仅依赖 PaidAt + 累计退款上限
- File: internal/modules/order/application/refund/service.go
- Function/Method: adminManualRefundInTx
- API: POST /api/v1/admin/orders/:id/refund（推测，Step-Up 保护）
- Table: orders / wallet_transactions
- Root Cause: 退款前置校验为 `order.PaidAt == nil`（:379）、退款窗口（:382）、`paidBase>0`（:390）、累计上限（:393-397），但未显式拒绝 `status in (canceled/failed)`。若一个 canceled/failed 订单仍带有非空 PaidAt 与 WalletPaidAmount，管理员对其再次退款入金，理论上可能与取消时的解冻/退款重复。
- Exploit/Trigger: 需管理员主动对异常状态订单手动退款 + Step-Up。普通用户无法触发。是否真会双计取决于 cancel/failed 流程是否同时写了 refunded_amount（**NOT VERIFIED**）。
- Impact: 低（管理员主动操作 + 累计 refunded_amount 上限兜底，最大不超过实付额）。
- Evidence:
  ```go
  // refund/service.go:379-397
  if order.PaidAt == nil { return nil, ErrOrderStatusInvalid }
  if IsOrderRefundWindowExpired(...) { return nil, ErrOrderRefundExpired }
  refundedBefore := order.RefundedAmount.Decimal.Round(2)
  refundable := paidBase.Sub(refundedBefore).Round(2)
  if amount.GreaterThan(refundable) { return nil, walletcontract.ErrRefundExceeded }
  ```
- Recommended Fix: 在 :380 后增加 `if order.Status != completed && order.Status != paid { return ErrOrderStatusInvalid }`，明确只允许对已完成/已支付订单退款；并确认 cancel 流程同步写 refunded_amount。

### ISSUE-M05
- Severity: P3
- Title: 退款入金钱包 reference 使用时间戳，未要求客户端幂等键（已被行锁+累计上限缓解）
- File: internal/modules/order/application/refund/wallet.go
- Function/Method: AdminRefundToWalletInTx
- API: POST /api/v1/admin/orders/:id/refund-to-wallet（Step-Up）
- Table: wallet_transactions / orders
- Root Cause: 钱包入金 reference = `order:%d:admin_refund:%d`（time.Now().UnixNano()），每次调用都不同；退款 handler 未像钱包调账那样强制 Idempotency-Key。
- Exploit/Trigger: 管理员双击/重试同一退款。由于函数开头 `GetByIDForUpdate` 行锁订单 + 累计 `refundedBefore` 校验，第二次会读到已更新的 refunded_amount，若金额超过剩余可退则被 `ErrRefundExceeded` 拒绝。因此不会超退。
- Impact: 无实际资金损失；仅在"两次退款金额之和恰好不超过 paidBase"时会被当作两笔合法退款（这正是设计意图）。
- Evidence:
  ```go
  // refund/wallet.go:91
  reference := fmt.Sprintf("order:%d:admin_refund:%d", input.OrderID, time.Now().UnixNano())
  ```
- Recommended Fix: 可选——为退款入金引入确定性 reference（如 `order:%d:refund:%d`，基于退款批次 id），使重试天然幂等。当前行锁+累计上限已足够。

---

## Money Flow Map（完整链路）

1. **下单**：`POST /api/v1/orders`（UserJWT）→ `create_handler.go` 收集 `items[product_id,sku_id,quantity]`、`coupon_code`、`affiliate_code`、`channel_id`、`use_balance`、`Idempotency-Key`。**无 price/amount/exchange_rate 字段**。
2. **定价**：`order_service.go` 在事务内：查商品/sku 取服务端价格 → `rateResolver.Resolve()` 取全局汇率（fail-closed，AUTO→MANUAL→拒绝）→ 换算 USDT（decimal，CEIL 2dp）→ coupon 折扣由 coupon 服务端计算（`plan.CouponDiscount`）→ 冻结/预留库存（cardsecret `ListAvailableByProductForUpdate` + `Reserve` rowsAffected 校验）→ 锁定 coupon、校验使用次数 → INSERT order（含 `ExchangeRate` 快照、`IdempotencyKey`、fingerprint）。
3. **支付**：`CreatePayment` → 在线金额 + 钱包金额拆分。钱包部分 `ApplyOrderBalance`（`GetAccountByUserIDForUpdate` 行锁 → 校验余额 → 扣减 → 写 ledger，reference=`order:{id}:wallet_pay`）。
4. **支付回调**：渠道 webhook → `HandleSyncCallback`/**先 `VerifyCallback` 验签（webhook.go:51）失败即拒绝** → `HandleCallback` 二次强校验（channelID/orderNo/币种/**amount==payment.Amount**）→ 事务内锁 payment+order → 已成功则幂等直接返回 → 按 `RemainingOnlineAmountCNY` 足额则完成订单（reserve cardsecret、解锁 coupon、发积分、生成多级佣金）；不足额则差额退入钱包（reference=`payment:{id}:underpaid_credit`）。
5. **完成**：`markOrderCompletedInTx` → 履约（发卡密）→ affiliate `HandleOrderCompletedInTx`（base=WalletPaidAmount，按层级 decimal 计算，唯一约束幂等）→ 积分发放（reference 幂等）。
6. **退款**：管理员 `AdminManualRefund` → 锁订单 → 校验 PaidAt/窗口 → `refundable = paidBase - refundedBefore` 累计上限 → 更新 refunded_amount/refund_status → affiliate `HandleOrderRefunded`（按比例 REVERSAL ledger，已出金转 DEBT）→ 积分回滚。`AdminRefundToWalletInTx` 额外把金额 credit 回用户钱包。
7. **提现**：用户 `CreateWithdrawal`（强制 Idempotency-Key + TOTP + min/max + 日限）→ 事务内行锁账户、扣可用余额、写 ledger（reference 确定）、写提现单 pending → 管理员 Approve/MarkProcessing/Complete（状态机，打款 txid 必填）/Reject（退回 request_amount，reference=`withdrawal:refund:{id}` 幂等）。

---

## Money Safety Matrix

| Flow | Transaction | Row Lock | Idempotency | DB Constraint | Result |
|---|---|---|---|---|---|
| 创建订单 | 是（uow.WithinTransaction） | cardsecret FOR UPDATE / coupon FOR UPDATE | 业务预检 + fingerprint 冲突检测 | orders(user_id,idempotency_key) 部分唯一索引 | SAFE |
| 钱包扣款(下单支付) | 是（payment tx 内） | account FOR UPDATE (GetAccountByUserIDForUpdate) | reference=`order:{id}:wallet_pay` | wallet_transactions.reference uniqueIndex | SAFE（并发双扣被行锁串行化，after<0 拒绝） |
| 在线支付回调 | 是 | payment+order FOR UPDATE | 已 success 直接返回不重放 | payment 状态机 | SAFE（验签+金额==订单额） |
| 退款(累计上限) | 是 | order FOR UPDATE | refunded_amount 累计 | orders.refunded_amount | SAFE（60+60 被第二笔 refundable=40 拒绝） |
| 退款入钱包 | 是（order tx 内 CreditInTransaction） | order + wallet | 时间戳 reference（无客户端 key） | wallet_transactions.reference unique | SAFE（行锁+累计上限兜底，见 M05） |
| 佣金发放/回滚 | 是（order tx 或独立 tx） | commissions FOR UPDATE | (order,beneficiary,level,type) 唯一 + duplicate-key 容忍 | affiliate_commissions 唯一 | SAFE |
| 用户提现 | 是 | account FOR UPDATE + withdrawal FOR UPDATE | 强制 Idempotency-Key + reference 预检 | wallet_transactions.reference unique | SAFE |
| 提现拒绝退款 | 是 | withdrawal FOR UPDATE → account FOR UPDATE | refundReference(w.ID) 确定性 | wallet_transactions.reference unique | SAFE |
| 管理员调账 | 是（changeBalance 内 tx） | account FOR UPDATE | Idempotency-Key header 派生 reference | wallet_transactions.reference unique | SAFE（错误码见 M01） |
| 礼品卡兑换 | 是（RedeemTransaction） | card FOR UPDATE | reference=`gift_card:{id}` | wallet_transactions.reference unique | SAFE（重复兑换被状态+唯一拦截） |
| 钱包充值(USDT) | 回调内 | payment+order | 回调幂等 + underpaid reference | payment 状态机 | SAFE（金额来自验签回调==订单额） |

---

## State Machine Matrix

### 订单主状态（ordermachine/machine.go + order_service.go allowedTransitions）
- pending_payment → paid / canceled
- paid → completed / failed / refunded（受服务端状态机约束，admin 不可直接 set paid）
- completed → 不允许跳回 processing/pending（终态方向收敛）
- 非法跳转（completed→processing、canceled→completed、failed→refund）被 `CanTransition` 拒绝；Controller 不直接改 status，一律经 Service。

### 退款状态 refund_status
- none → partial → full（由 refunded_amount 与 paidBase 比较驱动：`markRefunded = newRefunded >= paidBase`，service.go:406）

### 提现状态机（walletwithdrawal/domain/state_machine.go）
| from \ to | approved | rejected | canceled | processing | completed |
|---|---|---|---|---|---|
| pending | ✅ | ✅ | ✅ | ❌ | ❌ |
| approved | ❌ | ✅ | ❌ | ✅ | ❌ |
| processing | ❌ | ✅ | ❌ | ❌ | ✅ |
| completed/rejected/canceled | 终态，无出边 | | | | |
- 自跳转（from==to）一律 false，重复审批被拒。

---

## 已验证的测试场景（代码分析确认）

- ✅ **金额无 float 参与核心计算**：钱包/订单/退款/提现/佣金金额字段均为 `money.Amount`(=decimal)；全局 grep float64 仅命中配置阈值（c2c MinTradeUSDT）、展示比率（affiliate conversion）、分页 math，资金加减/汇率乘除均 decimal。
- ✅ **下单金额不可客户端伪造**：OrderItemRequest 仅含 product_id/sku_id/quantity（create_handler.go），price 从 catalog 服务端读取；请求体无 amount/price/exchange_rate/discount 字段。
- ✅ **汇率不可客户端篡改**：order create 请求无汇率字段；`Resolve()` 服务端取 AUTO/MANUAL/fail-closed，订单落 `ExchangeRate` 快照（exchangerate/service.go:43-96）。
- ✅ **并发双扣防护**：钱包扣减一律 `GetAccountByUserIDForUpdate`（`clause.Locking{Strength:"UPDATE"}`）事务内行锁，`after < 0` → ErrInsufficientBalance。余额100同时两笔80：第二笔行锁等待后读到已扣减余额，余额不足拒绝。
- ✅ **退款累计上限服务端强制**：refund/service.go:393-397 `refundable = paidBase - refundedBefore; if amount > refundable → ErrRefundExceeded`。订单100连续 refund60+refund60：第二笔 refundedBefore=60, refundable=40, 60>40 拒绝，不可能退 120。
- ✅ **支付回调不可伪造**：HandleSyncCallback 在处理前先 `verifier.VerifyCallback(channel.ConfigJSON, form, body)`（webhook.go:51），验签失败直接返回；通过后 `validateCallbackPaymentFacts` 强校验 channelID/orderNo/币种/`input.Amount==payment.Amount`（callback service）。
- ✅ **回调幂等**：payment 已 success 时重放仅更新 meta，不重复履约/退款。
- ✅ **提现超余额防护**：create.go:118 `if before < amount → ErrInsufficientBalance`，行锁账户；min/max/TOTP/日限/首提限齐全。
- ✅ **提现打款幂等**：余额在 CreateWithdrawal 已扣；Complete 仅记 txid；Reject 退款用确定性 `refundReference(w.ID)` + reference 唯一索引。
- ✅ **佣金随退款回滚**：HandleOrderRefunded 按 `delta/remaining` 比例生成 REVERSAL ledger；已出金(PAID)佣金转 DEBT 而非重复扣减。
- ✅ **礼品卡不可重复兑换**：redeem.go:33 `GetByCodeForUpdate` 行锁 + status 校验 + 确定性 reference。
- ✅ **管理员调账有审计**：记录 operator_admin_id、BalanceBefore/After、remark、Step-Up + Idempotency-Key。

### NOT VERIFIED（本次未深入，禁止视为安全）
- ❓ procurement 上游回调：transport/http 仅见 admin_handler/routes，疑似由 poll.go 主动轮询驱动（公网 webhook 入口未确认），上游金额对账细节未逐行核。
- ❓ reconciliation（对账）模块的自动匹配/差异处理逻辑未逐行核。
- ❓ pricing / profitguard 的利润核算与红线拦截未逐行核（仅确认金额类型为 decimal）。
- ❓ cardsecret 卡密发放与订单履约的并发占用（rowsAffected 校验已见，但卡密本身状态机未逐行核）。
- ❓ coupon/promotion 的折扣封顶（discount 是否可能 > total）未逐行核 buildOrderResult。
- ❓ cancel/failed 订单是否同步写 refunded_amount（关系 M04 是否真会双计）未确认。
- ❓ bootstrap 层 wire 是否将上述 service 全部正确注入（编译期应可 `go build` 验证，本次未执行以遵守零改动/只读约束）。

---

## 修复优先级建议
1. M01（错误 sentinel）一行修复，成本最低，建议先改。
2. M04（退款补订单状态校验）一行防御，建议补。
3. M03/M05 属内部运营加固，可排入迭代。
4. M02 可接受，仅在文档中约定"金额一律字符串提交"。

> 说明：本报告仅审计，未修改任何源码。所有结论均附文件与行号；标注 NOT VERIFIED 项需后续专项审计。
