package browser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

// Chromium 书签文件里的时间是自 1601-01-01 UTC 起的微秒数。
func chromiumNow() string {
	return strconv.FormatInt(time.Since(time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)).Microseconds(), 10)
}

type node = map[string]interface{}

// EnsureDefaultBookmarks 把默认书签补进实例的书签栏 (Default/Bookmarks)。
// 已有相同网址 (任意文件夹里) 的不再添加, 用户自己的书签不动。
func EnsureDefaultBookmarks(userDataDir string, defaults []config.BrowserBookmark) error {
	if len(defaults) == 0 {
		return nil
	}
	dir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 profile 目录失败: %w", err)
	}
	file := filepath.Join(dir, "Bookmarks")
	now := chromiumNow()

	doc := node{}
	if raw, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(raw, &doc)
	}
	roots, _ := doc["roots"].(node)
	if roots == nil {
		roots = node{}
		doc["roots"], doc["version"] = roots, 1
	}
	bar, _ := roots["bookmark_bar"].(node)
	if bar == nil {
		bar = permanentFolder("1", "0bc5d13f-2cba-5d74-951f-3f233fe6c908", "书签栏", now)
		roots["bookmark_bar"] = bar
		if roots["other"] == nil {
			roots["other"] = permanentFolder("2", "82b081ec-3dd3-529c-8475-ab6c344590dd", "其他书签", now)
		}
		if roots["synced"] == nil {
			roots["synced"] = permanentFolder("3", "4cf2e351-0e85-532b-bb37-df045d8f8d0f", "移动设备书签", now)
		}
	}

	have := map[string]bool{}
	nextID := 0
	for _, r := range roots {
		if folder, ok := r.(node); ok {
			walkBookmarks(folder, func(n node) {
				if id, err := strconv.Atoi(fmt.Sprint(n["id"])); err == nil && id > nextID {
					nextID = id
				}
				if n["type"] == "url" {
					if u, ok := n["url"].(string); ok && NormalizeBookmarkURL(u) != "" {
						have[NormalizeBookmarkURL(u)] = true
					}
				}
			})
		}
	}

	children, _ := bar["children"].([]interface{})
	added := false
	for _, b := range defaults {
		key := NormalizeBookmarkURL(b.URL)
		if key != "" && have[key] {
			continue
		}
		have[key] = true
		nextID++
		children = append(children, node{
			"type": "url", "id": strconv.Itoa(nextID), "name": b.Name, "url": b.URL,
			"guid":       uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String(),
			"date_added": now, "date_last_used": "0",
		})
		added = true
	}
	if !added {
		return nil
	}
	bar["children"], bar["date_modified"] = children, now
	// 内容变了, 旧校验和不再成立; 留空时 Chromium 会重新计算
	doc["checksum"] = ""
	out, err := json.MarshalIndent(doc, "", "   ")
	if err != nil {
		return fmt.Errorf("序列化书签失败: %w", err)
	}
	return os.WriteFile(file, out, 0o644)
}

func permanentFolder(id, guid, name, now string) node {
	return node{
		"type": "folder", "id": id, "guid": guid, "name": name, "children": []interface{}{},
		"date_added": now, "date_last_used": "0", "date_modified": "0",
	}
}

func walkBookmarks(n node, visit func(node)) {
	visit(n)
	kids, _ := n["children"].([]interface{})
	for _, k := range kids {
		if child, ok := k.(node); ok {
			walkBookmarks(child, visit)
		}
	}
}

// NormalizeBookmarkURL 用于书签判重: 协议与主机名转小写, 去掉只有 "/" 的路径。
func NormalizeBookmarkURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}
	u.Scheme, u.Host = strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	if u.Path == "/" && u.RawPath == "" {
		u.Path = ""
	}
	return u.String()
}
