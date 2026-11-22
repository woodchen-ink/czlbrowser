package auth

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/errcode"
)

// refreshMu 串行化令牌刷新: 同步、团队、付费几路请求同时发现令牌快过期时只刷新一次。
var refreshMu sync.Mutex

// AccessToken 返回可用的访问令牌, 剩余不足 2 分钟时先刷新。供同步 / 团队 / 付费等 Connect 接口使用。
// 没登录返回 40100; 刷新被服务端拒绝 (refresh_token 失效) 返回 40104。网络错误时沿用旧令牌, 由接口自己失败重试。
func (m *Manager) AccessToken(ctx context.Context) (string, error) {
	m.mu.RLock()
	tok := m.token
	m.mu.RUnlock()
	if tok == nil || tok.AccessToken == "" {
		return "", errcode.ErrAuthRequired
	}
	if time.Now().Unix() < tok.ExpiresAt-120 {
		return tok.AccessToken, nil
	}
	refreshMu.Lock()
	defer refreshMu.Unlock()
	m.mu.RLock()
	cur := m.token
	m.mu.RUnlock()
	if cur == nil {
		return "", errcode.ErrAuthRequired
	}
	if cur.AccessToken != tok.AccessToken && time.Now().Unix() < cur.ExpiresAt-120 {
		return cur.AccessToken, nil // 别的请求刚刷新过
	}
	if err := m.RefreshToken(); err != nil {
		if isInvalidGrant(err) {
			return "", errcode.ErrAuthExpired.Wrap(err)
		}
		return cur.AccessToken, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token.AccessToken, nil
}

// InvalidateToken 在接口回 401 时调用: 令牌仍是 tok 时标记为过期, 下次 AccessToken 会刷新。
func (m *Manager) InvalidateToken(tok string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != nil && m.token.AccessToken == tok {
		m.token.ExpiresAt = 0
	}
}

// UserID 返回登录用户的 CZL Connect 用户 id, 没登录或用户信息未取到时为空。
func (m *Manager) UserID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.token == nil || m.user == nil || m.user.ID == 0 {
		return ""
	}
	return strconv.FormatInt(m.user.ID, 10)
}
