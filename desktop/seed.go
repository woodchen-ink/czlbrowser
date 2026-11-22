package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

// defaultBookmarks 是全新安装时的默认书签。
var defaultBookmarks = []config.BrowserBookmark{
	{Name: "Google", URL: "https://www.google.com/"},
	{Name: "Gmail", URL: "https://mail.google.com/"},
	{Name: "Claude", URL: "https://claude.ai/"},
	{Name: "ChatGPT", URL: "https://chatgpt.com/"},
	{Name: "YouTube", URL: "https://www.youtube.com/"},
}

// migrateToSQLite 给空表填数据: 早期版本存在 config.yaml / proxies.yaml 里的内核、代理、实例、书签
// 搬进数据库; 什么都没有时写入默认值 (直连代理、一个默认实例、几条常用书签)。只在本机空间执行。
func (a *App) migrateToSQLite() {
	log := logger.New("Migration")
	m := a.browserMgr
	cfg := a.config.Browser

	if cores, err := m.CoreDAO.List(); err == nil && len(cores) == 0 {
		for _, c := range cfg.Cores {
			if err := m.CoreDAO.Upsert(c); err != nil {
				log.Error("内核迁移失败", logger.F("core_id", c.CoreId), logger.F("error", err))
			}
		}
	}

	if list, err := m.ProxyDAO.List(); err == nil && len(list) == 0 {
		src, _ := config.LoadProxies(a.resolveAppPath("proxies.yaml"))
		if len(src) == 0 {
			src = cfg.Proxies
		}
		if len(src) == 0 {
			src = []browser.Proxy{browser.DirectProxy()}
		}
		for _, p := range src {
			if err := m.ProxyDAO.Upsert(p); err != nil {
				log.Error("代理迁移失败", logger.F("proxy_id", p.ProxyId), logger.F("error", err))
			}
		}
	}

	if list, err := m.ProfileDAO.List(); err == nil && len(list) == 0 {
		for _, p := range legacyProfiles(cfg) {
			if err := m.ProfileDAO.Upsert(p); err != nil {
				log.Error("实例迁移失败", logger.F("profile_id", p.ProfileId), logger.F("error", err))
			}
		}
	}

	if list, err := m.BookmarkDAO.List(); err == nil && len(list) == 0 {
		src := cfg.DefaultBookmarks
		if len(src) == 0 {
			src = defaultBookmarks
		}
		if err := m.BookmarkDAO.ReplaceAll(src); err != nil {
			log.Error("书签迁移失败", logger.F("error", err))
		}
	}
}

// legacyProfiles 返回旧配置里的实例; 没有时给出一个默认实例 (内核跟随默认)。
func legacyProfiles(cfg config.BrowserConfig) []*browser.Profile {
	if len(cfg.Profiles) == 0 {
		now := time.Now().Format(time.RFC3339)
		return []*browser.Profile{{
			ProfileId: uuid.NewString(), ProfileName: "默认实例", UserDataDir: "default",
			FingerprintArgs: cfg.DefaultFingerprintArgs, LaunchArgs: cfg.DefaultLaunchArgs,
			Tags: []string{"默认"}, ProxyId: cfg.DefaultProxy, CreatedAt: now, UpdatedAt: now,
		}}
	}
	out := make([]*browser.Profile, 0, len(cfg.Profiles))
	for _, pc := range cfg.Profiles {
		out = append(out, &browser.Profile{
			ProfileId: pc.ProfileId, ProfileName: pc.ProfileName, UserDataDir: pc.UserDataDir, CoreId: pc.CoreId,
			FingerprintArgs: pc.FingerprintArgs, ProxyId: pc.ProxyId, ProxyConfig: pc.ProxyConfig, LaunchArgs: pc.LaunchArgs,
			Tags: pc.Tags, Keywords: pc.Keywords, CreatedAt: pc.CreatedAt, UpdatedAt: pc.UpdatedAt,
		})
	}
	return out
}

// registerKernelDirs 登记 chrome\ 下手动放入的内核目录 (含 chrome.exe 的子目录), 已登记的不动。
// chrome\ 本身就有 chrome.exe 时 (早期单内核结构) 登记为「默认内核」。没有默认内核时第一个设为默认。
func (a *App) registerKernelDirs() {
	m := a.browserMgr
	existing := map[string]bool{}
	hasDefault := false
	for _, c := range m.ListCores() {
		existing[strings.ToLower(filepath.Clean(c.CorePath))] = true
		hasDefault = hasDefault || c.IsDefault
	}
	root := a.resolveAppPath("chrome")
	var found []browser.Core
	if _, err := os.Stat(filepath.Join(root, "chrome.exe")); err == nil {
		found = append(found, browser.Core{CoreId: "default", CoreName: "默认内核", CorePath: "chrome"})
	} else if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, e.Name(), "chrome.exe")); err == nil {
				found = append(found, browser.Core{CoreId: "core-" + e.Name(), CoreName: e.Name(), CorePath: filepath.Join("chrome", e.Name())})
			}
		}
	}
	log := logger.New("Browser")
	for _, c := range found {
		if existing[strings.ToLower(filepath.Clean(c.CorePath))] {
			continue
		}
		c.IsDefault = !hasDefault
		hasDefault = true
		if err := m.CoreDAO.Upsert(c); err != nil {
			log.Error("登记内核失败", logger.F("path", c.CorePath), logger.F("error", err))
			continue
		}
		log.Info("发现内核目录, 已登记", logger.F("path", c.CorePath))
	}
	m.ListCores()
}

// autoDetectCores 启动时检查已登记内核的目录是否还在, 只记日志。
func (a *App) autoDetectCores() {
	log := logger.New("Browser")
	for _, c := range a.browserMgr.ListCores() {
		if r := a.browserMgr.ValidateCorePath(c.CorePath); !r.Valid {
			log.Warn("内核目录无效", logger.F("core_id", c.CoreId), logger.F("message", r.Message))
		}
	}
}

// loadProxies 把当前空间的代理池读进 a.config 作为内存副本 (数据库读不到时的兜底)。
func (a *App) loadProxies() {
	if a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return
	}
	if list, err := a.browserMgr.ProxyDAO.List(); err == nil && len(list) > 0 {
		a.config.Browser.Proxies = list
	}
}

// getLatestProxies 返回当前空间的代理池, 优先读数据库。
func (a *App) getLatestProxies() []browser.Proxy {
	if a.browserMgr != nil && a.browserMgr.ProxyDAO != nil {
		if list, err := a.browserMgr.ProxyDAO.List(); err == nil && len(list) > 0 {
			return list
		}
	}
	return a.config.Browser.Proxies
}
