import type { CoreKind } from '../types'

export const CORE_KIND_LABEL: Record<CoreKind, string> = {
  chrome: '标准 Chrome',
  clearcote: 'Clearcote',
}

/** 两类内核实现指纹的方式不同, 列表悬停时提示 */
export const CORE_KIND_HINT: Record<CoreKind, string> = {
  chrome: '官方 Chrome: 指纹开关不生效, 时区与反检测在启动后经 CDP 注入',
  clearcote: '引擎层指纹与时区, 直接用启动参数, 不注入脚本',
}
