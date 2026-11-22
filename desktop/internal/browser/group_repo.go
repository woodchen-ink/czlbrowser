package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// GroupDAO 存取当前空间的实例分组。分组只有一层; 删分组前由调用方把实例改挂。
type GroupDAO interface {
	List() ([]*Group, error)
	GetById(groupID string) (*Group, error)
	Create(input GroupInput) (*Group, error)
	Update(groupID string, input GroupInput) (*Group, error)
	Delete(groupID string) error
}

// GroupWire 是分组在同步文档里的内容。
type GroupWire struct {
	Name      string `json:"name"`
	ParentID  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// GroupWireOf 取分组的同步内容。
func GroupWireOf(g *Group) GroupWire {
	return GroupWire{Name: g.GroupName, ParentID: g.ParentId, SortOrder: g.SortOrder, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

// SQLiteGroupDAO 把分组存在 browser_groups 表。
type SQLiteGroupDAO struct{ spaceAware }

func NewSQLiteGroupDAO(db *sql.DB, spaces *space.State) *SQLiteGroupDAO {
	return &SQLiteGroupDAO{spaceAware{db: db, spaces: spaces}}
}

const groupSelect = `SELECT group_id, group_name, COALESCE(parent_id, ''), sort_order, created_at, updated_at, space FROM browser_groups`

func readGroup(s scanner) (*Group, error) {
	g := &Group{}
	return g, s.Scan(&g.GroupId, &g.GroupName, &g.ParentId, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt, &g.Space)
}

func (d *SQLiteGroupDAO) List() ([]*Group, error) {
	rows, err := d.db.Query(groupSelect+` WHERE space = ? AND deleted = 0 ORDER BY sort_order, created_at`, d.current())
	if err != nil {
		return nil, fmt.Errorf("读取分组失败: %w", err)
	}
	defer rows.Close()
	var out []*Group
	for rows.Next() {
		g, err := readGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("读取分组失败: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (d *SQLiteGroupDAO) GetById(id string) (*Group, error) {
	g, err := readGroup(d.db.QueryRow(groupSelect+` WHERE group_id = ? AND space = ? AND deleted = 0`, id, d.current()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("分组不存在: %s", id)
	}
	return g, err
}

func (d *SQLiteGroupDAO) Create(in GroupInput) (*Group, error) {
	if in.GroupName == "" {
		return nil, errors.New("分组名称不能为空")
	}
	sp, err := d.writable()
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339)
	g := &Group{GroupId: uuid.NewString(), Space: sp, GroupName: in.GroupName, SortOrder: in.SortOrder, CreatedAt: now, UpdatedAt: now}
	err = d.tx("创建分组", func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at, space) VALUES (?, ?, '', ?, ?, ?, ?)`,
			g.GroupId, g.GroupName, g.SortOrder, now, now, sp); err != nil {
			return err
		}
		return markIfSynced(tx, "browser_groups", "group_id", g.GroupId, sp)
	})
	if err != nil {
		return nil, err
	}
	d.spaces.Edited(sp, space.CollGroups, g.GroupId)
	return g, nil
}

func (d *SQLiteGroupDAO) Update(id string, in GroupInput) (*Group, error) {
	if in.GroupName == "" {
		return nil, errors.New("分组名称不能为空")
	}
	sp, err := d.writable()
	if err != nil {
		return nil, err
	}
	g, err := d.GetById(id)
	if err != nil {
		return nil, err
	}
	g.GroupName, g.ParentId, g.SortOrder, g.UpdatedAt = in.GroupName, "", in.SortOrder, time.Now().Format(time.RFC3339)
	err = d.tx("更新分组", func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE browser_groups SET group_name = ?, parent_id = '', sort_order = ?, updated_at = ? WHERE group_id = ? AND space = ?`,
			g.GroupName, g.SortOrder, g.UpdatedAt, id, sp); err != nil {
			return err
		}
		return markIfSynced(tx, "browser_groups", "group_id", id, sp)
	})
	if err != nil {
		return nil, err
	}
	d.spaces.Edited(sp, space.CollGroups, id)
	return g, nil
}

func (d *SQLiteGroupDAO) Delete(id string) error {
	sp, err := d.writable()
	if err != nil {
		return err
	}
	if _, err := d.GetById(id); err != nil {
		return err
	}
	if err := d.tx("删除分组", func(tx *sql.Tx) error { return tombstone(tx, "browser_groups", "group_id", id, sp) }); err != nil {
		return err
	}
	d.spaces.Edited(sp, space.CollGroups, id)
	return nil
}
