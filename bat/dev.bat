@echo off
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0.."

echo ========================================
echo   CZL Browser - Dev Launcher
echo ========================================
echo.
echo Current workdir: %CD%
echo.

call :cleanup_dev_logs

echo Cleaning stale processes...
taskkill /F /IM czl-browser-dev.exe >nul 2>&1
taskkill /F /IM czl-browser.exe >nul 2>&1
echo.

set FRONTEND_PORT=5218
set PORT_ERROR=0
set TEMP_DEV_DIST_CREATED=0
set TEMP_DEV_PLACEHOLDER_CREATED=0

call :cleanup_local_vite_port %FRONTEND_PORT%

echo Checking port status...
call :check_port %FRONTEND_PORT%

if "!PORT_ERROR!"=="1" (
    echo.
    echo Please close the process using the occupied port and retry.
    pause
    exit /b 1
)
echo.

set GOPROXY=https://goproxy.cn,direct

echo Checking dependencies...
if not exist "go.sum" (
    echo Installing Go dependencies...
    go mod download
    go mod tidy
)

if not exist "frontend\node_modules" (
    echo Installing frontend dependencies...
    pushd frontend
    call npm install
    popd
)
echo.

echo Regenerating Wails bindings...
if not exist "frontend\dist" (
    mkdir "frontend\dist"
    set TEMP_DEV_DIST_CREATED=1
)
if not exist "frontend\dist\__wails_placeholder__.txt" (
    echo placeholder> "frontend\dist\__wails_placeholder__.txt"
    set TEMP_DEV_PLACEHOLDER_CREATED=1
)
wails generate module
if errorlevel 1 (
    call :cleanup_temp_dist
    echo [ERROR] Failed to generate Wails bindings.
    pause
    exit /b 1
)

if exist "frontend\wailsjs" (
    xcopy /E /I /Y "frontend\wailsjs" "frontend\src\wailsjs" >nul
)
if not exist "frontend\src\wailsjs" (
    call :cleanup_temp_dist
    echo [ERROR] Wails bindings output folder not found.
    pause
    exit /b 1
)
call :cleanup_temp_dist
echo.

echo Starting dev server...
echo Frontend URL: http://127.0.0.1:%FRONTEND_PORT%
echo Wails dev endpoint: auto-select
echo.

wails dev -viteservertimeout 60
set EXIT_CODE=%errorlevel%

if not "%EXIT_CODE%"=="0" (
    echo.
    echo [ERROR] wails dev exited with code %EXIT_CODE%.
)

pause
exit /b %EXIT_CODE%

:cleanup_temp_dist
if "%TEMP_DEV_PLACEHOLDER_CREATED%"=="1" (
    del /F /Q "frontend\dist\__wails_placeholder__.txt" >nul 2>&1
)
if "%TEMP_DEV_DIST_CREATED%"=="1" (
    rmdir /S /Q "frontend\dist" >nul 2>&1
)
exit /b 0

:cleanup_dev_logs
for %%f in (
    "tmp-npm-dev.err.log"
    "tmp-npm-dev.log"
    "tmp-wails-err.log"
    "tmp-wails-out.log"
    "tmp-wails2-err.log"
    "tmp-wails2-out.log"
    "tmp-wails3-err.log"
    "tmp-wails3-out.log"
    "tmp-wails.err"
    "wails-dev-capture.log"
    "wails-dev-run.log"
    "wails-dev-stderr.log"
    "wails-dev-stdout.log"
) do (
    if exist %%~f del /F /Q %%~f >nul 2>&1
)
exit /b 0

:cleanup_local_vite_port
set "CHECK_PORT=%~1"
set "CHECK_PID="
set "CHECK_CMDLINE="
for /f "usebackq delims=" %%a in (`powershell -NoProfile -Command "$port=%CHECK_PORT%; $procId=(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess); if($procId){Write-Output $procId}"`) do (
    set "CHECK_PID=%%a"
)
if not defined CHECK_PID exit /b 0

for /f "usebackq delims=" %%a in (`powershell -NoProfile -Command "$line=Get-CimInstance Win32_Process | Where-Object { $_.ProcessId -eq %CHECK_PID% } | Select-Object -First 1 -ExpandProperty CommandLine; if($line){Write-Output $line}"`) do (
    set "CHECK_CMDLINE=%%a"
)

echo !CHECK_CMDLINE! | findstr /I /C:"%CD%\frontend" >nul
set "MATCH_PROJECT=!errorlevel!"
echo !CHECK_CMDLINE! | findstr /I /C:"vite" >nul
set "MATCH_VITE=!errorlevel!"

if "!MATCH_PROJECT!"=="0" if "!MATCH_VITE!"=="0" (
    echo Cleaning stale local Vite process on port %CHECK_PORT% ^(PID !CHECK_PID!^)...
    taskkill /F /PID !CHECK_PID! /T >nul 2>&1
    timeout /t 1 /nobreak >nul
)
exit /b 0

:check_port
set "CHECK_PORT=%~1"
set "CHECK_PID="
for /f "usebackq delims=" %%a in (`powershell -NoProfile -Command "$port=%CHECK_PORT%; $procId=(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess); if($procId){Write-Output $procId}"`) do (
    set "CHECK_PID=%%a"
)

if defined CHECK_PID (
    set PORT_ERROR=1
    echo [ERROR] Port %CHECK_PORT% is occupied. PID: !CHECK_PID!
) else (
    echo [OK] Port %CHECK_PORT% is available.
)
exit /b 0
