// Package profilestate 打包与还原实例的登录态 (Chromium 用户数据目录里的账号数据), 供云端同步在设备之间搬运。
//
// 打包沿用备份的白名单 (backup.NewProfileAwareSkip): 只留 Default / Profile N 等账号数据与 Local State,
// 缓存、组件、遥测一律不带。Chromium 的 Cookie 与密码用 Local State 里 DPAPI 加密的 os_crypt.encrypted_key 加密,
// 换一台电脑 (或换一个 Windows 用户) 解不开; 打包时用本机 DPAPI 解出原始密钥写进包内 czl-state.json,
// 还原时用目标机 DPAPI 重新加密写回 Local State。整个包随后由同步层做端到端加密, 服务端看不到密钥。
package profilestate

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/backup"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/dpapi"
)

// MaxPackBytes 是登录态包的大小上限 (压缩后)。裁剪后的账号数据通常几 MB 到几十 MB, 超过多半是扩展或站点存了大量离线数据。
const MaxPackBytes int64 = 512 << 20

// metaName 是包内的元数据文件, 放在根目录; 用户数据目录里不会有同名文件。
const metaName = "czl-state.json"

// dpapiPrefix 是 Chromium 写在 encrypted_key 前面的标记。
const dpapiPrefix = "DPAPI"

// ErrTooLarge 表示登录态超过 MaxPackBytes。
var ErrTooLarge = errors.New("profilestate: pack too large")

type meta struct {
	V int `json:"v"`
	// OSCryptKey 是 base64 的原始 AES 密钥 (已从本机 DPAPI 解出); 没有 Local State 或解不开时为空
	OSCryptKey string    `json:"osCryptKey,omitempty"`
	PackedAt   time.Time `json:"packedAt"`
}

// Result 是一次打包的结果。
type Result struct {
	Size   int64
	SHA256 string
}

// criticalNames 是登录态的关键文件: 读不出来 (被占用) 时整个打包失败, 不上传缺了它们的包,
// 否则另一台电脑还原后会掉登录。
var criticalNames = map[string]bool{
	"Local State": true, "Cookies": true, "Login Data": true, "Preferences": true, "Web Data": true,
}

// maxUnpackBytes 是解包后的总大小上限 (防止异常的包撑满磁盘)。
const maxUnpackBytes int64 = 4 << 30

// 运行期临时文件, 打包时跳过 (带上会让另一台电脑上的 Chromium 以为实例正被占用)。
var skipNames = map[string]bool{
	"SingletonLock": true, "SingletonCookie": true, "SingletonSocket": true, "lockfile": true,
}

// Pack 把 srcDir (实例用户数据目录) 打成 zip 写到 dst, 返回大小与 sha256。实例须已停止。
func Pack(srcDir, dst string) (*Result, error) {
	if st, err := os.Stat(srcDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("profilestate: source dir: %w", errOr(err, fs.ErrNotExist))
	}
	key, _ := readOSCryptKey(filepath.Join(srcDir, "Local State"))
	skip := backup.NewProfileAwareSkip(backup.DetectProfileRoots(srcDir))

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return nil, err
	}
	tmp := dst + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(tmp)
		}
	}()
	h := sha256.New()
	cw := &countWriter{w: io.MultiWriter(f, h)}
	zw := zip.NewWriter(cw)

	m := meta{V: 1, PackedAt: time.Now().UTC()}
	if len(key) > 0 {
		m.OSCryptKey = base64.StdEncoding.EncodeToString(key)
	}
	mb, _ := json.Marshal(m)
	w, err := zw.Create(metaName)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(mb); err != nil {
		return nil, err
	}

	err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// 读不到的文件 (被别的进程独占) 跳过, 不让整个打包失败
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil || rel == "." {
			return nil
		}
		if skip(rel) || skipNames[d.Name()] || strings.HasSuffix(d.Name(), ".tmp") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		if err := addFile(zw, path, filepath.ToSlash(rel)); err != nil {
			return err
		}
		if cw.n > MaxPackBytes {
			return ErrTooLarge
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if cw.n > MaxPackBytes {
		return nil, ErrTooLarge
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return nil, err
	}
	ok = true
	return &Result{Size: cw.n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func addFile(zw *zip.Writer, path, name string) error {
	src, err := os.Open(path)
	if err != nil {
		if criticalNames[filepath.Base(path)] && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("profilestate: %s is locked: %w", filepath.Base(path), err)
		}
		return nil // 缓存类文件被独占或刚被删掉, 跳过
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return nil
	}
	hdr, err := zip.FileInfoHeader(st)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

// Unpack 把 Pack 的包还原到 dstDir: 先解到旁边的临时目录、重包 Chromium 密钥, 再整目录替换;
// 原目录改名为 dstDir + ".prev" 保留一份 (上一份 .prev 删掉)。实例须已停止。
func Unpack(zipPath, dstDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	incoming := dstDir + ".incoming"
	if err := os.RemoveAll(incoming); err != nil {
		return err
	}
	if err := os.MkdirAll(incoming, 0o755); err != nil {
		return err
	}
	done := false
	defer func() {
		if !done {
			os.RemoveAll(incoming)
		}
	}()

	var m meta
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
		if total > uint64(maxUnpackBytes) {
			return ErrTooLarge
		}
		if f.Name == metaName {
			if err := readJSON(f, &m); err != nil {
				return fmt.Errorf("profilestate: meta: %w", err)
			}
			continue
		}
		if err := extract(f, incoming); err != nil {
			return err
		}
	}
	if m.V < 1 || m.V > 1 {
		return fmt.Errorf("profilestate: unsupported pack version %d", m.V)
	}
	if m.OSCryptKey != "" {
		key, err := base64.StdEncoding.DecodeString(m.OSCryptKey)
		if err != nil {
			return fmt.Errorf("profilestate: key: %w", err)
		}
		if err := writeOSCryptKey(filepath.Join(incoming, "Local State"), key); err != nil {
			return err
		}
	}

	prev := dstDir + ".prev"
	if _, err := os.Stat(dstDir); err == nil {
		if err := os.RemoveAll(prev); err != nil {
			return err
		}
		if err := os.Rename(dstDir, prev); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
		return err
	}
	if err := os.Rename(incoming, dstDir); err != nil {
		// 换不过去时把原目录挪回来, 不让实例丢数据
		_ = os.Rename(prev, dstDir)
		return err
	}
	done = true
	return nil
}

// extract 解出一个条目, 拒绝跳出目标目录的路径 (zip slip)。
func extract(f *zip.File, root string) error {
	name := filepath.FromSlash(f.Name)
	if filepath.IsAbs(name) || strings.HasPrefix(filepath.Clean(name), "..") {
		return fmt.Errorf("profilestate: bad entry %q", f.Name)
	}
	target := filepath.Join(root, name)
	if !strings.HasPrefix(target, filepath.Clean(root)+string(os.PathSeparator)) {
		return fmt.Errorf("profilestate: bad entry %q", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	// 按条目声明的大小限量读取, 声明与实际不符的包读到上限即截断报错
	if _, err := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1)); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Chtimes(target, f.Modified, f.Modified)
	return nil
}

func readJSON(f *zip.File, v any) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(v)
}

// readOSCryptKey 从 Local State 读出 os_crypt.encrypted_key 并用本机 DPAPI 解开。
func readOSCryptKey(localState string) ([]byte, error) {
	doc, err := readLocalState(localState)
	if err != nil {
		return nil, err
	}
	oc, _ := doc["os_crypt"].(map[string]any)
	enc, _ := oc["encrypted_key"].(string)
	if enc == "" {
		return nil, errors.New("profilestate: no encrypted_key")
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(raw, []byte(dpapiPrefix)) {
		return nil, errors.New("profilestate: encrypted_key without DPAPI prefix")
	}
	return dpapi.Unprotect(raw[len(dpapiPrefix):])
}

// writeOSCryptKey 用本机 DPAPI 重新加密 key 写回 Local State, 其余字段原样保留 (数字不丢精度)。
// app_bound_encrypted_key 是上一台电脑的系统级密钥, 这里解不开, 去掉让 Chromium 重建。
func writeOSCryptKey(localState string, key []byte) error {
	doc, err := readLocalState(localState)
	if err != nil {
		return fmt.Errorf("profilestate: local state: %w", err)
	}
	enc, err := dpapi.Protect(key)
	if err != nil {
		return fmt.Errorf("profilestate: protect key: %w", err)
	}
	oc, _ := doc["os_crypt"].(map[string]any)
	if oc == nil {
		oc = map[string]any{}
	}
	oc["encrypted_key"] = base64.StdEncoding.EncodeToString(append([]byte(dpapiPrefix), enc...))
	delete(oc, "app_bound_encrypted_key")
	doc["os_crypt"] = oc
	out, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(localState, out, 0o644)
}

func readLocalState(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func errOr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
