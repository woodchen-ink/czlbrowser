// Package update 实现应用自身的在线更新：拉取签名清单、下载校验安装包、启动静默安装。
//
// 发布链路：私有仓库 CI 构建安装包 → 用 Ed25519 私钥签名 latest.json → 二者作为附件发布到本仓库 Release
// → CZL Connect 代理该 Release 对外提供。客户端只信任内置公钥验签通过的清单，
// 因此清单经哪个地址获取、安装包从哪个地址下载都不影响安全性。
package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Envelope 签名信封：Manifest 为清单原始 JSON 的 base64，Signature 针对这段原始字节签名。
type Envelope struct {
	Manifest  string `json:"manifest"`
	Signature string `json:"signature"`
}

// Manifest 最新版本描述。
type Manifest struct {
	Version      string   `json:"version"`
	Platform     string   `json:"platform"`
	PublishedAt  string   `json:"publishedAt"`
	Changelog    string   `json:"changelog"`
	FileName     string   `json:"fileName"`
	Size         int64    `json:"size"`
	SHA256       string   `json:"sha256"`
	DownloadURLs []string `json:"downloadUrls"`
}

// Platform 当前客户端对应的发布平台标识。
const Platform = "windows-x64"

// 安装包文件名会直接用作本地文件名，严格限制格式以杜绝路径穿越。
var (
	fileNamePattern = regexp.MustCompile(`^CZLBrowser-Setup-[0-9][0-9.]*\.exe$`)
	sha256Pattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// maxInstallerSize 安装包体积上限，防止异常清单导致写满磁盘。
const maxInstallerSize = 512 << 20

// ParseEnvelope 验签并解析清单；验签使用内置公钥。
func ParseEnvelope(data []byte) (*Manifest, error) {
	return parseEnvelopeWithKey(data, publicKey)
}

func parseEnvelopeWithKey(data []byte, key ed25519.PublicKey) (*Manifest, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("信封解析失败: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(env.Manifest)
	if err != nil {
		return nil, fmt.Errorf("清单编码无效: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, errors.New("签名格式无效")
	}
	if !ed25519.Verify(key, payload, sig) {
		return nil, errors.New("签名校验失败")
	}

	var m Manifest
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, fmt.Errorf("清单解析失败: %w", err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) validate() error {
	m.SHA256 = strings.ToLower(strings.TrimSpace(m.SHA256))
	switch {
	case !IsReleaseVersion(m.Version):
		return fmt.Errorf("版本号无效: %q", m.Version)
	case m.Platform != Platform:
		return fmt.Errorf("平台不匹配: %q", m.Platform)
	case !fileNamePattern.MatchString(m.FileName):
		return fmt.Errorf("安装包文件名无效: %q", m.FileName)
	case m.Size <= 0 || m.Size > maxInstallerSize:
		return fmt.Errorf("安装包大小无效: %d", m.Size)
	case !sha256Pattern.MatchString(m.SHA256):
		return errors.New("安装包 SHA256 无效")
	}

	urls := make([]string, 0, len(m.DownloadURLs))
	for _, u := range m.DownloadURLs {
		if strings.HasPrefix(u, "https://") {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return errors.New("清单中没有可用的下载地址")
	}
	m.DownloadURLs = urls
	return nil
}
