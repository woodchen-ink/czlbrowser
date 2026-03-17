package backend

import (
	"czl-browser/backend/internal/logger"
	"strings"
)

// webglPreset 定义一组 WebGL vendor + renderer 组合
type webglPreset struct {
	vendor   string
	renderer string
}

// 按平台分组的常见 WebGL 配置，用于自动填充。
// vendor/renderer 值与前端 FingerprintPanel 预设保持一致（短格式），
// fingerprint-chromium 内核会自动包装为 ANGLE 格式。
var webglPresetsByPlatform = map[string][]webglPreset{
	"windows": {
		{"Intel", "Intel(R) UHD Graphics 630"},
		{"Intel", "Intel(R) UHD Graphics 620"},
		{"Intel", "Intel(R) Iris(R) Xe Graphics"},
		{"NVIDIA", "NVIDIA GeForce GTX 1660"},
		{"NVIDIA", "NVIDIA GeForce RTX 3060"},
		{"AMD", "AMD Radeon RX 580"},
		{"AMD", "AMD Radeon RX 6600"},
	},
	"mac": {
		{"Apple", "Apple M1"},
		{"Apple", "Apple M2"},
		{"Apple", "Apple M3"},
	},
	"linux": {
		{"Intel", "Intel(R) UHD Graphics 630"},
		{"Intel", "Intel(R) HD Graphics 520"},
		{"NVIDIA", "NVIDIA GeForce GTX 1080 Ti"},
		{"AMD", "AMD Radeon RX 580"},
	},
}

// autoFillWebGLArgs 在启动参数中自动补全 WebGL vendor/renderer。
// 仅当用户未显式配置 --fingerprint-webgl-vendor 和 --fingerprint-webgl-renderer 时才自动填充。
// 根据 --fingerprint-platform 选择与平台匹配的真实 WebGL 配置。
func autoFillWebGLArgs(args []string, log *logger.Logger, profileID string) []string {
	hasVendor := false
	hasRenderer := false
	platform := ""

	for _, arg := range args {
		lower := strings.ToLower(arg)
		if strings.HasPrefix(lower, "--fingerprint-webgl-vendor=") {
			val := strings.TrimPrefix(arg, arg[:strings.Index(arg, "=")+1])
			if strings.TrimSpace(val) != "" {
				hasVendor = true
			}
		}
		if strings.HasPrefix(lower, "--fingerprint-webgl-renderer=") {
			val := strings.TrimPrefix(arg, arg[:strings.Index(arg, "=")+1])
			if strings.TrimSpace(val) != "" {
				hasRenderer = true
			}
		}
		if strings.HasPrefix(lower, "--fingerprint-platform=") {
			platform = strings.TrimSpace(arg[len("--fingerprint-platform="):])
		}
	}

	// 两者都已配置，无需自动填充
	if hasVendor && hasRenderer {
		return args
	}

	if platform == "" {
		platform = "windows"
	}

	presets, ok := webglPresetsByPlatform[strings.ToLower(platform)]
	if !ok || len(presets) == 0 {
		presets = webglPresetsByPlatform["windows"]
	}

	// 基于 profileID 确定性选择，同一实例每次启动选同一个
	idx := stableIndex(profileID, len(presets))
	chosen := presets[idx]

	if !hasVendor {
		args = upsertKeyValueArg(args, "--fingerprint-webgl-vendor", chosen.vendor)
	}
	if !hasRenderer {
		args = upsertKeyValueArg(args, "--fingerprint-webgl-renderer", chosen.renderer)
	}

	log.Info("自动填充 WebGL 指纹",
		logger.F("profile_id", profileID),
		logger.F("platform", platform),
		logger.F("webgl_vendor", chosen.vendor),
		logger.F("webgl_renderer", chosen.renderer),
	)

	return args
}

// stableIndex 根据字符串确定性地选择一个索引（同一字符串每次返回同一索引）
func stableIndex(s string, n int) int {
	if n <= 0 {
		return 0
	}
	h := 0
	for _, ch := range s {
		h = h*31 + int(ch)
	}
	if h < 0 {
		h = -h
	}
	return h % n
}
