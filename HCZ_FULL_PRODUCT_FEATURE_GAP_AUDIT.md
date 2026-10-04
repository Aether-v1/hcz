# HCZ Full Product Feature Gap Audit

> 审计日期：2026-10-04
> 审计范围：工单系统 / C2C USDT 交易 / 邀请绑定 / 10级返利 / Wallet提现 / 前端装修 / 消息通知
> 审计方式：实际代码搜索 + 文件读取验证，未修改任何代码
> 前置状态：9 P0 + 2 安全 P1 已关闭，Linux CI 全绿，充值主链（Wallet/Order/Refund/After-Sale）完整可用

---

## 执行摘要

### 整体平台完成度：约 45%

| 层面 | 完成度 | 说明 |
|---|---|---|
| 充值主链（Auth/Product/Wallet/Order/Refund/After-Sale/Admin） | **90%** | 核心交易闭环完整，P0 全关 |
| 资金出金（Withdrawal） | **0%** | 钱包无提现，仅 Affiliate/Reseller 有佣金提现可参考 |
| 增长引擎（Invitation + Multi-level Affiliate） | **20%** | 有 cookie 归因 + 单级返利，无永久绑定 + 无多级 |
| C2C 交易 | **0%** | 完全从零，且需 Wallet 双余额改造 |
| 客服体系（通用工单） | **10%** | 仅有订单售后专用 after_sale_tickets |
| 前端装修（CMS） | **70%** | Banner/文章/公告/品牌/SEO/导航完整，缺品牌色/社交链接/快捷入口 |
| 用户消息通知 | **5%** | 仅有订单邮件（SMTP），无站内消息/红点/通知中心 |

### 核心结论

**充值主链已具备独立上线条件，但整个 HCZ 平台远未达到"完整 V1"状态。** 7 个审计维度中 4 个需要从零构建（C2C / 工单 / 提现 / 用户通知），1 个需中度改造（邀请绑定），1 个需重构（10级返利），1 个基础扎实需补齐（前端装修）。

---

## 一、完整功能矩阵

### 维度 1：工单系统（Ticket / Support）

| 功能项 | 状态 | 判断 | 优先级 | 证据 |
|---|---|---|---|---|
| 订单售后工单（after_sale） | ✅ DONE | KEEP | — | after_sale_tickets 表 + 完整 CRUD + 退款联动 |
| 通用工单（不依附订单） | 🔴 缺失 | BUILD | P1 | 零代码 |
| 工单分类（咨询/投诉/技术） | 🔴 缺失 | BUILD | P1 | ticket_category 零匹配 |
| 多轮回复对话 | 🔴 缺失 | BUILD | P1 | 无 conversation/reply/message 表 |
| 附件上传 | 🔴 缺失 | BUILD | P2 | attachment 零匹配（工单域） |
| 未读/已读跟踪 | 🔴 缺失 | BUILD | P1 | 无 unread/read 字段 |
| 工单状态机（open/answered/closed） | 🔴 缺失 | BUILD | P1 | 仅 after_sale 三态 |
| 工单优先级/标签/分配 | 🔴 缺失 | BUILD | P2 | 零代码 |
| 工单消息通知 | 🔴 缺失 | BUILD | P1 | 依赖维度7通知系统 |

**判断：BUILD ｜ 优先级 P1 ｜ 工作量 L**
> After-Sale 是订单售后专用，与通用工单完全不同维度。需从零构建通用 ticket domain。

---

### 维度 2：C2C USDT 交易

| 功能项 | 状态 | 判断 | 优先级 | 证据 |
|---|---|---|---|---|
| C2C 模块代码 | 🔴 完全缺失 | BUILD | P0 | c2c/otc/p2p 零文件匹配 |
| Buy/Sell Offer 挂单 | 🔴 缺失 | BUILD | P0 | 零代码 |
| 订单匹配 | 🔴 缺失 | BUILD | P0 | 零代码 |
| 托管（Escrow） | 🔴 缺失 | BUILD | P0 | escrow 零匹配 |
| Wallet 双余额（available/frozen） | 🔴 缺失 | BUILD | P0 | wallet/account.go 仅单一 Balance |
| 冻结/释放/结算 | 🔴 缺失 | BUILD | P0 | freeze/release/settle 零匹配 |
| 付款确认 | 🔴 缺失 | BUILD | P0 | 零代码 |
| 超时取消 + 资金退回 | 🔴 缺失 | BUILD | P0 | 零代码 |
| 争议仲裁（Dispute） | 🔴 缺失 | BUILD | P1 | dispute/arbitration 零匹配 |
| 商家体系（Merchant） | 🔴 缺失 | BUILD | P1 | merchant 零匹配 |
| C2C 手续费 | 🔴 缺失 | BUILD | P1 | 零代码 |
| 限额（单笔/单日） | 🔴 缺失 | BUILD | P1 | 零代码 |
| 黑名单风控 | 🔴 缺失 | BUILD | P2 | 零代码 |
| Reseller 双余额参考 | ✅ 有参考 | KEEP | — | reseller/accounting.go Available+Locked 分离 |

**判断：BUILD ｜ 优先级 P0 ｜ 工作量 XL**
> 完全从零构建的大型模块，且需改造 Wallet 核心资金模型（单余额→双余额），涉及核心账务变更，风险最高。

---

### 维度 3：邀请绑定（Invitation）

| 功能项 | 状态 | 判断 | 优先级 | 证据 |
|---|---|---|---|---|
| User 表 inviter_id 字段 | 🔴 缺失 | BUILD | P0 | user.go 零匹配 |
| User 表 invite_code 字段 | 🔴 缺失 | BUILD | P0 | 零匹配 |
| 注册时绑定邀请人 | 🔴 缺失 | MODIFY | P0 | Register() 无 invite 参数 |
| Cookie/Link 归因（Click Tracking） | ✅ DONE | KEEP | — | affiliate/click.go + 30天归因窗口 |
| 下单时归因快照 | ✅ DONE | KEEP | — | order_service.go:531 ResolveOrderAffiliateSnapshot |
| 防自邀（订单维度） | ✅ DONE | KEEP | — | attribution.go:34-36 |
| 防自邀（用户注册维度） | 🔴 缺失 | BUILD | P0 | 零代码 |
| 防循环邀请（A→B→A） | 🔴 缺失 | BUILD | P0 | 零代码 |
| 邀请记录表 | 🔴 缺失 | BUILD | P1 | invitation_record 零匹配 |
| 注册后补绑定 | 🔴 缺失 | DROP | P2 | 业务规则需确认（通常不允许） |
| 换上级/解绑 | 🔴 缺失 | DROP | P2 | 通常永久绑定 |
| Affiliate Code 生成 | ✅ DONE | KEEP | — | profile.go:139-152 |

**判断：MODIFY ｜ 优先级 P0 ｜ 工作量 M**
> 有 cookie 归因底座，但用户级永久绑定关系完全缺失。需加 user 表字段 + 注册流程改造 + 防循环检测。是 10 级返利的前置依赖。

---

### 维度 4：10 级返利（Multi-level Affiliate）

| 功能项 | 状态 | 判断 | 优先级 | 证据 |
|---|---|---|---|---|
| 单级返利（direct inviter） | ✅ DONE | KEEP | — | commission.go 每订单一条 Commission |
| 多级遍历（parent chain） | 🔴 缺失 | BUILD | P0 | parent_chain/hierarchy 零匹配 |
| 每级比例配置（L1-L10） | 🔴 缺失 | BUILD | P0 | 仅单一全局 CommissionRate |
| 最大层级限制 | 🔴 缺失 | BUILD | P0 | 零代码 |
| Commission level 字段 | 🔴 缺失 | BUILD | P0 | Commission 表无 level |
| 比例快照（订单时锁定） | ✅ DONE | KEEP | — | commission.go:97 RatePercent |
| 退款按比例回退 | ✅ DONE | KEEP | — | commission.go:153-257 HandleOrderRefunded |
| 订单取消佣金回退 | ✅ DONE | KEEP | — | commission.go:116-150 HandleOrderCanceled |
| 多级退款回退 | 🔴 缺失 | MODIFY | P0 | 需扩展现有逻辑遍历每级 |
| User 收益明细 | ✅ DONE | KEEP | — | query.go:52 ListUserCommissions |
| 佣金提现 + Admin 审核 | ✅ DONE | KEEP | — | withdraw.go ApplyWithdraw + ReviewWithdraw |
| 邀请关系树（依赖维度3） | 🔴 缺失 | BUILD | P0 | 依赖维度3完成 |

**判断：BUILD ｜ 优先级 P0 ｜ 工作量 L**
> 当前是单级返利的完整实现，改造到 10 级需重构 commission 计算为多级遍历。强依赖维度3（邀请绑定）。

---

### 维度 5：Wallet Withdrawal（提现）

| 功能项 | 状态 | 判断 | 优先级 | 证据 |
|---|---|---|---|---|
| Wallet 提现 API | 🔴 缺失 | BUILD | P0 | wallet/ 下 withdraw 零匹配 |
| USDT TRC20 地址管理 | 🔴 缺失 | BUILD | P0 | trc20/usdt_address 零匹配 |
| TRC20 地址格式校验 | 🔴 缺失 | BUILD | P0 | 零代码 |
| 提现手续费 | 🔴 缺失 | BUILD | P1 | 零代码 |
| 最低/最高限额 | 🔴 缺失 | BUILD | P0 | 零代码（setting 有 MinWithdrawAmount 供 affiliate 用） |
| 2FA / Step-Up 验证 | 🔴 缺失 | BUILD | P0 | 零代码 |
| 提现状态机 | 🔴 缺失 | BUILD | P0 | wallet 无 withdraw 表 |
| Admin approve/reject | 🔴 缺失 | BUILD | P0 | wallet 无审核路由 |
| Wallet debit（扣 USDT） | 🔴 缺失 | BUILD | P0 | 零代码 |
| Reject 时退回余额 | 🔴 缺失 | BUILD | P0 | 零代码 |
| Ledger 流水记录 | 🟡 可复用 | MODIFY | P0 | wallet transaction 表可加 withdraw 类型 |
| 提现历史查询 | 🔴 缺失 | BUILD | P0 | 零代码 |
| 地址白名单 | 🔴 缺失 | BUILD | P2 | 零代码 |
| 单日限额/风控 | 🔴 缺失 | BUILD | P1 | 零代码 |
| 自动链上转账（TRON） | 🔴 缺失 | BUILD | P1 | 零代码（当前均为手动打款） |
| Affiliate 佣金提现（参考） | ✅ 有参考 | KEEP | — | affiliate/withdraw.go 完整状态机 |
| Reseller 提现（参考） | ✅ 有参考 | KEEP | — | reseller/accounting.go WithdrawRequest |

**判断：BUILD ｜ 优先级 P0 ｜ 工作量 L**
> 钱包本身完全没有提现功能，但 Affiliate/Reseller 已有成熟的提现申请+审核状态机可复用。核心缺口是钱包余额扣减 + TRC20 地址管理 + 风控 + 链上转账。

---

### 维度 6：前端装修（CMS）

| 功能项 | 状态 | 判断 | 优先级 | 证据 |
|---|---|---|---|---|
| 首页 Banner 轮播（后台可配） | ✅ DONE | KEEP | — | Banners.vue + banner.go + useBannerCarousel.ts |
| Banner 多位置投放 | 🟡 仅 home_hero | MODIFY | P2 | positionOptions 硬编码单一位置 |
| 首页公告弹窗 | ✅ DONE | KEEP | — | SettingsHomeAnnouncementTab + AnnouncementModal |
| 文章/公告 CMS | ✅ DONE | KEEP | — | Posts.vue + post.go + Blog/Notice 页面 |
| 文章分类（层级） | ✅ DONE | KEEP | — | PostCategories.vue + post_category.go |
| 推荐商品区块 | 🟡 静态取前15个 | MODIFY | P2 | Home.vue:263 不可选商品/不可改文案 |
| 首页快捷入口宫格 | 🔴 不适用 | DROP | P2 | 本项目非交易所，无此需求 |
| 发现页/Feed | 🔴 缺失 | DROP | P2 | 无 /discover 路由 |
| Logo/品牌名/图标/favicon | ✅ DONE | KEEP | — | site_normalize.go:141-161 |
| 品牌色 theme_color | 🔴 缺失 | BUILD | P1 | site_config 无 theme_color，CSS 硬编码 |
| Footer 自定义链接 | ✅ DONE | KEEP | — | site_normalize.go:94-125 + Footer.vue |
| SEO meta（多语言） | ✅ DONE | KEEP | — | site_normalize.go:40 + usePageSeo.ts |
| 联系方式 | 🟡 仅 TG/WhatsApp | MODIFY | P2 | 缺邮箱/电话/更多社交 |
| 社交链接 | 🔴 缺失（Footer被注释） | BUILD | P1 | Footer.vue:26-34 注释掉，无 social_links 配置 |
| About 页面配置 | ✅ DONE | KEEP | — | site_normalize.go:226-269 + About.vue |
| 导航配置（内置+自定义） | ✅ DONE | KEEP | — | SettingsNavigationTab + useNavConfig.ts |
| 自定义脚本注入 | ✅ DONE | KEEP | — | site_normalize.go:56-92 |
| 模板模式（card/list + classic/vault） | ✅ DONE | KEEP | — | template_mode + storefront_template |

**判断：MODIFY ｜ 优先级 P1 ｜ 工作量 M**
> CMS 基础能力扎实（Banner/文章/公告/品牌/SEO/导航/Footer/About/脚本），前后端闭环完整。主要缺口：品牌色、社交链接。不阻塞上线。

---

### 维度 7：消息通知（Notification）

| 功能项 | 状态 | 判断 | 优先级 | 证据 |
|---|---|---|---|---|
| 管理员告警中心（email/tg/feishu） | ✅ DONE | KEEP | — | notification/send.go + Notifications.vue |
| 订单邮件通知（SMTP 给用户） | 🟡 仅邮件 | KEEP | — | order_email_template.go 5种模板 |
| 用户站内消息表（inbox） | 🔴 缺失 | BUILD | P0 | notification_logs 无 user_id/read 字段 |
| 用户通知中心页面 | 🔴 缺失 | BUILD | P0 | 无 /me/notifications 路由 |
| 未读红点/计数 | 🔴 缺失 | BUILD | P0 | Navbar 无 bell/badge |
| 已读/未读标记 | 🔴 缺失 | BUILD | P0 | 无 mark-as-read API |
| 订单状态站内通知 | 🔴 缺失 | BUILD | P0 | 仅邮件，无站内 |
| 充值到账站内通知 | 🔴 缺失 | BUILD | P0 | 仅管理员告警+邮件 |
| 提现状态站内通知 | 🔴 缺失 | BUILD | P0 | 无提现模块 |
| 佣金到账站内通知 | 🔴 缺失 | BUILD | P1 | affiliate 无通知触发 |
| 售后回复站内通知 | 🔴 缺失 | BUILD | P1 | 无通知触发 |
| 实时推送（WebSocket/SSE） | 🔴 缺失 | BUILD | P2 | 可先轮询，非必须 |
| Admin → 用户站内广播 | 🔴 缺失 | BUILD | P1 | 现有 Notifications.vue 是管理员告警配置 |
| Telegram Bot 广播 | ✅ DONE | KEEP | — | TelegramBotBroadcasts.vue |
| 首页公告弹窗 | ✅ DONE | KEEP | — | home_announcement |

**判断：BUILD ｜ 优先级 P0 ｜ 工作量 L**
> 现有 notification 模块是管理员告警中心，不是用户站内消息系统。用户侧消息通知体系为零。订单邮件是唯一触达用户的通道（且依赖 SMTP 配置）。充值/钱包类业务的到账通知是核心体验。

---

## 二、汇总矩阵

| # | 维度 | 判断 | 优先级 | 工作量 | 关键缺口 |
|---|---|---|---|---|---|
| 1 | 工单系统 | BUILD | P1 | L | 无通用工单、无多轮对话、无附件、无分类 |
| 2 | C2C USDT 交易 | BUILD | P0 | XL | 完全从零 + Wallet 单余额→双余额 + 托管/争议/商家 |
| 3 | 邀请绑定 | MODIFY | P0 | M | User 表无 inviter_id、注册不绑定、无防循环 |
| 4 | 10 级返利 | BUILD | P0 | L | 仅单级、无 parent_chain 遍历、无每级比例、依赖维度3 |
| 5 | Wallet 提现 | BUILD | P0 | L | 钱包无提现 API、无 TRC20 地址、无 2FA、无链上转账 |
| 6 | 前端装修 | MODIFY | P1 | M | 缺品牌色、社交链接；基础 CMS 完整 |
| 7 | 用户消息通知 | BUILD | P0 | L | 无站内消息、无红点、无通知中心、仅邮件 |

### 按优先级统计

| 优先级 | 数量 | 维度 |
|---|---|---|
| **P0（上线必须）** | 5 | C2C、邀请绑定、10级返利、Wallet提现、用户通知 |
| **P1（首版建议）** | 2 | 工单系统、前端装修补齐 |
| **P2（上线后）** | — | 实时推送、地址白名单、C2C 商家体系、发现页等 |

---

## 三、版本 A：HCZ 最小可上线版本（Minimum Viable Launch）

> 目标：以充值/数字商品购买为核心，最小功能集即可面向真实用户。

### 已完成（无需额外开发）

| 模块 | 状态 |
|---|---|
| 用户认证（注册/登录/2FA/找回密码/Google/Telegram） | ✅ |
| 商品/分类/内容管理 | ✅ |
| Wallet USDT 充值（Payment Gateway） | ✅ |
| Global Exchange Rate + USDT 结算 | ✅ |
| 商品下单（Wallet-Only） | ✅ |
| 5 态订单状态机 | ✅ |
| 订单履约（自动/手动） | ✅ |
| 退款（部分/全额） | ✅ |
| After-Sale 售后 | ✅ |
| Admin 全量管理 + RBAC | ✅ |
| 前端基础装修（Banner/文章/公告/品牌/SEO/导航） | ✅ |
| 管理员告警通知 | ✅ |
| 单级 Affiliate（cookie 归因） | ✅ |
| 安全合规（Payment Compliance / 速率限制 / IDOR） | ✅ |

### 上线前必须补齐（P0 阻断项）

| # | 功能 | 原因 | 工作量 |
|---|---|---|---|
| 1 | **用户站内消息通知** | 用户无法在站内感知订单/充值状态，仅依赖邮件（SMTP 不一定配置）。充值到账无通知是核心体验硬伤。 | L |
| 2 | **邀请绑定（user 级）** | 当前 affiliate 仅 cookie 归因，用户注册后无永久上级关系。若推广分销是核心增长引擎，必须补齐。若上线时不依赖推广，可降级为 P1。 | M |

### 可降级/延后（不阻断最小上线）

| 功能 | 降级方案 |
|---|---|
| Wallet 提现 | 充值平台用户主要是消费而非提现，可暂不开放。需在 UI 明确"余额仅用于消费"。或仅支持客服手动提现。 |
| 10 级返利 | 保持单级返利（已有），推广期够用。 |
| C2C 交易 | 完全不做，V1.1+ 再规划。 |
| 通用工单 | After-Sale 覆盖订单问题，通用咨询用 Telegram/邮件客服。 |
| 品牌色/社交链接 | 用默认主题色，Footer 社交区暂不展示。 |

### Version A 结论

**如果推广分销不是上线核心依赖，HCZ 充值主链 + 用户站内通知即可最小上线。** 完成度约 95%（仅缺用户通知）。
**如果推广分销是核心增长引擎，还需邀请绑定（M 工作量）。** 完成度约 85%。

---

## 四、版本 B：HCZ 完整 V1

> 目标：充值业务 + Wallet + Withdrawal + Invitation + 10-level Affiliate + Ticket + After-Sale + C2C + Multi-template + Frontend Decoration + Notification

### 完整 V1 功能清单

| 模块 | 子功能 | 当前状态 | V1 要求 |
|---|---|---|---|
| **Auth** | 注册/登录/2FA/找回/OAuth | ✅ DONE | KEEP |
| **Product** | 商品/分类/库存/卡密 | ✅ DONE | KEEP |
| **Wallet** | USDT 余额/充值/流水 | ✅ DONE | KEEP |
| **Wallet** | 双余额（available/frozen） | 🔴 BUILD | C2C 前置 |
| **Wallet** | 提现（TRC20/审核/风控） | 🔴 BUILD | P0 |
| **Exchange Rate** | Global Rate + 快照 | ✅ DONE | KEEP |
| **Order** | 下单/5态机/履约 | ✅ DONE | KEEP |
| **Refund** | 部分/全额/自动退款 | ✅ DONE | KEEP |
| **After-Sale** | 售后工单/退款联动 | ✅ DONE | KEEP |
| **Ticket** | 通用工单/对话/附件/分类 | 🔴 BUILD | P1 |
| **C2C** | 挂单/匹配/托管/争议/商家 | 🔴 BUILD | P0（如 V1 含 C2C） |
| **Invitation** | 永久绑定/防循环/邀请记录 | 🔴 MODIFY | P0 |
| **Affiliate** | 10 级返利/每级比例/多级回退 | 🔴 BUILD | P0 |
| **Notification** | 用户站内消息/红点/通知中心 | 🔴 BUILD | P0 |
| **Notification** | 实时推送（WS/SSE） | 🔴 BUILD | P2 |
| **Frontend CMS** | Banner/文章/公告/品牌/SEO/导航 | ✅ DONE | KEEP |
| **Frontend CMS** | 品牌色/社交链接/快捷入口 | 🔴 BUILD | P1 |
| **Admin** | 全量管理/RBAC/合规 | ✅ DONE | KEEP |
| **Security** | 速率限制/IDOR/Step-Up/审计 | ✅ DONE | KEEP |

### Version B 完成度

| 大类 | 子项数 | 已完成 | 需 BUILD | 需 MODIFY | 完成度 |
|---|---|---|---|---|---|
| 核心交易 | 8 | 8 | 0 | 0 | 100% |
| 资金 | 4 | 1 | 3 | 0 | 25% |
| 增长 | 3 | 0 | 2 | 1 | 0% |
| 客服 | 2 | 1 | 1 | 0 | 50% |
| C2C | 1 | 0 | 1 | 0 | 0% |
| 通知 | 2 | 0 | 2 | 0 | 0% |
| 前端装修 | 3 | 1 | 1 | 1 | 33% |
| **总计** | **23** | **11** | **10** | **2** | **48%** |

---

## 五、依赖关系与推荐开发顺序

### 依赖图

```
邀请绑定（维度3, M）
    └→ 10级返利（维度4, L）

Wallet 双余额改造
    ├→ C2C 交易（维度2, XL）
    └→ Wallet 提现（维度5, L）

用户通知系统（维度7, L）
    ├→ 订单/充值事件接入
    ├→ 提现状态通知（依赖维度5）
    ├→ 佣金到账通知（依赖维度4）
    └→ 工单/售后通知（依赖维度1）

前端装修补齐（维度6, M）— 独立，可随时并行
```

### 推荐开发顺序（按依赖 + 价值排序）

| 阶段 | 内容 | 工作量 | 依赖 | 交付价值 |
|---|---|---|---|---|
| **Phase 1** | 用户站内消息通知 | L | 无 | 核心交易闭环体验，最小上线必备 |
| **Phase 2** | 邀请绑定（user 级永久关系） | M | 无 | 增长引擎基础，为多级返利铺路 |
| **Phase 3** | Wallet 提现（USDT TRC20） | L | 无 | 资金出金能力，平台完整性 |
| **Phase 4** | 10 级返利 | L | Phase 2 | 增长引擎完整化 |
| **Phase 5** | Wallet 双余额改造 | M | 无 | C2C 前置，资金模型升级 |
| **Phase 6** | C2C USDT 交易 | XL | Phase 5 | 新业务线，最大工程量 |
| **Phase 7** | 通用工单系统 | L | Phase 1（通知） | 客服能力，用户支持 |
| **Phase 8** | 前端装修补齐（品牌色/社交链接） | M | 无 | 品牌完整性，可穿插并行 |

### 并行建议

- **Phase 1（通知）+ Phase 2（邀请绑定）+ Phase 8（前端装修）** 可完全并行，无依赖
- **Phase 3（提现）** 可与 Phase 1/2 并行
- **Phase 4（10级返利）** 必须等 Phase 2
- **Phase 5/6（C2C）** 独立大工程，建议在核心功能稳定后启动

---

## 六、关键风险提示

1. **Wallet 双余额改造是最高风险项**：当前 Wallet 是单余额 direct-debit 模型，已通过 P0 审计和全量测试。改为 available/frozen 分离涉及核心账务变更，必须充分测试（含并发/事务/回滚），建议在独立分支开发并做完整回归。

2. **C2C 不应复用充值订单资金模型**：C2C 是用户间交易，需要 escrow 托管 + 冻结/释放/结算，与充值订单（用户→平台→商品）的资金流完全不同。必须独立设计。

3. **邀请绑定需明确业务规则**：注册后是否允许补绑定？是否允许换上级？是否永久绑定？这些规则影响数据模型设计，需产品决策。

4. **用户通知系统应先于工单/C2C**：工单回复、C2C 状态变化、提现状态都需要通知触达。先建统一通知底座，后续业务接入成本低。

5. **提现的链上转账集成**：当前 Affiliate/Reseller 提现均为手动打款（Admin 标记 paid）。自动 TRON 链上转账需要节点/API 集成 + 私钥管理 + 安全审计，建议 V1 先手动打款，V1.1 再自动化。

---

## 七、最终结论

### 当前 HCZ 是否可以正式上线？

**作为"充值/数字商品购买平台"最小版本：接近可上线，缺用户站内通知。**
- 充值主链（Auth/Product/Wallet/Order/Refund/After-Sale/Admin）完整且经过 P0 审计 + 全量回归 + Linux CI 验证
- 唯一阻断：用户无法在站内感知订单/充值状态（仅邮件）
- 补齐用户通知（L 工作量）后即可最小上线

**作为"完整 HCZ 平台"（含 C2C/提现/10级返利/工单）：不可上线，完成度约 48%。**
- 5 个 P0 维度需从零构建或中度改造
- C2C 是最大工程（XL），且需 Wallet 核心改造
- 建议按 Phase 1→8 顺序推进

### 推荐下一阶段

1. **立即启动 Phase 1**：用户站内消息通知（P0，最小上线必备）
2. **并行启动 Phase 2 + Phase 8**：邀请绑定 + 前端装修补齐
3. **产品决策**：确认 V1 是否包含 C2C、提现是否需要自动链上转账、邀请绑定业务规则
4. **最小上线目标**：Phase 1 完成后即可部署生产，后续功能按 Phase 迭代
