//go:build windows

// Package dpapi 是 Windows DPAPI 的薄封装 (按当前 Windows 用户加解密)。
// 用于本机密钥库 (internal/keystore) 与 Chromium 登录态密钥重包 (internal/profilestate)。
package dpapi

import (
	"errors"
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

// Supported 报告本平台能否用 DPAPI。
func Supported() bool { return true }

// Protect 按当前 Windows 用户加密。
func Protect(raw []byte) ([]byte, error) {
	return call(procCryptProtectData, raw)
}

// Unprotect 解密 Protect 的结果 (或 Chromium 写入的 DPAPI 数据)。
func Unprotect(payload []byte) ([]byte, error) {
	return call(procCryptUnprotectData, payload)
}

func call(proc *windows.LazyProc, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("dpapi: empty input")
	}
	in := dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var out dataBlob
	r1, _, err := proc.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0,
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("dpapi: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	if out.pbData == nil || out.cbData == 0 {
		return []byte{}, nil
	}
	src := unsafe.Slice(out.pbData, out.cbData)
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst, nil
}
