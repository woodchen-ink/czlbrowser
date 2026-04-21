@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

REM 切换到项目根目录（脚本所在目录的上一级）
cd /d "%~dp0.."

echo ========================================
echo   CZL Browser - 发布打包脚本
echo ========================================
echo.
echo 当前工作目录: %CD%
echo.

REM ======== [1/6] 检测 NSIS ========
echo [1/6] 检测 NSIS 安装...
echo   支持环境变量：MAKENSIS_PATH / NSIS_PATH / NSIS_HOME
echo.

set "MAKENSIS="

REM 优先级 1：MAKENSIS_PATH 直接指向 makensis.exe
if defined MAKENSIS_PATH (
    if exist "!MAKENSIS_PATH!" (
        set "MAKENSIS=!MAKENSIS_PATH!"
        goto :nsis_found
    )
)

REM 优先级 2：NSIS_PATH 可以是 makensis.exe 或 NSIS 目录
if defined NSIS_PATH (
    if exist "!NSIS_PATH!\makensis.exe" (
        set "MAKENSIS=!NSIS_PATH!\makensis.exe"
        goto :nsis_found
    )
    if exist "!NSIS_PATH!" (
        set "MAKENSIS=!NSIS_PATH!"
        goto :nsis_found
    )
)

REM 优先级 3：NSIS_HOME 为 NSIS 安装根目录
if defined NSIS_HOME (
    if exist "!NSIS_HOME!\makensis.exe" (
        set "MAKENSIS=!NSIS_HOME!\makensis.exe"
        goto :nsis_found
    )
)

REM 优先级 4：系统 PATH
for /f "delims=" %%i in ('where makensis.exe 2^>nul') do (
    set "MAKENSIS=%%i"
    goto :nsis_found
)

REM 优先级 5：常见安装目录
if exist "C:\Program Files (x86)\NSIS\makensis.exe" (
    set "MAKENSIS=C:\Program Files (x86)\NSIS\makensis.exe"
    goto :nsis_found
)
if exist "C:\Program Files\NSIS\makensis.exe" (
    set "MAKENSIS=C:\Program Files\NSIS\makensis.exe"
    goto :nsis_found
)

echo ✗ 未找到 NSIS（makensis.exe）
echo.
echo   请安装 NSIS 后，通过以下任一方式配置（PowerShell）：
echo     setx MAKENSIS_PATH "D:\tools\NSIS\makensis.exe"
echo     setx NSIS_PATH     "D:\tools\NSIS"
echo     setx NSIS_HOME     "D:\tools\NSIS"
echo.
echo   或下载安装：https://nsis.sourceforge.io/Download
echo.
pause
exit /b 1

:nsis_found
echo ✓ NSIS 已就绪: !MAKENSIS!
echo.

REM ======== [2/6] 读取版本号 ========
echo [2/6] 读取版本号...

set "VERSION="
for /f "usebackq delims=" %%v in (`powershell -NoProfile -Command "(Get-Content -Encoding UTF8 wails.json | ConvertFrom-Json).info.productVersion"`) do (
    set "VERSION=%%v"
)

if "!VERSION!"=="" (
    echo ✗ 无法从 wails.json 读取版本号
    pause
    exit /b 1
)
echo ✓ 版本号: !VERSION!
echo.

REM ======== [3/6] Wails 构建 ========
echo [3/6] 执行 Wails 构建...

set GOPROXY=https://goproxy.cn,direct
wails build
if %errorlevel% neq 0 (
    echo ✗ Wails 构建失败
    pause
    exit /b 1
)

if not exist "build\bin\czl-browser.exe" (
    echo ✗ 构建产物不存在: build\bin\czl-browser.exe
    pause
    exit /b 1
)
echo ✓ 构建成功: build\bin\czl-browser.exe
echo.

REM ======== [4/6] 组装 staging 目录 ========
echo [4/6] 组装 staging 目录...

set "STAGING=publish\staging"
set "RELEASE_CONFIG=publish\config.init.yaml"

if exist "!STAGING!" rmdir /S /Q "!STAGING!"
mkdir "!STAGING!"

copy /Y "build\bin\czl-browser.exe" "!STAGING!\czl-browser.exe" >nul
if errorlevel 1 (
    echo ✗ 复制 czl-browser.exe 失败
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
if not exist "!STAGING!\czl-browser.exe" (
    echo ✗ staging 中缺少 czl-browser.exe
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
echo ✓ 复制 czl-browser.exe

if not exist "!RELEASE_CONFIG!" (
    echo ✗ 未找到发布配置模板: !RELEASE_CONFIG!
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
copy /Y "!RELEASE_CONFIG!" "!STAGING!\config.yaml" >nul
if errorlevel 1 (
    echo ✗ 复制发布配置模板失败: !RELEASE_CONFIG!
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
if not exist "!STAGING!\config.yaml" (
    echo ✗ staging 中缺少 config.yaml（来源: !RELEASE_CONFIG!）
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
echo ✓ 复制发布配置模板 !RELEASE_CONFIG! -> config.yaml

if not exist "bin" (
    echo ✗ bin\ 目录不存在，缺少代理运行时文件
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
if not exist "bin\xray.exe" (
    echo ✗ 缺少运行时文件: bin\xray.exe
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
if not exist "bin\sing-box.exe" (
    echo ✗ 缺少运行时文件: bin\sing-box.exe
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)

xcopy /E /I /Y bin "!STAGING!\bin" >nul
if not exist "!STAGING!\bin\xray.exe" (
    echo ✗ 复制后仍缺少 !STAGING!\bin\xray.exe
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
if not exist "!STAGING!\bin\sing-box.exe" (
    echo ✗ 复制后仍缺少 !STAGING!\bin\sing-box.exe
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
echo ✓ 复制 bin\（xray.exe, sing-box.exe）

if not exist "chrome" (
    echo ✗ chrome\ 目录不存在，Chrome 内核为必需文件
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
xcopy /E /I /Y chrome "!STAGING!\chrome" >nul
echo ✓ 复制 chrome\

mkdir "!STAGING!\data"
echo ✓ 创建空 data 目录（不打包 app.db，首次启动自动初始化）

echo.
echo ✓ staging 目录组装完成
echo.

REM ======== [5/6] NSIS 打包 ========
echo [5/6] 调用 NSIS 打包...

if not exist "publish\output" mkdir "publish\output"

REM 确保 installer.nsi 是 UTF-8 with BOM（NSIS Unicode True 要求）
powershell -NoProfile -Command "$f=(Resolve-Path 'publish\installer.nsi').Path; $c=[System.IO.File]::ReadAllText($f,[System.Text.Encoding]::UTF8); [System.IO.File]::WriteAllText($f,$c,[System.Text.UTF8Encoding]::new($true))" >nul

for /f "usebackq delims=" %%p in (`powershell -NoProfile -Command "(Resolve-Path '!STAGING!').Path"`) do (
    set "STAGING_ABS=%%p"
)

"!MAKENSIS!" /DVERSION=!VERSION! "/DSTAGINGDIR=!STAGING_ABS!" publish\installer.nsi
if %errorlevel% neq 0 (
    echo ✗ NSIS 打包失败
    rmdir /S /Q "!STAGING!"
    pause
    exit /b 1
)
echo ✓ 安装包生成成功
echo.

REM ======== [6/6] 清理 staging ========
echo [6/6] 清理临时文件...
rmdir /S /Q "!STAGING!"
echo ✓ staging 目录已清理
echo.

echo ========================================
echo   ✓ 发布完成！
echo ========================================
echo.
echo 安装包位置: publish\output\CZLBrowser-Setup-!VERSION!.exe
echo.
echo 提示：用户安装后可将旧的 data\ 目录粘贴到安装目录覆盖初始数据
echo.

exit /b 0
