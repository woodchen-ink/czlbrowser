package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// defaultYAML 是内置默认配置, 也是新安装时 scripts/config.init.yaml 的蓝本。
//
//go:embed config.default.yaml
var defaultYAML []byte

// DefaultConfig 返回一份全新的默认配置。
func DefaultConfig() *Config {
	var c Config
	if err := yaml.Unmarshal(defaultYAML, &c); err != nil {
		panic("内置默认配置无法解析: " + err.Error())
	}
	return &c
}

// Load 读取配置文件并补齐缺失项。文件不存在时返回默认配置。
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	c.fillDefaults(DefaultConfig())
	return &c, nil
}

// Save 写回配置文件。
func (c *Config) Save(path string) error {
	raw, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

// fillDefaults 只补空值, 不改用户填过的内容; 另外把几个旧版默认值换成新默认值。
func (c *Config) fillDefaults(d *Config) {
	setStr := func(dst *string, def string) {
		if strings.TrimSpace(*dst) == "" {
			*dst = def
		}
	}
	setInt := func(dst *int, def int) {
		if *dst <= 0 {
			*dst = def
		}
	}
	setList := func(dst *[]string, def []string) {
		if len(*dst) == 0 {
			*dst = append([]string(nil), def...)
		}
	}

	setStr(&c.Database.Type, d.Database.Type)
	setStr(&c.Database.SQLite.Path, d.Database.SQLite.Path)
	setStr(&c.App.Name, d.App.Name)

	// 旧版默认窗口 1750x1000、最小 1200x700 过大, 未改过的换成新默认值
	w := &c.App.Window
	if w.Width == 1750 && w.Height == 1000 {
		w.Width, w.Height = 0, 0
	}
	if w.MinWidth == 1200 && w.MinHeight == 700 {
		w.MinWidth, w.MinHeight = 0, 0
	}
	setInt(&w.Width, d.App.Window.Width)
	setInt(&w.Height, d.App.Window.Height)
	setInt(&w.MinWidth, d.App.Window.MinWidth)
	setInt(&w.MinHeight, d.App.Window.MinHeight)

	setInt(&c.Runtime.MaxMemoryMB, d.Runtime.MaxMemoryMB)
	setInt(&c.Runtime.GCPercent, d.Runtime.GCPercent)

	setStr(&c.Logging.Level, d.Logging.Level)
	// 旧版日志写在数据根下的 logs/app.log, 统一挪到 data/logs
	if strings.EqualFold(filepath.ToSlash(strings.TrimSpace(c.Logging.FilePath)), "logs/app.log") {
		c.Logging.FilePath = ""
	}
	setStr(&c.Logging.FilePath, d.Logging.FilePath)
	setInt(&c.Logging.Rotation.MaxSizeMB, d.Logging.Rotation.MaxSizeMB)
	setInt(&c.Logging.Rotation.MaxBackups, d.Logging.Rotation.MaxBackups)

	b := &c.Browser
	setStr(&b.UserDataRoot, d.Browser.UserDataRoot)
	setList(&b.DefaultFingerprintArgs, d.Browser.DefaultFingerprintArgs)
	setList(&b.DefaultLaunchArgs, d.Browser.DefaultLaunchArgs)
	if b.DefaultBookmarks == nil {
		b.DefaultBookmarks = []BrowserBookmark{}
	}
	if b.Cores == nil {
		b.Cores = []BrowserCore{}
	}
	if b.Proxies == nil {
		b.Proxies = []BrowserProxy{}
	}
	if b.Profiles == nil {
		b.Profiles = []BrowserProfileConfig{}
	}
}

// legacyProxyFile 是早期版本单独存代理的 proxies.yaml 结构。
type legacyProxyFile struct {
	Proxies []BrowserProxy `yaml:"proxies"`
}

// LoadProxies 读取旧版 proxies.yaml; 文件不存在时返回 nil。
func LoadProxies(path string) ([]BrowserProxy, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取代理文件失败: %w", err)
	}
	var f legacyProxyFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("解析代理文件失败: %w", err)
	}
	return f.Proxies, nil
}

// SaveProxies 写 proxies.yaml (备份包里携带代理列表时使用)。
func SaveProxies(path string, proxies []BrowserProxy) error {
	raw, err := yaml.Marshal(legacyProxyFile{Proxies: proxies})
	if err != nil {
		return fmt.Errorf("序列化代理数据失败: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("写入代理文件失败: %w", err)
	}
	return nil
}
