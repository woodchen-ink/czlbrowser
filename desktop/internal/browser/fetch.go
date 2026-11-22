package browser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	fetchParts    = 8
	fetchAttempts = 3
)

// fetchToFile 把 url 下载到 dst。服务器支持 Range 时分段并行下载, 每段失败可重试;
// 否则单连接下载。report(已下载, 总大小) 最多每秒调用一次, 总大小未知时为 0。
func fetchToFile(ctx context.Context, client *http.Client, src string, dst *os.File, report func(done, total int64, parallel bool)) error {
	total, ranged, err := probeSize(ctx, client, src)
	if err != nil {
		return err
	}
	var done atomic.Int64
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				report(done.Load(), total, ranged)
			case <-stop:
				return
			}
		}
	}()

	if !ranged || total <= 0 {
		return fetchRange(ctx, client, src, -1, -1, &countWriter{w: &offsetWriter{f: dst}, n: &done})
	}
	if err := dst.Truncate(total); err != nil {
		return err
	}
	size := total / fetchParts
	var wg sync.WaitGroup
	errs := make([]error, fetchParts)
	for i := 0; i < fetchParts; i++ {
		from, to := int64(i)*size, int64(i+1)*size-1
		if i == fetchParts-1 {
			to = total - 1
		}
		wg.Add(1)
		go func(i int, from, to int64) {
			defer wg.Done()
			for attempt := 1; attempt <= fetchAttempts; attempt++ {
				var got atomic.Int64
				w := &countWriter{w: &offsetWriter{f: dst, off: from}, n: &done, attempt: &got}
				err := fetchRange(ctx, client, src, from, to, w)
				if err == nil || ctx.Err() != nil {
					errs[i] = err
					return
				}
				// 本段从头重下, 已计入的进度扣回
				done.Add(-got.Load())
				errs[i] = err
				time.Sleep(time.Duration(attempt) * time.Second)
			}
		}(i, from, to)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.Join(errs...)
}

// probeSize 用 Range: bytes=0-0 探测文件大小与是否支持分段。
func probeSize(ctx context.Context, client *http.Client, src string) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, false, err
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Content-Range: bytes 0-0/12345
		if _, after, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok {
			if n, err := strconv.ParseInt(after, 10, 64); err == nil {
				return n, true, nil
			}
		}
		return 0, false, nil
	case http.StatusOK:
		return resp.ContentLength, false, nil
	}
	return 0, false, fmt.Errorf("HTTP状态码异常: %d", resp.StatusCode)
}

// fetchRange 下载 [from, to] 区间 (from < 0 表示整个文件) 写入 w。
func fetchRange(ctx context.Context, client *http.Client, src string, from, to int64, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	want := http.StatusOK
	if from >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, to))
		want = http.StatusPartialContent
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		return fmt.Errorf("HTTP状态码异常: %d", resp.StatusCode)
	}
	n, err := io.CopyBuffer(w, resp.Body, make([]byte, 256<<10))
	if err == nil && from >= 0 && n != to-from+1 {
		return fmt.Errorf("分段长度不符: 期望 %d, 实际 %d", to-from+1, n)
	}
	return err
}

// countWriter 统计写入字节: n 是总进度, attempt 是本次尝试的量 (失败重试时扣回)。
type countWriter struct {
	w       io.Writer
	n       *atomic.Int64
	attempt *atomic.Int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	k, err := c.w.Write(p)
	c.n.Add(int64(k))
	if c.attempt != nil {
		c.attempt.Add(int64(k))
	}
	return k, err
}

// offsetWriter 从 off 处顺序写文件, 多个分段可以并发写同一个文件。
type offsetWriter struct {
	f   *os.File
	off int64
}

func (w *offsetWriter) Write(p []byte) (int, error) {
	n, err := w.f.WriteAt(p, w.off)
	w.off += int64(n)
	return n, err
}
