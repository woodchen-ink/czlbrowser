package cloudsync

// 附件引用校正: 登录态按 24 MB 分片作为附件上传, 端到端加密下服务端看不到清单内容, 不知道哪些分片还在用。
// 同步成功后由空间的所有者 / 管理员把仍被引用的分片上报 (PUT blob-refs), 服务端把其余附件标记为未引用,
// 30 天后由 Connect 的清理任务删除并扣减用量。不上报的话旧登录态与已删实例的分片会一直占着空间。
//
// 上报的集合必须完整: 漏掉一个仍在用的分片, 30 天后它就会被删, 另一台电脑再也还原不了那份登录态。
// 所以只在本机确定看全了服务端清单时才上报 (见 blobRefsReady)。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/woodchen-ink/go-web-utils/syncsdk"
)

// blobRefsInterval 引用集合没变时重复上报的间隔 (让服务端把漏标的也补上)。
const blobRefsInterval = 24 * time.Hour

type blobRefsMark struct {
	Hash string    `json:"hash"`
	At   time.Time `json:"at"`
}

func blobRefsKey(local string) string { return "sync_blob_refs:" + local }

// markRefsUnsafe 记下这个空间有服务端登录态清单没落到本机 (解析失败、空间冲突等): 本次运行不再上报引用。
func (s *Service) markRefsUnsafe(local string) {
	s.mu.Lock()
	s.refsUnsafe[local] = true
	s.mu.Unlock()
}

// stateBlobRefs 从本机的登录态清单算出仍被引用的分片。ok 为 false 表示集合可能不完整, 不能上报。
func (s *Service) stateBlobRefs(local string) (refs []string, ok bool) {
	s.mu.Lock()
	unsafe := s.refsUnsafe[local]
	_, held := s.holds[local]
	s.mu.Unlock()
	if unsafe || held {
		return nil, false // 有没拉下来或没落地的服务端版本
	}
	payloads, dirty, err := s.opts.Store.StatePayloads(local)
	if err != nil || dirty > 0 {
		return nil, false // 本机有待上传的登录态: 本机清单与服务端不一致
	}
	seen := map[string]bool{}
	for _, p := range payloads {
		var m StateManifest
		if err := json.Unmarshal([]byte(p), &m); err != nil || m.V != 1 {
			return nil, false
		}
		for _, part := range m.Parts {
			if part.BlobID == "" {
				return nil, false
			}
			seen[part.BlobID] = true
		}
	}
	refs = make([]string, 0, len(seen))
	for id := range seen {
		refs = append(refs, id)
	}
	sort.Strings(refs)
	return refs, true
}

// goReportBlobRefs 在后台上报 (调用方持有 s.mu, 不能在这里取锁)。同一空间同时只跑一个, 正在跑时跳过这次。
func (s *Service) goReportBlobRefs(local string) {
	v, _ := s.refsMu.LoadOrStore(local, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	root := s.root
	if root == nil {
		return
	}
	go func() {
		if !mu.TryLock() {
			return
		}
		defer mu.Unlock()
		ctx, cancel := context.WithTimeout(root, 30*time.Second)
		defer cancel()
		s.reportBlobRefs(ctx, local)
	}()
}

// reportBlobRefs 在空间同步成功后调用: 引用集合变了、或距上次上报超过一天时上报一次。
func (s *Service) reportBlobRefs(ctx context.Context, local string) {
	remote, ok := remoteOf(local)
	sp := s.space(local)
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if !ok || sp == nil || client == nil || sp.Removed || sp.ReadOnly || s.keysStatus() != KeysUnlocked {
		return
	}
	if sp.Kind == syncsdk.SpaceTypeOrg && sp.Role != syncsdk.RoleOwner && sp.Role != syncsdk.RoleAdmin {
		return // 只有所有者 / 管理员能校正引用
	}
	refs, ok := s.stateBlobRefs(local)
	if !ok {
		return
	}
	sum := sha256.Sum256([]byte(strings.Join(refs, "\n")))
	hash := hex.EncodeToString(sum[:])
	var last blobRefsMark
	if found, err := s.opts.Store.GetJSON(blobRefsKey(local), &last); err == nil && found &&
		last.Hash == hash && time.Since(last.At) < blobRefsInterval {
		return
	}
	marked, err := client.PutBlobRefs(ctx, remote, refs)
	if err != nil {
		log.Printf("sync.blob_refs %s: %v", local, err)
		return
	}
	if marked > 0 {
		log.Printf("sync.blob_refs %s: %d 个附件标记为未引用", local, marked)
	}
	if err := s.opts.Store.SetJSON(blobRefsKey(local), blobRefsMark{Hash: hash, At: time.Now()}); err != nil {
		log.Printf("sync.blob_refs_mark %s: %v", local, err)
	}
}
