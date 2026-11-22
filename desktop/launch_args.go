package main

import (
	"encoding/json"
	"strings"
	"sync"
)

// upsertKeyValueArg 把 key=value 写进启动参数: 已有的第一处 (key=x 或 "key x" 两段式) 改成新值,
// 其余同名参数删掉; 没有则追加。参数前后空白会被去掉。
func upsertKeyValueArg(args []string, key, value string) []string {
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	if key == "" || value == "" {
		return args
	}
	want := key + "=" + value
	out := make([]string, 0, len(args)+1)
	done := false
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		name, _, hasValue := strings.Cut(arg, "=")
		if !strings.EqualFold(name, key) {
			out = append(out, arg)
			continue
		}
		if !hasValue && i+1 < len(args) && !strings.HasPrefix(strings.TrimSpace(args[i+1]), "--") {
			i++ // 两段式的值一并去掉
		}
		if !done {
			out, done = append(out, want), true
		}
	}
	if !done {
		out = append(out, want)
	}
	return out
}

// extractSwitchValueArg 从参数里拿掉所有 key (两种写法), 返回剩下的参数和最后一个非空值。
func extractSwitchValueArg(args []string, key string) ([]string, string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return args, ""
	}
	out := make([]string, 0, len(args))
	value := ""
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		name, v, hasValue := strings.Cut(arg, "=")
		if !strings.EqualFold(name, key) {
			out = append(out, arg)
			continue
		}
		if !hasValue && i+1 < len(args) {
			if next := strings.TrimSpace(args[i+1]); next != "" && !strings.HasPrefix(next, "--") {
				v = next
				i++
			}
		}
		if v = strings.TrimSpace(v); v != "" {
			value = v
		}
	}
	return out, value
}

// normalizeNonEmptyStrings 去掉空白项与首尾空白。
func normalizeNonEmptyStrings(items []string) []string {
	var out []string
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// fingerprintSeed 是实例没有手动指定 --fingerprint 时的默认种子, 由实例 id 推出,
// 保证同一实例每次启动指纹一致。算法不能改, 否则已有实例的指纹会变。
func fingerprintSeed(profileID string) int {
	seed := 0
	for _, r := range profileID {
		seed = seed*31 + int(r)
	}
	if seed < 0 {
		seed = -seed
	}
	return seed
}

// languageByCountry 是按代理出口国家设置的浏览器语言, 其余国家用 en-US。
var languageByCountry = map[string]string{
	"DE": "de-DE", "AT": "de-DE", "CH": "de-DE",
	"FR": "fr-FR", "ES": "es-ES", "IT": "it-IT",
	"JP": "ja-JP", "KR": "ko-KR", "RU": "ru-RU",
	"CN": "zh-CN", "TW": "zh-TW", "HK": "zh-HK",
	"GB": "en-GB",
}

func defaultLanguageByCountryCode(cc string) string {
	if lang, ok := languageByCountry[strings.ToUpper(strings.TrimSpace(cc))]; ok {
		return lang
	}
	return "en-US"
}

// parseProxyLocaleFromHealthJSON 从保存的出口检测结果里取时区与国家代码 (依次看顶层、rawData、data)。
func parseProxyLocaleFromHealthJSON(raw string) (timezone, countryCode string, ok bool) {
	var doc map[string]interface{}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &doc) != nil {
		return "", "", false
	}
	layers := []map[string]interface{}{doc}
	for _, k := range []string{"rawData", "data"} {
		if m, isMap := doc[k].(map[string]interface{}); isMap {
			layers = append(layers, m)
		}
	}
	for _, m := range layers {
		if timezone == "" {
			timezone = strings.TrimSpace(mapString(m, "timezone"))
		}
		for _, k := range []string{"countryCode", "country_code"} {
			if countryCode == "" {
				countryCode = strings.TrimSpace(mapString(m, k))
			}
		}
	}
	return timezone, strings.ToUpper(countryCode), timezone != ""
}

// cappedLogBuffer 只保留最后 max 字节的输出 (用来在启动失败时附上内核的报错)。
type cappedLogBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newCappedLogBuffer(max int) *cappedLogBuffer { return &cappedLogBuffer{max: max} }

func (b *cappedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		b.buf = append(b.buf[:0], b.buf[over:]...)
	}
	return len(p), nil
}

func (b *cappedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// summarizeProcessOutput 取输出最后 maxLines 个非空行, 用 " | " 连接, 超过 maxChars 时保留末尾。
func summarizeProcessOutput(raw string, maxLines, maxChars int) string {
	raw = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(raw)
	lines := normalizeNonEmptyStrings(strings.Split(raw, "\n"))
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	s := strings.Join(lines, " | ")
	if maxChars > 0 && len(s) > maxChars {
		s = s[len(s)-maxChars:]
	}
	return s
}
