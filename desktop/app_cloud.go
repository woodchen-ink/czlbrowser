package main

// 云端同步绑定 (CZL Connect 数据同步, 端到端加密)。同步、团队、付费的设计见 docs/cloud-sync.md。
// 空间: 本机 local / 个人同步 me:<用户> / 团队 org:<组织>; 实例、代理、分组、书签按空间存, 列表按当前空间过滤。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/woodchen-ink/go-web-utils/syncsdk"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/billing"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/cloudsync"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/orgs"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/syncstore"
)

// Wails 绑定类型别名。
type SyncState = cloudsync.State
type SyncSpaceInfo = cloudsync.SpaceInfo
type SyncDeviceRequest = cloudsync.DeviceRequestInfo
type SyncPresence = syncsdk.Presence

// initCloud 组装同步、团队、付费客户端 (DAO 注入之后调用)。已登录时开始同步。
func (a *App) initCloud() {
	a.syncStore = syncstore.New(a.db.GetConn())
	a.cloud = cloudsync.New(cloudsync.Options{
		Store:      a.syncStore,
		Spaces:     a.spaces,
		Tokens:     a.authMgr,
		DataDir:    a.resolveAppPath(filepath.Join("data", "sync")),
		DeviceName: deviceName(),
		UserDataDir: func(profileID string) string {
			return a.browserMgr.ResolveUserDataDir(&browser.Profile{ProfileId: profileID, UserDataDir: profileID})
		},
		IsRunning: func(profileID string) bool {
			a.browserMgr.Mutex.Lock()
			defer a.browserMgr.Mutex.Unlock()
			p, ok := a.browserMgr.Profiles[profileID]
			return ok && p.Running
		},
		Emit:      a.emit,
		OnApplied: a.onSyncApplied,
		Subscribe: true,
	})
	a.orgs = orgs.New("", a.authMgr, nil)
	a.billing = billing.New("", a.authMgr, nil)
	a.startCloud()
}

// startCloud 已登录且知道用户 id 时开始同步 (启动恢复会话、登录成功、后台补全用户信息后调用, 重复调用无副作用)。
func (a *App) startCloud() {
	if a.cloud == nil || a.authMgr == nil {
		return
	}
	uid := a.authMgr.UserID()
	if uid == "" {
		return
	}
	if err := a.cloud.Start(uid); err != nil {
		logger.New("Sync").Error("启动同步失败", logger.F("error", err))
	}
}

func (a *App) emit(event string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, event, data)
	}
}

// onSyncApplied 拉到别处的修改后重载内存里的实例 (分组、代理、书签每次都从库里读, 不用重载)。
func (a *App) onSyncApplied(_ string, collections []string) {
	for _, c := range collections {
		if c == space.CollProfiles {
			if err := a.browserMgr.ReloadProfiles(); err != nil {
				logger.New("Sync").Error("重载实例失败", logger.F("error", err))
			}
			return
		}
	}
}

func deviceName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "未命名电脑"
	}
	return "Windows · " + host
}

func (a *App) cloudCtx(d time.Duration) (context.Context, context.CancelFunc) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, d)
}

func logCloud(op string, err error) {
	if err == nil {
		return
	}
	var ec *errcode.Error
	if errors.As(err, &ec) && ec.Cause() != nil {
		logger.New("Cloud").Error(op, logger.F("error", err.Error()), logger.F("cause", ec.Cause().Error()))
		return
	}
	logger.New("Cloud").Error(op, logger.F("error", err.Error()))
}

// ─── 状态与空间 ────────────────────────────────────────────────────────────

// SyncGetState 返回同步状态 (不联网)。
func (a *App) SyncGetState() SyncState {
	if a.cloud == nil {
		return SyncState{Current: space.LocalSpace}
	}
	return a.cloud.State()
}

// SyncRefresh 联网核对密钥状态、刷新可访问的空间 (打开同步设置、加入团队、付款到账后)。
func (a *App) SyncRefresh() SyncState {
	if a.cloud == nil {
		return a.SyncGetState()
	}
	a.startCloud()
	ctx, cancel := a.cloudCtx(30 * time.Second)
	defer cancel()
	return a.cloud.RecheckKeys(ctx)
}

// SyncSwitchSpace 切换当前空间 (local / me:<用户> / org:<组织>)。
func (a *App) SyncSwitchSpace(key string) (SyncState, error) {
	if a.cloud == nil {
		return a.SyncGetState(), errcode.ErrAuthRequired
	}
	err := a.cloud.SwitchSpace(key)
	logCloud("sync.switch", err)
	return a.cloud.State(), err
}

// SyncNow 立即同步一个空间。
func (a *App) SyncNow(key string) error {
	if a.cloud == nil {
		return errcode.ErrAuthRequired
	}
	ctx, cancel := a.cloudCtx(2 * time.Minute)
	defer cancel()
	err := a.cloud.SyncNow(ctx, key)
	logCloud("sync.now", err)
	return err
}

// SyncPresence 空间里他人正在使用的实例。
func (a *App) SyncPresence(key string) []SyncPresence {
	if a.cloud == nil {
		return []SyncPresence{}
	}
	return a.cloud.Presence(key)
}

// SyncForgetSpace 清掉本机一个已被移出的团队空间 (本机副本、用户数据目录)。有运行中的实例时拒绝;
// 有没推送的修改时 force 为 false 返回 40910 由界面确认。
func (a *App) SyncForgetSpace(key string, force bool) error {
	if a.cloud == nil {
		return errcode.ErrAuthRequired
	}
	if err := a.checkSpaceIdle(key, force); err != nil {
		return err
	}
	ctx, cancel := a.cloudCtx(time.Minute)
	defer cancel()
	err := a.cloud.ForgetSpace(ctx, key)
	logCloud("sync.forget", err)
	if err == nil {
		_ = a.browserMgr.ReloadProfiles()
	}
	return err
}

// checkSpaceIdle 离开空间前的检查: 实例都已停止, 没推送的修改由用户确认。
func (a *App) checkSpaceIdle(key string, force bool) error {
	a.browserMgr.Mutex.Lock()
	for _, p := range a.browserMgr.Profiles {
		if p.Space == key && p.Running {
			a.browserMgr.Mutex.Unlock()
			return errcode.ErrProfileRunning
		}
	}
	a.browserMgr.Mutex.Unlock()
	if force {
		return nil
	}
	n, err := a.cloud.SpacePending(key)
	if err != nil {
		return err
	}
	if n > 0 {
		return errcode.ErrLeaveUnpushed
	}
	return nil
}

// ─── 端到端加密 ────────────────────────────────────────────────────────────

// SyncKeysEnroll 首次开通端到端加密, 返回恢复码 (只此一次)。
func (a *App) SyncKeysEnroll() (string, error) {
	if a.cloud == nil {
		return "", errcode.ErrAuthRequired
	}
	ctx, cancel := a.cloudCtx(30 * time.Second)
	defer cancel()
	code, err := a.cloud.Enroll(ctx)
	logCloud("sync.enroll", err)
	return code, err
}

// SyncKeysUnlock 用恢复码在本机解锁。
func (a *App) SyncKeysUnlock(recoveryCode string) error {
	if a.cloud == nil {
		return errcode.ErrAuthRequired
	}
	ctx, cancel := a.cloudCtx(30 * time.Second)
	defer cancel()
	err := a.cloud.UnlockWithRecoveryCode(ctx, strings.TrimSpace(recoveryCode))
	logCloud("sync.unlock", err)
	return err
}

// SyncKeysResetRecovery 生成新恢复码 (旧的作废), 只返回这一次。
func (a *App) SyncKeysResetRecovery() (string, error) {
	if a.cloud == nil {
		return "", errcode.ErrAuthRequired
	}
	ctx, cancel := a.cloudCtx(30 * time.Second)
	defer cancel()
	code, err := a.cloud.ResetRecoveryCode(ctx)
	logCloud("sync.reset_recovery", err)
	return code, err
}

// SyncSaveRecoveryCode 弹出保存对话框, 把恢复码存成文本文件。用户取消返回 false。
func (a *App) SyncSaveRecoveryCode(code string) (bool, error) {
	if a.ctx == nil || strings.TrimSpace(code) == "" {
		return false, errcode.ErrFileWrite
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存恢复码",
		DefaultFilename: "CZL Browser 恢复码.txt",
		Filters:         []runtime.FileFilter{{DisplayName: "文本文件 (*.txt)", Pattern: "*.txt"}},
	})
	if err != nil {
		return false, errcode.ErrFileWrite.Wrap(err)
	}
	if path == "" {
		return false, nil
	}
	user := ""
	if st := a.AuthGetState(); st.User != nil {
		user = st.User.Username
	}
	content := fmt.Sprintf("CZL Browser 云端同步恢复码\r\n\r\n账号：%s\r\n恢复码：%s\r\n生成时间：%s\r\n\r\n"+
		"换电脑或重装后，用这个恢复码解锁云端数据。CZL 无法帮你找回，请妥善保管，不要发给别人。\r\n",
		user, code, time.Now().Format("2006-01-02 15:04"))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return false, errcode.ErrFileWrite.Wrap(err)
	}
	return true, nil
}

// SyncDeviceStart 请已登录的其他设备批准本机 (本机显示 6 位码, 经事件 sync:device:code 推给界面)。
func (a *App) SyncDeviceStart() error {
	if a.cloud == nil {
		return errcode.ErrAuthRequired
	}
	ctx, cancel := a.cloudCtx(30 * time.Second)
	defer cancel()
	err := a.cloud.StartDeviceEnrollment(ctx)
	logCloud("sync.device_start", err)
	return err
}

// SyncDeviceCancel 撤回本机发起的授权请求。
func (a *App) SyncDeviceCancel() {
	if a.cloud != nil {
		a.cloud.CancelDeviceEnrollment()
	}
}

// SyncDeviceRequests 列出待批准的新设备。
func (a *App) SyncDeviceRequests() ([]SyncDeviceRequest, error) {
	if a.cloud == nil {
		return []SyncDeviceRequest{}, nil
	}
	ctx, cancel := a.cloudCtx(20 * time.Second)
	defer cancel()
	out, err := a.cloud.PendingDeviceRequests(ctx)
	logCloud("sync.device_requests", err)
	return out, err
}

// SyncDeviceApproveBegin 开始批准新设备, 返回本机算出的 6 位码给用户核对。
func (a *App) SyncDeviceApproveBegin(requestID string) (string, error) {
	if a.cloud == nil {
		return "", errcode.ErrAuthRequired
	}
	ctx, cancel := a.cloudCtx(2 * time.Minute)
	defer cancel()
	code, err := a.cloud.BeginDeviceApproval(ctx, requestID)
	logCloud("sync.device_begin", err)
	return code, err
}

// SyncDeviceApprove 批准或拒绝新设备。
func (a *App) SyncDeviceApprove(requestID string, approve bool) error {
	if a.cloud == nil {
		return errcode.ErrAuthRequired
	}
	ctx, cancel := a.cloudCtx(30 * time.Second)
	defer cancel()
	err := a.cloud.ApproveDevice(ctx, requestID, approve)
	logCloud("sync.device_approve", err)
	return err
}

// ─── 退出登录 ──────────────────────────────────────────────────────────────

// SyncUnpushedCount 还没上传的修改数 (退出登录前提示用)。
func (a *App) SyncUnpushedCount() int {
	if a.cloud == nil {
		return 0
	}
	n, err := a.cloud.UnpushedCount()
	logCloud("sync.unpushed", err)
	return n
}

// stopCloud 退出登录: 停同步、清订阅缓存, 当前空间回到本机。本地数据与本机密钥保留, 重新登录同一账号继续。
func (a *App) stopCloud() {
	if a.cloud != nil {
		a.cloud.Stop()
	}
	a.resetBilling()
	if a.browserMgr != nil {
		_ = a.browserMgr.ReloadProfiles()
	}
}
