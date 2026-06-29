package backup

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// cache_filter 负责判定 Chromium 用户数据目录中"导出时可丢弃"的路径。
//
// 识别策略：先扫描目录树定位所有 profile 根目录（直接含 Default 子目录或
// Local State 文件的目录即为 profile 根，这是 Chromium user-data-dir 的铁律，
// 与目录命名无关——profile 目录名可能是 UUID，也可能是用户自定义的任意字符串）。
// 对 profile 根内部采用「白名单」：只保留 Default / Profile N 等真实账号数据目录
// 和 Local State 等关键顶层文件，其余 component-updater 组件、指标遥测、模型缓存
// 等一律跳过；保留下来的数据目录内部再用「缓存黑名单」剔除 Cache / Code Cache /
// Service Worker 缓存等可重建缓存。
//
// profile 根之外的路径（数据库、快照、auth 文件等业务数据）一律保留。
// 跳过的全部是 Chromium 下次启动会自动重建的数据，不含登录态与站点持久内容。

// profileKeepTopLevelFiles 是 profile 根下需保留的关键顶层文件（profile 级设置 / 加密密钥）。
var profileKeepTopLevelFiles = map[string]struct{}{
	"Local State":  {},
	"First Run":    {},
	"Last Version": {},
	"Last Browser": {},
}

// profileKeepDirExact 是 profile 根下需保留的数据目录（精确名）。
var profileKeepDirExact = map[string]struct{}{
	"Default":        {},
	"System Profile": {},
	"Guest Profile":  {},
}

// chromiumDefaultCacheDirNames 是数据目录内部的可重建缓存目录，任意层级命中即整目录跳过。
var chromiumDefaultCacheDirNames = map[string]struct{}{
	"Cache":                {},
	"Code Cache":           {},
	"GPUCache":             {},
	"DawnCache":            {},
	"DawnGraphiteCache":    {},
	"DawnWebGPUCache":      {},
	"ShaderCache":          {},
	"GrShaderCache":        {},
	"GraphiteDawnCache":    {},
	"Crashpad":             {},
	"component_crx_cache":  {},
	"extensions_crx_cache": {},
	"Safe Browsing":        {},
	"GCM Store":            {},
}

// chromiumDefaultCacheSubPaths 按"父目录/子目录"组合判定的缓存子目录。
var chromiumDefaultCacheSubPaths = [][2]string{
	{"Service Worker", "CacheStorage"},
	{"Service Worker", "ScriptCache"},
	{"Network", "Cache"},
}

// chromiumExtensionBlocklist 是导入/导出时跳过的可重建或外部联动扩展本体。
var chromiumExtensionBlocklist = map[string]struct{}{
	"efaidnbmnnnibpcajpcglclefindmkaj": {}, // Adobe Acrobat
	"ghbmnnjooekpmoecnnnilnnbdlolhkhi": {}, // Google Docs Offline
	"ngpampappnmepgilojfohadhhmbhlaek": {}, // IDM Integration Module
	"nmmhkkegccagdldgiimedpiccmgmieda": {}, // Chrome Web Store Payments
}

// profileRootMarkers 是判定某目录为 profile 根的标志性直接子项。
var profileRootMarkers = []string{"Default", "Local State"}

// DetectProfileRoots 扫描 srcDir，返回所有 profile 根目录相对 srcDir 的路径集合（斜杠分隔）。
// 判据：目录直接含 Default 子目录或 Local State 文件。srcDir 本身也参与判定（rel="."）。
func DetectProfileRoots(srcDir string) map[string]struct{} {
	roots := make(map[string]struct{})
	_ = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || !d.IsDir() {
			return nil
		}
		if !dirHasProfileMarker(path) {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return nil
		}
		roots[filepath.ToSlash(rel)] = struct{}{}
		// profile 根内部不会再嵌套 profile 根，跳过其子树以省扫描。
		return filepath.SkipDir
	})
	return roots
}

// dirHasProfileMarker 判断目录是否直接包含 profile 根标志（Default 子目录或 Local State 文件）。
func dirHasProfileMarker(dir string) bool {
	for _, marker := range profileRootMarkers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// NewProfileAwareSkip 基于预扫描得到的 profile 根集合，返回用于打包/同步的"是否跳过"判定。
// rel 为相对扫描根目录的路径（斜杠或系统分隔均可）。
func NewProfileAwareSkip(profileRoots map[string]struct{}) func(rel string) bool {
	return func(rel string) bool {
		segs := splitPathSegments(rel)
		if len(segs) == 0 {
			return false
		}
		rootDepth, ok := matchProfileRootDepth(segs, profileRoots)
		if !ok {
			// 不在任何 profile 根之下，保留（数据库 / 快照 / auth 等业务数据）。
			return false
		}
		inner := segs[rootDepth:]
		return shouldSkipInsideProfile(inner)
	}
}

// matchProfileRootDepth 找出 segs 的最长前缀命中 profileRoots 的深度。
// 返回 profile 根在 segs 中占用的段数；未命中返回 false。
func matchProfileRootDepth(segs []string, profileRoots map[string]struct{}) (int, bool) {
	// profile 根相对路径可能是多段（如 data 下的 <uuid>）或就是扫描根本身（rel="."）。
	if _, ok := profileRoots["."]; ok {
		return 0, true
	}
	for depth := len(segs); depth >= 1; depth-- {
		key := strings.Join(segs[:depth], "/")
		if _, ok := profileRoots[key]; ok {
			return depth, true
		}
	}
	return 0, false
}

// shouldSkipInsideProfile 判定 profile 根内部相对路径段是否可丢弃。
func shouldSkipInsideProfile(inner []string) bool {
	if len(inner) == 0 {
		return false
	}
	top := inner[0]
	if isProfileKeepTop(top) {
		return isDefaultInternalCache(inner[1:])
	}
	return true
}

// isProfileKeepTop 判断 profile 根下的直接子项是否需保留。
func isProfileKeepTop(name string) bool {
	if _, ok := profileKeepDirExact[name]; ok {
		return true
	}
	if _, ok := profileKeepTopLevelFiles[name]; ok {
		return true
	}
	if strings.HasPrefix(name, "Profile ") {
		return true
	}
	return false
}

// isDefaultInternalCache 判断数据目录内部路径是否落在可重建缓存目录。
func isDefaultInternalCache(segs []string) bool {
	if isBlockedExtensionPath(segs) {
		return true
	}
	for _, seg := range segs {
		if _, ok := chromiumDefaultCacheDirNames[seg]; ok {
			return true
		}
	}
	for i := 0; i+1 < len(segs); i++ {
		for _, pair := range chromiumDefaultCacheSubPaths {
			if segs[i] == pair[0] && segs[i+1] == pair[1] {
				return true
			}
		}
	}
	return false
}

// isBlockedExtensionPath 判断路径是否命中需排除的 Chromium 扩展本体目录。
func isBlockedExtensionPath(segs []string) bool {
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] != "Extensions" {
			continue
		}
		extensionID := strings.ToLower(strings.TrimSpace(segs[i+1]))
		if _, ok := chromiumExtensionBlocklist[extensionID]; ok {
			return true
		}
	}
	return false
}

// splitPathSegments 将相对路径按斜杠或系统分隔符切分为非空路径段。
func splitPathSegments(rel string) []string {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return nil
	}
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
