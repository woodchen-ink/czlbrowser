package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func signEnvelope(t *testing.T, priv ed25519.PrivateKey, m any) []byte {
	t.Helper()
	payload, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Envelope{
		Manifest:  base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload)),
	})
	return data
}

func testManifest(content []byte, urls ...string) Manifest {
	sum := sha256.Sum256(content)
	return Manifest{
		Version:      "2026.09.17.13",
		Platform:     Platform,
		FileName:     "CZLBrowser-Setup-2026.09.17.13.exe",
		Size:         int64(len(content)),
		SHA256:       hex.EncodeToString(sum[:]),
		DownloadURLs: urls,
	}
}

func TestParseEnvelope(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	good := testManifest([]byte("installer"), "https://example.com/a.exe")

	if _, err := parseEnvelopeWithKey(signEnvelope(t, priv, good), pub); err != nil {
		t.Fatalf("合法清单验签失败: %v", err)
	}
	if _, err := parseEnvelopeWithKey(signEnvelope(t, otherPriv, good), pub); err == nil {
		t.Fatal("其它私钥签名的清单不应通过")
	}

	// 篡改签名后的清单内容
	var env Envelope
	_ = json.Unmarshal(signEnvelope(t, priv, good), &env)
	payload, _ := base64.StdEncoding.DecodeString(env.Manifest)
	env.Manifest = base64.StdEncoding.EncodeToString([]byte(strings.Replace(string(payload), "example.com", "evil.com", 1)))
	tampered, _ := json.Marshal(env)
	if _, err := parseEnvelopeWithKey(tampered, pub); err == nil {
		t.Fatal("篡改后的清单不应通过")
	}

	bad := good
	bad.FileName = `..\..\evil.exe`
	if _, err := parseEnvelopeWithKey(signEnvelope(t, priv, bad), pub); err == nil {
		t.Fatal("非法文件名不应通过")
	}
	bad = good
	bad.DownloadURLs = []string{"http://example.com/a.exe"}
	if _, err := parseEnvelopeWithKey(signEnvelope(t, priv, bad), pub); err == nil {
		t.Fatal("非 https 下载地址不应通过")
	}
}

// TestReleaseEnvelope 供 CI 在发布前用内置公钥校验刚签好的 latest.json。
func TestReleaseEnvelope(t *testing.T) {
	path := os.Getenv("UPDATE_ENVELOPE_FILE")
	if path == "" {
		t.Skip("未设置 UPDATE_ENVELOPE_FILE")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseEnvelope(data)
	if err != nil {
		t.Fatalf("发布清单无法通过客户端校验: %v", err)
	}
	t.Logf("清单校验通过: version=%s size=%d urls=%v", m.Version, m.Size, m.DownloadURLs)
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2026.09.17.12", "2026.09.17.13", -1},
		{"v2026.09.17.13", "2026.09.17.13", 0},
		{"2026.10.01.2", "2026.09.30.99", 1},
		{"2026.09.17.9", "2026.09.17.10", -1},
		{"dev", "2026.09.17.1", -1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	if IsReleaseVersion("dev") || IsReleaseVersion("") {
		t.Error("dev 构建不应视为正式版本")
	}
}

func TestDownloadFallsBackAndVerifies(t *testing.T) {
	content := []byte(strings.Repeat("czl-browser-installer", 1000))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/broken.exe":
			w.Write(content[:len(content)-1]) // 截断，大小不符
		case "/good.exe":
			w.Write(content)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := testManifest(content, srv.URL+"/missing.exe", srv.URL+"/broken.exe", srv.URL+"/good.exe")
	dir := t.TempDir()
	path, err := Download(context.Background(), srv.Client(), &m, dir, nil)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	if filepath.Base(path) != m.FileName {
		t.Fatalf("安装包路径异常: %s", path)
	}
	if err := VerifyFile(path, &m); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Fatal("临时文件未清理")
	}

	// 全部地址内容错误时不得留下安装包
	bad := testManifest(content, srv.URL+"/broken.exe")
	bad.FileName = "CZLBrowser-Setup-2026.09.17.14.exe"
	if _, err := Download(context.Background(), srv.Client(), &bad, dir, nil); err == nil {
		t.Fatal("内容不符时应返回错误")
	}
	if _, err := os.Stat(filepath.Join(dir, bad.FileName)); !os.IsNotExist(err) {
		t.Fatal("校验失败的安装包不应保留")
	}
}
