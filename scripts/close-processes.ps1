# 安装 / 卸载前结束安装目录里仍在运行的程序 (主程序、xray、sing-box、内核等)。
# -Skip 指定的文件不结束: 在线更新时安装包本身就放在安装目录下的 data\_update 里。
# 10 秒内结束不掉则返回 1, 由安装程序提示用户重试。
param(
  [Parameter(Mandatory)][string]$Dir,
  [string]$Skip = ''
)
$ErrorActionPreference = 'SilentlyContinue'
if (-not (Test-Path -LiteralPath $Dir)) { exit 0 }

$prefix = [IO.Path]::GetFullPath($Dir).TrimEnd('\') + '\'
$skipPath = if ($Skip) { [IO.Path]::GetFullPath($Skip) } else { '' }
$ci = [StringComparison]::OrdinalIgnoreCase

function Find-Running {
  Get-CimInstance Win32_Process | Where-Object {
    $path = $_.ExecutablePath
    $path -and $path.StartsWith($prefix, $ci) -and -not ($skipPath -and $path.Equals($skipPath, $ci))
  }
}

$until = (Get-Date).AddSeconds(10)
while ((Get-Date) -lt $until) {
  $running = @(Find-Running)
  if ($running.Count -eq 0) { exit 0 }
  $running | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
  Start-Sleep -Milliseconds 400
}
$left = @(Find-Running)
if ($left.Count -eq 0) { exit 0 }
Write-Output ('仍在运行: ' + (($left | ForEach-Object { "$($_.Name)#$($_.ProcessId)" }) -join ', '))
exit 1
