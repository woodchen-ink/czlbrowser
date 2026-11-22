// Package cloudsync 把 CZL Browser 接到 CZL Connect 的数据同步: 个人多设备同步与团队空间, 端到端加密。
//
// 协议、推拉、冲突、加解密与密钥生命周期都由 SDK (github.com/woodchen-ink/go-web-utils/syncsdk) 完成,
// 本包只做浏览器特有的部分:
//
//   - 出站队列即本地库: Outbox / CursorStore 落在各表的 dirty / rev 列与 sync_spaces 上 (adapter.go, internal/syncstore)
//   - 端到端加密的界面流程: 开通 (恢复码)、恢复码解锁、新设备批准、重置恢复码 (keys.go); 私钥存 DPAPI 密钥库
//   - 登录态: 实例停止后打包上传 (分片附件), 启动前按需下载还原, 运行期间 presence 占用提示 (states.go)
//   - 空间: 本地标识 me:<用户> / org:<组织> 与服务端空间 id 的对应、当前空间、只读 (本文件)
package cloudsync

import (
	"context"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/woodchen-ink/go-web-utils/syncsdk"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/keystore"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/syncstore"
)

// DefaultBaseURL 是同步服务地址。
const DefaultBaseURL = "https://connect.czl.net/api/sync/v1"

// 推给界面的事件。
const (
	// EventState 同步状态变了, 载荷 State
	EventState = "sync:state"
	// EventChanged 某空间拉到了别处的修改, 载荷 {space, collections}
	EventChanged = "sync:changed"
	// EventConflict 推送遇到冲突、已采用服务端版本, 载荷 ConflictInfo
	EventConflict = "sync:conflict"
	// EventPresence 他人正在使用的实例变了, 载荷 {space, items}
	EventPresence = "sync:presence"
	// EventDeviceCode 新设备授权: 本机 (新设备) 拿到了 6 位核对码, 载荷 {code}
	EventDeviceCode = "sync:device:code"
	// EventDeviceDone 新设备授权结束, 载荷 {ok, error}
	EventDeviceDone = "sync:device:done"
	// EventDeviceRequests 已解锁的设备上有待批准的新设备, 载荷 [DeviceRequestInfo]
	EventDeviceRequests = "sync:device:requests"
	// EventStateError 登录态打包或上传失败, 载荷 {profileId, error}
	EventStateError = "sync:state:error"
)

// TokenSource 提供 CZL Connect 用户令牌, 由 internal/auth 实现。
type TokenSource interface {
	AccessToken(ctx context.Context) (string, error)
	InvalidateToken(token string)
}

// Options 配置 Service。
type Options struct {
	// BaseURL 为空用 DefaultBaseURL
	BaseURL string
	Store   *syncstore.Store
	Spaces  *space.State
	Tokens  TokenSource
	// DataDir 是同步的本机目录 (data/sync): 密钥库 keys/、附件缓存 blobs/、待上传的登录态包 packs/
	DataDir string
	// DeviceName 是新设备授权时给旧设备看的名字, 如「Windows · DESKTOP-1」
	DeviceName string
	// UserDataDir 返回实例的用户数据目录 (绝对路径)
	UserDataDir func(profileID string) string
	// IsRunning 报告实例是否在运行 (打包登录态前再确认一次, 重启时不打包正在用的目录); nil 视为都没在运行
	IsRunning func(profileID string) bool
	// Emit 把事件推给界面, 可为 nil
	Emit func(event string, data any)
	// OnApplied 在某空间落地了拉到的修改后调用 (App 据此重载内存里的实例), 可为 nil
	OnApplied func(space string, collections []string)
	// HTTPClient / Retry 测试注入
	HTTPClient *http.Client
	Retry      syncsdk.RetryPolicy
	// Subscribe 为 false 时不开后台订阅, 只在 SyncNow 时同步 (测试用)
	Subscribe        bool
	SubscribeOptions syncsdk.SubscribeOptions
	// KeyStore 测试注入; 正式环境为 nil, 用 DataDir/keys 下的 DPAPI 密钥库
	KeyStore syncsdk.KeyStore
}

// Service 管一个登录账号的同步。并发安全。登录后 Start, 退出登录 Stop。
type Service struct {
	opts Options
	// startMu 串行化 Start: 启动恢复、登录回调、刷新可能同时调用, 不能组装出两套引擎
	startMu sync.Mutex

	mu     sync.Mutex
	user   string
	client *syncsdk.Client
	keys   *syncsdk.KeyManager
	engine *syncsdk.Engine
	root   context.Context
	cancel context.CancelFunc
	subs   map[string]*subscription // 服务端空间 id → 后台订阅
	state  State
	spaces map[string]*SpaceInfo // 本地空间标识 → 状态

	presence map[string][]syncsdk.Presence
	running  map[string]func() // 实例 id → 停止 presence 心跳
	holds    map[string]int64
	rejected map[string]map[string]bool
	// refsUnsafe 有服务端登录态清单没落到本机的空间: 附件引用集合可能不完整, 不上报 (blobrefs.go)
	refsUnsafe map[string]bool
	refsMu     sync.Map // 本地空间 → *sync.Mutex, 同一空间的引用上报不并发
	// forgetting 正在清掉的空间: 还在跑的同步不再往本地写
	forgetting map[string]bool

	device   *deviceFlow
	approval map[string]*syncsdk.DeviceApproval

	stateMu sync.Map // 实例 id → *sync.Mutex, 串行化同一实例的打包与下载
}

// State 是给界面的同步总状态。
type State struct {
	SignedIn bool `json:"signedIn"`
	// Available 同步服务连得上
	Available bool `json:"available"`
	// Error 最近一次总体失败的错误串 (数字码 + 英文短语)
	Error string `json:"error"`
	// Keys 端到端加密: unknown / not_enrolled / locked / unlocked
	Keys string `json:"keys"`
	// UserID 当前账号 (presence 里 userId 等于它的, 是本人另一台电脑)
	UserID string `json:"userId"`
	// Current 当前空间
	Current string      `json:"current"`
	Spaces  []SpaceInfo `json:"spaces"`
}

// 端到端加密状态。
const (
	KeysUnknown     = "unknown"
	KeysNotEnrolled = "not_enrolled"
	KeysLocked      = "locked"
	KeysUnlocked    = "unlocked"
)

// SpaceInfo 是一个同步空间在本机的状态。
type SpaceInfo struct {
	Key   string `json:"key"`
	Kind  string `json:"kind"` // personal / org
	OrgID string `json:"orgId,omitempty"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	// ReadOnly viewer、订阅到期、组织超席位, 或已不在这个组织
	ReadOnly bool `json:"readOnly"`
	// SettingsReadOnly 团队书签只有 owner / admin 能改
	SettingsReadOnly bool `json:"settingsReadOnly"`
	// Removed 服务端已不再列出这个空间 (被移出团队、团队解散): 本机副本只读
	Removed bool `json:"removed"`
	// KeyPending 团队密钥还没授予本人 (等 owner / admin 在线)
	KeyPending bool   `json:"keyPending"`
	Status     string `json:"status"` // idle / ok / error
	Error      string `json:"error"`
	Pending    int    `json:"pending"`
	SyncedAt   string `json:"syncedAt"`
	UsedBytes  int64  `json:"usedBytes"`
	QuotaBytes int64  `json:"quotaBytes"`
}

// New 创建 Service, 不联网。
func New(opts Options) *Service {
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	s := &Service{opts: opts, subs: map[string]*subscription{}, spaces: map[string]*SpaceInfo{},
		presence: map[string][]syncsdk.Presence{}, running: map[string]func(){}, holds: map[string]int64{}, refsUnsafe: map[string]bool{},
		rejected: map[string]map[string]bool{}, forgetting: map[string]bool{}, approval: map[string]*syncsdk.DeviceApproval{}}
	opts.Spaces.OnLocalEdit(s.onLocalEdit)
	return s
}

func (s *Service) emit(event string, data any) {
	if s.opts.Emit != nil {
		s.opts.Emit(event, data)
	}
}

func (s *Service) docFailed(local, collection, id string, err error) {
	log.Printf("sync.doc %s %s/%s: %v", local, collection, id, err)
}

func (s *Service) userID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.user
}

func (s *Service) engineRef() *syncsdk.Engine {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.engine
}

func (s *Service) keysRef() *syncsdk.KeyManager {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keys
}

// ─── 生命周期 ──────────────────────────────────────────────────────────────

// Start 为登录账号准备同步: 组装 SDK、载入本机记得的空间并切回上次的空间, 后台检查密钥与刷新空间。
// 同一账号重复调用无副作用。
func (s *Service) Start(userID string) error {
	if userID == "" {
		return errcode.ErrAuthRequired
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.mu.Lock()
	if s.user == userID && s.engine != nil {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	s.Stop()

	// 本机设备标识: 「使用中」按设备区分, 同一账号另一台电脑上开着的实例也能提示。
	// 存在 data/sync 下 (不进备份), 换电脑或重装后是新设备。
	deviceID, err := syncsdk.LoadOrCreateDeviceID(filepath.Join(s.opts.DataDir, "device_id"))
	if err != nil {
		log.Printf("sync.device_id: %v", err)
		deviceID = "" // 读写不了就退回按账号区分, 不影响同步
	}
	client, err := syncsdk.NewClient(syncsdk.ClientOptions{
		BaseURL: s.opts.BaseURL, Tokens: tokenAdapter{s.opts.Tokens}, HTTPClient: s.opts.HTTPClient,
		Retry: s.opts.Retry, UserAgent: "czl-browser", DeviceID: deviceID, DeviceName: s.opts.DeviceName,
	})
	if err != nil {
		return errcode.ErrSyncUnavailable.Wrap(err)
	}
	ks := s.opts.KeyStore
	if ks == nil {
		ks = keystore.New(filepath.Join(s.opts.DataDir, "keys"))
	}
	keys, err := syncsdk.NewKeyManager(client, ks, syncsdk.KeyManagerOptions{Namespace: "czlbrowser/" + userID})
	if err != nil {
		return errcode.ErrSyncUnavailable.Wrap(err)
	}
	a := adapter{s: s, user: userID}
	engine, err := syncsdk.NewEngine(syncsdk.EngineOptions{
		Client: client, Keys: keys, Outbox: a, Cursors: a, Applier: a, Resolver: a.resolve,
		OnPresence: s.onPresence, BlobCacheDir: filepath.Join(s.opts.DataDir, "blobs"),
		OnRejected: func(_ context.Context, item syncsdk.OutboxItem, err error) bool {
			return s.onRejected(userID, item, err)
		},
	})
	if err != nil {
		return errcode.ErrSyncUnavailable.Wrap(err)
	}

	root, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.user, s.client, s.keys, s.engine, s.root, s.cancel = userID, client, keys, engine, root, cancel
	s.state = State{SignedIn: true, Keys: KeysUnknown, UserID: userID}
	s.spaces = map[string]*SpaceInfo{}
	for _, sp := range s.loadKnown() {
		sp := sp
		sp.Status = "idle"
		s.spaces[sp.Key] = &sp
	}
	s.mu.Unlock()
	s.restoreSpace()

	// 本机有私钥就先当作已解锁, 离线也能用; 联网后再核对服务端公钥
	if ok, err := keys.Load(root); err == nil && ok {
		s.setKeys(KeysUnlocked)
	}
	go func() {
		ctx, cancel := context.WithTimeout(root, 30*time.Second)
		defer cancel()
		s.checkKeys(ctx)
		s.Refresh(ctx)
	}()
	go s.watchDeviceRequests(root)
	return nil
}

// Stop 停掉全部订阅 (退出登录、换账号、关程序), 当前空间回到本机。本地已同步的数据留在库里。
func (s *Service) Stop() {
	s.mu.Lock()
	cancel, subs := s.cancel, s.subs
	for id, stop := range s.running {
		stop()
		delete(s.running, id)
	}
	s.user, s.client, s.keys, s.engine, s.cancel = "", nil, nil, nil, nil
	s.holds = map[string]int64{}
	s.refsUnsafe = map[string]bool{}
	s.rejected = map[string]map[string]bool{}
	s.subs = map[string]*subscription{}
	s.spaces = map[string]*SpaceInfo{}
	s.presence = map[string][]syncsdk.Presence{}
	s.approval = map[string]*syncsdk.DeviceApproval{}
	s.device = nil
	s.state = State{}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, sub := range subs {
		sub.wait()
	}
	s.opts.Spaces.Set(space.LocalSpace, space.Access{})
	s.emit(EventState, s.State())
}

type tokenAdapter struct{ t TokenSource }

func (a tokenAdapter) Token(ctx context.Context) (string, error) { return a.t.AccessToken(ctx) }
func (a tokenAdapter) InvalidateToken(tok string)                { a.t.InvalidateToken(tok) }

// ─── 状态与空间 ────────────────────────────────────────────────────────────

// State 返回当前状态 (不联网)。
func (s *Service) State() State {
	s.mu.Lock()
	st := s.state
	list := make([]SpaceInfo, 0, len(s.spaces))
	for _, sp := range s.spaces {
		list = append(list, *sp)
	}
	s.mu.Unlock()
	if st.Keys == "" {
		st.Keys = KeysUnknown
	}
	for i := range list {
		if n, err := s.opts.Store.PendingCount(list[i].Key); err == nil {
			list[i].Pending = n
		}
	}
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Kind != b.Kind {
			return a.Kind == syncsdk.SpaceTypePersonal
		}
		return a.Name < b.Name
	})
	st.Spaces = list
	st.Current = s.opts.Spaces.Current()
	return st
}

func (s *Service) space(key string) *SpaceInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sp, ok := s.spaces[key]; ok {
		cp := *sp
		return &cp
	}
	return nil
}

func (s *Service) setState(fn func(*State)) {
	s.mu.Lock()
	fn(&s.state)
	s.mu.Unlock()
	s.emit(EventState, s.State())
}

func (s *Service) setKeys(status string) {
	s.setState(func(st *State) { st.Keys = status })
	// 解锁 / 锁定会改变同步空间能不能写
	s.syncAccess()
}

func (s *Service) keysStatus() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Keys
}

// accessOf 是本机在空间里的写权限。密钥没解锁 (没开通、这台电脑没解锁、还没核对完) 时一律只读:
// 写进去的修改推不上去, 只会卡在本机, 不如直接挡住并提示去开通 / 解锁。
func accessOf(sp *SpaceInfo, keys string) space.Access {
	locked := keys != KeysUnlocked
	return space.Access{
		ReadOnly:         sp.ReadOnly || sp.Removed || locked,
		SettingsReadOnly: sp.SettingsReadOnly || locked,
	}
}

// SwitchSpace 切换当前空间并记下 (下次启动、重新登录时恢复)。key 为空或 local 回到本机空间。
func (s *Service) SwitchSpace(key string) error {
	if key == "" || key == space.LocalSpace {
		s.opts.Spaces.Set(space.LocalSpace, space.Access{})
	} else {
		sp := s.space(key)
		if sp == nil {
			return errcode.ErrSpaceUnknown
		}
		s.opts.Spaces.Set(key, accessOf(sp, s.keysStatus()))
	}
	if user := s.userID(); user != "" {
		if err := s.opts.Store.SetJSON(lastSpaceKey(user), s.opts.Spaces.Current()); err != nil {
			log.Printf("sync.last_space: %v", err)
		}
	}
	s.emit(EventState, s.State())
	return nil
}

func lastSpaceKey(userID string) string { return "sync_last_space:" + userID }

// restoreSpace 切回这个账号上次用的空间 (本机还认识它时)。
func (s *Service) restoreSpace() {
	user := s.userID()
	var last string
	if ok, err := s.opts.Store.GetJSON(lastSpaceKey(user), &last); err != nil || !ok {
		return
	}
	if sp := s.space(last); sp != nil {
		s.opts.Spaces.Set(last, accessOf(sp, s.keysStatus()))
	}
}

// syncAccess 把当前空间的只读状态同步到 space.State (后台刷新后)。
func (s *Service) syncAccess() {
	cur := s.opts.Spaces.Current()
	if sp := s.space(cur); sp != nil {
		s.opts.Spaces.SetAccessIf(cur, accessOf(sp, s.keysStatus()))
	}
}

// Refresh 联网刷新可访问的空间, 密钥已解锁时为每个空间开后台订阅。连不上不算错误, Available 为 false。
func (s *Service) Refresh(ctx context.Context) State {
	s.mu.Lock()
	client, user := s.client, s.user
	s.mu.Unlock()
	if client == nil {
		return s.State()
	}
	remote, err := client.Spaces(ctx)
	if err != nil {
		s.setState(func(st *State) {
			st.Available = false
			st.Error = classify(err).Error()
		})
		return s.State()
	}
	s.mu.Lock()
	if s.user != user {
		s.mu.Unlock()
		return s.State()
	}
	s.state.Available, s.state.Error = true, ""
	seen := map[string]bool{}
	for _, r := range remote {
		key := localKey(user, r.ID)
		seen[key] = true
		sp := s.spaces[key]
		if sp == nil {
			sp = &SpaceInfo{Key: key, Status: "idle"}
			s.spaces[key] = sp
		}
		sp.Kind, sp.Name, sp.Role, sp.Removed = r.Type, r.Name, r.Role, false
		sp.UsedBytes, sp.QuotaBytes = r.Usage.Bytes, r.Quota.Bytes
		if r.Type == syncsdk.SpaceTypeOrg {
			sp.OrgID = r.ID
		}
		sp.ReadOnly = r.ReadOnly || r.Role == syncsdk.RoleViewer
		sp.SettingsReadOnly = sp.ReadOnly ||
			(r.Type == syncsdk.SpaceTypeOrg && r.Role != syncsdk.RoleOwner && r.Role != syncsdk.RoleAdmin)
	}
	for key, sp := range s.spaces {
		if !seen[key] {
			sp.Removed, sp.ReadOnly, sp.SettingsReadOnly = true, true, true
			if r, ok := remoteOf(key); ok {
				s.unsubscribeLocked(r)
			}
		}
	}
	if s.opts.Subscribe && s.state.Keys == KeysUnlocked {
		for _, r := range remote {
			s.subscribeLocked(r.ID)
		}
	}
	known := make([]SpaceInfo, 0, len(s.spaces))
	for _, sp := range s.spaces {
		known = append(known, *sp)
	}
	s.mu.Unlock()

	s.saveKnown(known)
	s.syncAccess()
	st := s.State()
	s.emit(EventState, st)
	return st
}

// subscription 是一个空间的后台订阅。done 在订阅协程退出后关闭。
type subscription struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (sub *subscription) wait() {
	sub.cancel()
	select {
	case <-sub.done:
	case <-time.After(15 * time.Second):
		log.Printf("sync.unsubscribe: still running after 15s")
	}
}

func (s *Service) subscribeLocked(remote string) {
	if _, ok := s.subs[remote]; ok || s.engine == nil {
		return
	}
	delete(s.forgetting, localKey(s.user, remote))
	ctx, cancel := context.WithCancel(s.root)
	sub := &subscription{cancel: cancel, done: make(chan struct{})}
	s.subs[remote] = sub
	engine, key := s.engine, localKey(s.user, remote)
	opts := s.opts.SubscribeOptions
	opts.OnSync = func(res syncsdk.SyncResult, err error) { s.onSync(key, res, err) }
	go func() {
		defer close(sub.done)
		_ = engine.Subscribe(ctx, remote, opts)
	}()
}

func (s *Service) unsubscribeLocked(remote string) *subscription {
	sub, ok := s.subs[remote]
	if !ok {
		return nil
	}
	sub.cancel()
	delete(s.subs, remote)
	return sub
}

// ─── 拉取失败时游标停住 ──────────────────────────────────────────────────

func (s *Service) holdBefore(local string, rev int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.holds[local]; !ok || rev-1 < h {
		s.holds[local] = rev - 1
	}
}

func (s *Service) hold(local string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.holds[local]
	return h, ok
}

func (s *Service) clearHold(local string) {
	s.mu.Lock()
	delete(s.holds, local)
	s.mu.Unlock()
}

func (s *Service) isForgetting(local string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forgetting[local]
}

// SyncNow 立即同步一个空间。
func (s *Service) SyncNow(ctx context.Context, key string) error {
	remote, ok := remoteOf(key)
	eng := s.engineRef()
	if !ok || eng == nil || s.space(key) == nil {
		return errcode.ErrSpaceUnknown
	}
	if s.keysStatus() != KeysUnlocked {
		return errcode.ErrKeysLocked
	}
	res, err := eng.Sync(ctx, remote)
	s.onSync(key, res, err)
	if err != nil {
		return classify(err)
	}
	return nil
}

func (s *Service) onSync(key string, res syncsdk.SyncResult, err error) {
	s.mu.Lock()
	sp := s.spaces[key]
	if sp == nil {
		s.mu.Unlock()
		return
	}
	switch {
	case err == nil:
		sp.Status, sp.Error, sp.SyncedAt, sp.KeyPending = "ok", "", time.Now().UTC().Format(time.RFC3339), false
		s.goReportBlobRefs(key)
	case errors.Is(err, syncsdk.ErrKeyNotGranted):
		sp.Status, sp.Error, sp.KeyPending = "error", errcode.ErrSpaceKeyPending.Error(), true
	default:
		sp.Status, sp.Error = "error", classify(err).Error()
		if ae := (*syncsdk.APIError)(nil); errors.As(err, &ae) && ae.Code == codeReadOnly {
			sp.ReadOnly, sp.SettingsReadOnly = true, true
		}
	}
	s.mu.Unlock()
	if res.Pull.Reset {
		s.applied(key, map[string]bool{"profiles": true})
	}
	s.syncAccess()
	s.emit(EventState, s.State())
}

// applied 落地了拉到的修改: 通知 App 重载并推给界面。
func (s *Service) applied(local string, touched map[string]bool) {
	colls := make([]string, 0, len(touched))
	for c := range touched {
		colls = append(colls, c)
	}
	sort.Strings(colls)
	if s.opts.OnApplied != nil {
		s.opts.OnApplied(local, colls)
	}
	s.emit(EventChanged, map[string]any{"space": local, "collections": colls})
}

// onRejected 处理服务端逐条永久拒绝的写入。角色不够 (40311)、集合没登记 (40310) 的放弃本地修改,
// 游标归零让服务端版本改回来; 其余 (超大小、超配额) 留在本地, 下次再试。
func (s *Service) onRejected(userID string, item syncsdk.OutboxItem, err error) bool {
	local := localKey(userID, item.Space)
	s.docFailed(local, item.Collection, item.ID, err)
	var ae *syncsdk.APIError
	if !errors.As(err, &ae) || (ae.Code != codeRoleTooLow && ae.Code != codeNotRegistered) {
		s.mu.Lock()
		if s.rejected[local] == nil {
			s.rejected[local] = map[string]bool{}
		}
		s.rejected[local][item.Collection+"/"+item.ID] = true
		s.mu.Unlock()
		return false
	}
	if err := s.opts.Store.ForgetRev(local, item.Collection, item.ID); err != nil {
		log.Printf("sync.reject_forget: %v", err)
		return false
	}
	if err := s.opts.Store.SetCursor(local, 0, wireVersion); err != nil {
		log.Printf("sync.reject_cursor: %v", err)
	}
	return true
}

// 服务端业务码 (CZL Connect /api/sync/v1)。SDK 只按 HTTP 状态归类, 细分在这里。
const (
	codeNotMember     = 40301
	codeReadOnly      = 40302
	codeNotEnabled    = 40303
	codeUserDisabled  = 40304
	codeNotRegistered = 40310
	codeRoleTooLow    = 40311
)

// classify 把 SDK 错误换成本应用的错误码。
func classify(err error) error {
	var ec *errcode.Error
	var ae *syncsdk.APIError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ec):
		return err
	case errors.Is(err, syncsdk.ErrLocked):
		return errcode.ErrKeysLocked.Wrap(err)
	case errors.Is(err, syncsdk.ErrNotEnrolled):
		return errcode.ErrKeysNotEnrolled.Wrap(err)
	case errors.Is(err, syncsdk.ErrKeyNotGranted):
		return errcode.ErrSpaceKeyPending.Wrap(err)
	case errors.As(err, &ae) && ae.Code == codeNotEnabled:
		return errcode.ErrSyncNotEnabled.Wrap(err)
	case errors.As(err, &ae) && ae.Code == codeUserDisabled:
		return errcode.ErrSyncAccountDisabled.Wrap(err)
	case errors.As(err, &ae) && ae.Code == codeNotMember:
		return errcode.ErrSpaceUnknown.Wrap(err)
	case errors.Is(err, syncsdk.ErrForbidden):
		return errcode.ErrSyncReadOnly.Wrap(err)
	case syncsdk.IsTransient(err), errors.Is(err, syncsdk.ErrNotFound), errors.Is(err, syncsdk.ErrUnauthorized):
		return errcode.ErrSyncUnavailable.Wrap(err)
	default:
		return errcode.ErrSyncRequest.Wrap(err)
	}
}

// ─── 本机记得的空间 ────────────────────────────────────────────────────────

func knownKey(userID string) string { return "sync_spaces:" + userID }

func (s *Service) loadKnown() []SpaceInfo {
	var out []SpaceInfo
	if _, err := s.opts.Store.GetJSON(knownKey(s.user), &out); err != nil {
		log.Printf("sync.known: %v", err)
	}
	return out
}

func (s *Service) saveKnown(list []SpaceInfo) {
	user := s.userID()
	if user == "" {
		return
	}
	for i := range list {
		list[i].Status, list[i].Error, list[i].Pending = "", "", 0
	}
	if err := s.opts.Store.SetJSON(knownKey(user), list); err != nil {
		log.Printf("sync.known: %v", err)
	}
}

// ForgetSpace 清掉本机一个空间: 停订阅、删本地数据 (含实例用户数据目录与待上传的包)、从记得的列表里去掉。
// 调用方须先确认用户接受丢掉未推送的修改, 且空间里没有运行中的实例。
func (s *Service) ForgetSpace(ctx context.Context, key string) error {
	remote, ok := remoteOf(key)
	if !ok {
		return errcode.ErrSpaceUnknown
	}
	s.mu.Lock()
	s.forgetting[key] = true
	sub := s.unsubscribeLocked(remote)
	s.mu.Unlock()
	if sub != nil {
		sub.wait()
	}
	ids, packs, err := s.opts.Store.DeleteSpace(key)
	if err != nil {
		return err
	}
	for _, p := range packs {
		removePack(p)
	}
	if s.opts.UserDataDir != nil {
		for _, id := range ids {
			removeProfileDir(s.opts.UserDataDir(id))
		}
	}
	s.mu.Lock()
	delete(s.spaces, key)
	delete(s.presence, key)
	known := make([]SpaceInfo, 0, len(s.spaces))
	for _, sp := range s.spaces {
		known = append(known, *sp)
	}
	s.mu.Unlock()
	s.saveKnown(known)
	if s.opts.Spaces.Current() == key {
		_ = s.SwitchSpace(space.LocalSpace)
	}
	s.applied(key, map[string]bool{"profiles": true})
	s.emit(EventState, s.State())
	return nil
}

// ─── 本地写入 ──────────────────────────────────────────────────────────────

// onLocalEdit 由 DAO 在同步空间写入后调用: 经 SDK 的 Save 唤醒该空间的推送。内容以本地库为准。
func (s *Service) onLocalEdit(local, collection, id string) {
	remote, ok := remoteOf(local)
	eng := s.engineRef()
	if !ok || eng == nil || localKey(s.userID(), remote) != local {
		return
	}
	if err := eng.Save(context.Background(), remote, collection, id, 0, []byte("null")); err != nil {
		log.Printf("sync.poke %s %s/%s: %v", local, collection, id, err)
	}
}

// UnpushedCount 统计还能推上去的未上传修改 (只读空间、已被拒收的不算)。退出登录前提示用。
func (s *Service) UnpushedCount() (int, error) {
	s.mu.Lock()
	type item struct {
		key      string
		rejected int
	}
	var list []item
	for key, sp := range s.spaces {
		if !sp.Removed && !sp.ReadOnly {
			list = append(list, item{key, len(s.rejected[key])})
		}
	}
	s.mu.Unlock()
	total := 0
	for _, it := range list {
		n, err := s.opts.Store.PendingCount(it.key)
		if err != nil {
			return 0, err
		}
		if n -= it.rejected; n > 0 {
			total += n
		}
	}
	return total, nil
}

// SpacePending 返回一个空间里还没推送的修改数 (离开团队前提示用)。
func (s *Service) SpacePending(key string) (int, error) { return s.opts.Store.PendingCount(key) }
