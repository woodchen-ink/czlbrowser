package browser

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

func TestFetchToFileRangedAndPlain(t *testing.T) {
	payload := bytes.Repeat([]byte("0123456789abcdef"), 70000) // ~1.1MB
	ranged := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "k.zip", time.Time{}, bytes.NewReader(payload))
	}))
	defer ranged.Close()
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload) // 不支持 Range
	}))
	defer plain.Close()

	for name, srv := range map[string]*httptest.Server{"ranged": ranged, "plain": plain} {
		t.Run(name, func(t *testing.T) {
			f, err := os.CreateTemp(t.TempDir(), "dl")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := fetchToFile(context.Background(), srv.Client(), srv.URL, f, func(int64, int64, bool) {}); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(f.Name())
			if !bytes.Equal(got, payload) {
				t.Fatalf("内容不一致: %d vs %d 字节", len(got), len(payload))
			}
		})
	}
}

func TestFetchToFileHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	f, _ := os.CreateTemp(t.TempDir(), "dl")
	defer f.Close()
	if err := fetchToFile(context.Background(), srv.Client(), srv.URL, f, func(int64, int64, bool) {}); err == nil {
		t.Fatal("404 应返回错误")
	}
}

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.zip")
	out, _ := os.Create(p)
	zw := zip.NewWriter(out)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = out.Close()
	return p
}

func TestUnzipFlatten(t *testing.T) {
	noop := func(int, string) {}
	t.Run("strips shared top dir", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "k")
		err := unzipFlatten(makeZip(t, map[string]string{"Chrome-bin/chrome.exe": "x", "Chrome-bin/153.0.1.2/chrome.dll": "y"}), dest, noop)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dest, "chrome.exe")); err != nil {
			t.Fatal("顶层目录应被去掉")
		}
	})
	t.Run("keeps mixed root", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "k")
		if err := unzipFlatten(makeZip(t, map[string]string{"chrome.exe": "x", "sub/a.txt": "y"}), dest, noop); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dest, "sub", "a.txt")); err != nil {
			t.Fatal("没有公共顶层目录时应原样解压")
		}
	})
	t.Run("rejects zip slip", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "k")
		err := unzipFlatten(makeZip(t, map[string]string{"../evil.txt": "x", "a/b.txt": "y"}), dest, noop)
		if err == nil || !strings.Contains(err.Error(), "非法") {
			t.Fatalf("应拒绝越界路径: %v", err)
		}
	})
}

func TestProfilesInMemory(t *testing.T) {
	m := NewManager(config.DefaultConfig(), t.TempDir(), "")
	p, err := m.Create(ProfileInput{ProfileName: "A", Tags: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.UserDataDir != p.ProfileId || p.Space != "local" {
		t.Fatalf("默认值不对: %+v", p)
	}
	cp, err := m.Copy(p.ProfileId, "")
	if err != nil || cp.ProfileName != "A (副本)" || cp.UserDataDir == p.UserDataDir {
		t.Fatalf("复制不对: %+v %v", cp, err)
	}
	if got := m.ListByTag("X"); len(got) != 2 {
		t.Fatalf("标签筛选应不区分大小写: %d", len(got))
	}
	if err := m.Delete(p.ProfileId); err != nil || len(m.List()) != 1 {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := m.Update("missing", ProfileInput{}); err == nil {
		t.Fatal("不存在的实例应报错")
	}
}
