import type { BrowserCoreExtended } from '../types'

const major = (v: string) => Number.parseInt(v.split('.')[0] ?? '', 10)

/**
 * 实例换内核时要提醒的影响。Cookie 与本地数据换内核后都还在 (两类内核都只用 DPAPI 加密),
 * 变的是网站看到的指纹, 以及大版本降级时个别数据库可能读不了。
 */
export function coreSwitchWarnings(before?: BrowserCoreExtended, after?: BrowserCoreExtended): string[] {
  if (!before || !after || before.coreId === after.coreId) return []
  const out: string[] = []
  if (before.coreKind !== after.coreKind) {
    out.push('内核类型不同 (标准 Chrome 与 Clearcote 的指纹实现不一样), 网站看到的品牌、GPU、Canvas 等会变化, 已登录的账号可能触发风控。登录数据会保留。')
  }
  const from = major(before.chromeVersion)
  const to = major(after.chromeVersion)
  if (Number.isFinite(from) && Number.isFinite(to) && to < from) {
    out.push(`从 ${from} 降到 ${to}, 个别网站的本地数据可能读不了, 建议先做个快照。`)
  }
  return out
}
