package main

import (
	"context"
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

// layout 描述本次运行用到的三个位置。
//   - programDir: 随程序分发的只读资源 (bin/xray.exe 等) 所在目录
//   - dataDir: 可写数据根 (data/ chrome/ logs/ ...)
//   - configFile: 主配置文件
type layout struct {
	dev        bool
	programDir string
	dataDir    string
	configFile string
}

// isDevBuild 判断当前 exe 是否由 wails dev 产生: 它会把程序编译到 build/bin 或系统临时目录。
func isDevBuild(exeDir string) bool {
	dir := strings.ToLower(filepath.ToSlash(exeDir))
	if strings.HasSuffix(dir, "/build/bin") {
		return true
	}
	tmp := os.TempDir()
	if resolved, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = resolved
	}
	return strings.HasPrefix(dir, strings.ToLower(filepath.ToSlash(tmp)))
}

// resolveLayout 开发模式一切都在工程目录 (当前工作目录);
// 正式安装时程序资源在 exe 旁边, 数据与配置收敛到 %LOCALAPPDATA%\CZL\CZL Browser。
func resolveLayout() layout {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	exe, err := os.Executable()
	if err != nil {
		return layout{dev: true, programDir: cwd, dataDir: cwd, configFile: filepath.Join(cwd, "config.yaml")}
	}
	exeDir := filepath.Dir(exe)
	if resolved, err := filepath.EvalSymlinks(exeDir); err == nil {
		exeDir = resolved
	}
	if isDevBuild(exeDir) {
		return layout{dev: true, programDir: cwd, dataDir: cwd, configFile: filepath.Join(cwd, "config.yaml")}
	}

	// 从快捷方式启动时工作目录不可靠, 统一切到程序目录。
	_ = os.Chdir(exeDir)
	root, err := InstallRoot()
	if err != nil {
		log.Fatalf("解析安装根目录失败: %v", err)
	}
	moved, failures := MigrateLegacyLayout(root)
	for _, p := range moved {
		log.Printf("已迁移旧版目录: %s", p)
	}
	for _, e := range failures {
		log.Printf("迁移旧版目录失败: %v", e)
	}
	return layout{programDir: exeDir, dataDir: root, configFile: ConfigPathIn(root)}
}

func main() {
	lay := resolveLayout()
	log.Printf("程序目录: %s, 数据目录: %s, 配置: %s (dev=%v)", lay.programDir, lay.dataDir, lay.configFile, lay.dev)

	cfg, err := LoadConfig(lay.configFile)
	if err != nil {
		log.Printf("加载配置失败, 使用默认配置: %v", err)
		cfg = DefaultConfig()
	}

	app := NewApp(lay.programDir)
	app.SetLayout(lay.dev, lay.dataDir, lay.configFile)

	err = wails.Run(&options.App{
		Title:            cfg.App.Name,
		Width:            cfg.App.Window.Width,
		Height:           cfg.App.Window.Height,
		MinWidth:         cfg.App.Window.MinWidth,
		MinHeight:        cfg.App.Window.MinHeight,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 255},
		OnStartup: func(ctx context.Context) {
			go RunTray(TrayCallbacks{
				OnShow: func() {
					runtime.WindowShow(ctx)
					runtime.WindowUnminimise(ctx)
				},
				OnQuit: app.ForceQuit,
			})
			app.startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			QuitTray()
			app.shutdown(ctx)
		},
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
		Windows:       &windows.Options{},
	})
	if err != nil {
		log.Fatal("启动应用失败: ", err)
	}
}
