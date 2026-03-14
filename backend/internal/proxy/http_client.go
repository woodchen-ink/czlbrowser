package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"

	xproxy "golang.org/x/net/proxy"
)

// buildProxyHTTPClient 根据代理配置构建 HTTP 客户端，统一用于测速/健康检测场景。
func buildProxyHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	timeout time.Duration,
) (*http.Client, error) {
	l := strings.ToLower(strings.TrimSpace(src))
	if l == "" || l == "direct://" {
		return &http.Client{Timeout: timeout}, nil
	}

	if IsSingBoxProtocol(src) {
		if singboxMgr == nil {
			return nil, fmt.Errorf("sing-box 管理器未初始化")
		}
		socks5Addr, err := singboxMgr.EnsureBridge(src, proxies, proxyId)
		if err != nil {
			return nil, fmt.Errorf("sing-box 桥接启动失败: %w", err)
		}
		return buildSocks5HTTPClient(socks5Addr, timeout)
	}

	if RequiresBridge(src, proxies, proxyId) {
		if xrayMgr == nil {
			return nil, fmt.Errorf("xray 管理器未初始化")
		}
		socks5Addr, err := xrayMgr.EnsureBridge(src, proxies, proxyId)
		if err != nil {
			return nil, fmt.Errorf("xray 桥接启动失败: %w", err)
		}
		return buildSocks5HTTPClient(socks5Addr, timeout)
	}

	if strings.HasPrefix(l, "socks5://") {
		return buildSocks5HTTPClient(src, timeout)
	}

	proxyURL, err := url.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func buildSocks5HTTPClient(socks5Addr string, timeout time.Duration) (*http.Client, error) {
	raw := strings.TrimSpace(socks5Addr)
	if raw == "" {
		return nil, fmt.Errorf("SOCKS5 地址为空")
	}
	if !strings.HasPrefix(strings.ToLower(raw), "socks5://") {
		raw = "socks5://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 地址解析失败: %w", err)
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, fmt.Errorf("SOCKS5 地址解析失败: host 为空")
	}

	var auth *xproxy.Auth
	if u.User != nil {
		pass, _ := u.User.Password()
		auth = &xproxy.Auth{
			User:     u.User.Username(),
			Password: pass,
		}
	}

	dialer, err := xproxy.SOCKS5("tcp", u.Host, auth, xproxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
	}
	contextDialer, ok := dialer.(xproxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer 不支持 ContextDialer")
	}
	transport := &http.Transport{DialContext: contextDialer.DialContext}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}
