// Package browser 管理浏览器实例、内核、代理池、分组与书签的数据, 以及内核下载。
// 结构体的 json 标签是前端契约 (经 Wails 生成到 frontend/src/wailsjs), 改名需同步前端。
package browser

import "github.com/woodchen-ink/czlbrowser/desktop/internal/config"

// Profile 是一个浏览器实例。Running 之后的字段是运行态, 只在内存里。
type Profile struct {
	ProfileId       string   `json:"profileId"`
	ProfileName     string   `json:"profileName"`
	UserDataDir     string   `json:"userDataDir"` // 相对用户数据根目录; 空时用 ProfileId
	CoreId          string   `json:"coreId"`      // 空表示跟随默认内核
	FingerprintArgs []string `json:"fingerprintArgs"`
	ProxyId         string   `json:"proxyId"`     // 代理池 id
	ProxyConfig     string   `json:"proxyConfig"` // ProxyId 为空时是自定义代理地址
	LaunchArgs      []string `json:"launchArgs"`
	Tags            []string `json:"tags"`
	Keywords        []string `json:"keywords"`
	GroupId         string   `json:"groupId"`
	Space           string   `json:"space"` // local / me:<用户> / org:<组织>

	Running     bool   `json:"running"`
	DebugPort   int    `json:"debugPort"`
	Pid         int    `json:"pid"`
	LastError   string `json:"lastError"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	LastStartAt string `json:"lastStartAt"`
	LastStopAt  string `json:"lastStopAt"`
}

// ProfileInput 是新建 / 编辑实例时前端提交的字段。
type ProfileInput struct {
	ProfileName     string   `json:"profileName"`
	UserDataDir     string   `json:"userDataDir"`
	CoreId          string   `json:"coreId"`
	FingerprintArgs []string `json:"fingerprintArgs"`
	ProxyId         string   `json:"proxyId"`
	ProxyConfig     string   `json:"proxyConfig"`
	LaunchArgs      []string `json:"launchArgs"`
	Tags            []string `json:"tags"`
	Keywords        []string `json:"keywords"`
	GroupId         string   `json:"groupId"`
}

// Tab 是实例里的一个标签页。
type Tab struct {
	TabId  string `json:"tabId"`
	Title  string `json:"title"`
	Url    string `json:"url"`
	Active bool   `json:"active"`
}

// Settings 是「新实例默认值」页面编辑的全局设置。
type Settings struct {
	UserDataRoot           string   `json:"userDataRoot"`
	DefaultFingerprintArgs []string `json:"defaultFingerprintArgs"`
	DefaultLaunchArgs      []string `json:"defaultLaunchArgs"`
	DefaultProxy           string   `json:"defaultProxy"`
}

// CoreValidateResult 是内核目录校验结果。
type CoreValidateResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// CoreExtendedInfo 是内核列表额外展示的信息。
type CoreExtendedInfo struct {
	CoreId        string `json:"coreId"`
	ChromeVersion string `json:"chromeVersion"`
	InstanceCount int    `json:"instanceCount"`
	CoreKind      string `json:"coreKind"` // chrome / clearcote, 决定启动时怎么注入指纹
}

// Group 是实例分组 (单层, ParentId 恒为空, 字段为兼容旧数据保留)。
type Group struct {
	GroupId   string `json:"groupId"`
	Space     string `json:"space"`
	GroupName string `json:"groupName"`
	ParentId  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// GroupInput 是新建 / 编辑分组的字段。
type GroupInput struct {
	GroupName string `json:"groupName"`
	ParentId  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
}

// GroupWithCount 附带分组下的实例数。
type GroupWithCount struct {
	Group
	InstanceCount int `json:"instanceCount"`
}

type (
	Proxy         = config.BrowserProxy
	Core          = config.BrowserCore
	Environment   = config.BrowserEnvironment
	ProfileConfig = config.BrowserProfileConfig
)
