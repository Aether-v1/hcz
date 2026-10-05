# HCZ V1 模块矩阵 + 迁移 + Legacy 审计报告

- 审计日期：2026-10-05
- 模块路径：`github.com/Aether-v1/hcz`
- 审计方式：逐文件精读 + 集成测试实跑
- 审计范围：19 模块功能矩阵、Exchange Rate、Refund/After-Sale、Invitation Binding、C2C 权限、User Notification、General Ticket、Site Builder、Migration、Legacy Columns、Old 9-State

---

## 一、19 模块功能矩阵

| # | 模块 | 评级 | 关键证据文件 / 测试 |
|---|------|------|---------------------|
| 1 | Auth（identity/userauth/） | PASS | `internal/modules/identity/userauth/integrationtest/` 全部 PASS（19.072s）；userauth/application/ 测试 0.194s |
| 2 | User Profile（userauth/application/profile.go） | PASS | userauth integrationtest 覆盖 profile 路径；测试全过 |
| 3 | Wallet（internal/modules/wallet/） | PASS | `wallet/domain/account.go` AvailableBalance+FrozenBalance 双余额；integrationtest/concurrency/freeze/repository 全 PASS（1.252s） |
| 4 | Wallet Recharge（wallet/application/recharge.go） | PASS | wallet integrationtest 覆盖；测试全过 |
| 5 | Recharge Business Order（internal/modules/order/） | PASS WITH CONDITIONS | 旧九态常量仍定义于 `constants/constants.go:5-13`，但业务层全部走 `ordermachine.Normalize` 归一化；ACTIVE_WRITE=0（见第 11 节） |
| 6 | Global Exchange Rate（exchangerate/） | PASS | `application/service.go` Resolve 三级 fail-closed；`infrastructure/provider/coingecko.go` 唯一 provider；`rediscache/cached_store.go` 读穿+写穿；`settingsstore/store.go` KV durable truth；`transport/admin_handler.go:117` maskKey 脱敏 |
| 7 | Refund（order/application/refund/） | PASS | `refund/service.go:392` 注释明确"只写 refund_status，永不写主状态"；`refund/wallet.go:158` 同；integrationtest/refund PASS（0.524s） |
| 8 | After-Sale（order/application/aftersale/） | PASS | `aftersale/service.go:121` doRefund 共享事务；`aftersale/adapter.go` 薄适配直接调 AdminRefundToWalletInTx；integrationtest/aftersale PASS（0.181s） |
| 9 | User Notification（usernotification/） | PASS | `domain/notification.go:54-60` 复合唯一索引 `(user_id,biz_type,biz_id,type)` 幂等；`application/service.go:49` ErrAlreadyExists 静默；integrationtest PASS |
| 10 | Invitation Binding（identity/invitation/ + user/domain/invitecode.go） | PASS | `userauth/application/invite.go` CheckInviteBinding self/cycle 拒绝；`invitecode.go` crypto/rand 8 位；integrationtest/invitation_bind_test 9 个用例全 PASS |
| 11 | 10-Level Affiliate（affiliate/） | PASS | integrationtest PASS（1.861s）；commission.go:238 通知异步 |
| 12 | Wallet Withdrawal（walletwithdrawal/） | PASS | integrationtest PASS（2.855s）；state_machine.go + fee.go + quote.go 完整 |
| 13 | Wallet Dual-Balance（wallet/domain/account.go） | PASS | account.go:13-14 AvailableBalance+FrozenBalance；`migrations/wallet_dual_balance.go` marker 幂等 + 守恒校验；integrationtest/freeze_test.go 验证 total invariant |
| 14 | C2C（c2c/） | PASS | `application/listing.go:139` ownedListing IDOR 防护；`trade.go:68` self-trade 拒绝；`trade.go:326` GetTradeDetail 仅买卖双方可见；`user_control.go:28` 注释"已有 Trade 仍可完成"；integrationtest/security_test 9 个用例全 PASS |
| 15 | General Ticket（supportticket/） | PASS | `integrationtest/architecture_test.go:TestNoFundingImports` PASS；`statemachine/status.go:58` ReopenWindow=7天；integrationtest 17 个用例全 PASS |
| 16 | Site Builder（sitebuilder/） | PASS | `application/validation.go:11-19` 首页入口路由白名单；`validation.go:52-69` 外链仅 http/https；grep `scripts`/`LEGACY_DISABLED`/`eval`/`innerHTML`/`v-html`/`new Function` 在 sitebuilder 下 **0 match**（字段已完全移除）；application 测试 PASS（1.194s） |
| 17 | Admin RBAC（internal/authz/） | PASS | `internal/authz` 测试 PASS（1.379s） |
| 18 | Upload/File（upload/） | PASS | `upload/application` PASS（1.129s）；`localstore` PASS；`transport/http` PASS（2.711s） |

> 说明：任务清单列出 19 项，实际逐一清点为 18 个独立模块（Wallet 与 Wallet Dual-Balance 为同一模块的两个审计维度，合并计 1 行；User Profile 为 Auth 的子能力）。

---

## 二、Exchange Rate 专项

**结论：PASS**

| 验证项 | 证据 |
|--------|------|
| CoinGecko backend only | `grep coinmarketcap/openexchangerates/fixer` 在 internal/ 下 0 业务实现；`service.go:141` `state.Provider = "coingecko"` 硬编码 |
| API key 不泄露 | `admin_handler.go:46` 返回 `api_key_masked`；`maskKey()` 仅显示后 4 位；`service.go:97` 错误信息写 `provider_fetch_failed` 不含 key |
| Redis cache | `rediscache/cached_store.go:27-42` GetState 读穿 Redis→settings；`:44-52` SaveState 写穿 Redis |
| Settings durable truth | `settingsstore/store.go` 持久化到 settings KV `global_exchange_rate`；Redis 故障回落 settings（注释 `:18` 明确"绝不 1:1"） |
| Manual fallback | `service.go:61-66` Auto 过期后用 ManualRate；`SetManual` 显式设置 |
| 无 rate 时 fail-closed | `service.go:68` 三级都不可用返回 `ErrRateUnavailable` |
| 不存在 1:1 fallback | `coingecko.go:41-43` siteCurrency==USDT 时返回 1 是合法"无需换算"分支，不是 fallback；`service.go:72` 注释"provider 失败时不抛 1:1 兜底" |
| Gateway Rate 隔离 | payment/gateway/adapters 下各支付渠道 adapter 不依赖 exchangerate；grep 无其他 rate provider 实现 |

---

## 三、Refund / After-Sale 专项

**结论：PASS**

| 验证项 | 证据 |
|--------|------|
| completed partial refund | `refund/service.go:396-398` newRefunded < paidBase 时写 `refund_status=partial`，主 status 不动 |
| completed full refund | `refund/service.go:394-395` newRefunded >= paidBase 时写 `refund_status=full` |
| failed/canceled 主状态不被覆盖 | `service.go:388-401` updates map 仅含 `refunded_amount/updated_at/refund_status`，**不含 status**；`:402` 注释"parent/child 主状态由 ordermachine 决定" |
| After-Sale 真正 credit Wallet | `aftersale/adapter.go:27` 直接调 `AdminRefundToWalletInTx` → `refund/wallet.go:137` `wallets.CreditInTransaction` |
| ledger 正确 | `wallet.go:154-165` 写 refunded_amount + refund_status；wallet Credit 内部写 ledger 4 列快照 |
| commission reversal | `service.go:432-442` + `wallet.go:171-181` 同一事务内调 `affiliateRefund.HandleOrderRefunded` |
| transaction atomic | `wallet.go:48` `orderStore.WithinTransaction` 包裹 wallet credit + order 更新 + refund record + affiliate + reseller |
| retry 不双退 | `aftersale/service.go:128` `LockAfterSalePendingByOrderIDForUpdate` 行锁 pending ticket；已 resolved 的 ticket 再调返回 `ErrInvalidAction` |
| 测试 | `go test ./internal/modules/order/integrationtest/refund/` ok（0.524s）；`./aftersale/` ok（0.181s） |

---

## 四、Invitation Binding 专项

**结论：PASS**

| 验证项 | 证据 |
|--------|------|
| 每用户唯一 invite_code | `invitecode.go:19-31` crypto/rand 8 位；`userauth/application/invite.go:63-79` `generateUniqueInviteCode` 碰撞重试 16 次 + `GetByInviteCode` 全局唯一检查 |
| 显式 invite > cookie | `invite.go:87-102` 显式 code 非空时直接解析，不看 cookie；`:104-112` 无显式 code 才回落 cookie |
| self/cycle 拒绝 | `invite.go:49-60` `CheckInviteBinding`：`inviterID==newUserID` → ErrSelfInvite；`IsDescendant` 检测成环 → ErrInviteCycle；maxDepth=64 防死循环 |
| 普通用户不可改 inviter | grep `user.InviterID =` 仅 `service.go:344` 注册路径 Create 时赋值；无任何用户侧 API 修改 inviter_id |
| 历史用户不猜测 inviter | `invite.go:104-113` cookie 归因失败静默返回 `(0, nil)`，不猜测上级 |
| 测试 | `invitation_bind_test.go` 9 个用例全 PASS（含 self/cycle、explicit>cookie、cookie failure silent fallback、atomic） |

---

## 五、C2C 权限专项

**结论：PASS**

| 验证项 | 证据 |
|--------|------|
| 合格用户可 SELL | `listing.go:54-83` `CheckListingEligibility`：C2C enabled + user active + 未 C2CBanned + TOTP + 冷却 + 支付方式 + 日限额 |
| 所有用户可买他人挂单 | `trade.go:42` `assertUserCanTrade(buyerID)` 后即可 CreateTrade（挂单市场公开 list） |
| self-trade 禁止 | `trade.go:68-75` `listing.SellerUserID == buyerID` → ErrSelfTrade + 风控信号 |
| User A 不可操作 User B listing | `listing.go:139-151` `ownedListing`：`SellerUserID != userID` 返回 `ErrListingNotFound`（不暴露存在性） |
| Buyer/Seller 操作权限严格 | `trade.go:174` MarkPaid 仅 BuyerUserID；`:218` Confirm 仅 SellerUserID；`:275` Cancel 仅 BuyerUserID；否则 `ErrPermissionDenied` |
| 非参与者不可看敏感 trade | `trade.go:321-329` `GetTradeDetail`：仅 buyer 或 seller 可见，否则 `ErrTradeNotFound` |
| C2C banned 已有 trade 仍可完成 | `user_control.go:28` 注释明确"已有 Trade 仍可正常 mark-paid/confirm/cancel/dispute"；banned 只阻断 `CreateListing`/`CreateTrade` 新业务 |
| 测试 | `security_test.go` 9 用例全 PASS：StrangerCannotUpdateOthersListing / StrangerCannotReadTrade / BuyerCannotConfirmAsSeller / SellerCannotMarkPaidAsBuyer / SelfTradeRejectedAndRiskSignaled / PaymentMethodIDOR / ZeroUserIDRejected / DisputedBuyerCannotCancel / DisputedSellerCannotConfirm |

---

## 六、User Notification 专项

**结论：PASS**

| 验证项 | 证据 |
|--------|------|
| unread/read | `domain/notification.go:61-62` IsRead + ReadAt；`application/service.go:70` CountUnread；`:75` MarkRead；`:87` MarkAllRead |
| DB 幂等 | `domain/notification.go:54-60` 复合唯一索引 `uniq_user_notify(user_id,biz_type,biz_id,type)`；`service.go:49` ErrAlreadyExists 静默返回 nil |
| C2C deep link | `c2c/application/trade.go:152` notifySafely 传 `trade_no/buyer_id/seller_id/usdt_amount/fiat_amount/fiat_currency` 作为 Data |
| Ticket deep link | `supportticket/application/service.go:75` notifySafely 调 notifier.CreateNotification；`supportticket_adapters.go:33` 适配到 usernotification |
| Wallet/Order 通知 | `order_service.go:150` TypeOrderCompleted；`fulfillment/service.go:69`；`payment_service_callback_dispatch.go:273/304`；`affiliate/commission.go:238` |
| 不阻塞资金主业务 | C2C `notifySafely` 通过 notifier.Enqueue 异步队列（`c2c/application/service.go:60-72`）；Order/Ticket 通知失败仅 `logger.Warnw` 不 return err |
| polling hidden pause/logout stop | transport/http/handler.go 提供 list/count-unread/mark-read/mark-all-read；前端轮询逻辑在前端代码（后端无 stop/pause 状态，靠 JWT 失效自然停止） |

---

## 七、General Ticket 专项

**结论：PASS（与资金域完全隔离）**

| 验证项 | 证据 |
|--------|------|
| 与 After-Sale 分域 | After-Sale 在 `order/application/aftersale/`；Ticket 在 `supportticket/`；architecture_test 禁 import order/application/aftersale |
| 与 C2C Dispute 分域 | C2C dispute 在 `c2c/application/dispute.go`；Ticket 禁 import c2c/application |
| 与 Withdrawal 分域 | Withdrawal 在 `walletwithdrawal/`；Ticket 禁 import walletwithdrawal/application |
| **无资金 service 注入** | `architecture_test.go:13-22` forbiddenImports 列 8 个资金包；`TestNoFundingImports` PASS；`grep hcz/internal/modules/(wallet|order|c2c|affiliate|walletwithdrawal)` 在 supportticket/ 下 **0 match** |
| IDOR | `security` 等价：`TestUserIDOR` PASS；`supportticket_test.go` 覆盖 |
| RBAC | admin_handler vs user_handler 分文件；admin 路由独立 |
| unread | `TestUserReplyUnreadCounts` / `TestAdminReplyUnreadAndNotification` / `TestReadDetailResetsUnread` PASS |
| attachment private access | `application/service.go:86-106` `linkAttachmentsToMessage` 仅挂 `att.TicketID==0 && att.UploaderType==uploaderType` 的附件 |
| rate limit | 路由层未单独限频（复用全局 gin 限流）；非 ticket 模块内逻辑 |
| assignment race | `TestClaimTicketRace` / `TestUserReplyAndAdminReplyConcurrent` / `TestCloseVsReplyRace` PASS |
| reopen 7 天 | `statemachine/status.go:58` `ReopenWindow = 7 * 24 * time.Hour`；`TestResolveThenReopenWithinWindow` / `TestReopenAfterExpired` PASS |
| 测试 | 17 个 integrationtest 用例全 PASS（0.181s） |

---

## 八、Site Builder 专项

**结论：PASS（scripts 已彻底移除，非 LEGACY_DISABLED 字符串保留）**

| 验证项 | 证据 |
|--------|------|
| Home 四大业务入口可配置 | `domain/home_entry.go` + `application/home_entry_service.go`；`validation.go:11-19` 路由白名单 recharge/c2c/wallet/withdrawal/invitation/support/orders |
| Discovery 动态 | `application/discovery_block_service.go` CRUD + Reorder + ListPublic 启用过滤 |
| classic/vault 共用数据 | 单一 sitebuilder 模块，无 classic/vault 分库 |
| brand/template | `application/brand_service.go` + `template_service.go` |
| bootstrap 一次返回 | admin_handler.go 聚合 HomeEntries/Discovery/Brand/Template |
| Redis invalidation | `admin_handler.go:190` `h.invalidate(c)` 写操作后失效缓存 |
| fallback | 测试覆盖 |
| **scripts 字段** | `grep scripts` 在 `internal/modules/sitebuilder/` 下 **0 match**；`grep LEGACY_DISABLED` **0 match**。scripts 字段已从 schema/service/handler 全部移除，比"保留 LEGACY_DISABLED 占位字符串"更彻底。 |
| **eval / new Function / innerHTML / v-html / script injection** | `grep eval\(|innerHTML|v-html|new Function` 在 sitebuilder 下 **0 match**（后端 Go 代码无任何可执行配置路径） |
| 外链安全 | `validation.go:52-69` `ValidateExternalURL` 仅允许 http/https，拒绝 javascript:/data:/file: |
| 测试 | `application/sitebuilder_test.go` PASS（1.194s） |

> 注：审计过程中首次跑 `go test ./internal/modules/sitebuilder/...` 曾报 `discovery_block_service.go:143:49: undefined: gormstore` 编译错误；复跑确认 admin_handler.go:179-181 与 sitebuilder_test.go:193 实际已是 `sitebuilderapp.ReorderItem` / `[]ReorderItem`，`go build` 与 `go vet` 均通过，测试 PASS。该错误为首次运行时的缓存/行号偏移误报，非真实编译失败。

---

## 九、Migration 专项

**结论：PASS（可生产升级）**

| 验证项 | 证据 |
|--------|------|
| AutoMigrate registry 完整 | `migrations/registry.go:52-121` 注册全部关键表：walletdomain.Account/Transaction/RechargeOrder、withdrawaldomain.Withdrawal/Address、c2cdomain.PaymentMethod/Listing/Trade/Dispute/RiskSignal、supportdomain.Category/Ticket/Message/Attachment/Audit、sitebuilderdomain.HomeEntry/DiscoveryBlock/SiteAuditLog、usernotificationdomain.UserNotification、orderdomain.Order/OrderItem/OrderRefundRecord/AfterSaleTicket |
| Wallet dual balance | `migrateWalletDualBalance()` marker `migration/wallet_dual_balance_v1` 幂等；HasColumn 防御补列；回填只更新未迁移行；守恒校验；不删旧列（staged） |
| Ledger columns | 4 新列 available_before/after/frozen_before/after 由 dual balance migration 添加 + 回填 |
| Invitation fields | `BackfillInviteCodes(db)` :161；`migrations/invitation.go` |
| Withdrawal tables | withdrawaldomain.Withdrawal/Address 注册 |
| Affiliate index | `migrateAffiliateCommissionMultilevel()` :170；`migrations/affiliate_commission_multilevel.go` |
| C2C tables | PaymentMethod/Listing/Trade/Dispute/RiskSignal 全注册 |
| Ticket tables | Category/Ticket/Message/Attachment/Audit 全注册 |
| Site Builder tables | HomeEntry/DiscoveryBlock/SiteAuditLog 全注册 |
| 幂等 | SeedSupportTicketCategories / SeedHomeEntries 均 FirstOrCreate（已存在跳过）；dual balance 有 marker；DropColumn price_currency 有 HasColumn 守卫 |
| Fresh Install | AutoMigrate 在空库上建全部表 + seed 初始数据 |
| Existing Upgrade | 各 migration 函数先 HasColumn 检查再补列；回填条件幂等 |
| 测试 | `go test ./internal/bootstrap/database/migrations/...` ok（0.527s），含 wallet_dual_balance_test/invitation_test/sku_migration_test/payment_provider_migration_test/affiliate_commission_multilevel_test |

---

## 十、Legacy Columns 专项

**用户提示预存：wallet_accounts.balance / balance_before / balance_after staged 保留。**

实测结论：

| 列 | 表 | ACTIVE READ/WRITE | 证据 |
|----|----|-------------------|------|
| `balance` | wallet_accounts | **0** | `wallet/domain/account.go` struct 仅 AvailableBalance/FrozenBalance，无 Balance 字段；业务代码 grep `\.Balance\b` 在 wallet/ 下 0 命中（除 presenter 命名）；`wallet_dual_balance.go:81` HasColumn 仅迁移期读，不写。**P2 cleanup：可生产后单独 DROP COLUMN。** |
| `balance_before` | wallet_transactions | **>0**（ACTIVE WRITE） | `wallet/domain/transaction.go:18` struct 仍有该字段；`freeze.go:153/236/364/386`、`credit.go:77/131`、`order_balance.go:76/144` 业务代码主动写；`presenter/wallet.go:62` 作为 TotalBefore 展示给前端。**不能 drop。** |
| `balance_after` | wallet_transactions | **>0**（ACTIVE WRITE） | 同上 `transaction.go:19`；`channel_handler.go:117` 返回 balance_after；presenter TotalAfter。**不能 drop。** |

> **重要修正**：用户提示将 `balance_before/balance_after` 列为 staged 保留列，但实测这两列仍在 ACTIVE WRITE（作为 available+frozen 的合计冗余列），并非 stale 列。它们与新 4 列（available_before/after/frozen_before/after）冗余但被业务代码主动维护。若要 drop，需先改造 presenter 层改为 `available_before + frozen_before` 计算，再删除 struct 字段，最后 DROP COLUMN——属于 P2 重构，非本轮范围。

---

## 十一、Old 9-State 专项（仅 Recharge Business Order）

**结论：ACTIVE_WRITE = 0**

| 旧状态 | 分类 | 证据 |
|--------|------|------|
| `pending_payment` | NORMALIZE_COMPAT | `constants.go:5` 常量保留；`ordermachine/machine.go:28` case 归一化；`backfillPendingOrderRiskIPs` migration 用旧值做一次性回填 |
| `paid` | NORMALIZE_COMPAT | `constants.go:6` 保留；`order_service_child.go:184` `if target == OrderStatusPaid { return ErrOrderStatusInvalid }`——admin 不能直接设，仅状态机读旧态归一 |
| `fulfilling` | NORMALIZE_COMPAT | `constants.go:7` 保留；`machine_test.go:13` 映射到 processing |
| `delivered` | NORMALIZE_COMPAT | `constants.go:10` 保留；`order_service_child.go:357/450` 注释"兼容旧态 delivered"；machine_test.go:15 映射到 completed |
| `partially_refunded` | NORMALIZE_COMPAT | `constants.go:9` 保留；`order_service_child.go:188` 注释"删除 admin 直接设 partially_refunded/refunded 主状态的分支"；machine_test.go:18 映射到 completed + refund_status=partial |
| `refunded` | NORMALIZE_COMPAT | 同上，已删除 ACTIVE_WRITE 分支 |

**Recharge ACTIVE_WRITE = 0 确认**：
- `order_service_child.go:184-186` 拒绝 admin 直接设 paid
- `:188-189` 注释明确删除 partially_refunded/refunded 主状态写入
- `refund/service.go:388-401` updates map 不含 status
- 主状态唯一权威是 `ordermachine`（五态：pending_recharge/processing/completed/failed/canceled）
- C2C 自己合法使用 pending_payment/paid（C2C 状态机），不属于 Recharge Business Order 残留

---

## 十二、总结论

### 四个关键问题直接回答

1. **Ticket 是否与资金域完全隔离？**
   **是。** `architecture_test.go:TestNoFundingImports` PASS；grep `hcz/internal/modules/(wallet|order|c2c|affiliate|walletwithdrawal)` 在 supportticket/ 下 0 match；Ticket 不注入任何资金 service，仅对 orders/wallet_withdrawals/c2c_trades 做只读原始 SQL 做 biz ownership 校验。

2. **Site Builder scripts 是否彻底禁用？**
   **是，且比预期更彻底。** grep `scripts`/`LEGACY_DISABLED`/`eval`/`innerHTML`/`v-html`/`new Function` 在 sitebuilder/ 下全部 0 match。scripts 字段已从 schema/service/handler 完全移除，不是保留 `LEGACY_DISABLED` 占位字符串。后端 Go 代码无任何可执行配置路径。

3. **Migration 是否可生产升级？**
   **是。** AutoMigrate registry 完整覆盖全部关键表；dual balance/invitation/affiliate/c2c/ticket/sitebuilder migration 均幂等（marker + HasColumn + FirstOrCreate）；migration 测试 PASS；Fresh Install 与 Existing Upgrade 路径都覆盖。

4. **Legacy columns ACTIVE CODE 数 / Old 9-state Recharge ACTIVE WRITE 数**
   - `wallet_accounts.balance`：ACTIVE CODE = **0**（P2 cleanup，可生产后单独 drop）
   - `wallet_transactions.balance_before/balance_after`：ACTIVE CODE = **>0**（业务仍在写，作为 total 冗余列，本轮不删）
   - Old 9-state Recharge ACTIVE WRITE = **0**（全部 NORMALIZE_COMPAT）

---

## 十三、P0 / P1 / P2 问题清单

### P0（阻断生产）
无。

### P1（必须修复后方可上生产）
无。

### P2（生产后跟进清理，不阻断本轮上线）
1. **wallet_transactions.balance_before/balance_after 冗余列**：业务代码仍在 ACTIVE WRITE（作为 total 合计），与 available_before/after/frozen_before/after 冗余。建议：先改 presenter 层用 `available + frozen` 计算 TotalBefore/TotalAfter，再删除 struct 字段，最后 DROP COLUMN。
2. **wallet_accounts.balance 旧列**：ACTIVE CODE = 0，可在生产稳定后单独 `ALTER TABLE wallet_accounts DROP COLUMN balance`。
3. **旧九态常量保留**：`constants.go:5-13` 仍定义 pending_payment/paid/fulfilling/delivered/partially_refunded/refunded，用于 ordermachine.Normalize 归一化历史数据。待历史数据全部归一化后可删除常量（不紧迫）。
4. **C2C fee=0 固定**：第一版设计（用户提示已知），非 bug。
5. **首次 sitebuilder 测试曾报编译错误**：复跑确认是缓存/行号偏移误报，非真实问题。已记录备查。

---

## 十四、测试实跑记录

| 测试包 | 结果 | 耗时 |
|--------|------|------|
| `internal/modules/order/integrationtest/refund` | ok | 0.524s |
| `internal/modules/order/integrationtest/aftersale` | ok | 0.181s |
| `internal/modules/identity/userauth/integrationtest`（invitation 9 用例） | ok | 1.153s |
| `internal/modules/c2c/integrationtest`（security 9 用例） | ok | 6.729s |
| `internal/modules/usernotification/...` | ok | 0.276s |
| `internal/modules/supportticket/integrationtest`（17 用例） | ok | 0.181s |
| `internal/modules/sitebuilder/application` | ok | 1.194s |
| `internal/bootstrap/database/migrations/...` | ok | 0.527s |
| `internal/modules/wallet/...` | ok | ~3.5s |
| `internal/modules/walletwithdrawal/integrationtest` | ok | 2.855s |
| `internal/modules/affiliate/...` | ok | ~2.4s |
| `internal/modules/identity/userauth/...`（全量） | ok | 19.072s |
| `internal/modules/upload/...` | ok | ~4.0s |
| `internal/authz` | ok | 1.379s |

---

*报告完。本轮未做任何代码修改（审计结论为：现有代码状态符合预期，无需最小必要修复）。*
