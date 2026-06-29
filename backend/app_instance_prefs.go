package backend

import (
	"czl-browser/backend/internal/logger"
	"encoding/json"
	"os"
	"path/filepath"
)

// patchChromePreferencesForCleanStart 在启动 Chrome 前修补 Default/Preferences 文件，
// 将 exit_type 设为 "Normal"、exited_cleanly 设为 true，
// 避免 Chrome 因上次非正常退出而弹出"未正常关闭"恢复页面或不显示窗口。
func patchChromePreferencesForCleanStart(userDataDir string, log *logger.Logger, profileId string) {
	prefsPath := filepath.Join(userDataDir, "Default", "Preferences")
	data, err := os.ReadFile(prefsPath)
	if err != nil {
		// 首次启动，Preferences 文件尚不存在，无需处理
		return
	}

	var prefs map[string]interface{}
	if err := json.Unmarshal(data, &prefs); err != nil {
		log.Error("Preferences 文件解析失败，跳过修补",
			logger.F("profile_id", profileId), logger.F("error", err.Error()))
		return
	}

	modified := false

	// 修补 profile.exit_type 和 exited_cleanly
	if profileMap, ok := prefs["profile"].(map[string]interface{}); ok {
		if exitType, _ := profileMap["exit_type"].(string); exitType != "Normal" {
			profileMap["exit_type"] = "Normal"
			modified = true
		}
		if exitedCleanly, _ := profileMap["exited_cleanly"].(bool); !exitedCleanly {
			profileMap["exited_cleanly"] = true
			modified = true
		}
	} else {
		// profile 节点不存在，创建
		prefs["profile"] = map[string]interface{}{
			"exit_type":      "Normal",
			"exited_cleanly": true,
		}
		modified = true
	}

	if !modified {
		return
	}

	out, err := json.Marshal(prefs)
	if err != nil {
		log.Error("Preferences 序列化失败",
			logger.F("profile_id", profileId), logger.F("error", err.Error()))
		return
	}

	if err := os.WriteFile(prefsPath, out, 0644); err != nil {
		log.Error("Preferences 写入失败",
			logger.F("profile_id", profileId), logger.F("error", err.Error()))
	}
}

// cleanupChromeSingletonLocks 清理用户数据目录下的 Singleton 锁文件。
// Chromium 用 SingletonLock/SingletonCookie/SingletonSocket 防止同一 profile 被多进程打开，
// 这些文件（Windows 下为指向"主机名-PID"的特殊符号链接）在跨机器导入备份后会指向失效的旧主机，
// 导致 Chromium 误判"已有实例运行"或锁状态异常而启动后立即退出。
// 本实例由本程序独占管理，启动前残留的 Singleton 必为陈旧锁，直接清理即可。
func cleanupChromeSingletonLocks(userDataDir string, log *logger.Logger, profileId string) {
	for _, name := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		p := filepath.Join(userDataDir, name)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Warn("清理 Singleton 锁失败",
				logger.F("profile_id", profileId), logger.F("file", name), logger.F("error", err.Error()))
		}
	}
}
