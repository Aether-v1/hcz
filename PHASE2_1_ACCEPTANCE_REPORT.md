# HCZ User Frontend Phase 2.1 — Real API Integration Acceptance Report

- 验收时间：2026-10-05（Asia/Shanghai）
- 前端：`E:\Users\orang\Downloads\Compressed\hcz_user`（Vue 3 + Vite 7.3.6 + Pinia + TS）
- 后端：`E:\Users\orang\Downloads\Compressed\hcz_v1`（Go 1.26.5 / Gin / GORM / SQLite WAL / Redis）
- 测试账号：`test@hcz.local` / `Test1234`
- 管理员：`admin` / `HczDev@2026`
- 本轮代码改动：1 个文件（P2-2 断网恢复自动重取）

---

## 1. 后端实际启动方式

| 项 | 结论 |
|---|---|
| 技术栈 | Go 1.26.5，单二进制，Gin + GORM，SQLite（WAL）+ Redis |
| 构建命令 | `go build -o hcz-api.exe ./cmd/server`（一次通过，无依赖错误） |
| 启动命令 | `.\hcz-api.exe`（工作目录必须是项目根 `E:\Users\orang\Downloads\Compressed\hcz_v1`） |
| 启动模式 | 默认 `all`（HTTP + Worker），可选 `api` / `worker` |
| 监听地址 | `0.0.0.0:8080`（日志确认 `app_start addr=0.0.0.0:8080`） |
| 配置加载 | viper 从工作目录找 `config.yml`（搜索路径 `./`、`../`、`./etc`），支持环境变量覆盖（`.`→`_`，无前缀） |
| 密钥校验 | 启动时强制校验 `app.secret_key`、`jwt.secret`、`user_jwt.secret`：各自 ≥32 字符、不含默认值子串、三者互不相同，否则 Fatal 退出 |
| 数据库 | 启动时自动 `MkdirAll("db","uploads","logs")` + `AutoMigrate()`，无需手动建表 |
| 默认管理员 | `bootstrap.default_admin_username/password` 或环境变量 `DJ_DEFAULT_ADMIN_USERNAME/PASSWORD`，首次启动自动创建 |

### 本地 config.yml（开发专用，非生产凭据）
已基于 `config.yml.example` 创建，关键配置：
- `server.mode: debug`
- `database.driver: sqlite`, `dsn: ./db/hcz.db`
- `redis.enabled: true`（127.0.0.1:6379，无密码）
- `queue.enabled: true`
- `cors.allowed_origins: ["*"]`
- `email.enabled: false`（本地开发无需 SMTP）
- 三个 64 字符十六进制随机密钥（PowerShell `RandomNumberGenerator` 生成，彼此不同）

---

## 2. 本地运行依赖

| 依赖 | 要求 | 状态 |
|---|---|---|
| Go | ≥1.21（实际 1.26.5） | ✅ 已安装 |
| SQLite | 内嵌（go-sqlite3），无需外部服务 | ✅ 自动创建 `./db/hcz.db` |
| Redis | 必需（cache + queue），默认 127.0.0.1:6379 | ✅ 正在运行（PID 5936） |
| config.yml | 必需，含 3 个强密钥 | ✅ 已创建 |
| SMTP / 邮件服务 | 非必需（`email.enabled: false`） | ✅ 已禁用 |
| Node.js / pnpm | 前端构建（实际 npm） | ✅ 已安装 |

> 注：注册接口默认要求邮箱验证码（settings 中邮箱验证默认开启），即使 `email.enabled=false`。本地开发需通过 admin API 或直接操作 `email_verify_codes` 表插入验证码完成注册。

---

## 3. API Base URL

| 环境 | Base URL | API 前缀 |
|---|---|---|
| 后端直连 | `http://localhost:8080` | `/api/v1` |
| 前端 dev（Vite 代理） | 相对路径（空 base） | `/api/v1` → 代理到 `http://localhost:8080` |
| 前端生产 | `import.meta.env.VITE_API_BASE_URL` | `/api/v1` |

- Vite 代理配置（`vite.config.ts:54-72`）：`/api`、`/uploads`、`/sitemap.xml`、`/robots.txt` → `http://localhost:8080`，`changeOrigin: false`（保留原始 Host 供分销商租户解析）
- 前端 API client（`src/api/client.ts:19-20`）：`API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''`，`API_PREFIX = '/api/v1'`

---

## 4. 成功联调的接口清单

所有接口 HTTP status 恒为 200（除 `/health` 和不存在的端点），业务成败在 body `status_code`（0=成功，非0=错误）。响应包格式：`{status_code, msg, data, pagination?}`。

| # | 端点 | Method | 鉴权 | status_code | 耗时 | 关键响应 |
|---|---|---|---|---|---|---|
| 1 | `/health` | GET | 无 | — | 80ms | `{"status":"ok"}`（无信封） |
| 2 | `/api/v1/public/config` | GET | 无 | 0 | 17ms | `data.currency:"CNY"`, `storefront_template:"classic"`, `wallet_only_payment:true` |
| 3 | `/api/v1/auth/login`（错误密码） | POST | 无 | **401** | 134ms | `msg:"邮箱或密码错误"` |
| 4 | `/api/v1/auth/login`（正确） | POST | 无 | 0 | 153ms | `data.{token, user{id,email,nickname}, expires_at, requires_totp:false}` |
| 5 | `/api/v1/auth/register` | POST | 无 | 0 | — | `data.{token, user, expires_at}`（需邮箱验证码） |
| 6 | `/api/v1/me` | GET | User JWT | 0 | 117ms | `data.{id,email,nickname,locale:"zh-CN",total_recharged:"0.00",total_spent:"0.00"}` |
| 7 | `/api/v1/wallet` | GET | User JWT | 0 | 1-56ms | `data.{available_balance:"0.00",frozen_balance:"0.00",total_balance:"0.00",currency:"USDT"}` |
| 8 | `/api/v1/public/categories`（无 token） | GET | 无 | **401** | 0ms | `msg:"缺少 Authorization header"` |
| 9 | `/api/v1/public/categories`（带 token） | GET | User JWT | 0 | 10-53ms | 4 个分类：game-cards/membership/gift-cards/software |
| 10 | `/api/v1/public/products?page=1&page_size=8` | GET | User JWT | 0 | 23ms | 3 个商品 + pagination，`price_amount` 为 string |
| 11 | `/api/v1/public/products/:slug` | GET | User JWT | 0 | 5ms | 单个商品对象 |
| 12 | `/api/v1/public/banners?position=home_hero&limit=5` | GET | User JWT | 0 | 10ms | `data:[]`（空） |
| 13 | `/api/v1/public/posts?type=notice&page=1&page_size=2` | GET | User JWT | 0 | 11ms | `data:[]` + pagination（空） |
| 14 | `/api/v1/admin/login` | POST | 无 | 0 | — | `data.{token, expires_at, user{id,username}}` |
| 15 | `/api/v1/auth/refresh` | POST | User JWT | **404** | 75ms | **不存在 refresh token 端点** |

---

## 5. 登录态验证结果

| 验证项 | 结果 | 证据 |
|---|---|---|
| 未登录访问 `/` | ✅ 重定向到 `/auth/login?redirect=/` | 路由守卫 `requiresUserAuth:true`（`router/index.ts:119`）；浏览器实测 URL 变为登录页 |
| 登录页面渲染 | ✅ 正常 | 浏览器实测，email/password 输入框可定位 |
| 登录成功 | ✅ `status_code:0`，返回 token | API #4；浏览器实测点击登录后跳转 |
| Token 保存 | ✅ `localStorage.user_token` | 浏览器实测 `token_prefix: "eyJhbGciOiJIUzI1NiIs..."` |
| 登录后跳转 | ✅ 到 `/`（因 `redirect=/` 参数） | 浏览器实测 `post_login_url: http://127.0.0.1:5174/` |
| /me 用户信息 | ✅ 返回完整用户对象 | API #6 |
| 首页登录态 | ✅ 钱包入口显示、个人中心按钮 | 浏览器截图确认 |
| 钱包余额 | ✅ `0.00 USDT` | API #7 + 浏览器截图 Hero 区确认 |
| 刷新页面 | ⚠️ 仅靠 localStorage token 存在判定，不主动调 /me 验证 | `userAuth.ts:27` `isAuthenticated = !!token`；token 过期后需等首个 API 401 才被发现 |
| Token 过期 | ⚠️ 无主动过期检查，依赖后端 401 被动发现 | `client.ts:167-172` 业务码 401 → 清 token + 跳登录 |
| API 401 处理 | ✅ 三处 401 分支均清 token + 跳登录 | `client.ts:138-143, 156-160, 167-172` |
| 登录失败提示 | ✅ 错误密码返回 `msg:"邮箱或密码错误"` | API #3 |
| Refresh 机制 | ❌ **不存在** | 后端 `POST /auth/refresh` → 404；前端无 refresh 逻辑 |
| 退出登录 | ✅ 清 localStorage + 重定向登录页 | 浏览器实测：清 token 后访问 `/` → 跳 `/auth/login?redirect=/`；但 logout 不调服务端登出接口 |

---

## 6. 首页每个模块真实数据验证结果

### A. Hero
- 内容：品牌标题"让生活更简单"、副标题"全球充值·游戏点卡·生活服务"、USDT 钱包入口
- 数据来源：纯静态 i18n 文案 + `walletAPI.account()`（仅登录态）
- 结论：✅ 无需业务数据，钱包余额真实显示 `0.00 USDT`

### B. 搜索
- 实现：首页搜索框提交 → `router.push({path:'/products', query:{search:keyword}})`（`HomeExperience.vue:286-289`）
- 参数编码：`URLSearchParams` 自动编码（`client.ts:31-42`）
- 中文/英文/特殊字符：✅ 安全编码
- 空关键词：✅ 按钮 disabled + 提交时 trim 守卫
- 落地页：`/products?search=<kw>` → `useProductList.ts` 消费，`productAPI.list({search})`
- 结论：✅ 搜索参数正确传递

### C. 四个核心服务入口
- 实现：`HomeExperience.vue:202-214` 用硬编码中文正则匹配 `slug + 所有语言 name`
- 后端实际分类（4 个）：

| slug | name.zh-CN | name.en | 前端匹配 |
|---|---|---|---|
| `game-cards` | 游戏充值 | Game Topup | ✅ `/游戏\|遊戲\|game/` 命中 → "游戏娱乐" |
| `membership` | 会员服务 | Membership | ❌ 不匹配 |
| `gift-cards` | 礼品卡 | Gift Cards | ❌ 不匹配 |
| `software` | 软件订阅 | Software | ❌ 不匹配 |

- 话费充值(phone)、流量充值(data)、生活缴费(bills)：**后端无对应分类** → 3 个入口显示"暂未提供"（disabled 卡片）
- 跳转目标：用 `category.slug`（正确）→ `/categories/:slug`
- 结论：⚠️ **P1 信息架构风险** — 前端用中文正则匹配（非 slug/id 精确契约），后端缺少 phone/data/bills 核心分类。3/4 入口恒为"暂未提供"。本轮按约定未改前端匹配逻辑。

### D. 热门推荐
- API：`GET /api/v1/public/products?page=1&page_size=8` → 3 个商品
- 商品字段：`price_amount`（string，如 `"99.90"`）、`title{en,zh-CN}`、`images[]`、`tags[]`、`category{...}`、`is_sold_out:true`、`stock_status:"out_of_stock"`
- 价格展示：`formatPrice(priceAmount, siteCurrency)` → `99.90 CNY` / `15.00 CNY` / `22.50 CNY` ✅
- 原价/划线价：后端无 `original_price` / `promotion_price_amount` 字段 → `hasPromotionPrice=false` → 原价行整段隐藏 ✅（不会出现同价划线）
- 标签：`tags[]` 展示（如 "hot"）
- 跳转详情：`RouterLink` 到商品详情页 ✅
- disabled/unavailable：⚠️ **P2** — 3 个商品全部 `is_sold_out:true`，但 `HomeServiceCard.vue` 未使用 `isSoldOut` helper，售罄商品仍以正常可点击卡片展示，无售罄标签/禁用
- 图片：id=1 商品 images 为外链（`https://example.com/a.png`，加载失败）→ `@error` fallback 到分类图标 ✅；id=2/3 images=null → 直接显示分类图标 ✅
- 结论：✅ 价格/名称/跳转正确；⚠️ 售罄状态未展示

### E. 更多服务
- 实现：复用 categories + products 数据，"全部服务"链接跳转商品列表页
- 结论：✅ 正常渲染

### F. Banner / Article
- Banner API：`GET /api/v1/public/banners?position=home_hero&limit=5` → `data:[]`（空）
- 前端：`v-if="bannerCount > 0"` → 整段 Banner section 隐藏 ✅
- 资讯 API：`GET /api/v1/public/posts?type=notice&page=1&page_size=2` → `data:[]`（空）
- 前端：`v-if="newsPosts.length"` → 整段新闻 section 隐藏 ✅
- 结论：✅ 空数据安全处理，无报错、无空壳

### G. Wallet
- API：`GET /api/v1/wallet` → `data.{available_balance:"0.00", frozen_balance:"0.00", total_balance:"0.00", currency:"USDT"}`
- 前端读取：`HomeExperience.vue:264-265` 读 `wallet.available_balance` + `wallet.currency`
- **重要确认**：首页读 `available_balance` 与后端实际字段**一致**，是正确的。全站其余 4 处（`useCheckout.ts:914`、`usePayment.ts:567`、`WalletPanel.vue:282`、`WalletWithdrawal.vue:220`）和 TS 类型 `WalletAccountData`（`types.ts:93-95`）读 `balance`，与后端不符（但超出本阶段首页范围）。
- 数据类型：`available_balance` 是 **string**（定点小数字符串），前端直接字符串拼接，无 `Number()` 转换 ✅
- null/missing 安全：`if (wallet?.available_balance !== undefined && !== null)` 守卫 ✅
- 未登录：不请求（early return）+ 入口隐藏（`v-if="auth.isAuthenticated"`）✅
- 结论：✅ 首页钱包字段正确、类型安全、USDT 语义正确

---

## 7. CNY / USDT 金额语义验证

### 商品售价（站点币 CNY）
- 后端字段：`price_amount` = string（如 `"99.90"`），商品对象内**无 currency 字段**
- 币种来源：`/public/config` → `data.currency = "CNY"` → `siteCurrency`（`useProduct.ts:15-18`，过 `/^[A-Z]{3}$/` 校验，默认 `CNY`）
- 展示：`formatPrice(priceAmount, siteCurrency)` → `amountToCents` → `centsToAmount` → 输出 `"99.90 CNY"`（3 位币种代码后缀，非 ¥ 符号）
- 浏览器实测：首页商品卡片显示 `99.90 CNY`、`15.00 CNY`、`22.50 CNY` ✅

### 钱包余额（固定 USDT）
- 后端字段：`available_balance` = string `"0.00"`，`currency` = `"USDT"`
- 展示：`` `${wallet.available_balance} ${wallet.currency || 'USDT'}` `` → `"0.00 USDT"`
- 浏览器实测：Hero 区钱包入口显示 `0.00 USDT` ✅

### 最终订单扣款
- 由后端计算（汇率快照 + USDT 扣款），前端不参与计算 ✅

### 错误行为检查（全部不存在）
| 检查项 | 结果 | 证据 |
|---|---|---|
| 把 CNY 当 USDT 显示 | ❌ 不存在 | 商品用 siteCurrency=CNY，钱包用 wallet.currency=USDT，来源独立 |
| 给 CNY 加 USDT 后缀 | ❌ 不存在 | `formatPrice` 输出 `XX.XX CNY` |
| 前端自行换算 CNY↔USDT | ❌ 不存在 | `money.ts:73-104` 注释 "display only, NO money calculation" |
| 用钱包汇率计算商品展示价 | ❌ 不存在 | 首页无 `exchangeRate` 引用 |
| `Number()` / `parseFloat` 强转金额 | ❌ 不存在 | `parseDecimalToScaledInt`（`money.ts:3-32`）字符串逐位解析，仅对整数部分做 `Number()`，无浮点四则运算 |
| `toFixed()` 不必要舍入 | ❌ 不存在 | `centsToAmount` 整数转字符串，固定 2 位小数 |
| 直连 CoinGecko / 外部行情 | ❌ 不存在 | 全局搜索 0 命中 |

### 结论
✅ **金额语义完全正确**，无 P0 问题。商品价 CNY、钱包 USDT、后端计算扣款，三者语义清晰分离。

---

## 8. Offline / Recovery 测试

### 13 种运行时状态验证

| # | 状态 | 实际表现 | 验证方式 | 文件:行号 |
|---|---|---|---|---|
| 1 | 未登录 | 路由守卫重定向 `/auth/login?redirect=/` | 浏览器实测 + 代码 | `router/index.ts:119,364-367` |
| 2 | 已登录 | `Promise.allSettled` 并发拉 categories/products/banners/wallet/news | 浏览器实测 + API | `HomeExperience.vue:308` |
| 3 | Loading | 分类 loading 文案；商品 ×4 骨架屏；钱包 loading 文案 | 代码分析 | `HomeExperience.vue:56,80-82,19` |
| 4 | API timeout | 10s 超时 + AbortController → reject `networkError` | 代码分析 | `client.ts:62,94-95,107-111` |
| 5 | API 500 | `!response.ok` → reject serverError → 首页显示错误块+重试 | 代码分析 | `client.ts:53,153-163`; `HomeExperience.vue:57-60,83-87` |
| 6 | API 401 | 业务码 401 → 清 token + 跳登录 | API 实测 + 代码 | `client.ts:167-172` |
| 7 | 空分类 | 4 核心入口全部 disabled "unavailable" | 代码分析 | `HomeExperience.vue:186-197,208-214,49-53` |
| 8 | 空商品 | 空态 PackageOpen 图标 + noProducts 文案 | 代码分析 | `HomeExperience.vue:219,91` |
| 9 | 空 Banner | `bannerCount=0` → 整段隐藏 | API 实测 + 代码 | `HomeExperience.vue:110`; `useBannerCarousel.ts:31` |
| 10 | 图片加载失败 | `@error` → fallback 分类图标 | 代码分析 + 浏览器实测 | `HomeServiceCard.vue:5-6` |
| 11 | 断网启动 | `navigator.onLine=false` → 离线条 + API 失败 → 各模块错误/空态 | 代码分析 | `HomeExperience.vue:30-32,240-244,252-256` |
| 12 | 在线后断网 | `offline` 事件 → 离线条 + 后续 API 失败 | 代码分析 | `HomeExperience.vue:290-297,301-302` |
| 13 | 断网后恢复网络 | ✅ **已修复**：`updateOnline` 检测离线→在线跳变 → `Promise.allSettled` 重取全部数据 | 代码分析（修复 diff） | `HomeExperience.vue:290-297`（本轮新增） |

### P2-2 修复详情
**文件**：`src/components/home/HomeExperience.vue:290-297`

修复前：
```js
const updateOnline = () => { online.value = navigator.onLine }
```

修复后：
```js
const updateOnline = () => {
  const wasOffline = !online.value
  online.value = navigator.onLine
  // 离线→在线跳变后自动重取首页全部数据；loadWallet 内部自带登录态守卫
  if (wasOffline && navigator.onLine) {
    void Promise.allSettled([loadCategories(), loadProducts(), loadHomeBanners(), loadWallet(), loadNews()])
  }
}
```

- 函数名取自实际 `onMounted`（`:308`）调用，未臆造
- `loadWallet`（`:258-259`）登录守卫保留
- 未改动离线条显示逻辑
- 已通过生产 build（vue-tsc + vite build，0 错误）

### 结论
✅ 恢复网络后页面能够自动重新获取数据，无需用户刷新浏览器。

---

## 9. Browser Console / Network 问题

### 测试环境
- 浏览器：Chrome 154.0.8037.93（Selenium 4.50.0 自动化）
- 前端地址：`http://127.0.0.1:5174/`（用户前端 dev server）
- 8 个视口：Desktop 1366/1440/1920、Tablet 768/1024、Mobile 375/390/430（CDP `Emulation.setDeviceMetricsOverride` 精确控制）

### 逐分辨率结果

| 视口 | 尺寸 | Console Errors | Network Errors | 页面文本长度 | USDT | CNY | 商品链接 | 通过 |
|---|---|---|---|---|---|---|---|---|
| Desktop 1366 | 1366×768 | 0 | 1* | 302 | ✅ | ✅ | 8 | ✅ |
| Desktop 1440 | 1440×900 | 0 | 0 | 302 | ✅ | ✅ | 8 | ✅ |
| Desktop 1920 | 1920×1080 | 0 | 0 | 302 | ✅ | ✅ | 8 | ✅ |
| Tablet 768 | 768×1024 | 0 | 0 | 292 | ✅ | ✅ | 8 | ✅ |
| Tablet 1024 | 1024×768 | 0 | 0 | 300 | ✅ | ✅ | 8 | ✅ |
| Mobile 375 | 375×667 | 0 | 0 | 292 | ✅ | ✅ | 8 | ✅ |
| Mobile 390 | 390×844 | 0 | 0 | 292 | ✅ | ✅ | 8 | ✅ |
| Mobile 430 | 430×932 | 0 | 0 | 292 | ✅ | ✅ | 8 | ✅ |

\* Desktop 1366 的 1 个 network error 为 `net::ERR_ABORTED`（空 URL），系视口切换/页面刷新导致的导航中止，非真实请求失败。

### 视觉检查（截图确认）
- **首页加载闪烁**：无明显闪烁，骨架屏合理
- **Skeleton**：商品区 ×4 骨架屏，分类区 loading 文案 ✅
- **Header 跳动**：无跳动，布局稳定 ✅
- **价格布局**：`XX.XX CNY` 格式，布局整齐 ✅
- **长商品名/超长分类名**：当前种子数据名称较短，未触发溢出；CSS 有 truncate 处理
- **空状态**：Banner/资讯空数组 → 整段隐藏，无空白区域 ✅
- **Error 状态**：分类/商品有错误提示+重试按钮（代码确认）
- **图片比例**：商品卡片图片 fallback 为分类图标，比例一致 ✅
- **登录后余额**：Hero 区显示 `0.00 USDT` ✅
- **未登录行为**：重定向登录页 ✅
- **Bottom Nav**：移动端显示（首页/订单/发现/消息/我的），桌面端隐藏 ✅
- **页面滚动**：正常 ✅
- **Console errors**：**0**（全部 8 分辨率）✅
- **Network errors**：仅 1 个预期外的 ERR_ABORTED（视口切换）✅

### 截图文件
所有截图保存在 `E:\Users\orang\Downloads\Compressed\hcz_user\browser_screenshots\`：
- `desktop_1366_1366x768.png` ~ `mobile_430_430x932.png`（共 8 张）

---

## 10. 本轮修复文件

| 文件 | 改动 | 原因 |
|---|---|---|
| `src/components/home/HomeExperience.vue:290-297` | `updateOnline` 增加离线→在线跳变检测，恢复网络后 `Promise.allSettled` 重取全部首页数据 | P2-2：任务明确要求"恢复网络后页面应能够重新获取数据，不要要求用户必须刷新浏览器" |

**未修改的文件**：
- 未修改后端任何代码
- 未修改 API client
- 未修改认证体系
- 未修改分类匹配逻辑（P1-2 按约定未改，列为信息架构风险）
- 未修改售罄商品展示（P2，非阻断）

---

## 11. 前端问题

### P1（高优先级）
| # | 问题 | 位置 | 说明 |
|---|---|---|---|
| P1-F1 | 核心入口信息架构风险：前端用硬编码中文正则匹配分类，后端仅有 game-cards/membership/gift-cards/software，phone/data/bills 无对应分类 → 3/4 核心入口恒为"暂未提供" | `HomeExperience.vue:202-214` | 无稳定 slug 契约；建议后端补 phone/data/bills 分类，或前端改为按 slug 精确配置映射 |

### P2（中低优先级）
| # | 问题 | 位置 | 说明 |
|---|---|---|---|
| P2-F1 | 售罄商品无标签/禁用：`is_sold_out=true` 的商品仍以可点击卡片展示 | `HomeServiceCard.vue` | `useProduct.ts:76` 已有 `isSoldOut` helper 但未使用 |
| P2-F2 | Banner/资讯/钱包失败为静默 catch，无手动重试入口 | `useBannerCarousel.ts:143-147`; `HomeExperience.vue:267-269,277-285` | P2-2 修复后断网恢复可自动重取，部分缓解 |
| P2-F3 | 无 refresh token、无启动时 /me 校验、logout 不调服务端 | `userAuth.ts:195-198`; `router/index.ts:348-384` | token 24h 过期后前端无感知，直到下一个 401 |
| P2-F4 | 首页钱包未用 `formatWalletMoney`，内联拼接无两位小数规整 | `HomeExperience.vue:265` | 后端若返回 `12.5` 会显示 `12.5 USDT` 而非 `12.50 USDT` |
| P2-F5 | `WalletAccountData` 类型漂移：只有 `balance`，与首页实际读 `available_balance`/`currency` 不符 | `api/types.ts:93-95` | 因返回 any 未被 TS 拦截；其余 4 处读 `balance` 也与后端不符（超范围） |

### P0
**无。**

---

## 12. 后端问题（仅记录，不修复）

### P1（高优先级）
| # | 问题 | 证据 |
|---|---|---|
| P1-B1 | `/public/*`（categories/products/banners/posts）实际挂在需 JWT 的 `authedPublic` 组，无 token 返回 `status_code:401 "缺少 Authorization header"`，与 "public" 路径语义不符 | API #8 无 token → 401；代码 `storefront.Group("/public", UserJWTAuthMiddleware(...))` |

### P2（中低优先级）
| # | 问题 | 证据 |
|---|---|---|
| P2-B1 | 无 refresh token 端点，`POST /api/v1/auth/refresh` → 404 | API #15 |
| P2-B2 | 全接口 HTTP status 恒 200，业务成败靠 body `status_code`，非标准 HTTP 语义 | API #1/#2/#5 |
| P2-B3 | 商品无 `original_price` / `currency` 字段，原价/币种全靠站点 config 与 `promotion_price_amount` | API #10 |
| P2-B4 | 当前 3 个商品全部 `is_sold_out=true`、`stock_status=out_of_stock` | API #10 |
| P2-B5 | `internal/selfupdate` 包 7 个测试在 Windows 失败（`unsupported_os`、文件权限、文件锁等平台限制），与本轮触达接口无关 | `go test ./internal/selfupdate/...` 输出 |

### P0
**无。**

---

## 13. P0 / P1 / P2 分级汇总

### P0（阻断/资金安全）
**0 个。**

### P1（高优先级，功能/信息架构风险）
| ID | 问题 | 归属 |
|---|---|---|
| P1-F1 | 核心入口中文正则匹配分类 + 后端缺 phone/data/bills 分类 → 3/4 入口"暂未提供" | 前端 + 后端 |
| P1-B1 | `/public/*` 实际需 JWT，与 public 语义不符 | 后端 |

### P2（中低优先级，体验/健壮性）
| ID | 问题 | 归属 |
|---|---|---|
| P2-F1 | 售罄商品无标签/禁用 | 前端 |
| P2-F2 | Banner/资讯/钱包失败静默无重试 | 前端 |
| P2-F3 | 无 refresh / 启动 /me / 服务端 logout | 前端 + 后端 |
| P2-F4 | 首页钱包未用 formatWalletMoney | 前端 |
| P2-F5 | WalletAccountData 类型漂移 | 前端 |
| P2-B1 | 无 refresh token 端点 | 后端 |
| P2-B2 | HTTP status 恒 200 | 后端 |
| P2-B3 | 商品无 original_price/currency | 后端 |
| P2-B4 | 种子商品全部售罄 | 后端数据 |
| P2-B5 | selfupdate 测试 Windows 平台失败 | 后端测试环境 |

---

## 14. 测试结果

### 前端
| 命令 | 结果 | 详情 |
|---|---|---|
| `npm run build`（= `vue-tsc -b && vite build`） | ✅ 成功 | 3007 modules transformed，`✓ built in 22.05s`，0 错误 |
| `npm test`（= `node --experimental-strip-types --test tests/*.test.ts`） | ✅ 通过 | `# tests 62 / # pass 62 / # fail 0`，耗时 1650ms |
| `npx vue-tsc -b` | ✅ 无类型错误 | exit code 0 |

### 后端
| 范围 | 结果 | 详情 |
|---|---|---|
| `go test ./cmd/server/...` | ✅ 通过 | 入口测试通过 |
| `go test -short ./internal/modules/...` | ✅ 全部通过 | catalog（category/product）、content（banner/post）、auth、wallet、cart 等所有模块 cached/通过 |
| `go test ./internal/shared/money/...` | ✅ 通过 | money.Amount 处理测试通过 |
| `go test ./internal/selfupdate/...` | ⚠️ 7 个失败 | 全部为 Windows 平台限制（unsupported_os、文件权限 0500、文件锁行为），与本轮触达接口无关 |

### 浏览器自动化
| 项 | 结果 |
|---|---|
| Selenium 4.50.0 + Chrome 154 | ✅ |
| 8 分辨率截图 | ✅ 全部生成 |
| Console errors | ✅ 0（全部 8 分辨率） |
| 登录流程 | ✅ 成功 |
| 登出流程 | ✅ 成功 |

---

## 15. 最终 Verdict

### 进入 Phase 3 门槛核对

| 门槛 | 状态 | 证据 |
|---|---|---|
| P0 = 0 | ✅ 满足 | 第 13 节：P0 为 0 |
| 真实登录成功 | ✅ 满足 | 第 5 节：登录 → token 保存 → 首页加载 → 钱包显示 → 登出，全链路浏览器实测通过 |
| 首页核心真实数据链成功 | ✅ 满足 | 第 6 节：分类/商品/钱包/Banner/资讯 API 全部联调成功，真实数据渲染；3/4 核心入口因后端缺分类显示"暂未提供"（非前端阻断） |
| 金额语义正确 | ✅ 满足 | 第 7 节：商品 CNY、钱包 USDT、无换算、无精度丢失，浏览器实测显示正确 |
| 恢复网络可用 | ✅ 满足 | 第 8 节：P2-2 已修复，断网恢复后自动重取全部首页数据 |

### Verdict：**PASS WITH CONDITIONS**

### 条件（进入 Phase 3 前建议处理）
1. **P1-F1 分类信息架构**：后端需补充 phone/data/bills 核心分类（或前端改为按 slug 精确映射），否则首页 3/4 核心入口恒为"暂未提供"
2. **P1-B1 /public 鉴权**：确认 `/public/*` 需 JWT 是否为有意的"全站登录收口"设计；若是，需确保未登录落地页/SEO 有替代方案
3. **P2-F1 售罄商品展示**：首页商品卡片应展示售罄标签/禁用状态，避免用户点击售罄商品

### 不阻断项（可在 Phase 3 并行处理）
- 无 refresh token（前后端均未实现，token 24h 有效期可接受）
- Banner/资讯/钱包静默失败（P2-2 修复后部分缓解）
- 钱包类型漂移（超范围，影响其他页面）
- selfupdate 测试 Windows 平台失败（与本轮无关）

---

## 附录：环境与进程状态

- 后端 `hcz-api.exe`：保持运行，监听 `0.0.0.0:8080`
- 前端用户 dev server：运行在 `http://127.0.0.1:5174/`（5173 端口被 admin 前端占用）
- Redis：运行在 `127.0.0.1:6379`
- 测试用户：`test@hcz.local` / `Test1234`
- 管理员：`admin` / `HczDev@2026`
- 浏览器截图：`E:\Users\orang\Downloads\Compressed\hcz_user\browser_screenshots\`
- 浏览器测试脚本：`E:\Users\orang\Downloads\Compressed\hcz_user\browser_acceptance.py`
- 浏览器测试结果 JSON：`E:\Users\orang\Downloads\Compressed\hcz_user\browser_acceptance_result.json`

> **重要发现**：验收过程中发现端口 5173 上运行的是 **admin 前端**（来自 `HCZ-admin- wed` 目录），而非用户前端。本阶段用户前端实际运行在 5174 端口。前端 dev server 的 Vite 代理配置（`/api` → `localhost:8080`）与端口无关，因此 API 联调不受影响。
