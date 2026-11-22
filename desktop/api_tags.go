package main

import (
	"errors"
	"strings"
	"time"
)

// removeProfileTags 从多个实例上去掉指定标签。
func (a *App) removeProfileTags(ids, tags []string) error {
	return a.editProfiles(ids, func(p *BrowserProfile) bool {
		kept := make([]string, 0, len(p.Tags))
		for _, t := range p.Tags {
			if !containsString(tags, t) {
				kept = append(kept, t)
			}
		}
		changed := len(kept) != len(p.Tags)
		p.Tags = kept
		return changed
	})
}

// renameProfileTag 把当前空间实例上的标签 old (不区分大小写) 改名为 new, 改名后重复的合并。
func (a *App) renameProfileTag(oldName, newName string) error {
	oldName, newName = strings.TrimSpace(oldName), strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return errors.New("标签名称不能为空")
	}
	a.browserMgr.Mutex.Lock()
	cur := a.browserMgr.CurrentSpace()
	var ids []string
	for id, p := range a.browserMgr.Profiles {
		sp := p.Space
		if sp == "" {
			sp = "local"
		}
		if sp == cur {
			ids = append(ids, id)
		}
	}
	a.browserMgr.Mutex.Unlock()
	return a.editProfiles(ids, func(p *BrowserProfile) bool {
		renamed, out := false, make([]string, 0, len(p.Tags))
		for _, t := range p.Tags {
			if strings.EqualFold(t, oldName) {
				t, renamed = newName, true
			}
			if !containsString(out, t) {
				out = append(out, t)
			}
		}
		if renamed {
			p.Tags = out
		}
		return renamed
	})
}

// editProfiles 对每个存在的实例执行 edit, 有改动的更新时间并保存。遇到保存失败立即返回。
func (a *App) editProfiles(ids []string, edit func(*BrowserProfile) bool) error {
	m := a.browserMgr
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	now := time.Now().Format(time.RFC3339)
	for _, id := range ids {
		p, ok := m.Profiles[id]
		if !ok || !edit(p) {
			continue
		}
		p.UpdatedAt = now
		if m.ProfileDAO != nil {
			if err := m.ProfileDAO.Upsert(p); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
