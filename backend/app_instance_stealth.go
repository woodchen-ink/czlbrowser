package backend

import (
	"czl-browser/backend/internal/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// stealthScript 会注入到每个页面的新文档上下文中，补齐常见的反自动化探针。
const stealthScript = `
Object.defineProperty(navigator, 'webdriver', {
  get: () => false,
  configurable: true,
});

if (!window.chrome) {
  window.chrome = {};
}
if (!window.chrome.runtime) {
  window.chrome.runtime = {};
}

const originalQuery = window.navigator.permissions.query;
window.navigator.permissions.query = function(parameters) {
  if (parameters.name === 'notifications') {
    return Promise.resolve({ state: Notification.permission });
  }
  return originalQuery.call(this, parameters);
};

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

const originalPrepareStackTrace = Error.prepareStackTrace;
if (originalPrepareStackTrace) {
  Error.prepareStackTrace = function(error, structuredStackTrace) {
    return originalPrepareStackTrace(error, structuredStackTrace.filter(
      frame => !frame.getFileName()?.includes('pptr:')
    ));
  };
}
`

type cdpBrowserVersion struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	stateMu sync.Mutex
	nextID  int
	pending map[int]chan cdpResponse
	events  chan cdpResponse
	closed  chan struct{}
	readErr error
}

func newCDPConn(wsURL string) (*cdpConn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadDeadline(time.Time{})

	c := &cdpConn{
		conn:    conn,
		nextID:  1,
		pending: make(map[int]chan cdpResponse),
		events:  make(chan cdpResponse, 256),
		closed:  make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

func (c *cdpConn) readLoop() {
	defer close(c.closed)
	defer close(c.events)

	for {
		var msg cdpResponse
		if err := c.conn.ReadJSON(&msg); err != nil {
			c.failPending(err)
			c.stateMu.Lock()
			c.readErr = err
			c.stateMu.Unlock()
			return
		}

		if msg.Id != 0 {
			c.stateMu.Lock()
			waiter := c.pending[msg.Id]
			delete(c.pending, msg.Id)
			c.stateMu.Unlock()
			if waiter != nil {
				waiter <- msg
				close(waiter)
			}
			continue
		}

		select {
		case c.events <- msg:
		case <-c.closed:
			return
		}
	}
}

func (c *cdpConn) failPending(err error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	for id, waiter := range c.pending {
		waiter <- cdpResponse{Error: &struct {
			Message string `json:"message"`
		}{Message: err.Error()}}
		close(waiter)
		delete(c.pending, id)
	}
}

func (c *cdpConn) call(method string, params map[string]any, sessionID string) (map[string]any, error) {
	c.stateMu.Lock()
	id := c.nextID
	c.nextID++
	waiter := make(chan cdpResponse, 1)
	c.pending[id] = waiter
	c.stateMu.Unlock()

	msg := cdpMessage{
		Id:        id,
		Method:    method,
		Params:    params,
		SessionID: sessionID,
	}

	c.writeMu.Lock()
	err := c.conn.WriteJSON(msg)
	c.writeMu.Unlock()
	if err != nil {
		c.stateMu.Lock()
		delete(c.pending, id)
		c.stateMu.Unlock()
		return nil, err
	}

	select {
	case resp, ok := <-waiter:
		if !ok {
			return nil, fmt.Errorf("CDP 连接已关闭")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s", strings.TrimSpace(resp.Error.Message))
		}
		return resp.Result, nil
	case <-time.After(5 * time.Second):
		c.stateMu.Lock()
		delete(c.pending, id)
		c.stateMu.Unlock()
		return nil, fmt.Errorf("CDP 调用超时: %s", method)
	case <-c.closed:
		c.stateMu.Lock()
		readErr := c.readErr
		c.stateMu.Unlock()
		if readErr != nil {
			return nil, readErr
		}
		return nil, fmt.Errorf("CDP 连接已关闭")
	}
}

func (c *cdpConn) close() {
	_ = c.conn.Close()
	<-c.closed
}

// injectStealthScripts 通过浏览器级 CDP 会话统一管理所有 target，
// 确保现有标签页和后续新开的标签页/弹窗都会收到同一套 stealth 与时区覆盖。
func injectStealthScripts(debugPort int, log *logger.Logger, profileID string, timezone string) {
	wsURL, err := waitCDPBrowserWebSocketURL(debugPort, 10, 300*time.Millisecond)
	if err != nil {
		log.Warn("浏览器级 CDP 连接准备失败，跳过 stealth 注入",
			logger.F("profile_id", profileID),
			logger.F("debug_port", debugPort),
			logger.F("error", err.Error()),
		)
		return
	}

	conn, err := newCDPConn(wsURL)
	if err != nil {
		log.Warn("浏览器级 CDP WebSocket 连接失败，跳过 stealth 注入",
			logger.F("profile_id", profileID),
			logger.F("error", err.Error()),
		)
		return
	}
	defer conn.close()

	if _, err := conn.call("Target.setAutoAttach", map[string]any{
		"autoAttach":             true,
		"waitForDebuggerOnStart": false,
		"flatten":                true,
	}, ""); err != nil {
		log.Warn("启用浏览器级 target 自动附加失败",
			logger.F("profile_id", profileID),
			logger.F("error", err.Error()),
		)
		return
	}
	_, _ = conn.call("Target.setDiscoverTargets", map[string]any{"discover": true}, "")

	var appliedMu sync.Mutex
	appliedTargets := make(map[string]struct{})

	markApplied := func(targetID string) bool {
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			return false
		}
		appliedMu.Lock()
		defer appliedMu.Unlock()
		if _, exists := appliedTargets[targetID]; exists {
			return false
		}
		appliedTargets[targetID] = struct{}{}
		return true
	}

	applySession := func(sessionID string, targetID string, targetType string) {
		if !shouldManageStealthTarget(targetType) {
			return
		}
		if !markApplied(targetID) {
			return
		}
		if err := applyStealthToSession(conn, sessionID, timezone); err != nil {
			log.Warn("target stealth/timezone 注入失败",
				logger.F("profile_id", profileID),
				logger.F("target_id", targetID),
				logger.F("target_type", targetType),
				logger.F("error", err.Error()),
			)
			return
		}
		log.Info("target stealth/timezone 注入成功",
			logger.F("profile_id", profileID),
			logger.F("target_id", targetID),
			logger.F("target_type", targetType),
			logger.F("timezone", timezone),
		)
	}

	if err := attachStealthToExistingTargets(conn, applySession); err != nil {
		log.Warn("补挂现有 target 失败",
			logger.F("profile_id", profileID),
			logger.F("error", err.Error()),
		)
	}

	log.Info("浏览器级 stealth/timezone 自动附加已启用",
		logger.F("profile_id", profileID),
		logger.F("debug_port", debugPort),
		logger.F("timezone", timezone),
	)

	for event := range conn.events {
		sessionID, targetID, targetType, ok := parseAttachedTargetEvent(event)
		if !ok {
			continue
		}
		applySession(sessionID, targetID, targetType)
	}
}

func attachStealthToExistingTargets(conn *cdpConn, applySession func(sessionID string, targetID string, targetType string)) error {
	result, err := conn.call("Target.getTargets", nil, "")
	if err != nil {
		return err
	}

	targetInfos, ok := result["targetInfos"].([]any)
	if !ok {
		return nil
	}

	for _, raw := range targetInfos {
		info, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		targetID := strings.TrimSpace(mapString(info, "targetId"))
		targetType := strings.TrimSpace(mapString(info, "type"))
		if targetID == "" || !shouldManageStealthTarget(targetType) {
			continue
		}

		attachResult, err := conn.call("Target.attachToTarget", map[string]any{
			"targetId": targetID,
			"flatten":  true,
		}, "")
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "already attached") {
				continue
			}
			return err
		}
		sessionID := strings.TrimSpace(mapString(attachResult, "sessionId"))
		if sessionID == "" {
			continue
		}
		applySession(sessionID, targetID, targetType)
	}
	return nil
}

func applyStealthToSession(conn *cdpConn, sessionID string, timezone string) error {
	if sessionID == "" {
		return fmt.Errorf("empty session id")
	}
	if _, err := conn.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{
		"source": stealthScript,
	}, sessionID); err != nil {
		return fmt.Errorf("Page.addScriptToEvaluateOnNewDocument failed: %w", err)
	}
	if timezone != "" {
		if _, err := conn.call("Emulation.setTimezoneOverride", map[string]any{
			"timezoneId": timezone,
		}, sessionID); err != nil {
			return fmt.Errorf("Emulation.setTimezoneOverride failed: %w", err)
		}
	}
	if _, err := conn.call("Runtime.evaluate", map[string]any{
		"expression":            stealthScript,
		"includeCommandLineAPI": false,
	}, sessionID); err != nil {
		return fmt.Errorf("Runtime.evaluate failed: %w", err)
	}
	return nil
}

func shouldManageStealthTarget(targetType string) bool {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "page", "background_page", "webview":
		return true
	default:
		return false
	}
}

func parseAttachedTargetEvent(event cdpResponse) (string, string, string, bool) {
	if event.Method != "Target.attachedToTarget" || event.Params == nil {
		return "", "", "", false
	}
	sessionID := strings.TrimSpace(mapString(event.Params, "sessionId"))
	targetInfo, ok := event.Params["targetInfo"].(map[string]any)
	if !ok {
		return "", "", "", false
	}
	targetID := strings.TrimSpace(mapString(targetInfo, "targetId"))
	targetType := strings.TrimSpace(mapString(targetInfo, "type"))
	if sessionID == "" || targetID == "" {
		return "", "", "", false
	}
	return sessionID, targetID, targetType, true
}

func waitCDPBrowserWebSocketURL(debugPort int, attempts int, delay time.Duration) (string, error) {
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		wsURL, err := getCDPBrowserWebSocketURL(debugPort)
		if err == nil && wsURL != "" {
			return wsURL, nil
		}
		lastErr = err
		time.Sleep(delay)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no websocket url")
	}
	return "", lastErr
}

func getCDPBrowserWebSocketURL(debugPort int) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", debugPort))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload cdpBrowserVersion
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.WebSocketDebuggerURL) == "" {
		return "", fmt.Errorf("no websocket url")
	}
	return strings.TrimSpace(payload.WebSocketDebuggerURL), nil
}
