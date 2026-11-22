; CZL Browser 安装程序 (NSIS 3, 由 scripts/build.ps1 -Installer 调用)
;   makensis /DVERSION=<版本> /DSTAGE=<安装内容目录> /DOUTDIR=<输出目录> /DICON=<图标> installer.nsi
;
; 相对路径都相对本脚本所在目录 (makensis 会切到这里)。
; 按用户安装到固定目录 %LOCALAPPDATA%\CZL\CZL Browser, 不弹 UAC, 卸载信息写 HKCU。
; 程序内在线更新会以 /S /RELAUNCH /WAITPID=<pid> 运行本安装包。

Unicode True
; 压缩方式必须在任何插件调用之前设定
SetCompressor /SOLID lzma

!ifndef VERSION
  !define VERSION "dev"
!endif
!ifndef STAGE
  !error "需要 /DSTAGE=<安装内容目录>"
!endif
!ifndef OUTDIR
  !define OUTDIR "..\dist"
!endif
!ifndef ICON
  !define ICON "..\desktop\build\windows\icon.ico"
!endif

!define APP_NAME   "CZL Browser"
!define APP_EXE    "czl-browser.exe"
!define REG_KEY    "Software\Microsoft\Windows\CurrentVersion\Uninstall\CZLBrowser"
!define PS         "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"
; 旧版 (Program Files + %APPDATA%) 的配置; 存在时不放默认配置, 由程序首次启动迁移
!define OLD_CONFIG "$APPDATA\CZL Browser\config.yaml"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"

Name "${APP_NAME} ${VERSION}"
OutFile "${OUTDIR}\CZLBrowser-Setup-${VERSION}.exe"
InstallDir "$LOCALAPPDATA\CZL\CZL Browser"
RequestExecutionLevel user
BrandingText "CZL"

!define MUI_ICON   "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "启动 CZL Browser"
; 借用完成页的第二个勾选框创建桌面快捷方式
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_TEXT "创建桌面快捷方式"
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION MakeDesktopShortcut

!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

; ---------- 结束安装目录里的进程 ----------
; SKIP: 不结束的文件 (正在运行的安装包 / 卸载程序自身)
!macro StopRunning SKIP
  InitPluginsDir
  File "/oname=$PLUGINSDIR\close-processes.ps1" "close-processes.ps1"
  ${Do}
    ${IfNot} ${FileExists} "${PS}"
      nsExec::Exec '"$SYSDIR\taskkill.exe" /F /T /IM ${APP_EXE}'
      Pop $0
      nsExec::Exec '"$SYSDIR\taskkill.exe" /F /T /IM xray.exe'
      Pop $0
      nsExec::Exec '"$SYSDIR\taskkill.exe" /F /T /IM sing-box.exe'
      Pop $0
      Sleep 1500
      ${Break}
    ${EndIf}
    nsExec::Exec '"${PS}" -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\close-processes.ps1" -Dir "$INSTDIR" -Skip "${SKIP}"'
    Pop $0
    ${If} $0 == 0
      ${Break}
    ${EndIf}
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "安装目录中还有程序在运行, 无法继续:$\r$\n$INSTDIR$\r$\n$\r$\n关闭 CZL Browser 及其浏览器窗口后点「重试」。" /SD IDCANCEL IDRETRY +2
    Abort "已取消: 安装目录中的程序仍在运行。"
  ${Loop}
!macroend

; ---------- 安装 ----------
Function .onInit
  ; 在线更新: 先等旧进程自己退出 (实例关闭、数据库落盘), 最多 60 秒
  ${GetParameters} $R0
  ClearErrors
  ${GetOptions} $R0 "/WAITPID=" $R1
  ${IfNot} ${Errors}
  ${AndIf} $R1 != ""
    ${If} ${FileExists} "${PS}"
      nsExec::Exec '"${PS}" -NoProfile -Command "Wait-Process -Id $R1 -Timeout 60 -ErrorAction SilentlyContinue"'
      Pop $0
    ${Else}
      Sleep 5000
    ${EndIf}
  ${EndIf}

  ; 旧版本按管理员装在 Program Files: 提示先卸载 (旧卸载程序需要 UAC, 必须经 ShellExecute)
  ReadRegStr $R2 HKLM "${REG_KEY}" "InstallLocation"
  ${If} $R2 != ""
  ${AndIf} ${FileExists} "$R2\Uninstall.exe"
    MessageBox MB_YESNO|MB_ICONQUESTION "发现旧版本安装在:$\r$\n$R2$\r$\n$\r$\n新版本改为装在:$\r$\n$INSTDIR$\r$\n$\r$\n现在卸载旧版本吗? 需要管理员授权。$\r$\n卸载时询问是否删除数据请选「否」, 新版首次启动会自动迁移实例数据。" /SD IDNO IDNO +2
    ExecShellWait "" "$R2\Uninstall.exe" "_?=$R2"
  ${EndIf}
FunctionEnd

Section "Install"
  !insertmacro StopRunning "$EXEPATH"

  SetOutPath "$INSTDIR"
  File "${STAGE}\${APP_EXE}"
  SetOutPath "$INSTDIR\bin"
  File "${STAGE}\bin\xray.exe"
  File "${STAGE}\bin\sing-box.exe"
!if /FileExists "${STAGE}\chrome\*.*"
  SetOutPath "$INSTDIR\chrome"
  File /r "${STAGE}\chrome\*.*"
!endif

  ; 默认配置只在全新安装时放: 已有配置或待迁移的旧配置都不覆盖
  ${IfNot} ${FileExists} "$INSTDIR\config\config.yaml"
  ${AndIfNot} ${FileExists} "${OLD_CONFIG}"
    SetOutPath "$INSTDIR\config"
    File "/oname=config.yaml" "${STAGE}\config.yaml"
  ${EndIf}

  SetOutPath "$INSTDIR"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "${REG_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${REG_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${REG_KEY}" "Publisher" "CZL"
  WriteRegStr HKCU "${REG_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${REG_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE}"
  WriteRegStr HKCU "${REG_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKCU "${REG_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${REG_KEY}" "NoRepair" 1

  ; 开始菜单只放一个快捷方式, 与其它 CZL 应用一致; 卸载走 设置 → 应用。
  ; 旧版本建的「CZL Browser」文件夹 (含卸载快捷方式) 顺手删掉
  RMDir /r "$SMPROGRAMS\${APP_NAME}"
  CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
SectionEnd

Function MakeDesktopShortcut
  CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
FunctionEnd

Function .onInstSuccess
  ${GetParameters} $R0
  ClearErrors
  ${GetOptions} $R0 "/RELAUNCH" $R1
  ${IfNot} ${Errors}
    Exec '"$INSTDIR\${APP_EXE}"'
  ${EndIf}
FunctionEnd

; ---------- 卸载 ----------
; 程序与快捷方式总是删除; config / data / chrome / logs 默认保留, 由用户决定
Section "Uninstall"
  !insertmacro StopRunning "$INSTDIR\Uninstall.exe"

  Delete "$INSTDIR\${APP_EXE}"
  RMDir /r "$INSTDIR\bin"
  Delete "$DESKTOP\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}.lnk"
  RMDir /r "$SMPROGRAMS\${APP_NAME}"
  DeleteRegKey HKCU "${REG_KEY}"

  MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2 "同时删除所有数据吗?$\r$\n$\r$\n包括配置、浏览器实例与登录状态、下载的内核和日志, 删除后无法恢复。$\r$\n选「否」则保留, 重新安装后可以继续使用。" /SD IDNO IDNO keep
  RMDir /r /REBOOTOK "$INSTDIR\config"
  RMDir /r /REBOOTOK "$INSTDIR\data"
  RMDir /r /REBOOTOK "$INSTDIR\chrome"
  RMDir /r /REBOOTOK "$INSTDIR\logs"
keep:
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  RMDir "$LOCALAPPDATA\CZL"
SectionEnd
