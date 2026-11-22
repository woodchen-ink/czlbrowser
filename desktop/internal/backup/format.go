// Package backup 定义完整备份包 (zip) 的格式与导出范围, 以及导出 / 导入时 Chromium 用户数据的筛选。
// 条目 id、归档路径与 manifest 字段是备份文件格式, 改动会让旧备份无法导入。
package backup

const (
	// PackageFormat 写在 manifest.json 里, 导入时据此识别备份包。
	PackageFormat = "czl-browser-full-backup"
	// ManifestVersion 是 manifest.json 的结构版本。
	ManifestVersion = 1
)

// Category 是条目的大类, 导入时按大类决定还原方式。
type Category string

const (
	CategorySystemConfig Category = "system_config"
	CategoryAppData      Category = "app_data"
	CategoryBrowserData  Category = "browser_data"
	CategoryCoreData     Category = "core_data" // 只出现在旧版备份包里, 新版不导出内核
	CategoryLogs         Category = "logs"
)

type EntryType string

const (
	EntryTypeFile EntryType = "file"
	EntryTypeDir  EntryType = "dir"
)

// ScopeEntry 是一个要打进备份包的本机文件或目录。
type ScopeEntry struct {
	ID          string    `json:"id"`
	Category    Category  `json:"category"`
	EntryType   EntryType `json:"entryType"`
	Required    bool      `json:"required"`
	SourcePath  string    `json:"sourcePath"`
	ArchivePath string    `json:"archivePath"`
	Exists      bool      `json:"exists"`
	Description string    `json:"description,omitempty"`
}

// Scope 是一次导出的全部条目。
type Scope struct {
	Format          string       `json:"format"`
	ManifestVersion int          `json:"manifestVersion"`
	AppRoot         string       `json:"appRoot"`
	Entries         []ScopeEntry `json:"entries"`
}

// Manifest 写在备份包根目录的 manifest.json, 不含本机绝对路径。
type Manifest struct {
	Format          string          `json:"format"`
	ManifestVersion int             `json:"manifestVersion"`
	CreatedAt       string          `json:"createdAt"`
	App             ManifestAppInfo `json:"app"`
	Entries         []ManifestEntry `json:"entries"`
}

type ManifestAppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ManifestEntry struct {
	ID          string    `json:"id"`
	Category    Category  `json:"category"`
	EntryType   EntryType `json:"entryType"`
	Required    bool      `json:"required"`
	ArchivePath string    `json:"archivePath"`
	Description string    `json:"description,omitempty"`
}
