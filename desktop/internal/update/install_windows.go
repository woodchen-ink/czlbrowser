//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// StartInstaller 以静默模式启动安装包，调用方随后应退出应用。
// /WAITPID 让安装器先等待当前进程正常退出（数据库落盘）再覆盖文件，/RELAUNCH 让安装完成后重新启动应用。
func StartInstaller(path string) error {
	cmd := exec.Command(path, "/S", "/RELAUNCH", fmt.Sprintf("/WAITPID=%d", os.Getpid()))
	// 脱离当前进程的控制台与进程组，避免应用退出时连带结束安装器
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动安装程序失败: %w", err)
	}
	return cmd.Process.Release()
}
