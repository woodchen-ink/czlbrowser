package logger

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// recentCap 内存里保留的最近日志条数。
const recentCap = 500

// Entry 是「运行日志」页面展示的一条日志。JSON 字段名是前端契约。
type Entry struct {
	Time      string         `json:"time"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

var recent = struct {
	sync.Mutex
	buf  [recentCap]Entry
	next int // 下一条写入位置
	full bool
}{}

func remember(t time.Time, lv slog.Level, component, msg string, fields []Field) {
	e := Entry{
		Time:      t.Format("2006-01-02 15:04:05"),
		Level:     lv.String(),
		Component: component,
		Message:   msg,
	}
	if len(fields) > 0 {
		e.Fields = make(map[string]any, len(fields))
		for _, f := range fields {
			e.Fields[f.Key] = displayValue(f.Value)
		}
	}
	recent.Lock()
	recent.buf[recent.next] = e
	recent.next = (recent.next + 1) % recentCap
	if recent.next == 0 {
		recent.full = true
	}
	recent.Unlock()
}

// displayValue 把不能直接 JSON 化的值 (如 error、自定义类型) 转成文本, 避免前端拿到 {}。
func displayValue(v any) any {
	switch x := v.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return x
	case time.Duration:
		return x.String()
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", x)
	}
}

// Recent 返回内存中的最近日志, 旧的在前。
func Recent() []Entry {
	recent.Lock()
	defer recent.Unlock()
	if !recent.full {
		out := make([]Entry, recent.next)
		copy(out, recent.buf[:recent.next])
		return out
	}
	out := make([]Entry, 0, recentCap)
	out = append(out, recent.buf[recent.next:]...)
	return append(out, recent.buf[:recent.next]...)
}

// ClearRecent 清空内存日志。
func ClearRecent() {
	recent.Lock()
	recent.next, recent.full = 0, false
	recent.Unlock()
}
