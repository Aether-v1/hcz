# HCZ Phase 1 用户站内通知系统 — 前端实施报告

> 范围：`frontend/user`（Vue 3 + TS + Vite，classic + vault 双模板）
> 约束：不引入 WebSocket / SSE；不做 Admin 推送 / 筛选 / 通知偏好；不修改后端代码；classic + vault 共用同一份 business logic。
> 验证：`npx vue-tsc --noEmit` → exit 0；`npm run build` → exit 0（真实执行，见末节）。

---

## 一、修改 / 新增文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `src/api/notification.ts` | 新增 | 通知 API 封装（复用 `userApi`） |
| `src/api/index.ts` | 修改 | 导出 `notificationAPI` |
| `src/stores/notification.ts` | 新增 | Pinia store：状态 + 轮询 + visibility + 登录态联动 |
| `src/views/Notifications.vue` | 新增 | 通知中心页 |
| `src/router/index.ts` | 修改 | 注册 `/notifications` 路由（`requiresUserAuth`） |
| `src/components/Navbar.vue` | 修改 | classic 顶栏加 bell + 红点（含移动端 drawer 入口） |
| `src/templates/vault/layout/VaultLayout.vue` | 修改 | vault 顶栏加 bell + 红点（含移动端 more 菜单入口） |
| `src/i18n/locales/zh-CN.json` | 修改 | 新增 `notifications.*` |
| `src/i18n/locales/zh-TW.json` | 修改 | 新增 `notifications.*` |
| `src/i18n/locales/en-US.json` | 修改 | 新增 `notifications.*` |

后端代码 0 改动。

---

## 二、API 层（严格匹配后端契约）

`src/api/notification.ts`，直接复用项目已有 `userApi`（`src/api/client.ts`，自带 `/api/v1` 前缀、Bearer 注入、`X-Lang`、401 跳登录、错误归一）：

```ts
import { userApi } from './client'

export const notificationAPI = {
    list: (page = 1, pageSize = 20) =>
        userApi.get('/notifications', { params: { page, page_size: pageSize } }),
    unreadCount: () => userApi.get('/notifications/unread-count'),
    markRead: (id: number) => userApi.post(`/notifications/${id}/read`),
    markAllRead: () => userApi.post('/notifications/read-all'),
}
```

- 后端 envelope 为 `{ status_code, msg, data }`，业务数据在 `res.data.data`，store 内按此解包。
- 路径与契约一致：`/notifications`、`/notifications/unread-count`、`/notifications/:id/read`、`/notifications/read-all`。

---

## 三、Store / Composable 设计（business logic 唯一来源）

`src/stores/notification.ts`（Pinia setup store，classic 与 vault 共用，两套模板均不复制逻辑）。

**State**
- `unreadCount: number` — 红点计数
- `notifications: NotificationItem[]` — 当前页列表
- `page / pageSize / total` — 分页
- `loadingList` — 列表加载态
- `hasMore` (computed)、`badgeText` (computed，>99 → `99+`，0 → 空串)

**Actions**
- `fetchUnreadCount()` — 静默轮询拉取（失败 catch 静默，不打扰用户）
- `fetchList(page, append)` — 拉列表；`append=true` 时追加（加载更多）
- `loadMore()` — `hasMore && !loading` 时拉下一页
- `markRead(id)` — 乐观更新：本地先置 `is_read=true`、`unreadCount-1`；API 失败回滚本地状态并 rethrow
- `markAllRead()` — 乐观更新全部已读；失败回滚
- `startPolling() / stopPolling() / reset()`

**轮询机制（60s）**
- 仅登录态启动：`watch(() => auth.isAuthenticated, …, { immediate: true })` —— 登录 → `startPolling()`（启动时立即拉一次，红点不滞后）；登出 → `reset()`（停表 + 清空）。
- `startPolling()` 内先判断 `document.visibilityState === 'hidden'`，后台不启表。
- 全局 `visibilitychange` 监听：
  - `hidden` → `stopPolling()`
  - `visible` → 立即 `fetchUnreadCount()` + `startPolling()`（切回前台数据新鲜）
- 未登录时所有动作直接 return；页面刷新带 token 时 immediate watcher 自动恢复轮询。
- 参考现有模式：`composables/useRechargeOrderDetail.ts` 的 `setInterval/clearInterval` 写法。

**为什么放 Navbar / VaultLayout 实例化**：两者都是 App 外壳级组件（`App.vue` 按模板二选一挂载），store 在其 setup 内首次创建，immediate watcher 即生效，无需额外在 `main.ts` 手动 init。

---

## 四、Navbar bell + 红点

### classic：`src/components/Navbar.vue`
- 购物车按钮旁、仅 `v-if="userAuthStore.isAuthenticated"` 显示 bell 按钮，点击跳 `/notifications`。
- 红点复用 `components/ui/badge/Badge.vue`（`variant="destructive" size="xs"`），仅 `unreadCount > 0` 渲染，文本取 `notificationStore.badgeText`（>99 显示 `99+`，0 不渲染）。
- 移动端 drawer（更多菜单）同步加入口，带未读数 Badge。

### vault：`src/templates/vault/layout/VaultLayout.vue`
- 架构事实：vault **不复用** `components/Navbar.vue`，而是自带 `VaultLayout.vue` 顶栏（`App.vue` 按模板二选一挂载）。因此 bell 在 vault 顶栏 cart 按钮旁同样加一份**纯展示**代码，但数据全部来自同一个 `useNotificationStore()` —— 逻辑零复制。
- 样式沿用 vault 圆形 icon 按钮风格（`bg-secondary` 圆形 + 右上角红色计数）。
- 移动端 more 菜单同步加入口。

两处均只做「订阅 store + 跳转」，轮询/计数/标记逻辑全部在 store。

---

## 五、通知中心页

`src/views/Notifications.vue`，路由 `/notifications`（`meta.requiresUserAuth: true`，未登录被路由守卫重定向到登录页）。vault 无同名页，`templateView('Notifications', …)` 自动回退本视图，双模板共用。

功能：
- 顶部：标题 + 副标题 + 「全部已读」按钮（`unreadCount === 0` 或请求中禁用，成功/失败 toast 反馈）。
- 列表：每条 = 类型图标（彩色底）+ title（未读加粗、已读灰化）+ body（`line-clamp-2`）+ 时间（`toLocaleString` 按当前 locale）+ 未读蓝点 / 已读 Badge。
- 点击单条 → `markRead(id)`，本地立即变已读样式（失败回滚 + toast）。
- 分页：「加载更多」按钮（`hasMore` 时显示，追加下一页）。
- 空状态：复用 `components/EmptyState.vue` + Bell 图标。
- 加载骨架屏。
- title/body 直接渲染后端文本（后端已按 locale 渲染，前端不做 type→文案翻译）。

**类型图标/颜色映射**：
| type | 图标 | 颜色 |
|---|---|---|
| wallet_recharge | Wallet | 绿（emerald） |
| order_processing | Package | 蓝（blue） |
| order_completed | CheckCircle2 | 绿（emerald） |
| refund_success | BadgeDollarSign | 绿（emerald） |
| aftersale_update | MessageSquareWarning | 橙（amber） |
| commission_confirmed | TrendingUp | 紫（purple） |
| order_canceled | XCircle | 灰（zinc） |
| 其他 / 未知 | Bell | 中性灰 |

---

## 六、i18n（三语言同步）

`notifications.*` 键：`title / subtitle / markAllRead / markAllReadSuccess / markAllReadFailed / markReadFailed / loadMore / loading / empty / read / unread`。

| key | zh-CN | zh-TW | en-US |
|---|---|---|---|
| title | 通知 | 通知 | Notifications |
| subtitle | 账户订单、钱包与售后动态都会在这里提醒你 | 帳戶訂單、錢包與售後動態都會在這裡提醒你 | Order, wallet and aftersale updates will show up here |
| markAllRead | 全部已读 | 全部已讀 | Mark all as read |
| loadMore | 加载更多 | 載入更多 | Load more |
| loading | 加载中… | 載入中… | Loading… |
| empty | 暂无通知 | 暫無通知 | No notifications |
| read | 已读 | 已讀 | Read |
| unread | 未读 | 未讀 | Unread |

三文件均通过 JSON 语法校验（`ConvertFrom-Json` OK）。

---

## 七、验证结果（真实执行）

环境：`frontend/user`，`npx vue-tsc --noEmit` / `npm run build`。

1. **`npx vue-tsc --noEmit`** → `EXITCODE=0`，0 错误。
2. **`npm run build`**（= `vue-tsc -b && vite build`）→
   - 首次构建报 1 个错误：`Notifications.vue(83,10): 'computed' is declared but its value is never read`（TS6133）。已删除未使用 import。
   - 复跑 → `✓ built in 20.65s`，`EXITCODE=0`。产物含 `dist/assets/Notifications-*.js`（5.36 kB）与 `Notifications-*.css`（0.13 kB）。

### 逻辑走查（代码审查，对应验收点）
- bell 登录后出现、未登录不出现：`Navbar.vue` 与 `VaultLayout.vue` 均 `v-if="userAuthStore.isAuthenticated"`。✅
- unread count 60s 轮询：`setInterval(..., 60_000)` + 启动即拉一次。✅
- 通知列表正常渲染：`fetchList(1)` onMounted，按 `items/total/page/page_size` 解包。✅
- mark read 后样式变化：乐观更新 `is_read` → 标题灰化 + 蓝点消失 + 已读 Badge。✅
- mark all read 后红点清零：`unreadCount = 0` → Badge `v-if > 0` 隐藏。✅
- logout 后轮询停止：watcher → `reset()` → `clearInterval` + 状态清空。✅
- 页面隐藏时轮询暂停：`visibilitychange` hidden → `stopPolling()`；visible → 立即拉取 + 恢复。✅
- 红点 >99 显示 99+、0 隐藏：`badgeText` computed + `v-if="unreadCount > 0"`。✅

### 边界说明
- 轮询期间接口失败静默（console.error 由 client 统一打印），不弹 toast，避免每 60s 打扰。
- `markRead / markAllRead` 为乐观更新 + 失败回滚，网络失败时 toast 提示。
- reseller 控制台路由（`meta.resellerConsole`）不挂 Navbar/VaultLayout，故控制台内无 bell —— 与现有购物车等入口行为一致，符合「用户站通知」定位。
- 未做：下拉预览、toast 实时弹出、通知跳转业务详情页（`data` 字段暂仅随 body 展示，Phase 1 按要求不做复杂渲染）。
