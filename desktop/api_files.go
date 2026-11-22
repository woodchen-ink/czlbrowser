package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OpenUserDataDir 在资源管理器里打开实例数据目录; 传空打开用户数据根目录。目录不存在时先创建。
func (a *App) OpenUserDataDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if !filepath.IsAbs(dir) {
		root := strings.TrimSpace(a.config.Browser.UserDataRoot)
		if root == "" {
			root = "data"
		}
		dir = filepath.Join(a.resolveAppPath(root), dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	return revealInExplorer(dir)
}

// OpenCorePath 在资源管理器里打开内核目录。
func (a *App) OpenCorePath(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("内核路径不能为空")
	}
	dir = a.resolveAppPath(dir)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("路径不存在: %s", dir)
	}
	return revealInExplorer(dir)
}

// revealInExplorer 打开目录, 或打开文件所在目录并选中它。
// 不能给 explorer 加 HideWindow, 否则窗口不会出现。
func revealInExplorer(p string) error {
	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return exec.Command("explorer.exe", abs).Start()
	}
	return exec.Command("explorer.exe", "/select,", abs).Start()
}
