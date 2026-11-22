package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// scanner 是 *sql.Row 与 *sql.Rows 的共同接口。
type scanner interface {
	Scan(dest ...any) error
}

// spaceAware 是按当前空间读写的 DAO 共用部分。spaces 为 nil 时一律视为本机空间 (测试)。
type spaceAware struct {
	db     *sql.DB
	spaces *space.State
}

func (s spaceAware) current() string {
	if s.spaces == nil {
		return space.LocalSpace
	}
	return s.spaces.Current()
}

// writable 返回当前空间; 只读时返回 40905。
func (s spaceAware) writable() (string, error) {
	if s.spaces == nil {
		return space.LocalSpace, nil
	}
	return s.spaces.Writable()
}

// tx 在事务里执行 fn, fn 出错或提交失败都回滚; 错误加上 what 前缀。
func (s spaceAware) tx(what string, fn func(*sql.Tx) error) error {
	t, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("%s失败: %w", what, err)
	}
	defer t.Rollback()
	if err := fn(t); err != nil {
		var coded *errcode.Error
		if errors.As(err, &coded) {
			return err // 带错误码的业务错误原样返回, 前端按码提示
		}
		return fmt.Errorf("%s失败: %w", what, err)
	}
	if err := t.Commit(); err != nil {
		return fmt.Errorf("%s失败: %w", what, err)
	}
	return nil
}

// markIfSynced 同步空间里给这一行分配新的出站序号, 等待上传。
func markIfSynced(tx *sql.Tx, table, keyCol, id, spaceID string) error {
	if !space.IsSynced(spaceID) {
		return nil
	}
	return space.MarkDirty(tx, table, keyCol, id, spaceID)
}

// tombstone 删除一行: 本机空间直接删; 同步空间改成墓碑并标脏 (即使是还没确认的新建,
// 那次推送可能已到服务端, 直接删会被下次拉取写回), 推送确认后由同步层删掉墓碑。
func tombstone(tx *sql.Tx, table, keyCol, id, spaceID string) error {
	if !space.IsSynced(spaceID) {
		_, err := tx.Exec(`DELETE FROM `+table+` WHERE `+keyCol+` = ? AND space = ?`, id, spaceID)
		return err
	}
	if _, err := tx.Exec(`UPDATE `+table+` SET deleted = 1 WHERE `+keyCol+` = ? AND space = ?`, id, spaceID); err != nil {
		return err
	}
	return space.MarkDirty(tx, table, keyCol, id, spaceID)
}

// sameJSON 判断两个值序列化后是否相同, 用来决定同步字段有没有变化。
func sameJSON(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ja) == string(jb)
}

// jsonStrings 解析数据库里的 JSON 字符串数组, 无效或为空时返回空切片 (前端不接受 null)。
func jsonStrings(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = []string{}
	}
	return out
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
