package main

import (
	"fmt"
	"strings"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
)

// 统一分组与标签: 实例 (分组 + 标签)、代理 (分组 + 标签)、内核与书签 (标签)。
// 实例分组是同步的分组表 (单层); 代理分组是代理上的分组名; 标签都是各自记录上的字符串数组。
// 这里放按名字批量改 / 删的操作, 设置页「分组与标签」与各列表的筛选菜单共用。

// renameIn 把 tags 里等于 oldName 的 (不区分大小写) 换成 newName, newName 为空表示删除。返回是否有变化。
func renameIn(tags []string, oldName, newName string) ([]string, bool) {
	changed := false
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if strings.EqualFold(strings.TrimSpace(t), oldName) {
			changed = true
			if newName != "" {
				out = append(out, newName)
			}
			continue
		}
		out = append(out, t)
	}
	return browser.NormalizeTags(out), changed
}

func cleanTaxonomyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("名称不能为空")
	}
	if len([]rune(name)) > 32 {
		return "", fmt.Errorf("名称最长 32 个字")
	}
	return name, nil
}

// TaxonomyRenameTag 重命名某类记录上的标签。kind: profile / proxy / core / bookmark
func (a *App) TaxonomyRenameTag(kind, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	nn, err := cleanTaxonomyName(newName)
	if err != nil {
		return err
	}
	return a.rewriteTag(kind, oldName, nn)
}

// TaxonomyDeleteTag 从某类记录上去掉一个标签 (记录本身保留)。
func (a *App) TaxonomyDeleteTag(kind, name string) error {
	return a.rewriteTag(kind, strings.TrimSpace(name), "")
}

func (a *App) rewriteTag(kind, oldName, newName string) error {
	if oldName == "" {
		return fmt.Errorf("标签不能为空")
	}
	switch kind {
	case "profile":
		if newName == "" {
			ids := []string{}
			for _, p := range a.browserMgr.ListByTag(oldName) {
				ids = append(ids, p.ProfileId)
			}
			if len(ids) == 0 {
				return nil
			}
			return a.removeProfileTags(ids, []string{oldName})
		}
		return a.renameProfileTag(oldName, newName)
	case "proxy":
		list := a.getLatestProxies()
		changed := false
		for i := range list {
			if tags, ok := renameIn(list[i].Tags, oldName, newName); ok {
				list[i].Tags = tags
				changed = true
			}
		}
		if !changed {
			return nil
		}
		return a.SaveBrowserProxies(withoutVirtualDirect(list))
	case "core":
		for _, c := range a.browserMgr.ListCores() {
			if tags, ok := renameIn(c.Tags, oldName, newName); ok {
				if err := a.BrowserCoreSetTags(c.CoreId, tags); err != nil {
					return err
				}
			}
		}
		return nil
	case "bookmark":
		list := a.BookmarkList()
		changed := false
		for i := range list {
			if tags, ok := renameIn(list[i].Tags, oldName, newName); ok {
				list[i].Tags = tags
				changed = true
			}
		}
		if !changed {
			return nil
		}
		return a.BookmarkSave(list)
	}
	return fmt.Errorf("未知类型: %s", kind)
}

// TaxonomyRenameProxyGroup 重命名代理分组; newName 为空表示删除分组 (代理移到未分组)。
func (a *App) TaxonomyRenameProxyGroup(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	if oldName == "" {
		return fmt.Errorf("分组不能为空")
	}
	if newName != "" {
		nn, err := cleanTaxonomyName(newName)
		if err != nil {
			return err
		}
		newName = nn
	}
	list := a.getLatestProxies()
	changed := false
	for i := range list {
		if strings.EqualFold(strings.TrimSpace(list[i].GroupName), oldName) {
			list[i].GroupName = newName
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return a.SaveBrowserProxies(withoutVirtualDirect(list))
}

// BrowserCoreSetTags 设置内核标签 (内核只在本机, 不同步)。
func (a *App) BrowserCoreSetTags(coreId string, tags []string) error {
	return a.browserMgr.SetCoreTags(coreId, browser.NormalizeTags(tags))
}

// withoutVirtualDirect 去掉同步空间列表里补上的虚拟直连, 避免整表保存时写回。
func withoutVirtualDirect(list []BrowserProxy) []BrowserProxy {
	out := make([]BrowserProxy, 0, len(list))
	for _, p := range list {
		if p.ProxyId == browser.DirectProxyID {
			continue
		}
		out = append(out, p)
	}
	return out
}
