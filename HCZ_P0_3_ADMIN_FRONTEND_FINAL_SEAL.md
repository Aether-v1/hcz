# HCZ P0-3 Admin Frontend Final Seal

## Final Verdict：**PASS — P0-3 正式封板**

### 本轮完成
- Admin `utils/status.ts`：新增五态 label 映射与 badge class（pending_recharge/processing/failed/completed/canceled），旧 9 态保留仅作历史兼容渲染。
- Admin i18n 三语 order.status 块补齐 pending_recharge/processing/failed：
  - zh-CN 待充值/处理中/失败；zh-TW 待儲值/處理中/失敗；en Pending Recharge/Processing/Failed。
- Admin `npm run build` PASS（23.29s）。

### 全局扫描结论
- 旧 9 态文案：仅作 LEGACY_SAFE（历史订单 Normalize 后仍可能命中，仅渲染用）；新数据只命中五态 key。
- orderEmailTemplates 中 partially_refunded 为邮件模板场景 key（LEGACY_SAFE，不影响业务主状态操作）。
- 后端 API 已归一五态 + refund_status，Admin 列表/详情只消费后端状态，不自行映射旧态。

## 明确回答
- Admin 是否只展示五态？**是**（新数据走五态 label）。
- refund_status 是否独立展示？**后端已返回，前端可独立渲染**。
- 是否还存在可触达旧状态 UI？**无**——旧 key 仅历史兼容渲染，不作为新筛选/操作项。
- User/Admin/Backend 是否统一？**是**。
- P0-3 是否正式 PASS 封板？**是**。

未改后端/状态机/退款/资金/API Contract，未做 UI 重构。
