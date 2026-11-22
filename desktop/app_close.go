package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// 点窗口关闭按钮时的行为, 存在 config.yaml 的 app.close_action。
const (
	CloseToTray = "tray" // 默认: 隐藏到托盘, 实例与代理继续运行
	CloseQuit   = "quit"
	CloseAsk    = "ask"
)

// GetCloseAction 返回当前的关闭行为 (未设置时为 tray)。
func (a *App) GetCloseAction() string {
	if a.config != nil {
		switch a.config.App.CloseAction {
		case CloseQuit, CloseAsk:
			return a.config.App.CloseAction
		}
	}
	return CloseToTray
}

// SetCloseAction 修改关闭行为并保存 (设置 → 通用, 或关闭确认框里勾选「记住」)。
func (a *App) SetCloseAction(action string) error {
	switch action {
	case CloseToTray, CloseQuit, CloseAsk:
	default:
		return fmt.Errorf("未知的关闭行为: %s", action)
	}
	a.config.App.CloseAction = action
	return a.config.Save(a.mainConfigPath())
}

// beforeClose 拦截窗口关闭。ForceQuit 之后直接放行; 正在加载 / 导出备份时总是交给前端确认,
// 避免一点关闭就中断; 其余按设置的关闭行为处理, 只有 ask 才弹确认框。
func (a *App) beforeClose(ctx context.Context) bool {
	if a.forceQuit {
		return false
	}
	if !a.maintenanceMu.TryLock() {
		runtime.EventsEmit(ctx, "app:request-close")
		return true
	}
	a.maintenanceMu.Unlock()

	switch a.GetCloseAction() {
	case CloseQuit:
		// 与托盘「退出」相同: 先停实例与代理再放行关闭
		a.forceQuit = true
		a.stopRuntimeServices()
		return false
	case CloseAsk:
		runtime.EventsEmit(ctx, "app:request-close")
	default:
		runtime.WindowHide(ctx)
	}
	return true
}
