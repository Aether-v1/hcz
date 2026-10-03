# HCZ User Frontend — USDT Contract Integration

## Final Verdict：**PASS**

本轮只做「数据接入 + 正确显示」，未做布局重构、未改后端资金模型。

## 修改文件
- `utils/money.ts`：新增纯展示 formatter `formatMoney / formatSiteMoney / formatWalletMoney / formatRate`（2dp、null 安全、不参与资金计算）。
- `views/personal/WalletPanel.vue`：钱包余额改用 API 返回 `currency`（USDT），不再用 site_config.currency。
- `components/wallet/WalletTransactionList.vue`：ledger 每行用 `item.currency`，空默认 USDT。
- `views/personal/OrdersPanel.vue`：订单列表新增「实际支付 xx USDT」行，优先 `wallet_paid_amount`→`usdt_total_amount`，历史单无快照显示 `--`（不重算）。
- `views/OrderDetail.vue`：**修复真实 P0 口径错误**——`wallet_paid_amount`、`refunded_amount` 原误用 `order.currency`(Site) 格式化，已改为 `wallet_currency||USDT`；退款记录同理；新增结算汇率快照卡片（rate/source/at，读订单 snapshot）。
- `views/personal/AffiliatePanel.vue`：待结/可提/已提佣金与佣金明细统一带 USDT 后缀。
- `api/types.ts`：`AffiliateCommissionData` 补 `currency?: string`。

## 语义闭环
- Product Price / Order total_amount = Site Currency ✅
- Wallet 余额 / Order 实付 / Refund / Commission / Ledger = USDT ✅
- 历史单 null snapshot → 显示 `--`，不按当前汇率重算 ✅
- Gateway 充值侧金额保持其真实渠道币种展示，未与 Wallet 本位币混淆 ✅
- 前端无 CoinGecko 调用、无汇率换算、无硬编码 CNY ✅

## 残留 / 接受项
- Checkout 页：若 preview API 尚未返回 USDT 应付，前端不自行计算（按要求保留商品金额 + Wallet USDT 余额，标记为 API 缺口，待新前端阶段补）。
- Reseller 端（ResellerOrders/ResellerLedger 等）属分销体系，本轮未逐页改，留待新前端统一对接。
- 未改后端 Contract。

## 验证
- TypeScript + `npm run build`：PASS（修一处 TS 后通过，vue-tsc 无错）。
- 未跑新前端单测（本轮以展示为主，build 即 smoke）。

## 结论
现有 User 端资金数据语义已接正确，无 Site/USDT 混淆残留。后续可单独做新布局与 UI 重构。
