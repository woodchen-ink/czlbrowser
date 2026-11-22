package proxy

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// clashFields 是 Clash 节点 YAML 的键值, 取值时统一做类型宽容。
type clashFields map[string]any

func asFields(v any) clashFields {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := make(clashFields, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out
	}
	return nil
}

func (f clashFields) str(key string) string {
	switch v := f[key].(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.Itoa(int(v))
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func (f clashFields) num(key string) int {
	switch v := f[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func (f clashFields) flag(key string) bool {
	switch v := f[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	case int:
		return v != 0
	case float64:
		return v != 0
	}
	return false
}

func (f clashFields) sub(key string) clashFields { return asFields(f[key]) }

func (f clashFields) firstStr(keys ...string) string {
	for _, k := range keys {
		if s := f.str(k); s != "" {
			return s
		}
	}
	return ""
}

// list 读取字符串或字符串数组。
func (f clashFields) list(key string) []string {
	switch v := f[key].(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// loadClashNode 读取 Clash YAML 或 clash://base64(YAML), 取出第一个节点。
// 支持 proxies 列表、单个 proxy 键、裸节点对象与裸数组。
func loadClashNode(src string) (clashFields, error) {
	text := strings.TrimSpace(src)
	if hasScheme(text, "clash://") {
		esc, _ := url.QueryUnescape(text[len("clash://"):])
		b, err := decodeBase64(esc)
		if err != nil {
			return nil, err
		}
		text = string(b)
	}
	var doc any
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	if list, ok := doc.([]any); ok {
		if len(list) == 0 {
			return nil, errors.New("clash 节点解析失败")
		}
		if node := asFields(list[0]); node != nil {
			return node, nil
		}
		return nil, errors.New("clash 节点解析失败")
	}
	root := asFields(doc)
	if root == nil {
		return nil, errors.New("clash 节点解析失败")
	}
	if list, ok := root["proxies"].([]any); ok && len(list) > 0 {
		if node := asFields(list[0]); node != nil {
			return node, nil
		}
		return nil, errors.New("clash 节点解析失败")
	}
	if node := root.sub("proxy"); node != nil {
		return node, nil
	}
	return root, nil
}

// clashTransport 读取 ws-opts / grpc-opts。
func clashTransport(f clashFields) Transport {
	switch f.str("network") {
	case "ws":
		t := Transport{Network: "ws"}
		if ws := f.sub("ws-opts"); ws != nil {
			t.Path = ws.str("path")
			if h := ws.sub("headers"); h != nil {
				t.Host = h.str("Host")
			}
		}
		return t
	case "grpc":
		t := Transport{Network: "grpc"}
		if g := f.sub("grpc-opts"); g != nil {
			t.ServiceName = g.str("grpc-service-name")
		}
		return t
	}
	return Transport{}
}

// clashToNode 把 Clash 节点转成 Node。
func clashToNode(f clashFields) (Node, error) {
	sni := f.firstStr("sni", "servername")
	base := Node{Server: f.str("server"), Port: f.num("port")}

	switch kind := strings.ToLower(f.str("type")); kind {
	case "socks5", "http", "https":
		n := base
		n.Kind, n.User, n.Password = kind, f.str("username"), f.str("password")
		return n, nil

	case "vmess":
		n := base
		n.Kind, n.UUID, n.Cipher = "vmess", f.str("uuid"), f.str("cipher")
		if n.Cipher == "" {
			n.Cipher = "auto"
		}
		if f.flag("tls") {
			n.TLS = TLSOptions{Mode: "tls", ServerName: sni, Insecure: boolPtr(f.flag("skip-cert-verify"))}
		}
		n.Transport = clashTransport(f)
		return n, nil

	case "vless":
		n := base
		n.Kind, n.UUID, n.Flow = "vless", f.str("uuid"), f.str("flow")
		if _, reality := f["reality-opts"]; reality {
			n.TLS = TLSOptions{Mode: "reality", ServerName: sni, Fingerprint: f.str("client-fingerprint")}
			if n.TLS.Fingerprint == "" {
				n.TLS.Fingerprint = "chrome"
			}
			if r := f.sub("reality-opts"); r != nil {
				n.TLS.PublicKey, n.TLS.ShortID = r.str("public-key"), r.str("short-id")
			}
		} else if f.flag("tls") || strings.EqualFold(f.str("tls"), "tls") {
			n.TLS = TLSOptions{Mode: "tls", ServerName: sni, Insecure: boolPtr(f.flag("skip-cert-verify"))}
		}
		n.Transport = clashTransport(f)
		return n, nil

	case "trojan":
		n := base
		n.Kind, n.Password = "trojan", f.str("password")
		n.TLS = TLSOptions{Mode: "tls", ServerName: sni, KeepEmptyName: true, Insecure: boolPtr(f.flag("skip-cert-verify"))}
		n.Transport = clashTransport(f)
		return n, nil

	case "ss", "shadowsocks":
		n := base
		n.Kind, n.Password = "shadowsocks", f.str("password")
		n.Cipher = f.firstStr("cipher", "method")
		if n.Cipher == "" {
			n.Cipher = "aes-256-gcm"
		}
		// obfs / v2ray-plugin 之类的插件 Xray 不支持, 忽略
		return n, nil

	case "hysteria2", "hysteria":
		n := base
		n.Kind, n.Password = "hysteria2", f.str("password")
		n.TLS = TLSOptions{ServerName: sni, Insecure: boolPtr(f.flag("skip-cert-verify"))}
		n.ObfsPassword = f.str("obfs-password")
		if s := f.str("up"); s != "" {
			v := mbps(s)
			n.UpMbps = &v
		}
		if s := f.str("down"); s != "" {
			v := mbps(s)
			n.DownMbps = &v
		}
		return n, nil

	case "tuic":
		n := base
		n.Kind, n.UUID, n.Password = "tuic", f.str("uuid"), f.str("password")
		n.TLS = TLSOptions{ServerName: f.str("sni"), Insecure: boolPtr(f.flag("skip-cert-verify"))}
		n.ALPN = f.list("alpn")
		return n, nil

	case "ssr":
		return Node{}, errSSRUnsupported
	default:
		return Node{}, errors.New("不支持的节点类型")
	}
}

// mbps 把 "100 Mbps" / "100M" / "100" 这类带宽写法转成整数 Mbps。
func mbps(s string) int {
	s = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	for _, suffix := range []string{"BPS", "B", "M"} {
		s = strings.TrimSuffix(s, suffix)
	}
	n, _ := strconv.Atoi(s)
	return n
}
