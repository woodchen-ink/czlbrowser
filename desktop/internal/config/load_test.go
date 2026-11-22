package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultConfigValues(t *testing.T) {
	d := DefaultConfig()
	if d.Database.Type != "sqlite" || d.Database.SQLite.Path != "data/app.db" {
		t.Fatalf("database = %+v", d.Database)
	}
	if d.App.Window != (WindowConfig{1280, 800, 1024, 640}) {
		t.Fatalf("window = %+v", d.App.Window)
	}
	if d.Logging.FilePath != "data/logs/app.log" || d.Logging.Rotation.MaxBackups != 5 {
		t.Fatalf("logging = %+v", d.Logging)
	}
	if len(d.Browser.DefaultLaunchArgs) != 2 || d.Browser.UserDataRoot != "data" {
		t.Fatalf("browser = %+v", d.Browser)
	}
}

func TestMissingFileGivesDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.yaml"))
	if err != nil || !reflect.DeepEqual(c, DefaultConfig()) {
		t.Fatalf("Load(missing) = %+v, %v", c, err)
	}
}

func TestEmptySectionsAreFilled(t *testing.T) {
	c, err := Load(writeConfig(t, "app:\n  used_cd_keys: [X]\nlogging: {}\nbrowser: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.App.Name != "CZL Browser" || c.Runtime.MaxMemoryMB != 1024 || c.Logging.Level != "info" {
		t.Fatalf("未补齐: %+v", c)
	}
	if c.Browser.Cores == nil || c.Browser.Proxies == nil || c.Browser.Profiles == nil || c.Browser.DefaultBookmarks == nil {
		t.Fatal("列表应为空切片而不是 nil")
	}
}

func TestExplicitValuesKept(t *testing.T) {
	c, err := Load(writeConfig(t, `
app:
  name: Custom
  window: {width: 1400, height: 900, min_width: 900, min_height: 600}
database:
  sqlite: {path: custom/app.db}
runtime: {max_memory_mb: 2048, gc_percent: 80}
logging: {level: debug, file_enabled: true, file_path: custom/x.log}
browser:
  user_data_root: profiles
  default_fingerprint_args: [--fingerprint-platform=mac]
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.App.Name != "Custom" || c.App.Window.Width != 1400 || c.Database.SQLite.Path != "custom/app.db" {
		t.Fatalf("app/db 被覆盖: %+v %+v", c.App, c.Database)
	}
	if c.Runtime.GCPercent != 80 || c.Logging.Level != "debug" || !c.Logging.FileEnabled || c.Logging.FilePath != "custom/x.log" {
		t.Fatalf("runtime/logging 被覆盖: %+v %+v", c.Runtime, c.Logging)
	}
	if c.Browser.UserDataRoot != "profiles" || c.Browser.DefaultFingerprintArgs[0] != "--fingerprint-platform=mac" {
		t.Fatalf("browser 被覆盖: %+v", c.Browser)
	}
}

func TestLegacyDefaultsReplaced(t *testing.T) {
	c, err := Load(writeConfig(t, `
app:
  window: {width: 1750, height: 1000, min_width: 1200, min_height: 700}
logging: {file_path: logs/app.log}
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.App.Window != (WindowConfig{1280, 800, 1024, 640}) {
		t.Fatalf("旧窗口尺寸未替换: %+v", c.App.Window)
	}
	if c.Logging.FilePath != "data/logs/app.log" {
		t.Fatalf("旧日志路径未迁移: %s", c.Logging.FilePath)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	c := DefaultConfig()
	c.Browser.Proxies = []BrowserProxy{{ProxyId: "p", ProxyName: "n", ProxyConfig: "socks5://1.1.1.1:1", LastLatencyMs: 9}}
	if err := c.Save(p); err != nil {
		t.Fatal(err)
	}
	back, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if back.Browser.Proxies[0].ProxyConfig != "socks5://1.1.1.1:1" || back.Browser.Proxies[0].LastLatencyMs != 0 {
		t.Fatalf("测速结果不应写入 yaml: %+v", back.Browser.Proxies[0])
	}
}

func TestLegacyProxyFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "proxies.yaml")
	if got, err := LoadProxies(p); got != nil || err != nil {
		t.Fatalf("缺失文件应返回 nil: %v %v", got, err)
	}
	in := []BrowserProxy{{ProxyId: "a", ProxyConfig: "http://h:1", Tags: []string{"t"}}}
	if err := SaveProxies(p, in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProxies(p)
	if err != nil || len(got) != 1 || got[0].ProxyId != "a" || got[0].Tags[0] != "t" {
		t.Fatalf("LoadProxies = %+v %v", got, err)
	}
}
