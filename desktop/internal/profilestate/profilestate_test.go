//go:build windows

package profilestate

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/dpapi"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackUnpackRewrapsKey(t *testing.T) {
	src := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := dpapi.Protect(key)
	if err != nil {
		t.Fatal(err)
	}
	ls := map[string]any{
		"os_crypt": map[string]any{
			"encrypted_key":           base64.StdEncoding.EncodeToString(append([]byte("DPAPI"), enc...)),
			"app_bound_encrypted_key": "x",
		},
		"big": json.Number("13340000000000000123"),
	}
	b, _ := json.Marshal(ls)
	write(t, filepath.Join(src, "Local State"), string(b))
	write(t, filepath.Join(src, "Default", "Network", "Cookies"), "cookie-db")
	write(t, filepath.Join(src, "Default", "Cache", "data_0"), "cache")
	write(t, filepath.Join(src, "SingletonLock"), "lock")
	write(t, filepath.Join(src, "WidevineCdm", "x"), "component")

	pack := filepath.Join(t.TempDir(), "state.zip")
	res, err := Pack(src, pack)
	if err != nil {
		t.Fatal(err)
	}
	if res.Size <= 0 || len(res.SHA256) != 64 {
		t.Fatalf("bad result %+v", res)
	}

	dst := filepath.Join(t.TempDir(), "profile")
	write(t, filepath.Join(dst, "old"), "old")
	if err := Unpack(pack, dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "Default", "Network", "Cookies")); string(got) != "cookie-db" {
		t.Fatalf("cookies not restored: %q", got)
	}
	for _, p := range []string{"Default/Cache/data_0", "SingletonLock", "WidevineCdm/x", "old"} {
		if _, err := os.Stat(filepath.Join(dst, p)); err == nil {
			t.Fatalf("%s should not exist", p)
		}
	}
	if _, err := os.Stat(filepath.Join(dst+".prev", "old")); err != nil {
		t.Fatalf("previous dir not kept: %v", err)
	}
	got, err := readOSCryptKey(filepath.Join(dst, "Local State"))
	if err != nil || string(got) != string(key) {
		t.Fatalf("key not rewrapped: %v", err)
	}
	doc, _ := readLocalState(filepath.Join(dst, "Local State"))
	if doc["big"].(json.Number).String() != "13340000000000000123" {
		t.Fatalf("number precision lost: %v", doc["big"])
	}
	if _, ok := doc["os_crypt"].(map[string]any)["app_bound_encrypted_key"]; ok {
		t.Fatal("app_bound_encrypted_key should be removed")
	}
}
