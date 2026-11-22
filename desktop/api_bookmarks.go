package main

import (
	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
)

type BrowserBookmark = config.BrowserBookmark

// BookmarkList 返回当前空间的默认书签; 还没有时返回内置的几条。
func (a *App) BookmarkList() []BrowserBookmark {
	if list, err := a.browserMgr.BookmarkDAO.List(); err == nil && len(list) > 0 {
		return list
	}
	if len(a.config.Browser.DefaultBookmarks) > 0 {
		return append([]BrowserBookmark{}, a.config.Browser.DefaultBookmarks...)
	}
	return append([]BrowserBookmark{}, defaultBookmarks...)
}

// BookmarkSave 保存默认书签: 去掉缺名称或网址的, 同一网址 (忽略大小写与末尾斜杠) 只留第一条。
func (a *App) BookmarkSave(items []BrowserBookmark) error {
	seen := map[string]bool{}
	keep := make([]BrowserBookmark, 0, len(items))
	for _, b := range items {
		key := browser.NormalizeBookmarkURL(b.URL)
		if b.Name == "" || key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keep = append(keep, b)
	}
	return a.browserMgr.BookmarkDAO.ReplaceAll(keep)
}

// BookmarkReset 恢复内置默认书签。
func (a *App) BookmarkReset() error {
	return a.BookmarkSave(append([]BrowserBookmark{}, defaultBookmarks...))
}
