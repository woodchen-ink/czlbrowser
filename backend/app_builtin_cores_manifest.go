package backend

import (
	"context"
	"czl-browser/backend/internal/browser"
	"czl-browser/backend/internal/logger"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 内置内核清单：远程拉取 → 本地缓存 → 编译内嵌 三级回退
// ============================================================================
//
// 清单托管在公开仓库 woodchen-ink/Chrome-exe 的 cores.json，由该仓库的 Actions 每日同步
// Chrome 稳定版与 Clearcote 上游版本，客户端无需升级即可拿到新内核。
// 两个地址并发请求取先成功者：raw.githubusercontent.com 更新即时，jsDelivr 在国内可达性更好（分支缓存约 12 小时）。

var builtinCoreManifestURLs = []string{
	"https://raw.githubusercontent.com/woodchen-ink/Chrome-exe/main/cores.json",
	"https://cdn.jsdelivr.net/gh/woodchen-ink/Chrome-exe@main/cores.json",
}

const (
	builtinCoreManifestCacheFile = "data/builtin_cores_cache.json"
	builtinCoreManifestFetchTTL  = 10 * time.Minute
	builtinCoreManifestTimeout   = 6 * time.Second
	builtinCoreManifestMaxBytes  = 1 << 20
)

// builtinCoresJSON 编译时内嵌的兜底清单（CI 构建前会从远程刷新一次）。
//
//go:embed builtin_cores.json
var builtinCoresJSON []byte

var embeddedBuiltinCores = mustParseBuiltinCores(builtinCoresJSON)

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// builtinCoreManifestState 进程内的清单状态，避免每次打开下载弹窗都请求网络。
type builtinCoreManifestState struct {
	mu        sync.Mutex
	cores     []builtinCoreManifestEntry
	fetchedAt time.Time
}

var builtinManifest builtinCoreManifestState

func mustParseBuiltinCores(data []byte) []builtinCoreManifestEntry {
	cores, err := parseBuiltinCoreManifest(data)
	if err != nil {
		panic(fmt.Sprintf("内置内核清单 builtin_cores.json 无效: %v", err))
	}
	return cores
}

// parseBuiltinCoreManifest 解析并校验清单；远程清单同样经过此校验，防止异常数据进入下载流程。
func parseBuiltinCoreManifest(data []byte) ([]builtinCoreManifestEntry, error) {
	var manifest struct {
		Cores []builtinCoreManifestEntry `json:"cores"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if len(manifest.Cores) == 0 {
		return nil, errors.New("清单为空")
	}

	seen := make(map[string]bool, len(manifest.Cores))
	valid := make([]builtinCoreManifestEntry, 0, len(manifest.Cores))
	for _, entry := range manifest.Cores {
		entry.SHA256 = strings.ToLower(strings.TrimSpace(entry.SHA256))
		switch {
		case entry.Id == "" || entry.Name == "" || entry.Version == "":
			continue
		case seen[entry.Id]:
			continue
		// 只允许从 GitHub 下载，限制清单被篡改时的影响面
		case !strings.HasPrefix(entry.DownloadURL, "https://github.com/"):
			continue
		case entry.SHA256 != "" && !sha256HexPattern.MatchString(entry.SHA256):
			continue
		}
		seen[entry.Id] = true
		valid = append(valid, entry)
	}
	if len(valid) == 0 {
		return nil, errors.New("清单中没有有效条目")
	}
	return valid, nil
}

// builtinCoreEntries 返回当前可用的内置内核清单：TTL 内直接用内存结果，否则拉取远程，
// 失败时依次回退到本地缓存与内嵌清单。
func (a *App) builtinCoreEntries(forceRefresh bool) []builtinCoreManifestEntry {
	builtinManifest.mu.Lock()
	defer builtinManifest.mu.Unlock()

	if !forceRefresh && builtinManifest.cores != nil && time.Since(builtinManifest.fetchedAt) < builtinCoreManifestFetchTTL {
		return builtinManifest.cores
	}

	log := logger.New("Browser")
	cachePath := a.resolveAppPath(builtinCoreManifestCacheFile)

	data, source, err := fetchRemoteBuiltinCoreManifest()
	if err == nil {
		cores, parseErr := parseBuiltinCoreManifest(data)
		if parseErr == nil {
			builtinManifest.cores = cores
			builtinManifest.fetchedAt = time.Now()
			if writeErr := writeFileAtomic(cachePath, data); writeErr != nil {
				log.Warn("内核清单缓存写入失败", logger.F("path", cachePath), logger.F("error", writeErr.Error()))
			}
			log.Info("内核清单已从远程更新", logger.F("source", source), logger.F("count", len(cores)))
			return cores
		}
		err = fmt.Errorf("%s: %w", source, parseErr)
	}
	log.Warn("远程内核清单获取失败，使用本地清单", logger.F("error", err.Error()))

	// 失败后同样记录时间，TTL 内不再反复请求网络
	builtinManifest.fetchedAt = time.Now()
	if cached, readErr := os.ReadFile(cachePath); readErr == nil {
		if cores, parseErr := parseBuiltinCoreManifest(cached); parseErr == nil {
			builtinManifest.cores = cores
			return cores
		}
	}
	if builtinManifest.cores == nil {
		builtinManifest.cores = embeddedBuiltinCores
	}
	return builtinManifest.cores
}

// fetchRemoteBuiltinCoreManifest 并发请求所有清单地址，返回第一个成功的响应体。
func fetchRemoteBuiltinCoreManifest() ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), builtinCoreManifestTimeout)
	defer cancel()

	client := &http.Client{Transport: &http.Transport{Proxy: browser.SystemProxyURL}}

	type result struct {
		data   []byte
		source string
		err    error
	}
	results := make(chan result, len(builtinCoreManifestURLs))
	for _, manifestURL := range builtinCoreManifestURLs {
		go func(manifestURL string) {
			data, err := fetchManifestURL(ctx, client, manifestURL)
			if err == nil {
				if _, parseErr := parseBuiltinCoreManifest(data); parseErr != nil {
					err = parseErr
				}
			}
			results <- result{data: data, source: manifestURL, err: err}
		}(manifestURL)
	}

	var errs []error
	for range builtinCoreManifestURLs {
		r := <-results
		if r.err == nil {
			return r.data, r.source, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", r.source, r.err))
	}
	return nil, "", errors.Join(errs...)
}

func fetchManifestURL(ctx context.Context, client *http.Client, manifestURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, builtinCoreManifestMaxBytes))
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
