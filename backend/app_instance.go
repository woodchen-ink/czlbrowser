package backend

import (
	"bytes"
	"czl-chrome/backend/internal/browser"
	"czl-chrome/backend/internal/logger"
	"czl-chrome/backend/internal/proxy"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// 浏览器实例管理 API
// ============================================================================

func (a *App) BrowserInstanceStart(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, nil, nil, false)
}

// BrowserInstanceStartWithParams 通过额外参数启动实例（仅本次启动生效，不落库）
func (a *App) BrowserInstanceStartWithParams(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs)
}

func (a *App) browserInstanceStartInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool) (*BrowserProfile, error) {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		err := fmt.Errorf("实例启动失败：未找到实例配置（ID=%s）。请刷新列表后重试。", profileId)
		log.Error("实例不存在", logger.F("profile_id", profileId), logger.F("reason", err.Error()))
		return nil, err
	}
	if profile.Running {
		return profile, nil
	}

	proxyChanged := a.browserMgr.ApplyDefaults(profile)
	if proxyChanged {
		_ = a.browserMgr.SaveProfiles()
	}

	chromeBinaryPath, err := a.browserMgr.ResolveChromeBinary(profile)
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：%w", err)
		log.Error("内核路径解析失败", logger.F("profile_id", profileId), logger.F("error", err), logger.F("reason", startErr.Error()))
		profile.LastError = startErr.Error()
		return profile, startErr
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		startErr := fmt.Errorf("实例启动失败：无法创建用户数据目录 %s。原因：%w。请检查目录权限或路径配置。", userDataDir, err)
		log.Error("用户数据目录创建失败", logger.F("profile_id", profileId), logger.F("dir", userDataDir), logger.F("error", err), logger.F("reason", startErr.Error()))
		profile.LastError = startErr.Error()
		return profile, startErr
	}
	// 每次启动时合并默认书签（已存在的 URL 不重复添加）
	if err := browser.EnsureDefaultBookmarks(userDataDir, a.BookmarkList()); err != nil {
		log.Error("默认书签写入失败", logger.F("error", err.Error()))
	}

	proxies := a.getLatestProxies()
	acquiredXrayBridgeKey := ""
	releaseXrayBridge := false
	defer func() {
		if releaseXrayBridge && acquiredXrayBridgeKey != "" && a.xrayMgr != nil {
			a.xrayMgr.ReleaseBridge(acquiredXrayBridgeKey)
		}
	}()

	// 解析实际代理配置（可能来自 proxyId 引用）
	resolvedProxyConfig := strings.TrimSpace(profile.ProxyConfig)
	if profile.ProxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, profile.ProxyId) {
				resolvedProxyConfig = strings.TrimSpace(item.ProxyConfig)
				break
			}
		}
	}
	effectiveProxy := resolvedProxyConfig
	log.Info("代理配置检查",
		logger.F("profile_id", profileId),
		logger.F("proxy_id", profile.ProxyId),
		logger.F("profile_proxy_config", profile.ProxyConfig),
		logger.F("resolved_proxy_config", resolvedProxyConfig),
	)
	if supported, errorMsg := proxy.ValidateProxyConfig(resolvedProxyConfig, proxies, profile.ProxyId); !supported {
		startErr := fmt.Errorf("实例启动失败：%s", errorMsg)
		profile.LastError = startErr.Error()
		log.Error("代理配置无效", logger.F("profile_id", profileId), logger.F("proxy_id", profile.ProxyId), logger.F("error", errorMsg), logger.F("reason", startErr.Error()))
		return profile, startErr
	}

	if proxy.IsSingBoxProtocol(resolvedProxyConfig) {
		// hysteria2 / tuic → sing-box 桥接
		socksURL, bridgeErr := a.singboxMgr.EnsureBridge(resolvedProxyConfig, proxies, profile.ProxyId)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：代理桥接启动失败（sing-box）。原因：%v。请检查代理节点配置、sing-box.exe 是否存在，以及本地端口是否被占用。", bridgeErr)
			log.Error("代理桥接失败(sing-box)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			profile.LastError = startErr.Error()
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "proxy:bridge:failed", map[string]interface{}{
					"profileId":   profileId,
					"profileName": profile.ProfileName,
					"error":       startErr.Error(),
				})
			}
			return profile, startErr
		}
		effectiveProxy = socksURL
		log.Info("sing-box 桥接成功", logger.F("socks_url", socksURL))
	} else if proxy.RequiresBridge(resolvedProxyConfig, proxies, profile.ProxyId) {
		// vmess / vless / trojan / ss → xray 桥接
		socksURL, bridgeKey, bridgeErr := a.xrayMgr.AcquireBridge(resolvedProxyConfig, proxies, profile.ProxyId)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：代理桥接启动失败（xray）。原因：%v。请检查代理节点配置、xray.exe 是否存在，以及本地端口是否被占用。", bridgeErr)
			log.Error("代理桥接失败(xray)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			profile.LastError = startErr.Error()
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "proxy:bridge:failed", map[string]interface{}{
					"profileId":   profileId,
					"profileName": profile.ProfileName,
					"error":       startErr.Error(),
				})
			}
			return profile, startErr
		}
		acquiredXrayBridgeKey = bridgeKey
		releaseXrayBridge = bridgeKey != ""
		effectiveProxy = socksURL
		log.Info("xray 桥接成功", logger.F("socks_url", socksURL))
	}

	debugPort, err := nextAvailablePort()
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：本地调试端口分配失败。原因：%v。请关闭占用端口的程序后重试。", err)
		log.Error("调试端口分配失败", logger.F("profile_id", profileId), logger.F("error", err), logger.F("reason", startErr.Error()))
		profile.LastError = startErr.Error()
		return profile, startErr
	}

	args := []string{
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		fmt.Sprintf("--remote-debugging-port=%d", debugPort),
		"--disable-session-crashed-bubble",
		"--hide-crash-restore-bubble",
		"--no-default-browser-check",
		// 禁用密码保存提示（减少 UI 干扰）
		"--disable-save-password-bubble",
	}

	hasFingerprint := false
	for _, arg := range profile.FingerprintArgs {
		if strings.HasPrefix(arg, "--fingerprint=") {
			hasFingerprint = true
			break
		}
	}
	if !hasFingerprint {
		seed := 0
		for _, char := range profile.ProfileId {
			seed = (seed << 5) - seed + int(char)
		}
		if seed < 0 {
			seed = -seed
		}
		args = append(args, fmt.Sprintf("--fingerprint=%d", seed))
	}

	if effectiveProxy == "direct://" {
		// 强制直连，覆盖系统全局代理
		args = append(args, "--proxy-server=direct://")
	} else if effectiveProxy != "" {
		args = append(args, fmt.Sprintf("--proxy-server=%s", effectiveProxy))
	}
	args = append(args, profile.FingerprintArgs...)
	args = append(args, profile.LaunchArgs...)
	args = append(args, normalizeNonEmptyStrings(extraLaunchArgs)...)
	var proxyTimezone string
	args, proxyTimezone = a.applyProxyLocaleArgs(args, profile.ProxyId, resolvedProxyConfig, proxies, log, profileId)
	args = autoFillWebGLArgs(args, log, profileId)
	// 有代理时，若用户未配置 WebRTC 策略，则强制禁用非代理 UDP 防止真实 IP 泄露
	if effectiveProxy != "" && effectiveProxy != "direct://" {
		args = upsertKeyValueArg(args, "--webrtc-ip-handling-policy", "disable_non_proxied_udp")
	}

	if normalizedURLs := normalizeNonEmptyStrings(startURLs); len(normalizedURLs) > 0 {
		args = append(args, normalizedURLs...)
	} else if !skipDefaultStartURLs {
		args = browser.BuildLaunchArgs(args, profile)
	}

	// 启动前修补 Preferences，清除上次可能的异常退出标记，避免 Chrome 显示"未正常关闭"恢复页面
	patchChromePreferencesForCleanStart(userDataDir, log, profileId)

	cmd := exec.Command(chromeBinaryPath, args...)
	// 注意：不对 Chrome 进程调用 hideWindow——Chrome 是 GUI 程序，HideWindow 会导致浏览器窗口不可见
	cmd.Dir = filepath.Dir(chromeBinaryPath)
	// 通过 TZ 环境变量在进程级别覆盖时区，对所有标签页生效（比 CDP per-target 更可靠）
	if proxyTimezone != "" {
		cmd.Env = append(os.Environ(), "TZ="+proxyTimezone)
		log.Info("设置浏览器进程时区环境变量", logger.F("profile_id", profileId), logger.F("TZ", proxyTimezone))
	}
	stderrCapture := newCappedLogBuffer(64 * 1024)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderrCapture
	if err := cmd.Start(); err != nil {
		startErr := fmt.Errorf("%s", describeChromeProcessStartError(chromeBinaryPath, err))
		log.Error("浏览器进程启动失败", logger.F("profile_id", profileId), logger.F("chrome", chromeBinaryPath), logger.F("error", err), logger.F("reason", startErr.Error()))
		profile.LastError = startErr.Error()
		return profile, startErr
	}
	if err := waitBrowserDebugPortReady(debugPort, cmd, browserStartReadyTimeout); err != nil {
		launchOutput := summarizeProcessOutput(stderrCapture.String(), 8, 1800)
		startErr := fmt.Errorf("%s", describeBrowserReadyTimeout(debugPort, browserStartReadyTimeout))
		if errors.Is(err, errBrowserProcessExitedBeforeReady) {
			startErr = fmt.Errorf("实例启动失败：浏览器进程在调试端口 %d 就绪前提前退出。可能是用户数据目录异常或被占用，请检查或重置实例数据后重试。", debugPort)
		}
		if launchOutput != "" {
			startErr = fmt.Errorf("%s\n内核输出（末尾）：%s", startErr.Error(), launchOutput)
		}
		log.Error("浏览器启动未就绪", logger.F("profile_id", profileId), logger.F("chrome", chromeBinaryPath), logger.F("error", err), logger.F("reason", startErr.Error()))
		_ = a.cleanupFailedBrowserStart(cmd)
		profile.LastError = startErr.Error()
		return profile, startErr
	}

	a.browserMgr.BrowserProcesses[profileId] = cmd
	profile.Running = true
	profile.DebugPort = debugPort
	profile.Pid = cmd.Process.Pid
	profile.LastStartAt = time.Now().Format(time.RFC3339)
	profile.LastError = ""
	if acquiredXrayBridgeKey != "" {
		a.bindProfileXrayBridge(profileId, acquiredXrayBridgeKey)
		releaseXrayBridge = false
	}

	log.Info("实例启动", logger.F("profile_id", profileId), logger.F("debug_port", debugPort), logger.F("pid", profile.Pid), logger.F("proxy", effectiveProxy), logger.F("args", strings.Join(args, " ")))
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "browser:instance:started", profileId)
	}

	// 异步注入 stealth 脚本（navigator.webdriver 等反检测补丁）+ 时区覆盖
	go injectStealthScripts(debugPort, log, profileId, proxyTimezone)

	go a.waitBrowserProcess(profileId, cmd)
	return profile, nil
}

func (a *App) BrowserInstanceStop(profileId string) (*BrowserProfile, error) {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}

	cmd := a.browserMgr.BrowserProcesses[profileId]
	if cmd != nil && cmd.Process != nil {
		if err := a.stopBrowserProcess(cmd); err != nil {
			log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("error", err))
			profile.LastError = err.Error()
			return profile, err
		}
	}

	profile.Running = false
	profile.LastStopAt = time.Now().Format(time.RFC3339)
	delete(a.browserMgr.BrowserProcesses, profileId)
	a.releaseProfileXrayBridge(profileId)

	log.Info("实例停止", logger.F("profile_id", profileId))
	return profile, nil
}

func (a *App) BrowserInstanceRestart(profileId string) (*BrowserProfile, error) {
	if _, err := a.BrowserInstanceStop(profileId); err != nil {
		return nil, err
	}
	return a.BrowserInstanceStart(profileId)
}

// BrowserProfileBatchSetTags 批量为实例设置标签（追加模式：将 tags 加入已有标签；replace 模式：直接替换）
func (a *App) BrowserProfileBatchSetTags(profileIds []string, tags []string, replace bool) error {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	for _, profileId := range profileIds {
		profile, exists := a.browserMgr.Profiles[profileId]
		if !exists {
			continue
		}
		if replace {
			profile.Tags = tags
		} else {
			// 追加去重
			existing := make(map[string]struct{})
			for _, t := range profile.Tags {
				existing[t] = struct{}{}
			}
			for _, t := range tags {
				if _, ok := existing[t]; !ok {
					profile.Tags = append(profile.Tags, t)
					existing[t] = struct{}{}
				}
			}
		}
		profile.UpdatedAt = time.Now().Format(time.RFC3339)
		if a.browserMgr.ProfileDAO != nil {
			if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
				log.Error("批量设置标签失败", logger.F("profile_id", profileId), logger.F("error", err))
				return err
			}
		}
	}
	return nil
}

// BrowserProfileBatchRemoveTags 批量从实例移除指定标签
func (a *App) BrowserProfileBatchRemoveTags(profileIds []string, tags []string) error {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	removeSet := make(map[string]struct{})
	for _, t := range tags {
		removeSet[t] = struct{}{}
	}

	for _, profileId := range profileIds {
		profile, exists := a.browserMgr.Profiles[profileId]
		if !exists {
			continue
		}
		filtered := profile.Tags[:0]
		for _, t := range profile.Tags {
			if _, ok := removeSet[t]; !ok {
				filtered = append(filtered, t)
			}
		}
		profile.Tags = filtered
		profile.UpdatedAt = time.Now().Format(time.RFC3339)
		if a.browserMgr.ProfileDAO != nil {
			if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
				log.Error("批量移除标签失败", logger.F("profile_id", profileId), logger.F("error", err))
				return err
			}
		}
	}
	return nil
}

// BrowserRenameTag 重命名所有实例中的指定标签
func (a *App) BrowserRenameTag(oldName string, newName string) error {
	log := logger.New("Browser")
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("标签名称不能为空")
	}

	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	changedCount := 0
	for profileId, profile := range a.browserMgr.Profiles {
		tagChanged := false
		var newTags []string
		for _, t := range profile.Tags {
			if strings.EqualFold(t, oldName) {
				newTags = append(newTags, newName)
				tagChanged = true
			} else {
				newTags = append(newTags, t)
			}
		}

		if tagChanged {
			// 去重
			uniqueTags := make([]string, 0)
			seen := make(map[string]struct{})
			for _, t := range newTags {
				if _, ok := seen[t]; !ok {
					uniqueTags = append(uniqueTags, t)
					seen[t] = struct{}{}
				}
			}

			profile.Tags = uniqueTags
			profile.UpdatedAt = time.Now().Format(time.RFC3339)
			if a.browserMgr.ProfileDAO != nil {
				if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
					log.Error("重命名标签保存失败", logger.F("profile_id", profileId), logger.F("error", err))
					return err
				}
			}
			changedCount++
		}
	}

	if changedCount > 0 && a.browserMgr.ProfileDAO == nil {
		if err := a.browserMgr.SaveProfiles(); err != nil {
			return err
		}
	}

	if changedCount > 0 {
		log.Info("重命名标签成功", logger.F("old", oldName), logger.F("new", newName), logger.F("changed_profiles", changedCount))
	}
	return nil
}

func (a *App) BrowserInstanceStatus(profileId string) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	return profile, nil
}

func (a *App) BrowserInstanceOpenUrl(profileId string, targetUrl string) bool {
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	a.browserMgr.Mutex.Unlock()
	if !exists || !profile.Running {
		return false
	}
	return true
}

func (a *App) BrowserInstanceGetTabs(profileId string) []BrowserTab {
	return []BrowserTab{
		{TabId: "tab-1", Title: "新标签页", Url: "about:blank", Active: true},
		{TabId: "tab-2", Title: "示例站点", Url: "https://example.com", Active: false},
	}
}

func (a *App) waitBrowserProcess(profileId string, cmd *exec.Cmd) {
	err := cmd.Wait()

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	wasRunning := exists && profile.Running
	if exists {
		profile.Running = false
		profile.LastStopAt = time.Now().Format(time.RFC3339)
	}
	delete(a.browserMgr.BrowserProcesses, profileId)
	a.browserMgr.Mutex.Unlock()
	a.releaseProfileXrayBridge(profileId)

	if a.ctx == nil {
		return
	}

	// 进程是正常退出（用户手动关闭）还是异常崩溃
	if wasRunning && err != nil {
		log := logger.New("Browser")
		// 异常退出，推送崩溃通知
		profileName := profileId
		if exists {
			profileName = profile.ProfileName
			profile.LastError = fmt.Sprintf("实例运行异常退出：%s", err.Error())
		}
		log.Error("浏览器进程异常退出", logger.F("profile_id", profileId), logger.F("profile_name", profileName), logger.F("error", err))
		runtime.EventsEmit(a.ctx, "browser:instance:crashed", map[string]interface{}{
			"profileId":   profileId,
			"profileName": profileName,
			"error":       err.Error(),
		})
	} else {
		runtime.EventsEmit(a.ctx, "browser:instance:stopped", profileId)
	}
}

type cappedLogBuffer struct {
	max int
	buf bytes.Buffer
}

func newCappedLogBuffer(maxBytes int) *cappedLogBuffer {
	return &cappedLogBuffer{max: maxBytes}
}

func (b *cappedLogBuffer) Write(p []byte) (int, error) {
	if b == nil || b.max <= 0 || len(p) == 0 {
		return len(p), nil
	}
	if len(p) >= b.max {
		b.buf.Reset()
		_, _ = b.buf.Write(p[len(p)-b.max:])
		return len(p), nil
	}
	overflow := b.buf.Len() + len(p) - b.max
	if overflow > 0 {
		_ = b.buf.Next(overflow)
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *cappedLogBuffer) String() string {
	if b == nil {
		return ""
	}
	return b.buf.String()
}

func summarizeProcessOutput(raw string, maxLines int, maxChars int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\r\n", "\n")
	trimmed = strings.ReplaceAll(trimmed, "\r", "\n")
	lines := strings.Split(trimmed, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		v := strings.TrimSpace(line)
		if v != "" {
			cleaned = append(cleaned, v)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	if maxLines > 0 && len(cleaned) > maxLines {
		cleaned = cleaned[len(cleaned)-maxLines:]
	}
	summary := strings.Join(cleaned, " | ")
	if maxChars > 0 && len(summary) > maxChars {
		return summary[len(summary)-maxChars:]
	}
	return summary
}

func (a *App) cleanupFailedBrowserStart(cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	_ = a.stopProcessCmd(cmd)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	return nil
}

// applyProxyLocaleArgs 根据代理出口 IP 的地理信息设置语言参数，并返回解析到的 IANA 时区标识。
// 时区不再通过 --timezone 启动参数设置（标准 Chromium 不支持），改为通过 CDP Emulation.setTimezoneOverride 覆盖。
func (a *App) applyProxyLocaleArgs(args []string, proxyID string, resolvedProxyConfig string, proxies []browser.Proxy, log *logger.Logger, profileID string) ([]string, string) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return args, ""
	}
	if strings.EqualFold(strings.TrimSpace(resolvedProxyConfig), "direct://") {
		return args, ""
	}

	timezone, countryCode, ok := a.resolveProxyLocale(proxyID, proxies)
	if !ok || timezone == "" {
		log.Warn("代理出口时区解析失败，浏览器时区可能与 IP 地区不一致，建议先对代理执行 IP 检测",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", proxyID),
		)
		return args, ""
	}
	language := defaultLanguageByCountryCode(countryCode)
	if language == "" {
		language = "en-US"
	}

	updated := upsertKeyValueArg(args, "--lang", language)
	log.Info("按代理出口自动设置时区与语言",
		logger.F("profile_id", profileID),
		logger.F("proxy_id", proxyID),
		logger.F("timezone", timezone),
		logger.F("language", language),
		logger.F("country_code", countryCode),
	)
	return updated, timezone
}

func (a *App) resolveProxyLocale(proxyID string, proxies []browser.Proxy) (string, string, bool) {
	for _, item := range proxies {
		if !strings.EqualFold(item.ProxyId, proxyID) {
			continue
		}
		if timezone, countryCode, ok := parseProxyLocaleFromHealthJSON(item.LastIPHealthJSON); ok {
			return timezone, countryCode, true
		}

		data, err := proxy.FetchIPInfo(proxyID, proxies, a.xrayMgr, a.singboxMgr)
		if err != nil {
			return "", "", false
		}
		result := buildProxyIPHealthResult(proxyID, data, nil)
		a.persistProxyIPHealthResult(result)
		if result.Timezone != "" {
			return result.Timezone, strings.ToUpper(strings.TrimSpace(result.CountryCode)), true
		}
		return "", "", false
	}
	return "", "", false
}

func parseProxyLocaleFromHealthJSON(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", "", false
	}

	pick := func(m map[string]interface{}, keys ...string) string {
		for _, key := range keys {
			if v := strings.TrimSpace(mapString(m, key)); v != "" {
				return v
			}
		}
		return ""
	}

	timezone := pick(payload, "timezone")
	countryCode := pick(payload, "countryCode", "country_code")

	if rawData, ok := payload["rawData"].(map[string]interface{}); ok {
		if timezone == "" {
			timezone = pick(rawData, "timezone")
		}
		if countryCode == "" {
			countryCode = pick(rawData, "countryCode", "country_code")
		}
	}
	if data, ok := payload["data"].(map[string]interface{}); ok {
		if timezone == "" {
			timezone = pick(data, "timezone")
		}
		if countryCode == "" {
			countryCode = pick(data, "countryCode", "country_code")
		}
	}

	timezone = strings.TrimSpace(timezone)
	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	if timezone == "" {
		return "", "", false
	}
	return timezone, countryCode, true
}

func defaultLanguageByCountryCode(countryCode string) string {
	switch strings.ToUpper(strings.TrimSpace(countryCode)) {
	case "DE", "AT", "CH":
		return "de-DE"
	case "FR":
		return "fr-FR"
	case "ES":
		return "es-ES"
	case "IT":
		return "it-IT"
	case "JP":
		return "ja-JP"
	case "KR":
		return "ko-KR"
	case "RU":
		return "ru-RU"
	case "CN":
		return "zh-CN"
	case "TW":
		return "zh-TW"
	case "HK":
		return "zh-HK"
	case "GB":
		return "en-GB"
	case "US", "CA", "AU", "NZ":
		return "en-US"
	default:
		return "en-US"
	}
}

func upsertKeyValueArg(args []string, key string, value string) []string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return args
	}

	prefix := strings.ToLower(key + "=")
	replacement := key + "=" + value
	out := make([]string, 0, len(args)+1)
	replaced := false

	for i := 0; i < len(args); i++ {
		current := strings.TrimSpace(args[i])
		lower := strings.ToLower(current)

		if strings.HasPrefix(lower, prefix) {
			if !replaced {
				out = append(out, replacement)
				replaced = true
			}
			continue
		}
		if strings.EqualFold(current, key) {
			if !replaced {
				out = append(out, replacement)
				replaced = true
			}
			if i+1 < len(args) && !strings.HasPrefix(strings.TrimSpace(args[i+1]), "--") {
				i++
			}
			continue
		}

		out = append(out, current)
	}

	if !replaced {
		out = append(out, replacement)
	}
	return out
}

func normalizeNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func filterManagedLaunchArgs(args []string) ([]string, []string) {
	if len(args) == 0 {
		return nil, nil
	}

	managedValueFlags := map[string]bool{
		"--proxy-server":          true,
		"--proxy-pac-url":         true,
		"--remote-debugging-port": true,
		"--user-data-dir":         true,
	}
	managedBoolFlags := map[string]bool{
		"--proxy-auto-detect":     true,
		"--no-proxy-server":       true,
		"--remote-debugging-pipe": true,
	}
	managedPrefixFlags := []string{
		"--proxy-server=",
		"--proxy-pac-url=",
		"--remote-debugging-port=",
		"--user-data-dir=",
	}

	filtered := make([]string, 0, len(args))
	removed := make([]string, 0, 8)
	for i := 0; i < len(args); i++ {
		v := strings.TrimSpace(args[i])
		if v == "" {
			continue
		}
		l := strings.ToLower(v)

		removedByPrefix := false
		for _, prefix := range managedPrefixFlags {
			if strings.HasPrefix(l, prefix) {
				removedByPrefix = true
				break
			}
		}
		if removedByPrefix {
			removed = append(removed, v)
			continue
		}

		if managedBoolFlags[l] {
			removed = append(removed, v)
			continue
		}

		if managedValueFlags[l] {
			removed = append(removed, v)
			// 支持两段式参数: --proxy-server socks5://127.0.0.1:1080
			if i+1 < len(args) {
				nextRaw := strings.TrimSpace(args[i+1])
				if nextRaw != "" && !strings.HasPrefix(nextRaw, "--") {
					removed = append(removed, nextRaw)
					i++
				}
			}
			continue
		}

		filtered = append(filtered, v)
	}
	return filtered, removed
}

func (a *App) stopBrowserProcess(cmd *exec.Cmd) error {
	return a.stopProcessCmd(cmd)
}

func (a *App) stopProcessCmd(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Windows 下优先非强制 taskkill，尽量让 Chromium 走正常退出路径，减少“恢复页面”提示。
	if stdruntime.GOOS == "windows" {
		pid := cmd.Process.Pid
		if pid > 0 {
			softKillCmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T")
			hideWindow(softKillCmd)
			if err := softKillCmd.Run(); err == nil {
				if waitProcessExitWindows(pid, 3*time.Second) {
					return nil
				}
				forceKillCmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid), "/T")
				hideWindow(forceKillCmd)
				if forceErr := forceKillCmd.Run(); forceErr == nil {
					_ = waitProcessExitWindows(pid, 2*time.Second)
					return nil
				}
			}
		}
	}

	err := cmd.Process.Kill()
	if err == nil || isProcessAlreadyFinished(err) {
		return nil
	}
	return err
}

func isProcessAlreadyFinished(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "process already finished") {
		return true
	}
	if strings.Contains(msg, "not found") {
		return true
	}
	if strings.Contains(msg, "no process") {
		return true
	}
	if strings.Contains(msg, "不存在") {
		return true
	}
	return false
}

func waitProcessExitWindows(pid int, timeout time.Duration) bool {
	if pid <= 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alive, err := isProcessAliveWindows(pid)
		if err == nil && !alive {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	alive, err := isProcessAliveWindows(pid)
	if err != nil {
		return false
	}
	return !alive
}

func isProcessAliveWindows(pid int) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return false, nil
	}
	if strings.HasPrefix(strings.ToUpper(line), "INFO:") {
		return false, nil
	}
	token := fmt.Sprintf("\",\"%d\",", pid)
	return strings.Contains(line, token), nil
}
