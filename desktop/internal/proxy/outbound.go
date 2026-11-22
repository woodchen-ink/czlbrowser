package proxy

import (
	"errors"
	"fmt"
	"net/url"
)

// outboundTag 是生成的代理出站在 Xray / sing-box 配置里的 tag, 路由规则按它转发。
const outboundTag = "proxy-out"

type obj = map[string]interface{}

// xrayOutbound 把节点写成 Xray 出站配置。
func xrayOutbound(n Node) (obj, error) {
	var settings obj
	switch n.Kind {
	case "vmess":
		settings = obj{"vnext": []interface{}{obj{
			"address": n.Server, "port": n.Port,
			"users": []interface{}{obj{"id": n.UUID, "security": n.Cipher}},
		}}}
	case "vless":
		settings = obj{"vnext": []interface{}{obj{
			"address": n.Server, "port": n.Port,
			"users": []interface{}{obj{"id": n.UUID, "flow": n.Flow, "encryption": "none"}},
		}}}
	case "trojan":
		settings = obj{"address": n.Server, "port": n.Port, "password": n.Password}
	case "shadowsocks":
		settings = obj{"address": n.Server, "port": n.Port, "method": n.Cipher, "password": n.Password}
	case "http", "socks5":
		protocol := "http"
		if n.Kind == "socks5" {
			protocol = "socks"
		}
		return obj{"protocol": protocol, "tag": outboundTag, "settings": obj{"servers": []interface{}{obj{
			"address": n.Server, "port": n.Port,
			"users": []interface{}{obj{"user": n.User, "pass": n.Password}},
		}}}}, nil
	case "hysteria2", "tuic":
		return nil, errors.New("Xray 不支持 hysteria2 / tuic, 这类节点由 sing-box 处理")
	default:
		return nil, fmt.Errorf("Xray 不支持 %q 节点", n.Kind)
	}

	out := obj{"protocol": n.Kind, "tag": outboundTag, "settings": settings}
	if stream := xrayStream(n); len(stream) > 0 {
		out["streamSettings"] = stream
	}
	return out, nil
}

func xrayStream(n Node) obj {
	stream := obj{}
	switch n.TLS.Mode {
	case "tls":
		stream["security"] = "tls"
		tls := obj{}
		if n.TLS.ServerName != "" || n.TLS.KeepEmptyName {
			tls["serverName"] = n.TLS.ServerName
		}
		if n.TLS.Insecure != nil {
			tls["allowInsecure"] = *n.TLS.Insecure
		}
		if len(tls) > 0 {
			stream["tlsSettings"] = tls
		}
	case "reality":
		// reality 只能跑在 tcp 上, 不写 network Xray 校验会失败
		stream["security"] = "reality"
		stream["network"] = "tcp"
		r := obj{"spiderX": "", "fingerprint": n.TLS.Fingerprint}
		if n.TLS.ServerName != "" {
			r["serverName"] = n.TLS.ServerName
		}
		if n.TLS.PublicKey != "" {
			r["publicKey"] = n.TLS.PublicKey
		}
		if n.TLS.ShortID != "" {
			r["shortId"] = n.TLS.ShortID
		}
		stream["realitySettings"] = r
	}

	switch t := n.Transport; t.Network {
	case "ws":
		stream["network"] = "ws"
		ws := obj{}
		if t.Path != "" {
			ws["path"] = t.Path
		}
		if t.Host != "" {
			ws["headers"] = obj{"Host": t.Host}
		}
		if len(ws) > 0 || !t.OmitEmptyWS {
			stream["wsSettings"] = ws
		}
	case "grpc":
		stream["network"] = "grpc"
		if t.ServiceName != "" {
			stream["grpcSettings"] = obj{"serviceName": t.ServiceName}
		}
	}
	return stream
}

// singBoxOutbound 把 hysteria2 / tuic 节点写成 sing-box 出站配置。
func singBoxOutbound(n Node) (obj, error) {
	if n.Server == "" || n.Port == 0 {
		return nil, fmt.Errorf("%s 节点信息不完整", n.Kind)
	}
	tls := obj{"enabled": true, "insecure": n.TLS.Insecure != nil && *n.TLS.Insecure}
	if n.TLS.ServerName != "" {
		tls["server_name"] = n.TLS.ServerName
	}
	out := obj{"type": n.Kind, "tag": outboundTag, "server": n.Server, "server_port": n.Port, "password": n.Password, "tls": tls}

	switch n.Kind {
	case "hysteria2":
		if n.UpMbps != nil {
			out["up_mbps"] = *n.UpMbps
		}
		if n.DownMbps != nil {
			out["down_mbps"] = *n.DownMbps
		}
		if n.ObfsPassword != "" {
			out["obfs"] = obj{"type": "salamander", "password": n.ObfsPassword}
		}
	case "tuic":
		out["uuid"] = n.UUID
		out["congestion_control"] = "bbr"
		if len(n.ALPN) > 0 {
			tls["alpn"] = n.ALPN
		}
	default:
		return nil, fmt.Errorf("sing-box 不处理 %q 节点", n.Kind)
	}
	return out, nil
}

// plainURL 把 Clash 里的 http / https / socks5 节点还原成代理地址; 缺少地址或端口时返回空串。
func plainURL(n Node) string {
	if n.Server == "" || n.Port == 0 {
		return ""
	}
	u := url.URL{Scheme: n.Kind, Host: n.Address()}
	if n.User != "" {
		u.User = url.UserPassword(n.User, n.Password)
	}
	return u.String()
}
