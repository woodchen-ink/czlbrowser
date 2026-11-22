package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// ProfileDAO 存取实例。List 返回全部空间未删除的实例 (Manager 内存里保留全部,
// 切空间后正在运行的实例不丢), 界面再按当前空间过滤。
type ProfileDAO interface {
	List() ([]*Profile, error)
	GetById(profileID string) (*Profile, error)
	Upsert(profile *Profile) error
	Delete(profileID string) error
}

// ProfileWire 是实例在同步文档里的内容。用户数据目录与运行态只留本机。
type ProfileWire struct {
	Name            string   `json:"name"`
	CoreID          string   `json:"coreId"`
	FingerprintArgs []string `json:"fingerprintArgs"`
	ProxyID         string   `json:"proxyId"`
	ProxyConfig     string   `json:"proxyConfig"`
	LaunchArgs      []string `json:"launchArgs"`
	Tags            []string `json:"tags"`
	Keywords        []string `json:"keywords"`
	GroupID         string   `json:"groupId"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// WireOf 取实例的同步内容; 空列表输出 [] 而不是 null, 保证比较稳定。
func WireOf(p *Profile) ProfileWire {
	list := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	return ProfileWire{
		Name: p.ProfileName, CoreID: p.CoreId, FingerprintArgs: list(p.FingerprintArgs),
		ProxyID: p.ProxyId, ProxyConfig: p.ProxyConfig, LaunchArgs: list(p.LaunchArgs),
		Tags: list(p.Tags), Keywords: list(p.Keywords), GroupID: p.GroupId,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// SQLiteProfileDAO 把实例存在 browser_profiles 表。
type SQLiteProfileDAO struct{ spaceAware }

func NewSQLiteProfileDAO(db *sql.DB, spaces *space.State) *SQLiteProfileDAO {
	return &SQLiteProfileDAO{spaceAware{db: db, spaces: spaces}}
}

const profileSelect = `SELECT profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config,
	launch_args, tags, keywords, COALESCE(group_id, ''), created_at, updated_at, space, COALESCE(last_start_at, ''), deleted
	FROM browser_profiles`

// readProfile 扫描一行, 同时返回墓碑标记。
func readProfile(s scanner) (*Profile, bool, error) {
	p := &Profile{}
	var fp, launch, tags, keywords string
	var deleted bool
	err := s.Scan(&p.ProfileId, &p.ProfileName, &p.UserDataDir, &p.CoreId, &fp, &p.ProxyId, &p.ProxyConfig,
		&launch, &tags, &keywords, &p.GroupId, &p.CreatedAt, &p.UpdatedAt, &p.Space, &p.LastStartAt, &deleted)
	if err != nil {
		return nil, false, err
	}
	p.FingerprintArgs, p.LaunchArgs = jsonStrings(fp), jsonStrings(launch)
	p.Tags, p.Keywords = jsonStrings(tags), jsonStrings(keywords)
	return p, deleted, nil
}

// List 返回全部空间未删除的实例, 按创建时间排序。
func (d *SQLiteProfileDAO) List() ([]*Profile, error) {
	rows, err := d.db.Query(profileSelect + ` WHERE deleted = 0 ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("读取实例失败: %w", err)
	}
	defer rows.Close()
	var out []*Profile
	for rows.Next() {
		p, _, err := readProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("读取实例失败: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *SQLiteProfileDAO) GetById(id string) (*Profile, error) {
	p, _, err := readProfile(d.db.QueryRow(profileSelect+` WHERE profile_id = ? AND deleted = 0`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("实例不存在: %s", id)
	}
	return p, err
}

// MarkStarted 记录最近启动时间。本机字段: 不同步、不标脏、不改 updated_at。
func (d *SQLiteProfileDAO) MarkStarted(id, at string) error {
	_, err := d.db.Exec(`UPDATE browser_profiles SET last_start_at = ? WHERE profile_id = ?`, at, id)
	return err
}

// rejectReadOnly 只拦当前空间处于只读的写入; 别的空间 (切空间前启动的实例) 交给服务端判断。
func (d *SQLiteProfileDAO) rejectReadOnly(spaceID string) error {
	if d.spaces == nil {
		return nil
	}
	if cur, access := d.spaces.CurrentAccess(); cur == spaceID && access.ReadOnly {
		return errcode.ErrSyncReadOnly
	}
	return nil
}

// Upsert 写一个实例。空间取实例自带的, 其次已有行的, 再次当前空间; 同一 id 已在别的空间返回 40904。
// 同步字段有变化 (含新建、复活墓碑) 时在同步空间标脏。
func (d *SQLiteProfileDAO) Upsert(p *Profile) error {
	now := time.Now().Format(time.RFC3339)
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	if p.UpdatedAt == "" {
		p.UpdatedAt = now
	}
	var changed bool
	err := d.tx("保存实例配置", func(tx *sql.Tx) error {
		old, deleted, err := readProfile(tx.QueryRow(profileSelect+` WHERE profile_id = ?`, p.ProfileId))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		sp := p.Space
		if sp == "" && old != nil {
			sp = old.Space
		}
		if sp == "" {
			sp = d.current()
		}
		if old != nil && old.Space != sp {
			return errcode.ErrSpaceMismatch
		}
		p.Space = sp
		changed = old == nil || deleted || !sameJSON(WireOf(old), WireOf(p))
		if changed {
			if err := d.rejectReadOnly(sp); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args,
				proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at, space, deleted)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
			ON CONFLICT(profile_id) DO UPDATE SET profile_name = excluded.profile_name, user_data_dir = excluded.user_data_dir,
				core_id = excluded.core_id, fingerprint_args = excluded.fingerprint_args, proxy_id = excluded.proxy_id,
				proxy_config = excluded.proxy_config, launch_args = excluded.launch_args, tags = excluded.tags,
				keywords = excluded.keywords, group_id = excluded.group_id, updated_at = excluded.updated_at, deleted = 0`,
			p.ProfileId, p.ProfileName, p.UserDataDir, p.CoreId, toJSON(p.FingerprintArgs), p.ProxyId, p.ProxyConfig,
			toJSON(p.LaunchArgs), toJSON(p.Tags), toJSON(p.Keywords), p.GroupId, p.CreatedAt, p.UpdatedAt, sp); err != nil {
			return err
		}
		if changed {
			return markIfSynced(tx, "browser_profiles", "profile_id", p.ProfileId, sp)
		}
		return nil
	})
	if err == nil && changed {
		d.spaces.Edited(p.Space, space.CollProfiles, p.ProfileId)
	}
	return err
}

// Delete 删除实例 (连同登录态快照)。同步空间留墓碑, 本机空间直接删。
func (d *SQLiteProfileDAO) Delete(id string) error {
	var sp string
	err := d.tx("删除实例配置", func(tx *sql.Tx) error {
		old, deleted, err := readProfile(tx.QueryRow(profileSelect+` WHERE profile_id = ?`, id))
		if errors.Is(err, sql.ErrNoRows) || (err == nil && deleted) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := d.rejectReadOnly(old.Space); err != nil {
			return err
		}
		sp = old.Space
		if err := tombstone(tx, "browser_profiles", "profile_id", id, sp); err != nil {
			return err
		}
		return dropLoginState(tx, id, sp)
	})
	if err == nil && space.IsSynced(sp) {
		d.spaces.Edited(sp, space.CollProfiles, id)
	}
	return err
}

// dropLoginState 删实例时一并删登录态快照; 同步空间推墓碑, 服务端随引用校正清理附件。
func dropLoginState(tx *sql.Tx, profileID, spaceID string) error {
	var rev int64
	err := tx.QueryRow(`SELECT rev FROM profile_states WHERE profile_id = ?`, profileID).Scan(&rev)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !space.IsSynced(spaceID) {
		_, err := tx.Exec(`DELETE FROM profile_states WHERE profile_id = ?`, profileID)
		return err
	}
	if _, err := tx.Exec(`UPDATE profile_states SET deleted = 1, pack_path = '' WHERE profile_id = ?`, profileID); err != nil {
		return err
	}
	return space.MarkDirty(tx, "profile_states", "profile_id", profileID, spaceID)
}
