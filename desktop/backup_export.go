package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/backup"
)

type exportResult struct{ included, skipped, files int }

// exportPackage 写出备份包: 先写到 dest.tmp, 全部成功再改名, 中途失败不留半成品。
// 实例数据目录只打包账号数据 (缓存、组件更新等可重建内容由 backup.NewProfileAwareSkip 过滤),
// data/sync 下的本机密钥与待上传包不打包。
func (a *App) exportPackage(dest string, report progressFn) (exportResult, error) {
	report(step("preparing", 8, "正在收集导出范围..."))
	// 把 WAL 里的改动写回主库文件, 否则复制出的 app.db 可能缺最近的修改
	if a.db != nil && a.db.GetConn() != nil {
		if _, err := a.db.GetConn().Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			return exportResult{}, fmt.Errorf("整理数据库失败: %w", err)
		}
	}
	scope, err := backup.BuildScope(backup.BuildOptions{DataRoot: a.dataRoot, ConfigPath: a.mainConfigPath(), Config: a.config})
	if err != nil {
		return exportResult{}, err
	}
	name := "CZL Browser"
	if a.config != nil && strings.TrimSpace(a.config.App.Name) != "" {
		name = strings.TrimSpace(a.config.App.Name)
	}
	manifest := backup.BuildManifest(scope, name, "1.0.0", time.Now())

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return exportResult{}, fmt.Errorf("创建导出目录失败: %w", err)
	}
	report(step("writing", 18, "正在创建导出文件..."))
	tmp := dest + ".tmp"
	res, err := writeBackupZip(tmp, dest, scope, manifest, report)
	if err == nil {
		err = os.Rename(tmp, dest)
	}
	if err != nil {
		os.Remove(tmp)
		return exportResult{}, err
	}
	report(step("done", 100, "导出完成"))
	return res, nil
}

func writeBackupZip(path, finalPath string, scope backup.Scope, manifest backup.Manifest, report progressFn) (res exportResult, err error) {
	f, err := os.Create(path)
	if err != nil {
		return res, fmt.Errorf("创建导出文件失败: %w", err)
	}
	zw := zip.NewWriter(f)
	defer func() {
		if cerr := zw.Close(); err == nil {
			err = cerr
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	report(step("writing", 20, "正在写入备份清单..."))
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	w, err := zw.Create("manifest.json")
	if err == nil {
		_, err = w.Write(raw)
	}
	if err != nil {
		return res, err
	}
	res.files++

	total := len(scope.Entries)
	if total == 0 {
		report(step("writing", 90, "没有可导出的目录条目"))
	}
	for i, e := range scope.Entries {
		label := componentLabel(e.Description, e.ID)
		at := func(done int) int { return 20 + done*70/max(total, 1) }
		tagged := func(p backupProgress) backupProgress {
			p.ComponentID, p.ComponentName, p.EntryIndex, p.EntryTotal = e.ID, label, i+1, total
			return p
		}
		report(tagged(step("writing", at(i), fmt.Sprintf("开始处理组件 %d/%d：%s", i+1, total, label))))

		info, statErr := os.Stat(e.SourcePath)
		switch {
		case os.IsNotExist(statErr) && !e.Required:
			res.skipped++
			report(tagged(step("writing", at(i+1), fmt.Sprintf("组件跳过：%s（源路径不存在）", label))))
			continue
		case statErr != nil:
			return res, fmt.Errorf("读取导出源失败(%s): %w", e.ID, statErr)
		case !info.IsDir() && samePathFold(e.SourcePath, finalPath):
			res.skipped++
			report(tagged(step("writing", at(i+1), fmt.Sprintf("组件跳过：%s（导出文件本身）", label))))
			continue
		}

		added := 1
		if info.IsDir() {
			added, err = zipTree(zw, e.SourcePath, e.ArchivePath, finalPath, skipFilterFor(e))
		} else {
			err = zipOneFile(zw, e.SourcePath, strings.TrimSuffix(e.ArchivePath, "/"))
		}
		if err != nil {
			return res, fmt.Errorf("写入失败(%s): %w", e.ID, err)
		}
		res.files += added
		res.included++
		report(tagged(step("writing", at(i+1), fmt.Sprintf("组件完成：%s（新增 %d 个文件）", label, added))))
	}
	return res, nil
}

func componentLabel(desc, id string) string {
	if s := strings.TrimSpace(desc); s != "" {
		return s
	}
	if s := strings.TrimSpace(id); s != "" {
		return s
	}
	return "未知模块"
}

// skipFilterFor 返回含浏览器数据的条目在打包 / 导入时要跳过的路径判断, 其它条目不过滤。
func skipFilterFor(e backup.ScopeEntry) func(string) bool {
	switch e.ID {
	case "app_data_root":
		return anySkip(skipSyncDir, backup.NewProfileAwareSkip(backup.DetectProfileRoots(e.SourcePath)))
	case "browser_user_data_root":
		return backup.NewProfileAwareSkip(backup.DetectProfileRoots(e.SourcePath))
	}
	return nil
}

// zipTree 把目录写到 zip 的 base/ 下; 跳过符号链接、导出文件自身和 skip 命中的路径。返回写入的文件数。
func zipTree(zw *zip.Writer, dir, base, exclude string, skip func(string) bool) (int, error) {
	base = strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(base)), "/")
	if base == "" {
		return 0, fmt.Errorf("archive base 不能为空")
	}
	n := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == "." || d.Type()&os.ModeSymlink != 0 || samePathFold(p, exclude) {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			_, err := zw.Create(base + "/" + rel + "/")
			return err
		}
		if err := zipOneFile(zw, p, base+"/"+rel); err != nil {
			return err
		}
		n++
		return nil
	})
	return n, err
}

// zipOneFile 以 deflate 写入一个文件, 保留修改时间与权限。
func zipOneFile(zw *zip.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name, hdr.Method = strings.TrimPrefix(filepath.ToSlash(name), "/"), zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(w, in)
	return err
}
