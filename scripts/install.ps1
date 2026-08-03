# PaperValet 一键安装脚本（Windows PowerShell）。
#
# 用法：
#   iwr -useb https://raw.githubusercontent.com/PaperValet/PaperValet/main/scripts/install.ps1 | iex
#   iwr -useb .../install.ps1 | iex -ArgumentList '--version','v0.2.0'
#   iwr -useb .../install.ps1 | iex -ArgumentList '--home','C:\papervalet'
#
# 选项：
#   --version <tag>         指定 release tag（默认 latest）
#   --home <dir>            安装目录（默认 $env:USERPROFILE\.papervalet）
#   --phone <+E164>         写入 config 并提示设置 PAPERVALET_PHONE
#   --api-id <int>          写入 config.telegram.api_id
#   --api-hash <str>        写入 config.telegram.api_hash
#   --no-config             不生成 config.json
#   --help                  显示帮助

[CmdletBinding()]
param(
    [string]$Version = 'latest',
    [string]$Home,
    [string]$Phone,
    [string]$ApiId,
    [string]$ApiHash,
    [string]$Repo = 'PaperValet/PaperValet',
    [switch]$NoConfig,
    [switch]$Help
)

$ErrorActionPreference = 'Stop'

if ($Help) {
    Write-Host @"
用法：install.ps1 [--version <tag>] [--home <dir>] [--phone <e164>] [--api-id <int>] [--api-hash <str>] [--no-config] [--help]
"@
    exit 0
}

# ===== 架构探测 =====
switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $Arch = 'amd64' }
    'ARM64' {
        $Arch = 'arm64'
        Write-Warning 'arm64 检测到：bundle 内 .so 为 amd64（见 README 备注）'
    }
    default { throw "不支持的架构: $env:PROCESSOR_ARCHITECTURE" }
}
$Os = 'windows'

# ===== 安装目录 =====
if (-not $Home) {
    if ($env:PAPERVALET_HOME) {
        $Home = $env:PAPERVALET_HOME
    } else {
        $Home = Join-Path $env:USERPROFILE '.papervalet'
    }
}
Write-Host "📦 安装目录: $Home"

# ===== 解析 release 元数据 =====
$apiUrl = if ($Version -eq 'latest') {
    "https://api.github.com/repos/$Repo/releases/latest"
} else {
    "https://api.github.com/repos/$Repo/releases/tags/$Version"
}

Write-Host "📡 获取 release 元数据: $apiUrl"
try {
    $headers = @{ 'User-Agent' = 'papervalet-install' }
    $release = Invoke-RestMethod -Uri $apiUrl -Headers $headers -TimeoutSec 30
} catch {
    throw "无法获取 release 信息: $_"
}

$bundleName = "papervalet-$Os-$Arch.zip"
$asset = $release.assets | Where-Object { $_.name -eq $bundleName } | Select-Object -First 1
if (-not $asset) {
    Write-Host "❌ 在 release 中找不到 $bundleName" -ForegroundColor Red
    Write-Host "   可用的资产：" -ForegroundColor Red
    $release.assets | ForEach-Object { Write-Host "     $($_.name)" -ForegroundColor Red }
    exit 1
}

# ===== 下载 =====
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmp | Out-Null
$archive = Join-Path $tmp $bundleName

Write-Host "⬇️  下载中: $($asset.browser_download_url)"
try {
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $archive -UseBasicParsing
} catch {
    throw "下载失败: $_"
}

# ===== 安装目录确认 =====
if (Test-Path $Home) {
    $ans = Read-Host "⚠️  目标目录已存在 $Home，是否覆盖? [y/N]"
    if ($ans -notin @('y','Y','yes','YES')) {
        Write-Host '已取消'
        exit 0
    }
    Remove-Item -Recurse -Force $Home
}

# ===== 解压 =====
Write-Host "📦 解压到 $Home ..."
Expand-Archive -Path $archive -DestinationPath $Home -Force

# ===== config.json =====
if (-not $NoConfig) {
    $cfg = Join-Path $Home 'config.json'
    $example = Join-Path $Home 'config.example.json'
    if ((-not (Test-Path $cfg)) -and (Test-Path $example)) {
        Copy-Item $example $cfg
        Write-Host "✏️  已写入默认 config.json: $cfg"
    }

    if ($ApiId -or $ApiHash) {
        # PowerShell 3+ 自带 ConvertFrom-Json，无需 jq
        try {
            $json = Get-Content $cfg -Raw | ConvertFrom-Json
            if ($ApiId)   { $json.telegram.api_id   = [int]$ApiId }
            if ($ApiHash) { $json.telegram.api_hash = $ApiHash }
            $json | ConvertTo-Json -Depth 10 | Set-Content -Path $cfg -Encoding UTF8
        } catch {
            Write-Warning "写入 api_id/api_hash 失败，请手动编辑 $cfg"
        }
    }
}

# ===== 收尾 =====
$pluginCount = (Get-ChildItem -Path (Join-Path $Home 'plugins') -Filter '*.so' -ErrorAction SilentlyContinue | Measure-Object).Count

Write-Host ""
Write-Host "✅ PaperValet 已安装到: $Home" -ForegroundColor Green
Write-Host "   ├── bin\papervalet.exe"
Write-Host "   ├── plugins\        (含 $pluginCount 个 .so)"
Write-Host "   ├── config.json     $(if (Test-Path (Join-Path $Home 'config.json')) { '✓' } else { '✗' })"
Write-Host "   └── run.cmd"

Write-Host ""
Write-Host "下一步："
if (-not (Test-Path (Join-Path $Home 'config.json'))) {
    Write-Host "   1) Copy-Item '$Home\config.example.json' '$Home\config.json'"
    Write-Host "   2) 编辑 config.json 填入 api_id / api_hash（https://my.telegram.org 申请）"
    Write-Host "   3) & '$Home\run.cmd'"
} elseif (-not $ApiId -or -not $ApiHash) {
    Write-Host "   1) 编辑 $Home\config.json 填入 api_id / api_hash"
    Write-Host "   2) & '$Home\run.cmd'"
} else {
    Write-Host "   • 直接启动: & '$Home\run.cmd'"
}

if ($Phone) {
    Write-Host ""
    Write-Host "   • 你传入了 --phone=$Phone，请设置环境变量后再启动：" -ForegroundColor Yellow
    Write-Host "       `$env:PAPERVALET_PHONE = '$Phone'" -ForegroundColor Yellow
    Write-Host "       & '$Home\run.cmd'" -ForegroundColor Yellow
}

Remove-Item -Recurse -Force $tmp