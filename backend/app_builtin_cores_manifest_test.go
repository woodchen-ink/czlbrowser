package backend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const testManifestJSON = `{"cores":[
  {"id":"clearcote","name":"Clearcote","version":"150.0.0.1","kind":"clearcote","downloadUrl":"https://github.com/a/b/c.zip","sha256":"F40C83049AAE13D8C0322E9822EEB2D7236FDDD5E7915304EB6F0F5F1F79FCB6"},
  {"id":"evil","name":"Evil","version":"1.0.0.0","kind":"chrome","downloadUrl":"https://evil.example.com/x.zip","sha256":""},
  {"id":"badsha","name":"Bad","version":"1.0.0.0","kind":"chrome","downloadUrl":"https://github.com/a/b/d.zip","sha256":"xyz"},
  {"id":"clearcote","name":"Dup","version":"1.0.0.0","kind":"clearcote","downloadUrl":"https://github.com/a/b/e.zip","sha256":""}
]}`

func resetBuiltinManifestState(t *testing.T, urls []string) {
	t.Helper()
	oldURLs := builtinCoreManifestURLs
	builtinCoreManifestURLs = urls
	builtinManifest = builtinCoreManifestState{}
	t.Cleanup(func() {
		builtinCoreManifestURLs = oldURLs
		builtinManifest = builtinCoreManifestState{}
	})
}

func TestParseBuiltinCoreManifestFiltersInvalidEntries(t *testing.T) {
	cores, err := parseBuiltinCoreManifest([]byte(testManifestJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 1 || cores[0].Id != "clearcote" || cores[0].Version != "150.0.0.1" {
		t.Fatalf("应只保留第一个合法 clearcote 条目: %+v", cores)
	}
	if cores[0].SHA256 != "f40c83049aae13d8c0322e9822eeb2d7236fddd5e7915304eb6f0f5f1f79fcb6" {
		t.Fatalf("SHA256 应规范为小写: %s", cores[0].SHA256)
	}
	if _, err := parseBuiltinCoreManifest([]byte(`{"cores":[]}`)); err == nil {
		t.Fatalf("空清单应报错")
	}
}

func TestBuiltinCoreEntriesPrefersRemoteAndWritesCache(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer failing.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testManifestJSON))
	}))
	defer ok.Close()
	resetBuiltinManifestState(t, []string{failing.URL, ok.URL})

	app := &App{dataRoot: t.TempDir()}
	cores := app.builtinCoreEntries(false)
	if len(cores) != 1 || cores[0].Version != "150.0.0.1" {
		t.Fatalf("应使用远程清单: %+v", cores)
	}
	if _, err := os.Stat(filepath.Join(app.dataRoot, builtinCoreManifestCacheFile)); err != nil {
		t.Fatalf("远程清单应写入缓存: %v", err)
	}
}

func TestBuiltinCoreEntriesFallsBackToCacheThenEmbedded(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer failing.Close()
	resetBuiltinManifestState(t, []string{failing.URL})

	app := &App{dataRoot: t.TempDir()}
	if cores := app.builtinCoreEntries(false); len(cores) != len(embeddedBuiltinCores) || cores[0].Id != embeddedBuiltinCores[0].Id {
		t.Fatalf("无缓存时应回退内嵌清单: %+v", cores)
	}

	cachePath := filepath.Join(app.dataRoot, builtinCoreManifestCacheFile)
	if err := writeFileAtomic(cachePath, []byte(testManifestJSON)); err != nil {
		t.Fatal(err)
	}
	if cores := app.builtinCoreEntries(true); len(cores) != 1 || cores[0].Version != "150.0.0.1" {
		t.Fatalf("远程失败时应优先使用本地缓存: %+v", cores)
	}
}
