package main

import (
	"errors"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDescribeChromeProcessStartError(t *testing.T) {
	cases := map[string]string{
		`fork/exec C:\chrome.exe: The system cannot find the file specified.`: "浏览器可执行文件不存在",
		`fork/exec C:\chrome.exe: Access is denied.`:                          "系统拒绝启动浏览器进程",
		`%1 is not a valid Win32 application`:                                 "与系统架构不兼容",
		`something else`:                                                      "原因：something else",
	}
	for raw, want := range cases {
		if got := describeChromeProcessStartError(`C:\chrome.exe`, errors.New(raw)); !strings.Contains(got, want) {
			t.Errorf("%q -> %q, want contains %q", raw, got, want)
		}
	}
	if got := describeBrowserReadyTimeout(9222, 10*time.Second); !strings.Contains(got, "调试端口 9222 未开启") {
		t.Errorf("timeout message: %q", got)
	}
}

// Windows 上启动进程可能先退出 (交给另一个 chrome 进程), 只要端口随后就绪就算成功。
func TestWaitDebugPortAfterLauncherExit(t *testing.T) {
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cmd", "/C", "exit", "0")
	hideWindow(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(250 * time.Millisecond)
		if ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port)); err == nil {
			defer ln.Close()
			if c, err := ln.Accept(); err == nil {
				c.Close()
			}
		}
	}()
	if err := waitBrowserDebugPortReady(port, cmd, 2*time.Second); err != nil {
		t.Fatalf("应等到端口就绪: %v", err)
	}
}

func TestWaitDebugPortTimesOutAfterExit(t *testing.T) {
	port, _ := nextAvailablePort()
	cmd := exec.Command("cmd", "/C", "exit", "0")
	hideWindow(cmd)
	_ = cmd.Run()
	if err := waitBrowserDebugPortReady(port, cmd, 300*time.Millisecond); !errors.Is(err, errBrowserProcessExitedBeforeReady) {
		t.Fatalf("进程已退出且端口未开应返回专用错误, got %v", err)
	}
}
