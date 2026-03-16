package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

const ipInfoQueryURL = "https://ipinfo.czl.net/api/query"
const ipTimezoneQueryURLTemplate = "http://ip-api.com/json/%s?fields=status,message,countryCode,timezone"

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

// FetchIPInfo 获取代理出口 IP 的地理信息。
// 如果代理配置中的 host 是公网 IP，直接用该 IP 查询，跳过出口 IP 检测；
// 如果是内网 IP 或域名，则通过代理请求 IP 回显服务获取出口 IP。
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

	// 尝试从代理配置中直接提取公网 IP，跳过出口 IP 检测
	exitIP := ""
	if host := extractProxyHost(src); host != "" {
		if addr, err := netip.ParseAddr(host); err == nil && isPublicIP(addr) {
			exitIP = addr.String()
			log.Debug("从代理配置直接提取公网 IP，跳过出口 IP 检测",
				logger.F("proxy_id", proxyId),
				logger.F("ip", exitIP),
			)
		}
	}

	// 如果无法从配置中提取公网 IP（内网 IP / 域名），走原流程
	if exitIP == "" {
		proxyClient, err := buildProxyHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, 20*time.Second)
		if err != nil {
			return nil, fmt.Errorf("构建代理客户端失败: %w", err)
		}

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
	}

	// 调用 ipinfo.czl.net 查询 IP 地理信息（不走代理）
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

	data, err := parseIPInfoPayload(body)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(mapStringValue(data, "timezone")) == "" {
		timezone, countryCode, tzErr := fetchTimezoneByIP(exitIP, directClient)
		if tzErr != nil {
			log.Debug("timezone fallback lookup failed",
				logger.F("ip", exitIP),
				logger.F("error", tzErr.Error()),
			)
		} else if timezone != "" {
			data["timezone"] = timezone
			if strings.TrimSpace(mapStringValue(data, "country_code")) == "" && countryCode != "" {
				data["country_code"] = countryCode
			}
			if strings.TrimSpace(mapStringValue(data, "countryCode")) == "" && countryCode != "" {
				data["countryCode"] = countryCode
			}
			log.Debug("timezone filled by ip fallback",
				logger.F("ip", exitIP),
				logger.F("timezone", timezone),
				logger.F("country_code", countryCode),
			)
		}
	}

	return data, nil
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

func fetchTimezoneByIP(ip string, client *http.Client) (string, string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", "", fmt.Errorf("empty ip")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	endpoint := fmt.Sprintf(ipTimezoneQueryURLTemplate, url.PathEscape(ip))
	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AntChrome/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("timezone lookup request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("timezone lookup read response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("timezone lookup HTTP %d: %s", resp.StatusCode, bodySnippet(body, 300))
	}
	return parseIPTimezonePayload(body)
}

func parseIPTimezonePayload(body []byte) (string, string, error) {
	var payload struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		CountryCode string `json:"countryCode"`
		Timezone    string `json:"timezone"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", fmt.Errorf("timezone lookup JSON parse failed: %w", err)
	}

	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status != "" && status != "success" {
		msg := strings.TrimSpace(payload.Message)
		if msg == "" {
			msg = bodySnippet(body, 300)
		}
		return "", "", fmt.Errorf("timezone lookup failed: %s", msg)
	}

	timezone := strings.TrimSpace(payload.Timezone)
	countryCode := strings.ToUpper(strings.TrimSpace(payload.CountryCode))
	if timezone == "" {
		return "", countryCode, fmt.Errorf("timezone missing in lookup response")
	}
	return timezone, countryCode, nil
}

// extractProxyHost 从代理配置 URL 中提取 host 部分（不含端口）。
// 支持 socks5://user:pass@1.2.3.4:1080、http://1.2.3.4:8080 等标准 URL 格式。
func extractProxyHost(src string) string {
	src = strings.TrimSpace(src)
	l := strings.ToLower(src)
	if !strings.HasPrefix(l, "socks5://") && !strings.HasPrefix(l, "http://") && !strings.HasPrefix(l, "https://") {
		return ""
	}
	u, err := url.Parse(src)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// isPublicIP 判断是否为公网 IP（非 loopback、非内网、非保留地址）。
func isPublicIP(addr netip.Addr) bool {
	if !addr.IsValid() || addr.IsLoopback() || addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	return true
}

func mapStringValue(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
