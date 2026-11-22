package proxy

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

const (
	// xray 进程在没有实例引用后保留这么久, 便于测速等短请求复用
	xrayIdleTTL     = 45 * time.Second
	xrayIdleCheckIn = 15 * time.Second
)

// XrayManager 管理 Xray 转换进程。Config / AppRoot / DataRoot / OnBridgeDied 来自内嵌的 bridgePool。
type XrayManager struct {
	*bridgePool
	stop     chan struct{}
	stopOnce sync.Once
}

// NewXrayManager 创建 Xray 管理器并启动空闲回收。
func NewXrayManager(cfg *config.Config, appRoot, dataRoot string) *XrayManager {
	m := &XrayManager{
		bridgePool: newBridgePool(engine{
			name: "Xray", exe: "xray.exe", envVar: "XRAY_BINARY_PATH", workSubdir: "_xray",
			configuredPath: func(c *config.Config) string { return c.Browser.XrayBinaryPath },
			writeConfig:    writeXrayConfig,
		}, cfg, appRoot, dataRoot),
		stop: make(chan struct{}),
	}
	go func() {
		t := time.NewTicker(xrayIdleCheckIn)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				m.recycleIdle(xrayIdleTTL)
			case <-m.stop:
				return
			}
		}
	}()
	return m
}

// EnsureBridge 用于测速、查出口 IP 等短请求: 返回可用的代理地址, 不占引用。
// 不需要转换的普通代理原样返回。
func (m *XrayManager) EnsureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyID string) (string, error) {
	addr, _, err := m.prepare(proxyConfig, proxies, proxyID, false)
	return addr, err
}

// AcquireBridge 用于浏览器实例: 返回代理地址与引用 key, 实例退出时用 ReleaseBridge 归还。
func (m *XrayManager) AcquireBridge(proxyConfig string, proxies []config.BrowserProxy, proxyID string) (string, string, error) {
	return m.prepare(proxyConfig, proxies, proxyID, true)
}

// ReleaseBridge 归还 AcquireBridge 拿到的引用; 进程在空闲超时后被回收。
func (m *XrayManager) ReleaseBridge(key string) { m.release(key) }

// StopAll 结束全部 Xray 进程并停止空闲回收。
func (m *XrayManager) StopAll() {
	m.stopOnce.Do(func() { close(m.stop) })
	m.bridgePool.StopAll()
}

func (m *XrayManager) prepare(proxyConfig string, proxies []config.BrowserProxy, proxyID string, pin bool) (string, string, error) {
	p, _ := lookupProxy(proxyConfig, proxies, proxyID)
	src := normalizeNodeScheme(p.ProxyConfig)
	if src == "" {
		return "", "", errNoNode
	}
	plain, outbound, err := ParseProxyNode(src)
	if err != nil {
		logger.New("Xray").Error("节点解析失败", logger.F("error", err))
		return "", "", err
	}
	if plain != "" {
		auth, ok := authBridgeOutbound(plain)
		if !ok {
			return plain, "", nil
		}
		outbound = auth
	}
	key := nodeKey(src, p.DnsServers)
	addr, err := m.obtain(key, outbound, p.DnsServers, pin)
	if err != nil {
		return "", "", err
	}
	if !pin {
		key = ""
	}
	return addr, key, nil
}

func writeXrayConfig(dir string, outbound map[string]interface{}, port int, dns string) (string, string, error) {
	logFile := filepath.Join(dir, "xray-error.log")
	cfg := map[string]interface{}{
		"log": map[string]interface{}{"loglevel": "info", "error": logFile},
		"inbounds": []interface{}{map[string]interface{}{
			"tag": "socks-in", "listen": "127.0.0.1", "port": port, "protocol": "socks",
			"settings": map[string]interface{}{"udp": true},
			"sniffing": map[string]interface{}{"enabled": false},
		}},
		"outbounds": []interface{}{
			outbound,
			map[string]interface{}{"protocol": "direct", "tag": "direct"},
			map[string]interface{}{"protocol": "blackhole", "tag": "block"},
		},
		"routing": map[string]interface{}{"rules": []interface{}{map[string]interface{}{
			"type": "field", "inboundTag": []string{"socks-in"}, "outboundTag": outboundTag,
		}}},
	}
	if d := xrayDNS(dns); d != nil {
		cfg["dns"] = d
	}
	cfgFile := filepath.Join(dir, "xray-config.json")
	return cfgFile, logFile, writeJSON(cfgFile, cfg)
}

// SingBoxManager 管理 sing-box 转换进程 (hysteria2 / tuic)。
// 实例不归还引用, 所以这里不做空闲回收, 进程活到 StopAll。
type SingBoxManager struct {
	*bridgePool
}

// NewSingBoxManager 创建 sing-box 管理器。
func NewSingBoxManager(cfg *config.Config, appRoot, dataRoot string) *SingBoxManager {
	return &SingBoxManager{bridgePool: newBridgePool(engine{
		name: "SingBox", exe: "sing-box.exe", envVar: "SINGBOX_BINARY_PATH", workSubdir: "_singbox",
		configuredPath: func(c *config.Config) string { return c.Browser.SingBoxBinaryPath },
		writeConfig:    writeSingBoxConfig,
	}, cfg, appRoot, dataRoot)}
}

// EnsureBridge 返回节点对应的本机 socks5 地址。
func (m *SingBoxManager) EnsureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyID string) (string, error) {
	p, _ := lookupProxy(proxyConfig, proxies, proxyID)
	src := normalizeNodeScheme(p.ProxyConfig)
	if strings.TrimSpace(src) == "" {
		return "", errNoNode
	}
	outbound, err := BuildSingBoxOutbound(src)
	if err != nil {
		logger.New("SingBox").Error("节点解析失败", logger.F("error", err))
		return "", err
	}
	return m.obtain(nodeKey(src), outbound, "", false)
}

func writeSingBoxConfig(dir string, outbound map[string]interface{}, port int, _ string) (string, string, error) {
	logFile := filepath.Join(dir, "singbox.log")
	cfg := map[string]interface{}{
		"log": map[string]interface{}{"level": "info", "output": logFile, "timestamp": true},
		"inbounds": []interface{}{map[string]interface{}{
			"type": "socks", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": port,
		}},
		"outbounds": []interface{}{outbound, map[string]interface{}{"type": "direct", "tag": "direct"}},
		"route": map[string]interface{}{"rules": []interface{}{map[string]interface{}{
			"inbound": []string{"socks-in"}, "outbound": outboundTag,
		}}},
	}
	cfgFile := filepath.Join(dir, "singbox-config.json")
	return cfgFile, logFile, writeJSON(cfgFile, cfg)
}
