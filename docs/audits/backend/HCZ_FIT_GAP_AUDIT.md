# HCZ Business Fit-Gap Audit

**审计对象**：Aether-v1/hcz（Go + Gin + GORM + Vue3 双端 SPA）
**审计日期**：2026-10-03
**审计原则**：不新增业务代码，仅映射现有能力 → HCZ 需求
**目标结论**：底座复用率、真正待开发量、最短上线闭环

---

## 0. 底座总览

| 维度 | 现状 |
|---|---|
| 后端语言 | Go（module `github.com/Aether-v1/hcz`） |
| Web 框架 | Gin |
| ORM | GORM（SQLite / Postgres） |
| 缓存/队列 | Redis（asynq 队列） |
| 鉴权 | JWT（Admin / User 双密钥）+ Casbin RBAC |
| 前端 | 独立双 SPA：`frontend/admin`（Vue3+TS）、`frontend/user`（Vue3+TS） |
| API 前缀 | `/api/v1`（前台）、`/api/v1/admin/*`（后台）—— **与目标 `/api/admin/v1` 不一致** |
| 领域模块 | 30+ 个 DDD 分层模块（application/domain/infrastructure/transport） |
| 迁移方式 | Go 代码注册迁移（`bootstrap/database/migrations`），非 SQL 文件 |
| 发布 | goreleaser + Dockerfile + selfupdate 热更新 + `scripts/hcz-manager.sh` |

**底座复用率预估：约 70-75%**。核心交易链路（下单/支付/钱包/退款/佣金/履约）已完整且经过测试，HCZ 主要工作在**币种双轨制（CNY 展示 + USDT 结算）、订单状态精简、汇率模块、充值动态字段落地、售后/未收到状态**。

---

## 1. 逐模块审计

### 1.1 User / Auth

**当前已有能力**
- 邮箱+密码注册/登录，密码策略可配（长度/大小写/数字/特殊字符）
- TOTP 2FA（用户端），AES-GCM 加密存储 secret，恢复码
- Telegram 登录（旧版 Widget + 新版 OIDC）
- Google OAuth 登录
- 邮箱验证、忘记密码
- Token 版本控制（全量失效）、登录限流（Redis）
- 用户资料、显示名、语言偏好、会员等级、累计充值/消费

**可直接复用（KEEP）**
- 整套用户认证体系（注册/登录/JWT/2FA/邮箱验证）
- 密码策略、登录限流、Token 失效机制
- 用户资料 CRUD

**必须改造（MODIFY）**
- 无：HCZ 对用户认证无特殊改造需求

**必须新增（BUILD）**
- 无（若 HCZ 要求手机号注册则需新增，当前仅邮箱）

**可删除/暂时不用（DROP）**
- Telegram 登录、Google OAuth（HCZ 第一版可关闭，配置项已支持 `enabled: false`）—— 代码保留，配置关闭即可

**数据库影响**：无
**API 影响**：无
**前端影响**：用户端登录/注册/安全中心页面已存在，可直接用
**安全风险**：低，已有成熟实现
**迁移风险**：无

**结论：KEEP（P0）**

---

### 1.2 Admin / RBAC / 2FA

**当前已有能力**
- 管理员账号（用户名+密码），超管 `is_super` 标志
- Casbin RBAC：角色（role）+ 策略（sub/obj/act），`keyMatch2` 路径匹配
- 内置角色不可变保护，权限目录自动从路由生成（`/authz/permissions/catalog`）
- 管理员 TOTP 2FA + 恢复码
- 管理员登录限流
- 审计日志（auditlog 模块，记录管理操作）

**可直接复用（KEEP）**
- 整套 Casbin RBAC（角色/策略/管理员-角色绑定）
- 管理员 2FA、恢复码
- 审计日志
- 权限目录自动生成

**必须改造（MODIFY）**
- API 前缀从 `/api/v1/admin/authz/*` 迁移到 `/api/admin/v1/authz/*`（全局路由重构的一部分）

**必须新增（BUILD）**
- 无（HCZ 对 RBAC 无新增需求）

**可删除/暂时不用（DROP）**
- 无

**数据库影响**：`casbin_rule` 表已存在，无需改动
**API 影响**：路由前缀迁移（全局）
**前端影响**：Admin 端 `Authz.vue`、`AuthzAuditLogs.vue` 已存在，需调整 API base URL
**安全风险**：低
**迁移风险**：路由前缀变更需同步前端，Casbin 策略中的 obj 路径需同步更新

**结论：KEEP（P0）+ 路由前缀 MODIFY（P0）**

---

### 1.3 Wallet

**当前已有能力**
- `wallet_accounts`：用户钱包账户，单币种余额（`decimal(20,2)`），`UserID` 唯一索引
- `wallet_transactions`：流水明细，含 `type`/`direction`/`amount`/`balance_before`/`balance_after`/`currency`/`reference`（唯一）/`order_id`/`operator_admin_id`/`remark`
- `wallet_recharge_orders`：充值支付单，关联 payment
- 钱包服务：充值、扣款（订单支付）、退款回退、管理员加减款、余额查询、流水查询
- 事务保护：余额变更带 `balance_before/after`，`reference` 唯一索引防重复
- 默认币种 CNY（`wallet/application/service.go: defaultCurrency = "CNY"`）

**可直接复用（KEEP）**
- 钱包账户模型、流水模型（含 reference 幂等、balance_before/after）
- 充值/扣款/退款/管理员加减款服务
- 事务与幂等机制

**必须改造（MODIFY）**
- **币种从 CNY 改为 USDT**：`wallet_accounts` 无需改结构（单币种），但所有写入的 `currency` 字段需固定为 `USDT`；`defaultCurrency` 常量改为 `USDT`
- 钱包充值（用户充 USDT 进钱包）的金额单位需明确为 USDT
- 订单扣款金额需为 USDT（由汇率模块从 CNY 换算）

**必须新增（BUILD）**
- 无新表，钱包本身已支持单币种

**可删除/暂时不用（DROP）**
- 无

**数据库影响**：无表结构变更；数据迁移时若有旧 CNY 余额需按汇率折算（全新部署无此问题）
**API 影响**：钱包接口返回的 `currency` 变为 `USDT`，前端需适配显示
**前端影响**：`WalletPanel.vue`、`RechargeOrderDetail.vue` 需将显示币种改为 USDT
**安全风险**：中——币种切换涉及所有金额计算，需确保扣款/退款/充值全程 USDT 一致，禁止混合
**迁移风险**：若有存量 CNY 数据需折算；全新部署无风险

**结论：MODIFY（P0）—— 币种切换为 USDT，核心逻辑复用**

---

### 1.4 Ledger（账本）

**当前已有能力**
- 底座无独立 Ledger 模块，**钱包流水 `wallet_transactions` 即账本**
- 每笔流水含：类型、方向、金额、变更前余额、变更后余额、关联订单、操作管理员、幂等 reference
- 退款流水类型 `order_refund`，订单支付流水 `order_pay`
- 管理员加减款有独立类型

**可直接复用（KEEP）**
- `wallet_transactions` 作为账本完全满足 HCZ 需求
- 幂等、余额快照、关联追踪

**必须改造（MODIFY）**
- 流水 `currency` 固定为 USDT（随 Wallet 改造）
- 可考虑新增 `exchange_rate_snapshot` 字段记录该笔流水对应的 USDT/CNY 汇率（可选，P1）

**必须新增（BUILD）**
- 无（若 HCZ 需要独立的财务对账报表，可基于流水聚合，底座已有 `reconciliation` 模块但面向上游对账）

**可删除/暂时不用（DROP）**
- `reconciliation` 模块（上游对账，HCZ v1 无供应商）—— 代码保留，不注册路由即可

**数据库影响**：可选新增 `exchange_rate` 字段到 `wallet_transactions`（P1）
**API 影响**：无
**前端影响**：无
**安全风险**：低
**迁移风险**：无

**结论：KEEP（P0）—— 钱包流水即账本**

---

### 1.5 Product（商品）

**当前已有能力**
- `products`：完整商品模型，含多语言标题/描述/详情、价格、成本价、批发价阶梯、图片、标签、分类
- SKU 体系（`product_skus`）、库存策略（手动库存/自动库存）
- **`manual_form_schema` JSON 字段**：人工交付表单动态 schema，已支持 `text`/`number`/`select` 类型，含 `required`/`options`/`regex` 校验
- `manual_form_submission` 在订单项中快照存储用户提交值
- 交付类型 `auto`/`manual`
- 商品上架/下架、排序、SEO
- 分类管理、商品映射（上游对接）

**可直接复用（KEEP）**
- 商品基础模型、分类、SKU、图片、上下架
- **`manual_form_schema` 动态字段体系**——这正是 HCZ 充值商品需要的（手机号/游戏UID/区服/账号/面额/quantity）
- 订单项中 `manual_form_schema_snapshot` + `manual_form_submission` 快照机制

**必须改造（MODIFY）**
- 商品价格 `price_amount` 为 CNY（前台显示币种），需明确这是 CNY 定价
- 商品详情/列表前台显示 CNY 价格（底座已支持多语言，币种显示需前端统一为 ¥）
- `manual_form_schema` 需验证是否支持 `quantity` 作为独立字段（当前 quantity 在订单项已有独立字段，表单 schema 中可额外配置面额等）

**必须新增（BUILD）**
- 无新表；`manual_form_schema` 已覆盖动态字段需求
- 若 HCZ 需要"面额"作为商品维度而非表单字段，可复用 SKU 或批发价阶梯

**可删除/暂时不用（DROP）**
- 商品映射（`catalog/mapping`）、上游同步——HCZ v1 无供应商，代码保留不启用
- 自动库存（卡密 `cardsecret`）——HCZ v1 人工履约，可关闭
- 批发价阶梯、会员价、活动价、优惠券——HCZ v1 可暂不启用

**数据库影响**：无
**API 影响**：商品前台接口返回 CNY 价格，无需改
**前端影响**：`Products.vue`、`ProductDetail.vue` 需确保价格显示为 CNY（¥），动态表单渲染已支持
**安全风险**：低——`manual_form_schema` 已有服务端校验（`ValidateAndNormalize`）
**迁移风险**：无

**结论：KEEP（P0）—— 商品+动态表单体系完全复用**

---

### 1.6 Order（订单）

**当前已有能力**
- `orders`：完整订单模型，含订单号、用户、游客支持、状态、币种、原始金额/优惠/实付、钱包支付/在线支付、已退款金额、会员等级快照、优惠券、推广返利快照、分销快照、风控 IP、过期时间/支付时间/取消时间
- `order_items`：订单项，含商品快照、SKU 快照、单价/数量/小计、成本价、优惠分摊、**`manual_form_schema_snapshot` + `manual_form_submission`**、交付类型
- 订单状态（10 种）：`pending_payment` / `paid` / `fulfilling` / `partially_delivered` / `partially_refunded` / `delivered` / `completed` / `canceled` / `refunded`
- 后端定价：`buildOrderResult` 从商品数据重新计算金额，**不信任前端传入金额**
- 订单预览、创建、取消、支付渠道查询
- 风控检查（`orderrisk` 模块：IP 限流、黑名单）
- 游客下单支持

**可直接复用（KEEP）**
- 订单/订单项模型（金额字段、快照、动态表单提交）
- 后端定价机制（`buildOrderResult` 重算金额）—— **直接满足"后端计算实际扣除 USDT，禁止前端传最终金额"**
- 订单预览/创建/取消流程
- 风控、订单号生成、过期机制

**必须改造（MODIFY）**
- **订单状态从 10 种精简为 HCZ 5 种**：
  - `pending_payment` → `pending`（待充值/待支付）
  - `paid` + `fulfilling` → `processing`（处理中）
  - `delivered` + `completed` → `completed`（已完成）
  - 新增 `failed`（失败）—— 当前无此状态
  - `canceled` → `canceled`（已取消）
  - `partially_delivered`/`partially_refunded`/`refunded` → 合并入 `completed`（部分退款通过 `refunded_amount` 字段体现，不单独设状态）
- **订单币种**：`orders.currency` 为 CNY（商品定价币种），但钱包扣款为 USDT，需新增 `usdt_amount` / `exchange_rate_snapshot` 字段记录实际扣除 USDT 和下单时汇率
- 下单时保存汇率快照（新增字段）
- 后端计算 USDT 扣款金额 = CNY 总价 × 汇率快照

**必须新增（BUILD）**
- `orders` 表新增字段：`exchange_rate`（decimal，下单时 USDT/CNY 汇率快照）、`usdt_total_amount`（decimal，实际扣除 USDT）
- 订单状态机改造：5 状态流转规则
- 管理员手动处理订单状态（底座已有 admin order handler，需扩展状态流转接口）

**可删除/暂时不用（DROP）**
- 游客下单（HCZ 需登录用户，可关闭）
- 分销快照字段（`reseller_id` 等，HCZ v1 无分销）—— 字段保留不填
- 优惠券/活动价/会员价/批发价—— HCZ v1 可暂不启用

**数据库影响**：`orders` 新增 2 字段（汇率快照、USDT 金额）；状态枚举值变更
**API 影响**：订单创建/详情/列表接口返回新字段和新状态枚举；管理员状态流转接口
**前端影响**：`OrdersPanel.vue`、`OrderDetail.vue`、`Checkout.vue`、Admin `Orders.vue`/`OrderDetailDialog.vue` 需适配 5 状态和新字段
**安全风险**：中——状态机改造需严格校验流转合法性，禁止跳变；汇率快照需防止篡改
**迁移风险**：中——状态枚举变更，若有存量订单需数据迁移（全新部署无风险）

**结论：MODIFY（P0）—— 状态精简 + 汇率快照 + USDT 金额，核心链路复用**

---

### 1.7 Refund（退款）

**当前已有能力**
- `order_refund_records`：退款记录，含订单、用户、类型（`manual`/`wallet`）、金额、手续费退还、币种、备注
- 退款服务（`order/application/refund`）：
  - 管理员手动退款
  - **钱包退款联动**（`wallet.go`）：退款退回钱包，生成 `order_refund` 流水
  - **佣金回滚联动**（`affiliateRefundProcessor.HandleOrderRefunded`）：按退款比例回滚佣金
  - 分销账本扣减联动
  - 部分退款支持（累计退款金额不超过实付）
  - 支付手续费按比例退还计算（`CalculatePaymentFeeRefundAmount`）
- 退款记录列表/详情查询
- 订单 `refunded_amount` 字段累计已退款

**可直接复用（KEEP）**
- 退款记录模型
- **钱包退款联动**（直接满足"退款必须联动钱包账本"）
- **佣金回滚联动**（直接满足"退款必须联动佣金"）
- 部分退款支持（直接满足"已完成订单支持部分退款"）
- 手续费退还计算

**必须改造（MODIFY）**
- 退款金额币种为 USDT（随 Wallet 改造）
- **失败/取消自动退款**：当前退款为管理员手动触发，需新增在订单状态变为 `failed`/`canceled` 时自动触发全额退款的逻辑
- 退款需按汇率快照换算？—— HCZ 需求是钱包余额 USDT，退款应退 USDT（原扣多少退多少），无需重新换算

**必须新增（BUILD）**
- 订单失败/取消时的自动退款编排（在订单状态服务中调用退款服务）
- 自动退款的幂等保护（防止重复退款）

**可删除/暂时不用（DROP）**
- 支付渠道在线退款（HCZ v1 仅钱包支付，无在线支付渠道退款）—— 代码保留，仅用 wallet 退款
- 分销账本扣减（HCZ v1 无分销）

**数据库影响**：无新表，复用 `order_refund_records`
**API 影响**：管理员退款接口已存在；自动退款为内部编排，无新 API
**前端影响**：Admin `OrderRefunds.vue`、`OrderRefundsDialog.vue` 已存在，无需大改
**安全风险**：中——自动退款需严格幂等，状态流转+退款需在同一事务中；部分退款需校验不超过已付 USDT
**迁移风险**：低

**结论：KEEP（P0）+ MODIFY（P0）—— 退款引擎完全复用，新增自动退款编排**

---

### 1.8 Affiliate / Commission（邀请/佣金）

**当前已有能力**
- `affiliate_profiles`：推广用户档案，含用户 ID、推广码（唯一）、状态
- `affiliate_commissions`：佣金记录，含推广用户、订单、订单项、佣金类型、基数金额、比例、佣金金额、状态（`pending_confirm`/`available`/`rejected`/`withdrawn`）、确认到期时间、可提现时间、提现申请、失效原因
- `affiliate_clicks`：推广点击记录
- `affiliate_withdraw_requests`：提现申请
- 佣金服务：
  - 订单完成时生成佣金（按比例）
  - **退款时回滚佣金**（`HandleOrderRefunded`，按退款比例）
  - 佣金确认期机制（`settlement_confirm_days`）
  - 提现申请/审核
- 推广码注册关联（用户注册时携带推广码建立邀请关系）
- 订单中 `affiliate_profile_id` + `affiliate_code` 快照

**可直接复用（KEEP）**
- 邀请关系（推广码 + 注册关联）
- 佣金生成（订单完成时按比例）
- **佣金回滚（退款联动）**——直接满足 HCZ 需求
- 佣金状态机（待确认/可提现/已拒绝/已提现）
- 提现申请流程
- 推广点击追踪

**必须改造（MODIFY）**
- 佣金基数/金额币种为 USDT（随 Wallet/Order 改造）
- 佣金比例配置：当前在 settings 中，HCZ 需确保可配置
- 邀请关系展示：用户端需显示"我的邀请人/下级"（当前 affiliate 用户端已有面板）

**必须新增（BUILD）**
- 无（邀请+佣金体系完整）

**可删除/暂时不用（DROP）**
- 提现申请（HCZ v1 可暂不开放提现，佣金仅记账）—— 代码保留，配置关闭
- 推广点击追踪（HCZ v1 可简化）

**数据库影响**：无表结构变更；佣金金额币种变为 USDT
**API 影响**：佣金接口返回金额为 USDT
**前端影响**：`AffiliatePanel.vue`、Admin `AffiliateCommissions.vue`/`AffiliateUsers.vue`/`AffiliateWithdraws.vue` 已存在，需适配 USDT 显示
**安全风险**：低——佣金回滚已有事务保护
**迁移风险**：低

**结论：KEEP（P0）—— 邀请+佣金体系完全复用，币种随全局改造**

---

### 1.9 Payment（支付）

**当前已有能力**
- `payments`：支付记录，含订单、渠道、提供方类型、渠道类型、交互方式、金额、手续费率/固定手续费/手续费金额、手续费策略快照、币种、状态（`initiated`/`pending`/`success`/`failed`/`expired`）、第三方流水号、网关订单号、回调数据、支付链接/二维码
- `payment_channels`：支付渠道配置，含启用状态、手续费策略
- 10+ 支付网关适配器：Alipay、WeChatPay、Stripe、PayPal、EPUSDT、BEPUSDT、TokenPay、OKPay、EPay、独角数卡
- **渠道级换汇汇率快照**（`paymentExchangeRate`）：创建支付时存储渠道汇率，用于回调时金额校验
- 支付服务：创建支付、回调处理、钱包支付、混合支付（钱包+在线）、支付过期、迟到回调处理
- 钱包充值支付（`wallet_recharge_orders`）
- 支付合规声明（`compliance` 模块）

**可直接复用（KEEP）**
- 支付记录模型、渠道配置
- 支付创建/回调/过期机制
- 钱包支付（`WalletPaidAmount`）—— **HCZ v1 核心支付方式**
- 手续费策略

**必须改造（MODIFY）**
- **HCZ v1 仅钱包支付（USDT 余额），不接在线支付渠道**——支付渠道可全部禁用，仅保留钱包支付
- **全局 USDT/CNY 汇率模块**：当前仅有渠道级换汇快照，无全局汇率服务。需新增：
  - 汇率表（当前汇率、手动 fallback 汇率、更新时间、来源）
  - 汇率获取服务（自动从外部 API 获取 USDT/CNY 汇率 + 后台手动 fallback）
  - 下单时调用汇率服务获取汇率并快照到订单
- 钱包充值（用户给钱包充 USDT）：HCZ v1 若不接在线支付，充值需管理员手动加款或后续接 USDT 链上充值

**必须新增（BUILD）**
- **全局汇率模块**（`exchange_rate` 表 + service + admin API）—— HCZ 核心需求
- Admin 汇率设置页（前端）
- 订单创建时汇率快照写入

**可删除/暂时不用（DROP）**
- 所有在线支付网关适配器（Alipay/WeChat/Stripe/PayPal/EPUSDT 等）—— HCZ v1 不接，代码保留不注册
- 支付合规声明模块——HCZ v1 无需
- 混合支付——仅钱包支付

**数据库影响**：新增 `exchange_rates` 表（当前汇率、手动 fallback、来源、更新时间）
**API 影响**：新增 Admin 汇率设置 API；订单创建接口内部调用汇率服务
**前端影响**：Admin 新增汇率设置页（`ExchangeRate.vue`）；用户端支付页仅显示钱包余额支付
**安全风险**：中——汇率服务需防篡改，下单时快照而非实时计算；手动 fallback 需权限控制
**迁移风险**：低——新增模块，无存量数据

**结论：BUILD（P0）—— 全局汇率模块为 HCZ 核心新增；支付引擎 KEEP，在线网关 DROP**

---

### 1.10 Notification（通知）

**当前已有能力**
- `notification_logs`：通知发送日志，含事件类型、业务类型、业务 ID、渠道、收件人、语言、标题、正文、状态、错误信息、变量
- 通知渠道：SMTP 邮件、飞书 Webhook、Telegram Bot
- 通知模板：多语言（简中/繁中/英文），含订单状态邮件模板、充值通知模板
- 通知服务：异步队列发送（`asyncqueue`）、模板渲染、变量替换
- Admin 通知配置页（`Notifications.vue`）、SMTP 配置、邮件模板编辑

**可直接复用（KEEP）**
- 通知日志模型
- SMTP 邮件发送（订单状态变更通知、退款通知）
- 多语言模板体系
- 异步队列发送
- Admin 通知配置

**必须改造（MODIFY）**
- 通知模板中的币种变量需适配 USDT/CNY 双轨（订单金额 CNY + 扣款 USDT）
- 订单状态通知需适配 HCZ 5 状态

**必须新增（BUILD）**
- **用户站内消息中心（in-app inbox）**：当前通知仅为 outbound（邮件/飞书/Telegram），无用户站内消息表。HCZ 需求"User 消息页面"需要：
  - `user_notifications` 表（用户 ID、标题、正文、类型、已读状态、关联业务 ID）
  - 用户端消息列表/已读 API
  - 订单状态变更时写入用户站内消息
- 这是 P1（上线前建议），第一版可仅用邮件通知

**可删除/暂时不用（DROP）**
- 飞书、Telegram 通知渠道（HCZ v1 可仅用邮件）—— 代码保留，配置关闭

**数据库影响**：新增 `user_notifications` 表（P1）
**API 影响**：新增用户消息列表/已读 API（P1）
**前端影响**：用户端新增消息中心页面（P1）；Admin 通知页已存在
**安全风险**：低
**迁移风险**：低

**结论：KEEP（P0，邮件通知）+ BUILD（P1，用户站内消息中心）**

---

### 1.11 Upstream（上游/供应商）

**当前已有能力**
- `internal/upstream`：上游适配器（独角数卡 `dujiao_next`）、签名器
- `procurement` 模块：采购单（向上游下单）、上游回调
- `catalog/mapping` 模块：商品映射（本地商品 ↔ 上游商品）、加价、库存同步
- `reconciliation` 模块：上游对账
- `upstreamapi` 传输层：上游产品查询、回调接收
- `downstreamcallback` 模块：向下游回调订单状态
- `siteconnection` 模块：站点对接连接管理

**可直接复用（KEEP）**
- 无（HCZ v1 明确"第一版不接供应商，管理员人工履约"）

**必须改造（MODIFY）**
- 无

**必须新增（BUILD）**
- 无（v1 不接供应商）

**可删除/暂时不用（DROP）**
- **整个上游体系**：`upstream`、`procurement`、`catalog/mapping`、`reconciliation`、`upstreamapi`、`downstreamcallback`、`siteconnection`
- 代码全部保留，**不注册路由、不初始化服务**即可
- 商品 `fulfillment_type` 统一使用 `manual`，禁用 `upstream`/`auto`

**数据库影响**：相关表不创建（迁移中跳过）或创建但不使用
**API 影响**：不注册上游相关路由
**前端影响**：Admin 端隐藏/移除 `ProductMappings.vue`、`ProcurementOrders.vue`、`Reconciliation.vue`、`SiteConnections.vue`
**安全风险**：低
**迁移风险**：低——仅停用，不删除代码

**结论：DROP（P0，v1 停用）—— 代码保留，v2 接供应商时复用**

---

### 1.12 Installer / Release（安装/发布）

**当前已有能力**
- `cmd/server/main.go`：单一入口，启动 HTTP 服务
- `internal/selfupdate`：热更新模块（文件锁、版本元数据、更新器、重启）
- `.goreleaser.yaml`：多平台发布配置
- `Dockerfile`：容器化部署
- `scripts/hcz-manager.sh`：Linux 管理脚本（启动/停止/重启/更新/日志）
- `config.yml.example`：配置模板
- 首次启动自动初始化：数据库迁移、默认管理员（可配置 `bootstrap.default_admin_username/password`）
- `internal/admincmd`：管理员命令行工具

**可直接复用（KEEP）**
- 整套发布/部署体系
- 首次启动初始化（数据库迁移 + 默认管理员）
- 配置管理
- Docker 部署

**必须改造（MODIFY）**
- 无（HCZ 对安装发布无特殊需求）

**必须新增（BUILD）**
- 无

**可删除/暂时不用（DROP）**
- 无

**数据库影响**：无
**API 影响**：无
**前端影响**：无
**安全风险**：低
**迁移风险**：无

**结论：KEEP（P0）**

---

## 2. 底座其他模块处置（非重点审计但需明确）

| 模块 | 处置 | 说明 |
|---|---|---|
| Reseller（分销/白标） | DROP（v1 停用） | HCZ v1 无分销，代码保留不启用 |
| Coupon（优惠券） | DROP（v1 停用） | HCZ v1 无优惠券 |
| Promotion（活动价） | DROP（v1 停用） | HCZ v1 无活动 |
| MemberLevel（会员等级） | DROP（v1 停用） | HCZ v1 无会员体系 |
| GiftCard（礼品卡） | DROP（v1 停用） | HCZ v1 无礼品卡 |
| CardSecret（卡密） | DROP（v1 停用） | HCZ v1 人工履约，无卡密 |
| Cart（购物车） | MODIFY | HCZ 充值商品通常单件购买，可简化或保留 |
| Content（博客/文章/Banner） | DROP（v1 停用） | HCZ v1 可暂不启用内容管理 |
| Sitemap | DROP（v1 停用） | 随内容模块停用 |
| Telegram Bot（群发/频道/帮助中心） | DROP（v1 停用） | HCZ v1 可暂不启用 |
| AdProxy（广告代理） | DROP（v1 停用） | HCZ v1 无广告 |
| Compliance（支付合规声明） | DROP（v1 停用） | 随在线支付停用 |
| OrderRisk（订单风控） | KEEP | IP 限流/黑名单可保留 |
| ApiCredential（用户 API 凭证） | DROP（v1 停用） | HCZ v1 不开放 API |
| ChannelClient（渠道客户端） | DROP（v1 停用） | 随上游停用 |
| AuditLog（审计日志） | KEEP | 管理操作审计必需 |
| Upload（文件上传） | KEEP | 商品图片上传必需 |
| Dashboard（仪表盘） | KEEP | Admin 首页统计 |
| Reporting（报表） | KEEP/P1 | 可后续增强 |
| Captcha（人机验证） | KEEP | 登录/注册防护 |

---

## 3. HCZ Fit-Gap Matrix（汇总）

| # | 能力项 | HCZ 需求 | 底座现状 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 1 | 用户注册/登录 | 邮箱+密码 | 完整 | KEEP | P0 |
| 2 | 用户 2FA | 可选 | TOTP 完整 | KEEP | P1 |
| 3 | 管理员登录 | 必需 | 完整 | KEEP | P0 |
| 4 | 管理员 RBAC | 必需 | Casbin 完整 | KEEP | P0 |
| 5 | 管理员 2FA | 必需 | TOTP 完整 | KEEP | P0 |
| 6 | 钱包账户 | USDT 余额 | 单币种 CNY | MODIFY（改 USDT） | P0 |
| 7 | 钱包流水/账本 | 完整流水 | 完整（balance_before/after + reference） | KEEP | P0 |
| 8 | 商品管理 | CNY 定价+动态字段 | 完整 + manual_form_schema | KEEP | P0 |
| 9 | 商品动态字段 | 手机号/UID/区服/账号/面额/quantity | manual_form_schema 支持 text/number/select + required/options | KEEP | P0 |
| 10 | 订单创建 | 后端算金额，禁止前端传 | buildOrderResult 重算 | KEEP | P0 |
| 11 | 订单 5 状态 | 待充值/处理中/已完成/失败/已取消 | 10 状态 | MODIFY（精简） | P0 |
| 12 | 订单汇率快照 | 下单时保存 | 无 | BUILD | P0 |
| 13 | 订单 USDT 扣款金额 | 后端计算 | 无（当前 CNY） | BUILD | P0 |
| 14 | 管理员手动履约 | 人工处理订单 | fulfillment manual + admin handler | KEEP | P0 |
| 15 | 管理员手动改订单状态 | 必需 | admin order handler 已有 | MODIFY（扩展状态流转） | P0 |
| 16 | 失败/取消自动退款 | 必需 | 仅手动退款 | MODIFY（新增自动编排） | P0 |
| 17 | 已完成订单部分退款 | 必需 | 完整支持 | KEEP | P0 |
| 18 | 退款联动钱包账本 | 必需 | 完整（wallet.go） | KEEP | P0 |
| 19 | 退款联动佣金 | 必需 | 完整（HandleOrderRefunded） | KEEP | P0 |
| 20 | 用户"未收到"/售后状态 | 必需 | 无 | BUILD | P1 |
| 21 | 邀请关系 | 推广码+注册关联 | 完整 | KEEP | P0 |
| 22 | 订单佣金 | 按比例+确认期 | 完整 | KEEP | P0 |
| 23 | 佣金退款回滚 | 必需 | 完整 | KEEP | P0 |
| 24 | 全局 USDT/CNY 汇率 | 自动+手动 fallback | 仅渠道级换汇快照 | BUILD | P0 |
| 25 | Admin 汇率设置页 | 必需 | 无 | BUILD | P0 |
| 26 | Admin 订单管理 | 必需 | 完整（Orders.vue + 退款 + 履约） | KEEP | P0 |
| 27 | Admin 退款管理 | 必需 | 完整（OrderRefunds.vue） | KEEP | P0 |
| 28 | Admin 售后管理 | 必需 | 无 | BUILD | P1 |
| 29 | 钱包支付 | USDT 余额支付 | 完整（WalletPaidAmount） | KEEP | P0 |
| 30 | 在线支付渠道 | v1 不接 | 10+ 网关 | DROP（v1 停用） | P0 |
| 31 | 邮件通知 | 订单状态/退款 | 完整（SMTP+模板） | KEEP | P0 |
| 32 | 用户站内消息中心 | 消息页面 | 无（仅 outbound） | BUILD | P1 |
| 33 | 上游/供应商对接 | v1 不接 | 完整（独角数卡） | DROP（v1 停用） | P0 |
| 34 | API 前缀统一 | /api/v1 + /api/admin/v1 | /api/v1 + /api/v1/admin | MODIFY | P0 |
| 35 | 用户首页/商品/订单/邀请页面 | 后续接入 | 完整（Home/Products/OrderDetail/AffiliatePanel） | KEEP | P1 |
| 36 | 安装/发布/热更新 | 必需 | 完整（goreleaser+Docker+selfupdate） | KEEP | P0 |

### 统计

| 结论 | 数量 | 占比 |
|---|---|---|
| KEEP | 24 | 67% |
| MODIFY | 7 | 19% |
| BUILD | 5 | 14% |
| DROP | 2（项级；模块级 DROP 约 15 个） | — |

**底座能力复用率：约 80%（KEEP + MODIFY 中的复用部分）**
**真正需从零开发：全局汇率模块、订单汇率/USDT 字段、用户售后/未收到状态、用户站内消息中心**

---

## 4. 数据库影响汇总

### 新增表（P0）
| 表名 | 用途 |
|---|---|
| `exchange_rates` | USDT/CNY 汇率（当前值、手动 fallback、来源、更新时间） |

### 新增表（P1）
| 表名 | 用途 |
|---|---|
| `user_notifications` | 用户站内消息 |
| `order_disputes` | 售后/未收到工单（或在 orders 加 `dispute_status` 字段） |

### 表结构变更（P0）
| 表 | 变更 |
|---|---|
| `orders` | 新增 `exchange_rate`（decimal 10,6）、`usdt_total_amount`（decimal 20,2）；状态枚举值变更 |
| `wallet_transactions` | `currency` 默认值从 CNY 改为 USDT（可选加 `exchange_rate` 字段，P1） |
| `wallet_recharge_orders` | `currency` 默认值改为 USDT |
| `payments` | v1 仅钱包支付，可保留表结构不使用在线渠道 |

### 不创建/停用的表（v1）
上游相关表、分销相关表、卡密表、礼品卡表、优惠券表等——迁移中跳过或创建后不使用。

---

## 5. API 影响汇总

### 新增 API（P0）
| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/admin/v1/exchange-rate` | 获取当前汇率+手动 fallback |
| PUT | `/api/admin/v1/exchange-rate` | 设置手动 fallback 汇率 |
| POST | `/api/admin/v1/orders/:id/status` | 管理员手动流转订单状态（处理中→已完成/失败） |

### 新增 API（P1）
| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/notifications` | 用户站内消息列表 |
| PUT | `/api/v1/notifications/:id/read` | 标记已读 |
| POST | `/api/v1/orders/:id/dispute` | 用户提交"未收到"/售后 |
| GET/POST | `/api/admin/v1/disputes` | 管理员售后工单管理 |

### 改造 API（P0）
- **全局路由前缀**：`/api/v1/admin/*` → `/api/admin/v1/*`
- 订单创建/详情/列表：返回新增 `exchange_rate`、`usdt_total_amount` 字段
- 订单状态枚举值变更为 5 状态
- 钱包接口返回 `currency: USDT`

### 停用 API（v1）
- 所有在线支付渠道相关（创建支付/回调等，v1 仅钱包支付）
- 上游/供应商相关
- 分销相关
- 优惠券/活动/会员/礼品卡/卡密相关

---

## 6. 前端影响汇总

### Admin 端（frontend/admin）
| 页面 | 处置 |
|---|---|
| Login / Dashboard / Authz / Users / Wallet / Settings / Notifications | KEEP，适配 API 前缀 |
| Products / Categories | KEEP，商品价格 CNY 显示 |
| Orders / OrderDetailDialog / OrderFulfillmentModal | MODIFY，适配 5 状态 + 汇率/USDT 字段 + 状态流转 |
| OrderRefunds / OrderRefundsDialog | KEEP |
| PaymentChannels / Payments | DROP（v1 隐藏） |
| **ExchangeRate（新增）** | BUILD，汇率设置页 |
| AffiliateCommissions / AffiliateUsers / AffiliateWithdraws | KEEP，适配 USDT |
| ProductMappings / ProcurementOrders / Reconciliation / SiteConnections | DROP（v1 隐藏） |
| Reseller* / Coupons / Promotions / MemberLevels / GiftCards / CardSecrets | DROP（v1 隐藏） |
| **Disputes（新增）** | BUILD（P1），售后工单管理 |

### User 端（frontend/user）
| 页面 | 处置 |
|---|---|
| Home / Products / ProductDetail | KEEP，CNY 价格显示 + 动态表单渲染 |
| Cart / Checkout / Payment | MODIFY，仅钱包支付，显示 USDT 扣款 |
| PersonalCenter / ProfilePanel / SecurityPanel | KEEP |
| OrdersPanel / OrderDetail | MODIFY，适配 5 状态 + 汇率/USDT 字段 |
| WalletPanel / RechargeOrderDetail | MODIFY，USDT 显示 |
| AffiliatePanel | KEEP，适配 USDT |
| **Notifications（新增）** | BUILD（P1），消息中心 |
| Reseller* / GiftCardPanel / ApiPanel | DROP（v1 隐藏/移除入口） |

---

## 7. 安全风险汇总

| 风险点 | 等级 | 说明 | 缓解措施 |
|---|---|---|---|
| 汇率篡改 | 高 | 若下单时汇率可被前端影响，可导致低价购买 | 汇率由后端从汇率服务获取并快照，禁止前端传汇率 |
| 金额篡改 | 高 | 前端传最终金额可导致低价 | 底座已用 buildOrderResult 重算，保持此机制 |
| 自动退款幂等 | 高 | 失败/取消自动退款若重复触发可导致超退 | 退款 reference 唯一索引 + 状态机校验 + 事务 |
| 币种混淆 | 中 | CNY/USDT 双轨若混用可导致账目错误 | 全链路明确：商品定价 CNY、钱包结算 USDT、汇率快照隔离 |
| 部分退款超额 | 中 | 部分退款累计超过已付 | 退款服务已有累计校验，保持 |
| 佣金回滚 | 中 | 退款时佣金未回滚可导致超发 | 底座已有 HandleOrderRefunded，保持事务 |
| 路由前缀变更 | 低 | Casbin 策略 obj 路径需同步 | 全局替换 + 权限目录重新生成 |
| 停用模块残留接口 | 低 | 未注册但可能被直接访问 | 确保路由不注册，中间件层拦截 |

---

## 8. 迁移风险汇总

| 风险点 | 等级 | 说明 | 缓解措施 |
|---|---|---|---|
| 订单状态枚举变更 | 中 | 10→5 状态，存量订单需映射 | 全新部署无此问题；若有存量需写迁移脚本映射 |
| 币种切换 CNY→USDT | 中 | 钱包/订单/佣金金额含义变化 | 全新部署无此问题；存量需按汇率折算或清零 |
| 路由前缀变更 | 低 | /api/v1/admin → /api/admin/v1 | 前端同步修改 base URL，Casbin 策略同步 |
| 停用模块表 | 低 | 迁移中创建了但不使用 | 迁移脚本中条件跳过，或创建后不影响 |
| 汇率模块新增 | 低 | 新表新服务，无存量 | 无 |

**HCZ 为全新部署，存量数据迁移风险极低。**

---

## 9. 推荐开发顺序（最短上线闭环）

### Phase 0：基础设施（0.5 周）
1. **API 前缀统一**：`/api/v1/admin/*` → `/api/admin/v1/*`，同步 Casbin 策略、前端 base URL
2. **停用模块裁剪**：上游/分销/在线支付/优惠券/活动/会员/礼品卡/卡密/内容/Telegram Bot 等模块不注册路由、不初始化服务；Admin 前端隐藏对应菜单
3. **验证底座核心链路可运行**：用户注册→商品浏览→下单→钱包支付→管理员履约→完成

### Phase 1：币种双轨 + 汇率（P0，1-1.5 周）
4. **全局汇率模块**：新建 `exchange_rates` 表 + service（自动获取 USDT/CNY + 手动 fallback）+ Admin API
5. **Admin 汇率设置页**：前端新增页面
6. **Wallet 币种切换**：`defaultCurrency` 改为 USDT，所有流水 currency 固定 USDT
7. **Order 扩展**：新增 `exchange_rate` + `usdt_total_amount` 字段，下单时从汇率服务获取快照并计算 USDT 扣款
8. **订单创建流程改造**：CNY 总价 × 汇率快照 = USDT 扣款，后端计算，禁止前端传

### Phase 2：订单状态 + 履约（P0，1 周）
9. **订单状态精简为 5 状态**：pending / processing / completed / failed / canceled
10. **状态机实现**：定义合法流转（pending→processing→completed/failed；pending→canceled；processing→failed/canceled）
11. **管理员手动状态流转 API**：处理中→已完成/失败/取消
12. **失败/取消自动退款**：状态变为 failed/canceled 时自动触发全额钱包退款（幂等+事务）
13. **前端适配**：Admin Orders 页 + 用户 OrdersPanel 适配 5 状态

### Phase 3：退款 + 佣金验证（P0，0.5 周）
14. **退款链路验证**：手动退款、自动退款、部分退款，确认钱包流水+佣金回滚正确
15. **佣金币种适配**：佣金基数/金额为 USDT
16. **邀请关系验证**：推广码注册→下单→佣金生成→退款回滚

### Phase 4：商品动态字段落地（P0，0.5 周）
17. **充值商品模板**：利用 manual_form_schema 配置手机号/游戏UID/区服/账号/面额等字段
18. **前端动态表单渲染验证**：ProductDetail 页根据 schema 渲染表单，服务端校验
19. **订单项快照验证**：manual_form_submission 正确存储

### Phase 5：售后 + 消息（P1，1 周，可并行）
20. **用户"未收到"/售后状态**：orders 加 `dispute_status` 或新建 `order_disputes` 表；用户提交售后→管理员处理→退款/拒绝
21. **用户站内消息中心**：`user_notifications` 表 + API + 前端页面；订单状态变更写入消息
22. **Admin 售后管理页**

### Phase 6：收尾 + 上线（0.5 周）
23. 全链路 E2E 测试：注册→充值（管理员加款）→浏览商品→下单（CNY 显示+USDT 扣款+汇率快照）→钱包支付→管理员人工履约→完成→部分退款→佣金回滚
24. 安全审计：汇率防篡改、金额防篡改、退款幂等、权限校验
25. 部署配置：config.yml、Docker、数据库初始化

---

## 10. 最短上线闭环（MVP 定义）

**P0 必须完成项（Phase 0-4）**：
- API 前缀统一 + 停用模块裁剪
- 全局汇率模块（自动+手动 fallback）+ Admin 汇率页
- Wallet USDT 化
- Order 汇率快照 + USDT 扣款金额
- 订单 5 状态 + 状态机 + 管理员手动流转
- 失败/取消自动退款
- 退款联动钱包+佣金（底座已有，验证即可）
- 邀请+佣金（底座已有，验证即可）
- 商品动态字段（底座已有，配置即可）
- 管理员人工履约（底座已有，验证即可）

**P0 完成后即可上线的闭环**：
> 用户注册登录 → 管理员手动给用户钱包加 USDT → 用户浏览 CNY 定价商品 → 填写动态字段（手机号/UID 等）→ 下单（后端按汇率快照算 USDT 扣款）→ 钱包余额扣 USDT → 订单进入"处理中" → 管理员人工充值 → 标记"已完成" → 佣金生成 → 用户可申请"未收到"售后（P1）或管理员部分退款 → 退款退 USDT + 佣金回滚

**P1（上线前建议，可上线后快速补）**：
- 用户站内消息中心
- 用户"未收到"/售后工单
- 用户 2FA（底座已有，开启即可）

**P2（后续版本）**：
- 在线支付渠道接入（USDT 链上充值 / 法币充值）
- 上游供应商对接（自动充值）
- 分销/白标
- 优惠券/活动/会员体系
- 用户 API 开放

---

## 11. 结论

**底座复用率约 80%**，核心交易引擎（订单/钱包/退款/佣金/履约/商品动态表单）已完整且经过测试，HCZ 真正需要开发的核心增量仅 4 项：

1. **全局 USDT/CNY 汇率模块**（P0）—— 底座仅有渠道级换汇快照，无全局汇率服务
2. **订单汇率快照 + USDT 扣款金额**（P0）—— 订单表加字段，下单时快照
3. **订单状态精简为 5 状态 + 失败/取消自动退款**（P0）—— 状态机改造 + 自动退款编排
4. **用户售后/未收到状态 + 站内消息中心**（P1）—— 底座无售后工单和用户 inbox

**最短上线闭环约 4-5 周开发量**（含测试），P0 完成后即可上线运营，P1 可在上线后 1-2 周内补齐。

**审计完成，等待下一阶段开发指令。**
