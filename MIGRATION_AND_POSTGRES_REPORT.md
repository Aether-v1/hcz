# HCZ 数据库 Migration 分析 + PostgreSQL 部署报告

> 生成时间：2026-10-05
> 范围：`github.com/Aether-v1/hcz`
> 目标：为生产 PostgreSQL 切换做部署准备、幂等验证、备份/恢复/预检脚本。

---

## 1. AutoMigrate 调用点全清单

### 1.1 唯一生产入口

| 位置 | 说明 |
|---|---|
| `cmd/server/main.go:143` | 启动时调用 `databasemigrations.AutoMigrate()`，紧接 `startupGuard.MarkMigrationStarted()` 之后、HTTP server 启动之前。 |
| `internal/bootstrap/database/migrations/registry.go:50` | `func AutoMigrate() error` —— 全量 schema 注册 + 有序数据回填。 |

`registry.go:52-121` 一次性 `db.AutoMigrate(...)` 注册了 **68 个 model**（含 reseller 模块通过 `resellerstore.Migrate(db)` 在 `registry.go:131` 额外注册的 9 张表，合计 77 张表，与本地幂等验证实测一致）。

### 1.2 八个关键领域模型清单

| 领域 | 表名 | 结构体文件 | AutoMigrate 注册行 | 关键字段 |
|---|---|---|---|---|
| **Wallet 双余额** | `wallet_accounts` | `internal/modules/wallet/domain/account.go:10` | `registry.go:60` | `available_balance` decimal(20,2)、`frozen_balance` decimal(20,2)，user_id 唯一索引 |
| **Ledger 流水** | `wallet_transactions` | `internal/modules/wallet/domain/transaction.go:10` | `registry.go:61` | `type` varchar(40)、`direction` varchar(16)（in/out）、`amount` decimal(20,2)、`available_before/after`、`frozen_before/after`、`reference` 唯一索引 |
| **Invitation** | `users`（内嵌） | `internal/modules/identity/user/domain/user.go:10` | `registry.go:54` | `inviter_id`、`invite_code` varchar(12)、`invite_bound_at`；唯一索引 `uni_users_invite_code` 由 `invitation.go:55-64` 在 backfill 后创建 |
| **Withdrawal** | `wallet_withdrawals` / `wallet_withdrawal_addresses` | `internal/modules/walletwithdrawal/domain/withdrawal.go:20,57` | `registry.go:63-64` | `withdrawal_no` 唯一、`request_amount/fee_amount/net_amount`、`status` 6 态、`reference` 唯一 |
| **10-Level Affiliate** | `affiliate_profiles` / `affiliate_clicks` / `affiliate_commissions` / `affiliate_withdraw_requests` | `internal/modules/affiliate/domain/commission.go:10` | `registry.go:56-59` | `beneficiary_user_id`、`source_user_id`、`level` (1-10)、`commission_type`；多级别唯一索引 `idx_affiliate_commission_multilevel_unique` (order_id, commission_type, beneficiary_user_id, level) |
| **C2C** | `c2c_payment_methods` / `c2c_listings` / `c2c_trades` / `c2c_disputes` / `c2c_risk_signals` | `internal/modules/c2c/domain/trade.go:10` | `registry.go:108-112` | `trade_no` 唯一、`buyer_receive_usdt`、`status`、`expired_at`；幂等唯一索引 `idx_c2c_trade_idem` (buyer_user_id, idempotency_key) |
| **Ticket** | `support_ticket_categories` / `support_tickets` / `support_ticket_messages` / `support_ticket_attachments` / `support_ticket_audits` | `internal/modules/supportticket/domain/ticket.go:6` | `registry.go:113-117` | `ticket_no` 唯一、`status/priority`、`user_unread_count/admin_unread_count` |
| **Site Builder** | `home_entries` / `discovery_blocks` / `site_audit_logs` | `internal/modules/sitebuilder/domain/home_entry.go:6` | `registry.go:118-120` | `key` 唯一、`action_type/action_target`、`sort_order`、`enabled` |

> 注：任务描述中提到的 `ledger_entries` / `withdrawals` / `tickets` / `site_config` / `site_blocks` 表名，在本项目中实际命名为 `wallet_transactions` / `wallet_withdrawals` / `support_tickets` / `home_entries`+`discovery_blocks`（无 site_config/site_blocks 表）。SQL 字段名均以上述实际结构体为准。

### 1.3 有序数据回填（AutoMigrate 之后依次执行）

`registry.go:125-186` 依次调用：

1. `ensureUserOAuthIdentityUserProviderUniqueIndex`（`migrations.go:456`）
2. `backfillPendingOrderRiskIPs`（`registry.go:258`）
3. `resellerstore.Migrate(db)`（注册 9 张 reseller 表）
4. `migrateCartSKUUniqueIndex`（`migrations.go:433`）
5. `ensureProductSKUMigration` / `ensureManualStockRemainingMigration` / `ensureCategoryParentMigration`
6. `ensurePaymentProviderBepusdtRenameMigration` / `ensurePaymentChannelBepusdtConfigMigration` / `ensurePaymentFeePolicyMigration` / `ensureOrderRefundPaymentFeeMigration` / `ensureOrderItemOriginalPriceMigration`
7. `BackfillInviteCodes(db)`（`invitation.go:21`）
8. `ensureCartForeignKeyConstraints` / `ensureProcurementOrderForeignKeyConstraint`
9. `migrateAffiliateCommissionMultilevel()`（`affiliate_commission_multilevel.go:35`）
10. `migrateWalletDualBalance()`（`wallet_dual_balance.go:36`）
11. **`DropColumn products.price_currency`**（`registry.go:176-180`，见第 2 节）
12. `SeedSupportTicketCategories` / `SeedHomeEntries`（FirstOrCreate 幂等 seed）

---

## 2. 只 ADD 不 DROP 确认证据

### 2.1 结论

**几乎全部是 ADD（建表/加列/加索引/UPDATE 回填），但存在 1 处 DROP COLUMN 和 2 处 DROP INDEX。没有 DROP TABLE。**

### 2.2 DROP 操作全清单

| 文件:行号 | 操作 | 对象 | 性质 |
|---|---|---|---|
| `registry.go:177` | `DropColumn` | `products.price_currency` | **唯一的 DROP COLUMN**。这是一个被废弃的旧列，迁移逻辑：`if HasColumn(...) { DropColumn(...) }`。仅在极旧库上存在该列时执行一次。 |
| `affiliate_commission_multilevel.go:138` | `DropIndex` | `idx_affiliate_commission_unique` | 旧唯一索引替换为多级别唯一索引（先 drop 旧、再 create 新）。 |
| `migrations.go:438` | `DropIndex` | `idx_cart_user_product` | 购物车旧唯一索引替换为 (user_id, product_id, sku_id) 新索引。 |

### 2.3 ADD 操作证据

- GORM `AutoMigrate()` 本身只新建表/列/索引/约束，不删表、不删活跃列（GORM 官方行为）。
- `wallet_dual_balance.go:55-76`：`HasColumn` 判断后 `AddColumn`，旧 `balance` 列**显式保留不删**（注释 L35："不删除旧 balance 列（staged migration）"）。
- `affiliate_commission_multilevel.go:52-66`：`AddColumn beneficiary_user_id / source_user_id / level`，先 `HasColumn` 再补。
- `invitation.go:55-64`：`CREATE UNIQUE INDEX IF NOT EXISTS`，幂等。
- 所有数据回填都是 `UPDATE ... WHERE 未回填条件`，不删除历史行。

### 2.4 对生产升级的含义

- 升级是向前兼容的：旧二进制 + 新库（多了列/表）可读；新二进制 + 旧库（缺列）由 AutoMigrate 补齐。
- **唯一需要注意**：`products.price_currency` 列会被 DROP。如果回滚到旧版本且旧代码仍读这个列，会报错。但该列已废弃，旧代码应不再依赖。上线前请确认无旧版本仍在写 `price_currency`。

---

## 3. 本地 SQLite 幂等验证结果（实际执行）

### 3.1 验证方式

新增测试文件 `internal/bootstrap/database/migrations/idempotency_verify_test.go`：

1. 创建临时文件 SQLite（`t.TempDir()/hcz_idempotency_verify.db`，开启 `foreign_keys=1`）
2. 第一次执行完整 `AutoMigrate()`（与 `main.go:143` 同一入口）
3. `PRAGMA table_info` 记录所有 77 张表的列数
4. 第二次执行同一 `AutoMigrate()`
5. 对比前后表数量 + 每表列数，断言完全一致

### 3.2 实际测试输出

```
=== RUN   TestIdempotencyVerifyOnTempFile
idempotency_verify_test.go:46: first AutoMigrate: OK
idempotency_verify_test.go:53: schema after first migrate: 77 tables
idempotency_verify_test.go:62: second AutoMigrate: OK
idempotency_verify_test.go:89: IDEMPOTENCY VERIFICATION PASSED: 77 tables, second run produced no schema change
--- PASS: TestIdempotencyVerifyOnTempFile (38.95s)
PASS
ok  github.com/Aether-v1/hcz/internal/bootstrap/database/migrations  40.593s
```

同时现有测试 `TestAutoMigrateOwnsResellerSchemaAndCrossModuleConstraints`（in-memory SQLite 连跑两遍 AutoMigrate）也 PASS：

```
--- PASS: TestAutoMigrateOwnsResellerSchemaAndCrossModuleConstraints (0.23s)
```

### 3.3 77 张表清单（首次迁移后）

```
admins, admin_login_logs, after_sale_tickets, affiliate_clicks, affiliate_commissions,
affiliate_profiles, affiliate_withdraw_requests, api_credentials, authz_audit_logs,
banners, card_secret_batches, card_secrets, cart_items, categories, channel_clients,
c2c_disputes, c2c_listings, c2c_payment_methods, c2c_risk_signals, c2c_trades,
coupon_usages, coupons, discovery_blocks, downstream_order_refs, email_verify_codes,
fulfillments, gift_card_batches, gift_cards, home_entries, media, member_level_prices,
member_levels, notification_logs, order_items, order_refund_records, order_risk_lock_keys,
orders, payment_channels, payments, post_categories, post_products, posts,
procurement_orders, product_mappings, product_skus, products, promotions,
reconciliation_items, reconciliation_jobs, reseller_balance_accounts, reseller_domains,
reseller_ledger_entries, reseller_order_snapshots, reseller_product_settings,
reseller_profiles, reseller_related_accounts, reseller_site_configs, reseller_withdraw_requests,
settings, site_audit_logs, site_connections, sku_mappings, support_ticket_audits,
support_ticket_attachments, support_ticket_categories, support_ticket_messages, support_tickets,
telegram_broadcasts, user_login_logs, user_notifications, user_oauth_identities, users,
wallet_accounts, wallet_recharge_orders, wallet_transactions, wallet_withdrawal_addresses,
wallet_withdrawals
```

**结论：AutoMigrate 在 SQLite 上可重复执行且幂等，77 张表二次运行无任何 schema 变化。**

---

## 4. 资产守恒验证 SQL 模板（PostgreSQL 语法）

> 字段名均来自实际结构体：
> - `wallet_accounts.available_balance` / `frozen_balance`（`account.go:13-14`）
> - `wallet_transactions.direction` = `in` / `out`（`constants.go:164-165`）
> - `wallet_transactions.amount` decimal(20,2)
> - 所有表带 `deleted_at` 软删除过滤

### 4.1 迁移前快照（在 staging clone 上执行）

```sql
-- (1) 钱包总资产
SELECT
  COUNT(*)                              AS acct_count,
  COALESCE(SUM(available_balance),0)    AS sum_available,
  COALESCE(SUM(frozen_balance),0)      AS sum_frozen,
  COALESCE(SUM(available_balance + frozen_balance),0) AS sum_total
FROM wallet_accounts
WHERE deleted_at IS NULL;

-- (2) 流水 in/out 汇总
SELECT
  COUNT(*)                                                        AS txn_count,
  COALESCE(SUM(amount) FILTER (WHERE direction='in'),0)           AS sum_in,
  COALESCE(SUM(amount) FILTER (WHERE direction='out'),0)          AS sum_out
FROM wallet_transactions
WHERE deleted_at IS NULL;
```

### 4.2 迁移后快照

同上两条 SQL，在 AutoMigrate 完成后再跑一次。

### 4.3 守恒公式

```
迁移前 sum_total  ==  迁移后 sum_total          （钱包总资产不变）
迁移前 sum_in     ==  迁移后 sum_in
迁移前 sum_out    ==  迁移后 sum_out
```

跨用户校验（理论等式，迁移前后应保持成立）：

```sql
-- 净流入应等于当前钱包总资产
SELECT
  (COALESCE(SUM(amount) FILTER (WHERE direction='in'),0)
   - COALESCE(SUM(amount) FILTER (WHERE direction='out'),0)) AS net_flow,
  (SELECT COALESCE(SUM(available_balance + frozen_balance),0)
     FROM wallet_accounts WHERE deleted_at IS NULL)         AS wallet_total
FROM wallet_transactions
WHERE deleted_at IS NULL;
-- net_flow 应 ≈ wallet_total（差异由提现已扣未打款、C2C 在途冻结等在途业务造成，迁移前后差值应一致）
```

> 说明：`c2c_freeze` / `c2c_unfreeze` 是 available↔frozen 内部转移（不改变个人总资产），`c2c_settle`/`c2c_receive` 是跨用户转移（不改变系统总盘）。迁移只加列不改金额，所以 sum_total 迁移前后必须严格相等。

---

## 5. PostgreSQL 生产部署配置模板

基于 `config.yml.example:28-35` 的 `database:` 段，生产用 PostgreSQL：

```yaml
database:
  driver: postgres
  # DSN（lib/pg 关键字格式，gorm.io/driver/postgres 底层为 pgx）
  # - sslmode=require：生产强制 TLS；若自签证书请用 verify-full 并配 sslrootcert
  # - TimeZone=Asia/Shanghai：连接会话时区；应用内统一用 UTC（见 gormdb/db.go:44 NowFunc=UTC），
  #   此参数仅影响 now()/pg_dump 输出展示，不改变业务时间戳语义
  dsn: "host=__PG_HOST__ port=5432 user=__PG_USER__ password=__PG_PASS__ dbname=hcz sslmode=require TimeZone=Asia/Shanghai"
  pool:
    max_open_conns: 25                 # 生产推荐：CPU 核数 * 2 ~ 50；25 是中小规模稳妥值
    max_idle_conns: 5                  # 空闲连接保留数，避免频繁建连
    conn_max_lifetime_seconds: 300     # 单连接最长存活 5 分钟，配合 PgBouncer/防火墙 idle timeout
    conn_max_idle_time_seconds: 60      # 空闲 60s 后回收
```

### 关键说明

| 参数 | 推荐值 / 说明 |
|---|---|
| `sslmode` | 生产必须 `require`（强制 TLS，不校验证书）或 `verify-full`（自签 CA 时用，配 `sslrootcert`）。禁止 `disable`。 |
| `TimeZone` | 应用 `NowFunc` 固定 UTC（`gormdb/db.go:44`），DB 列存的是 timestamp。DSN TimeZone 只影响会话内 `now()` 显示，建议与业务对账时区一致。 |
| `max_open_conns` | SQLite 本地开发是 1（单写）；PostgreSQL 可并发，推荐 25。PgBouncer 前置时可更小。 |
| `max_idle_conns` | 5，避免连接抖动。 |
| `conn_max_lifetime_seconds` | 300s，低于云厂商 RDS / PgBouncer 的默认 idle timeout（通常 600s）。 |
| 密码安全 | 不要写进 config.yml 提交 git；建议用环境变量 `PGPASSWORD` 或 secrets manager 注入。 |

---

## 6. PostgreSQL 版本要求

### 6.1 代码中使用的 PostgreSQL 特性

grep `internal/` 下的 SQL 特性：

| 特性 | 使用位置 | 说明 |
|---|---|---|
| `clause.Locking{Strength: "UPDATE"}`（即 `SELECT ... FOR UPDATE`） | wallet、c2c、order、affiliate、cardsecret、fulfillment、payment、withdrawal、giftcard、coupon、userauth 等 20+ 处 | 行锁。`order_store.go:833` 注释明确："SQLite 上 clause.Locking 是 no-op，PostgreSQL 上是真锁。" |
| `clause.OnConflict{DoNothing: true}` | `order/infrastructure/gormstore/order_store.go:715` | 依赖 `INSERT ... ON CONFLICT DO NOTHING`，PostgreSQL 9.5+ 支持。 |
| 部分唯一索引 `CREATE UNIQUE INDEX ... WHERE deleted_at IS NULL` | `invitation.go:58` | PostgreSQL partial index，9.x+ 支持。 |
| `FILTER (WHERE ...)` 聚合 | 本报告 4.3 节 SQL 模板（代码中未直接用，但供运维对账用） | PostgreSQL 9.4+。 |
| `decimal(20,2)` | 所有金额字段 | PostgreSQL numeric，全版本支持。 |

### 6.2 未使用的高级特性

- 无 `JSONB` / `GIN` / `GIST` 索引（grep 无命中）。
- 无 `SELECT FOR UPDATE SKIP LOCKED`、无 CTE 递归、无窗口函数依赖。
- 无存储过程 / 触发器（schema 纯表 + 索引）。

### 6.3 最低版本建议

**PostgreSQL 14+。**

理由：
- 代码实际用到的特性（FOR UPDATE / ON CONFLICT / partial index）9.6 就够；
- 但 14+ 有更好的原地索引建大表性能、逻辑复制改进、`pg_basebackup` 流式复制稳定性；
- pg_dump / pg_restore 工具链建议与服务端大版本一致，避免低版本 pg_dump 备份高版本库时报 "server version X, pg_dump version Y" 错误。

---

## 7. SELECT FOR UPDATE 资金路径验证证据

### 7.1 Wallet 扣款 / 加款路径

| 文件:行号 | 函数 | 用途 |
|---|---|---|
| `internal/modules/wallet/infrastructure/gormstore/store.go:65-79` | `GetAccountByUserIDForUpdate` | `Clauses(clause.Locking{Strength:"UPDATE"})` 锁 `wallet_accounts` 行 |
| `internal/modules/wallet/application/credit.go:152,163` | 加款路径 | 调用上述 ForUpdate，事务内扣加余额 |
| `internal/modules/wallet/application/freeze.go:47,73` | C2C freeze/unfreeze | 调用上述 ForUpdate，锁账户行 |
| `internal/modules/walletwithdrawal/application/create.go:108` | 提现申请（扣款） | `tx.Wallets().GetAccountByUserIDForUpdate` |
| `internal/modules/walletwithdrawal/application/cancel.go:40` | 提现取消（退款） | 同上 |
| `internal/modules/walletwithdrawal/application/admin.go:73` | 管理员审批 | 同上 |
| `internal/modules/wallet/infrastructure/gormstore/store.go:237-251` | `GetRechargeOrderByPaymentIDForUpdate` | 充值单锁 |

### 7.2 C2C freeze / unfreeze / settle 路径

| 文件:行号 | 函数 | 用途 |
|---|---|---|
| `internal/modules/c2c/infrastructure/gormstore/store.go:151-164` | `GetListingByIDForUpdate` | 锁挂单 |
| `internal/modules/c2c/infrastructure/gormstore/store.go:249-262` | `GetTradeByIDForUpdate` | 锁交易单 |
| `internal/modules/c2c/infrastructure/gormstore/store.go:360-373` | `GetDisputeByTradeIDForUpdate` | 锁争议单 |
| `internal/modules/c2c/application/trade.go:57` | 创建交易 | `GetListingByIDForUpdate` |
| `internal/modules/c2c/application/trade.go:167,211,268` | 付款 / 确认 / 取消 | `GetTradeByIDForUpdate` |
| `internal/modules/c2c/application/expire.go:21` | 超时自动取消 | `GetTradeByIDForUpdate` |
| `internal/modules/c2c/application/dispute.go:30` / `arbitration.go:39` | 申诉 / 仲裁 | `GetTradeByIDForUpdate` |

### 7.3 PostgreSQL 下锁是否真实生效

- GORM `clause.Locking{Strength: "UPDATE"}` 在 postgres 驱动下生成 `SELECT ... FOR UPDATE`，在事务内对命中行加 ROW EXCLUSIVE 锁。
- SQLite 驱动下 `clause.Locking` 是 **no-op**（`order_store.go:833` 注释明确说明），所以本地 SQLite 并发测试只能验证业务逻辑，不能验证锁语义；**PostgreSQL 是这套行锁真正生效的环境**。
- 资金路径（钱包加减款、提现、C2C freeze/settle）全部在 `db.Transaction(func(tx)...)` 包裹内先 `GetAccountByUserIDForUpdate` 再 `UpdateAccount`，PostgreSQL 下会串行化并发扣款，防止超卖/超扣。

---

## 8. Backup / Restore / Migration Preflight 脚本

三个脚本已写入 `scripts/` 目录：

| 脚本 | 路径 | 作用 |
|---|---|---|
| 备份 | `scripts/prod_backup.ps1` | pg_dump -F c 全量备份 + config.yml + uploads zip + hcz.exe + frontend dist，每产物 SHA256，输出 manifest CSV |
| 恢复验证 | `scripts/prod_restore_verify.ps1` | createdb 临时库 → pg_restore → 跑 users/wallet_accounts/orders/wallet_transactions 校验查询 → 清理临时库 |
| 迁移预检 | `scripts/prod_migration_preflight.ps1` | staging clone 上：pg_dump schema before → 记录资产 → 启动新 binary 触发 AutoMigrate → 记录资产 after → 守恒对比 → 二次启动幂等验证 → schema diff |

### 8.1 快速用法

```powershell
# 备份（在生产服务器上）
$env:PGPASSWORD = '***'
.\scripts\prod_backup.ps1 -PGHost pg.internal -PGUser hcz -PGDatabase hcz `
    -AppDir E:\app\hcz -BackupRoot E:\backup\hcz

# 恢复验证（在任意能连 PG 的机器上）
.\scripts\prod_restore_verify.ps1 -DumpFile E:\backup\hcz\20261005_210000\hcz_20261005_210000.dump `
    -PGHost pg.internal -PGUser hcz -PGPassword '***'

# 迁移预检（在 staging clone 上）
.\scripts\prod_migration_preflight.ps1 `
    -NewExe E:\app\hcz\new\hcz.exe `
    -StagingDSN "host=staging-pg port=5432 user=hcz password=*** dbname=hcz_staging sslmode=require" `
    -PGHost staging-pg -PGUser hcz -PGDatabase hcz_staging -PGPassword '***'
```

---

## 9. 阻塞项

| # | 阻塞项 | 影响 | 解锁方式 |
|---|---|---|---|
| 1 | **本地无 PostgreSQL 实例** | 第 3 节幂等验证仅在 SQLite 完成；PostgreSQL 上的 AutoMigrate 真实验证、FOR UPDATE 锁语义验证、pg_dump/restore 全流程均未在本地跑通 | 准备一台 staging PostgreSQL 14+，克隆生产库后执行 `prod_migration_preflight.ps1` 和 `prod_restore_verify.ps1` |
| 2 | 无生产 / staging 真实数据 | 资产守恒 SQL 模板（第 4 节）未在真实数据上跑过 | staging clone 后用脚本自动对比 |
| 3 | `products.price_currency` DropColumn | 若线上有旧版本 binary 仍在读此列，回滚会报错 | 上线前确认旧版本已无该列读写；或临时注释掉 `registry.go:176-180` 的 DropColumn 延迟清理 |
| 4 | pg_dump / pg_restore 未在本机验证 | 脚本语法经过审阅，但未实际执行 | staging 环境首次跑通后固化到运维手册 |

---

## 10. 附：本次新增/修改文件清单

| 文件 | 说明 |
|---|---|
| `internal/bootstrap/database/migrations/idempotency_verify_test.go` | 本次新增：临时文件 SQLite 幂等验证测试（PASS） |
| `scripts/prod_backup.ps1` | 本次新增：生产备份脚本 |
| `scripts/prod_restore_verify.ps1` | 本次新增：恢复验证脚本 |
| `scripts/prod_migration_preflight.ps1` | 本次新增：迁移预检脚本 |
| `MIGRATION_AND_POSTGRES_REPORT.md` | 本报告 |

未修改任何业务代码。
