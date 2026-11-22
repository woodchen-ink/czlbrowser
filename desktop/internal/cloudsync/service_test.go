//go:build windows

package cloudsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodchen-ink/go-web-utils/syncsdk"
	"github.com/woodchen-ink/go-web-utils/syncsdk/synctest"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/config"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/database"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/dpapi"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/syncstore"
)

type staticTokens string

func (t staticTokens) AccessToken(context.Context) (string, error) { return string(t), nil }
func (t staticTokens) InvalidateToken(string)                      {}

// device 是一台装了浏览器的电脑: 独立的数据库、数据目录与密钥库。
type device struct {
	svc      *Service
	spaces   *space.State
	profiles *browser.SQLiteProfileDAO
	proxies  *browser.SQLiteProxyDAO
	bookmark *browser.SQLiteBookmarkDAO
	store    *syncstore.Store
	userData string
}

func newDevice(t *testing.T, srv *synctest.Server, token, name string) *device {
	t.Helper()
	dir := t.TempDir()
	db, err := database.NewDB(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	st := space.NewState()
	d := &device{
		spaces:   st,
		profiles: browser.NewSQLiteProfileDAO(db.GetConn(), st),
		proxies:  browser.NewSQLiteProxyDAO(db.GetConn(), st),
		bookmark: browser.NewSQLiteBookmarkDAO(db.GetConn(), st),
		store:    syncstore.New(db.GetConn()),
		userData: filepath.Join(dir, "user-data"),
	}
	d.svc = New(Options{
		BaseURL: srv.BaseURL(), Store: d.store, Spaces: st, Tokens: staticTokens(token),
		DataDir: filepath.Join(dir, "sync"), DeviceName: name,
		UserDataDir: func(id string) string { return filepath.Join(d.userData, id) },
		Retry:       syncsdk.RetryPolicy{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
		KeyStore:    syncsdk.NewMemoryKeyStore(),
	})
	t.Cleanup(d.svc.Stop)
	return d
}

func newServer(t *testing.T) *synctest.Server {
	t.Helper()
	srv := synctest.New(synctest.Options{E2E: syncsdk.E2ERequired})
	t.Cleanup(srv.Close)
	for _, c := range []string{space.CollProfiles, space.CollProxies, space.CollGroups, space.CollStates} {
		srv.RegisterCollection(c, synctest.Collection{})
	}
	srv.RegisterCollection(space.CollSettings, synctest.Collection{MinRole: syncsdk.RoleAdmin})
	return srv
}

func mustSync(t *testing.T, d *device, key string) {
	t.Helper()
	if err := d.svc.SyncNow(context.Background(), key); err != nil {
		t.Fatalf("sync %s: %v", key, err)
	}
}

func writeLoginState(t *testing.T, dir string, cookie string) {
	t.Helper()
	enc, err := dpapi.Protect([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	ls, _ := json.Marshal(map[string]any{"os_crypt": map[string]any{
		"encrypted_key": base64.StdEncoding.EncodeToString(append([]byte("DPAPI"), enc...)),
	}})
	for name, content := range map[string]string{
		"Local State":             string(ls),
		"Default/Network/Cookies": cookie,
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEndToEndSyncAcrossDevices(t *testing.T) {
	ctx := context.Background()
	srv := newServer(t)
	token := srv.AddUser("u1")
	me := space.PersonalPrefix + "u1"

	// 设备 A: 首次开通端到端加密, 拿到恢复码
	a := newDevice(t, srv, token, "Windows · A")
	if err := a.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	if st := a.svc.RecheckKeys(ctx); st.Keys != KeysNotEnrolled {
		t.Fatalf("keys = %s, want not_enrolled", st.Keys)
	}
	code, err := a.svc.Enroll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a.svc.Refresh(ctx)
	if err := a.svc.SwitchSpace(me); err != nil {
		t.Fatal(err)
	}

	// 在个人空间里建代理与实例
	if err := a.proxies.Upsert(browser.Proxy{ProxyId: "px1", ProxyName: "美国住宅", ProxyConfig: "socks5://u:p@1.2.3.4:1080"}); err != nil {
		t.Fatal(err)
	}
	p := &browser.Profile{ProfileId: "pf1", ProfileName: "店铺 A 账号", ProxyId: "px1", Tags: []string{"店铺"}}
	if err := a.profiles.Upsert(p); err != nil {
		t.Fatal(err)
	}
	if err := a.bookmark.ReplaceAll([]config.BrowserBookmark{{Name: "后台", URL: "https://seller.example.com/"}}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	if n, _ := a.store.PendingCount(me); n != 0 {
		t.Fatalf("pending after push = %d", n)
	}

	// 服务端只存密文: 业务内容不出现在存储的文档里
	raw := srv.RawDoc(synctest.PersonalKey("u1"), space.CollProfiles, "pf1")
	if raw == nil || strings.Contains(string(raw.Payload), "店铺") {
		t.Fatalf("server stored plaintext: %+v", raw)
	}

	// 登录态: A 上用过实例后打包上传
	writeLoginState(t, filepath.Join(a.userData, "pf1"), "cookies-from-A")
	if err := a.svc.packState(me, "pf1"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)

	// 设备 B: 恢复码解锁后拉到全部数据
	b := newDevice(t, srv, token, "Windows · B")
	if err := b.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	if st := b.svc.RecheckKeys(ctx); st.Keys != KeysLocked {
		t.Fatalf("keys = %s, want locked", st.Keys)
	}
	if err := b.svc.UnlockWithRecoveryCode(ctx, "0000-0000"); err == nil {
		t.Fatal("bad recovery code accepted")
	}
	if err := b.svc.UnlockWithRecoveryCode(ctx, code); err != nil {
		t.Fatal(err)
	}
	b.svc.Refresh(ctx)
	mustSync(t, b, me)
	if err := b.svc.SwitchSpace(me); err != nil {
		t.Fatal(err)
	}
	got, err := b.profiles.GetById("pf1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileName != "店铺 A 账号" || got.Space != me || got.ProxyId != "px1" || got.UserDataDir != "pf1" {
		t.Fatalf("profile on B = %+v", got)
	}
	proxies, _ := b.proxies.List()
	if len(proxies) != 2 || proxies[0].ProxyId != browser.DirectProxyID || proxies[1].ProxyConfig != "socks5://u:p@1.2.3.4:1080" {
		t.Fatalf("proxies on B = %+v", proxies)
	}
	if bm, _ := b.bookmark.List(); len(bm) != 1 || bm[0].URL != "https://seller.example.com/" {
		t.Fatalf("bookmarks on B = %+v", bm)
	}
	if n, _ := b.store.PendingCount(me); n != 0 {
		t.Fatalf("B pending after pull = %d (pulled rows must be clean)", n)
	}

	// B 启动前下载 A 的登录态
	if _, err := b.svc.PrepareLaunch(ctx, "pf1", false); err != nil {
		t.Fatal(err)
	}
	cookie, err := os.ReadFile(filepath.Join(b.userData, "pf1", "Default", "Network", "Cookies"))
	if err != nil || string(cookie) != "cookies-from-A" {
		t.Fatalf("login state not restored on B: %q %v", cookie, err)
	}
	// 已应用的版本不再重复下载
	row, _ := b.store.State("pf1")
	if row == nil || row.AppliedRev != row.Rev {
		t.Fatalf("applied rev not recorded: %+v", row)
	}

	// B 用过后上传, A 再启动时拿到 B 的登录态
	writeLoginState(t, filepath.Join(b.userData, "pf1"), "cookies-from-B")
	if err := b.svc.packState(me, "pf1"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, b, me)
	mustSync(t, a, me)
	if _, err := a.svc.PrepareLaunch(ctx, "pf1", false); err != nil {
		t.Fatal(err)
	}
	cookie, _ = os.ReadFile(filepath.Join(a.userData, "pf1", "Default", "Network", "Cookies"))
	if string(cookie) != "cookies-from-B" {
		t.Fatalf("A did not get B's login state: %q", cookie)
	}

	// B 改名, A 拉到; 没改的再保存不标脏
	got.ProfileName = "店铺 A 主号"
	got.UpdatedAt = time.Now().Add(time.Second).Format(time.RFC3339)
	if err := b.profiles.Upsert(got); err != nil {
		t.Fatal(err)
	}
	if err := b.profiles.Upsert(got); err != nil {
		t.Fatal(err)
	}
	mustSync(t, b, me)
	mustSync(t, a, me)
	if p, _ := a.profiles.GetById("pf1"); p == nil || p.ProfileName != "店铺 A 主号" {
		t.Fatalf("rename not pulled on A: %+v", p)
	}

	// A 删除, B 拉到墓碑
	if err := a.profiles.Delete("pf1"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	mustSync(t, b, me)
	if _, err := b.profiles.GetById("pf1"); err == nil {
		t.Fatal("deleted profile still on B")
	}
}

func TestReadOnlySpaceRejectsLocalWrites(t *testing.T) {
	ctx := context.Background()
	srv := newServer(t)
	token := srv.AddUser("u1")
	me := space.PersonalPrefix + "u1"
	a := newDevice(t, srv, token, "A")
	if err := a.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	a.svc.RecheckKeys(ctx)
	if _, err := a.svc.Enroll(ctx); err != nil {
		t.Fatal(err)
	}
	srv.SetReadOnly(synctest.PersonalKey("u1"), true)
	a.svc.Refresh(ctx)
	if err := a.svc.SwitchSpace(me); err != nil {
		t.Fatal(err)
	}
	err := a.profiles.Upsert(&browser.Profile{ProfileId: "x", ProfileName: "x"})
	if err == nil {
		t.Fatal("write into read-only space accepted")
	}
	if err := a.svc.SwitchSpace(space.LocalSpace); err != nil {
		t.Fatal(err)
	}
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "y", ProfileName: "y"}); err != nil {
		t.Fatalf("local space should stay writable: %v", err)
	}
}

// 没开通端到端加密时同步空间只读 (写进去也推不上去, 只会卡在本机); 开通后恢复可写。
// 另一台电脑开通过、这台没解锁时同样只读。
func TestSpacesReadOnlyUntilKeysUnlocked(t *testing.T) {
	ctx := context.Background()
	srv := newServer(t)
	token := srv.AddUser("u1")
	me := space.PersonalPrefix + "u1"
	a := newDevice(t, srv, token, "A")
	if err := a.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	a.svc.RecheckKeys(ctx)
	if got := a.svc.State().Keys; got != KeysNotEnrolled {
		t.Fatalf("keys = %s, want not_enrolled", got)
	}
	if err := a.svc.SwitchSpace(me); err != nil {
		t.Fatal(err)
	}
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "x", ProfileName: "x"}); err == nil {
		t.Fatal("write accepted before enrolling")
	}
	if _, err := a.svc.Enroll(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "x", ProfileName: "x"}); err != nil {
		t.Fatalf("write rejected after enrolling: %v", err)
	}

	// 第二台电脑: 账号已开通, 本机没有私钥 → locked, 只读
	b := newDevice(t, srv, token, "B")
	if err := b.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	b.svc.RecheckKeys(ctx)
	if got := b.svc.State().Keys; got != KeysLocked {
		t.Fatalf("device B keys = %s, want locked", got)
	}
	if err := b.svc.SwitchSpace(me); err != nil {
		t.Fatal(err)
	}
	if err := b.profiles.Upsert(&browser.Profile{ProfileId: "y", ProfileName: "y"}); err == nil {
		t.Fatal("write accepted on a locked device")
	}
}

func TestTeamMemberGetsKeysFromAdmin(t *testing.T) {
	ctx := context.Background()
	srv := newServer(t)
	ownerTok := srv.AddUser("owner")
	memberTok := srv.AddUser("member")
	srv.CreateOrg("org1", "运营团队", "owner")
	srv.AddMember("org1", "member", syncsdk.RoleMember)
	team := space.OrgPrefix + "org1"

	owner := newDevice(t, srv, ownerTok, "owner-pc")
	if err := owner.svc.Start("owner"); err != nil {
		t.Fatal(err)
	}
	owner.svc.RecheckKeys(ctx)
	if _, err := owner.svc.Enroll(ctx); err != nil {
		t.Fatal(err)
	}
	owner.svc.Refresh(ctx)
	if err := owner.svc.SwitchSpace(team); err != nil {
		t.Fatal(err)
	}
	if err := owner.profiles.Upsert(&browser.Profile{ProfileId: "t1", ProfileName: "团队账号"}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, owner, team)

	member := newDevice(t, srv, memberTok, "member-pc")
	if err := member.svc.Start("member"); err != nil {
		t.Fatal(err)
	}
	member.svc.RecheckKeys(ctx)
	if _, err := member.svc.Enroll(ctx); err != nil {
		t.Fatal(err)
	}
	member.svc.Refresh(ctx)
	// 管理员还没在线授予密钥: 看得到空间, 读不到内容
	if err := member.svc.SyncNow(ctx, team); err == nil {
		t.Fatal("member synced before key grant")
	}
	if sp := member.svc.space(team); sp == nil || !sp.KeyPending {
		t.Fatalf("key pending not reported: %+v", sp)
	}
	mustSync(t, owner, team) // owner 同步时自动为新成员封装密钥
	mustSync(t, member, team)
	if err := member.svc.SwitchSpace(team); err != nil {
		t.Fatal(err)
	}
	if p, err := member.profiles.GetById("t1"); err != nil || p.ProfileName != "团队账号" {
		t.Fatalf("member did not get team profile: %+v %v", p, err)
	}
	// 普通成员不能改团队书签
	if err := member.bookmark.ReplaceAll([]config.BrowserBookmark{{Name: "x", URL: "https://x/"}}); err == nil {
		t.Fatal("member changed team bookmarks")
	}
}

// pair 建两台都已解锁、都在个人空间的设备。
func pair(t *testing.T) (a, b *device, me string) {
	t.Helper()
	ctx := context.Background()
	srv := newServer(t)
	token := srv.AddUser("u1")
	me = space.PersonalPrefix + "u1"
	a = newDevice(t, srv, token, "A")
	if err := a.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	a.svc.RecheckKeys(ctx)
	code, err := a.svc.Enroll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a.svc.Refresh(ctx)
	b = newDevice(t, srv, token, "B")
	if err := b.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	b.svc.RecheckKeys(ctx)
	if err := b.svc.UnlockWithRecoveryCode(ctx, code); err != nil {
		t.Fatal(err)
	}
	b.svc.Refresh(ctx)
	for _, d := range []*device{a, b} {
		if err := d.svc.SwitchSpace(me); err != nil {
			t.Fatal(err)
		}
	}
	return a, b, me
}

// 运行期间拉到了别人更新的登录态 (没下载), 本机关闭后打的包基于旧版本: 推送必须冲突并放弃本机包, 不能覆盖别人的。
func TestStaleLoginStateDoesNotOverwriteNewer(t *testing.T) {
	ctx := context.Background()
	a, b, me := pair(t)
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "pf", ProfileName: "x"}); err != nil {
		t.Fatal(err)
	}
	writeLoginState(t, filepath.Join(a.userData, "pf"), "v1-from-A")
	if err := a.svc.packState(me, "pf"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	mustSync(t, b, me)
	if _, err := b.svc.PrepareLaunch(ctx, "pf", false); err != nil { // B 用的是 v1
		t.Fatal(err)
	}

	// B 运行期间, A 又上传了 v2; B 拉到了但没下载 (实例在运行)
	writeLoginState(t, filepath.Join(a.userData, "pf"), "v2-from-A")
	if err := a.svc.packState(me, "pf"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	mustSync(t, b, me)

	// B 关闭, 基于 v1 打包上传
	writeLoginState(t, filepath.Join(b.userData, "pf"), "v1-edited-on-B")
	if err := b.svc.packState(me, "pf"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, b, me)
	if n, _ := b.store.PendingCount(me); n != 0 {
		t.Fatalf("stale pack still pending: %d", n)
	}
	// A 再拉: 服务端仍是 A 的 v2, 没被 B 的旧包盖掉
	mustSync(t, a, me)
	row, _ := a.store.State("pf")
	if row == nil || row.Rev != row.AppliedRev {
		t.Fatalf("A's state was overwritten by B's stale pack: %+v", row)
	}
	// B 下次启动拿到 v2
	if _, err := b.svc.PrepareLaunch(ctx, "pf", false); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(b.userData, "pf", "Default", "Network", "Cookies"))
	if string(got) != "v2-from-A" {
		t.Fatalf("B did not get the newer state: %q", got)
	}
}

// 删除与别人的修改冲突时以删除为准; 新建后没推送就删掉也不会被拉回来。
func TestDeleteWinsAndDoesNotResurrect(t *testing.T) {
	a, b, me := pair(t)
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "p1", ProfileName: "x"}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	mustSync(t, b, me)

	// B 改名并推送, A 同时删除
	p, err := b.profiles.GetById("p1")
	if err != nil {
		t.Fatal(err)
	}
	p.ProfileName, p.UpdatedAt = "renamed", time.Now().Add(time.Minute).Format(time.RFC3339)
	if err := b.profiles.Upsert(p); err != nil {
		t.Fatal(err)
	}
	mustSync(t, b, me)
	if err := a.profiles.Delete("p1"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	mustSync(t, b, me)
	for name, d := range map[string]*device{"A": a, "B": b} {
		if _, err := d.profiles.GetById("p1"); err == nil {
			t.Fatalf("p1 resurrected on %s", name)
		}
	}

	// 新建后立刻删除 (还没推送): 推墓碑, 不留残行, 拉取也不会写回
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "p2", ProfileName: "y"}); err != nil {
		t.Fatal(err)
	}
	if err := a.profiles.Delete("p2"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	mustSync(t, a, me)
	if _, err := a.profiles.GetById("p2"); err == nil {
		t.Fatal("p2 came back")
	}
	if n, _ := a.store.PendingCount(me); n != 0 {
		t.Fatalf("pending after delete = %d", n)
	}
}

// 同一账号: A 电脑开着实例时, B 电脑启动前拿到 40917 (本人另一台电脑在用), 并带上 A 的设备名;
// A 关掉后 B 可以正常启动。
func TestInUseOnOwnOtherDevice(t *testing.T) {
	ctx := context.Background()
	a, b, me := pair(t)
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "pf", ProfileName: "pf"}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	mustSync(t, b, me)

	a.svc.OnStarted("pf")
	time.Sleep(200 * time.Millisecond) // 等第一次心跳发出去
	mustSync(t, b, me)
	check, err := b.svc.PrepareLaunch(ctx, "pf", false)
	if !errors.Is(err, errcode.ErrProfileInUseElsewhere) {
		t.Fatalf("err = %v, want 40917", err)
	}
	if check.InUseOn != "A" {
		t.Fatalf("InUseOn = %q, want A", check.InUseOn)
	}
	// A 自己看不到自己的心跳
	if got := a.svc.Presence(me); len(got) != 0 {
		t.Fatalf("device A sees its own presence: %+v", got)
	}
	// 强制启动不受占用影响
	if _, err := b.svc.PrepareLaunch(ctx, "pf", true); err != nil {
		t.Fatalf("forced launch: %v", err)
	}
}

func stateParts(t *testing.T, d *device, profileID string) []string {
	t.Helper()
	row, err := d.store.State(profileID)
	if err != nil || row == nil {
		t.Fatalf("state %s: %+v %v", profileID, row, err)
	}
	var m StateManifest
	if err := json.Unmarshal(row.Payload, &m); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range m.Parts {
		ids = append(ids, p.BlobID)
	}
	if len(ids) == 0 {
		t.Fatal("manifest has no parts")
	}
	return ids
}

// 附件引用校正: 换了登录态后旧分片标为未引用、新分片不标; 删除实例后它的分片也标为未引用;
// 本机有待上传的登录态时不上报 (集合可能不完整)。
func TestBlobRefsReportedAfterReplaceAndDelete(t *testing.T) {
	ctx := context.Background()
	srv := newServer(t)
	token := srv.AddUser("u1")
	me := space.PersonalPrefix + "u1"
	a := newDevice(t, srv, token, "A")
	if err := a.svc.Start("u1"); err != nil {
		t.Fatal(err)
	}
	a.svc.RecheckKeys(ctx)
	if _, err := a.svc.Enroll(ctx); err != nil {
		t.Fatal(err)
	}
	a.svc.Refresh(ctx)
	if err := a.svc.SwitchSpace(me); err != nil {
		t.Fatal(err)
	}
	key := synctest.PersonalKey("u1")
	if err := a.profiles.Upsert(&browser.Profile{ProfileId: "pf", ProfileName: "x"}); err != nil {
		t.Fatal(err)
	}
	writeLoginState(t, filepath.Join(a.userData, "pf"), "v1")
	if err := a.svc.packState(me, "pf"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	v1 := stateParts(t, a, "pf")

	writeLoginState(t, filepath.Join(a.userData, "pf"), "v2-different-content")
	if err := a.svc.packState(me, "pf"); err != nil {
		t.Fatal(err)
	}
	// 待上传时不报
	if _, ok := a.svc.stateBlobRefs(me); ok {
		t.Fatal("refs reported while a login state is pending upload")
	}
	mustSync(t, a, me)
	v2 := stateParts(t, a, "pf")
	a.svc.reportBlobRefs(ctx, me)
	for _, id := range v1 {
		if !srv.BlobUnreferenced(key, id) {
			t.Fatalf("old part %s not marked unreferenced", id)
		}
	}
	for _, id := range v2 {
		if srv.BlobUnreferenced(key, id) {
			t.Fatalf("current part %s marked unreferenced", id)
		}
	}

	if err := a.profiles.Delete("pf"); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, me)
	a.svc.reportBlobRefs(ctx, me)
	for _, id := range v2 {
		if !srv.BlobUnreferenced(key, id) {
			t.Fatalf("part %s of deleted profile not marked unreferenced", id)
		}
	}
}
