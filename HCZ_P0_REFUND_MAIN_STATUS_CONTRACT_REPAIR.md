# HCZ P0 Refund / Main-State Contract Repair

## Final Verdict：**PASS WITH CONDITIONS**

本次修复了真实 P0 合同冲突：Refund Service 越权把 Business Order 主状态写成旧 9 态（`partially_refunded` / `refunded`），违反 P0-3 已封板的五主状态合同。修复后退款链只写 `refund_status`，主状态唯一权威是 ordermachine。

---

## 一、审计结论（修复前）

| 路径 | 主状态由谁写 | refund_status 由谁写 | 依赖旧 partially_refunded/refunded |
|---|---|---|---|
| `AdminRefundToWallet(InTx)`（P0-2 正式 USDT 钱包退款链） | **退款链自己写**（越权） | 退款链写 | 是 |
| `AdminManualRefund(InTx)`（仅记账退款） | **退款链自己写**（越权） | 退款链写 | 是 |
| after-sale partial/full refund | 经 WalletRefunderAdapter → AdminRefundToWalletInTx → 被覆盖 | 同左 | 是 |
| failed 自动退款 | 状态机写 failed，若后续调退款则被覆盖 | — | 风险 |
| canceled 自动退款 | 状态机写 canceled，走 ReleaseWalletBalance（不写状态） | ReleaseWalletBalance 不写 refund_status | 否（但 refund_status 缺失） |
| parent/child 退款 | `applyParentRefundChildStatusUpdates` 批量写子单旧状态 | — | 是 |

**根因**：`refund/wallet.go:162-166` 与 `refund/service.go:396-400` 在退款成功后执行 `updates["status"] = OrderStatusRefunded/PartiallyRefunded`，并调用 `applyParentRefundChildStatusUpdates` 批量覆盖子单主状态。

---

## 二、修复内容（最小合同修复，未改资金模型）

### 生产代码
1. **`internal/modules/order/application/refund/wallet.go`**
   - 删除 `updates["status"] = OrderStatusRefunded / OrderStatusPartiallyRefunded`
   - 保留 `updates["refund_status"] = full / partial`
   - 删除 `applyParentRefundChildStatusUpdates(...)` 调用（退款不再驱动子单主状态）
   - 删除 `orderapp.SyncParentStatus(...)` 调用（子单退款不再触发父单主状态重算）
   - 移除未使用的 `orderapp` import

2. **`internal/modules/order/application/refund/service.go`**（AdminManualRefundInTx）
   - 同上：删除主状态写入、删除 parent/child 状态同步、保留 refund_status
   - 移除未使用的 `orderapp` import

3. **删除 `internal/modules/order/application/refund/child_status.go`**
   - `applyParentRefundChildStatusUpdates` 已无任何调用方，整文件删除
   - 该函数唯一作用就是批量写子单旧主状态，属于退款越权逻辑

### 测试更新（10 个旧断言用例）
- `integrationtest/refund/wallet_test.go`：5 个用例改为断言"主状态不变 + refund_status=partial/full + 子单主状态不变"
- `integrationtest/refund/service_test.go`：5 个用例同上
- 测试名保留（未重命名，避免扩大 diff），但断言语义已更新

### 未改动
- USDT 金额算法、Wallet 模型、Exchange Rate、五态定义、AfterSale 状态模型、API Contract
- `AdminRefundToWallet` / `AdminManualRefund` 对外签名完全兼容
- 旧调用方（Admin HTTP handler、after-sale adapter）无需修改

---

## 三、验证结果

| 验证项 | 结果 |
|---|---|
| `go build ./...` | **PASS**（EXIT=0） |
| aftersale real DB integration（4 用例） | **PASS** |
| ├ TestPartialRefundRealDB | status 保持 completed、after_sale_status=resolved、refund_status=partial、wallet +4.00 USDT、ledger 1 条 refund、refund record 1 条、ticket resolved |
| ├ TestFullRefundRealDB | refund_status=full、wallet +10.00 USDT、status 保持 completed |
| ├ TestRefundFailureRollsBack | 超额退款被拒，wallet/ledger/refund_record/refund_status/after_sale_status 全部不变 |
| └ TestDuplicateRefundIdempotent | 第二次退款被拒，wallet 只 credit 一次 |
| refund integration（全量） | **PASS** |
| order/application（含 ordermachine、aftersale 单测） | **PASS** |
| wallet（全量） | **PASS** |
| affiliate/commission（全量） | **PASS** |
| order/transport/http + presenter | **PASS** |

### 全量 `go test ./...` 中的失败（均为前置遗留/Windows 环境，非本次引入）
1. `internal/app/httpserver` — `TestAllAdminRoutesCoveredByBuiltinRoles`：exchangerate Admin 路由（GET/PUT/POST /admin/settings/exchange-rate）未加入 builtin role seeds，P0-2 遗留
2. `internal/architecture` — `TestDependencyRules`：exchangerate admin_handler import gin，P0-2 遗留；`TestOrderAdminHTTPLivesInTransport`：order/domain 6 文件、gormstore 9 文件超预算（P1 after-sale 新增 after_sale.go / aftersale_store.go）
3. `internal/logger` — `TestNewReleaseWritesToConfiguredFile`：Windows 文件占用 flaky
4. `internal/modules/order/infrastructure/gormstore` — 2 个 RiskGate 测试：TempDir RemoveAll cleanup 文件占用，已知 Windows flaky（断言本身通过）

以上 4 类均与本次退款主状态修复无关，已逐一核实根因。

---

## 四、全局扫描结论

搜索生产代码所有 `OrderStatusRefunded` / `OrderStatusPartiallyRefunded` 写入点：

| 分类 | 文件 | 说明 |
|---|---|---|
| **ACTIVE_WRITE_BLOCKER（已清零）** | refund/wallet.go、refund/service.go | 已删除主状态写入 |
| **已删除** | refund/child_status.go | 整文件删除 |
| **LEGACY_READ_ONLY / NORMALIZE_COMPAT** | constants.go、ordermachine/machine.go、order_service_child.go（豁免校验）、order_service_query.go、user_handler.go、notification/smtp、dashboard/*、reseller/*、procurement/lifecycle.go（calculateParentStatus 只读派生） | 仅读取/映射/统计旧状态，用于历史兼容展示，不产生新写入 |
| **TEST_FIXTURE** | reseller/integrationtest/order_test.go:346 | 测试构造历史数据，非生产 |

**新业务不再产生 `refunded` / `partially_refunded` 主状态。** 历史数据继续由 ordermachine.Normalize 兼容映射。

---

## 五、明确回答

1. **新退款链是否彻底不再写主状态？** **是**。`AdminRefundToWalletInTx` / `AdminManualRefundInTx` 只写 `refunded_amount` + `refund_status` + wallet + ledger + refund record + commission。
2. **failed/canceled 是否保持终态？** **是**。退款链不再覆盖主状态；状态机写的 failed/canceled 不会被退款冲掉。cancel 路径走 `ReleaseWalletBalance`（本身不写状态），同样安全。
3. **completed 售后退款是否保持 completed？** **是**。aftersale integration TestPartialRefundRealDB / TestFullRefundRealDB 已真实证明：退款后 `orders.status` 仍为 completed，`refund_status` = partial/full。
4. **parent/child 是否还存在退款驱动主状态写入？** **否**。`applyParentRefundChildStatusUpdates` 已删除，退款不再调用 `SyncParentStatus`。子单主状态由 ordermachine/履约路径决定。
5. **是否还会产生新的 refunded/partially_refunded 主状态？** **否**。生产代码 ACTIVE_WRITE_BLOCKER 已清零。
6. **是否可以重新解锁 After-Sale Handler + Route 阶段？** **可以**。退款链合同已修复，after-sale 后端核心（store/service/refund 复用/共享事务/真实 DB integration）全部就绪。

---

## 六、剩余 P1/P2（非本次范围，如实记录）

1. **canceled 订单 refund_status 未写入**：cancel 路径走 `ReleaseWalletBalance`，只 credit wallet + 写 ledger，不更新 `orders.refund_status`。已取消订单的 refund_status 仍为 none。建议后续让 cancel 链也写 refund_status=full（或在 Normalize 层推断）。
2. **failed 自动退款链需专项核实**：当前仅确认 cancel 路径有 `ReleaseWalletBalance`；failed 路径是否退款、走哪条链，需单独审计（本次未触碰）。
3. **procurement lifecycle.SyncParentStatus**：仍存在基于子单状态派生父单主状态的逻辑（含旧 refunded/partially_refunded 分支），由履约路径触发，非退款驱动。历史子单若含旧状态可能派生旧主状态。建议后续纳入 ordermachine 统一收口。
4. **P0-2 遗留**：exchangerate Admin 路由 RBAC builtin role 未注册、exchangerate handler import gin（architecture 测试）。
5. **P1 遗留**：order/domain、gormstore 文件数超 architecture 预算（after-sale 新增文件）。
6. **并发真实验证**：aftersale 并发双退款（refund vs refund、reject vs refund）尚未在 Linux/PostgreSQL 真实 FOR UPDATE 环境验证；SQLite 不支持真实行锁。当前单元层验证了 pending 锁语义与幂等。

---

## 七、修改文件清单

| 文件 | 作用 |
|---|---|
| `internal/modules/order/application/refund/wallet.go` | 删除主状态写入 + parent/child 状态同步 |
| `internal/modules/order/application/refund/service.go` | 同上（AdminManualRefundInTx） |
| `internal/modules/order/application/refund/child_status.go` | 删除（已无调用方） |
| `internal/modules/order/integrationtest/refund/wallet_test.go` | 5 用例断言更新为主状态不变 + refund_status |
| `internal/modules/order/integrationtest/refund/service_test.go` | 5 用例同上 |
