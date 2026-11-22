package browser

import (
	"context"
	"sync"
	"time"
)

// SpeedTestFunc 测一个代理, 返回是否可用、延迟毫秒与错误说明。
type SpeedTestFunc func(proxyID string) (ok bool, latencyMs int64, errMsg string)

// 启动后等一会再测第一轮, 不和启动抢资源。
const firstSpeedRound = 10 * time.Second

// ProxySpeedScheduler 定期给当前空间的代理测速, 结果写回数据库 (列表的延迟列读它)。
type ProxySpeedScheduler struct {
	dao      ProxyDAO
	test     SpeedTestFunc
	every    time.Duration
	parallel int

	mu     sync.Mutex
	cancel context.CancelFunc
}

// NewProxySpeedScheduler 创建调度器: 每 every 一轮, 同时最多测 parallel 个。
func NewProxySpeedScheduler(dao ProxyDAO, test SpeedTestFunc, every time.Duration, parallel int) *ProxySpeedScheduler {
	if parallel <= 0 {
		parallel = 5
	}
	return &ProxySpeedScheduler{dao: dao, test: test, every: every, parallel: parallel}
}

// Start 开始定时测速; 已在运行时不重复启动。
func (s *ProxySpeedScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		wait := firstSpeedRound
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			s.round(ctx)
			wait = s.every
		}
	}()
}

// Stop 停止定时测速, 正在进行的一轮测完已开始的代理后结束。
func (s *ProxySpeedScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// RunOnce 立即在后台测一轮。
func (s *ProxySpeedScheduler) RunOnce() { go s.round(context.Background()) }

func (s *ProxySpeedScheduler) round(ctx context.Context) {
	list, err := s.dao.List()
	if err != nil {
		return
	}
	slots := make(chan struct{}, s.parallel)
	var wg sync.WaitGroup
	for _, p := range list {
		if p.ProxyConfig == "direct://" {
			continue
		}
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case slots <- struct{}{}:
		}
		wg.Add(1)
		go func(id string) {
			defer func() { <-slots; wg.Done() }()
			ok, ms, _ := s.test(id)
			_ = s.dao.UpdateSpeedResult(id, ok, ms, time.Now().Format(time.RFC3339))
		}(p.ProxyId)
	}
	wg.Wait()
}
