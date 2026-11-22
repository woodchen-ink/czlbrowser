package database

import (
	"path/filepath"
	"testing"
)

func TestMigrateFreshAndRerun(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 2; i++ {
		if err := db.Migrate(); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", i+1, err)
		}
	}
	var version, count int
	if err := db.conn.QueryRow(`SELECT MAX(version), COUNT(*) FROM schema_migrations`).Scan(&version, &count); err != nil {
		t.Fatal(err)
	}
	if version != steps[len(steps)-1].version || count != len(steps) {
		t.Fatalf("version=%d count=%d", version, count)
	}
	for _, col := range []string{"space", "rev", "dirty", "deleted", "group_id", "last_start_at"} {
		var n int
		_ = db.conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('browser_profiles') WHERE name = ?`, col).Scan(&n)
		if n != 1 {
			t.Errorf("browser_profiles 缺少列 %s", col)
		}
	}
	var launchCodes int
	_ = db.conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'launch_codes'`).Scan(&launchCodes)
	if launchCodes != 0 {
		t.Error("launch_codes 应在 v6 删除")
	}
}

func TestMigrateFromOldVersion(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 模拟停在 v7 的旧库: 实例内核等于默认内核, v8 应改回跟随默认
	if _, err := db.conn.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, desc TEXT, applied_at DATETIME)`); err != nil {
		t.Fatal(err)
	}
	for _, s := range steps[:7] {
		if err := db.apply(s); err != nil {
			t.Fatal(err)
		}
	}
	mustExec := func(q string) {
		if _, err := db.conn.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`INSERT INTO browser_cores (core_id, core_name, core_path, is_default) VALUES ('c1', 'C', 'p', 1)`)
	mustExec(`INSERT INTO browser_profiles (profile_id, profile_name, core_id, created_at, updated_at, space) VALUES ('a', 'A', 'c1', 'x', 'x', 'me:1')`)
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	var core string
	var dirty int
	_ = db.conn.QueryRow(`SELECT core_id, dirty FROM browser_profiles WHERE profile_id = 'a'`).Scan(&core, &dirty)
	if core != "" || dirty == 0 {
		t.Fatalf("v8 未生效: core=%q dirty=%d", core, dirty)
	}
}
