package browser

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// windowsProxy 读取 Windows「设置 → 代理」里的手动代理 (Clash 等工具开系统代理时写这里)。
// Go 的 http.ProxyFromEnvironment 只看环境变量, 看不到这个设置。
func windowsProxy() (*url.URL, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()
	if on, _, err := k.GetIntegerValue("ProxyEnable"); err != nil || on == 0 {
		return nil, errors.New("系统代理未启用")
	}
	server, _, err := k.GetStringValue("ProxyServer")
	server = strings.TrimSpace(server)
	if err != nil || !strings.Contains(server, ":") {
		return nil, errors.New("系统代理地址无效")
	}
	// 也可能是 "http=host:port;https=host:port" 的分协议写法, 取 https 或第一段
	if strings.Contains(server, "=") {
		parts := map[string]string{}
		for _, seg := range strings.Split(server, ";") {
			if k, v, ok := strings.Cut(seg, "="); ok {
				parts[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
			}
		}
		switch {
		case parts["https"] != "":
			server = parts["https"]
		case parts["http"] != "":
			server = parts["http"]
		case parts["socks"] != "":
			server = "socks5://" + parts["socks"]
		}
	}
	if !strings.HasPrefix(server, "http") && !strings.HasPrefix(server, "socks") {
		server = "http://" + server
	}
	return url.Parse(server)
}

// SystemProxyURL 可直接用作 http.Transport.Proxy: 优先 Windows 系统代理, 其次环境变量, 都没有则直连。
func SystemProxyURL(req *http.Request) (*url.URL, error) {
	if u, err := windowsProxy(); err == nil {
		return u, nil
	}
	return http.ProxyFromEnvironment(req)
}
