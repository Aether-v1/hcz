# HCZ 生产部署 / Smoke / 回滚 / 监控 Runbook

> 适用版本：Release baseline `875a24c`（代码，user 前端 DOMPurify 消毒同步，CI run #22 全绿；前序 `5f0ab23`/`c850d5a`）
> 部署形态：**单镜像全栈** —— admin + user 两个 SPA 由 `go:embed` 打进同一个 Go 二进制 `./cmd/server`，同一进程同一端口 `8080` 提供 API + Admin + User。
> 运行时：Alpine + `/app/hcz`，配置 `/app/config.yml`，EXPOSE 8080。
> 数据库：生产必须 **PostgreSQL**；GORM AutoMigrate 在进程启动时自动执行（只 ADD 不 DROP）。
> Redis：db 0 = 缓存，db 1 = 队列。
> 安装形态（来自 `scripts/hcz-manager.sh`）：Linux + systemd，unit 名 `hcz.service`，安装目录 `/opt/hcz`，二进制 `/opt/hcz/hcz`，配置 `/opt/hcz/config.yml`，上传目录 `/opt/hcz/uploads`，日志 `/opt/hcz/logs`。
>
> 本 runbook 同时给出 **systemd/裸金属** 与 **Docker** 两种执行命令；按你实际形态选择其一。命令行语法以 **Linux bash** 为准（生产环境）；括号内标注 PowerShell（开发机/运维跳板机）等效写法。

---

## 0. 上线前自检（Pre-flight）

在开始任何部署动作之前，先确认以下事实，避免「按 runbook 走完才发现环境不对」。

| 检查项 | 命令（Linux） | 通过标准 |
|---|---|---|
| 当前运行版本 | `systemctl cat hcz.service \| grep ExecStart` 或 `docker inspect <c> --format '{{.Config.Image}}'` | 能看到当前版本号 / image tag |
| 当前二进制路径 | `ls -l /opt/hcz/hcz` 或 `docker ps --filter name=hcz` | 存在且进程在跑 |
| DB 类型 | `grep -E '^\s*driver:' /opt/hcz/config.yml` | 必须为 `postgres`（生产） |
| Redis 可达 | `redis-cli -n 0 ping && redis-cli -n 1 ping` | 两个 db 都返回 PONG |
| 磁盘剩余 | `df -h /opt/hcz /var/lib/postgresql` | 至少预留 1.5× DB dump 大小 |
| 健康检查 | `curl -fsS http://127.0.0.1:8080/health` | 返回 `{"status":"ok"}` |
| 备份脚本可用 | `which pg_dump` | 存在且版本与 PG 服务端匹配 |

> 健康检查端点已确认：`internal/app/httpserver/router.go:336` 注册 `GET /health`，返回 200 + `{"status":"ok"}`。**项目内没有 `/metrics` Prometheus 端点**（grep `prometheus|promhttp|/metrics` 全仓 0 命中），监控只能走「日志关键字 + SQL 探针 + 外部黑盒」三条路。

---

## 1. Deployment Order（部署顺序）

> 全程预计 **25–40 分钟**（不含大表 migration 等待）。每一步都有「回退点」—— 意味着这一步做完后如果下一步失败，能退到这个点而不丢数据。

### Step 1 — 维护模式 / 流量保护

**目的**：在切流前停止新写入，避免半切换状态下产生「新代码写、旧代码读」的脏数据。

**执行命令（Linux，Nginx 反代形态）**：
```bash
# 在前置 Nginx 上挂一个 503 维护页
sudo tee /etc/nginx/maintenance.enabled >/dev/null <<'EOF'
# temporary maintenance
EOF
sudo nginx -t && sudo systemctl reload nginx

# 验证：外网入口应返回 503
curl -sS -o /dev/null -w '%{http_code}\n' https://<你的域名>/
# 期望：503
```

**Docker 形态**：在 LB / Cloudflare / 上游网关把域名切到维护页 origin，或直接 `docker pause` 前置 nginx。

**验证标准**：外网 503；内网 `curl http://127.0.0.1:8080/health` 仍 200（本机回环不受维护页影响）。
**预计耗时**：1 分钟。
**回退点**：删除 `maintenance.enabled` 再 reload 即可恢复。

> 说明：HCZ 自身**没有**内建的「维护模式开关」配置项（grep `maintenance` 无业务开关），所以这一层必须在 Nginx / LB 上做。如果你没有 Nginx，最不济就是 `systemctl stop hcz.service`（等价于全量下线），但不推荐。

---

### Step 2 — PostgreSQL 全量备份（pg_dump）

**执行命令（Linux）**：
```bash
# 从 /opt/hcz/config.yml 里读出 DSN，或直接读环境变量。
# 下面用标准环境变量写法：
export PGPASSWORD='<你的DB密码>'
TS=$(date -u +%Y%m%dT%H%M%SZ)
BK=/opt/hcz/backups/db_${TS}.dump
mkdir -p /opt/hcz/backups

pg_dump -Fc -h <DB_HOST> -U <DB_USER> -d <DB_NAME> -f "$BK"

# 记录 size / sha256
ls -lh "$BK"
sha256sum "$BK" | tee "${BK}.sha256"
```

**PowerShell（跳板机）等效**：
```powershell
$ts = Get-Date -Format "yyyyMMddTHHmmssZ"
$bk = "E:\backups\hcz_$ts.dump"
$env:PGPASSWORD = '<你的DB密码>'
& pg_dump.exe -Fc -h <DB_HOST> -U <DB_USER> -d <DB_NAME> -f $bk
Get-FileHash $bk -Algorithm SHA256 | Format-List
```

**验证标准**：
- 文件大小 > 0（与上次 dump 大小同量级，±30% 内）。
- `pg_restore --list "$BK" | head` 能列出表。
- 把 `path / timestamp / size / sha256` 四元组写进部署工单。

**预计耗时**：取决于 DB 大小，10 GB 约 2–5 分钟。
**回退点**：备份文件存在即回退点。后续任何一步失败都能从这里恢复。

---

### Step 3 — 配置备份

```bash
sudo cp -a /opt/hcz/config.yml /opt/hcz/backups/config.yml.$(date -u +%Y%m%dT%H%M%SZ).bak
ls -l /opt/hcz/backups/
```

**验证标准**：备份文件大小与现网 `config.yml` 一致。
**预计耗时**：秒级。
**回退点**：`config.yml` 已留底，Step 6 改配置可随时覆盖回去。

---

### Step 4 — Uploads / 静态资源备份

```bash
sudo tar -C /opt/hcz -czf /opt/hcz/backups/uploads_$(date -u +%Y%m%dT%H%M%SZ).tgz uploads/
sudo ls -lh /opt/hcz/backups/*.tgz
```

> 注意：`/uploads/support_ticket/` 下的工单附件是用户敏感材料，必须一并备份。
> Docker 形态：把容器内 `/app/uploads` 对应的 volume 同样打包。

**验证标准**：tgz 大小与 `du -sh /opt/hcz/uploads` 对得上。
**预计耗时**：取决于 uploads 体量，通常 1–3 分钟。
**回退点**：uploads 归档完成。

---

### Step 5 — 旧二进制 / 旧镜像备份

**裸金属（systemd）形态**：
```bash
# hcz 自带自更新机制，升级时会自动把旧二进制留成 hcz.backup + hcz.backup.json。
# 但为了这一次手动部署，再显式留一份：
sudo cp -a /opt/hcz/hcz /opt/hcz/backups/hcz.$(date -u +%Y%m%dT%H%M%SZ).bak
# 顺便记下当前版本号
sudo /opt/hcz/hcz --version 2>&1 | tee /opt/hcz/backups/version.before.txt
```

**Docker 形态**：
```bash
# 记下当前 running image tag（不 stop，仅打 tag 以便回滚）
CUR=$(docker inspect hcz --format '{{.Config.Image}}')
echo "$CUR" | tee /opt/hcz/backups/image.before.txt
# 把当前 image 额外 retag 成 rollback-<ts>，避免被 docker system prune 清掉
docker tag "$CUR" "hcz:rollback-$(date -u +%Y%m%dT%H%M%SZ)"
```

**验证标准**：备份文件 / retag 完成，`image.before.txt` 里有可回滚的具体 tag。
**预计耗时**：秒级 ~10 秒。
**回退点**：旧二进制 / 旧 image tag 已固化。

---

### Step 6 — 部署新版本二进制 / 镜像（触发 AutoMigrate）

> **关键**：AutoMigrate 在进程启动、连完 DB 之后**同步执行**（`cmd/server/main.go:143`），只 ADD 不 DROP。在 staging 已经验证幂等是硬前提。
> 进程内还有 startup guard：迁移开始前会落盘 `*.rollback.json`，迁移完成后再标记 `startup_completed`，CLI `hcz rollback` 据此判断安全性。

**裸金属形态（systemd）**：
```bash
# 1) 下发新二进制到安装目录（不要直接覆盖正在跑的文件，先放新名字再切）
sudo install -m 0750 -o hcz -g hcz /tmp/hcz.new /opt/hcz/hcz.new

# 2) 切版本（systemd 下建议用 hcz-manager.sh，或手动原子替换）
sudo systemctl stop hcz.service
sudo mv /opt/hcz/hcz.new /opt/hcz/hcz
# 旧二进制已由自更新机制留成 /opt/hcz/hcz.backup（若本次是手动替换，则靠 Step 5 的 .bak）

# 3) 启动 —— 这一步会触发 AutoMigrate
sudo systemctl start hcz.service

# 4) 看日志确认 migration 跑完、进程活着
sudo journalctl -u hcz.service -f
# 等待出现：完整启动完成 / gin 监听 8080 的日志
```

**Docker 形态**：
```bash
# 拉新镜像
docker pull <registry>/hcz:<new_tag>

# 停旧、起新（同 volume、同 config）
docker stop hcz && docker rm hcz
docker run -d --name hcz \
  --restart=on-failure \
  -p 127.0.0.1:8080:8080 \
  -v /opt/hcz/config.yml:/app/config.yml:ro \
  -v /opt/hcz/uploads:/app/uploads \
  -v /opt/hcz/logs:/app/logs \
  <registry>/hcz:<new_tag>

docker logs -f hcz
# 等待 AutoMigrate 完成、listening on 8080
```

**验证标准**：
- 进程退出码 0（systemd 下 `systemctl is-active hcz` = active；Docker 下 `docker ps` 健康）。
- 日志里**没有** `数据库迁移失败` / `数据库初始化失败` / `Fatal` 字样。
- `curl -fsS http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- `sudo /opt/hcz/hcz --version`（或 `docker exec hcz /app/hcz --version`）打印新版本号。

**预计耗时**：2–5 分钟（migration 在万级行表内通常秒级；大表加列可能 1–3 分钟，看 PG 锁）。
**回退点**：这一步失败 → 直接跳到 [第 6 节 Rollback](#6-rollback-runbook) 的「Application rollback」。**不要**反复重启硬试。

---

### Step 7 — Backend Deploy 确认

Backend 已经在 Step 6 随二进制一起起来了。这一步只是**二次确认**进程稳态：

```bash
# 连续打 3 次 health，间隔 5s
for i in 1 2 3; do curl -fsS http://127.0.0.1:8080/health; echo; sleep 5; done
# 期望：3 次都是 {"status":"ok"}
```

**验证标准**：health 连续 3 次 200；`journalctl -u hcz --since "2 minutes ago" | grep -iE 'error|fatal|panic'` 为空。
**预计耗时**：30 秒。

---

### Step 8 — Admin Frontend 确认

> Admin SPA 已 `go:embed` 进二进制，随 Step 6 一起部署，**没有独立部署动作**。

```bash
# admin 路径由 config.yml 的 web.admin_path 决定，默认 /admin（生产应改成难猜路径）
ADMIN_PATH=$(grep -E '^\s*admin_path:' /opt/hcz/config.yml | awk '{print $2}')
curl -fsS -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:8080${ADMIN_PATH}"
# 期望：200（HTML 首页）

# 顺手打一下 admin login API
curl -fsS -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/admin/login \
  -H 'Content-Type: application/json' -d '{}'
# 期望：400/401（说明路由活了，只是没传凭据；不是 404/500）
```

**验证标准**：admin 首页 200；login API 不是 404。
**预计耗时**：30 秒。

---

### Step 9 — User Frontend 确认

> User SPA 同样 embed 在二进制里，挂在 `/`。

```bash
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/
# 期望：200

curl -fsS http://127.0.0.1:8080/api/v1/public/config
# 期望：200 + JSON（站点公开配置）
```

**验证标准**：首页 200 + `/api/v1/public/config` 200。
**预计耗时**：30 秒。

---

### Step 10 — Redis / Cache Invalidation

新版本可能改了缓存 key 结构，最安全是**清掉 db 0 的业务缓存**（db 1 是队列，别动）：

```bash
# 先看一眼有哪些 key
redis-cli -n 0 --scan --pattern 'dj:*' | head

# 精确清：站点配置 / 汇率 / 商品列表（推荐）
redis-cli -n 0 --scan --pattern 'dj:settings:*' | xargs -r redis-cli -n 0 DEL
redis-cli -n 0 --scan --pattern 'dj:exchange_rate*' | xargs -r redis-cli -n 0 DEL

# 或者整库 flush（简单粗暴，会导致冷启动慢一点）
# 仅在确认 db 0 全是业务缓存时执行：
# redis-cli -n 0 FLUSHDB
```

> 注意：配置里 `redis.prefix: "dj"`，所以业务 key 都是 `dj:*`。**不要 flush db 1**——那是 Asynq 队列，里面有在跑的任务（汇率刷新、上游同步等）。

**验证标准**：`redis-cli -n 0 --dbsize` 数字显著下降；访问一次首页后数字回升（说明缓存重建了）。
**预计耗时**：10 秒。
**回退点**：缓存本来就是可再生的，清错了最坏就是缓存重建，不影响数据。

---

### Step 11 — Health Check（外部门面）

```bash
# 本机
curl -fsS http://127.0.0.1:8080/health
# 外网（还挂在维护页后，先临时放行）
curl -fsS https://<你的域名>/health
```

**验证标准**：内网 + 外网都返回 `{"status":"ok"}`。
**预计耗时**：10 秒。

---

### Step 12 — Smoke Test

按 [第 2 节 Production Smoke Checklist](#2-production-smoke-checklist) 完整跑一遍。
**预计耗时**：15–20 分钟。
**回退点**：Smoke 失败 → 直接进 [第 6 节 Rollback](#6-rollback-runbook)。

---

### Step 13 — Reopen Traffic

```bash
sudo rm -f /etc/nginx/maintenance.enabled
sudo nginx -t && sudo systemctl reload nginx

# 外网应恢复 200
curl -sS -o /dev/null -w '%{http_code}\n' https://<你的域名>/
# 期望：200
```

**验证标准**：外网 200；日志里能看到真实用户请求。
**预计耗时**：30 秒。
**回退点**：重新 `touch /etc/nginx/maintenance.enabled` + reload 即回到维护模式。

---

## 2. Production Smoke Checklist

> **铁律**：
> - 只用**专用测试账号**（至少 3 个：`smoke-user-a`、`smoke-user-b`、`smoke-admin`）。
> - 所有资金操作金额 **≤ 10 CNY**（或等价最小单位）。
> - 每个用例前记录一次钱包余额（见 [第 3 节对账](#3-wallet-reconciliation-验证)）。
> - 所有路由均来自代码实际注册，不编造。基础前缀 `/api/v1`。
> - 下面用 `TOKEN_A` / `TOKEN_B` / `TOKEN_ADMIN` 表示三个账号的登录 JWT。

### 2.0 准备：拿 Token

```bash
# 用户登录（实际路由：POST /api/v1/auth/login）
TOKEN_A=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"smoke-user-a@test.local","password":"<pwd>"}' | jq -r '.data.token // .token')

TOKEN_B=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"smoke-user-b@test.local","password":"<pwd>"}' | jq -r '.data.token // .token')

# Admin 登录（实际路由：POST /api/v1/admin/login）
TOKEN_ADMIN=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"smoke-admin","password":"<pwd>"}' | jq -r '.data.token // .token')
```

**通过标准**：三个 token 都非空、不是 `null`。

---

### 2.1 Auth：Register / Login / Rate Limit

| 操作 | 端点 | 预期 |
|---|---|---|
| 注册新测试用户 | `POST /api/v1/auth/register` | 200，返回 user_id |
| 正确密码登录 | `POST /api/v1/auth/login` | 200，返回 token |
| 错误密码连打 6 次 | `POST /api/v1/auth/login` | 第 6 次起 429（配置 `login_rate_limit.max_attempts=5`，block 900s） |
| 等 block 窗口过后再登 | 同上 | 恢复 200 |

```bash
# 限流验证（连续 6 次错密码）
for i in 1 2 3 4 5 6; do
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"email":"smoke-user-a@test.local","password":"wrong"}')
  echo "attempt $i -> $code"
done
# 期望前 5 次是 401，第 6 次是 429
```

**通过标准**：注册 200；正确登录 200；第 6 次错密码 429。

---

### 2.2 Wallet Recharge：下单 + Callback 到账

| 操作 | 端点 | 预期 |
|---|---|---|
| 创建充值订单 | `POST /api/v1/wallet/recharge` | 200，返回 `recharge_no`、`payment_id`、`status=pending` |
| 查订单 | `GET /api/v1/wallet/recharges/:recharge_no` | 200，status 与上面一致 |
| 模拟支付网关回调到账 | `POST /api/v1/payments/callback` | 200/200 兼容 GET；服务端把订单标 `completed`，钱包 available 增加 |
| 再查订单 | `GET /api/v1/wallet/recharges/:recharge_no` | status = `completed`，`paid_at` 非空 |
| 查钱包 | `GET /api/v1/wallet` | available_balance 增加了充值额 |
| 查流水 | `GET /api/v1/wallet/transactions` | 头部一条 direction=`credit`、type=recharge 的流水 |

```bash
# 下单（金额 1 CNY）
RECH=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/wallet/recharge \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"channel_id":<测试渠道ID>,"amount":1}')
echo "$RECH" | jq .
RECHARGE_NO=$(echo "$RECH" | jq -r '.data.recharge_no')
PAYMENT_ID=$(echo "$RECH" | jq -r '.data.payment_id')

# 模拟回调（具体 payload 字段以你接的渠道为准；这里只是打一下端点确认路由活）
curl -fsS -X POST "http://127.0.0.1:8080/api/v1/payments/callback?payment_id=$PAYMENT_ID&status=success"
```

> ⚠️ 回调的具体签名/字段由你接的支付渠道决定（代码里还有 `/api/v1/payments/webhook/stripe|paypal|dujiaopay`）。**生产 smoke 不要真打第三方回调**，用 admin 后台的「补单 / 手动确认」或测试渠道 mock 回调更安全。

**通过标准**：订单状态流转 `pending -> completed`，钱包 available 增加对应金额，流水表有一条 credit 记录。

---

### 2.3 Recharge Order 状态机

对应表 `wallet_recharge_orders`（`internal/modules/wallet/domain/recharge_order.go`）。
状态字符串由业务层控制，smoke 里只验证可观察到的字段：

| 阶段 | 观察 |
|---|---|
| 下单后 | `status != ''`，`paid_at IS NULL` |
| 回调后 | `status = 'completed'`（或代码里对应的成功态字符串），`paid_at` 非空 |
| 钱包流水 | `wallet_transactions.direction = 'credit'`，`reference` 唯一 |

SQL 探针：
```sql
SELECT recharge_no, status, amount, paid_at
FROM wallet_recharge_orders
WHERE user_id = <smoke-user-a-id>
ORDER BY id DESC LIMIT 5;
```

---

### 2.4 Refund / After-Sale（小额）

| 操作 | 端点 | 预期 |
|---|---|---|
| 用户对一笔已完成订单发起售后 | `POST /api/v1/orders/:order_no/after-sale` | 201/200，生成 after_sale 记录 |
| Admin 同意 partial refund | `POST /api/v1/admin/orders/:id/refund-to-wallet` | 200，钱包 available 增加退款额 |
| Admin 走全额 manual refund | `POST /api/v1/admin/orders/:id/manual-refund` | 200，订单状态关闭 |

```bash
# 以 user A 视角申请售后（先得有一笔已完成 order）
curl -fsS -X POST "http://127.0.0.1:8080/api/v1/orders/$ORDER_NO/after-sale" \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"reason":"smoke test","amount":1}'

# Admin 退款回钱包
curl -fsS -X POST "http://127.0.0.1:8080/api/v1/admin/orders/$ORDER_ID/refund-to-wallet" \
  -H "Authorization: Bearer $TOKEN_ADMIN" -H 'Content-Type: application/json' \
  -d '{"amount":1,"remark":"smoke"}'
```

**通过标准**：用户钱包 available 增加退款金额；`wallet_transactions` 多一条 credit、`type=refund`；订单状态正确推进。

---

### 2.5 Withdrawal（小额申请 / Reject / Cancel）

| 操作 | 端点 | 预期 |
|---|---|---|
| 用户加提现地址 | `POST /api/v1/wallet/withdrawal-addresses` | 200 |
| 用户申请提现 1 CNY | `POST /api/v1/wallet/withdrawals` | 201，钱包 available **减少**、frozen **增加**（申请即冻结） |
| Admin 拒掉这笔 | `POST /api/v1/admin/wallet/withdrawals/:id/reject` | 200，钱包 frozen **释放回** available |
| 再申请一笔然后用户自己取消 | `POST /api/v1/wallet/withdrawals` → `POST /api/v1/wallet/withdrawals/:id/cancel` | cancel 后 frozen 也释放 |

```bash
# 用户申请提现
WD=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/wallet/withdrawals \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"address_id":<addr_id>,"amount":1}')
WD_ID=$(echo "$WD" | jq -r '.data.id')

# Admin reject
curl -fsS -X POST "http://127.0.0.1:8080/api/v1/admin/wallet/withdrawals/$WD_ID/reject" \
  -H "Authorization: Bearer $TOKEN_ADMIN" -H 'Content-Type: application/json' \
  -d '{"remark":"smoke reject"}'
```

**通过标准**：
- 申请瞬间：available -= amount，frozen += amount，total 不变。
- reject/cancel 后：frozen -= amount，available += amount，total 不变。
- `wallet_transactions` 出现 freeze / unfreeze 两条对应流水。

---

### 2.6 Invitation / Affiliate

| 操作 | 端点 | 预期 |
|---|---|---|
| 邀请人 A 查自己邀请信息 | `GET /api/v1/invitation/me` | 200，返回自己的邀请码 |
| 被邀请人 B 注册时带邀请码注册 | `POST /api/v1/auth/register`（body 带 invitation_code） | 200，B 与 A 绑定关系落库 |
| A 开通 affiliate | `POST /api/v1/affiliate/open` | 200 |
| B 下一笔已完成订单后 | `GET /api/v1/affiliate/commissions`（A 的 token） | 列表里出现新佣金记录 |

```bash
# A 查邀请码
curl -fsS -H "Authorization: Bearer $TOKEN_A" http://127.0.0.1:8080/api/v1/invitation/me

# B 注册（带邀请码；实际字段名以 DTO 为准，常见 invite_code / referral_code）
curl -fsS -X POST http://127.0.0.1:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"smoke-user-b@test.local","password":"...","invite_code":"<A的邀请码>"}'

# B 完成订单后，A 看佣金
curl -fsS -H "Authorization: Bearer $TOKEN_A" http://127.0.0.1:8080/api/v1/affiliate/commissions
```

**通过标准**：绑定关系落库；B 完成订单后 A 的佣金列表新增一条。

---

### 2.7 C2C（两个测试用户 A=Seller / B=Buyer）

实际路由（`internal/modules/c2c/transport/http/routes.go`）：
- `POST /api/v1/c2c/listings` — 挂单
- `GET /api/v1/c2c/listings/market` — 市场列表
- `POST /api/v1/c2c/trades` — 下单
- `POST /api/v1/c2c/trades/:id/mark-paid` — 买家标已付款
- `POST /api/v1/c2c/trades/:id/confirm` — 卖家放行
- `POST /api/v1/c2c/trades/:id/cancel` — 取消
- 状态机（`internal/modules/c2c/statemachine/trade_status.go`）：
  `pending_payment → paid → completed`；`pending_payment → expired`；`paid → disputed → completed`

#### 用例 2.7.1：正常成交

| 步骤 | 谁 | 端点 | 钱包变化 |
|---|---|---|---|
| A 挂 SELL 100 USDT | A | `POST /c2c/listings` | 无 |
| B 下单买 50 USDT | B | `POST /c2c/trades` | A 的 frozen += 50 USDT 等值；A available -= 50 |
| B mark-paid | B | `POST /c2c/trades/:id/mark-paid` | 状态 `pending_payment → paid` |
| A confirm 放行 | A | `POST /c2c/trades/:id/confirm` | A frozen -= 50；B available += 50（扣手续费后）；状态 `paid → completed` |

```bash
# A 挂单
LIST=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/c2c/listings \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"side":"SELL","price":7.0,"total_amount":100,"min_amount":10,"payment_method_ids":[<pm_id>]}')
LIST_ID=$(echo "$LIST" | jq -r '.data.id')

# B 下单 50
TRADE=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/c2c/trades \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' \
  -d "{\"listing_id\":$LIST_ID,\"usdt_amount\":50}")
TRADE_ID=$(echo "$TRADE" | jq -r '.data.id')

# B 标付款
curl -fsS -X POST http://127.0.0.1:8080/api/v1/c2c/trades/$TRADE_ID/mark-paid \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' \
  -d '{"payment_reference":"smoke-001"}'

# A 放行
curl -fsS -X POST http://127.0.0.1:8080/api/v1/c2c/trades/$TRADE_ID/confirm \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' -d '{}'
```

**通过标准**：
- 下单瞬间 A 的 `frozen_balance` 增加 50 USDT 等值。
- confirm 后 A 的 `frozen_balance` 减 50，B 的 `available_balance` 加 ~50。
- `c2c_trades.status = 'completed'`，`completed_at` 非空。
- `wallet_transactions` 出现 freeze / unfreeze / credit 三条对应记录。

#### 用例 2.7.2：Cancel / Expire → Seller Unfreeze

| 步骤 | 端点 | 钱包变化 |
|---|---|---|
| A 再挂一单、B 再下一单 | 同上 | A frozen += X |
| B 直接 cancel | `POST /c2c/trades/:id/cancel` | A frozen -= X，available 恢复 |

```bash
# 第二笔同样流程下到 pending_payment 后
curl -fsS -X POST http://127.0.0.1:8080/api/v1/c2c/trades/$TRADE2_ID/cancel \
  -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d '{}'
```

**通过标准**：cancel 后 A 的 frozen 完全释放回 available，total 不变。
（Expire 路径由后台 job 扫描 `expired_at < now()` 自动推进，smoke 里可人工等超时或直接信 `expire.go` 单测。）

---

### 2.8 Notification

| 操作 | 端点 | 预期 |
|---|---|---|
| 触发一条通知（例如 Admin 回工单后 user A 收到） | `GET /api/v1/notifications/unread-count` | 数字 > 0 |
| 拉列表 | `GET /api/v1/notifications` | 列表第一条就是刚触发的那条，带 deep_link 字段 |
| 标已读 | `POST /api/v1/notifications/:id/read` | unread-count 减 1 |
| 全部已读 | `POST /api/v1/notifications/read-all` | unread-count = 0 |

```bash
curl -fsS -H "Authorization: Bearer $TOKEN_A" http://127.0.0.1:8080/api/v1/notifications/unread-count
```

**通过标准**：红点数字变化正确，deep_link 字段非空（前端能跳）。

---

### 2.9 Ticket（User 创建 / Admin 回复 / User 收通知）

| 操作 | 谁 | 端点 | 预期 |
|---|---|---|---|
| 建工单 | User A | `POST /api/v1/support/tickets` | 201，返回 ticket id |
| 上传附件（可选） | User A | `POST /api/v1/support/attachments` | 200 |
| Admin 回复 | Admin | `POST /api/v1/admin/support/tickets/:id/replies` | 200 |
| User A 收到通知 | User A | `GET /api/v1/notifications/unread-count` | 数字 +1 |
| User A 看工单详情 | User A | `GET /api/v1/support/tickets/:id` | 能看到 Admin 那条回复 |

```bash
TICKET=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/support/tickets \
  -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"category_id":<id>,"subject":"smoke","content":"hi"}')
TICKET_ID=$(echo "$TICKET" | jq -r '.data.id')

curl -fsS -X POST http://127.0.0.1:8080/api/v1/admin/support/tickets/$TICKET_ID/replies \
  -H "Authorization: Bearer $TOKEN_ADMIN" -H 'Content-Type: application/json' \
  -d '{"content":"hello from admin"}'
```

**通过标准**：全链路闭环；User A 通知数 +1；工单详情里能看到双方消息。

---

### 2.10 Site Builder / 品牌配置 + Redis Invalidate

| 操作 | 端点 | 预期 |
|---|---|---|
| Admin 改品牌名 | `PUT /api/v1/admin/site/brand` | 200 |
| Admin 改首页入口 | `PUT /api/v1/admin/site/home-entries/:id` 或 `POST /api/v1/admin/site/home-entries/reorder` | 200 |
| 等缓存失效 / 手动清缓存 | `redis-cli -n 0 --scan --pattern 'dj:settings:*' \| xargs redis-cli -n 0 DEL` | 清完 |
| User B 拉公开配置 | `GET /api/v1/public/config` | 看到新品牌名 |

```bash
# Admin 改品牌
curl -fsS -X PUT http://127.0.0.1:8080/api/v1/admin/site/brand \
  -H "Authorization: Bearer $TOKEN_ADMIN" -H 'Content-Type: application/json' \
  -d '{"site_name":"HCZ-Smoke-Test"}'

# 清缓存
redis-cli -n 0 --scan --pattern 'dj:settings:*' | xargs -r redis-cli -n 0 DEL

# User 端立即看到
curl -fsS http://127.0.0.1:8080/api/v1/public/config | jq '.site_name'
# 期望："HCZ-Smoke-Test"
```

**通过标准**：改完 + 清缓存后 user 端公开配置立刻生效。

---

## 3. Wallet Reconciliation 验证

### 3.1 表结构（已从代码确认）

**`wallet_accounts`**（`internal/modules/wallet/domain/account.go`）：

| 列 | 类型 | 说明 |
|---|---|---|
| id | uint PK | |
| user_id | uint unique | |
| available_balance | decimal(20,2) | 可用余额 |
| frozen_balance | decimal(20,2) | 冻结余额 |
| created_at / updated_at / deleted_at | | |

**`wallet_transactions`**（`internal/modules/wallet/domain/transaction.go`）：

| 列 | 类型 | 说明 |
|---|---|---|
| id | uint PK | |
| user_id | uint index | |
| operator_admin_id | uint? | 后台操作人 |
| order_id | uint? | |
| type | varchar(40) | recharge / refund / withdrawal_freeze / withdrawal_unfreeze / c2c_freeze / c2c_release / ... |
| direction | varchar(16) | `credit` / `debit` |
| amount | decimal(20,2) | 带符号前的绝对额 |
| balance_before / balance_after | decimal(20,2) | 旧 balance 列快照 |
| available_before / available_after | decimal(20,2) | |
| frozen_before / frozen_after | decimal(20,2) | |
| currency | varchar(16) | 默认 CNY |
| reference | varchar(120) unique | 幂等键 |
| remark | varchar(255) | |

### 3.2 Smoke 前后快照

在跑 smoke 之前 / 之后，对每个测试账号各跑一次：

```sql
-- 替换 <user_id>
SELECT id,
       user_id,
       available_balance,
       frozen_balance,
       (available_balance + frozen_balance) AS total_balance,
       updated_at
FROM wallet_accounts
WHERE user_id IN (<uid_a>, <uid_b>, <uid_admin>);
```

把三行结果存成 CSV（`smoke_before.csv` / `smoke_after.csv`）。

同时拉出该窗口内的所有流水：

```sql
SELECT id, user_id, type, direction, amount,
       available_before, available_after,
       frozen_before, frozen_after,
       reference, remark, created_at
FROM wallet_transactions
WHERE user_id IN (<uid_a>, <uid_b>)
  AND created_at >= '<smoke开始时间>'
ORDER BY id;
```

### 3.3 对账公式

对每个 user：

```
final.available + final.frozen
  = initial.available + initial.frozen
    + SUM(credit.amount)
    - SUM(debit.amount)
```

SQL 版（一次性跑）：

```sql
WITH init_snap AS (
  SELECT user_id, available_balance AS a0, frozen_balance AS f0
  FROM wallet_accounts_snapshot_before  -- 你自己建的临时表，或手工填
),
delta AS (
  SELECT user_id,
         SUM(CASE WHEN direction='credit' THEN amount ELSE 0 END) AS credits,
         SUM(CASE WHEN direction='debit'  THEN amount ELSE 0 END) AS debits
  FROM wallet_transactions
  WHERE user_id IN (<uid_a>, <uid_b>)
    AND created_at >= '<smoke开始时间>'
  GROUP BY user_id
),
final_snap AS (
  SELECT user_id, available_balance AS a1, frozen_balance AS f1
  FROM wallet_accounts
  WHERE user_id IN (<uid_a>, <uid_b>)
)
SELECT f.user_id,
       (a0 + f0)                                   AS initial_total,
       COALESCE(credits,0) - COALESCE(debits,0)    AS net_change,
       (a1 + f1)                                   AS final_total,
       ((a0 + f0) + COALESCE(credits,0) - COALESCE(debits,0)) - (a1 + f1) AS diff
FROM final_snap f
JOIN init_snap i USING (user_id)
LEFT JOIN delta d USING (user_id);
```

**通过标准**：每行 `diff = 0.00`。
**失败处理**：`diff != 0` 立刻停止继续 smoke，进入 [第 6.4 节 资金数据异常应急](#64-资金数据异常应急)。

### 3.4 余额自洽校验（每笔流水前后快照必须连续）

```sql
-- 同一个 user 的流水，后一行的 *_before 必须等于前一行的 *_after
WITH t AS (
  SELECT id, user_id,
         available_before, available_after,
         frozen_before, frozen_after,
         LAG(available_after)  OVER (PARTITION BY user_id ORDER BY id) AS prev_avail_after,
         LAG(frozen_after)     OVER (PARTITION BY user_id ORDER BY id) AS prev_frozen_after
  FROM wallet_transactions
  WHERE user_id IN (<uid_a>, <uid_b>)
)
SELECT * FROM t
WHERE prev_avail_after IS NOT NULL
  AND (available_before <> prev_avail_after
       OR frozen_before <> prev_frozen_after);
```

**通过标准**：0 行返回。有返回说明流水链断了 → 资金异常。

---

## 4. Monitoring 配置

### 4.1 项目内已有 / 没有的能力

| 能力 | 状态 | 证据 |
|---|---|---|
| `GET /health` | ✅ 有，返回 `{"status":"ok"}` | `internal/app/httpserver/router.go:336` |
| `/metrics` Prometheus | ❌ 没有 | grep `prometheus|promhttp` 全仓 0 命中 |
| 结构化日志 | ✅ 有，zap（`internal/logger`），JSON 输出 | `config.log.*` 配置文件路径、滚动 |
| panic 恢复 | ✅ 有，`middleware.RecoveryMiddleware` | `router.go:303` |
| 后台 job | ✅ 有，Asynq 在 Redis db 1 | `internal/app/jobs/consumer/*` |

**结论**：监控只能走三条路：
1. **黑盒**：外部探针打 `/health`、打关键页面。
2. **日志关键字**：从 `/opt/hcz/logs/app.log` 或 `journalctl -u hcz` 里 grep。
3. **SQL 探针**：定时跑下面的 SELECT，把结果送进 Prometheus (postgres_exporter / custom script)。

### 4.2 监控清单

| # | 监控对象 | 告警阈值 | 数据来源 |
|---|---|---|---|
| 1 | API 5xx 率 | 5 分钟内 5xx 占比 > 1% | Nginx access log `status=5xx`；或日志中间件输出 |
| 2 | panic | 任意 1 条即告警 | 日志关键字 `panic` / `recovery` / `stack trace` |
| 3 | PG 连接数 | `numbackends` > 80% 配置上限；慢查询 > 5s；复制延迟 > 1MB | SQL：`SELECT numbackends FROM pg_stat_database WHERE datname='<db>';`；`SELECT now() - pg_last_xact_replay_timestamp();`（只读从库） |
| 4 | Redis | 内存 > 80% maxmemory；连接数 > 80% maxclients；命中率 < 90% | `redis-cli INFO memory`、`redis-cli INFO stats`（keyspace_hits / keyspace_misses） |
| 5 | CoinGecko refresh 失败 | 连续 3 次失败 | 日志关键字 `exchange_rate_refresh_failed`（`internal/app/jobs/consumer/consumer_exchangerate.go:18`） |
| 6 | payment callbacks 失败率 | 10 分钟内 callback 非 2xx / 验签失败 > 5% | 日志关键字 `payment_callback` / `callback.*fail`；Nginx log 里 `/api/v1/payments/callback*` 状态码 |
| 7 | wallet ledger 写入失败 | 任意 1 条即告警 | 日志关键字 `ledger` / `wallet_transactions.*error` / `insufficient balance` |
| 8 | withdrawal failures | 10 分钟内 `wallet_withdrawals` 出现 `failed` 状态 > 0 | SQL：`SELECT count(*) FROM wallet_withdrawals WHERE status='failed' AND created_at > now() - interval '10 minutes';` |
| 9 | C2C frozen 异常 | 任一 user `frozen_balance > 0` 持续 > 24h 且对应 trade 不在 `paid` 状态 | SQL：见 4.4 |
| 10 | C2C disputed backlog | `c2c_trades.status='disputed'` 数量 > 0 且最长 > 2h | SQL：`SELECT count(*), max(created_at) FROM c2c_trades WHERE status='disputed';` |
| 11 | ticket backlog | `support_tickets` 未关闭且 > 24h 未回复 | SQL：见 4.4 |
| 12 | notification 推送失败率 | 10 分钟内推送失败 > 1% | 日志关键字 `notification.*fail` / `telegram.*send.*error` |

### 4.3 Prometheus 告警规则示例 YAML

> 项目本身**没有** `/metrics`，下面规则假定你已经用 node_exporter / postgres_exporter / redis_exporter / 黑盒 blackbox_exporter 把这些指标拉到 Prometheus。

```yaml
groups:
  - name: hcz
    rules:
      # 1. 健康检查挂了（blackbox_exporter 打 /health）
      - alert: HczHealthDown
        expr: probe_success{job="hcz"} == 0
        for: 1m
        labels: { severity: critical }
        annotations:
          summary: "HCZ /health 探测失败"

      # 2. 5xx 率（从 nginx ingress / gin log 经 promtail -> loki，或 nginx-vts-exporter）
      - alert: HczHigh5xx
        expr: sum(rate(http_requests_total{job="hcz",status=~"5.."}[5m]))
              / sum(rate(http_requests_total{job="hcz"}[5m])) > 0.01
        for: 5m
        labels: { severity: critical }

      # 3. PG 连接数
      - alert: PgHighConnections
        expr: pg_stat_activity_count / pg_settings_max_connections > 0.8
        for: 5m
        labels: { severity: warning }

      # 4. Redis 内存
      - alert: RedisHighMem
        expr: redis_memory_used_bytes / redis_memory_max_bytes > 0.8
        for: 5m
        labels: { severity: warning }

      # 5. Redis 命中率
      - alert: RedisLowHitRate
        expr: rate(redis_keyspace_hits_total[5m])
              / (rate(redis_keyspace_hits_total[5m]) + rate(redis_keyspace_misses_total[5m])) < 0.9
        for: 10m
        labels: { severity: warning }

      # 6. C2C 争议积压
      #   建议用 custom exporter 跑 4.4 的 SQL，暴露 gauge c2c_disputed_count
      - alert: C2CDisputeBacklog
        expr: c2c_disputed_count > 0
        for: 2h
        labels: { severity: warning }
```

### 4.4 SQL 探针（定时跑，例如每分钟）

```sql
-- 9. C2C frozen 异常：frozen 非零但没有对应的 in-flight trade
SELECT wa.user_id, wa.frozen_balance,
       (SELECT count(*) FROM c2c_trades t
         WHERE t.buyer_user_id = wa.user_id
            OR t.seller_user_id = wa.user_id
           AND t.status IN ('pending_payment','paid','disputed')) AS active_trades
FROM wallet_accounts wa
WHERE wa.frozen_balance > 0
HAVING (SELECT count(*) FROM c2c_trades t
         WHERE (t.buyer_user_id = wa.user_id OR t.seller_user_id = wa.user_id)
           AND t.status IN ('pending_payment','paid','disputed')) = 0;

-- 11. Ticket backlog：未关闭且超过 24h 没有 admin 回复
SELECT t.id, t.subject, t.created_at,
       (SELECT max(created_at) FROM support_ticket_replies r
         WHERE r.ticket_id = t.id AND r.admin_id IS NOT NULL) AS last_admin_reply
FROM support_tickets t
WHERE t.status NOT IN ('closed','resolved')
  AND t.created_at < now() - interval '24 hours'
  AND (SELECT max(created_at) FROM support_ticket_replies r
        WHERE r.ticket_id = t.id AND r.admin_id IS NOT NULL) < now() - interval '24 hours';
```

> 表名 `support_ticket_replies` / `support_tickets` 请以实际 `domain/*.go` 的 `TableName()` 为准；若名字不同，grep 一次 `TableName()` 替换即可。

### 4.5 日志告警关键字（Loki / ELK）

| 关键字 | 含义 | 级别 |
|---|---|---|
| `panic` | goroutine panic | critical |
| `database migration failed` / `数据库迁移失败` | migration 挂了 | critical |
| `database initialization failed` / `数据库初始化失败` | 连不上 PG | critical |
| `exchange_rate_refresh_failed` | CoinGecko 拉不到 | warning |
| `payment_callback` + `error` | 支付回调验签/入账失败 | critical |
| `insufficient balance` / `余额不足` | 钱包扣减失败（可能业务，但连续出现要查） | warning |
| `ledger` + `error` | 流水写入失败 | critical |
| `queue.*error` / `asynq` + `error` | 后台 job 失败 | warning |

---

## 5. 部署后观察窗口

发布完成后**至少观察 30 分钟**再下班：

```bash
# 实时盯日志
sudo journalctl -u hcz -f
# 或 Docker
docker logs -f --since 5m hcz
```

重点看：
- 有没有 `panic` / `Fatal` / `database migration failed`。
- 5xx 比例。
- C2C / Withdrawal / Recharge 有没有非预期状态。

---

## 6. Rollback Runbook

### 6.0 触发条件速查（决策树）

```
                ┌─ health 5xx / 起不来 ───────────────► 6.1 Application Rollback
                │
   Smoke / 观察 ─┼─ 资金路径异常（余额对不上） ────────► 6.4 资金应急（先停写入口）
                │
                ├─ 配置写错、密钥错 ──────────────────► 6.2 Config Rollback
                │
                ├─ AutoMigrate 报错、起不来 ──────────► 6.1（DB 向后兼容）+ 人工排查
                │
                └─ 前端页面白屏 / 路由错乱 ────────────► 6.1（前端 embed 在 binary 里）
```

---

### 6.1 Application Rollback（最常用）

> **前置**：Step 5 已经留了旧 binary / 旧 image tag。
> 项目自带 CLI：`./hcz rollback [--force]`（`cmd/server/main.go:312`）。它会：
> 1. 把磁盘上的 `hcz` 换回 `hcz.backup`。
> 2. 不连 DB、不加载 config，纯文件操作。
> 3. 若新版本已经开始 migration 或成功启动过，会**拒绝回滚**（fail-closed），要求 `--force`。

#### 裸金属形态（systemd）

```bash
# 1) 先看一眼状态（不会动任何东西）
sudo /opt/hcz/hcz rollback
# 输出会告诉你：当前版本、回滚到哪一版、是否有风险提示

# 2) 正常回滚（DB 没动过 / 新版本没启动成功）
sudo systemctl stop hcz.service
sudo /opt/hcz/hcz rollback

# 3) 如果提示 "已拒绝回滚：新版本已开始迁移..."，先确认 PG 已备份，再强制
sudo /opt/hcz/hcz rollback --force

# 4) 重启
sudo systemctl start hcz.service

# 5) 验证
curl -fsS http://127.0.0.1:8080/health
sudo journalctl -u hcz --since "1 minute ago" | grep -iE 'error|fatal|panic'
```

> ⚠️ `rollback --force` 意味着「新 DB schema 配旧二进制」。本项目 migration 只 ADD 不 DROP，旧二进制**通常**能跑（它不认识新列而已），但**不是零风险**。强制前必须确保 Step 2 的 pg_dump 在手上。

#### Docker 形态

```bash
# 1) 停新容器
docker stop hcz && docker rm hcz

# 2) 用 Step 5 记下的旧 tag 起回来
OLD_TAG=$(cat /opt/hcz/backups/image.before.txt)
docker run -d --name hcz \
  --restart=on-failure \
  -p 127.0.0.1:8080:8080 \
  -v /opt/hcz/config.yml:/app/config.yml:ro \
  -v /opt/hcz/uploads:/app/uploads \
  -v /opt/hcz/logs:/app/logs \
  "$OLD_TAG"

# 3) 验证
curl -fsS http://127.0.0.1:8080/health
docker logs --tail 100 hcz
```

#### 前端怎么办？

> Admin / User SPA 都 `go:embed` 在二进制里，**回滚二进制 = 回滚前端**，没有独立回滚动作。

#### 验证标准

- `systemctl is-active hcz` = active（或 `docker ps` 看到 hcz Up）。
- `/health` 200。
- 旧版本号能打印出来。
- Smoke 里最关键的 3 个用例（登录 / 钱包查询 / C2C 列表）能跑通。

**预计耗时**：3–5 分钟。

---

### 6.2 Config Rollback

触发：新版本起不来，日志明确是 `config ... error` / 密钥弱检查失败 / admin_path 非法。

```bash
# 1) 找到 Step 3 留的备份
ls -l /opt/hcz/backups/config.yml.*.bak

# 2) 覆盖回去
sudo cp -a /opt/hcz/backups/config.yml.<旧时间戳>.bak /opt/hcz/config.yml

# 3) 重启
sudo systemctl restart hcz.service

# 4) 验证
curl -fsS http://127.0.0.1:8080/health
```

**注意**：如果新版本引入了**新的必填配置项**，旧 config 起来后可能会报「字段缺失」。这种情况下不能纯 config rollback，必须配合 6.1 一起回滚二进制。

---

### 6.3 DB Rollback（尽量不做）

**原则**：
- 当前 migration 只 ADD 不 DROP。新版本加的列/表对旧二进制**无影响**（GORM 忽略未知列）。
- 所以 **99% 的场景下，做 6.1 就够了，DB 不用动**。
- **不要**手动 DROP 新版本加的列/表 —— 那会让正在跑的新代码（如果还有一个副本）直接挂掉。

**只有以下两种情况才动 DB**：
1. 新版本 migration 加了**带副作用**的列（例如加了 NOT NULL 但没 default、建了唯一约束把旧数据挡在外面）。
2. migration 中途崩了，表结构半新半旧，旧二进制也起不来。

**做法（从 pg_dump 恢复，RPO = 备份时刻，数据会丢）**：

```bash
# 1) 先停应用，杜绝新写入
sudo systemctl stop hcz.service

# 2) 用 Step 2 的 dump 恢复
export PGPASSWORD='<DB密码>'
pg_restore -h <DB_HOST> -U <DB_USER> -d <DB_NAME> --clean --if-exists /opt/hcz/backups/db_<TS>.dump

# 3) 同时把二进制回到旧版本（6.1）
sudo /opt/hcz/hcz rollback --force

# 4) 启动
sudo systemctl start hcz.service

# 5) 验证 + 对账
curl -fsS http://127.0.0.1:8080/health
# 跑 3.3 的对账 SQL，确认钱包余额自洽
```

**RPO / RTO 声明**：从 pg_dump 恢复会丢失「备份时刻之后到现在」的所有写入（用户充值、订单、提现）。**执行前必须**：
- 通知业务方接受数据丢失。
- 把当前 WAL 或增量 dump 再留一份，以便事后补账。

---

### 6.4 资金数据异常应急

触发：Smoke 对账 `diff != 0`；或观察窗口内用户报「钱不对」；或日志大量 `ledger.*error`。

**动作顺序**：

1. **立刻停写入口**（不要先查代码）：
   ```bash
   # 挂维护页（同 Step 1）
   sudo touch /etc/nginx/maintenance.enabled && sudo systemctl reload nginx

   # 或者只关资金相关 API（更精准，靠 Nginx location 拒绝）：
   #  location ~ ^/api/v1/(wallet|c2c|orders) { return 503; }
   ```

2. **冻结对应业务**（admin 后台操作）：
   - 关闭充值渠道：`PUT /api/v1/admin/payment-channels/:id` 把 `enabled=false`。
   - 禁用 C2C 用户：`POST /api/v1/admin/c2c/users/:id/disable`（先 disable 测试账号自己，再看情况扩大）。
   - 关闭提现：admin settings 里把提现开关关掉（或直接把 `wallet_withdrawals` 新单 status 拦住）。

3. **不要**自行 `UPDATE wallet_accounts SET available_balance=...` 修余额。
   - 余额必须由 `wallet_transactions` 流水推导出来，反向改余额会让流水链断掉（3.4 的校验直接红）。

4. **执行对账**：按 3.3 / 3.4 跑 SQL，定位是「哪笔交易」断了：
   ```sql
   -- 找前后快照不连续的流水
   SELECT id, user_id, type, direction, amount,
          available_before, available_after, frozen_before, frozen_after, reference
   FROM wallet_transactions
   WHERE id IN (<可疑区间>)
   ORDER BY user_id, id;
   ```

5. **定位后由开发团队出修复方案**（补流水 / 补余额 / 给用户人工补偿），在 staging 演练一遍再上生产。

---

### 6.5 回滚决策树（一页纸版）

```
新版本部署后
├── /health 不通 / 进程 crash-loop
│   ├── 日志：config 错 ───────────────► 6.2 Config Rollback
│   ├── 日志：migration 错 ────────────► 6.1 Application Rollback（DB 不动）
│   └── 其他 ─────────────────────────► 6.1 Application Rollback
│
├── /health 通，但 Smoke 失败
│   ├── 前端白屏 / 路由 404 ────────────► 6.1（前端 embed 在 binary）
│   ├── 资金路径不对账 ────────────────► 6.4 资金应急（先停写！）
│   ├── C2C / Withdrawal 业务逻辑错 ────► 6.1
│   └── 通知 / 工单 / SiteBuilder 错 ──► 6.1（非资金，可观察一会再决定）
│
├── /health 通，Smoke 通，观察窗口告警
│   ├── CoinGecko 连续失败 ────────────► 不用回滚，切备用汇率源 / 手动 refresh
│   ├── PG / Redis 资源告警 ────────────► 扩容，不回滚
│   └── 5xx 飙升 ───────────────────────► 先看日志定位；定位不到就 6.1
│
└── DB migration 已经跑了一半就崩
    └── 6.1（旧二进制向后兼容）+ 人工看 migration 日志
        └── 真的半新半旧跑不起来 ──► 6.3 DB Rollback（接受 RPO 数据丢失）
```

---

## 附录 A. 关键路径速查（全部来自代码 grep）

| 用途 | 方法 | 路径 |
|---|---|---|
| 健康检查 | GET | `/health` |
| 用户注册 | POST | `/api/v1/auth/register` |
| 用户登录 | POST | `/api/v1/auth/login` |
| Admin 登录 | POST | `/api/v1/admin/login` |
| 我的信息 | GET | `/api/v1/me` |
| 我的钱包 | GET | `/api/v1/wallet` |
| 钱包流水 | GET | `/api/v1/wallet/transactions` |
| 创建充值 | POST | `/api/v1/wallet/recharge` |
| 充值单列表 | GET | `/api/v1/wallet/recharges` |
| 充值单详情 | GET | `/api/v1/wallet/recharges/:recharge_no` |
| 支付回调 | POST/GET | `/api/v1/payments/callback` |
| Stripe / PayPal / 独角数卡 webhook | POST | `/api/v1/payments/webhook/{stripe,paypal,dujiaopay}` |
| 申请提现 | POST | `/api/v1/wallet/withdrawals` |
| 取消提现 | POST | `/api/v1/wallet/withdrawals/:id/cancel` |
| Admin 拒提现 | POST | `/api/v1/admin/wallet/withdrawals/:id/reject` |
| Admin 通过提现 | POST | `/api/v1/admin/wallet/withdrawals/:id/approve` |
| 创建订单 | POST | `/api/v1/orders` |
| 取消订单 | POST | `/api/v1/orders/:order_no/cancel` |
| 申请售后 | POST | `/api/v1/orders/:order_no/after-sale` |
| Admin 退款到钱包 | POST | `/api/v1/admin/orders/:id/refund-to-wallet` |
| C2C 挂单 | POST | `/api/v1/c2c/listings` |
| C2C 市场 | GET | `/api/v1/c2c/listings/market` |
| C2C 下单 | POST | `/api/v1/c2c/trades` |
| C2C 标付款 | POST | `/api/v1/c2c/trades/:id/mark-paid` |
| C2C 确认放行 | POST | `/api/v1/c2c/trades/:id/confirm` |
| C2C 取消 | POST | `/api/v1/c2c/trades/:id/cancel` |
| C2C 争议 | POST | `/api/v1/c2c/trades/:id/dispute` |
| 通知列表 | GET | `/api/v1/notifications` |
| 未读数 | GET | `/api/v1/notifications/unread-count` |
| 工单创建 | POST | `/api/v1/support/tickets` |
| Admin 回工单 | POST | `/api/v1/admin/support/tickets/:id/replies` |
| 邀请信息 | GET | `/api/v1/invitation/me` |
| 开通 affiliate | POST | `/api/v1/affiliate/open` |
| affiliate 佣金 | GET | `/api/v1/affiliate/commissions` |
| 品牌配置 | GET/PUT | `/api/v1/admin/site/brand` |
| 首页入口 | GET/POST/PUT/DELETE | `/api/v1/admin/site/home-entries...` |
| 汇率手动刷新 | POST | `/api/v1/admin/settings/exchange-rate/refresh` |

## 附录 B. 关键表速查

| 表 | 说明 | 关键字段 |
|---|---|---|
| `wallet_accounts` | 用户钱包 | user_id, available_balance, frozen_balance |
| `wallet_transactions` | 钱包流水（账本） | user_id, type, direction(credit/debit), amount, reference(unique), available_before/after, frozen_before/after |
| `wallet_recharge_orders` | 充值单 | recharge_no, user_id, payment_id, status, amount, paid_at |
| `wallet_withdrawals` | 提现单 | user_id, amount, status |
| `c2c_trades` | C2C 交易 | trade_no, buyer_user_id, seller_user_id, usdt_amount, status(pending_payment/paid/completed/expired/disputed), expired_at |
| `c2c_listings` | C2C 挂单 | seller_user_id, side(SELL/BUY), price, total_amount, status |
| `c2c_disputes` | C2C 争议 | trade_id, status |

---

**End of Runbook.**
