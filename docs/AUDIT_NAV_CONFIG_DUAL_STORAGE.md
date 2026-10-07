# HCZ Navigation 双存储审计报告

> 审计日期：2026-10-06
> 范围：`nav_config`（全局 settings key）与 `site_config.nav_config`（reseller_site_configs.nav_config_json）双存储定位、读写调用图、Admin 页面重复度、用户端实际消费源、迁移建议。
> 性质：只读审计，未修改任何代码。

---

## 0. 结论速览（TL;DR）

代码里实际存在 **三套** 与导航相关的存储，不是两套：

| # | 存储位置 | 写入入口 | 前端编辑页面 | 是否对用户端生效 |
|---|---|---|---|---|
| **A** | `settings` 表，`key='nav_config'`，`value_json` 为 JSON | `PUT /admin/settings`（generic） | `SettingsNavigationTab.vue`（设置 → 导航） | ✅ **生效**（主站默认源） |
| **B** | `reseller_site_configs.nav_config_json`（JSON column，按 reseller_id 分行） | `PUT /admin/resellers/site-configs/:id`、`PUT /api/v1/user/reseller/site-config` | `ResellerSiteConfigs.vue`（管理员）、`ResellerSiteConfigPanel.vue`（分销自助） | ✅ **生效**（分销租户覆盖源） |
| **C** | `settings` 表，`key='site_config'`，value 内嵌 `nav_config` 字段 | `PUT /admin/settings`（key=site_config，整体回写） | `NavFooter.vue`（站点装修 → 导航/Footer） | ❌ **死存储**（写了但后端从不读） |

**关键 Bug**：站点装修页面 `NavFooter.vue` 对导航的编辑（Storage C）会被后端 `public/handler.go:228-236` 无条件覆盖，用户在该页面改导航开关/自定义项**保存后前台完全不生效**。只有 Footer 链接部分（`footer_links`）是活的。

---

## 1. Storage A —— 全局 `nav_config`（settings 表）

### 1.1 定义
- 常量：`internal/constants/constants.go:554`
  ```go
  SettingKeyNavConfig = "nav_config"
  ```
- 注册表：`internal/modules/settings/application/default_registry.go:60-63`
  ```go
  Definition{
      Key:       constants.SettingKeyNavConfig,
      Normalize: func(value jsonmap.JSON) jsonmap.JSON { return normalizeNavConfig(value) },
      Effects:   []Effect{EffectInvalidatePublicConfigCache},
  }
  ```
- 归一化函数：`internal/modules/settings/application/site_normalize.go:335 normalizeNavConfig()`
  - 结构：`{ builtin: {blog, notice, about: bool}, custom_items: [{id, title:{zh-CN,zh-TW,en-US}, link_type, url, target, sort_order, enabled, icon}] }`
  - 默认 builtin 三项均 `true`；custom_items 上限 10 条；标题截断 120 rune；URL 截断 2000 rune。

### 1.2 存储方式
- DB 表：`settings`（`internal/modules/settings/infrastructure/gormstore/store.go:11-19`）
  - `Key string gorm:"primarykey"`
  - `ValueJSON jsonmap.JSON gorm:"type:json"`
  - `TableName() => "settings"`
- 持久化：`Store.Upsert(key, value)`（store.go:45-64），存在则 `Save`，不存在则 `Create`。
- 无 Redis 直写；读路径上有 public config 缓存（`EffectInvalidatePublicConfigCache` 触发 `DelAllPublicConfig`）。

### 1.3 读入口
| 文件 | 函数 | 行号 | 说明 |
|---|---|---|---|
| `internal/modules/settings/transport/http/public/handler.go` | `Handler.GetConfig` | 228-236 | `navConfigVal, _ := h.settings.GetByKey(constants.SettingKeyNavConfig)` → 写入 `data["nav_config"]`；nil 时回落到硬编码默认 `{builtin:{blog:true,notice:true,about:true}, custom_items:[]}` |

> 注：这是全局唯一读取 `settings.nav_config` key 的业务代码点。

### 1.4 写入口
| 文件 | 函数 | 行号 | 说明 |
|---|---|---|---|
| `internal/modules/settings/transport/http/admin_handler.go` | `AdminHandler.Update` | 58-87 | 通用 `PUT /admin/settings`，body `{key:"nav_config", value:{...}}` → `h.settings.UpdateWithEffects(key, value)` → 走 registry normalize → `Store.Upsert` |
| 路由 | `internal/modules/settings/transport/http/routes.go` | 5-8 | `admin.GET("/settings", ...)` / `admin.PUT("/settings", ...)` |

> 没有专门的 nav_config update handler，复用通用 settings 写接口。

### 1.5 前端调用方
| 文件 | 调用 | 行号 |
|---|---|---|
| `frontend/admin/src/views/admin/components/SettingsNavigationTab.vue` | `adminAPI.getSettings({ key: 'nav_config' })` | 88 |
| 同上 | `adminAPI.updateSettings({ key: 'nav_config', value: {...} })` | 126-141 |

---

## 2. Storage B —— reseller `site_config.nav_config`（reseller_site_configs 表）

### 2.1 定义
- Domain 模型：`internal/modules/reseller/domain/site.go:30-49`
  ```go
  type SiteConfig struct {
      ID               uint         `gorm:"primarykey"`
      ResellerID       uint         `gorm:"not null;index"`
      ...
      NavConfigJSON    jsonmap.JSON `gorm:"type:json" json:"nav_config_json"`
      ...
  }
  func (SiteConfig) TableName() string { return "reseller_site_configs" }
  ```
- Application 输入结构：`internal/modules/reseller/application/site_config.go:52-66`
  ```go
  type ResellerNavConfigInput struct {
      Builtin     map[string]bool           `json:"builtin"`
      CustomItems []ResellerFooterLinkInput `json:"custom_items"`
  }
  type ResellerSiteConfigInput struct {
      ...
      NavConfig    ResellerNavConfigInput    `json:"nav_config"`
  }
  ```
- 归一化：`internal/modules/reseller/application/site_config.go:225 normalizeResellerNavConfig()`

### 2.2 存储方式
- DB 表：`reseller_site_configs`，JSON column `nav_config_json`，按 `reseller_id` 分行（软删除 `deleted_at`）。
- 持久化：`internal/modules/reseller/infrastructure/gormstore/site_config.go:16 UpsertSiteConfig()`，line 41 `existing.NavConfigJSON = input.NavConfigJSON`。

### 2.3 读入口
| 文件 | 函数 | 行号 | 说明 |
|---|---|---|---|
| `internal/modules/reseller/application/site_config.go` | `SiteConfigService.ApplyPublicConfigOverlay` | 358-399 | 对 reseller 租户，line 397 `applyResellerSiteConfigToPublicConfig(out, cfg)` |
| 同上 | `applyResellerSiteConfigToPublicConfig` | 511-513 | `if len(cfg.NavConfigJSON) > 0 { out["nav_config"] = cfg.NavConfigJSON }` —— **整体覆盖** base 的 nav_config |
| 装配 | `internal/bootstrap/publicconfig/adapters.go:143` | — | overlay adapter 桥接 |
| 调用点 | `internal/modules/settings/transport/http/public/handler.go:247` | — | `overlaid, overlayErr := h.overlay.ApplyPublicConfigOverlay(...)` |

### 2.4 写入口
| 文件 | 函数 | 行号 | 路由 |
|---|---|---|---|
| `internal/modules/reseller/transport/http/admin/admin_site_config_handler.go` | `AdminSiteConfigHandler.UpdateSiteConfig` | 115-142 | `PUT /admin/resellers/site-configs/:reseller_id` |
| 同上 | `AdminSiteConfigHandler.ResetSiteConfig` | 145-160 | `POST /admin/resellers/site-configs/:reseller_id/reset`（软删整行，导航回落到 Storage A） |
| `internal/modules/reseller/transport/http/user/user_handler.go` | `UserHandler.UpdateSiteConfig` | 165-186 | `PUT /api/v1/user/reseller/site-config` |
| 路由 | `internal/modules/reseller/transport/http/admin/routes.go:33-36` | — | admin 路由注册 |
| 路由 | `internal/modules/reseller/transport/http/user/routes.go:15-16` | — | user 路由注册 |

### 2.5 前端调用方
| 文件 | 调用 | 行号 |
|---|---|---|
| `frontend/admin/src/views/admin/ResellerSiteConfigs.vue` | admin 端编辑单个 reseller 的 site-config（含 nav_config.builtin 开关） | 69-71, 141-143, 198, 230-233 |
| `frontend/user/src/components/reseller/ResellerSiteConfigPanel.vue` | 分销自助编辑面板，navigation tab | 273-328, 408, 506-509, 568-571 |

---

## 3. Storage C（死存储）—— `settings.site_config.nav_config`（嵌套）

### 3.1 定义
- `SettingKeySiteConfig = "site_config"`（constants.go:538），与 `SettingKeyNavConfig` 是**两个独立的 settings key**。
- `normalizeSiteSetting`（site_normalize.go:33-56）会把输入里所有 key 原样拷贝进 normalized（line 35-37 `for key, raw := range value { normalized[key] = raw }`），其中包括 `nav_config` 嵌套字段——但**不做任何归一化**（不像独立 `nav_config` key 那样走 `normalizeNavConfig`）。

### 3.2 存储方式
- 同一行 `settings` 表，`key='site_config'`，`value_json.nav_config` 是裸 JSON，未归一化。

### 3.3 读入口
- **后端没有任何业务代码读取 `site_config.nav_config`。**
- `GetConfig(defaults)`（general.go:82-103）确实会把 `site_config` 的所有顶层 key merge 进 `data`（line 99-101 `for k, v := range value { data[k] = v }`），所以 `data["nav_config"]` 会被瞬时赋值为嵌套的 nav_config。
- 但紧接着 public/handler.go:228-236 **无条件覆盖** `data["nav_config"]`：
  ```go
  navConfigVal, _ := h.settings.GetByKey(constants.SettingKeyNavConfig)
  if navConfigVal != nil {
      data["nav_config"] = navConfigVal          // Storage A 覆盖
  } else {
      data["nav_config"] = map[string]interface{}{
          "builtin":      map[string]interface{}{"blog": true, "notice": true, "about": true},
          "custom_items": make([]interface{}, 0),
      }                                          // 硬编码默认值覆盖
  }
  ```
  两种分支都会把刚 merge 进来的 `site_config.nav_config` 冲掉。

### 3.4 写入口
| 文件 | 函数 | 行号 | 说明 |
|---|---|---|---|
| `frontend/admin/src/views/site-builder/NavFooter.vue` | `save()` | 91-131 | 先 `GET site_config`，再 `payload.nav_config = {...}; payload.footer_links = {...}; PUT /admin/settings {key:'site_config', value: payload}` |
| 后端 | `AdminHandler.Update` | admin_handler.go:58 | 通用写接口，key=site_config → `normalizeSiteSetting` → `Upsert` |

### 3.5 前端调用方
- 仅 `frontend/admin/src/views/site-builder/NavFooter.vue`。
- 该页面同时编辑「顶部导航」和「Footer 链接」两块。其中：
  - **Footer 链接**（`footer_links`）：是活的——`normalizeSiteSetting` 会归一化它（site_normalize.go:47），`GetConfig` merge 进 `data["footer_links"]`，前端 `useSiteConfig.footerLinks`（useSiteConfig.ts:71-81）读取它，reseller overlay 也只在 `len(FooterLinksJSON)>0` 时覆盖。
  - **顶部导航**（`nav_config` 嵌套）：死的——见 3.3。

---

## 4. READ/WRITE CALL GRAPH（合并视图）

### 4.1 主站（main tenant，ResellerID == nil）
```
[写]
  Admin UI: SettingsNavigationTab.vue
    → PUT /admin/settings {key:"nav_config", value:...}
    → AdminHandler.Update
    → settingsapp.UpdateWithEffects
    → normalizeNavConfig
    → settingsstore.Store.Upsert  →  settings 表 (key=nav_config)   [Storage A]

  Admin UI: NavFooter.vue
    → GET /admin/settings?key=site_config
    → PUT /admin/settings {key:"site_config", value:{...nav_config, footer_links}}
    → normalizeSiteSetting (nav_config 原样拷贝, footer_links 归一化)
    → settingsstore.Store.Upsert  →  settings 表 (key=site_config)   [Storage C, nav_config 部分死]

[读]
  Public: GET /api/v1/.../config
    → Handler.GetConfig (public/handler.go:141)
    → settingsapp.GetConfig(defaults)   // merge site_config → data.brand/contact/seo/footer_links/...
    → settingsapp.GetByKey("nav_config") → data.nav_config             // Storage A 覆盖
    → overlay.ApplyPublicConfigOverlay  // main tenant 直接返回，不读 reseller_site_configs
    → cache → response
```

### 4.2 分销租户（reseller tenant，ResellerID != nil）
```
[写]
  Reseller Admin UI: ResellerSiteConfigs.vue
    → PUT /admin/resellers/site-configs/:reseller_id {nav_config:{...}}
    → AdminSiteConfigHandler.UpdateSiteConfig
    → resellerapp.UpdateAdminSiteConfig
    → normalizeResellerNavConfig
    → gormstore.UpsertSiteConfig  →  reseller_site_configs.nav_config_json  [Storage B]

  Reseller 自助 UI: ResellerSiteConfigPanel.vue
    → PUT /api/v1/user/reseller/site-config {nav_config:{...}}
    → UserHandler.UpdateSiteConfig
    → resellerapp.UpdateUserSiteConfig
    → gormstore.UpsertSiteConfig  →  reseller_site_configs.nav_config_json  [Storage B]

[读]
  Public: GET /api/v1/.../config (reseller host)
    → Handler.GetConfig
    → settingsapp.GetConfig(defaults)   // base = Storage A + site_config 其他字段
    → settingsapp.GetByKey("nav_config") → data.nav_config = Storage A     // 先放主站默认
    → overlay.ApplyPublicConfigOverlay
        → repo.GetSiteConfigByResellerID(resellerID)
        → applyResellerSiteConfigToPublicConfig
            → if len(cfg.NavConfigJSON) > 0: out["nav_config"] = cfg.NavConfigJSON  // Storage B 覆盖
    → cache → response
```

### 4.3 前端用户端消费点
| 文件 | 行号 | 读取方式 |
|---|---|---|
| `frontend/user/src/composables/useNavConfig.ts` | 81-83 | `appStore.config?.navigation \|\| appStore.config?.nav_config` |
| `frontend/user/src/composables/useSiteConfig.ts` | 63 | `config.value?.navigation \|\| config.value?.nav_config \|\| {}` |
| `frontend/user/src/components/Navbar.vue` | 208, 222 | `useNavConfig()` → primaryNavItems/secondaryNavItems |
| `frontend/user/src/components/Footer.vue` | 80 | `appStore.config?.nav_config?.builtin \|\| appStore.config?.navigation?.builtin` |
| `frontend/user/src/components/home/HomeExperience.vue` | 263-264 | `appStore.config?.nav_config?.builtin?.notice / .blog` |

> 前端代码里有 `config.navigation` 优先于 `config.nav_config` 的兼容逻辑，但**后端没有任何地方写入 `navigation` key**（全仓 grep `data["navigation"]` 0 命中），所以实际命中的永远是 `nav_config`。

---

## 5. Admin 两个导航编辑页面对比

| 维度 | Settings → Navigation | Site Builder → NavFooter |
|---|---|---|
| 文件 | `frontend/admin/src/views/admin/components/SettingsNavigationTab.vue` | `frontend/admin/src/views/site-builder/NavFooter.vue` |
| 读取 API | `GET /admin/settings?key=nav_config` | `GET /admin/settings?key=site_config`（再从 `data.nav_config` 取） |
| 写入 API | `PUT /admin/settings` body `{key:"nav_config", value:{builtin, custom_items}}` | `PUT /admin/settings` body `{key:"site_config", value:{...current, nav_config:{...}, footer_links:[...]}}` |
| 落地存储 | **Storage A**：`settings` 表独立行 `key='nav_config'` | **Storage C**：`settings` 表 `key='site_config'` 行的嵌套字段 |
| 后端归一化 | 走 `normalizeNavConfig`（严格校验标题/URL/类型/图标） | `normalizeSiteSetting` 原样拷贝，**不归一化 nav_config** |
| 是否生效于前台 | ✅ 主站前台直接生效 | ❌ 前台导航被 Storage A 覆盖；只有同表单里的 `footer_links` 生效 |
| 功能重复度 | — | **导航部分 100% 重复且为死代码**；Footer 部分独立有用 |
| UI 能力 | builtin 3 开关 + custom_items（最多 10，多语言标题、图标选择器、link_type/target/sort_order/enabled） | builtin 3 开关 + custom_items（多语言 tab、图标走 HOME_ENTRY_ICONS）+ footer_links 列表 |
| 默认值 | builtin 全 true，custom_items 空数组 | builtin 全 false（form 初始值 line 30），读取后回填 |

**结论**：两个页面在「顶部导航」功能上完全重复，且 `NavFooter.vue` 的导航编辑是**静默失败**——管理员保存后不报错，但前台不变。这是典型的「以为改了其实没改」的坑。

---

## 6. 用户端实际生效的数据源

| 场景 | 实际生效的 nav_config 来源 |
|---|---|
| 主站域名访问（无 reseller overlay） | **Storage A**：`settings.nav_config`（由 `SettingsNavigationTab.vue` 编辑） |
| 分销域名访问，且该 reseller 在 `reseller_site_configs` 有行且 `nav_config_json` 非空 | **Storage B**：`reseller_site_configs.nav_config_json`（由 `ResellerSiteConfigs.vue` 或 `ResellerSiteConfigPanel.vue` 编辑） |
| 分销域名访问，但该 reseller 没配过 nav_config（`len(NavConfigJSON)==0`） | 回落到 **Storage A**（base 值保留） |
| 任何场景下 `NavFooter.vue` 改的导航 | **不生效**（Storage C 死存储） |

---

## 7. DB Migration / Seed 初始数据检查

- `db/` 目录下仅有 `hcz.db`（SQLite 二进制），**无 migrations/、seed/ 目录**。
- Schema 由 `internal/bootstrap/database/migrations/registry.go:52 AutoMigrate()` 统一建表：
  - `settings` 表：line 97 `&settingsstore.SettingRecord{}`
  - `reseller_site_configs` 表：line 138 `resellerstore.Migrate(db)`（reseller 模块自注册）
- **无任何 seed 代码向 `settings` 表插入 `key='nav_config'` 或 `key='site_config'` 初始行**。
- 初始值完全由后端硬编码兜底：
  - `public/handler.go:232-235`：nav_config 缺失时返回 `{builtin:{blog:true,notice:true,about:true}, custom_items:[]}`
  - `settingsapp.GetConfig(defaults)`：defaults 里没有 nav_config，所以 site_config.nav_config 缺失时前端拿到的是 handler.go 的硬编码默认。
- 测试侧 seed：`internal/modules/reseller/integrationtest/application_site_config_test.go:161` 有构造 `nav_config` 的测试数据，但仅用于集成测试，不进生产。

---

## 8. 推荐的唯一 Source of Truth

### 8.1 推荐方案：保留分层模型，删除 Storage C

**主站默认 = Storage A（`settings.nav_config`），分销覆盖 = Storage B（`reseller_site_configs.nav_config_json`）。**

理由（基于真实调用量）：
1. **Storage A 是主站唯一活源**：`SettingsNavigationTab.vue` 写、`public/handler.go:228` 读，且有 `normalizeNavConfig` 严格归一化和缓存失效 effect。它就是主站事实标准。
2. **Storage B 是白标隔离的必要分层**：分销场景必须按 reseller_id 隔离，不能合并到全局 settings；overlay 机制（`ApplyPublicConfigOverlay`）已经正确实现了「reseller 非空才覆盖」的语义。
3. **Storage C 是纯死代码**：
   - 写入方只有 `NavFooter.vue` 一处；
   - 后端读路径被 `public/handler.go:228-236` 硬覆盖；
   - 无归一化、无测试保护；
   - 留着只会持续误导管理员。

### 8.2 不推荐的方案
- **把 nav_config 收进 `settings.site_config.nav_config`（即让 Storage C 成为唯一源）**：需要改动 public handler 的读取逻辑、删除独立 `nav_config` key、迁移存量数据、改 `SettingsNavigationTab.vue` 写入路径——收益为零，因为 Storage A 已经工作得好好的。
- **把 reseller 的 nav_config_json 合并到 settings 表**：破坏白标隔离，每个 reseller 一行的语义被打散，且当前 overlay 机制已经清晰。

---

## 9. 迁移策略建议（分阶段，不要求本轮实施）

### Phase 1：止血（P0，1 人日）
1. **删除 `NavFooter.vue` 里的「顶部导航」区块**，只保留「Footer 链接」区块。
   - 避免管理员继续在死存储上做无效操作。
   - 把导航编辑入口完全收敛到 `Settings → Navigation`（即 Storage A）。
2. （可选）在 `NavFooter.vue` 顶部加一条提示：「导航项请在 设置 → 导航 中维护」。

### Phase 2：数据清理（P1，半人日）
1. 写一次性迁移脚本（或在启动 migration 里加 idempotent 步骤）：
   - 读取 `settings` 表中 `key='site_config'` 行的 `value_json.nav_config`；
   - 如果 `value_json.nav_config` 非空，且 `settings` 表中 `key='nav_config'` 行不存在或为空，则把它 normalize 后写入 `key='nav_config'` 行（数据抢救）；
   - 然后从 `site_config` 行的 value_json 里删掉 `nav_config` 字段（避免后续误读）。
2. 在 `normalizeSiteSetting`（site_normalize.go:33）里**显式丢弃** `value["nav_config"]`，即使前端误传也不存。这样 Storage C 被物理封死。

### Phase 3：统一命名（P2，可选）
1. 前端 `useNavConfig.ts:82` 和 `useSiteConfig.ts:63` 里的 `config.navigation || config.nav_config` 兼容逻辑可以保留（向后兼容），但后端永远只写 `nav_config`。
2. 长期如果想把字段名改成 `navigation`，需要后端同步改 public handler 输出 key + reseller overlay 输出 key + 数据库迁移，建议放在下一个大版本做，本轮不做。

### Phase 4：分销侧一致性（P3，可选）
1. `ResellerNavConfigInput.CustomItems` 当前类型是 `[]ResellerFooterLinkInput`（只有 name/url），而 Storage A 的 custom_items 有 `title/link_type/target/sort_order/enabled/icon`——结构不对齐。
2. 前端 `useNavConfig.ts:113` 已经在做兼容：「分销站配置面板写入的 custom_items 没有 link_type，只能按 URL 形态判断」。
3. 建议把 `ResellerNavConfigInput.CustomItems` 结构对齐 Storage A，统一走同一个 normalize 函数（抽出共享的 `normalizeNavCustomItem`），避免长期漂移。

---

## 10. 所有相关文件路径清单

### 后端（Go）
- `internal/constants/constants.go`（line 538, 554 — key 常量）
- `internal/modules/settings/infrastructure/gormstore/store.go`（settings 表模型 + Upsert）
- `internal/modules/settings/application/default_registry.go`（line 60-63 — nav_config 注册）
- `internal/modules/settings/application/site_normalize.go`（line 33 normalizeSiteSetting；line 335 normalizeNavConfig）
- `internal/modules/settings/application/general.go`（line 82 GetConfig；line 352 GetSiteCurrency；line 371 GetSiteBrand）
- `internal/modules/settings/transport/http/admin_handler.go`（通用 settings GET/PUT）
- `internal/modules/settings/transport/http/routes.go`（admin 路由）
- `internal/modules/settings/transport/http/public/handler.go`（line 141 GetConfig；line 228-236 nav_config 读取+覆盖；line 247 overlay）
- `internal/modules/reseller/domain/site.go`（line 30 SiteConfig 模型；line 40 NavConfigJSON）
- `internal/modules/reseller/infrastructure/gormstore/site_config.go`（UpsertSiteConfig / GetSiteConfigByResellerID）
- `internal/modules/reseller/application/site_config.go`（line 52 ResellerNavConfigInput；line 225 normalizeResellerNavConfig；line 358 ApplyPublicConfigOverlay；line 511-513 overlay 写 nav_config）
- `internal/modules/reseller/transport/http/admin/admin_site_config_handler.go`（admin reseller site-config handler）
- `internal/modules/reseller/transport/http/admin/routes.go`（line 33-36 admin 路由）
- `internal/modules/reseller/transport/http/user/user_handler.go`（line 56-78 请求结构；line 151/165 GET/PUT）
- `internal/modules/reseller/transport/http/user/routes.go`（line 15-16 user 路由）
- `internal/modules/reseller/transport/http/presenter/reseller.go`（line 104, 138, 310, 370 presenter DTO）
- `internal/bootstrap/publicconfig/adapters.go`（line 143 overlay adapter）
- `internal/bootstrap/publicconfig/wiring.go`（overlay 装配）
- `internal/bootstrap/database/migrations/registry.go`（line 52 AutoMigrate；line 97 settings 表；line 138 reseller migrate）

### 前端 Admin（Vue）
- `frontend/admin/src/views/admin/components/SettingsNavigationTab.vue`（Storage A 编辑页）
- `frontend/admin/src/views/site-builder/NavFooter.vue`（Storage C 编辑页——导航部分死代码）
- `frontend/admin/src/views/site-builder/NavItemRow.vue`（NavFooter 子组件）
- `frontend/admin/src/views/site-builder/FooterLinkRow.vue`（NavFooter 子组件）
- `frontend/admin/src/views/admin/ResellerSiteConfigs.vue`（Storage B 管理员编辑页）
- `frontend/admin/src/api/types.ts`（nav_config 类型定义）

### 前端 User（Vue）
- `frontend/user/src/composables/useNavConfig.ts`（导航消费入口）
- `frontend/user/src/composables/useSiteConfig.ts`（line 63 navigation 兼容）
- `frontend/user/src/components/Navbar.vue`（line 208/222 使用 useNavConfig）
- `frontend/user/src/components/Footer.vue`（line 80 读 nav_config.builtin）
- `frontend/user/src/components/home/HomeExperience.vue`（line 263-264 读 nav_config.builtin）
- `frontend/user/src/components/reseller/ResellerSiteConfigPanel.vue`（Storage B 分销自助编辑）
- `frontend/user/src/types/siteConfig.ts`（类型）
- `frontend/user/src/api/types.ts`（类型）

### 测试
- `internal/modules/settings/application/default_registry_test.go`
- `internal/modules/settings/application/site_normalize_test.go`
- `internal/modules/reseller/integrationtest/application_site_config_test.go`
