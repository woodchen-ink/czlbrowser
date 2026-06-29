//go:build windows
// +build windows

package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func killResidualRuntimeProcesses(appRoot string, dataRoot string) error {
	appRoot = strings.TrimSpace(appRoot)
	dataRoot = strings.TrimSpace(dataRoot)
	if appRoot == "" && dataRoot == "" {
		return nil
	}
	// 只读资源（bin/xray.exe 等）在安装目录 appRoot；chrome 内核已迁至可写数据目录 dataRoot。
	// 两个根都需扫描，避免迁移后漏杀运行在 dataRoot\chrome 下的内核进程。
	if _, err := os.Stat(appRoot); err != nil {
		appRoot = ""
	}
	if _, err := os.Stat(dataRoot); err != nil {
		dataRoot = ""
	}
	if appRoot == "" && dataRoot == "" {
		return nil
	}

	exePath, _ := os.Executable()
	psScript := `param([string]$AppRoot, [string]$DataRoot, [string]$ExcludePath)
$ErrorActionPreference = 'SilentlyContinue'
$prefixes = @()
if (-not [string]::IsNullOrWhiteSpace($AppRoot) -and (Test-Path -LiteralPath $AppRoot)) {
  $appRootFull = [System.IO.Path]::GetFullPath($AppRoot).TrimEnd('\') + '\'
  $prefixes += ($appRootFull + 'bin\')
}
if (-not [string]::IsNullOrWhiteSpace($DataRoot) -and (Test-Path -LiteralPath $DataRoot)) {
  $dataRootFull = [System.IO.Path]::GetFullPath($DataRoot).TrimEnd('\') + '\'
  $prefixes += ($dataRootFull + 'chrome\')
  $prefixes += ($dataRootFull + 'bin\')
}
if ($prefixes.Count -eq 0) { exit 0 }
$exclude = ''
if (-not [string]::IsNullOrWhiteSpace($ExcludePath)) {
  $exclude = [System.IO.Path]::GetFullPath($ExcludePath)
}
function Get-AntChromeResidualProcesses {
  @(
    Get-CimInstance Win32_Process | Where-Object {
      $proc = $_
      $proc.ExecutablePath -and (
        @($prefixes | Where-Object { $proc.ExecutablePath.StartsWith($_, [System.StringComparison]::OrdinalIgnoreCase) }).Count -gt 0
      ) -and (
        $exclude -eq '' -or
        -not $proc.ExecutablePath.Equals($exclude, [System.StringComparison]::OrdinalIgnoreCase)
      )
    }
  )
}
$targets = @(Get-AntChromeResidualProcesses | Sort-Object ProcessId -Descending)
foreach ($p in $targets) {
  try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop } catch {}
}
Start-Sleep -Milliseconds 400
$left = @(Get-AntChromeResidualProcesses)
if ($left.Count -gt 0) {
  $names = ($left | ForEach-Object { $_.Name + '#' + $_.ProcessId }) -join ', '
  Write-Host ('still running: ' + $names)
  exit 1
}
exit 0
`

	tempFile, err := os.CreateTemp("", "czl-browser-cleanup-*.ps1")
	if err != nil {
		return fmt.Errorf("创建清理脚本失败: %w", err)
	}
	scriptPath := tempFile.Name()
	if _, err := tempFile.WriteString(psScript); err != nil {
		tempFile.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("写入清理脚本失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("关闭清理脚本失败: %w", err)
	}
	defer os.Remove(scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	powershellPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := os.Stat(powershellPath); err != nil {
		if fallbackPath, lookErr := exec.LookPath("powershell.exe"); lookErr == nil {
			powershellPath = fallbackPath
		} else {
			return fmt.Errorf("未找到 powershell.exe")
		}
	}

	cmd := exec.CommandContext(ctx, powershellPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Clean(scriptPath), "-AppRoot", appRoot, "-DataRoot", dataRoot, "-ExcludePath", exePath)
	hideWindow(cmd)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("清理残留进程超时")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("清理残留进程失败: %s", message)
	}
	return nil
}
