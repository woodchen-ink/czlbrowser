package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/proxy"
)

// launchFailure 记录到实例的 LastError 并原样返回给前端。
func launchFailure(p *BrowserProfile, format string, args ...any) (*BrowserProfile, error) {
	err := fmt.Errorf(format, args...)
	p.LastError = err.Error()
	logger.New("Browser").Error("实例启动失败", logger.F("profile_id", p.ProfileId), logger.F("reason", err.Error()))
	return p, err
}

// startInstance 启动实例。全程持有实例锁; 调用方负责同步空间的占用检查与登录态下载。
// extraArgs 追加到启动参数末尾, startURLs 作为启动时打开的页面。
func (a *App) startInstance(profileID string, extraArgs, startURLs []string) (*BrowserProfile, error) {
	m := a.browserMgr
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	p, ok := m.Profiles[profileID]
	if !ok {
		return nil, fmt.Errorf("实例启动失败：未找到实例配置（ID=%s）。请刷新列表后重试。", profileID)
	}
	if p.Running {
		return p, nil
	}
	// 代理地址随代理池更新; 只写这一个实例 (整体保存会把远端已删、仍在运行的实例写回)
	if m.ApplyDefaults(p) && m.ProfileDAO != nil {
		_ = m.ProfileDAO.Upsert(p)
	}

	exe, err := m.ResolveChromeBinary(p)
	if err != nil {
		return launchFailure(p, "实例启动失败：%w", err)
	}
	dataDir := m.ResolveUserDataDir(p)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return launchFailure(p, "实例启动失败：无法创建用户数据目录 %s。原因：%w。请检查目录权限或路径配置。", dataDir, err)
	}
	if err := browser.EnsureDefaultBookmarks(dataDir, a.BookmarkList()); err != nil {
		logger.New("Browser").Warn("默认书签写入失败", logger.F("error", err))
	}

	pool := a.getLatestProxies()
	route, err := a.routeProxy(p, pool)
	if err != nil {
		a.emit("proxy:bridge:failed", map[string]interface{}{"profileId": p.ProfileId, "profileName": p.ProfileName, "error": err.Error()})
		p.LastError = err.Error()
		return p, err
	}
	keepBridge := false
	defer func() {
		if !keepBridge && route.bridgeKey != "" {
			a.xrayMgr.ReleaseBridge(route.bridgeKey)
		}
	}()

	port, err := nextAvailablePort()
	if err != nil {
		return launchFailure(p, "实例启动失败：本地调试端口分配失败。原因：%v。请关闭占用端口的程序后重试。", err)
	}
	args, timezone := a.launchArgs(p, dataDir, port, route, pool, extraArgs)
	plan := buildKernelLaunchPlan(browser.DetectCoreKind(exe), args, timezone, logger.New("Browser"), p.ProfileId)
	args = plan.Args
	if route.effective != "" && route.effective != "direct://" {
		// 走代理时如未自定义 WebRTC 策略, 禁止不经代理的 UDP, 防止泄露真实 IP
		args = upsertKeyValueArg(args, "--webrtc-ip-handling-policy", "disable_non_proxied_udp")
	}
	args = append(args, normalizeNonEmptyStrings(startURLs)...)

	prepareUserDataDir(dataDir, p.ProfileId)
	cmd, err := spawnChrome(exe, args, port)
	if err != nil {
		return launchFailure(p, "%s", err.Error())
	}

	now := time.Now().Format(time.RFC3339)
	m.BrowserProcesses[profileID] = cmd
	p.Running, p.DebugPort, p.Pid, p.LastStartAt, p.LastError = true, port, cmd.Process.Pid, now, ""
	if dao, ok := m.ProfileDAO.(interface{ MarkStarted(string, string) error }); ok {
		_ = dao.MarkStarted(profileID, now)
	}
	if route.bridgeKey != "" {
		a.bindProfileXrayBridge(profileID, route.bridgeKey)
		keepBridge = true
	}
	logger.New("Browser").Info("实例已启动", logger.F("profile_id", profileID), logger.F("pid", p.Pid), logger.F("port", port),
		logger.F("proxy", route.effective), logger.F("core_kind", string(plan.Kind)), logger.F("args", strings.Join(args, " ")))
	a.emit("browser:instance:started", profileID)
	if plan.InjectViaCDP {
		go injectStealthScripts(port, logger.New("Browser"), profileID, timezone)
	}
	go a.waitBrowserProcess(profileID, cmd)
	return p, nil
}

// proxyRoute 是实例实际使用的代理。
type proxyRoute struct {
	configured string // 代理池里的原始配置
	effective  string // 交给 Chrome 的地址 (需要转换时为本机 socks5)
	bridgeKey  string // 占用的 Xray 桥接, 实例退出时归还
}

// routeProxy 校验代理并在需要时拉起 Xray / sing-box 转换。
func (a *App) routeProxy(p *BrowserProfile, pool []BrowserProxy) (proxyRoute, error) {
	r := proxyRoute{configured: strings.TrimSpace(p.ProxyConfig)}
	if p.ProxyId != "" {
		for _, item := range pool {
			if strings.EqualFold(item.ProxyId, p.ProxyId) {
				r.configured = strings.TrimSpace(item.ProxyConfig)
				break
			}
		}
	}
	r.effective = r.configured
	if ok, msg := proxy.ValidateProxyConfig(r.configured, pool, p.ProxyId); !ok {
		return r, fmt.Errorf("实例启动失败：%s", msg)
	}
	var err error
	switch {
	case proxy.IsSingBoxProtocol(r.configured):
		if r.effective, err = a.singboxMgr.EnsureBridge(r.configured, pool, p.ProxyId); err != nil {
			return r, fmt.Errorf("实例启动失败：代理桥接启动失败（sing-box）。原因：%v。请检查代理节点配置、sing-box.exe 是否存在，以及本地端口是否被占用。", err)
		}
	case proxy.RequiresBridge(r.configured, pool, p.ProxyId):
		if r.effective, r.bridgeKey, err = a.xrayMgr.AcquireBridge(r.configured, pool, p.ProxyId); err != nil {
			return r, fmt.Errorf("实例启动失败：代理桥接启动失败（xray）。原因：%v。请检查代理节点配置、xray.exe 是否存在，以及本地端口是否被占用。", err)
		}
	}
	return r, nil
}

// launchArgs 组装 Chrome 启动参数, 返回参数与实例应使用的时区 (代理出口时区优先于手动设置)。
func (a *App) launchArgs(p *BrowserProfile, dataDir string, port int, route proxyRoute, pool []BrowserProxy, extra []string) ([]string, string) {
	args := []string{
		"--user-data-dir=" + dataDir,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--disable-session-crashed-bubble",
		"--hide-crash-restore-bubble",
		"--no-default-browser-check",
		"--disable-save-password-bubble",
	}
	hasSeed := false
	for _, arg := range p.FingerprintArgs {
		hasSeed = hasSeed || strings.HasPrefix(arg, "--fingerprint=")
	}
	if !hasSeed {
		args = append(args, fmt.Sprintf("--fingerprint=%d", fingerprintSeed(p.ProfileId)))
	}
	if route.effective != "" {
		// direct:// 也要显式传, 覆盖系统全局代理
		args = append(args, "--proxy-server="+route.effective)
	}
	args = append(args, p.FingerprintArgs...)
	args = append(args, p.LaunchArgs...)
	args = append(args, normalizeNonEmptyStrings(extra)...)

	args, manualTZ := extractSwitchValueArg(args, "--timezone")
	args, exitTZ := a.applyProxyLocaleArgs(args, p.ProxyId, route.configured, pool, p.ProfileId)
	if exitTZ != "" {
		return args, exitTZ
	}
	return args, manualTZ
}

// applyProxyLocaleArgs 按代理出口地区设置浏览器语言, 返回出口时区 (取不到时为空)。
// 时区怎么生效取决于内核: 官方 Chrome 用 CDP 覆盖, Clearcote 用原生 --timezone (见 buildKernelLaunchPlan)。
func (a *App) applyProxyLocaleArgs(args []string, proxyID, configured string, pool []BrowserProxy, profileID string) ([]string, string) {
	if strings.TrimSpace(proxyID) == "" || strings.EqualFold(strings.TrimSpace(configured), "direct://") {
		return args, ""
	}
	tz, cc, ok := a.resolveProxyLocale(proxyID, pool)
	if !ok {
		logger.New("Browser").Warn("代理出口时区未知, 浏览器时区可能与 IP 地区不一致, 建议先对代理执行出口检测",
			logger.F("profile_id", profileID), logger.F("proxy_id", proxyID))
		return args, ""
	}
	return upsertKeyValueArg(args, "--lang", defaultLanguageByCountryCode(cc)), tz
}

// resolveProxyLocale 先用已保存的出口检测结果, 没有再现查一次并保存。
func (a *App) resolveProxyLocale(proxyID string, pool []BrowserProxy) (string, string, bool) {
	for _, item := range pool {
		if !strings.EqualFold(item.ProxyId, proxyID) {
			continue
		}
		if tz, cc, ok := parseProxyLocaleFromHealthJSON(item.LastIPHealthJSON); ok {
			return tz, cc, true
		}
		data, err := proxy.FetchIPInfo(proxyID, pool, a.xrayMgr, a.singboxMgr)
		if err != nil {
			return "", "", false
		}
		r := buildProxyIPHealthResult(proxyID, data, nil)
		a.persistProxyIPHealthResult(r)
		return r.Timezone, strings.ToUpper(strings.TrimSpace(r.CountryCode)), r.Timezone != ""
	}
	return "", "", false
}

// prepareUserDataDir 启动前整理用户数据目录: 清掉陈旧的单实例锁 (跨机器导入后会让 Chrome 秒退)、
// 删除被屏蔽的扩展本体、清除上次异常退出标记 (避免「恢复页面」提示)。
func prepareUserDataDir(dir, profileID string) {
	log := logger.New("Browser")
	cleanupChromeSingletonLocks(dir, log, profileID)
	cleanupBlockedChromeExtensions(dir, log, profileID)
	patchChromePreferencesForCleanStart(dir, log, profileID)
}

// spawnChrome 启动 Chrome 并等调试端口就绪; 失败时结束进程并附上内核输出的末尾。
// 不要给 Chrome 设 HideWindow, 否则浏览器窗口会不可见。
func spawnChrome(exe string, args []string, port int) (*exec.Cmd, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	stderr := newCappedLogBuffer(64 << 10)
	cmd.Stdout, cmd.Stderr = io.Discard, stderr
	if err := cmd.Start(); err != nil {
		return nil, errors.New(describeChromeProcessStartError(exe, err))
	}
	err := waitBrowserDebugPortReady(port, cmd, browserStartReadyTimeout)
	if err == nil {
		return cmd, nil
	}
	msg := describeBrowserReadyTimeout(port, browserStartReadyTimeout)
	if errors.Is(err, errBrowserProcessExitedBeforeReady) {
		msg = fmt.Sprintf("实例启动失败：浏览器进程在调试端口 %d 就绪前提前退出。可能是用户数据目录异常或被占用，请检查或重置实例数据后重试。", port)
	}
	if out := summarizeProcessOutput(stderr.String(), 8, 1800); out != "" {
		msg += "\n内核输出（末尾）：" + out
	}
	abandonProcess(cmd)
	return nil, errors.New(msg)
}

// abandonProcess 结束启动失败的进程并最多等 2 秒回收。
func abandonProcess(cmd *exec.Cmd) {
	_ = stopProcessTree(cmd.Process.Pid, 3*time.Second)
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

// waitBrowserProcess 等实例进程退出, 更新状态并通知前端 (异常退出报崩溃)。
func (a *App) waitBrowserProcess(profileID string, cmd *exec.Cmd) {
	exitErr := cmd.Wait()
	m := a.browserMgr
	m.Mutex.Lock()
	p, ok := m.Profiles[profileID]
	wasRunning := ok && p.Running
	name := profileID
	if ok {
		p.Running, p.LastStopAt = false, time.Now().Format(time.RFC3339)
		name = p.ProfileName
		if wasRunning && exitErr != nil {
			p.LastError = "实例运行异常退出：" + exitErr.Error()
		}
	}
	delete(m.BrowserProcesses, profileID)
	m.Mutex.Unlock()

	a.releaseProfileXrayBridge(profileID)
	if a.cloud != nil {
		a.cloud.OnStopped(profileID) // 同步空间: 停心跳, 后台打包上传登录态
	}
	if wasRunning && exitErr != nil {
		logger.New("Browser").Error("实例异常退出", logger.F("profile_id", profileID), logger.F("error", exitErr))
		a.emit("browser:instance:crashed", map[string]interface{}{"profileId": profileID, "profileName": name, "error": exitErr.Error()})
		return
	}
	a.emit("browser:instance:stopped", profileID)
}
