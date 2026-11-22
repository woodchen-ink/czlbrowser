package main

import (
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"path/filepath"
	"testing"
)

func TestReloadConfigLoadsFromDisk(t *testing.T) {
	root := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.App.Name = "Reload-Test-App"
	if err := cfg.Save(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	app := NewApp(root)
	app.config = config.DefaultConfig()

	if err := app.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig 失败: %v", err)
	}

	if app.config == nil {
		t.Fatalf("ReloadConfig 后 config 为空")
	}
	if app.config.App.Name != "Reload-Test-App" {
		t.Fatalf("ReloadConfig 未生效，got=%q", app.config.App.Name)
	}
}
