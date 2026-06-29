package backup

import (
	"os"
	"path/filepath"
	"testing"
)

// buildProfileTree 在 root 下造一个 Chromium profile 目录结构，profile 根名由 profileDir 指定。
func buildProfileTree(t *testing.T, root, profileDir string) {
	t.Helper()
	mk := func(parts ...string) {
		p := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
	}
	touch := func(parts ...string) {
		p := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir parent of %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	// profile 根标志：Default 子目录 + Local State 文件
	mk(profileDir, "Default")
	touch(profileDir, "Local State")
	// 账号数据
	touch(profileDir, "Default", "Cookies")
	touch(profileDir, "Default", "Login Data")
	// 缓存（应跳过）
	touch(profileDir, "Default", "Cache", "data_0")
	// 可重建组件 / 遥测（应跳过）
	touch(profileDir, "BrowserMetrics", "x.pma")
	touch(profileDir, "WasmTtsEngine", "20260528.1", "bindings_main.wasm")
}

func TestDetectProfileRoots_FindsByMarker(t *testing.T) {
	root := t.TempDir()
	// 两个命名风格不同的 profile：UUID 与自定义名
	buildProfileTree(t, root, "6f48a959-9bc1-4482-b797-e313c466bd4d")
	buildProfileTree(t, root, "repro_acl")
	// 非 profile 的业务目录
	if err := os.MkdirAll(filepath.Join(root, "snapshots", "abc"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.db"), []byte("db"), 0644); err != nil {
		t.Fatal(err)
	}

	roots := DetectProfileRoots(root)
	for _, want := range []string{"6f48a959-9bc1-4482-b797-e313c466bd4d", "repro_acl"} {
		if _, ok := roots[want]; !ok {
			t.Errorf("未识别 profile 根: %s (got=%v)", want, roots)
		}
	}
	if _, ok := roots["snapshots"]; ok {
		t.Errorf("snapshots 不应被识别为 profile 根")
	}
}

func TestProfileAwareSkip_SkipsAndKeeps(t *testing.T) {
	root := t.TempDir()
	buildProfileTree(t, root, "repro_acl")
	if err := os.WriteFile(filepath.Join(root, "app.db"), []byte("db"), 0644); err != nil {
		t.Fatal(err)
	}
	skip := NewProfileAwareSkip(DetectProfileRoots(root))

	shouldSkip := []string{
		"repro_acl/BrowserMetrics/x.pma",
		"repro_acl/WasmTtsEngine/20260528.1/bindings_main.wasm",
		"repro_acl/Default/Cache/data_0",
		"repro_acl/Default/Service Worker/CacheStorage/a",
		"repro_acl/Default/Extensions/efaidnbmnnnibpcajpcglclefindmkaj/26.4.1.0_0/manifest.json",
		"repro_acl/Default/Extensions/ghbmnnjooekpmoecnnnilnnbdlolhkhi/1.104.1_0/manifest.json",
		"repro_acl/Default/Extensions/ngpampappnmepgilojfohadhhmbhlaek/6.42.64_0/manifest.json",
		"repro_acl/Default/Extensions/nmmhkkegccagdldgiimedpiccmgmieda/1.0.0.6_0/manifest.json",
	}
	for _, rel := range shouldSkip {
		if !skip(rel) {
			t.Errorf("应跳过但未命中: %s", rel)
		}
	}

	shouldKeep := []string{
		"repro_acl",                 // profile 根本身
		"repro_acl/Local State",     // 关键顶层文件
		"repro_acl/Default",         // 数据目录
		"repro_acl/Default/Cookies", // 账号数据
		"repro_acl/Default/Login Data",
		"repro_acl/Default/IndexedDB/https_x/000001.ldb",
		"repro_acl/Default/Extensions/nngceckbapebfimnlniiiahkandclblb/2026.2.0_0/manifest.json", // Bitwarden 保留
		"repro_acl/Default/Local Extension Settings/efaidnbmnnnibpcajpcglclefindmkaj/000003.log", // 扩展状态保留
		"app.db",             // 非 profile 业务数据
		"snapshots/abc/file", // 非 profile 目录
		"",
	}
	for _, rel := range shouldKeep {
		if skip(rel) {
			t.Errorf("应保留但被跳过: %s", rel)
		}
	}
}

// profile 根就是扫描根本身（browser_user_data_root 指向单个 profile 目录的情形）。
func TestProfileAwareSkip_RootIsProfileItself(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Default"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Default", "Cookies"), []byte("c"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Default", "Cache", "x"), []byte("c"), 0644); err == nil {
		// 需要先建目录
	}
	_ = os.MkdirAll(filepath.Join(root, "BrowserMetrics"), 0755)
	_ = os.WriteFile(filepath.Join(root, "BrowserMetrics", "x.pma"), []byte("m"), 0644)

	roots := DetectProfileRoots(root)
	if _, ok := roots["."]; !ok {
		t.Fatalf("扫描根本身应被识别为 profile 根: %v", roots)
	}
	skip := NewProfileAwareSkip(roots)
	if !skip("BrowserMetrics/x.pma") {
		t.Errorf("根 profile 下的 BrowserMetrics 应跳过")
	}
	if skip("Default/Cookies") {
		t.Errorf("根 profile 下的 Default/Cookies 应保留")
	}
}
