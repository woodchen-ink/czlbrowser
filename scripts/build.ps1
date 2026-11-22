# 构建桌面端, 可选打包安装程序。本地与 CI 共用。
#
#   pwsh scripts/build.ps1                          # 只构建 desktop/build/bin/czl-browser.exe, 版本号 dev
#   pwsh scripts/build.ps1 -Installer               # 另外生成安装包 (需要 NSIS)
#   pwsh scripts/build.ps1 -Installer -Version 2026.11.22.1 -WithKernel
#
# 安装包输出到 dist/CZLBrowser-Setup-<Version>.exe。
# -WithKernel 会把 desktop/chrome 一起打进安装包 (CI 没有内核, 不加这个开关)。
param(
  [string]$Version = 'dev',
  [switch]$Installer,
  [switch]$WithKernel
)
$ErrorActionPreference = 'Stop'
$repo = Resolve-Path (Join-Path $PSScriptRoot '..')
$desktop = Join-Path $repo 'desktop'

Push-Location $desktop
try {
  wails build -platform windows/amd64 -trimpath -ldflags "-X main.AppVersion=$Version"
  if ($LASTEXITCODE -ne 0) { throw "wails build 失败 ($LASTEXITCODE)" }
} finally {
  Pop-Location
}
$exe = Join-Path $desktop 'build/bin/czl-browser.exe'
if (-not (Test-Path $exe)) { throw "缺少构建产物: $exe" }
Write-Host "程序: $exe"
if (-not $Installer) { return }

# ---- 组装安装包内容 ----
$stage = Join-Path $repo 'dist/stage'
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Force -Path (Join-Path $stage 'bin') | Out-Null
Copy-Item $exe (Join-Path $stage 'czl-browser.exe')
Copy-Item (Join-Path $PSScriptRoot 'config.init.yaml') (Join-Path $stage 'config.yaml')
foreach ($tool in 'xray.exe', 'sing-box.exe') {
  $src = Join-Path $desktop "bin/$tool"
  if (-not (Test-Path $src)) { throw "缺少代理运行时: $src" }
  Copy-Item $src (Join-Path $stage "bin/$tool")
}
if ($WithKernel) {
  $kernels = Join-Path $desktop 'chrome'
  if (-not (Test-Path $kernels)) { throw "没有可打包的内核: $kernels" }
  Copy-Item -Recurse $kernels (Join-Path $stage 'chrome')
}

# ---- NSIS ----
$makensis = @(
  (Get-Command makensis -ErrorAction SilentlyContinue).Source,
  "${env:ProgramFiles(x86)}\NSIS\makensis.exe",
  "$env:ProgramFiles\NSIS\makensis.exe"
) | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
if (-not $makensis) { throw '找不到 makensis.exe, 请先安装 NSIS 3' }

$out = Join-Path $repo 'dist'
& $makensis "/DVERSION=$Version" "/DSTAGE=$stage" "/DOUTDIR=$out" "/DICON=$(Join-Path $desktop 'build/windows/icon.ico')" (Join-Path $PSScriptRoot 'installer.nsi')
if ($LASTEXITCODE -ne 0) { throw "makensis 失败 ($LASTEXITCODE)" }
$setup = Join-Path $out "CZLBrowser-Setup-$Version.exe"
if (-not (Test-Path $setup)) { throw "未生成安装包: $setup" }
Write-Host "安装包: $setup"
