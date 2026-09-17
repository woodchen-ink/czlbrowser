package backend

import (
	"context"
	"czl-browser/backend/internal/browser"
	"czl-browser/backend/internal/logger"
	"czl-browser/backend/internal/update"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// 应用在线更新：检查签名清单 → 下载并校验安装包 → 静默安装并重启
// ============================================================================

// AppVersion 应用版本号，由 CI 通过 -ldflags "-X czl-browser/backend.AppVersion=..." 注入。
// 本地构建保持 "dev"，不提示更新。
var AppVersion = "dev"

const updateDownloadDir = "data/_update"

// UpdateCheckResult 更新检查结果。
type UpdateCheckResult struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	DevBuild       bool   `json:"devBuild"`
	Changelog      string `json:"changelog"`
	PublishedAt    string `json:"publishedAt"`
	Size           int64  `json:"size"`
	Downloaded     bool   `json:"downloaded"`
	Downloading    bool   `json:"downloading"`
	CheckedAt      string `json:"checkedAt"`
}

// UpdateProgress 下载进度事件 update:progress 的负载。
type UpdateProgress struct {
	Phase      string `json:"phase"` // downloading / ready / error
	Progress   int    `json:"progress"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Message    string `json:"message"`
}

type appUpdateState struct {
	mu            sync.Mutex
	latest        *update.Manifest
	checkedAt     time.Time
	downloading   bool
	installerPath string // 已下载且校验通过的安装包，仅对应 latest
}

var appUpdate appUpdateState

func updateHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                 browser.SystemProxyURL,
		ResponseHeaderTimeout: 20 * time.Second,
	}}
}

// UpdateCheck 拉取最新版本清单并与当前版本比较。
func (a *App) UpdateCheck() (*UpdateCheckResult, error) {
	m, err := update.FetchLatest(context.Background(), updateHTTPClient())
	if err != nil {
		logger.New("Update").Warn("检查更新失败", logger.F("error", err.Error()))
		return nil, errors.New("检查更新失败，请检查网络后重试")
	}

	appUpdate.mu.Lock()
	if appUpdate.latest == nil || appUpdate.latest.Version != m.Version {
		appUpdate.installerPath = ""
	}
	appUpdate.latest = m
	appUpdate.checkedAt = time.Now()
	appUpdate.mu.Unlock()

	return a.updateCheckResult(), nil
}

func (a *App) updateCheckResult() *UpdateCheckResult {
	appUpdate.mu.Lock()
	defer appUpdate.mu.Unlock()

	devBuild := !update.IsReleaseVersion(AppVersion)
	result := &UpdateCheckResult{
		CurrentVersion: AppVersion,
		DevBuild:       devBuild,
		Downloading:    appUpdate.downloading,
		Downloaded:     appUpdate.installerPath != "",
	}
	if !appUpdate.checkedAt.IsZero() {
		result.CheckedAt = appUpdate.checkedAt.Format(time.RFC3339)
	}
	if m := appUpdate.latest; m != nil {
		result.LatestVersion = m.Version
		result.Changelog = m.Changelog
		result.PublishedAt = m.PublishedAt
		result.Size = m.Size
		result.HasUpdate = !devBuild && update.CompareVersions(m.Version, AppVersion) > 0
	}
	return result
}

// UpdateDownload 后台下载最新安装包，进度通过 update:progress 事件推送。
func (a *App) UpdateDownload() error {
	appUpdate.mu.Lock()
	m := appUpdate.latest
	switch {
	case m == nil:
		appUpdate.mu.Unlock()
		return errors.New("请先检查更新")
	case !update.IsReleaseVersion(AppVersion) || update.CompareVersions(m.Version, AppVersion) <= 0:
		appUpdate.mu.Unlock()
		return errors.New("当前已是最新版本")
	case appUpdate.downloading:
		appUpdate.mu.Unlock()
		return nil
	}
	appUpdate.downloading = true
	appUpdate.mu.Unlock()

	go a.runUpdateDownload(m)
	return nil
}

func (a *App) runUpdateDownload(m *update.Manifest) {
	log := logger.New("Update")
	emit := func(p UpdateProgress) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "update:progress", p)
		}
	}

	dir := a.resolveAppPath(updateDownloadDir)
	cleanupOldInstallers(dir, m.FileName)
	emit(UpdateProgress{Phase: "downloading", Total: m.Size, Message: "正在下载新版本安装包..."})

	path, err := update.Download(context.Background(), updateHTTPClient(), m, dir, func(downloaded, total int64, source string) {
		emit(UpdateProgress{
			Phase:      "downloading",
			Progress:   int(downloaded * 100 / total),
			Downloaded: downloaded,
			Total:      total,
			Message:    "正在下载新版本安装包...",
		})
	})

	appUpdate.mu.Lock()
	appUpdate.downloading = false
	stillLatest := appUpdate.latest != nil && appUpdate.latest.Version == m.Version
	if err == nil && stillLatest {
		appUpdate.installerPath = path
	}
	appUpdate.mu.Unlock()

	if err != nil {
		log.Error("下载更新失败", logger.F("version", m.Version), logger.F("error", err.Error()))
		emit(UpdateProgress{Phase: "error", Total: m.Size, Message: "下载失败，请稍后重试"})
		return
	}
	log.Info("更新安装包已下载并校验", logger.F("version", m.Version), logger.F("path", path))
	emit(UpdateProgress{Phase: "ready", Progress: 100, Downloaded: m.Size, Total: m.Size, Message: "安装包已就绪"})
}

// UpdateInstall 关闭所有实例与代理服务，启动静默安装并退出应用；安装完成后由安装器重新启动应用。
func (a *App) UpdateInstall() error {
	appUpdate.mu.Lock()
	m, path := appUpdate.latest, appUpdate.installerPath
	appUpdate.mu.Unlock()
	if m == nil || path == "" {
		return errors.New("安装包尚未下载完成")
	}
	// 启动前再次校验，防止下载后文件被替换
	if err := update.VerifyFile(path, m); err != nil {
		appUpdate.mu.Lock()
		appUpdate.installerPath = ""
		appUpdate.mu.Unlock()
		os.Remove(path)
		return fmt.Errorf("安装包校验失败，请重新下载: %w", err)
	}

	log := logger.New("Update")
	log.Info("开始安装更新", logger.F("from", AppVersion), logger.F("to", m.Version))

	// 安装器带 /WAITPID 会先等待本进程退出，所以可以先启动安装器；启动失败时应用保持原状继续运行。
	if err := update.StartInstaller(path); err != nil {
		log.Error("启动安装程序失败", logger.F("error", err.Error()))
		return err
	}
	go func() {
		// 留出时间让前端收到调用结果；ForceQuit 会正常关闭浏览器实例与代理进程
		time.Sleep(300 * time.Millisecond)
		a.ForceQuit()
	}()
	return nil
}

// cleanupOldInstallers 删除更新目录中非当前版本的安装包与残留临时文件。
func cleanupOldInstallers(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == keep || !strings.HasPrefix(name, "CZLBrowser-Setup-") {
			continue
		}
		os.Remove(filepath.Join(dir, name))
	}
}
