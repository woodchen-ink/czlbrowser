package proxy

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	xproxy "golang.org/x/net/proxy"
)

// userAgent 是本程序对外 HTTP 请求的 UA。
const userAgent = "CZLBrowser/1.0"

// TestResult 是一次测速的结果。
type TestResult struct {
	ProxyId   string
	Ok        bool
	LatencyMs int64
	Error     string
}

// SpeedTestConfig 测速参数, 零值字段使用默认值。
type SpeedTestConfig struct {
	Timeout    time.Duration // HTTP 测速超时
	TCPTimeout time.Duration // 回退到 TCP 握手时的超时
	URLs       []string      // 只用第一个
}

const defaultProbeURL = "https://www.google.com/generate_204"

func (c *SpeedTestConfig) withDefaults() SpeedTestConfig {
	out := SpeedTestConfig{Timeout: 10 * time.Second, TCPTimeout: 5 * time.Second, URLs: []string{defaultProbeURL}}
	if c == nil {
		return out
	}
	if c.Timeout > 0 {
		out.Timeout = c.Timeout
	}
	if c.TCPTimeout > 0 {
		out.TCPTimeout = c.TCPTimeout
	}
	if len(c.URLs) > 0 {
		out.URLs = c.URLs
	}
	return out
}

func tcpHandshake(proxyID, addr string, timeout time.Duration) TestResult {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{ProxyId: proxyID, LatencyMs: ms, Error: err.Error()}
	}
	_ = conn.Close()
	return TestResult{ProxyId: proxyID, Ok: true, LatencyMs: ms}
}

// SpeedTest 测延迟: http / https / socks5 代理经代理发一次 HEAD 请求 (不复用连接, 计入握手);
// 其它协议不启动转换进程, 直接测服务器 TCP 握手。直连视为 0 延迟。
func SpeedTest(proxyID string, proxies []config.BrowserProxy, cfg *SpeedTestConfig) TestResult {
	opts := cfg.withDefaults()
	p, _ := lookupProxy("", proxies, proxyID)
	src := p.ProxyConfig
	switch {
	case src == "":
		return TestResult{ProxyId: proxyID, Error: "代理配置为空"}
	case strings.EqualFold(src, "direct://"):
		return TestResult{ProxyId: proxyID, Ok: true}
	case !isPlainURL(src):
		addr, err := endpointOf(src)
		if err != nil {
			return TestResult{ProxyId: proxyID, Error: fmt.Sprintf("无法解析代理地址: %v", err)}
		}
		r := tcpHandshake(proxyID, addr, opts.TCPTimeout)
		if !r.Ok {
			r.Error = "TCP 连接失败: " + r.Error
		}
		return r
	}

	client, err := plainProxyClient(src, opts.Timeout, false)
	if err != nil {
		return TestResult{ProxyId: proxyID, Error: err.Error()}
	}
	req, _ := http.NewRequest(http.MethodHead, opts.URLs[0], nil)
	req.Header.Set("User-Agent", userAgent)
	start := time.Now()
	resp, err := client.Do(req)
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{ProxyId: proxyID, LatencyMs: ms, Error: err.Error()}
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return TestResult{ProxyId: proxyID, LatencyMs: ms, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return TestResult{ProxyId: proxyID, Ok: true, LatencyMs: ms}
}

// plainProxyClient 经 http / https / socks5 代理发请求的客户端。
func plainProxyClient(src string, timeout time.Duration, keepAlive bool) (*http.Client, error) {
	u, err := url.Parse(strings.TrimSpace(src))
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("代理地址解析失败: %s", src)
	}
	tr := &http.Transport{DisableKeepAlives: !keepAlive}
	if strings.EqualFold(u.Scheme, "socks5") {
		var auth *xproxy.Auth
		if u.User != nil {
			pass, _ := u.User.Password()
			auth = &xproxy.Auth{User: u.User.Username(), Password: pass}
		}
		d, err := xproxy.SOCKS5("tcp", u.Host, auth, xproxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 连接器创建失败: %w", err)
		}
		cd, ok := d.(xproxy.ContextDialer)
		if !ok {
			return nil, errors.New("SOCKS5 连接器不支持上下文")
		}
		tr.DialContext = cd.DialContext
	} else {
		tr.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Transport: tr, Timeout: timeout}, nil
}

// clientThrough 返回经指定代理发请求的客户端, 需要转换的协议会先拉起 Xray / sing-box。
func clientThrough(src, proxyID string, proxies []config.BrowserProxy, xray *XrayManager, singBox *SingBoxManager, timeout time.Duration) (*http.Client, error) {
	s := strings.TrimSpace(src)
	if s == "" || strings.EqualFold(s, "direct://") {
		return &http.Client{Timeout: timeout}, nil
	}
	addr := s
	switch {
	case IsSingBoxProtocol(s):
		if singBox == nil {
			return nil, errors.New("sing-box 管理器未初始化")
		}
		a, err := singBox.EnsureBridge(s, proxies, proxyID)
		if err != nil {
			return nil, fmt.Errorf("sing-box 桥接启动失败: %w", err)
		}
		addr = a
	case RequiresBridge(s, proxies, proxyID):
		if xray == nil {
			return nil, errors.New("xray 管理器未初始化")
		}
		a, err := xray.EnsureBridge(s, proxies, proxyID)
		if err != nil {
			return nil, fmt.Errorf("xray 桥接启动失败: %w", err)
		}
		addr = a
	}
	return plainProxyClient(addr, timeout, true)
}
