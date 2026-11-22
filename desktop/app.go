package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/auth"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/billing"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/cloudsync"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/database"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/orgs"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/proxy"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/syncstore"
)

// App 是绑定给前端的对象: 它的导出方法经 Wails 生成到 frontend/src/wailsjs。
// 生命周期 (startup / shutdown / beforeClose) 为小写方法, 不暴露给前端。
type App struct {
	ctx    context.Context
	config *config.Config
	db     *database.DB
	spaces *space.State // 当前数据空间: 本机 / 个人同步 / 团队

	browserMgr     *browser.Manager
	xrayMgr        *proxy.XrayManager
	singboxMgr     *proxy.SingBoxManager
	speedScheduler *browser.ProxySpeedScheduler
	authMgr        *auth.Manager
	syncStore      *syncstore.Store
	cloud          *cloudsync.Service
	orgs           *orgs.Client
	billing        *billing.Client
	billingCache   billingCache

	appRoot    string // 程序目录, 只读资源 (bin\xray.exe 等) 在这里
	dataRoot   string // 可写数据根: data / chrome / logs
	configPath string // config.yaml
	devMode    bool

	forceQuit        bool       // 为 true 时关闭窗口不再询问
	maintenanceMu    sync.Mutex // 备份导入导出等维护操作互斥
	bridgeMu         sync.Mutex
	xrayBridgeRefs   map[string]string // 实例 id -> 它占用的 Xray 桥接 key
	stopServicesOnce sync.Once
	finalizeOnce     sync.Once
}

// NewApp 创建应用; 开发模式下数据也放在程序目录, 正式运行由 SetLayout 指到安装根。
func NewApp(appRoot string) *App {
	root := strings.TrimSpace(appRoot)
	return &App{appRoot: root, dataRoot: root, xrayBridgeRefs: map[string]string{}}
}

// SetLayout 由 main 在启动前注入目录布局。
func (a *App) SetLayout(dev bool, dataRoot, configPath string) {
	a.devMode, a.dataRoot, a.configPath = dev, dataRoot, configPath
}

// resolveAppPath 把相对数据根的路径转成绝对路径。
func (a *App) resolveAppPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	base := a.dataRoot
	if base == "" {
		base = a.appRoot
	}
	if base == "" {
		base, _ = os.Getwd()
	}
	return filepath.Join(base, p)
}

// mainConfigPath 返回 config.yaml 的位置 (测试里未注入时取数据根下的 config.yaml)。
func (a *App) mainConfigPath() string {
	if p := strings.TrimSpace(a.configPath); p != "" {
		return p
	}
	return a.resolveAppPath("config.yaml")
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := LoadConfig(a.mainConfigPath())
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.config = cfg
	a.applyRuntimeConfig(cfg.Runtime)
	a.setupLogging()

	log := logger.New("App")
	log.Info("应用启动中", logger.F("version", AppVersion), logger.F("data", a.dataRoot))
	if err := os.MkdirAll(a.resolveAppPath("data"), 0o755); err != nil {
		log.Error("创建 data 目录失败", logger.F("error", err))
	}

	a.restoreLogin()
	if err := a.openDatabase(); err != nil {
		log.Error("初始化数据库失败", logger.F("error", err))
		runtime.LogFatal(ctx, fmt.Sprintf("初始化数据库失败: %v", err))
		return
	}
	a.xrayMgr = proxy.NewXrayManager(cfg, a.appRoot, a.dataRoot)
	a.singboxMgr = proxy.NewSingBoxManager(cfg, a.appRoot, a.dataRoot)
	a.xrayMgr.OnBridgeDied = func(key string, err error) {
		runtime.EventsEmit(a.ctx, "proxy:bridge:died", map[string]interface{}{"key": key[:8], "error": err.Error()})
	}

	// 迁移与默认数据只写本机空间, 之后再恢复上次的空间并开始同步
	a.migrateToSQLite()
	a.registerKernelDirs()
	a.browserMgr.InitData()
	a.initCloud()
	a.autoDetectCores()
	a.loadProxies()
	a.startSpeedScheduler()
	log.Info("应用启动完成")
}

func (a *App) setupLogging() {
	file := ""
	if a.config.Logging.FileEnabled {
		file = a.resolveAppPath(a.config.Logging.FilePath)
	}
	if err := logger.Setup(logger.Options{
		Level:      a.config.Logging.Level,
		File:       file,
		MaxSizeMB:  a.config.Logging.Rotation.MaxSizeMB,
		MaxBackups: a.config.Logging.Rotation.MaxBackups,
	}); err != nil {
		logger.New("App").Warn("日志文件不可用, 仅输出到控制台", logger.F("error", err))
	}
}

// restoreLogin 先从本机恢复登录 (不联网, 弱网也能用), 再在后台刷新令牌与用户信息。
func (a *App) restoreLogin() {
	a.authMgr = auth.NewManager(a.dataRoot)
	if !a.authMgr.TryRestoreSession() {
		return
	}
	logger.New("App").Info("已恢复登录")
	a.authMgr.RefreshSessionAsync(func(r auth.LoginResult) {
		if a.ctx == nil {
			return
		}
		if r.Success {
			a.startCloud() // 拿到用户信息后才知道用户 id
			runtime.EventsEmit(a.ctx, "auth:restored", r.State)
			return
		}
		a.stopCloud()
		runtime.EventsEmit(a.ctx, "auth:session:expired", r.Error)
	})
}

// openDatabase 打开并迁移数据库, 创建实例管理器并注入各 DAO。
func (a *App) openDatabase() error {
	db, err := database.NewDB(a.resolveAppPath(a.config.Database.SQLite.Path))
	if err != nil {
		return err
	}
	a.db = db
	if err := db.Migrate(); err != nil {
		logger.New("App").Error("数据库迁移失败", logger.F("error", err))
	}
	conn := db.GetConn()
	a.spaces = space.NewState()
	m := browser.NewManager(a.config, a.dataRoot, a.mainConfigPath())
	m.Spaces = a.spaces
	m.ProfileDAO = browser.NewSQLiteProfileDAO(conn, a.spaces)
	m.ProxyDAO = browser.NewSQLiteProxyDAO(conn, a.spaces)
	m.CoreDAO = browser.NewSQLiteCoreDAO(conn)
	m.BookmarkDAO = browser.NewSQLiteBookmarkDAO(conn, a.spaces)
	m.GroupDAO = browser.NewSQLiteGroupDAO(conn, a.spaces)
	a.browserMgr = m
	return nil
}

// startSpeedScheduler 每 5 分钟给代理池测一轮速, 同时最多 5 个。
func (a *App) startSpeedScheduler() {
	if a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return
	}
	a.speedScheduler = browser.NewProxySpeedScheduler(a.browserMgr.ProxyDAO, func(id string) (bool, int64, string) {
		r := proxy.SpeedTest(id, a.getLatestProxies(), nil)
		return r.Ok, r.LatencyMs, r.Error
	}, 5*time.Minute, 5)
	a.speedScheduler.Start()
}

// ReloadConfig 重新读取 config.yaml (手动改过配置文件后使用)。
func (a *App) ReloadConfig() error {
	cfg, err := LoadConfig(a.mainConfigPath())
	if err != nil {
		return fmt.Errorf("重载配置文件失败: %w", err)
	}
	a.config = cfg
	a.applyRuntimeConfig(cfg.Runtime)
	if a.browserMgr != nil {
		a.browserMgr.Config = cfg
		a.loadProxies()
	}
	if a.xrayMgr != nil {
		a.xrayMgr.Config = cfg
	}
	if a.singboxMgr != nil {
		a.singboxMgr.Config = cfg
	}
	return nil
}

// applyRuntimeConfig 设置 Go 运行时的 GC 比例与内存软上限。
func (a *App) applyRuntimeConfig(rc config.RuntimeConfig) {
	if rc.GCPercent > 0 {
		debug.SetGCPercent(rc.GCPercent)
	}
	if rc.MaxMemoryMB > 0 {
		debug.SetMemoryLimit(int64(rc.MaxMemoryMB) << 20)
	}
}

func (a *App) shutdown(context.Context) {
	logger.New("App").Info("应用正在关闭")
	if a.authMgr != nil {
		a.authMgr.Shutdown() // 结束登录回调监听, 否则会拖住退出
	}
	a.stopRuntimeServices()
	a.finalizeShutdown()
}

// ForceQuit 不再询问, 直接退出 (托盘菜单「退出」与在线更新使用)。
func (a *App) ForceQuit() {
	a.forceQuit = true
	a.stopRuntimeServices()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// ---- 实例占用的 Xray 桥接 ----

func (a *App) bindProfileXrayBridge(profileID, key string) {
	profileID, key = strings.TrimSpace(profileID), strings.TrimSpace(key)
	if profileID == "" || key == "" {
		return
	}
	a.bridgeMu.Lock()
	a.xrayBridgeRefs[profileID] = key
	a.bridgeMu.Unlock()
}

func (a *App) releaseProfileXrayBridge(profileID string) {
	a.bridgeMu.Lock()
	key := a.xrayBridgeRefs[strings.TrimSpace(profileID)]
	delete(a.xrayBridgeRefs, strings.TrimSpace(profileID))
	a.bridgeMu.Unlock()
	if key != "" && a.xrayMgr != nil {
		a.xrayMgr.ReleaseBridge(key)
	}
}

func (a *App) clearProfileXrayBridges() {
	a.bridgeMu.Lock()
	a.xrayBridgeRefs = map[string]string{}
	a.bridgeMu.Unlock()
}
