package proxy

import "testing"

func TestBuildSocks5HTTPClientSupportsAuthURL(t *testing.T) {
	t.Parallel()

	_, err := buildSocks5HTTPClient("socks5://czlgost:wiejasjdkfnaejiw@194.55.15.221:28443", 0)
	if err != nil {
		t.Fatalf("expected auth socks5 url to be accepted, got err=%v", err)
	}
}

func TestBuildSocks5HTTPClientSupportsAuthHostWithoutScheme(t *testing.T) {
	t.Parallel()

	_, err := buildSocks5HTTPClient("czlgost:wiejasjdkfnaejiw@194.55.15.221:28443", 0)
	if err != nil {
		t.Fatalf("expected socks5 addr without scheme to be accepted, got err=%v", err)
	}
}

