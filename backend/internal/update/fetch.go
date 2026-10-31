package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ManifestURLs 清单地址：CZL Connect 代理公开仓库 main 分支的 latest.json，国内直连可达，为主地址；
// raw 与 jsDelivr 作兜底（CI 发布后会主动 purge jsDelivr）。三者内容相同且都需验签，取版本最高者。
var ManifestURLs = []string{
	"https://connect.czl.net/dl/czlbrowser/latest/update-manifest",
	"https://raw.githubusercontent.com/woodchen-ink/czlbrowser-releases/main/latest.json",
	"https://cdn.jsdelivr.net/gh/woodchen-ink/czlbrowser-releases@main/latest.json",
}

const (
	manifestTimeout  = 8 * time.Second
	manifestMaxBytes = 64 << 10
)

// FetchLatest 并发请求所有清单地址，返回验签通过的最高版本。
// 与内核清单"先到先用"不同，这里等待全部结果：某个 CDN 缓存旧清单时不会掩盖新版本。
func FetchLatest(ctx context.Context, client *http.Client) (*Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, manifestTimeout)
	defer cancel()

	type result struct {
		manifest *Manifest
		err      error
	}
	results := make(chan result, len(ManifestURLs))
	for _, u := range ManifestURLs {
		go func(u string) {
			m, err := fetchManifest(ctx, client, u)
			if err != nil {
				err = fmt.Errorf("%s: %w", u, err)
			}
			results <- result{manifest: m, err: err}
		}(u)
	}

	var best *Manifest
	var errs []error
	for range ManifestURLs {
		r := <-results
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		if best == nil || CompareVersions(r.manifest.Version, best.Version) > 0 {
			best = r.manifest
		}
	}
	if best == nil {
		return nil, errors.Join(errs...)
	}
	return best, nil
}

func fetchManifest(ctx context.Context, client *http.Client, u string) (*Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, manifestMaxBytes))
	if err != nil {
		return nil, err
	}
	return ParseEnvelope(data)
}
