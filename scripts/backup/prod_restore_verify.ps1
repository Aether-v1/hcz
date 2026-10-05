<#
.SYNOPSIS
    HCZ 备份恢复验证脚本（在临时库上 pg_restore 并跑只读校验查询）。

.DESCRIPTION
    本脚本不触碰生产库：
      1. createdb 创建临时库 hcz_restore_verify
      2. pg_restore 把指定 .dump 恢复到临时库
      3. 跑只读校验查询（表清单 / users / wallet_accounts / orders / wallet_transactions）
      4. 打印结果，人工判断是否符合预期
      5. DROP DATABASE 清理临时库

    用法：
      .\prod_restore_verify.ps1 -DumpFile E:\backup\hcz\20261005_210000\hcz_20261005_210000.dump
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$DumpFile,

    [string]$PGHost = "127.0.0.1",
    [int]$PGPort = 5432,
    [string]$PGUser = "hcz",
    [string]$PGPassword = "",
    [string]$TempDB = "hcz_restore_verify"
)

$ErrorActionPreference = 'Stop'
if ($PGPassword -ne "") { $env:PGPASSWORD = $PGPassword }

if (-not (Test-Path $DumpFile)) { throw "dump 文件不存在: $DumpFile" }

# 用单引号 here-string 定义 SQL，避免 PowerShell 变量替换和引号嵌套问题
$sqlDropDb = 'DROP DATABASE IF EXISTS ' + $TempDB + ';'
$sqlTableCount = "SELECT count(*) AS table_count FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE';"
$sqlUsers = "SELECT count(*) AS users_count FROM users WHERE deleted_at IS NULL;"
$sqlWallet = "SELECT count(*) AS acct_count, COALESCE(SUM(available_balance),0) AS sum_available, COALESCE(SUM(frozen_balance),0) AS sum_frozen, COALESCE(SUM(available_balance+frozen_balance),0) AS sum_total FROM wallet_accounts WHERE deleted_at IS NULL;"
$sqlOrders = "SELECT count(*) AS orders_count FROM orders WHERE deleted_at IS NULL;"
$sqlLedger = "SELECT count(*) AS tx_count, COALESCE(SUM(amount) FILTER (WHERE direction='in'),0) AS sum_in, COALESCE(SUM(amount) FILTER (WHERE direction='out'),0) AS sum_out FROM wallet_transactions WHERE deleted_at IS NULL;"

# 0. 如果临时库已存在，先删掉
Write-Host "==> 清理可能残留的临时库 $TempDB"
& psql -h $PGHost -p $PGPort -U $PGUser -d postgres -c $sqlDropDb
if ($LASTEXITCODE -ne 0) { throw "DROP DATABASE 残留失败" }

# 1. 创建临时库
Write-Host "==> createdb $TempDB"
& createdb -h $PGHost -p $PGPort -U $PGUser $TempDB
if ($LASTEXITCODE -ne 0) { throw "createdb 失败" }

try {
    # 2. pg_restore
    Write-Host "==> pg_restore $DumpFile -> $TempDB"
    & pg_restore -h $PGHost -p $PGPort -U $PGUser -d $TempDB --no-owner --no-privileges $DumpFile
    Write-Host ("    pg_restore exit={0}（warning 可忽略）" -f $LASTEXITCODE)

    # 3. 校验查询
    Write-Host ""
    Write-Host "==> 校验查询结果"

    Write-Host ""
    Write-Host "--- 表数量（public schema）---"
    & psql -h $PGHost -p $PGPort -U $PGUser -d $TempDB -c $sqlTableCount

    Write-Host ""
    Write-Host "--- users 行数 ---"
    & psql -h $PGHost -p $PGPort -U $PGUser -d $TempDB -c $sqlUsers

    Write-Host ""
    Write-Host "--- wallet_accounts：行数 / available / frozen ---"
    & psql -h $PGHost -p $PGPort -U $PGUser -d $TempDB -c $sqlWallet

    Write-Host ""
    Write-Host "--- orders 行数 ---"
    & psql -h $PGHost -p $PGPort -U $PGUser -d $TempDB -c $sqlOrders

    Write-Host ""
    Write-Host "--- wallet_transactions 行数 + in/out 汇总 ---"
    & psql -h $PGHost -p $PGPort -U $PGUser -d $TempDB -c $sqlLedger

    Write-Host ""
    Write-Host "=============================================="
    Write-Host "恢复验证完成。请人工核对上述数字与备份前预期一致。"
    Write-Host "=============================================="
}
finally {
    # 4. 清理临时库
    Write-Host ""
    Write-Host "==> 清理临时库 $TempDB"
    & psql -h $PGHost -p $PGPort -U $PGUser -d postgres -c $sqlDropDb
    if ($LASTEXITCODE -ne 0) { Write-Warning ("DROP DATABASE 清理失败，请手动处理 " + $TempDB) }
}
