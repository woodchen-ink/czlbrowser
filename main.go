package main

import (
	"context"
	"czl-browser/backend"
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// appRoot 应用根目录，所有相对路径基于此目录解析。
// 生产环境 = exe 所在目录；dev 环境 = 项目源码根目录（CWD）。
var appRoot string

// isDevMode 标识当前是否为 wails dev 模式（exe 在临时目录）
var isDevMode bool

type App struct {
	*backend.App
}

func NewApp(appRoot string) *App {
	return &App{App: backend.NewApp(appRoot)}
}

func (a *App) startup(ctx context.Context) {
	backend.Start(a.App, ctx)
}

func (a *App) shutdown(ctx context.Context) {
	backend.Stop(a.App, ctx)
}

func (a *App) shouldBlockClose(ctx context.Context) bool {
	return backend.ShouldBlockClose(a.App, ctx)
}

func main() {
	// 确定应用根目录：
	// 1. 生产环境：exe 所在目录（快捷方式启动时 CWD 可能不对，需要修正）
	// 2. dev 环境：wails dev 时 exe 可能在 temp 目录或 build/bin 目录，使用当前工作目录
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		tempDir := os.TempDir()
		if resolved, err := filepath.EvalSymlinks(exeDir); err == nil {
			exeDir = resolved
		}
		if resolved, err := filepath.EvalSymlinks(tempDir); err == nil {
			tempDir = resolved
		}

		exeDirLower := strings.ToLower(exeDir)
		inTemp := strings.HasPrefix(exeDirLower, strings.ToLower(tempDir))
		// wails dev 会把 exe 编译到 build/bin/ 目录
		inBuildBin := strings.HasSuffix(filepath.ToSlash(exeDirLower), "/build/bin")

		if inTemp || inBuildBin {
			// dev 模式：exe 在临时目录或 build/bin，使用 CWD 作为根目录
			isDevMode = true
			if cwd, err := os.Getwd(); err == nil {
				appRoot = cwd
			} else {
				appRoot = "."
			}
		} else {
			// 生产模式：使用 exe 所在目录
			isDevMode = false
			appRoot = exeDir
			os.Chdir(exeDir)
		}
	} else {
		// 兜底：使用 CWD
		if cwd, err := os.Getwd(); err == nil {
			appRoot = cwd
		} else {
			appRoot = "."
		}
	}

	log.Printf("应用根目录: %s (dev=%v)", appRoot, isDevMode)

	// 确定配置文件路径：dev 模式使用项目目录，生产模式使用 %APPDATA%
	var configPath string
	if isDevMode {
		configPath = filepath.Join(appRoot, "config.yaml")
	} else {
		configPath = backend.ConfigPath()
	}
	log.Printf("配置文件路径: %s", configPath)

	// 加载配置（磁盘文件不存在时回退到嵌入默认配置）
	cfg, err := backend.LoadConfig(configPath)
	if err != nil {
		log.Printf("加载配置失败，使用默认配置: %v", err)
		cfg = backend.DefaultConfig()
	}

	// 创建应用实例
	app := NewApp(appRoot)
	app.SetDevMode(isDevMode)

	var wailsCtx context.Context

	// 启动应用
	err = wails.Run(&options.App{
		Title:     cfg.App.Name,
		Width:     cfg.App.Window.Width,
		Height:    cfg.App.Window.Height,
		MinWidth:  cfg.App.Window.MinWidth,
		MinHeight: cfg.App.Window.MinHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 255},
		OnStartup: func(ctx context.Context) {
			wailsCtx = ctx
			// 启动系统托盘（非阻塞）
			go backend.RunTray(backend.TrayCallbacks{
				OnShow: func() {
					runtime.WindowShow(wailsCtx)
					runtime.WindowUnminimise(wailsCtx)
				},
				OnQuit: func() {
					app.ForceQuit()
				},
			})
			app.startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			backend.QuitTray()
			app.shutdown(ctx)
		},
		// 拦截关闭按钮事件，由前端处理自定义对话框
		OnBeforeClose: func(ctx context.Context) bool {
			return app.shouldBlockClose(ctx)
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		log.Fatal("启动应用失败:", err)
	}
}
