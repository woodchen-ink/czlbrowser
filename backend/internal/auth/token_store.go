package auth

import (
	"czl-browser/backend/internal/logger"
	"encoding/json"
	"os"
	"path/filepath"
)

// saveToken 将令牌保存到本地文件
func (m *Manager) saveToken() {
	m.mu.RLock()
	token := m.token
	m.mu.RUnlock()

	if token == nil {
		return
	}

	fullPath := filepath.Join(m.appRoot, m.tokenFile)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		logger.New("Auth").Error("创建令牌目录失败", logger.F("error", err))
		return
	}

	data, err := json.Marshal(token)
	if err != nil {
		logger.New("Auth").Error("序列化令牌失败", logger.F("error", err))
		return
	}

	storedData, err := encodeStoredToken(data)
	if err != nil {
		logger.New("Auth").Error("保护令牌数据失败", logger.F("error", err))
		return
	}

	if err := os.WriteFile(fullPath, storedData, 0600); err != nil {
		logger.New("Auth").Error("写入令牌文件失败", logger.F("error", err))
	}
}

// loadToken 从本地文件加载令牌
func (m *Manager) loadToken() bool {
	fullPath := filepath.Join(m.appRoot, m.tokenFile)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false
	}

	decodedData, migratedLegacyFormat, err := decodeStoredToken(data)
	if err != nil {
		return false
	}

	var token TokenData
	if err := json.Unmarshal(decodedData, &token); err != nil {
		return false
	}

	m.mu.Lock()
	m.token = &token
	m.mu.Unlock()

	// 兼容历史明文 token 文件，成功读取后按当前存储策略重写一次。
	if migratedLegacyFormat {
		m.saveToken()
	}

	return true
}

// deleteToken 删除本地令牌文件
func (m *Manager) deleteToken() {
	fullPath := filepath.Join(m.appRoot, m.tokenFile)
	os.Remove(fullPath)

	m.mu.Lock()
	m.token = nil
	m.user = nil
	m.mu.Unlock()
}
