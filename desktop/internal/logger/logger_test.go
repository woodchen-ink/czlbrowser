package logger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLevelFilterAndRecent(t *testing.T) {
	ClearRecent()
	SetLevel("warn")
	defer SetLevel("info")

	log := New("Test")
	log.Info("被过滤")
	log.Warn("保留", F("err", errors.New("boom")), F("n", 3))

	got := Recent()
	if len(got) != 1 {
		t.Fatalf("应只记录 1 条, got %d", len(got))
	}
	e := got[0]
	if e.Level != "WARN" || e.Component != "Test" || e.Message != "保留" {
		t.Fatalf("条目内容不对: %+v", e)
	}
	if e.Fields["err"] != "boom" || e.Fields["n"] != 3 {
		t.Fatalf("字段不对: %+v", e.Fields)
	}
	if LevelName() != "WARN" {
		t.Fatalf("LevelName = %s", LevelName())
	}
}

func TestRecentWrapsOldestFirst(t *testing.T) {
	ClearRecent()
	log := New("Ring")
	for i := 0; i < recentCap+3; i++ {
		log.Info("m", F("i", i))
	}
	got := Recent()
	if len(got) != recentCap {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Fields["i"] != 3 || got[recentCap-1].Fields["i"] != recentCap+2 {
		t.Fatalf("顺序不对: first=%v last=%v", got[0].Fields["i"], got[recentCap-1].Fields["i"])
	}
}

func TestFileOutputAndRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "app.log")
	if err := Setup(Options{Level: "info", File: path, MaxSizeMB: 1, MaxBackups: 2}); err != nil {
		t.Fatal(err)
	}
	defer Close()

	log := New("File")
	log.Info("第一行", F("k", "v"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "msg=第一行") || !strings.Contains(string(raw), "component=File") {
		t.Fatalf("文件内容不对: %s", raw)
	}

	big := strings.Repeat("x", 300<<10)
	for i := 0; i < 8; i++ {
		log.Info(big)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("应产生 .1 轮转文件: %v", err)
	}
	if _, err := os.Stat(path + ".3"); err == nil {
		t.Fatalf("只应保留 2 份旧文件")
	}
}
