package backend

import (
	"czl-browser/backend/internal/backup"
	"czl-browser/backend/internal/logger"
	"os"
	"path/filepath"
	"strconv"
)

// cleanupBlockedChromeExtensions 删除旧备份带入的黑名单扩展本体，保留扩展状态目录。
func cleanupBlockedChromeExtensions(userDataDir string, log *logger.Logger, profileID string) {
	profileNames := []string{"Default", "System Profile", "Guest Profile"}
	for i := 1; i <= 32; i++ {
		profileNames = append(profileNames, "Profile "+strconv.Itoa(i))
	}
	for _, profileName := range profileNames {
		extensionsRoot := filepath.Join(userDataDir, profileName, "Extensions")
		if _, err := os.Stat(extensionsRoot); err != nil {
			continue
		}
		for extensionID, extensionName := range backup.BlockedChromeExtensionNames() {
			target := filepath.Join(extensionsRoot, extensionID)
			if _, err := os.Stat(target); err != nil {
				continue
			}
			if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
				if log != nil {
					log.Warn("清理黑名单扩展失败",
						logger.F("profile_id", profileID),
						logger.F("profile_dir", profileName),
						logger.F("extension_id", extensionID),
						logger.F("extension_name", extensionName),
						logger.F("error", err.Error()))
				}
				continue
			}
			if log != nil {
				log.Info("清理黑名单扩展",
					logger.F("profile_id", profileID),
					logger.F("profile_dir", profileName),
					logger.F("extension_id", extensionID),
					logger.F("extension_name", extensionName))
			}
		}
	}
}
