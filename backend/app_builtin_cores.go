package backend

import "fmt"

// ============================================================================
// 内置内核源
// ============================================================================

// BuiltinCoreSource 内置内核定义
type BuiltinCoreSource struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	DownloadURL string `json:"-"` // 不暴露给前端
	Installed   bool   `json:"installed"`
}

// 内置内核源列表（下载地址仅后端可见）
var builtinCoreSources = []struct {
	Id          string
	Name        string
	Version     string
	Description string
	DownloadURL string
}{
	{
		Id:          "chrome-stable",
		Name:        "Chrome",
		Version:     "146.0.7680.80",
		Description: "标准 Chrome 内核，TLS/指纹完全真实，推荐使用",
		DownloadURL: "https://github.com/woodchen-ink/Chrome-exe/releases/download/146.0.7680.80/Chrome-bin.zip",
	},
}

// BrowserBuiltinCores 返回内置可用内核列表，标记已安装状态
func (a *App) BrowserBuiltinCores() []BuiltinCoreSource {
	existingCores := a.browserMgr.ListCores()

	result := make([]BuiltinCoreSource, 0, len(builtinCoreSources))
	for _, src := range builtinCoreSources {
		installed := false
		for _, core := range existingCores {
			// 通过内核名称匹配判断是否已安装
			if core.CoreName == src.Name+" "+src.Version || core.CoreName == src.Name {
				installed = true
				break
			}
		}
		result = append(result, BuiltinCoreSource{
			Id:          src.Id,
			Name:        src.Name,
			Version:     src.Version,
			Description: src.Description,
			Installed:   installed,
		})
	}
	return result
}

// BrowserBuiltinCoreDownload 下载指定的内置内核
func (a *App) BrowserBuiltinCoreDownload(builtinId string, proxyConfig string) error {
	if a.ctx == nil {
		return errAppContextNil
	}

	for _, src := range builtinCoreSources {
		if src.Id == builtinId {
			coreName := src.Name + " " + src.Version
			go a.browserMgr.DownloadAndExtractCore(a.ctx, coreName, src.DownloadURL, proxyConfig)
			return nil
		}
	}
	return errBuiltinCoreNotFound
}

var (
	errAppContextNil      = fmt.Errorf("app context is nil")
	errBuiltinCoreNotFound = fmt.Errorf("未找到该内置内核")
)
