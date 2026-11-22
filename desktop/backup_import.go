package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/backup"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

// importIssues 收集导入中各模块的问题; 某模块出错不影响其它模块, 结果里标为「部分成功」。
type importIssues struct {
	names  map[string]string // 模块 id -> 名称 (备份包里实际存在的模块)
	failed map[string]bool
	list   []map[string]string
}

func (ii *importIssues) add(id, name string, err error) {
	if err == nil {
		return
	}
	if name == "" {
		name = ii.names[id]
	}
	if name == "" {
		name = "未知模块"
	}
	if id != "" {
		ii.names[id] = name
		ii.failed[id] = true
	}
	ii.list = append(ii.list, map[string]string{"componentId": id, "componentName": name, "error": err.Error()})
}

// importPackage 校验并导入备份包。各模块依次: 配置 → 旧版 proxies.yaml → 数据库表 → 文件目录。
func (a *App) importPackage(zipPath string, resetFirst bool, report progressFn) (map[string]interface{}, error) {
	a.stopForMaintenance()
	report(step("preparing", 10, "正在解压并校验备份包..."))
	root, manifest, err := extractBackup(zipPath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root)
	report(step("preparing", 20, "备份包校验通过，开始加载数据..."))

	issues := &importIssues{names: map[string]string{}, failed: map[string]bool{}}
	for _, e := range manifest.Entries {
		if p := strings.TrimSuffix(strings.TrimSpace(e.ArchivePath), "/"); e.ID != "" && p != "" && exists(filepath.Join(root, filepath.FromSlash(p))) {
			issues.names[e.ID] = componentLabel(e.Description, e.ID)
		}
	}
	_, hasDB := issues.names["database_sqlite_main"]
	stats := &mergeStats{}

	if resetFirst {
		report(step("preparing", 30, "正在初始化系统数据..."))
		if _, err := a.resetToDefaults(false); err != nil {
			return nil, err
		}
		report(step("preparing", 40, "初始化完成，继续加载备份内容..."))
	}

	payload := filepath.Join(root, "payload")
	report(step("importing", 50, "正在解析备份配置..."))
	incoming, err := loadIncomingConfig(payload)
	if err != nil {
		issues.add("system_config_main", "主配置文件", fmt.Errorf("解析配置失败: %w", err))
	} else if incoming == nil && resetFirst {
		issues.add("system_config_main", "主配置文件", errors.New("备份包缺少 payload/system/config.yaml，已保留默认配置继续加载其余模块"))
	}
	if incoming != nil {
		report(step("importing", 58, "正在应用系统配置..."))
		issues.add("system_config_main", "主配置文件", a.applyIncomingConfig(incoming, resetFirst))
	}

	report(step("importing", 66, "正在合并代理配置..."))
	issues.add("system_config_proxies", "代理配置文件", a.mergeLegacyProxyFile(payload, resetFirst, stats))

	if db := findBackupDB(payload); db != "" {
		report(step("importing", 76, "正在合并数据库数据..."))
		issues.add("database_sqlite_main", "SQLite 主数据库", a.mergeBackupDB(db, resetFirst, stats))
	} else if hasDB {
		issues.add("database_sqlite_main", "SQLite 主数据库", errors.New("备份包缺少数据库文件"))
	}

	report(step("importing", 86, "正在同步文件数据..."))
	a.importFileTrees(payload, incoming, resetFirst, stats, issues)

	report(step("importing", 94, "正在刷新运行时配置..."))
	if err := a.reloadAfterMaintenance(); err != nil {
		return nil, err
	}

	total, failed := len(issues.names), len(issues.failed)
	msg := "加载完成"
	if failed > 0 {
		msg = fmt.Sprintf("加载完成（部分成功）：成功 %d 个模块，异常 %d 个模块", max(total-failed, 0), failed)
	}
	report(step("done", 100, msg))
	if issues.list == nil {
		issues.list = []map[string]string{}
	}
	return map[string]interface{}{
		"cancelled": false, "zipPath": zipPath, "resetFirst": resetFirst,
		"imported": stats.Imported, "skipped": stats.Skipped, "conflicts": stats.Conflicts,
		"partial": failed > 0, "componentTotal": total, "componentSuccess": max(total-failed, 0), "componentFailed": failed,
		"failedComponents": issues.list, "message": msg,
	}, nil
}

// extractBackup 解压到临时目录并校验 manifest。
func extractBackup(zipPath string) (string, backup.Manifest, error) {
	var m backup.Manifest
	dir, err := os.MkdirTemp("", "czl-browser-import-*")
	if err != nil {
		return "", m, err
	}
	fail := func(e error) (string, backup.Manifest, error) {
		os.RemoveAll(dir)
		return "", m, e
	}
	if err := unzipTo(zipPath, dir); err != nil {
		return fail(fmt.Errorf("解压备份包失败: %w", err))
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return fail(errors.New("备份包缺少 manifest.json"))
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return fail(fmt.Errorf("manifest.json 解析失败: %w", err))
	}
	switch {
	case m.Format != backup.PackageFormat:
		return fail(fmt.Errorf("不支持的备份格式: %s", m.Format))
	case m.ManifestVersion != backup.ManifestVersion:
		return fail(fmt.Errorf("不支持的 manifest 版本: %d", m.ManifestVersion))
	case !exists(filepath.Join(dir, "payload")):
		return fail(errors.New("备份包缺少 payload 目录"))
	}
	return dir, m, nil
}

// loadIncomingConfig 读备份包里的配置; 没有时返回 nil, nil。
func loadIncomingConfig(payload string) (*config.Config, error) {
	p := filepath.Join(payload, "system", "config.yaml")
	if !exists(p) {
		return nil, nil
	}
	return config.Load(p)
}

// applyIncomingConfig 全量导入时直接用备份的配置, 合并导入时只合并列表; 数据库位置始终用本机的。
func (a *App) applyIncomingConfig(in *config.Config, replace bool) error {
	cur := a.config
	if cur == nil {
		cur = config.DefaultConfig()
	}
	next := mergeConfig(cur, in)
	if replace {
		cp := *in
		next = &cp
	}
	next.Database = cur.Database
	if err := next.Save(a.mainConfigPath()); err != nil {
		return fmt.Errorf("保存导入配置失败: %w", err)
	}
	a.config = next
	a.applyRuntimeConfig(next.Runtime)
	return nil
}

// mergeConfig 以本机配置为准, 把备份里的书签、内核、代理、实例 (旧版写在配置里的) 中本机没有的追加进来。
func mergeConfig(cur, in *config.Config) *config.Config {
	out := *cur
	if strings.TrimSpace(out.App.Name) == "" {
		out.App.Name = in.App.Name
	}
	b, ib := &out.Browser, in.Browser
	b.DefaultBookmarks = unionBy(b.DefaultBookmarks, ib.DefaultBookmarks, func(x config.BrowserBookmark) []string { return []string{x.URL} })
	b.Cores = unionBy(b.Cores, ib.Cores, func(x config.BrowserCore) []string { return []string{x.CoreId, x.CorePath} })
	b.Proxies = unionBy(b.Proxies, ib.Proxies, func(x config.BrowserProxy) []string { return []string{x.ProxyId, x.ProxyConfig} })
	b.Profiles = unionBy(b.Profiles, ib.Profiles, func(x config.BrowserProfileConfig) []string { return []string{x.ProfileId, x.UserDataDir} })
	return &out
}

// unionBy 依次保留 a、b 中的元素, 任一键 (去空白、不区分大小写) 已出现过就跳过; 全部键为空的元素丢弃。
func unionBy[T any](a, b []T, keys func(T) []string) []T {
	seen := map[int]map[string]bool{}
	out := make([]T, 0, len(a)+len(b))
	for _, item := range append(append([]T{}, a...), b...) {
		ks := keys(item)
		dup, empty := false, true
		for i, k := range ks {
			k = strings.ToLower(strings.TrimSpace(k))
			ks[i] = k
			if k != "" {
				empty = false
				dup = dup || seen[i][k]
			}
		}
		if empty || dup {
			continue
		}
		for i, k := range ks {
			if k == "" {
				continue
			}
			if seen[i] == nil {
				seen[i] = map[string]bool{}
			}
			seen[i][k] = true
		}
		out = append(out, item)
	}
	return out
}

// mergeLegacyProxyFile 处理早期版本备份里的 proxies.yaml: 全量导入时覆盖, 合并时按 id 与地址去重追加。
func (a *App) mergeLegacyProxyFile(payload string, replace bool, stats *mergeStats) error {
	src := filepath.Join(payload, "system", "proxies.yaml")
	dst := a.resolveAppPath("proxies.yaml")
	if !exists(src) {
		if replace {
			_ = os.Remove(dst)
		}
		return nil
	}
	if replace {
		return copyFileAtomic(src, dst)
	}
	incoming, err := config.LoadProxies(src)
	if err != nil {
		return err
	}
	current, err := config.LoadProxies(dst)
	if err != nil {
		return err
	}
	merged := unionBy(current, incoming, func(x config.BrowserProxy) []string { return []string{x.ProxyId, x.ProxyConfig} })
	stats.Imported += len(merged) - len(current)
	stats.Skipped += len(current) + len(incoming) - len(merged)
	return config.SaveProxies(dst, merged)
}

func findBackupDB(payload string) string {
	for _, p := range []string{filepath.Join(payload, "app", "database", "app.db"), filepath.Join(payload, "app", "data", "app.db")} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// tableMerge 描述备份数据库里一张表怎么并进本机数据库 (都落在本机空间)。
// cols 是目标列, from 是对应的来源表达式 (s 为来源表别名, {TAGS} 按来源有没有 tags 列替换),
// dup 是合并导入时判定「本机已有」的条件 (t 为本机表别名)。
type tableMerge struct {
	table string
	cols  string
	from  string
	dup   string
}

var backupTables = []tableMerge{
	{"browser_groups", "group_id, group_name, parent_id, sort_order, created_at, updated_at",
		"s.group_id, s.group_name, '', s.sort_order, s.created_at, s.updated_at",
		"t.group_id = s.group_id OR lower(t.group_name) = lower(s.group_name)"},
	{"browser_cores", "core_id, core_name, core_path, is_default, sort_order, created_at, tags",
		"s.core_id, s.core_name, s.core_path, s.is_default, s.sort_order, s.created_at, {TAGS}",
		"t.core_id = s.core_id OR lower(t.core_path) = lower(s.core_path)"},
	{"browser_proxies", "proxy_id, proxy_name, proxy_config, dns_servers, group_name, source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at, tags",
		"s.proxy_id, s.proxy_name, s.proxy_config, s.dns_servers, COALESCE(s.group_name,''), COALESCE(s.source_id,''), COALESCE(s.source_url,''), COALESCE(s.source_name_prefix,''), COALESCE(s.source_auto_refresh,0), COALESCE(s.source_refresh_interval_m,0), COALESCE(s.source_last_refresh_at,''), COALESCE(s.last_latency_ms,-1), COALESCE(s.last_test_ok,0), COALESCE(s.last_tested_at,''), COALESCE(s.last_ip_health_json,''), s.sort_order, s.created_at, {TAGS}",
		"t.proxy_id = s.proxy_id OR lower(t.proxy_config) = lower(s.proxy_config)"},
	{"browser_profiles", "profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at",
		"s.profile_id, s.profile_name, s.user_data_dir, s.core_id, s.fingerprint_args, s.proxy_id, s.proxy_config, s.launch_args, s.tags, s.keywords, COALESCE(s.group_id,''), s.created_at, s.updated_at",
		"t.profile_id = s.profile_id OR lower(t.user_data_dir) = lower(s.user_data_dir)"},
	{"browser_bookmarks", "name, url, sort_order, tags", "s.name, s.url, s.sort_order, {TAGS}",
		"lower(t.url) = lower(s.url)"},
}

// mergeBackupDB 挂载备份数据库, 在一个事务里逐表导入。合并导入时跳过本机已有的行。
func (a *App) mergeBackupDB(src string, replace bool, stats *mergeStats) error {
	if a.db == nil || a.db.GetConn() == nil {
		return errors.New("数据库未初始化")
	}
	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ATTACH DATABASE ? AS src`, src); err != nil {
		return fmt.Errorf("挂载备份数据库失败: %w", err)
	}
	defer tx.Exec(`DETACH DATABASE src`)

	for _, m := range backupTables {
		cols, err := srcColumns(tx, m.table)
		if err != nil {
			return err
		}
		if len(cols) == 0 {
			continue // 旧备份没有这张表
		}
		var total int
		if err := tx.QueryRow(`SELECT COUNT(1) FROM src.` + m.table).Scan(&total); err != nil {
			return err
		}
		if total == 0 {
			continue
		}
		tags := "'[]'" // 早期备份没有 tags 列
		if cols["tags"] {
			tags = "COALESCE(s.tags, '[]')"
		}
		q := fmt.Sprintf(`INSERT INTO %s (%s) SELECT %s FROM src.%s s`, m.table, m.cols, strings.ReplaceAll(m.from, "{TAGS}", tags), m.table)
		if !replace {
			q += fmt.Sprintf(` WHERE NOT EXISTS (SELECT 1 FROM %s t WHERE %s)`, m.table, m.dup)
		}
		res, err := tx.Exec(q)
		if err != nil {
			return fmt.Errorf("导入数据表失败(%s): %w", m.table, err)
		}
		n, _ := res.RowsAffected()
		stats.Imported += int(n)
		if !replace {
			stats.Skipped += total - int(n)
		}
	}
	return tx.Commit()
}

// srcColumns 返回备份库里表的列名集合; 表不存在时为空。
func srcColumns(tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.Query(`SELECT lower(name) FROM pragma_table_info(?, 'src')`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols[c] = true
	}
	return cols, rows.Err()
}

// importFileTrees 还原数据目录、实例数据目录, 以及旧版备份里的内核目录。
// 全量导入先清空目标 (数据库文件除外) 再覆盖, 合并导入不覆盖已有文件。
func (a *App) importFileTrees(payload string, incoming *config.Config, replace bool, stats *mergeStats, issues *importIssues) {
	// 清空目标时始终保留正在使用的数据库文件 (实例数据根可能就是 data 目录)
	keep := dbFiles(a.dbPathOf(a.config))
	restore := func(id, name, src, dst string, skip func(string) bool) {
		if !exists(src) {
			return
		}
		if replace {
			if err := clearDirExcept(dst, keep); err != nil {
				issues.add(id, name, err)
				return
			}
		}
		issues.add(id, name, syncTree(src, dst, replace, stats, skip))
	}

	dataSrc := filepath.Join(payload, "app", "data")
	restore("app_data_root", "应用数据目录（含数据库、快照及默认浏览器数据）", dataSrc, a.resolveAppPath("data"),
		anySkip(skipDBFiles, skipSyncDir, backup.NewProfileAwareSkip(backup.DetectProfileRoots(dataSrc))))

	userSrc := filepath.Join(payload, "browser", "user-data")
	restore("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", userSrc, a.userDataRootOf(a.config),
		backup.NewProfileAwareSkip(backup.DetectProfileRoots(userSrc)))

	restore("browser_core_root", "默认内核目录", filepath.Join(payload, "browser", "cores", "chrome"), a.resolveAppPath("chrome"), nil)

	// 旧版备份: 配置里指向 chrome\ 之外的内核, 按名称排序后与配置里的内核路径一一对应
	extRoot := filepath.Join(payload, "browser", "cores", "external")
	entries, err := os.ReadDir(extRoot)
	if err != nil {
		if exists(extRoot) {
			issues.add("browser_core_external", "额外内核目录（来自配置 cores）", err)
		}
		return
	}
	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	sort.Strings(folders)
	targets := a.externalCorePaths(incoming)
	for i, f := range folders {
		id := "browser_core_external_" + f
		switch {
		case incoming == nil:
			issues.add(id, "额外内核目录（来自配置 cores）", errors.New("缺少可用配置，无法映射目标路径"))
		case i >= len(targets):
			stats.Skipped++
			issues.add(id, "额外内核目录（来自配置 cores）", errors.New("目标配置缺失，无法导入该外部内核目录"))
		default:
			restore(id, "额外内核目录（来自配置 cores）", filepath.Join(extRoot, f), targets[i], nil)
		}
	}
}

// externalCorePaths 返回配置里位于 chrome\ 之外的内核目录 (去重排序)。
func (a *App) externalCorePaths(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	chrome := a.resolveAppPath("chrome")
	seen := map[string]bool{}
	var out []string
	for _, c := range cfg.Browser.Cores {
		if strings.TrimSpace(c.CorePath) == "" {
			continue
		}
		p := a.resolveAppPath(strings.TrimSpace(c.CorePath))
		if pathInside(p, chrome) || seen[foldPath(p)] {
			continue
		}
		seen[foldPath(p)] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
