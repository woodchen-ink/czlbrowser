package syncstore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// SettingWire 是 settings 集合的文档内容, 文档 id 即 Key。
type SettingWire struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// payload 读一行并转成线上内容 (各集合的 Wire 结构)。
func (s *Store) payload(spaceID, collection, id string) (json.RawMessage, error) {
	var v any
	switch collection {
	case space.CollProfiles:
		p, err := s.profile(spaceID, id)
		if err != nil {
			return nil, err
		}
		v = browser.WireOf(p)
	case space.CollProxies:
		p, err := s.proxy(spaceID, id)
		if err != nil {
			return nil, err
		}
		v = browser.ProxyWireOf(*p)
	case space.CollGroups:
		var g browser.Group
		err := s.db.QueryRow(`SELECT group_id, group_name, parent_id, sort_order, created_at, updated_at FROM browser_groups
			WHERE group_id = ? AND space = ?`, id, spaceID).Scan(&g.GroupId, &g.GroupName, &g.ParentId, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, errcode.ErrDBQuery.Wrap(err)
		}
		v = browser.GroupWireOf(&g)
	case space.CollSettings:
		var raw string
		if err := s.db.QueryRow(`SELECT value FROM space_settings WHERE space = ? AND key = ?`, spaceID, id).Scan(&raw); err != nil {
			return nil, errcode.ErrDBQuery.Wrap(err)
		}
		if !json.Valid([]byte(raw)) {
			return nil, errcode.ErrSyncPayload.Wrap(fmt.Errorf("setting %q is not json", id))
		}
		v = SettingWire{Key: id, Value: json.RawMessage(raw)}
	default:
		return nil, errcode.ErrSyncPayload.Wrap(fmt.Errorf("no payload for %q", collection))
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, errcode.ErrPayloadCodec.Wrap(err)
	}
	return b, nil
}

func (s *Store) profile(spaceID, id string) (*browser.Profile, error) {
	var p browser.Profile
	var fp, la, tags, kw string
	err := s.db.QueryRow(`SELECT profile_id, profile_name, core_id, fingerprint_args, proxy_id, proxy_config, launch_args,
		tags, keywords, group_id, created_at, updated_at FROM browser_profiles WHERE profile_id = ? AND space = ?`, id, spaceID).Scan(
		&p.ProfileId, &p.ProfileName, &p.CoreId, &fp, &p.ProxyId, &p.ProxyConfig, &la, &tags, &kw, &p.GroupId, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, errcode.ErrDBQuery.Wrap(err)
	}
	for _, f := range []struct {
		raw string
		dst *[]string
	}{{fp, &p.FingerprintArgs}, {la, &p.LaunchArgs}, {tags, &p.Tags}, {kw, &p.Keywords}} {
		if err := json.Unmarshal([]byte(f.raw), f.dst); err != nil {
			*f.dst = []string{}
		}
	}
	return &p, nil
}

func (s *Store) proxy(spaceID, id string) (*browser.Proxy, error) {
	var p browser.Proxy
	var auto int
	var tags string
	err := s.db.QueryRow(`SELECT proxy_id, proxy_name, proxy_config, dns_servers, COALESCE(group_name, ''), sort_order,
		COALESCE(source_id, ''), COALESCE(source_url, ''), COALESCE(source_name_prefix, ''), COALESCE(source_auto_refresh, 0),
		COALESCE(source_refresh_interval_m, 0), COALESCE(tags, '[]') FROM browser_proxies WHERE proxy_id = ? AND space = ?`, id, spaceID).Scan(
		&p.ProxyId, &p.ProxyName, &p.ProxyConfig, &p.DnsServers, &p.GroupName, &p.SortOrder,
		&p.SourceID, &p.SourceURL, &p.SourceNamePrefix, &auto, &p.SourceRefreshIntervalM, &tags)
	if err != nil {
		return nil, errcode.ErrDBQuery.Wrap(err)
	}
	_ = json.Unmarshal([]byte(tags), &p.Tags)
	p.SourceAutoRefresh = auto == 1
	return &p, nil
}

func decode(collection, id string, payload []byte, v any) error {
	if err := json.Unmarshal(payload, v); err != nil {
		return errcode.ErrSyncPayload.Wrap(fmt.Errorf("%s/%s: %w", collection, id, err))
	}
	return nil
}

func nz(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// writeRemote 把服务端版本写进本地行, 落库后是干净的 (dirty = 0)。本机字段 (用户数据目录、测速结果) 保留。
func writeRemote(tx *sql.Tx, spaceID, collection, id string, rev int64, payload []byte) error {
	const dirty = 0
	switch collection {
	case space.CollProfiles:
		var w browser.ProfileWire
		if err := decode(collection, id, payload, &w); err != nil {
			return err
		}
		if w.Name == "" {
			return errcode.ErrSyncPayload.Wrap(fmt.Errorf("profiles/%s: empty name", id))
		}
		fp, _ := json.Marshal(nz(w.FingerprintArgs))
		la, _ := json.Marshal(nz(w.LaunchArgs))
		tags, _ := json.Marshal(nz(w.Tags))
		kw, _ := json.Marshal(nz(w.Keywords))
		now := time.Now().Format(time.RFC3339)
		if w.CreatedAt == "" {
			w.CreatedAt = now
		}
		if w.UpdatedAt == "" {
			w.UpdatedAt = now
		}
		// 同步空间的实例用户数据目录一律是 data 下的实例 id, 不随文档走
		_, err := tx.Exec(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args,
			proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at, space, rev, dirty, deleted)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
			ON CONFLICT(profile_id) DO UPDATE SET profile_name = excluded.profile_name, core_id = excluded.core_id,
			  fingerprint_args = excluded.fingerprint_args, proxy_id = excluded.proxy_id, proxy_config = excluded.proxy_config,
			  launch_args = excluded.launch_args, tags = excluded.tags, keywords = excluded.keywords, group_id = excluded.group_id,
			  created_at = excluded.created_at, updated_at = excluded.updated_at, rev = excluded.rev, dirty = excluded.dirty, deleted = 0`,
			id, w.Name, id, w.CoreID, string(fp), w.ProxyID, w.ProxyConfig, string(la), string(tags), string(kw), w.GroupID,
			w.CreatedAt, w.UpdatedAt, spaceID, rev, dirty)
		return wrapDB(err)

	case space.CollProxies:
		var w browser.ProxyWire
		if err := decode(collection, id, payload, &w); err != nil {
			return err
		}
		if w.Name == "" || w.Config == "" || id == browser.DirectProxyID {
			return errcode.ErrSyncPayload.Wrap(fmt.Errorf("proxies/%s: invalid", id))
		}
		auto := 0
		if w.SourceAutoRefresh {
			auto = 1
		}
		ptags, _ := json.Marshal(nz(w.Tags))
		_, err := tx.Exec(`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, sort_order,
			source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, created_at, space, rev, dirty, deleted, tags)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)
			ON CONFLICT(proxy_id) DO UPDATE SET proxy_name = excluded.proxy_name, proxy_config = excluded.proxy_config,
			  dns_servers = excluded.dns_servers, group_name = excluded.group_name, sort_order = excluded.sort_order,
			  source_id = excluded.source_id, source_url = excluded.source_url, source_name_prefix = excluded.source_name_prefix,
			  source_auto_refresh = excluded.source_auto_refresh, source_refresh_interval_m = excluded.source_refresh_interval_m,
			  tags = excluded.tags, rev = excluded.rev, dirty = excluded.dirty, deleted = 0`,
			id, w.Name, w.Config, w.DNSServers, w.GroupName, w.SortOrder, w.SourceID, w.SourceURL, w.SourceNamePrefix, auto,
			w.SourceRefreshIntervalM, time.Now().Format(time.RFC3339), spaceID, rev, dirty, string(ptags))
		return wrapDB(err)

	case space.CollGroups:
		var w browser.GroupWire
		if err := decode(collection, id, payload, &w); err != nil {
			return err
		}
		if w.Name == "" {
			return errcode.ErrSyncPayload.Wrap(fmt.Errorf("groups/%s: empty name", id))
		}
		now := time.Now().Format(time.RFC3339)
		if w.CreatedAt == "" {
			w.CreatedAt = now
		}
		if w.UpdatedAt == "" {
			w.UpdatedAt = now
		}
		_, err := tx.Exec(`INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at, space, rev, dirty, deleted)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
			ON CONFLICT(group_id) DO UPDATE SET group_name = excluded.group_name, parent_id = excluded.parent_id,
			  sort_order = excluded.sort_order, created_at = excluded.created_at, updated_at = excluded.updated_at,
			  rev = excluded.rev, dirty = excluded.dirty, deleted = 0`,
			id, w.Name, w.ParentID, w.SortOrder, w.CreatedAt, w.UpdatedAt, spaceID, rev, dirty)
		return wrapDB(err)

	case space.CollSettings:
		var w SettingWire
		if err := decode(collection, id, payload, &w); err != nil {
			return err
		}
		if w.Key != id || id != browser.SettingBookmarks || !json.Valid(w.Value) {
			return errcode.ErrSyncPayload.Wrap(fmt.Errorf("settings/%s: invalid", id))
		}
		_, err := tx.Exec(`INSERT INTO space_settings (space, key, value, rev, dirty, deleted, updated_at) VALUES (?, ?, ?, ?, ?, 0, ?)
			ON CONFLICT(space, key) DO UPDATE SET value = excluded.value, rev = excluded.rev, dirty = excluded.dirty, deleted = 0,
			  updated_at = excluded.updated_at`,
			spaceID, id, string(w.Value), rev, dirty, time.Now().UTC().Format(time.RFC3339))
		return wrapDB(err)

	case space.CollStates:
		// 只记清单与版本, 不下载: 实例启动前按需下载 (applied_rev < rev 时)
		if !json.Valid(payload) {
			return errcode.ErrSyncPayload.Wrap(fmt.Errorf("states/%s: invalid", id))
		}
		_, err := tx.Exec(`INSERT INTO profile_states (profile_id, space, rev, dirty, deleted, payload, updated_at) VALUES (?, ?, ?, 0, 0, ?, ?)
			ON CONFLICT(profile_id) DO UPDATE SET rev = excluded.rev, payload = excluded.payload, deleted = 0, updated_at = excluded.updated_at`,
			id, spaceID, rev, string(payload), time.Now().UTC().Format(time.RFC3339))
		return wrapDB(err)
	}
	return errcode.ErrSyncPayload.Wrap(fmt.Errorf("unknown collection %q", collection))
}

// ─── 登录态快照 ───────────────────────────────────────────────────────────

// StateRow 是一个实例登录态在本机的状态。
type StateRow struct {
	ProfileID string
	Space     string
	// Rev 服务端最新的快照版本 (本机已知)
	Rev   int64
	Dirty int64
	// Payload 最近一次已知的清单 (服务端版本, 或本机上传成功后的)
	Payload json.RawMessage
	// PackPath 待上传的本地包, 空 = 没有
	PackPath string
	// AppliedRev 本机用户数据目录对应的快照版本
	AppliedRev int64
}

// State 取实例的登录态状态, 没有返回 nil。
func (s *Store) State(profileID string) (*StateRow, error) {
	r := &StateRow{ProfileID: profileID}
	var payload string
	var deleted bool
	err := s.db.QueryRow(`SELECT space, rev, dirty, deleted, payload, pack_path, applied_rev FROM profile_states WHERE profile_id = ?`,
		profileID).Scan(&r.Space, &r.Rev, &r.Dirty, &deleted, &payload, &r.PackPath, &r.AppliedRev)
	if errors.Is(err, sql.ErrNoRows) || deleted {
		return nil, nil
	}
	if err != nil {
		return nil, errcode.ErrDBQuery.Wrap(err)
	}
	if payload != "" {
		r.Payload = json.RawMessage(payload)
	}
	return r, nil
}

// SetStatePack 记下实例停止后打好的待上传包并排进出站队列, 返回被替换掉的旧包路径 (调用方删除)。
func (s *Store) SetStatePack(spaceID, profileID, packPath string) (string, error) {
	var old string
	err := s.inTx(func(tx *sql.Tx) error {
		var owner string
		err := tx.QueryRow(`SELECT space, pack_path FROM profile_states WHERE profile_id = ?`, profileID).Scan(&owner, &old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return errcode.ErrDBQuery.Wrap(err)
		}
		if err == nil && owner != spaceID {
			return errcode.ErrSpaceMismatch
		}
		seq, err := space.NextSeq(tx)
		if err != nil {
			return errcode.ErrDBQuery.Wrap(err)
		}
		_, err = tx.Exec(`INSERT INTO profile_states (profile_id, space, dirty, pack_path, updated_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(profile_id) DO UPDATE SET dirty = excluded.dirty, pack_path = excluded.pack_path, deleted = 0,
			  updated_at = excluded.updated_at`,
			profileID, spaceID, seq, packPath, time.Now().UTC().Format(time.RFC3339))
		return wrapDB(err)
	})
	if old == packPath {
		old = ""
	}
	return old, err
}

// SetApplied 记下本机用户数据目录已还原到快照版本 rev (下载解包成功后)。
func (s *Store) SetApplied(profileID string, rev int64) error {
	_, err := s.db.Exec(`UPDATE profile_states SET applied_rev = ? WHERE profile_id = ? AND applied_rev < ?`, rev, profileID, rev)
	return wrapDB(err)
}

// ProfileSpace 返回实例所在空间, 实例不存在返回空串。
func (s *Store) ProfileSpace(profileID string) (string, error) {
	var sp string
	err := s.db.QueryRow(`SELECT space FROM browser_profiles WHERE profile_id = ? AND deleted = 0`, profileID).Scan(&sp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return sp, wrapDB(err)
}
