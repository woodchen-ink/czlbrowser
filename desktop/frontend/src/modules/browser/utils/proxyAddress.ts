import yaml from 'js-yaml'

/** 代理池支持手填的协议; 订阅导入的 Clash YAML 节点只展示不在表单里编辑。 */
export const PROXY_SCHEMES = ['socks5', 'http', 'https'] as const
export type ProxyScheme = (typeof PROXY_SCHEMES)[number]

export interface ProxyFields {
  scheme: ProxyScheme
  host: string
  port: string
  user: string
  pass: string
}

export const DIRECT_CONFIG = 'direct://'

const isScheme = (s: string): s is ProxyScheme => (PROXY_SCHEMES as readonly string[]).includes(s)

/** 旧数据里的单个 Clash 节点 (YAML 对象或只含一个元素的数组) */
function clashNode(cfg: string): Record<string, unknown> | null {
  try {
    const v = yaml.load(cfg)
    const node = Array.isArray(v) ? v[0] : v
    return node && typeof node === 'object' ? (node as Record<string, unknown>) : null
  } catch {
    return null
  }
}

/** 列表里显示用: 协议、服务器、端口。认不出时字段为空。 */
export function describeProxy(cfg: string): { type: string; host: string; port: number } {
  const s = cfg.trim()
  if (s === DIRECT_CONFIG) return { type: 'direct', host: '', port: 0 }
  const m = /^([a-z][a-z0-9+.-]*):\/\//i.exec(s)
  if (m) {
    try {
      const u = new URL(s)
      return { type: m[1].toLowerCase(), host: u.hostname, port: Number(u.port) || 0 }
    } catch {
      return { type: m[1].toLowerCase(), host: '', port: 0 }
    }
  }
  const n = clashNode(s)
  return { type: String(n?.type ?? ''), host: String(n?.server ?? ''), port: Number(n?.port) || 0 }
}

/** 代理地址 → 表单字段。 */
export function splitProxy(cfg: string): ProxyFields {
  const s = cfg.trim()
  const blank: ProxyFields = { scheme: 'socks5', host: '', port: '', user: '', pass: '' }
  const m = /^(socks5|https?):\/\//i.exec(s)
  if (m) {
    try {
      const u = new URL(s)
      return { scheme: m[1].toLowerCase() as ProxyScheme, host: u.hostname, port: u.port, user: decodeURIComponent(u.username), pass: decodeURIComponent(u.password) }
    } catch {
      return { ...blank, scheme: m[1].toLowerCase() as ProxyScheme }
    }
  }
  const n = clashNode(s)
  if (!n) return blank
  const t = String(n.type ?? '').toLowerCase()
  return { scheme: isScheme(t) ? t : 'socks5', host: String(n.server ?? ''), port: n.port ? String(n.port) : '', user: String(n.username ?? ''), pass: String(n.password ?? '') }
}

/** 表单字段 → 代理地址; 账号密码做 URL 编码。 */
export function joinProxy(f: ProxyFields): string {
  const auth = f.user ? `${encodeURIComponent(f.user)}:${encodeURIComponent(f.pass)}@` : ''
  return `${f.scheme}://${auth}${f.host.trim()}:${f.port.trim()}`
}

/** 隐去地址里的账号密码, 用于悬停提示 */
export const maskProxy = (cfg: string) => cfg.replace(/\/\/[^@/]*@/, '//***@')
