package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SnapshotInfo 是实例用户数据的一份快照 (实例详情页「快照」)。
// 存放在 data/snapshots/<实例 id>/<快照 id>.zip, 旁边的 .meta.json 记录这些信息。
// 文件名不带快照名称: 名称里的冒号在 NTFS 上会变成备用数据流, 文件就找不到了。
// 旧版本留下的 <快照 id>_<名称>.zip 仍按 id 前缀识别。
type SnapshotInfo struct {
	SnapshotId string  `json:"snapshotId"`
	ProfileId  string  `json:"profileId"`
	Name       string  `json:"name"`
	SizeMB     float64 `json:"sizeMB"`
	CreatedAt  string  `json:"createdAt"`
	FilePath   string  `json:"filePath,omitempty"` // 只写在 meta 文件里, 不返回给前端
}

func (a *App) snapshotFolder(profileID string) (string, error) {
	dir := filepath.Join(a.resolveAppPath("data"), "snapshots", profileID)
	return dir, os.MkdirAll(dir, 0o755)
}

// stoppedProfileDir 返回已停止实例的用户数据目录; 运行中的实例返回错误。
func (a *App) stoppedProfileDir(profileID, action string) (string, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	p, ok := a.browserMgr.Profiles[profileID]
	if !ok {
		return "", fmt.Errorf("实例不存在: %s", profileID)
	}
	if p.Running {
		return "", fmt.Errorf("请先停止实例再%s", action)
	}
	return a.browserMgr.ResolveUserDataDir(p), nil
}

// BrowserSnapshotCreate 把实例当前的用户数据打包成快照。
func (a *App) BrowserSnapshotCreate(profileID, name string) (SnapshotInfo, error) {
	src, err := a.stoppedProfileDir(profileID, "创建快照")
	if err != nil {
		return SnapshotInfo{}, err
	}
	if _, err := os.Stat(src); err != nil {
		return SnapshotInfo{}, errors.New("用户数据目录不存在，无法创建快照")
	}
	dir, err := a.snapshotFolder(profileID)
	if err != nil {
		return SnapshotInfo{}, err
	}
	info := SnapshotInfo{SnapshotId: uuid.NewString(), ProfileId: profileID, Name: name, CreatedAt: time.Now().Format(time.RFC3339)}
	base := filepath.Join(dir, info.SnapshotId)
	info.FilePath = base + ".zip"
	if err := zipFolder(src, info.FilePath); err != nil {
		os.Remove(info.FilePath)
		return SnapshotInfo{}, fmt.Errorf("压缩失败: %w", err)
	}
	if st, err := os.Stat(info.FilePath); err == nil {
		info.SizeMB = float64(st.Size()) / (1 << 20)
	}
	meta, _ := json.Marshal(info)
	if err := os.WriteFile(base+".meta.json", meta, 0o644); err != nil {
		return SnapshotInfo{}, err
	}
	info.FilePath = ""
	return info, nil
}

// BrowserSnapshotList 列出实例的快照, 新的在前。
func (a *App) BrowserSnapshotList(profileID string) ([]SnapshotInfo, error) {
	dir, err := a.snapshotFolder(profileID)
	if err != nil {
		return nil, err
	}
	metas, _ := filepath.Glob(filepath.Join(dir, "*.meta.json"))
	list := []SnapshotInfo{}
	for _, m := range metas {
		var info SnapshotInfo
		if raw, err := os.ReadFile(m); err == nil && json.Unmarshal(raw, &info) == nil {
			info.FilePath = ""
			list = append(list, info)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt > list[j].CreatedAt })
	return list, nil
}

// snapshotFiles 找到快照的 meta 与 zip 文件。
func (a *App) snapshotFiles(profileID, snapshotID string) (meta, archive string, err error) {
	dir, err := a.snapshotFolder(profileID)
	if err != nil {
		return "", "", err
	}
	found, _ := filepath.Glob(filepath.Join(dir, snapshotID+"*.meta.json"))
	if len(found) == 0 || snapshotID == "" {
		return "", "", fmt.Errorf("快照不存在: %s", snapshotID)
	}
	meta = found[0]
	archive = strings.TrimSuffix(meta, ".meta.json") + ".zip"
	if _, err := os.Stat(archive); err != nil {
		return "", "", fmt.Errorf("快照文件不存在: %s", archive)
	}
	return meta, archive, nil
}

// BrowserSnapshotRestore 用快照替换实例的用户数据 (原数据会被清空)。
func (a *App) BrowserSnapshotRestore(profileID, snapshotID string) error {
	dest, err := a.stoppedProfileDir(profileID, "恢复快照")
	if err != nil {
		return err
	}
	_, archive, err := a.snapshotFiles(profileID, snapshotID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("清空用户数据目录失败: %w", err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return unzipTo(archive, dest)
}

// BrowserSnapshotDelete 删除快照文件。
func (a *App) BrowserSnapshotDelete(profileID, snapshotID string) error {
	meta, archive, err := a.snapshotFiles(profileID, snapshotID)
	if err != nil {
		return err
	}
	_ = os.Remove(archive)
	_ = os.Remove(meta)
	return nil
}
