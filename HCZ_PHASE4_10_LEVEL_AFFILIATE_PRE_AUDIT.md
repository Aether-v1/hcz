# HCZ Phase 4 — 10-Level Affiliate Pre-Audit

> 本轮只审计，不修改代码。所有结论基于当前代码库实际实现。
> 审计日期：2026-10-04
> 代码基线：`internal/modules/affiliate/` 全模块 + 关联模块

---

## 0. 结论速览

| 审计项 | 结论 |
|---|---|
| 现有单级 affiliate 可复用比例 | **约 70%**。domain/store/withdraw/refund reversal/settings/frontend 框架可复用；commission 生成逻辑、唯一索引、归因来源需改造 |
| 是否需要新 commission 表 | **需要**。现有 `affiliate_commissions` 唯一索引不含 level，且归因基于 affiliate_profile 而非 inviter_id 链。推荐新增 `affiliate_commissions_v2` 或在现有表加 `level` 列并迁移唯一索引 |
| inviter_id 是否足够作为 10 级关系基础 | **是**。`users.inviter_id` 是永久绑定真源，向上递归遍历即可构建 10 级链。cookie/click 仅负责注册前归因 |
| 返利资格定义 | **必须开启 affiliate profile + status=active**。有邀请关系 ≠ 有返利资格。未开通 profile 的 inviter 跳过该级，继续向上遍历 |
| L1~L10 配置保存 | **Admin Settings `affiliate_config` 扩展**，新增 `level_rates` 数组（L1~L10，每级 enable + rate）。不新建独立表 |
| Commission snapshot | **必须**。每笔佣金保存 order_id / beneficiary_user_id / source_user_id / level / rate / base_amount_usdt / commission_amount_usdt / status。Admin 改比例不影响历史 |
| 什么时候 credit Wallet | **订单 completed 后确认**。当前实现在 order paid 时创建 pending_confirm，需改为 completed 时创建（或 paid 时创建 pending 但 completed 才转 available）。推荐：completed 时一次性创建 available 佣金 |
| partial/full refund 反冲 | **复用现有 `HandleOrderRefunded` 比例算法**，但需扩展为按 level 逐行处理。公式：`deduct = current_commission * refund_delta / remaining_paid`。full refund 后每级归零 |
| 幂等保证 | **唯一索引 `(order_id, beneficiary_user_id, level)`** + 应用层预检查 + 事务行锁。防 completed callback 重放、refund 重放 |
| 历史单级数据兼容 | **保留现有 `affiliate_commissions` 数据不动**。新 10 级佣金写入新表或新结构。历史单级 commission 标记 `level=1` 但不 backfill 2~10 级（禁止猜测历史关系） |
| 最小实施范围 | 改造 affiliate commission 生成逻辑 + 唯一索引 + inviter_id 链遍历 + settings 扩展 + refund 扩展 + 前端层级展示 |
| 主要风险 | ① 与现有 paid 时点佣金生成的兼容性 ② 多级递归查询性能 ③ 总比例超 100% ④ 自邀/环收益 ⑤ 历史数据迁移 |

---

## 1. 现有 Affiliate 完整审计

### 1.1 模块结构

```
internal/modules/affiliate/
├── domain/
│   ├── profile.go          # AffiliateProfile (UserID, AffiliateCode, Status)
│   ├── commission.go       # Commission (AffiliateProfileID, OrderID, CommissionType, BaseAmount, RatePercent, CommissionAmount, Status...)
│   ├── click.go            # Click (cookie/click 归因记录)
│   ├── order_reference.go  # OrderReference (只读投影)
│   └── withdraw_request.go # WithdrawRequest (佣金提现申请)
├── application/
│   ├── service.go          # Service 结构体 + 依赖注入
│   ├── commission.go       # HandleOrderPaid / ConfirmDueCommissions / HandleOrderCanceled / HandleOrderRefunded
│   ├── attribution.go      # ResolveOrderAffiliateSnapshot / TrackClick / ResolveRegistrationInviterUserID
│   ├── profile.go          # profile 管理
│   ├── query.go            # GetUserDashboard / ListUserCommissions
│   ├── withdraw.go         # ApplyWithdraw / ReviewWithdraw
│   ├── types.go            # DTO 类型
│   └── errors.go           # 领域错误
├── contract/
│   └── store.go            # Store 接口（含 WithinTransaction）
├── infrastructure/gormstore/
│   └── store.go            # GORM 实现
├── transport/http/
│   ├── handler.go          # User 端 handler
│   ├── admin_handler.go    # Admin 端 handler
│   ├── channel_handler.go  # Channel 端 handler
│   └── routes.go           # 路由注册
└── transport/presenter/
    └── presenter.go        # 响应 DTO
```

### 1.2 affiliate_profiles 表

**文件**: `internal/modules/affiliate/domain/profile.go`

| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | PK |
| UserID | uint | uniqueIndex，一用户一 profile |
| AffiliateCode | varchar(32) | uniqueIndex，推广码 |
| Status | varchar(20) | active / disabled |
| CreatedAt / UpdatedAt / DeletedAt | | |

**审计结论**: KEEP。10 级返利仍需 affiliate profile 作为返利资格门槛。无需改结构。

### 1.3 affiliate_commissions 表（核心）

**文件**: `internal/modules/affiliate/domain/commission.go`

| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | PK |
| AffiliateProfileID | uint | index + **uniqueIndex(affiliate_profile_id, order_id, commission_type)** |
| OrderID | uint | index + uniqueIndex |
| OrderItemID | *uint | 订单项 |
| CommissionType | varchar(20) | uniqueIndex，默认 "order" |
| BaseAmount | decimal(20,2) | 佣金基数（USDT） |
| RatePercent | decimal(10,2) | 比例（百分比数值，如 5.00 = 5%） |
| CommissionAmount | decimal(20,2) | 佣金金额（USDT） |
| Status | varchar(32) | pending_confirm / available / rejected / withdrawn |
| ConfirmAt | *time.Time | 待确认到期时间 |
| AvailableAt | *time.Time | 转可提现时间 |
| WithdrawRequestID | *uint | 关联提现申请 |
| InvalidReason | varchar(255) | 失效原因 |
| CreatedAt / UpdatedAt / DeletedAt | | |

**关键问题**:
1. 唯一索引是 `(affiliate_profile_id, order_id, commission_type)` — **不含 level**，无法支持同一订单同一 affiliate 的多级佣金（但实际上多级佣金的 beneficiary 是不同用户，所以 affiliate_profile_id 不同，理论上可以共存。但语义上需要 level 字段来标识层级）
2. 归因基于 `AffiliateProfileID`（来自订单快照 `order.AffiliateProfileID`），**不是 inviter_id 链**
3. 无 `beneficiary_user_id` 字段（需通过 AffiliateProfile → User 关联查询）
4. 无 `source_user_id` 字段（产生佣金的下单用户）
5. 无 `level` 字段

**审计结论**: MODIFY。需要：
- 新增 `level` 列（int, default 1）
- 新增 `beneficiary_user_id` 列（冗余，避免 join profile）
- 新增 `source_user_id` 列（下单用户）
- 唯一索引变更为 `(order_id, beneficiary_user_id, level)`
- 或者：新建 `affiliate_commissions_v2` 表，旧表保留历史数据只读

### 1.4 Commission 生成逻辑

**文件**: `internal/modules/affiliate/application/commission.go` — `HandleOrderPaid()`

当前流程:
1. 读取 affiliate setting，校验 enabled + commission_rate > 0
2. 读取订单
3. `resolveAffiliateProfileForOrder(order)` — 从 `order.AffiliateProfileID` 或 `order.AffiliateCode` 解析（订单创建时的快照）
4. 校验 profile status=active
5. 校验 `profile.UserID != order.UserID`（防自邀）
6. 幂等检查 `GetCommissionByOrderAndProfile`
7. `calculateCommissionBaseAmount(order)` — 按商品 `IsAffiliateEnabled` 过滤，计算 `item.TotalPrice - item.CouponDiscount`，有 ExchangeRate 快照时换算为 USDT
8. `commission = base * rate / 100`
9. 状态：confirm_days=0 → available；否则 pending_confirm + ConfirmAt
10. 创建 commission

**触发时机**: 订单 **PAID** 时（payment callback 中调用，`payment_service_callback_dispatch.go`）

**关键问题**:
1. **归因来源错误** — 当前用订单快照的 AffiliateProfileID/AffiliateCode，这是 cookie/click 归因的结果。10 级返利必须改用 `users.inviter_id` 链遍历
2. **单级** — 只生成一级佣金
3. **生成时机** — 当前在 paid 时生成。用户要求 completed 后确认。需调整
4. **基数计算** — 当前按商品级 `IsAffiliateEnabled` 过滤，这是合理的，10 级应复用
5. **自邀校验** — 当前只校验 `profile.UserID != order.UserID`，10 级需校验链上每一级都不等于下单用户

**审计结论**: MODIFY。核心生成逻辑需重写为 inviter_id 链遍历 + 多级生成。

### 1.5 Commission 状态机

当前状态:
```
pending_confirm → available → withdrawn
     ↓
  rejected
```

- `pending_confirm`: 已创建，等待确认期（confirm_days）
- `available`: 可提现（确认期已过，或 confirm_days=0）
- `withdrawn`: 已进入提现流程
- `rejected`: 因订单取消/退款失效

**定时任务**: `ConfirmDueCommissions` — 定时将到期的 pending_confirm 转为 available（`TaskAffiliateConfirmCommissions`）

**审计结论**: KEEP（状态机可复用），但生成时机需调整为 completed。如果 completed 时直接创建 available，则 pending_confirm 状态对新佣金不再使用（但保留给历史数据）。

### 1.6 Refund / Cancel Reversal

**文件**: `internal/modules/affiliate/application/commission.go`

**HandleOrderCanceled(orderID, reason)**:
- 查询该订单所有 pending_confirm + available 佣金
- 未进入提现流程（WithdrawRequestID == nil）的，标记为 rejected
- 已进入提现流程的，跳过（不影响用户提现）

**HandleOrderRefunded(repoTx, order, refundDelta, refundedBefore, reason)**:
- 在调用方事务内执行
- `totalAmount = order.WalletPaidAmount`（USDT 实付额），无 USDT 快照时回退 TotalAmount
- `remaining = totalAmount - refundedBefore`（当前剩余未退款金额）
- `delta = min(refundDelta, remaining)`
- 行锁查询该订单所有 pending_confirm + available 佣金（`ListCommissionsByOrderForUpdate`）
- 对每条佣金：`deduct = current_commission * delta / remaining`，`next_commission = current_commission - deduct`
- 同步按比例扣减 BaseAmount
- next_commission <= 0 → 标记 rejected
- 已进入提现流程的跳过

**审计结论**: KEEP（算法可复用），但需扩展：
- 当前按 `AffiliateProfileID` 逐行处理，10 级后每行有 level，算法本身不变（逐行比例扣减）
- 需确保 full refund 后每一级都归零
- 需确保 refund 重放不重复扣减（通过 `refundedBefore` 参数和 `remaining` 计算天然防重）

### 1.7 Affiliate Withdraw（佣金提现）

**文件**: `internal/modules/affiliate/application/withdraw.go`

- 用户从 available 佣金中申请提现（ApplyWithdraw）
- 行锁 available 佣金，按金额拆分/选中，关联 WithdrawRequestID
- Admin 审核（ReviewWithdraw）：reject → 释放佣金；pay → 标记佣金 withdrawn
- **注意**: affiliate 佣金提现是独立的提现流程，**不直接 credit 用户 wallet**。佣金在 affiliate_commissions 表中累计，用户通过 affiliate withdraw 单独提取。

**审计结论**: KEEP。10 级返利的佣金仍走同一套 affiliate withdraw 流程。无需为 10 级新增 wallet credit。

### 1.8 Attribution（归因）

**文件**: `internal/modules/affiliate/application/attribution.go`

- `ResolveOrderAffiliateSnapshot(userID, rawCode, rawVisitorKey)` — 下单时解析归因，优先 visitorKey（cookie/click 30天窗口），其次 affiliateCode
- `TrackClick(input)` — 记录推广点击
- `ResolveRegistrationInviterUserID(visitorKey)` — 注册时根据 cookie/click 解析 inviter，写入 `users.inviter_id`

**关键区分**:
- cookie/click 归因 → 仅用于注册时设置 `users.inviter_id`
- `users.inviter_id` → 永久邀请关系真源
- 订单快照 `order.AffiliateProfileID/AffiliateCode` → 当前佣金归因来源（10 级需废弃）

**审计结论**: 
- cookie/click 模块 KEEP（仍负责注册前归因）
- 订单佣金归因 MODIFY（从 order.AffiliateProfileID 改为 inviter_id 链遍历）

### 1.9 Admin Settings

**文件**: `internal/modules/settings/schema/integration/affiliate.go`

当前 `AffiliateSetting`:
```go
type AffiliateSetting struct {
    Enabled           bool
    CommissionRate    float64   // 单级比例
    ConfirmDays       int
    MinWithdrawAmount float64
    WithdrawChannels  []string
}
```

**审计结论**: MODIFY。需扩展：
- `CommissionRate` 保留为 L1 默认（向后兼容）
- 新增 `LevelRates []LevelRate`（L1~L10，每级 enable + rate）
- 新增 `MaxLevels int`（默认 10）
- 新增 `RequireProfile bool`（是否必须开启 affiliate profile，默认 true）
- 新增 `TotalRateCap float64`（总比例上限，默认 100）

### 1.10 User / Admin 前端

**User**: `frontend/user/src/views/personal/AffiliatePanel.vue` + `api/affiliate.ts`
- 返利中心 dashboard（待确认/可提现/已提现）
- 佣金记录列表
- 推广码/推广链接
- 提现申请

**Admin**: `frontend/admin/src/views/admin/`
- `AffiliateUsers.vue` — 用户管理
- `AffiliateCommissions.vue` — 佣金记录
- `AffiliateWithdraws.vue` — 提现审核
- `AffiliateSettings.vue` — 配置

**审计结论**: KEEP（框架可复用），MODIFY（需增加 level 筛选/展示、L1~L10 配置 UI、层级分布统计）。

### 1.11 KEEP / MODIFY / RETIRE / BUILD 汇总

| 组件 | 决策 | 说明 |
|---|---|---|
| affiliate_profiles 表 | KEEP | 返利资格门槛，无需改结构 |
| affiliate_commissions 表 | MODIFY | 加 level/beneficiary_user_id/source_user_id，改唯一索引；或新建 v2 表 |
| affiliate_clicks 表 | KEEP | 注册前归因，不变 |
| affiliate_withdraw_requests 表 | KEEP | 佣金提现流程，不变 |
| Commission 状态机 | KEEP | pending_confirm/available/rejected/withdrawn 可复用 |
| HandleOrderPaid 生成逻辑 | MODIFY | 改为 inviter_id 链遍历 + 多级 + completed 时机 |
| HandleOrderCanceled | KEEP | 逐行失效，算法不变 |
| HandleOrderRefunded | KEEP | 逐行比例扣减，算法不变，天然支持多级 |
| Affiliate Withdraw 流程 | KEEP | 不变 |
| Attribution (cookie/click) | KEEP | 注册前归因，不变 |
| 订单快照 AffiliateProfileID | RETIRE | 不再作为佣金归因来源（但保留字段不删） |
| Admin Settings | MODIFY | 扩展 LevelRates 配置 |
| User 前端 | MODIFY | 增加层级展示 |
| Admin 前端 | MODIFY | 增加 level 筛选 + L1~L10 配置 |
| inviter_id 链遍历 | BUILD | 新增多级关系解析逻辑 |
| 唯一索引 (order_id, beneficiary_user_id, level) | BUILD | 新增幂等约束 |

---

## 2. 邀请关系真源

### 2.1 当前真源

**文件**: `internal/modules/identity/user/domain/user.go` L32-34

```go
InviterID     *uint      // 直接上级用户ID，可空
InviteCode    string     // 个人邀请码（全局唯一）
InviteBoundAt *time.Time // 绑定上级的时间
```

- `inviter_id` 在注册时通过 `ResolveRegistrationInviterUserID(visitorKey)` 从 cookie/click 归因解析后写入
- 一旦绑定，永久不变（Phase 2 已完成永久邀请绑定）
- `invite_code` 是用户个人推广码，用于生成推广链接

### 2.2 10 级关系构建

从订单用户开始，沿 `inviter_id` 向上递归：

```
order_user (level 0, source)
  → inviter_id → user_A (level 1, beneficiary)
    → inviter_id → user_B (level 2, beneficiary)
      → inviter_id → user_C (level 3, beneficiary)
        → ... 最多 10 层
```

### 2.3 终止条件

| 条件 | 处理 |
|---|---|
| `inviter_id == nil` | 停止，链结束 |
| 达到 10 层 | 停止 |
| inviter 用户不存在（已删除） | 跳过该级，继续向上（deleted 用户的 inviter_id 仍有效） |
| inviter 用户 status != active | 跳过该级，继续向上（disabled 用户不获得佣金，但链不断） |
| inviter 未开启 affiliate profile | 跳过该级，继续向上（有邀请关系 ≠ 有返利资格） |
| inviter == order_user（自邀/环） | 跳过该级，停止（防止环无限循环） |
| 链上出现重复 user_id（环检测） | 停止，防止环 |

### 2.4 环检测

- 维护已访问的 `user_id` 集合，遍历时检查
- 正常业务逻辑下 inviter_id 不会形成环（注册时只能绑定已存在用户），但防御性编程必须有
- 检测到环时记录日志并停止，不产生佣金

### 2.5 性能考虑

- 10 级递归 = 最多 10 次 `users` 表单点查询（by ID），可接受
- 可优化为批量查询：先收集所有 inviter_id，再 `WHERE id IN (?)` 一次查询
- 推荐实现：递归收集 ID → 批量查询用户 → 过滤资格 → 构建层级链
- 不需要新建闭包表或物化路径

---

## 3. 返利资格

### 3.1 推荐方案

**必须同时满足以下条件才能获得某级佣金：**

1. 用户在 inviter_id 链上（有邀请关系）
2. 用户已开启 affiliate profile（`affiliate_profiles` 存在且 `status = active`）
3. 用户不是下单用户本人（防自邀）
4. 该级比例 > 0 且该级 enable = true
5. 订单包含可返利商品（`product.IsAffiliateEnabled = true`）

### 3.2 为什么不自动给所有 inviter 返利

- affiliate profile 是用户主动开通的，代表用户同意参与返利计划
- 未开通 profile 的用户可能不知道自己在返利链上，自动返利会造成预期外的资金流动
- Admin 可以通过 profile status 控制单个用户的返利资格（禁用某用户不影响其邀请关系链）
- 与现有单级逻辑一致（当前也校验 profile status=active）

### 3.3 跳过规则

- 未开通 profile 或 profile disabled 的 inviter → **跳过该级，不占层级名额**，继续向上遍历
- 这意味着实际产生佣金的层级可能少于 10 级（中间跳过不补位）
- 例如：L1 未开通 → L2 仍然是 L2（比例按 L2 配置），不递补为 L1

---

## 4. L1~L10 比例配置

### 4.1 配置存储

**推荐：扩展现有 Admin Settings `affiliate_config`**，不新建独立表。

理由：
- 现有 `AffiliateSetting` 已有完整的 Settings Registry 模式（Normalize/Decode/Encode/NormalizeJSON）
- 配置变更频率低，不需要独立表的事务性
- Admin 前端已有 `AffiliateSettings.vue`，扩展 UI 即可
- 与现有 `commission_rate` 字段向后兼容

### 4.2 配置结构

```go
type AffiliateSetting struct {
    Enabled           bool
    CommissionRate    float64         // 保留：L1 默认，向后兼容
    ConfirmDays       int
    MinWithdrawAmount float64
    WithdrawChannels  []string
    // 新增
    MaxLevels         int             // 最大层级，默认 10，范围 1~10
    RequireProfile    bool            // 是否必须开启 affiliate profile，默认 true
    TotalRateCap      float64         // 总比例上限（%），默认 100，范围 0~100
    LevelRates        []LevelRateConfig // L1~L10 配置，索引 0 = L1
}

type LevelRateConfig struct {
    Level   int     // 1~10
    Enabled bool    // 该级是否启用
    Rate    float64 // 比例（%），0~100，2dp
}
```

### 4.3 默认值

| 级别 | 默认 enable | 默认 rate |
|---|---|---|
| L1 | true | 5.00% |
| L2 | true | 3.00% |
| L3 | true | 2.00% |
| L4 | false | 0.00% |
| L5 | false | 0.00% |
| L6 | false | 0.00% |
| L7 | false | 0.00% |
| L8 | false | 0.00% |
| L9 | false | 0.00% |
| L10 | false | 0.00% |

总比例默认 = 10%，远低于 100% 上限。

### 4.4 配置校验规则

1. `MaxLevels` 范围 1~10
2. 每级 `Rate` 范围 0~100，2dp
3. `TotalRateCap` 范围 0~100
4. 所有 enable 级的 rate 之和 ≤ `TotalRateCap`（超出时 Normalize 自动按比例缩减或报错，推荐报错让 Admin 明确）
5. `CommissionRate`（旧字段）与 L1 rate 同步：修改 L1 时同步更新 CommissionRate，反之亦然
6. 允许 0%（该级 enable 但 rate=0 时不产生佣金，等同于 disable，但保留配置位）

### 4.5 配置变更影响

- 修改配置**只影响未来订单**，不影响已创建的 commission（snapshot 机制保证）
- 已创建的 commission 的 rate/base_amount/commission_amount 均为快照，不受配置变更影响
- Admin 前端需提示"修改比例仅影响新订单"

---

## 5. 佣金计算基数

### 5.1 固定规则

- **只能按订单实际 USDT 金额计算**
- 推荐基数：`order.WalletPaidAmount`（USDT 实扣额）
- 禁止使用 `order.TotalAmount`（Site Currency）
- 禁止当前汇率重新换算（必须使用订单创建时的 `ExchangeRate` 快照）

### 5.2 当前实现

**文件**: `internal/modules/affiliate/application/commission.go` — `calculateCommissionBaseAmount()`

当前逻辑:
1. 收集订单所有商品 ID
2. 过滤 `product.IsAffiliateEnabled = true` 的商品
3. 对每个可返利订单项：`payable = item.TotalPrice - item.CouponDiscount`（Site Currency）
4. 有 `order.ExchangeRate` 快照时：`total_usdt = total_site_currency / exchange_rate`
5. 返回 USDT 金额

**审计结论**: KEEP。此逻辑已正确使用 USDT 快照，10 级返利直接复用。

### 5.3 基数选择澄清

用户提到"wallet_paid_amount 或冻结的 USDT snapshot"。当前实现使用的是**商品级可返利金额换算 USDT**，不是直接用 `order.WalletPaidAmount`。

两者区别:
- `order.WalletPaidAmount` = 订单钱包支付总额（含不可返利商品）
- 当前实现 = 仅可返利商品的金额（扣除 coupon），换算 USDT

**推荐：继续使用当前商品级可返利金额**，因为：
1. 某些商品可能不参与返利（`IsAffiliateEnabled = false`）
2. Coupon 折扣应从基数中扣除
3. 与现有单级逻辑一致，避免行为突变

如果产品需求是"按钱包实付总额计算"，则需明确变更。当前审计按现有逻辑推荐。

---

## 6. Commission Snapshot（重点）

### 6.1 每笔佣金必须保存的快照字段

| 字段 | 说明 |
|---|---|
| order_id | 产生佣金的订单 ID |
| beneficiary_user_id | 获得佣金的用户 ID（链上某级 inviter） |
| source_user_id | 下单用户 ID（产生佣金的源头用户） |
| level | 层级（1~10） |
| rate | 该级比例快照（%） |
| base_amount_usdt | 佣金基数快照（USDT，2dp） |
| commission_amount_usdt | 佣金金额快照（USDT，2dp） |
| status | 佣金状态 |
| affiliate_profile_id | 关联的 affiliate profile（保留现有字段） |
| commission_type | 佣金类型（保留，默认 "order"） |
| order_item_id | 订单项（保留，可空） |
| confirm_at / available_at | 时间戳 |
| withdraw_request_id | 提现关联 |
| invalid_reason | 失效原因 |
| created_at / updated_at | |

### 6.2 Snapshot 不可变性

- commission 创建后，`rate` / `base_amount_usdt` / `commission_amount_usdt` / `level` / `beneficiary_user_id` / `source_user_id` / `order_id` 均不可修改
- 唯一可变更的字段：`status` / `commission_amount`（refund 时按比例扣减）/ `base_amount`（refund 时同步扣减）/ `withdraw_request_id` / `invalid_reason` / 时间戳
- Admin 修改返利比例不影响历史 commission

### 6.3 为什么需要 beneficiary_user_id 和 source_user_id

- `beneficiary_user_id`：直接标识佣金归属用户，避免 join affiliate_profiles 查询
- `source_user_id`：标识佣金来源用户，用于风控分析（如 same-source multi-beneficiary）、层级追溯
- 两者都是快照，即使后续 inviter_id 关系变更（理论上不变）也不影响历史

---

## 7. 结算时点

### 7.1 当前实现

- 佣金在订单 **PAID** 时创建（`HandleOrderPaid` 在 payment callback 中调用）
- 状态：confirm_days=0 → available；否则 pending_confirm
- 定时任务 `ConfirmDueCommissions` 将到期 pending_confirm 转为 available

### 7.2 审计推荐：completed 后确认

**推荐方案：订单 completed 时创建佣金，状态直接为 available（或 pending_confirm 视 confirm_days 配置）。**

理由:
1. 用户明确要求"completed 后确认佣金，pending/processing 不实际 credit Wallet"
2. 订单在 pending/processing 阶段可能取消，paid 时创建佣金后又取消会产生大量 rejected 记录
3. completed 意味着订单已履约，佣金确定应发放
4. 减少 refund reversal 的频率（completed 后退款概率低于 paid 后取消）

### 7.3 实现方式

**方案 A（推荐）：修改触发时机**
- 将 `HandleOrderPaid` 重命名/改造为 `HandleOrderCompleted`
- 在订单状态流转为 completed 时调用（order module 的状态变更逻辑中）
- paid 时不再创建佣金
- 历史已在 paid 时创建的佣金不受影响（已存在的保留）

**方案 B（保守）：paid 时创建 pending，completed 时转 available**
- paid 时创建 status=pending（新增状态或复用 pending_confirm）
- completed 时转为 available
- 订单取消/失败时直接删除 pending 佣金
- 优点：paid 时即可展示"待确认佣金"给用户
- 缺点：状态更复杂，需要处理 paid→completed 之间的取消

**审计推荐方案 A**，更简洁，符合用户"completed 后确认"的要求。

### 7.4 与现有定时任务的关系

- 如果 completed 时直接创建 available（confirm_days=0），则 `ConfirmDueCommissions` 定时任务对新佣金不再需要
- 如果保留 confirm_days > 0，则 completed 时创建 pending_confirm，定时任务仍需运行
- 推荐：第一版 confirm_days=0（completed 即 available），简化流程。confirm_days 配置保留供未来使用。

---

## 8. Wallet 入账

### 8.1 当前实现

Affiliate 佣金**不直接 credit 用户 wallet**。佣金在 `affiliate_commissions` 表中累计，用户通过 `affiliate_withdraw_requests` 单独申请提现，Admin 审核后打款。

这与 User Wallet Withdrawal（Phase 3）是两套独立的提现流程。

### 8.2 审计结论：KEEP 现有模式

10 级返利的佣金仍走现有 affiliate commission → affiliate withdraw 流程，**不直接 credit 用户 wallet**。

理由:
1. 现有 affiliate withdraw 流程已完善（申请、审核、打款、佣金拆分锁定）
2. 佣金是"待提现"性质，不是直接可用余额，用户需要主动申请提现
3. 与 User Wallet Withdrawal（Phase 3）隔离，避免资金混淆
4. Admin 审核机制提供额外风控层

### 8.3 幂等保证

- 同一 order + beneficiary + level 只产生一条 commission（唯一索引）
- commission 转为 withdrawn 时通过 `WithdrawRequestID` 关联，行锁防止重复提现
- affiliate withdraw 的 `ApplyWithdraw` 已实现行锁 + 佣金拆分 + 事务

---

## 9. Failed / Canceled 订单

### 9.1 推荐方案下（completed 时创建佣金）

- 订单在 pending/processing/canceled/failed 状态时，**佣金尚未创建**
- 因此不需要对 failed/canceled 做佣金 reversal
- 订单 completed 后才创建佣金，completed 后的退款走 refund reversal 流程

### 9.2 如果采用保守方案（paid 时创建 pending）

- pending 状态的佣金在订单 canceled/failed 时直接删除（或标记 rejected）
- 已 available 的佣金走现有 `HandleOrderCanceled` 逻辑
- 已进入提现流程的佣金跳过（不影响用户提现）

### 9.3 审计推荐

采用方案 A（completed 时创建），则 failed/canceled 无需特殊处理，天然干净。

---

## 10. Partial Refund（重点）

### 10.1 现有算法

**文件**: `internal/modules/affiliate/application/commission.go` — `HandleOrderRefunded()`

公式:
```
total_paid = order.WalletPaidAmount (USDT)
remaining = total_paid - refunded_before (当前剩余未退款金额)
delta = min(refund_delta, remaining)
对每条佣金:
  deduct = current_commission * delta / remaining
  next_commission = current_commission - deduct
  同步扣减 base_amount
  next_commission <= 0 → rejected
```

### 10.2 10 级扩展

现有算法**天然支持多级**，因为它是逐行（逐 commission）比例扣减，不关心 level。10 级后同一订单会有最多 10 条 commission（每级一条），算法逐行处理即可。

### 10.3 示例

原订单实付 100 USDT:
- L1 rate=5% → commission=5 USDT
- L2 rate=3% → commission=3 USDT
- L3 rate=2% → commission=2 USDT

用户退款 20 USDT（refund_delta=20, refunded_before=0, remaining=100）:
- L1: deduct = 5 * 20/100 = 1 → remaining = 4 USDT
- L2: deduct = 3 * 20/100 = 0.6 → remaining = 2.4 USDT
- L3: deduct = 2 * 20/100 = 0.4 → remaining = 1.6 USDT

再次退款 30 USDT（refund_delta=30, refunded_before=20, remaining=80）:
- L1: deduct = 4 * 30/80 = 1.5 → remaining = 2.5 USDT
- L2: deduct = 2.4 * 30/80 = 0.9 → remaining = 1.5 USDT
- L3: deduct = 1.6 * 30/80 = 0.6 → remaining = 1.0 USDT

**关键**: `refunded_before` 参数确保多次退款时不重复扣减，分母用 `remaining`（当前剩余未退款额）而非原始总额。

### 10.4 2dp 舍入

- 每次 `deduct` 计算后 Round(2)
- `next_commission = current_commission - deduct` Round(2)
- 可能出现的舍入误差：多次退款后 commission 剩余 0.01 但实际应归零
- 处理：当 `remaining_paid <= 0.01` 时，将所有未 rejected 的佣金直接归零（full refund 兜底）

---

## 11. Full Refund

### 11.1 规则

- full refund 后（`remaining_paid <= 0` 或 `refunded_amount >= total_paid`），每一级该订单佣金都应最终归零
- 禁止多退（commission 不能变为负数）
- 禁止重复 reverse（通过 `refunded_before` 和状态机防重）
- 禁止某一级遗漏（逐行处理，遍历所有该订单的 active commission）

### 11.2 实现

现有 `HandleOrderRefunded` 在 `delta >= remaining` 时，`deduct = current_commission * 1.0 = current_commission`，`next_commission = 0`，自动标记 rejected。天然支持 full refund。

额外兜底：当 `remaining <= 0.01` 时，强制将所有 pending_confirm/available 且未进入提现流程的佣金标记为 rejected，commission_amount=0。

---

## 12. 多级佣金与幂等

### 12.1 唯一约束

**推荐唯一索引**: `(order_id, beneficiary_user_id, level)`

- 同一订单、同一受益用户、同一层级，只能有一条 commission
- 替代现有唯一索引 `(affiliate_profile_id, order_id, commission_type)`
- `commission_type` 保留但不再参与唯一索引（未来可能有多种佣金类型，但 order+beneficiary+level 已足够唯一）

### 12.2 防重场景

| 场景 | 防护 |
|---|---|
| completed callback 重放 | 应用层 `GetCommissionByOrderAndBeneficiaryAndLevel` 预检查 + 唯一索引兜底 |
| refund 重放 | `refunded_before` 参数确保不重复扣减 + 行锁 |
| admin action 重放 | 状态机校验（终态不可再操作）+ 行锁 |
| 并发 completed + refund | 行锁 `ListCommissionsByOrderForUpdate` 串行化 |
| 多级生成时某级已存在 | 逐级幂等检查，已存在的跳过（不覆盖） |

### 12.3 应用层幂等流程

```
HandleOrderCompleted(orderID):
  BEGIN transaction
    遍历 inviter_id 链，构建 10 级 beneficiary 列表
    对每一级:
      existing = GetCommissionByOrderAndBeneficiaryAndLevel(orderID, beneficiary_id, level)
      if existing != nil: skip（幂等返回）
      计算 base_amount / rate / commission_amount
      CREATE commission
  COMMIT
```

如果唯一索引冲突（并发场景），捕获 duplicate key error，跳过该级继续。

---

## 13. 历史单级 Affiliate 兼容

### 13.1 现有数据

- `affiliate_commissions` 表中已有单级佣金数据（level 隐含为 1）
- 这些数据的归因基于 `order.AffiliateProfileID`（cookie/click 归因），不是 inviter_id 链

### 13.2 兼容策略

| 项 | 策略 |
|---|---|
| 历史 commission 数据 | **保留不动**，继续有效，可正常提现/退款 |
| 历史 commission 的 level | 隐含为 1，不回填 level 字段（或回填 level=1） |
| 历史订单的 2~10 级佣金 | **不 backfill**。禁止猜测历史 inviter_id 链关系（虽然 inviter_id 存在，但历史订单在 10 级上线前已结算，追溯发放会造成资金混乱） |
| 新订单 | 10 级上线后产生的订单，按 inviter_id 链生成 1~10 级佣金 |
| 唯一索引迁移 | 旧索引 `(affiliate_profile_id, order_id, commission_type)` 需删除，新建 `(order_id, beneficiary_user_id, level)`。迁移时需确保旧数据不冲突（旧数据 beneficiary_user_id 可从 affiliate_profile 反查） |
| affiliate_profile 关联 | 保留 `affiliate_profile_id` 字段，新佣金也写入此字段（beneficiary 用户的 profile） |

### 13.3 迁移步骤

1. 新增 `level` / `beneficiary_user_id` / `source_user_id` 列（可空，默认值）
2. 回填历史数据：`level=1`，`beneficiary_user_id = (SELECT user_id FROM affiliate_profiles WHERE id = affiliate_profile_id)`，`source_user_id = (SELECT user_id FROM orders WHERE id = order_id)`
3. 删除旧唯一索引 `idx_affiliate_commission_unique`
4. 创建新唯一索引 `(order_id, beneficiary_user_id, level)`
5. 上线新代码

**或者**：新建 `affiliate_commissions_v2` 表，旧表只读保留，新数据写 v2 表。优点是迁移风险低，缺点是查询需 union。

**审计推荐：在现有表上扩展 + 数据迁移**，因为 affiliate commission 查询逻辑集中，改表比双表 union 更简洁。迁移需在低峰期执行，回填 SQL 需预演。

---

## 14. Admin 配置

### 14.1 配置项清单

| 配置项 | 类型 | 默认 | 说明 |
|---|---|---|---|
| enabled | bool | false | 返利总开关 |
| max_levels | int | 10 | 最大层级（1~10） |
| require_profile | bool | true | 是否必须开启 affiliate profile |
| total_rate_cap | float | 100 | 总比例上限（%） |
| confirm_days | int | 0 | 佣金确认天数（0=立即可用） |
| min_withdraw_amount | float | 10 | 最低提现金额（USDT） |
| withdraw_channels | []string | ["USDT-TRC20"] | 提现渠道 |
| level_rates | []LevelRate | 见下 | L1~L10 配置 |

### 14.2 LevelRates 默认

```json
[
  {"level": 1, "enabled": true, "rate": 5.00},
  {"level": 2, "enabled": true, "rate": 3.00},
  {"level": 3, "enabled": true, "rate": 2.00},
  {"level": 4, "enabled": false, "rate": 0.00},
  {"level": 5, "enabled": false, "rate": 0.00},
  {"level": 6, "enabled": false, "rate": 0.00},
  {"level": 7, "enabled": false, "rate": 0.00},
  {"level": 8, "enabled": false, "rate": 0.00},
  {"level": 9, "enabled": false, "rate": 0.00},
  {"level": 10, "enabled": false, "rate": 0.00}
]
```

### 14.3 Admin 前端

- 在现有 `AffiliateSettings.vue` 中扩展 L1~L10 配置 UI
- 每级：enable switch + rate 输入框
- 实时显示总比例合计，超过 total_rate_cap 时警告
- 提示"修改仅影响新订单"

---

## 15. User 前端

### 15.1 最小页面需求

在现有 `AffiliatePanel.vue` 中扩展：

| 模块 | 内容 |
|---|---|
| 概览 | 我的邀请人数（直接下级 + 全链路）、总返利、待确认返利、已到账返利、已提现 |
| 返利明细 | 列表：订单号、来源用户（脱敏）、层级 L1~L10、基数、比例、佣金金额、状态、时间 |
| 层级分布 | 按 L1~L10 统计佣金金额/笔数的分布图 |
| 推广工具 | 推广码、推广链接（不变） |
| 提现 | 现有 affiliate withdraw 流程（不变） |

### 15.2 隐私保护

- 不暴露下级用户的敏感个人数据（邮箱、真实姓名）
- 来源用户展示：脱敏邮箱（如 `a***@b.com`）或仅显示"下级用户"
- 不展示下级用户的订单明细（仅展示与自己佣金相关的汇总）

### 15.3 层级标识

- 佣金明细中明确显示 `L1` / `L2` / ... / `L10` 标签
- 层级分布用 L1~L10 柱状图或表格

---

## 16. Admin 前端

### 16.1 最小页面需求

在现有 Admin 页面中扩展：

| 页面 | 扩展内容 |
|---|---|
| AffiliateCommissions.vue | 增加 level 筛选（全部/L1~L10）、beneficiary_user 搜索、source_user 搜索、按 level 分组统计 |
| AffiliateUsers.vue | 增加用户的层级位置展示（该用户在链上的层级）、直接下级数、全链路下级数 |
| AffiliateSettings.vue | 增加 L1~L10 配置 UI、max_levels、total_rate_cap、require_profile |
| AffiliateWithdraws.vue | 不变 |

### 16.2 Commission 列表字段

- 订单号、来源用户、受益用户、层级（L1~L10）、基数（USDT）、比例（%）、佣金（USDT）、状态、创建时间、退款/反冲记录

---

## 17. 用户通知

### 17.1 通知事件

| 事件 | 触发时机 | 接收者 | 渠道 |
|---|---|---|---|
| commission_confirmed | 佣金创建（completed 时） | beneficiary_user | 邮件 + 站内 |

第一版只在真正创建佣金时通知（completed 时）。

### 17.2 实现

- 通过现有 `NotificationEnqueuer.Enqueue()` 异步发送
- EventType: `commission_confirmed`
- BizType: `affiliate_commission`
- BizID: commission ID
- Data: { order_no, level, commission_amount, base_amount, rate }
- 通知在事务提交后发出，不阻塞主流程
- 多级佣金每级一条通知（或合并为一条"您有 N 笔佣金到账"）

**推荐**：合并通知。同一订单产生的多级佣金，合并为一条通知"您来自订单 XXX 的佣金已到账（L1: X, L2: Y, ...）"，减少通知轰炸。

---

## 18. 安全与资金不变量

### 18.1 总返利比例

- 所有 enable 级的 rate 之和 ≤ `total_rate_cap`（默认 100%）
- 配置校验时强制检查，超出则拒绝保存
- 运行时不依赖此校验（snapshot 已固定），但配置层保证不会创建超 100% 的组合

### 18.2 Decimal Precision

- 所有金额使用 `money.Amount`（shopspring decimal，强制 2dp）
- rate 使用 `decimal(10,2)`，百分比数值（如 5.00 = 5%）
- commission = base * rate / 100，Round(2)
- refund deduct = commission * delta / remaining，Round(2)

### 18.3 Rounding 策略

- **每一级独立 Round(2)**，不使用累计后统一 Round
- 理由：每级 commission 是独立记录，独立舍入保证每条记录的金额都是合法 2dp
- 舍入误差不会跨级累积（每级独立计算）
- refund 时同样每条独立 Round(2)

### 18.4 最小佣金

- commission_amount < 0.01 USDT 时，不创建该级佣金（金额过小无实际意义）
- 可配置 `min_commission_threshold`，默认 0.01
- 这不是"最小佣金"保底，而是"低于此值不发放"

### 18.5 自邀防护

- 链上每一级 beneficiary_user_id != source_user_id（下单用户）
- 如果 inviter_id 链上出现 source_user_id（环），停止遍历
- 现有单级逻辑已有 `profile.UserID != order.UserID` 校验，10 级需在每级校验

### 18.6 环检测

- 维护已访问 user_id 集合，遍历时检查
- 检测到重复 user_id → 停止，记录日志
- 正常业务逻辑下不会形成环（inviter_id 只能指向已存在用户），但防御性编程必须有

### 18.7 金额不变量

- USDT，decimal，2dp
- 禁止 float 计算
- 所有金额变更在事务内 + 行锁
- commission 创建和 refund 均在调用方事务内执行

---

## 19. 最小实施范围与主要风险

### 19.1 最小实施范围

**后端**:
1. `affiliate_commissions` 表扩展：加 `level` / `beneficiary_user_id` / `source_user_id` 列，迁移唯一索引
2. `AffiliateSetting` 扩展：`LevelRates` / `MaxLevels` / `RequireProfile` / `TotalRateCap`
3. 新增 inviter_id 链遍历逻辑（application 层）
4. 改造 `HandleOrderPaid` → `HandleOrderCompleted`：多级佣金生成
5. 扩展 `HandleOrderRefunded`：天然支持多级，需验证
6. 新增 `GetCommissionByOrderAndBeneficiaryAndLevel` store 方法
7. User/Admin handler 扩展 level 参数
8. 通知事件 `commission_confirmed`

**前端**:
1. User AffiliatePanel：层级展示、层级分布、佣金明细 level 标签
2. Admin AffiliateCommissions：level 筛选
3. Admin AffiliateSettings：L1~L10 配置 UI

**测试**:
- 10 级链遍历（含跳过/环/终止）
- 多级佣金生成（幂等、snapshot）
- partial refund 多级反冲
- full refund 多级归零
- 自邀/环防护
- 配置校验（总比例上限）
- 历史数据兼容

### 19.2 主要风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| 唯一索引迁移失败（旧数据冲突） | 高 | 预演迁移 SQL，低峰期执行，备份后迁移 |
| completed 时机变更与现有 paid 逻辑冲突 | 高 | 明确切换点，旧订单保留 paid 时创建的佣金，新订单走 completed |
| 多级递归查询性能 | 中 | 批量查询用户，最多 10 次单表查询，可接受 |
| 总比例配置错误导致超发 | 中 | 配置层强制校验 total_rate_cap，运行时 snapshot 固定 |
| 舍入误差累积 | 低 | 每级独立 Round(2)，refund 时 full refund 兜底归零 |
| 自邀/环导致异常收益 | 中 | 每级校验 beneficiary != source，环检测停止 |
| 历史数据 backfill 争议 | 中 | 明确不 backfill 2~10 级，仅迁移 level=1 |
| refund 重放重复扣减 | 低 | refunded_before 参数 + 行锁 + 状态机 |
| 通知轰炸（10 级 = 10 条通知） | 低 | 合并为一条通知 |

### 19.3 与 C2C / Wallet Withdrawal 的关系

- 10 级返利与 C2C 完全解耦
- 10 级返利与 User Wallet Withdrawal（Phase 3）完全解耦（affiliate commission 走独立的 affiliate withdraw 流程）
- 10 级返利不修改 wallet_accounts 结构
- 10 级返利不影响提现资金链

---

## 20. 附录：现有代码引用索引

| 能力 | 文件路径 |
|---|---|
| AffiliateProfile domain | `internal/modules/affiliate/domain/profile.go` |
| Commission domain | `internal/modules/affiliate/domain/commission.go` |
| Click domain | `internal/modules/affiliate/domain/click.go` |
| WithdrawRequest domain | `internal/modules/affiliate/domain/withdraw_request.go` |
| Commission 生成/退款 | `internal/modules/affiliate/application/commission.go` |
| Attribution | `internal/modules/affiliate/application/attribution.go` |
| Withdraw 流程 | `internal/modules/affiliate/application/withdraw.go` |
| Query/Dashboard | `internal/modules/affiliate/application/query.go` |
| Service 定义 | `internal/modules/affiliate/application/service.go` |
| Store 接口 | `internal/modules/affiliate/contract/store.go` |
| GORM Store | `internal/modules/affiliate/infrastructure/gormstore/store.go` |
| AffiliateSetting | `internal/modules/settings/schema/integration/affiliate.go` |
| User inviter_id | `internal/modules/identity/user/domain/user.go` L32-34 |
| User invite_code | `internal/modules/identity/user/domain/invitecode.go` |
| 订单 Affiliate 快照 | `internal/modules/order/domain/order.go` L40-41 |
| 订单 WalletPaidAmount | `internal/modules/order/domain/order.go` L30 |
| Payment callback 触发 | `internal/modules/payment/application/payment_service_callback_dispatch.go` |
| Order refund 触发 affiliate | `internal/modules/order/application/refund/service.go` L432-436 |
| Order cancel 触发 affiliate | `internal/modules/order/application/order_service_child.go` |
| Affiliate 常量 | `internal/constants/constants.go` L170-197 |
| User 前端 | `frontend/user/src/views/personal/AffiliatePanel.vue` |
| User API | `frontend/user/src/api/affiliate.ts` |
| Admin Commissions | `frontend/admin/src/views/admin/AffiliateCommissions.vue` |
| Admin Settings | `frontend/admin/src/views/admin/AffiliateSettings.vue` |
| Admin Users | `frontend/admin/src/views/admin/AffiliateUsers.vue` |
| Admin Withdraws | `frontend/admin/src/views/admin/AffiliateWithdraws.vue` |
| 定时任务确认佣金 | `internal/app/jobs/service.go` L55-61 |

---

*审计完成。本文件为 Phase 4 开发的输入基线，实施前需据此拆解任务。*
