// 指纹参数: 一张字段表同时驱动表单渲染和启动参数的读写。
// 启动参数格式 (--flag=value) 是与后端 launch_args.go 的契约, 改名要两边一起改。

export type Option = { value: string; label: string }

/** 表单里的一组指纹取值; 空字段表示不设置。key 与 FIELDS 中的 key 对应。 */
export type Fingerprint = Partial<Record<FieldKey, string>> & {
  /** 认不出的参数原样保留, 写回时追加在末尾 */
  extra?: string[]
}

type FieldKind = 'select' | 'bool' | 'text'

interface FieldSpec {
  key: string
  flag: string
  label: string
  kind: FieldKind
  group: FieldGroup
  options?: Option[]
  placeholder?: string
  /** select 末尾追加"自定义", 选中后改为文本输入 */
  custom?: boolean
}

export type FieldGroup = '基础身份' | '屏幕与硬件' | '渲染指纹' | '网络与隐私'

const opts = (...pairs: (string | [string, string])[]): Option[] =>
  pairs.map((p) => (Array.isArray(p) ? { value: p[0], label: p[1] } : { value: p, label: p }))

export const RESOLUTIONS = ['1920,1080', '2560,1440', '1440,900', '1366,768', '1600,900', '1280,800']

/** 各 GPU 厂商常见的渲染器名; 后端对 Clearcote 内核会再展开成 ANGLE 完整格式。 */
export const RENDERERS: Record<string, string[]> = {
  Intel: ['Intel(R) UHD Graphics 630', 'Intel(R) UHD Graphics 620', 'Intel(R) Iris(R) Xe Graphics', 'Intel(R) HD Graphics 520'],
  NVIDIA: ['NVIDIA GeForce RTX 4060', 'NVIDIA GeForce RTX 3080', 'NVIDIA GeForce RTX 3060', 'NVIDIA GeForce GTX 1660'],
  AMD: ['AMD Radeon RX 6600', 'AMD Radeon RX 580', 'AMD Radeon(TM) Graphics'],
  Apple: ['Apple M1', 'Apple M2', 'Apple M3'],
}

// 顺序即表单与写出参数的顺序。seed / timezone 有专门的控件, 不在分组里渲染。
const SPECS = [
  { key: 'seed', flag: '--fingerprint', label: '指纹种子', kind: 'text', group: '基础身份' },
  { key: 'brand', flag: '--fingerprint-brand', label: '浏览器品牌', kind: 'select', group: '基础身份', options: opts('Chrome', 'Edge') },
  { key: 'platform', flag: '--fingerprint-platform', label: '操作系统', kind: 'select', group: '基础身份', options: opts(['windows', 'Windows'], ['mac', 'macOS'], ['linux', 'Linux']) },
  {
    key: 'lang', flag: '--lang', label: '语言', kind: 'select', group: '基础身份',
    options: opts(['zh-CN', '简体中文 zh-CN'], ['zh-TW', '繁體中文 zh-TW'], ['en-US', 'English en-US'], ['en-GB', 'English en-GB'], ['ja-JP', '日本語 ja-JP'], ['ko-KR', '한국어 ko-KR'], ['de-DE', 'Deutsch de-DE'], ['fr-FR', 'Français fr-FR'], ['es-ES', 'Español es-ES']),
  },
  { key: 'timezone', flag: '--timezone', label: '时区', kind: 'select', group: '基础身份' },
  { key: 'resolution', flag: '--window-size', label: '窗口尺寸', kind: 'select', group: '屏幕与硬件', options: opts(...RESOLUTIONS), custom: true, placeholder: '宽,高' },
  { key: 'colorDepth', flag: '--fingerprint-color-depth', label: '色深', kind: 'select', group: '屏幕与硬件', options: opts(['24', '24 位'], ['30', '30 位 (HDR)'], ['32', '32 位']) },
  { key: 'hardwareConcurrency', flag: '--fingerprint-hardware-concurrency', label: 'CPU 核心数', kind: 'select', group: '屏幕与硬件', options: ['2', '4', '6', '8', '12', '16', '24'].map((n) => ({ value: n, label: `${n} 核` })) },
  { key: 'deviceMemory', flag: '--fingerprint-device-memory', label: '设备内存', kind: 'select', group: '屏幕与硬件', options: ['2', '4', '8', '16', '32'].map((n) => ({ value: n, label: `${n} GB` })) },
  { key: 'touchPoints', flag: '--fingerprint-touch-points', label: '触摸点数', kind: 'select', group: '屏幕与硬件', options: opts(['0', '0 (无触屏)'], '1', '5', '10') },
  { key: 'webglVendor', flag: '--fingerprint-webgl-vendor', label: 'GPU 厂商', kind: 'select', group: '渲染指纹', options: opts(...Object.keys(RENDERERS)) },
  { key: 'webglRenderer', flag: '--fingerprint-webgl-renderer', label: 'GPU 型号', kind: 'select', group: '渲染指纹', custom: true, placeholder: '渲染器名称' },
  { key: 'canvasNoise', flag: '--fingerprint-canvas-noise', label: 'Canvas 噪声', kind: 'bool', group: '渲染指纹' },
  { key: 'audioNoise', flag: '--fingerprint-audio-noise', label: 'Audio 噪声', kind: 'bool', group: '渲染指纹' },
  {
    key: 'webrtcPolicy', flag: '--webrtc-ip-handling-policy', label: 'WebRTC', kind: 'select', group: '网络与隐私',
    options: opts(['disable_non_proxied_udp', '禁止非代理 UDP (推荐)'], ['default_public_interface_only', '只暴露公网接口'], ['default_public_and_private_interfaces', '公网与内网接口']),
  },
  { key: 'doNotTrack', flag: '--fingerprint-do-not-track', label: 'Do Not Track', kind: 'bool', group: '网络与隐私' },
  { key: 'mediaDevices', flag: '--fingerprint-media-devices', label: '媒体设备数', kind: 'text', group: '网络与隐私', placeholder: '摄像头,麦克风,扬声器 如 1,1,1' },
  { key: 'fonts', flag: '--fingerprint-fonts', label: '字体列表', kind: 'text', group: '网络与隐私', placeholder: '逗号分隔, 如 Arial,Segoe UI' },
] as const satisfies readonly FieldSpec[]

export type FieldKey = (typeof SPECS)[number]['key']
export const FIELDS: readonly FieldSpec[] = SPECS
export const GROUPS: FieldGroup[] = ['基础身份', '屏幕与硬件', '渲染指纹', '网络与隐私']

const byFlag = new Map(FIELDS.map((f) => [f.flag, f.key as FieldKey]))

/** 时区取值 "system" 表示保存时换成本机时区。 */
export const SYSTEM_TZ = 'system'

export function localTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
}

/** 启动参数 → 表单取值; 不认识的参数进 extra。 */
export function parseArgs(args: string[]): Fingerprint {
  const fp: Fingerprint = { extra: [] }
  for (const raw of args) {
    const arg = raw.trim()
    if (!arg) continue
    const eq = arg.indexOf('=')
    const key = eq > 0 ? byFlag.get(arg.slice(0, eq)) : undefined
    if (key) fp[key] = arg.slice(eq + 1)
    else fp.extra!.push(arg)
  }
  return fp
}

/** 表单取值 → 启动参数, 按字段表顺序, 空值不写。 */
export function toArgs(fp: Fingerprint): string[] {
  const out: string[] = []
  for (const f of FIELDS) {
    let v = fp[f.key as FieldKey]?.trim()
    if (!v) continue
    if (f.key === 'timezone' && v === SYSTEM_TZ) v = localTimezone()
    out.push(`${f.flag}=${v}`)
  }
  return [...out, ...(fp.extra ?? [])]
}

/** 1 到 2^31-1 之间的随机种子。 */
export function randomSeed(): string {
  const buf = new Uint32Array(1)
  crypto.getRandomValues(buf)
  return String((buf[0] % 0x7fffffff) + 1)
}

// ---- 预设 ----

export interface Preset {
  id: string
  name: string
  values: Fingerprint
}

// 同一平台共用的基础取值, 预设只写差异
const windowsBase: Fingerprint = {
  brand: 'Chrome', platform: 'windows', colorDepth: '24', touchPoints: '0', canvasNoise: 'true', audioNoise: 'true',
  webrtcPolicy: 'disable_non_proxied_udp', fonts: 'Arial,Segoe UI,Calibri,Times New Roman,Courier New,Verdana',
}
const macBase: Fingerprint = {
  brand: 'Chrome', platform: 'mac', colorDepth: '30', touchPoints: '0', canvasNoise: 'true', audioNoise: 'true',
  webrtcPolicy: 'disable_non_proxied_udp', webglVendor: 'Apple', fonts: 'Helvetica Neue,Helvetica,Arial,Times,Courier,Menlo',
}

export const PRESETS: Preset[] = [
  {
    id: 'win-cn', name: 'Windows 办公 (中国)',
    values: { ...windowsBase, lang: 'zh-CN', timezone: 'Asia/Shanghai', resolution: '1920,1080', hardwareConcurrency: '8', deviceMemory: '8', webglVendor: 'Intel', webglRenderer: 'Intel(R) UHD Graphics 630', fonts: windowsBase.fonts + ',Microsoft YaHei,SimSun,SimHei' },
  },
  {
    id: 'win-us', name: 'Windows 家用 (美国)',
    values: { ...windowsBase, lang: 'en-US', timezone: 'America/Chicago', resolution: '1920,1080', hardwareConcurrency: '12', deviceMemory: '16', webglVendor: 'NVIDIA', webglRenderer: 'NVIDIA GeForce RTX 3060' },
  },
  {
    id: 'win-uk', name: 'Windows 笔记本 (英国)',
    values: { ...windowsBase, lang: 'en-GB', timezone: 'Europe/London', resolution: '1366,768', hardwareConcurrency: '4', deviceMemory: '8', webglVendor: 'Intel', webglRenderer: 'Intel(R) Iris(R) Xe Graphics' },
  },
  {
    id: 'win-de', name: 'Windows 游戏 (德国)',
    values: { ...windowsBase, lang: 'de-DE', timezone: 'Europe/Berlin', resolution: '2560,1440', hardwareConcurrency: '16', deviceMemory: '32', webglVendor: 'NVIDIA', webglRenderer: 'NVIDIA GeForce RTX 4060' },
  },
  {
    id: 'mac-us', name: 'MacBook (美国)',
    values: { ...macBase, lang: 'en-US', timezone: 'America/Los_Angeles', resolution: '1440,900', hardwareConcurrency: '8', deviceMemory: '8', webglRenderer: 'Apple M1' },
  },
  {
    id: 'mac-jp', name: 'MacBook (日本)',
    values: { ...macBase, lang: 'ja-JP', timezone: 'Asia/Tokyo', resolution: '1440,900', hardwareConcurrency: '8', deviceMemory: '16', webglRenderer: 'Apple M2', fonts: macBase.fonts + ',Hiragino Sans,Hiragino Kaku Gothic ProN' },
  },
  {
    id: 'mac-cn', name: 'iMac 设计 (中国)',
    values: { ...macBase, lang: 'zh-CN', timezone: 'Asia/Shanghai', resolution: '2560,1440', hardwareConcurrency: '10', deviceMemory: '16', webglRenderer: 'Apple M3', fonts: macBase.fonts + ',PingFang SC,Hiragino Sans GB' },
  },
]

// 常用时区; 列表外的值 (如从别处导入) 仍会作为一个选项显示
export const TIMEZONES = [
  'Asia/Shanghai', 'Asia/Hong_Kong', 'Asia/Taipei', 'Asia/Tokyo', 'Asia/Seoul', 'Asia/Singapore', 'Asia/Bangkok', 'Asia/Kolkata', 'Asia/Dubai',
  'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Amsterdam', 'Europe/Madrid', 'Europe/Moscow',
  'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/Toronto', 'America/Sao_Paulo',
  'Australia/Sydney', 'Pacific/Auckland', 'UTC',
]
