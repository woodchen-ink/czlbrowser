//go:build windows

package auth

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cryptProtectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32                = windows.NewLazySystemDLL("crypt32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

func protectStoredTokenPayload(raw []byte) (string, []byte, error) {
	protected, err := dpapiProtect(raw)
	if err != nil {
		return "", nil, err
	}
	return tokenStoreSchemeDPAPI, protected, nil
}

func unprotectStoredTokenPayload(scheme string, payload []byte) ([]byte, error) {
	switch scheme {
	case tokenStoreSchemeDPAPI:
		return dpapiUnprotect(payload)
	case tokenStoreSchemePlain:
		return payload, nil
	default:
		return nil, fmt.Errorf("不支持的令牌存储方案: %s", scheme)
	}
}

func dpapiProtect(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("空 token 数据")
	}

	in := dataBlob{
		cbData: uint32(len(raw)),
		pbData: &raw[0],
	}
	var out dataBlob

	r1, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("DPAPI 加密失败: %w", err)
	}
	defer localFree(out.pbData)

	return copyBlobBytes(out), nil
}

func dpapiUnprotect(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("空 token 负载")
	}

	in := dataBlob{
		cbData: uint32(len(payload)),
		pbData: &payload[0],
	}
	var out dataBlob

	r1, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("DPAPI 解密失败: %w", err)
	}
	defer localFree(out.pbData)

	return copyBlobBytes(out), nil
}

func copyBlobBytes(blob dataBlob) []byte {
	if blob.pbData == nil || blob.cbData == 0 {
		return nil
	}

	src := unsafe.Slice(blob.pbData, blob.cbData)
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func localFree(ptr *byte) {
	if ptr == nil {
		return
	}
	procLocalFree.Call(uintptr(unsafe.Pointer(ptr)))
}
