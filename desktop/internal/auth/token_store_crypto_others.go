//go:build !windows

package auth

import "fmt"

func protectStoredTokenPayload(raw []byte) (string, []byte, error) {
	if len(raw) == 0 {
		return "", nil, fmt.Errorf("空 token 数据")
	}
	return tokenStoreSchemePlain, raw, nil
}

func unprotectStoredTokenPayload(scheme string, payload []byte) ([]byte, error) {
	switch scheme {
	case tokenStoreSchemePlain:
		return payload, nil
	default:
		return nil, fmt.Errorf("不支持的令牌存储方案: %s", scheme)
	}
}
