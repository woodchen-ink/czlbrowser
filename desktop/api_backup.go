package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// 备份 / 恢复 / 初始化 (设置 → 备份与恢复)。三者互斥, 期间所有实例与转换进程都会被关闭。

// backupProgress 是 backup:export:progress / backup:import:progress 事件的内容。
type backupProgress struct {
	Phase         string `json:"phase"` // starting / preparing / writing / importing / done / cancelled / error
	Progress      int    `json:"progress"`
	Message       string `json:"message"`
	ComponentID   string `json:"componentId,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	EntryIndex    int    `json:"entryIndex,omitempty"`
	EntryTotal    int    `json:"entryTotal,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}

// progressFn 推送一次进度; 进度限定在 0-100。
type progressFn func(p backupProgress)

func (a *App) progressTo(event string) progressFn {
	return func(p backupProgress) {
		p.Progress = min(max(p.Progress, 0), 100)
		p.Timestamp = time.Now().Format("15:04:05")
		a.emit(event, p)
	}
}

func step(phase string, progress int, msg string) backupProgress {
	return backupProgress{Phase: phase, Progress: progress, Message: msg}
}

// BackupInitializeSystem 恢复到全新安装的状态: 默认配置、清空本机数据 (数据库文件本身保留)。
func (a *App) BackupInitializeSystem() (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	return a.resetToDefaults(true)
}

// BackupExportPackage 选择保存位置后导出完整备份包。
func (a *App) BackupExportPackage() (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	if a.ctx == nil {
		return nil, errors.New("应用上下文未初始化")
	}
	report := a.progressTo("backup:export:progress")
	report(step("starting", 0, "等待选择导出路径..."))
	dest, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出配置",
		DefaultFilename: fmt.Sprintf("czl-browser-backup-%s.zip", time.Now().Format("20060102-150405")),
		Filters:         []wailsruntime.FileFilter{{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil {
		return nil, fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(dest) == "" {
		report(step("cancelled", 0, "已取消导出"))
		return map[string]interface{}{"cancelled": true, "message": "已取消导出"}, nil
	}
	if !strings.EqualFold(filepath.Ext(dest), ".zip") {
		dest += ".zip"
	}
	res, err := a.exportPackage(dest, report)
	if err != nil {
		report(step("error", 100, "导出失败: "+err.Error()))
		return nil, err
	}
	return map[string]interface{}{
		"cancelled": false, "zipPath": dest, "message": "导出完成",
		"includedEntries": res.included, "skippedEntries": res.skipped, "fileCount": res.files,
	}, nil
}

// BackupImportPackage 选择备份包后导入。resetFirst 为 true 时先初始化再全量导入, 否则与现有数据合并 (已有的跳过)。
func (a *App) BackupImportPackage(resetFirst bool) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	if a.ctx == nil {
		return nil, errors.New("应用上下文未初始化")
	}
	report := a.progressTo("backup:import:progress")
	report(step("starting", 0, "等待选择 ZIP 配置文件..."))
	src, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "加载配置",
		Filters: []wailsruntime.FileFilter{{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil {
		report(step("error", 100, "打开文件对话框失败: "+err.Error()))
		return nil, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if strings.TrimSpace(src) == "" {
		report(step("cancelled", 0, "已取消加载"))
		return map[string]interface{}{"cancelled": true, "message": "已取消加载"}, nil
	}
	result, err := a.importPackage(src, resetFirst, report)
	if err != nil {
		report(step("error", 100, "加载失败: "+err.Error()))
		return nil, err
	}
	return result, nil
}
