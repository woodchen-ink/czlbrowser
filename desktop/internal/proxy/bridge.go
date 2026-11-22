package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

const (
	launchAttempts = 3
	readyTimeout   = 10 * time.Second
	reuseProbe     = 800 * time.Millisecond
)

// engine 描述一种转换程序 (xray / sing-box) 的差异部分。
type engine struct {
	name       string // 日志组件名与报错用
	exe        string // bin/ 下的文件名
	envVar     string // 覆盖程序路径的环境变量
	workSubdir string // data/ 下的工作目录名
	// configuredPath 从配置里取用户指定的程序路径
	configuredPath func(*config.Config) string
	// writeConfig 在 dir 下写好运行配置, 返回配置文件路径与日志文件路径
	writeConfig func(dir string, outbound map[string]interface{}, port int, dns string) (cfgFile, logFile string, err error)
}

// bridge 是一个正在运行的转换进程, 在本机 127.0.0.1:port 提供无认证 socks5。
type bridge struct {
	key      string
	port     int
	cmd      *exec.Cmd
	exited   chan struct{}
	refs     int
	lastUsed time.Time
	stopping bool
}

func (b *bridge) socksURL() string { return fmt.Sprintf("socks5://127.0.0.1:%d", b.port) }

func (b *bridge) running() bool {
	select {
	case <-b.exited:
		return false
	default:
		return true
	}
}

// bridgePool 按节点复用转换进程。同一节点 (含 DNS 设置) 只起一个进程。
type bridgePool struct {
	Config       *config.Config
	AppRoot      string // 程序目录, 用于找 bin/ 下的可执行文件
	DataRoot     string // 可写数据根, 工作目录在 <DataRoot>/<数据目录>/<workSubdir>/<key>
	OnBridgeDied func(key string, err error)

	eng   engine
	mu    sync.Mutex
	live  map[string]*bridge
	locks map[string]*sync.Mutex // 同一节点的启动串行化
}

func newBridgePool(eng engine, cfg *config.Config, appRoot, dataRoot string) *bridgePool {
	return &bridgePool{
		Config: cfg, AppRoot: appRoot, DataRoot: dataRoot, eng: eng,
		live: map[string]*bridge{}, locks: map[string]*sync.Mutex{},
	}
}

func nodeKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.Join(parts, "\x00"))))
	return hex.EncodeToString(sum[:])
}

func (p *bridgePool) keyLock(key string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	l, ok := p.locks[key]
	if !ok {
		l = &sync.Mutex{}
		p.locks[key] = l
	}
	return l
}

// obtain 返回节点对应的 socks5 地址, 必要时启动新进程。pin 为 true 时增加引用计数,
// 由调用方用 release 归还 (长期占用, 如浏览器实例)。
func (p *bridgePool) obtain(key string, outbound map[string]interface{}, dns string, pin bool) (string, error) {
	l := p.keyLock(key)
	l.Lock()
	defer l.Unlock()

	if addr, ok := p.reuse(key, pin); ok {
		return addr, nil
	}
	b, err := p.launch(key, outbound, dns)
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	if pin {
		b.refs = 1
	}
	p.live[key] = b
	p.mu.Unlock()
	go p.watch(b)
	return b.socksURL(), nil
}

// reuse 复用仍在运行且端口可连的进程; 已失效的进程顺手清理。
func (p *bridgePool) reuse(key string, pin bool) (string, bool) {
	p.mu.Lock()
	b := p.live[key]
	p.mu.Unlock()
	if b == nil {
		return "", false
	}
	if b.running() && waitPort(b.port, reuseProbe) == nil {
		p.mu.Lock()
		if pin {
			b.refs++
		}
		b.lastUsed = time.Now()
		p.mu.Unlock()
		logger.New(p.eng.name).Info("复用桥接进程", logger.F("key", short(key)), logger.F("port", b.port))
		return b.socksURL(), true
	}
	p.retire(key, b)
	return "", false
}

// retire 从表里移除并结束进程 (主动结束, 不触发 OnBridgeDied)。
func (p *bridgePool) retire(key string, b *bridge) {
	p.mu.Lock()
	if p.live[key] == b {
		delete(p.live, key)
	}
	b.stopping = true
	p.mu.Unlock()
	if b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
}

func (p *bridgePool) launch(key string, outbound map[string]interface{}, dns string) (*bridge, error) {
	log := logger.New(p.eng.name)
	exe, err := p.findExecutable()
	if err != nil {
		log.Error("找不到转换程序", logger.F("error", err))
		return nil, err
	}
	dir := p.workdir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= launchAttempts; attempt++ {
		port, err := freePort()
		if err != nil {
			lastErr = err
			continue
		}
		cfgFile, logFile, err := p.eng.writeConfig(dir, outbound, port, dns)
		if err != nil {
			return nil, fmt.Errorf("%s 配置生成失败: %w", p.eng.name, err)
		}
		stderrFile := filepath.Join(dir, p.eng.exe+".stderr.log")
		b, err := startProcess(exe, cfgFile, stderrFile)
		if err != nil {
			log.Error("启动失败", logger.F("error", err), logger.F("attempt", attempt))
			lastErr = err
			continue
		}
		b.key, b.port, b.lastUsed = key, port, time.Now()
		log.Info("桥接进程已启动", logger.F("key", short(key)), logger.F("pid", b.cmd.Process.Pid), logger.F("port", port))

		if err := waitPort(port, readyTimeout); err != nil {
			_ = b.cmd.Process.Kill()
			<-b.exited
			log.Error("桥接端口未就绪", logger.F("port", port), logger.F("attempt", attempt),
				logger.F("output", firstNonEmptyFile(stderrFile, logFile)))
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		return b, nil
	}
	return nil, fmt.Errorf("%s 启动失败（已重试 %d 次）: %w", p.eng.name, launchAttempts, lastErr)
}

func startProcess(exe, cfgFile, stderrFile string) (*bridge, error) {
	cmd := exec.Command(exe, "run", "-c", cfgFile)
	cmd.Dir = filepath.Dir(cfgFile)
	hideWindow(cmd)
	errOut, _ := os.Create(stderrFile)
	if errOut != nil {
		cmd.Stderr = errOut
	}
	if err := cmd.Start(); err != nil {
		if errOut != nil {
			_ = errOut.Close()
		}
		return nil, err
	}
	b := &bridge{cmd: cmd, exited: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		if errOut != nil {
			_ = errOut.Close()
		}
		close(b.exited)
	}()
	return b, nil
}

// watch 等进程退出; 不是我们主动结束的就通知上层 (正在用它的实例会失去网络)。
func (p *bridgePool) watch(b *bridge) {
	<-b.exited
	p.mu.Lock()
	if p.live[b.key] == b {
		delete(p.live, b.key)
	}
	expected := b.stopping
	p.mu.Unlock()
	if !expected && p.OnBridgeDied != nil {
		p.OnBridgeDied(b.key, fmt.Errorf("%s 桥接进程意外退出", p.eng.name))
	}
}

// release 归还 obtain(pin=true) 拿到的引用。
func (p *bridgePool) release(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if b := p.live[strings.TrimSpace(key)]; b != nil {
		if b.refs > 0 {
			b.refs--
		}
		b.lastUsed = time.Now()
	}
}

// recycleIdle 结束没有引用且空闲超过 idle 的进程。
func (p *bridgePool) recycleIdle(idle time.Duration) {
	var stale []*bridge
	p.mu.Lock()
	for key, b := range p.live {
		if b.refs == 0 && time.Since(b.lastUsed) >= idle {
			stale = append(stale, b)
			delete(p.live, key)
			b.stopping = true
		}
	}
	p.mu.Unlock()
	for _, b := range stale {
		logger.New(p.eng.name).Info("回收空闲桥接进程", logger.F("key", short(b.key)))
		_ = b.cmd.Process.Kill()
	}
}

// StopAll 结束全部转换进程。
func (p *bridgePool) StopAll() {
	p.mu.Lock()
	all := make([]*bridge, 0, len(p.live))
	for key, b := range p.live {
		b.stopping = true
		all = append(all, b)
		delete(p.live, key)
	}
	p.mu.Unlock()
	for _, b := range all {
		_ = b.cmd.Process.Kill()
	}
}

// findExecutable 依次查找: 配置指定路径、环境变量、程序目录 bin/、当前 exe 旁 bin/、PATH。
func (p *bridgePool) findExecutable() (string, error) {
	var candidates []string
	if p.Config != nil && p.eng.configuredPath != nil {
		if c := strings.TrimSpace(p.eng.configuredPath(p.Config)); c != "" {
			if !filepath.IsAbs(c) && p.AppRoot != "" {
				candidates = append(candidates, filepath.Join(p.AppRoot, c))
			}
			candidates = append(candidates, c)
		}
	}
	if env := strings.TrimSpace(os.Getenv(p.eng.envVar)); env != "" {
		candidates = append(candidates, env)
	}
	if p.AppRoot != "" {
		candidates = append(candidates, filepath.Join(p.AppRoot, "bin", p.eng.exe))
	}
	if self, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), "bin", p.eng.exe))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	if found, err := exec.LookPath(p.eng.exe); err == nil {
		return found, nil
	}
	return "", fmt.Errorf("未找到 %s, 请确认程序目录下的 bin\\%s 存在", p.eng.exe, p.eng.exe)
}

func (p *bridgePool) workdir(key string) string {
	root := "data"
	if p.Config != nil && strings.TrimSpace(p.Config.Browser.UserDataRoot) != "" {
		root = strings.TrimSpace(p.Config.Browser.UserDataRoot)
	}
	if !filepath.IsAbs(root) && p.DataRoot != "" {
		root = filepath.Join(p.DataRoot, root)
	}
	return filepath.Join(root, p.eng.workSubdir, key)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// freePort 让系统分配一个本机空闲端口。拿到到使用之间有极小的被占用窗口, 由 launch 重试兜底。
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("无法分配本机端口: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitPort 在超时前反复尝试连接本机端口。
func waitPort(port int, timeout time.Duration) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("端口 %d 不可用", port)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func firstNonEmptyFile(paths ...string) string {
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) > 0 {
			return string(b)
		}
	}
	return ""
}

func short(key string) string {
	if len(key) > 8 {
		return key[:8]
	}
	return key
}

var errNoNode = errors.New("未找到代理节点")
