package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	tokenStoreFormatVersion = 1
	tokenStoreSchemePlain   = "plain"
	tokenStoreSchemeDPAPI   = "dpapi"
)

type storedTokenEnvelope struct {
	Version int    `json:"version"`
	Scheme  string `json:"scheme"`
	Payload string `json:"payload"`
}

// encodeStoredToken 将 token 数据编码为带版本的存储格式。
func encodeStoredToken(raw []byte) ([]byte, error) {
	scheme, protected, err := protectStoredTokenPayload(raw)
	if err != nil {
		return nil, err
	}

	envelope := storedTokenEnvelope{
		Version: tokenStoreFormatVersion,
		Scheme:  scheme,
		Payload: base64.RawURLEncoding.EncodeToString(protected),
	}

	return json.Marshal(envelope)
}

// decodeStoredToken 解析存储格式；若读取到旧版明文 JSON，则返回原始数据并标记需要迁移。
func decodeStoredToken(data []byte) ([]byte, bool, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, false, fmt.Errorf("空 token 数据")
	}

	var envelope storedTokenEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version == tokenStoreFormatVersion && envelope.Scheme != "" && envelope.Payload != "" {
		protected, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
		if err != nil {
			return nil, false, fmt.Errorf("解析令牌存储负载失败: %w", err)
		}

		raw, err := unprotectStoredTokenPayload(envelope.Scheme, protected)
		if err != nil {
			return nil, false, err
		}
		return raw, false, nil
	}

	// 历史格式直接存 JSON token，读取成功后会自动迁移为当前格式。
	return data, true, nil
}
