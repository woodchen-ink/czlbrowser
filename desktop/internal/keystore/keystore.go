// Package keystore 是端到端加密私钥与空间密钥的本机存储 (实现 syncsdk.KeyStore)。
//
// 每个名称一个文件, 文件名是名称的 SHA-256 (名称里有用户 id 与空间, 不直接落盘),
// 内容经 DPAPI 按当前 Windows 用户加密: 拷到别的账号或电脑解不开。目录在 data/sync/keys, 不进备份。
package keystore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/woodchen-ink/go-web-utils/syncsdk"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/dpapi"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
)

// Store 是基于 DPAPI 文件的 syncsdk.KeyStore。并发安全。
type Store struct {
	dir string
	mu  sync.Mutex
	// protect / unprotect 测试可替换
	protect   func([]byte) ([]byte, error)
	unprotect func([]byte) ([]byte, error)
}

var _ syncsdk.KeyStore = (*Store)(nil)

// New 创建密钥库, dir 不存在时在首次写入时创建。
func New(dir string) *Store {
	return &Store{dir: dir, protect: dpapi.Protect, unprotect: dpapi.Unprotect}
}

func (s *Store) path(name string) string {
	sum := sha256.Sum256([]byte(name))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".key")
}

// Get 实现 syncsdk.KeyStore。
func (s *Store) Get(_ context.Context, name string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enc, err := os.ReadFile(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, syncsdk.ErrKeyNotFound
	}
	if err != nil {
		return nil, errcode.ErrKeyStore.Wrap(err)
	}
	raw, err := s.unprotect(enc)
	if err != nil {
		return nil, errcode.ErrKeyStore.Wrap(err)
	}
	return raw, nil
}

// Put 实现 syncsdk.KeyStore: 先写临时文件再改名, 写到一半断电不留半个文件。
func (s *Store) Put(_ context.Context, name string, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	enc, err := s.protect(secret)
	if err != nil {
		return errcode.ErrKeyStore.Wrap(err)
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return errcode.ErrKeyStore.Wrap(err)
	}
	p := s.path(name)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o600); err != nil {
		return errcode.ErrKeyStore.Wrap(err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return errcode.ErrKeyStore.Wrap(err)
	}
	return nil
}

// Delete 实现 syncsdk.KeyStore, 不存在不报错。
func (s *Store) Delete(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errcode.ErrKeyStore.Wrap(err)
	}
	return nil
}
