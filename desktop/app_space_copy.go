package main

// 跨空间复制: 把本机的实例、代理复制进团队或个人同步空间 (或反过来), 实现「部分给团队、部分不公开」。
// 复制不是移动, 一律换新 id; 实例引用的代理一并复制到目标空间, 分组不带 (分组属于空间)。

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/cloudsync"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// SpaceCopyResult 是一次跨空间复制的结果。
type SpaceCopyResult struct {
	Profiles int `json:"profiles"`
	Proxies  int `json:"proxies"`
	// StateErrors 登录态没复制成的实例名 (配置已复制), 界面提示用户
	StateErrors []string `json:"stateErrors"`
}

// checkTargetSpace 目标空间须是本机空间, 或本机认识、可写的同步空间。
func (a *App) checkTargetSpace(target string) error {
	if target == "" || target == space.LocalSpace {
		return nil
	}
	st := a.SyncGetState()
	for _, sp := range st.Spaces {
		if sp.Key == target {
			// 没开通 / 没解锁加密时同步空间只读, 复制进去也推不上去
			switch st.Keys {
			case cloudsync.KeysUnlocked:
			case cloudsync.KeysNotEnrolled:
				return errcode.ErrKeysNotEnrolled
			default:
				return errcode.ErrKeysLocked
			}
			if sp.ReadOnly || sp.Removed {
				return errcode.ErrSyncReadOnly
			}
			return nil
		}
	}
	return errcode.ErrSpaceUnknown
}

// SyncCopyProxies 把当前空间的代理复制到 target 空间。
func (a *App) SyncCopyProxies(proxyIDs []string, target string) (*SpaceCopyResult, error) {
	res := &SpaceCopyResult{StateErrors: []string{}}
	if err := a.prepareCopy(target); err != nil {
		return res, err
	}
	all, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return res, err
	}
	want := map[string]bool{}
	for _, id := range proxyIDs {
		want[id] = true
	}
	for _, p := range all {
		if !want[p.ProxyId] || p.ProxyId == browser.DirectProxyID {
			continue
		}
		if _, err := a.copyProxy(p, target); err != nil {
			return res, err
		}
		res.Proxies++
	}
	return res, nil
}

// SyncCopyProfiles 把当前空间的实例复制到 target 空间; withState 为 true 时连同登录态 (实例须已停止)。
func (a *App) SyncCopyProfiles(profileIDs []string, target string, withState bool) (*SpaceCopyResult, error) {
	res := &SpaceCopyResult{StateErrors: []string{}}
	if err := a.prepareCopy(target); err != nil {
		return res, err
	}
	target = normalizeSpace(target)
	proxies, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return res, err
	}
	proxyByID := map[string]browser.Proxy{}
	for _, p := range proxies {
		proxyByID[p.ProxyId] = p
	}
	copied := map[string]string{} // 源代理 id → 目标空间里的新 id

	for _, id := range profileIDs {
		a.browserMgr.Mutex.Lock()
		src, ok := a.browserMgr.Profiles[id]
		var cp browser.Profile
		if ok {
			cp = *src
		}
		a.browserMgr.Mutex.Unlock()
		if !ok {
			continue
		}
		if cp.Running && withState {
			return res, errcode.ErrProfileRunning
		}
		newID := uuid.NewString()
		now := time.Now().Format(time.RFC3339)
		dst := &browser.Profile{
			ProfileId: newID, Space: target, ProfileName: cp.ProfileName, UserDataDir: newID, CoreId: cp.CoreId,
			FingerprintArgs: append([]string{}, cp.FingerprintArgs...), ProxyConfig: cp.ProxyConfig,
			LaunchArgs: append([]string{}, cp.LaunchArgs...), Tags: append([]string{}, cp.Tags...),
			Keywords: append([]string{}, cp.Keywords...), CreatedAt: now, UpdatedAt: now,
		}
		if pid := strings.TrimSpace(cp.ProxyId); pid != "" {
			if pid == browser.DirectProxyID {
				dst.ProxyId = pid
			} else if nid, ok := copied[pid]; ok {
				dst.ProxyId = nid
			} else if p, ok := proxyByID[pid]; ok {
				nid, err := a.copyProxy(p, target)
				if err != nil {
					return res, err
				}
				copied[pid], dst.ProxyId = nid, nid
				res.Proxies++
			}
		}
		if err := a.browserMgr.ProfileDAO.Upsert(dst); err != nil {
			return res, err
		}
		a.browserMgr.Mutex.Lock()
		a.browserMgr.Profiles[newID] = dst
		a.browserMgr.Mutex.Unlock()
		res.Profiles++

		if withState && a.cloud != nil {
			srcDir := a.browserMgr.ResolveUserDataDir(&cp)
			dstDir := a.browserMgr.ResolveUserDataDir(dst)
			if err := a.cloud.CopyState(srcDir, newID, dstDir, target); err != nil {
				logCloud("sync.copy_state", err)
				res.StateErrors = append(res.StateErrors, cp.ProfileName)
			}
		}
	}
	return res, nil
}

func (a *App) prepareCopy(target string) error {
	target = normalizeSpace(target)
	if target == a.spaces.Current() {
		return errcode.ErrSpaceSame
	}
	return a.checkTargetSpace(target)
}

func (a *App) copyProxy(p browser.Proxy, target string) (string, error) {
	dao, ok := a.browserMgr.ProxyDAO.(*browser.SQLiteProxyDAO)
	if !ok {
		return "", errcode.ErrDBQuery
	}
	p.ProxyId = uuid.NewString()
	p.SourceID = ""
	if err := dao.UpsertIn(normalizeSpace(target), p); err != nil {
		return "", err
	}
	return p.ProxyId, nil
}

func normalizeSpace(s string) string {
	if strings.TrimSpace(s) == "" {
		return space.LocalSpace
	}
	return s
}
