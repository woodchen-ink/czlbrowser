package database

// step 是一次表结构变更。已发布的版本只能追加, 不能修改 (用户数据库已经执行过)。
type step struct {
	version int
	desc    string
	sql     []string
}

// 同步相关列的含义: space 为数据空间 (local / me:<用户> / org:<组织>);
// rev 为服务端序号; dirty 为待上传的出站序号, 0 表示已同步; deleted 为墓碑标记。
var steps = []step{
	{1, "初始化核心表结构", []string{
		`CREATE TABLE IF NOT EXISTS launch_codes (profile_id TEXT PRIMARY KEY, code TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_launch_codes_code ON launch_codes(code)`,
		`CREATE TABLE IF NOT EXISTS browser_profiles (profile_id TEXT PRIMARY KEY, profile_name TEXT NOT NULL,
			user_data_dir TEXT NOT NULL DEFAULT '', core_id TEXT NOT NULL DEFAULT '', fingerprint_args TEXT NOT NULL DEFAULT '[]',
			proxy_id TEXT NOT NULL DEFAULT '', proxy_config TEXT NOT NULL DEFAULT '', launch_args TEXT NOT NULL DEFAULT '[]',
			tags TEXT NOT NULL DEFAULT '[]', keywords TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_browser_profiles_created_at ON browser_profiles(created_at)`,
		`CREATE TABLE IF NOT EXISTS browser_proxies (proxy_id TEXT PRIMARY KEY, proxy_name TEXT NOT NULL, proxy_config TEXT NOT NULL,
			dns_servers TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS browser_cores (core_id TEXT PRIMARY KEY, core_name TEXT NOT NULL, core_path TEXT NOT NULL,
			is_default INTEGER NOT NULL DEFAULT 0, sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS browser_bookmarks (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE, sort_order INTEGER NOT NULL DEFAULT 0)`,
	}},
	{2, "添加实例分组支持", []string{
		`CREATE TABLE IF NOT EXISTS browser_groups (group_id TEXT PRIMARY KEY, group_name TEXT NOT NULL, parent_id TEXT DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE INDEX IF NOT EXISTS idx_browser_groups_parent_id ON browser_groups(parent_id)`,
		`ALTER TABLE browser_profiles ADD COLUMN group_id TEXT DEFAULT ''`,
	}},
	{3, "代理表添加分组和测速字段", addColumns("browser_proxies",
		"group_name TEXT NOT NULL DEFAULT ''",
		"last_latency_ms INTEGER NOT NULL DEFAULT -1",
		"last_test_ok INTEGER NOT NULL DEFAULT 0",
		"last_tested_at TEXT NOT NULL DEFAULT ''",
	)},
	{4, "代理表添加 IP 健康结果字段", addColumns("browser_proxies", "last_ip_health_json TEXT NOT NULL DEFAULT ''")},
	{5, "代理表添加 URL 来源与自动刷新字段", addColumns("browser_proxies",
		"source_id TEXT NOT NULL DEFAULT ''",
		"source_url TEXT NOT NULL DEFAULT ''",
		"source_name_prefix TEXT NOT NULL DEFAULT ''",
		"source_auto_refresh INTEGER NOT NULL DEFAULT 0",
		"source_refresh_interval_m INTEGER NOT NULL DEFAULT 0",
		"source_last_refresh_at TEXT NOT NULL DEFAULT ''",
	)},
	{6, "移除 LaunchCode 自动化能力", []string{
		`DROP INDEX IF EXISTS idx_launch_codes_code`,
		`DROP TABLE IF EXISTS launch_codes`,
	}},
	{7, "数据空间与云端同步", concat(
		syncColumns("browser_profiles", "space, deleted"),
		syncColumns("browser_proxies", "space, deleted, sort_order"),
		syncColumns("browser_groups", "space, deleted"),
		[]string{
			// 同步空间的设置 (目前只有默认书签); 本机空间的书签仍在 browser_bookmarks
			`CREATE TABLE IF NOT EXISTS space_settings (space TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL DEFAULT '',
				rev INTEGER NOT NULL DEFAULT 0, dirty INTEGER NOT NULL DEFAULT 0, deleted INTEGER NOT NULL DEFAULT 0,
				updated_at TEXT NOT NULL DEFAULT '', PRIMARY KEY (space, key))`,
			`CREATE INDEX IF NOT EXISTS idx_space_settings_dirty ON space_settings(space, dirty)`,
			// 实例登录态: payload 为最近已知的清单, pack_path 为待上传的本地包, applied_rev 为本机目录对应的版本
			`CREATE TABLE IF NOT EXISTS profile_states (profile_id TEXT PRIMARY KEY, space TEXT NOT NULL,
				rev INTEGER NOT NULL DEFAULT 0, dirty INTEGER NOT NULL DEFAULT 0, deleted INTEGER NOT NULL DEFAULT 0,
				payload TEXT NOT NULL DEFAULT '', pack_path TEXT NOT NULL DEFAULT '', applied_rev INTEGER NOT NULL DEFAULT 0,
				updated_at TEXT NOT NULL DEFAULT '')`,
			`CREATE INDEX IF NOT EXISTS idx_profile_states_dirty ON profile_states(space, dirty)`,
			`CREATE TABLE IF NOT EXISTS sync_counter (id INTEGER PRIMARY KEY CHECK (id = 1), value INTEGER NOT NULL DEFAULT 0)`,
			`CREATE TABLE IF NOT EXISTS sync_spaces (space TEXT PRIMARY KEY, cursor INTEGER NOT NULL DEFAULT 0,
				wire INTEGER NOT NULL DEFAULT 0, synced_at TEXT NOT NULL DEFAULT '')`,
			`CREATE TABLE IF NOT EXISTS app_kv (key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '')`,
		},
	)},
	{8, "实例内核等于默认内核的改回跟随默认", concat(bumpCounter(), []string{
		// 旧版把空 coreId 固化成当时的默认内核, 之后切换默认不再生效
		`UPDATE browser_profiles SET core_id = '', dirty = ` + dirtyIfSynced + `
			WHERE deleted = 0 AND core_id != '' AND core_id IN (SELECT core_id FROM browser_cores WHERE is_default = 1)`,
	})},
	{9, "统一分组与标签: 代理 / 内核 / 书签加标签, 实例分组改单层", concat(
		[]string{
			`ALTER TABLE browser_proxies ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'`,
			`ALTER TABLE browser_cores ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'`,
			`ALTER TABLE browser_bookmarks ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'`,
		},
		bumpCounter(),
		// 多级分组压平: 子分组提到根级, 实例归属不变
		[]string{`UPDATE browser_groups SET parent_id = '', dirty = ` + dirtyIfSynced + ` WHERE COALESCE(parent_id, '') != ''`},
	)},
	{10, "本机实例指向不存在的内核时改回跟随默认", []string{
		// 同步空间的内核 id 可能来自别的电脑, 不动
		`UPDATE browser_profiles SET core_id = ''
			WHERE space = 'local' AND deleted = 0 AND core_id != '' AND core_id NOT IN (SELECT core_id FROM browser_cores)`,
	}},
	{11, "实例记录最近启动时间 (本机字段, 不同步)", addColumns("browser_profiles", "last_start_at TEXT NOT NULL DEFAULT ''")},
}

// dirtyIfSynced 在数据迁移里给同步空间的行打上新的出站序号, 让改动上传; 本机空间不变。
const dirtyIfSynced = `CASE WHEN space != 'local' THEN (SELECT value FROM sync_counter WHERE id = 1) ELSE dirty END`

func bumpCounter() []string {
	return []string{
		`INSERT OR IGNORE INTO sync_counter (id, value) VALUES (1, 0)`,
		`UPDATE sync_counter SET value = value + 1 WHERE id = 1`,
	}
}

func addColumns(table string, defs ...string) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = "ALTER TABLE " + table + " ADD COLUMN " + d
	}
	return out
}

// syncColumns 给表加 space / rev / dirty / deleted 四列与两个索引。
func syncColumns(table, listIndex string) []string {
	return append(addColumns(table,
		"space TEXT NOT NULL DEFAULT 'local'",
		"rev INTEGER NOT NULL DEFAULT 0",
		"dirty INTEGER NOT NULL DEFAULT 0",
		"deleted INTEGER NOT NULL DEFAULT 0",
	),
		"CREATE INDEX IF NOT EXISTS idx_"+table+"_space ON "+table+"("+listIndex+")",
		"CREATE INDEX IF NOT EXISTS idx_"+table+"_dirty ON "+table+"(space, dirty)",
	)
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
