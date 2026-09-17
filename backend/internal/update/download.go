package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// stallTimeout 下载无任何数据超过该时长即放弃当前地址，切换下一个镜像。
const stallTimeout = 30 * time.Second

// ProgressFunc 下载进度回调，total 为清单声明的安装包大小。
type ProgressFunc func(downloaded, total int64, source string)

// Download 依次尝试清单中的下载地址，下载到 dir 并校验大小与 SHA256，返回安装包路径。
// 目标文件已存在且校验通过时直接复用，不重复下载。
func Download(ctx context.Context, client *http.Client, m *Manifest, dir string, progress ProgressFunc) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建更新目录失败: %w", err)
	}
	target := filepath.Join(dir, m.FileName)
	if VerifyFile(target, m) == nil {
		return target, nil
	}

	var errs []error
	for _, u := range m.DownloadURLs {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := downloadFrom(ctx, client, u, target, m, progress); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", u, err))
			continue
		}
		return target, nil
	}
	return "", errors.Join(errs...)
}

// VerifyFile 校验安装包大小与 SHA256。
func VerifyFile(path string, m *Manifest) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() != m.Size {
		return fmt.Errorf("文件大小不符: %d != %d", info.Size(), m.Size)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if sum := hex.EncodeToString(h.Sum(nil)); sum != m.SHA256 {
		return fmt.Errorf("SHA256 不符: %s", sum)
	}
	return nil
}

func downloadFrom(ctx context.Context, client *http.Client, u, target string, m *Manifest, progress ProgressFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	part := target + ".part"
	f, err := os.Create(part)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(part)
		}
	}()

	// 停滞看门狗：长时间收不到数据时取消请求，让调用方切换镜像
	var lastRead atomic.Int64
	lastRead.Store(time.Now().UnixNano())
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Since(time.Unix(0, lastRead.Load())) > stallTimeout {
					cancel()
					return
				}
			}
		}
	}()

	h := sha256.New()
	buf := make([]byte, 64<<10)
	var written int64
	var lastReport time.Time
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			lastRead.Store(time.Now().UnixNano())
			if written+int64(n) > m.Size {
				return errors.New("下载内容超过清单声明的大小")
			}
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			h.Write(buf[:n])
			written += int64(n)
			if progress != nil && time.Since(lastReport) > 200*time.Millisecond {
				lastReport = time.Now()
				progress(written, m.Size, u)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if ctx.Err() != nil && written < m.Size {
				return fmt.Errorf("下载超时或已取消: %w", readErr)
			}
			return readErr
		}
	}

	if written != m.Size {
		return fmt.Errorf("下载不完整: %d / %d", written, m.Size)
	}
	if sum := hex.EncodeToString(h.Sum(nil)); sum != m.SHA256 {
		return fmt.Errorf("SHA256 校验失败: %s", sum)
	}
	if progress != nil {
		progress(written, m.Size, u)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(part, target); err != nil {
		return err
	}
	ok = true
	return nil
}
