@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

REM 切换到项目根目录（脚本所在目录的上一级）
cd /d "%~dp0.."

echo ========================================
echo   CZL Browser - Wails 构建脚本
echo ========================================
echo.
echo 当前工作目录: %CD%
echo.

REM ======== 代理配置 ========
REM 本地代理地址（例如 Clash、V2Ray 等）
set PROXY_HOST=127.0.0.1
set PROXY_PORT=7890
set USE_PROXY=0

REM 如果不需要使用代理，将 USE_PROXY 设置为 0
REM set USE_PROXY=0

REM 设置代理环境变量
if "%USE_PROXY%"=="1" (
    echo [0/7] 正在配置代理...
    set HTTP_PROXY=http://%PROXY_HOST%:%PROXY_PORT%
    set HTTPS_PROXY=http://%PROXY_HOST%:%PROXY_PORT%
    set http_proxy=http://%PROXY_HOST%:%PROXY_PORT%
    set https_proxy=http://%PROXY_HOST%:%PROXY_PORT%
    
    REM 配置 npm 代理
    call npm config set proxy http://%PROXY_HOST%:%PROXY_PORT% 2>nul
    call npm config set https-proxy http://%PROXY_HOST%:%PROXY_PORT% 2>nul
    
    REM 配置 Go 代理环境变量
    set GOPROXY=https://goproxy.cn,direct
    
    echo ✓ 代理已配置: %PROXY_HOST%:%PROXY_PORT%
    echo.
)

REM 定义清理函数（用于恢复代理设置）
goto :skip_cleanup_function
:cleanup
if "%USE_PROXY%"=="1" (
    echo.
    echo [清理代理配置...]
    call npm config delete proxy 2>nul
    call npm config delete https-proxy 2>nul
    echo ✓ 代理配置已清理
)
exit /b
:skip_cleanup_function


echo [1/7] 安装前端依赖...
cd frontend
call npm install
if %errorlevel% neq 0 (
    echo ✗ 安装前端依赖失败
    cd ..
    call :cleanup
    pause
    exit /b 1
)
cd ..

echo.
echo [2/7] 安装 Go 依赖...
go mod download
go mod tidy
if %errorlevel% neq 0 (
    echo ✗ 安装 Go 依赖失败
    call :cleanup
    pause
    exit /b 1
)

echo.
echo [3/7] 创建临时 dist 目录...
if not exist "frontend\dist" (
    mkdir "frontend\dist"
    echo. > "frontend\dist\index.html"
    echo ✓ 临时 dist 目录已创建
) else (
    echo ✓ dist 目录已存在
)

echo.
echo [4/7] 生成 Wails 绑定文件...
call bat\generate-bindings.bat --no-pause
if %errorlevel% neq 0 (
    echo ✗ 生成绑定文件失败
    call :cleanup
    pause
    exit /b 1
)

echo.
echo [5/7] 构建前端项目...
REM 清理临时 dist 目录
if exist "frontend\dist" (
    rmdir /S /Q "frontend\dist" 2>nul
    echo ✓ 临时 dist 目录已清理
)
cd frontend
call npm run build
if %errorlevel% neq 0 (
    echo ✗ 构建前端失败
    cd ..
    call :cleanup
    pause
    exit /b 1
)
cd ..

echo.
echo [6/7] 构建应用...
wails build
if %errorlevel% neq 0 (
    echo ✗ 构建失败
    call :cleanup
    pause
    exit /b 1
)

echo.
echo [7/7] 复制运行时依赖...
if exist "bin" (
    xcopy /E /I /Y bin build\bin\bin >nul
    echo ✓ bin 目录已复制到 build\bin\bin\
) else (
    echo [Warn] bin 目录不存在，跳过复制
)

echo.
echo ========================================
echo   ✓ 构建成功！
echo ========================================
echo.
echo 可执行文件位置: build\bin\czl-chrome.exe
echo.

REM 清理代理配置
call :cleanup

exit /b 0
