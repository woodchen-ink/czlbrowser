package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	appconfig "github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	apptray "github.com/woodchen-ink/czlbrowser/desktop/internal/tray"
)

type Config = appconfig.Config
type TrayCallbacks = apptray.Callbacks

// LoadConfig 读取主配置。文件损坏时先把原文件另存为 .broken-<时间>, 再写入一份默认配置,
// 保证启动不会卡在坏配置上。
func LoadConfig(path string) (*Config, error) {
	cfg, loadErr := appconfig.Load(path)
	if loadErr == nil {
		return cfg, nil
	}

	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		broken := path + ".broken-" + time.Now().Format("20060102-150405")
		if err := os.WriteFile(broken, raw, 0o644); err != nil {
			return appconfig.DefaultConfig(), fmt.Errorf("加载配置失败: %w；备份损坏配置失败: %v", loadErr, err)
		}
	}

	fresh := appconfig.DefaultConfig()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fresh, fmt.Errorf("加载配置失败: %w；创建配置目录失败: %v", loadErr, err)
	}
	if err := fresh.Save(path); err != nil {
		return fresh, fmt.Errorf("加载配置失败: %w；重建默认配置失败: %v", loadErr, err)
	}
	return fresh, nil
}

func DefaultConfig() *Config { return appconfig.DefaultConfig() }

// InstallRoot 返回生产环境安装根目录 (%LOCALAPPDATA%\CZL\CZL Browser)。
func InstallRoot() (string, error) { return appconfig.InstallRoot() }

// ConfigPathIn 返回安装根目录下的主配置文件路径。
func ConfigPathIn(root string) string { return appconfig.ConfigPathIn(root) }

// MigrateLegacyLayout 把旧版 %APPDATA% / %LOCALAPPDATA%\CZL Browser 布局搬迁到安装根目录。
func MigrateLegacyLayout(root string) ([]string, []error) {
	return appconfig.MigrateLegacyLayout(root)
}

func RunTray(cb TrayCallbacks) { apptray.Run(cb) }

func QuitTray() { apptray.Quit() }
