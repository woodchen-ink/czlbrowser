package proxy

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

// testdata/golden.json 记录了各类节点写法应得到的输出 (出站配置、是否需要转换、
// 能否通过校验等), 修改解析逻辑后输出必须与之一致, 除非在下面显式列出原因。
type golden struct {
	Nodes []struct {
		In          string         `json:"in"`
		Std         string         `json:"std"`
		Outbound    map[string]any `json:"outbound"`
		ParseErr    bool           `json:"parseErr"`
		Bridge      bool           `json:"bridge"`
		BridgeOut   map[string]any `json:"bridgeOut"`
		ValidOK     bool           `json:"validOk"`
		SingBox     bool           `json:"singBox"`
		SingBoxOut  map[string]any `json:"singBoxOut"`
		SingBoxErr  bool           `json:"singBoxErr"`
		Endpoint    string         `json:"endpoint"`
		EndpointErr bool           `json:"endpointErr"`
		Normalized  string         `json:"normalized"`
	} `json:"nodes"`
	DNS []struct {
		In  string         `json:"in"`
		Out map[string]any `json:"out"`
	} `json:"dns"`
}

func loadGolden(t *testing.T) golden {
	t.Helper()
	raw, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func normalize(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func TestGoldenNodes(t *testing.T) {
	g := loadGolden(t)
	for _, c := range g.Nodes {
		name := c.In
		if len(name) > 60 {
			name = name[:60]
		}
		t.Run(name, func(t *testing.T) {
			std, out, err := ParseProxyNode(c.In)
			if std != c.Std || (err != nil) != c.ParseErr || !reflect.DeepEqual(normalize(out), c.Outbound) {
				t.Errorf("ParseProxyNode = %q %v err=%v, want %q %v err=%v", std, normalize(out), err, c.Std, c.Outbound, c.ParseErr)
			}

			wantBridge := c.Bridge
			// tuic 由 sing-box 处理, 不需要 Xray 转换 (调用方总是先判断 IsSingBoxProtocol)
			if strings.Contains(c.In, "type: tuic") {
				wantBridge = false
			}
			if got := RequiresBridge(c.In, nil, ""); got != wantBridge {
				t.Errorf("RequiresBridge = %v, want %v", got, wantBridge)
			}

			bo, _ := authBridgeOutbound(c.In)
			if !reflect.DeepEqual(normalize(bo), c.BridgeOut) {
				t.Errorf("authBridgeOutbound = %v, want %v", normalize(bo), c.BridgeOut)
			}
			if ok, msg := ValidateProxyConfig(c.In, []config.BrowserProxy{}, ""); ok != c.ValidOK {
				t.Errorf("ValidateProxyConfig = %v (%s), want %v", ok, msg, c.ValidOK)
			}
			if got := IsSingBoxProtocol(c.In); got != c.SingBox {
				t.Errorf("IsSingBoxProtocol = %v, want %v", got, c.SingBox)
			}
			sb, serr := BuildSingBoxOutbound(normalizeNodeScheme(c.In))
			if (serr != nil) != c.SingBoxErr || !reflect.DeepEqual(normalize(sb), c.SingBoxOut) {
				t.Errorf("BuildSingBoxOutbound = %v err=%v, want %v err=%v", normalize(sb), serr, c.SingBoxOut, c.SingBoxErr)
			}
			if got := normalizeNodeScheme(c.In); got != c.Normalized {
				t.Errorf("normalizeNodeScheme = %q, want %q", got, c.Normalized)
			}
			// 端点: 以前能解析出的必须一致; 以前带 user:pass@ (无法拨号) 或解析不了的现在可以更好
			if !c.EndpointErr && !strings.Contains(c.Endpoint, "@") {
				if got, err := endpointOf(c.In); err != nil || got != c.Endpoint {
					t.Errorf("endpointOf = %q err=%v, want %q", got, err, c.Endpoint)
				}
			}
		})
	}
}

func TestGoldenDNS(t *testing.T) {
	for _, c := range loadGolden(t).DNS {
		if got := normalize(xrayDNS(c.In)); !reflect.DeepEqual(got, c.Out) {
			t.Errorf("xrayDNS(%q) = %v, want %v", c.In, got, c.Out)
		}
	}
}

func TestEndpointImprovements(t *testing.T) {
	cases := map[string]string{
		"socks5://u:p@1.2.3.4:1080":                    "1.2.3.4:1080",
		"trojan://pw@t.example.com:443":                "t.example.com:443",
		"ss://YWVzLTI1Ni1nY206cHc=@s.example.com:8388": "s.example.com:8388",
		"hysteria2://pw@h.example.com:443":             "h.example.com:443",
	}
	for in, want := range cases {
		if got, err := endpointOf(in); err != nil || got != want {
			t.Errorf("endpointOf(%q) = %q err=%v, want %q", in, got, err, want)
		}
	}
}

func TestLookupByProxyID(t *testing.T) {
	pool := []config.BrowserProxy{{ProxyId: "P1", ProxyConfig: " vmess://abc ", DnsServers: "8.8.8.8"}}
	p, ok := lookupProxy("ignored", pool, "p1")
	if !ok || p.ProxyConfig != "vmess://abc" || p.DnsServers != "8.8.8.8" {
		t.Fatalf("lookup = %+v %v", p, ok)
	}
	if ok, msg := ValidateProxyConfig("", pool, "missing"); ok || !strings.Contains(msg, "不存在") {
		t.Fatalf("missing id should fail: %v %s", ok, msg)
	}
	if !RequiresBridge("", []config.BrowserProxy{{ProxyId: "x", ProxyConfig: "socks5://u:p@1.1.1.1:1080"}}, "x") {
		t.Fatal("authenticated socks5 should need a bridge")
	}
}

func TestPlainProxyClient(t *testing.T) {
	for _, src := range []string{"socks5://user:secret@127.0.0.1:1080", "http://127.0.0.1:8080"} {
		if _, err := plainProxyClient(src, 0, true); err != nil {
			t.Errorf("plainProxyClient(%q): %v", src, err)
		}
	}
	if _, err := plainProxyClient("socks5://", 0, true); err == nil {
		t.Error("empty host should fail")
	}
}

func TestSpeedTestDirectAndEmpty(t *testing.T) {
	pool := []config.BrowserProxy{{ProxyId: "d", ProxyConfig: "direct://"}}
	if r := SpeedTest("d", pool, nil); !r.Ok {
		t.Fatalf("direct should be ok: %+v", r)
	}
	if r := SpeedTest("none", pool, nil); r.Ok || r.Error == "" {
		t.Fatalf("unknown id should fail: %+v", r)
	}
}

func TestXrayConfigFile(t *testing.T) {
	dir := t.TempDir()
	out := map[string]any{"protocol": "vmess", "tag": outboundTag}
	cfgFile, _, err := writeXrayConfig(dir, out, 12345, "1.1.1.1,tls://x")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(cfgFile)
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	in := cfg["inbounds"].([]any)[0].(map[string]any)
	if in["port"].(float64) != 12345 || in["listen"] != "127.0.0.1" {
		t.Fatalf("inbound = %v", in)
	}
	if dns := cfg["dns"].(map[string]any)["servers"].([]any); len(dns) != 1 || dns[0] != "1.1.1.1" {
		t.Fatalf("dns = %v", dns)
	}
}

func TestBridgePoolRefCounting(t *testing.T) {
	p := newBridgePool(engine{name: "test"}, nil, "", "")
	b := &bridge{key: "k", exited: make(chan struct{}), refs: 2}
	p.live["k"] = b
	p.release("k")
	if b.refs != 1 {
		t.Fatalf("refs = %d", b.refs)
	}
	p.recycleIdle(0)
	if p.live["k"] == nil {
		t.Fatal("referenced bridge must not be recycled")
	}
}
