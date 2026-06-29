package backend

import (
	"fmt"
	"net"
	"os/exec"
	stdruntime "runtime"
	"strings"
	"testing"
	"time"
)

func TestDescribeChromeProcessStartError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "file not found",
			err:  fmt.Errorf("fork/exec C:\\chrome.exe: The system cannot find the file specified."),
			want: "浏览器可执行文件不存在",
		},
		{
			name: "access denied",
			err:  fmt.Errorf("fork/exec C:\\chrome.exe: Access is denied."),
			want: "系统拒绝启动浏览器进程",
		},
		{
			name: "invalid win32",
			err:  fmt.Errorf("%%1 is not a valid Win32 application"),
			want: "与系统架构不兼容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeChromeProcessStartError(`C:\chrome.exe`, tt.err)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("expected %q to contain %q", got, tt.want)
			}
		})
	}
}

func TestDescribeBrowserReadyTimeout(t *testing.T) {
	got := describeBrowserReadyTimeout(9222, 10*time.Second)
	if !strings.Contains(got, "调试端口 9222 未开启") {
		t.Fatalf("unexpected timeout message: %q", got)
	}
}

func TestWaitBrowserDebugPortReadyWindowsAllowsLauncherExit(t *testing.T) {
	if stdruntime.GOOS != "windows" {
		t.Skip("Windows launcher handoff behavior only applies on Windows")
	}
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("cmd", "/C", "exit", "0")
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	ready := make(chan struct{})
	go func() {
		time.Sleep(250 * time.Millisecond)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			close(ready)
			return
		}
		defer ln.Close()
		close(ready)
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	if err := waitBrowserDebugPortReady(port, cmd, 2*time.Second); err != nil {
		t.Fatalf("wait should continue after launcher exit on Windows: %v", err)
	}
	<-ready
}
