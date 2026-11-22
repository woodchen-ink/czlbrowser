# 本地开发: 结束上次残留的开发进程, 补齐依赖, 然后 wails dev (前端热更新 + Go 重编译)。
# 用法: pwsh scripts/dev.ps1
$ErrorActionPreference = 'Stop'
$desktop = Join-Path $PSScriptRoot '..\desktop' | Resolve-Path
Set-Location $desktop

Get-Process czl-browser-dev -ErrorAction SilentlyContinue | Stop-Process -Force

if (-not (Test-Path 'frontend/node_modules')) {
  Push-Location frontend
  npm install
  Pop-Location
}

wails dev
