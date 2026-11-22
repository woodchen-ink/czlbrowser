// Package syncstore 是同步层 (internal/cloudsync) 的本地存储: 各表 dirty > 0 的行就是同步 SDK 的出站队列
// (dirty 即条目 Seq, rev 即 BaseRev, deleted 即删除), 这里按 SDK 语义实现取队列、确认、改基线、放弃,
// 以及拉取结果落地、游标与登录态快照的本机状态。
//
// 与界面编辑 (browser 包的 DAO) 的区别: 这里的方法都显式带空间, 不看当前空间 —— 后台同步的空间不一定是界面上正在看的;
// 拉取落地不标脏、不通知同步。
package syncstore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// Store 是同步本地存储, 与 DAO 共用同一个 SQLite 连接。
type Store struct {
	db *sql.DB
}

// New 创建 Store。
func New(db *sql.DB) *Store { return &Store{db: db} }

// Collections 是要同步的全部集合, 推送按这个顺序取脏数据 (被引用的先推)。
var Collections = []string{
	space.CollSettings, space.CollGroups, space.CollProxies, space.CollProfiles, space.CollStates,
}

type table struct {
	name   string
	key    string
	keyCol string // 主键在 SQL 里的写法
}

var tables = map[string]table{
	space.CollProfiles: {"browser_profiles", "profile_id", "profile_id"},
	space.CollProxies:  {"browser_proxies", "proxy_id", "proxy_id"},
	space.CollGroups:   {"browser_groups", "group_id", "group_id"},
	space.CollSettings: {"space_settings", "key", "key"},
	space.CollStates:   {"profile_states", "profile_id", "profile_id"},
}

func tableOf(collection string) (table, error) {
	t, ok := tables[collection]
	if !ok {
		return table{}, errcode.ErrSyncPayload.Wrap(fmt.Errorf("unknown collection %q", collection))
	}
	return t, nil
}

// Syncable 判断拉到的文档是否属于本机认识的集合。
func Syncable(collection string) bool {
	_, ok := tables[collection]
	return ok
}

// Doc 是同步层看到的一条本地文档。
type Doc struct {
	Collection string
	ID         string
	// Rev 本地已知的服务端序号, 推送时作 baseRev; 0 = 服务端还没有
	Rev int64
	// Dirty 出站序号, 0 = 干净
	Dirty   int64
	Deleted bool
	// Payload 文档内容 (各集合的 Wire 结构 JSON), 墓碑为空。states 集合为最近一次已知的清单
	Payload json.RawMessage
	// PackPath 仅 states: 待上传的本地登录态包, 空 = 没有
	PackPath string
	// Err 本地这行内容读不出来 (Payload 为空), 只影响这一条
	Err error
}

func (s *Store) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return errcode.ErrDBQuery.Wrap(err)
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return errcode.ErrDBQuery.Wrap(err)
	}
	return nil
}

// ─── 出站队列 ─────────────────────────────────────────────────────────────

// DirtyDocs 按出站序号升序取空间里待推送的文档, 至多 limit 条。
func (s *Store) DirtyDocs(spaceID string, limit int) ([]Doc, error) {
	type ref struct {
		coll  string
		id    string
		dirty int64
	}
	var refs []ref
	for _, coll := range Collections {
		t := tables[coll]
		rows, err := s.db.Query(`SELECT `+t.keyCol+`, dirty FROM `+t.name+` WHERE space = ? AND dirty > 0 ORDER BY dirty LIMIT ?`, spaceID, limit)
		if err != nil {
			return nil, errcode.ErrDBQuery.Wrap(err)
		}
		for rows.Next() {
			var r ref
			r.coll = coll
			if err := rows.Scan(&r.id, &r.dirty); err != nil {
				rows.Close()
				return nil, errcode.ErrDBQuery.Wrap(err)
			}
			refs = append(refs, r)
		}
		rows.Close()
	}
	// 合并各表后按序号排, 取前 limit 条
	for i := 1; i < len(refs); i++ {
		for j := i; j > 0 && refs[j].dirty < refs[j-1].dirty; j-- {
			refs[j], refs[j-1] = refs[j-1], refs[j]
		}
	}
	if len(refs) > limit {
		refs = refs[:limit]
	}
	out := make([]Doc, 0, len(refs))
	for _, r := range refs {
		d, err := s.DocState(spaceID, r.coll, r.id)
		if err != nil {
			return nil, err
		}
		if d != nil {
			out = append(out, *d)
		}
	}
	return out, nil
}

// DocState 取空间里的一条文档 (含墓碑), 没有返回 nil。
func (s *Store) DocState(spaceID, collection, id string) (*Doc, error) {
	t, err := tableOf(collection)
	if err != nil {
		return nil, err
	}
	d := &Doc{Collection: collection, ID: id}
	var q string
	switch collection {
	case space.CollStates:
		var payload string
		var applied int64
		q = `SELECT rev, dirty, deleted, payload, pack_path, applied_rev FROM profile_states WHERE profile_id = ? AND space = ?`
		err = s.db.QueryRow(q, id, spaceID).Scan(&d.Rev, &d.Dirty, &d.Deleted, &payload, &d.PackPath, &applied)
		d.Payload = json.RawMessage(payload)
		// 待上传的登录态以本机用户数据目录对应的版本为基线: 运行期间拉到了别人更新的快照 (rev 前进但没下载),
		// 这时打的包是基于旧版本的, 推送必须 409 走冲突, 不能悄悄盖掉别人的登录态
		if d.Dirty > 0 && !d.Deleted && applied < d.Rev {
			d.Rev = applied
		}
	default:
		q = `SELECT rev, dirty, deleted FROM ` + t.name + ` WHERE ` + t.keyCol + ` = ? AND space = ?`
		err = s.db.QueryRow(q, id, spaceID).Scan(&d.Rev, &d.Dirty, &d.Deleted)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errcode.ErrDBQuery.Wrap(err)
	}
	if !d.Deleted && collection != space.CollStates {
		d.Payload, d.Err = s.payload(spaceID, collection, id)
	}
	if len(d.Payload) == 0 {
		d.Payload = nil
	}
	return d, nil
}

// AckPushed 记下推送成功: 出站序号没变的清掉 dirty; 推送途中又改过的照旧算脏。确认的是墓碑时直接删行。
// states 集合确认后本机用户数据目录就是这一版 (applied_rev = rev), 返回不再需要的本地包路径由调用方删除。
func (s *Store) AckPushed(spaceID, collection, id string, seq, rev int64, payload json.RawMessage) (string, error) {
	t, err := tableOf(collection)
	if err != nil {
		return "", err
	}
	var pack string
	err = s.inTx(func(tx *sql.Tx) error {
		if collection == space.CollStates {
			_ = tx.QueryRow(`SELECT pack_path FROM profile_states WHERE profile_id = ? AND space = ? AND dirty = ?`, id, spaceID, seq).Scan(&pack)
		}
		if _, err := tx.Exec(`DELETE FROM `+t.name+` WHERE `+t.keyCol+` = ? AND space = ? AND deleted = 1 AND dirty = ?`, id, spaceID, seq); err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		if collection == space.CollStates {
			_, err := tx.Exec(`UPDATE profile_states SET rev = ?, applied_rev = CASE WHEN dirty = ? THEN ? ELSE applied_rev END,
				payload = CASE WHEN dirty = ? THEN ? ELSE payload END,
				pack_path = CASE WHEN dirty = ? THEN '' ELSE pack_path END,
				dirty = CASE WHEN dirty = ? THEN 0 ELSE dirty END
				WHERE profile_id = ? AND space = ?`,
				rev, seq, rev, seq, string(payload), seq, seq, id, spaceID)
			if err != nil {
				return errcode.ErrDBQuery.Wrap(err)
			}
			return nil
		}
		if _, err := tx.Exec(`UPDATE `+t.name+` SET rev = ?, dirty = CASE WHEN dirty = ? THEN 0 ELSE dirty END
			WHERE `+t.keyCol+` = ? AND space = ?`, rev, seq, id, spaceID); err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		return nil
	})
	return pack, err
}

// Rebase 把待推送条目的基线改为 rev 以便覆盖重推 (SDK 重加密时用到)。
func (s *Store) Rebase(spaceID, collection, id string, seq, baseRev, rev int64) error {
	t, err := tableOf(collection)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE `+t.name+` SET rev = ? WHERE `+t.keyCol+` = ? AND space = ? AND dirty > 0 AND (dirty = ? OR rev = ?)`,
		rev, id, spaceID, seq, baseRev); err != nil {
		return errcode.ErrDBQuery.Wrap(err)
	}
	return nil
}

// DropPending 放弃一条待推送修改: 出站序号仍是 seq 时清掉 dirty, 本地内容保留; 是墓碑的直接删行。
// states 集合同时返回被放弃的本地包路径, 由调用方删除。
func (s *Store) DropPending(spaceID, collection, id string, seq int64) (string, error) {
	t, err := tableOf(collection)
	if err != nil {
		return "", err
	}
	var pack string
	err = s.inTx(func(tx *sql.Tx) error {
		if collection == space.CollStates {
			_ = tx.QueryRow(`SELECT pack_path FROM profile_states WHERE profile_id = ? AND space = ? AND dirty = ?`, id, spaceID, seq).Scan(&pack)
			if _, err := tx.Exec(`UPDATE profile_states SET pack_path = '' WHERE profile_id = ? AND space = ? AND dirty = ?`, id, spaceID, seq); err != nil {
				return errcode.ErrDBQuery.Wrap(err)
			}
		}
		if _, err := tx.Exec(`DELETE FROM `+t.name+` WHERE `+t.keyCol+` = ? AND space = ? AND deleted = 1 AND dirty = ?`, id, spaceID, seq); err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		if _, err := tx.Exec(`UPDATE `+t.name+` SET dirty = 0 WHERE `+t.keyCol+` = ? AND space = ? AND dirty = ?`, id, spaceID, seq); err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		return nil
	})
	return pack, err
}

// ForgetRev 把一条文档的基线清零、出站标记清掉: 服务端永久拒绝了本机修改 (角色不够) 时调用,
// 配合游标归零, 下次拉取用服务端版本把本地改回来。
func (s *Store) ForgetRev(spaceID, collection, id string) error {
	t, err := tableOf(collection)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE `+t.name+` SET rev = 0, dirty = 0 WHERE `+t.keyCol+` = ? AND space = ?`, id, spaceID); err != nil {
		return errcode.ErrDBQuery.Wrap(err)
	}
	return nil
}

// CommitRev 记下服务端接受本机写入后的 rev, 只往前走、只改干净的行。
func (s *Store) CommitRev(spaceID, collection, id string, rev int64) error {
	t, err := tableOf(collection)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE `+t.name+` SET rev = ? WHERE `+t.keyCol+` = ? AND space = ? AND dirty = 0 AND rev < ?`,
		rev, id, spaceID, rev); err != nil {
		return errcode.ErrDBQuery.Wrap(err)
	}
	return nil
}

// ResetRevIfClean 把一条没有未推送修改的文档基线清零, 下次拉到服务端版本时一定覆盖。
func (s *Store) ResetRevIfClean(spaceID, collection, id string) error {
	t, err := tableOf(collection)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE `+t.name+` SET rev = 0 WHERE `+t.keyCol+` = ? AND space = ? AND dirty = 0`, id, spaceID); err != nil {
		return errcode.ErrDBQuery.Wrap(err)
	}
	return nil
}

// PendingCount 统计空间里还没推送的文档数。
func (s *Store) PendingCount(spaceID string) (int, error) {
	total := 0
	for _, coll := range Collections {
		t := tables[coll]
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+t.name+` WHERE space = ? AND dirty > 0`, spaceID).Scan(&n); err != nil {
			return 0, errcode.ErrDBQuery.Wrap(err)
		}
		total += n
	}
	return total, nil
}

// StatePayloads 返回空间里已与服务端一致的登录态清单 (未删除的 profile_states.payload), 以及待上传的条数。
// 用于附件引用校正: 有待上传的就不该上报 (本机清单可能比服务端旧或新, 报出的集合不完整)。
func (s *Store) StatePayloads(spaceID string) (payloads []string, dirty int, err error) {
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM profile_states WHERE space = ? AND dirty > 0`, spaceID).Scan(&dirty); err != nil {
		return nil, 0, errcode.ErrDBQuery.Wrap(err)
	}
	rows, err := s.db.Query(`SELECT payload FROM profile_states WHERE space = ? AND deleted = 0 AND rev > 0 AND payload <> ''`, spaceID)
	if err != nil {
		return nil, 0, errcode.ErrDBQuery.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, 0, errcode.ErrDBQuery.Wrap(err)
		}
		payloads = append(payloads, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errcode.ErrDBQuery.Wrap(err)
	}
	return payloads, dirty, nil
}

// ─── 拉取落地 ─────────────────────────────────────────────────────────────

// ErrLocalChanged 表示本地这条有未推送修改: 服务端版本没有写入, 推送时会以 409 走冲突处理。
var ErrLocalChanged = errors.New("syncstore: local doc changed during sync")

// Remote 是一次服务端版本落地。
type Remote struct {
	Rev     int64
	Deleted bool
	// Payload 各集合的 Wire 结构 JSON (已经过 cloudsync 校验), 墓碑为空
	Payload json.RawMessage
}

// ApplyRemote 让服务端版本落地: 墓碑删掉本地这条, 否则写入且落库后是干净的。
// 本地这条有未推送修改返回 ErrLocalChanged; 同一 id 在本机另一个空间返回 40904。
func (s *Store) ApplyRemote(spaceID, collection, id string, w Remote) error {
	t, err := tableOf(collection)
	if err != nil {
		return err
	}
	return s.inTx(func(tx *sql.Tx) error {
		var dirty int64
		var owner string
		q := `SELECT dirty, space FROM ` + t.name + ` WHERE ` + t.keyCol + ` = ?`
		args := []any{id}
		if collection == space.CollSettings {
			q += ` AND space = ?`
			args = append(args, spaceID)
		}
		err := tx.QueryRow(q, args...).Scan(&dirty, &owner)
		exists := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return errcode.ErrDBQuery.Wrap(err)
		}
		if exists && owner != spaceID {
			return errcode.ErrSpaceMismatch
		}
		if dirty != 0 {
			return ErrLocalChanged
		}
		if w.Deleted {
			if collection == space.CollStates {
				// 登录态快照删了 (实例删了): 只清清单, 本机用户数据目录由实例删除流程处理
				_, err := tx.Exec(`DELETE FROM profile_states WHERE profile_id = ? AND space = ? AND dirty = 0`, id, spaceID)
				return wrapDB(err)
			}
			_, err := tx.Exec(`DELETE FROM `+t.name+` WHERE `+t.keyCol+` = ? AND space = ? AND dirty = 0`, id, spaceID)
			return wrapDB(err)
		}
		return writeRemote(tx, spaceID, collection, id, w.Rev, w.Payload)
	})
}

func wrapDB(err error) error {
	if err != nil {
		return errcode.ErrDBQuery.Wrap(err)
	}
	return nil
}

// ResetSpace 丢掉空间里已同步且没有本地修改的文档 (游标过期时), 随后从 0 全量重拉。
func (s *Store) ResetSpace(spaceID string) error {
	return s.inTx(func(tx *sql.Tx) error {
		for _, coll := range Collections {
			t := tables[coll]
			if coll == space.CollStates {
				// 登录态行保留 applied_rev (本机目录是哪一版), 只清服务端版本, 重拉后版本相同就不重复下载
				if _, err := tx.Exec(`UPDATE profile_states SET rev = 0, payload = '' WHERE space = ? AND dirty = 0`, spaceID); err != nil {
					return errcode.ErrDBQuery.Wrap(err)
				}
				continue
			}
			if _, err := tx.Exec(`DELETE FROM `+t.name+` WHERE space = ? AND dirty = 0`, spaceID); err != nil {
				return errcode.ErrDBQuery.Wrap(err)
			}
		}
		return nil
	})
}

// DeleteSpace 删除空间的全部本地数据与游标 (离开团队、团队解散)。返回各实例 id 与待上传包路径, 调用方删用户数据目录与包文件。
func (s *Store) DeleteSpace(spaceID string) (profileIDs []string, packs []string, err error) {
	if !space.IsSynced(spaceID) {
		return nil, nil, errcode.ErrSpaceMismatch
	}
	err = s.inTx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT profile_id FROM browser_profiles WHERE space = ?`, spaceID)
		if err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				profileIDs = append(profileIDs, id)
			}
		}
		rows.Close()
		rows, err = tx.Query(`SELECT pack_path FROM profile_states WHERE space = ? AND pack_path <> ''`, spaceID)
		if err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err == nil {
				packs = append(packs, p)
			}
		}
		rows.Close()
		for _, coll := range Collections {
			if _, err := tx.Exec(`DELETE FROM `+tables[coll].name+` WHERE space = ?`, spaceID); err != nil {
				return errcode.ErrDBQuery.Wrap(err)
			}
		}
		if _, err := tx.Exec(`DELETE FROM sync_spaces WHERE space = ?`, spaceID); err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		return nil
	})
	return profileIDs, packs, err
}

// ─── 游标 ─────────────────────────────────────────────────────────────────

// Cursor 取空间的拉取游标; 上次用的线上格式版本低于 wire 时返回 0 (从头重拉, 补回旧版跳过的文档)。
func (s *Store) Cursor(spaceID string, wire int) (int64, error) {
	var c int64
	var w int
	err := s.db.QueryRow(`SELECT cursor, wire FROM sync_spaces WHERE space = ?`, spaceID).Scan(&c, &w)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, errcode.ErrDBQuery.Wrap(err)
	}
	if w < wire {
		return 0, nil
	}
	return c, nil
}

// SetCursor 记下空间的拉取游标、线上格式版本与同步时间。
func (s *Store) SetCursor(spaceID string, cursor int64, wire int) error {
	_, err := s.db.Exec(`INSERT INTO sync_spaces (space, cursor, wire, synced_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(space) DO UPDATE SET cursor = excluded.cursor, wire = excluded.wire, synced_at = excluded.synced_at`,
		spaceID, cursor, wire, time.Now().UTC().Format(time.RFC3339))
	return wrapDB(err)
}

// LowerCursor 把游标退到 cursor (只退不进), 下次同步从那里重拉。
func (s *Store) LowerCursor(spaceID string, cursor int64) error {
	if cursor < 0 {
		cursor = 0
	}
	_, err := s.db.Exec(`UPDATE sync_spaces SET cursor = MIN(cursor, ?) WHERE space = ?`, cursor, spaceID)
	return wrapDB(err)
}

// ─── 键值 ─────────────────────────────────────────────────────────────────

// GetJSON 读一个本机键值, 不存在返回 false。
func (s *Store) GetJSON(key string, v any) (bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM app_kv WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, errcode.ErrDBQuery.Wrap(err)
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return false, errcode.ErrPayloadCodec.Wrap(err)
	}
	return true, nil
}

// SetJSON 写一个本机键值。
func (s *Store) SetJSON(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return errcode.ErrPayloadCodec.Wrap(err)
	}
	_, err = s.db.Exec(`INSERT INTO app_kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, string(b))
	return wrapDB(err)
}
