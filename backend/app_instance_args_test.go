package backend

import "testing"

func TestFilterManagedLaunchArgs(t *testing.T) {
	t.Parallel()

	in := []string{
		"--proxy-server=socks5://u:p@1.2.3.4:1080",
		"--proxy-pac-url=http://127.0.0.1/proxy.pac",
		"--proxy-auto-detect",
		"--no-proxy-server",
		"--remote-debugging-port=9222",
		"--user-data-dir=C:\\temp\\chrome-data",
		"--remote-debugging-port", "9333",
		"--proxy-server", "http://127.0.0.1:7890",
		"--disable-sync",
		"  --lang=zh-CN  ",
		"",
		"   ",
	}

	filtered, removed := filterManagedLaunchArgs(in)
	if len(removed) != 10 {
		t.Fatalf("unexpected removed count: %d", len(removed))
	}
	if len(filtered) != 2 {
		t.Fatalf("unexpected filtered count: %d", len(filtered))
	}
	if filtered[0] != "--disable-sync" || filtered[1] != "--lang=zh-CN" {
		t.Fatalf("unexpected filtered args: %#v", filtered)
	}
}
