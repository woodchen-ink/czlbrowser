package main

import (
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

// stopRuntimeServices 退出前关闭所有实例、转换进程与定时任务, 并清理残留进程。只执行一次。
func (a *App) stopRuntimeServices() {
	a.stopServicesOnce.Do(func() {
		a.closeAllInstances()
		if a.xrayMgr != nil {
			a.xrayMgr.StopAll()
		}
		a.clearProfileXrayBridges()
		if a.singboxMgr != nil {
			a.singboxMgr.StopAll()
		}
		if a.speedScheduler != nil {
			a.speedScheduler.Stop()
			a.speedScheduler = nil
		}
		dirs := []string{filepath.Join(a.appRoot, "bin"), filepath.Join(a.dataRoot, "bin"), filepath.Join(a.dataRoot, "chrome")}
		if err := killLeftoverProcesses(dirs...); err != nil {
			logger.New("App").Error("退出前清理残留进程失败", logger.F("error", err))
		}
	})
}

// finalizeShutdown 关闭数据库与日志文件。只执行一次。
func (a *App) finalizeShutdown() {
	a.finalizeOnce.Do(func() {
		if a.db != nil {
			_ = a.db.Close()
			a.db = nil
		}
		_ = logger.Close()
	})
}

// closeAllInstances 并行关闭所有正在运行的浏览器实例。
func (a *App) closeAllInstances() {
	if a.browserMgr == nil {
		return
	}
	m := a.browserMgr
	now := time.Now().Format(time.RFC3339)
	m.Mutex.Lock()
	running := m.BrowserProcesses
	m.BrowserProcesses = map[string]*exec.Cmd{}
	for id := range running {
		if p := m.Profiles[id]; p != nil {
			p.Running, p.LastStopAt = false, now
		}
	}
	m.Mutex.Unlock()

	var wg sync.WaitGroup
	for id, cmd := range running {
		if cmd == nil || cmd.Process == nil {
			continue
		}
		wg.Add(1)
		go func(id string, pid int) {
			defer wg.Done()
			if err := stopProcessTree(pid, 3*time.Second); err != nil {
				logger.New("App").Error("退出前关闭实例失败", logger.F("profile_id", id), logger.F("pid", pid), logger.F("error", err))
			}
		}(id, cmd.Process.Pid)
	}
	wg.Wait()
}
