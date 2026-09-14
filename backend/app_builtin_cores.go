package backend

import (
	"fmt"
)

// ============================================================================
// 内置内核源
// ============================================================================

// BuiltinCoreSource 内置内核定义
type BuiltinCoreSource struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Kind        string `json:"kind"` // chrome | clearcote
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
}

// builtinCoreManifestEntry 内置内核清单条目（下载地址与校验值仅后端可见）。
// 清单来源与回退策略见 app_builtin_cores_manifest.go。
type builtinCoreManifestEntry struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Kind        string `json:"kind"`
	Release     string `json:"release,omitempty"`
	Description string `json:"description"`
	DownloadURL string `json:"downloadUrl"`
	SHA256      string `json:"sha256"`
}

// BrowserBuiltinCores 返回内置可用内核列表（10 分钟内复用上次拉取的清单），标记已安装状态
func (a *App) BrowserBuiltinCores() []BuiltinCoreSource {
	return a.builtinCoreSources(false)
}

// BrowserBuiltinCoresRefresh 强制从远程重新拉取内核清单
func (a *App) BrowserBuiltinCoresRefresh() []BuiltinCoreSource {
	return a.builtinCoreSources(true)
}

func (a *App) builtinCoreSources(forceRefresh bool) []BuiltinCoreSource {
	entries := a.builtinCoreEntries(forceRefresh)
	existingCores := a.browserMgr.ListCores()

	result := make([]BuiltinCoreSource, 0, len(entries))
	for _, src := range entries {
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
			Kind:        src.Kind,
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

	for _, src := range a.builtinCoreEntries(false) {
		if src.Id == builtinId {
			coreName := src.Name + " " + src.Version
			go a.browserMgr.DownloadAndExtractCore(a.ctx, coreName, src.DownloadURL, src.SHA256, proxyConfig)
			return nil
		}
	}
	return errBuiltinCoreNotFound
}

var (
	errAppContextNil       = fmt.Errorf("app context is nil")
	errBuiltinCoreNotFound = fmt.Errorf("未找到该内置内核")
)
