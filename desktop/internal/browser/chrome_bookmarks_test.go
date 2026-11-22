package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

func TestNormalizeBookmarkURL(t *testing.T) {
	cases := map[string]string{
		"https://Platform.OpenAI.com/": "https://platform.openai.com",
		"https://example.com/docs/":    "https://example.com/docs/",
		" not a url ":                  "not a url",
		"":                             "",
	}
	for in, want := range cases {
		if got := NormalizeBookmarkURL(in); got != want {
			t.Errorf("NormalizeBookmarkURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func readBar(t *testing.T, dir string) []interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "Default", "Bookmarks"))
	if err != nil {
		t.Fatal(err)
	}
	var doc node
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc["roots"].(node)["bookmark_bar"].(node)["children"].([]interface{})
}

func TestEnsureDefaultBookmarksCreatesAndDedupes(t *testing.T) {
	dir := t.TempDir()
	defaults := []config.BrowserBookmark{{Name: "A", URL: "https://a.example/"}, {Name: "B", URL: "https://b.example/x"}}
	if err := EnsureDefaultBookmarks(dir, defaults); err != nil {
		t.Fatal(err)
	}
	if bar := readBar(t, dir); len(bar) != 2 {
		t.Fatalf("应写入 2 个书签, got %d", len(bar))
	}
	// 再次执行 (含仅大小写 / 末尾斜杠不同的网址) 不应重复
	defaults = append(defaults, config.BrowserBookmark{Name: "A2", URL: "HTTPS://A.EXAMPLE"})
	if err := EnsureDefaultBookmarks(dir, defaults); err != nil {
		t.Fatal(err)
	}
	bar := readBar(t, dir)
	if len(bar) != 2 {
		t.Fatalf("重复书签不应追加, got %d", len(bar))
	}
	if bar[0].(node)["id"] == bar[1].(node)["id"] {
		t.Fatal("书签 id 应唯一")
	}
}

func TestEnsureDefaultBookmarksKeepsUserBookmarks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"checksum":"x","version":1,"roots":{"bookmark_bar":{"type":"folder","id":"1","children":[
		{"type":"folder","id":"7","name":"F","children":[{"type":"url","id":"9","name":"mine","url":"https://a.example"}]}]},
		"other":{"type":"folder","id":"2","children":[]}}}`
	if err := os.WriteFile(filepath.Join(dir, "Default", "Bookmarks"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	err := EnsureDefaultBookmarks(dir, []config.BrowserBookmark{{Name: "A", URL: "https://a.example/"}, {Name: "C", URL: "https://c.example"}})
	if err != nil {
		t.Fatal(err)
	}
	bar := readBar(t, dir)
	if len(bar) != 2 || bar[1].(node)["id"] != "10" {
		t.Fatalf("应保留用户文件夹并以最大 id+1 追加: %v", bar)
	}
}
