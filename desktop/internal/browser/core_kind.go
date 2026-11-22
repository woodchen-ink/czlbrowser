package browser

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CoreKind 内核类型，决定实例启动时指纹/时区的注入方式。
type CoreKind string

const (
	// CoreKindChrome 标准 Chrome/Chromium：不认指纹开关，时区与反检测依赖 CDP 注入。
	CoreKindChrome CoreKind = "chrome"
	// CoreKindClearcote Clearcote（开源指纹 Chromium）：引擎级指纹与 --timezone 原生开关，无需 JS 注入。
	CoreKindClearcote CoreKind = "clearcote"
)

// chromeVersionPattern 匹配 Chrome 四段式版本号目录名，如 153.0.8010.37
var chromeVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

// clearcoteDLLMarker 是 Clearcote 补丁独有的命令行开关名，编译进 chrome.dll 的字符串常量。
var clearcoteDLLMarker = []byte("canvas-bridge-url")

type coreKindCacheEntry struct {
	modTime time.Time
	size    int64
	kind    CoreKind
}

var (
	coreKindCacheMu sync.Mutex
	coreKindCache   = map[string]coreKindCacheEntry{}
)

// DetectCoreKind 根据 chrome.exe 所在目录识别内核类型。
// 通过扫描 chrome.dll 中的 Clearcote 专有开关字符串识别，结果按 dll 路径 + 修改时间缓存；
// 手动添加、备份导入的内核同样适用，无需持久化类型字段。识别失败时按标准 Chrome 处理。
func DetectCoreKind(chromeExePath string) CoreKind {
	dllPath := findChromeDLL(filepath.Dir(chromeExePath))
	if dllPath == "" {
		return CoreKindChrome
	}
	info, err := os.Stat(dllPath)
	if err != nil {
		return CoreKindChrome
	}

	cacheKey := strings.ToLower(dllPath)
	coreKindCacheMu.Lock()
	if entry, ok := coreKindCache[cacheKey]; ok && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		coreKindCacheMu.Unlock()
		return entry.kind
	}
	coreKindCacheMu.Unlock()

	kind := CoreKindChrome
	if found, err := fileContains(dllPath, clearcoteDLLMarker); err == nil && found {
		kind = CoreKindClearcote
	}

	coreKindCacheMu.Lock()
	coreKindCache[cacheKey] = coreKindCacheEntry{modTime: info.ModTime(), size: info.Size(), kind: kind}
	coreKindCacheMu.Unlock()
	return kind
}

// ResolveCoreKind 识别内核配置对应的内核类型。
func (m *Manager) ResolveCoreKind(core Core) CoreKind {
	exePath, err := m.ResolveCoreExecutable(core)
	if err != nil {
		return CoreKindChrome
	}
	return DetectCoreKind(exePath)
}

// findChromeDLL 查找 chrome.dll：Clearcote 放在根目录，官方 Chrome 放在 <版本号>/ 子目录。
func findChromeDLL(coreDir string) string {
	candidate := filepath.Join(coreDir, "chrome.dll")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	matches, _ := filepath.Glob(filepath.Join(coreDir, "*", "chrome.dll"))
	for _, match := range matches {
		if chromeVersionPattern.MatchString(filepath.Base(filepath.Dir(match))) {
			return match
		}
	}
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// fileContains 以分块流式读取判断文件是否包含指定字节序列（块间保留重叠，避免跨块漏检）。
func fileContains(path string, needle []byte) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	const chunkSize = 4 << 20
	overlap := len(needle) - 1
	buf := make([]byte, chunkSize+overlap)
	carry := 0
	for {
		n, readErr := io.ReadFull(f, buf[carry:])
		total := carry + n
		if bytes.Contains(buf[:total], needle) {
			return true, nil
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			return false, nil
		}
		if readErr != nil {
			return false, readErr
		}
		carry = min(overlap, total)
		copy(buf, buf[total-carry:total])
	}
}
