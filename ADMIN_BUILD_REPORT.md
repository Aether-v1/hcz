# Admin 前端生产构建报告

## 构建结论

**BUILD SUCCESS** — 从当前源码全新构建成功，未复用旧 dist。

## 构建时间

- 开始时间：2026-10-05 20:57:36 (UTC+8, Asia/Shanghai)
- 结束时间：2026-10-05 20:58:46 (UTC+8, Asia/Shanghai)
- 总耗时：约 70 秒（其中 `vite build` 自身 23.94s）

## 构建命令

工作目录：`E:\Users\orang\Downloads\Compressed\hcz_v1\frontend\admin`

```powershell
# 1. 删除旧 dist（禁止复用）
Remove-Item -Recurse -Force dist -ErrorAction SilentlyContinue

# 2. 类型检查
npx vue-tsc -b

# 3. fullstack 生产构建（PowerShell 下用 env 变量，等价于 build:fullstack）
$env:VITE_FULLSTACK="1"
npx vite build
```

对应 package.json 脚本：`build:fullstack` = `vue-tsc -b && VITE_FULLSTACK=1 vite build`

## 各阶段结果

| 阶段 | 结果 | 退出码 |
| --- | --- | --- |
| 旧 dist 删除 | 成功（删除后 dist 不存在） | — |
| `vue-tsc -b` 类型检查 | 通过，无类型错误 | 0 |
| `vite build`（VITE_FULLSTACK=1） | 成功，2960 modules transformed，built in 23.94s | 0 |

## 产物概览

- dist 目录：`E:\Users\orang\Downloads\Compressed\hcz_v1\frontend\admin\dist`
- 文件总数：142
- 总大小：3,381,625 字节（约 3.22 MB）
- 入口：`dist/index.html`

## dist/assets 文件列表

### JS 文件（按大小降序）

| 大小(字节) | 文件名 |
| ---: | --- |
| 504,217 | index-BPAWOvQg.js |
| 374,342 | vendor-tiptap-VY_yUuld.js |
| 175,354 | vendor-vue-CtB3a5Y6.js |
| 172,832 | vendor-ui-Vtt6TaBm.js |
| 113,733 | Settings-DgJdor8s.js |
| 97,406 | PaymentChannels-BbEWxWnP.js |
| 83,839 | AdminLayout-0MmH7zMN.js |
| 68,072 | Orders-BkJz3QiM.js |
| 60,170 | Products-5jYoFAAg.js |
| 40,967 | Notifications-fMwTQL3H.js |
| 39,847 | ProductMappings-CAVK963w.js |
| 38,169 | Security-5Uk7Kxlt.js |
| 35,112 | Dashboard-Ck4Psp--.js |
| 33,630 | Authz-B3TwfMAR.js |
| 33,249 | UserDetail-D50fFpys.js |
| 33,124 | CardSecrets-BQ5lI9Rd.js |
| 30,944 | ProcurementOrders-CAZ2I4Wv.js |
| 29,203 | ResellerProfileDetail-s05qLsRY.js |
| 29,043 | purify.es-BGtG0CB9.js |
| 24,381 | Coupons-CaqgLy9x.js |
| 24,186 | Payments-utxCY828.js |
| 21,734 | Reconciliation-N4GGBIxx.js |
| 21,511 | GiftCards-CAQmBeXu.js |
| 21,385 | ResellerSiteConfigs-BuistHMl.js |
| 20,373 | ResellerProductSettings-BYHLaYqJ.js |
| 20,047 | CardSecretExports-fW_zXTvR.js |
| 19,611 | Promotions-qUzzC8Hy.js |
| 19,360 | ResellerProfiles-B4sUfzDH.js |
| 19,290 | Posts-DQT_F3OH.js |
| 18,974 | Users-DXonnEDc.js |
| 18,837 | TicketDetail-Ca8F55YY.js |
| 18,205 | DiscoveryBlocks-BVRjvtq8.js |
| 17,601 | OrderRefunds-DdniqPnF.js |
| 17,391 | Banners.vue_vue_type_script_setup_true_lang-b7r-4W0W.js |
| 17,242 | OrderRiskControl-DC25UNzb.js |
| 16,831 | RichEditor-DdPgfV8F.js |
| 16,465 | ResellerOperationsDashboard-BCatOMZd.js |
| 16,399 | CardSecretImports-DgC-vP8H.js |
| 16,368 | SiteConnections-SCOnHwuG.js |
| 16,230 | WholesalePrices-Bfvksthu.js |
| 15,303 | MemberLevels-Cy4dUTyl.js |
| 14,839 | MediaPicker-D24fuRUj.js |
| 14,366 | TelegramBotChannelClients-CZoMxwiQ.js |
| 13,676 | TelegramBotBroadcastCreate-Cc2BGTWZ.js |
| 13,620 | WalletRecharges-uBbF43Qz.js |
| 12,858 | ResellerDomains-CErH_xVs.js |
| 12,552 | ResellerWithdraws-De87dTpU.js |
| 12,490 | ResellerLedgerEntries-CkuunC_z.js |
| 12,376 | PostCategories-Cxvm9CfI.js |
| 12,060 | ApiCredentials-BMvdOVeF.js |
| 11,784 | Categories-BAlAlG6R.js |
| 11,704 | HomeEntries-DAX-deYf.js |
| 11,412 | AffiliateUsers-DC17GbCD.js |
| 11,342 | ComplianceGuardWrapper.vue_vue_type_script_setup_true_lang-6Ef379_D.js |
| 10,967 | TicketList-DEv8REjm.js |
| 10,961 | WalletWithdrawalDetail-C4h7Rr8c.js |
| 10,797 | AffiliateCommissions-D2L0UBb-.js |
| 10,738 | AuthzAuditLogs-DlDfzoyt.js |
| 10,545 | Login-DUWQM5Gt.js |
| 10,009 | AffiliateWithdraws-BNUrr6pa.js |
| 9,842 | WalletWithdrawals-BBhLIeCZ.js |
| 9,835 | NavFooter-RQoa0GfN.js |
| 9,636 | TelegramBotBroadcasts-DK-YcT_q.js |
| 9,487 | C2CDisputeDetail-CofTJ4GK.js |
| 9,324 | TelegramBot-DB4MmgBx.js |
| 9,272 | CategoriesManagement-BQlJp_Qy.js |
| 9,166 | Media-DjySwuSZ.js |
| 9,120 | ResellerBalanceAccounts-BypvkGZc.js |
| 8,748 | UserLoginLogs-C_vJ-mSS.js |
| 8,498 | BrandSettings-CX8G7m0U.js |
| 7,864 | AffiliateSettings-BuwLrCmD.js |
| 7,776 | TelegramBotHelpCenter-DMjRRdgt.js |
| 7,262 | C2CTrades-DsYYBVLt.js |
| 7,149 | C2COverview-DxM4Iw32.js |
| 6,802 | C2CListings-BsllwPSW.js |
| 6,787 | TelegramBotSettings-7a65c5XB.js |
| 6,437 | TelegramBotStatus-CXd6uC1B.js |
| 6,318 | TelegramBotMenuSettings-CR5EuefC.js |
| 6,144 | TelegramBotBroadcastDetail-CIoWEZAB.js |
| 6,027 | C2CRiskSignals-DC43un8p.js |
| 5,833 | C2CSettings-BEb48so4.js |
| 5,652 | CallbackRoutes-B8pPeZMv.js |
| 5,051 | C2CDisputes-DuKsIlcQ.js |
| 4,927 | SelectValue.vue_vue_type_script_setup_true_lang-H1YwSK0f.js |
| 4,922 | C2CUsers-CX9JhVnK.js |
| 4,510 | MultiSelect.vue_vue_type_script_setup_true_lang-VWNcSDpr.js |
| 4,421 | SupportDashboard-bw6506Be.js |
| 4,148 | useTelegramBotSettings-DlunFVkV.js |
| 4,049 | BannerAnnouncement-DkQD4dcE.js |
| 4,021 | Wallet-BlHY6oye.js |
| 2,949 | ListPagination.vue_vue_type_script_setup_true_lang-CTj3zFjA.js |
| 2,731 | TemplateSettings-BUXoVRuo.js |
| 2,338 | wholesalePricing-h3K9nu6M.js |
| 2,279 | status-Dj5mgM4M.js |
| 2,126 | TabsTrigger.vue_vue_type_script_setup_true_lang-tN-Q4fXH.js |
| 1,995 | SiteBuilder-C2o8CV4M.js |
| 1,847 | DialogContent.vue_vue_type_script_setup_true_lang-h-iUlVVV.js |
| 1,704 | TableHeader.vue_vue_type_script_setup_true_lang-BjrFimZe.js |
| 1,664 | Forbidden-Jxg_ICHB.js |
| 1,510 | DialogScrollContent.vue_vue_type_script_setup_true_lang-C9ibLyLm.js |
| 1,458 | site-builder-CdD09wRh.js |
| 1,430 | Switch.vue_vue_type_script_setup_true_lang-B-MowjTS.js |
| 1,322 | Checkbox.vue_vue_type_script_setup_true_lang-4g5k7xfZ.js |
| 1,318 | useFormValidation-GPDSPU-K.js |
| 1,243 | sku-BL8oqpAK.js |
| 1,116 | StatusBadge.vue_vue_type_script_setup_true_lang-QUj20Oo1.js |
| 1,113 | IdCell.vue_vue_type_script_setup_true_lang-C0zyLVBn.js |
| 1,085 | FileInput.vue_vue_type_script_setup_true_lang-DAVxfBkl.js |
| 1,077 | support-V7un4jln.js |
| 1,032 | format-oWlmQ8Dh.js |
| 978 | category-46Pc1JPr.js |
| 975 | Banners-1Y1DaHXs.js |
| 954 | index-Dz-vd5qQ.js |
| 908 | Input.vue_vue_type_script_setup_true_lang-DbuUTdUx.js |
| 901 | ComplianceRequired-BNimlkL_.js |
| 860 | c2c-DoUB8UPD.js |
| 841 | Textarea.vue_vue_type_script_setup_true_lang-D_C61ZCp.js |
| 800 | compliance-D3ruspKt.js |
| 761 | TableSkeleton.vue_vue_type_script_setup_true_lang-DW2Xi4Cp.js |
| 759 | reseller-Cjv5b3-K.js |
| 586 | CardTitle.vue_vue_type_script_setup_true_lang-C31dTL11.js |
| 553 | Label.vue_vue_type_script_setup_true_lang-CXqolmid.js |
| 548 | TabsContent.vue_vue_type_script_setup_true_lang-WOwZWGG2.js |
| 375 | Card.vue_vue_type_script_setup_true_lang-Dogsj-m0.js |
| 375 | resellerManagement-DcC3hb5F.js |
| 364 | confirm-Dr35nP7q.js |
| 360 | CardDescription.vue_vue_type_script_setup_true_lang-nAbnu7k6.js |
| 337 | CardContent.vue_vue_type_script_setup_true_lang-CeeWlo8i.js |
| 305 | image-DleukfuU.js |
| 281 | useListRefresh-Dc87cobf.js |
| 216 | upload-BBqwszKw.js |
| 216 | favicon-BuJ8-mow.js |
| 204 | affiliate-Jigll1Sg.js |

### CSS 文件（按大小降序）

| 大小(字节) | 文件名 |
| ---: | --- |
| 100,149 | index-Dts71NQy.css |
| 7,173 | RichEditor-DaouQMWn.css |
| 1,911 | AdminLayout-BDQ423Q6.css |
| 198 | MediaPicker-BpRr0B5q.css |
| 193 | ComplianceGuardWrapper-BeNBTosO.css |
| 75 | OrderRefunds-CzOC7fPc.css |
| 75 | Payments-DFN-48SH.css |

## fullstack 占位符验证

在 dist 全目录 grep `__DJ_ADMIN_BASE__`，命中以下文件：

- `dist/index.html` —— 内容含 `<base href="__DJ_ADMIN_BASE__/">`
- `dist/assets/index-BPAWOvQg.js` —— 主入口 JS 内含占位符引用

结论：fullstack 模式占位符已正确注入，生产二进制启动时会按 `web.admin_path` 替换为实际路径。

## SHA256 哈希

- 文件：`dist/index.html`
- 算法：SHA256
- 哈希值：`A3A99317AA8174B885DCEA794DBDB678AE8499ADC820BEBD5B112B1811F07CB3`

## 最终判定

**BUILD SUCCESS**

- 旧 dist 已删除后从当前源码全新构建；
- `vue-tsc -b` 类型检查通过（exit 0）；
- `vite build`（VITE_FULLSTACK=1）成功（exit 0）；
- fullstack 占位符 `__DJ_ADMIN_BASE__` 已注入；
- 产物可直接复制到 `internal/web/dist/admin` 供 go:embed 打入二进制。
