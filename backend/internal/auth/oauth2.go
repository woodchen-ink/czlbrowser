package auth

import (
	"context"
	"crypto/rand"
	"czl-chrome/backend/internal/logger"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// UserInfo OAuth2 用户信息
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
	Groups   string `json:"groups"` // 逗号分隔的权限等级
}

// TokenData OAuth2 令牌数据
type TokenData struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"` // 过期时间戳（自行计算）
}

// AuthState 认证状态
type AuthState struct {
	LoggedIn bool      `json:"loggedIn"`
	User     *UserInfo `json:"user,omitempty"`
	Level    string    `json:"level"` // 当前最高等级
}

// 内置 OAuth2 常量配置
const (
	oauthClientID     = "client_79372998"
	oauthClientSecret = "1e3c0a04de4003a5bd92ec779afe2f95d1a36a387c77627b454bb2671dd90e22"
	oauthAuthURL      = "https://connect.czl.net/oauth2/authorize"
	oauthTokenURL     = "https://connect.czl.net/api/oauth2/token"
	oauthUserInfoURL  = "https://connect.czl.net/api/oauth2/userinfo"
	oauthRedirectPort = 18429
)

// Manager OAuth2 认证管理器
type Manager struct {
	token       *TokenData
	user        *UserInfo
	mu          sync.RWMutex
	appRoot     string
	tokenFile   string
	callbackSrv *http.Server // 当前回调服务器（用于重复登录时关闭上一个）
	callbackMu  sync.Mutex
}

// NewManager 创建认证管理器
func NewManager(appRoot string) *Manager {
	return &Manager{
		appRoot:   appRoot,
		tokenFile: "data/auth_token.json",
	}
}

// GetAuthState 获取当前认证状态
func (m *Manager) GetAuthState() AuthState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.user == nil || m.token == nil {
		return AuthState{LoggedIn: false}
	}

	return AuthState{
		LoggedIn: true,
		User:     m.user,
		Level:    m.parseHighestLevel(),
	}
}

// StartLogin 启动 OAuth2 登录流程，返回授权 URL
func (m *Manager) StartLogin(ctx context.Context) (string, error) {
	log := logger.New("Auth")

	// 关闭上一次可能残留的回调服务器
	m.shutdownCallbackServer()

	state, err := generateState()
	if err != nil {
		return "", fmt.Errorf("生成 state 失败: %w", err)
	}

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", oauthRedirectPort)

	// 启动本地回调服务器
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", oauthRedirectPort))
	if err != nil {
		return "", fmt.Errorf("回调端口 %d 被占用，请检查或修改 config.yaml 中的 auth.redirect_port", oauthRedirectPort)
	}

	server := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		returnedState := r.URL.Query().Get("state")
		if returnedState != state {
			http.Error(w, "state 验证失败", http.StatusBadRequest)
			errCh <- fmt.Errorf("state 不匹配")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "未收到授权码"
			}
			http.Error(w, errMsg, http.StatusBadRequest)
			errCh <- fmt.Errorf("授权失败: %s", errMsg)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>登录成功</title></head><body style="display:flex;justify-content:center;align-items:center;height:100vh;font-family:system-ui;background:#F8F7F6;color:#141413;"><div style="text-align:center"><h2>登录成功</h2><p>请返回 CZL Browser 应用</p><p style="color:#999;font-size:14px">此窗口可以关闭</p></div></body></html>`))
		codeCh <- code
	})
	server.Handler = mux

	// 保存引用，以便下次登录时关闭
	m.callbackMu.Lock()
	m.callbackSrv = server
	m.callbackMu.Unlock()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 异步等待回调并换取令牌
	go func() {
		defer m.shutdownCallbackServer()

		select {
		case code := <-codeCh:
			if err := m.exchangeToken(code, redirectURI); err != nil {
				log.Error("令牌交换失败", logger.F("error", err))
				return
			}
			if err := m.fetchUserInfo(); err != nil {
				log.Error("获取用户信息失败", logger.F("error", err))
				return
			}
			m.saveToken()
			log.Info("OAuth2 登录成功",
				logger.F("username", m.user.Username),
				logger.F("level", m.parseHighestLevel()),
			)
		case err := <-errCh:
			log.Error("OAuth2 回调失败", logger.F("error", err))
		case <-time.After(5 * time.Minute):
			log.Warn("OAuth2 登录超时")
		case <-ctx.Done():
			log.Info("OAuth2 登录流程取消")
		}
	}()

	// 构建授权 URL
	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&scope=read&state=%s",
		oauthAuthURL,
		url.QueryEscape(oauthClientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
	)

	log.Info("OAuth2 授权链接已生成", logger.F("url", authURL))
	return authURL, nil
}

// shutdownCallbackServer 关闭回调服务器
func (m *Manager) shutdownCallbackServer() {
	m.callbackMu.Lock()
	srv := m.callbackSrv
	m.callbackSrv = nil
	m.callbackMu.Unlock()

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}
}

// exchangeToken 用授权码换取令牌
func (m *Manager) exchangeToken(code, redirectURI string) error {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", oauthClientID)
	data.Set("client_secret", oauthClientSecret)
	data.Set("redirect_uri", redirectURI)

	resp, err := http.Post(oauthTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("请求令牌失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取令牌响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("令牌接口返回 %d: %s", resp.StatusCode, string(body))
	}

	var token TokenData
	if err := json.Unmarshal(body, &token); err != nil {
		return fmt.Errorf("解析令牌响应失败: %w", err)
	}

	token.ExpiresAt = time.Now().Unix() + token.ExpiresIn

	m.mu.Lock()
	m.token = &token
	m.mu.Unlock()

	return nil
}

// RefreshToken 刷新令牌
func (m *Manager) RefreshToken() error {
	m.mu.RLock()
	refreshToken := ""
	if m.token != nil {
		refreshToken = m.token.RefreshToken
	}
	m.mu.RUnlock()

	if refreshToken == "" {
		return fmt.Errorf("无可用的 refresh_token")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", oauthClientID)
	data.Set("client_secret", oauthClientSecret)

	resp, err := http.Post(oauthTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("刷新令牌失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取刷新响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("刷新令牌接口返回 %d: %s", resp.StatusCode, string(body))
	}

	var token TokenData
	if err := json.Unmarshal(body, &token); err != nil {
		return fmt.Errorf("解析刷新响应失败: %w", err)
	}

	token.ExpiresAt = time.Now().Unix() + token.ExpiresIn

	m.mu.Lock()
	m.token = &token
	m.mu.Unlock()

	m.saveToken()
	return nil
}

// fetchUserInfo 获取用户信息
func (m *Manager) fetchUserInfo() error {
	m.mu.RLock()
	accessToken := ""
	if m.token != nil {
		accessToken = m.token.AccessToken
	}
	m.mu.RUnlock()

	if accessToken == "" {
		return fmt.Errorf("无有效的 access_token")
	}

	req, err := http.NewRequest("GET", oauthUserInfoURL, nil)
	if err != nil {
		return fmt.Errorf("创建用户信息请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取用户信息失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("用户信息接口返回 %d: %s", resp.StatusCode, string(body))
	}

	var user UserInfo
	if err := json.Unmarshal(body, &user); err != nil {
		return fmt.Errorf("解析用户信息失败: %w", err)
	}

	m.mu.Lock()
	m.user = &user
	m.mu.Unlock()

	return nil
}

// Logout 登出
func (m *Manager) Logout() {
	m.mu.Lock()
	m.token = nil
	m.user = nil
	m.mu.Unlock()
	m.deleteToken()
}

// TryRestoreSession 尝试从本地文件恢复会话
func (m *Manager) TryRestoreSession() bool {
	log := logger.New("Auth")

	if !m.loadToken() {
		return false
	}

	m.mu.RLock()
	token := m.token
	m.mu.RUnlock()

	if token == nil {
		return false
	}

	// 检查 token 是否过期
	if time.Now().Unix() >= token.ExpiresAt {
		// 尝试刷新
		if err := m.RefreshToken(); err != nil {
			log.Warn("刷新令牌失败，需要重新登录", logger.F("error", err))
			m.deleteToken()
			return false
		}
	}

	// 获取用户信息
	if err := m.fetchUserInfo(); err != nil {
		log.Warn("恢复会话时获取用户信息失败", logger.F("error", err))
		m.deleteToken()
		return false
	}

	log.Info("会话恢复成功", logger.F("username", m.user.Username))
	return true
}

// parseHighestLevel 解析用户最高等级
func (m *Manager) parseHighestLevel() string {
	if m.user == nil || m.user.Groups == "" {
		return "visitor"
	}

	groups := strings.Split(m.user.Groups, ",")
	groupSet := make(map[string]bool, len(groups))
	for _, g := range groups {
		groupSet[strings.TrimSpace(g)] = true
	}

	// 按优先级从高到低检查
	levels := []string{"admin", "t5", "t4", "t3", "t2", "t1", "t0", "viewer"}
	for _, level := range levels {
		if groupSet[level] {
			return level
		}
	}

	return "visitor"
}

// generateState 生成随机 state 参数
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
