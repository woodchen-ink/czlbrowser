//go:build windows

// Package tray 提供 Windows 通知区域图标: 单击 / 双击或「显示窗口」唤出主窗口, 「退出」结束程序。
package tray

import (
	_ "embed"

	"github.com/energye/systray"
)

//go:embed icon.ico
var icon []byte

// Callbacks 是托盘菜单触发的动作, 可以为 nil。
type Callbacks struct {
	OnShow func()
	OnQuit func()
}

func call(f func()) {
	if f != nil {
		f()
	}
}

// Run 显示托盘图标并阻塞到 Quit, 需要放在单独的 goroutine 里调用。
func Run(cb Callbacks) {
	setup := func() {
		systray.SetIcon(icon)
		systray.SetTitle("CZL Browser")
		systray.SetTooltip("CZL Browser")

		show := func(systray.IMenu) { call(cb.OnShow) }
		systray.SetOnClick(show)
		systray.SetOnDClick(show)

		systray.AddMenuItem("显示窗口", "显示主窗口").Click(func() { call(cb.OnShow) })
		systray.AddSeparator()
		systray.AddMenuItem("退出", "退出 CZL Browser").Click(func() {
			systray.Quit()
			call(cb.OnQuit)
		})
	}
	systray.Run(setup, nil)
}

// Quit 移除托盘图标并结束 Run。
func Quit() { systray.Quit() }
