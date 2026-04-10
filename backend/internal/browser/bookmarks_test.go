package browser

import (
	"czl-browser/backend/internal/config"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBookmarkURL(t *testing.T) {
	t.Run("root path slash is normalized", func(t *testing.T) {
		got := NormalizeBookmarkURL("https://Platform.OpenAI.com/")
		want := "https://platform.openai.com"
		if got != want {
			t.Fatalf("NormalizeBookmarkURL() = %q, want %q", got, want)
		}
	})

	t.Run("non-root trailing slash is preserved", func(t *testing.T) {
		got := NormalizeBookmarkURL("https://example.com/docs/")
		want := "https://example.com/docs/"
		if got != want {
			t.Fatalf("NormalizeBookmarkURL() = %q, want %q", got, want)
		}
	})
}

func TestEnsureDefaultBookmarks_DedupesRootSlashVariant(t *testing.T) {
	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "Default")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	root := newEmptyBookmarkRoot(toChromiumTime(chromiumEpoch))
	roots := root["roots"].(map[string]interface{})
	bar := roots["bookmark_bar"].(map[string]interface{})
	bar["children"] = []interface{}{
		map[string]interface{}{
			"type": "url",
			"name": "OpenAI API",
			"url":  "https://platform.openai.com/",
			"id":   "4",
		},
	}
	roots["bookmark_bar"] = bar
	root["roots"] = roots

	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "Bookmarks"), data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = EnsureDefaultBookmarks(tempDir, []config.BrowserBookmark{
		{Name: "OpenAI API", URL: "https://platform.openai.com"},
	})
	if err != nil {
		t.Fatalf("EnsureDefaultBookmarks() error = %v", err)
	}

	resultData, err := os.ReadFile(filepath.Join(profileDir, "Bookmarks"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	resultRoots := result["roots"].(map[string]interface{})
	resultBar := resultRoots["bookmark_bar"].(map[string]interface{})
	children := resultBar["children"].([]interface{})
	if len(children) != 1 {
		t.Fatalf("children count = %d, want 1", len(children))
	}
}
