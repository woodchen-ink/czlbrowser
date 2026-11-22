package proxy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

// ParseNode 把任意支持的代理写法解析成 Node。
func ParseNode(src string) (Node, error) {
	s := strings.TrimSpace(src)
	switch {
	case s == "":
		return Node{}, errors.New("代理节点为空")
	case isPlainURL(s):
		return parsePlainURL(s)
	case hasScheme(s, "vmess://"):
		return parseVmessLink(s)
	case hasScheme(s, "vless://"):
		return parseVlessLink(s)
	case hasScheme(s, "trojan://"):
		return parseTrojanLink(s)
	case hasScheme(s, "ss://"):
		return parseShadowsocksLink(s)
	case hasScheme(s, "ssr://"):
		return Node{}, errSSRUnsupported
	case hasScheme(s, "hysteria2://", "hysteria://"):
		return parseHysteria2Link(s)
	case looksLikeClash(s):
		f, err := loadClashNode(s)
		if err != nil {
			return Node{}, err
		}
		return clashToNode(f)
	}
	return Node{}, errors.New("不支持的节点协议")
}

// ParseProxyNode 返回浏览器可直接使用的代理地址, 或需要 Xray 转换的出站配置 (二者只有一个非空)。
// hysteria2 / tuic 返回错误, 它们走 BuildSingBoxOutbound。
func ParseProxyNode(src string) (string, map[string]interface{}, error) {
	s := strings.TrimSpace(src)
	if isPlainURL(s) {
		return s, nil, nil
	}
	n, err := ParseNode(s)
	if err != nil {
		return "", nil, err
	}
	if n.IsPlain() {
		if u := plainURL(n); u != "" {
			return u, nil, nil
		}
		return "", nil, errors.New("不支持的节点协议")
	}
	out, err := xrayOutbound(n)
	if err != nil {
		return "", nil, err
	}
	return "", out, nil
}

// IsSingBoxProtocol 判断配置是否为 hysteria / hysteria2 / tuic (由 sing-box 处理)。
// 只看文本特征, 不做完整解析, 调用方据此选择桥接方式。
func IsSingBoxProtocol(proxyConfig string) bool {
	s := strings.ToLower(strings.TrimSpace(proxyConfig))
	if strings.HasPrefix(s, "hysteria2://") || strings.HasPrefix(s, "hysteria://") {
		return true
	}
	compact := strings.ReplaceAll(s, "type: ", "type:")
	return strings.Contains(compact, "type:hysteria") || strings.Contains(compact, "type:tuic")
}

// BuildSingBoxOutbound 解析 hysteria2 / tuic 节点, 返回 sing-box 出站配置。
func BuildSingBoxOutbound(src string) (map[string]interface{}, error) {
	s := strings.TrimSpace(src)
	if !hasScheme(s, "hysteria2://", "hysteria://") && !looksLikeClash(s) {
		return nil, errors.New("不支持的 sing-box 节点格式")
	}
	n, err := ParseNode(s)
	if err != nil {
		return nil, err
	}
	if !n.UsesSingBox() {
		return nil, fmt.Errorf("不支持的 sing-box 节点类型: %s", n.Kind)
	}
	return singBoxOutbound(n)
}

// authBridgeOutbound 带账号密码的 http / socks5 代理: Chrome 不支持 socks5 认证,
// http 认证会弹窗, 所以统一经 Xray 转成本机无认证 socks5。
func authBridgeOutbound(src string) (map[string]interface{}, bool) {
	if !hasScheme(src, "http://", "socks5://") {
		return nil, false
	}
	n, err := parsePlainURL(strings.TrimSpace(src))
	if err != nil || n.User == "" || n.Server == "" || n.Port <= 0 {
		return nil, false
	}
	out, err := xrayOutbound(n)
	return out, err == nil
}

// lookupProxy 有 proxyId 时以代理池里的配置为准。
func lookupProxy(proxyConfig string, proxies []config.BrowserProxy, proxyID string) (config.BrowserProxy, bool) {
	if proxyID == "" {
		return config.BrowserProxy{ProxyConfig: strings.TrimSpace(proxyConfig)}, true
	}
	for _, p := range proxies {
		if strings.EqualFold(p.ProxyId, proxyID) {
			p.ProxyConfig = strings.TrimSpace(p.ProxyConfig)
			return p, true
		}
	}
	return config.BrowserProxy{ProxyConfig: strings.TrimSpace(proxyConfig)}, false
}

// RequiresBridge 判断该代理是否需要 Xray 转换。
func RequiresBridge(proxyConfig string, proxies []config.BrowserProxy, proxyID string) bool {
	p, _ := lookupProxy(proxyConfig, proxies, proxyID)
	src := p.ProxyConfig
	switch {
	case src == "":
		return false
	case isPlainURL(src):
		_, ok := authBridgeOutbound(src)
		return ok
	case IsSingBoxProtocol(src):
		return false
	case hasScheme(src, "vmess://", "vless://", "trojan://", "ss://"):
		return true
	}
	return looksLikeClash(src)
}

// ValidateProxyConfig 启动实例前检查代理能否使用, 不能用时返回给用户看的原因。
func ValidateProxyConfig(proxyConfig string, proxies []config.BrowserProxy, proxyID string) (bool, string) {
	p, found := lookupProxy(proxyConfig, proxies, proxyID)
	if !found {
		return false, fmt.Sprintf("代理链路不可用：代理池节点已不存在（proxyId=%s）。可能因订阅刷新后节点下线或被删除，请重新选择代理后再启动。", proxyID)
	}
	src := p.ProxyConfig
	if src == "" || strings.EqualFold(src, "direct://") || isPlainURL(src) {
		return true, ""
	}
	var err error
	if IsSingBoxProtocol(src) {
		_, err = BuildSingBoxOutbound(src)
	} else {
		_, _, err = ParseProxyNode(src)
	}
	if err != nil {
		return false, fmt.Sprintf("代理配置解析失败: %v", err)
	}
	return true, ""
}

// endpointOf 返回代理服务器的 host:port, 用于 TCP 测速。
func endpointOf(src string) (string, error) {
	n, err := ParseNode(src)
	if err != nil {
		// 不支持的 Clash 节点类型 (如 ssr / wireguard) 仍能测服务器是否可达
		f, cerr := loadClashNode(src)
		if !looksLikeClash(src) || cerr != nil {
			return "", err
		}
		n = Node{Server: f.str("server"), Port: f.num("port")}
	}
	if n.Server == "" || n.Port == 0 {
		return "", errors.New("缺少服务器地址或端口")
	}
	return n.Address(), nil
}
