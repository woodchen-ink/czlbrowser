// Package config 读写主配置文件 config.yaml。
// yaml 标签是磁盘格式, json 标签是前端契约, 两者改名都会破坏已有用户数据。
package config

// Config 是 config.yaml 的完整结构。
type Config struct {
	Database DatabaseConfig `yaml:"database"`
	App      AppConfig      `yaml:"app"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
	Logging  LoggingConfig  `yaml:"logging"`
	Browser  BrowserConfig  `yaml:"browser"`
}

type DatabaseConfig struct {
	Type   string       `yaml:"type"` // 目前只有 sqlite
	SQLite SQLiteConfig `yaml:"sqlite"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"` // 相对数据根目录
}

type AppConfig struct {
	Name   string       `yaml:"name"` // 窗口标题
	Window WindowConfig `yaml:"window"`
	// CloseAction 点窗口关闭按钮时: tray 最小化到托盘 (默认, 空值同此), quit 退出, ask 每次询问
	CloseAction string `yaml:"close_action"`
}

type WindowConfig struct {
	Width     int `yaml:"width"`
	Height    int `yaml:"height"`
	MinWidth  int `yaml:"min_width"`
	MinHeight int `yaml:"min_height"`
}

// RuntimeConfig 是 Go 运行时的内存软上限与 GC 比例。
type RuntimeConfig struct {
	MaxMemoryMB int `yaml:"max_memory_mb"`
	GCPercent   int `yaml:"gc_percent"`
}

type LoggingConfig struct {
	Level       string         `yaml:"level"`
	FileEnabled bool           `yaml:"file_enabled"`
	FilePath    string         `yaml:"file_path"` // 相对数据根目录
	Rotation    RotationConfig `yaml:"rotation"`
}

type RotationConfig struct {
	MaxSizeMB  int `yaml:"max_size_mb"`
	MaxBackups int `yaml:"max_backups"`
}

// BrowserConfig 是浏览器相关设置。实例、代理、内核如今存在数据库,
// Cores / Proxies / Profiles 只在旧版迁移与备份包里出现。
type BrowserConfig struct {
	UserDataRoot           string            `yaml:"user_data_root"`
	DefaultFingerprintArgs []string          `yaml:"default_fingerprint_args"`
	DefaultLaunchArgs      []string          `yaml:"default_launch_args"`
	DefaultProxy           string            `yaml:"default_proxy"` // 代理池 id; 含 :// 的旧值视为自定义代理地址
	DefaultBookmarks       []BrowserBookmark `yaml:"default_bookmarks,omitempty"`

	Cores    []BrowserCore          `yaml:"cores,omitempty"`
	Proxies  []BrowserProxy         `yaml:"proxies,omitempty"`
	Profiles []BrowserProfileConfig `yaml:"profiles,omitempty"`

	XrayBinaryPath    string `yaml:"xray_binary_path,omitempty"`    // 为空时用程序目录 bin\xray.exe
	SingBoxBinaryPath string `yaml:"singbox_binary_path,omitempty"` // 为空时用程序目录 bin\sing-box.exe

	// 以下为早期版本的「环境」写法, 读到后迁移成 Cores 并清空
	ChromeBinaryPath     string               `yaml:"chrome_binary_path,omitempty"`
	CoreRoot             string               `yaml:"core_root,omitempty"`
	DefaultCoreId        string               `yaml:"default_core_id,omitempty"`
	DefaultConnectorType string               `yaml:"default_connector_type,omitempty"`
	Environments         []BrowserEnvironment `yaml:"environments,omitempty"`
}

type BrowserBookmark struct {
	Name string   `yaml:"name" json:"name"`
	URL  string   `yaml:"url" json:"url"`
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// BrowserCore 是一个浏览器内核目录。
type BrowserCore struct {
	CoreId    string   `yaml:"core_id" json:"coreId"`
	CoreName  string   `yaml:"core_name" json:"coreName"`
	CorePath  string   `yaml:"core_path" json:"corePath"`
	IsDefault bool     `yaml:"is_default" json:"isDefault"`
	Tags      []string `yaml:"tags,omitempty" json:"tags"`
}

// BrowserProxy 是代理池里的一条代理。Last* 是测速结果, 只存数据库不写 yaml。
type BrowserProxy struct {
	ProxyId     string   `yaml:"proxy_id" json:"proxyId"`
	ProxyName   string   `yaml:"proxy_name" json:"proxyName"`
	ProxyConfig string   `yaml:"proxy_config" json:"proxyConfig"`
	DnsServers  string   `yaml:"dns_servers,omitempty" json:"dnsServers,omitempty"`
	GroupName   string   `yaml:"group_name,omitempty" json:"groupName,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags"`
	SortOrder   int      `yaml:"sort_order,omitempty" json:"sortOrder,omitempty"`

	// 从订阅链接导入时记录来源, 用于定时刷新
	SourceID               string `yaml:"source_id,omitempty" json:"sourceId,omitempty"`
	SourceURL              string `yaml:"source_url,omitempty" json:"sourceUrl,omitempty"`
	SourceNamePrefix       string `yaml:"source_name_prefix,omitempty" json:"sourceNamePrefix,omitempty"`
	SourceAutoRefresh      bool   `yaml:"source_auto_refresh,omitempty" json:"sourceAutoRefresh,omitempty"`
	SourceRefreshIntervalM int    `yaml:"source_refresh_interval_m,omitempty" json:"sourceRefreshIntervalM,omitempty"`
	SourceLastRefreshAt    string `yaml:"source_last_refresh_at,omitempty" json:"sourceLastRefreshAt,omitempty"`

	LastLatencyMs    int64  `yaml:"-" json:"lastLatencyMs"`
	LastTestOk       bool   `yaml:"-" json:"lastTestOk"`
	LastTestedAt     string `yaml:"-" json:"lastTestedAt"`
	LastIPHealthJSON string `yaml:"-" json:"lastIPHealthJson,omitempty"`
}

// BrowserEnvironment 是早期版本的内核 + 代理组合, 仅用于迁移。
type BrowserEnvironment struct {
	CoreId        string `yaml:"core_id" json:"coreId"`
	CoreName      string `yaml:"core_name" json:"coreName"`
	CorePath      string `yaml:"core_path" json:"corePath"`
	ProxyConfig   string `yaml:"proxy_config" json:"proxyConfig"`
	ConnectorType string `yaml:"connector_type" json:"connectorType"`
	IsDefault     bool   `yaml:"is_default" json:"isDefault"`
}

// BrowserProfileConfig 是实例在配置文件里的写法 (旧版与备份包)。
type BrowserProfileConfig struct {
	ProfileId       string   `yaml:"profile_id" json:"profileId"`
	ProfileName     string   `yaml:"profile_name" json:"profileName"`
	UserDataDir     string   `yaml:"user_data_dir" json:"userDataDir"`
	CoreId          string   `yaml:"core_id" json:"coreId"`
	FingerprintArgs []string `yaml:"fingerprint_args" json:"fingerprintArgs"`
	ProxyId         string   `yaml:"proxy_id" json:"proxyId"`
	ProxyConfig     string   `yaml:"proxy_config" json:"proxyConfig"`
	LaunchArgs      []string `yaml:"launch_args" json:"launchArgs"`
	Tags            []string `yaml:"tags" json:"tags"`
	Keywords        []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	CreatedAt       string   `yaml:"created_at" json:"createdAt"`
	UpdatedAt       string   `yaml:"updated_at" json:"updatedAt"`
}
