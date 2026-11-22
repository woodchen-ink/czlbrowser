package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// SettingBookmarks 是同步空间里默认书签的设置键 (settings 集合的文档 id)。
const SettingBookmarks = "bookmarks"

// BookmarkDAO 存取当前空间的默认书签 (新实例首次启动时写进书签栏)。
// 本机空间用 browser_bookmarks 表; 同步空间整份 JSON 存在 space_settings 的 bookmarks 键, 随 settings 集合同步。
type BookmarkDAO interface {
	List() ([]config.BrowserBookmark, error)
	ReplaceAll(bookmarks []config.BrowserBookmark) error
}

type SQLiteBookmarkDAO struct{ spaceAware }

func NewSQLiteBookmarkDAO(db *sql.DB, spaces *space.State) *SQLiteBookmarkDAO {
	return &SQLiteBookmarkDAO{spaceAware{db: db, spaces: spaces}}
}

func (d *SQLiteBookmarkDAO) List() ([]config.BrowserBookmark, error) {
	sp := d.current()
	if space.IsSynced(sp) {
		var raw string
		err := d.db.QueryRow(`SELECT value FROM space_settings WHERE space = ? AND key = ? AND deleted = 0`, sp, SettingBookmarks).Scan(&raw)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("读取书签失败: %w", err)
		}
		list := []config.BrowserBookmark{}
		_ = json.Unmarshal([]byte(raw), &list)
		return list, nil
	}

	rows, err := d.db.Query(`SELECT name, url, COALESCE(tags, '[]') FROM browser_bookmarks ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("读取书签失败: %w", err)
	}
	defer rows.Close()
	var out []config.BrowserBookmark
	for rows.Next() {
		var b config.BrowserBookmark
		var tags string
		if err := rows.Scan(&b.Name, &b.URL, &tags); err != nil {
			return nil, fmt.Errorf("读取书签失败: %w", err)
		}
		b.Tags = decodeTags(tags)
		out = append(out, b)
	}
	return out, rows.Err()
}

// ReplaceAll 整体替换当前空间的书签, 缺名称或网址的条目跳过。团队空间只有 owner / admin 能改。
func (d *SQLiteBookmarkDAO) ReplaceAll(list []config.BrowserBookmark) error {
	clean := make([]config.BrowserBookmark, 0, len(list))
	for _, b := range list {
		if b.Name != "" && b.URL != "" {
			clean = append(clean, b)
		}
	}
	if space.IsSynced(d.current()) {
		return d.replaceSynced(clean)
	}
	return d.tx("保存书签", func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM browser_bookmarks`); err != nil {
			return err
		}
		for i, b := range clean {
			if _, err := tx.Exec(`INSERT INTO browser_bookmarks (name, url, sort_order, tags) VALUES (?, ?, ?, ?)`,
				b.Name, b.URL, i, encodeTags(b.Tags)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *SQLiteBookmarkDAO) replaceSynced(list []config.BrowserBookmark) error {
	sp, err := d.spaces.SettingsWritable()
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Tags = NormalizeTags(list[i].Tags)
		if len(list[i].Tags) == 0 {
			list[i].Tags = nil // 没有标签时不写字段, 与旧版本的文档一致
		}
	}
	value, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("保存书签失败: %w", err)
	}
	unchanged := false
	err = d.tx("保存书签", func(tx *sql.Tx) error {
		var old string
		var deleted bool
		err := tx.QueryRow(`SELECT value, deleted FROM space_settings WHERE space = ? AND key = ?`, sp, SettingBookmarks).Scan(&old, &deleted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && !deleted && old == string(value) {
			unchanged = true
			return nil
		}
		seq, err := space.NextSeq(tx)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO space_settings (space, key, value, dirty, deleted, updated_at) VALUES (?, ?, ?, ?, 0, ?)
			ON CONFLICT(space, key) DO UPDATE SET value = excluded.value, dirty = excluded.dirty, deleted = 0, updated_at = excluded.updated_at`,
			sp, SettingBookmarks, string(value), seq, time.Now().UTC().Format(time.RFC3339))
		return err
	})
	if err == nil && !unchanged {
		d.spaces.Edited(sp, space.CollSettings, SettingBookmarks)
	}
	return err
}
