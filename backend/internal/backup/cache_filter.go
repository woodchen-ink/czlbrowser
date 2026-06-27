package backup

import (
	"path/filepath"
	"strings"
)

// cache_filter 负责判定 Chromium 用户数据目录中"可重建缓存"路径。
// 导出备份包时跳过这些路径，避免把 GB 级浏览器缓存打进配置备份；
// 这些目录在浏览器下次启动时会自动重建，不影响登录态与本地存储等持久数据。

// chromiumCacheDirNames 是出现在任意层级即视为缓存、整目录跳过的目录名。
// 均为 Chromium 可重建缓存 / 崩溃转储 / 临时数据，不含登录态与站点持久数据。
var chromiumCacheDirNames = map[string]struct{}{
	"Cache":                          {},
	"Code Cache":                     {},
	"GPUCache":                       {},
	"DawnCache":                      {},
	"DawnGraphiteCache":              {},
	"DawnWebGPUCache":                {},
	"ShaderCache":                    {},
	"GrShaderCache":                  {},
	"GraphiteDawnCache":              {},
	"Crashpad":                       {},
	"component_crx_cache":            {},
	"extensions_crx_cache":           {},
	"Safe Browsing":                  {},
	"optimization_guide_model_store": {},
	"segmentation_platform":          {},
}

// chromiumCacheSubPaths 是需要按"父目录/子目录"组合判定的缓存子目录。
// 例如 Service Worker 下的 CacheStorage / ScriptCache 是缓存，
// 但 Service Worker/Database 关联站点注册数据，需保留，故不整目录跳过。
var chromiumCacheSubPaths = [][2]string{
	{"Service Worker", "CacheStorage"},
	{"Service Worker", "ScriptCache"},
	{"Network", "Cache"},
}

// IsChromiumCacheRelPath 判断给定相对路径（相对于 user-data 根，斜杠分隔或系统分隔均可）
// 是否落在 Chromium 可重建缓存目录内。命中则导出时跳过。
func IsChromiumCacheRelPath(rel string) bool {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return false
	}
	segments := splitPathSegments(rel)
	if len(segments) == 0 {
		return false
	}

	for _, seg := range segments {
		if _, ok := chromiumCacheDirNames[seg]; ok {
			return true
		}
	}

	for i := 0; i+1 < len(segments); i++ {
		for _, pair := range chromiumCacheSubPaths {
			if segments[i] == pair[0] && segments[i+1] == pair[1] {
				return true
			}
		}
	}

	return false
}

// splitPathSegments 将相对路径按斜杠或系统分隔符切分为非空路径段。
func splitPathSegments(rel string) []string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." {
			continue
		}
		out = append(out, p)
	}
	return out
}
