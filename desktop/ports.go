package main

import (
	"fmt"
	"net"
)

// nextAvailablePort 让系统分配一个本机空闲端口 (给浏览器的调试端口用)。
// 拿到端口到 Chrome 绑定之间仍可能被占用, 启动失败时由上层重试。
func nextAvailablePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("无法分配可用端口: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
