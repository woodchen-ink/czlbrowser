//go:build windows

package tray

import (
	_ "embed"

	"github.com/energye/systray"
)

//go:embed icon.ico
var iconData []byte

// Callbacks 托盘回调
type Callbacks struct {
	OnShow func()
	OnQuit func()
}

// Run 启动系统托盘（阻塞，需在独立 goroutine 中调用）
func Run(cb Callbacks) {
	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTitle("CZL Browser")
		systray.SetTooltip("CZL Browser")

		mShow := systray.AddMenuItem("显示窗口", "显示主窗口")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "退出应用")

		systray.SetOnClick(func(menu systray.IMenu) {
			if cb.OnShow != nil {
				cb.OnShow()
			}
		})

		systray.SetOnDClick(func(menu systray.IMenu) {
			if cb.OnShow != nil {
				cb.OnShow()
			}
		})

		mShow.Click(func() {
			if cb.OnShow != nil {
				cb.OnShow()
			}
		})

		mQuit.Click(func() {
			systray.Quit()
			if cb.OnQuit != nil {
				cb.OnQuit()
			}
		})
	}, func() {
		// onExit: 托盘退出时什么都不做，由 OnQuit 回调处理
	})
}

// Quit 主动退出托盘循环
func Quit() {
	systray.Quit()
}
