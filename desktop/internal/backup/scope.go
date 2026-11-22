package backup

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

// BuildOptions 指定导出范围的计算依据。
type BuildOptions struct {
	DataRoot   string // 可写数据根 (正式安装为 %LOCALAPPDATA%\CZL\CZL Browser)
	AppRoot    string // DataRoot 为空时使用 (旧调用方式与测试)
	ConfigPath string // config.yaml 的实际位置; 为空时取数据根下的 config.yaml
	Config     *config.Config
}

// BuildScope 列出要导出的条目。已被前面目录包含的路径不再单列, 避免同一文件导出两次。
// 内核目录不导出: 单个内核就有数百 MB, 而内核的路径引用已随配置与数据库导出,
// 换机后放回同名内核即可; 导入仍兼容旧备份包里的内核目录。
func BuildScope(opts BuildOptions) (Scope, error) {
	root := strings.TrimSpace(opts.DataRoot)
	if root == "" {
		root = strings.TrimSpace(opts.AppRoot)
	}
	if root == "" {
		return Scope{}, errors.New("data root 不能为空")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Scope{}, err
	}
	cfg := opts.Config
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	at := func(p string) string { return under(root, p) }
	configFile := strings.TrimSpace(opts.ConfigPath)
	if configFile == "" {
		configFile = at("config.yaml")
	}

	var s scopeSet
	s.add("system_config_main", CategorySystemConfig, EntryTypeFile, true, configFile, "payload/system/config.yaml", "主配置文件")
	s.add("system_config_proxies", CategorySystemConfig, EntryTypeFile, false, at("proxies.yaml"), "payload/system/proxies.yaml", "代理配置文件（存在时导出）")
	s.add("app_data_root", CategoryAppData, EntryTypeDir, true, at("data"), "payload/app/data/", "应用数据目录（含数据库、快照及默认浏览器数据）")
	s.add("browser_user_data_root", CategoryBrowserData, EntryTypeDir, true, at(orDefault(cfg.Browser.UserDataRoot, "data")),
		"payload/browser/user-data/", "浏览器用户数据根目录（若与 data 重合则自动去重）")

	if t := strings.TrimSpace(cfg.Database.Type); t == "" || strings.EqualFold(t, "sqlite") {
		db := at(orDefault(cfg.Database.SQLite.Path, "data/app.db"))
		s.add("database_sqlite_main", CategoryAppData, EntryTypeFile, true, db, "payload/app/database/app.db", "SQLite 主数据库（若已被 data 覆盖则自动去重）")
		s.add("database_sqlite_wal", CategoryAppData, EntryTypeFile, false, db+"-wal", "payload/app/database/app.db-wal", "SQLite WAL 文件（存在时导出）")
		s.add("database_sqlite_shm", CategoryAppData, EntryTypeFile, false, db+"-shm", "payload/app/database/app.db-shm", "SQLite SHM 文件（存在时导出）")
	}
	if logFile := strings.TrimSpace(cfg.Logging.FilePath); logFile != "" {
		if dir := filepath.Dir(at(logFile)); dir != "." {
			s.add("logs_root", CategoryLogs, EntryTypeDir, false, dir, "payload/app/logs/", "日志目录（存在时导出）")
		}
	}

	sort.SliceStable(s, func(i, j int) bool { return s[i].ID < s[j].ID })
	return Scope{Format: PackageFormat, ManifestVersion: ManifestVersion, AppRoot: root, Entries: s}, nil
}

// scopeSet 按添加顺序去重: 被已有目录包含、或与已有文件相同的路径跳过 (必选标记向上合并)。
type scopeSet []ScopeEntry

func (s *scopeSet) add(id string, cat Category, kind EntryType, required bool, src, archive, desc string) {
	src = filepath.Clean(src)
	for i, e := range *s {
		if sameFile(e.SourcePath, src) {
			(*s)[i].Required = e.Required || required
			return
		}
		if e.EntryType == EntryTypeDir && inside(src, e.SourcePath) {
			return
		}
	}
	_, statErr := os.Stat(src)
	*s = append(*s, ScopeEntry{
		ID: id, Category: cat, EntryType: kind, Required: required,
		SourcePath: src, ArchivePath: archive, Exists: statErr == nil, Description: desc,
	})
}

// BuildManifest 由导出范围生成 manifest (去掉本机路径, 按 id 排序)。
func BuildManifest(scope Scope, appName, appVersion string, createdAt time.Time) Manifest {
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	entries := make([]ManifestEntry, len(scope.Entries))
	for i, e := range scope.Entries {
		entries[i] = ManifestEntry{ID: e.ID, Category: e.Category, EntryType: e.EntryType, Required: e.Required, ArchivePath: e.ArchivePath, Description: e.Description}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return Manifest{
		Format:          PackageFormat,
		ManifestVersion: ManifestVersion,
		CreatedAt:       createdAt.UTC().Format(time.RFC3339),
		App:             ManifestAppInfo{Name: orDefault(appName, "CZL Browser"), Version: orDefault(appVersion, "1.0.0")},
		Entries:         entries,
	}
}

func orDefault(s, def string) string {
	if s = strings.TrimSpace(s); s == "" {
		return def
	}
	return s
}

// under 把相对路径接到 root 下; 绝对路径原样返回。
func under(root, p string) string {
	p = strings.TrimSpace(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, p)
}

// comparable 规范化路径用于比较 (Windows 不区分大小写)。
func comparable(p string) string {
	p = filepath.Clean(strings.TrimSpace(p))
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func sameFile(a, b string) bool { return comparable(a) == comparable(b) }

// inside 判断 p 是否就是 dir 或在 dir 之下。
func inside(p, dir string) bool {
	p, dir = comparable(p), comparable(dir)
	return p == dir || strings.HasPrefix(p, strings.TrimSuffix(dir, string(filepath.Separator))+string(filepath.Separator))
}
