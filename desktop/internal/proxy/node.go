// Package proxy 负责把用户填写的代理配置 (URL、分享链接、Clash 节点) 变成浏览器能用的出口:
// 普通 http / socks5 直接交给浏览器; vmess / vless / trojan / shadowsocks 以及带账号密码的
// socks5 / http 由本机 Xray 转成无认证 socks5; hysteria2 / tuic 由 sing-box 转换。
package proxy

import (
	"net"
	"strconv"
	"strings"
)

// Node 是解析后的代理节点, 与来源格式无关。部分字段只决定输出形状 (见 xrayOutbound)。
type Node struct {
	Kind   string // http https socks5 vmess vless trojan shadowsocks hysteria2 tuic
	Server string
	Port   int

	User     string // http / socks5 用户名
	Password string // http / socks5 / trojan / shadowsocks / hysteria2 / tuic 密码
	UUID     string // vmess / vless / tuic
	Cipher   string // vmess 加密方式或 shadowsocks method
	Flow     string // vless flow

	TLS       TLSOptions
	Transport Transport

	// sing-box 专用
	ObfsPassword string
	UpMbps       *int
	DownMbps     *int
	ALPN         []string
}

// TLSOptions 描述节点的传输层加密。
type TLSOptions struct {
	Mode       string // "" / "tls" / "reality"
	ServerName string
	// KeepEmptyName 为 true 时即使 ServerName 为空也输出该字段 (trojan 的写法)
	KeepEmptyName bool
	// Insecure 为 nil 表示不输出 allowInsecure
	Insecure    *bool
	Fingerprint string
	PublicKey   string
	ShortID     string
}

// Transport 描述 ws / grpc 传输。
type Transport struct {
	Network     string // "" / "ws" / "grpc"
	Path        string
	Host        string
	ServiceName string
	// OmitEmptyWS 为 true 时 ws 没有任何参数就不输出 wsSettings
	OmitEmptyWS bool
}

// Address 返回 host:port, 用于直接测 TCP 连通性。
func (n Node) Address() string {
	return net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
}

// UsesSingBox 表示该节点只能交给 sing-box。
func (n Node) UsesSingBox() bool { return n.Kind == "hysteria2" || n.Kind == "tuic" }

// IsPlain 表示 http / https / socks5 这类浏览器可直接使用的代理。
func (n Node) IsPlain() bool { return n.Kind == "http" || n.Kind == "https" || n.Kind == "socks5" }

func boolPtr(v bool) *bool { return &v }

// hasScheme 不区分大小写判断前缀。
func hasScheme(s string, schemes ...string) bool {
	l := strings.ToLower(s)
	for _, sc := range schemes {
		if strings.HasPrefix(l, sc) {
			return true
		}
	}
	return false
}

// isPlainURL 判断是否为 http / https / socks5 形式的代理地址。
func isPlainURL(s string) bool { return hasScheme(s, "http://", "https://", "socks5://") }

// looksLikeClash 判断输入是否像 Clash 节点 (YAML 或 clash:// 分享链接)。
func looksLikeClash(s string) bool {
	l := strings.ToLower(s)
	return strings.HasPrefix(l, "clash://") || strings.Contains(l, "type:") || strings.Contains(l, "proxies:")
}
