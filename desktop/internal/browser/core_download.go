package browser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

// DownloadProgress 是 download:progress 事件的内容。
type DownloadProgress struct {
	Phase    string `json:"phase"`    // downloading / extracting / done / error
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
}

// downloadProxyMode 是内核下载可选的代理方式 (前端传入)。
const (
	proxySystem = "__system__"
	proxyDirect = "__direct__"
)

// DownloadAndExtractCore 下载内核压缩包到 chrome/<name>, 校验后解压并登记。
// 全程通过 download:progress 事件报告进度, 适合放在 goroutine 里调用。
// sha256 非空时先校验压缩包; proxy 为 __system__ / 空 / 直连 / 具体代理地址。
func (m *Manager) DownloadAndExtractCore(ctx context.Context, name, src, sha string, proxy string) {
	emit := func(phase string, percent int, msg string) {
		runtime.EventsEmit(ctx, "download:progress", DownloadProgress{Phase: phase, Progress: percent, Message: msg})
	}
	fail := func(msg string) { emit("error", 0, msg) }
	log := logger.New("Browser")
	started := time.Now()

	name = strings.TrimSpace(name)
	for _, c := range m.ListCores() {
		if strings.EqualFold(c.CoreName, name) || filepath.Base(c.CorePath) == name {
			fail("名称已存在，请换一个名称")
			return
		}
	}
	kernels := m.ResolveRelativePath("chrome")
	if err := os.MkdirAll(kernels, 0o755); err != nil {
		fail("创建 chrome 目录失败")
		return
	}
	dest := filepath.Join(kernels, name)
	if _, err := os.Stat(dest); err == nil {
		fail("同名文件夹已存在: " + name)
		return
	}

	client, note, err := downloadClient(proxy)
	if err != nil {
		fail(err.Error())
		return
	}
	emit("downloading", 0, "开始下载: "+src)
	if note != "" {
		emit("downloading", 0, note)
	}

	tmp, err := os.CreateTemp(kernels, "download_*.zip")
	if err != nil {
		fail("创建临时文件失败: " + err.Error())
		return
	}
	defer os.Remove(tmp.Name())
	err = fetchToFile(ctx, client, src, tmp, func(done, total int64, parallel bool) {
		mode := "单线程下载中"
		if parallel {
			mode = "分段并行下载中"
		}
		percent := 0
		if total > 0 {
			percent = int(done * 100 / total)
		}
		emit("downloading", percent, fmt.Sprintf("%s... %.2f MB / %.2f MB", mode, mb(done), mb(total)))
	})
	tmp.Close()
	if err != nil {
		fail("下载失败: " + err.Error())
		return
	}

	if sha = strings.TrimSpace(sha); sha != "" {
		emit("extracting", 0, "下载完成，正在校验文件完整性(SHA256)...")
		got, err := sha256File(tmp.Name())
		if err != nil {
			fail("校验文件失败: " + err.Error())
			return
		}
		if !strings.EqualFold(got, sha) {
			log.Error("内核压缩包 SHA256 不一致", logger.F("url", src), logger.F("expected", sha), logger.F("actual", got))
			fail("文件校验失败：SHA256 与发布值不一致，下载可能损坏或被篡改，请重试")
			return
		}
	}
	log.Info("内核下载完成", logger.F("url", src), logger.F("cost", time.Since(started).String()))

	emit("extracting", 0, "正在解压...")
	if err := unzipFlatten(tmp.Name(), dest, func(p int, msg string) { emit("extracting", p, msg) }); err != nil {
		os.RemoveAll(dest)
		fail("解压失败: " + err.Error())
		return
	}
	rel := filepath.Join("chrome", name)
	if !m.ValidateCorePath(rel).Valid {
		os.RemoveAll(dest)
		fail("解压后在目录未找到 chrome.exe 执行文件，请检查压缩包内容！")
		return
	}
	if err := m.RegisterDownloadedCore(name, rel); err != nil {
		fail("保存配置入库失败: " + err.Error())
		return
	}
	emit("done", 100, "内核下载与配置成功！")
}

// downloadClient 按代理方式构造下载客户端 (不设整体超时, 靠 ctx 取消)。
func downloadClient(proxy string) (*http.Client, string, error) {
	tr := &http.Transport{}
	note := ""
	switch p := strings.TrimSpace(proxy); p {
	case "", "direct://", proxyDirect:
	case proxySystem:
		if u, err := windowsProxy(); err == nil {
			tr.Proxy = http.ProxyURL(u)
			note = "使用系统代理: " + u.String()
		} else {
			tr.Proxy = http.ProxyFromEnvironment
			note = "未检测到系统代理，按环境变量设置连接"
		}
	default:
		u, err := url.Parse(p)
		if err != nil {
			return nil, "", fmt.Errorf("代理地址解析失败: %v", err)
		}
		tr.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Transport: tr}, note, nil
}

func mb(n int64) float64 { return float64(n) / (1 << 20) }

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
