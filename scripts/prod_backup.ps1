<#
.SYNOPSIS
    HCZ 生产环境备份脚本（PostgreSQL + 配置 + 上传文件 + 旧版本 binary）。

.DESCRIPTION
    本脚本在生产服务器上执行一次完整备份：
      1. pg_dump 全量备份 PostgreSQL（custom 格式 -F c，支持 pg_restore 选择性恢复）
      2. 复制 application config.yml 到备份目录
      3. 复制 uploads/ 目录（用户上传文件）
      4. 复制当前 hcz.exe 与 frontend dist（用于回滚到旧版本）
      5. 为每个产物计算 SHA256 校验和
      6. 输出备份清单 CSV（path, timestamp, size_bytes, sha256）

    使用前请先设置下方 $BackupRoot、$PGHost 等变量，或通过环境变量传入。

.NOTES
    - 需要本机已安装 pg_dump（与目标 PostgreSQL 大版本匹配），并在 PATH 中。
    - PostgreSQL 密码通过环境变量 PGPASSWORD 传入，不写在脚本里。
    - 建议在 systemd 服务停服 / 或低峰期执行；pg_dump -F c 在线一致性备份不阻塞读写。
#>

[CmdletBinding()]
param(
    # 备份根目录（每次执行会在其下创建 yyyyMMdd_HHmmss 子目录）
    [string]$BackupRoot = "E:\backup\hcz",

    # PostgreSQL 连接参数
    [string]$PGHost = "127.0.0.1",
    [int]$PGPort = 5432,
    [string]$PGUser = "hcz",
    [string]$PGDatabase = "hcz",
    # 密码建议通过 $env:PGPASSWORD 传入，不要硬编码
    [string]$PGPassword = "",

    # 应用部署目录（含 hcz.exe / config.yml / uploads/ / frontend/dist）
    [string]$AppDir = "E:\app\hcz",

    # frontend 静态资源目录（fullstack 二进制内嵌时可留空跳过）
    [string]$FrontendDist = "E:\app\hcz\frontend\dist"
)

$ErrorActionPreference = 'Stop'
$stamp = Get-Date -Format 'yyyyMMdd_HHmmss'
$dest = Join-Path $BackupRoot $stamp
New-Item -ItemType Directory -Path $dest -Force | Out-Null

# PGPASSWORD：优先用参数传入，其次用环境变量
if ($PGPassword -ne "") {
    $env:PGPASSWORD = $PGPassword
}

$manifest = @()
function Add-Manifest {
    param([string]$Path, [string]$Category)
    if (-not (Test-Path $Path)) {
        Write-Warning "跳过（不存在）: $Path"
        return
    }
    $item = Get-Item $Path
    $hash = (Get-FileHash -Path $Path -Algorithm SHA256).Hash
    $script:manifest += [PSCustomObject]@{
        category   = $Category
        path       = $Path
        timestamp  = (Get-Date).ToString('o')
        size_bytes = $item.Length
        sha256     = $hash
    }
    Write-Host ("[OK] {0}: {1} ({2:N0} bytes)" -f $Category, $Path, $item.Length)
}

# 1. PostgreSQL 全量备份（custom 格式）
$dbDump = Join-Path $dest "hcz_${stamp}.dump"
Write-Host "==> pg_dump -> $dbDump"
& pg_dump -h $PGHost -p $PGPort -U $PGUser -d $PGDatabase -F c -f $dbDump
if ($LASTEXITCODE -ne 0) { throw "pg_dump 失败，exit=$LASTEXITCODE" }
Add-Manifest -Path $dbDump -Category 'postgres_dump'

# 2. application config.yml
$srcConfig = Join-Path $AppDir 'config.yml'
if (Test-Path $srcConfig) {
    $dstConfig = Join-Path $dest 'config.yml'
    Copy-Item $srcConfig $dstConfig -Force
    Add-Manifest -Path $dstConfig -Category 'config'
} else {
    Write-Warning "config.yml 不存在: $srcConfig"
}

# 3. uploads/ 目录（整个目录压缩为 zip，避免逐文件记录过多条目）
$srcUploads = Join-Path $AppDir 'uploads'
if (Test-Path $srcUploads) {
    $zipUploads = Join-Path $dest "uploads_${stamp}.zip"
    Write-Host "==> Compress uploads -> $zipUploads"
    Compress-Archive -Path $srcUploads -DestinationPath $zipUploads -Force
    Add-Manifest -Path $zipUploads -Category 'uploads'
} else {
    Write-Warning "uploads/ 不存在: $srcUploads"
}

# 4. 当前 binary（hcz.exe）用于回滚
$srcExe = Join-Path $AppDir 'hcz.exe'
if (Test-Path $srcExe) {
    $dstExe = Join-Path $dest "hcz_${stamp}.exe"
    Copy-Item $srcExe $dstExe -Force
    Add-Manifest -Path $dstExe -Category 'binary'
} else {
    Write-Warning "hcz.exe 不存在: $srcExe"
}

# 5. frontend dist（如果独立于二进制）
if ((Test-Path $FrontendDist) -and ($FrontendDist -ne $AppDir)) {
    $zipDist = Join-Path $dest "frontend_dist_${stamp}.zip"
    Write-Host "==> Compress frontend dist -> $zipDist"
    Compress-Archive -Path $FrontendDist -DestinationPath $zipDist -Force
    Add-Manifest -Path $zipDist -Category 'frontend_dist'
}

# 输出清单 CSV
$csvPath = Join-Path $dest "manifest_${stamp}.csv"
$manifest | Export-Csv -Path $csvPath -NoTypeInformation -Encoding UTF8

Write-Host ""
Write-Host "=============================================="
Write-Host "备份完成: $dest"
Write-Host "清单 CSV: $csvPath"
Write-Host "产物数: $($manifest.Count)"
Write-Host "=============================================="
$manifest | Format-Table category, size_bytes, sha256 -AutoSize
