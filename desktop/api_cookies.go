package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// CookieInfo 是实例里的一条 Cookie (实例详情页「Cookie」)。
type CookieInfo struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HttpOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// runningDebugPort 返回运行中实例的调试端口。
func (a *App) runningDebugPort(profileID string) (int, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	p, ok := a.browserMgr.Profiles[profileID]
	if !ok {
		return 0, fmt.Errorf("profile not found: %s", profileID)
	}
	if !p.Running || p.DebugPort == 0 {
		return 0, errors.New("实例未运行或调试端口不可用")
	}
	return p.DebugPort, nil
}

// BrowserGetCookies 读取运行中实例的全部 Cookie。
func (a *App) BrowserGetCookies(profileID string) ([]CookieInfo, error) {
	port, err := a.runningDebugPort(profileID)
	if err != nil {
		return nil, err
	}
	cookies := []CookieInfo{}
	err = withBrowserCDP(port, func(c *cdpConn) error {
		res, err := c.call("Storage.getCookies", nil, "")
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(res["cookies"])
		return json.Unmarshal(raw, &cookies)
	})
	return cookies, err
}

// BrowserClearCookies 清空运行中实例的全部 Cookie。
func (a *App) BrowserClearCookies(profileID string) error {
	port, err := a.runningDebugPort(profileID)
	if err != nil {
		return err
	}
	return withBrowserCDP(port, func(c *cdpConn) error {
		_, err := c.call("Storage.clearCookies", nil, "")
		return err
	})
}

// BrowserExportCookies 以 Netscape cookies.txt 格式导出 Cookie。
func (a *App) BrowserExportCookies(profileID string) (string, error) {
	cookies, err := a.BrowserGetCookies(profileID)
	if err != nil {
		return "", err
	}
	flag := func(b bool) string {
		if b {
			return "TRUE"
		}
		return "FALSE"
	}
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n# Exported by CZL Browser\n\n")
	for _, c := range cookies {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			c.Domain, flag(strings.HasPrefix(c.Domain, ".")), c.Path, flag(c.Secure), max(int64(c.Expires), 0), c.Name, c.Value)
	}
	return b.String(), nil
}
