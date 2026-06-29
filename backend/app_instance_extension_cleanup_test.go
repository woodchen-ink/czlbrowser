package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupBlockedChromeExtensions(t *testing.T) {
	root := t.TempDir()
	blockedID := "efaidnbmnnnibpcajpcglclefindmkaj"
	bitwardenID := "nngceckbapebfimnlniiiahkandclblb"
	paths := []string{
		filepath.Join(root, "Default", "Extensions", blockedID, "26.4.1.0_0", "manifest.json"),
		filepath.Join(root, "Default", "Extensions", bitwardenID, "2026.2.0_0", "manifest.json"),
		filepath.Join(root, "Default", "Local Extension Settings", blockedID, "000003.log"),
		filepath.Join(root, "Profile 1", "Extensions", "ghbmnnjooekpmoecnnnilnnbdlolhkhi", "1.104.1_0", "manifest.json"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cleanupBlockedChromeExtensions(root, nil, "profile-1")

	if _, err := os.Stat(filepath.Join(root, "Default", "Extensions", blockedID)); !os.IsNotExist(err) {
		t.Fatalf("blocked extension should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Profile 1", "Extensions", "ghbmnnjooekpmoecnnnilnnbdlolhkhi")); !os.IsNotExist(err) {
		t.Fatalf("blocked extension in Profile 1 should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Default", "Extensions", bitwardenID)); err != nil {
		t.Fatalf("Bitwarden should be kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Default", "Local Extension Settings", blockedID)); err != nil {
		t.Fatalf("extension state should be kept: %v", err)
	}
}
