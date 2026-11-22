//go:build !windows

package update

import "errors"

// StartInstaller 仅 Windows 发布安装包。
func StartInstaller(path string) error {
	return errors.New("当前平台不支持在线更新")
}
