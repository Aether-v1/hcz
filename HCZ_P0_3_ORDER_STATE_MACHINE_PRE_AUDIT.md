# HCZ P0-3 Order State Machine Pre-Audit（只读审计，未改代码）

## 0. 背景与边界
- P0-2 已冻结：Site/USDT 资金模型、Exchange Rate、Wallet、Refund/Commission 金额口径、Frontend Contract 字段一律不动。
- HCZ 最终 Business Order **主状态仅 5 个**：待充值 / 处理中 / 已完成 / 失败 / 已取消。
- HCZ 模式：无供应商自动履约；用户下单即扣 USDT；Admin 人工去外部平台完成后手动标记状态。**不设计供应商 API 状态机。**

## 1. 当前真实状态模型（代码证据：internal/constants/constants.go:4-14）
主订单 `orders.status` 现有 9 个 DB 值：
| DB 值 | 含义 |
|---|---|
| pending_payment | 待支付 |
| paid | 已支付（钱包已扣） |
| fulfilling | 履约中 |
| partially_delivered | 部分交付 |
| delivered | 已交付 |
| completed | 已完成 |
| partially_refunded | 部分退款 |
| refunded | 已退款 |
| canceled | 已取消 |

其它独立状态维度（constants.go:24-39 等）：
- `payment_status`：initiated/pending/success/failed/expired（充值/支付单，非商品主状态）
- `fulfillment_status`：pending/delivered（履约子状态）
- 退款：由 `refunded_amount` + refund records 体现；无独立 `refund_status` 列（用主状态 partially_refunded/refunded 兼任）
- 售后/未收到/确认：当前无独立列（HCZ 暂未要求）

## 2. 状态写入入口（审计）
- 下单：wallet-only 下单成功后订单进入 `paid`（钱包已扣），不经过 gateway。
- Admin：`transport/http/admin_handler.go` 有手动改状态入口；procurement 模块会把 paid→fulfilling（提交履约），拒绝则回 paid。
- Payment/Webhook：仅充值路径写 payment_status；商品订单 webhook 不再产生 gateway payment（P0-1 已收口）。
- Refund：`order/application/refund/service.go` 把订单标记 partially_refunded / refunded（本轮 P0-2 已修金额口径）。
- 自动任务：`consumer_order.go` 对 delivered/completed 做后续；migrations 对 pending_payment 做风控/超时扫描。
- Affiliate：订单 `paid` 时触发佣金（OnOrderPaid）。

## 3. Current → HCZ 5-State 映射
| 当前 DB 状态 | HCZ 主状态 | 处置 |
|---|---|---|
| pending_payment | 待充值 | MERGE/过渡（wallet-only 下几乎不停留，保留兼容历史单） |
| paid | **待充值** | KEEP→重命名语义（钱包已扣、等 Admin 充值） |
| fulfilling / partially_delivered | **处理中** | MERGE（二者合并为处理中） |
| delivered / completed | **已完成** | MERGE（二者合并为已完成） |
| （无） | **失败** | BUILD 新值 `failed`（Admin 人工充值失败时用） |
| canceled | **已取消** | KEEP |
| partially_refunded / refunded | 主状态保持已完成/已取消，**独立 refund_status** | MOVE：退款信息不进主状态 |

- KEEP：paid(→待充值语义)、canceled。
- MERGE：fulfilling+partially_delivered→处理中；delivered+completed→已完成。
- MOVE 到独立子状态：refund（refund_status: none/partial/full）、fulfillment metadata（保留 fulfillment_status 列）。
- BUILD：新增主状态 `failed`。
- DROP：partially_delivered/delivered/partially_refunded/refunded 不再作为**主状态展示值**（DB 值保留做历史兼容，映射层归并）。

## 4. 合法迁移矩阵（人工履约版）
| From | To | 操作方 | 说明 |
|---|---|---|---|
| 待充值(paid) | 处理中(fulfilling) | Admin | 开始外部充值 |
| 待充值(paid) | 已取消(canceled) | Admin / User(时限内) | 取消→自动退回 USDT |
| 待充值(paid) | 失败(failed) | Admin | 外部充值失败→自动退回 USDT |
| 处理中(fulfilling) | 已完成(completed) | Admin | 外部充值成功 |
| 处理中(fulfilling) | 失败(failed) | Admin | 失败→自动退回 USDT |
| 已完成(completed) | （终态） | — | 不再改主状态；部分退款走 refund_status=partial |
| 失败(failed) | （终态） | — | 已退款，禁止再改 |
| 已取消(canceled) | （终态） | — | 已退款 |

- User 可操作：时限内取消待充值单。
- Admin：其余所有迁移。
- 系统自动：失败/取消触发自动退款（USDT 快照口径，P0-2 已冻结，不改金额算法）。
- 已完成后部分退款：主状态保持「已完成」，`refund_status=partial`。

## 5. 自动退款规则
- 待充值→已取消、待充值/处理中→失败：必须自动按订单 USDT 快照退款（复用现有 refund wallet 链，不改金额口径）。
- 已完成→失败：不允许（终态）。

## 6. Migration 风险
- 主状态收敛用**映射层**而非物理改写：新增 `failed`，旧值（delivered/partially_refunded 等）保留，查询/展示层归并到 5 态。
- 不做破坏性 UPDATE；历史订单 NULL/旧值可正常映射。
- 建议新增 `refund_status` 列（nullable，默认 none）替代主状态兼任退款。
- 风险：前端现有 9 态映射（`utils/status.ts`、OrdersPanel 筛选项）需同步收敛到 5 态展示。

## 7. 最小实施范围（P0-3 待批）
1. constants 新增 `OrderStatusFailed="failed"`；引入 5 态归一映射函数。
2. Admin 状态迁移入口加白名单校验（按矩阵），失败/取消自动退款。
3. 新增/复用 `refund_status` 列；已完成部分退款不改主状态。
4. User 取消订单入口按时限 + 自动退款。
5. 前端状态映射收敛到 5 态文案，退款/履约信息用子状态展示。

## 8. 测试矩阵
- 合法迁移（Admin）全部通过；非法迁移被拒。
- User 仅能取消待充值；越权改状态 403。
- 待充值→失败/取消自动退款 USDT 正确。
- 已完成部分退款后主状态仍已完成、refund_status=partial。
- 旧 9 态历史单正确映射到 5 态展示。
- 退款金额口径不变（P0-2 回归）。

## 结论
- 不建议把退款/售后/付款/履约塞进主状态；主状态收敛到 5 态，refund/fulfillment 走独立子状态。
- 本轮纯审计，未改代码。最小改动安全，不触碰 P0-2 冻结资金面。
