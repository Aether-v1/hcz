# HCZ V1 前端全量 Build + Contract 扫描审计报告

- 审计时间：2026-10-05（Asia/Shanghai）
- 环境：Windows / Node v22.23.2 / npm 10.9.8 / Vue 3.5 + TS / Vite 7.3
- 范围：Admin 前端（`hcz_v1/frontend/admin`）+ User 前端（`hcz_user`，classic/vault 双主题）
- 原则：不新增功能，只审计 + 构建验证 + 最小必要修复；所有结论以实际构建输出和可验证源码为准。

---

## 0. 总结论（先读）

| 结论项 | 结果 |
|---|---|
| User 前端是否全绿 | **是**（vue-tsc PASS / tests 72/72 PASS / build PASS） |
| Admin 前端是否全绿 | **是**（vue-tsc PASS / tests 28/28 PASS / build PASS） |
| classic / vault 是否都可构建/可启动 | **是**。二者同一 Vite 构建，产物中 classic（`src/views/*`）与 vault（`src/templates/vault/*`，`import.meta.glob` 动态 chunk）双份 chunk 均成功编译 |
| Frontend Contract 是否有违规 | **无 P0；发现 1 类 P1 安全缺口（未消毒 v-html），已最小修复并复验全绿；其余为 P2 观察项/信息项** |
| Site Builder 前端 scripts 是否禁用 | **是**。`applyCustomScripts` 为 deprecated 空实现且全项目零调用，config scripts 字段既不执行也不注入运行时 |

**P0：无。**

---

## 1. 项目结构与构建命令基线

### Admin（`hcz_v1/frontend/admin`，git repo 内）
- `package.json` scripts：`build = vue-tsc -b && vite build`，`test = vitest run`。无独立 type-check 脚本，类型检查并入 build / 单独 `npx vue-tsc -b`。
- `node_modules` 已存在，跳过 install。

### User（`hcz_user`，非 git repo，独立项目）
- `package.json` scripts：`build = vue-tsc -b && vite build`，`test = node --experimental-strip-types --test tests/*.test.ts`（Node 内置 test runner，无 vitest）。
- classic / vault 组织方式（读 `src/templates/registry.ts`）：
  - classic = `src/views/*`；vault = `src/templates/vault/*`。
  - **不是两个独立构建目标**，而是同一 Vite 单构建，运行时按 `templateView()` 切换：vault 缺页自动回退 classic。vault 页面通过 `import.meta.glob('./vault/**/*.vue')` 打动态 chunk。
  - 因此 "classic/vault 都可构建" 的判据 = 单次 `vite build` 同时产出两套页面 chunk（见 §3 证据）。
- `node_modules` 已存在，跳过 install。

---

## 2. Frontend Contract 扫描结果

货币语境区分基准：Site Currency（站点展示货币，后台可配）/ Wallet USDT（钱包记账单位，恒 USDT）/ C2C fiat（C2C 法币，如 CNY/USD）/ C2C USDT（C2C 加密计价单位）。

| # | 问题类型 | 命中位置（文件:行） | 严重度 | 修复状态 / 判定 |
|---|---|---|---|---|
| C1 | 旧 wallet 单 balance 字段 | User `src/api/types.ts:93` `WalletAccountData { balance: string }`；读取点 `useCheckout.ts:914`、`usePayment.ts:567`、`personal/WalletPanel.vue:282`、`WalletWithdrawal.vue:220`、`GiftCardPanel.vue:137` | P2（观察） | 未改。端用户钱包为单 `balance`，无 available/frozen 拆分；而分销商 balance accounts 已用新模型 `available_amount`/`locked_amount`（`types.ts:369`、`utils/resellerFinance.ts:52`）。端用户钱包是否应拆分需后端契约确认，前端无法自证，不在本次前端最小修复范围 |
| C2 | 硬编码 CNY（Site Currency 兜底） | Admin `Products.vue:112/283`、`Users.vue:73/119`、`UserDetail.vue:86/103`、`WholesalePrices.vue:34/231`、`Settings.vue:92/191/399`；User `useCheckout.ts:234`、`useCart.ts:26`、`useProductDetail.ts:606`、`useProduct.ts:17`、`GiftCardPanel.vue:131`、`WalletPanel.vue:262` | P2（信息项） | 未改。全部为 `data.currency \|\| 'CNY'` / `config.currency \|\| 'CNY'` 形式：先读后端配置，仅在非法/缺失时回退 CNY。属 Site Currency 语境的防御兜底，非无视配置的硬编码 |
| C3 | 硬编码 CNY（C2C/渠道法币） | Admin `components/PaymentChannelModal.vue:144/167/187/416/439/459`（`fiat`/`base_currency`/`fiat_currency: 'CNY'`） | P2（信息项） | 未改。支付渠道（bepusdt/tokenpay/dujiaopay 等）法币配置表单的默认值，可编辑，属 C2C fiat 语境。Admin `C2CListings.vue:113` 的 "法币币种 (如 CNY)" 仅为输入框 placeholder 提示文案（P3） |
| C4 | 硬货币符号 | 全仓搜索 `¥`/`RMB`/`人民币`（排除 i18n） | — | **无命中**。金额一律走 `formatMoney`/`formatWalletMoney` 格式化，无符号级硬编码 |
| C5 | 旧订单九态 | 双端 `utils/status.ts`；Admin `views/admin/Orders.vue`、`OrderDetailDialog.vue`、`ProcurementOrders.vue`、`c2c.ts`；User `usePayment.ts`、`useOrderDisplayHelpers.ts`、`resellerConsole.ts`、OrderDetail/GuestOrderDetail 等 | P2（观察） | 未改。经核对：双端 `status.ts` 共用同一套完整状态枚举（pending_recharge/processing/failed/completed/canceled + pending_payment/paid/fulfilling/partially_delivered/partially_refunded/delivered/expired/refunded），且与业务逻辑比较、API 类型联合完全一致。这是**当前生效契约**，非前端孤立残留。建议后端确认这些字符串仍是 API 实际返回值（前端无法自证） |
| C6 | guest 购买 | User `api/order.ts:59` `guestOrderAPI`；`useCheckout.ts:546` `isGuestCheckout`；`useGuestOrders.ts`、`useGuestOrderDetail.ts`；views/GuestOrders、GuestOrderDetail（classic+vault） | P2（信息项） | 未改。有意实现的完整游客下单流程：`checkoutMode==='guest'` 且需邮箱+密码+邮箱校验，配合 `guestOrderAuth` 令牌查询订单。非泄露残留；是否开放属产品决策。Admin 端无 guest 逻辑 |
| C7 | scripts sink（eval/new Function/document.write） | 全仓 | — | **无命中** |
| C8 | scripts sink（innerHTML 赋值） | Admin `TurnstileCaptcha.vue:75`；User `useLogin.ts:377`、`SecurityPanel.vue:403`、`TurnstileCaptcha.vue:75` | — | 安全。全部为 `container.innerHTML = ''` 清空容器（用于 captcha / Telegram / 2FA widget 注入前清理），不写入外部内容 |
| C9 | scripts sink（v-html 未消毒） | User `views/BlogDetail.vue:58`、`templates/vault/BlogDetail.vue:36`（post.content）；User `views/ProductDetail.vue:328`、`templates/vault/ProductDetail.vue:195`（product.content）；User `views/Legal.vue:19`、`templates/vault/Legal.vue:12`（config.legal.*）；Admin `TelegramBotBroadcastDetail.vue:149`（message_html） | **P1** | **已修复**（见 §5）。这些 v-html 源此前仅经 `processHtmlForDisplay`（只重写图片路径）或裸渲染，未过 DOMPurify |
| C10 | scripts sink（v-html 已消毒，对照） | User `AnnouncementModal.vue:29`（DOMPurify）、`useOrderDisplayHelpers.ts:95`（订单履约 instructions 严格白名单）、`reseller/ResellerSiteConfigPanel.vue:535`（DOMPurify）；Admin `SystemUpdateDialog.vue`→`renderReleaseNotes`（marked→DOMPurify + 24 例回归测试） | — | 合规。既有安全模式 |
| C11 | 前端自行汇率换算 | User `utils/money.ts`；`views/OrderDetail.vue:375-379`；Admin `PaymentChannelModal.vue` | — | **合规**。`money.ts` 明确注释 "display only, NO money calculation…不做任何换算"；`formatWalletMoney` 固定 USDT；前端仅**展示**后端 `order.exchange_rate`（带 `exchange_rate_source` + `exchange_rate_at` 溯源）；`calculateFeeCents` 是支付渠道手续费（basis points）非 FX；Admin 的 `exchange_rate` 是渠道配置录入表单。前端无任何 `价格 × 汇率` 换算 |

---

## 3. Admin 前端构建结果

工作目录：`hcz_v1/frontend/admin`。日志：`docs/_audit_admin_*.log`。

| 步骤 | 命令 | exit code | 结果 |
|---|---|---|---|
| 类型检查 | `npx vue-tsc -b --force` | **0** | PASS。~32s，无错误 |
| 单元测试 | `npm run test`（vitest run） | **0** | PASS。Test Files 3 passed，Tests 28 passed（oauthIdentity 3 / orderEmailTemplates 1 / releaseNotes 24） |
| 生产构建 | `npm run build`（vue-tsc -b && vite build） | **0** | PASS。31.07s，2960 modules transformed，`✓ built` |

构建唯一告警（非失败，P3）：`src/utils/status.ts:41` `case 'completed'` 重复——第 30 行已先命中 `completed→emerald`，第 41 行的 `case 'completed'` 为不可达死代码。不影响产物与行为，未改。

---

## 4. User 前端构建结果

工作目录：`hcz_user`。日志：`docs/_audit_user_*.log`。

| 步骤 | 命令 | exit code | 结果 |
|---|---|---|---|
| 类型检查 | `npx vue-tsc -b --force` | **0** | PASS。~21s，无错误 |
| 单元测试 | `npm run test`（node --test） | **0** | PASS。# tests 72 / # pass 72 / # fail 0 |
| 生产构建 | `npm run build`（vue-tsc -b && vite build） | **0** | PASS。22.98s，3070 modules，`✓ built` |

**classic / vault 可构建证据**（同一次 build 产物 chunk 清单中双份并存）：
- classic（`src/views/*`）：`Home-*.js`、`BlogDetail-*.js`、`ProductDetail-*.js`、`OrderDetail-*.js`、`Checkout-*.js`、`Payment-*.js`、`Legal-*.js` 等；
- vault（`src/templates/vault/*`）：同名第二组 chunk（`Home-<hash2>`、`BlogDetail-<hash2>`、`ProductDetail-<hash2>`、`VaultOrderFulfillment-*.js`、`VaultBannerHero-*.js`、vault `Legal-*.js` 等）。
- 即 classic 与 vault 两套模板均成功编译进同一产物。

---

## 5. Site Builder 前端 scripts 禁用验证

- `src/utils/customScripts.ts`：模块头部标注 `@deprecated 已停用`，明确写 "前端不执行任何来自 /public/config 的 JS…不再创建/插入任何 `<script>` 节点"。`clearCustomScripts` / `applyCustomScripts` 均为**故意空实现**。
- 全仓搜索 `applyCustomScripts`/`clearCustomScripts`/`customScripts`：**除定义文件本身外零调用**——config 下发的 scripts 字段既不被执行，也不被传入任何运行时注入器。
- Discovery blocks / Home entries 渲染组件（`src/components/home/`：HomeEntryGrid、AnnouncementBar、HomeExperience、SiteBannerStrip、HomeServiceCard）：**无 v-html、无 innerHTML、无 eval、无 script 节点注入**，一律文本插值 `{{ }}`。

结论：**Site Builder scripts 字段前端不执行，已禁用。**

---

## 6. 最小必要修复记录（P1：未消毒 v-html）

修复目标：把 §2-C9 中 6 处（User）+1 处（Admin）未消毒 v-html 用项目已有的 `dompurify` 依赖包一层，对齐既有安全模式（公告弹窗 / 订单履约 / release notes / 分销商站点预览均已消毒）。DOMPurify 默认白名单保留正常排版标签，仅剥离 `<script>`、`on*` 事件属性、`javascript:`/`data:` 可执行协议，不改变合法富文本展示。

修复前 → 修复后证据：

| 文件 | 修复前 | 修复后 |
|---|---|---|
| User `src/utils/content.ts` | 无 DOMPurify；`processHtmlForDisplay` 只重写图片路径 | 新增 `sanitizeRichHtml(html) = DOMPurify.sanitize(processHtmlForDisplay(html))`，并加注释说明 |
| User `views/BlogDetail.vue:58` | `v-html="processHtmlForDisplay(getLocalizedText(post.content))"` | `v-html="sanitizeRichHtml(getLocalizedText(post.content))"`（import 同步替换） |
| User `templates/vault/BlogDetail.vue:36` | 同上 | 同上（vault） |
| User `views/ProductDetail.vue:328` | `v-html="processHtmlForDisplay(getLocalizedText(product.content))"` | `v-html="sanitizeRichHtml(...)"`（import 同步替换） |
| User `templates/vault/ProductDetail.vue:195` | 同上 | 同上（vault） |
| User `composables/useLegal.ts` | `content` 裸返回 `config.legal.*`（Legal classic+vault 共用） | 返回值统一 `DOMPurify.sanitize(raw)`，一处覆盖双主题 Legal.vue |
| Admin `views/admin/TelegramBotBroadcastDetail.vue:149` | `v-html="broadcast.message_html"` | 新增 computed `sanitizedMessageHtml = DOMPurify.sanitize(broadcast.message_html)`，模板改用之 |

修复后复验：
- User `npm run build`：**exit 0**（18.95s，vue-tsc 通过，classic+vault chunk 齐全）；
- User `npm run test`：**exit 0**，72/72 通过（无回归）；
- Admin `npx vue-tsc -b --force`：**exit 0**（无类型错误）。

---

## 7. 问题分级清单

### P0（阻断/严重）
- 无。

### P1（已修复）
- 未消毒 v-html 富文本 sink：博客正文（classic+vault）、商品详情（classic+vault）、条款/隐私 legal（classic+vault）、Admin Telegram 广播详情。已全部包 DOMPurify 并复验全绿。
  - 残留风险说明：内容源为管理员/站长/分销商后台富文本录入，非终端用户输入；此为纵深防御加固（防止后台账号被滥用时脚本落到访客/其他管理员浏览器）。

### P2（观察/信息项，未改，需后端或产品确认）
1. 端用户钱包 `WalletAccountData.balance` 为单字段，未做 available/frozen 拆分（分销商 balance accounts 已用新模型）。需后端确认端用户钱包是否应拆分。
2. 订单旧态字符串（pending_payment/paid/fulfilling/delivered/partially_refunded/refunded）在双端统一为当前契约，需后端确认其仍是 API 实际返回值。
3. CNY 作为 Site Currency / 支付渠道法币的兜底默认值（`|| 'CNY'`），均为读后端配置后的防御兜底，非硬编码违规。
4. guest checkout 为有意实现的完整功能（邮箱+密码校验+令牌查询），是否开放属产品决策。

### P3（提示）
- Admin `src/utils/status.ts:41` `case 'completed'` 重复死代码，esbuild 构建告警（不影响产物）。

---

## 8. 附：原始日志文件
- `docs/_audit_admin_tsc.log` / `_audit_admin_test.log` / `_audit_admin_build.log`
- `docs/_audit_user_tsc.log` / `_audit_user_test.log` / `_audit_user_build.log` / `_audit_user_build2.log`（修复后）
