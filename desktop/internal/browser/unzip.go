package browser

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// unzipFlatten 解压到 dest。压缩包里所有条目都在同一个顶层目录下时 (如 Chrome-bin/...),
// 去掉这一层直接放进 dest。拒绝指向 dest 之外的条目 (zip slip)。
func unzipFlatten(archive, dest string, progress func(percent int, msg string)) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	if len(r.File) == 0 {
		return errors.New("空的压缩包")
	}

	strip := sharedTopDir(r.File)
	root := filepath.Clean(dest) + string(os.PathSeparator)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for i, f := range r.File {
		if i%50 == 0 {
			progress(i*100/len(r.File), fmt.Sprintf("正在解压文件 %d / %d...", i+1, len(r.File)))
		}
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), strip)
		if name == "" || name == "/" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, root) {
			return fmt.Errorf("非法文件路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := writeZipEntry(f, target); err != nil {
			return err
		}
	}
	progress(100, "解压完成！")
	return nil
}

// sharedTopDir 返回所有条目共同的顶层目录 (带结尾斜杠), 没有则返回空串。
func sharedTopDir(files []*zip.File) string {
	top := ""
	for _, f := range files {
		name := filepath.ToSlash(f.Name)
		first, _, nested := strings.Cut(name, "/")
		if first == "" {
			continue
		}
		if !nested && !f.FileInfo().IsDir() {
			return "" // 根目录下直接有文件
		}
		if top == "" {
			top = first
		} else if top != first {
			return ""
		}
	}
	if top == "" {
		return ""
	}
	return top + "/"
}

func writeZipEntry(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("读取压缩包文件失败 %s: %v", f.Name, err)
	}
	defer src.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()|0o200)
	if err != nil {
		return fmt.Errorf("写入文件失败 %s: %v", target, err)
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return fmt.Errorf("写入文件失败 %s: %v", target, err)
	}
	return out.Close()
}
