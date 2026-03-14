package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

const ipInfoQueryURL = "https://ipinfo.czl.net/api/query"

// ipEchoURLs 用于通过代理获取出口 IP 的服务列表，按优先级排列
var ipEchoURLs = []string{
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
	"https://icanhazip.com",
	"https://checkip.amazonaws.com",
	"http://api.ipify.org",
	"http://ifconfig.me/ip",
	"http://icanhazip.com",
	"http://checkip.amazonaws.com",
	"http://ip-api.com/line/?fields=query",
}

// FetchIPInfo 通过指定代理链路获取出口 IP，再调用 ipinfo.czl.net 查询 IP 信息。
func FetchIPInfo(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
) (map[string]interface{}, error) {
	log := logger.New("IPHealth")

	src := ""
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			src = strings.TrimSpace(item.ProxyConfig)
			break
		}
	}
	if src == "" {
		return nil, fmt.Errorf("未找到代理配置")
	}

	// 步骤 1：通过代理获取出口 IP
	proxyClient, err := buildProxyHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, 20*time.Second)
	if err != nil {
		return nil, fmt.Errorf("构建代理客户端失败: %w", err)
	}

	exitIP := ""
	echoErrors := make([]string, 0, len(ipEchoURLs))
	for _, echoURL := range ipEchoURLs {
		req, _ := http.NewRequest(http.MethodGet, echoURL, nil)
		req.Header.Set("User-Agent", "AntChrome/1.0")
		resp, err := proxyClient.Do(req)
		if err != nil {
			log.Debug("IP 回显服务请求失败", logger.F("url", echoURL), logger.F("error", err.Error()))
			echoErrors = append(echoErrors, fmt.Sprintf("%s: %v", echoURL, err))
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			exitIP = extractIPAddress(string(body))
			if exitIP != "" {
				break
			}
			echoErrors = append(echoErrors, fmt.Sprintf("%s: invalid ip response (%s)", echoURL, bodySnippet(body, 80)))
			continue
		}
		echoErrors = append(echoErrors, fmt.Sprintf("%s: HTTP %d", echoURL, resp.StatusCode))
	}
	if exitIP == "" {
		if len(echoErrors) > 0 {
			return nil, fmt.Errorf("无法通过代理获取出口 IP（%s）", strings.Join(echoErrors, "; "))
		}
		return nil, fmt.Errorf("无法通过代理获取出口 IP")
	}

	// 步骤 2：直接调用 ipinfo.czl.net 查询 IP 信息（不走代理）
	queryBody, _ := json.Marshal(map[string]string{"ip": exitIP})
	req, _ := http.NewRequest(http.MethodPost, ipInfoQueryURL, bytes.NewReader(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AntChrome/1.0")

	directClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := directClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 IP 信息查询接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 IP 信息响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("IP 信息查询 HTTP %d: %s", resp.StatusCode, bodySnippet(body, 500))
	}

	return parseIPInfoPayload(body)
}

func bodySnippet(body []byte, max int) string {
	s := strings.TrimSpace(string(body))
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func extractIPAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if addr, err := netip.ParseAddr(raw); err == nil {
		return addr.String()
	}

	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '.' || r == ':' || r == '[' || r == ']')
	})
	for _, token := range tokens {
		token = strings.Trim(token, "[]")
		if token == "" {
			continue
		}
		if addr, err := netip.ParseAddr(token); err == nil {
			return addr.String()
		}
	}
	return ""
}

func parseIPInfoPayload(body []byte) (map[string]interface{}, error) {
	// 兼容格式 A: {"success": true, "data": {...}}
	var wrapped struct {
		Success *bool                  `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Message string                 `json:"message"`
		Error   string                 `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil {
		if wrapped.Success != nil {
			if !*wrapped.Success {
				msg := strings.TrimSpace(wrapped.Message)
				if msg == "" {
					msg = strings.TrimSpace(wrapped.Error)
				}
				if msg == "" {
					msg = bodySnippet(body, 500)
				}
				return nil, fmt.Errorf("IP 信息查询失败: %s", msg)
			}
			if len(wrapped.Data) > 0 {
				return normalizeIPInfoPayload(wrapped.Data), nil
			}
		} else if len(wrapped.Data) > 0 {
			return normalizeIPInfoPayload(wrapped.Data), nil
		}
	}

	// 兼容格式 B: 直接返回 IP 信息对象
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("IP 信息 JSON 解析失败: %w", err)
	}

	if data, ok := payload["data"].(map[string]interface{}); ok && len(data) > 0 {
		return normalizeIPInfoPayload(data), nil
	}

	if looksLikeIPInfoPayload(payload) {
		return normalizeIPInfoPayload(payload), nil
	}

	return nil, fmt.Errorf("IP 信息查询失败: %s", bodySnippet(body, 500))
}

func looksLikeIPInfoPayload(payload map[string]interface{}) bool {
	if len(payload) == 0 {
		return false
	}

	if v, ok := payload["ip"]; ok && strings.TrimSpace(fmt.Sprint(v)) != "" {
		return true
	}

	keys := []string{"country", "countryCode", "country_code", "region", "city", "isp", "organization", "asn"}
	for _, key := range keys {
		if _, ok := payload[key]; ok {
			return true
		}
	}

	return false
}

func normalizeIPInfoPayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return map[string]interface{}{}
	}

	out := make(map[string]interface{}, len(payload)+8)
	for k, v := range payload {
		out[k] = v
	}

	copyAlias := func(src, dst string) {
		if _, exists := out[dst]; exists {
			return
		}
		if v, ok := out[src]; ok && v != nil {
			out[dst] = v
		}
	}

	copyAlias("countryCode", "country_code")
	copyAlias("asName", "as_name")
	copyAlias("org", "organization")

	if locationRaw, ok := out["location"]; ok {
		if location, ok := locationRaw.(map[string]interface{}); ok {
			if _, exists := out["latitude"]; !exists {
				if v, ok := location["latitude"]; ok {
					out["latitude"] = v
				}
			}
			if _, exists := out["longitude"]; !exists {
				if v, ok := location["longitude"]; ok {
					out["longitude"] = v
				}
			}
			if _, exists := out["accuracy_radius"]; !exists {
				if v, ok := location["accuracy_radius"]; ok {
					out["accuracy_radius"] = v
				} else if v, ok := location["accuracyRadius"]; ok {
					out["accuracy_radius"] = v
				}
			}
		}
	}

	return out
}
