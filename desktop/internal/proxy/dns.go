package proxy

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// xrayDNS 把代理池里的 DNS 设置转成 Xray 的 dns 段, 没有可用地址时返回 nil。
// 接受两种写法: Clash 的 dns 块 (取 nameserver + fallback), 或逗号分隔的地址列表。
// Xray 不支持 tls:// (DoT) 写法, 这类地址会被丢掉。
func xrayDNS(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var candidates []string
	var block struct {
		DNS struct {
			Nameserver []string `yaml:"nameserver"`
			Fallback   []string `yaml:"fallback"`
		} `yaml:"dns"`
	}
	if yaml.Unmarshal([]byte(raw), &block) == nil && len(block.DNS.Nameserver) > 0 {
		candidates = append(block.DNS.Nameserver, block.DNS.Fallback...)
	}
	servers := usableDNS(candidates)
	if len(servers) == 0 {
		servers = usableDNS(strings.Split(raw, ","))
	}
	if len(servers) == 0 {
		return nil
	}
	return map[string]interface{}{"servers": servers}
}

func usableDNS(list []string) []interface{} {
	var out []interface{}
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s != "" && !hasScheme(s, "tls://") {
			out = append(out, s)
		}
	}
	return out
}
