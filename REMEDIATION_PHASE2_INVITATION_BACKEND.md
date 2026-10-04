# REMEDIATION PHASE 2 — 邀请绑定系统（Invitation Binding）后端

> 范围：仅实现"用户级永久绑定关系"。**不实现 10 级返利**，不改动已封板的 P0/P1 Contract（Wallet-Only / Global Rate+USDT / 五态机 / After-Sale / Notification），不改写现有 affiliate 单级返利逻辑。

---

## 1. 修改文件清单

### 新增文件
| 文件 | 作用 |
|---|---|
| `internal/modules/identity/user/domain/invitecode.go` | 邀请码生成器（crypto/rand，排除 0/O/1/I）+ 归一化 |
| `internal/modules/identity/userauth/application/invite.go` | `IsDescendant` / `CheckInviteBinding` 防循环、邀请码唯一生成、`resolveRegistrationInviter` 绑定解析 |
| `internal/modules/identity/invitation/application/service.go` | `GET /invitation/me` 查询服务（只读） |
| `internal/modules/identity/invitation/transport/http/handler.go` | 登录态 handler |
| `internal/modules/identity/invitation/transport/http/routes.go` | `/invitation/me` 路由装配 |
| `internal/bootstrap/database/migrations/invitation.go` | 历史用户 `invite_code` backfill + 唯一索引创建 |
| `internal/modules/identity/userauth/integrationtest/invitation_bind_test.go` | 注册绑定集成测试 |
| `internal/modules/identity/invitation/application/service_test.go` | `/me` 返回字段测试 |
| `internal/bootstrap/database/migrations/invitation_test.go` | backfill + 唯一约束测试 |

### 修改文件
| 文件 | 改动 |
|---|---|
| `internal/modules/identity/user/domain/user.go` | 新增 `InviterID *uint`、`InviteCode string`、`InviteBoundAt *time.Time` |
| `internal/modules/identity/user/contract/store.go` | 新增 `GetByInviteCode`、`CountDirectInvitees` |
| `internal/modules/identity/user/infrastructure/gormstore/store.go` | 实现上述两个方法 |
| `internal/modules/identity/userauth/application/errors.go` | 新增 `ErrInviteCodeInvalid` / `ErrSelfInvite` / `ErrInviteCycle` |
| `internal/modules/identity/userauth/application/ports.go` | 新增 `AffiliateAttributor` 端口 |
| `internal/modules/identity/userauth/application/service.go` | `Register` 改为 `RegisterInput`；绑定字段在 `Create` 前写入用户结构体；注入 attributor setter |
| `internal/modules/identity/userauth/transport/http/user_login_handler.go` | 请求体加 `invite_code`/`visitor_key`；接口与错误映射更新 |
| `internal/bootstrap/userauth/adapters.go` | transport adapter `Register` 改 `RegisterInput` 透传 |
| `internal/modules/affiliate/application/attribution.go` | **只读新增** `ResolveRegistrationInviterUserID`（复用现有 click 归因，不改佣金） |
| `internal/app/container/services_foundation.go` | `c.UserAuthService.SetAffiliateAttributor(c.AffiliateService)` |
| `internal/app/httpserver/routes_storefront.go` | 在登录态 `user` 组装配 `GET /invitation/me` |
| `internal/bootstrap/database/migrations/registry.go` | `AutoMigrate` 末尾调用 `BackfillInviteCodes(db)` |
| `internal/modules/identity/userauth/integrationtest/domain_policy_test.go` | 2 处旧 `Register` 调用点改为 `RegisterInput` |
| `internal/app/httpserver/p1_2_auth_rate_limit_test.go` | fake `p1LoginAuth` 改新签名 |

> 说明：工作区中 `frontend/user/...` 的改动为前端并行开发的既有未提交改动，本轮后端未触碰。

---

## 2. 数据模型决策：不复用 affiliate_code

**结论：新建 `users.invite_code` 字段，不复用 `affiliate_profiles.affiliate_code`。**

实际代码验证（`internal/modules/affiliate/application/profile.go`）：
- `OpenAffiliate(userID)` 是**按需开通**——只有用户主动开通推广时才创建 `affiliate_profiles` 行。
- 注册时并不存在 `affiliate_profile`，因此新注册用户没有 `affiliate_code`。
- 若复用 `affiliate_code` 作为用户邀请码，等于在注册时给每个用户强制开通推广档案，会改动已封板的 affiliate 语义，违反"不修改现有 affiliate 单级返利逻辑"。
- 二者语义不同：`affiliate_code` 是推广渠道码，`invite_code` 是个人增长邀请码。

**新字段（`internal/modules/identity/user/domain/user.go`）：**
```go
InviterID     *uint      `gorm:"index"`                                  // 直接上级，可空
InviteCode    string     `gorm:"type:varchar(12);not null;default:''"`   // 全局唯一（索引由 migration 建）
InviteBoundAt *time.Time `gorm:"index"`                                  // 绑定时间
```
- `inviter_id` 可空（无上级），单值即保证"一个用户最多一个直接上级"。
- `invite_code` NOT NULL，但**不打 `uniqueIndex` tag**——因为历史行默认为空串，AutoMigrate 直接建唯一索引会因多行空串冲突失败；唯一索引改为 backfill 完成后由 migration 手动创建（见第 6 节）。

---

## 3. 注册绑定流程（同事务一致性）

`Register` 签名改为 `RegisterInput{ Email, Password, Code, AgreementAccepted, EmailVerificationEnabled, InviteCode, VisitorKey }`。

绑定优先级：**显式 `invite_code` > cookie/click 归因 > 无上级**。

执行顺序（`internal/modules/identity/userauth/application/service.go`）：
1. 既有校验：协议、邮箱、密码强度、邮箱已存在、邮箱验证码。
2. `resolveRegistrationInviter(invite_code, visitor_key)`：
   - 显式码非空 → `GetByInviteCode`；查不到 → **`ErrInviteCodeInvalid`（硬失败，此时尚未写库）**。
   - 显式码有效 → 返回该 inviter.ID（短路，不再查 cookie）。
3. `generateUniqueInviteCode()`：为新用户生成全局唯一码（碰撞重试 16 次）。
4. 构造 `User{... InviteCode: newCode, InviterID: &inviterID, InviteBoundAt: &now}`。
5. `userRepo.Create(user)` —— **单条 INSERT**。

**事务一致性硬要求的实现方式**：`inviter_id` / `invite_code` / `invite_bound_at` 都是 `users` 行的列，绑定关系与用户在**同一条 INSERT 语句**中落库，不存在"用户创建成功但绑定半失败"的中间态。任何前置失败（邀请码无效、码生成失败）都发生在 `Create` 之前，用户不会落库（测试 `TestRegisterWithInvalidInviteCodeFailsAndNoUserCreated` 用 `countUsers` 断言）。

> 安全约束：前端只能提交 `invite_code`，**不能提交 `inviter_id`**；后端通过码反查 inviter，杜绝越权指定上级。

---

## 4. Cookie / Click 归因回退

实际代码审计：本项目的"cookie 归因"并非后端种 HTTP cookie，而是前端把 `visitor_key` 在请求体里上报（与下单 `affiliate_visitor_key` 同机制）。因此注册请求体新增可选 `visitor_key`。

回退逻辑（`resolveRegistrationInviter`）：
- 仅当**无显式邀请码**时触发。
- 调用 `AffiliateAttributor.ResolveRegistrationInviterUserID(visitor_key)`（新增于 affiliate Service，只读），内部复用 `GetLatestActiveProfileByVisitorKey(visitor_key, now-30d)` → `profile.UserID`，并复用 `setting.Enabled` 开关。
- 解析到的 inviter 再经 `userRepo.GetByID` 二次确认存在且未删除。
- **任何异常 / 无归因 → 静默降级为无上级（不报错、不阻断注册）**。

该方法对 affiliate 是纯新增只读方法，**不触碰 commission / 下单快照 / 退款回退**。

---

## 5. 防循环方案

`internal/modules/identity/userauth/application/invite.go`：
- `IsDescendant(inviterID, candidate, finder)`：从 `inviterID` 沿 `inviter_id` 向上遍历（深度上限 64，防脏数据死循环），若经过 `candidate` 则说明 `candidate` 是 `inviterID` 的祖先——把 `candidate` 挂到 `inviterID` 下会成环。
- `CheckInviteBinding(inviterID, newUserID, finder)`：
  - `inviterID == 0` → 无上级，合法；
  - `inviterID == newUserID` → `ErrSelfInvite`；
  - `IsDescendant` 为真 → `ErrInviteCycle`。

注册时新用户尚未落库（`newUserID=0`），自邀/成环不会触发——这与审计结论一致（注册时新用户无下级）。该校验主要为未来 Admin 调整上下级关系预留，已被单元测试直接覆盖。

---

## 6. API 契约

### 注册请求（POST 注册，公开态）
请求体新增可选字段：
```json
{ "email": "...", "password": "...", "code": "...", "agreement_accepted": true,
  "invite_code": "ABCDEFGH", "visitor_key": "..." }
```
- `invite_code` 无效 → `400 error.invite_code_invalid`。

### GET /api/v1/invitation/me（登录态）
挂载在 `routes_storefront.go` 的 `user` JWT 鉴权组。IDOR 安全：仅用 JWT 中的 `userID` 查自己，不接受外部 `user_id`。

响应：
```json
{
  "invite_code": "K7Q2P9XR",
  "invite_url": "https://site/register?invite=K7Q2P9XR",
  "inviter_code": "BOSSCODE",
  "inviter_display_name": "BossName",
  "direct_invite_count": 3,
  "invite_bound_at": "2026-10-04T10:00:00Z"
}
```
- `inviter_code` / `inviter_display_name` / `invite_bound_at` 为 `null` 表示无上级。
- `inviter_display_name` 取昵称（缺省邮箱前缀），**不暴露完整邮箱**。
- `direct_invite_count` = `COUNT(*) WHERE inviter_id = 当前用户 AND deleted_at IS NULL`。
- `invite_url` = `brand.site_url`（后台配置，去尾斜杠）+ `/register?invite=` + code；未配置站点根地址时回退为相对路径 `/register?invite=...`。

---

## 7. 历史用户 Backfill 策略

`internal/bootstrap/database/migrations/invitation.go` 的 `BackfillInviteCodes(db)`（在 `AutoMigrate` 末尾调用）：
1. 防御性检查 `invite_code` 列存在。
2. 选出所有 `invite_code IS NULL OR = ''` 的用户。
3. 逐个生成全局唯一码（`uniqueInviteCode` 查库去重，碰撞重试），`Update` 写回。**已有码用户不动**（幂等）。
4. backfill 完成后创建**部分唯一索引** `CREATE UNIQUE INDEX uni_users_invite_code ON users(invite_code) WHERE deleted_at IS NULL`。

**历史用户 `inviter_id` 保持 NULL**——不根据旧订单 / commission 反推历史邀请关系（明确按要求不猜测）。

幂等性：只处理空码行；重复执行无副作用（测试覆盖二次运行）。

---

## 8. 测试结果

新增/修改测试全部通过：

**注册绑定（`userauth/integrationtest/invitation_bind_test.go`）**
1. ✅ `TestRegisterGeneratesUniqueInviteCode` — 自动生成、全局唯一、长度 8
2. ✅ `TestRegisterWithValidInviteCodeBindsInviter` — inviter_id 绑定 + invite_bound_at 持久化
3/9. ✅ `TestRegisterWithInvalidInviteCodeFailsAndNoUserCreated` — 无效码错误 + 用户不落库
4. ✅ `TestCheckInviteBindingSelfAndCycle` — 自邀 `ErrSelfInvite`、成环 `ErrInviteCycle`、`IsDescendant` 方向
5. ✅ 已绑定关系不可变（注册只一次；`TestBindingAndUserCreationAtomicOnSuccess` 验证落库一致性）
6. ✅ `TestExplicitInviteCodeBeatsCookieAttribution` — 显式码短路（attributor 未被调用即证明优先级）
7. ✅ `TestCookieAttributionBindsWhenNoExplicitCode` — cookie 归因正常绑定
- ✅ `TestCookieAttributionFailureSilentFallback` — 归因异常静默降级无上级
8. ✅ `TestBindingAndUserCreationAtomicOnSuccess` — 绑定与用户同一条 INSERT（原子）
10. ✅ `TestDirectInviteCount` — 直接下级计数正确

**`/invitation/me`（`invitation/application/service_test.go`）**
11. ✅ `TestGetMyInvitationFields` — 全部字段、上级/无上级两种分支、不暴露邮箱、invite_url 拼接
- ✅ `TestGetMyInvitationDirectCount` — direct_invite_count

**Backfill / 唯一约束（`migrations/invitation_test.go`）**
12. ✅ `TestBackfillInviteCodes` — 历史用户补码、唯一、inviter_id 保持 null、已有码不被改写、幂等
13. ✅ `TestInviteCodeUniqueIndexEnforced` — 唯一索引建成后重复码插入失败

---

## 9. 验证结果

| 命令 | 结果 |
|---|---|
| `go build ./...` | **exit 0** |
| `go test ./internal/modules/identity/...` | **ok**（含 userauth 注册回归、invitation、user gormstore） |
| `go test ./internal/modules/affiliate/...` | **ok**（单级返利未破坏） |
| `go test ./internal/app/httpserver/...` | **ok**（含 rate-limit 注册回归） |
| `go test ./internal/bootstrap/database/migrations/...` | **ok** |

---

## 10. 边界与未做项

- **不实现 10 级返利**：本阶段只落 `inviter_id` 直接上下级关系，不做 parent_chain 遍历、不分 L1-L10 比例、不改 commission。
- **不建 invitation_record 表**（审计标 P1，本轮按"只做绑定关系"裁剪）。
- **不做注册后补绑定 / 换上级 / 解绑**（审计标 DROP/P2）。
- cookie 归因依赖前端上报 `visitor_key`；后端不种 cookie，与既有下单归因机制保持一致。
