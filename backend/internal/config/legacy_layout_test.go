package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallRootRequiresLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	if _, err := InstallRoot(); err == nil {
		t.Fatal("LOCALAPPDATA 为空时应返回错误")
	}

	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	root, err := InstallRoot()
	if err != nil {
		t.Fatalf("InstallRoot: %v", err)
	}
	if want := filepath.Join(base, "CZL", "CZL Browser"); root != want {
		t.Fatalf("InstallRoot = %s, want %s", root, want)
	}
}

func TestMigrateLegacyLayout(t *testing.T) {
	appData := t.TempDir()
	localAppData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", localAppData)

	writeFile(t, filepath.Join(appData, "CZL Browser", "config.yaml"), "legacy-config")
	writeFile(t, filepath.Join(localAppData, "CZL Browser", "data", "app.db"), "legacy-db")
	writeFile(t, filepath.Join(localAppData, "CZL Browser", "chrome", "chrome.exe"), "legacy-core")
	writeFile(t, filepath.Join(localAppData, "CZL Browser", "logs", "app.log"), "legacy-log")

	root, err := InstallRoot()
	if err != nil {
		t.Fatalf("InstallRoot: %v", err)
	}
	// 新布局已存在的文件不被覆盖；已存在的目录（安装器预建）逐项合并
	writeFile(t, filepath.Join(root, "logs", "app.log"), "new-log")
	if err := os.MkdirAll(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatal(err)
	}

	moved, errs := MigrateLegacyLayout(root)
	if len(errs) != 0 {
		t.Fatalf("迁移出错: %v", errs)
	}
	if len(moved) != 3 {
		t.Fatalf("moved = %v, want 3 entries", moved)
	}

	assertContent(t, ConfigPathIn(root), "legacy-config")
	assertContent(t, filepath.Join(root, "data", "app.db"), "legacy-db")
	assertContent(t, filepath.Join(root, "chrome", "chrome.exe"), "legacy-core")
	assertContent(t, filepath.Join(root, "logs", "app.log"), "new-log")

	if _, err := os.Stat(filepath.Join(appData, "CZL Browser")); !os.IsNotExist(err) {
		t.Fatalf("旧配置目录应已清理, err=%v", err)
	}
	// 冲突条目保留在旧目录，旧目录因此非空而不删除
	assertContent(t, filepath.Join(localAppData, "CZL Browser", "logs", "app.log"), "legacy-log")

	// 再次执行幂等
	moved, errs = MigrateLegacyLayout(root)
	if len(errs) != 0 || len(moved) != 0 {
		t.Fatalf("二次迁移应无操作: moved=%v errs=%v", moved, errs)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
