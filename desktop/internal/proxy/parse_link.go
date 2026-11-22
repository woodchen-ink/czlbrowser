package proxy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var errSSRUnsupported = errors.New("不支持 ShadowsocksR (SSR), 请改用 SS / VMess / VLESS / Trojan 节点")

// decodeBase64 依次尝试标准与 URL 安全、带或不带填充的 base64。
func decodeBase64(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("base64 内容为空")
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("不是有效的 base64")
}

// parseVmessLink 解析 vmess://base64(JSON)。
func parseVmessLink(s string) (Node, error) {
	raw, err := decodeBase64(strings.TrimSpace(s[len("vmess://"):]))
	if err != nil {
		return Node{}, fmt.Errorf("vmess 解析失败: %v", err)
	}
	var v struct {
		Add  string          `json:"add"`
		Port json.RawMessage `json:"port"`
		ID   string          `json:"id"`
		Net  string          `json:"net"`
		Host string          `json:"host"`
		Path string          `json:"path"`
		TLS  string          `json:"tls"`
		SNI  string          `json:"sni"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Node{}, fmt.Errorf("vmess 配置解析失败: %v", err)
	}
	n := Node{Kind: "vmess", Server: v.Add, Port: jsonPort(v.Port), UUID: v.ID, Cipher: "auto"}
	if v.TLS == "tls" {
		n.TLS = TLSOptions{Mode: "tls", ServerName: v.SNI}
	}
	if v.Net == "ws" {
		n.Transport = Transport{Network: "ws", Path: v.Path, Host: v.Host, OmitEmptyWS: true}
	}
	return n, nil
}

// jsonPort 兼容 "443" 与 443 两种写法。
func jsonPort(raw json.RawMessage) int {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	p, _ := strconv.Atoi(s)
	return p
}

// parseVlessLink 解析 vless://uuid@host:port?参数。reality 按 tls 处理 (链接里缺少 reality 公钥)。
func parseVlessLink(s string) (Node, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Node{}, fmt.Errorf("vless 解析失败: %v", err)
	}
	q := u.Query()
	port, _ := strconv.Atoi(u.Port())
	n := Node{Kind: "vless", Server: u.Hostname(), Port: port, UUID: u.User.Username(), Flow: q.Get("flow")}
	switch strings.ToLower(q.Get("security")) {
	case "tls", "reality":
		n.TLS = TLSOptions{Mode: "tls", ServerName: q.Get("sni")}
	}
	network := q.Get("type")
	if network == "" {
		network = q.Get("network")
	}
	if network == "ws" {
		host := q.Get("host")
		if host == "" {
			host = u.Hostname()
		}
		n.Transport = Transport{Network: "ws", Path: q.Get("path"), Host: host}
	}
	return n, nil
}

// parseTrojanLink 解析 trojan://password@host:port?参数。
func parseTrojanLink(s string) (Node, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Node{}, fmt.Errorf("trojan 解析失败: %v", err)
	}
	q := u.Query()
	port, _ := strconv.Atoi(u.Port())
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer")
	}
	insecure := q.Get("allowInsecure")
	n := Node{
		Kind: "trojan", Server: u.Hostname(), Port: port, Password: u.User.Username(),
		TLS: TLSOptions{Mode: "tls", ServerName: sni, KeepEmptyName: true,
			Insecure: boolPtr(insecure == "1" || strings.EqualFold(insecure, "true"))},
	}
	if q.Get("type") == "ws" {
		n.Transport = Transport{Network: "ws", Path: q.Get("path"), Host: q.Get("host")}
	}
	return n, nil
}

// parseShadowsocksLink 解析两种 ss:// 写法:
//   - SIP002: ss://base64(method:password)@host:port 或明文 method:password@host:port
//   - 旧式:   ss://base64(method:password@host:port)
func parseShadowsocksLink(s string) (Node, error) {
	body := s[len("ss://"):]
	if i := strings.IndexByte(body, '#'); i >= 0 {
		body = body[:i]
	}
	body = strings.TrimSpace(body)

	n := Node{Kind: "shadowsocks"}
	if strings.Contains(body, "@") {
		u, err := url.Parse("ss://" + body)
		if err != nil {
			return Node{}, fmt.Errorf("ss 解析失败: %v", err)
		}
		n.Server = u.Hostname()
		n.Port, _ = strconv.Atoi(u.Port())
		cred := u.User.String()
		if b, err := decodeBase64(cred); err == nil {
			cred = string(b)
		}
		n.Cipher, n.Password, _ = strings.Cut(cred, ":")
		if !strings.Contains(cred, ":") {
			n.Cipher = ""
		}
	} else {
		b, err := decodeBase64(body)
		if err != nil {
			return Node{}, fmt.Errorf("ss base64 解析失败: %v", err)
		}
		plain := string(b)
		at := strings.LastIndexByte(plain, '@')
		if at < 0 {
			return Node{}, errors.New("ss 格式错误")
		}
		if method, pass, ok := strings.Cut(plain[:at], ":"); ok {
			n.Cipher, n.Password = method, pass
		}
		if hp := strings.Split(plain[at+1:], ":"); len(hp) == 2 {
			n.Server = hp[0]
			n.Port, _ = strconv.Atoi(hp[1])
		}
	}
	if n.Server == "" || n.Port == 0 || n.Cipher == "" {
		return Node{}, errors.New("ss 节点信息不完整")
	}
	return n, nil
}

// parseHysteria2Link 解析 hysteria2:// (hysteria:// 视为同义)。
func parseHysteria2Link(s string) (Node, error) {
	u, err := url.Parse(normalizeNodeScheme(s))
	if err != nil {
		return Node{}, fmt.Errorf("hysteria2 链接解析失败: %v", err)
	}
	q := u.Query()
	port, _ := strconv.Atoi(u.Port())
	if u.Hostname() == "" || port == 0 {
		return Node{}, fmt.Errorf("hysteria2 节点信息不完整: host=%s port=%d", u.Hostname(), port)
	}
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer")
	}
	insecure := q.Get("insecure")
	return Node{
		Kind: "hysteria2", Server: u.Hostname(), Port: port, Password: u.User.Username(),
		TLS:          TLSOptions{ServerName: sni, Insecure: boolPtr(insecure == "1" || strings.EqualFold(insecure, "true"))},
		ObfsPassword: q.Get("obfs-password"),
	}, nil
}

// parsePlainURL 解析 http / https / socks5 地址。端口缺省时为 0。
func parsePlainURL(s string) (Node, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Node{}, fmt.Errorf("代理地址解析失败: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())
	n := Node{Kind: strings.ToLower(u.Scheme), Server: u.Hostname(), Port: port}
	if u.User != nil {
		n.User = strings.TrimSpace(u.User.Username())
		n.Password, _ = u.User.Password()
	}
	return n, nil
}

// normalizeNodeScheme 把 hysteria:// 统一成 hysteria2://。
func normalizeNodeScheme(s string) string {
	s = strings.TrimSpace(s)
	if hasScheme(s, "hysteria://") {
		return "hysteria2://" + s[len("hysteria://"):]
	}
	return s
}
