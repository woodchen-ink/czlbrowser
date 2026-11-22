package browser

import (
	"encoding/json"
	"strings"
)

// NormalizeTags 去空白、去重 (不区分大小写, 保留第一次出现的写法), 永不返回 nil。
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}

func encodeTags(tags []string) string {
	b, _ := json.Marshal(NormalizeTags(tags))
	return string(b)
}

func decodeTags(raw string) []string {
	var tags []string
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &tags)
	}
	return NormalizeTags(tags)
}
