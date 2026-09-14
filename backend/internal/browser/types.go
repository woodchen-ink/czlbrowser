package browser

import (
	"czl-browser/backend/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Profile 浏览器配置文件
type Profile struct {
	ProfileId       string   `json:"profileId"`
	ProfileName     string   `json:"profileName"`
	UserDataDir     string   `json:"userDataDir"`
	CoreId          string   `json:"coreId"`
	FingerprintArgs []string `json:"fingerprintArgs"`
	ProxyId         string   `json:"proxyId"`
	ProxyConfig     string   `json:"proxyConfig"`
	LaunchArgs      []string `json:"launchArgs"`
	Tags            []string `json:"tags"`
	Keywords        []string `json:"keywords"`
	GroupId         string   `json:"groupId"` // 所属分组ID
	Running         bool     `json:"running"`
	DebugPort       int      `json:"debugPort"`
	Pid             int      `json:"pid"`
	LastError       string   `json:"lastError"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
	LastStartAt     string   `json:"lastStartAt"`
	LastStopAt      string   `json:"lastStopAt"`
}

// ProfileInput 创建/更新配置文件的输入
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
	GroupId         string   `json:"groupId"` // 所属分组ID
}

// Tab 浏览器标签页
type Tab struct {
	TabId  string `json:"tabId"`
	Title  string `json:"title"`
	Url    string `json:"url"`
	Active bool   `json:"active"`
}

// Settings 浏览器全局设置
type Settings struct {
	UserDataRoot           string   `json:"userDataRoot"`
	DefaultFingerprintArgs []string `json:"defaultFingerprintArgs"`
	DefaultLaunchArgs      []string `json:"defaultLaunchArgs"`
	DefaultProxy           string   `json:"defaultProxy"`
}

// CoreValidateResult 内核路径验证结果
type CoreValidateResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// CoreExtendedInfo 内核扩展信息
type CoreExtendedInfo struct {
	CoreId        string `json:"coreId"`
	ChromeVersion string `json:"chromeVersion"`
	InstanceCount int    `json:"instanceCount"`
	CoreKind      string `json:"coreKind"` // chrome | clearcote，决定实例启动时的注入方式
}

// Group 实例分组
type Group struct {
	GroupId   string `json:"groupId"`
	GroupName string `json:"groupName"`
	ParentId  string `json:"parentId"` // 空字符串表示根级分组
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// GroupInput 创建/更新分组的输入
type GroupInput struct {
	GroupName string `json:"groupName"`
	ParentId  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
}

// GroupWithCount 带实例计数的分组
type GroupWithCount struct {
	Group
	InstanceCount int `json:"instanceCount"`
}

// 类型别名
type Proxy = config.BrowserProxy
type Core = config.BrowserCore
type Environment = config.BrowserEnvironment
type ProfileConfig = config.BrowserProfileConfig

// Manager 浏览器管理器
type Manager struct {
	Config           *config.Config
	DataRoot         string // 用户可写数据根目录，chrome 内核与用户数据相对路径基于此解析（生产=%LOCALAPPDATA%/CZL Browser，dev=项目根）
	ConfigPath       string // 主配置文件 config.yaml 实际路径（生产=%APPDATA%，dev=项目根）；DAO 未注入时的降级保存目标
	Profiles         map[string]*Profile
	Mutex            sync.Mutex
	BrowserProcesses map[string]*exec.Cmd
	XrayBridges      map[string]*XrayBridge

	// DAO 层（注入后使用 SQLite 存储，未注入时降级到 config.yaml）
	ProfileDAO  ProfileDAO
	ProxyDAO    ProxyDAO
	CoreDAO     CoreDAO
	BookmarkDAO BookmarkDAO
	GroupDAO    GroupDAO
}

// XrayBridge Xray 桥接进程
type XrayBridge struct {
	NodeKey   string
	Port      int
	Cmd       *exec.Cmd
	Pid       int
	Running   bool
	LastError string
}

// NewManager 创建浏览器管理器
func NewManager(cfg *config.Config, dataRoot string, configPath string) *Manager {
	return &Manager{
		Config:           cfg,
		DataRoot:         dataRoot,
		ConfigPath:       configPath,
		Profiles:         make(map[string]*Profile),
		BrowserProcesses: make(map[string]*exec.Cmd),
		XrayBridges:      make(map[string]*XrayBridge),
	}
}

// configSavePath 返回主配置保存路径：优先显式 ConfigPath，
// 兜底回退到 DataRoot/config.yaml（兼容未注入 ConfigPath 的旧调用与测试）。
func (m *Manager) configSavePath() string {
	if p := filepath.Clean(m.ConfigPath); m.ConfigPath != "" {
		return p
	}
	return m.ResolveRelativePath("config.yaml")
}

// ResolveRelativePath 将相对路径解析为绝对路径（基于 DataRoot）。
// chrome 内核目录与用户数据目录均为可写数据，经此解析到 %LOCALAPPDATA%（生产）。
// 如果传入的已经是绝对路径则直接返回。
func (m *Manager) ResolveRelativePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if m.DataRoot != "" {
		return filepath.Join(m.DataRoot, p)
	}
	// 兜底：使用 CWD
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, p)
	}
	return p
}
