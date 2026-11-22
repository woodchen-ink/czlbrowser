package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// Manager 持有全部实例 (所有空间) 的内存副本与各类数据访问对象。
// 调用方读写 Profiles / BrowserProcesses 时须持有 Mutex。
type Manager struct {
	Config     *config.Config
	DataRoot   string // 可写数据根: 内核目录与实例数据目录的相对路径以它为基准
	ConfigPath string // config.yaml 的实际位置

	Mutex            sync.Mutex
	Profiles         map[string]*Profile
	BrowserProcesses map[string]*exec.Cmd

	// Spaces 是当前数据空间, 列表按它过滤、新建落在它上面; nil 时一律视为本机空间
	Spaces *space.State

	ProfileDAO  ProfileDAO
	ProxyDAO    ProxyDAO
	CoreDAO     CoreDAO
	BookmarkDAO BookmarkDAO
	GroupDAO    GroupDAO

	loaded bool
}

// NewManager 创建管理器; DAO 由调用方随后注入。
func NewManager(cfg *config.Config, dataRoot, configPath string) *Manager {
	return &Manager{
		Config:           cfg,
		DataRoot:         dataRoot,
		ConfigPath:       configPath,
		Profiles:         map[string]*Profile{},
		BrowserProcesses: map[string]*exec.Cmd{},
	}
}

// ResolveRelativePath 把相对数据根的路径转成绝对路径。
func (m *Manager) ResolveRelativePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	base := m.DataRoot
	if base == "" {
		base, _ = os.Getwd()
	}
	return filepath.Join(base, p)
}

func (m *Manager) configFile() string {
	if m.ConfigPath != "" {
		return filepath.Clean(m.ConfigPath)
	}
	return m.ResolveRelativePath("config.yaml")
}

// ResolveUserDataDir 返回实例的 Chromium 用户数据目录 (绝对路径)。
func (m *Manager) ResolveUserDataDir(p *Profile) string {
	dir := strings.TrimSpace(p.UserDataDir)
	if dir == "" {
		dir = p.ProfileId
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	root := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	return filepath.Join(m.ResolveRelativePath(root), dir)
}

// proxyConfigByID 在代理池 (数据库优先, 其次旧版配置) 里查代理地址。
func (m *Manager) proxyConfigByID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", false
	}
	var pool []Proxy
	if m.ProxyDAO != nil {
		if list, err := m.ProxyDAO.List(); err == nil {
			pool = list
		}
	}
	pool = append(pool, m.Config.Browser.Proxies...)
	for _, p := range pool {
		if strings.EqualFold(p.ProxyId, id) {
			return strings.TrimSpace(p.ProxyConfig), true
		}
	}
	return "", false
}

// migrateEnvironments 把早期版本的 environments 转成 cores 并写回配置文件。
func (m *Manager) migrateEnvironments() {
	b := &m.Config.Browser
	if len(b.Environments) == 0 || len(b.Cores) > 0 {
		return
	}
	for _, env := range b.Environments {
		b.Cores = append(b.Cores, Core{CoreId: env.CoreId, CoreName: env.CoreName, CorePath: env.CorePath, IsDefault: env.IsDefault})
	}
	b.Environments = nil
	b.ChromeBinaryPath, b.CoreRoot, b.DefaultCoreId, b.DefaultConnectorType = "", "", "", ""
	log := logger.New("Browser")
	if err := m.Config.Save(m.configFile()); err != nil {
		log.Error("旧版内核配置迁移后保存失败", logger.F("error", err))
		return
	}
	log.Info("旧版内核配置已迁移", logger.F("cores", len(b.Cores)))
}
