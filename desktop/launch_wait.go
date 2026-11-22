package main

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// browserStartReadyTimeout 是等待 Chrome 打开调试端口的最长时间。
const browserStartReadyTimeout = 10 * time.Second

var errBrowserProcessExitedBeforeReady = errors.New("browser process exited before debug port became ready")

// waitBrowserDebugPortReady 等 Chrome 的调试端口可连。
// Windows 上 chrome.exe 有时会把工作交给另一个进程后自己退出, 所以启动进程退出不等于失败,
// 继续等到超时; 超时时若进程已退出则返回 errBrowserProcessExitedBeforeReady。
func waitBrowserDebugPortReady(port int, cmd *exec.Cmd, timeout time.Duration) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond); err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	if cmd != nil && cmd.Process != nil && waitProcessExit(cmd.Process.Pid, 0) {
		return errBrowserProcessExitedBeforeReady
	}
	return fmt.Errorf("浏览器进程未在 %s 内完成启动，调试端口 %d 未就绪", timeout.Round(time.Second), port)
}

// startErrorHints 把系统返回的启动错误翻译成提示; 按顺序匹配 (小写比较)。
var startErrorHints = []struct {
	needles []string
	message string
}{
	{[]string{"access is denied", "permission denied", "拒绝访问"},
		"实例启动失败：系统拒绝启动浏览器进程。可执行文件：%s。请检查文件权限、杀毒软件拦截，或尝试以管理员身份运行。"},
	{[]string{"not a valid win32 application", "不是有效的 win32 应用程序", "bad exe format"},
		"实例启动失败：当前浏览器内核与系统架构不兼容。可执行文件：%s。请更换为可用的 Windows 64 位 Chrome 内核。"},
	{[]string{"系统找不到指定的文件", "file not found", "no such file", "cannot find the file"},
		"实例启动失败：浏览器可执行文件不存在。可执行文件：%s。请检查内核路径是否正确，或重新下载内核。"},
	{[]string{"目录名称无效", "directory name is invalid"},
		"实例启动失败：浏览器工作目录无效。当前目录：%s。请检查内核路径配置是否正确。"},
}

// describeChromeProcessStartError 给 chrome.exe 无法启动的错误配上原因说明。
func describeChromeProcessStartError(exe string, err error) string {
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)
	for _, h := range startErrorHints {
		for _, n := range h.needles {
			if strings.Contains(lower, n) {
				return fmt.Sprintf(h.message, exe)
			}
		}
	}
	return fmt.Sprintf("实例启动失败：浏览器进程拉起失败。可执行文件：%s。原因：%s。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", exe, raw)
}

func describeBrowserReadyTimeout(port int, timeout time.Duration) string {
	return fmt.Sprintf("实例启动失败：浏览器进程已拉起，但在 %s 内未完成就绪，调试端口 %d 未开启。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", timeout.Round(time.Second), port)
}
