# HCZ Production Source Excludelist

> 明确不得进入远程正式仓库的文件与目录。
> 生成时间：2026-10-06
> 这些文件已从 Git index 移除（git rm --cached）或已通过 .gitignore 排除，
> 开发过程文档已移动到仓库外部归档目录：`E:\Users\orang\Downloads\Compressed\HCZ-ARCHIVE\`

## 开发过程文档（已移出仓库）

```
docs/audits/             # 所有审计报告（backend, frontend, security, infrastructure, final）
docs/reports/            # 所有阶段报告、P0/P1 闭环报告、修复报告、验收报告
docs/audit_frontend.md   # 前端审计报告
docs/archive/            # 仓库内归档目录（归档统一在仓库外）
```

**已移动到外部归档的文件数量**：
- audits: 29 个文件
- reports: 61 个文件
- 其他: 2 个文件（audit_frontend.md, PRODUCTION_CONFIG_AND_RATE_REPORT.md）
- **合计: 92 个文件**

## 日志与运行时数据

```
logs/                    # 应用日志目录
*.log                    # 任何日志文件
runtime/                 # 运行时数据
db/                      # 本地数据库文件目录
```

## 测试产物（非测试源码）

```
test-results/            # 测试运行结果
test-output/             # 测试输出
coverage/                # 覆盖率报告
*.lcov                   # LCOV 覆盖率数据
.nyc_output/             # NYC 覆盖率输出
coverage.txt
coverage.html
coverage.xml
junit.xml                # JUnit 测试结果
*.out                    # 测试二进制输出
htmlcov/                 # HTML 覆盖率报告
```

## 截图与可视化证据

```
screenshots/             # 任何截图目录
browser screenshots      # 浏览器截图
Selenium / Playwright 截图
*.png (测试截图)
*.jpg (测试截图)
```

## 构建产物

```
dist/                    # 前端构建输出
build/                   # 构建目录
frontend/*/dist/         # 各前端 dist
frontend/*/node_modules/ # 前端依赖
frontend/*/coverage/     # 前端覆盖率
frontend/*/.vite/        # Vite 缓存
internal/web/dist/       # go:embed 嵌入的前端构建产物（构建时生成）
*.exe                    # Windows 二进制
*.dll                    # 动态链接库
*.so                     # Linux 共享库
*.dylib                  # macOS 动态库
*.test                   # Go 测试二进制
/hcz                     # 主二进制
/dujiao-next
/dujiao-api
/hcz-api
```

## 本地配置与密钥

```
config.yml               # 本地实际配置（含真实密钥）
config.yml.local         # 本地配置
config.yml.dev.bak       # 开发配置备份
.env                     # 环境变量（含真实密钥）
.env.*                   # 任何 .env 变体（!.env.example 除外）
*.db                     # SQLite 数据库
*.db-shm
*.db-wal
*.sqlite
*.sqlite3
*.sqlite-wal
*.sqlite-shm
*.dump                   # SQL dump
*.sql                    # SQL dump（正式 migration 除外）
uploads/                 # 用户上传文件
```

## 临时文件

```
*.tmp                    # 临时文件
*.bak                    # 备份文件
*.old                    # 旧版本文件
*.orig                   # 合并冲突原始文件
*.cache                  # 缓存文件
*.pid                    # 进程 ID 文件
tmp/                     # 临时目录
temp/                    # 临时目录
push_live.txt            # 临时推送记录
test_regression.txt      # 临时回归记录
.staging_creds.txt       # 临时凭证
pg_hba.conf.bak.finalgate  # 临时配置备份
```

## 一次性脚本（非正式生产脚本）

```
# 以下类型不得提交：
# - 一次性修复脚本
# - 临时验证脚本
# - AI 辅助扫描脚本
# - 本地 ad-hoc powershell
# - 临时 browser automation script
# 正式脚本保留在 scripts/{backup,deployment,maintenance,migration,testing}/
```

## IDE 与编辑器

```
.vscode/
.idea/
*.swp
*.swo
*~
```

## OS 文件

```
.DS_Store
Thumbs.db
desktop.ini
```

## Python（测试脚本用）

```
.venv/
venv/
__pycache__/
*.pyc
*.pyo
.pytest_cache/
```

## Agents 与工具锁

```
.agents/
skills-lock.json
```

## 归档的 hcz_user（旧版）

```
# 如有 archived hcz_user 目录，不提交正式仓库
```

## AI 生成过程文档

```
# 任何 AI 执行过程记录、调试输出、阶段决策记录
# 统一移动到仓库外部归档目录
```
