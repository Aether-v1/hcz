# Remediation — Phase 2 邀请绑定系统（前端）

- 项目根：`E:\Users\orang\Downloads\Compressed\hcz_v1\frontend\user`
- 范围：注册页邀请码处理 + 邀请中心页 + API 封装 + 用户中心入口 + i18n
- 严格约束：未实现 10 级收益显示；未修改后端代码；classic + vault 共用逻辑走 composable / 共享 Panel

---

## 1. 修改 / 新增文件清单

### 新增
| 文件 | 作用 |
|---|---|
| `src/api/invitation.ts` | 邀请 API：`invitationAPI.me()` → `GET /api/v1/invitation/me`，导出 `MyInvitationData` 类型 |
| `src/views/personal/InvitationPanel.vue` | 邀请中心 Panel（classic + vault 共用，由 PersonalCenter 壳渲染） |

### 修改
| 文件 | 改动 |
|---|---|
| `src/api/index.ts` | 导出 `invitationAPI` 与 `MyInvitationData` 类型 |
| `src/composables/useRegister.ts` | 读取 URL `?invite=`、注册请求携带 `invite_code`、注册成功清 URL 参数、暴露 `inviteCode` / `inviteCodeTail` |
| `src/views/auth/Register.vue` | 解构 `inviteCode` / `inviteCodeTail`，加邀请提示横幅，导入 `Gift` 图标 |
| `src/composables/usePersonalCenter.ts` | `PersonalSection` 联合类型加 `invitation`；新增侧栏导航项 + 路由映射；导入 `Share2` |
| `src/router/index.ts` | 新增 `/me/invitation` 路由（`requiresUserAuth` 登录态守卫，props section=`invitation`） |
| `src/views/PersonalCenter.vue` | 导入并按 `currentSection === 'invitation'` 渲染 `InvitationPanel` |
| `src/i18n/locales/zh-CN.json` | 新增邀请中心相关文案 |
| `src/i18n/locales/zh-TW.json` | 同上（繁体） |
| `src/i18n/locales/en-US.json` | 同上（英文） |

---

## 2. 注册页改动

实际搜索后确认注册页为 `src/views/auth/Register.vue`，其逻辑抽离在 `src/composables/useRegister.ts`（classic + vault 双模板共用，模板经 `templates/registry` 选择）。

### 读取 URL 参数
- `onMounted` 中读取 `route.query.invite`（兼容数组形式），`trim()` 后存入 `inviteCode`。
- 新增 `normalizedInviteCode`（trim 后）与 `inviteCodeTail`（仅取后 4 位，用于展示脱敏）。

### 随注册请求提交
- `performRegister` 的 `userAuthStore.register({...})` payload 新增：
  ```ts
  invite_code: normalizedInviteCode.value || undefined
  ```
  无邀请码时为 `undefined`，序列化后不带该字段（可选字段，符合 contract）。

### 提示 UI
- `Register.vue` 在标题下方、表单上方新增 info 横幅：
  - 仅当 `inviteCode` 存在时显示；
  - 文案 `auth.register.inviteHint`（“您将通过邀请链接注册”）；
  - 后接脱敏尾码 `***XXXX`（仅后 4 位），不暴露完整邀请码 / 上级信息。

### 邀请码无效提示
- 注册失败时，若当前携带了邀请码且后端错误信息命中 invite/邀请 关键字，回落使用 `auth.register.errors.invalidInviteCode`（“邀请码无效，请检查后重试”）；其余错误沿用后端 `data.msg`（client 层已本地化透传）。

### 注册成功清除 URL invite 参数
- 注册成功后先 `router.replace({ query: 去掉 invite })` 清理当前 query，再 `router.push('/me/orders')`，避免刷新 / 回退重复携带 `?invite=`。

---

## 3. 邀请中心页

路由：`/me/invitation`，`meta.requiresUserAuth: true`（复用全局路由守卫，未登录跳 `/auth/login?redirect=...`）。

组件 `src/views/personal/InvitationPanel.vue`，由 `PersonalCenter.vue` 壳在 `section === 'invitation'` 时渲染（与 wallet / affiliate 等 Panel 同一模式，classic + vault 共用）。

页面内容（严格按 contract，不含 10 级收益、不含下级列表）：
- **我的邀请码**：大字号等宽显示 `invite_code` + 复制按钮。
- **邀请链接**：显示 `invite_url`（原样使用后端返回值，前端不拼接，保证与后端一致）+ 复制按钮。
- **我的上级**：有 `inviter_display_name` 则显示（脱敏展示名）；为 `null` 显示“暂无上级”。不显示 `inviter_code`。
- **直接邀请人数**：`direct_invite_count`。
- **绑定时间**：`invite_bound_at`（`null` 显示 `—`）。

交互 / 状态：
- 加载中：骨架屏 pulse。
- 加载失败：destructive Alert + “重新加载”按钮。
- 复制：复用 `src/utils/clipboard.ts` 的 `copyText`；复制后按钮短暂变为“已复制”（1.6s），失败静默。

---

## 4. API 封装

`src/api/invitation.ts`（参考 `api/wallet.ts` / `api/affiliate.ts` 的 `userApi` 用法）：

```ts
export interface MyInvitationData {
    invite_code: string
    invite_url: string
    inviter_code: string | null
    inviter_display_name: string | null
    direct_invite_count: number
    invite_bound_at: string | null
}

export const invitationAPI = {
    me: () => userApi.get('/invitation/me'),
}
```

- 实际请求：`GET {VITE_API_BASE_URL}/api/v1/invitation/me`（client 自动带 `Authorization: Bearer <token>` 与 `X-Lang`）。
- 在 `src/api/index.ts` 追加导出：`export { invitationAPI, type MyInvitationData } from './invitation'`。
- Panel 内消费：`invitationAPI.me()` → `response.data.data as MyInvitationData`（与现有 Panel 取数方式一致）。

---

## 5. 用户中心入口

`src/composables/usePersonalCenter.ts` 是 classic + vault 个人中心壳的**公共导航配置**（唯一来源），在其中加入：
- `PersonalSection` 联合类型追加 `'invitation'`；
- `sectionItems` 追加 `{ key: 'invitation', label: 'personalCenter.tabs.invitation', icon: Share2 }`（排在“推广返利”之后）；
- `sectionRouteMap` 追加 `invitation: '/me/invitation'`。

因导航项集中在该 composable，classic 与 vault 两套模板**同时生效**，无需重复改两处。`router/index.ts` 新增对应路由；`PersonalCenter.vue` 增加 `InvitationPanel` 渲染分支。

---

## 6. i18n（zh-CN / zh-TW / en-US）

新增 key：

| key | zh-CN | zh-TW | en-US |
|---|---|---|---|
| `personalCenter.tabs.invitation` | 邀请中心 | 邀請中心 | Invitation |
| `personalCenter.invitation.title` | 邀请中心 | 邀請中心 | Invitation Center |
| `personalCenter.invitation.subtitle` | 分享你的邀请码或邀请链接… | 分享你的邀請碼或邀請連結… | Share your invite code or link… |
| `personalCenter.invitation.myCode` | 我的邀请码 | 我的邀請碼 | My Invite Code |
| `personalCenter.invitation.inviteUrl` | 邀请链接 | 邀請連結 | Invite Link |
| `personalCenter.invitation.myInviter` | 我的上级 | 我的上級 | My Inviter |
| `personalCenter.invitation.noInviter` | 暂无上级 | 暫無上級 | No inviter yet |
| `personalCenter.invitation.directCount` | 直接邀请人数 | 直接邀請人數 | Direct Invites |
| `personalCenter.invitation.boundAt` | 绑定时间 | 綁定時間 | Bound At |
| `personalCenter.invitation.copy` | 复制 | 複製 | Copy |
| `personalCenter.invitation.copied` | 已复制 | 已複製 | Copied |
| `personalCenter.invitation.loadFailed` | 邀请信息加载失败… | 邀請資訊載入失敗… | Failed to load invitation info… |
| `personalCenter.common.loadRetry` | 重新加载 | 重新載入 | Retry |
| `auth.register.inviteHint` | 您将通过邀请链接注册 | 您將透過邀請連結註冊 | You are registering via an invitation link |
| `auth.register.errors.invalidInviteCode` | 邀请码无效，请检查后重试 | 邀請碼無效，請檢查後重試 | Invalid invite code, please check and try again |

三个 JSON 均已通过 `ConvertFrom-Json` 解析校验。

---

## 7. 验证结果

| 检查 | 命令 | 结果 |
|---|---|---|
| 类型检查 | `npx vue-tsc --noEmit` | exit 0，0 错误 |
| 生产构建 | `npm run build`（= `vue-tsc -b && vite build`） | exit 0，`✓ built in 19.51s`，2997 modules transformed |
| i18n JSON | `ConvertFrom-Json` 三文件 | zh-CN / zh-TW / en-US 均 OK |

### 代码走查
- ✅ 注册页读取 `?invite=` 参数（`useRegister.ts` onMounted，兼容数组 query）。
- ✅ 注册请求携带 `invite_code`（payload 新增，空值省略）。
- ✅ 邀请中心页正常渲染（`InvitationPanel.vue`，loading / error / success 三态）。
- ✅ 复制按钮功能（`copyText`，复制后“已复制”反馈）。
- ✅ 登录态守卫（`/me/invitation` 路由 `meta.requiresUserAuth: true`，复用全局 beforeEach）。
- ✅ 邀请链接原样使用后端 `invite_url`，前端不拼接，与后端一致。
- ✅ 不暴露上级敏感信息：仅显示 `inviter_display_name`，不展示 `inviter_code`。
- ✅ 未实现 10 级收益 / 下级列表（Phase 2 范围外）。
- ✅ 未修改后端代码。

---

## 8. 待后端联调点（仅提示，不阻塞）
- 后端 `POST /api/v1/auth/register` 需接受可选 `invite_code`，无效时返回非 0 `status_code` + 本地可识别 msg。
- `GET /api/v1/invitation/me` 返回结构需与 `MyInvitationData` 完全一致（`inviter_code` / `inviter_display_name` / `invite_bound_at` 无上级时为 `null`）。
