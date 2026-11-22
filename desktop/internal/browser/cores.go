package browser

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/woodchen-ink/czlbrowser/desktop/internal/logger"
)

// 内核列表的权威来源是数据库; m.Config.Browser.Cores 是它的内存镜像, 每次改动后刷新。

// ListCores 返回全部内核。
func (m *Manager) ListCores() []Core {
	m.refreshCores()
	return m.Config.Browser.Cores
}

func (m *Manager) refreshCores() {
	if m.CoreDAO == nil {
		return
	}
	if list, err := m.CoreDAO.List(); err == nil {
		m.Config.Browser.Cores = list
	}
}

// GetCore 按 id (不区分大小写) 找内核。
func (m *Manager) GetCore(id string) (Core, bool) {
	id = strings.TrimSpace(id)
	for _, c := range m.Config.Browser.Cores {
		if id != "" && strings.EqualFold(c.CoreId, id) {
			return c, true
		}
	}
	return Core{}, false
}

// GetDefaultCore 返回默认内核; 都没标默认时取第一个。
func (m *Manager) GetDefaultCore() (Core, bool) {
	cores := m.Config.Browser.Cores
	for _, c := range cores {
		if c.IsDefault {
			return c, true
		}
	}
	if len(cores) > 0 {
		return cores[0], true
	}
	return Core{}, false
}

// effectiveCore 返回实例实际使用的内核: 未指定或本机没有指定的内核时用默认内核。
func (m *Manager) effectiveCore(coreID string) (Core, bool) {
	if c, ok := m.GetCore(coreID); ok {
		return c, true
	}
	return m.GetDefaultCore()
}

// ResolveCoreExecutable 返回内核目录下 chrome.exe 的绝对路径。
func (m *Manager) ResolveCoreExecutable(c Core) (string, error) {
	dir := strings.TrimSpace(c.CorePath)
	if dir == "" {
		return "", errors.New("浏览器内核路径为空，请在“内核管理”中补充内核目录")
	}
	exe := filepath.Join(m.ResolveRelativePath(dir), "chrome.exe")
	if _, err := os.Stat(exe); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("浏览器内核目录无效：未找到 chrome.exe（%s）。请检查内核目录是否完整或重新下载内核", exe)
		}
		return "", fmt.Errorf("浏览器内核目录不可访问：%s。原因：%v", exe, err)
	}
	return exe, nil
}

// ResolveChromeBinary 返回实例启动要用的 chrome.exe。
func (m *Manager) ResolveChromeBinary(p *Profile) (string, error) {
	c, ok := m.effectiveCore(p.CoreId)
	if !ok {
		return "", errors.New("未配置可用浏览器内核。请先在“内核管理”中添加内核并设置默认内核")
	}
	exe, err := m.ResolveCoreExecutable(c)
	if err != nil {
		logger.New("Browser").Error("内核不可用", logger.F("core_id", c.CoreId), logger.F("error", err))
	}
	return exe, err
}

// ValidateCorePath 检查目录里是否有 chrome.exe。
func (m *Manager) ValidateCorePath(dir string) CoreValidateResult {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return CoreValidateResult{Message: "路径不能为空"}
	}
	abs := m.ResolveRelativePath(dir)
	if _, err := os.Stat(abs); err != nil {
		return CoreValidateResult{Message: "目录不存在: " + abs}
	}
	exe := filepath.Join(abs, "chrome.exe")
	if _, err := os.Stat(exe); err != nil {
		return CoreValidateResult{Message: "chrome.exe 不存在: " + exe}
	}
	return CoreValidateResult{Valid: true, Message: "路径有效: " + exe}
}

// RegisterDownloadedCore 登记一个下载好的内核; 这是第一个内核时设为默认。
func (m *Manager) RegisterDownloadedCore(name, dir string) error {
	name, dir = strings.TrimSpace(name), strings.TrimSpace(dir)
	if name == "" || dir == "" {
		return errors.New("内核名称与路径不能为空")
	}
	c := Core{CoreId: uuid.NewString(), CoreName: name, CorePath: dir, IsDefault: len(m.ListCores()) == 0}
	if err := m.saveCore(c); err != nil {
		return err
	}
	logger.New("Browser").Info("已登记下载的内核", logger.F("core_id", c.CoreId), logger.F("name", name))
	return nil
}

func (m *Manager) saveCore(c Core) error {
	if m.CoreDAO == nil {
		return errors.New("内核数据未初始化")
	}
	if err := m.CoreDAO.Upsert(c); err != nil {
		return err
	}
	m.refreshCores()
	return nil
}

// DeleteCore 删除内核登记 (不删目录)。
func (m *Manager) DeleteCore(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("内核ID不能为空")
	}
	if m.CoreDAO == nil {
		return errors.New("内核数据未初始化")
	}
	if err := m.CoreDAO.Delete(id); err != nil {
		return err
	}
	m.refreshCores()
	return nil
}

// SetDefaultCore 把指定内核设为默认。
func (m *Manager) SetDefaultCore(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("内核ID不能为空")
	}
	if m.CoreDAO == nil {
		return errors.New("内核数据未初始化")
	}
	if err := m.CoreDAO.SetDefault(id); err != nil {
		return err
	}
	m.refreshCores()
	return nil
}

// SetCoreTags 修改内核标签。
func (m *Manager) SetCoreTags(id string, tags []string) error {
	c, ok := m.GetCore(id)
	if !ok {
		return fmt.Errorf("内核不存在: %s", id)
	}
	c.Tags = tags
	return m.saveCore(c)
}

// GetChromeVersion 读内核版本号, 依次尝试: manifest.json、<版本>.manifest 文件名、
// 官方 Chrome-bin 布局里 chrome.dll 所在的版本目录名。
func (m *Manager) GetChromeVersion(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	base := m.ResolveRelativePath(dir)
	if raw, err := os.ReadFile(filepath.Join(base, "manifest.json")); err == nil {
		var mf struct {
			Version string `json:"version"`
		}
		_ = json.Unmarshal(raw, &mf)
		return mf.Version
	}
	if found, _ := filepath.Glob(filepath.Join(base, "*.manifest")); len(found) > 0 {
		if v := strings.TrimSuffix(filepath.Base(found[0]), ".manifest"); v != "" {
			return v
		}
	}
	if dll := findChromeDLL(base); dll != "" {
		if v := filepath.Base(filepath.Dir(dll)); chromeVersionPattern.MatchString(v) {
			return v
		}
	}
	return ""
}

// CountInstancesByCore 统计当前空间里启动时会用到该内核的实例数 (跟随默认或指向本机没有的内核的, 算在默认内核上)。
func (m *Manager) CountInstancesByCore(id string) int {
	n := 0
	for _, p := range m.List() {
		if c, ok := m.effectiveCore(p.CoreId); ok && strings.EqualFold(c.CoreId, strings.TrimSpace(id)) {
			n++
		}
	}
	return n
}

// GetCoresExtendedInfo 返回内核列表页要展示的版本、实例数与类型。
func (m *Manager) GetCoresExtendedInfo() []CoreExtendedInfo {
	cores := m.ListCores()
	out := make([]CoreExtendedInfo, 0, len(cores))
	for _, c := range cores {
		out = append(out, CoreExtendedInfo{
			CoreId:        c.CoreId,
			ChromeVersion: m.GetChromeVersion(c.CorePath),
			InstanceCount: m.CountInstancesByCore(c.CoreId),
			CoreKind:      string(m.ResolveCoreKind(c)),
		})
	}
	return out
}
