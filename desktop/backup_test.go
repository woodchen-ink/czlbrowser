package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

func noProgress(backupProgress) {}

// newTestApp 在临时目录里准备一个带数据库的 App (不启动界面与同步)。
func newTestApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	a := NewApp(root)
	a.config = config.DefaultConfig()
	if err := a.config.Save(a.mainConfigPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.resolveAppPath("data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.openDatabase(); err != nil {
		t.Fatal(err)
	}
	a.migrateToSQLite()
	a.browserMgr.InitData()
	t.Cleanup(func() {
		if a.speedScheduler != nil {
			a.speedScheduler.Stop()
		}
		a.db.Close()
	})
	return a
}

func TestUnionBy(t *testing.T) {
	a := []config.BrowserProxy{{ProxyId: "p1", ProxyConfig: "http://a:1"}}
	b := []config.BrowserProxy{
		{ProxyId: "P1", ProxyConfig: "http://other:1"}, // id 重复
		{ProxyId: "p2", ProxyConfig: "HTTP://A:1"},     // 地址重复
		{ProxyId: "p3", ProxyConfig: "socks5://b:2"},
		{},
	}
	got := unionBy(a, b, func(x config.BrowserProxy) []string { return []string{x.ProxyId, x.ProxyConfig} })
	if len(got) != 2 || got[1].ProxyId != "p3" {
		t.Fatalf("unionBy = %+v", got)
	}
}

func TestSyncTreeMergeAndOverwrite(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	write := func(dir, name, body string) {
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write(src, "same.txt", "x")
	write(dst, "same.txt", "x")
	write(src, "diff.txt", "new")
	write(dst, "diff.txt", "old")
	write(src, "sub/new.txt", "n")
	write(src, "sync/key", "secret")

	stats := &mergeStats{}
	if err := syncTree(src, dst, false, stats, skipSyncDir); err != nil {
		t.Fatal(err)
	}
	if stats.Imported != 1 || stats.Skipped != 1 || stats.Conflicts != 1 {
		t.Fatalf("合并统计 = %+v", stats)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "diff.txt")); string(b) != "old" {
		t.Fatal("合并导入不应覆盖本机文件")
	}
	if exists(filepath.Join(dst, "sync", "key")) {
		t.Fatal("data/sync 不应导入")
	}
	if err := syncTree(src, dst, true, &mergeStats{}, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "diff.txt")); string(b) != "new" {
		t.Fatal("全量导入应覆盖")
	}
}

func TestBackupRoundTrip(t *testing.T) {
	src := newTestApp(t)
	created, err := src.browserMgr.Create(browser.ProfileInput{ProfileName: "店铺 A", Tags: []string{"t"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.SaveBrowserProxies([]BrowserProxy{{ProxyName: "美国", ProxyConfig: "socks5://u:p@1.2.3.4:1080"}}); err != nil {
		t.Fatal(err)
	}
	// 实例数据里一份 Cookie (账号数据) 与一份缓存 (应被过滤)
	ud := src.browserMgr.ResolveUserDataDir(created)
	// 默认实例的数据目录 data\default 也存在 (此前它会让整个 data 被误判成 profile 根)
	os.MkdirAll(filepath.Join(src.resolveAppPath("data"), "default", "Default"), 0o755)
	os.MkdirAll(filepath.Join(ud, "Default", "Cache"), 0o755)
	os.WriteFile(filepath.Join(ud, "Default", "Cookies"), []byte("c"), 0o644)
	os.WriteFile(filepath.Join(ud, "Default", "Cache", "blob"), []byte("cache"), 0o644)

	zipPath := filepath.Join(t.TempDir(), "b.zip")
	res, err := src.exportPackage(zipPath, noProgress)
	if err != nil || res.files == 0 {
		t.Fatalf("导出失败: %+v %v", res, err)
	}

	for _, replace := range []bool{false, true} {
		dst := newTestApp(t)
		out, err := dst.importPackage(zipPath, replace, noProgress)
		if err != nil {
			t.Fatalf("replace=%v 导入失败: %v", replace, err)
		}
		if out["partial"] != false {
			t.Fatalf("replace=%v 不应部分失败: %+v", replace, out["failedComponents"])
		}
		names := map[string]bool{}
		for _, q := range dst.browserMgr.List() {
			names[q.ProfileName] = true
		}
		if !names["店铺 A"] {
			t.Fatalf("replace=%v 实例未导入: %v", replace, names)
		}
		found := false
		for _, q := range dst.getLatestProxies() {
			found = found || q.ProxyConfig == "socks5://u:p@1.2.3.4:1080"
		}
		if !found {
			t.Fatalf("replace=%v 代理未导入", replace)
		}
		got := dst.browserMgr.List()
		var imported BrowserProfile
		for _, q := range got {
			if q.ProfileName == "店铺 A" {
				imported = q
			}
		}
		dir := dst.browserMgr.ResolveUserDataDir(&imported)
		if !exists(filepath.Join(dir, "Default", "Cookies")) {
			t.Fatalf("replace=%v Cookie 未还原", replace)
		}
		if exists(filepath.Join(dir, "Default", "Cache", "blob")) {
			t.Fatalf("replace=%v 缓存不应进入备份", replace)
		}
	}
}

func TestResetToDefaultsKeepsDatabase(t *testing.T) {
	a := newTestApp(t)
	os.WriteFile(a.resolveAppPath("data/junk.txt"), []byte("x"), 0o644)
	if _, err := a.resetToDefaults(true); err != nil {
		t.Fatal(err)
	}
	if exists(a.resolveAppPath("data/junk.txt")) {
		t.Fatal("数据目录应被清空")
	}
	if !exists(a.dbPathOf(a.config)) {
		t.Fatal("数据库文件应保留")
	}
	if list := a.browserMgr.List(); len(list) != 1 || list[0].ProfileName != "默认实例" {
		t.Fatalf("初始化后应只有默认实例: %+v", list)
	}
}
