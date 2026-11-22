package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/woodchen-ink/czlbrowser/desktop/internal/browser"
)

// DashboardOverview 控制台概览: 当前空间的实例 / 代理 / 内核状态与需要处理的事项。
// 数据量是本机几十到几百条, 每次打开控制台现算。
type DashboardOverview struct {
	Instances DashboardInstances `json:"instances"`
	Proxies   DashboardProxies   `json:"proxies"`
	Cores     DashboardCores     `json:"cores"`
	// Recent 最近启动的实例 (最多 8 个, 运行中的排前面)
	Recent []DashboardRecent `json:"recent"`
	// Attention 需要处理的事项, 没有时为空数组
	Attention []DashboardAttention `json:"attention"`
	Version   string               `json:"version"`
}

type DashboardInstances struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	// Active7d 近 7 天启动过的实例数
	Active7d int `json:"active7d"`
	// Never 从没启动过的实例数
	Never int `json:"never"`
	// Errored 上次异常退出 (带错误信息) 且当前未运行
	Errored int `json:"errored"`
	// Starts7d 近 7 天每天启动过的实例数 (按最后启动日期, 下标 0 = 6 天前, 6 = 今天)
	Starts7d []int `json:"starts7d"`
}

type DashboardProxies struct {
	Total    int `json:"total"`
	Ok       int `json:"ok"`
	Failed   int `json:"failed"`
	Untested int `json:"untested"`
	// AvgLatencyMs 测速成功的平均延迟, 没有成功的为 0
	AvgLatencyMs int `json:"avgLatencyMs"`
	// InstancesOnFailed 绑定在测速失败代理上的实例数
	InstancesOnFailed int `json:"instancesOnFailed"`
}

type DashboardCores struct {
	Total          int    `json:"total"`
	DefaultName    string `json:"defaultName"`
	DefaultVersion string `json:"defaultVersion"`
	// Missing 文件缺失的内核数
	Missing int `json:"missing"`
	// FollowDefault 内核跟随默认的实例数
	FollowDefault int `json:"followDefault"`
}

type DashboardRecent struct {
	ProfileId   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	Running     bool   `json:"running"`
	LastStartAt string `json:"lastStartAt"`
	LastError   string `json:"lastError"`
	ProxyName   string `json:"proxyName"`
}

type DashboardAttention struct {
	// Level: error / warning / info
	Level string `json:"level"`
	Text  string `json:"text"`
	// Link 前端路由, 点击跳转处理
	Link string `json:"link"`
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// DashboardOverview 返回控制台概览。
func (a *App) DashboardOverview() DashboardOverview {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	out := DashboardOverview{Version: AppVersion, Recent: []DashboardRecent{}, Attention: []DashboardAttention{}}
	out.Instances.Starts7d = make([]int, 7)

	// 代理
	proxies := a.BrowserProxyList()
	proxyByID := make(map[string]browser.Proxy, len(proxies))
	failedProxy := map[string]bool{}
	latencySum, latencyN := 0, 0
	for _, p := range proxies {
		proxyByID[p.ProxyId] = p
		if p.ProxyId == browser.DirectProxyID || p.ProxyConfig == "direct://" {
			continue
		}
		out.Proxies.Total++
		switch {
		case strings.TrimSpace(p.LastTestedAt) == "":
			out.Proxies.Untested++
		case p.LastTestOk:
			out.Proxies.Ok++
			if p.LastLatencyMs > 0 {
				latencySum += int(p.LastLatencyMs)
				latencyN++
			}
		default:
			out.Proxies.Failed++
			failedProxy[p.ProxyId] = true
		}
	}
	if latencyN > 0 {
		out.Proxies.AvgLatencyMs = latencySum / latencyN
	}

	// 内核
	cores := a.browserMgr.ListCores()
	ext := map[string]browser.CoreExtendedInfo{}
	for _, e := range a.browserMgr.GetCoresExtendedInfo() {
		ext[e.CoreId] = e
	}
	coreIDs := map[string]bool{}
	out.Cores.Total = len(cores)
	for _, c := range cores {
		coreIDs[c.CoreId] = true
		if c.IsDefault {
			out.Cores.DefaultName = c.CoreName
			out.Cores.DefaultVersion = ext[c.CoreId].ChromeVersion
		}
		if !a.browserMgr.ValidateCorePath(c.CorePath).Valid {
			out.Cores.Missing++
		}
	}

	// 实例
	profiles := a.browserMgr.List()
	out.Instances.Total = len(profiles)
	for _, p := range profiles {
		if p.Running {
			out.Instances.Running++
		}
		if strings.TrimSpace(p.CoreId) == "" {
			out.Cores.FollowDefault++
		}
		started := parseTime(p.LastStartAt)
		if started.IsZero() {
			out.Instances.Never++
		} else {
			if now.Sub(started) <= 7*24*time.Hour {
				out.Instances.Active7d++
			}
			day := int(today.Sub(time.Date(started.Year(), started.Month(), started.Day(), 0, 0, 0, 0, now.Location())).Hours() / 24)
			if day >= 0 && day < 7 {
				out.Instances.Starts7d[6-day]++
			}
		}
		if !p.Running && strings.TrimSpace(p.LastError) != "" {
			out.Instances.Errored++
		}
		if p.ProxyId != "" && failedProxy[p.ProxyId] {
			out.Proxies.InstancesOnFailed++
		}
	}

	recent := make([]browser.Profile, 0, len(profiles))
	for _, p := range profiles {
		if p.Running || !parseTime(p.LastStartAt).IsZero() {
			recent = append(recent, p)
		}
	}
	sort.SliceStable(recent, func(i, j int) bool {
		if recent[i].Running != recent[j].Running {
			return recent[i].Running
		}
		return parseTime(recent[i].LastStartAt).After(parseTime(recent[j].LastStartAt))
	})
	if len(recent) > 8 {
		recent = recent[:8]
	}
	for _, p := range recent {
		name := "直连"
		if p.ProxyId != "" && p.ProxyId != browser.DirectProxyID {
			if px, ok := proxyByID[p.ProxyId]; ok {
				name = px.ProxyName
			} else {
				name = "代理已删除"
			}
		} else if strings.TrimSpace(p.ProxyConfig) != "" {
			name = "自定义代理"
		}
		out.Recent = append(out.Recent, DashboardRecent{
			ProfileId: p.ProfileId, ProfileName: p.ProfileName, Running: p.Running,
			LastStartAt: p.LastStartAt, LastError: p.LastError, ProxyName: name,
		})
	}

	// 需要处理的事项: 先错误后提醒
	add := func(level, text, link string) {
		out.Attention = append(out.Attention, DashboardAttention{Level: level, Text: text, Link: link})
	}
	if out.Cores.Total == 0 {
		add("error", "还没有浏览器内核，实例无法启动", "/browser/cores")
	} else if out.Cores.DefaultName == "" {
		add("error", "没有设置默认内核", "/browser/cores")
	}
	if out.Cores.Missing > 0 {
		add("error", strconv.Itoa(out.Cores.Missing)+" 个内核的文件缺失", "/browser/cores")
	}
	if out.Instances.Errored > 0 {
		add("warning", strconv.Itoa(out.Instances.Errored)+" 个实例上次异常退出", "/browser/list")
	}
	if out.Proxies.InstancesOnFailed > 0 {
		add("warning", strconv.Itoa(out.Proxies.InstancesOnFailed)+" 个实例绑定的代理测速失败", "/browser/proxy-pool")
	} else if out.Proxies.Failed > 0 {
		add("warning", strconv.Itoa(out.Proxies.Failed)+" 个代理测速失败", "/browser/proxy-pool")
	}
	if out.Proxies.Untested > 0 && out.Proxies.Untested == out.Proxies.Total {
		add("info", "代理还没测过速，测一次能看到延迟和出口地区", "/browser/proxy-pool")
	}
	return out
}
