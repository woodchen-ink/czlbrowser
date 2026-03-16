package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"

	xproxy "golang.org/x/net/proxy"
)

const defaultTestURL = "https://www.google.com/generate_204"

// SpeedTestConfig 测速参数
type SpeedTestConfig struct {
	Timeout    time.Duration
	TCPTimeout time.Duration
	URLs       []string
}

var DefaultSpeedTestConfig = SpeedTestConfig{
	Timeout:    10 * time.Second,
	TCPTimeout: 5 * time.Second,
}

// SpeedTest 对 socks5/http 代理进行 HTTP 延迟测速。
// 禁用 keep-alive，每次请求走完整链路（TCP 连接 → TLS → HTTP），测出真实代理延迟。
// 不支持的代理格式直接降级到 TCP ping。
func SpeedTest(
	proxyId string,
	proxies []config.BrowserProxy,
	cfg *SpeedTestConfig,
) TestResult {
	log := logger.New("SpeedTest")

	if cfg == nil {
		c := DefaultSpeedTestConfig
		cfg = &c
	}

	// 查找代理配置
	src := ""
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			src = strings.TrimSpace(item.ProxyConfig)
			break
		}
	}
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Error: "代理配置为空"}
	}

	if strings.ToLower(src) == "direct://" {
		return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: 0}
	}

	testURL := defaultTestURL
	if len(cfg.URLs) > 0 {
		testURL = cfg.URLs[0]
	}

	// 构建 HTTP 客户端（仅支持 socks5/http/https，禁用 keep-alive）
	client, err := buildSimpleProxyHTTPClient(src, cfg.Timeout)
	if err != nil {
		log.Debug("HTTP 客户端构建失败，降级到 TCP ping",
			logger.F("proxy_id", proxyId),
			logger.F("error", err.Error()),
		)
		return tcpPingFallback(proxyId, src, cfg.TCPTimeout, log)
	}

	// 直接计时完整请求（TCP + TLS + HTTP）
	start := time.Now()
	req, _ := http.NewRequest(http.MethodHead, testURL, nil)
	req.Header.Set("User-Agent", "AntChrome/1.0")
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: err.Error()}
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency,
			Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

// buildSimpleProxyHTTPClient 根据 socks5/http/https 代理 URL 构建 HTTP 客户端。
// 不支持 Clash YAML、vmess、vless 等复杂格式。
func buildSimpleProxyHTTPClient(src string, timeout time.Duration) (*http.Client, error) {
	l := strings.ToLower(strings.TrimSpace(src))

	if strings.HasPrefix(l, "socks5://") {
		u, err := url.Parse(src)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 地址解析失败: %w", err)
		}
		if strings.TrimSpace(u.Host) == "" {
			return nil, fmt.Errorf("SOCKS5 地址解析失败: host 为空")
		}
		var auth *xproxy.Auth
		if u.User != nil {
			pass, _ := u.User.Password()
			auth = &xproxy.Auth{User: u.User.Username(), Password: pass}
		}
		dialer, err := xproxy.SOCKS5("tcp", u.Host, auth, xproxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
		}
		contextDialer, ok := dialer.(xproxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5 dialer 不支持 ContextDialer")
		}
		transport := &http.Transport{
			DialContext:       contextDialer.DialContext,
			DisableKeepAlives: true,
		}
		return &http.Client{Transport: transport, Timeout: timeout}, nil
	}

	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		proxyURL, err := url.Parse(src)
		if err != nil {
			return nil, fmt.Errorf("代理地址解析失败: %w", err)
		}
		transport := &http.Transport{
			Proxy:             http.ProxyURL(proxyURL),
			DisableKeepAlives: true,
		}
		return &http.Client{Transport: transport, Timeout: timeout}, nil
	}

	return nil, fmt.Errorf("不支持的代理格式（仅支持 socks5/http/https）: %s", l[:min(30, len(l))])
}

// ─── TCP Ping 降级 ───

func tcpPingFallback(proxyId, src string, timeout time.Duration, log *logger.Logger) TestResult {
	endpoint, err := proxyEndpoint(src)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("无法解析代理地址: %v", err)}
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: fmt.Sprintf("TCP 连接失败: %v", err)}
	}
	conn.Close()
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
