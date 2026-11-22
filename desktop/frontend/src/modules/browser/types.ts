// 浏览器模块的数据结构, 字段名与后端 JSON 一致 (见 desktop/internal/browser/model.go 与 api_*.go)。

// ---- 实例 ----

/** 实例里用户可编辑的部分, 新建 / 更新时提交 */
export interface BrowserProfileInput {
  profileName: string
  /** 留空由后端生成 */
  userDataDir: string
  /** 空串表示跟随默认内核, 启动时才解析 */
  coreId: string
  fingerprintArgs: string[]
  /** 代理池 id; 为空时用 proxyConfig */
  proxyId: string
  proxyConfig: string
  launchArgs: string[]
  tags: string[]
  /** 已不在界面维护, 编辑时原样带回 */
  keywords: string[]
  groupId?: string
}

export interface BrowserProfile extends BrowserProfileInput {
  profileId: string
  running: boolean
  debugPort: number
  pid: number
  lastError: string
  createdAt: string
  updatedAt: string
  lastStartAt?: string
  lastStopAt?: string
}

export interface BrowserTab {
  tabId: string
  title: string
  url: string
  active: boolean
}

export interface CookieInfo {
  name: string
  value: string
  domain: string
  path: string
  /** Unix 秒; 0 或负数表示会话 Cookie */
  expires: number
  httpOnly: boolean
  secure: boolean
  sameSite: string
}

export interface SnapshotInfo {
  snapshotId: string
  profileId: string
  name: string
  sizeMB: number
  createdAt: string
}

/** 设置 → 新实例默认值 */
export interface BrowserSettings {
  userDataRoot: string
  defaultFingerprintArgs: string[]
  defaultLaunchArgs: string[]
  /** 代理池 id; 含 :// 的是旧版直接存的地址 */
  defaultProxy: string
}

// ---- 分组 (单层) ----

export interface BrowserGroupInput {
  groupName: string
  /** 恒为空串, 保留字段兼容旧数据 */
  parentId: string
  sortOrder: number
}

export interface BrowserGroup extends BrowserGroupInput {
  groupId: string
  createdAt: string
  updatedAt: string
}

export interface BrowserGroupWithCount extends BrowserGroup {
  instanceCount: number
}

// ---- 内核 ----

/** chrome = 官方 Chrome (启动后经 CDP 注入时区与反检测); clearcote = 引擎层指纹, 用原生启动参数 */
export type CoreKind = 'chrome' | 'clearcote'

export interface BrowserCore {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
  tags?: string[]
}

/** 内核的派生信息 (版本、类型、使用实例数), 由后端扫描得出 */
export interface BrowserCoreExtended {
  coreId: string
  chromeVersion: string
  instanceCount: number
  coreKind: CoreKind
}

export interface BrowserCoreValidateResult {
  valid: boolean
  message: string
}

// ---- 代理 ----

export interface BrowserProxy {
  proxyId: string
  proxyName: string
  proxyConfig: string
  dnsServers?: string
  groupName?: string
  tags?: string[]
  // 订阅导入的来源 (没有来源地址时后端会清空这几项)
  sourceId?: string
  sourceUrl?: string
  sourceNamePrefix?: string
  sourceAutoRefresh?: boolean
  sourceRefreshIntervalM?: number
  sourceLastRefreshAt?: string
  // 后端记录的最近一次测速 / 出口查询
  lastLatencyMs?: number
  lastTestOk?: boolean
  lastTestedAt?: string
  lastIPHealthJson?: string
}

/** 代理出口 IP 查询结果 (ip.czl.net) */
export interface ProxyIPHealthResult {
  proxyId: string
  ok: boolean
  source: string
  error: string
  ip: string
  country: string
  countryCode: string
  region: string
  city: string
  timezone: string
  isp: string
  organization: string
  asn: number
  asName: string
  rawData: Record<string, unknown>
  updatedAt: string
}

// ---- 书签 ----

export interface BrowserBookmark {
  name: string
  url: string
  tags?: string[]
}
