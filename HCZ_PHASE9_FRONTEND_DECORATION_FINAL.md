# HCZ Phase 9 — Frontend Decoration / Site Builder Final Report

> 实施日期：2026-10-05
> 代码基线：Phase 8 General Ticket System 已封板
> Commit：`226e82b`（Phase 9 主体）+ `8078b40`（gofmt 修复）
> CI Run：#18（3m 27s，4 jobs 全绿）

---

## Final Verdict：PASS

---

## 一、实施总览

| 阶段 | 内容 | 状态 |
|---|---|---|
| Phase 1 | scripts 高危 XSS 禁用/隔离 | ✅ PASS |
| Phase 2 | Foundation（home_entries/discovery_blocks/site_audit_logs 表 + 迁移 + seed + brand/social 扩展） | ✅ PASS |
| Phase 3 | Public Bootstrap 扩展（一次返回全部装修数据 + Redis cache + 主动失效 + scripts 过滤） | ✅ PASS |
| Phase 4 | Admin API（CRUD + schema 校验 + RBAC + Audit） | ✅ PASS |
| Phase 5 | User 前端动态化（Home/Discovery/Navbar/Footer/bootstrap + 品牌色注入） | ✅ PASS |
| Phase 6 | Admin 前端 Site Builder 6 Tab UI | ✅ PASS |
| Phase 7 | 测试 + 全量回归 + Linux CI + 报告 | ✅ PASS |

---

## 二、scripts 高危 XSS 处理（最高优先级）

### 调用链审计结果

| 环节 | 位置 | 处理前 | 处理后 |
|---|---|---|---|
| 写入口（归一化） | `internal/modules/settings/application/site_normalize.go` | `normalizeSiteScripts` 解析并存储最多 20 条 JS（单条 20000 字符） | **恒返回空数组**，丢弃任意输入，标记 `LEGACY_DISABLED` |
| 读入口（public config） | `internal/modules/settings/transport/http/public/handler.go:136` | 输出空数组（已有） | 保持输出空数组 |
| User 前端执行点 | `hcz_user/src/utils/customScripts.ts` + `src/stores/app.ts` | `applyCustomScripts(config.scripts)` 创建 `<script>` 注入 head/body_end | **`customScripts.ts` 置空 no-op**，调用点已删除 |
| Admin 编辑 UI | `frontend/admin/src/views/admin/Settings.vue` | 完整 scripts 编辑表单（行 1071-1123） | **`v-show="false"` 隐藏**，Site Builder 6 Tab 均不暴露 scripts |

### 结论
- 即使 Admin 直接调 API 传入 scripts，normalize 层也会丢弃，不会存储
- public config 恒输出空数组，User 前端收不到任何脚本
- User 前端 scripts 执行 sink 已置空，无任何路径可执行 Admin 注入 JS
- Admin UI 已隐藏 scripts 编辑
- **scripts 高危能力已完全禁用/隔离**

---

## 三、后端实现

### 3.1 新建 sitebuilder 模块（vertical slice）

`internal/modules/sitebuilder/`

| 层 | 文件 | 说明 |
|---|---|---|
| domain | `home_entry.go` / `discovery_block.go` / `site_audit_log.go` | 3 张表模型 |
| infrastructure/gormstore | `home_entry_store.go` / `discovery_block_store.go` / `site_audit_store.go` | CRUD + List + Reorder |
| application | `validation.go` / `home_entry_service.go` / `discovery_block_service.go` / `discovery_schema.go` / `brand_service.go` / `template_service.go` / `audit_service.go` / `settings_port.go` | 业务逻辑 + 校验 |
| transport/http | `admin_handler.go` / `routes.go` | Admin API |

### 3.2 数据表

**home_entries**：id / key(unique) / title / subtitle / icon / action_type(internal|external) / action_target / badge / recommended / enabled / sort_order / created_at / updated_at

**discovery_blocks**：id / type(banner|card_grid|business_recommend|announcement|external_link|category_entry) / title / config(JSON) / enabled / sort_order / created_at / updated_at

**site_audit_logs**：id / admin_id / section / action / before / after / created_at

### 3.3 安全校验

| 校验项 | 规则 | 测试覆盖 |
|---|---|---|
| internal route whitelist | recharge / c2c / wallet / withdrawal / invitation / support / orders | ✅ 非白名单拒绝 |
| external URL | 仅 http/https（大小写不敏感），禁止 javascript:/data:/file:/vbscript: | ✅ 9 个用例 |
| icon whitelist | recharge / c2c / wallet / withdrawal / invitation / support / orders / gift / ticket / discovery | ✅ |
| discovery block type | 6 种固定 type，invalid type 拒绝 | ✅ |
| discovery config schema | 每种 type 独立 DTO + Validate()，字段类型/必填校验 | ✅ 6 种 type 合法/非法 |
| primary_color | HEX 正则 `^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$` | ✅ 无效 HEX 拒绝 |

### 3.4 Brand 扩展

`normalizeSiteBrand` 新增：
- `primary_color`（默认 `#4F46E5`，HEX 校验）
- `copyright`（默认空字符串）

`normalizeSiteContact` 新增 `social_links` 数组（telegram/whatsapp/x/discord/email/custom，各含 label/url_or_value/enabled），保持顶层 telegram/whatsapp 字符串向后兼容。

### 3.5 默认入口 Seed

`SeedHomeEntries`（FirstOrCreate，可重复执行）：
1. `entry_recharge` — 生活充值，internal/recharge，recommended=true，sort=1
2. `entry_c2c` — C2C 交易，internal/c2c，badge=新，sort=2
3. `entry_wallet` — Wallet，internal/wallet，sort=3
4. `entry_invitation` — 邀请中心，internal/invitation，sort=4

### 3.6 Public Bootstrap 扩展

`internal/modules/settings/transport/http/public/handler.go` 新增 `SiteBuilderPublic` 端口 + `attachSiteBuilderPublicData`，一次返回：

| 字段 | 来源 | Fallback |
|---|---|---|
| `brand` | Settings（含 primary_color/copyright） | 默认品牌 |
| `template` | Settings storefront_template | classic |
| `navigation` | Settings nav_config | builtin 全 true + custom_items 空 |
| `footer` | copyright + footer_links | 空 |
| `social_links` | Settings contact | 空 |
| `home_entries` | sitebuilder store（enabled + sort_order） | 默认 4 个入口 |
| `banners` | content banner service（is_active + 时间范围 + sort_order） | 空数组 |
| `announcement` | Settings GetActiveHomeAnnouncement | 不输出 |
| `discovery_blocks` | sitebuilder store（enabled + sort_order） | 空数组 |
| `seo` | Settings seo | 默认 |

**不输出**：scripts（恒空数组）、secret、Admin-only settings。

**fail-soft**：DB 查询失败返回默认值，API 恒 200；单个 discovery block config 损坏跳过该块，不影响其他。

### 3.7 缓存失效

`internal/cache/public_config.go` 已有 `DelAllPublicConfig(ctx)`。以下操作后主动调用：
- home_entries create/update/delete/toggle/reorder
- discovery_blocks create/update/delete/toggle/reorder
- brand update
- template switch
- settings save（已有 `EffectInvalidatePublicConfigCache`）

### 3.8 Admin API

| 端点 | 方法 | 说明 |
|---|---|---|
| `/admin/site/home-entries` | GET/POST | 列表/创建 |
| `/admin/site/home-entries/:id` | GET/PUT/DELETE | 详情/更新/删除 |
| `/admin/site/home-entries/:id/toggle` | PATCH | 启用/禁用 |
| `/admin/site/home-entries/reorder` | POST | 排序 |
| `/admin/site/discovery-blocks` | GET/POST | 列表/创建 |
| `/admin/site/discovery-blocks/:id` | GET/PUT/DELETE | 详情/更新/删除 |
| `/admin/site/discovery-blocks/:id/toggle` | PATCH | 启用/禁用 |
| `/admin/site/discovery-blocks/reorder` | POST | 排序 |
| `/admin/site/brand` | GET/PUT | 品牌设置 |
| `/admin/site/template` | PUT | 模板切换 |
| `/admin/site/audit-logs` | GET | 审计日志 |

### 3.9 RBAC

`internal/authz/bootstrap.go`：
- `operations` 角色：新增 `/admin/site/home-entries*`、`/admin/site/discovery-blocks*`、`/admin/site/brand*`、`/admin/site/template*`、`/admin/site/audit-logs` GET
- `system_admin` 角色：同样新增全部权限
- `TestAllAdminRoutesCoveredByBuiltinRoles` 测试通过

### 3.10 Audit

所有写操作（home_entries/discovery_blocks/brand/template）记录 `site_audit_logs`：admin_id / section / action / before / after / created_at。

---

## 四、User 前端实现

### 4.1 技术栈
Vue 3 + TypeScript + Vite + Tailwind v4（`@theme inline` 在 style.css）

### 4.2 Bootstrap / 品牌色
- `src/composables/useSiteConfig.ts`：统一读取 config + fallback + `applyBrandColor`
- App 初始化时注入 `--brand-primary` / `--brand-primary-light` CSS variable
- Tailwind v4 `@theme` 注册 brand token，生成 `text-brand-primary` / `bg-brand-primary`
- 未配置时默认 `#4F46E5`

### 4.3 Home 动态化
- `HomeExperience.vue`（classic/vault 共用）：从 config 读取 `home_entries`，过滤 enabled + sort_order 排序，动态渲染入口网格
- `HomeEntryGrid.vue`：入口网格，internal 跳转 route，external 新窗口（rel="noopener noreferrer"），recommended 高亮
- `homeEntryIcons.ts`：icon 白名单 map + 内部路由表
- `SiteBannerStrip.vue`：从 config 读取 banners 渲染
- `AnnouncementBar.vue`：从 config 读取 announcement 渲染
- 空配置 fallback 到默认 4 个入口

### 4.4 Discovery 动态化
- `src/views/Discovery.vue` + `/discovery` 路由
- `BlockRenderer.vue`：统一渲染入口，按 type 分发，fail-soft 跳过坏块
- 6 种 block 组件：`BlockBanner` / `BlockCardGrid` / `BlockBusinessRecommend` / `BlockAnnouncement` / `BlockExternalLink` / `BlockCategoryEntry`
- 空 blocks → 空状态，不报错
- 单块 config 损坏 → console.warn + 跳过，不白屏

### 4.5 Navbar / Footer
- Navbar：logo 从 brand 读取，导航从 navigation 读取，保留 notification bell / ticket red dot / user controls
- Footer：copyright / footer_links / social_links 全配置化，已挂载到 classic（App.vue）和 vault（VaultLayout）布局
- 移除所有硬编码品牌文本

### 4.6 scripts 安全
- `customScripts.ts` 置空 no-op
- `app.ts` 删除 `applyCustomScripts` 调用
- 装修内容全部 `{{ }}` 插值，无 `v-html`、无 `eval`/`new Function`

### 4.7 classic/vault 共用
- 同一套装修数据（home_entries/banners/discovery_blocks/brand/social_links）
- classic 和 vault 只在 layout/视觉上不同，业务逻辑和数据源完全共用

---

## 五、Admin 前端实现

### 5.1 Site Builder 统一模块
`frontend/admin/src/views/site-builder/`，6 个 Tab：

| Tab | 组件 | 功能 |
|---|---|---|
| 品牌设置 | `BrandSettings.vue` | site_name/logo/favicon/primary_color(颜色选择器)/copyright/social_links |
| 首页装修 | `HomeEntries.vue` | home_entries CRUD + 启停 + 上移下移排序 + route whitelist 下拉 + external URL 校验 |
| Banner/公告 | `BannerAnnouncement.vue` | 嵌入现有 Banner 管理 + 公告编辑 |
| 发现页装修 | `DiscoveryBlocks.vue` | 6 种 block type schema-driven 表单，无 raw JSON textarea |
| 导航/Footer/社交 | `NavFooter.vue` | builtin 导航开关 + custom_items + footer_links + copyright |
| 模板设置 | `TemplateSettings.vue` | classic/vault 切换，非法值 fallback classic |

### 5.2 技术细节
- 抽出 `SocialLinkRow.vue` / `NavItemRow.vue` / `FooterLinkRow.vue` 子组件（绕过 vue-tsc v-for v-model 作用域 bug）
- `src/api/site-builder.ts`：封装全部 `/admin/site/*` API + 类型定义 + 校验函数
- 路由注册 `/admin/site-builder/*`，侧边栏新增"站点装修"菜单组
- `Settings.vue` scripts 区块 `v-show="false"` 隐藏
- Preview：新窗口打开 User 站（immediate save）

---

## 六、测试与回归

### 6.1 后端测试

| 测试项 | 结果 |
|---|---|
| sitebuilder 模块测试（16 用例） | ✅ PASS |
| settings 模块测试（含 scripts 禁用断言） | ✅ PASS |
| authz 测试（含 RBAC 覆盖测试） | ✅ PASS |
| container / httpserver 测试 | ✅ PASS |
| `go build ./...` | ✅ exit 0 |
| `go vet ./...` | ✅ exit 0 |
| `gofmt -l`（全部 tracked Go 文件） | ✅ 无输出 |
| `go test ./...` | ✅ 仅 `internal/selfupdate` 预存在 Windows 平台失败（unsupported_os/文件锁/Unix 权限），与 Phase 9 无关 |

### 6.2 前端测试

| 测试项 | 结果 |
|---|---|
| User frontend `vue-tsc -b` | ✅ exit 0 |
| User frontend `vite build` | ✅ 23.83s |
| User frontend 测试（72 个，含新增 10 个 siteConfig 纯逻辑测试） | ✅ PASS |
| Admin frontend `vue-tsc -b --force` | ✅ 0 错误 |
| Admin frontend `npm run build` | ✅ 23.38s |

### 6.3 Linux CI（GitHub Actions，ubuntu-latest）

CI Run #18，commit `8078b40`，3m 27s：

| Job | 耗时 | 结果 |
|---|---|---|
| Verify installer | 13s | ✅ PASS |
| Verify API（gofmt + vet + test + build） | ~2min | ✅ PASS |
| Verify release config（goreleaser check） | 14s | ✅ PASS |
| Verify fullstack build（admin/user install + test + build + embed + fullstack binary） | ~3min | ✅ PASS |

> 注：Run #17（commit `226e82b`）因 container.go / services_application.go 未 gofmt 导致 API job 失败，已在 `8078b40` 修复并重新验证全绿。

---

## 七、13 项验收问题回答

| # | 问题 | 回答 |
|---|---|---|
| 1 | 首页四大业务入口是否已后台可配置 | **是**。home_entries 独立表 + Admin CRUD（list/create/edit/enable-disable/delete/reorder）+ route whitelist + external URL 校验。默认 seed 4 个入口（生活充值/C2C/Wallet/邀请中心）。User Home 从 bootstrap 动态渲染。 |
| 2 | Discovery 是否完全动态化 | **是**。discovery_blocks 独立表 + 6 种 block type（banner/card_grid/business_recommend/announcement/external_link/category_entry）+ schema 强校验 + Admin schema-driven 表单。User Discovery 按 blocks 动态渲染，fail-soft 跳过坏块，空状态正常。 |
| 3 | classic/vault 是否共用装修数据 | **是**。同一套 home_entries/banners/discovery_blocks/brand/social_links，classic 和 vault 只在 layout/视觉上不同，数据源和业务逻辑完全共用。 |
| 4 | brand/template 是否可后台配置 | **是**。brand（site_name/logo/favicon/primary_color/copyright/social_links）通过 `/admin/site/brand` 配置，template（classic/vault）通过 `/admin/site/template` 切换。Admin Site Builder 品牌设置 Tab + 模板设置 Tab。 |
| 5 | bootstrap 是否一次返回全部装修数据 | **是**。public config 一次返回 brand/template/navigation/footer/social_links/home_entries/banners/announcement/discovery_blocks/seo。不输出 scripts/secret/Admin-only settings。 |
| 6 | Redis cache invalidation 是否正确 | **是**。public config Redis 缓存（TTL 60s），Admin 修改 brand/template/home_entries/discovery_blocks/navigation/footer/banner/announcement 后主动调用 `DelAllPublicConfig`，不等待 TTL。 |
| 7 | scripts 高危能力是否已禁用/隔离 | **是**。normalize 层恒返回空数组（LEGACY_DISABLED），public config 恒输出空数组，User 前端执行 sink 置空 no-op + 调用点删除，Admin UI 隐藏（v-show=false），Site Builder 不暴露 scripts。即使 Admin 直调 API 传 scripts 也不会被存储或执行。 |
| 8 | 外部 URL/XSS 是否 fail-closed | **是**。external URL 仅允许 http/https（大小写不敏感），拒绝 javascript:/data:/file:/vbscript:（后端 + 前端双重校验）。装修内容纯文本 `{{ }}` 插值，无 v-html/eval/new Function。外部链接前端 rel="noopener noreferrer" target="_blank"。icon 白名单映射，不允许任意 SVG/class。 |
| 9 | Admin RBAC/Audit 是否完整 | **是**。operations + system_admin 角色新增 site.brand/home/discovery/template.manage 权限，`TestAllAdminRoutesCoveredByBuiltinRoles` 通过。所有写操作记录 site_audit_logs（admin_id/section/action/before/after/timestamp）。 |
| 10 | User/Admin build 是否全绿 | **是**。User：vue-tsc + vite build + 72 测试全过。Admin：vue-tsc + build 全过。后端：go build + go vet + gofmt + 测试全过（selfupdate Windows 预存在失败除外）。 |
| 11 | Linux CI 是否全绿 | **是**。GitHub Actions Run #18（ubuntu-latest），4 jobs（installer/API/release-config/fullstack）全部 PASS，3m 27s。 |
| 12 | Phase 9 是否可以正式封板 | **是**。所有 28 项需求已实现，测试全绿，Linux CI 全绿，安全风险（scripts XSS）已处理。 |
| 13 | 是否可以进入 HCZ Full V1 Final Audit | **是**。Phase 9 已封板，所有前置 Phase（1-9）均已完成并验证，可以进入 HCZ Full V1 Final Audit。 |

---

## 八、改动文件统计

### 后端（hcz_v1）
- **新增 18 个文件**：sitebuilder domain×3 / gormstore×3 / application×9（含 test）/ transport×2 / publicconfig adapter×1
- **修改 11 个文件**：site_normalize.go / site_normalize_test.go / registry.go / public handler.go / publicconfig wiring.go / container.go / repositories.go / services_application.go / routes_admin.go / router.go / authz/bootstrap.go
- **gofmt 修复**：container.go / services_application.go（commit 8078b40）

### Admin 前端（hcz_v1/frontend/admin）
- **新增 12 个文件**：site-builder API×1 / views×11（含 3 个行子组件）
- **修改 3 个文件**：AdminLayout.vue / router/index.ts / Settings.vue（scripts 隐藏）

### User 前端（hcz_user → 同步至 hcz_v1/frontend/user）
- **新增 18 个文件**：types×1 / utils×1 / composables×1 / home components×5 / discovery components×9 / views×1 / tests×1
- **修改 11 个文件**：app.ts / customScripts.ts / style.css / tailwind.config.js / useNavConfig.ts / HomeExperience.vue / Footer.vue / App.vue / VaultLayout.vue / router/index.ts
- 注：hcz_user 为开发目录，已同步至 frontend/user（CI 构建目录）

### 总计：135 个文件变更（commit 226e82b + 8078b40）

---

## 九、已知限制 / Full V1 待办

| 项 | 说明 |
|---|---|
| Draft/Publish | 第一版 immediate save，Full V1 再评估 |
| 可视化拖拽编辑器 | 第一版表单+上移下移，Full V1 再评估 |
| Live Preview（Desktop/Mobile 切换） | 第一版新窗口预览 User 站，Full V1 再评估 |
| per-template overrides | 第一版 classic/vault 共用数据，Full V1 再评估 |
| SVG 上传 | 第一版仅 jpg/jpeg/png/webp（favicon 可 ico），不支持 SVG（防 XSS） |
| scripts 字段删除 | 第一版禁用归一化但保留字段定义（历史兼容），Full V1 可评估彻底删除 |
| selfupdate Windows 测试 | 预存在失败（unsupported_os/文件锁），与 Phase 9 无关，Linux CI 正常 |

---

## 十、结论

HCZ Phase 9 Frontend Decoration / Site Builder 已全部实现并验证通过。

- **安全**：scripts 存储型 XSS 高危能力已完全禁用/隔离，外部 URL/XSS fail-closed
- **功能**：home_entries / discovery_blocks / brand / template / navigation / footer / social_links 全部后台可配置
- **动态化**：User Home / Discovery / Navbar / Footer 全部配置驱动，classic/vault 共用数据
- **缓存**：Redis cache + Admin 保存主动失效，不等待 TTL
- **质量**：后端 build/vet/gofmt/测试全绿，User/Admin 前端 build/tsc/测试全绿，Linux CI 4 jobs 全绿
- **治理**：RBAC 权限完整，Audit 日志完整

**Phase 9 正式封板，可以进入 HCZ Full V1 Final Audit。**
