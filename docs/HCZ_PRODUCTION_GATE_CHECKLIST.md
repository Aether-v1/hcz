# HCZ 生产上线门禁检查清单（Production Gate Checklist）

> 状态：**模板 / Checklist Template**。本清单不填生产实测值，所有「实际结果」默认 `PENDING`，「负责人」统一 `ADMIN`。
> 配套文档：`docs/HCZ_PRODUCTION_PRICING_CONFIG_TEMPLATE.md`（定价配置模板）。
> 字段口径基于真实后端 Contract（profitguard / exchangerate / catalog / wallet / affiliate）。

- 生成日期：2026-10-06
- 环境：PRODUCTION（待上线）
- 执行窗口：上线当天、正式切流前

---

## 0. 总体状态汇总

| # | 检查项 | 状态 | 实际结果 |
|---|---|---|---|
| 1 | DB Backup | PENDING | PENDING |
| 2 | Migration | PENDING | PENDING |
| 3 | Cost Data | PENDING | PENDING |
| 4 | FX（汇率源） | PENDING | PENDING |
| 5 | Profit Guard | PENDING | PENDING |
| 6 | C2C Migration | PENDING | PENDING |
| 7 | Affiliate Reconciliation | PENDING | PENDING |
| 8 | Wallet Reconciliation | PENDING | PENDING |
| 9 | Frontend User Build | PENDING | PENDING |
| 10 | Frontend Admin Build | PENDING | PENDING |
| 11 | API Smoke | PENDING | PENDING |
| 12 | E2E 跨域资金流 | PENDING | PENDING |
| 13 | Logs | PENDING | PENDING |
| 14 | Rollback | PENDING | PENDING |

**总体结论：PENDING**（任一检查项为 FAIL / BLOCKED，即不得切流）

---

## 1. 逐项检查

> 状态枚举：`PENDING`（未开始）/ `PASS`（通过）/ `FAIL`（失败）/ `BLOCKED`（被阻塞）。
> 实际结果列填实测输出摘要或关键数值，禁止只写「应该可以」。

### 1. DB Backup

| 项 | 内容 |
|---|---|
| 检查项名称 | 生产数据库全量备份已完成且可恢复 |
| 检查方法/命令 | `pg_dump`（或对应 DB 工具）导出全量；在恢复环境验证一次可恢复性；记录备份文件路径与大小 |
| 预期结果 | 备份文件生成成功；抽样恢复验证通过；备份时间早于本次 Migration |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 2. Migration

| 项 | 内容 |
|---|---|
| 检查项名称 | 数据库迁移已执行，新增字段到位 |
| 检查方法/命令 | 执行迁移后核对：`products` 表存在 `cost_price_amount`、`is_cost_exempt` 列；`orders` 表存在 `exchange_rate / exchange_rate_source / exchange_rate_at` 列；迁移版本号与代码一致 |
| 预期结果 | 迁移全部 up；上述列存在；无 pending migration |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 3. Cost Data

| 项 | 内容 |
|---|---|
| 检查项名称 | 所有 active 商品已录入真实成本价（COST_MISSING = 0） |
| 检查方法/命令 | SQL：`SELECT COUNT(*) FROM products WHERE is_active = true AND deleted_at IS NULL AND cost_price_amount <= 0 AND is_cost_exempt = false;` |
| 预期结果 | 查询结果 = 0（COST_MISSING=0）。即每个 active 商品要么 `cost_price_amount > 0`，要么 `is_cost_exempt = true` |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 4. FX

| 项 | 内容 |
|---|---|
| 检查项名称 | 汇率源配置正确（自动源 + 手动兜底 + 新鲜度阈值） |
| 检查方法/命令 | 查 settings KV `global_exchange_rate`：`auto_enabled = true`、`provider = coingecko`、`api_key` 已配置；`manual_rate > 0`；`max_auto_rate_age_minutes > 0`；观察最近 `exchange_rate_refresh_ok` 日志时间戳 |
| 预期结果 | 自动源可成功拉取；Manual Fallback Rate 已设置为正值；Max Auto Rate Age 已配置；最近一次拉取成功时间在 MaxAutoRateAge 窗口内 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 5. Profit Guard

| 项 | 内容 |
|---|---|
| 检查项名称 | Profit Guard 配置已确认，Pricing Preview 验证通过 |
| 检查方法/命令 | 查 settings KV `profit_guard_config`：`enabled / require_cost_price / minimum_profit_amount_cny / minimum_profit_rate_percent / rate_safety_buffer_percent` 均已由 ADMIN 确认（非模板默认值）；在 Admin Pricing Preview 接口对代表性商品逐单预览 |
| 预期结果 | 配置项无 `ADMIN_CONFIRM_REQUIRED` 残留；Pricing Preview 对正利润单 PASS、对负利润单拒单（guard_reason=`product_unprofitable`）、对成本缺失单拒单（`product_cost_not_configured`）、对豁免商品 PASS |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 6. C2C Migration

| 项 | 内容 |
|---|---|
| 检查项名称 | 历史 active listing 迁移已执行，迁移后对账通过 |
| 检查方法/命令 | 确认无历史 active listing，或已执行 C2C listing 冻结/迁移脚本；迁移前后 listing 数与冻结金额对账（参考 `docs/audits/HCZ_C2C_LISTING_FREEZE_MIGRATION_FINAL.md`） |
| 预期结果 | 历史 listing 要么为空，要么已全部迁移/处理；迁移后 listing 冻结金额与 wallet frozen 一致，无漂移 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 7. Affiliate Reconciliation

| 项 | 内容 |
|---|---|
| 检查项名称 | Affiliate 资金对账 PASS |
| 检查方法/命令 | 对每个 affiliate 账户校验恒等式：`可用余额 = credit − reversal − debt − settled − locked`（口径以 `docs/audits/HCZ_GO_AFFILIATE_FINANCE_CLOSURE_FINAL.md` 为准） |
| 预期结果 | 全量账户恒等式成立；无差异行 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 8. Wallet Reconciliation

| 项 | 内容 |
|---|---|
| 检查项名称 | 钱包资金对账 PASS |
| 检查方法/命令 | 逐用户校验：`available + frozen = total`；并校验 `frozen` 金额与 freeze entries 明细合计一致（参考 `docs/audits/HCZ_C2C_FUND_FREEZE_CHAIN_AUDIT.md`） |
| 预期结果 | 全量钱包守恒成立；frozen 与冻结流水逐条对平；守恒违规数 = 0 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 9. Frontend User Build

| 项 | 内容 |
|---|---|
| 检查项名称 | frontend/user typecheck + test + build 全部 PASS |
| 检查方法/命令 | `cd frontend/user`；依次执行 typecheck、单元测试、生产构建（`npm run` 对应脚本） |
| 预期结果 | typecheck 0 error；test 全绿；build 产物生成成功 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 10. Frontend Admin Build

| 项 | 内容 |
|---|---|
| 检查项名称 | frontend/admin typecheck + test + build 全部 PASS |
| 检查方法/命令 | `cd frontend/admin`；依次执行 typecheck、单元测试、生产构建 |
| 预期结果 | typecheck 0 error；test 全绿；build 产物生成成功 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 11. API Smoke

| 项 | 内容 |
|---|---|
| 检查项名称 | 核心 API 冒烟测试通过 |
| 检查方法/命令 | 对核心链路发真实请求：用户登录 → 商品列表 → 提交下单 → 钱包余额查询；核对返回码与关键字段 |
| 预期结果 | 四个链路均 200/业务成功；下单返回含 `exchange_rate / exchange_rate_source / exchange_rate_at` 快照；钱包查询余额与对账一致 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 12. E2E

| 项 | 内容 |
|---|---|
| 检查项名称 | 跨域资金流 E2E 测试全部 PASS |
| 检查方法/命令 | 执行 E2E 用例集，覆盖：正利润通过 / 负利润拒单 / Cost Missing 拒单 / FX 异常（stale 或不可用）/ Wallet-only 下单 / Refund / Partial Refund |
| 预期结果 | 7 个场景全部符合预期：拒单场景不产生资金流动；退款/部分退款后钱包守恒重新成立 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 13. Logs

| 项 | 内容 |
|---|---|
| 检查项名称 | 结构化日志已配置，关键事件可观测 |
| 检查方法/命令 | 确认结构化日志输出；触发一次各关键事件并查日志关键字：Profit Guard 拒单、FX 异常（`exchange_rate_refresh_failed`）、钱包守恒违规 |
| 预期结果 | 三类关键事件均有结构化日志（含订单号/用户/金额等上下文字段），可据此告警与追溯 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

### 14. Rollback

| 项 | 内容 |
|---|---|
| 检查项名称 | 回滚方案已准备并演练 |
| 检查方法/命令 | 演练三项：① 关闭 Profit Guard（`profit_guard_config.enabled=false`，热更新不影响在途订单）；② 回滚 C2C 迁移（按迁移脚本的逆向/恢复方式）；③ 回退前端版本（切回上一稳定构建产物） |
| 预期结果 | 三项均可在约定 RTO 内执行成功；回滚后系统行为与回滚前版本一致，无资金状态破损 |
| 实际结果 | PENDING |
| 负责人 | ADMIN |
| 状态 | PENDING |

---

## 2. 声明

**本清单 14 项检查全部为 PASS，才允许宣布 PRODUCTION READY 并正式切流。**

- 任一项目为 `FAIL` 或 `BLOCKED`：**禁止上线**，必须修复并重跑对应检查项。
- 任一项目仍为 `PENDING`：视为未完成，不得默认放行。
- 检查过程中产生的任何生产实际值、命令输出摘要，须回填到对应「实际结果」列，禁止事后补写「已通过」而无证据。

| 最终判定 | 签字 | 日期 |
|---|---|---|
| ADMIN_CONFIRM_REQUIRED（PRODUCTION READY = 是/否） | ADMIN_CONFIRM_REQUIRED | ADMIN_CONFIRM_REQUIRED |
