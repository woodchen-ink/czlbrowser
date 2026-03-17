package launchcode_test

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"czl-chrome/backend/internal/launchcode"
)

func TestLaunchServerStartWithAutoPort(t *testing.T) {
	svc := launchcode.NewLaunchCodeService(launchcode.NewMemoryLaunchCodeDAO())
	srv := launchcode.NewLaunchServer(svc, nil, nil, 0)

	if err := srv.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer func() {
		_ = srv.Stop()
	}()

	port := srv.Port()
	if port <= 0 {
		t.Fatalf("自动端口分配失败: got=%d", port)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
	if err != nil {
		t.Fatalf("健康检查请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("健康检查状态码错误: got=%d", resp.StatusCode)
	}
}

func TestLaunchServerFallbackToRandomPortWhenPreferredIsBusy(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占用端口失败: %v", err)
	}
	defer occupied.Close()

	busyPort := occupied.Addr().(*net.TCPAddr).Port
	svc := launchcode.NewLaunchCodeService(launchcode.NewMemoryLaunchCodeDAO())
	srv := launchcode.NewLaunchServer(svc, nil, nil, busyPort)

	if err := srv.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer func() {
		_ = srv.Stop()
	}()

	actualPort := srv.Port()
	if actualPort <= 0 {
		t.Fatalf("随机回退端口无效: got=%d", actualPort)
	}
	if actualPort == busyPort {
		t.Fatalf("期望回退到随机端口，但仍使用了被占用端口: %d", actualPort)
	}
}
