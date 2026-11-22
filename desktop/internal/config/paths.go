package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	// VendorDirName 是 CZL 所有桌面程序共用的厂商目录。
	VendorDirName = "CZL"
	// ProductDirName 是本产品目录, 与安装程序里的名字一致, 定下后不再改。
	ProductDirName = "CZL Browser"
)

// InstallRoot 返回正式安装的根目录 %LOCALAPPDATA%\CZL\CZL Browser。
// 程序、配置、数据库、实例数据、内核与日志都在这里。不使用漫游的 %APPDATA%,
// 环境变量缺失时直接报错, 不退回工作目录。
func InstallRoot() (string, error) {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		return "", errors.New("1001 LOCALAPPDATA not set")
	}
	return filepath.Join(base, VendorDirName, ProductDirName), nil
}

// ConfigPathIn 返回安装根目录下的配置文件路径。
func ConfigPathIn(root string) string {
	return filepath.Join(root, "config", "config.yaml")
}
