package backup

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

func entryIDs(t *testing.T, opts BuildOptions) map[string]ScopeEntry {
	t.Helper()
	s, err := BuildScope(opts)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format != PackageFormat || s.ManifestVersion != ManifestVersion {
		t.Fatalf("format = %s v%d", s.Format, s.ManifestVersion)
	}
	out := map[string]ScopeEntry{}
	for i, e := range s.Entries {
		out[e.ID] = e
		if i > 0 && s.Entries[i-1].ID > e.ID {
			t.Fatal("条目应按 id 排序")
		}
	}
	return out
}

func TestDefaultScopeDedupesInsideData(t *testing.T) {
	got := entryIDs(t, BuildOptions{AppRoot: t.TempDir(), Config: config.DefaultConfig()})
	want := []string{"app_data_root", "system_config_main", "system_config_proxies"}
	if len(got) != len(want) {
		t.Fatalf("默认配置下数据库、实例数据与日志都在 data 里, 不应单列: %v", got)
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("缺少 %s", id)
		}
	}
	if !got["system_config_main"].Required || got["system_config_proxies"].Required {
		t.Error("必选标记不对")
	}
}

func TestCustomPathsListedSeparately(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = "profiles"
	cfg.Database.SQLite.Path = "db/main.db"
	cfg.Logging.FilePath = "runtime/logs/app.log"
	got := entryIDs(t, BuildOptions{AppRoot: root, Config: cfg, ConfigPath: filepath.Join(root, "config", "config.yaml")})
	for _, id := range []string{"browser_user_data_root", "database_sqlite_main", "database_sqlite_wal", "database_sqlite_shm", "logs_root"} {
		if _, ok := got[id]; !ok {
			t.Errorf("缺少 %s", id)
		}
	}
	if got["database_sqlite_main"].SourcePath != filepath.Join(root, "db", "main.db") {
		t.Errorf("数据库路径 = %s", got["database_sqlite_main"].SourcePath)
	}
	if got["logs_root"].SourcePath != filepath.Join(root, "runtime", "logs") {
		t.Errorf("日志目录 = %s", got["logs_root"].SourcePath)
	}
	if got["system_config_main"].SourcePath != filepath.Join(root, "config", "config.yaml") {
		t.Errorf("配置文件路径应使用 ConfigPath")
	}
}

func TestManifestHasNoLocalPaths(t *testing.T) {
	s, _ := BuildScope(BuildOptions{AppRoot: t.TempDir()})
	m := BuildManifest(s, "", "", time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC))
	if m.CreatedAt != "2026-03-02T12:00:00Z" || m.App.Name != "CZL Browser" || m.App.Version != "1.0.0" {
		t.Fatalf("manifest = %+v", m)
	}
	if len(m.Entries) != len(s.Entries) {
		t.Fatal("条目数不一致")
	}
}

func TestInside(t *testing.T) {
	if !inside(`C:\a\b\c.txt`, `C:\a\b`) || inside(`C:\a\bc`, `C:\a\b`) || !inside(`C:\a`, `C:\a`) {
		t.Fatal("inside 判断错误")
	}
}
