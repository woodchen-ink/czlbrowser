package cloudsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/woodchen-ink/go-web-utils/syncsdk"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/space"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/syncstore"
)

// wireVersion 是线上文档格式版本。拉到比它新的说明是新版客户端写的, 跳过并记日志, 升级后游标归零重拉。
const wireVersion = 1

// wireDoc 是线上明文 (加密前) 的外壳。
type wireDoc struct {
	V    int             `json:"v"`
	Data json.RawMessage `json:"data"`
}

func encodeWire(data []byte) (json.RawMessage, error) {
	b, err := json.Marshal(wireDoc{V: wireVersion, Data: data})
	if err != nil {
		return nil, errcode.ErrPayloadCodec.Wrap(err)
	}
	return b, nil
}

func decodeWire(collection, id string, plain []byte) (json.RawMessage, error) {
	var w wireDoc
	if err := json.Unmarshal(plain, &w); err != nil {
		return nil, errcode.ErrSyncPayload.Wrap(fmt.Errorf("%s/%s: %w", collection, id, err))
	}
	if w.V < 1 || w.V > wireVersion || len(w.Data) == 0 {
		return nil, errcode.ErrSyncPayload.Wrap(fmt.Errorf("%s/%s: wire version %d", collection, id, w.V))
	}
	return w.Data, nil
}

// localKey 由服务端空间 id 生成本地空间标识。服务端个人空间一律叫 "me", 本地带上账号, 换账号登录看不到上一个人的数据。
func localKey(userID, remote string) string {
	if remote == syncsdk.PersonalSpace {
		return space.PersonalPrefix + userID
	}
	return space.OrgPrefix + remote
}

// remoteOf 由本地空间标识取服务端空间 id, 本机空间与认不出的返回 false。
func remoteOf(local string) (string, bool) {
	if strings.HasPrefix(local, space.PersonalPrefix) {
		return syncsdk.PersonalSpace, true
	}
	if id, ok := strings.CutPrefix(local, space.OrgPrefix); ok && id != "" {
		return id, true
	}
	return "", false
}

// adapter 把本地库接到同步 SDK: 出站队列、游标、拉取落地与冲突回调。SDK 传进来的空间都是服务端 id。
// user 在组装引擎时定下: 退出登录与换账号时旧引擎可能还有一页在落地, 不能落到新账号的空间里。
type adapter struct {
	s    *Service
	user string
}

var (
	_ syncsdk.Outbox      = adapter{}
	_ syncsdk.CursorStore = adapter{}
	_ syncsdk.Applier     = adapter{}
)

func (a adapter) db() *syncstore.Store       { return a.s.opts.Store }
func (a adapter) local(remote string) string { return localKey(a.user, remote) }

func outboxItem(remote string, d *syncstore.Doc) syncsdk.OutboxItem {
	return syncsdk.OutboxItem{Seq: d.Dirty, Space: remote, Collection: d.Collection, ID: d.ID, BaseRev: d.Rev, Deleted: d.Deleted}
}

// Enqueue: 本地写入时 DAO 已经给这一行分配了出站序号, 这里只回报它。内容以本地库为准。
func (a adapter) Enqueue(_ context.Context, item syncsdk.OutboxItem) (syncsdk.OutboxItem, error) {
	d, err := a.db().DocState(a.local(item.Space), item.Collection, item.ID)
	if err != nil {
		return item, err
	}
	if d != nil && d.Dirty > 0 {
		item.Seq, item.BaseRev, item.Deleted = d.Dirty, d.Rev, d.Deleted
	}
	return item, nil
}

// Pending 按出站序号取待推送的文档并转成线上明文; 登录态快照在这里上传附件。
// 内容读不出的一条记下跳过 (不阻塞其它, 也不占名额); 上传失败整体返回错误, 下次再推。
func (a adapter) Pending(ctx context.Context, remote string, limit int) ([]syncsdk.OutboxItem, error) {
	local := a.local(remote)
	skipped := 0
	for {
		docs, err := a.db().DirtyDocs(local, limit+skipped)
		if err != nil {
			return nil, err
		}
		out := make([]syncsdk.OutboxItem, 0, len(docs))
		bad := 0
		for i := range docs {
			d := &docs[i]
			item := outboxItem(remote, d)
			if !d.Deleted {
				data, err := a.outgoing(ctx, remote, d)
				if errors.Is(err, errcode.ErrSyncPayload) || errors.Is(err, errcode.ErrStateTooLarge) {
					bad++
					a.s.docFailed(local, d.Collection, d.ID, err)
					continue
				}
				if err != nil {
					return nil, err
				}
				if item.Value, err = encodeWire(data); err != nil {
					return nil, err
				}
			}
			out = append(out, item)
		}
		if bad == skipped || len(docs) < limit+skipped {
			if len(out) > limit {
				out = out[:limit]
			}
			return out, nil
		}
		skipped = bad
	}
}

// outgoing 取一条脏文档的线上内容: 登录态快照先上传包, 其余用本地行转好的内容。
func (a adapter) outgoing(ctx context.Context, remote string, d *syncstore.Doc) (json.RawMessage, error) {
	if d.Err != nil {
		return nil, errcode.ErrSyncPayload.Wrap(d.Err)
	}
	if d.Collection != space.CollStates {
		return d.Payload, nil
	}
	if d.PackPath == "" {
		return nil, errcode.ErrSyncPayload.Wrap(errors.New("state without pack"))
	}
	return a.s.uploadPack(ctx, remote, d.ID, d.PackPath)
}

// Lookup 只供 SDK 判断「本地还有没推送的修改」, 不带内容。
func (a adapter) Lookup(_ context.Context, remote, collection, id string) (*syncsdk.OutboxItem, error) {
	d, err := a.db().DocState(a.local(remote), collection, id)
	if err != nil || d == nil || d.Dirty == 0 {
		return nil, err
	}
	item := outboxItem(remote, d)
	return &item, nil
}

func (a adapter) Ack(_ context.Context, item syncsdk.OutboxItem, newRev int64) error {
	local := a.local(item.Space)
	if a.s.isForgetting(local) {
		return nil
	}
	var manifest json.RawMessage
	if item.Collection == space.CollStates && !item.Deleted {
		data, err := decodeWire(item.Collection, item.ID, item.Value)
		if err != nil {
			return err
		}
		manifest = data
	}
	pack, err := a.db().AckPushed(local, item.Collection, item.ID, item.Seq, newRev, manifest)
	removePack(pack)
	return err
}

func (a adapter) Rebase(_ context.Context, item syncsdk.OutboxItem, newBaseRev int64) error {
	if a.s.isForgetting(a.local(item.Space)) {
		return nil
	}
	return a.db().Rebase(a.local(item.Space), item.Collection, item.ID, item.Seq, item.BaseRev, newBaseRev)
}

func (a adapter) Drop(_ context.Context, item syncsdk.OutboxItem) error {
	if a.s.isForgetting(a.local(item.Space)) {
		return nil
	}
	pack, err := a.db().DropPending(a.local(item.Space), item.Collection, item.ID, item.Seq)
	removePack(pack)
	return err
}

func removePack(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("sync.pack_remove: %v", err)
	}
}

// Get 在每轮拉取开始时调用, 顺带清掉上一轮记下的「停在失败文档之前」。
func (a adapter) Get(_ context.Context, remote string) (int64, error) {
	local := a.local(remote)
	a.s.clearHold(local)
	return a.db().Cursor(local, wireVersion)
}

// Set 记下游标。本轮有文档没处理成时, 游标停在第一条失败之前, 下次从那里重拉。
func (a adapter) Set(_ context.Context, remote string, cursor int64) error {
	local := a.local(remote)
	if a.s.isForgetting(local) {
		return nil
	}
	if hold, ok := a.s.hold(local); ok && hold < cursor {
		cursor = hold
	}
	return a.db().SetCursor(local, cursor, wireVersion)
}

// Apply 让拉到的一页变更落地。只在服务端版本比本地新、且本地没有未推送修改时覆盖。
func (a adapter) Apply(_ context.Context, remote string, changes []syncsdk.Change) error {
	local := a.local(remote)
	if a.s.isForgetting(local) {
		return nil
	}
	touched := map[string]bool{}
	for _, c := range changes {
		if !syncstore.Syncable(c.Collection) {
			continue
		}
		if c.Err != nil {
			a.refsUnsafe(local, c)
			a.s.docFailed(local, c.Collection, c.ID, errcode.ErrSyncPayload.Wrap(c.Err))
			a.retryLater(local, c)
			continue
		}
		if c.Pending {
			continue
		}
		d, err := a.db().DocState(local, c.Collection, c.ID)
		if err != nil {
			return err
		}
		if d != nil && d.Rev >= c.Rev {
			continue
		}
		w := syncstore.Remote{Rev: c.Rev, Deleted: c.Deleted}
		if !c.Deleted {
			if w.Payload, err = decodeWire(c.Collection, c.ID, c.Value); err != nil {
				a.refsUnsafe(local, c)
				a.s.docFailed(local, c.Collection, c.ID, err)
				a.retryLater(local, c)
				continue
			}
		}
		err = a.db().ApplyRemote(local, c.Collection, c.ID, w)
		switch {
		case errors.Is(err, syncstore.ErrLocalChanged):
			continue
		case errors.Is(err, errcode.ErrSpaceMismatch):
			// 同一 id 已在本机另一个空间: 不挡游标, 只记日志 (复制一律换新 id, 只有手工搬过数据库才会遇到)
			a.refsUnsafe(local, c)
			a.s.docFailed(local, c.Collection, c.ID, err)
			continue
		case errors.Is(err, errcode.ErrSyncPayload):
			a.refsUnsafe(local, c)
			a.s.docFailed(local, c.Collection, c.ID, err)
			a.retryLater(local, c)
			continue
		case err != nil:
			return err
		}
		touched[c.Collection] = true
	}
	if len(touched) > 0 {
		a.s.applied(local, touched)
	}
	return nil
}

// refsUnsafe 登录态清单没落到本机时, 这个空间的附件引用集合不完整, 本次运行不再上报。
func (a adapter) refsUnsafe(local string, c syncsdk.Change) {
	if c.Collection == space.CollStates {
		a.s.markRefsUnsafe(local)
	}
}

// retryLater 让一条没落地的服务端版本下次重拉: 本地这条基线清零, 游标退到它之前。
func (a adapter) retryLater(local string, c syncsdk.Change) {
	if err := a.db().ResetRevIfClean(local, c.Collection, c.ID); err != nil {
		log.Printf("sync.retry_rev %s %s/%s: %v", local, c.Collection, c.ID, err)
	}
	if err := a.db().LowerCursor(local, c.Rev-1); err != nil {
		log.Printf("sync.retry_cursor %s: %v", local, err)
	}
	a.s.holdBefore(local, c.Rev)
}

func (a adapter) Committed(_ context.Context, c syncsdk.Commit) error {
	if a.s.isForgetting(a.local(c.Space)) || c.Deleted {
		return nil
	}
	return a.db().CommitRev(a.local(c.Space), c.Collection, c.ID, c.Rev)
}

func (a adapter) ResetSpace(_ context.Context, remote string) error {
	local := a.local(remote)
	if a.s.isForgetting(local) {
		return nil
	}
	if err := a.db().ResetSpace(local); err != nil {
		return err
	}
	a.s.applied(local, map[string]bool{space.CollProfiles: true, space.CollProxies: true, space.CollGroups: true, space.CollSettings: true})
	return nil
}

// resolve 是冲突回调: 本机删除时以删除为准, 其余以服务端版本为准, 本地修改放弃并提示。
// 浏览器的数据另存副本没有意义 (实例副本是一个新指纹、新登录态的实例), 不像单据那样留「冲突副本」。
// 登录态快照冲突时本机这次的包丢弃, 下次启动下载服务端版本, 本机旧目录留在 .prev。
func (a adapter) resolve(_ context.Context, c syncsdk.Conflict) (syncsdk.Resolution, error) {
	local := a.local(c.Local.Space)
	// 删除优先: 本机删了而服务端版本还在 (别人改过, 或本机新建的推送已到服务端而确认没回来), 以删除覆盖,
	// 否则删掉的实例会被拉回来
	if c.Local.Deleted && !c.Remote.Deleted {
		return syncsdk.Resolution{Kind: syncsdk.Overwrite}, nil
	}
	if !c.Local.Deleted && !a.s.isForgetting(local) {
		a.s.emit(EventConflict, ConflictInfo{Space: local, Collection: c.Local.Collection, ID: c.Local.ID, By: c.Remote.UpdatedBy})
	}
	return syncsdk.Resolution{Kind: syncsdk.KeepRemote}, nil
}

// ConflictInfo 是推给界面的冲突提示: 本机对 ID 的修改与他人冲突, 已采用服务端版本。
type ConflictInfo struct {
	Space      string `json:"space"`
	Collection string `json:"collection"`
	ID         string `json:"id"`
	// By 是改了服务端版本的用户标识 (服务端写入, 可信)
	By string `json:"by"`
}
