//go:build windows

package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hideWindow 让命令行子进程不弹出控制台窗口。不要用在 Chrome 上, 否则浏览器窗口也看不到。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// waitProcessExit 等进程退出, 超时返回 false。进程已经不存在时立即返回 true。
func waitProcessExit(pid int, timeout time.Duration) bool {
	if pid <= 0 {
		return true
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true // 打不开通常意味着已经退出
	}
	defer windows.CloseHandle(h)
	ev, _ := windows.WaitForSingleObject(h, uint32(timeout/time.Millisecond))
	return ev == windows.WAIT_OBJECT_0
}

// stopProcessTree 结束进程及其子进程: 先发普通关闭 (Chrome 会正常退出, 下次不弹「恢复页面」),
// grace 内没退出再强制结束。
func stopProcessTree(pid int, grace time.Duration) error {
	if pid <= 0 {
		return nil
	}
	taskkill := func(force bool) {
		args := []string{"/PID", strconv.Itoa(pid), "/T"}
		if force {
			args = append([]string{"/F"}, args...)
		}
		cmd := exec.Command("taskkill", args...)
		hideWindow(cmd)
		_ = cmd.Run()
	}
	taskkill(false)
	if waitProcessExit(pid, grace) {
		return nil
	}
	taskkill(true)
	if waitProcessExit(pid, 2*time.Second) {
		return nil
	}
	return errors.New("进程未能结束")
}

// killLeftoverProcesses 强制结束可执行文件位于 dirs 任一目录下的进程 (跳过本程序自身)。
// 退出时用来清理内核与 xray / sing-box 的残留进程。
func killLeftoverProcesses(dirs ...string) error {
	var prefixes []string
	for _, d := range dirs {
		if d = strings.TrimSpace(d); d != "" {
			prefixes = append(prefixes, strings.ToLower(filepath.Clean(d))+`\`)
		}
	}
	if len(prefixes) == 0 {
		return nil
	}
	self := uint32(windows.GetCurrentProcessId())
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var failed []string
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		pid := entry.ProcessID
		if pid == self || pid == 0 {
			continue
		}
		path := strings.ToLower(processImage(pid))
		if path == "" || !hasAnyPrefix(path, prefixes) {
			continue
		}
		if killErr := terminate(pid); killErr != nil {
			failed = append(failed, filepath.Base(path)+"#"+strconv.Itoa(int(pid)))
		}
	}
	if len(failed) > 0 {
		return errors.New("仍在运行: " + strings.Join(failed, ", "))
	}
	return nil
}

func processImage(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func terminate(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if err := windows.TerminateProcess(h, 1); err != nil {
		return err
	}
	_, _ = windows.WaitForSingleObject(h, 2000)
	return nil
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
