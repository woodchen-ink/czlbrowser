package backend

import (
	"czl-browser/backend/internal/browser"
	"czl-browser/backend/internal/logger"
	"slices"
	"testing"
)

func TestBuildKernelLaunchPlanChromeKeepsCDPInjection(t *testing.T) {
	args := []string{"--fingerprint=1", "--fingerprint-platform=windows"}
	plan := buildKernelLaunchPlan(browser.CoreKindChrome, args, "America/Los_Angeles", logger.New("test"), "p1")

	if !plan.InjectViaCDP {
		t.Fatalf("标准 Chrome 应走 CDP 注入")
	}
	for _, arg := range plan.Args {
		if arg == "--timezone=America/Los_Angeles" {
			t.Fatalf("标准 Chrome 不应追加 --timezone: %v", plan.Args)
		}
	}
	if !containsPrefix(plan.Args, "--fingerprint-webgl-vendor=") {
		t.Fatalf("标准 Chrome 应保留短格式 webgl 参数: %v", plan.Args)
	}
}

func TestBuildKernelLaunchPlanClearcoteUsesNativeSwitches(t *testing.T) {
	args := []string{
		"--fingerprint=1",
		"--fingerprint-platform=windows",
		"--fingerprint-webgl-vendor=NVIDIA",
		"--fingerprint-webgl-renderer=NVIDIA GeForce GTX 1660",
		"--fingerprint-touch-points=5",
		"--lang=en-US",
	}
	plan := buildKernelLaunchPlan(browser.CoreKindClearcote, args, "America/Los_Angeles", logger.New("test"), "p1")

	if plan.InjectViaCDP {
		t.Fatalf("Clearcote 不应走 CDP 注入")
	}
	want := []string{
		"--fingerprint=1",
		"--fingerprint-platform=windows",
		"--fingerprint-max-touch-points=5",
		"--lang=en-US",
		"--fingerprint-gpu-vendor=Google Inc. (NVIDIA)",
		"--fingerprint-gpu-renderer=ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 (0x00002184) Direct3D11 vs_5_0 ps_5_0, D3D11)",
		"--timezone=America/Los_Angeles",
	}
	if !slices.Equal(plan.Args, want) {
		t.Fatalf("Clearcote 参数改写不符合预期\n got: %q\nwant: %q", plan.Args, want)
	}
}

func TestBuildKernelLaunchPlanClearcoteAutoFillsGPU(t *testing.T) {
	plan := buildKernelLaunchPlan(browser.CoreKindClearcote, []string{"--fingerprint-platform=mac"}, "", logger.New("test"), "p1")

	if containsPrefix(plan.Args, "--fingerprint-webgl-") || containsPrefix(plan.Args, "--timezone=") {
		t.Fatalf("不应残留 webgl 参数或空时区: %v", plan.Args)
	}
	if !slices.Contains(plan.Args, "--fingerprint-platform=macos") {
		t.Fatalf("mac 应映射为 macos: %v", plan.Args)
	}
	if !slices.Contains(plan.Args, "--fingerprint-gpu-vendor=Google Inc. (Apple)") {
		t.Fatalf("应自动填充 ANGLE 格式 GPU vendor: %v", plan.Args)
	}
}

func TestAngleWebGLStrings(t *testing.T) {
	cases := []struct {
		platform, vendor, renderer string
		wantVendor, wantRenderer   string
	}{
		{"windows", "AMD", "AMD Radeon RX 580", "Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon RX 580 (0x000067DF) Direct3D11 vs_5_0 ps_5_0, D3D11)"},
		{"windows", "Intel", "Unknown GPU", "Google Inc. (Intel)", "ANGLE (Intel, Unknown GPU Direct3D11 vs_5_0 ps_5_0, D3D11)"},
		{"macos", "Apple", "Apple M2", "Google Inc. (Apple)", "ANGLE (Apple, ANGLE Metal Renderer: Apple M2, Unspecified Version)"},
		{"windows", "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, custom, D3D11)", "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, custom, D3D11)"},
	}
	for _, c := range cases {
		vendor, renderer := angleWebGLStrings(c.platform, c.vendor, c.renderer)
		if vendor != c.wantVendor || renderer != c.wantRenderer {
			t.Errorf("angleWebGLStrings(%q,%q,%q) = %q, %q; want %q, %q", c.platform, c.vendor, c.renderer, vendor, renderer, c.wantVendor, c.wantRenderer)
		}
	}
}

func TestBuiltinCoresManifest(t *testing.T) {
	if len(embeddedBuiltinCores) == 0 {
		t.Fatalf("内置内核清单为空")
	}
	ids := map[string]bool{}
	for _, src := range embeddedBuiltinCores {
		if src.Id == "" || src.Name == "" || src.Version == "" || src.DownloadURL == "" {
			t.Errorf("清单条目字段不完整: %+v", src)
		}
		if src.Kind != string(browser.CoreKindChrome) && src.Kind != string(browser.CoreKindClearcote) {
			t.Errorf("未知内核类型 %q: %s", src.Kind, src.Id)
		}
		if ids[src.Id] {
			t.Errorf("清单 id 重复: %s", src.Id)
		}
		ids[src.Id] = true
	}
	for _, id := range []string{"clearcote", "chrome-stable"} {
		if !ids[id] {
			t.Fatalf("清单缺少 %s 条目（Chrome-exe 仓库的清单同步 workflow 依赖该 id）", id)
		}
	}
}

func containsPrefix(args []string, prefix string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return len(arg) >= len(prefix) && arg[:len(prefix)] == prefix
	})
}
