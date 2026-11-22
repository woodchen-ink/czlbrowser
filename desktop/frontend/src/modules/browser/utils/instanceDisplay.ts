import type { BrowserCore, BrowserCoreExtended, BrowserProfile, BrowserProxy, ProxyIPHealthResult } from '../types'

// 实例列表 / 控制台共用的展示字段: 代理出口地区、内核名称与版本、指纹平台、相对时间。

const DIRECT_PROXY_ID = '__direct__'

export interface ProxyDisplay {
  name: string
  /** 出口地区 (最近一次 IP 检测结果), 没检测过为空 */
  region: string
  /** 最近一次测速失败 */
  failed: boolean
}

export function proxyDisplay(profile: BrowserProfile, proxies: BrowserProxy[]): ProxyDisplay {
  if (!profile.proxyId && !profile.proxyConfig) return { name: '直连', region: '', failed: false }
  if (profile.proxyId === DIRECT_PROXY_ID) return { name: '直连', region: '', failed: false }
  const proxy = profile.proxyId ? proxies.find((p) => p.proxyId === profile.proxyId) : undefined
  if (!proxy) {
    return { name: profile.proxyId ? '代理已删除' : '自定义代理', region: '', failed: !!profile.proxyId }
  }
  let region = ''
  if (proxy.lastIPHealthJson) {
    try {
      const h = JSON.parse(proxy.lastIPHealthJson) as ProxyIPHealthResult
      if (h.ok) region = [h.country, h.city && h.city !== h.country ? h.city : ''].filter(Boolean).join(' · ')
    } catch {
      // 旧数据格式不对时不显示地区
    }
  }
  const failed = !!proxy.lastTestedAt && proxy.lastTestOk === false
  return { name: proxy.proxyName, region, failed }
}

export interface CoreDisplay {
  name: string
  version: string
  /** 跟随全局默认内核 */
  followsDefault: boolean
  missing: boolean
}

export function coreDisplay(profile: BrowserProfile, cores: BrowserCore[], ext: Map<string, BrowserCoreExtended>): CoreDisplay {
  const def = cores.find((c) => c.isDefault)
  const followsDefault = !profile.coreId
  const core = followsDefault ? def : cores.find((c) => c.coreId === profile.coreId)
  if (!core) {
    // 同步空间里别的电脑选的内核, 本机没有时启动会回落到默认内核
    return { name: followsDefault ? '未设默认内核' : def ? def.coreName : '内核不存在', version: '', followsDefault, missing: true }
  }
  const version = ext.get(core.coreId)?.chromeVersion || ''
  // 名称里已带版本号 (自动扫描注册的内核) 时不再重复显示
  return { name: core.coreName, version: version && core.coreName.includes(version) ? '' : version, followsDefault, missing: false }
}

const PLATFORM_LABEL: Record<string, string> = {
  windows: 'Windows',
  mac: 'macOS',
  macos: 'macOS',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
}

/** 指纹里的操作系统平台, 没设置时为空 */
export function platformOf(args: string[] | undefined | null): string {
  const hit = (args || []).find((a) => a.startsWith('--fingerprint-platform='))
  if (!hit) return ''
  const v = hit.split('=')[1]?.trim().toLowerCase() || ''
  return PLATFORM_LABEL[v] || v
}

/** 相对时间: 刚刚 / 5 分钟前 / 3 小时前 / 2 天前 / 2026/11/02 */
export function relativeTime(value?: string): string {
  if (!value) return ''
  const t = new Date(value).getTime()
  if (Number.isNaN(t) || t <= 0) return ''
  const diff = Date.now() - t
  const min = 60_000
  if (diff < min) return '刚刚'
  if (diff < 60 * min) return `${Math.floor(diff / min)} 分钟前`
  if (diff < 24 * 60 * min) return `${Math.floor(diff / (60 * min))} 小时前`
  if (diff < 30 * 24 * 60 * min) return `${Math.floor(diff / (24 * 60 * min))} 天前`
  return new Date(t).toLocaleDateString('zh-CN')
}

/** 名称自然排序 (数字按数值比较) */
export function naturalCompare(a: string, b: string): number {
  return a.localeCompare(b, 'zh-CN', { numeric: true, sensitivity: 'base' })
}
