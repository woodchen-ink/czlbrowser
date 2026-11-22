package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// DirectProxyID 是内置的「直连」代理。每个空间都有它但不同步: 本机空间里是真实的一行,
// 同步空间里由 List 补上一条虚拟的。
const DirectProxyID = "__direct__"

// DirectProxy 返回内置直连代理。
func DirectProxy() Proxy {
	return Proxy{ProxyId: DirectProxyID, ProxyName: "直连（不走代理）", ProxyConfig: "direct://"}
}

// ProxyDAO 存取当前空间的代理池。
type ProxyDAO interface {
	List() ([]Proxy, error)
	ListByGroup(groupName string) ([]Proxy, error)
	ListGroups() ([]string, error)
	Upsert(proxy Proxy) error
	Delete(proxyID string) error
	// ReplaceAll 让当前空间的代理变成 proxies: 不在列表里的删掉, 其余按 id 更新 (没变的不标脏)
	ReplaceAll(proxies []Proxy) error
	UpdateSpeedResult(proxyID string, ok bool, latencyMs int64, testedAt string) error
	UpdateIPHealthResult(proxyID string, healthJSON string) error
}

// ProxyWire 是代理在同步文档里的内容。测速、出口检测与订阅刷新时间只留本机。
type ProxyWire struct {
	Name                   string   `json:"name"`
	Config                 string   `json:"config"`
	DNSServers             string   `json:"dnsServers"`
	GroupName              string   `json:"groupName"`
	Tags                   []string `json:"tags"`
	SortOrder              int      `json:"sortOrder"`
	SourceID               string   `json:"sourceId"`
	SourceURL              string   `json:"sourceUrl"`
	SourceNamePrefix       string   `json:"sourceNamePrefix"`
	SourceAutoRefresh      bool     `json:"sourceAutoRefresh"`
	SourceRefreshIntervalM int      `json:"sourceRefreshIntervalM"`
}

// ProxyWireOf 取代理的同步内容。
func ProxyWireOf(p Proxy) ProxyWire {
	return ProxyWire{
		Name: p.ProxyName, Config: p.ProxyConfig, DNSServers: p.DnsServers, GroupName: p.GroupName,
		Tags: NormalizeTags(p.Tags), SortOrder: p.SortOrder,
		SourceID: p.SourceID, SourceURL: p.SourceURL, SourceNamePrefix: p.SourceNamePrefix,
		SourceAutoRefresh: p.SourceAutoRefresh, SourceRefreshIntervalM: p.SourceRefreshIntervalM,
	}
}

// SQLiteProxyDAO 把代理存在 browser_proxies 表。
type SQLiteProxyDAO struct{ spaceAware }

func NewSQLiteProxyDAO(db *sql.DB, spaces *space.State) *SQLiteProxyDAO {
	return &SQLiteProxyDAO{spaceAware{db: db, spaces: spaces}}
}

// proxySelect 列出读取代理需要的列, 与 readProxy 的扫描顺序一致。
const proxySelect = `SELECT proxy_id, proxy_name, proxy_config, dns_servers, COALESCE(group_name, ''), COALESCE(tags, '[]'), sort_order,
	COALESCE(source_id, ''), COALESCE(source_url, ''), COALESCE(source_name_prefix, ''), COALESCE(source_auto_refresh, 0),
	COALESCE(source_refresh_interval_m, 0), COALESCE(source_last_refresh_at, ''),
	COALESCE(last_latency_ms, -1), COALESCE(last_test_ok, 0), COALESCE(last_tested_at, ''), COALESCE(last_ip_health_json, ''),
	space, deleted FROM browser_proxies`

// proxyRow 是一行代理及其所属空间与墓碑标记。
type proxyRow struct {
	Proxy
	space   string
	deleted bool
}

func readProxy(s scanner) (proxyRow, error) {
	var r proxyRow
	var tags string
	err := s.Scan(&r.ProxyId, &r.ProxyName, &r.ProxyConfig, &r.DnsServers, &r.GroupName, &tags, &r.SortOrder,
		&r.SourceID, &r.SourceURL, &r.SourceNamePrefix, &r.SourceAutoRefresh, &r.SourceRefreshIntervalM, &r.SourceLastRefreshAt,
		&r.LastLatencyMs, &r.LastTestOk, &r.LastTestedAt, &r.LastIPHealthJSON, &r.space, &r.deleted)
	r.Tags = decodeTags(tags)
	return r, err
}

func (d *SQLiteProxyDAO) query(where string, args ...any) ([]Proxy, error) {
	rows, err := d.db.Query(proxySelect+` WHERE deleted = 0 AND `+where+` ORDER BY sort_order, created_at`, args...)
	if err != nil {
		return nil, fmt.Errorf("读取代理失败: %w", err)
	}
	defer rows.Close()
	var out []Proxy
	for rows.Next() {
		r, err := readProxy(rows)
		if err != nil {
			return nil, fmt.Errorf("读取代理失败: %w", err)
		}
		out = append(out, r.Proxy)
	}
	return out, rows.Err()
}

// List 返回当前空间的代理; 同步空间在最前面补上内置直连。
func (d *SQLiteProxyDAO) List() ([]Proxy, error) {
	sp := d.current()
	list, err := d.ListIn(sp)
	if err != nil || !space.IsSynced(sp) {
		return list, err
	}
	return append([]Proxy{DirectProxy()}, list...), nil
}

// ListIn 返回指定空间的代理 (不补内置直连)。
func (d *SQLiteProxyDAO) ListIn(spaceID string) ([]Proxy, error) {
	return d.query(`space = ?`, spaceID)
}

func (d *SQLiteProxyDAO) ListByGroup(group string) ([]Proxy, error) {
	return d.query(`space = ? AND group_name = ?`, d.current(), group)
}

// ListGroups 返回当前空间用到的代理分组名, 按名称排序。
func (d *SQLiteProxyDAO) ListGroups() ([]string, error) {
	rows, err := d.db.Query(`SELECT DISTINCT group_name FROM browser_proxies WHERE space = ? AND deleted = 0 AND group_name != '' ORDER BY group_name`, d.current())
	if err != nil {
		return nil, fmt.Errorf("读取代理分组失败: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Upsert 写当前空间的一条代理。
func (d *SQLiteProxyDAO) Upsert(p Proxy) error {
	sp, err := d.writable()
	if err != nil {
		return err
	}
	return d.UpsertIn(sp, p)
}

// UpsertIn 写指定空间的一条代理 (跨空间复制时用, 调用方负责检查目标空间可写)。
func (d *SQLiteProxyDAO) UpsertIn(spaceID string, p Proxy) error {
	if p.ProxyId == DirectProxyID && space.IsSynced(spaceID) {
		return nil
	}
	var changed bool
	err := d.tx("保存代理", func(tx *sql.Tx) (err error) {
		changed, err = writeProxy(tx, spaceID, p)
		return err
	})
	if err == nil && changed {
		d.spaces.Edited(spaceID, space.CollProxies, p.ProxyId)
	}
	return err
}

// writeProxy 写入一行; 同步字段有变化 (含新建、复活墓碑) 时在同步空间标脏。返回是否有变化。
func writeProxy(tx *sql.Tx, sp string, p Proxy) (bool, error) {
	old, err := readProxy(tx.QueryRow(proxySelect+` WHERE proxy_id = ?`, p.ProxyId))
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if exists && old.space != sp {
		return false, errcode.ErrSpaceMismatch
	}
	changed := !exists || old.deleted || !reflect.DeepEqual(ProxyWireOf(old.Proxy), ProxyWireOf(p))
	_, err = tx.Exec(`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, tags, sort_order,
			source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at,
			created_at, space, deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(proxy_id) DO UPDATE SET proxy_name = excluded.proxy_name, proxy_config = excluded.proxy_config,
			dns_servers = excluded.dns_servers, group_name = excluded.group_name, tags = excluded.tags, sort_order = excluded.sort_order,
			source_id = excluded.source_id, source_url = excluded.source_url, source_name_prefix = excluded.source_name_prefix,
			source_auto_refresh = excluded.source_auto_refresh, source_refresh_interval_m = excluded.source_refresh_interval_m,
			source_last_refresh_at = excluded.source_last_refresh_at, deleted = 0`,
		p.ProxyId, p.ProxyName, p.ProxyConfig, p.DnsServers, p.GroupName, encodeTags(p.Tags), p.SortOrder,
		p.SourceID, p.SourceURL, p.SourceNamePrefix, boolInt(p.SourceAutoRefresh), p.SourceRefreshIntervalM, p.SourceLastRefreshAt,
		time.Now().Format(time.RFC3339), sp)
	if err != nil {
		return false, err
	}
	if changed {
		if err := markIfSynced(tx, "browser_proxies", "proxy_id", p.ProxyId, sp); err != nil {
			return false, err
		}
	}
	return changed, nil
}

// Delete 删除当前空间的一条代理。
func (d *SQLiteProxyDAO) Delete(id string) error {
	sp, err := d.writable()
	if err != nil {
		return err
	}
	if err := d.tx("删除代理", func(tx *sql.Tx) error { return tombstone(tx, "browser_proxies", "proxy_id", id, sp) }); err != nil {
		return err
	}
	d.spaces.Edited(sp, space.CollProxies, id)
	return nil
}

// ReplaceAll 在一个事务里把当前空间的代理改成 list。
func (d *SQLiteProxyDAO) ReplaceAll(list []Proxy) error {
	sp, err := d.writable()
	if err != nil {
		return err
	}
	keep := make(map[string]bool, len(list))
	for _, p := range list {
		keep[p.ProxyId] = true
	}
	var edited []string
	err = d.tx("保存代理", func(tx *sql.Tx) error {
		ids, err := liveIDs(tx, `SELECT proxy_id FROM browser_proxies WHERE space = ? AND deleted = 0`, sp)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if !keep[id] {
				if err := tombstone(tx, "browser_proxies", "proxy_id", id, sp); err != nil {
					return err
				}
				edited = append(edited, id)
			}
		}
		for _, p := range list {
			if p.ProxyId == DirectProxyID && space.IsSynced(sp) {
				continue
			}
			changed, err := writeProxy(tx, sp, p)
			if err != nil {
				return err
			}
			if changed {
				edited = append(edited, p.ProxyId)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, id := range edited {
		d.spaces.Edited(sp, space.CollProxies, id)
	}
	return nil
}

func liveIDs(tx *sql.Tx, q string, args ...any) ([]string, error) {
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// UpdateSpeedResult 记录测速结果 (本机字段, 不标脏)。
func (d *SQLiteProxyDAO) UpdateSpeedResult(id string, ok bool, latencyMs int64, testedAt string) error {
	if _, err := d.db.Exec(`UPDATE browser_proxies SET last_latency_ms = ?, last_test_ok = ?, last_tested_at = ? WHERE proxy_id = ?`,
		latencyMs, boolInt(ok), testedAt, id); err != nil {
		return fmt.Errorf("记录测速结果失败: %w", err)
	}
	return nil
}

// UpdateIPHealthResult 记录出口检测结果 JSON (本机字段, 不标脏)。
func (d *SQLiteProxyDAO) UpdateIPHealthResult(id, healthJSON string) error {
	if _, err := d.db.Exec(`UPDATE browser_proxies SET last_ip_health_json = ? WHERE proxy_id = ?`, healthJSON, id); err != nil {
		return fmt.Errorf("记录出口检测结果失败: %w", err)
	}
	return nil
}
