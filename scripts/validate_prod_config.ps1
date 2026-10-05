<#
.SYNOPSIS
  校验 config.yml.production 是否满足生产安全基线。

.DESCRIPTION
  检查项：
    1. 无禁止值：localhost、example、changeme、debug 模式、空 password
    2. 三个核心 secret 占位符存在：${APP_SECRET} / ${JWT_SECRET} / ${USER_JWT_SECRET}
    3. database.driver = postgres
    4. server.mode = release
    5. cors.allowed_origins 不含 "*"
    6. redis.password 非空

  退出码：0 = PASS；1 = FAIL。

.PARAMETER ConfigPath
  待校验的配置文件路径，默认脚本上一级目录的 config.yml.production。
#>
[CmdletBinding()]
param(
  [string]$ConfigPath = (Join-Path $PSScriptRoot "..\config.yml.production")
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $ConfigPath)) {
  Write-Host "[FAIL] 配置文件不存在: $ConfigPath" -ForegroundColor Red
  exit 1
}

$lines = Get-Content -Path $ConfigPath -Encoding UTF8
$raw   = $lines -join "`n"

$problems = New-Object System.Collections.Generic.List[string]

function Add-Problem([string]$msg) { $script:problems.Add($msg) }

# ---------------------------------------------------------------------------
# 1. 禁止值扫描（整文件，含注释）
# ---------------------------------------------------------------------------
if ($raw -match "(?i)localhost") { Add-Problem("禁止值出现：localhost（应使用 127.0.0.1 / ::1）") }
if ($raw -match "(?i)example")     { Add-Problem("禁止值出现：example（示例域名不得出现在文件中）") }
if ($raw -match "(?i)changeme")   { Add-Problem("禁止值出现：changeme（弱默认口令）") }

# 1b. server.mode 必须为 release
$modeLine = $lines | Where-Object { $_ -match "^\s*mode\s*:\s*(\S+)" } | Select-Object -First 1
if (-not $modeLine) {
  Add-Problem("缺少 server.mode 配置")
} else {
  $modeVal = ($modeLine -replace "^\s*mode\s*:\s*", "")
  $modeVal = ($modeVal -split "#", 2)[0].Trim().Trim('"').Trim("'")
  if ($modeVal -ne "release") { Add-Problem("server.mode = '$modeVal'，生产必须为 release") }
}

# 1c. 空 password 检查：所有 password: 字段值不得为空
foreach ($line in $lines) {
  if ($line -match "^\s*([A-Za-z0-9_]*password)\s*:\s*(.*)$") {
    $key = $Matches[1]
    $val = $Matches[2].Trim()
    # 去掉行内注释
    $val = ($val -split "#", 2)[0].Trim()
    if ($val -eq "" -or $val -eq '""' -or $val -eq "''") {
      Add-Problem("空密码字段：$key（必须设置占位符或真实值）")
    }
  }
}

# ---------------------------------------------------------------------------
# 2. 三个核心 secret 占位符
# ---------------------------------------------------------------------------
foreach ($ph in @('${APP_SECRET}', '${JWT_SECRET}', '${USER_JWT_SECRET}')) {
  if ($raw -notlike "*$ph*") { Add-Problem("缺少 secret 占位符：$ph") }
}

# ---------------------------------------------------------------------------
# 3. database.driver = postgres
# ---------------------------------------------------------------------------
$driverLine = $lines | Where-Object { $_ -match "^\s*driver\s*:\s*(\S+)" } | Select-Object -First 1
if (-not $driverLine) {
  Add-Problem("缺少 database.driver 配置")
} else {
  $driverVal = ($driverLine -replace "^\s*driver\s*:\s*", "")
  $driverVal = ($driverVal -split "#", 2)[0].Trim().Trim('"').Trim("'")
  if ($driverVal -ne "postgres") { Add-Problem("database.driver = '$driverVal'，生产必须为 postgres") }
}

# ---------------------------------------------------------------------------
# 4. cors.allowed_origins 不含 "*"
# ---------------------------------------------------------------------------
$inCors = $false
$inAllowedOrigins = $false
foreach ($line in $lines) {
  if ($line -match "^cors\s*:\s*$") { $inCors = $true; continue }
  if ($inCors -and $line -match "^[A-Za-z]") { $inCors = $false; $inAllowedOrigins = $false }
  if ($inCors -and $line -match "^\s*allowed_origins\s*:") {
    $inAllowedOrigins = $true
    # 行内直接写了 "*"
    if ($line -match '"\*"|:\s*\*\s*$') { Add-Problem("cors.allowed_origins 包含通配符 *") }
    continue
  }
  if ($inCors -and $inAllowedOrigins) {
    # 列表项以 - 开头；遇到非列表、非空行则结束该列表
    if ($line -match "^\s*-\s*(.+?)\s*$") {
      $item = $Matches[1].Trim().Trim('"').Trim("'")
      if ($item -eq "*") { Add-Problem("cors.allowed_origins 包含通配符 *") }
    } elseif ($line -match "^\s*#") {
      continue
    } elseif ($line.Trim() -eq "") {
      continue
    } else {
      $inAllowedOrigins = $false
    }
  }
}

# ---------------------------------------------------------------------------
# 5. redis.password 非空（再次显式校验 redis 段）
# ---------------------------------------------------------------------------
$inRedis = $false
foreach ($line in $lines) {
  if ($line -match "^redis\s*:\s*$") { $inRedis = $true; continue }
  if ($inRedis -and $line -match "^[A-Za-z]") { $inRedis = $false }
  if ($inRedis -and $line -match "^\s*password\s*:\s*(.*)$") {
    $val = ($Matches[1] -split "#", 2)[0].Trim()
    if ($val -eq "" -or $val -eq '""' -or $val -eq "''") { Add-Problem("redis.password 为空") }
  }
}

# ---------------------------------------------------------------------------
# 输出结果
# ---------------------------------------------------------------------------
Write-Host "== 生产配置校验: $ConfigPath ==" -ForegroundColor Cyan
if ($problems.Count -eq 0) {
  Write-Host "PASS - 未发现生产配置基线问题。" -ForegroundColor Green
  exit 0
} else {
  Write-Host "FAIL - 发现 $($problems.Count) 个问题：" -ForegroundColor Red
  foreach ($p in $problems) { Write-Host "  - $p" -ForegroundColor Yellow }
  exit 1
}
