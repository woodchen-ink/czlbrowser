package backend

import (
	"czl-browser/backend/internal/auth"
	"czl-browser/backend/internal/logger"
	"fmt"
	"os/exec"
	goruntime "runtime"

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

	authURL, err := a.authMgr.StartLogin(a.ctx, func(result auth.LoginResult) {
		if a.ctx == nil {
			return
		}
		if result.Success {
			runtime.EventsEmit(a.ctx, "auth:login:success", result.State)
		} else {
			runtime.EventsEmit(a.ctx, "auth:login:error", result.Error)
		}
	})
	if err != nil {
		log.Error("启动 OAuth2 登录失败", logger.F("error", err))
		return fmt.Errorf("启动登录失败: %w", err)
	}

	// 打开系统浏览器进行授权
	if err := openSystemBrowser(authURL); err != nil {
		log.Warn("打开浏览器失败，返回 URL 供手动访问", logger.F("error", err))
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "auth:login:url", authURL)
		}
	}

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

// openSystemBrowser 打开系统默认浏览器
func openSystemBrowser(url string) error {
	switch goruntime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
