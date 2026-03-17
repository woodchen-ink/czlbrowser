package launchcode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"czl-browser/backend/internal/browser"
	"czl-browser/backend/internal/logger"
)

// BrowserStarter 浏览器启动接口（由 App 层实现并注入）
type BrowserStarter interface {
	StartInstance(profileId string) (*browser.Profile, error)
}

// LaunchRequestParams 支持外部自动化透传的一次性启动参数
type LaunchRequestParams struct {
	LaunchArgs           []string `json:"launchArgs"`
	StartURLs            []string `json:"startUrls"`
	SkipDefaultStartURLs bool     `json:"skipDefaultStartUrls"`
}

// LaunchRequest POST /api/launch 的请求体
type LaunchRequest struct {
	Code string `json:"code"`
	LaunchRequestParams
}

// BrowserStarterWithParams 可选接口：支持带参数启动实例
type BrowserStarterWithParams interface {
	StartInstanceWithParams(profileId string, params LaunchRequestParams) (*browser.Profile, error)
}

// LaunchCallRecord 接口调用记录
type LaunchCallRecord struct {
	Timestamp   string              `json:"timestamp"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	ClientIP    string              `json:"clientIp"`
	Code        string              `json:"code"`
	ProfileID   string              `json:"profileId"`
	ProfileName string              `json:"profileName"`
	Params      LaunchRequestParams `json:"params"`
	OK          bool                `json:"ok"`
	Status      int                 `json:"status"`
	Error       string              `json:"error"`
	DurationMs  int64               `json:"durationMs"`
}

// LaunchServer 本地 HTTP 唤起服务
type LaunchServer struct {
	service    *LaunchCodeService
	starter    BrowserStarter
	browserMgr *browser.Manager
	port       int
	server     *http.Server
	mu         sync.Mutex
	logMu      sync.Mutex
	callLogs   []LaunchCallRecord
}

// NewLaunchServer 创建 LaunchServer
func NewLaunchServer(service *LaunchCodeService, starter BrowserStarter, mgr *browser.Manager, port int) *LaunchServer {
	return &LaunchServer{
		service:    service,
		starter:    starter,
		browserMgr: mgr,
		port:       port,
	}
}

// Start 非阻塞启动 HTTP 服务。
// 规则：
//   - port <= 0：自动分配随机可用端口
//   - port > 0：优先使用指定端口；若被占用则回退到随机可用端口
func (s *LaunchServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/launch", s.handleLaunchWithBody)
	mux.HandleFunc("/api/launch/logs", s.handleLaunchLogs)
	mux.HandleFunc("/api/launch/", s.handleLaunch)

	handler := s.localhostMiddleware(mux)

	preferredPort := s.port
	ln, port, usedFallbackRandom, err := bindLaunchListener(preferredPort)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.port = port
	s.server = &http.Server{Handler: handler}
	s.mu.Unlock()

	log := logger.New("LaunchServer")
	if preferredPort <= 0 {
		log.Info("LaunchServer 使用随机端口", logger.F("port", port))
	} else if usedFallbackRandom {
		log.Warn("LaunchServer 首选端口不可用，已切换随机端口",
			logger.F("preferred_port", preferredPort),
			logger.F("port", port),
		)
	}
	log.Info("LaunchServer 已启动", logger.F("port", port))

	go func() {
		if serveErr := s.server.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Error("LaunchServer 异常退出", logger.F("error", serveErr.Error()))
		}
	}()

	return nil
}

func bindLaunchListener(preferredPort int) (net.Listener, int, bool, error) {
	if preferredPort <= 0 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, 0, false, fmt.Errorf("自动分配端口失败: %w", err)
		}
		port, err := listenerPort(ln)
		if err != nil {
			_ = ln.Close()
			return nil, 0, false, err
		}
		return ln, port, false, nil
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(preferredPort))
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		return ln, preferredPort, false, nil
	}

	fallbackLn, fallbackErr := net.Listen("tcp", "127.0.0.1:0")
	if fallbackErr != nil {
		return nil, 0, false, fmt.Errorf("端口 %d 不可用且自动分配失败: %w", preferredPort, err)
	}
	port, portErr := listenerPort(fallbackLn)
	if portErr != nil {
		_ = fallbackLn.Close()
		return nil, 0, false, portErr
	}
	return fallbackLn, port, true, nil
}

func listenerPort(ln net.Listener) (int, error) {
	if ln == nil {
		return 0, fmt.Errorf("listener is nil")
	}
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		return tcpAddr.Port, nil
	}

	_, rawPort, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return 0, fmt.Errorf("解析监听地址失败: %w", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return 0, fmt.Errorf("解析端口失败: %w", err)
	}
	return port, nil
}

// Stop 优雅关闭（5 秒超时）
func (s *LaunchServer) Stop() error {
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()

	if srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// Port 返回实际绑定的端口
func (s *LaunchServer) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// localhostMiddleware 只允许 127.0.0.1 访问
func (s *LaunchServer) localhostMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || host != "127.0.0.1" {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok":    false,
				"error": "forbidden: only localhost is allowed",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleHealth GET /api/health
func (s *LaunchServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleLaunch GET /api/launch/{code}
func (s *LaunchServer) handleLaunch(w http.ResponseWriter, r *http.Request) {
	startAt := time.Now()
	clientIP := remoteIP(r.RemoteAddr)
	if r.Method != http.MethodGet {
		msg := "method not allowed"
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", LaunchRequestParams{}, false, http.StatusMethodNotAllowed, msg, "", "", startAt)
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/api/launch/")
	if strings.TrimSpace(code) == "" {
		msg := "launch code not found"
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", LaunchRequestParams{}, false, http.StatusNotFound, msg, "", "", startAt)
		return
	}

	profile, status, errMsg := s.launchByCode(code, LaunchRequestParams{})
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, code, LaunchRequestParams{}, false, status, errMsg, "", "", startAt)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"profileId":   profile.ProfileId,
		"profileName": profile.ProfileName,
		"pid":         profile.Pid,
		"debugPort":   profile.DebugPort,
	})
	s.appendLaunchLog(r.Method, r.URL.Path, clientIP, code, LaunchRequestParams{}, true, http.StatusOK, "", profile.ProfileId, profile.ProfileName, startAt)
}

// handleLaunchWithBody POST /api/launch
func (s *LaunchServer) handleLaunchWithBody(w http.ResponseWriter, r *http.Request) {
	startAt := time.Now()
	clientIP := remoteIP(r.RemoteAddr)
	if r.Method != http.MethodPost {
		msg := "method not allowed"
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", LaunchRequestParams{}, false, http.StatusMethodNotAllowed, msg, "", "", startAt)
		return
	}

	var req LaunchRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		msg := "invalid request body"
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", LaunchRequestParams{}, false, http.StatusBadRequest, msg, "", "", startAt)
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		msg := "code is required"
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", req.LaunchRequestParams, false, http.StatusBadRequest, msg, "", "", startAt)
		return
	}

	req.LaunchArgs = normalizeStringSlice(req.LaunchArgs)
	req.StartURLs = normalizeStringSlice(req.StartURLs)
	profile, status, errMsg := s.launchByCode(req.Code, req.LaunchRequestParams)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, req.Code, req.LaunchRequestParams, false, status, errMsg, "", "", startAt)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"profileId":   profile.ProfileId,
		"profileName": profile.ProfileName,
		"pid":         profile.Pid,
		"debugPort":   profile.DebugPort,
	})
	s.appendLaunchLog(r.Method, r.URL.Path, clientIP, req.Code, req.LaunchRequestParams, true, http.StatusOK, "", profile.ProfileId, profile.ProfileName, startAt)
}

// handleLaunchLogs GET /api/launch/logs?limit=50
func (s *LaunchServer) handleLaunchLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "method not allowed",
		})
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			if n < 1 {
				n = 1
			}
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}

	items := s.listLaunchLogs(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"items": items,
	})
}

func (s *LaunchServer) launchByCode(code string, params LaunchRequestParams) (*browser.Profile, int, string) {
	profileId, err := s.service.Resolve(strings.TrimSpace(code))
	if err != nil {
		return nil, http.StatusNotFound, "launch code not found"
	}

	var profile *browser.Profile
	if starterWithParams, ok := s.starter.(BrowserStarterWithParams); ok {
		profile, err = starterWithParams.StartInstanceWithParams(profileId, params)
	} else {
		profile, err = s.starter.StartInstance(profileId)
	}
	if err != nil {
		return nil, http.StatusInternalServerError, err.Error()
	}

	return profile, http.StatusOK, ""
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NewTestHandler 返回不含 localhost 限制的 handler，仅供测试使用
func NewTestHandler(s *LaunchServer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/launch", s.handleLaunchWithBody)
	mux.HandleFunc("/api/launch/logs", s.handleLaunchLogs)
	mux.HandleFunc("/api/launch/", s.handleLaunch)
	return mux
}

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *LaunchServer) appendLaunchLog(method, path, clientIP, code string, params LaunchRequestParams, ok bool, status int, errMsg, profileID, profileName string, startAt time.Time) {
	entry := LaunchCallRecord{
		Timestamp:   time.Now().Format(time.RFC3339),
		Method:      method,
		Path:        path,
		ClientIP:    clientIP,
		Code:        strings.TrimSpace(code),
		ProfileID:   profileID,
		ProfileName: profileName,
		Params:      params,
		OK:          ok,
		Status:      status,
		Error:       errMsg,
		DurationMs:  time.Since(startAt).Milliseconds(),
	}

	s.logMu.Lock()
	s.callLogs = append(s.callLogs, entry)
	if len(s.callLogs) > 500 {
		s.callLogs = append([]LaunchCallRecord(nil), s.callLogs[len(s.callLogs)-500:]...)
	}
	s.logMu.Unlock()

	log := logger.New("LaunchServer")
	if ok {
		log.Info("Launch API 调用", logger.F("method", method), logger.F("path", path), logger.F("code", entry.Code), logger.F("profile_id", profileID), logger.F("status", status), logger.F("duration_ms", entry.DurationMs))
		return
	}
	log.Warn("Launch API 调用失败", logger.F("method", method), logger.F("path", path), logger.F("code", entry.Code), logger.F("status", status), logger.F("error", errMsg), logger.F("duration_ms", entry.DurationMs))
}

func (s *LaunchServer) listLaunchLogs(limit int) []LaunchCallRecord {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > len(s.callLogs) {
		limit = len(s.callLogs)
	}
	if limit == 0 {
		return []LaunchCallRecord{}
	}

	out := make([]LaunchCallRecord, 0, limit)
	for i := len(s.callLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.callLogs[i])
	}
	return out
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
