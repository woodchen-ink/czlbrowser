package backend

import (
	"czl-chrome/backend/internal/auth"
	"czl-chrome/backend/internal/logger"
	"fmt"
	"os/exec"
	goruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AuthState Wails 绑定类型别名
type AuthState = auth.AuthState
type AuthUserInfo = auth.UserInfo

// AuthGetState 获取当前认证状态
func (a *App) AuthGetState() AuthState {
	if a.authMgr == nil {
		return AuthState{LoggedIn: false}
	}
	return a.authMgr.GetAuthState()
}

// AuthLogin 发起 OAuth2 登录
func (a *App) AuthLogin() error {
	log := logger.New("Auth")

	if a.authMgr == nil {
		return fmt.Errorf("认证模块未初始化")
	}

	authURL, err := a.authMgr.StartLogin(a.ctx)
	if err != nil {
		log.Error("启动 OAuth2 登录失败", logger.F("error", err))
		return fmt.Errorf("启动登录失败: %w", err)
	}

	// 打开系统浏览器进行授权
	if err := openBrowser(authURL); err != nil {
		log.Warn("打开浏览器失败，返回 URL 供手动访问", logger.F("error", err))
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "auth:login:url", authURL)
		}
	}

	// 异步等待登录完成，通过事件通知前端
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		timeout := time.After(5 * time.Minute)

		for {
			select {
			case <-ticker.C:
				state := a.authMgr.GetAuthState()
				if state.LoggedIn {
					if a.ctx != nil {
						runtime.EventsEmit(a.ctx, "auth:login:success", state)
					}
					return
				}
			case <-timeout:
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "auth:login:timeout", nil)
				}
				return
			case <-a.ctx.Done():
				return
			}
		}
	}()

	return nil
}

// AuthLogout 登出
func (a *App) AuthLogout() {
	if a.authMgr != nil {
		a.authMgr.Logout()
	}
}

// AuthRefresh 刷新令牌并重新获取用户信息
func (a *App) AuthRefresh() (AuthState, error) {
	if a.authMgr == nil {
		return AuthState{LoggedIn: false}, fmt.Errorf("认证模块未初始化")
	}

	if err := a.authMgr.RefreshToken(); err != nil {
		return AuthState{LoggedIn: false}, err
	}

	return a.authMgr.GetAuthState(), nil
}

// openBrowser 打开系统默认浏览器
func openBrowser(url string) error {
	switch goruntime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
