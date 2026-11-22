package cloudsync

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/woodchen-ink/go-web-utils/syncsdk"
	"github.com/woodchen-ink/go-web-utils/syncsdk/e2e"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
)

// 端到端加密的界面流程。密码学与协议都在 SDK, 这里只把它们串成界面能调用的几步, 并在解锁后开始同步。
//
//   - 未开通: Enroll 生成恢复码 (只返回这一次), 界面要求用户保存
//   - 本机未解锁: UnlockWithRecoveryCode, 或 StartDeviceEnrollment 请已登录的设备批准 (两端核对 6 位码)
//   - 已解锁: 每 30 秒查一次待批准的新设备并推给界面; ResetRecoveryCode 换新恢复码

// checkKeys 联网核对密钥状态, 解锁时开始同步。
func (s *Service) checkKeys(ctx context.Context) {
	keys := s.keysRef()
	if keys == nil {
		return
	}
	st, err := keys.Status(ctx)
	if err != nil {
		log.Printf("sync.keys_status: %v", err)
		return // 连不上时保持本机判断 (有私钥就继续用)
	}
	switch st {
	case syncsdk.KeyStatusNotEnrolled:
		s.setKeys(KeysNotEnrolled)
	case syncsdk.KeyStatusLocked:
		s.setKeys(KeysLocked)
	case syncsdk.KeyStatusUnlocked:
		s.setKeys(KeysUnlocked)
	}
}

// RecheckKeys 重新核对密钥状态并刷新空间 (界面打开同步设置时)。
func (s *Service) RecheckKeys(ctx context.Context) State {
	s.checkKeys(ctx)
	return s.Refresh(ctx)
}

// onUnlocked 本机刚拿到私钥: 开始同步。
func (s *Service) onUnlocked() {
	s.setKeys(KeysUnlocked)
	s.mu.Lock()
	root := s.root
	s.mu.Unlock()
	if root == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(root, 30*time.Second)
		defer cancel()
		s.Refresh(ctx)
	}()
}

// Enroll 首次开通端到端加密, 返回恢复码 (只此一次, 界面须让用户保存)。
// 别的设备已经开通过时返回 40922, 界面改走恢复码或设备批准。
func (s *Service) Enroll(ctx context.Context) (string, error) {
	keys := s.keysRef()
	if keys == nil {
		return "", errcode.ErrAuthRequired
	}
	code, err := keys.Enroll(ctx)
	if errors.Is(err, syncsdk.ErrConflict) {
		s.setKeys(KeysLocked)
		return "", errcode.ErrKeysAlreadyEnrolled.Wrap(err)
	}
	if err != nil {
		return "", classify(err)
	}
	str := code.String()
	code.Destroy()
	s.onUnlocked()
	return str, nil
}

// UnlockWithRecoveryCode 用恢复码在本机解锁。
func (s *Service) UnlockWithRecoveryCode(ctx context.Context, recoveryCode string) error {
	keys := s.keysRef()
	if keys == nil {
		return errcode.ErrAuthRequired
	}
	err := keys.UnlockWithRecoveryCode(ctx, recoveryCode)
	switch {
	case errors.Is(err, e2e.ErrInvalidRecoveryCode):
		return errcode.ErrRecoveryCodeInvalid.Wrap(err)
	case errors.Is(err, e2e.ErrDecrypt):
		return errcode.ErrRecoveryCodeWrong.Wrap(err)
	case err != nil:
		return classify(err)
	}
	s.onUnlocked()
	return nil
}

// ResetRecoveryCode 生成新恢复码 (旧的作废), 只返回这一次。
func (s *Service) ResetRecoveryCode(ctx context.Context) (string, error) {
	keys := s.keysRef()
	if keys == nil {
		return "", errcode.ErrAuthRequired
	}
	code, err := keys.ResetRecoveryCode(ctx)
	if err != nil {
		return "", classify(err)
	}
	str := code.String()
	code.Destroy()
	return str, nil
}

// ─── 新设备 (本机) 请求授权 ───────────────────────────────────────────────

type deviceFlow struct {
	cancel context.CancelFunc
	enr    *syncsdk.DeviceEnrollment
}

// StartDeviceEnrollment 请已登录的其他设备批准本机: 后台等对方响应, 拿到 6 位码时推 EventDeviceCode,
// 批准后推 EventDeviceDone 并开始同步。同一时间只有一个请求, 再次调用会撤回上一个。
func (s *Service) StartDeviceEnrollment(ctx context.Context) error {
	keys := s.keysRef()
	if keys == nil {
		return errcode.ErrAuthRequired
	}
	s.CancelDeviceEnrollment()
	enr, err := keys.StartDeviceEnrollment(ctx, s.opts.DeviceName)
	if errors.Is(err, syncsdk.ErrNotEnrolled) {
		return errcode.ErrKeysNotEnrolled.Wrap(err)
	}
	if err != nil {
		return classify(err)
	}
	s.mu.Lock()
	root := s.root
	s.mu.Unlock()
	if root == nil {
		return errcode.ErrAuthRequired
	}
	fctx, cancel := context.WithDeadline(root, enr.ExpiresAt())
	flow := &deviceFlow{cancel: cancel, enr: enr}
	s.mu.Lock()
	s.device = flow
	s.mu.Unlock()
	go func() {
		defer cancel()
		done := func(err error) {
			s.mu.Lock()
			if s.device == flow {
				s.device = nil
			}
			s.mu.Unlock()
			payload := map[string]any{"ok": err == nil}
			if err != nil {
				payload["error"] = deviceError(err).Error()
			}
			s.emit(EventDeviceDone, payload)
		}
		code, err := enr.WaitCode(fctx)
		if err != nil {
			done(err)
			return
		}
		s.emit(EventDeviceCode, map[string]any{"code": code})
		if err := enr.WaitApproval(fctx); err != nil {
			done(err)
			return
		}
		s.onUnlocked()
		done(nil)
	}()
	return nil
}

// CancelDeviceEnrollment 撤回本机发起的授权请求。
func (s *Service) CancelDeviceEnrollment() {
	s.mu.Lock()
	flow := s.device
	s.device = nil
	s.mu.Unlock()
	if flow == nil {
		return
	}
	flow.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := flow.enr.Cancel(ctx); err != nil {
		log.Printf("sync.device_cancel: %v", err)
	}
}

func deviceError(err error) error {
	switch {
	case errors.Is(err, syncsdk.ErrRequestExpired), errors.Is(err, syncsdk.ErrRequestRejected),
		errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return errcode.ErrDeviceRequestExpired.Wrap(err)
	}
	return classify(err)
}

// ─── 已解锁的设备批准新设备 ───────────────────────────────────────────────

// DeviceRequestInfo 是一条待批准的新设备请求。
type DeviceRequestInfo struct {
	ID         string    `json:"id"`
	DeviceName string    `json:"deviceName"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// PendingDeviceRequests 列出待批准的新设备 (本机须已解锁)。
func (s *Service) PendingDeviceRequests(ctx context.Context) ([]DeviceRequestInfo, error) {
	keys := s.keysRef()
	if keys == nil || s.keysStatus() != KeysUnlocked {
		return []DeviceRequestInfo{}, nil
	}
	reqs, err := keys.PendingDeviceRequests(ctx)
	if err != nil {
		return nil, classify(err)
	}
	out := make([]DeviceRequestInfo, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, DeviceRequestInfo{ID: r.ID, DeviceName: r.DeviceName, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt})
	}
	return out, nil
}

// watchDeviceRequests 在已解锁时每 30 秒查一次待批准的新设备, 有变化推给界面。
func (s *Service) watchDeviceRequests(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if s.keysStatus() != KeysUnlocked {
			continue
		}
		qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		reqs, err := s.PendingDeviceRequests(qctx)
		cancel()
		if err != nil {
			continue
		}
		sig := ""
		for _, r := range reqs {
			sig += r.ID + ","
		}
		if sig != last {
			last = sig
			s.emit(EventDeviceRequests, reqs)
		}
	}
}

// BeginDeviceApproval 开始批准一个新设备, 返回本机算出的 6 位码; 用户核对与新设备一致后调 ApproveDevice。
func (s *Service) BeginDeviceApproval(ctx context.Context, requestID string) (string, error) {
	keys := s.keysRef()
	if keys == nil || s.keysStatus() != KeysUnlocked {
		return "", errcode.ErrKeysLocked
	}
	reqs, err := keys.PendingDeviceRequests(ctx)
	if err != nil {
		return "", classify(err)
	}
	for _, r := range reqs {
		if r.ID != requestID {
			continue
		}
		ap, err := keys.BeginDeviceApproval(ctx, r)
		if err != nil {
			return "", deviceError(err)
		}
		wctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		code, err := ap.WaitCode(wctx)
		if err != nil {
			return "", deviceError(err)
		}
		s.mu.Lock()
		s.approval[requestID] = ap
		s.mu.Unlock()
		return code, nil
	}
	return "", errcode.ErrDeviceRequestUnknown
}

// ApproveDevice 批准 (approve 为 true) 或拒绝新设备。拒绝不需要先 Begin。
func (s *Service) ApproveDevice(ctx context.Context, requestID string, approve bool) error {
	keys := s.keysRef()
	if keys == nil {
		return errcode.ErrAuthRequired
	}
	s.mu.Lock()
	ap := s.approval[requestID]
	delete(s.approval, requestID)
	s.mu.Unlock()
	if approve {
		if ap == nil {
			return errcode.ErrDeviceRequestUnknown
		}
		if err := ap.Approve(ctx); err != nil {
			return deviceError(err)
		}
		return nil
	}
	if ap != nil {
		return deviceError(ap.Reject(ctx))
	}
	reqs, err := keys.PendingDeviceRequests(ctx)
	if err != nil {
		return classify(err)
	}
	for _, r := range reqs {
		if r.ID == requestID {
			ap, err := keys.BeginDeviceApproval(ctx, r)
			if err != nil {
				return deviceError(err)
			}
			return deviceError(ap.Reject(ctx))
		}
	}
	return nil
}
