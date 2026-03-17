package backend

import (
	"czl-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// stealthScript 是注入到每个页面的反检测 JS 脚本。
// 通过 CDP Page.addScriptToEvaluateOnNewDocument 在页面 JS 执行前注入。
// 参考 puppeteer-extra-plugin-stealth 的实现。
const stealthScript = `
// === 1. navigator.webdriver ===
// 标准 Chrome 在 CDP 模式下 navigator.webdriver 为 true，需要覆盖为 false
Object.defineProperty(navigator, 'webdriver', {
  get: () => false,
  configurable: true,
});

// === 2. window.chrome ===
// 标准 Chrome 有 window.chrome 对象，但某些自动化模式下缺失
if (!window.chrome) {
  window.chrome = {};
}
if (!window.chrome.runtime) {
  window.chrome.runtime = {};
}

// === 3. Permissions API ===
// 真实 Chrome 中 Notification permission 查询返回的状态由用户设置决定
// CDP 模式下可能返回异常值，覆盖 query 方法使其返回正常
const originalQuery = window.navigator.permissions.query;
window.navigator.permissions.query = function(parameters) {
  if (parameters.name === 'notifications') {
    return Promise.resolve({ state: Notification.permission });
  }
  return originalQuery.call(this, parameters);
};

// === 4. plugins & mimeTypes ===
// 无痕/自动化模式下 plugins 可能为空数组，真实 Chrome 至少有几个默认插件
if (navigator.plugins.length === 0) {
  Object.defineProperty(navigator, 'plugins', {
    get: () => {
      const ChromiumPDFPlugin = { name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format' };
      const ChromiumPDFViewer = { name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: '' };
      const NativeClient = { name: 'Native Client', filename: 'internal-nacl-plugin', description: '' };
      const plugins = [ChromiumPDFPlugin, ChromiumPDFViewer, NativeClient];
      plugins.item = (i) => plugins[i] || null;
      plugins.namedItem = (name) => plugins.find(p => p.name === name) || null;
      plugins.refresh = () => {};
      return plugins;
    },
    configurable: true,
  });
}

// === 5. WebGL 调试扩展 ===
// 如果 WEBGL_debug_renderer_info 不可用，不修改——标准 Chrome 默认支持
// 但部分检测站会检查 getParameter 的实现是否被篡改
// 这里不修改 WebGL 返回值，由启动参数或真实硬件决定（标准 Chrome 返回真实值，自洽）

// === 6. 屏幕分辨率一致性 ===
// outerWidth/outerHeight 和 innerWidth/innerHeight 在真实浏览器中有细微差异
// 自动化模式下有时完全相等，这里不强制修改——标准 Chrome 本身就有正常差异

// === 7. 隐藏 CDP Runtime.enable 痕迹 ===
// 某些检测站通过检查全局错误处理器来判断 CDP 是否被启用
// 这里通过覆盖 Error.prepareStackTrace 来隐藏 CDP 注入的调用栈
const originalPrepareStackTrace = Error.prepareStackTrace;
if (originalPrepareStackTrace) {
  Error.prepareStackTrace = function(error, structuredStackTrace) {
    return originalPrepareStackTrace(error, structuredStackTrace.filter(
      frame => !frame.getFileName()?.includes('pptr:')
    ));
  };
}
`

// injectStealthScripts 通过 CDP 注入反检测脚本到浏览器实例。
// 使用 Page.addScriptToEvaluateOnNewDocument 确保脚本在每个新页面的 JS 执行前运行。
// 若 timezone 非空，则通过 Emulation.setTimezoneOverride 覆盖浏览器时区，使其与代理出口 IP 一致。
// 在浏览器调试端口就绪后异步调用。
func injectStealthScripts(debugPort int, log *logger.Logger, profileId string, timezone string) {
	// 等待 CDP /json 端点稳定可用
	var wsURL string
	for i := 0; i < 10; i++ {
		time.Sleep(300 * time.Millisecond)
		url, err := getCDPWebSocketURL(debugPort)
		if err == nil && url != "" {
			wsURL = url
			break
		}
	}
	if wsURL == "" {
		log.Warn("CDP 连接失败，跳过 stealth 注入",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
		)
		return
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Warn("CDP WebSocket 连接失败，跳过 stealth 注入",
			logger.F("profile_id", profileId),
			logger.F("error", err.Error()),
		)
		return
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Page.addScriptToEvaluateOnNewDocument: 在每个新文档加载前注入脚本
	msg := cdpMessage{
		Id:     1,
		Method: "Page.addScriptToEvaluateOnNewDocument",
		Params: map[string]any{
			"source": stealthScript,
		},
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Warn("CDP stealth 脚本发送失败",
			logger.F("profile_id", profileId),
			logger.F("error", err.Error()),
		)
		return
	}

	var resp cdpResponse
	if err := conn.ReadJSON(&resp); err != nil {
		log.Warn("CDP stealth 脚本响应读取失败",
			logger.F("profile_id", profileId),
			logger.F("error", err.Error()),
		)
		return
	}
	if resp.Error != nil {
		log.Warn("CDP stealth 脚本注入失败",
			logger.F("profile_id", profileId),
			logger.F("error", resp.Error.Message),
		)
		return
	}

	// 对当前已打开的页面也执行一次脚本（addScriptToEvaluateOnNewDocument 只对后续页面生效）
	evalMsg := cdpMessage{
		Id:     2,
		Method: "Runtime.evaluate",
		Params: map[string]any{
			"expression": stealthScript,
		},
	}
	_ = conn.WriteJSON(evalMsg)
	_ = conn.ReadJSON(&resp)

	log.Info("stealth 脚本注入成功",
		logger.F("profile_id", profileId),
		logger.F("debug_port", debugPort),
	)

	// 通过 CDP 覆盖浏览器时区，使 Intl.DateTimeFormat / new Date() 等 API 返回与代理 IP 一致的时区
	if timezone != "" {
		tzMsg := cdpMessage{
			Id:     3,
			Method: "Emulation.setTimezoneOverride",
			Params: map[string]any{
				"timezoneId": timezone,
			},
		}
		if err := conn.WriteJSON(tzMsg); err != nil {
			log.Warn("CDP 时区覆盖发送失败",
				logger.F("profile_id", profileId),
				logger.F("timezone", timezone),
				logger.F("error", err.Error()),
			)
		} else if err := conn.ReadJSON(&resp); err != nil {
			log.Warn("CDP 时区覆盖响应读取失败",
				logger.F("profile_id", profileId),
				logger.F("timezone", timezone),
				logger.F("error", err.Error()),
			)
		} else if resp.Error != nil {
			log.Warn("CDP 时区覆盖失败",
				logger.F("profile_id", profileId),
				logger.F("timezone", timezone),
				logger.F("error", resp.Error.Message),
			)
		} else {
			log.Info("CDP 时区覆盖成功",
				logger.F("profile_id", profileId),
				logger.F("timezone", timezone),
			)
		}
	}
}

// getCDPWebSocketURL 从调试端口获取第一个 page target 的 WebSocket URL
func getCDPWebSocketURL(debugPort int) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json", debugPort))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil || len(targets) == 0 {
		return "", fmt.Errorf("no targets")
	}

	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerUrl != "" {
			return t.WebSocketDebuggerUrl, nil
		}
	}
	if targets[0].WebSocketDebuggerUrl != "" {
		return targets[0].WebSocketDebuggerUrl, nil
	}
	return "", fmt.Errorf("no websocket url")
}
