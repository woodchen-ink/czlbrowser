package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// BrowserInstanceStart 启动实例。同步空间的实例会先检查占用、按需下载云端登录态;
// 他人或本人另一台电脑正在使用时返回 40915 / 40917, 由界面确认后改调 BrowserInstanceStartForce。
func (a *App) BrowserInstanceStart(profileID string) (*BrowserProfile, error) {
	return a.startWithCloud(profileID, false)
}

// BrowserInstanceStartForce 跳过占用检查启动; 登录态下载失败时也用本机数据启动。
func (a *App) BrowserInstanceStartForce(profileID string) (*BrowserProfile, error) {
	return a.startWithCloud(profileID, true)
}

func (a *App) startWithCloud(profileID string, force bool) (*BrowserProfile, error) {
	if a.cloud != nil && !a.isRunning(profileID) {
		// 下载登录态可能较久, 不持有实例锁
		ctx, cancel := a.cloudCtx(5 * time.Minute)
		_, err := a.cloud.PrepareLaunch(ctx, profileID, force)
		cancel()
		if err != nil {
			logCloud("sync.prepare_launch", err)
			return nil, err
		}
	}
	p, err := a.startInstance(profileID, nil, nil)
	if err == nil && a.cloud != nil && p != nil && p.Running {
		a.cloud.OnStarted(profileID)
	}
	return p, err
}

// isRunning 报告实例是否在运行 (不存在视为未运行)。
func (a *App) isRunning(profileID string) bool {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	p, ok := a.browserMgr.Profiles[profileID]
	return ok && p.Running
}

// BrowserInstanceStop 关闭实例 (先正常关闭, 超时再强制结束)。
func (a *App) BrowserInstanceStop(profileID string) (*BrowserProfile, error) {
	m := a.browserMgr
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	p, ok := m.Profiles[profileID]
	if !ok {
		return nil, errProfileNotFound
	}
	if err := a.stopProcessCmd(m.BrowserProcesses[profileID]); err != nil {
		p.LastError = err.Error()
		return p, err
	}
	p.Running, p.LastStopAt = false, time.Now().Format(time.RFC3339)
	delete(m.BrowserProcesses, profileID)
	a.releaseProfileXrayBridge(profileID)
	return p, nil
}

var errProfileNotFound = errors.New("profile not found")

func (a *App) BrowserInstanceRestart(profileID string) (*BrowserProfile, error) {
	if _, err := a.BrowserInstanceStop(profileID); err != nil {
		return nil, err
	}
	return a.BrowserInstanceStart(profileID)
}

// stopProcessCmd 结束一个实例进程: 先正常关闭, 3 秒内没退出再强制结束。
func (a *App) stopProcessCmd(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := stopProcessTree(cmd.Process.Pid, 3*time.Second); err == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

// BrowserInstanceOpenUrl 在运行中的实例里新开一个标签页打开网址。
func (a *App) BrowserInstanceOpenUrl(profileID, target string) bool {
	port, err := a.runningDebugPort(profileID)
	if err != nil || strings.TrimSpace(target) == "" {
		return false
	}
	return withBrowserCDP(port, func(c *cdpConn) error {
		_, err := c.call("Target.createTarget", map[string]any{"url": strings.TrimSpace(target)}, "")
		return err
	}) == nil
}

// BrowserInstanceGetTabs 列出运行中实例的标签页; 实例未运行时返回空列表。
func (a *App) BrowserInstanceGetTabs(profileID string) []BrowserTab {
	tabs := []BrowserTab{}
	port, err := a.runningDebugPort(profileID)
	if err != nil {
		return tabs
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return tabs
	}
	defer resp.Body.Close()
	var targets []struct {
		ID, Type, Title, URL string
	}
	if json.NewDecoder(resp.Body).Decode(&targets) != nil {
		return tabs
	}
	for _, t := range targets {
		if t.Type != "page" {
			continue
		}
		if u, err := url.Parse(t.URL); err == nil && (u.Scheme == "devtools" || u.Scheme == "chrome-extension") {
			continue
		}
		// /json/list 按最近使用排序, 第一个页面就是当前标签
		tabs = append(tabs, BrowserTab{TabId: t.ID, Title: t.Title, Url: t.URL, Active: len(tabs) == 0})
	}
	return tabs
}
