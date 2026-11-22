package cloudsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/woodchen-ink/go-web-utils/syncsdk"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/profilestate"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
)

// 登录态同步。实例停止后把用户数据目录里的账号数据打成包 (profilestate), 记进 profile_states 并排进出站队列;
// 推送时 (adapter.Pending) 按 partSize 分片上传成端到端加密附件, 文档内容是分片清单。别的设备启动这个实例前,
// 服务端版本比本机用户数据目录新就下载、校验、解包替换。运行期间发 presence 心跳, 团队里别人启动时提示占用。

// partSize 是登录态包的分片大小 (服务端单个附件上限 32 MB)。
const partSize int64 = 24 << 20

// packDelay 是实例退出到开始打包之间的等待。
var packDelay = 3 * time.Second

// StateManifest 是 states 集合的文档内容。
type StateManifest struct {
	V       int         `json:"v"`
	Parts   []StatePart `json:"parts"`
	Size    int64       `json:"size"`
	SHA256  string      `json:"sha256"`
	SavedAt time.Time   `json:"savedAt"`
	// Device 是上传这份登录态的设备名, 界面展示「最近在哪台设备上用过」
	Device string `json:"device"`
}

// StatePart 是一个分片附件。
type StatePart struct {
	BlobID string `json:"blobId"`
	Size   int64  `json:"size"`
}

func (s *Service) lockState(profileID string) func() {
	v, _ := s.stateMu.LoadOrStore(profileID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *Service) packDir() string { return filepath.Join(s.opts.DataDir, "packs") }

// uploadPack 把待上传的本地包分片上传, 返回清单 (推送时由 adapter 调用)。同空间同内容的分片服务端去重。
func (s *Service) uploadPack(ctx context.Context, remote, profileID, packPath string) (json.RawMessage, error) {
	eng := s.engineRef()
	if eng == nil {
		return nil, errcode.ErrSyncUnavailable
	}
	st, err := os.Stat(packPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errcode.ErrSyncPayload.Wrap(fmt.Errorf("states/%s: pack missing", profileID))
	}
	if err != nil {
		return nil, errcode.ErrStatePack.Wrap(err)
	}
	if st.Size() > profilestate.MaxPackBytes {
		return nil, errcode.ErrStateTooLarge
	}
	sum, err := fileSHA256(packPath)
	if err != nil {
		return nil, errcode.ErrStatePack.Wrap(err)
	}
	m := StateManifest{V: 1, Size: st.Size(), SHA256: sum, SavedAt: st.ModTime().UTC(), Device: s.opts.DeviceName}
	for off := int64(0); off < st.Size(); off += partSize {
		n := min(partSize, st.Size()-off)
		start := off
		id, err := eng.UploadBlob(ctx, remote, func() (io.ReadCloser, error) {
			f, err := os.Open(packPath)
			if err != nil {
				return nil, err
			}
			return sectionCloser{io.NewSectionReader(f, start, n), f}, nil
		}, "application/zip")
		if err != nil {
			return nil, err
		}
		m.Parts = append(m.Parts, StatePart{BlobID: id, Size: n})
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, errcode.ErrPayloadCodec.Wrap(err)
	}
	return b, nil
}

type sectionCloser struct {
	*io.SectionReader
	f *os.File
}

func (c sectionCloser) Close() error { return c.f.Close() }

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// LaunchCheck 是启动同步空间里的实例前的检查结果。
type LaunchCheck struct {
	// InUseBy 正在使用这个实例的成员标识 (presence 里的 userId), 空 = 没人在用
	InUseBy string `json:"inUseBy"`
	// InUseOn 使用中的那台设备的名字 (老客户端不带, 可能为空)
	InUseOn string `json:"inUseOn"`
}

// PrepareLaunch 在启动实例前调用: 同步空间里检查占用, 服务端登录态比本机新时下载还原。
// force 为 true 时跳过占用检查, 下载失败也照常用本机数据启动。本机空间直接返回。
func (s *Service) PrepareLaunch(ctx context.Context, profileID string, force bool) (*LaunchCheck, error) {
	local, err := s.opts.Store.ProfileSpace(profileID)
	if err != nil || !space.IsSynced(local) {
		return &LaunchCheck{}, err
	}
	if !force {
		for _, p := range s.Presence(local) {
			if p.Collection == space.CollProfiles && p.ID == profileID {
				check := &LaunchCheck{InUseBy: p.UserID, InUseOn: p.DeviceName}
				if p.UserID == s.userID() {
					return check, errcode.ErrProfileInUseElsewhere // 本人另一台电脑
				}
				return check, errcode.ErrProfileInUse
			}
		}
	}
	unlock := s.lockState(profileID)
	defer unlock()
	if err := s.downloadState(ctx, local, profileID); err != nil {
		if force {
			log.Printf("sync.state_download %s (forced launch): %v", profileID, err)
			return &LaunchCheck{}, nil
		}
		return &LaunchCheck{}, err
	}
	return &LaunchCheck{}, nil
}

// downloadState 服务端登录态比本机新、且本机没有未上传的包时下载并还原。
func (s *Service) downloadState(ctx context.Context, local, profileID string) error {
	row, err := s.opts.Store.State(profileID)
	if err != nil || row == nil || row.Space != local || row.Rev <= row.AppliedRev || row.Dirty > 0 || len(row.Payload) == 0 {
		return err
	}
	var m StateManifest
	if err := json.Unmarshal(row.Payload, &m); err != nil || m.V != 1 || len(m.Parts) == 0 {
		return errcode.ErrSyncPayload.Wrap(fmt.Errorf("states/%s: manifest", profileID))
	}
	remote, ok := remoteOf(local)
	eng := s.engineRef()
	if !ok || eng == nil || s.keysStatus() != KeysUnlocked {
		return errcode.ErrSyncUnavailable
	}
	if s.opts.UserDataDir == nil {
		return errcode.ErrStatePack.Wrap(errors.New("no user data dir resolver"))
	}

	if err := os.MkdirAll(s.packDir(), 0o700); err != nil {
		return errcode.ErrStatePack.Wrap(err)
	}
	tmp := filepath.Join(s.packDir(), profileID+"-download.zip")
	defer os.Remove(tmp)
	out, err := os.Create(tmp)
	if err != nil {
		return errcode.ErrStatePack.Wrap(err)
	}
	h := sha256.New()
	w := io.MultiWriter(out, h)
	var total int64
	var cached []string
	// 附件缓存是解密后的明文: 无论成败都删掉, 不留在磁盘上
	defer func() {
		for _, p := range cached {
			_ = os.Remove(p)
		}
	}()
	for _, part := range m.Parts {
		p, err := eng.BlobPath(ctx, remote, part.BlobID)
		if err != nil {
			out.Close()
			return classify(err)
		}
		cached = append(cached, p)
		f, err := os.Open(p)
		if err != nil {
			out.Close()
			return errcode.ErrStatePack.Wrap(err)
		}
		n, err := io.Copy(w, f)
		f.Close()
		if err != nil {
			out.Close()
			return errcode.ErrStatePack.Wrap(err)
		}
		total += n
	}
	if err := out.Close(); err != nil {
		return errcode.ErrStatePack.Wrap(err)
	}
	if total != m.Size || hex.EncodeToString(h.Sum(nil)) != strings.ToLower(m.SHA256) {
		return errcode.ErrSyncPayload.Wrap(fmt.Errorf("states/%s: checksum mismatch", profileID))
	}
	if err := profilestate.Unpack(tmp, s.opts.UserDataDir(profileID)); err != nil {
		return errcode.ErrStatePack.Wrap(err)
	}
	return s.opts.Store.SetApplied(profileID, row.Rev)
}

// OnStarted 在实例启动后调用: 同步空间里发 presence 心跳 (团队成员看得到「正在使用」)。
func (s *Service) OnStarted(profileID string) {
	local, err := s.opts.Store.ProfileSpace(profileID)
	remote, ok := remoteOf(local)
	if err != nil || !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.engine == nil || s.root == nil || localKey(s.user, remote) != local {
		return
	}
	if stop, ok := s.running[profileID]; ok {
		stop()
	}
	s.running[profileID] = s.engine.Editing(s.root, remote, space.CollProfiles, profileID)
}

// OnStopped 在实例进程退出后调用: 停心跳, 同步空间里 (可写时) 后台打包登录态并排进出站队列。
func (s *Service) OnStopped(profileID string) {
	s.mu.Lock()
	if stop, ok := s.running[profileID]; ok {
		stop()
		delete(s.running, profileID)
	}
	user := s.user
	s.mu.Unlock()

	local, err := s.opts.Store.ProfileSpace(profileID)
	if err != nil || !space.IsSynced(local) || user == "" || s.opts.UserDataDir == nil {
		return
	}
	if sp := s.space(local); sp == nil || sp.ReadOnly || sp.Removed || s.keysStatus() != KeysUnlocked {
		return // 推不上去的空间不打包, 免得一直挂在未上传里
	}
	go func() {
		// 浏览器主进程退出后子进程还会持有 Cookie 等文件片刻, 稍等再打包, 免得读到半截的数据库
		time.Sleep(packDelay)
		if err := s.packState(local, profileID); err != nil {
			log.Printf("sync.state_pack %s: %v", profileID, err)
			s.emit(EventStateError, map[string]any{"profileId": profileID, "error": err.Error()})
		}
	}()
}

func (s *Service) packState(local, profileID string) error {
	unlock := s.lockState(profileID)
	defer unlock()
	// 等待期间又启动了 (重启): 目录正在用, 不打包, 下次停止时再打
	if s.opts.IsRunning != nil && s.opts.IsRunning(profileID) {
		return nil
	}
	if err := os.MkdirAll(s.packDir(), 0o700); err != nil {
		return errcode.ErrStatePack.Wrap(err)
	}
	dst := filepath.Join(s.packDir(), fmt.Sprintf("%s-%d.zip", profileID, time.Now().UnixNano()))
	if _, err := profilestate.Pack(s.opts.UserDataDir(profileID), dst); err != nil {
		if errors.Is(err, profilestate.ErrTooLarge) {
			return errcode.ErrStateTooLarge.Wrap(err)
		}
		return errcode.ErrStatePack.Wrap(err)
	}
	old, err := s.opts.Store.SetStatePack(local, profileID, dst)
	if err != nil {
		removePack(dst)
		return err
	}
	removePack(old)
	s.onLocalEdit(local, space.CollStates, profileID)
	return nil
}

// ─── presence ─────────────────────────────────────────────────────────────

func (s *Service) onPresence(remote string, items []syncsdk.Presence) {
	key := localKey(s.userID(), remote)
	s.mu.Lock()
	s.presence[key] = items
	s.mu.Unlock()
	if items == nil {
		items = []syncsdk.Presence{}
	}
	s.emit(EventPresence, map[string]any{"space": key, "items": items})
}

// Presence 返回空间里他人正在使用的实例。
func (s *Service) Presence(key string) []syncsdk.Presence {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]syncsdk.Presence(nil), s.presence[key]...)
	if out == nil {
		out = []syncsdk.Presence{}
	}
	now := time.Now()
	live := out[:0]
	for _, p := range out {
		if p.ExpiresAt.IsZero() || p.ExpiresAt.After(now) {
			live = append(live, p)
		}
	}
	return live
}

// removeProfileDir 删除实例用户数据目录及其 .prev 备份 (离开团队、清理被移出的副本)。
func removeProfileDir(dir string) {
	if dir == "" {
		return
	}
	for _, d := range []string{dir, dir + ".prev", dir + ".incoming"} {
		if err := os.RemoveAll(d); err != nil {
			log.Printf("sync.profile_dir_remove %s: %v", d, err)
		}
	}
}

// CopyState 把一个实例的登录态复制给另一个实例 (跨空间复制): 打一次包, 解到新实例的用户数据目录;
// 目标是同步空间时这个包直接作为新实例的待上传快照, 否则删掉。源实例须已停止。
func (s *Service) CopyState(srcDir, dstProfileID, dstDir, target string) error {
	if _, err := os.Stat(srcDir); errors.Is(err, os.ErrNotExist) {
		return nil // 源实例还没启动过, 没有登录态
	}
	unlock := s.lockState(dstProfileID)
	defer unlock()
	if err := os.MkdirAll(s.packDir(), 0o700); err != nil {
		return errcode.ErrStatePack.Wrap(err)
	}
	pack := filepath.Join(s.packDir(), fmt.Sprintf("%s-%d.zip", dstProfileID, time.Now().UnixNano()))
	if _, err := profilestate.Pack(srcDir, pack); err != nil {
		if errors.Is(err, profilestate.ErrTooLarge) {
			return errcode.ErrStateTooLarge.Wrap(err)
		}
		return errcode.ErrStatePack.Wrap(err)
	}
	if err := profilestate.Unpack(pack, dstDir); err != nil {
		removePack(pack)
		return errcode.ErrStatePack.Wrap(err)
	}
	_ = os.RemoveAll(dstDir + ".prev")
	if !space.IsSynced(target) {
		removePack(pack)
		return nil
	}
	old, err := s.opts.Store.SetStatePack(target, dstProfileID, pack)
	if err != nil {
		removePack(pack)
		return err
	}
	removePack(old)
	s.onLocalEdit(target, space.CollStates, dstProfileID)
	return nil
}
