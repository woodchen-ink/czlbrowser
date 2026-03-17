package proxy

import (
	"czl-browser/backend/internal/config"
	"czl-browser/backend/internal/logger"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

// SingBoxBridge sing-box 桥接进程
type SingBoxBridge struct {
	NodeKey   string
	Port      int
	Cmd       *exec.Cmd
	Pid       int
	Running   bool
	LastError string
}

// SingBoxManager sing-box 桥接管理器
type SingBoxManager struct {
	Config       *config.Config
	AppRoot      string // 应用根目录，所有相对路径基于此解析
	Bridges      map[string]*SingBoxBridge
	OnBridgeDied func(key string, err error)
}

// NewSingBoxManager 创建 sing-box 管理器
func NewSingBoxManager(cfg *config.Config, appRoot string) *SingBoxManager {
	return &SingBoxManager{
		Config:  cfg,
		AppRoot: appRoot,
		Bridges: make(map[string]*SingBoxBridge),
	}
}

// EnsureBridge 确保 sing-box 桥接进程运行，返回 socks5://127.0.0.1:port
func (m *SingBoxManager) EnsureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	log := logger.New("SingBox")
	src := strings.TrimSpace(proxyConfig)
	if proxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, proxyId) {
				src = strings.TrimSpace(item.ProxyConfig)
				break
			}
		}
	}
	if src == "" {
		return "", fmt.Errorf("未找到代理节点")
	}

	src = normalizeNodeScheme(src)
	outbound, err := BuildSingBoxOutbound(src)
	if err != nil {
		log.Error("节点解析失败", logger.F("error", err))
		return "", err
	}

	key := computeNodeKey(src)

	// 复用已有桥接
	if bridge, ok := m.Bridges[key]; ok && bridge != nil && bridge.Running {
		alive := bridge.Cmd != nil && bridge.Cmd.Process != nil && bridge.Cmd.ProcessState == nil
		if alive {
			if err := waitPortReady("127.0.0.1", bridge.Port, 800*time.Millisecond); err == nil {
				log.Info("复用 sing-box 桥接", logger.F("key", key[:8]), logger.F("port", bridge.Port))
				return fmt.Sprintf("socks5://127.0.0.1:%d", bridge.Port), nil
			}
		}
		log.Info("sing-box 桥接已失效，重新启动", logger.F("key", key[:8]))
		if bridge.Cmd != nil && bridge.Cmd.Process != nil {
			_ = bridge.Cmd.Process.Kill()
		}
		bridge.Running = false
		delete(m.Bridges, key)
	}

	binaryPath, err := m.resolveBinary()
	if err != nil {
		log.Error("sing-box 不可用", logger.F("error", err), logger.F("appRoot", m.AppRoot))
		return "", err
	}
	log.Debug("sing-box binary", logger.F("path", binaryPath))

	const maxRetries = 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		port, err := nextAvailablePort()
		if err != nil {
			lastErr = err
			continue
		}

		cfgPath, err := m.buildConfig(key, outbound, port)
		if err != nil {
			return "", fmt.Errorf("sing-box 配置生成失败: %w", err)
		}

		cmd := exec.Command(binaryPath, "run", "-c", cfgPath)
		hideWindow(cmd)
		cmd.Dir = filepath.Dir(cfgPath)
		stderrPath := filepath.Join(filepath.Dir(cfgPath), "singbox-stderr.log")
		stderrFile, _ := os.Create(stderrPath)
		if stderrFile != nil {
			cmd.Stderr = stderrFile
		}

		if err := cmd.Start(); err != nil {
			if stderrFile != nil {
				stderrFile.Close()
			}
			log.Error("sing-box 启动失败", logger.F("error", err), logger.F("attempt", attempt))
			lastErr = err
			continue
		}

		bridge := &SingBoxBridge{
			NodeKey: key,
			Port:    port,
			Cmd:     cmd,
			Pid:     cmd.Process.Pid,
			Running: true,
		}
		m.Bridges[key] = bridge
		log.Info("sing-box 启动", logger.F("key", key[:8]), logger.F("pid", bridge.Pid), logger.F("port", port))

		if err := waitPortReady("127.0.0.1", port, 10*time.Second); err != nil {
			if stderrFile != nil {
				stderrFile.Close()
			}
			if content, readErr := os.ReadFile(stderrPath); readErr == nil && len(content) > 0 {
				log.Error("sing-box stderr", logger.F("output", string(content)))
			}
			_ = cmd.Process.Kill()
			bridge.Running = false
			bridge.LastError = err.Error()
			delete(m.Bridges, key)
			log.Error("sing-box 端口不可用，重试", logger.F("error", err), logger.F("attempt", attempt))
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if stderrFile != nil {
			stderrFile.Close()
		}

		go func(b *SingBoxBridge, nodeKey string) {
			_ = b.Cmd.Wait()
			b.Running = false
			if m.OnBridgeDied != nil {
				m.OnBridgeDied(nodeKey, fmt.Errorf("sing-box 桥接进程意外退出"))
			}
		}(bridge, key)

		return fmt.Sprintf("socks5://127.0.0.1:%d", port), nil
	}

	return "", fmt.Errorf("sing-box 启动失败（已重试 %d 次）: %w", maxRetries, lastErr)
}

// StopAll 关闭所有 sing-box 桥接进程
func (m *SingBoxManager) StopAll() {
	for key, bridge := range m.Bridges {
		if bridge != nil && bridge.Cmd != nil && bridge.Cmd.Process != nil {
			_ = bridge.Cmd.Process.Kill()
		}
		delete(m.Bridges, key)
	}
}

func (m *SingBoxManager) resolveBinary() (string, error) {
	configPath := strings.TrimSpace(m.Config.Browser.SingBoxBinaryPath)
	if configPath != "" {
		resolved := resolveEnvPath(configPath, m.AppRoot)
		if resolved != "" {
			if _, err := os.Stat(resolved); err == nil {
				return resolved, nil
			}
		}
	}
	if env := strings.TrimSpace(os.Getenv("SINGBOX_BINARY_PATH")); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
	}
	// 优先基于 appRoot 查找 bin/sing-box.exe
	if m.AppRoot != "" {
		candidate := filepath.Join(m.AppRoot, "bin", "sing-box.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	// 兜底：exe 目录
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "bin", "sing-box.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("sing-box"); err == nil {
		return path, nil
	}
	if goruntime.GOOS == "windows" {
		if path, err := exec.LookPath("sing-box.exe"); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("未找到 sing-box.exe。请将 sing-box.exe 放到 bin/ 目录，或在配置中设置 SingBoxBinaryPath")
}

func (m *SingBoxManager) buildConfig(key string, outbound map[string]interface{}, port int) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"level":     "info",
			"output":    filepath.Join(baseDir, "singbox.log"),
			"timestamp": true,
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"type":        "socks",
				"tag":         "socks-in",
				"listen":      "127.0.0.1",
				"listen_port": port,
			},
		},
		"outbounds": []interface{}{
			outbound,
			map[string]interface{}{
				"type": "direct",
				"tag":  "direct",
			},
		},
		"route": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"inbound":  []string{"socks-in"},
					"outbound": "proxy-out",
				},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}

	cfgPath := filepath.Join(baseDir, "singbox-config.json")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func (m *SingBoxManager) resolveWorkdir(key string) string {
	root := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	if !filepath.IsAbs(root) {
		if m.AppRoot != "" {
			root = filepath.Join(m.AppRoot, root)
		} else if exePath, err := os.Executable(); err == nil {
			root = filepath.Join(filepath.Dir(exePath), root)
		}
	}
	return filepath.Join(root, "_singbox", key)
}
