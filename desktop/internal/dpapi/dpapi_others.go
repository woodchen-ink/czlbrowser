//go:build !windows

// Package dpapi 是 Windows DPAPI 的薄封装; 其它平台不支持 (产品只发 Windows)。
package dpapi

import "errors"

var errUnsupported = errors.New("dpapi: unsupported platform")

// Supported 报告本平台能否用 DPAPI。
func Supported() bool { return false }

// Protect 在非 Windows 平台不可用。
func Protect([]byte) ([]byte, error) { return nil, errUnsupported }

// Unprotect 在非 Windows 平台不可用。
func Unprotect([]byte) ([]byte, error) { return nil, errUnsupported }
