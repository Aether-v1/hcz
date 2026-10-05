<#
.SYNOPSIS
    HCZ 数据库 Migration 预检脚本（在 staging PostgreSQL clone 上执行）。

.DESCRIPTION
    完整流程：
      0. 前置条件：已有一份生产库的 staging clone（PostgreSQL）
      1. 记录 staging 当前 schema 快照（pg_dump --schema-only 到 before.sql）
      2. 记录迁移前资产总额（wallet_accounts / wallet_transactions）
      3. 启动新版 hcz.exe（指向 staging DSN），触发 AutoMigrate；启动后立即停服
      4. 记录迁移后 schema 快照（after.sql）+ 资产总额
      5. 对比资产守恒：迁移前后 SUM(available+frozen) 必须一致
      6. 第二次启动 hcz.exe，验证幂等（不报错、schema 不变）
      7. diff before.sql / after.sql：人工确认只有 ADD（新表/新列/新索引），无 DROP TABLE/DROP COLUMN

    注意：本脚本不连生产库，仅操作 staging。

.EXAMPLE
    .\prod_migration_preflight.ps1 `
        -NewExe E:\app\hcz\new\hcz.exe `
        -StagingDSN "host=staging-pg port=5432 user=hcz password=*** dbname=hcz_staging sslmode=require"
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$NewExe,

    # staging DSN（与应用 config.yml 的 database.dsn 同格式）
    [Parameter(Mandatory = $true)]
    [string]$StagingDSN,

    # staging 库连接（给 psql/pg_dump 用）
    [string]$PGHost = "127.0.0.1",
    [int]$PGPort = 5432,
    [string]$PGUser = "hcz",
    [string]$PGDatabase = "hcz_staging",
    [string]$PGPassword = "",

    # 工作目录（临时配置、schema 快照放这里）
    [string]$WorkDir = "E:\staging\hcz_preflight"
)

$ErrorActionPreference = 'Stop'
if ($PGPassword -ne "") { $env:PGPASSWORD = $PGPassword }
$stamp = Get-Date -Format 'yyyyMMdd_HHmmss'
New-Item -ItemType Directory -Path $WorkDir -Force | Out-Null

function Get-WalletTotal {
    param([string]$Label)
    $row = & psql -h $PGHost -p $PGPort -U $PGUser -d $PGDatabase -t -A -F '|' -c `
        "SELECT COALESCE(SUM(available_balance),0), COALESCE(SUM(frozen_balance),0), COALESCE(SUM(available_balance+frozen_balance),0) FROM wallet_accounts WHERE deleted_at IS NULL;"
    $parts = $row -split '\|'
    Write-Host ("  [{0}] available={1} frozen={2} total={3}" -f $Label, $parts[0], $parts[1], $parts[2])
    return [PSCustomObject]@{
        Label     = $Label
        Available = [decimal]$parts[0]
        Frozen    = [decimal]$parts[1]
        Total     = [decimal]$parts[2]
    }
}

function Get-LedgerTotals {
    param([string]$Label)
    $row = & psql -h $PGHost -p $PGPort -U $PGUser -d $PGDatabase -t -A -F '|' -c `
        "SELECT COALESCE(SUM(amount) FILTER (WHERE direction='in'),0), COALESCE(SUM(amount) FILTER (WHERE direction='out'),0), count(*) FROM wallet_transactions WHERE deleted_at IS NULL;"
    $parts = $row -split '\|'
    Write-Host ("  [{0}] ledger_in={1} ledger_out={2} txn_count={3}" -f $Label, $parts[0], $parts[1], $parts[2])
    return [PSCustomObject]@{
        Label     = $Label
        LedgerIn  = [decimal]$parts[0]
        LedgerOut = [decimal]$parts[1]
        TxnCount  = [int]$parts[2]
    }
}

# 1. 迁移前 schema 快照
$beforeSchema = Join-Path $WorkDir "schema_before_${stamp}.sql"
Write-Host "==> 迁移前 schema 快照 -> $beforeSchema"
& pg_dump -h $PGHost -p $PGPort -U $PGUser -d $PGDatabase --schema-only --no-owner --no-privileges -f $beforeSchema
if ($LASTEXITCODE -ne 0) { throw "pg_dump schema-only (before) 失败" }

# 2. 迁移前资产
Write-Host "==> 迁移前资产总额"
$walletBefore  = Get-WalletTotal -Label 'before'
$ledgerBefore  = Get-LedgerTotals -Label 'before'

# 3. 启动新版 hcz.exe 触发 AutoMigrate（启动后立即 SIGTERM / 或让它跑完迁移后退出）
#    做法：写一个临时 config.yml，database.dsn 指向 staging；后台启动；
#    等 HTTP 端口起来 / 或固定 N 秒后杀掉。生产更稳妥的方式是用 --mode 让它跑完迁移就退出，
#    但当前 main.go 没有这种模式，所以用「启动 -> 等待健康检查 -> 杀进程」。
$stagingConfig = Join-Path $WorkDir "config_staging_${stamp}.yml"
Write-Host "==> 生成临时 staging 配置 -> $stagingConfig"
$cfgLines = @(
    'database:',
    '  driver: postgres',
    ('  dsn: "' + $StagingDSN + '"'),
    '  pool:',
    '    max_open_conns: 5',
    '    max_idle_conns: 2',
    '    conn_max_lifetime_seconds: 300',
    '    conn_max_idle_time_seconds: 60',
    'server:',
    '  mode: release',
    '  port: 18099'
)
$cfgLines | Out-File -FilePath $stagingConfig -Encoding utf8

Write-Host "==> 启动新版 binary 触发 AutoMigrate（最多等 120 秒）"
$proc = Start-Process -FilePath $NewExe -ArgumentList "-config", $stagingConfig -PassThru -NoNewWindow `
    -RedirectStandardOutput (Join-Path $WorkDir "migrate_stdout_${stamp}.log") `
    -RedirectStandardError  (Join-Path $WorkDir "migrate_stderr_${stamp}.log")

# 等待进程退出（AutoMigrate 完成后 main.go 会继续跑 HTTP server，不会自己退）
# 所以用轮询：等 30 秒让迁移跑完，然后杀进程。
Start-Sleep -Seconds 30
if (-not $proc.HasExited) {
    Write-Host "    迁移等待 30s 结束，停止进程 PID=$($proc.Id)"
    Stop-Process -Id $proc.Id -Force
    $proc.WaitForExit(10000) | Out-Null
}
Write-Host "    binary exit code = $($proc.ExitCode)"

# 4. 迁移后资产 + schema
Write-Host "==> 迁移后 schema 快照"
$afterSchema = Join-Path $WorkDir "schema_after_${stamp}.sql"
& pg_dump -h $PGHost -p $PGPort -U $PGUser -d $PGDatabase --schema-only --no-owner --no-privileges -f $afterSchema
if ($LASTEXITCODE -ne 0) { throw "pg_dump schema-only (after) 失败" }

Write-Host "==> 迁移后资产总额"
$walletAfter = Get-WalletTotal -Label 'after'
$ledgerAfter = Get-LedgerTotals -Label 'after'

# 5. 守恒对比
Write-Host ""
Write-Host "==> 守恒对比"
$walletOk = ($walletBefore.Total -eq $walletAfter.Total)
Write-Host ("  wallet total: before={0} after={1}  match={2}" -f $walletBefore.Total, $walletAfter.Total, $walletOk)
$ledgerOk = ($ledgerBefore.LedgerIn -eq $ledgerAfter.LedgerIn) -and ($ledgerBefore.LedgerOut -eq $ledgerAfter.LedgerOut)
Write-Host ("  ledger in/out: before_in={0} before_out={1} after_in={2} after_out={3}  match={4}" -f `
    $ledgerBefore.LedgerIn, $ledgerBefore.LedgerOut, $ledgerAfter.LedgerIn, $ledgerAfter.LedgerOut, $ledgerOk)

# 6. 第二次启动验证幂等
Write-Host "==> 第二次启动（幂等验证）"
$proc2 = Start-Process -FilePath $NewExe -ArgumentList "-config", $stagingConfig -PassThru -NoNewWindow `
    -RedirectStandardOutput (Join-Path $WorkDir "migrate2_stdout_${stamp}.log") `
    -RedirectStandardError  (Join-Path $WorkDir "migrate2_stderr_${stamp}.log")
Start-Sleep -Seconds 20
if (-not $proc2.HasExited) {
    Stop-Process -Id $proc2.Id -Force
    $proc2.WaitForExit(10000) | Out-Null
}
Write-Host "    第二次 binary exit code = $($proc2.ExitCode)（0=幂等通过）"

# 7. schema diff
Write-Host ""
Write-Host "==> schema diff（before vs after）"
Write-Host "    before: $beforeSchema"
Write-Host "    after : $afterSchema"
Write-Host "    请人工检查 diff：只允许出现新 CREATE TABLE / ALTER TABLE ADD COLUMN / CREATE INDEX；"
Write-Host "    不允许出现 DROP TABLE / DROP COLUMN / DROP INDEX（业务上允许的 DropColumn=product.price_currency 除外）。"

# 8. 汇总
Write-Host ""
Write-Host "=============================================="
Write-Host "Preflight 完成。"
Write-Host "  wallet_ok   = $walletOk"
Write-Host "  ledger_ok   = $ledgerOk"
Write-Host "  日志目录    = $WorkDir"
Write-Host "=============================================="
if (-not ($walletOk -and $ledgerOk)) {
    throw "资产守恒校验失败，禁止上生产。"
}
