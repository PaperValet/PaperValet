# PaperValet 一键安装脚本（Windows PowerShell）—— 交互菜单 + 命令行双形态。
#
# ===== 菜单形态（默认） =====
#   iwr -useb https://.../install.ps1 | iex
#   iwr -useb https://.../install.ps1 | iex -ArgumentList '--menu'
#
# 菜单项：
#   1) install      首次安装到 $env:USERPROFILE\.papervalet
#   2) reinstall    覆盖重装（保留 config.json / session.json / sessions.db）
#   3) upgrade      升级到指定 version（保留全部数据）
#   4) uninstall    删除安装（可保留 config 与 data）
#   5) status       查看当前安装信息
#   6) latest       列出 GitHub 最新 release 资产
#   7) set-phone    写入 PAPERVALET_PHONE 到环境变量 + 提示 2FA / CODE
#   8) run          前台启动 bot
#   9) doctor       自检
#   0) exit
#
# ===== 命令行形态 =====
#   --non-interactive        跳过菜单直接 install
#   --action <name>          指定动作（install / reinstall / upgrade / uninstall /
#                            status / latest / set-phone / run / doctor）
#   --version <tag>          指定 release tag（默认 latest）
#   --home <dir>             安装目录（默认 $env:USERPROFILE\.papervalet）
#   --phone <+E164>          设置 PAPERVALET_PHONE
#   --api-id <int>           写入 config.telegram.api_id
#   --api-hash <str>         写入 config.telegram.api_hash
#   --no-config              不生成 config.json
#   --keep-config            重装 / 升级时保留 config.json
#   --keep-data              重装 / 升级时保留 session.json / sessions.db
#   --repo <owner/repo>      指定仓库
#   --help                   显示帮助
#
# 环境变量：
#   PAPERVALET_REPO / PAPERVALET_HOME / PAPERVALET_VERSION / PAPERVALET_MENU=0

[CmdletBinding()]
param(
    [switch]$Menu,
    [switch]$NoMenu,
    [switch]$NonInteractive,
    [ValidateSet('install','reinstall','upgrade','uninstall','status','latest','set-phone','run','doctor')]
    [string]$Action = 'install',
    [string]$Version,
    [string]$Home,
    [string]$Phone,
    [string]$ApiId,
    [string]$ApiHash,
    [switch]$NoConfig,
    [switch]$KeepConfig,
    [switch]$KeepData,
    [string]$Repo,
    [switch]$Help
)

$ErrorActionPreference = 'Stop'

if ($Help) {
    Get-Content $PSCommandPath | Select-Object -First 40 | ForEach-Object { Write-Host ($_ -replace '^# ?','') }
    exit 0
}

# ===== 默认值 =====
if (-not $Repo)   { $Repo   = if ($env:PAPERVALET_REPO)   { $env:PAPERVALET_REPO }   else { 'PaperValet/PaperValet' } }
if (-not $Version) { $Version = if ($env:PAPERVALET_VERSION) { $env:PAPERVALET_VERSION } else { 'latest' } }
if (-not $Home)    {
    if ($env:PAPERVALET_HOME) { $Home = $env:PAPERVALET_HOME }
    else { $Home = Join-Path $env:USERPROFILE '.papervalet' }
}

# ===== 菜单判定 =====
$useMenu = 'auto'
if ($Menu)      { $useMenu = 'yes' }
if ($NoMenu)    { $useMenu = 'no' }
if ($NonInteractive) { $useMenu = 'no' }
if ($env:PAPERVALET_MENU -eq '0') { $useMenu = 'no' }
if ($useMenu -eq 'auto') {
    if ([Environment]::UserInteractive -and -not $Phone -and -not $ApiId -and -not $ApiHash) {
        $useMenu = 'yes'
    } else {
        $useMenu = 'no'
    }
}

# ===== 工具函数 =====
function Write-Info($msg)  { Write-Host "ℹ  $msg" -ForegroundColor Cyan }
function Write-Ok($msg)    { Write-Host "✓  $msg" -ForegroundColor Green }
function Write-Warn($msg)  { Write-Host "⚠  $msg" -ForegroundColor Yellow }
function Write-Fail($msg)  { Write-Host "❌ $msg" -ForegroundColor Red }

function Get-OsArch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { return @{ Os = 'windows'; Arch = 'amd64'; SoArch = 'amd64' } }
        'ARM64' {
            Write-Warn 'arm64：bundle 内 .so 为 amd64（见 README）'
            return @{ Os = 'windows'; Arch = 'arm64'; SoArch = 'amd64' }
        }
        default { throw "不支持的架构: $env:PROCESSOR_ARCHITECTURE" }
    }
}

function Get-ReleaseJson {
    param([string]$Version)
    $url = if ($Version -eq 'latest') {
        "https://api.github.com/repos/$Repo/releases/latest"
    } else {
        "https://api.github.com/repos/$Repo/releases/tags/$Version"
    }
    $headers = @{ 'User-Agent' = 'papervalet-install' }
    try { Invoke-RestMethod -Uri $url -Headers $headers -TimeoutSec 30 }
    catch { throw "无法获取 release 信息: $_" }
}

function Get-BundleUrl {
    param($Json, [string]$BundleName)
    $asset = $Json.assets | Where-Object { $_.name -eq $BundleName } | Select-Object -First 1
    if (-not $asset) { return $null }
    return $asset.browser_download_url
}

function Format-AssetList($Json) {
    $Json.assets | ForEach-Object { "  $($_.name)" }
}

function Save-ConfigFromInputs {
    param([string]$Cfg, [string]$Example, [string]$Phone, [string]$ApiId, [string]$ApiHash)
    if (-not $NoConfig) {
        if ((-not (Test-Path $Cfg)) -and (Test-Path $Example)) {
            Copy-Item $Example $Cfg
            Write-Ok "已写入默认 config.json: $Cfg"
        }
    }
    if ($ApiId -or $ApiHash) {
        try {
            $json = Get-Content $Cfg -Raw | ConvertFrom-Json
            if ($ApiId)   { $json.telegram.api_id   = [int]$ApiId }
            if ($ApiHash) { $json.telegram.api_hash = $ApiHash }
            $json | ConvertTo-Json -Depth 10 | Set-Content -Path $Cfg -Encoding UTF8
        } catch { Write-Warn "写入 api_id/api_hash 失败，请手动编辑 $Cfg" }
    }
}

function Set-PhoneEnvPersist {
    param([string]$Phone, [string]$HomeDir)
    if (-not $Phone) { return }
    $envFile = Join-Path $env:USERPROFILE '.papervalet.env.ps1'
    $lines = @(
        '# PaperValet login env (auto-generated by install.ps1)'
        "`$env:PAPERVALET_PHONE = '$Phone'"
        "`$env:PAPERVALET_HOME = '$HomeDir'"
    )
    if ($env:PAPERVALET_CODE)         { $lines += "`$env:PAPERVALET_CODE = '$env:PAPERVALET_CODE'" }
    if ($env:PAPERVALET_2FA_PASSWORD) { $lines += "`$env:PAPERVALET_2FA_PASSWORD = '$env:PAPERVALET_2FA_PASSWORD'" }
    Set-Content -Path $envFile -Value $lines -Encoding UTF8
    Write-Ok "环境变量持久化到 $envFile"
    Write-Host "   立即生效: . $envFile"
}

# ===== 动作：install / reinstall / upgrade =====
function Invoke-Install {
    $info = Get-OsArch
    $os = $info.Os; $arch = $info.Arch

    $json = Get-ReleaseJson -Version $Version

    if (Test-Path $Home) {
        Write-Warn "$Home 已存在"
        if ($Action -eq 'install') {
            if ($NonInteractive) { throw '已存在且 --non-interactive；改用 --action upgrade / reinstall' }
            $ans = Read-Host '   是否覆盖? [y/N]'
            if ($ans -notin @('y','Y','yes','YES')) { Write-Host '已取消'; return }
            Remove-Item -Recurse -Force $Home
        } else {
            # reinstall / upgrade：按需保留
            $tmpKeep = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
            New-Item -ItemType Directory -Path $tmpKeep | Out-Null
            if ($KeepConfig -or $Action -eq 'reinstall' -or $Action -eq 'upgrade') {
                if (Test-Path (Join-Path $Home 'config.json')) {
                    Copy-Item (Join-Path $Home 'config.json') (Join-Path $tmpKeep 'config.json')
                }
            }
            if ($KeepData -or $Action -eq 'upgrade') {
                $dataDir = Join-Path $tmpKeep 'data'; New-Item -ItemType Directory -Path $dataDir | Out-Null
                foreach ($f in 'session.json','sessions.db','sessions.db-wal','sessions.db-shm') {
                    $p = Join-Path $Home $f
                    if (Test-Path $p) { Copy-Item $p (Join-Path $dataDir $f) }
                }
            }
            Remove-Item -Recurse -Force $Home
            $script:lastKeep = $tmpKeep
        }
    }

    $ext = if ($os -eq 'windows') { 'zip' } else { 'tar.gz' }
    $bundleName = "papervalet-$os-$arch.$ext"
    $url = Get-BundleUrl -Json $json -BundleName $bundleName
    if (-not $url) {
        Write-Fail "release 中找不到 $bundleName"
        Write-Host '可用资产:' -ForegroundColor Red
        Format-AssetList $json | ForEach-Object { Write-Host $_ -ForegroundColor Red }
        return
    }

    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    $archive = Join-Path $tmp $bundleName

    try {
        Write-Info "下载: $url"
        Invoke-WebRequest -Uri $url -OutFile $archive -UseBasicParsing
        New-Item -ItemType Directory -Path $Home -Force | Out-Null
        Write-Info "解压到 $Home"
        Expand-Archive -Path $archive -DestinationPath $Home -Force

        # 还原 keep
        if ($script:lastKeep) {
            if (Test-Path (Join-Path $script:lastKeep 'config.json')) {
                Copy-Item (Join-Path $script:lastKeep 'config.json') (Join-Path $Home 'config.json') -Force
                Write-Ok '已保留 config.json'
            }
            $dataDir = Join-Path $script:lastKeep 'data'
            if (Test-Path $dataDir) {
                Copy-Item -Recurse -Force (Join-Path $dataDir '*') $Home
                Write-Ok '已保留 session / database'
            }
        }

        Save-ConfigFromInputs -Cfg (Join-Path $Home 'config.json') `
                              -Example (Join-Path $Home 'config.example.json') `
                              -Phone $Phone -ApiId $ApiId -ApiHash $ApiHash

        Set-PhoneEnvPersist -Phone $Phone -HomeDir $Home

        Show-FinishBanner -Phase $Action
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
        Remove-Item -Recurse -Force $script:lastKeep -ErrorAction SilentlyContinue
        $script:lastKeep = $null
    }
}

function Invoke-Uninstall {
    if (-not (Test-Path $Home)) {
        Write-Warn "$Home 不存在"
        return
    }
    Write-Host "即将卸载 $Home"
    $keepCfg = 'N'; $keepData = 'N'; $confirm = ''
    if (-not $NonInteractive) {
        $keepCfg  = Read-Host '   保留 config.json? [y/N]'
        $keepData = Read-Host '   保留 session.json / sessions.db? [y/N]'
        $confirm  = Read-Host '   确认删除? 输入 YES 继续: '
        if ($confirm -ne 'YES') { Write-Host '已取消'; return }
    }
    $bak = $null
    if ($keepCfg -match '^[Yy]$' -and (Test-Path (Join-Path $Home 'config.json'))) {
        $bak = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
        New-Item -ItemType Directory -Path $bak | Out-Null
        Copy-Item (Join-Path $Home 'config.json') (Join-Path $bak 'config.json')
    }
    if ($keepData -match '^[Yy]$') {
        if (-not $bak) {
            $bak = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
            New-Item -ItemType Directory -Path $bak | Out-Null
        }
        $dataDir = Join-Path $bak 'data'; New-Item -ItemType Directory -Path $dataDir | Out-Null
        foreach ($f in 'session.json','sessions.db','sessions.db-wal','sessions.db-shm') {
            $p = Join-Path $Home $f
            if (Test-Path $p) { Copy-Item $p (Join-Path $dataDir $f) }
        }
    }
    Remove-Item -Recurse -Force $Home
    Write-Ok '已卸载'
    if ($bak) {
        Write-Host "   备份: $bak"
        Write-Host "   恢复: Copy-Item -Recurse -Force $bak\* $Home"
    }
}

function Show-Status {
    Write-Host 'PaperValet 安装状态' -ForegroundColor Cyan
    Write-Host "  安装目录: $Home"
    if (Test-Path $Home) {
        $bin = Join-Path $Home 'bin\papervalet.exe'
        if (Test-Path $bin) {
            $ver = & $bin --version 2>&1 | Select-Object -First 1
            $hash = (Get-FileHash $bin -Algorithm SHA256).Hash.Substring(0,12)
            Write-Host "  二进制:   $bin  $ver  (sha256:$hash...)" -ForegroundColor Green
        } else { Write-Warn "  二进制缺失" }
        $cfg = Join-Path $Home 'config.json'
        if (Test-Path $cfg) {
            $size = (Get-Item $cfg).Length
            Write-Host "  config:   $cfg ($size bytes)"
        } else { Write-Warn '  config:   缺失' }
        $soCount = (Get-ChildItem -Path (Join-Path $Home 'plugins') -Filter '*.so' -ErrorAction SilentlyContinue | Measure-Object).Count
        Write-Host "  .so:      $soCount 个"
        foreach ($f in 'session.json','sessions.db') {
            $p = Join-Path $Home $f
            if (Test-Path $p) {
                $size = (Get-Item $p).Length
                Write-Host "  $f 存在 ($size bytes)"
            }
        }
        $envFile = Join-Path $env:USERPROFILE '.papervalet.env.ps1'
        if (Test-Path $envFile) { Write-Host "  环境变量持久化: $envFile" }
    } else { Write-Warn '未安装' }
}

function Show-Latest {
    $json = Get-ReleaseJson -Version latest
    Write-Host '最新 release' -ForegroundColor Cyan
    foreach ($k in 'tag_name','name','published_at','html_url') {
        $v = $json.$k
        if ($v) { Write-Host "  $k`: $v" }
    }
    Write-Host ''
    Write-Host '可用 bundle:'
    $json.assets | Where-Object { $_.name -like 'papervalet-*' } | ForEach-Object { Write-Host "  $($_.name)" }
}

function Set-PhoneInteractive {
    if (-not $Phone) {
        $script:Phone = Read-Host '   输入手机号（E.164，如 +8613800138000）'
    }
    if (-not $Phone) { Write-Fail '未提供手机号'; return }
    Set-PhoneEnvPersist -Phone $Phone -HomeDir $Home
    Write-Ok 'PAPERVALET_PHONE 已设置'
    Write-Host ''
    Write-Host '   启动方式:'
    Write-Host "     . `$env:USERPROFILE\.papervalet.env.ps1"
    Write-Host "     & '$Home\run.cmd'"
}

function Invoke-Run {
    $bin = Join-Path $Home 'bin\papervalet.exe'
    if (-not (Test-Path $bin)) { Write-Fail "$bin 不可用；先 install"; return }
    Write-Info '前台启动（Ctrl+C 退出）'
    $envFile = Join-Path $env:USERPROFILE '.papervalet.env.ps1'
    if (Test-Path $envFile) { . $envFile }
    Push-Location $Home
    try { & $bin -config "$Home\config.json" } finally { Pop-Location }
}

function Invoke-Doctor {
    Write-Host 'PaperValet 自检' -ForegroundColor Cyan
    foreach ($cmd in 'curl','tar','jq') {
        if (Get-Command $cmd -ErrorAction SilentlyContinue) { Write-Ok "$cmd 可用" }
        else { Write-Warn "$cmd 不可用" }
    }
    $info = Get-OsArch
    Write-Ok "OS=$($info.Os) ARCH=$($info.Arch) SO_ARCH=$($info.SoArch)"
    try {
        Get-ReleaseJson -Version latest | Out-Null
        Write-Ok 'GitHub API 可达'
    } catch { Write-Warn 'GitHub API 不可达；离线环境无法 install / upgrade' }
    if (Test-Path $Home) {
        $bin = Join-Path $Home 'bin\papervalet.exe'
        if (Test-Path $bin) { Write-Ok "本地安装: $bin" }
        else { Write-Warn '本地未安装或二进制缺失' }
    } else { Write-Warn '本地未安装' }
}

function Show-FinishBanner {
    param([string]$Phase)
    $soCount = 0
    if (Test-Path (Join-Path $Home 'plugins')) {
        $soCount = (Get-ChildItem -Path (Join-Path $Home 'plugins') -Filter '*.so' -ErrorAction SilentlyContinue | Measure-Object).Count
    }
    Write-Host ''
    Write-Host "✅ PaperValet $Phase : $Home" -ForegroundColor Green
    Write-Host "   ├── bin\papervalet.exe"
    Write-Host "   ├── plugins\        ($soCount 个 .so)"
    Write-Host "   ├── config.json     $('✓')"
    Write-Host "   └── run.cmd"
}

# ===== 菜单 =====
function Show-Menu {
    $info = Get-OsArch
    Clear-Host
    Write-Host 'PaperValet 安装器' -ForegroundColor Cyan
    Write-Host "  安装目录: $Home"
    Write-Host "  版本:     $Version"
    Write-Host "  仓库:     $Repo"
    Write-Host "  系统:     $($info.Os)/$($info.Arch)"
    Write-Host ''
    Write-Host '请选择操作：'
    Write-Host ''
    Write-Host '  1) install      首次安装'
    Write-Host '  2) reinstall    覆盖重装（保留 config）'
    Write-Host '  3) upgrade      升级（保留 config / session / db）'
    Write-Host '  4) uninstall    卸载'
    Write-Host '  5) status       查看当前安装'
    Write-Host '  6) latest       查看 GitHub 最新 release'
    Write-Host '  7) set-phone    设置 PAPERVALET_PHONE'
    Write-Host '  8) run          前台启动'
    Write-Host '  9) doctor       自检'
    Write-Host '  0) exit         退出'
    Write-Host ''
    Write-Host '  v) 切换 version'
    Write-Host '  h) 切换 home 目录'
}

function Run-Menu {
    while ($true) {
        Show-Menu
        $choice = Read-Host '选择 [0-9vh]'
        switch ($choice) {
            '1' { $script:Action = 'install';    Invoke-Install }
            '2' { $script:Action = 'reinstall';  Invoke-Install }
            '3' { $script:Action = 'upgrade';    Invoke-Install }
            '4' { $script:Action = 'uninstall';  Invoke-Uninstall }
            '5' { $script:Action = 'status';     Show-Status }
            '6' { $script:Action = 'latest';     Show-Latest }
            '7' { $script:Action = 'set-phone';  Set-PhoneInteractive }
            '8' { $script:Action = 'run';        Invoke-Run }
            '9' { $script:Action = 'doctor';     Invoke-Doctor }
            '0' { Write-Host 'bye'; return }
            'v' { $script:Version = Read-Host '新 version（latest 或 vX.Y.Z）' }
            'h' { $script:Home    = Read-Host '新 home 目录' }
            default { Write-Warn "无效选择 '$choice'" }
        }
        Write-Host ''
        $cont = Read-Host '按 Enter 返回菜单，q 退出'
        if ($cont -eq 'q') { return }
    }
}

# ===== 入口 =====
if ($useMenu -eq 'yes') {
    Run-Menu
} else {
    switch ($Action) {
        { $_ -in 'install','reinstall','upgrade' } { Invoke-Install }
        'uninstall'  { Invoke-Uninstall }
        'status'     { Show-Status }
        'latest'     { Show-Latest }
        'set-phone'  { if (-not $Phone) { $Phone = Read-Host '手机号' }; Set-PhoneInteractive }
        'run'        { Invoke-Run }
        'doctor'     { Invoke-Doctor }
        default      { Write-Fail "未知 action: $Action" }
    }
}