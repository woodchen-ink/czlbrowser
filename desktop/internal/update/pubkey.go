package update

import (
	"crypto/ed25519"
	"encoding/base64"
)

// publicKeyBase64 更新清单验签公钥，对应 GitHub Secret UPDATE_SIGNING_KEY 中的私钥种子。
// 更换密钥时需同时更新此处并发布新版本；旧客户端只认旧公钥，换钥前的最后一版仍需用旧私钥签名。
const publicKeyBase64 = "26jtUET7/5jFmd3JuMAFNJsymh+wMoBJtjVytheqyJU="

var publicKey = mustDecodePublicKey(publicKeyBase64)

func mustDecodePublicKey(s string) ed25519.PublicKey {
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(key) != ed25519.PublicKeySize {
		panic("更新验签公钥无效")
	}
	return ed25519.PublicKey(key)
}
