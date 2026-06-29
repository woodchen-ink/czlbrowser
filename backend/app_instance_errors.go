package backend

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	stdruntime "runtime"
	"strings"
	"time"
)

const browserStartReadyTimeout = 10 * time.Second

var errBrowserProcessExitedBeforeReady = errors.New("browser process exited before debug port became ready")

func waitBrowserDebugPortReady(debugPort int, cmd *exec.Cmd, timeout time.Duration) error {
	address := fmt.Sprintf("127.0.0.1:%d", debugPort)
	deadline := time.Now().Add(timeout)
	processExited := false

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		if browserProcessExited(cmd) {
			if stdruntime.GOOS != "windows" {
				return errBrowserProcessExitedBeforeReady
			}
			processExited = true
		}
		time.Sleep(150 * time.Millisecond)
	}

	if processExited {
		return errBrowserProcessExitedBeforeReady
	}
	return fmt.Errorf("浏览器进程未在 %s 内完成启动，调试端口 %d 未就绪", timeout.Round(time.Second), debugPort)
}

func browserProcessExited(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if stdruntime.GOOS == "windows" {
		alive, err := isProcessAliveWindows(cmd.Process.Pid)
		if err != nil {
			return false
		}
		return !alive
	}
	return cmd.ProcessState != nil && cmd.ProcessState.Exited()
}

func describeChromeProcessStartError(chromeBinaryPath string, err error) string {
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(lower, "access is denied"),
		strings.Contains(lower, "permission denied"),
		strings.Contains(raw, "拒绝访问"):
		return fmt.Sprintf("实例启动失败：系统拒绝启动浏览器进程。可执行文件：%s。请检查文件权限、杀毒软件拦截，或尝试以管理员身份运行。", chromeBinaryPath)
	case strings.Contains(lower, "not a valid win32 application"),
		strings.Contains(raw, "不是有效的 win32 应用程序"),
		strings.Contains(raw, "不是有效的 Win32 应用程序"),
		strings.Contains(raw, "bad exe format"):
		return fmt.Sprintf("实例启动失败：当前浏览器内核与系统架构不兼容。可执行文件：%s。请更换为可用的 Windows 64 位 Chrome 内核。", chromeBinaryPath)
	case strings.Contains(raw, "系统找不到指定的文件"),
		strings.Contains(lower, "file not found"),
		strings.Contains(lower, "no such file"),
		strings.Contains(lower, "cannot find the file"):
		return fmt.Sprintf("实例启动失败：浏览器可执行文件不存在。可执行文件：%s。请检查内核路径是否正确，或重新下载内核。", chromeBinaryPath)
	case strings.Contains(raw, "目录名称无效"),
		strings.Contains(lower, "directory name is invalid"):
		return fmt.Sprintf("实例启动失败：浏览器工作目录无效。当前目录：%s。请检查内核路径配置是否正确。", chromeBinaryPath)
	default:
		return fmt.Sprintf("实例启动失败：浏览器进程拉起失败。可执行文件：%s。原因：%s。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", chromeBinaryPath, raw)
	}
}

func describeBrowserReadyTimeout(debugPort int, timeout time.Duration) string {
	return fmt.Sprintf("实例启动失败：浏览器进程已拉起，但在 %s 内未完成就绪，调试端口 %d 未开启。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", timeout.Round(time.Second), debugPort)
}
