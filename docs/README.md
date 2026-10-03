# HCZ Documentation Index

本目录是 HCZ 项目所有文档、审计、报告、SOP 和决策记录的统一入口。
查找历史资料时先从本索引入手，避免在项目根目录散落文件。

## 目录结构

```
docs/
├── README.md                 # 本文件 — 文档索引
├── architecture/             # 架构设计文档
├── api/                      # API 契约、接口说明
├── deployment/               # 部署相关文档
├── runbooks/                 # 执行教程 / SOP（怎么做）
├── audits/                   # 审计结论（只放审计报告）
│   ├── backend/              # 后端业务审计
│   ├── security/             # 安全审计
│   ├── database/             # 数据库 / Schema 审计
│   └── legacy/               # 遗留系统 / 退役审计
├── reports/                  # 阶段报告、测试报告、回归报告
│   ├── tests/                # 测试报告
│   ├── regression/           # 回归报告
│   └── phases/               # 阶段完成报告
├── issues/                   # 问题调查 / RCA / Bug Investigation
├── decisions/                # 架构决策记录（ADR）
└── archive/                  # 已被取代但仍有历史价值的文档
```

## 分类说明

| 目录 | 放什么 | 不放什么 |
| --- | --- | --- |
| `audits/` | 审计结论类文档（边界审计、Schema 审计、安全审计） | 测试日志、执行输出 |
| `reports/` | Phase 报告、测试报告、E2E/Regression 报告、Release Readiness | 审计结论（去 audits/） |
| `runbooks/` | 怎么执行某件事（LOCAL_SETUP、RUN_TESTS、DEPLOYMENT_STEPS、ROLLBACK） | 执行结果报告（去 reports/） |
| `issues/` | 问题调查、Root Cause Analysis、未解决技术问题 | 已完成的审计（去 audits/） |
| `decisions/` | ADR：为什么这样设计、放弃了什么、最终决策 | 测试日志、临时笔记 |
| `archive/` | 被新报告取代但仍有历史价值的文件，保留原文件名 | 当前有效文档 |

## 长期规则

- **测试代码** → `internal/.../*_test.go`（Go 惯例，与源码同目录）
- **测试执行脚本** → `scripts/test/`（或现有 `scripts/tests/`）
- **审计工具** → `scripts/audit/`
- **审计报告** → `docs/audits/`
- **测试 / 回归报告** → `docs/reports/`
- **执行教程 / SOP** → `docs/runbooks/`
- **问题 / RCA** → `docs/issues/`
- **架构决策** → `docs/decisions/`
- **历史文件** → `docs/archive/`
- **临时 / Log / Coverage / 机器输出** → `runtime/`（已 gitignore，禁止提交）

## 当前文档清单

### Audits — Backend
- [HCZ Payment Boundary Audit](audits/backend/HCZ_PAYMENT_BOUNDARY_AUDIT.md) — Payment / Wallet / Order 资金边界审计
- [HCZ Business Fit-Gap Audit](audits/backend/HCZ_FIT_GAP_AUDIT.md) — 业务能力 Fit-Gap 分析
- [HCZ Global Site Currency Audit](audits/backend/HCZ_GLOBAL_CURRENCY_AUDIT.md) — 全站币种能力审计

### Reports
- [Repository Artifact Cleanup Report](reports/REPOSITORY_ARTIFACT_CLEANUP_REPORT.md) — 仓库文件治理整理报告
