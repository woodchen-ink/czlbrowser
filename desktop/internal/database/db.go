// Package database 打开本机 SQLite 数据库并按版本执行表结构迁移。
package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// DB 包装唯一的数据库连接。SQLite 只允许一个写者, 连接池固定为 1。
type DB struct {
	conn *sql.DB
}

// NewDB 打开数据库文件, 启用 WAL 与外键约束。
func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON"} {
		if _, err := conn.Exec(pragma); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("初始化数据库失败 (%s): %w", pragma, err)
		}
	}
	return &DB{conn: conn}, nil
}

// GetConn 返回底层连接。
func (db *DB) GetConn() *sql.DB { return db.conn }

// Close 关闭连接。
func (db *DB) Close() error {
	if db == nil || db.conn == nil {
		return nil
	}
	return db.conn.Close()
}

// Migrate 依次执行尚未执行过的迁移。每个版本一个事务, 执行记录在 schema_migrations。
func (db *DB) Migrate() error {
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, desc TEXT NOT NULL DEFAULT '', applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}
	var done int
	if err := db.conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&done); err != nil {
		return fmt.Errorf("读取数据库版本失败: %w", err)
	}
	for _, s := range steps {
		if s.version <= done {
			continue
		}
		if err := db.apply(s); err != nil {
			return fmt.Errorf("数据库迁移 v%d (%s) 失败: %w", s.version, s.desc, err)
		}
	}
	return nil
}

func (db *DB) apply(s step) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range s.sql {
		if _, err := tx.Exec(q); err != nil {
			// 早期版本没有迁移记录, 列或表可能已经存在
			if msg := err.Error(); strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists") {
				continue
			}
			head := strings.Join(strings.Fields(q), " ")
			if len(head) > 60 {
				head = head[:60] + "..."
			}
			return fmt.Errorf("[%s]: %w", head, err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, desc) VALUES (?, ?)`, s.version, s.desc); err != nil {
		return err
	}
	return tx.Commit()
}
