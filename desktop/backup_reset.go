package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

// resetToDefaults 写回默认配置, 清空本机业务表与数据目录 (正在使用的数据库文件保留)。
// reload 为 false 时由调用方 (导入流程) 在最后统一重载。
func (a *App) resetToDefaults(reload bool) (map[string]interface{}, error) {
	a.stopForMaintenance()
	old := a.config
	if old == nil {
		old = config.DefaultConfig()
	}
	fresh := config.DefaultConfig()
	keep := dbFiles(a.dbPathOf(old))

	if err := fresh.Save(a.mainConfigPath()); err != nil {
		return nil, fmt.Errorf("写入默认配置失败: %w", err)
	}
	a.config = fresh
	a.applyRuntimeConfig(fresh.Runtime)
	_ = os.Remove(a.resolveAppPath("proxies.yaml"))
	if err := a.clearLocalTables(); err != nil {
		return nil, err
	}

	var cleared []string
	data := a.resolveAppPath("data")
	if clearDirExcept(data, keep) == nil {
		cleared = append(cleared, data)
	}
	for _, root := range []string{a.userDataRootOf(old), a.userDataRootOf(fresh)} {
		if samePathFold(root, data) || containsFold(cleared, root) {
			continue
		}
		if clearDirExcept(root, keep) == nil {
			cleared = append(cleared, root)
		}
	}
	if reload {
		if err := a.reloadAfterMaintenance(); err != nil {
			return nil, err
		}
	}
	logger.New("Backup").Info("已初始化到默认状态", logger.F("cleared", strings.Join(cleared, ";")))
	return map[string]interface{}{"cancelled": false, "resetDone": true, "clearedDirs": cleared, "message": "系统已初始化到默认状态"}, nil
}

func containsFold(list []string, p string) bool {
	for _, s := range list {
		if samePathFold(s, p) {
			return true
		}
	}
	return false
}

func (a *App) dbPathOf(cfg *config.Config) string {
	p := strings.TrimSpace(cfg.Database.SQLite.Path)
	if p == "" {
		p = "data/app.db"
	}
	return a.resolveAppPath(p)
}

func (a *App) userDataRootOf(cfg *config.Config) string {
	p := strings.TrimSpace(cfg.Browser.UserDataRoot)
	if p == "" {
		p = "data"
	}
	return a.resolveAppPath(p)
}

// clearLocalTables 清空实例、代理、内核、书签、分组表。
func (a *App) clearLocalTables() error {
	if a.db == nil || a.db.GetConn() == nil {
		return errors.New("数据库未初始化")
	}
	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"browser_profiles", "browser_proxies", "browser_cores", "browser_bookmarks", "browser_groups"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil && !strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return fmt.Errorf("清空数据表失败(%s): %w", t, err)
		}
	}
	_, _ = tx.Exec(`DELETE FROM sqlite_sequence WHERE name = 'browser_bookmarks'`)
	return tx.Commit()
}

// stopForMaintenance 关闭所有实例、转换进程与测速, 避免文件被占用。
func (a *App) stopForMaintenance() {
	if m := a.browserMgr; m != nil {
		m.Mutex.Lock()
		for _, cmd := range m.BrowserProcesses {
			_ = a.stopProcessCmd(cmd)
		}
		m.BrowserProcesses = map[string]*exec.Cmd{}
		m.Mutex.Unlock()
	}
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
}

// reloadAfterMaintenance 重新读配置、重建内存数据并恢复测速。
func (a *App) reloadAfterMaintenance() error {
	if err := a.ReloadConfig(); err != nil {
		return err
	}
	if m := a.browserMgr; m != nil {
		m.Mutex.Lock()
		m.Profiles = map[string]*browser.Profile{}
		m.BrowserProcesses = map[string]*exec.Cmd{}
		m.Mutex.Unlock()
	}
	a.migrateToSQLite()
	a.registerKernelDirs()
	a.browserMgr.InitData()
	a.autoDetectCores()
	a.loadProxies()
	a.startSpeedScheduler()
	return nil
}
