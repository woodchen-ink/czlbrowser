package main

import (
	"fmt"
	"time"
)

// cdpMessage 是发给 Chrome DevTools 协议的一条命令。
type cdpMessage struct {
	Id        int            `json:"id"`
	Method    string         `json:"method,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
}

// cdpResponse 是收到的应答或事件 (事件没有 id, 带 method)。
type cdpResponse struct {
	Id        int            `json:"id,omitempty"`
	Method    string         `json:"method,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// withBrowserCDP 连上实例的浏览器级调试目标执行 fn, 结束后断开。
func withBrowserCDP(debugPort int, fn func(*cdpConn) error) error {
	wsURL, err := waitCDPBrowserWebSocketURL(debugPort, 3, 300*time.Millisecond)
	if err != nil {
		return fmt.Errorf("连接浏览器调试端口失败: %w", err)
	}
	conn, err := newCDPConn(wsURL)
	if err != nil {
		return fmt.Errorf("连接浏览器调试端口失败: %w", err)
	}
	defer conn.close()
	return fn(conn)
}
