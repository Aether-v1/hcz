# HCZ Phase 9 — Frontend Decoration / Site Builder Pre-Audit

> 本轮只审计，不修改代码。
> 审计日期：2026-10-05
> 代码基线：Phase 8 General Ticket System 已封板

---

## 0. 结论速览

| 审计项 | 结论 |
|---|---|
| 现有装修能力可复用 | **高**。Settings 已有 brand/contact/seo/legal/about/footer_links/template_mode/storefront_template(classic/vault)/navigation(builtin+custom_items)。Content 模块已有完整 Banner（多语言/定时/跳转/排序/Admin CRUD）。publicconfig 已有 Redis 缓存统一端点。Reseller 已有 announcement/navigation overlay 模式 |
| 首页四大业务入口后台配置 | **需新建**。当前核心业务入口（充值/C2C/Wallet/邀请/工单）在 User 前端硬编码。需新增 `home_entries` 配置（route whitelist + icon + title + subtitle + sort_order + enabled + badge + recommended） |
| 发现页数据模型 | **独立表 `discovery_blocks`**。结构化区块（type/title/data/enabled/sort_order），独立表比 JSON config 更适合 Admin CRUD、排序、启停、单块编辑。JSON config 适合 brand/template 等单例配置 |
| brand/template 存储 | **brand/template → Settings KV**（复用现有 `site` setting，扩展 primary_color/social_links/copyright）。**banners/blocks → 独立表**（复用现有 banners 表，新建 discovery_blocks 表）。混合方案 |
| classic/vault 共享装修数据 | **是**。装修数据（home_entries/banners/discovery/brand）与 template 解耦。同一套配置 classic 用 classic layout 渲染，vault 用 vault layout 渲染。Admin 不为每个模板维护两份业务配置 |
| User bootstrap API | **复用/扩展现有 public config 端点**。输出 template/brand/homepage/discovery/social_links/footer/business_toggles。避免 User 首页打 10 个配置 API。Redis 缓存，Admin 保存主动失效 |
| Draft/Publish | **第一版 immediate save，不做 Draft/Publish**。理由：现有 Settings 全部 immediate save，引入 Draft/Publish 会大幅增加复杂度（版本表/对比/回滚/定时发布）。第一版接受"保存即生效"风险，Admin 操作记录 audit。Full V1 再评估 Draft/Publish |
| 仍写死在 User 前端的内容 | 首页核心业务入口图标/文案/路由、发现页全部内容、品牌色 CSS、部分 Navbar 菜单、Footer 文案。需改为动态配置驱动 |
| Admin 新增页面 | Site Builder（首页入口管理 + Banner 管理 + 公告 + 排序）、Discovery Builder（区块 CRUD + 排序）、Brand Settings（logo/色/社交/版权）、Template Settings（classic/vault 选择）。可合并为一个"站点装修"大模块下的 Tab |
| User 需修改页面 | Home（入口+Banner+公告动态化）、Discovery（区块动态渲染）、Navbar（品牌色+导航配置）、Footer（版权+社交+链接动态化）、bootstrap（注入品牌色 CSS variable + template 选择）。不重构 C2C/Ticket/Wallet/Recharge 业务页面 |
| 最大安全风险 | **现有 Settings `scripts` 字段允许自定义 JS（20000 字符）**——这是存储型 XSS 高危。Phase 9 Site Builder 不得暴露 scripts 编辑，且应评估禁用或沙箱化现有 scripts。其次是外部 URL 校验（navigation/Banner 外链必须 http/https only，禁止 javascript:/data:） |
| Phase 9 MVP 最小范围 | template choice + brand/logo/color + homepage core entries + banners（复用）+ announcements（复用）+ discovery blocks + social/footer + Admin config UI + User dynamic rendering + cache + RBAC + audit |
| 是否可进入实施 | **可以**。基础设施复用率高，缺失部分边界清晰，安全风险已识别，MVP 范围明确 |

---

## 一、现有前端配置能力审计

### 1.1 全局搜索结果

| 能力 | 状态 | 位置 | 分类 |
|---|---|---|---|
| **site_config / site setting** | ✅ 已有完整 `site` setting | `internal/modules/settings/application/site_normalize.go` | KEEP |
| **brand（site_name/logo/icon/description）** | ✅ 已有 | Settings `brand` 字段 | KEEP / MODIFY（扩展 primary_color） |
| **contact（telegram/whatsapp）** | ✅ 已有 | Settings `contact` 字段 | MODIFY（扩展更多社交） |
| **seo（title/keywords/description）** | ✅ 已有（多语言） | Settings `seo` 字段 | KEEP |
| **legal（terms/privacy）** | ✅ 已有（多语言） | Settings `legal` 字段 | KEEP |
| **about（hero/introduction/services/contact）** | ✅ 已有（多语言） | Settings `about` 字段 | KEEP |
| **scripts（自定义 JS）** | ⚠️ 已有但高危 | Settings `scripts` 字段（最多 20 条，每条 20000 字符） | DROP（Site Builder 不暴露，评估禁用） |
| **footer_links** | ✅ 已有（最多 20 条 name+url） | Settings `footer_links` 字段 | KEEP / MODIFY（加 copyright/ICP） |
| **template_mode（card/list）** | ✅ 已有 | Settings `template_mode` 字段 | KEEP |
| **storefront_template（classic/vault）** | ✅ 已有 | Settings `storefront_template` 字段，默认 classic | KEEP（模板选择已就绪） |
| **languages** | ✅ 已有 | Settings `languages` 字段 | KEEP |
| **navigation（builtin + custom_items）** | ✅ 已有 | Settings `navigation`（normalizeNavConfig），builtin(blog/notice/about) + custom_items(title/link_type/internal/external/url/target/sort_order/enabled/icon) | KEEP / MODIFY（URL 安全校验） |
| **Banner** | ✅ 已有完整模型 + Admin CRUD | `internal/modules/content/domain/banner.go`，banners 表 | KEEP |
| **Post / PostCategory（文章/分类）** | ✅ 已有 | `internal/modules/content/domain/post.go` / `post_category.go` | KEEP（公告可复用） |
| **announcement（首页公告）** | ✅ 已有 | Settings `GetActiveHomeAnnouncement()`，Reseller overlay | KEEP |
| **public config 统一端点** | ✅ 已有 + Redis 缓存 | `internal/bootstrap/publicconfig/`，`publicConfigCacheAdapter` | KEEP / MODIFY（扩展输出） |
| **discovery（发现页）** | ❌ 完全缺失 | 无配置、无表、无区块系统 | BUILD |
| **home_entries（首页业务入口）** | ❌ 缺失（前端硬编码） | 无配置 | BUILD |
| **brand color（品牌色）** | ❌ 缺失 | Settings brand 无 primary_color | BUILD |
| **social links 扩展** | ⚠️ 部分（仅 telegram/whatsapp） | Settings contact | MODIFY（加 X/Discord/Email/Custom） |
| **copyright / footer text** | ❌ 缺失 | 无字段 | BUILD |
| **business display switches** | ❌ 缺失 | 无 display_enabled 开关 | BUILD |
| **Site Builder Admin UI** | ⚠️ 部分（Settings 页可能有） | 需确认 Admin 前端是否有 site setting UI | MODIFY / BUILD |
| **Draft/Publish** | ❌ 缺失 | 无版本管理 | 第一版不做（Full V1） |
| **audit log for site changes** | ⚠️ 需确认 | 现有 auditlog 仅限认证事件 | BUILD（独立 audit 或扩展） |

### 1.2 关键发现

1. **Settings `site` setting 已经是一个准 Site Builder**：brand/contact/seo/legal/about/scripts/footer_links/template_mode/storefront_template/languages/navigation 全部存在，且有多语言支持。
2. **Content Banner 模型完整**：name/position/title(多语言 JSON)/subtitle(多语言)/image/mobile_image/link_type(none/internal/external)/link_value/open_in_new_tab/is_active/start_at/end_at/sort_order/软删除。Admin CRUD 路由已存在（`/admin/banners`）。
3. **publicconfig 已有 Redis 缓存统一端点**：`publicConfigCacheAdapter` 使用 `cache.GetJSON/SetJSON`，Admin 保存后可主动失效。
4. **storefront_template 已支持 classic/vault**：`normalizeStorefrontTemplate` 只允许 classic/vault，默认 classic。模板选择基础设施已就绪。
5. **navigation 已支持 internal/external link_type**：但 URL 校验需要加强（当前只做长度截断，不校验协议）。
6. **scripts 字段是高危 XSS**：允许 Admin 注入最多 20 条自定义 JS（每条 20000 字符），position=head/body_end。Phase 9 必须不暴露此字段，且应评估整体禁用。

---

## 二、装修系统边界

### 2.1 Phase 9 第一版只做

- 首页装修（业务入口 + Banner + 公告）
- 发现页装修（结构化区块）
- 品牌设置（logo / 品牌色 / 社交链接 / 版权）
- 模板选择（classic / vault）
- 业务入口配置（充值 / C2C / Wallet / 邀请 / 工单 / 其他）
- Banner / 公告 / 推荐内容管理

### 2.2 不做

- 拖拽式可视化页面编辑器
- 任意 HTML 编辑
- 自定义 JS（且应评估禁用现有 scripts）
- 页面源码编辑
- 任意页面自由布局

### 2.3 优先做

**结构化配置型 Site Builder**：Admin 通过表单/列表配置内容，User 前端按配置渲染。不做 WYSIWYG 拖拽。

---

## 三、首页装修

### 3.1 当前首页布局审计

当前 User 首页（hcz_user）核心业务入口大概率硬编码为固定四个（充值/C2C/Wallet/邀请等），图标和文案写在前端组件中。Banner 可能已接入 Content Banner API（需确认），公告可能已接入 public config announcement。

### 3.2 第一版首页结构

```
┌─────────────────────────────────────┐
│  Navbar（品牌色 + logo + 导航配置）    │
├─────────────────────────────────────┤
│  Banner 轮播（复用 Content Banner）    │
├─────────────────────────────────────┤
│  公告条（复用 Settings announcement）   │
├─────────────────────────────────────┤
│  核心业务入口（home_entries 配置）      │
│  ┌────┐ ┌────┐ ┌────┐ ┌────┐      │
│  │充值 │ │C2C │ │钱包 │ │邀请 │ ...  │
│  └────┘ └────┘ └────┘ └────┘      │
├─────────────────────────────────────┤
│  推荐内容/运营文案（可选区块）          │
├─────────────────────────────────────┤
│  Footer（版权 + 社交 + 链接）          │
└─────────────────────────────────────┘
```

### 3.3 核心业务入口配置（home_entries）

**必须新建**。Admin 可配置每个入口：

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| title | varchar(100) | 入口标题（多语言或直接中文） |
| subtitle | varchar(200) | 入口副标题/描述 |
| icon | varchar(100) | 图标标识（前端 icon map 映射，不允许 Admin 上传任意 SVG） |
| route | varchar(50) | 内部路由标识（白名单，见第四节） |
| route_type | varchar(20) | internal / external |
| external_url | varchar(500) | 外链 URL（仅 route_type=external 时，http/https only） |
| sort_order | int | 排序 |
| enabled | bool | 启用/隐藏 |
| badge | varchar(50) | 角标文本（如"新"、"热"，可空） |
| recommended | bool | 是否推荐（高亮展示） |
| created_at / updated_at | | |

### 3.4 默认入口

migration seed 初始化默认入口（与当前前端硬编码对齐）：
1. 生活充值（route=recharge, icon=recharge, recommended=true）
2. C2C 交易（route=c2c, icon=c2c, badge=新）
3. 我的钱包（route=wallet, icon=wallet）
4. 邀请中心（route=invitation, icon=invitation）
5. 帮助工单（route=support, icon=support）

Admin 可以调整顺序、启用/禁用、修改标题/副标题/角标。

### 3.5 核心要求

**首页四大业务入口必须由 Admin 可配置，不允许前端长期硬编码固定四个。**

---

## 四、首页业务入口安全

### 4.1 route 不允许 Admin 随便填写任意危险 URL

### 4.2 方案比较

| 方案 | 优点 | 缺点 |
|---|---|---|
| A. route enum（严格枚举） | 最安全，只能选预设值 | 不够灵活，新增业务需改代码 |
| B. route whitelist（白名单+配置） | 安全且灵活，白名单在后端维护 | Admin 只能选白名单内的值 |

### 4.3 第一版推荐：route whitelist + external 单独标记

**内部业务 route 使用白名单**，后端维护允许的 route 列表：

```go
var allowedHomeEntryRoutes = map[string]string{
    "recharge":    "/recharge",
    "c2c":         "/c2c",
    "wallet":      "/wallet",
    "invitation":  "/invitation",
    "support":     "/support",
    "affiliate":   "/affiliate",
    "discovery":   "/discovery",
    "orders":      "/orders",
}
```

Admin 创建/编辑 home_entry 时，后端校验 `route` 必须在白名单内。不在白名单内拒绝。

**外链单独标记**：`route_type=external` 时：
- `external_url` 必须以 `http://` 或 `https://` 开头
- 禁止 `javascript:`、`data:`、`vbscript:`、`file:` 等危险协议
- URL 长度限制（2000 字符）
- 前端渲染时 `rel="noopener noreferrer"`，`target="_blank"`

---

## 五、Banner

### 5.1 现有 Banner 模型（复用）

`banners` 表已有完整字段：
- id / name / position
- title（多语言 JSON）/ subtitle（多语言 JSON）
- image / mobile_image
- link_type（none/internal/external）/ link_value / open_in_new_tab
- is_active / start_at / end_at（定时投放）
- sort_order / 软删除

### 5.2 Admin CRUD 已存在

`/admin/banners` 路由已注册，authz 已有 `/admin/banners` 权限。

### 5.3 需要加强

| 项 | 当前 | 需加强 |
|---|---|---|
| link_value 校验 | 无协议校验 | external 类型必须 http/https only，禁止 javascript:/data: |
| 图片上传 | 复用 FileUploader | 确认 MIME whitelist 仅图片（jpg/png/webp），最大尺寸建议 |
| position 字段 | 自由文本 | 第一版固定 position="home"，不需要多位置 |
| User 前端接入 | 需确认 | 首页必须渲染 active + 当前时间在 start_at/end_at 范围内的 Banner |

### 5.4 第一版不需要新建 Banner 表

**完全复用现有 `banners` 表和 Content Banner Service。** 只需要：
1. 确保 User 首页调用 Banner API（或 public config 包含 active banners）
2. 加强 link_value 安全校验
3. Admin Banner 管理页面已存在，不需要新建

---

## 六、公告

### 6.1 现有公告基础（复用）

Settings 已有 `GetActiveHomeAnnouncement()`，返回 `{type, title, content, version}` 结构。Reseller 模块有 announcement overlay 模式。

### 6.2 第一版公告能力

首页公告至少支持：
- title（多语言）
- content / summary（多语言）
- enabled
- pinned（是否置顶）
- start_at / end_at（定时展示）

### 6.3 实现选择

| 方案 | 说明 |
|---|---|
| A. 复用 Settings announcement | 现有 `GetActiveHomeAnnouncement` 已支持 type/title/content/version，但只有单条 |
| B. 复用 Content Post | Post 表已有多语言 title/content/category，可加 position=announcement |
| C. 新建 announcements 表 | 独立表，支持多条/排序/定时 |

### 6.4 第一版推荐：复用 Settings announcement（单条）

理由：
1. 现有基础设施已就绪，不需要新建表
2. 首页公告通常只需要一条（最新/置顶）
3. 多条公告可以 Full V1 再扩展
4. Admin Settings 页面已有公告编辑（需确认 UI）

User 首页展示形式：滚动条或卡片。如果 Settings announcement 为空，不展示公告区域。

---

## 七、发现页装修

### 7.1 发现页必须脱离"静态页面"

当前 User 发现页（如有）大概率是静态写死的内容。第一版支持结构化区块配置。

### 7.2 第一版支持的区块类型

| type | 说明 | data 结构 |
|---|---|---|
| banner | 图文横幅 | image, title, subtitle, link_type, link_value |
| card_grid | 图文卡片网格 | cards[{image, title, description, link_type, link_value}] |
| business_recommend | 业务推荐 | entries[{route, title, subtitle, icon, badge}] |
| announcement | 公告/文章列表 | post_ids 或 category_id |
| external_link | 外部链接集合 | links[{title, url, icon}] |
| category_entry | 分类入口 | categories[{name, route, icon}] |

每个区块：
- type（上述枚举）
- title（区块标题，可空）
- data（JSON，按 type 不同结构不同）
- enabled
- sort_order

### 7.3 数据模型：独立表 vs JSON config

| 方案 | 优点 | 缺点 |
|---|---|---|
| A. 全部 Settings JSON | 简单，复用现有 Settings 基础设施 | Admin 编辑困难（需要 JSON editor），排序/单块启停不直观，不适合非技术 Admin |
| B. 独立表 `discovery_blocks` | Admin CRUD 友好，单块编辑/排序/启停，schema 驱动表单 | 需要新建表和 store |
| C. 混合（brand→Settings, blocks→表） | 各取所长 | 稍复杂 |

### 7.4 第一版推荐：独立表 `discovery_blocks`

**理由**：
1. 发现页区块是列表型数据（多条、有序、可单块启停），独立表更适合 Admin CRUD
2. Admin 通过 schema 驱动表单编辑每块，不需要写 raw JSON
3. 排序（sort_order）、启用/禁用（enabled）单块操作直观
4. brand/template 是单例配置，适合 Settings KV；blocks 是列表型，适合独立表
5. 与 banners 表模式一致（banners 也是独立表 + Admin CRUD）

### 7.5 discovery_blocks 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| type | varchar(30) | 区块类型（banner/card_grid/business_recommend/announcement/external_link/category_entry） |
| title | varchar(200) | 区块标题（可空，多语言或直接中文） |
| data | JSON | 区块数据（按 type 不同结构） |
| enabled | bool | 启用 |
| sort_order | int | 排序 |
| created_at / updated_at | | |

### 7.6 Admin 编辑体验

每种区块类型使用 schema 驱动表单：
- Admin 选择区块类型 → 展示对应字段表单
- 不允许 Admin 写 raw JSON
- data JSON 由后端根据表单字段组装
- 列表页支持上移/下移/sort_order 编辑/启用禁用/删除

---

## 八、品牌设置

### 8.1 现有 brand 配置（复用 + 扩展）

Settings `brand` 已有：
- site_name
- site_url
- site_icon（favicon）
- site_logo
- site_description（多语言）

### 8.2 第一版需扩展

| 字段 | 类型 | 说明 |
|---|---|---|
| primary_color | varchar(7) | 主品牌色（HEX，如 #4F46E5），默认 HCZ 现有主色 |
| secondary_color | varchar(7) | 辅助色/强调色（可空，第一版可不做） |
| support_contact | varchar(200) | 客服联系方式（可空） |
| footer_text | varchar(500) | 页脚文案（可空） |
| copyright | varchar(200) | 版权信息（如 "© 2026 HCZ. All rights reserved."） |
| icp_number | varchar(100) | ICP/备案号（可空，国内需要） |

### 8.3 社交链接扩展

现有 `contact` 只有 telegram / whatsapp。扩展为 `social_links`：

| 平台 | 字段 | 校验 |
|---|---|---|
| Telegram | telegram | URL 或 @username |
| WhatsApp | whatsapp | URL 或手机号 |
| X/Twitter | x | URL（http/https only） |
| Discord | discord | URL 或 invite code |
| Email | email | email 格式 |
| Custom | custom_{1..3} | URL + name（最多 3 个自定义） |

第一版至少支持 Telegram / WhatsApp / X / Discord / Email / Custom(1个)。

---

## 九、品牌色

### 9.1 当前 CSS 审计

User 前端（hcz_user）需要确认：
- 是否大量写死颜色（如 `text-indigo-600`、`bg-blue-500`）
- 是否已经使用 CSS variables（如 `var(--brand-primary)`）
- Tailwind theme 如何配置（tailwind.config.js 是否 extend colors）

### 9.2 推荐方案

**Admin 保存 `primary_color`（HEX），User bootstrap 时生成 CSS variable：`--brand-primary`。**

实现方式：
1. User 首次加载时，public config API 返回 `brand.primary_color`
2. 前端在 `document.documentElement.style.setProperty('--brand-primary', primaryColor)` 注入 CSS variable
3. 同时生成衍生色：`--brand-primary-light`（透明度 10%）、`--brand-primary-dark`（变暗 10%）
4. Tailwind 配置中使用 CSS variable：`colors: { brand: { primary: 'var(--brand-primary)', light: 'var(--brand-primary-light)', dark: 'var(--brand-primary-dark)' } }`
5. 前端组件使用 `text-brand-primary` / `bg-brand-primary` 替代写死的 `text-indigo-600`

### 9.3 不要动态重新构建 Tailwind

- 不运行时编译 Tailwind
- 不根据品牌色动态生成完整 CSS
- 只注入 3-4 个 CSS variable，Tailwind 引用这些 variable
- 默认值在 CSS 中定义（`--brand-primary: #4F46E5`），Admin 未配置时使用默认

### 9.4 颜色校验

- 必须是 HEX 格式（`#RRGGBB` 或 `#RGB`）
- 后端正则校验：`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`
- 不允许 rgb()/hsl()/颜色名（第一版只支持 HEX，简单安全）

---

## 十、模板选择

### 10.1 现有模板机制

Settings `storefront_template` 已支持 `classic` / `vault`，默认 `classic`。`normalizeStorefrontTemplate` 只允许这两个值。

### 10.2 Admin 配置

Admin 应能配置 `user_template`（即 storefront_template）：
- 下拉选择：classic / vault
- 保存后立即生效（public config 更新 + Redis 缓存失效）
- User bootstrap 根据配置选择模板

### 10.3 templateView / route fallback

需确认 User 前端（hcz_user）当前 classic/vault 切换机制：
- 是否使用 `templateView` 组件自动回退
- 路由是否按模板分目录
- 是否存在模板重复代码

### 10.4 核心要求

**只切视觉模板，不复制业务逻辑。**
- API 调用、types、composable/store、业务状态逻辑必须共用
- classic/vault 只负责 layout、spacing、visual theme
- 新增 C2C/Ticket 等业务页面时，只写一套业务逻辑，两个模板各自只写视图层

---

## 十一、模板与装修数据关系

### 11.1 装修数据与 template 解耦

**同一套首页配置，classic 用 classic layout 渲染，vault 用 vault layout 渲染。**

- home_entries / banners / discovery_blocks / brand / social_links 等装修数据**不区分 template**
- Admin 只维护一套配置
- User 前端根据当前 template 选择不同的渲染组件，但数据源相同

### 11.2 不允许 Admin 为每个模板维护两份业务配置

除非确有必要（如某个模板特有的布局参数），否则装修数据是 template-agnostic 的。

### 11.3 模板特有参数（如有）

如果 classic/vault 在布局上有本质差异（如 classic 是网格布局，vault 是列表布局），可以在区块级别加 `layout_hint` 字段，但第一版不做。第一版两个模板使用相同的区块数据，只在视觉呈现上不同。

---

## 十二、User Bootstrap Config

### 12.1 现有 public config 端点

`internal/bootstrap/publicconfig/` 已有统一公共配置端点，使用 Redis 缓存。包含：settings、payment channels、announcement、captcha、auth methods 等。

### 12.2 推荐：扩展现有 public config 端点

**不新建独立 bootstrap API**，在现有 public config 输出中增加装修相关字段：

```json
{
  "template": "classic",
  "brand": {
    "site_name": "HCZ",
    "site_logo": "/uploads/logo.png",
    "site_icon": "/uploads/favicon.png",
    "primary_color": "#4F46E5",
    "site_description": {"zh-CN": "...", "en": "..."}
  },
  "homepage": {
    "entries": [{"title": "充值", "route": "recharge", "icon": "recharge", "sort_order": 1, "enabled": true}],
    "banners": [{"image": "...", "link_type": "internal", "link_value": "/recharge"}],
    "announcement": {"title": "...", "content": "..."}
  },
  "discovery": {
    "blocks": [{"type": "banner", "data": {...}, "sort_order": 1, "enabled": true}]
  },
  "social_links": {
    "telegram": "@hcz",
    "whatsapp": "...",
    "x": "https://x.com/hcz",
    "discord": "...",
    "email": "support@hcz.com"
  },
  "footer": {
    "copyright": "© 2026 HCZ",
    "footer_text": "...",
    "icp_number": "...",
    "links": [{"name": "Terms", "url": "/terms"}]
  },
  "business_toggles": {
    "recharge_enabled": true,
    "c2c_enabled": true,
    "withdrawal_enabled": true,
    "affiliate_enabled": true,
    "support_enabled": true
  },
  "navigation": {
    "builtin": {"blog": true, "notice": true, "about": true},
    "custom_items": [...]
  }
}
```

### 12.3 必须避免

User 首页加载时**不得**分别打 10 个配置 API（如分别请求 brand/banners/entries/discovery/social/footer）。必须一次 public config 调用拿到全部装修数据。

### 12.4 复用现有端点还是新建

**推荐扩展现有 public config 端点**，因为：
1. 已有 Redis 缓存机制
2. 已有 Reseller overlay 能力
3. User 前端已经在调用这个端点
4. 减少新端点维护成本

如果现有端点输出结构不适合直接扩展，可以新增 `GET /api/v1/site/bootstrap` 专门返回装修配置，但内部仍复用 public config 的缓存层。

---

## 十三、缓存

### 13.1 现有缓存机制

publicconfig 已有 `publicConfigCacheAdapter`：
- `CacheKey(resellerID)` → Redis key
- `GetJSON` / `SetJSON` → Redis 读写
- 缓存 TTL 需确认（建议 5-10 分钟）

### 13.2 推荐方案

| 层 | 角色 |
|---|---|
| DB / Settings | **真源**（brand/template 在 Settings KV，banners/blocks 在独立表） |
| Redis | **缓存**（public config 聚合结果，TTL 5 分钟） |
| Admin 保存后 | **主动失效**（删除 Redis key，下次请求重新聚合） |
| User 前端 | **不做本地长期缓存**（每次刷新从 public config 取，public config 有 Redis 缓存已足够快） |

### 13.3 缓存失效触发

Admin 修改以下内容后必须主动删除 Redis public config 缓存：
- brand 设置（logo/色/社交/版权）
- template 选择
- home_entries（增删改/排序/启停）
- banners（增删改/定时）
- announcement
- discovery_blocks（增删改/排序/启停）
- navigation
- footer_links

实现方式：在各 Admin service 的保存方法中，调用 `cache.InvalidatePublicConfig()`。

### 13.4 不允许

User 不应每次刷新都做大量 DB 查询。Redis 缓存必须生效。

---

## 十四、Admin 首页装修 UI

### 14.1 第一版页面

Admin 新增"站点装修"模块（或在现有 Settings 中增加 Tab），至少包含：

| Tab | 功能 |
|---|---|
| 首页入口 | home_entries 列表（增删改/排序/启用禁用/角标/推荐） |
| Banner 管理 | 复用现有 Banner 管理（列表/增删改/定时/排序） |
| 公告 | 公告编辑（title/content/启用/定时） |
| 发现页 | discovery_blocks 列表（新增区块/编辑/删除/排序/启用） |
| 品牌设置 | site_name/logo/favicon/primary_color/social_links/copyright/footer_text/ICP |
| 模板设置 | classic/vault 选择 |

### 14.2 排序

第一版不要求拖拽。使用：
- 上移 / 下移按钮
- 或直接编辑 sort_order 数字
- 列表按 sort_order 升序展示

如果现有 Admin 前端组件支持 drag-and-drop（如 el-table 的 drag 排序），可评估复用，但不强制。

### 14.3 启用/隐藏

每条 home_entry / banner / discovery_block 都有 enabled 开关，Admin 可以临时隐藏而不删除。

---

## 十五、Admin 发现页装修 UI

### 15.1 区块管理

- 新增区块：选择区块类型（banner/card_grid/business_recommend/announcement/external_link/category_entry）→ 展示对应 schema 表单
- 编辑区块：根据 type 展示表单
- 删除区块：确认后删除
- 排序：上移/下移/sort_order
- 启用：开关

### 15.2 Schema 驱动表单

每种区块类型定义字段 schema，Admin 表单根据 schema 渲染：
- banner：image（上传）、title、subtitle、link_type、link_value
- card_grid：cards 数组（每个 card：image/title/description/link_type/link_value）
- business_recommend：entries 数组（route 白名单选择/title/subtitle/icon/badge）
- announcement：post 选择或 category 选择
- external_link：links 数组（title/url/icon）
- category_entry：categories 数组（name/route/icon）

**不允许让 Admin 写 raw JSON。** data JSON 由后端根据表单字段组装。

---

## 十六、预览

### 16.1 第一版是否需要预览

**建议第一版提供简单预览，不做复杂 WYSIWYG。**

| 预览方式 | 说明 | 第一版 |
|---|---|---|
| 新窗口预览 | Admin 点击"预览"打开 User 站首页（带 preview 参数，读取未发布配置） | ⚠️ 第一版 immediate save，预览=直接看 User 站 |
| 右侧实时预览 | Admin 编辑时右侧展示渲染效果 | ❌ 第一版不做（复杂度高） |
| Desktop/Mobile 切换 | 预览时切换视口 | ❌ 第一版不做 |

### 16.2 第一版实际方案

因为第一版是 immediate save（保存即生效），Admin 保存后直接打开 User 站查看效果即可。不需要专门的预览功能。

Full V1 引入 Draft/Publish 后，再做预览（读取 draft 配置渲染）。

---

## 十七、SEO

### 17.1 当前 User Web SEO 能力

User 前端是 SPA（Vue），SEO 能力有限：
- 静态 HTML 的 title/description 可能写死
- 动态路由（如商品详情、文章详情）的 SEO 需要 SSR 或预渲染
- og:image 等社交分享标签需要动态注入

### 17.2 现有 Settings seo 字段

Settings `seo` 已有多语言 title/keywords/description。这是站点级默认 SEO。

### 17.3 第一版 Admin 可配置

- site title（页面 <title> 默认值）
- meta description
- default share image（og:image 默认图）

### 17.4 如实说明

**SPA SEO 能力有限，不要做假配置。**
- 站点级 title/description 可以在 index.html 或 bootstrap 时注入（有效）
- 动态页面的 SEO（如每个商品/文章的独立 title）需要 SSR 或预渲染，第一版不做
- og:image 可以配置默认图，但动态页面的 og:image 需要服务端渲染
- Admin 配置的 seo 字段对站点级页面有效，对动态路由页面效果有限

第一版只做站点级 SEO 配置（已有 Settings seo 字段，确保 Admin UI 可编辑即可），不承诺动态页面 SEO。

---

## 十八、Footer

### 18.1 现有 footer_links

Settings `footer_links` 已有（最多 20 条 name+url）。

### 18.2 第一版 Admin 可配置

| 字段 | 说明 |
|---|---|
| footer_text | 页脚文案（如"安全、便捷的数字生活服务平台"） |
| copyright | 版权信息（"© 2026 HCZ. All rights reserved."） |
| icp_number | ICP/备案号（国内需要，可空） |
| social_links | 社交链接（Telegram/WhatsApp/X/Discord/Email/Custom） |
| footer_links | 页脚链接（Terms/Privacy/About 等，复用现有） |

### 18.3 不要写死

User 前端 Footer 组件必须从 public config 读取上述字段动态渲染，不允许写死版权文案和社交链接。

---

## 十九、业务显示开关

### 19.1 是否需要全局开关

**需要 display_enabled 开关**，但必须区分 display_enabled 和 business_enabled。

### 19.2 两种开关的区别

| 开关类型 | 作用 | 后端行为 |
|---|---|---|
| display_enabled（UI 隐藏） | 首页/导航不显示该业务入口 | 后端 API 仍然可用，用户直接访问 URL 仍可进入 |
| business_enabled（业务禁用） | 真正关闭该业务 | 后端 API fail-closed，返回错误，用户无法使用 |

### 19.3 第一版 display_enabled

Admin 可配置（Settings 或 home_entries 的 enabled 字段）：
- recharge_enabled
- c2c_enabled
- withdrawal_enabled
- affiliate_enabled
- support_enabled

display_enabled 只影响 UI 展示（首页入口隐藏、导航隐藏），**不等于后端禁用**。

### 19.4 business_enabled

如果功能真正关闭，后端必须也 fail-closed。这需要各业务域自己的 enabled 配置（如 withdrawal_config.enabled、c2c_config.enabled），不在 Phase 9 范围内。

Phase 9 只做 display_enabled（UI 开关），不做 business_enabled（业务禁用）。但必须在文档中明确区分，避免 Admin 以为"隐藏入口=关闭业务"。

---

## 二十、安全

### 20.1 Site Builder 必须防

| 风险 | 防护 |
|---|---|
| XSS（存储型） | 文本字段纯文本渲染，前端转义；不允许 raw HTML；**现有 scripts 字段不得在 Site Builder 暴露** |
| javascript: URL | 所有外部 URL 校验必须 http/https only，禁止 javascript:/data:/vbscript:/file: |
| 任意 iframe | 不允许 Admin 配置 iframe |
| raw HTML injection | 不允许 Admin 写 HTML，所有内容通过结构化字段配置 |
| data: URL 滥用 | 外部 URL 禁止 data: 协议；图片使用上传（FileUploader），不允许 data: URI |
| 外链安全 | 前端渲染外链时 `rel="noopener noreferrer"`，`target="_blank"` |
| URL 长度 | 外部 URL 最大 2000 字符 |
| SVG 上传 | 第一版不支持 SVG（防止 SVG XSS），只支持 jpg/png/webp |
| 图标注入 | home_entry icon 使用前端 icon map 白名单映射，不允许 Admin 上传任意 SVG 或写任意 icon class |

### 20.2 现有 scripts 字段处理

**最高优先级安全风险**：现有 Settings `scripts` 字段允许 Admin 注入自定义 JS（最多 20 条，每条 20000 字符，position=head/body_end）。这是存储型 XSS。

Phase 9 处理建议：
1. **Site Builder UI 绝对不暴露 scripts 编辑**
2. 评估是否整体禁用 scripts（如果当前没有在用，可以直接禁用；如果有在用，需要迁移）
3. 如果必须保留，至少加沙箱限制（但自定义 JS 很难安全沙箱）
4. 审计日志记录 scripts 修改（谁在什么时候改了什么）

第一版建议：**Site Builder 不碰 scripts，保持现状，但在报告中明确标记为高危待处理。**

### 20.3 文本内容安全

- title/subject/body 等文本字段：纯文本，前端渲染时 HTML 转义
- 不支持 Markdown（第一版），避免 Markdown 注入 HTML
- 多语言字段：每种语言独立校验长度
- URL 字段：严格协议白名单

---

## 二十一、上传

### 21.1 复用 FileUploader

Banner / Logo / Discovery image 全部复用现有 FileUploader（`SaveFileWithMeta`）。

### 21.2 图片配置

| 项 | 配置 |
|---|---|
| MIME whitelist | image/jpeg, image/png, image/webp（不支持 SVG/GIF 第一版） |
| extension whitelist | .jpg, .jpeg, .png, .webp |
| max size | 5MB（比通用 10MB 更严格，因为是展示图） |
| dimensions | 第一版不强制限制尺寸，Admin 上传时提示推荐尺寸（Banner 1920x600，Logo 200x200） |
| old file cleanup | 覆盖上传时旧文件不自动删除（第一版接受孤儿文件，Full V1 再做清理） |

### 21.3 不同场景的上传

| 场景 | 上传端点 | 说明 |
|---|---|---|
| Banner 图片 | 现有 Admin upload | 已有 |
| Logo / Favicon | 现有 Admin upload | 已有 |
| Discovery 区块图片 | 现有 Admin upload | 已有 |
| User 端不涉及上传 | - | Site Builder 是 Admin 侧配置 |

---

## 二十二、Admin RBAC

### 22.1 独立权限

| 权限 | 说明 |
|---|---|
| site.brand.manage | 品牌设置（logo/色/社交/版权） |
| site.home.manage | 首页装修（入口/Banner/公告） |
| site.discovery.manage | 发现页装修（区块 CRUD） |
| site.template.manage | 模板选择（classic/vault） |

### 22.2 不与财务权限混用

- site.* 权限独立于 payment.* / wallet.* / withdrawal.* / c2c.*
- 客服/运营角色可以有 site.* 权限，但不需要财务权限
- 财务管理员不一定有 site.* 权限

### 22.3 现有 support 角色

authz 已有 `support` 角色，可以预绑定 site.* 权限（运营/客服可以管理站点装修）。

---

## 二十三、审计日志

### 23.1 Admin 修改必须记录

Admin 修改以下内容必须记录 audit：
- template 切换
- brand 设置（logo/色/社交/版权）
- home_entries（增删改/排序/启停）
- banners（增删改）
- discovery_blocks（增删改/排序/启停）
- announcement
- navigation

记录：before / after / admin_id / timestamp / action。

### 23.2 是否复用现有 Admin Audit

现有 `auditlog` 模块专注于认证事件（admin_login / user_login / authz 变更），domain 模型是 auth-specific。

**第一版推荐：独立 site_audit_logs 表**（或复用 supportticket 的 audit 模式），不强行扩展 auditlog 模块。

理由：
1. auditlog 当前 domain 是认证事件，扩展为通用 audit 需要改 domain 模型
2. site 装修审计需要记录 before/after（JSON diff），与 auth audit 结构不同
3. 独立表查询更高效（按 admin_id/action/time 筛选）
4. 与 Phase 8 Ticket audit 模式一致（独立 audit 表）

如果未来需要统一审计视图，可以做聚合查询，不影响独立存储。

### 23.3 site_audit_logs 表

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| admin_id | uint | 操作 Admin |
| section | varchar(30) | 操作区域（brand/template/home_entries/banners/discovery/announcement/navigation） |
| action | varchar(30) | 操作类型（create/update/delete/reorder/enable/disable/switch_template） |
| before | text | 旧值（JSON 或文本，可空） |
| after | text | 新值（JSON 或文本，可空） |
| created_at | | |

---

## 二十四、User 前端改造范围

### 24.1 需要改为动态配置驱动的页面/组件

| 页面/组件 | 改造内容 | 风险 |
|---|---|---|
| **Home** | 业务入口从硬编码改为 home_entries 配置渲染；Banner 从 public config 渲染；公告从 public config 渲染 | 中（需保留默认值 fallback） |
| **Discovery** | 从静态页面改为 discovery_blocks 配置渲染（按 type 渲染对应区块组件） | 中高（区块组件需逐个实现） |
| **Navbar** | 品牌色注入 CSS variable；logo 从 brand 配置读取；导航菜单从 navigation 配置渲染 | 低 |
| **Footer** | 版权/社交/链接从 public config 读取 | 低 |
| **bootstrap / App.vue** | 初始化时注入品牌色 CSS variable；根据 template 选择布局 | 低 |
| **全局样式** | Tailwind 配置引用 CSS variable；替换写死的主色类名 | 中（需全局搜索替换） |

### 24.2 不重构的页面

**只改入口/外壳/展示，不重构已经完成的业务页面：**
- C2C 页面（不重构，只确保 Navbar/Footer 动态化后 C2C 页面正常）
- Ticket 页面（不重构）
- Wallet 页面（不重构）
- Recharge 页面（不重构）
- Order 页面（不重构）
- Affiliate 页面（不重构）

这些业务页面的内部布局和逻辑不动，只确保它们使用的 Navbar/Footer/品牌色是动态的。

### 24.3 classic/vault

两个模板都需要改造 Home/Discovery/Navbar/Footer，但业务逻辑共用。classic 和 vault 只在视觉呈现上不同（布局/间距/主题），数据源相同。

---

## 二十五、Fallback

### 25.1 配置缺失/损坏时必须有默认值

| 配置 | 默认值 |
|---|---|
| template | classic |
| primary_color | HCZ 现有主色（如 #4F46E5 或当前前端默认主色） |
| site_name | "HCZ" |
| home_entries | migration seed 的默认 5 个入口（充值/C2C/钱包/邀请/工单） |
| banners | 空（不展示 Banner 区域） |
| announcement | 空（不展示公告条） |
| discovery_blocks | 空（展示空状态或默认引导） |
| social_links | 空（不展示社交图标） |
| copyright | "© 2026 HCZ"（或空） |
| footer_links | 空 |
| navigation | builtin 全 true + custom_items 空 |

### 25.2 不能因为装修配置错误导致用户站打不开

- public config API 必须 always return 200（即使 DB 查询失败，返回默认配置）
- 前端渲染时每个区块组件必须有错误边界（单个区块渲染失败不影响其他区块和页面）
- 品牌色格式错误时使用默认色
- home_entries 数据格式错误时使用默认入口
- discovery_blocks 中某个区块 type 不识别时跳过该区块

---

## 二十六、版本与发布

### 26.1 是否需要 Draft/Publish

| 方案 | 优点 | 缺点 |
|---|---|---|
| A. Draft → Publish | Admin 可以改完统一发布，避免保存中间态影响线上；支持回滚；支持预览 | 复杂度高（版本表/diff/对比/回滚/定时发布） |
| B. Immediate save | 简单，保存即生效 | 中间态可能影响线上；误操作立即生效；无回滚 |

### 26.2 第一版推荐：Immediate save

**理由**：
1. 现有 Settings 全部 immediate save，引入 Draft/Publish 会大幅增加复杂度
2. 需要新建版本表、diff 对比、回滚机制、定时发布、预览（读取 draft）
3. 第一版 Site Builder 主要是运营配置，误操作影响可控（可以改回来）
4. Admin 操作记录 audit，可追溯
5. Full V1 再评估 Draft/Publish（届时有更多运营场景需求）

### 26.3 风险说明

Immediate save 的风险：Admin 保存中间态（如只改了标题还没配图）会立即影响线上。缓解措施：
- 每个配置项有 enabled 开关，未完成的配置可以先 disabled
- Banner 有 start_at/end_at 定时，可以设置未来时间
- Admin 操作记录 audit，误操作可追溯并手动恢复
- 关键操作（如切换模板、删除所有入口）需要二次确认

---

## 二十七、数据模型

### 27.1 三种方案比较

| 方案 | 说明 | 适用场景 |
|---|---|---|
| A. 全部 Settings JSON | 所有装修配置存在 Settings KV（brand/template/home_entries/discovery 全 JSON） | 单例配置、非列表型 |
| B. 独立 tables | 所有装修配置建独立表（home_entries/discovery_blocks/banners 等） | 列表型、需 CRUD/排序/单块启停 |
| C. 混合方案 | brand/template → Settings KV；home_entries/discovery_blocks/banners → 独立表 | 各取所长 |

### 27.2 第一版推荐：混合方案 C

| 配置 | 存储 | 理由 |
|---|---|---|
| brand（site_name/logo/color/social/copyright） | Settings KV（扩展现有 `site` setting） | 单例配置，已有基础设施，多语言支持 |
| template（classic/vault） | Settings KV（复用现有 storefront_template） | 单例配置，已有 |
| home_entries | **独立表 `home_entries`** | 列表型，需 CRUD/排序/启停/角标，Admin 表单友好 |
| banners | **独立表 `banners`（复用现有）** | 已有完整模型+Admin CRUD+定时 |
| discovery_blocks | **独立表 `discovery_blocks`** | 列表型，需 CRUD/排序/启停/schema 驱动表单 |
| announcement | Settings KV（复用现有） | 单条，已有 |
| navigation | Settings KV（复用现有） | 已有 |
| footer_links | Settings KV（复用现有） | 已有 |
| business_toggles | Settings KV（新增 display 开关） | 单例配置 |
| site_audit_logs | **独立表** | 列表型，需查询/筛选 |

### 27.3 理由

1. **brand/template 是单例配置**：只有一份，不需要列表操作，Settings KV 更合适，且已有多语言支持和 normalize 机制
2. **home_entries/discovery_blocks 是列表型**：多条、有序、可单块启停/删除/排序，独立表更适合 Admin CRUD 和 schema 驱动表单
3. **banners 已有独立表**：完全复用，不需要迁移
4. **混合方案避免了 JSON editor**：Admin 不需要写 raw JSON，所有配置通过表单操作
5. **public config 聚合层统一输出**：无论数据存在 Settings 还是独立表，public config API 聚合后一次返回给 User 前端

---

## 二十八、测试规划

### 28.1 预审计必须规划的测试

| 测试类别 | 测试项 |
|---|---|
| bootstrap config | public config 返回完整装修数据（template/brand/homepage/discovery/social/footer） |
| template switch | Admin 切换 classic/vault 后 public config 更新，User 前端正确渲染 |
| brand color | Admin 保存 primary_color 后 public config 返回，前端注入 CSS variable；无效 HEX 被拒绝 |
| home entry reorder | Admin 修改 sort_order 后列表顺序正确；上移/下移 |
| hide/show | Admin disabled 某个 home_entry 后 User 首页不展示；enabled 后展示 |
| banner schedule | Banner start_at/end_at 定时生效；未到时间不展示；过期不展示 |
| discovery block render | 每种区块类型（banner/card_grid/business_recommend/announcement/external_link）正确渲染 |
| invalid external URL | javascript:/data:/ftp: URL 被后端拒绝；http/https 正常保存 |
| XSS content | 文本字段含 `<script>` 时前端转义，不执行；外部 URL 含 javascript: 被拒绝 |
| missing config fallback | DB 无配置时 public config 返回默认值；User 前端不报错 |
| cache invalidation | Admin 保存后 Redis 缓存失效；下次请求获取新配置 |
| RBAC | 无 site.* 权限的 Admin 不能访问装修 API；有对应权限可以 |
| audit | Admin 修改 brand/template/home_entries/discovery 后 audit 日志记录 before/after/admin/timestamp |
| IDOR | User 不能访问 Admin 装修 API；Admin 越权操作被拒绝 |
| home entry route whitelist | 不在白名单的 route 被拒绝；external URL 非 http/https 被拒绝 |
| discovery block schema | 无效 type 被拒绝；data 字段不匹配 type schema 被拒绝；Admin 不能写 raw JSON |

---

## 二十九、MVP

### 29.1 Phase 9 MVP

**至少包含**：

| 模块 | 内容 |
|---|---|
| template choice | classic/vault 选择（复用现有 storefront_template） |
| brand/logo/color | site_name/logo/favicon/primary_color/social_links/copyright/footer_text/ICP |
| homepage core entries | home_entries 独立表 + Admin CRUD + User 动态渲染（route whitelist + icon + title + subtitle + sort_order + enabled + badge + recommended） |
| banners | 复用现有 banners 表 + Admin CRUD + User 首页渲染（加强 link_value 安全校验） |
| announcements | 复用 Settings announcement + Admin 编辑 + User 首页渲染 |
| discovery blocks | discovery_blocks 独立表 + Admin schema 驱动表单 + User 按 type 渲染（6 种区块类型） |
| social/footer | social_links 扩展 + footer_links 复用 + copyright + User Footer 动态渲染 |
| Admin config UI | 站点装修模块（首页入口/Banner/公告/发现页/品牌/模板 6 个 Tab） |
| User dynamic rendering | Home/Discovery/Navbar/Footer/bootstrap 改为配置驱动 |
| cache | public config Redis 缓存 + Admin 保存主动失效 |
| RBAC | site.brand.manage / site.home.manage / site.discovery.manage / site.template.manage |
| audit | site_audit_logs 独立表 + Admin 操作记录 |
| fallback | 所有配置缺失时默认值 + 错误边界 + 不影响用户站打开 |
| 安全 | 外部 URL http/https only + 文本转义 + scripts 不暴露 + icon 白名单 |

### 29.2 Phase 9 Full V1（后续）

- drag/drop 可视化排序
- Draft/Publish（草稿/发布/回滚/定时发布）
- live preview（实时预览 + Desktop/Mobile 切换）
- per-template overrides（每个模板独立装修配置）
- scheduled campaigns（定时营销活动）
- advanced SEO（动态页面 SSR/预渲染、动态 og:image）
- scripts 安全沙箱化或禁用
- 图片尺寸自动裁剪/优化
- old file cleanup（覆盖上传时删除旧文件）
- 区块类型扩展（更多区块类型）
- A/B testing
- 访问统计/转化率分析

---

## 三十、最终明确回答

### 30.1 核心问题回答

| # | 问题 | 回答 |
|---|---|---|
| 1 | 当前已有多少装修能力可复用 | **高复用率**。Settings 已有 brand/contact/seo/legal/about/footer_links/template_mode/storefront_template(classic/vault)/navigation(builtin+custom_items)。Content 模块已有完整 Banner（多语言/定时/跳转/排序/Admin CRUD）。publicconfig 已有 Redis 缓存统一端点。Reseller 已有 announcement/navigation overlay。缺失：home_entries 配置、discovery_blocks、品牌色、社交链接扩展、Site Builder Admin UI |
| 2 | 首页四大业务入口如何后台配置 | **新建 `home_entries` 独立表**。Admin 可配置 title/subtitle/icon/route(白名单)/route_type/external_url/sort_order/enabled/badge/recommended。route 必须在后端白名单内（recharge/c2c/wallet/invitation/support/affiliate/discovery/orders），external URL 必须 http/https only。migration seed 默认 5 个入口。User 首页从 public config 读取动态渲染 |
| 3 | 发现页最适合什么数据模型 | **独立表 `discovery_blocks`**。结构化区块（type/title/data(JSON)/enabled/sort_order），6 种区块类型（banner/card_grid/business_recommend/announcement/external_link/category_entry）。独立表比 JSON config 更适合 Admin CRUD、排序、单块启停、schema 驱动表单。Admin 不写 raw JSON，data 由后端根据表单组装 |
| 4 | brand/template 如何存储 | **混合方案**。brand/template → Settings KV（复用现有 `site` setting，扩展 primary_color/social_links/copyright/footer_text/ICP）。home_entries/discovery_blocks/banners → 独立表。public config 聚合层统一输出给 User 前端 |
| 5 | classic/vault 是否能共享同一套装修数据 | **是**。装修数据（home_entries/banners/discovery/brand/social）与 template 解耦。同一套配置 classic 用 classic layout 渲染，vault 用 vault layout 渲染。Admin 不为每个模板维护两份业务配置。API/types/composable/store 共用，classic/vault 只负责视图层 |
| 6 | User bootstrap API 怎么设计 | **扩展现有 public config 端点**，不新建独立 API。输出增加 template/brand/homepage(entries+banners+announcement)/discovery(blocks)/social_links/footer(copyright+text+ICP+links)/business_toggles/navigation。User 首页一次调用拿到全部装修数据，不打 10 个配置 API。Redis 缓存，Admin 保存主动失效 |
| 7 | 是否需要 Draft/Publish | **第一版不需要，immediate save**。理由：现有 Settings 全部 immediate save，Draft/Publish 复杂度高（版本表/diff/回滚/定时/预览）。第一版接受保存即生效，通过 enabled 开关、Banner 定时、audit 日志、关键操作二次确认缓解风险。Full V1 再评估 |
| 8 | 哪些内容仍然写死在 User 前端 | 首页核心业务入口（图标/文案/路由）、发现页全部内容、品牌色 CSS、部分 Navbar 菜单、Footer 文案/版权/社交链接。需改为动态配置驱动。C2C/Ticket/Wallet/Recharge 等业务页面内部不重构 |
| 9 | Admin 需要新增哪些页面 | 站点装修模块（6 个 Tab）：首页入口管理（home_entries CRUD+排序+启停）、Banner 管理（复用现有）、公告编辑、发现页装修（discovery_blocks CRUD+排序+schema 表单）、品牌设置（logo/色/社交/版权）、模板设置（classic/vault） |
| 10 | User 需要修改哪些页面 | Home（入口+Banner+公告动态化）、Discovery（区块动态渲染）、Navbar（品牌色+logo+导航配置）、Footer（版权+社交+链接动态化）、bootstrap/App.vue（注入 CSS variable+模板选择）、全局样式（Tailwind 引用 CSS variable+替换写死主色）。不重构 C2C/Ticket/Wallet/Recharge 业务页面 |
| 11 | 最大安全风险是什么 | **现有 Settings `scripts` 字段允许 Admin 注入自定义 JS（20 条×20000 字符）——存储型 XSS 高危**。Phase 9 Site Builder 不得暴露 scripts 编辑，且应评估整体禁用。其次是外部 URL 校验（navigation/Banner/home_entry 外链必须 http/https only，禁止 javascript:/data:）。文本字段纯文本渲染转义，不允许 raw HTML/Markdown。图标使用前端白名单映射，不允许上传 SVG |
| 12 | Phase 9 MVP 最小范围 | template choice + brand/logo/color + homepage core entries(home_entries 表) + banners(复用) + announcements(复用) + discovery blocks(discovery_blocks 表+6种类型) + social/footer + Admin config UI(6 Tab) + User dynamic rendering(Home/Discovery/Navbar/Footer/bootstrap) + cache(Redis+主动失效) + RBAC(site.* 4权限) + audit(site_audit_logs) + fallback(默认值+错误边界) + 安全(URL白名单+XSS防护) |
| 13 | 是否可以进入正式实施 | **可以**。基础设施复用率高（Settings/Banner/publicconfig/Redis cache/FileUploader），缺失部分边界清晰（home_entries/discovery_blocks/品牌色/Site Builder UI），安全风险已识别（scripts 字段），MVP 范围明确，数据模型（混合方案）已定 |

### 30.2 实施建议

- 新建 `internal/modules/sitebuilder/`（或扩展 settings 模块 + 新建 home_entries/discovery_blocks store）
- 分阶段：①Foundation（home_entries/discovery_blocks 表+migration+store+brand 字段扩展+public config 聚合+缓存失效）②Admin API（home_entries CRUD/discovery_blocks CRUD/brand update/template switch+RBAC+audit）③User 前端（Home/Discovery/Navbar/Footer/bootstrap 动态化+品牌色注入）④Admin 前端（站点装修 6 Tab UI）⑤测试+回归+CI
- 资金红线：Site Builder 不涉及任何资金操作，不需要 Payment Compliance
- scripts 字段：第一版不碰，但必须在 Admin UI 中隐藏，且在最终报告中标记为高危待处理

---

*预审计完成。现有基础设施复用率高，缺失部分边界清晰，安全风险已识别，MVP 范围明确，可以进入正式实施。*
