// Package space 管理数据空间: 当前空间、本人写权限、出站序号与「本地写入后通知同步」。
//
// 空间标识: LocalSpace (本机, 永不上传)、"me:<用户 id>" (个人同步)、"org:<组织 id>" (团队)。
// 实例、代理、分组、书签都按空间存; 同步空间里的写入标脏 (dirty = 出站序号), 删除留墓碑 (deleted = 1)。
// DAO 与同步层 (internal/cloudsync) 共用这里的状态, 不互相引用。
package space

import (
	"database/sql"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"strings"
	"sync"
	"sync/atomic"
)

// LocalSpace 是本机空间: 未开通同步时的唯一空间, 永不推送。
const LocalSpace = "local"

// 同步空间本地标识的前缀。
const (
	PersonalPrefix = "me:"
	OrgPrefix      = "org:"
)

// IsSynced 判断空间里的写入是否要标脏、删除是否留墓碑。
func IsSynced(space string) bool { return space != "" && space != LocalSpace }

// 同步集合名, 与 Connect 应用里登记的集合一一对应。集合名与文档 id 对服务端可见, 不放业务信息。
const (
	CollProfiles = "profiles"
	CollProxies  = "proxies"
	CollGroups   = "groups"
	CollSettings = "settings"
	CollStates   = "states"
)

// Access 是本人在当前空间能写什么。
type Access struct {
	// ReadOnly 整个空间只读 (只读成员、订阅到期、组织超席位、已被移出)
	ReadOnly bool `json:"readOnly"`
	// SettingsReadOnly 团队书签 (settings 集合) 只有 owner / admin 能改
	SettingsReadOnly bool `json:"settingsReadOnly"`
}

type current struct {
	space string
	Access
}

// State 是全局的空间状态, 并发安全。
type State struct {
	cur    atomic.Value // current
	onEdit atomic.Value // func(space, collection, id string)
	seqMu  sync.Mutex
}

// NewState 创建状态, 当前空间为本机。
func NewState() *State {
	s := &State{}
	s.cur.Store(current{space: LocalSpace})
	return s
}

func (s *State) load() current {
	if s == nil {
		return current{space: LocalSpace}
	}
	if v, ok := s.cur.Load().(current); ok && v.space != "" {
		return v
	}
	return current{space: LocalSpace}
}

// Current 返回当前空间。
func (s *State) Current() string { return s.load().space }

// CurrentAccess 返回当前空间与本人的写权限。
func (s *State) CurrentAccess() (string, Access) {
	c := s.load()
	return c.space, c.Access
}

func normalize(space string, a Access) current {
	space = strings.TrimSpace(space)
	if space == "" {
		space = LocalSpace
	}
	if !IsSynced(space) {
		a = Access{}
	}
	a.SettingsReadOnly = a.SettingsReadOnly || a.ReadOnly
	return current{space: space, Access: a}
}

// Set 切换当前空间并给定写权限。空串回到本机空间; 本机空间永远可写。
func (s *State) Set(space string, a Access) { s.cur.Store(normalize(space, a)) }

// SetAccessIf 只在当前空间仍是 space 时改写权限 (同步层后台调用, 不覆盖用户刚做的切换)。
func (s *State) SetAccessIf(space string, a Access) bool {
	old := s.load()
	if old.space != space {
		return false
	}
	return s.cur.CompareAndSwap(old, normalize(space, a))
}

// Writable 是当前空间本地写入的入口检查: 只读空间拒绝。返回要写入的空间。
func (s *State) Writable() (string, error) {
	c := s.load()
	if c.ReadOnly {
		return "", errReadOnly
	}
	return c.space, nil
}

// SettingsWritable 是团队书签的写入检查。
func (s *State) SettingsWritable() (string, error) {
	c := s.load()
	if c.ReadOnly {
		return "", errReadOnly
	}
	if c.SettingsReadOnly {
		return "", errSettingsAdminOnly
	}
	return c.space, nil
}

// OnLocalEdit 设置同步空间本地写入后的回调 (同步引擎据此尽快推送)。
func (s *State) OnLocalEdit(fn func(space, collection, id string)) { s.onEdit.Store(fn) }

// Edited 在本地写入提交后通知同步层。本机空间不通知。
func (s *State) Edited(space, collection, id string) {
	if s == nil || !IsSynced(space) {
		return
	}
	if fn, ok := s.onEdit.Load().(func(space, collection, id string)); ok && fn != nil {
		fn(space, collection, id)
	}
}

// NextSeq 在事务内取下一个出站序号 (全局单调递增)。
func NextSeq(tx *sql.Tx) (int64, error) {
	if _, err := tx.Exec(`INSERT INTO sync_counter (id, value) VALUES (1, 1)
		ON CONFLICT(id) DO UPDATE SET value = value + 1`); err != nil {
		return 0, err
	}
	var v int64
	err := tx.QueryRow(`SELECT value FROM sync_counter WHERE id = 1`).Scan(&v)
	return v, err
}

// MarkDirty 在事务内给一行分配新的出站序号 (同步空间才调用)。
func MarkDirty(tx *sql.Tx, table, keyCol, id, spaceID string) error {
	seq, err := NextSeq(tx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE `+table+` SET dirty = ? WHERE `+keyCol+` = ? AND space = ?`, seq, id, spaceID)
	return err
}

var (
	errReadOnly          = errcode.ErrSyncReadOnly
	errSettingsAdminOnly = errcode.ErrTeamSettingsAdminOnly
)
