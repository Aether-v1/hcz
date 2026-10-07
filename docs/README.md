# HCZ Documentation Index

本目录是 HCZ 项目所有文档、审计、报告、SOP 和决策记录的统一入口。
查找历史资料时先从本索引入手，避免在项目根目录散落文件。

## 目录结构

```
docs/
├── README.md                 # 本文件 — 文档索引
├── api/                      # API 契约、接口说明
├── architecture/             # 架构设计文档
├── deployment/               # 部署相关文档（Runbook、配置、上线检查）
├── migration/                # 数据库迁移报告与指南
├── security/                 # 安全策略与指南
├── testing/                  # 测试策略与指南
├── runbooks/                 # 执行教程 / SOP（怎么做）
├── audits/                   # 审计结论
│   ├── final/                # 最终审计、生产就绪、上线门禁
│   ├── backend/              # 后端业务审计
│   ├── frontend/             # 前端审计
│   ├── infrastructure/       # 基础设施审计
│   └── security/             # 安全审计
├── reports/                  # 阶段报告与修复报告
│   ├── phase-01-notification/
│   ├── phase-02-invitation/
│   ├── phase-03-withdrawal/
│   ├── phase-04-affiliate/
│   ├── phase-05-wallet-dual-balance/
│   ├── phase-06-c2c/
│   ├── phase-07-c2c-frontend/
│   ├── phase-08-ticket/
│   ├── phase-09-sitebuilder/
│   ├── p0-p1-closure/       # P0/P1 闭环与子任务报告
│   └── remediation/          # 通用修复报告
├── issues/                   # 问题调查 / RCA / Bug Investigation
├── decisions/                # 架构决策记录（ADR）
└── archive/                  # 已被取代但仍有历史价值的文档
```

## 分类说明

| 目录 | 放什么 | 不放什么 |
| --- | --- | --- |
| `audits/final/` | 最终审计、生产就绪评估、上线门禁报告 | 阶段过程报告 |
| `audits/backend/` | 后端业务、架构、CI 审计 | 前端审计 |
| `audits/frontend/` | 前端功能、构建、UI 审计 | 后端审计 |
| `audits/security/` | 安全审计、漏洞评估、合规报告 | 通用基础设施 |
| `deployment/` | 部署 Runbook、配置报告、上线清单 | 迁移脚本 |
| `migration/` | 数据库迁移报告、PostgreSQL 评估 | 程序实际使用的 migration 源码 |
| `reports/phase-XX/` | 各 Phase 的 Pre-Audit、Final、Remediation 报告 | 最终审计结论 |
| `reports/p0-p1-closure/` | P0/P1 闭环、子任务、专项修复报告 | Phase 主线报告 |
| `runbooks/` | 执行教程 / SOP | 执行结果报告 |
| `archive/` | 被取代但有历史价值的文档 | 当前有效文档 |

## 长期规则

- **测试代码** → `internal/.../*_test.go`（Go 惯例，与源码同目录）
- **测试执行脚本** → `scripts/testing/`
- **审计报告** → `docs/audits/`
- **Phase 报告** → `docs/reports/phase-XX/`
- **部署文档** → `docs/deployment/`
- **迁移文档** → `docs/migration/`
- **执行教程 / SOP** → `docs/runbooks/`
- **问题 / RCA** → `docs/issues/`
- **架构决策** → `docs/decisions/`
- **历史文件** → `docs/archive/`
- **临时 / Log / Coverage / 机器输出** → `runtime/`（已 gitignore，禁止提交）

## 关键文档快速链接

### 最终审计与上线
- [HCZ Full V1 Final Audit](audits/final/HCZ_FULL_V1_FINAL_AUDIT.md)
- [Production Readiness Full Audit](audits/final/HCZ_PRODUCTION_READINESS_FULL_AUDIT.md)
- [Production Final Gate](audits/final/HCZ_PRODUCTION_FINAL_GATE.md)
- [Production Deployment Preflight](audits/final/HCZ_PRODUCTION_DEPLOYMENT_PREFLIGHT.md)

### 部署
- [Deployment Runbook](deployment/DEPLOYMENT_RUNBOOK.md)
- [Production Config and Rate Report](deployment/PRODUCTION_CONFIG_AND_RATE_REPORT.md)

### 迁移
- [Migration and Postgres Report](migration/MIGRATION_AND_POSTGRES_REPORT.md)

### API
- [Frontend API Contract](api/HCZ_FRONTEND_API_CONTRACT.md)
- [HCZ Points API（积分体系完整契约）](HCZ_POINTS_API.md)

### 积分体系（P0–P4）
- [P0 Points Core 实现报告](HCZ_POINTS_P0_IMPLEMENTATION_REPORT.md)
- [P1 Order Reward 报告](HCZ_POINTS_P1_ORDER_REWARD_REPORT.md)
- [P2 Check-in 报告](HCZ_POINTS_P2_CHECKIN_REPORT.md)
- [P3 Points Mall 报告](HCZ_POINTS_P3_MALL_REPORT.md)
- [P4 Admin / Operations / Finalization 报告](HCZ_POINTS_P4_FINALIZATION_REPORT.md)
