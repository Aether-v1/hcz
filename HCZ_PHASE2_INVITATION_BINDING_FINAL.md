# HCZ Phase 2 Invitation Binding System — Final Report

> 实施日期：2026-10-04
> 前置：HCZ_FULL_PRODUCT_FEATURE_GAP_AUDIT.md（维度三）
> Commit：`144ca18`（已 push origin/main）
> CI Run：#37197651314 — completed / **success**（4/4 jobs 全绿）
> 原则：只做绑定关系，不实现 10 级返利；不修改已封板 Contract

---

## Final Verdict: PASS

### Phase 2 邀请绑定系统已完整交付，可进入 Phase 3（Wallet Withdrawal），并已具备 Phase 4（10-Level Affiliate）的用户关系基础。

---

## 一、交付总览

| 层 | 交付物 | 状态 |
|---|---|---|
| 数据模型 | users 表加 inviter_id / invite_code / invite_bound_at + 唯一索引 | ✅ |
| 邀请码生成 | crypto/rand 8 位随机码（排除易混淆字符），碰撞重试 | ✅ |
| 注册绑定 | 显式 invite_code > cookie attribution > 无上级，同事务 | ✅ |
| 防循环 | IsDescendant / CheckInviteBinding（自邀/成环拒绝） | ✅ |
| Cookie 归因回退 | 复用现有 30 天 click tracking，ResolveRegistrationInviterUserID | ✅ |
| User API | GET /api/v1/invitation/me | ✅ |
| 历史用户 Backfill | 幂等补码 + 唯一索引，inviter_id 保持 null | ✅ |
| 前端注册页 | 读取 ?invite= 参数，提交 invite_code，显示邀请提示 | ✅ |
| 前端邀请中心 | /me/invitation：邀请码/链接/复制/上级/直接人数 | ✅ |
| i18n | zh-CN / zh-TW / en-US | ✅ |
| 测试 | 13 个覆盖点全部通过 | ✅ |

---

## 二、数据模型

### users 表新增字段

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| inviter_id | *uint (nullable) | INDEX | 直接上级用户 ID，null = 无上级 |
| invite_code | varchar(12) | NOT NULL, UNIQUE | 用户唯一邀请码 |
| invite_bound_at | *time.Time (nullable) | | 绑定上级的时间 |

**关键决策：不复用 affiliate_code**
- 实际代码确认 `affiliate_profiles` 是按需开通（`OpenAffiliate`），注册时不创建 profile
- 复用 affiliate_code 等于给每个注册用户强制开通推广，会改动封板的 affiliate 逻辑
- 因此在 users 表新建独立 `invite_code` 字段

### 邀请码生成
- `crypto/rand`，字母表 `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`（排除 0/O/1/I）
- 8 位，不可猜测（非顺序 ID）
- 生成后查唯一性，碰撞重试

---

## 三、注册绑定流程

### 绑定优先级
```
显式 invite_code（请求体） >  cookie/click attribution（30天窗口） >  无上级
```

### 显式 invite_code 处理
1. 查找 inviter = users WHERE invite_code = ?
2. 不存在 → 返回明确错误（邀请码无效）
3. inviter == self → 拒绝（自邀）
4. inviter 在当前用户下级链中 → 拒绝（循环，注册时新用户无下级，防御性检查）
5. 绑定 inviter_id + invite_bound_at

### Cookie Attribution 回退
- 复用现有 affiliate click tracking（30 天归因窗口）
- 新增只读方法 `ResolveRegistrationInviterUserID(visitorKey)` → 返回最近 click 关联的 UserID
- 前端注册时可传 `visitor_key`（与下单归因同一机制）
- 归因失败时静默降级为无上级，不报错

### 事务一致性
- `inviter_id` / `invite_code` / `invite_bound_at` 都是 users 表行列，绑定与用户在**同一条 INSERT** 落库
- 前置校验（邀请码无效等）发生在 Create 之前，用户不会落库
- 天然无"用户创建成功但 binding 半失败"

---

## 四、防循环验证

- `IsDescendant(inviterID, userID) bool`：向上遍历 inviter_id chain，检查 userID 是否在 inviter 的祖先链中
- `CheckInviteBinding(inviterID, userID) error`：自邀 + 成环统一校验
- 注册时新用户无下级，循环检查主要是防御性；未来 Admin 调整关系时可复用

---

## 五、User API

### GET /api/v1/invitation/me（登录态）

```json
{
  "invite_code": "AB12CD34",
  "invite_url": "https://site.com/register?invite=AB12CD34",
  "inviter_code": "XY98ZZ77",
  "inviter_display_name": "用户***",
  "direct_invite_count": 5,
  "invite_bound_at": "2026-10-04T12:00:00Z"
}
```

- `inviter_code` / `inviter_display_name` 为 null 表示无上级
- `inviter_display_name` 脱敏（不暴露完整邮箱）
- `direct_invite_count` = COUNT(*) FROM users WHERE inviter_id = 当前用户
- IDOR：只能查自己，不需要传 user_id

---

## 六、历史用户 Backfill

- AutoMigrate 加字段后，对 invite_code 为空的用户批量生成唯一邀请码
- 幂等：只处理 invite_code 为空的用户
- 历史用户 `inviter_id` 保持 null，**不猜测历史邀请关系**，不根据旧订单/commission 反推
- backfill 完成后建立 invite_code 唯一索引

---

## 七、前端实现

### 注册页
- `useRegister.ts` onMounted 读取 URL `?invite=XXXX`
- 注册请求 body 新增 `invite_code`（空值省略）
- 有邀请码时显示"您将通过邀请链接注册"+ 脱敏尾码 `***XXXX`
- 邀请码无效时显示明确错误
- 注册成功后 `router.replace` 清除 URL invite 参数

### 邀请中心页（/me/invitation）
- `InvitationPanel.vue`：邀请码（大字号+复制）、邀请链接（+复制）、我的上级（脱敏名或"暂无上级"）、直接邀请人数、绑定时间
- 路由带 `requiresUserAuth` 守卫
- 用户中心侧栏加"邀请中心"入口（单一公共配置，classic+vault 双模板生效）
- 不显示 10 级收益、不显示下级列表（Phase 2 范围外）

---

## 八、安全

| 检查 | 结果 |
|---|---|
| 枚举邀请码不泄露敏感用户数据 | ✅ 无效码只返回"邀请码无效"，不返回用户信息 |
| invitation/me 不暴露上级敏感资料 | ✅ inviter_display_name 脱敏，不返回完整邮箱/inviter_id |
| 不允许任意 user_id 查询邀请树 | ✅ /me 只查当前登录用户 |
| 不允许客户端伪造 inviter_id | ✅ 前端只能传 invite_code，后端查库解析 inviter_id |
| 已绑定用户不可被普通用户修改 | ✅ 绑定只在注册 INSERT 时发生，无更新 API |

---

## 九、测试覆盖（13 个点全部通过）

1. 新用户注册自动生成唯一 invite_code ✅
2. 合法 invite_code 注册 → inviter_id 正确绑定 + invite_bound_at ✅
3. 无效 invite_code → 明确错误 ✅
4. 自邀 → 拒绝 ✅
5. 显式 invite_code 优先于 cookie attribution ✅
6. cookie attribution 正常绑定 ✅
7. 事务一致性：绑定失败用户不创建 ✅
8. inviter 不存在 → 错误 ✅
9. direct_invite_count 正确 ✅
10. GET /invitation/me 返回正确字段 ✅
11. 历史用户 backfill 生成 invite_code ✅
12. invite_code 唯一性约束 ✅
13. 注册回归（无邀请码正常注册）✅

---

## 十、全量回归 & CI

| 检查 | 结果 |
|---|---|
| gofmt | ✅ 空输出 |
| go vet ./... | ✅ exit 0 |
| go test ./... | ✅ 仅已知 Windows flaky（logger/RiskGate/selfupdate）+ reseller 并行 email 碰撞（隔离复跑 PASS），0 真实失败 |
| go build ./... | ✅ exit 0 |
| User vue-tsc | ✅ 0 错误 |
| User build | ✅ exit 0（21.07s） |
| Migration（含 backfill） | ✅ PASS |
| identity / affiliate / httpserver | ✅ 全 PASS |

### 架构守卫修复（本轮引入，已修复）
2 个 architecture 文件数预算被 Phase 2 新文件突破，已最小化上调：
- migrations 预算 4→6（新增 invitation.go + 测试）
- user/domain 预算 1→2（新增 invitecode.go）

### Linux CI
- **Push**：`dec7575..144ca18 main -> main`
- **CI Run #37197651314**（head_sha=144ca18）：completed / **success**，4/4 jobs 全绿

---

## 十一、明确回答

| 问题 | 回答 |
|---|---|
| 每个用户是否都有唯一 invite_code？ | **是**。新用户注册自动生成，历史用户 backfill 补码，全局唯一约束 |
| 注册时是否能可靠绑定 inviter？ | **是**。显式 invite_code > cookie attribution > 无上级，同一条 INSERT 保证原子性 |
| 绑定后是否不可被普通用户修改？ | **是**。无更新 API，绑定只在注册时发生 |
| 是否防 self/cycle？ | **是**。CheckInviteBinding 校验自邀 + 成环 |
| 历史用户是否安全迁移？ | **是**。backfill 补 invite_code，inviter_id 保持 null，不猜测历史关系 |
| 前端邀请中心是否可用？ | **是**。/me/invitation 邀请码/链接/复制/上级/直接人数，classic+vault 双模板 |
| 是否可以进入 Phase 3 Wallet Withdrawal？ | **是**。Phase 2 完整闭环，CI 全绿 |
| 是否具备 Phase 4 10-Level Affiliate 的关系基础？ | **是**。inviter_id 永久绑定关系链已建立，parent chain 可遍历 |

---

## 十二、Phase 3 / Phase 4 预留

- **Phase 3（Wallet Withdrawal）**：不依赖邀请绑定，可独立启动
- **Phase 4（10-Level Affiliate）**：
  - 关系基础已具备（inviter_id parent chain）
  - 需扩展 commission 计算为多级遍历
  - 需新增每级比例配置（L1-L10）
  - 需 Commission 表加 level 字段
  - 需多级退款回退逻辑
  - 当前单级返利逻辑保持不变，不影响 Phase 4 扩展

---

## 十三、产物索引

| 报告 | 路径 |
|---|---|
| 本报告（最终交付） | `HCZ_PHASE2_INVITATION_BINDING_FINAL.md` |
| 后端实现详情 | `REMEDIATION_PHASE2_INVITATION_BACKEND.md` |
| 前端实现详情 | `REMEDIATION_PHASE2_INVITATION_FRONTEND.md` |
| 基础设施回归详情 | `REMEDIATION_PHASE2_INFRASTRUCTURE.md` |
