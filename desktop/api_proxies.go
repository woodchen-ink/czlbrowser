package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/proxy"
)

// ProxyValidationResult 是启动前的代理可用性检查结果。
type ProxyValidationResult struct {
	Supported bool   `json:"supported"`
	ErrorMsg  string `json:"errorMsg"`
}

// ProxyTestResult 是一次测速结果, 批量测速时也逐条以 proxy:speed:result 事件推送。
type ProxyTestResult struct {
	ProxyId   string `json:"proxyId"`
	Ok        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error"`
}

// ProxyIPHealthResult 是代理出口 IP 的地区信息 (经 ip.czl.net 查询), 也以 proxy:iphealth:result 事件推送。
type ProxyIPHealthResult struct {
	ProxyId      string                 `json:"proxyId"`
	Ok           bool                   `json:"ok"`
	Source       string                 `json:"source"`
	Error        string                 `json:"error"`
	IP           string                 `json:"ip"`
	Country      string                 `json:"country"`
	CountryCode  string                 `json:"countryCode"`
	Region       string                 `json:"region"`
	City         string                 `json:"city"`
	Timezone     string                 `json:"timezone"`
	ISP          string                 `json:"isp"`
	Organization string                 `json:"organization"`
	ASN          int64                  `json:"asn"`
	ASName       string                 `json:"asName"`
	RawData      map[string]interface{} `json:"rawData"`
	UpdatedAt    string                 `json:"updatedAt"`
}

func toTestResult(r proxy.TestResult) ProxyTestResult {
	return ProxyTestResult{ProxyId: r.ProxyId, Ok: r.Ok, LatencyMs: r.LatencyMs, Error: r.Error}
}

// ---- 列表 ----

func (a *App) BrowserProxyList() []BrowserProxy {
	return append([]BrowserProxy{}, a.getLatestProxies()...)
}

func (a *App) BrowserProxyListGroups() []string {
	groups, _ := a.browserMgr.ProxyDAO.ListGroups()
	return groups
}

// SaveBrowserProxies 用前端编辑后的整张列表替换当前空间的代理池 (顺序即排序)。
// 缺名称或地址的行丢弃; 内置直连总会保留; 订阅来源字段按是否有来源地址整理。
func (a *App) SaveBrowserProxies(list []BrowserProxy) error {
	out := make([]BrowserProxy, 0, len(list)+1)
	hasDirect := false
	for i, p := range list {
		p.ProxyName, p.ProxyConfig = strings.TrimSpace(p.ProxyName), strings.TrimSpace(p.ProxyConfig)
		if p.ProxyName == "" || p.ProxyConfig == "" {
			continue
		}
		if p.ProxyId = strings.TrimSpace(p.ProxyId); p.ProxyId == "" {
			p.ProxyId = uuid.NewString()
		}
		hasDirect = hasDirect || p.ProxyId == browser.DirectProxyID
		p.DnsServers, p.GroupName = strings.TrimSpace(p.DnsServers), strings.TrimSpace(p.GroupName)
		p.Tags = browser.NormalizeTags(p.Tags)
		p.SortOrder = i
		normalizeProxySource(&p)
		out = append(out, p)
	}
	if !hasDirect {
		out = append([]BrowserProxy{browser.DirectProxy()}, out...)
	}
	if err := a.browserMgr.ProxyDAO.ReplaceAll(out); err != nil {
		return err
	}
	a.config.Browser.Proxies = out
	return nil
}

// normalizeProxySource 没有订阅地址时清空全部来源字段; 有地址且开启自动刷新时间隔限制在 1-1440 分钟 (默认 60)。
func normalizeProxySource(p *BrowserProxy) {
	p.SourceURL = strings.TrimSpace(p.SourceURL)
	if p.SourceURL == "" {
		p.SourceID, p.SourceNamePrefix, p.SourceLastRefreshAt = "", "", ""
		p.SourceAutoRefresh, p.SourceRefreshIntervalM = false, 0
		return
	}
	p.SourceID = strings.TrimSpace(p.SourceID)
	p.SourceNamePrefix = strings.TrimSpace(p.SourceNamePrefix)
	p.SourceLastRefreshAt = strings.TrimSpace(p.SourceLastRefreshAt)
	if !p.SourceAutoRefresh {
		p.SourceRefreshIntervalM = 0
		return
	}
	switch {
	case p.SourceRefreshIntervalM <= 0:
		p.SourceRefreshIntervalM = 60
	case p.SourceRefreshIntervalM > 24*60:
		p.SourceRefreshIntervalM = 24 * 60
	}
}

// ---- 校验与测速 ----

func (a *App) ValidateProxyConfig(proxyConfig, proxyID string) ProxyValidationResult {
	ok, msg := proxy.ValidateProxyConfig(proxyConfig, a.getLatestProxies(), proxyID)
	return ProxyValidationResult{Supported: ok, ErrorMsg: msg}
}

// BrowserProxyTestSpeed 测一个代理并记录结果。
func (a *App) BrowserProxyTestSpeed(proxyID string) ProxyTestResult {
	return a.speedTestAndSave(proxyID, a.getLatestProxies())
}

func (a *App) speedTestAndSave(id string, pool []BrowserProxy) ProxyTestResult {
	r := toTestResult(proxy.SpeedTest(id, pool, nil))
	_ = a.browserMgr.ProxyDAO.UpdateSpeedResult(id, r.Ok, r.LatencyMs, time.Now().Format(time.RFC3339))
	return r
}

// BrowserProxyBatchTestSpeed 并发测速 (默认 20 个一起), 每测完一个推送一次事件。
func (a *App) BrowserProxyBatchTestSpeed(ids []string, parallel int) []ProxyTestResult {
	pool := a.getLatestProxies()
	return forEachParallel(ids, parallel, 20, func(id string) ProxyTestResult {
		r := a.speedTestAndSave(id, pool)
		a.emit("proxy:speed:result", r)
		return r
	})
}

// BrowserProxyCheckIPHealth 查一个代理的出口 IP 与地区并记录。
func (a *App) BrowserProxyCheckIPHealth(proxyID string) ProxyIPHealthResult {
	return a.checkIPHealth(proxyID, a.getLatestProxies())
}

// BrowserProxyBatchCheckIPHealth 并发查出口 (默认 10 个一起)。
func (a *App) BrowserProxyBatchCheckIPHealth(ids []string, parallel int) []ProxyIPHealthResult {
	pool := a.getLatestProxies()
	return forEachParallel(ids, parallel, 10, func(id string) ProxyIPHealthResult { return a.checkIPHealth(id, pool) })
}

func (a *App) checkIPHealth(id string, pool []BrowserProxy) ProxyIPHealthResult {
	data, err := proxy.FetchIPInfo(id, pool, a.xrayMgr, a.singboxMgr)
	r := buildProxyIPHealthResult(id, data, err)
	a.persistProxyIPHealthResult(r)
	a.emit("proxy:iphealth:result", r)
	return r
}

// forEachParallel 用固定数量的 worker 处理 ids, 结果按输入顺序返回。
func forEachParallel[T any](ids []string, parallel, def int, work func(string) T) []T {
	out := make([]T, len(ids))
	if len(ids) == 0 {
		return out
	}
	if parallel <= 0 {
		parallel = def
	}
	parallel = min(parallel, len(ids))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				out[i] = work(ids[i])
			}
		}()
	}
	for i := range ids {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

// buildProxyIPHealthResult 把查询结果整理成前端展示的字段。
func buildProxyIPHealthResult(id string, data map[string]interface{}, err error) ProxyIPHealthResult {
	r := ProxyIPHealthResult{ProxyId: id, Source: "ip.czl.net", RawData: map[string]interface{}{}, UpdatedAt: time.Now().Format(time.RFC3339)}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	if data != nil {
		r.RawData = data
	}
	r.Ok = true
	r.IP, r.Country, r.CountryCode = mapString(data, "ip"), mapString(data, "country"), mapString(data, "country_code")
	r.Region, r.City, r.Timezone = mapString(data, "region"), mapString(data, "city"), mapString(data, "timezone")
	r.ISP, r.Organization, r.ASName = mapString(data, "isp"), mapString(data, "organization"), mapString(data, "as_name")
	r.ASN = mapInt64(data, "asn")
	return r
}

// persistProxyIPHealthResult 把出口检测结果存进代理池 (本机字段)。
func (a *App) persistProxyIPHealthResult(r ProxyIPHealthResult) {
	if a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return
	}
	if raw, err := json.Marshal(r); err == nil {
		_ = a.browserMgr.ProxyDAO.UpdateIPHealthResult(r.ProxyId, string(raw))
	}
}

func mapString(m map[string]interface{}, key string) string {
	switch v := m[key].(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func mapInt64(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return int64(f)
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return int64(f)
		}
	}
	return 0
}
