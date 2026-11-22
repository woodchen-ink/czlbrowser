package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// mergeStats 统计导入时新增、跳过 (已存在且相同)、冲突 (已存在但不同, 保留本机) 的数量。
type mergeStats struct{ Imported, Skipped, Conflicts int }

// skipSyncDir 跳过 data/sync: 端到端加密的本机密钥 (DPAPI, 换电脑解不开)、附件缓存与待上传的登录态包都只属于本机。
func skipSyncDir(rel string) bool {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	return rel == "sync" || strings.HasPrefix(rel, "sync/")
}

// skipDBFiles 跳过数据目录下的数据库文件 (数据库单独按表合并)。
func skipDBFiles(rel string) bool {
	switch filepath.ToSlash(strings.TrimSpace(rel)) {
	case "app.db", "app.db-wal", "app.db-shm":
		return true
	}
	return false
}

// anySkip 合并多个跳过判断, 任一命中即跳过。
func anySkip(fns ...func(string) bool) func(string) bool {
	return func(rel string) bool {
		for _, f := range fns {
			if f != nil && f(rel) {
				return true
			}
		}
		return false
	}
}

// syncTree 把 src 目录复制到 dst。overwrite 为 false 时已存在的文件不覆盖, 只统计相同或冲突。
func syncTree(src, dst string, overwrite bool, stats *mergeStats, skip func(string) bool) error {
	if !exists(src) {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.Type()&os.ModeSymlink != 0 || (skip != nil && skip(rel)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			stats.Skipped++
			return nil
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !overwrite && exists(target) {
			same, err := sameContent(p, target)
			if err != nil {
				return err
			}
			if same {
				stats.Skipped++
			} else {
				stats.Conflicts++
			}
			return nil
		}
		if err := copyFileAtomic(p, target); err != nil {
			return err
		}
		stats.Imported++
		return nil
	})
}

// copyFileAtomic 先写到 dst.tmp 再改名, 避免中途失败留下半个文件。
func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// sameContent 逐块比较两个文件内容。
func sameContent(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if ia.Size() != ib.Size() {
		return false, nil
	}
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()
	bufA, bufB := make([]byte, 64<<10), make([]byte, 64<<10)
	for {
		na, errA := io.ReadFull(fa, bufA)
		nb, errB := io.ReadFull(fb, bufB)
		if na != nb || !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}
		if errA == io.EOF || errA == io.ErrUnexpectedEOF {
			return errB == io.EOF || errB == io.ErrUnexpectedEOF, nil
		}
		if errA != nil {
			return false, errA
		}
		if errB != nil {
			return false, errB
		}
	}
}

// clearDirExcept 删除目录下除 keep 之外的全部内容 (目录不存在时创建)。keep 里是规范化后的绝对路径。
func clearDirExcept(dir string, keep map[string]bool) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if keep[foldPath(p)] {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

// dbFiles 返回数据库文件及其 -wal / -shm, 用于清理时保留。
func dbFiles(db string) map[string]bool {
	return map[string]bool{foldPath(db): true, foldPath(db + "-wal"): true, foldPath(db + "-shm"): true}
}

func foldPath(p string) string      { return strings.ToLower(filepath.Clean(strings.TrimSpace(p))) }
func samePathFold(a, b string) bool { return foldPath(a) == foldPath(b) }
func exists(p string) bool          { _, err := os.Stat(p); return err == nil }
func pathInside(p, root string) bool {
	p, root = foldPath(p), foldPath(root)
	return p == root || strings.HasPrefix(p, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}
