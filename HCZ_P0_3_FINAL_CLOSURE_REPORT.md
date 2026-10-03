# HCZ P0-3 Final Closure Report

## Final Verdict：**PASS WITH CONDITIONS**

## 本轮完成（剩余 4 项）
1. **User Cancel 显式状态机校验**：`CancelOrder`（order_service_child.go）由「仅 pending_payment」改为 `ordermachine.AllowedByUser(normalize(status), canceled)`——只允许 pending_recharge→canceled，processing/completed/failed/canceled 全拒；继续复用现有 cancel + USDT refund + commission rollback 链。
2. **refund_status 物理落库**：order domain 新增 `RefundStatus string gorm:"not null;default:'none'"`（none/partial/full），AutoMigrate 非破坏性；历史 partially_refunded/refunded 仍由 Normalize 兼容，不批量改旧数据。
3. **parent/child failed**：复用现有 cancelOrderWithChildren 级联与 cancelSingleOrderInTx，未新造第二套，无双退款/双 rollback 新路径。
4. **状态写入扫描**：Admin 写入口 UpdateOrderStatus 已 CanTransition 守卫（豁免 legacy refund 状态）；User Cancel 已 AllowedByUser。

## 验证
- `go build ./...` EXIT=0
- order/application、ordermachine、integrationtest/application 全 PASS

## 明确回答
- 是否还有生产状态写入绕过 machine？Admin 主写入口与 User Cancel 已守卫；expire/system 内部取消走同一 cancelOrderWithChildren（系统自动，非用户可触达）。
- User Cancel 是否完整？**是**（AllowedByUser + 复用退款链）。
- refund_status 是否真实落库？**列已加（默认 none）**；退款流程回写 partial/full 为下一增量（当前 DTO 已从 Normalize 推断）。
- parent/child failed 是否安全？复用成熟级联，未引入双副作用。
- 前后端是否统一五态？**后端 API 已归一五态 + refund_status**；前端新五态文案/按钮为最小待办。
- P0-3 是否可封板？**暂不完全封板**——差前端五态文案与退款回写 refund_status 列两处收尾。

## 收尾 P1
- 前端五态 i18n 文案与按 transition 显隐按钮（后端已归一，前端只做展示）。
- 退款流程真实回写 refund_status=partial/full。

未引入 frozen balance，未改 P0-2 资金口径/汇率/Gateway/会员/商品。
