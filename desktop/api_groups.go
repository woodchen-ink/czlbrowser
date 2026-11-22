package main

import (
	"errors"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
)

type (
	BrowserGroup          = browser.Group
	BrowserGroupInput     = browser.GroupInput
	BrowserGroupWithCount = browser.GroupWithCount
)

var errGroupsUnavailable = errors.New("分组数据未初始化")

// ListGroups 返回当前空间的分组及各组实例数。
func (a *App) ListGroups() []BrowserGroupWithCount {
	out := []BrowserGroupWithCount{}
	if a.browserMgr.GroupDAO == nil {
		return out
	}
	groups, err := a.browserMgr.GroupDAO.List()
	if err != nil {
		return out
	}
	count := map[string]int{}
	for _, p := range a.browserMgr.List() {
		count[p.GroupId]++
	}
	for _, g := range groups {
		out = append(out, BrowserGroupWithCount{Group: *g, InstanceCount: count[g.GroupId]})
	}
	return out
}

// CreateGroup 新建分组 (分组只有一层)。
func (a *App) CreateGroup(in BrowserGroupInput) (*BrowserGroup, error) {
	if a.browserMgr.GroupDAO == nil {
		return nil, errGroupsUnavailable
	}
	in.ParentId = ""
	return a.browserMgr.GroupDAO.Create(in)
}

// UpdateGroup 修改分组名称或排序。
func (a *App) UpdateGroup(id string, in BrowserGroupInput) (*BrowserGroup, error) {
	if a.browserMgr.GroupDAO == nil {
		return nil, errGroupsUnavailable
	}
	in.ParentId = ""
	return a.browserMgr.GroupDAO.Update(id, in)
}

// DeleteGroup 删除分组, 组内实例变为未分组。
func (a *App) DeleteGroup(id string) error {
	if a.browserMgr.GroupDAO == nil {
		return errGroupsUnavailable
	}
	if _, err := a.browserMgr.GroupDAO.GetById(id); err != nil {
		return err
	}
	if err := a.browserMgr.MoveProfilesToGroup(a.browserMgr.ProfileIDsInGroup(id), ""); err != nil {
		return err
	}
	return a.browserMgr.GroupDAO.Delete(id)
}

// MoveInstancesToGroup 把实例移到分组, 空串表示未分组。
func (a *App) MoveInstancesToGroup(profileIDs []string, groupID string) error {
	return a.browserMgr.MoveProfilesToGroup(profileIDs, groupID)
}
