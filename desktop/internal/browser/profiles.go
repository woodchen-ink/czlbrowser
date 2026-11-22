package browser

import (
	"errors"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

var errProfileNotFound = errors.New("profile not found")

func nowStamp() string { return time.Now().Format(time.RFC3339) }

func cloneList(s []string) []string { return append([]string{}, s...) }

// InitData 准备内存结构; 内存里还没有实例时从数据库载入。可以重复调用。
func (m *Manager) InitData() {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if m.Profiles == nil {
		m.Profiles = map[string]*Profile{}
	}
	if m.BrowserProcesses == nil {
		m.BrowserProcesses = map[string]*exec.Cmd{}
	}
	m.migrateEnvironments()
	if len(m.Profiles) > 0 || m.ProfileDAO == nil {
		return
	}
	list, err := m.ProfileDAO.List()
	if err != nil {
		logger.New("Browser").Error("从数据库载入实例失败", logger.F("error", err))
		return
	}
	for _, p := range list {
		m.Profiles[p.ProfileId] = p
	}
	logger.New("Browser").Info("实例已载入", logger.F("count", len(list)))
}

// SaveProfiles 把内存里的全部实例写回数据库。
func (m *Manager) SaveProfiles() error {
	if m.ProfileDAO == nil {
		return nil
	}
	for _, p := range m.Profiles {
		if err := m.ProfileDAO.Upsert(p); err != nil {
			logger.New("Browser").Error("保存实例失败", logger.F("profile_id", p.ProfileId), logger.F("error", err))
			return err
		}
	}
	return nil
}

// visible 返回当前空间的实例 (调用方持锁)。
func (m *Manager) visible() []*Profile {
	cur := m.CurrentSpace()
	out := make([]*Profile, 0, len(m.Profiles))
	for _, p := range m.Profiles {
		if profileSpace(p) == cur {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProfileId < out[j].ProfileId })
	return out
}

// List 返回当前空间的实例副本, 按 id 排序。
func (m *Manager) List() []Profile {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	src := m.visible()
	out := make([]Profile, len(src))
	for i, p := range src {
		out[i] = *p
	}
	return out
}

func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// GetAllTags 返回当前空间实例用到的全部标签, 去重排序。
func (m *Manager) GetAllTags() []string {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	set := map[string]bool{}
	for _, p := range m.visible() {
		for _, t := range p.Tags {
			if t = strings.TrimSpace(t); t != "" {
				set[t] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// bindProxy 按代理池 id 取最新地址; id 为空时用 fallback 作为自定义代理。
// 找不到 id 时返回 ok=false, 地址保持 fallback。
func (m *Manager) bindProxy(id, fallback string) (string, string, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", strings.TrimSpace(fallback), true
	}
	if cfg, ok := m.proxyConfigByID(id); ok {
		return id, cfg, true
	}
	return id, strings.TrimSpace(fallback), false
}

// Create 在当前空间新建实例。没选代理时套用「新实例默认代理」; 内核留空表示跟随默认。
func (m *Manager) Create(in ProfileInput) (*Profile, error) {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	spaceID, err := m.writableSpace()
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	p := &Profile{
		ProfileId:       id,
		Space:           spaceID,
		ProfileName:     in.ProfileName,
		UserDataDir:     strings.TrimSpace(in.UserDataDir),
		CoreId:          strings.TrimSpace(in.CoreId),
		FingerprintArgs: in.FingerprintArgs,
		LaunchArgs:      in.LaunchArgs,
		Tags:            in.Tags,
		Keywords:        cloneList(in.Keywords),
		GroupId:         strings.TrimSpace(in.GroupId),
	}
	if p.UserDataDir == "" {
		p.UserDataDir = id
	}
	var ok bool
	p.ProxyId, p.ProxyConfig, ok = m.bindProxy(in.ProxyId, in.ProxyConfig)
	if !ok {
		logger.New("Browser").Error("代理池里没有该代理", logger.F("profile_id", id), logger.F("proxy_id", p.ProxyId))
	}
	if p.ProxyId == "" && p.ProxyConfig == "" {
		p.ProxyId, p.ProxyConfig = m.resolveDefaultProxy()
	}
	p.CreatedAt = nowStamp()
	p.UpdatedAt = p.CreatedAt
	if err := m.persist(p); err != nil {
		return nil, err
	}
	m.Profiles[id] = p
	logger.New("Browser").Info("已新建实例", logger.F("profile_id", id), logger.F("name", p.ProfileName))
	return p, nil
}

// Update 修改实例配置; 保存失败时内存恢复原样。
func (m *Manager) Update(id string, in ProfileInput) (*Profile, error) {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	p, ok := m.Profiles[id]
	if !ok {
		return nil, errProfileNotFound
	}
	if err := m.checkWritable(p.Space); err != nil {
		return nil, err
	}
	before := *p
	p.ProfileName = in.ProfileName
	p.UserDataDir = in.UserDataDir
	p.CoreId = in.CoreId
	p.FingerprintArgs = in.FingerprintArgs
	p.LaunchArgs = in.LaunchArgs
	p.Tags = in.Tags
	p.Keywords = cloneList(in.Keywords)
	p.GroupId = strings.TrimSpace(in.GroupId)
	var found bool
	p.ProxyId, p.ProxyConfig, found = m.bindProxy(in.ProxyId, in.ProxyConfig)
	if !found {
		// 代理池里已经没有这个代理: 不保留旧地址, 启动时会提示重新选择
		p.ProxyConfig = ""
		logger.New("Browser").Error("代理池里没有该代理", logger.F("profile_id", id), logger.F("proxy_id", p.ProxyId))
	}
	p.UpdatedAt = nowStamp()
	if err := m.persist(p); err != nil {
		*p = before
		return nil, err
	}
	return p, nil
}

// Delete 删除实例配置 (用户数据目录由调用方处理)。
func (m *Manager) Delete(id string) error {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	p, ok := m.Profiles[id]
	if !ok {
		return errProfileNotFound
	}
	if err := m.checkWritable(p.Space); err != nil {
		return err
	}
	if m.ProfileDAO != nil {
		if err := m.ProfileDAO.Delete(id); err != nil {
			return err
		}
	}
	delete(m.Profiles, id)
	return nil
}

// ApplyDefaults 启动前补齐实例缺省项, 并把代理地址同步成代理池里的最新值。
// 返回代理地址是否有变化。
func (m *Manager) ApplyDefaults(p *Profile) bool {
	if len(p.FingerprintArgs) == 0 {
		p.FingerprintArgs = cloneList(m.Config.Browser.DefaultFingerprintArgs)
	}
	if len(p.LaunchArgs) == 0 {
		p.LaunchArgs = cloneList(m.Config.Browser.DefaultLaunchArgs)
	}
	if strings.TrimSpace(p.UserDataDir) == "" {
		p.UserDataDir = p.ProfileId
	}
	before := p.ProxyConfig
	if p.ProxyId != "" {
		if cfg, ok := m.proxyConfigByID(p.ProxyId); ok && cfg != "" {
			p.ProxyConfig = cfg
		} else if !ok {
			logger.New("Browser").Error("实例绑定的代理不在代理池里", logger.F("profile_id", p.ProfileId), logger.F("proxy_id", p.ProxyId))
		}
	} else if p.ProxyConfig == "" {
		p.ProxyId, p.ProxyConfig = m.resolveDefaultProxy()
	}
	return p.ProxyConfig != before
}

// Copy 复制实例: 配置照搬, 用户数据目录新建, 指纹用默认值 (等于换一个新指纹)。
func (m *Manager) Copy(id, newName string) (*Profile, error) {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	src, ok := m.Profiles[id]
	if !ok {
		return nil, errProfileNotFound
	}
	if err := m.checkWritable(src.Space); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(newName)
	if name == "" {
		name = src.ProfileName + " (副本)"
	}
	newID := uuid.NewString()
	p := &Profile{
		ProfileId:       newID,
		Space:           src.Space,
		ProfileName:     name,
		UserDataDir:     newID,
		CoreId:          src.CoreId,
		FingerprintArgs: cloneList(m.Config.Browser.DefaultFingerprintArgs),
		ProxyId:         src.ProxyId,
		ProxyConfig:     src.ProxyConfig,
		LaunchArgs:      cloneList(src.LaunchArgs),
		Tags:            cloneList(src.Tags),
		Keywords:        cloneList(src.Keywords),
		GroupId:         src.GroupId,
		CreatedAt:       nowStamp(),
	}
	p.UpdatedAt = p.CreatedAt
	if err := m.persist(p); err != nil {
		return nil, err
	}
	m.Profiles[newID] = p
	return p, nil
}

// resolveDefaultProxy 解析「新实例默认代理」: 存的是代理池 id (当前空间没有就不用);
// 含 :// 的旧值当作自定义代理地址。
func (m *Manager) resolveDefaultProxy() (string, string) {
	def := strings.TrimSpace(m.Config.Browser.DefaultProxy)
	switch {
	case def == "":
		return "", ""
	case strings.Contains(def, "://"):
		return "", def
	}
	if cfg, ok := m.proxyConfigByID(def); ok {
		return def, cfg
	}
	return "", ""
}

// ListByTag 返回带指定标签 (不区分大小写) 的实例; 标签为空时返回全部。
func (m *Manager) ListByTag(tag string) []Profile {
	all := m.List()
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return all
	}
	out := make([]Profile, 0)
	for _, p := range all {
		if containsFold(p.Tags, tag) {
			out = append(out, p)
		}
	}
	return out
}
