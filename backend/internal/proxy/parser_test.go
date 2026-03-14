package proxy

import "testing"

func TestBuildAuthBridgeOutboundFromStandardProxySocks5(t *testing.T) {
	t.Parallel()

	out, ok := buildAuthBridgeOutboundFromStandardProxy("socks5://user:pass@127.0.0.1:1080")
	if !ok || out == nil {
		t.Fatalf("expected socks5 proxy with auth to build outbound")
	}
	if out["protocol"] != "socks" {
		t.Fatalf("unexpected protocol: %v", out["protocol"])
	}
}

func TestBuildAuthBridgeOutboundFromStandardProxyNoAuth(t *testing.T) {
	t.Parallel()

	out, ok := buildAuthBridgeOutboundFromStandardProxy("socks5://127.0.0.1:1080")
	if ok || out != nil {
		t.Fatalf("expected no-auth socks5 proxy not to require auth bridge")
	}
}
