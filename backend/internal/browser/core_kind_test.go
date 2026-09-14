package browser

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeFakeCore(t *testing.T, dllRelPath string, dllContent []byte) string {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "chrome.exe")
	if err := os.WriteFile(exe, []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	dll := filepath.Join(dir, dllRelPath)
	if err := os.MkdirAll(filepath.Dir(dll), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dll, dllContent, 0o644); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestDetectCoreKindClearcoteRootDLL(t *testing.T) {
	exe := writeFakeCore(t, "chrome.dll", []byte("xx--canvas-bridge-url=yy"))
	if got := DetectCoreKind(exe); got != CoreKindClearcote {
		t.Fatalf("got %q, want clearcote", got)
	}
}

func TestDetectCoreKindChromeVersionedDLL(t *testing.T) {
	exe := writeFakeCore(t, filepath.Join("147.0.7727.56", "chrome.dll"), []byte("plain chrome dll"))
	if got := DetectCoreKind(exe); got != CoreKindChrome {
		t.Fatalf("got %q, want chrome", got)
	}
}

func TestDetectCoreKindMissingDLLFallsBackToChrome(t *testing.T) {
	dir := t.TempDir()
	if got := DetectCoreKind(filepath.Join(dir, "chrome.exe")); got != CoreKindChrome {
		t.Fatalf("got %q, want chrome", got)
	}
}

func TestFileContainsAcrossChunkBoundary(t *testing.T) {
	const chunkSize = 4 << 20
	needle := []byte("canvas-bridge-url")
	content := bytes.Repeat([]byte{0}, chunkSize+64)
	// 让标记横跨第一个分块边界
	copy(content[chunkSize-5:], needle)

	path := filepath.Join(t.TempDir(), "chrome.dll")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := fileContains(path, needle)
	if err != nil || !found {
		t.Fatalf("跨块标记未检出: found=%v err=%v", found, err)
	}
}

func TestFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.zip")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestGetChromeVersionFromVersionedDir(t *testing.T) {
	exe := writeFakeCore(t, filepath.Join("153.0.8010.37", "chrome.dll"), []byte("dll"))
	m := &Manager{}
	if got := m.GetChromeVersion(filepath.Dir(exe)); got != "153.0.8010.37" {
		t.Fatalf("got %q, want 153.0.8010.37", got)
	}
}
