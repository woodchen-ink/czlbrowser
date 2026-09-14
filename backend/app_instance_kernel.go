package backend

import (
	"czl-browser/backend/internal/browser"
	"czl-browser/backend/internal/logger"
	"fmt"
	"strings"
)

// ============================================================================
// 按内核类型区分的启动策略
// ============================================================================
//
// 标准 Chrome：不认识 --fingerprint-* / --timezone，时区与 stealth 只能在启动后通过
// 浏览器级 CDP 会话逐 target 注入（连接存续期间有效，覆盖不到跨站 iframe / worker）。
//
// Clearcote：指纹与时区在 Blink/V8 引擎内实现，并由 --timezone 传递给所有渲染进程，
// 页面、iframe、Worker、弹窗一致生效。启动参数改写为 Clearcote 的开关名即可，
// 不再建立 CDP 注入连接（JS 注入本身就是可被探测的特征）。

// kernelLaunchPlan 描述某类内核的启动方式。
type kernelLaunchPlan struct {
	Kind browser.CoreKind
	// InjectViaCDP 为 true 时启动后通过 CDP 注入 stealth 脚本与时区覆盖。
	InjectViaCDP bool
	Args         []string
}

// buildKernelLaunchPlan 按内核类型改写启动参数并决定注入方式。
// args 中的 --timezone 已被提前剥离，timezone 为最终生效的 IANA 时区（可能为空）。
func buildKernelLaunchPlan(kind browser.CoreKind, args []string, timezone string, log *logger.Logger, profileID string) kernelLaunchPlan {
	args = autoFillWebGLArgs(args, log, profileID)

	if kind != browser.CoreKindClearcote {
		return kernelLaunchPlan{Kind: browser.CoreKindChrome, InjectViaCDP: true, Args: args}
	}

	args = translateArgsForClearcote(args)
	if timezone = strings.TrimSpace(timezone); timezone != "" {
		args = upsertKeyValueArg(args, "--timezone", timezone)
	}
	log.Info("Clearcote 内核：使用引擎级指纹与时区开关，跳过 CDP 注入",
		logger.F("profile_id", profileID),
		logger.F("timezone", timezone),
	)
	return kernelLaunchPlan{Kind: browser.CoreKindClearcote, InjectViaCDP: false, Args: args}
}

// translateArgsForClearcote 把前端/配置中的通用指纹开关改写为 Clearcote 的开关名与取值格式。
func translateArgsForClearcote(args []string) []string {
	platform := "windows"
	webglVendor, webglRenderer := "", ""
	result := make([]string, 0, len(args))

	for _, arg := range args {
		key, value, hasValue := strings.Cut(arg, "=")
		switch strings.ToLower(key) {
		case "--fingerprint-webgl-vendor":
			webglVendor = strings.TrimSpace(value)
			continue
		case "--fingerprint-webgl-renderer":
			webglRenderer = strings.TrimSpace(value)
			continue
		case "--fingerprint-touch-points":
			if hasValue {
				result = append(result, "--fingerprint-max-touch-points="+value)
			}
			continue
		case "--fingerprint-platform":
			platform = normalizeClearcotePlatform(value)
			result = append(result, "--fingerprint-platform="+platform)
			continue
		}
		result = append(result, arg)
	}

	if webglVendor != "" && webglRenderer != "" {
		vendor, renderer := angleWebGLStrings(platform, webglVendor, webglRenderer)
		result = upsertKeyValueArg(result, "--fingerprint-gpu-vendor", vendor)
		result = upsertKeyValueArg(result, "--fingerprint-gpu-renderer", renderer)
	}
	return result
}

// normalizeClearcotePlatform Clearcote 平台取值为 windows / linux / macos。
func normalizeClearcotePlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "mac", "macos", "osx", "darwin":
		return "macos"
	case "linux":
		return "linux"
	default:
		return "windows"
	}
}

// angleWebGLStrings 把短格式 vendor/renderer 包装成真实 Chrome 报告的 ANGLE 格式。
// Clearcote 原样返回传入值，不做包装；短格式（如 "NVIDIA GeForce GTX 1660"）在真实 Chrome 中不存在，属于明显特征。
// 已是 ANGLE 格式的取值保持不变。
func angleWebGLStrings(platform string, vendor string, renderer string) (string, string) {
	if strings.HasPrefix(renderer, "ANGLE (") {
		if !strings.HasPrefix(vendor, "Google Inc.") {
			vendor = fmt.Sprintf("Google Inc. (%s)", vendor)
		}
		return vendor, renderer
	}

	angleVendor := fmt.Sprintf("Google Inc. (%s)", vendor)
	switch normalizeClearcotePlatform(platform) {
	case "macos":
		return angleVendor, fmt.Sprintf("ANGLE (%s, ANGLE Metal Renderer: %s, Unspecified Version)", vendor, renderer)
	case "linux":
		return angleVendor, fmt.Sprintf("ANGLE (%s, %s, OpenGL 4.6)", vendor, renderer)
	default:
		device := renderer
		if id, ok := webglDeviceIDs[renderer]; ok {
			device = fmt.Sprintf("%s (%s)", renderer, id)
		}
		return angleVendor, fmt.Sprintf("ANGLE (%s, %s Direct3D11 vs_5_0 ps_5_0, D3D11)", vendor, device)
	}
}

// webglDeviceIDs 常见 GPU 的 PCI 设备 ID，Windows 下 Chrome 会在 renderer 中附带。
var webglDeviceIDs = map[string]string{
	"Intel(R) UHD Graphics 630":    "0x00003E92",
	"Intel(R) UHD Graphics 620":    "0x00005917",
	"Intel(R) HD Graphics 520":     "0x00001916",
	"Intel(R) Iris(R) Xe Graphics": "0x00009A49",
	"NVIDIA GeForce GTX 1660":      "0x00002184",
	"NVIDIA GeForce GTX 1080 Ti":   "0x00001B06",
	"NVIDIA GeForce RTX 3060":      "0x00002503",
	"NVIDIA GeForce RTX 3080":      "0x00002206",
	"AMD Radeon RX 580":            "0x000067DF",
	"AMD Radeon RX 6600":           "0x000073FF",
}
