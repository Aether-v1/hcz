# HCZ Phase 6 — C2C Internal Wallet Trading Pre-Audit v3（修订版）

> 本轮只审计，不修改代码。本文档为 v3 修订版，核心变更：业务模式从 Merchant-only 改为开放式 User-to-User C2C，Merchant 降级为非必须并推迟到 Full V1。
> 审计日期：2026-10-05（v3 修订）
> 代码基线：Phase 5 Wallet Dual-Balance 已封板 + 关联模块

---

## 0. 结论速览（修订版）

| 审计项 | 结论（v3 修订） |
|---|---|
| 第一版业务模式 | **开放式 User-to-User C2C**。所有已满足风控条件的普通用户都可以发布 SELL 挂单、购买其他用户的 SELL 挂单、作为 Seller/Buyer 完成交易。不要求 Merchant 身份 |
| 第一版挂单方向 | **只做 SELL 挂单**。所有用户通过 SELL 挂单即可实现买卖双方角色：想卖→自己发布 SELL 挂单；想买→购买其他人的 SELL 挂单。BUY listing 留到 Full V1 |
| Merchant 定位 | **降级为非必须，第一版完全推迟到 Full V1**。Merchant 未来可作为认证商家/专业交易员/更高限额/badge，但普通用户不需要 Merchant 身份也能挂单 |
| 挂单与交易单是否分离 | **是**。`c2c_listings`（挂单）与 `c2c_trades`（交易单）彻底分离 |
| C2C 交易资产 | **明确为 HCZ 平台内部 Wallet USDT Balance**。禁止 TRON API、链上转账、Withdrawal、Recharge Gateway |
| Trade 创建时 Freeze | 同一事务：lock listing → 校验 → Freeze(seller) → 创建 trade → 扣减 listing.available_usdt |
| Cancel/Expire 时 Unfreeze | 同一事务：lock trade → Unfreeze(seller) → 恢复 listing.available_usdt → trade → canceled/expired |
| Confirm/Arbitration 时 Settle | 同一事务：lock trade → 按 user_id 升序锁 seller/buyer → SettleFrozen(seller→buyer) → trade → completed |
| 最终状态机 | 6 态：pending_payment / paid / completed / canceled / expired / disputed |
| MVP 数据表 | **4 张**：`c2c_payment_methods` / `c2c_listings` / `c2c_trades` / `c2c_disputes`。`c2c_merchants` 推迟到 Full V1 |
| Payment Method | 所有 SELL 用户可配置自己的收款方式（Bank/Alipay/WeChat/PayNow/Custom），发布挂单至少选一个启用中的收款方式 |
| 第一版 fee | **fee = 0**。先跑通资金安全，Admin Settings 预留配置位 |
| 最大并发/资金风险 | 超卖靠 listing row lock + available_usdt atomic update + Freeze 同事务；双账户死锁靠 user_id 升序锁；幂等靠 reference 唯一键 + 状态机 |
| self-trade/wash-trade | 禁止吃自己的挂单（buyer != seller 强制校验）；高频互相交易/高频取消记录 risk signal；第一版不建复杂自动风控引擎 |
| C2C MVP 最小范围 | 所有合格用户可发布 SELL 挂单 + 所有用户可购买 + 固定价格 + 平台内部 Wallet USDT + Freeze + 我已付款 + 确认收款 + Cancel + Timeout + Dispute + Admin Arbitration + Payment Methods + Notification + User/Admin UI + 基础风控 |
| 是否可以进入实施 | **可以**。所有依赖基础设施已就绪，开放式 U2U 模型已明确，MVP 范围清晰 |

---

## 一、冻结业务术语

（与 v2 一致，无变更）

用户端中文统一使用：C2C 交易、买 USDT、卖 USDT、挂单、我的挂单、我的交易、交易详情、申诉、仲裁。不使用"广告"作为用户端正式中文术语。代码内部可以使用 listing/offer/trade/p2p。

---

## 二、冻结资产定义

（与 v2 一致，无变更）

C2C 交易的不是链上 USDT，而是 **HCZ 平台内部 Wallet USDT Balance**。

```
Seller 出售的是 wallet_accounts.available_balance
Buyer 收到的是 wallet_accounts.available_balance
```

资金流转：Trade 创建时 Freeze(seller) → Buyer 平台外支付法币 → Seller 确认收款后 SettleFrozen(seller→buyer)。

C2C 主链绝对禁止：调用 TRON API、链上 USDT 转账、调用 Withdrawal、调用 Wallet Recharge Gateway、任何私钥/签名操作。链上转出仍然只属于 Wallet Withdrawal 模块。

---

## 三、第一版业务模式（v3 核心修订）

### 3.1 修订后的业务模式：开放式 User-to-User C2C

**所有已满足风控条件的普通用户都可以：**
- 发布 SELL 挂单（卖平台 USDT）
- 修改自己的挂单（价格、额度、收款方式、条款）
- 暂停/关闭自己的挂单
- 购买其他用户发布的 SELL 挂单
- 作为 Seller 完成交易（确认收款）
- 作为 Buyer 完成交易（我已付款）

**不要求** `c2c_merchants.status=approved` 才能挂单。Merchant 身份不是挂单的必要条件。

### 3.2 三种方案对比（修订后）

| 维度 | 方案 A：Merchant-only（v2 原方案，已废弃） | 方案 B：开放式 U2U（v3 修订方案 ✅） | 方案 C：Merchant + 用户混合 |
|---|---|---|---|
| 挂单方 | Admin 授权商家 | 所有合格普通用户 | 商家 + 普通用户（有等级差异） |
| 吃单方 | 所有用户 | 所有用户 | 所有用户 |
| 流动性 | 依赖商家数量 | 高（所有用户互相提供） | 高 |
| 风控复杂度 | 低（商家可控） | 中（需基础风控 + 自助管理） | 中高 |
| 第一版可行性 | 高但不符合产品需求 | **高且符合产品需求** | 中（Merchant 等级增加复杂度） |
| 用户体验 | 普通用户只能买，不能卖 | 所有用户可买可卖 | 有等级差异 |

### 3.3 最终推荐：方案 B — 开放式 User-to-User C2C

**理由：**
1. **符合产品需求**：用户明确要求所有普通用户都能买卖平台 Wallet USDT Balance
2. **流动性更好**：所有用户都能提供挂单，不依赖少数 Merchant
3. **第一版只需 SELL 挂单**：所有用户通过 SELL 挂单即可实现买卖双方角色（见第四节），不需要 BUY 挂单
4. **风控可控**：第一版通过基础风控（2FA、新用户冷却、额度限制、self-trade 禁止、高频取消限制）即可控制风险，不需要复杂自动风控引擎
5. **Admin 角色简化**：Admin 不需要逐个审批用户是否可挂单，只需管理全局设置、处理申诉仲裁、执行风控禁用

### 3.4 所有用户可同时具有 Buyer/Seller 身份

同一个 User 可以：
- 今天挂单卖 USDT（作为 Seller）
- 同时购买其他人的 USDT（作为 Buyer）

但**禁止购买自己的挂单**：创建 Trade 时强制校验 `buyer_user_id != seller_user_id`。

---

## 四、第一版仍然只做 SELL 挂单

### 4.1 为什么只做 SELL 挂单就能完成"所有用户可以买卖"

```
用户想卖 USDT → 自己发布 SELL 挂单（作为 Seller）
用户想买 USDT → 购买其他用户发布的 SELL 挂单（作为 Buyer）
```

SELL 挂单的本质是"有人愿意以固定价格卖出 USDT"，任何用户都可以：
- 作为挂单方（Seller）发布 SELL 挂单
- 作为吃单方（Buyer）购买其他人的 SELL 挂单

因此第一版**无需额外实现 BUY listing**。BUY listing（"有人愿意以固定价格买入 USDT，卖家吃单"）是另一种市场结构，留到 Full V1。

### 4.2 BUY listing 推迟到 Full V1

Full V1 扩展时可增加 BUY listing，形成双向市场（订单簿）。第一版不做。

---

## 五、删除 Merchant 作为挂单权限前置

### 5.1 Merchant 重新定位

v2 原方案中 Merchant 是挂单的必要前置条件（必须 Admin 授权才能挂单）。v3 修订后：

**Merchant 不再是挂单的必要条件。** 普通用户不需要 Merchant 身份也能挂单。

Merchant 未来可作为增强功能：
- 认证商家（verified badge）
- 专业交易员标识
- 更高交易限额（单笔/每日）
- 更高挂单额度
- Merchant 专属展示位/排序优先
- 更低手续费（未来开启 fee 后）

### 5.2 第一版是否需要 c2c_merchants 表

**第一版不需要 c2c_merchants 表。**

理由：
1. Merchant 没有必须功能（挂单不需要 Merchant 身份）
2. Merchant 认证/等级/限额是增强功能，第一版不实现
3. 减少 MVP 复杂度，专注核心资金链
4. 普通用户的限额通过全局 C2C Settings 配置（所有用户统一限额），不需要 Merchant 级别的差异化限额

**c2c_merchants 表推迟到 Full V1。** Full V1 引入 Merchant 系统时再建表。

### 5.3 MVP 数据表从 5 张减为 4 张

| v2 原计划（5 张） | v3 修订后（4 张） | 说明 |
|---|---|---|
| c2c_merchants | ❌ 移除 | 推迟到 Full V1 |
| c2c_payment_methods | ✅ 保留 | 所有 SELL 用户配置收款方式 |
| c2c_listings | ✅ 保留 | 挂单 |
| c2c_trades | ✅ 保留 | 交易单 |
| c2c_disputes | ✅ 保留 | 申诉 |

---

## 六、User 挂单资格

### 6.1 所有用户可挂单，但必须满足基础安全条件

发布 SELL 挂单前必须校验：

| 条件 | 说明 | 第一版 |
|---|---|---|
| 已登录 | 有效 User JWT | ✅ 必须 |
| 账号状态正常 | user.status = active，未被禁用/封禁 | ✅ 必须 |
| TOTP 2FA 已开启 | 未开启 2FA 不得挂单（fail-closed） | ✅ 必须 |
| 新用户冷却期已结束 | `current_time - user.created_at >= new_user_cooldown_hours`（复用 Phase 3 配置或 C2C 独立配置） | ✅ 必须 |
| Wallet available_balance 足够 | `available_balance >= total_usdt`（挂单总额不超过可用余额，虽然挂单时不冻结，但需确保有足够资金可被未来 Freeze） | ✅ 必须 |
| 未超过单笔额度 | 挂单总额 <= 全局单笔挂单上限（Admin Settings） | ✅ 必须 |
| 未超过每日交易额度 | 当日已成交 + 挂单中冻结 <= 全局每日上限（Admin Settings） | ✅ 必须 |
| 未触发高频取消限制 | 近期取消率未超过阈值（如 24h 内取消 >= 5 单则限制挂单 N 小时） | ⚠️ 第一版简单实现（计数+阈值） |
| 未被 C2C 禁用 | Admin 未对该用户执行 C2C 禁用 | ✅ 必须 |

### 6.2 不满足条件时

- 发布挂单：拒绝，返回具体原因（如"请先开启两步验证"、"新用户冷却期未结束"）
- 吃单成交：同样校验吃单方资格（Buyer 也需满足基础条件，除了"余额足够"改为其他）
- 不允许前端作为判断真源，所有校验服务端执行

### 6.3 挂单时仍然不冻结

发布 SELL listing 时：
- **不冻结整张挂单额度**
- 仅记录 `total_usdt` 和 `available_usdt`（初始 = total_usdt）
- 校验 `available_balance >= total_usdt`（确保有足够资金可被未来 Freeze，但不实际冻结）

真正 Buyer 下单（创建 Trade）时：
- 才 Freeze Seller 本笔 USDT
- listing.available_usdt 扣减

### 6.4 挂单后余额变化的处理

如果 Seller 发布挂单后，在挂单未成交期间：
- Seller 提现/消费导致 available_balance 减少，低于挂单总额
- 此时 Buyer 吃单时 Freeze 会失败（available 不足）
- **处理方式**：吃单时 Freeze 失败 → 交易创建失败 → 返回"卖家余额不足"。挂单本身不自动关闭（Seller 可自行关闭或补充余额）
- 这是"挂单不冻结"模型的已知特性，第一版接受此行为

---

## 七、挂单模型

（与 v2 基本一致，移除 Merchant 相关约束）

### 7.1 c2c_listings 表设计

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| listing_no | varchar(32) | uniqueIndex，挂单号 |
| owner_user_id | uint | index，挂单人用户 ID（任意合格用户，非 Merchant-only） |
| side | varchar(8) | SELL / BUY，第一版只 SELL |
| fiat_currency | varchar(8) | 法币币种 |
| price | decimal(20,2) | 固定价格（1 USDT = X 法币） |
| min_fiat_amount | decimal(20,2) | 单笔最小法币金额 |
| max_fiat_amount | decimal(20,2) | 单笔最大法币金额 |
| total_usdt | decimal(20,2) | 挂单总 USDT 额度 |
| available_usdt | decimal(20,2) | 剩余可成交 USDT |
| payment_method_ids | JSON / varchar(255) | 支持的收款方式 ID 列表 |
| terms | text | 交易条款/备注 |
| status | varchar(20) | active / paused / closed / expired |
| created_at / updated_at | | |

### 7.2 关键设计

- `available_usdt` 是并发控制核心（吃单时 row lock + 原子扣减）
- 挂单时不冻结资金，只有 Trade 创建时才 Freeze
- status：active（可成交）→ paused（挂单人暂停）→ closed（挂单人关闭/额度售罄）→ expired（过期）
- 挂单人可以修改自己挂单的价格、额度、收款方式、条款（只影响新吃单，不影响在途交易）
- 挂单人可以暂停/关闭自己的挂单

---

## 八、挂单资金规则

（与 v2 一致）

挂单时不冻结整张挂单额度。只有真正生成交易单时才冻结本笔成交 USDT。

并发超卖防护：listing row lock + available_usdt 原子校验扣减 + Freeze 同事务，三重防护。绝不会超卖。

---

## 九、挂单与交易单必须分离

（与 v2 一致）

`c2c_listings`（挂单，长期，可被多笔吃单）与 `c2c_trades`（交易单，单笔生命周期）彻底分离。不允许一表两用。

---

## 十、Trade 角色

（与 v2 一致）

每笔交易必须显式保存：`buyer_user_id`、`seller_user_id`、`listing_owner_user_id`、`taker_user_id`。不允许只靠 side 临时推导。

第一版 SELL 挂单：`listing_owner = seller`，`taker = buyer`。四个字段都保存，为未来 BUY 挂单做准备。

---

## 十一、交易单模型

（与 v2 一致，移除 fee 相关的 Merchant 逻辑）

`c2c_trades` 表包含：id、trade_no、listing_id、buyer_user_id、seller_user_id、listing_owner_user_id、taker_user_id、fiat_currency、price、fiat_amount、usdt_amount、fee_amount（第一版=0）、buyer_receive_usdt（第一版=usdt_amount）、status、payment_method_snapshot、payment_reference、buyer_paid_at、seller_confirmed_at、expired_at、canceled_at、disputed_at、completed_at、created_at/updated_at。

金额、价格、收款方式必须 snapshot。挂单后续修改不得影响已生成的交易单。

---

## 十二、C2C 状态机

（与 v2 一致）

6 态：pending_payment / paid / completed / canceled / expired / disputed。

合法流转：
- pending_payment → paid（Buyer 我已付款）
- pending_payment → canceled（Buyer 取消）
- pending_payment → expired（系统超时）
- paid → completed（Seller 确认收款）
- paid → disputed（任一方申诉）
- disputed → completed（Admin 仲裁放行）
- disputed → canceled（Admin 仲裁退回）

completed/canceled/expired 为终态。paid 后不允许普通取消，只能申诉。

Seller 第一版不允许主动 cancel（任何阶段）。

---

## 十三 ~ 十六、创建交易 / Buyer 我已付款 / Seller 确认收款 / 取消

（与 v2 一致，无变更。核心资金操作：创建时 Freeze(seller)，确认时 SettleFrozen(seller→buyer)，取消时 Unfreeze(seller)+恢复 listing。）

---

## 十七、超时

（与 v2 一致）

复用 asynq 基础设施，创建 trade 时注册超时任务（process_in=timeout）。超时任务幂等：只处理 status=pending_payment 的 trade，执行 Unfreeze + 恢复 listing + trade→expired。

---

## 十八、申诉

（与 v2 一致）

paid 后 Buyer/Seller 可以申诉。`c2c_disputes` 表（trade_id uniqueIndex，一笔交易最多一个申诉）。disputed 后 USDT 继续冻结，禁止普通 cancel/confirm，等待 Admin 仲裁。

---

## 十九、Admin 仲裁

（与 v2 一致）

两个结果：release_to_buyer（SettleFrozen）/ return_to_seller（Unfreeze+恢复 listing）。

保护机制：Admin JWT + RBAC（c2c:arbitrate）+ Payment Compliance + Step-Up（TOTP）+ Reason + Audit + Idempotency-Key。仲裁只能执行一次。

---

## 二十、Payment Method（v3 修订：所有 SELL 用户可配置）

### 20.1 所有 SELL 用户可配置收款方式

v2 原方案中收款方式主要面向 Merchant。v3 修订后：

**所有发布 SELL 挂单的用户都必须配置至少一个启用中的收款方式。**

收款方式属于用户个人资产，与 Merchant 身份无关。

### 20.2 c2c_payment_methods 表设计

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | PK |
| user_id | uint | index，所属用户（任意 SELL 用户，非 Merchant-only） |
| type | varchar(20) | bank / alipay / wechat / paynow / custom |
| account_name | varchar(100) | 账户姓名 |
| account_identifier | varchar(255) | 账户标识（卡号/账号/手机号） |
| qr_image | varchar(500) | 收款码图片 URL |
| instructions | text | 付款说明/备注 |
| enabled | bool | 是否启用 |
| created_at / updated_at | | |

### 20.3 第一版支持的类型

Bank / Alipay / WeChat / PayNow / Custom。

### 20.4 挂单与收款方式

- 一个用户可以有多个收款方式
- 发布 SELL 挂单时**必须至少选择一个启用中的收款方式**（`payment_method_ids` 非空校验）
- Trade 创建时 snapshot 收款方式（`payment_method_snapshot`）
- 挂单后续修改收款方式不影响在途交易

### 20.5 敏感信息脱敏

| 场景 | 展示规则 |
|---|---|
| 收款方式列表（用户自己管理） | account_identifier 脱敏（如 ****1234） |
| 挂单详情（潜在 Buyer 查看） | 只显示 type + account_name（脱敏），不显示完整 account_identifier |
| 交易详情（成交双方） | 显示完整收款信息（Buyer 需要准确账户转账） |
| Admin 管理 | 显示完整信息 |

---

## 二十一、价格

（与 v2 一致）

第一版使用固定挂单价格。不做自动浮动价格。Global Rate/CoinGecko 最多作为用户挂单时的参考价格显示，不能作为已有 Trade 的重新结算依据。Trade 创建时 snapshot 的 price 是唯一结算依据。

---

## 二十二、手续费

（与 v2 一致）

第一版 fee = 0，先跑通资金安全。Admin Settings 预留配置位（`c2c_config.fee_enabled` / `fee_percentage`），默认 0。不引入双边手续费。

---

## 二十三、风控（v3 修订：开放式 U2U 加强版）

### 23.1 第一版风控措施

由于所有用户都能挂单，第一版风控必须比 Merchant-only 模式更严格。

| 风控项 | 实现方式 | 第一版 |
|---|---|---|
| 用户挂单资格校验 | 已登录/账号正常/2FA/新用户冷却/余额足够/单笔额度/每日额度/高频取消限制/C2C 未禁用 | ✅ 必须 |
| User 2FA | 挂单和吃单前校验 TOTP 已开启，fail-closed | ✅ 必须 |
| 新账号交易冷却 | 复用 `new_user_cooldown_hours`，注册后 N 小时内不能 C2C 交易（挂单+吃单） | ✅ 必须 |
| 单笔 min/max | 挂单 min_fiat_amount / max_fiat_amount + 全局单笔上限 | ✅ 必须 |
| 每日成交额度 | 全局每日成交上限（所有用户统一，Admin Settings） | ✅ 必须 |
| 高频取消限制 | 统计用户 N 小时内取消次数，超过阈值限制挂单/吃单 | ⚠️ 第一版简单实现 |
| self trade 禁止 | 创建 trade 时校验 buyer != seller（禁止吃自己的挂单） | ✅ 必须 |
| wash trade 检测 | 记录 risk signal：高频互相交易（A→B→A 循环）、同一设备/IP 多账号（第一版不做设备指纹，可记录 IP） | ⚠️ 第一版记录 signal，不自动拦截 |
| C2C 用户禁用 | Admin 可对违规用户执行 C2C 禁用（禁止挂单+吃单） | ✅ 必须 |
| 挂单后余额不足 | 吃单时 Freeze 失败则交易失败，不自动关闭挂单 | ✅ 第一版接受此行为 |
| 设备指纹 | 复杂设备指纹/IP 风控 | ❌ 第一版不做 |
| 自动风控引擎 | 机器学习/规则引擎自动拦截 | ❌ 第一版不做 |

### 23.2 Self Trade / Wash Trade 防护（重点）

由于所有用户都能挂单，self-trade/wash-trade 风险高于 Merchant-only 模式。

**第一版必须实现：**
1. **禁止吃自己的挂单**：创建 Trade 时强制校验 `buyer_user_id != seller_user_id`，不满足则拒绝
2. **同一 trade buyer/seller 不得相同**：trade 表层面保证
3. **记录 risk signal**：
   - 高频互相交易：用户 A 和用户 B 在短时间内多次互为 buyer/seller（wash trade 嫌疑）
   - 高频取消：用户短时间内多次取消交易（可能是虚假挂单/扰乱市场）
   - 新账号大额交易：新注册用户短期内大额成交
   - 这些 signal 记录到 `c2c_risk_signals` 表（或日志），供 Admin 人工审核，第一版不自动拦截
4. **Admin 人工审核**：Admin 可查看 risk signal，对可疑用户执行 C2C 禁用

**第一版不做：**
- 复杂设备指纹
- IP 关联分析自动拦截
- 机器学习风控模型
- 自动冻结可疑账户

### 23.3 2FA fail-closed

- 发布挂单前校验用户已开启 TOTP 2FA
- 吃单（创建 Trade）前校验 Buyer 已开启 TOTP 2FA
- 未开启 2FA：拒绝操作，返回错误提示
- 复用现有 userauth TOTP 机制

### 23.4 新账号冷却

- 复用 `new_user_cooldown_hours` 配置（或 C2C 独立配置 `c2c_new_user_cooldown_hours`）
- 发布挂单和吃单都校验冷却期
- 配置为 0 时关闭限制

---

## 二十四、并发

（与 v2 一致）

超卖防护：listing row lock + available_usdt atomic update + Freeze 同事务，三重防护。

Freeze 与 Order/Withdrawal 同时扣款：行锁串行化，不会超扣。

Cancel/Unfreeze 与 Settle 并发：trade 状态机是最终仲裁，不会既解冻又结算。

双账户反向 Settle 不死锁：`LockAccountsByUserIDOrder` 按 user_id 升序锁。

---

## 二十五、幂等

（与 v2 一致）

所有操作幂等：CreateTrade（Idempotency-Key + trade_no unique）、Freeze/Unfreeze/Settle（reference 唯一键）、状态机校验（终态不可再操作）。

Reference 命名：`c2c_freeze:trade:<trade_no>` / `c2c_unfreeze:trade:<trade_no>` / `c2c_settle:trade:<trade_no>` / `c2c_receive:trade:<trade_no>`。

---

## 二十六、Ledger

（与 v2 一致）

使用 Phase 5 已实现的 ledger 类型：c2c_freeze / c2c_unfreeze / c2c_settle / c2c_receive。6 字段快照（total + available + frozen）。可完整还原 Seller/Buyer 的资金变化。

---

## 二十七、通知

（与 v2 一致）

7 个事件：c2c_trade_created / c2c_buyer_paid / c2c_trade_completed / c2c_trade_canceled / c2c_trade_expired / c2c_disputed / c2c_arbitrated。复用 Phase 1 Notification，异步发送，不阻塞资金事务。

---

## 二十八、User 前端（v3 修订）

### 28.1 页面清单

| 页面 | 功能 | 第一版 |
|---|---|---|
| C2C 首页 | 入口 + 买/卖导航 + 挂单市场概览 | ✅ |
| **买 USDT** | 展示其他用户 SELL 挂单列表（按价格/支付方式/法币筛选），点击吃单 | ✅ |
| **卖 USDT** | 本质上进入"发布 SELL 挂单"页面：填写价格/额度/收款方式/条款，提交 | ✅ |
| 挂单市场 | 所有用户 SELL 挂单列表（买 USDT 的详细列表页） | ✅ |
| 发布挂单 | 发布 SELL 挂单表单（卖 USDT 的详细表单页） | ✅ |
| 下单（吃单） | 输入购买金额，确认价格/费用/收款方式，2FA 验证，提交 | ✅ |
| 交易详情 | 交易状态、金额、价格、收款方式、倒计时、操作按钮 | ✅ |
| 我已付款 | pending_payment 状态填写 payment_reference | ✅ |
| 确认收款 | paid 状态 Seller 确认（所有用户都可能是 Seller） | ✅ |
| 取消交易 | pending_payment 状态 Buyer 取消 | ✅ |
| 发起申诉 | paid 状态任一方申诉 | ✅ |
| 我的交易 | 交易历史列表（全部状态筛选，包含作为 Buyer 和 Seller 的交易） | ✅ |
| 我的挂单 | 挂单管理（创建/修改/暂停/关闭/查看成交，所有用户都有此页面） | ✅ |
| 收款方式 | 收款方式 CRUD（所有 SELL 用户都需要配置） | ✅ |

### 28.2 关键交互说明

- **"买 USDT"页面**：展示其他用户的 SELL 挂单列表，用户选择挂单后输入购买金额，提交吃单
- **"卖 USDT"页面**：本质是发布 SELL 挂单表单，用户填写价格/总额度/最小最大单笔/收款方式/条款，提交后挂单进入市场
- **"我的挂单"**：所有用户都有此页面，管理自己发布的 SELL 挂单（修改/暂停/关闭）
- **"我的交易"**：包含用户作为 Buyer 和 Seller 的所有交易
- **"收款方式"**：所有用户都需要配置（至少一个启用中的收款方式才能发布 SELL 挂单）
- 所有 UI 文案使用统一术语（挂单、我的挂单、我的交易、申诉、仲裁）

---

## 二十九、Admin 前端（v3 修订）

### 29.1 页面清单

| 页面 | 功能 | v3 变更 |
|---|---|---|
| C2C 概览 | 总交易额、活跃挂单数、在途交易数、申诉数、用户数 | 移除 Merchant 数，改为 C2C 用户数 |
| ~~商家管理~~ | ~~Merchant 审批/管理~~ | ❌ 第一版移除，推迟到 Full V1 |
| 挂单管理 | 挂单列表、状态筛选、查看详情、强制关闭 | 保留 |
| 交易管理 | 交易列表、状态筛选、查看详情、资金流水 | 保留 |
| 申诉仲裁 | 申诉列表、详情、仲裁操作（release_to_buyer / return_to_seller） | 保留 |
| C2C 设置 | 全局开关、超时时间、手续费（预留）、新账号冷却、单笔/每日额度、高频取消阈值 | 保留 |
| **用户 C2C 管理** | 用户 C2C 禁用/解禁、查看用户 C2C 交易/挂单历史、risk signal 查看 | ✅ v3 新增（替代 Merchant 审批） |
| 风控信号 | risk signal 列表（wash trade 嫌疑、高频取消、新账号大额等） | ✅ v3 新增 |

### 29.2 Admin 角色变更

v2 原方案中 Admin 负责批准每个用户是否可挂单（Merchant 审批）。v3 修订后：

**Admin 不负责批准每个用户是否可挂单。** 所有合格用户自动可挂单。

Admin 主要管理：
1. **C2C Settings**：全局开关、超时、额度、冷却期、风控阈值
2. **挂单管理**：查看/强制关闭违规挂单
3. **交易管理**：查看交易详情/资金流水
4. **申诉仲裁**：处理 dispute，执行仲裁
5. **风控限制**：查看 risk signal，对违规用户执行 C2C 禁用/解禁
6. **用户 C2C 管理**：查看用户 C2C 历史，执行禁用/解禁

Merchant management 降级为后续增强（Full V1）。

### 29.3 Admin 操作保护

所有 Admin 写操作复用：Admin JWT + RBAC（独立 c2c 权限）+ Payment Compliance + Step-Up（仲裁操作）+ Reason + Audit + Idempotency-Key。

---

## 三十、MVP（v3 修订版）

### 30.1 C2C MVP 范围

**包含**：
1. 所有合格用户可发布 SELL 挂单（开放式 U2U）
2. 所有用户可购买其他人的 SELL 挂单
3. 固定价格（Trade 创建时 snapshot）
4. 平台内部 Wallet USDT（禁止链上）
5. Trade 创建时 Freeze(seller)
6. Buyer 外部支付法币（平台外）
7. Buyer 我已付款（payment_reference）
8. Seller 确认收款（SettleFrozen）
9. Cancel（pending_payment 阶段 Buyer 取消）
10. Timeout（asynq 超时任务）
11. Dispute（paid 后申诉）
12. Admin Arbitration（release_to_buyer / return_to_seller）
13. Payment Methods（所有用户可配置，Bank/Alipay/WeChat/PayNow/Custom）
14. Notification（7 个事件）
15. User 前端（C2C 首页、买 USDT、卖 USDT/发布挂单、挂单市场、交易详情、我的交易、我的挂单、收款方式）
16. Admin 前端（概览、挂单管理、交易管理、申诉仲裁、C2C 设置、用户 C2C 管理、风控信号）
17. 基础风控（2FA、新用户冷却、self-trade 禁止、min/max、每日额度、高频取消限制、C2C 用户禁用、risk signal 记录）

**暂不做**：
- ❌ BUY 挂单
- ❌ 普通用户 Merchant 认证/等级系统（推迟到 Full V1）
- ❌ 自动浮动价格
- ❌ 链上 USDT / 自动 TRON
- ❌ 复杂手续费（双边手续费、分级费率）
- ❌ 商家等级/信誉系统
- ❌ 高级自动风控引擎（设备指纹、IP 关联、机器学习）
- ❌ 统计报表（高级数据分析）
- ❌ 评价系统/聊天系统

### 30.2 MVP 数据表（4 张）

| 表 | 说明 |
|---|---|
| `c2c_payment_methods` | 用户收款方式 |
| `c2c_listings` | 挂单（所有合格用户可发布） |
| `c2c_trades` | 交易单 |
| `c2c_disputes` | 申诉 |

`c2c_merchants` 推迟到 Full V1。

可选辅助表（第一版可简化为日志或 Admin Settings）：
- `c2c_risk_signals` — 风控信号记录（第一版可先用日志，不建表）
- `c2c_user_limits` — 用户级 C2C 禁用/额度覆盖（第一版可在 users 表加 c2c_banned 字段，或独立表）

### 30.3 MVP 后端模块

新建 `internal/modules/c2c/`（vertical slice 模式，参考 walletwithdrawal）：
```
c2c/
├── domain/          # listing, trade, dispute, payment_method
├── contract/        # store, types, errors, ports
├── application/     # listing, trade, dispute, payment_method, arbitration, risk
├── infrastructure/  # gormstore
├── transport/http/  # user_handler, admin_handler, routes
└── integrationtest/ # 全量测试
```

---

## 三十一、Full V1 后续扩展

| 功能 | 说明 |
|---|---|
| BUY 挂单 | 普通用户挂单买 USDT，形成双向市场 |
| Merchant 系统 | 认证商家、专业交易员、更高限额、badge、排序优先 |
| 普通用户 P2P 全开放 | 已有（MVP 已开放），Full V1 增加 Merchant 等级差异 |
| 动态价格 | 跟随市场价格浮动、溢价/折价 |
| 手续费 | Admin 可配置 fee percentage、双边手续费 |
| 高级风控 | 设备指纹、IP 关联、行为分析、自动拦截、黑名单 |
| 统计报表 | 交易额趋势、用户排名、纠纷率分析 |
| 评价系统 | 交易后双方评价、用户信誉分 |
| 聊天系统 | 买卖双方站内沟通 |

---

## 三十二、最终明确回答（v3 修订版）

### 32.1 核心问题回答

| # | 问题 | 回答（v3 修订） |
|---|---|---|
| 1 | 所有普通用户是否都能挂 SELL 单 | **是**。所有已满足风控条件（已登录/账号正常/2FA/新用户冷却/余额足够/额度限制/未被禁用）的普通用户都可以发布 SELL 挂单。不要求 Merchant 身份 |
| 2 | 所有用户是否都能买其他人的 SELL 单 | **是**。所有合格用户都可以作为 Buyer 购买其他用户发布的 SELL 挂单 |
| 3 | Merchant 是否降级为非必须模块 | **是**。Merchant 不再是挂单的必要条件，第一版完全推迟到 Full V1。未来可作为认证商家/更高限额/badge 的增强功能 |
| 4 | 是否仍只需 SELL listing 就能完成"所有用户可以买卖" | **是**。用户想卖→自己发布 SELL 挂单（作为 Seller）；用户想买→购买其他人的 SELL 挂单（作为 Buyer）。所有用户通过 SELL 挂单即可实现买卖双方角色，不需要 BUY listing |
| 5 | 普通用户挂单的风控条件 | 已登录、账号状态正常、TOTP 2FA 已开启、新用户冷却期已结束、Wallet available_balance >= 挂单总额、未超过单笔额度、未超过每日交易额度、未触发高频取消限制、未被 C2C 禁用。不满足时不得发布/成交，服务端校验，fail-closed |
| 6 | self-trade/wash-trade 防护 | 禁止吃自己的挂单（buyer != seller 强制校验）；同一 trade buyer/seller 不得相同；高频互相交易/高频取消/新账号大额记录 risk signal 供 Admin 人工审核；第一版不建复杂自动风控引擎；Admin 可对违规用户执行 C2C 禁用 |
| 7 | MVP 数据表是否从 5 张减为 4 张 | **是**。去掉 `c2c_merchants`（推迟到 Full V1），保留 `c2c_payment_methods` / `c2c_listings` / `c2c_trades` / `c2c_disputes` 共 4 张 |
| 8 | 是否可以按修订后的开放式 C2C 模型进入正式实施 | **可以**。所有依赖基础设施已就绪（Wallet freeze 原语、双账户升序锁、Ledger 6 字段快照、Notification、Admin Compliance、asynq Job、User TOTP 2FA、新用户冷却），开放式 U2U 业务模型已明确，MVP 范围清晰，4 张表设计完成 |

### 32.2 资金流转（核心，未变更）

```
Trade 创建:  Freeze(seller)          → seller.available↓ seller.frozen↑
Buyer 我已付款: 无资金操作（只记录 payment_reference）
Seller 确认:  SettleFrozen(seller→buyer) → seller.frozen↓ buyer.available↑
Cancel/Expire: Unfreeze(seller)        → seller.frozen↓ seller.available↑ + 恢复 listing
仲裁放行:     SettleFrozen(seller→buyer)
仲裁退回:     Unfreeze(seller) + 恢复 listing
```

### 32.3 依赖基础设施确认（未变更）

| 基础设施 | 状态 |
|---|---|
| Wallet Freeze/Unfreeze/SettleFrozen | ✅ Phase 5 已封板 |
| 双账户升序锁（LockAccountsByUserIDOrder） | ✅ Phase 5 已实现 |
| Ledger 6 字段快照 | ✅ Phase 5 已实现 |
| User Notification | ✅ Phase 1 已封板 |
| Admin Compliance + Step-Up | ✅ 已实现 |
| asynq Job（超时任务模式） | ✅ 已实现 |
| User TOTP 2FA | ✅ 已实现 |
| 新账号冷却 | ✅ Phase 3 已实现 |
| AutoMigrate registry | ✅ 已实现 |

**所有依赖基础设施均已就绪，可以进入 C2C 实现。**

### 32.4 主要风险（v3 修订后）

| 风险 | 等级 | 缓解 |
|---|---|---|
| 法币支付在平台外，Seller 不确认 | 高 | 超时机制 + 申诉仲裁 + 所有用户 2FA + 信誉机制（Full V1） |
| Buyer 虚假"我已付款" | 高 | payment_reference 凭证 + 申诉仲裁 + risk signal 记录 + Admin 人工审核 |
| 开放式 U2U 导致 wash trade | 中高 | self-trade 禁止 + risk signal（高频互相交易/高频取消）+ Admin 人工审核 + C2C 禁用 |
| 超卖 | 中 | listing row lock + atomic update + Freeze 同事务 |
| 双账户死锁 | 中 | user_id 升序锁（强制 helper） |
| 资金操作幂等 | 中 | reference 唯一键 + 状态机 + Idempotency-Key |
| 挂单后余额不足导致吃单失败 | 中 | 吃单时 Freeze 失败则交易失败，挂单不自动关闭（Seller 可自行管理） |
| 新账号欺诈 | 中 | 新账号冷却 + 2FA + 每日额度限制 + risk signal |
| 仲裁误判 | 低 | 审计日志 + Step-Up + Admin 人工复核 |

---

## 附录：代码引用索引

| 能力 | 文件路径 |
|---|---|
| Wallet Freeze 原语 | `internal/modules/wallet/application/freeze.go` |
| LockAccountsByUserIDOrder | `internal/modules/wallet/application/freeze.go` L25-56 |
| Freeze / Unfreeze / SettleFrozen | `internal/modules/wallet/application/freeze.go` |
| Wallet Account domain | `internal/modules/wallet/domain/account.go`（AvailableBalance + FrozenBalance） |
| Wallet Transaction domain | `internal/modules/wallet/domain/transaction.go`（6 字段快照） |
| Wallet contract types | `internal/modules/wallet/contract/types.go`（FreezeInput/UnfreezeInput/SettleInput） |
| Ledger 类型常量 | `internal/constants/constants.go`（c2c_freeze/unfreeze/settle/receive） |
| Notification Service | `internal/modules/notification/application/send.go` |
| Notification 异步分发 | `internal/modules/notification/infrastructure/asyncqueue/dispatch.go` |
| Payment Compliance middleware | `internal/app/httpserver/middleware/compliance_middleware.go` |
| asynq 任务定义 | `internal/queue/tasks.go` |
| asynq Job Service / Consumer | `internal/app/jobs/service.go` / `internal/app/jobs/consumer/` |
| User TOTP 2FA | `internal/modules/identity/userauth/` |
| Admin Step-Up | `internal/modules/identity/adminauth/challenge/challenge.go` |
| 新账号冷却配置 | `internal/modules/settings/schema/integration/`（withdrawal_config.new_user_cooldown_hours，C2C 可复用或独立配置） |
| AutoMigrate registry | `internal/bootstrap/database/migrations/registry.go` |
| Phase 3 提现模块（架构参考） | `internal/modules/walletwithdrawal/` |
| User 模型（inviter_id/invite_code/status） | `internal/modules/identity/user/domain/user.go` |

---

*v3 修订完成。核心变更：业务模式从 Merchant-only 改为开放式 User-to-User C2C，Merchant 降级为非必须并推迟到 Full V1，MVP 数据表从 5 张减为 4 张，风控加强（self-trade/wash-trade/risk signal），User/Admin 前端相应修正。所有依赖基础设施已就绪，可以进入正式实施。*
