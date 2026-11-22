package browser

import (
	"database/sql"
	"fmt"
	"time"
)

// CoreDAO 存取内核登记。
type CoreDAO interface {
	List() ([]Core, error)
	Upsert(core Core) error
	Delete(coreID string) error
	SetDefault(coreID string) error
}

// SQLiteCoreDAO 把内核存在 browser_cores 表 (本机数据, 不同步)。
type SQLiteCoreDAO struct{ db *sql.DB }

func NewSQLiteCoreDAO(db *sql.DB) *SQLiteCoreDAO { return &SQLiteCoreDAO{db: db} }

func (d *SQLiteCoreDAO) List() ([]Core, error) {
	rows, err := d.db.Query(`SELECT core_id, core_name, core_path, is_default = 1, COALESCE(tags, '[]') FROM browser_cores ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, fmt.Errorf("读取内核列表失败: %w", err)
	}
	defer rows.Close()
	var out []Core
	for rows.Next() {
		var c Core
		var tags string
		if err := rows.Scan(&c.CoreId, &c.CoreName, &c.CorePath, &c.IsDefault, &tags); err != nil {
			return nil, fmt.Errorf("读取内核列表失败: %w", err)
		}
		c.Tags = decodeTags(tags)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *SQLiteCoreDAO) Upsert(c Core) error {
	_, err := d.db.Exec(`INSERT INTO browser_cores (core_id, core_name, core_path, is_default, created_at, tags) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(core_id) DO UPDATE SET core_name = excluded.core_name, core_path = excluded.core_path,
		is_default = excluded.is_default, tags = excluded.tags`,
		c.CoreId, c.CoreName, c.CorePath, boolInt(c.IsDefault), time.Now().Format(time.RFC3339), encodeTags(c.Tags))
	if err != nil {
		return fmt.Errorf("保存内核失败: %w", err)
	}
	return nil
}

func (d *SQLiteCoreDAO) Delete(id string) error {
	if _, err := d.db.Exec(`DELETE FROM browser_cores WHERE core_id = ?`, id); err != nil {
		return fmt.Errorf("删除内核失败: %w", err)
	}
	return nil
}

// SetDefault 只保留一个默认内核; id 为空时清空默认标记。
func (d *SQLiteCoreDAO) SetDefault(id string) error {
	if _, err := d.db.Exec(`UPDATE browser_cores SET is_default = CASE WHEN core_id = ? THEN 1 ELSE 0 END`, id); err != nil {
		return fmt.Errorf("设置默认内核失败: %w", err)
	}
	return nil
}
