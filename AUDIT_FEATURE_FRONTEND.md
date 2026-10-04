# HCZ 前端装修与消息通知审计报告

> 审计范围：维度六（用户前端装修 / CMS）、维度七（消息通知）
> 审计方式：实际搜索与阅读代码，区分"前端有页面"与"后台可配置"
> 审计结论仅基于代码证据，不做推测

---

## 维度 6：用户前端装修（Frontend Decoration / CMS）

### 现状

#### 6.1 首页装修

**顶部 Banner（轮播图）** — ✅ 已有完整实现
- 后台：`frontend/admin/src/views/admin/Banners.vue` — 完整 CRUD，支持上传主图/移动端图、多语言标题/副标题、跳转类型（无/内链/外链）、是否新窗口、启用开关、生效/失效时间、排序、位置筛选
- 后端 domain：`internal/modules/content/domain/banner.go:10-28` — `Banner` 表含 Name/Position/TitleJSON/SubtitleJSON/Image/MobileImage/LinkType/LinkValue/OpenInNewTab/IsActive/StartAt/EndAt/SortOrder
- 公开 API：`internal/modules/content/transport/http/public_handler.go:89-107` — `GET /banners?position=home_hero&limit=N`
- 用户端消费：`frontend/user/src/composables/useBannerCarousel.ts:136-151` — `loadBanners()` 请求 `position=home_hero`，支持自动轮播（5s）、触摸滑动、左右切换按钮、指示点
- 用户端渲染：`frontend/user/src/views/Home.vue:7-57`（list 模式）、`166-261`（card 模式）
- **局限**：position 仅支持 `home_hero` 一种（`Banners.vue:50-52` positionOptions 硬编码），无多位置投放能力

**四个核心业务入口（充值/提现/C2C/邀请等图标入口）** — 🔴 完全缺失（不适用于本项目）
- 本项目是数字商品发卡/充值平台（Wallet/Order/CardSecret），不是交易所/钱包类产品，不存在"充值/提现/C2C/邀请"四个核心入口图标
- 首页无"快捷入口图标宫格"组件，无后台配置能力
- 首页结构固定为：Hero Banner → Featured Products → Latest Posts（blog/notice）
- 个人中心 `/me` 下有钱包/订单/礼品卡/API/推广分销等入口，但属于个人中心导航，非首页装修

**推荐业务模块（Featured Products）** — 🟡 仅静态前端（半配置）
- 用户端：`Home.vue:263-295` — "Featured" 区块展示前 15 个商品（`productAPI.list({page:1,page_size:15})`）
- 区块标题/副标题来自 i18n（`t('home.featured.title')`），**后台不可改文案**
- 无"选择推荐商品"能力，只是取商品列表第一页
- 无模块显示/隐藏开关（仅在 card 模式下显示）
- 后端无 "featured products" / "recommendation module" 配置表

**公告（Announcement）** — ✅ 已有完整实现
- 后台：`frontend/admin/src/views/admin/components/SettingsHomeAnnouncementTab.vue` — 启用开关、类型（normal/info/warning）、多语言标题+富文本内容、排期（start_at/end_at）
- 后端：`internal/modules/settings/schema/storefront/home_announcement.go:128-150` — `ActiveHomeAnnouncement()` 按排期判断是否展示，内容变化生成版本指纹
- 公开配置注入：`internal/modules/settings/transport/http/public/handler.go:225-227` — `data["announcement"] = activeAnnouncement`
- 用户端消费：`frontend/user/src/composables/useAnnouncement.ts` + `frontend/user/src/components/AnnouncementModal.vue`，在 `Home.vue:341-346` 挂载，按本地存储记录已读版本

**推送内容** — 🔴 无用户端推送内容体系
- 无 WebSocket/SSE/轮询推送
- 无"推送内容"配置项（详见维度七）

#### 6.2 发现页装修

**Discover / Feed 页面** — 🔴 完全缺失
- 用户路由（`frontend/user/src/router/index.ts`）中无 `/discover`、`/feed` 路由
- 无独立"发现页"概念

**内容/文章模块（Blog / Notice）** — ✅ 已有完整实现
- 后台：`frontend/admin/src/views/admin/Posts.vue` — Tab 切换 blog/notice，支持多语言标题/slug/摘要/富文本内容/缩略图/发布开关/分类关联/关联商品（post_products 多对多）
- 后台分类：`frontend/admin/src/views/admin/PostCategories.vue` — 层级分类（parent_id）、图标、排序、启用/停用切换
- 后端 domain：
  - `internal/modules/content/domain/post.go:10-28` — Post 表（Type: blog/notice, TitleJSON, SummaryJSON, ContentJSON, Thumbnail, CategoryID, IsPublished, PublishedAt）
  - `internal/modules/content/domain/post.go:31-42` — PostProduct 多对多关联
  - `internal/modules/content/domain/post_category.go` — 分类表
- 公开 API：`internal/modules/content/transport/http/routes.go:7-11` — `GET /posts`（支持 type 查询）、`GET /posts/:slug`、`GET /post-categories`
- 用户端消费：
  - `frontend/user/src/views/Blog.vue` — 博客列表
  - `frontend/user/src/views/BlogDetail.vue` — 博客详情（含关联商品）
  - `frontend/user/src/views/Notice.vue` — 公告列表
  - 首页 Latest 区块（`Home.vue:297-331`）展示最新 3 篇文章，受 nav_config.builtin.blog/notice 开关控制
- **局限**：无 Banner 投放至发现页、无外链卡片、无独立排序配置（文章按 published_at desc）

#### 6.3 品牌与全局

**Logo 配置** — ✅ 已有完整实现
- 后端归一化：`internal/modules/settings/application/site_normalize.go:141-161` — `brand.site_name / site_url / site_icon / site_logo / site_description(多语言)`
- 用户端消费：
  - Favicon：`frontend/user/src/stores/app.ts:18-21,47` — `brand.site_icon` 驱动 `<link rel="icon">`
  - 导航栏 Logo：Navbar.vue 使用 `brand.site_logo`
  - 页脚 Logo：`frontend/user/src/components/Footer.vue:149-152` — `brand.site_logo`
  - 站点名：Footer.vue:131-134、app.ts:44

**品牌色（theme_color）** — 🔴 完全缺失
- site_config 归一化（site_normalize.go）中无 theme_color / brand_color / primary_color 字段
- 后台 Settings.vue 无品牌色配置 Tab
- 前端使用 Tailwind CSS `primary` 主题色，硬编码在 CSS 变量中，不可后台配置

**Footer 链接** — ✅ 已有完整实现
- 后端：`site_normalize.go:94-125` — `footer_links` 数组（name + url），最多 20 条
- 用户端：`Footer.vue:174-183` — 渲染 `config.footer_links`

**SEO meta** — ✅ 已有完整实现
- 后端：`site_normalize.go:40` — `seo.title / keywords / description`（均多语言）
- 用户端：
  - 全局：`app.ts:37-63` — useHead 注入 title/keywords/description
  - 页面级：`frontend/user/src/composables/usePageSeo.ts` — canonical/og/twitter
- 另有 `legal.terms / legal.privacy` 多语言块（site_normalize.go:41）

**联系方式（contact）** — 🟡 部分实现
- 后端：`site_normalize.go:127-139` — 仅 `contact.telegram` + `contact.whatsapp` 两个字段
- 用户端：`Footer.vue:55-72` — 渲染 Telegram / WhatsApp 卡片链接
- **局限**：无邮箱、无电话、无地址、无更多社交渠道（Twitter/X/Facebook/Instagram 等）

**社交链接（social links）** — 🔴 完全缺失
- Footer.vue:26-34 社交图标区域被注释掉（硬编码占位）
- site_config 无 social_links 字段
- contact 仅 telegram/whatsapp

**About 页面配置** — ✅ 已有完整实现
- 后端：`site_normalize.go:226-269` — `about.hero.title/subtitle`、`about.introduction`、`about.services.title/items[]`（最多 12 项）、`about.contact.title/text`（均多语言）
- 用户端：`frontend/user/src/views/About.vue` + `useAbout.ts`

**自定义脚本（scripts）** — ✅ 已有完整实现
- 后端：`site_normalize.go:56-92` — scripts 数组（name/enabled/position:head|body_end/code），最多 20 条，单条代码上限 20000 字符
- 用户端：`app.ts:75,91` — `applyCustomScripts(config.scripts)`

**导航配置（nav_config）** — ✅ 已有完整实现
- 后台：`SettingsNavigationTab.vue` — 内置项（blog/notice/about 开关）+ 自定义项（最多 10 条，多语言标题/内链外链/打开方式/排序/图标预设）
- 后端：`site_normalize.go:320-412` — normalizeNavConfig
- 用户端：`frontend/user/src/composables/useNavConfig.ts` — 统一驱动导航栏和移动端抽屉
- 公开配置：`public/handler.go:215-223` — 注入 `nav_config`

**模板模式（template_mode / storefront_template）** — ✅ 已有完整实现
- `template_mode`: card / list（首页布局）
- `storefront_template`: classic / vault（两套主题模板）
- 用户端通过 `templates/registry.ts` 的 `templateView()` 动态切换

### 已有能力
- Banner 轮播图后台 CRUD（含多语言、跳转、排期、排序、启停），前端完整消费
- 首页公告弹窗（多语言、类型、排期），前端消费
- 文章/公告 CMS（blog/notice 双类型、富文本、分类、关联商品、缩略图、发布开关）
- 文章分类层级管理
- 品牌配置（站点名/Logo/图标/描述/favicon）
- SEO meta（title/keywords/description，多语言）
- Footer 自定义链接
- About 页面完全可配（hero/介绍/服务项/联系文本）
- 导航栏配置（内置项开关 + 自定义链接项）
- 自定义脚本注入（head/body_end）
- 联系方式（Telegram / WhatsApp）
- 模板模式切换（card/list 布局 + classic/vault 主题）

### 缺口
- 无品牌色（theme_color / primary color）后台配置
- 无社交链接配置（Twitter/X/Facebook/Instagram/Discord 等），Footer 社交区被注释
- 联系方式仅 Telegram/WhatsApp，缺邮箱/电话/地址
- Banner 仅支持 `home_hero` 一个位置，无法投放发现页/分类页/弹窗位
- 首页"推荐商品"区块不可配置（不可选商品、不可改文案、不可显隐）
- 无首页"快捷入口图标宫格"（充值/钱包/推广等业务入口图标可配）
- 无"发现页"概念（无 feed/discover/外链卡片/独立分类信息流）
- 无页面级装修拖拽/页面构建器（page builder）

### 判断：MODIFY
> CMS 基础能力（Banner/文章/公告/品牌/SEO/导航）已完整且前后端闭环，属于 KEEP；但品牌色、社交链接、首页快捷入口、发现页等缺口需要补齐，整体走 MODIFY 路线而非重建。

### 优先级：P1
> 核心装修能力已可用，不阻塞上线；品牌色和社交链接是品牌基础短板，建议 P1 补齐。

### 改造工作量估算：M
> 品牌色（L）+ 社交链接配置（S）+ 首页快捷入口宫格（M）+ Banner 多位置（S）≈ M。

---

## 维度 7：消息通知（Notification）

### 现状

#### 7.1 通知类型

**关键发现：现有 `notification` 模块是「管理员告警中心」，不是「用户站内消息系统」**

后端 `internal/modules/notification/` 全部为**向管理员发送告警**的出站通知：
- 事件类型（`internal/constants/constants.go:373-377`）：
  - `wallet_recharge_success` — 充值成功（通知管理员）
  - `order_paid_success` — 订单支付成功（通知管理员）
  - `manual_fulfillment_pending` — 人工发货待处理（通知管理员）
  - `exception_alert` — 异常告警
- 渠道（`constants.go:382-384`）：`email` / `telegram` / `feishu`，全部发往**管理员配置的收件人列表**
- 发送逻辑：`internal/modules/notification/application/send.go:128-267` — 遍历 settings 中配置的 admin recipients，通过 SMTP/Telegram Bot/飞书 Webhook 发送
- 后台页面：`frontend/admin/src/views/admin/Notifications.vue` — 管理员配置告警渠道/收件人/场景开关/多语言模板/测试发送/发送日志

**用户侧通知类型盘点：**

| 通知类型 | 状态 | 证据 |
|---|---|---|
| 订单状态通知（下单/支付/发货/完成/退款） | 🟡 仅邮件 | `order_email_template.go:42-49` 有 paid/delivered/delivered_with_content/refunded/partially_refunded 模板，通过 SMTP 发邮件给用户邮箱；**无站内消息** |
| 售后回复通知 | 🔴 无 | 无相关事件类型，无站内通知 |
| 工单回复通知 | 🔴 无工单模块 | 项目无 ticket/工单 domain |
| 钱包充值到账通知 | 🟡 仅邮件+管理员告警 | 充值成功触发 `wallet_recharge_success` 告警给管理员；用户侧仅有邮件（若开了 SMTP），无站内通知 |
| 提现状态通知 | 🔴 无 | 无 withdraw 事件类型，无站内通知 |
| 返利/推广佣金到账通知 | 🔴 无 | affiliate 模块无通知触发 |
| 系统公告通知 | 🟡 仅首页弹窗 | `home_announcement` 弹窗在首页展示一次，非消息中心推送 |

#### 7.2 通知能力

**通知 domain model / table** — 🟡 仅管理员告警日志
- `internal/modules/notification/domain/log.go:11-26` — `NotificationLog` 表：EventType/BizType/BizID/Channel/Recipient/Title/Body/Status/ErrorMessage/VariablesJSON
- **关键：无 user_id 字段、无 read/unread 字段、无收件人维度的站内消息表**
- 这是「出站发送日志」，不是「用户收件箱」

**站内消息列表（in-app message list）** — 🔴 完全缺失
- 用户路由（`router/index.ts`）中无 `/me/notifications`、`/messages`、`/inbox` 路由
- 用户 API（`frontend/user/src/api/`）中无 notification.ts / message.ts
- 后端 notification 路由（`routes.go:5-17`）全部挂在 admin 下，无 user 公开路由

**红点 unread 计数** — 🔴 完全缺失
- Navbar.vue 中无 bell 图标、无未读计数徽标（grep 确认 0 匹配）
- 无任何 unread_count API

**已读/未读标记** — 🔴 完全缺失
- 无 notification_reads 表，无 mark-as-read API

**推送（WebSocket / SSE / 轮询）** — 🔴 完全缺失
- 用户端无 WebSocket 连接（grep `websocket|WebSocket|EventSource` 在 user/src 下无业务代码匹配）
- 无 SSE
- 无定时轮询通知 API
- 仅有的"实时"通道是 Telegram Bot（管理员广播用，非用户站内）

**用户通知中心页面** — 🔴 完全缺失
- 无通知中心 UI
- 个人中心 PersonalCenter.vue 无通知入口

**Admin 发送系统通知（给用户）** — 🔴 完全缺失
- 现有 Notifications.vue 是配置管理员告警，不是给用户发消息
- Telegram Bot Broadcast（`TelegramBotBroadcasts.vue`）可向 Telegram 用户广播，但仅限 Telegram 渠道，非站内
- 无"选中用户/全量用户 → 发站内消息"的能力

### 已有能力
- 管理员告警中心（email/telegram/feishu 三渠道），场景化模板（多语言），去重（dedupe TTL），发送日志查询，测试发送
- 订单邮件模板（paid/delivered/refunded 等场景，多语言），通过 SMTP 发到用户邮箱
- Telegram Bot 广播（向 Telegram 用户群发消息）
- 首页公告弹窗（home_announcement，多语言，排期，本地已读记录）

### 缺口
- **无用户站内消息系统**（无 notification_inbox 表、无 user_id、无已读未读）
- **无用户通知中心页面**（路由/UI/API 全缺）
- **无红点/未读计数**（导航栏无 bell、无 badge）
- **无实时推送**（WebSocket/SSE/轮询全缺）
- **无 Admin → 用户的站内广播能力**
- **无业务事件触发站内消息**（订单状态、充值到账、提现、佣金、售后回复均无站内通知）
- 售后/工单类业务本身也未实现，自然无对应通知

### 判断：BUILD
> 现有 notification 模块定位错误（是管理员告警，不是用户消息），用户侧消息通知体系从零开始建设。订单邮件模板是已有资产，可复用其触发点接入新的站内消息。

### 优先级：P0
> 用户无法在站内感知订单/充值/提现状态变化，只能依赖邮件（且 SMTP 不一定配置），这是核心交易闭环的体验硬伤。对于充值/钱包类业务，到账通知是 P0。

### 改造工作量估算：L
> 需新建：user_notifications 表（user_id/type/title/body/biz_type/biz_id/is_read/created_at）+ 用户端 API（列表/未读数/标记已读/全部已读）+ 通知中心页面 + 导航栏 bell+红点 + 各业务事件接入（订单状态机/充值成功/提现状态/佣金到账）+ Admin 站内广播页面。前后端打通约 L。

---

## 汇总表

| 维度 | 子项 | 状态 | 判断 | 优先级 | 工作量 |
|---|---|---|---|---|---|
| **六、前端装修** | 首页 Banner 轮播 | ✅ 后台可配+前端消费 | KEEP | — | — |
| | 核心业务入口宫格 | 🔴 不适用/缺失 | BUILD（可选） | P2 | S |
| | 推荐商品模块 | 🟡 静态前端，不可配 | MODIFY | P2 | S |
| | 首页公告弹窗 | ✅ 后台可配+前端消费 | KEEP | — | — |
| | 发现页/Feed | 🔴 完全缺失 | DROP（暂不需要） | P2 | — |
| | 文章/公告 CMS | ✅ 完整（blog/notice/分类/富文本/关联商品） | KEEP | — | — |
| | Logo/品牌 | ✅ 完整（site_logo/site_icon/site_name） | KEEP | — | — |
| | 品牌色 theme_color | 🔴 完全缺失 | BUILD | P1 | M |
| | Footer 链接 | ✅ 后台可配+前端消费 | KEEP | — | — |
| | SEO meta | ✅ 完整（title/keywords/description 多语言） | KEEP | — | — |
| | 联系方式 | 🟡 仅 Telegram/WhatsApp | MODIFY | P2 | S |
| | 社交链接 | 🔴 完全缺失（Footer 社交区被注释） | BUILD | P1 | S |
| | About 页面配置 | ✅ 完整 | KEEP | — | — |
| | 导航配置 nav_config | ✅ 完整（内置开关+自定义项） | KEEP | — | — |
| | 自定义脚本 | ✅ 完整 | KEEP | — | — |
| **七、消息通知** | 管理员告警中心（email/tg/feishu） | ✅ 完整 | KEEP | — | — |
| | 订单邮件通知（SMTP 给用户） | 🟡 仅邮件 | KEEP | — | — |
| | 用户站内消息（in-app inbox） | 🔴 完全缺失 | BUILD | P0 | L |
| | 未读红点/计数 | 🔴 完全缺失 | BUILD（随站内消息） | P0 | — |
| | 已读/未读标记 | 🔴 完全缺失 | BUILD（随站内消息） | P0 | — |
| | 实时推送 WebSocket/SSE | 🔴 完全缺失 | BUILD/P2（可先轮询） | P2 | M |
| | 用户通知中心页面 | 🔴 完全缺失 | BUILD | P0 | — |
| | Admin 发站内系统通知 | 🔴 完全缺失 | BUILD | P1 | M |
| | 订单/充值/提现/佣金站内通知 | 🔴 完全缺失 | BUILD（随站内消息） | P0 | — |
| | 售后/工单通知 | 🔴 业务本身未实现 | DROP（待售后模块建设） | P2 | — |

### 整体结论

- **前端装修（维度六）**：基础 CMS 能力扎实（Banner/文章/公告/品牌/SEO/导航/Footer 链接/About/脚本），前后端闭环完整。主要缺口在品牌色、社交链接、首页快捷入口宫格。整体 **MODIFY，P1，工作量 M**。
- **消息通知（维度七）**：现有 notification 模块是管理员告警，**用户侧站内消息体系为零**。订单邮件是唯一触达用户的通道（且依赖 SMTP 配置）。需要从零建设用户通知中心（表+API+页面+红点+业务事件接入）。整体 **BUILD，P0，工作量 L**。
