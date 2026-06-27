package auth

import (
	"czl-browser/backend/internal/logger"
	"encoding/json"
	"os"
	"path/filepath"
)

// storedSession 落盘的会话内容，同时保存令牌与用户信息。
// 持久化用户信息是为了重启后能立即判定已登录并渲染用户态，
// 不必等待网络获取 userinfo，从而支持离线 / 弱网下的会话恢复。
type storedSession struct {
	Token *TokenData `json:"token"`
	User  *UserInfo  `json:"user,omitempty"`
}

// saveToken 将令牌与用户信息保存到本地文件
func (m *Manager) saveToken() {
	m.mu.RLock()
	session := storedSession{Token: m.token, User: m.user}
	m.mu.RUnlock()

	if session.Token == nil {
		return
	}

	fullPath := filepath.Join(m.appRoot, m.tokenFile)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		logger.New("Auth").Error("创建令牌目录失败", logger.F("error", err))
		return
	}

	data, err := json.Marshal(session)
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

	// 新格式 storedSession 同时含 token 与 user；历史格式仅含 TokenData。
	var session storedSession
	if err := json.Unmarshal(decodedData, &session); err == nil && session.Token != nil {
		m.mu.Lock()
		m.token = session.Token
		if session.User != nil {
			m.user = session.User
		}
		m.mu.Unlock()
	} else {
		// 兼容历史仅存 token 的负载。
		var token TokenData
		if err := json.Unmarshal(decodedData, &token); err != nil {
			return false
		}
		m.mu.Lock()
		m.token = &token
		m.mu.Unlock()
		migratedLegacyFormat = true
	}

	// 兼容历史明文 / 旧负载格式，成功读取后按当前存储策略重写一次。
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
