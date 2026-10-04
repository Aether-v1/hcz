# HCZ 后端功能缺口审计报告

> 审计日期：2026-10-04
> 审计范围：工单系统、C2C USDT 交易、邀请绑定、10级返利、钱包提现
> 审计方式：实际代码搜索 + 文件读取验证，未修改任何代码

---

## 维度 1：工单系统（Ticket / Support）

### 现状
- **仅有售后工单**：`internal/modules/order/domain/after_sale.go:19-34` 定义 `AfterSaleTicket` 结构体，表名 `after_sale_tickets`
- **类型单一**：仅支持 `not_received` 一种类型（`after_sale.go:11`）
- **状态机简单**：`pending / resolved / rejected` 三态（`after_sale.go:13-16`）
- **路由依附订单**：
  - 用户端：`POST/GET /orders/:id/after-sale`（`order/transport/http/routes.go:66-67`）
  - 管理端：`GET /orders/:id/after-sale` + `POST /orders/:id/after-sale/action`（`routes.go:75,83`）
- **无多轮对话**：工单只有 `Reason` + `Description` + `AdminNote` 单字段，无 conversation / reply / message 表
- **无附件**：搜索 `attachment` 仅匹配到邮件通知相关代码，无工单附件上传
- **无分类**：搜索 `ticket_category` / `ticket_status` 零匹配
- **无未读已读**：无 unread/read 状态字段
- **无消息通知**：工单状态变更无通知触发逻辑

### 已有能力
- 订单售后工单创建 / 查询 / 审核（通过/拒绝）
- 售后触发退款（`aftersale/service.go` 调用 wallet refunder）
- 售后与订单状态联动（订单 `after_sale_status` 字段）

### 缺口
- 无通用工单系统（用户不依附订单也可提交咨询/问题工单）
- 无工单分类（ticket_category：咨询 / 投诉 / 技术问题 / 其他）
- 无多轮回复对话（用户↔管理员来回沟通）
- 无附件上传能力
- 无未读 / 已读状态跟踪
- 无工单消息通知（邮件/站内信/Telegram）
- 工单状态机不完整（缺少 open / answered / closed / waiting_user 等中间态）
- 无工单优先级 / 标签 / 分配（assign to admin）

### 判断：BUILD
### 优先级：P1
### 改造工作量估算：L

> 说明：After-Sale 是订单售后专用，与通用工单完全不同维度。需从零构建通用 ticket domain，可复用 order 模块的消息通知基础设施，但对话、附件、分类、状态机均需新建。

---

## 维度 2：C2C USDT 交易

### 现状
- **完全无 C2C 代码**：搜索 `c2c / C2C / otc / OTC / p2p / P2P` 零文件匹配
- **无 Offer / Listing**：搜索 `offer / offer / listing` 仅匹配到支付网关配置（payment channel），非 C2C 挂单
- **无 Escrow / Dispute / Arbitration**：搜索 `escrow / dispute / arbitration` 零匹配
- **无 Merchant 体系**：搜索 `merchant` 零匹配（仅 payment gateway 配置中有商户号概念）
- **Wallet 资金模型**：
  - `wallet/domain/account.go:10-17`：仅单一 `Balance` 字段，**无 available_balance / frozen_balance 分离**
  - `wallet/domain/transaction.go:10-30`：流水表有 Type / Direction / Amount / BalanceBefore / BalanceAfter
- **Reseller 有冻结模型（参考）**：
  - `reseller/domain/accounting.go:60-77`：`BalanceAccount` 有 `AvailableAmountCache` + `LockedAmountCache` 分离
  - 但这是分销商内部账务，不是 C2C 托管资金

### 已有能力
- USDT 充值到账（通过 payment gateway：epusdt / bepusdt / tokenpay 等）
- 钱包余额扣减（订单支付时 direct-debit）
- 分销商账户有 available / locked 分离的设计参考

### 缺口
- 资金模型：Wallet 需从单余额改为 available_balance / frozen_balance 分离
- 托管（Escrow）：买 USDT 时冻结卖方余额，确认后释放给买方
- C2C 挂单：Buy / Sell Offer（价格、数量、限额、付款方式）
- 订单匹配：自动匹配 / 手动选择订单
- 付款确认：标记已付款 → 放币
- 超时取消：倒计时自动取消 + 资金退回
- 争议仲裁：dispute 流程 + admin 仲裁
- 商家体系：merchant 认证、评分、交易量
- 手续费：C2C 交易手续费
- 限额：单笔 / 单日限额
- 黑名单风控：用户黑名单 / 地址黑名单

### 判断：BUILD
### 优先级：P0
### 改造工作量估算：XL

> 说明：C2C 是从零构建的大型模块，且需要改造现有 Wallet 资金模型（单余额 → 双余额分离），涉及核心账务变更，风险高。

---

## 维度 3：邀请绑定（Invitation）

### 现状
- **User 表无邀请字段**：`identity/user/domain/user.go:10-34`，搜索 `inviter_id / invite_code / parent_id` 零匹配
- **注册函数无 invite 参数**：`identity/userauth/application/service.go:263` 的 `Register(email, password, code string, ...)` — `code` 是邮箱验证码，不是邀请码
- **注册后不绑定邀请人**：Register 函数内无任何 affiliate / inviter 关联逻辑
- **有 Click Tracking（cookie 归因）**：
  - `affiliate/domain/click.go:6-22`：`Click` 表记录 visitorKey + affiliateProfileID
  - `affiliate/application/attribution.go:62-106`：`TrackClick` 记录点击，30天归因窗口
- **下单时归因**：
  - `order/application/order_service.go:531`：创建订单时调用 `ResolveOrderAffiliateSnapshot(userID, affiliateCode, visitorKey)`
  - `order/domain/order.go:40-41`：Order 表快照 `AffiliateProfileID` + `AffiliateCode`
- **无用户级邀请关系链**：不存在 inviter → invitee 的永久绑定
- **无邀请记录表**：搜索 `invitation_record / invitee_id` 零匹配

### 已有能力
- Affiliate Code 生成（`affiliate/application/profile.go:139-152`）
- Click Tracking + 30天 cookie 归因窗口
- 下单时归因快照（最近点击优先 / 直接 code 兜底）
- 防自邀（订单维度）：`attribution.go:34-36` — 自己点击自己链接不归因

### 缺口
- User 表缺 `inviter_id` 字段（永久绑定上级）
- 注册 API 缺 `invite_code` 参数 + 绑定逻辑
- 注册后不允许补绑定（无后续绑定 API）
- 无防自邀（用户级：注册时自己不能邀请自己）
- 无防循环邀请（A→B→C→A 环检测）
- 无邀请记录表（谁邀请了谁、时间、状态）
- Cookie/Link attribution 只在下单时生效，不与注册绑定联动
- 无换上级 / 解绑机制（业务规则需确认）

### 判断：MODIFY
### 优先级：P0
### 改造工作量估算：M

> 说明：有 cookie 归因底座（click tracking + 下单快照），但用户级永久绑定关系完全缺失。需加 user 表字段 + 注册流程改造 + 防循环检测。

---

## 维度 4：10 级返利（Multi-level Affiliate）

### 现状
- **当前仅支持 1 级（直接邀请人）**：
  - `affiliate/application/commission.go:41`：`resolveAffiliateProfileForOrder(order)` 只返回单个 profile
  - `commission.go:92-103`：每个订单只创建一条 Commission 记录
- **无层级概念**：
  - Commission 表无 `level` / `parent_level` 字段
  - 搜索 `parent_chain / hierarchy / CommissionLevel` 零匹配
- **单一全局比例**：
  - `commission.go:71`：`rate := decimal.NewFromFloat(setting.CommissionRate)` — 只有一个全局比例
  - 无每级比例配置（L1 / L2 / ... / L10）
- **比例快照存在**：
  - `commission.go:97`：`RatePercent` 存入 Commission 记录 — 订单时快照，防后续改比例
- **退款回退完善**：
  - `commission.go:153-257`：`HandleOrderRefunded` 按退款比例扣减佣金，支持多次退款累加
  - `commission.go:116-150`：`HandleOrderCanceled` 订单取消时佣金失效
  - 已进入提现流程的佣金不回退（业务规则）
- **User 收益明细**：
  - `affiliate/application/query.go:52`：`ListUserCommissions` 用户收益列表
  - `affiliate/transport/http/routes.go:14`：`GET /affiliate/commissions`

### 已有能力
- 单级返利佣金生成 / 确认 / 可提现状态流转
- 比例快照（订单时锁定比例）
- 退款按比例回退（部分退款 / 全额退款）
- 订单取消佣金回退
- 用户收益明细查询
- 佣金提现申请 + Admin 审核

### 缺口
- 多级遍历：需从直接邀请人向上遍历 parent chain N 层
- 每级比例配置：Admin 可配置 L1% / L2% / ... / L10%
- 最大层级限制：可配置最大返利级数
- Commission 记录需加 `level` 字段
- User 表需加 `inviter_id`（依赖维度3邀请绑定改造）
- 邀请链构建：注册时绑定上级 → 形成树形结构
- 环检测：防 A→B→A 循环（依赖维度3）
- 多级退款回退：每级佣金按比例回退（需扩展现有逻辑）

### 判断：BUILD
### 优先级：P0
### 改造工作量估算：L

> 说明：当前是单级返利的完整实现，改造到 10 级需要重构 commission 计算逻辑为多级遍历。依赖维度3（邀请绑定）先完成，因为多级返利的前提是有完整的用户邀请关系链。

---

## 维度 5：Wallet Withdrawal（提现）

### 现状
- **Wallet 模块无提现功能**：
  - 搜索 `withdraw / payout` 在 `internal/modules/wallet/` 下零匹配
  - `wallet/transport/http/routes.go:6-18`：用户端仅有 `/wallet`（查询）、`/wallet/transactions`、`/wallet/recharge`（充值）
- **有 Affiliate 佣金提现（可参考）**：
  - `affiliate/domain/withdraw_request.go:11-32`：`WithdrawRequest` 表 `affiliate_withdraw_requests`
  - 字段：Amount / Channel / Account / Status(pending_review/rejected/paid) / RejectReason / ProcessedBy
  - `affiliate/application/withdraw.go:17-139`：`ApplyWithdraw` 用户申请提现
  - `affiliate/application/withdraw.go:142-202`：`ReviewWithdraw` Admin 审核（reject / pay）
- **有 Reseller 分销商提现（可参考）**：
  - `reseller/domain/accounting.go:38-57`：`WithdrawRequest` 表 `reseller_withdraw_requests`
  - 类似结构：Amount / Currency / Channel / Account / Status
- **两者均为手动打款**：Admin 标记 `paid` 即完成，无自动链上转账集成

### 已有能力
- Affiliate 佣金提现申请 + Admin 审核流程（状态机完整）
- Reseller 分销商提现申请 + Admin 审核流程
- 提现拒绝时佣金/额度退回（`withdraw.go:177-185`）
- 提现渠道配置（`setting.WithdrawChannels`）
- 最低提现金额校验（`setting.MinWithdrawAmount`）

### 缺口
- 用户钱包 USDT 提现 API（金额 + TRC20 地址）
- TRC20 地址格式校验
- 手续费配置（提现手续费率 / 固定手续费）
- 最低 / 最高提现限额
- 2FA / Step-Up 验证（提现二次验证）
- 完整提现状态机：pending / approved / rejected / processing / completed / failed
- Wallet debit：提现申请时冻结 / 扣减 USDT 余额
- Reject 时退回余额
- Ledger 流水记录（提现作为钱包交易类型）
- 提现历史查询
- 地址白名单功能
- 单日提现限额 / 风控
- 自动链上转账集成（TRON 节点 / 第三方 USDT API）

### 判断：BUILD
### 优先级：P0
### 改造工作量估算：L

> 说明：钱包本身完全没有提现功能，但 Affiliate / Reseller 已有成熟的提现申请 + 审核状态机可复用。核心缺口是钱包余额扣减 + TRC20 地址管理 + 风控校验 + 链上转账集成。

---

## 汇总表

| 维度 | 判断 | 优先级 | 工作量 | 关键缺口 |
|---|---|---|---|---|
| 1. 工单系统 | BUILD | P1 | L | 无通用工单、无多轮对话、无附件、无分类、无通知 |
| 2. C2C USDT 交易 | BUILD | P0 | XL | 完全从零构建 + Wallet 单余额→双余额改造 + 托管/争议/商家体系 |
| 3. 邀请绑定 | MODIFY | P0 | M | User 表无 inviter_id、注册不绑定上级、无防循环、仅 cookie 归因 |
| 4. 10级返利 | BUILD | P0 | L | 仅单级、无 parent_chain 遍历、无每级比例配置、依赖维度3 |
| 5. 钱包提现 | BUILD | P0 | L | 钱包无提现 API、无 TRC20 地址、无 2FA、无风控、无链上转账 |

---

## 依赖关系说明

```
维度3（邀请绑定）→ 维度4（10级返利）
    ↑ 多级返利的前提是有完整的用户邀请关系链

维度2（C2C）需要改造 Wallet 资金模型（单余额→双余额）
    → 与维度5（钱包提现）共享 Wallet 改造的底座

维度1（工单系统）相对独立，可并行开发
```

## 实施建议优先级

1. **P0 第一批**：维度3（邀请绑定）— 基础数据层改造，为多级返利铺路
2. **P0 第二批**：维度4（10级返利）— 依赖维度3完成
3. **P0 第三批**：维度5（钱包提现）— USDT 出金能力
4. **P0 第四批**：维度2（C2C）— 最大工程量，依赖 Wallet 双余额改造
5. **P1**：维度1（工单系统）— 客服能力，可在核心交易链路稳定后建设
