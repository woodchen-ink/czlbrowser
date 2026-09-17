; CZL Browser NSIS Installer Script
; Usage: makensis /DVERSION=1.0.0 /DSTAGINGDIR=C:\path\to\staging installer.nsi

Unicode True

!ifndef VERSION
  !define VERSION "1.0.0"
!endif
!ifndef STAGINGDIR
  !define STAGINGDIR "..\publish\staging"
!endif

!define PRODUCT_NAME    "CZL Browser"
!define PRODUCT_EXE     "czl-browser.exe"
!define UNINSTALL_KEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\CZLBrowser"
; 按用户安装到 CZL 统一厂商目录，不触发 UAC；安装根目录固定，不提供目录选择页
!define INSTALL_DIR     "$LOCALAPPDATA\CZL\CZL Browser"
; 旧版配置位置：存在时不写入默认配置，留给首次启动迁移
!define LEGACY_CONFIG   "$APPDATA\CZL Browser\config.yaml"
!define POWERSHELL_EXE  "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"

!include "MUI2.nsh"
!include "LogicLib.nsh"

!macro WriteCloseProcessScript HANDLE
  FileWrite ${HANDLE} "param([string]$$InstallDir, [string]$$ExcludePath)$\r$\n"
  FileWrite ${HANDLE} "$$ErrorActionPreference = 'SilentlyContinue'$\r$\n"
  FileWrite ${HANDLE} "if ([string]::IsNullOrWhiteSpace($$InstallDir) -or -not (Test-Path -LiteralPath $$InstallDir)) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$root = [System.IO.Path]::GetFullPath($$InstallDir).TrimEnd('\') + '\'$\r$\n"
  FileWrite ${HANDLE} "$$exclude = ''$\r$\n"
  FileWrite ${HANDLE} "if (-not [string]::IsNullOrWhiteSpace($$ExcludePath)) { $$exclude = [System.IO.Path]::GetFullPath($$ExcludePath) }$\r$\n"
  FileWrite ${HANDLE} "function Get-CZLBrowserProcesses {$\r$\n"
  FileWrite ${HANDLE} "  @(Get-CimInstance Win32_Process | Where-Object {$\r$\n"
  FileWrite ${HANDLE} "    $$_.ExecutablePath -and $$_.ExecutablePath.StartsWith($$root, [System.StringComparison]::OrdinalIgnoreCase) -and ($$exclude -eq '' -or -not $$_.ExecutablePath.Equals($$exclude, [System.StringComparison]::OrdinalIgnoreCase))$\r$\n"
  FileWrite ${HANDLE} "  })$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "$$deadline = (Get-Date).AddSeconds(10)$\r$\n"
  FileWrite ${HANDLE} "do {$\r$\n"
  FileWrite ${HANDLE} "  $$procs = Get-CZLBrowserProcesses$\r$\n"
  FileWrite ${HANDLE} "  if (-not $$procs -or $$procs.Count -eq 0) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "  foreach ($$p in $$procs) { try { Stop-Process -Id $$p.ProcessId -Force -ErrorAction Stop } catch {} }$\r$\n"
  FileWrite ${HANDLE} "  Start-Sleep -Milliseconds 400$\r$\n"
  FileWrite ${HANDLE} "} while ((Get-Date) -lt $$deadline)$\r$\n"
  FileWrite ${HANDLE} "$$left = Get-CZLBrowserProcesses$\r$\n"
  FileWrite ${HANDLE} "if ($$left -and $$left.Count -gt 0) {$\r$\n"
  FileWrite ${HANDLE} "  $$names = ($$left | ForEach-Object { $$_.Name + '#' + $$_.ProcessId }) -join ', '$\r$\n"
  FileWrite ${HANDLE} "  Write-Host ('still running: ' + $$names)$\r$\n"
  FileWrite ${HANDLE} "  exit 1$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "exit 0$\r$\n"
!macroend

Function CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "正在关闭安装目录中的残留进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath ""' $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到旧版本仍有进程占用安装目录。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“重试”将再次尝试关闭残留进程，点击“取消”将终止本次安装。" IDRETRY retry_powershell IDCANCEL install_abort

install_abort:
  Abort "安装已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  DetailPrint "PowerShell 不可用，回退到 taskkill 清理主进程和代理进程..."
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM ${PRODUCT_EXE}' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM xray.exe' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM sing-box.exe' $2
  Sleep 1500

done:
FunctionEnd

Function un.CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "正在关闭安装目录中的残留进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "$INSTDIR\Uninstall.exe"' $2
  Delete $0

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "检测到安装目录中仍有旧进程占用文件。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“重试”将再次尝试关闭残留进程，点击“取消”将终止本次卸载。" IDRETRY retry_powershell IDCANCEL uninstall_abort

uninstall_abort:
  Abort "卸载已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  DetailPrint "PowerShell 不可用，回退到 taskkill 清理主进程和代理进程..."
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM ${PRODUCT_EXE}' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM xray.exe' $2
  ExecWait '"$SYSDIR\taskkill.exe" /F /T /IM sing-box.exe' $2
  Sleep 1500

done:
FunctionEnd

Name "${PRODUCT_NAME} ${VERSION}"
OutFile "..\publish\output\CZLBrowser-Setup-${VERSION}.exe"
InstallDir "${INSTALL_DIR}"
RequestExecutionLevel user
SetCompressor /SOLID lzma

!define MUI_ICON "..\build\windows\icon.ico"
!define MUI_UNICON "..\build\windows\icon.ico"

!insertmacro MUI_PAGE_WELCOME
!define MUI_COMPONENTSPAGE_SMALLDESC
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Launch CZL Browser"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"

; 检测旧版管理员安装（C:\Program Files），提示先卸载。
; 旧版卸载器只删除其安装目录内文件；%LOCALAPPDATA%\CZL Browser 与 %APPDATA%\CZL Browser 中的数据
; 由新版首次启动时迁移到安装根目录。
Function .onInit
  ReadRegStr $R0 HKLM "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $R0 == ""
    Return
  ${EndIf}
  IfFileExists "$R0\Uninstall.exe" 0 legacy_done
  MessageBox MB_YESNO|MB_ICONQUESTION "检测到旧版本安装在：$\r$\n$R0$\r$\n$\r$\n新版本改为按用户安装到：$\r$\n${INSTALL_DIR}$\r$\n$\r$\n是否先卸载旧版本？需要管理员授权。$\r$\n卸载过程询问是否清理数据时请选择“否”，实例数据会在新版首次启动时自动迁移。" IDNO legacy_done
  ; 旧卸载器清单要求管理员，需经 ShellExecute 触发 UAC；_?= 让其原地运行以便等待结束
  ExecShellWait "" "$R0\Uninstall.exe" "_?=$R0"
legacy_done:
FunctionEnd

Section "CZL Browser (required)" SecMain
  SectionIn RO
  Call CloseInstalledProcesses
  SetOutPath "$INSTDIR"
  File "${STAGINGDIR}\${PRODUCT_EXE}"
!if /FileExists "${STAGINGDIR}\config.yaml"
  ; 仅首次安装写入默认配置：已有新版配置或存在待迁移的旧版配置时跳过
  ${IfNot} ${FileExists} "$INSTDIR\config\config.yaml"
  ${AndIfNot} ${FileExists} "${LEGACY_CONFIG}"
    SetOutPath "$INSTDIR\config"
    File "${STAGINGDIR}\config.yaml"
    SetOutPath "$INSTDIR"
  ${EndIf}
!else
  !echo "Warning: ${STAGINGDIR}\\config.yaml not found, installer will use runtime defaults."
!endif
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName"     "${PRODUCT_NAME}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher"       "CZL Browser Team"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon"     "$INSTDIR\${PRODUCT_EXE}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "NoModify"        "1"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "NoRepair"        "1"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Proxy Runtime (xray / sing-box)" SecRuntime
  SectionIn RO
  SetOutPath "$INSTDIR\bin"
  File "${STAGINGDIR}\bin\xray.exe"
  File "${STAGINGDIR}\bin\sing-box.exe"
SectionEnd

; Chrome 内核为可选打包：staging 中存在 chrome\ 才生成该 section。
; CI 构建不分发内核（chrome\ 被 gitignore），用户安装后自备内核。
!if /FileExists "${STAGINGDIR}\chrome\*.*"
Section "Chrome Engine" SecChrome
  SectionIn RO
  SetOutPath "$INSTDIR\chrome"
  File /r "${STAGINGDIR}\chrome\*.*"
SectionEnd
!define HAS_CHROME_SECTION
!endif

Section /o "Desktop Shortcut" SecDesktop
  CreateShortcut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain}    "CZL Browser main program and default config (required)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRuntime} "xray and sing-box proxy tools (vless/vmess/hysteria2)"
!ifdef HAS_CHROME_SECTION
  !insertmacro MUI_DESCRIPTION_TEXT ${SecChrome}  "Bundled Chrome engine for browser instances"
!endif
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Create a shortcut on the desktop"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  Call un.CloseInstalledProcesses

  ; 程序文件与快捷方式总是移除；config/data/chrome/logs 默认保留
  Delete /REBOOTOK "$INSTDIR\${PRODUCT_EXE}"
  Delete /REBOOTOK "$INSTDIR\Uninstall.exe"
  RMDir /r /REBOOTOK "$INSTDIR\bin"
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk"
  RMDir /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}"
  Delete /REBOOTOK "$DESKTOP\${PRODUCT_NAME}.lnk"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  MessageBox MB_ICONQUESTION|MB_YESNO|MB_DEFBUTTON2 "是否同时删除用户数据？$\r$\n$\r$\n选择“是”将删除 config、data（数据库/实例数据）、chrome（内核）与 logs 目录，此操作不可恢复。$\r$\n选择“否”保留这些数据，重新安装后可继续使用。" IDYES un_remove_all_data IDNO un_keep_user_data

un_remove_all_data:
  RMDir /r /REBOOTOK "$INSTDIR\config"
  RMDir /r /REBOOTOK "$INSTDIR\data"
  RMDir /r /REBOOTOK "$INSTDIR\chrome"
  RMDir /r /REBOOTOK "$INSTDIR\logs"
  RMDir /r /REBOOTOK "$INSTDIR"
  RMDir "$LOCALAPPDATA\CZL"
  IfFileExists "$INSTDIR\." 0 un_done
    MessageBox MB_ICONEXCLAMATION|MB_OK "检测到部分文件仍被占用，已标记为重启后自动删除。"
  Goto un_done

un_keep_user_data:
  RMDir /REBOOTOK "$INSTDIR"

un_done:
SectionEnd
