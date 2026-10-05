# HCZ Production Source Allowlist

> 最终建议推送到远程正式仓库的目录与文件类型。
> 生成时间：2026-10-06
> 原则：只保留生产运行、构建、部署、迁移所必需的源码和配置文件。

## 后端源码

```
cmd/                    # 应用入口
internal/               # 核心业务逻辑（DDD 分层）
  app/                  # HTTP server 组装
  bootstrap/            # 启动引导、依赖注入、数据库迁移注册
  modules/              # 业务模块（order, wallet, c2c, payment, identity, auditlog, reporting 等）
  shared/               # 共享库（密码策略、序列化、工具函数）
  architecture/         # 架构约束测试（*_test.go）
```

## 前端源码

```
frontend/user/src/      # 正式用户前端源码（Vue 3 + TypeScript）
  api/                  # API 客户端
  assets/               # 前端静态资源
  components/           # 组件
  composables/          # Vue composables
  i18n/                 # 国际化语言包
  router/               # 路由
  templates/            # 页面模板
  utils/                # 工具函数
  views/                # 页面视图
  App.vue               # 根组件
  main.ts               # 入口
  style.css             # 全局样式
frontend/user/tests/    # 前端单元测试源码（*.test.ts）
frontend/user/package.json
frontend/user/package-lock.json
frontend/user/tsconfig.json
frontend/user/vite.config.ts
frontend/user/tailwind.config.js
frontend/user/postcss.config.js
frontend/user/index.html

frontend/admin/src/     # 管理后台前端源码
frontend/admin/tests/   # 管理后台测试源码
frontend/admin/package.json
frontend/admin/package-lock.json
frontend/admin/tsconfig.json
frontend/admin/vite.config.ts
```

## 数据库迁移

```
internal/bootstrap/database/migrations/   # GORM AutoMigrate 注册 + 自定义迁移
db/                                        # SQL schema / seed（如有）
```

## 构建与部署

```
Dockerfile
Makefile
.goreleaser.yaml
.dockerignore
go.mod
go.sum
```

## 配置模板（不含真实 secret）

```
config.yml.example
config.yml.production    # 全 ${PLACEHOLDER} 模板，无真实密钥
.env.example             # 如有
```

## CI/CD

```
.github/workflows/       # CI 工作流
.github/ISSUE_TEMPLATE/  # Issue 模板
```

## 正式脚本

```
scripts/backup/          # 生产备份脚本
scripts/deployment/      # 部署校验脚本
scripts/maintenance/     # 运维管理脚本
scripts/migration/       # 迁移预检脚本
scripts/testing/         # 正式测试脚本
```

## 正式文档

```
README.md                # 项目说明（最小）
docs/README.md           # 文档索引
docs/api/                # 正式 API 文档
docs/architecture/       # 架构文档
docs/decisions/          # ADR 架构决策记录
docs/deployment/         # 部署文档 / Runbook
docs/issues/             # 已知问题记录
docs/migration/          # 数据库迁移文档
docs/runbooks/           # 运维手册
```

## 生产静态资源

```
assets/                  # 生产运行时静态资源（合作方 logo 等）
```

## 测试源码（正式质量保证体系）

```
**/*_test.go             # Go 单元/集成测试源码
**/*.test.ts             # 前端单元测试源码
internal/modules/*/integrationtest/   # 集成测试源码
```

## 其他

```
LICENSE
.gitignore
```
