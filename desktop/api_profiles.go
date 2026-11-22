package main

import (
	"strings"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

// 绑定到前端的类型别名 (Wails 按这些名字生成 TypeScript 类型)。
type (
	BrowserProfile            = browser.Profile
	BrowserProfileInput       = browser.ProfileInput
	BrowserTab                = browser.Tab
	BrowserSettings           = browser.Settings
	BrowserProxy              = browser.Proxy
	BrowserCore               = browser.Core
	BrowserCoreValidateResult = browser.CoreValidateResult
	BrowserCoreExtendedInfo   = browser.CoreExtendedInfo
)

// ---- 实例 ----

func (a *App) BrowserProfileList() []BrowserProfile { return a.browserMgr.List() }

func (a *App) BrowserGetAllTags() []string { return a.browserMgr.GetAllTags() }

func (a *App) BrowserProfileCreate(in BrowserProfileInput) (*BrowserProfile, error) {
	return a.browserMgr.Create(in)
}

func (a *App) BrowserProfileUpdate(id string, in BrowserProfileInput) (*BrowserProfile, error) {
	return a.browserMgr.Update(id, in)
}

func (a *App) BrowserProfileDelete(id string) error { return a.browserMgr.Delete(id) }

// BrowserProfileCopy 复制实例 (新用户数据目录, 指纹用默认值)。
func (a *App) BrowserProfileCopy(id, newName string) (*BrowserProfile, error) {
	return a.browserMgr.Copy(id, newName)
}

// ---- 新实例默认值 ----

func (a *App) GetBrowserSettings() BrowserSettings {
	b := a.config.Browser
	return BrowserSettings{
		UserDataRoot:           b.UserDataRoot,
		DefaultFingerprintArgs: append([]string{}, b.DefaultFingerprintArgs...),
		DefaultLaunchArgs:      append([]string{}, b.DefaultLaunchArgs...),
		DefaultProxy:           b.DefaultProxy,
	}
}

func (a *App) SaveBrowserSettings(s BrowserSettings) error {
	b := &a.config.Browser
	b.UserDataRoot = strings.TrimSpace(s.UserDataRoot)
	b.DefaultFingerprintArgs = append([]string{}, s.DefaultFingerprintArgs...)
	b.DefaultLaunchArgs = append([]string{}, s.DefaultLaunchArgs...)
	b.DefaultProxy = strings.TrimSpace(s.DefaultProxy)
	return a.config.Save(a.mainConfigPath())
}

// ---- 内核 ----

func (a *App) BrowserCoreList() []BrowserCore        { return a.browserMgr.ListCores() }
func (a *App) BrowserCoreDelete(id string) error     { return a.browserMgr.DeleteCore(id) }
func (a *App) BrowserCoreSetDefault(id string) error { return a.browserMgr.SetDefaultCore(id) }
func (a *App) BrowserCoreExtendedInfo() []BrowserCoreExtendedInfo {
	return a.browserMgr.GetCoresExtendedInfo()
}

func (a *App) BrowserCoreValidate(dir string) BrowserCoreValidateResult {
	return a.browserMgr.ValidateCorePath(dir)
}

// ---- 运行日志 ----

// GetAppLogs 返回内存里最近的日志 (设置 → 运行日志)。
func (a *App) GetAppLogs() []logger.Entry { return logger.Recent() }

func (a *App) ClearAppLogs() { logger.ClearRecent() }
