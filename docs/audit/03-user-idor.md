# 03 — 用户业务逻辑与 IDOR / 越权安全审计

- 项目：github.com/Aether-v1/hcz（Go + Gin + GORM）
- 审计日期：2026-10-07
- 审计方式：只读审计（未修改任何源码）。所有结论均追到 Service / Repository（GORM Store）层确认 ownership 是否真正落到 WHERE 子句。
- 鉴权基线：用户端全部端点挂 `UserJWTAuthMiddleware`（`internal/app/httpserver/middleware/middleware.go`）。该中间件校验 JWT 后执行 `c.Set("user_id", claims.UserID)`（约 L382/L430）。Handler 一律通过 `internal/platform/http/ginutil/context.go` 的 `ginutil.GetUserID(c)`（约 L52）取 `user_id`，取不到即 401。**因此 user_id 不可被客户端伪造。**

---

## 审计结论总览

整体 ownership 控制**非常扎实**：绝大多数 `{id}` 端点在 Service 层用 `WHERE id = ? AND user_id = ?` 或 `x.UserID != currentUser` 做了归属校验，并多处采用行锁（`FOR UPDATE`）+ reference 幂等。**仅发现 1 个真实越权缺陷（工单附件跨用户读取），无 P0。**

发现清单：

| ID | Severity | 标题 |
|----|----------|------|
| ISSUE-U01 | P2 | 客服工单附件缺少上传者归属，可跨用户引用并下载他人附件 |

---

### ISSUE-U01
- Severity: P2
- Title: 客服工单附件（support_attachments）未记录上传者 user_id，用户可把他人「未关联」附件挂到自己工单后下载，造成跨用户文件读取
- File:
  - `internal/modules/supportticket/domain/attachment.go`
  - `internal/modules/supportticket/application/user_service.go`
  - `internal/modules/supportticket/transport/http/user_handler.go`
- Function/Method:
  - `linkAttachmentsToMessage`（user_service.go）
  - `UploadAttachment`（user_service.go）
  - `DownloadAttachment`（user_service.go）
- API:
  - POST `/api/v1/support/attachments`（上传，返回 attachment id，此时 `ticket_id=0`）
  - POST `/api/v1/support/tickets` 或 POST `/api/v1/support/tickets/:id/replies`（请求体 `attachment_ids`）
  - GET `/api/v1/support/attachments/:id/download`
- Table: `support_attachments`
- Root Cause:
  1. `Attachment` 领域模型**根本没有「上传者 user_id」字段**，只有 `UploaderType`（user/admin）。
  2. `UploadAttachment` 落库时 `TicketID=0`、`UploaderType=SenderUser`，不记录是谁上传的。
  3. `linkAttachmentsToMessage` 在把附件挂到当前用户的工单/回复时，只校验 `att.TicketID == 0` 与 `att.UploaderType == 当前 uploaderType`，**从不校验附件是否由当前 user 上传**。
  4. `DownloadAttachment` 的归属校验是「附件 → 所属 ticket → ticket.UserID」。一旦攻击者把受害者的附件挂到自己名下的工单，该附件的 `ticket_id` 就变成攻击者的工单，`ticket.UserID == 攻击者`，于是下载被放行。
- Exploit/Trigger:
  1. 受害者正常流程：POST `/support/attachments` 上传敏感文件（身份证/截图），得到 attachment id=N（`ticket_id=0`）。
  2. 攻击者：新建工单（或回复），`attachment_ids` 填入 `[N]`（N 为顺序自增整数，可枚举附近值）。
  3. `linkAttachmentsToMessage` 判定 `att.TicketID==0` 且 `att.UploaderType=="user"` → 校验通过，把附件 N 链接到攻击者工单。
  4. 攻击者 GET `/support/attachments/N/download` → 此时 ticket 归攻击者所有 → 下载成功，读到受害者文件。
  利用窗口：附件处于「已上传未关联」（`ticket_id=0`）状态期间。正常上传到建单之间有数秒窗口；受害者上传后放弃建单则附件永久留在 `ticket_id=0`，窗口无限大。
- Impact: 跨用户敏感文件读取（ID 证件、支付截图、聊天记录等）。属于水平越权 / IDOR。受限于顺序 id 枚举 + 未关联窗口，定级 P2 而非 P1。
- Evidence:
  - `internal/modules/supportticket/domain/attachment.go`（Attachment 结构无 UploaderUserID 字段）：
    ```go
    type Attachment struct {
        ID           uint
        TicketID     uint
        MessageID    uint
        UploaderType string   // 仅 user/admin，无上传者 user_id
        FileName     string
        ObjectKey    string
        ...
    }
    ```
  - `internal/modules/supportticket/application/user_service.go` `UploadAttachment`（不记录上传者）：
    ```go
    att := &supportdomain.Attachment{
        TicketID:     0,
        UploaderType: statemachine.SenderUser,
        FileName:     result.Filename,
        ...
    }
    ```
  - `internal/modules/supportticket/application/user_service.go` `linkAttachmentsToMessage`（只校验 Type，不校验上传者）：
    ```go
    att, err := s.attachments.GetByID(ctx, id)
    if att == nil || att.TicketID != 0 || att.UploaderType != uploaderType {
        return nil // 拒绝已关联或非用户上传，但不校验 att 属于哪个 user
    }
    ```
- Recommended Fix:
  1. `support_attachments` 表增加 `uploader_user_id BIGINT NULL`，`UploadAttachment` 落库时写入当前 user_id。
  2. `linkAttachmentsToMessage` 增加校验：`att.UploaderUserID != currentUserID` → 拒绝（视为不存在）。
  3. 对已上传未关联附件设置 TTL/GC，避免长期悬挂可被枚举。
  4. （纵深）附件 ObjectKey 路径保持不可猜测随机串（现状已是 UUID 路径，保留）。

---

## IDOR Matrix（用户端所有带路径参数 `{id}` / `{order_no}` 的端点）

| Endpoint | Resource | Ownership Check（追到 Store/DB） | Result |
|----------|----------|----------------------------------|--------|
| GET `/orders/:order_no` | 订单详情 | `order_store.go` `GetByOrderNoAndUserScoped`: `WHERE order_no=? AND user_id=? AND parent_id IS NULL` | PASS |
| GET `/orders/:order_no/fulfillment/download` | 发货内容下载 | `GetFulfillmentByOrderNoAndUserScoped`: `WHERE order_no=? AND user_id=?` | PASS |
| POST `/orders/:order_no/cancel` | 取消订单 | `order_store.go` `GetByIDAndUser`: `WHERE id=? AND user_id=? AND parent_id IS NULL` | PASS |
| POST `/orders/:order_no/after-sale` | 申请售后 | `aftersale/service.go` `Request`: `GetByIDAndUser(orderID, userID)`（user_id 匹配 + 行锁） | PASS |
| GET `/orders/:order_no/after-sale` | 查询售后 | `GetByIDAndUser(orderID, uid)` | PASS |
| POST `/payments/:id/capture` | 捕获支付 | `write_handler.go` L166 `GetOrderByUserForTenant(payment.OrderID, uid)` 后才 capture | PASS |
| GET `/wallet/recharges/:recharge_no` | 充值记录 | `wallet/gormstore/store.go` L161 `GetByRechargeNo`: `WHERE user_id=? AND recharge_no=?` | PASS |
| POST `/wallet/recharge/payments/:id/capture` | 充值支付捕获 | `GetRechargeOrderByPaymentIDAndUser`: `WHERE payment_id=? AND user_id=?` | PASS |
| GET `/wallet/transactions` | 钱包流水列表 | `ListByUser` when `filter.UserID!=0`（无单条 {id} 详情端点） | PASS |
| GET `/wallet/withdraws/:id`（/wallet/withdraw/detail） | 提现详情 | `walletwithdrawal/application/query.go`: `w.UserID != userID` → 404 | PASS |
| POST `/wallet/withdraws/:id/cancel` | 取消提现 | `cancel.go`: 行锁后 `w.UserID != input.UserID`（+TOTP） | PASS |
| GET `/wallet/withdraws/addresses/:id` 删/设默认 | 提现地址 | `address.go`: `a.UserID != userID` → ErrNotOwner | PASS |
| GET `/points/account`、GET `/points/ledger` | 积分账户/流水 | handler 取 JWT uid，无 {id}，按 user 维度 | PASS |
| GET `/points/exchange-orders/:id` | 积分商城兑换单 | `pointsmall/store.go`: `WHERE id=? AND user_id=?` | PASS |
| POST `/points/exchange-orders/:id/cancel` | 取消兑换 | `pointsmall/service.go`: 事务内行锁 `order.UserID != input.UserID` | PASS |
| POST `/checkin/checkin` | 签到 | 仅 JWT uid；唯一索引 `(user_id, checkin_date)` 兜底并发重复奖励 | PASS |
| GET `/notifications/:id/read` | 标记通知已读 | `usernotification/gormstore/store.go`: `WHERE id=? AND user_id=?`；rows==0 → 404 | PASS |
| GET `/notifications`、未读数、全部已读 | 通知列表 | 全部 `WHERE user_id=?` | PASS |
| GET `/support/tickets/:id` | 工单详情 | `user_service.go`: `ticket.UserID != userID` → 404 | PASS |
| POST `/support/tickets/:id/replies` | 工单回复 | 事务内行锁 `ticket.UserID != in.UserID` | PASS |
| POST `/support/tickets/:id/close`、`/reopen` | 关闭/重开工单 | `ticket.UserID != userID` | PASS |
| GET `/support/attachments/:id/download` | 工单附件下载 | 校验「附件→ticket→ticket.UserID」——但因 ISSUE-U01 附件本身可被挂错工单，故链路存在缺陷 | **FAIL (ISSUE-U01)** |
| POST `/c2c/payment-methods/:id` PUT/DELETE/enabled | C2C 支付方式 | `payment_method.go` `GetPaymentMethodByIDForUser`: `pm.UserID != userID` | PASS |
| GET/PUT `/c2c/listings/:id`、pause/resume/close | C2C 挂单 | `listing.go` `ownedListing`: `l.SellerUserID != userID` | PASS |
| GET `/c2c/trades/:id` | C2C 交易详情 | `trade.go` `GetTradeDetail`: `t.BuyerUserID!=u && t.SellerUserID!=u` → 404 | PASS |
| POST `/c2c/trades/:id/mark-paid` | 标记付款 | `t.BuyerUserID != input.BuyerUserID` → 拒绝（仅买家） | PASS |
| POST `/c2c/trades/:id/cancel` | 取消交易 | `t.BuyerUserID != input.BuyerUserID`（仅买家） | PASS |
| POST `/c2c/trades/:id/confirm` | 卖家放币 | `t.SellerUserID != input.SellerID`（仅卖家，资金结算） | PASS |
| POST `/c2c/trades/:id/dispute` | 发起争议 | `t.BuyerUserID!=u && t.SellerUserID!=u` → ErrPermissionDenied | PASS |
| GET `/cart`、POST/PUT/DELETE `/cart/items` | 购物车 | 全部以 JWT uid 为键；价格从商品目录读取，不接受客户端价 | PASS |
| GET `/apicredentials`、POST、regenerate、active | 用户 API 凭证 | 全部按 JWT userID 查询/变更，无 {id}；secret 仅掩码 tail | PASS |
| GET `/auditlog/login-logs` | 用户登录日志 | `ListByUser(uid)` | PASS |
| GET `/invitation/me` | 我的邀请 | `GetMyInvitation(userID)`，无 {id} | PASS |
| GET `/affiliate/*`、commissions、withdraws、transfer | 推广返利 | 全部 JWT uid；TransferToWallet 显式注释「禁止请求体传 user_id」 | PASS |
| GET `/reseller/orders/:order_no` | 分销订单详情 | `order_query.go`: `requireActiveProfileByUser(uid)` → `GetOrderSnapshotByResellerOrderNo(profile.ID, orderNo)`（按 reseller_id 隔离） | PASS |
| GET/PUT `/reseller/product-settings/:product_id` | 分销商品配置 | 经 reseller profile（由 userID 推导）作用域 | PASS |
| GET `/gift-cards/redeem` | 礼品卡兑换 | code 型，无 {id}；`GetByCodeForUpdate` 行锁 + 状态校验 + `gift_card:{id}` reference 幂等 | PASS |
| GET `/members/*`、promotion preview | 会员/活动价 | 无用户可指定的 {id}；价格服务端按 catalog 计算 | PASS（NOT VERIFIED 深入刷量） |

> 说明：`/reseller/*` 的中间件 `RequireMainTenantForResellerConsole` 只做「主域名/分销端子域」隔离，**不校验用户是否为分销商**；但 Service 层 `requireActiveProfileByUser` 对非分销商返回 `ErrNotOpened`，数据仍按当前用户 profile.ID 隔离，故不构成越权。已验证。

---

## Mass Assignment 清单

逐模块核查是否存在 `ShouldBindJSON(&domainModel)` 后直接 `Save(&domainModel)`、从而允许客户端提交 `role/is_admin/balance/status/commission/verified/user_id/owner_id/created_at` 等隐藏字段。

- 全仓搜索 `ShouldBindJSON(&...Model)` / `BindJSON(&...Model)` 模式：**0 命中**。
- 各用户端写接口均使用**显式请求 DTO**，且敏感字段由服务端覆盖：
  - 下单 `create_handler.go`：`CreateOrderRequest` 显式字段，`UserID: uid` 由 JWT 强制覆盖（不读请求体 user_id）。
  - 积分商城兑换 `user_handler.go`：仅接受 `product_id`，`points_price/quantity/stock` 全部服务端快照。
  - 工单 `ReplyToTicket`/`CreateTicket`：显式 DTO（CategoryID/Subject/Body/BizType/AttachmentIDs），无 user_id 入参。
  - 提现、购物车、API 凭证、C2C 各写接口：均为显式 DTO。
- **结论：未发现 Mass Assignment 缺陷。**

---

## 输入校验缺口清单

| 接口 | 校验情况 | 备注 |
|------|----------|------|
| 下单 quantity | `order_service_validate.go`: `Quantity<=0` → ErrInvalidOrderItem；min/max purchase 校验（有集成测试） | PASS |
| 下单 product_id/sku | `ProductID==0` 拒绝；价格服务端从 catalog 读取，不接受客户端价 | PASS |
| C2C 金额 | `trade.go`: amount<=0 拒绝；min/max trade、fiat 区间校验；法币金额 = amount×挂单价（服务端价，非客户端） | PASS |
| 提现金额/地址 | TOTP 要求；余额行锁；金额解析 `decimal.NewFromString` | PASS |
| 签到日期 | 月份正则 `^\d{4}-\d{2}$` 校验 | PASS |
| 工单 content/附件 | 附件枚举白名单 + size 限制；reply 内容 trim | 附件见 ISSUE-U01 |
| 礼品卡兑换 | 验证码 + 限流规则 `giftCardRedeemRule` | PASS |
| 备注 | null/""/-1/0/超大串 等极端值未做逐 fuzz；但关键数值（金额/数量）均在 Service 层用 decimal 严格解析并拒绝 <=0 | NOT VERIFIED（动态 fuzz） |

---

## 业务逻辑 / 状态机核查要点

- **优惠券**：`coupon` 模块无用户端路由，仅 admin；下单时服务端 `ApplyCoupon`，用量按 `(coupon_id,user_id)` 落 `coupon_usages`，`used_count` 原子增减，单用户限领/限用在 usage store 计数。未发现可重复使用/转让路径。（深层叠加边界 NOT VERIFIED）
- **积分**：`points/application/mutation.go` 行锁 `FOR UPDATE` + 溢出保护 + REDEEM 禁止负余额 + reference 幂等（非空 reference）；`checkin.go` 签到 reference 预查 + 唯一索引。并发刷分有兜底。PASS
- **积分商城**：`pointsmall/service.go` 事务内：幂等键 → 余额校验（REDEEM 不允许负）→ 商品行锁 `FOR UPDATE` 扣库存 → 每用户限购 → 价格服务端快照；取消后 reference 幂等退款。PASS
- **签到并发**：唯一索引 `(user_id, checkin_date)` 作为最终防线，重复签到落唯一索引冲突回滚。PASS
- **邀请奖励/自邀**：C2C `CreateTrade` 显式拒绝自买自卖并写风控信号 `self_trade_attempt`；邀请关系走注册绑定，`/invitation/me` 只读。自邀伪造路径 NOT VERIFIED（注册绑定逻辑未深入）。
- **会员等级**：等级由服务端按消费/配置计算，`ApplyCoupon` 接收 `memberLevelID` 仅作 eligibility 入参，不接受客户端直接设等级。PASS
- **C2C 放币/仲裁**：`Confirm` 仅卖家且状态=paid；`SettleFrozen` 内部按 user_id 升序锁双账户 + reference 幂等；争议仅买卖双方、仅 paid 状态、唯一 open 申诉。PASS
- **状态机**：取消仅 pending；售后仅 completed 且无 pending 单（`ErrPendingExists`）；C2C 各动作均有 `Status != expected` 拒绝。PASS

---

## 已验证 / NOT VERIFIED

**已验证（追到 Store/DB 层有证据）：**
- 订单详情/下载/取消/售后的 user_id 归属（`WHERE ... AND user_id=?`）
- 钱包流水/充值/提现/提现地址的 user_id 归属
- 积分、积分商城、签到的归属与并发幂等
- 通知标记已读的 `id AND user_id`
- 工单详情/回复/关闭/重开/附件下载的 `ticket.UserID` 归属
- C2C 交易与挂单、支付方式的买卖家/卖家归属
- 购物车、API 凭证、登录日志、邀请、affiliate、reseller 订单的 user_id 作用域
- 支付 capture 的订单归属二次校验
- 礼品卡兑换原子幂等
- 无 Mass Assignment 反模式

**NOT VERIFIED（受只读审计范围/时间限制，未动态验证）：**
- 优惠券叠加/超过订单金额的边界组合（仅静态看到服务端重算折扣）
- 邀请关系伪造/自邀奖励重复发放的完整注册链路
- 会员等级判定是否存在可被客户端污染的旁路
- 对 null/""/超大串/特殊字符/SQL/HTML 注入的动态 fuzz（静态层面输入均走 DTO + decimal 严格解析，未见明显注入点）
- 上传附件 TTL/GC 清理策略（影响 ISSUE-U01 利用窗口大小）
