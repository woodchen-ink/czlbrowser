package backend

import (
	"os"
	"path/filepath"
	"strings"
)

// appRootAbs 返回应用根目录的绝对路径，优先使用 App 注入的 appRoot。
func (a *App) appRootAbs() string {
	root := strings.TrimSpace(a.appRoot)
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	if root == "" {
		return ""
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

// mainConfigPath 返回主配置文件路径：优先已初始化的 configPath（生产=<安装根>/config，dev=项目根），
// 为空时（如未调用 SetLayout 的测试场景）回退到 dataRoot/config.yaml。
func (a *App) mainConfigPath() string {
	if strings.TrimSpace(a.configPath) != "" {
		return a.configPath
	}
	return a.resolveAppPath("config.yaml")
}

// appDataDir 返回 data 根目录绝对路径。
func (a *App) appDataDir() string {
	return a.resolveAppPath("data")
}

// appChromeDir 返回 chrome 根目录绝对路径。
func (a *App) appChromeDir() string {
	return a.resolveAppPath("chrome")
}
