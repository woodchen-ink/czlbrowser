// Package logger 是全局日志入口: 同时写控制台、日志文件 (按大小轮转) 与一个内存环形缓冲,
// 后者给「设置 → 运行日志」页面读取。底层格式化交给 log/slog 的文本处理器。
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Field 是一条日志附带的键值。
type Field struct {
	Key   string
	Value any
}

// F 构造一个日志字段。error 值会被转成它的 Error() 文本。
func F(key string, value any) Field {
	if err, ok := value.(error); ok && err != nil {
		value = err.Error()
	}
	return Field{Key: key, Value: value}
}

// Options 控制日志的输出位置与级别。
type Options struct {
	Level      string // debug / info / warn / error, 默认 info
	File       string // 日志文件路径, 为空则不写文件
	MaxSizeMB  int    // 单个文件上限, 超过后轮转, 默认 10
	MaxBackups int    // 保留几份轮转后的旧文件, 默认 5
}

var (
	mu      sync.Mutex
	level             = new(slog.LevelVar)
	out     io.Writer = os.Stderr
	file    *rotatingFile
	handler slog.Handler = newHandler(os.Stderr)
)

func newHandler(w io.Writer) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02 15:04:05.000"))
			}
			return a
		},
	})
}

// Setup 按配置重建输出。可以重复调用 (例如重新加载配置时)。
func Setup(opts Options) error {
	mu.Lock()
	defer mu.Unlock()

	level.Set(parseLevel(opts.Level))
	if file != nil {
		_ = file.Close()
		file = nil
	}
	out = os.Stderr
	var setupErr error
	if strings.TrimSpace(opts.File) != "" {
		f, err := openRotatingFile(opts.File, opts.MaxSizeMB, opts.MaxBackups)
		if err != nil {
			setupErr = err
		} else {
			file = f
			// 文件放前面: GUI 程序没有可用的 stderr, MultiWriter 遇错即停。
			out = io.MultiWriter(f, os.Stderr)
		}
	}
	handler = newHandler(out)
	return setupErr
}

// Close 关闭日志文件。之后的日志只写控制台与内存缓冲。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return nil
	}
	err := file.Close()
	file = nil
	out = os.Stderr
	handler = newHandler(out)
	return err
}

// SetLevel 运行时调整全局级别。
func SetLevel(name string) { level.Set(parseLevel(name)) }

// LevelName 返回当前全局级别, 形如 INFO。
func LevelName() string { return level.Level().String() }

func parseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Logger 是带组件名的日志器, 用 New 创建; nil 也能安全调用。
type Logger struct {
	component string
}

// New 返回指定组件的日志器。创建代价很小, 可以随用随建。
func New(component string) *Logger { return &Logger{component: component} }

func (l *Logger) Debug(msg string, fields ...Field) { l.emit(slog.LevelDebug, msg, fields) }
func (l *Logger) Info(msg string, fields ...Field)  { l.emit(slog.LevelInfo, msg, fields) }
func (l *Logger) Warn(msg string, fields ...Field)  { l.emit(slog.LevelWarn, msg, fields) }
func (l *Logger) Error(msg string, fields ...Field) { l.emit(slog.LevelError, msg, fields) }

func (l *Logger) emit(lv slog.Level, msg string, fields []Field) {
	if lv < level.Level() {
		return
	}
	component := ""
	if l != nil {
		component = l.component
	}
	now := time.Now()
	attrs := make([]slog.Attr, 0, len(fields)+1)
	attrs = append(attrs, slog.String("component", component))
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}

	mu.Lock()
	h := handler
	mu.Unlock()
	rec := slog.NewRecord(now, lv, msg, 0)
	rec.AddAttrs(attrs...)
	_ = h.Handle(context.Background(), rec)

	remember(now, lv, component, msg, fields)
}
