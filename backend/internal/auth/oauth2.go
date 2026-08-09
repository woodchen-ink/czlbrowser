package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"czl-browser/backend/internal/logger"
	"encoding/base64"
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

// UserInfo OAuth2 用户信息。
// CZL Connect 还会下发 groups（权限分组数组），当前产品开源免费、不做等级区分，
// 故不定义该字段，由 encoding/json 直接忽略。
type UserInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
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
}

// 内置 OAuth2 常量配置
const (
	oauthClientID     = "client_79372998"
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
	appRoot     string // 可写数据根目录（token 文件落盘基准；生产指向 %LOCALAPPDATA%）
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

	// 令牌存在即视为已登录；用户信息可能尚未补全（后台异步拉取），
	// 此时返回空用户对象而非 null，前端按空对象渲染，待补全后再更新。
	if m.token == nil {
		return AuthState{LoggedIn: false}
	}

	user := m.user
	if user == nil {
		user = &UserInfo{}
	}

	return AuthState{
		LoggedIn: true,
		User:     user,
	}
}

// LoginResult 登录结果回调
type LoginResult struct {
	Success bool
	State   AuthState
	Error   string
}

// StartLogin 启动 OAuth2 登录流程，返回授权 URL。
// onResult 在异步登录完成（成功或失败）时被调用。
func (m *Manager) StartLogin(ctx context.Context, onResult func(LoginResult)) (string, error) {
	log := logger.New("Auth")

	// 关闭上一次可能残留的回调服务器
	m.shutdownCallbackServer()

	state, err := generateState()
	if err != nil {
		return "", fmt.Errorf("生成 state 失败: %w", err)
	}
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return "", fmt.Errorf("生成 code_verifier 失败: %w", err)
	}
	codeChallenge := buildCodeChallenge(codeVerifier)

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", oauthRedirectPort)

	// 启动本地回调服务器
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", oauthRedirectPort))
	if err != nil {
		return "", fmt.Errorf("回调端口 %d 被占用", oauthRedirectPort)
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

		notifyError := func(msg string) {
			if onResult != nil {
				onResult(LoginResult{Success: false, Error: msg})
			}
		}

		select {
		case code := <-codeCh:
			if err := m.exchangeToken(code, redirectURI, codeVerifier); err != nil {
				log.Error("令牌交换失败", logger.F("error", err))
				notifyError(fmt.Sprintf("令牌交换失败: %v", err))
				return
			}
			if err := m.fetchUserInfo(); err != nil {
				log.Error("获取用户信息失败", logger.F("error", err))
				notifyError(fmt.Sprintf("获取用户信息失败: %v", err))
				return
			}
			m.saveToken()
			// 统一从 GetAuthState 取快照，避免脱离读锁直接访问 m.user。
			authState := m.GetAuthState()
			log.Info("OAuth2 登录成功", logger.F("username", authState.User.Username))
			if onResult != nil {
				onResult(LoginResult{Success: true, State: authState})
			}
		case err := <-errCh:
			log.Error("OAuth2 回调失败", logger.F("error", err))
			notifyError(fmt.Sprintf("回调失败: %v", err))
		case <-time.After(5 * time.Minute):
			log.Warn("OAuth2 登录超时")
			notifyError("登录超时，请重试")
		case <-ctx.Done():
			log.Info("OAuth2 登录流程取消")
		}
	}()

	// 构建授权 URL
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", oauthClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", "read")
	query.Set("state", state)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	authURL := oauthAuthURL + "?" + query.Encode()

	log.Info("OAuth2 授权链接已生成", logger.F("url", authURL))
	return authURL, nil
}

// Shutdown 应用退出时调用，关闭回调服务器释放资源
func (m *Manager) Shutdown() {
	m.shutdownCallbackServer()
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

// exchangeToken 用授权码和 PKCE verifier 换取令牌
func (m *Manager) exchangeToken(code, redirectURI, codeVerifier string) error {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", oauthClientID)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", codeVerifier)

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

// TryRestoreSession 从本地文件同步恢复会话。
// 仅做磁盘读取与本地有效性判断，不发起任何网络请求，确保启动阶段不阻塞、
// 弱网 / 离线时也能立即判定登录态。令牌的刷新与用户信息更新交给 RefreshSessionAsync。
// 返回 true 表示本地存在有效（未过期或可刷新）的会话。
func (m *Manager) TryRestoreSession() bool {
	log := logger.New("Auth")

	if !m.loadToken() {
		return false
	}

	m.mu.RLock()
	token := m.token
	user := m.user
	m.mu.RUnlock()

	if token == nil {
		return false
	}

	// 已过期但缺少 refresh_token：无法续期，视为会话失效。
	if time.Now().Unix() >= token.ExpiresAt && token.RefreshToken == "" {
		log.Warn("本地令牌已过期且无 refresh_token，需要重新登录")
		m.deleteToken()
		return false
	}

	if user != nil {
		log.Info("会话从本地恢复", logger.F("username", user.Username))
	} else {
		log.Info("本地令牌已恢复，等待后台补全用户信息")
	}
	return true
}

// RefreshSessionAsync 在后台校验并刷新已恢复的会话：
// 令牌临近过期时刷新，再拉取最新用户信息，最终通过 onResult 回传结果。
// 仅当刷新被服务端明确拒绝（refresh_token 失效）时才清除本地会话；
// 网络抖动等临时错误不删除令牌，保留下次启动重试的机会。
func (m *Manager) RefreshSessionAsync(onResult func(LoginResult)) {
	go func() {
		log := logger.New("Auth")

		m.mu.RLock()
		token := m.token
		m.mu.RUnlock()
		if token == nil {
			return
		}

		// 令牌已过期或剩余有效期不足 5 分钟时主动刷新。
		needRefresh := time.Now().Unix() >= token.ExpiresAt-int64(5*time.Minute/time.Second)
		if needRefresh && token.RefreshToken != "" {
			if err := m.RefreshToken(); err != nil {
				log.Warn("后台刷新令牌失败", logger.F("error", err))
				// refresh_token 失效才登出，其余错误保留本地会话。
				if isInvalidGrant(err) {
					m.deleteToken()
					if onResult != nil {
						onResult(LoginResult{Success: false, Error: "登录已过期，请重新登录"})
					}
					return
				}
			}
		}

		if err := m.fetchUserInfo(); err != nil {
			log.Warn("后台获取用户信息失败，沿用本地缓存", logger.F("error", err))
			// 拉取失败不影响已恢复的本地会话；若本地也无用户信息则无可回传。
			return
		}

		m.saveToken()
		if onResult != nil {
			onResult(LoginResult{Success: true, State: m.GetAuthState()})
		}
	}()
}

// isInvalidGrant 判断刷新错误是否为 refresh_token 失效（需重新登录）。
func isInvalidGrant(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "400") ||
		strings.Contains(msg, "401")
}

// generateState 生成随机 state 参数
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateCodeVerifier 生成 PKCE code_verifier。
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// buildCodeChallenge 根据 verifier 生成 S256 challenge。
func buildCodeChallenge(codeVerifier string) string {
	sum := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
