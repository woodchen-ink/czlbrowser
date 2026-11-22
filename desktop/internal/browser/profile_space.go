package browser

import (
	"strings"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// CurrentSpace 返回当前数据空间, 列表与新建都在它上面。
func (m *Manager) CurrentSpace() string {
	if m.Spaces == nil {
		return space.LocalSpace
	}
	return m.Spaces.Current()
}

// profileSpace 取实例所属空间, 旧数据 (未带空间) 视为本机空间。
func profileSpace(p *Profile) string {
	if p.Space == "" {
		return space.LocalSpace
	}
	return p.Space
}

// writableSpace 返回当前空间; 只读时返回 40905。
func (m *Manager) writableSpace() (string, error) {
	if m.Spaces == nil {
		return space.LocalSpace, nil
	}
	return m.Spaces.Writable()
}

// checkWritable 拒绝改写当前只读空间里的实例。切空间前已打开的别的空间的实例不拦, 服务端会再判一次。
func (m *Manager) checkWritable(spaceID string) error {
	if m.Spaces == nil {
		return nil
	}
	cur, access := m.Spaces.CurrentAccess()
	if spaceID == "" {
		spaceID = space.LocalSpace
	}
	if cur == spaceID && access.ReadOnly {
		return errcode.ErrSyncReadOnly
	}
	return nil
}

// persist 保存一个实例; 未注入数据库时只留在内存 (测试)。调用方持有 m.Mutex。
func (m *Manager) persist(p *Profile) error {
	if m.ProfileDAO == nil {
		return nil
	}
	return m.ProfileDAO.Upsert(p)
}

// ReloadProfiles 从数据库重新载入实例配置 (同步拉到别处的修改后调用): 新增的加上、删掉的去掉 (运行中的保留到停止),
// 已有的只换配置字段, 运行态 (进程、端口、错误) 不动。
func (m *Manager) ReloadProfiles() error {
	if m.ProfileDAO == nil {
		return nil
	}
	list, err := m.ProfileDAO.List()
	if err != nil {
		return err
	}
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if m.Profiles == nil {
		m.Profiles = make(map[string]*Profile)
	}
	seen := make(map[string]bool, len(list))
	for _, fresh := range list {
		seen[fresh.ProfileId] = true
		cur, ok := m.Profiles[fresh.ProfileId]
		if !ok {
			m.Profiles[fresh.ProfileId] = fresh
			continue
		}
		cur.Space = fresh.Space
		cur.ProfileName = fresh.ProfileName
		cur.UserDataDir = fresh.UserDataDir
		cur.CoreId = fresh.CoreId
		cur.FingerprintArgs = fresh.FingerprintArgs
		cur.ProxyId = fresh.ProxyId
		cur.ProxyConfig = fresh.ProxyConfig
		cur.LaunchArgs = fresh.LaunchArgs
		cur.Tags = fresh.Tags
		cur.Keywords = fresh.Keywords
		cur.GroupId = fresh.GroupId
		cur.CreatedAt = fresh.CreatedAt
		cur.UpdatedAt = fresh.UpdatedAt
	}
	for id, p := range m.Profiles {
		if !seen[id] && !p.Running {
			delete(m.Profiles, id)
		}
	}
	return nil
}

// MoveProfilesToGroup 把当前空间的实例挂到分组 (空串为未分组), 内存与数据库一起改。
func (m *Manager) MoveProfilesToGroup(profileIDs []string, groupID string) error {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	groupID = strings.TrimSpace(groupID)
	now := time.Now().Format(time.RFC3339)
	for _, id := range profileIDs {
		p, ok := m.Profiles[id]
		if !ok || p.GroupId == groupID {
			continue
		}
		if err := m.checkWritable(p.Space); err != nil {
			return err
		}
		old := p.GroupId
		p.GroupId, p.UpdatedAt = groupID, now
		if err := m.persist(p); err != nil {
			p.GroupId = old
			return err
		}
	}
	return nil
}

// ProfileIDsInGroup 返回当前空间里挂在分组下的实例。
func (m *Manager) ProfileIDsInGroup(groupID string) []string {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	cur := m.CurrentSpace()
	var out []string
	for id, p := range m.Profiles {
		if p.GroupId == groupID && profileSpace(p) == cur {
			out = append(out, id)
		}
	}
	return out
}
