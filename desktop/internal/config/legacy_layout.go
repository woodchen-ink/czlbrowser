package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// 旧版布局（安装到 C:\Program Files 时期）：
//   - 配置: %APPDATA%\CZL Browser\config.yaml
//   - 可写数据: %LOCALAPPDATA%\CZL Browser\{data,chrome,logs,...}
// 新版统一收敛到 InstallRoot()，首次启动时按条目搬迁，目标已存在的条目不覆盖。

// legacyConfigPath 返回旧版漫游目录中的配置文件路径；APPDATA 缺失时返回空串。
func legacyConfigPath() string {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, ProductDirName, "config.yaml")
}

// legacyDataRoot 返回旧版可写数据根目录；LOCALAPPDATA 缺失时返回空串。
func legacyDataRoot() string {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, ProductDirName)
}

// MigrateLegacyLayout 把旧版配置与数据搬迁到新安装根目录 root。
// 逐条目 os.Rename（同卷，瞬时完成），单条失败不影响其余条目；返回搬迁记录与失败列表供日志输出。
func MigrateLegacyLayout(root string) (moved []string, errs []error) {
	if strings.TrimSpace(root) == "" {
		return nil, []error{errors.New("1002 install root empty")}
	}

	if src := legacyConfigPath(); src != "" {
		dst := ConfigPathIn(root)
		ok, err := moveIfAbsent(src, dst)
		if err != nil {
			errs = append(errs, err)
		} else if ok {
			moved = append(moved, src)
		}
		removeIfEmpty(filepath.Dir(src))
	}

	oldRoot := legacyDataRoot()
	if oldRoot == "" || samePath(oldRoot, root) {
		return moved, errs
	}
	entries, err := os.ReadDir(oldRoot)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("1003 read legacy data root failed: %w", err))
		}
		return moved, errs
	}
	for _, entry := range entries {
		m, e := mergeMove(filepath.Join(oldRoot, entry.Name()), filepath.Join(root, entry.Name()))
		moved = append(moved, m...)
		errs = append(errs, e...)
	}
	removeIfEmpty(oldRoot)
	return moved, errs
}

// mergeMove 搬迁 src 到 dst：dst 不存在时整体重命名；两者均为目录时逐子项递归合并
// （安装器可能已预建 data/ 或解出内置 chrome/），同名文件保留 dst 不覆盖。
func mergeMove(src, dst string) (moved []string, errs []error) {
	srcInfo, srcErr := os.Lstat(src)
	dstInfo, dstErr := os.Lstat(dst)
	if srcErr == nil && dstErr == nil && srcInfo.IsDir() && dstInfo.IsDir() {
		entries, err := os.ReadDir(src)
		if err != nil {
			return nil, []error{fmt.Errorf("1003 read legacy dir failed: %s: %w", src, err)}
		}
		for _, entry := range entries {
			m, e := mergeMove(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()))
			moved = append(moved, m...)
			errs = append(errs, e...)
		}
		removeIfEmpty(src)
		return moved, errs
	}
	ok, err := moveIfAbsent(src, dst)
	if err != nil {
		return nil, []error{err}
	}
	if ok {
		moved = append(moved, src)
	}
	return moved, nil
}

// moveIfAbsent 在 src 存在且 dst 不存在时把 src 重命名为 dst。
func moveIfAbsent(src, dst string) (bool, error) {
	if _, err := os.Lstat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("1004 stat legacy path failed: %s: %w", src, err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("1005 stat target path failed: %s: %w", dst, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, fmt.Errorf("1006 create target dir failed: %s: %w", dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return false, fmt.Errorf("1007 move legacy path failed: %s -> %s: %w", src, dst, err)
	}
	return true, nil
}

// removeIfEmpty 删除空目录；非空或不存在时静默忽略。
func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
