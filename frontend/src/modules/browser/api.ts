import type { BrowserProfile, BrowserProfileInput, BrowserTab, BrowserSettings, BrowserCore, BrowserCoreInput, BrowserCoreValidateResult, BrowserProxy, BrowserCoreExtended, CookieInfo, SnapshotInfo, BrowserBookmark, BrowserGroup, BrowserGroupInput, BrowserGroupWithCount, ProxyIPHealthResult } from './types'

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

let mockProfiles: BrowserProfile[] = [
  {
    profileId: 'mock-1',
    profileName: '默认指纹配置',
    userDataDir: 'data/default',
    coreId: 'default',
    fingerprintArgs: ['--fingerprint-brand=Chrome', '--fingerprint-platform=windows'],
    proxyId: '',
    proxyConfig: '',
    launchArgs: ['--disable-features=Translate'],
    tags: ['默认'],
    keywords: [],
    running: false,
    debugPort: 0,
    pid: 0,
    lastError: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
]

let mockCores: BrowserCore[] = []

let mockProxies: BrowserProxy[] = []

// ============================================================================
// Profile API
// ============================================================================

export async function fetchBrowserProfiles(): Promise<BrowserProfile[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileList) {
    return (await bindings.BrowserProfileList()) || []
  }
  return mockProfiles
}

export async function fetchBrowserProfilesByTag(tag: string): Promise<BrowserProfile[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileListByTag) {
    return (await bindings.BrowserProfileListByTag(tag)) || []
  }
  return mockProfiles.filter(p => p.tags?.includes(tag))
}

export async function fetchAllTags(): Promise<string[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserGetAllTags) {
    return (await bindings.BrowserGetAllTags()) || []
  }
  const set = new Set<string>()
  mockProfiles.forEach(p => p.tags?.forEach(t => set.add(t)))
  return Array.from(set).sort()
}

export async function createBrowserProfile(input: BrowserProfileInput): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileCreate) {
    return (await bindings.BrowserProfileCreate(input)) || null
  }
  const profile: BrowserProfile = {
    profileId: `mock-${Date.now()}`,
    ...input,
    keywords: input.keywords || {},
    running: false,
    debugPort: 0,
    pid: 0,
    lastError: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
  mockProfiles = [profile, ...mockProfiles]
  return profile
}

export async function updateBrowserProfile(profileId: string, input: BrowserProfileInput): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileUpdate) {
    return (await bindings.BrowserProfileUpdate(profileId, input)) || null
  }
  const index = mockProfiles.findIndex(item => item.profileId === profileId)
  if (index === -1) return null
  mockProfiles[index] = { ...mockProfiles[index], ...input, updatedAt: new Date().toISOString() }
  return mockProfiles[index]
}

export async function deleteBrowserProfile(profileId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileDelete) {
    await bindings.BrowserProfileDelete(profileId)
    return true
  }
  mockProfiles = mockProfiles.filter(item => item.profileId !== profileId)
  return true
}

export async function copyBrowserProfile(profileId: string, newName: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileCopy) {
    return (await bindings.BrowserProfileCopy(profileId, newName)) || null
  }
  // mock
  const src = mockProfiles.find(p => p.profileId === profileId)
  if (!src) return null
  const copy: BrowserProfile = {
    ...src,
    profileId: `mock-${Date.now()}`,
    profileName: newName || src.profileName + ' (副本)',
    userDataDir: `mock-${Date.now()}`,
    running: false,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
  mockProfiles = [copy, ...mockProfiles]
  return copy
}

// ============================================================================
// Instance API
// ============================================================================

export async function startBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStart) {
    return (await bindings.BrowserInstanceStart(profileId)) || null
  }
  mockProfiles = mockProfiles.map(item =>
    item.profileId === profileId ? { ...item, running: true, debugPort: 9222, pid: Math.floor(Math.random() * 100000), lastStartAt: new Date().toISOString() } : item
  )
  return mockProfiles.find(item => item.profileId === profileId) || null
}

export async function stopBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStop) {
    return (await bindings.BrowserInstanceStop(profileId)) || null
  }
  mockProfiles = mockProfiles.map(item =>
    item.profileId === profileId ? { ...item, running: false, pid: 0, lastStopAt: new Date().toISOString() } : item
  )
  return mockProfiles.find(item => item.profileId === profileId) || null
}

export async function restartBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceRestart) {
    return (await bindings.BrowserInstanceRestart(profileId)) || null
  }
  await stopBrowserInstance(profileId)
  return await startBrowserInstance(profileId)
}

export async function openBrowserUrl(profileId: string, targetUrl: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceOpenUrl) {
    return (await bindings.BrowserInstanceOpenUrl(profileId, targetUrl)) === true
  }
  return true
}

export async function fetchBrowserTabs(profileId: string): Promise<BrowserTab[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceGetTabs) {
    return (await bindings.BrowserInstanceGetTabs(profileId)) || []
  }
  return [
    { tabId: 'tab-1', title: '新标签页', url: 'about:blank', active: true },
    { tabId: 'tab-2', title: '示例站点', url: 'https://example.com', active: false },
  ]
}

// ============================================================================
// Settings API
// ============================================================================

export async function fetchBrowserSettings(): Promise<BrowserSettings> {
  const bindings: any = await getBindings()
  if (bindings?.GetBrowserSettings) {
    return (await bindings.GetBrowserSettings()) || { userDataRoot: 'data', defaultFingerprintArgs: [], defaultLaunchArgs: [], defaultProxy: '' }
  }
  return { userDataRoot: 'data', defaultFingerprintArgs: [], defaultLaunchArgs: [], defaultProxy: '' }
}

export async function saveBrowserSettings(settings: BrowserSettings): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SaveBrowserSettings) {
    await bindings.SaveBrowserSettings(settings)
    return true
  }
  return true
}

// ============================================================================
// Core API
// ============================================================================

export async function fetchBrowserCores(): Promise<BrowserCore[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreList) {
    return (await bindings.BrowserCoreList()) || []
  }
  return mockCores
}

export async function saveBrowserCore(input: BrowserCoreInput): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreSave) {
    await bindings.BrowserCoreSave(input)
    return true
  }
  const index = mockCores.findIndex(c => c.coreId === input.coreId)
  if (index >= 0) {
    mockCores[index] = input
  } else {
    mockCores.push({ ...input, coreId: input.coreId || `core-${Date.now()}` })
  }
  return true
}

export async function deleteBrowserCore(coreId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreDelete) {
    await bindings.BrowserCoreDelete(coreId)
    return true
  }
  mockCores = mockCores.filter(c => c.coreId !== coreId)
  return true
}

export async function setDefaultBrowserCore(coreId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreSetDefault) {
    await bindings.BrowserCoreSetDefault(coreId)
    return true
  }
  mockCores = mockCores.map(c => ({ ...c, isDefault: c.coreId === coreId }))
  return true
}

export async function validateBrowserCorePath(corePath: string): Promise<BrowserCoreValidateResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreValidate) {
    return (await bindings.BrowserCoreValidate(corePath)) || { valid: false, message: '验证失败' }
  }
  return { valid: true, message: '路径有效（模拟）' }
}

export async function fetchCoreExtendedInfo(): Promise<BrowserCoreExtended[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreExtendedInfo) {
    return (await bindings.BrowserCoreExtendedInfo()) || []
  }
  return []
}

export async function scanBrowserCores(): Promise<BrowserCore[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreScan) {
    return (await bindings.BrowserCoreScan()) || []
  }
  return mockCores
}

export async function BrowserCoreDownload(coreName: string, url: string, proxyConfig?: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreDownload) {
    await bindings.BrowserCoreDownload(coreName, url, proxyConfig || '')
    return true
  }
  return true
}

export interface BuiltinCoreSource {
  id: string
  name: string
  version: string
  description: string
  installed: boolean
}

export async function fetchBuiltinCores(): Promise<BuiltinCoreSource[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserBuiltinCores) {
    return (await bindings.BrowserBuiltinCores()) || []
  }
  return []
}

export async function downloadBuiltinCore(builtinId: string, proxyConfig: string): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserBuiltinCoreDownload) {
    await bindings.BrowserBuiltinCoreDownload(builtinId, proxyConfig)
  }
}

// ============================================================================
// Proxy API
// ============================================================================

export async function fetchBrowserProxies(): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyList) {
    return (await bindings.BrowserProxyList()) || []
  }
  return mockProxies
}

export async function fetchBrowserProxyGroups(): Promise<string[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListGroups) {
    return (await bindings.BrowserProxyListGroups()) || []
  }
  return []
}

export async function fetchBrowserProxiesByGroup(groupName: string): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListByGroup) {
    return (await bindings.BrowserProxyListByGroup(groupName)) || []
  }
  return mockProxies.filter(p => p.groupName === groupName)
}

export interface ClashImportURLResult {
  url: string
  content: string
  proxyCount: number
  dnsServers?: string
  suggestedGroup?: string
}

export async function fetchClashImportFromURL(targetURL: string): Promise<ClashImportURLResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyFetchClashByURL) {
    return (await bindings.BrowserProxyFetchClashByURL(targetURL)) || {
      url: targetURL,
      content: '',
      proxyCount: 0,
    }
  }

  // 兜底：wailsjs 尚未刷新时，直接通过 window.go 调用后端绑定
  const goApp = (window as any).go?.main?.App
  if (goApp?.BrowserProxyFetchClashByURL) {
    return (await goApp.BrowserProxyFetchClashByURL(targetURL)) || {
      url: targetURL,
      content: '',
      proxyCount: 0,
    }
  }

  throw new Error('当前环境不支持 URL 导入 Clash 配置')
}

export async function saveBrowserProxies(proxies: BrowserProxy[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SaveBrowserProxies) {
    await bindings.SaveBrowserProxies(proxies)
    return true
  }
  mockProxies = proxies
  return true
}

export async function validateProxyConfig(proxyConfig: string, proxyId: string): Promise<{ supported: boolean; errorMsg: string }> {
  const bindings: any = await getBindings()
  if (bindings?.ValidateProxyConfig) {
    return (await bindings.ValidateProxyConfig(proxyConfig, proxyId)) || { supported: true, errorMsg: '' }
  }
  return { supported: true, errorMsg: '' }
}

export async function testProxyConnectivity(proxyId: string, proxyConfig: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyConnectivity) {
    return (await bindings.TestProxyConnectivity(proxyId, proxyConfig)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  // mock: simulate latency
  await new Promise(r => setTimeout(r, 300 + Math.random() * 500))
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 200), error: '' }
}

export async function testProxyRealConnectivity(proxyId: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyRealConnectivity) {
    return (await bindings.TestProxyRealConnectivity(proxyId)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  // mock: simulate latency 300-800ms
  await new Promise(r => setTimeout(r, 300 + Math.random() * 500))
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }
}

export async function browserProxyTestSpeed(proxyId: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyTestSpeed) {
    return (await bindings.BrowserProxyTestSpeed(proxyId)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  await new Promise(r => setTimeout(r, 300 + Math.random() * 500))
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }
}

export async function browserProxyBatchTestSpeed(proxyIds: string[], concurrency: number = 20): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchTestSpeed) {
    return (await bindings.BrowserProxyBatchTestSpeed(proxyIds, concurrency)) || []
  }
  // mock
  await new Promise(r => setTimeout(r, 1000))
  return proxyIds.map(id => ({ proxyId: id, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }))
}

export async function browserProxyCheckIPHealth(proxyId: string): Promise<ProxyIPHealthResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCheckIPHealth) {
    return (await bindings.BrowserProxyCheckIPHealth(proxyId)) || {
      proxyId,
      ok: false,
      source: 'ip.czl.net',
      error: '调用失败',
      ip: '',
      country: '',
      countryCode: '',
      region: '',
      city: '',
      timezone: '',
      isp: '',
      organization: '',
      asn: 0,
      asName: '',
      rawData: {},
      updatedAt: new Date().toISOString(),
    }
  }
  await new Promise(r => setTimeout(r, 600))
  return {
    proxyId,
    ok: true,
    source: 'ip.czl.net',
    error: '',
    ip: '127.0.0.1',
    country: 'Mock',
    countryCode: 'MK',
    region: 'Mock',
    city: 'Mock',
    timezone: 'UTC',
    isp: 'Mock ISP',
    organization: 'Mock Org',
    asn: 12345,
    asName: 'MOCK',
    rawData: {},
    updatedAt: new Date().toISOString(),
  }
}

export async function browserProxyBatchCheckIPHealth(proxyIds: string[], concurrency: number = 10): Promise<ProxyIPHealthResult[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchCheckIPHealth) {
    return (await bindings.BrowserProxyBatchCheckIPHealth(proxyIds, concurrency)) || []
  }
  await new Promise(r => setTimeout(r, 1200))
  return proxyIds.map(proxyId => ({
    proxyId,
    ok: true,
    source: 'ip.czl.net',
    error: '',
    ip: '127.0.0.1',
    country: 'Mock',
    countryCode: 'MK',
    region: 'Mock',
    city: 'Mock',
    timezone: 'UTC',
    isp: 'Mock ISP',
    organization: 'Mock Org',
    asn: 12345,
    asName: 'MOCK',
    rawData: {},
    updatedAt: new Date().toISOString(),
  }))
}

export async function openUserDataDir(userDataDir: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.OpenUserDataDir) {
    await bindings.OpenUserDataDir(userDataDir)
    return true
  }
  return false
}

export async function openCorePath(corePath: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.OpenCorePath) {
    await bindings.OpenCorePath(corePath)
    return true
  }
  return false
}

// ============================================================================
// Cookie API
// ============================================================================

export async function fetchBrowserCookies(profileId: string): Promise<CookieInfo[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserGetCookies) {
    return (await bindings.BrowserGetCookies(profileId)) || []
  }
  // mock data
  return [
    { name: 'session', value: 'abc123', domain: '.example.com', path: '/', expires: Date.now() / 1000 + 3600, httpOnly: true, secure: true, sameSite: 'Lax' },
    { name: 'pref', value: 'dark', domain: 'example.com', path: '/', expires: -1, httpOnly: false, secure: false, sameSite: 'None' },
  ]
}

export async function clearBrowserCookies(profileId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserClearCookies) {
    await bindings.BrowserClearCookies(profileId)
    return true
  }
  return true
}

export async function exportBrowserCookies(profileId: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExportCookies) {
    return (await bindings.BrowserExportCookies(profileId)) || ''
  }
  return '# Netscape HTTP Cookie File\n# Generated by BrowserManager\n\n.example.com\tTRUE\t/\tTRUE\t0\tsession\tabc123\n'
}

// ============================================================================
// Snapshot API
// ============================================================================

export async function listSnapshots(profileId: string): Promise<SnapshotInfo[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotList) {
    return (await bindings.BrowserSnapshotList(profileId)) || []
  }
  return []
}

export async function createSnapshot(profileId: string, name: string): Promise<SnapshotInfo | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotCreate) {
    return (await bindings.BrowserSnapshotCreate(profileId, name)) || null
  }
  // mock
  return {
    snapshotId: `snap-${Date.now()}`,
    profileId,
    name,
    sizeMB: 12.5,
    createdAt: new Date().toISOString(),
  }
}

export async function restoreSnapshot(profileId: string, snapshotId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotRestore) {
    await bindings.BrowserSnapshotRestore(profileId, snapshotId)
    return true
  }
  return true
}

export async function deleteSnapshot(profileId: string, snapshotId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotDelete) {
    await bindings.BrowserSnapshotDelete(profileId, snapshotId)
    return true
  }
  return true
}

// ============================================================================
// Bookmark API
// ============================================================================

export async function fetchBookmarks(): Promise<BrowserBookmark[]> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkList) {
    return (await bindings.BookmarkList()) || []
  }
  return [
    { name: 'Google', url: 'https://www.google.com/' },
    { name: 'Gmail', url: 'https://mail.google.com/' },
    { name: 'Claude', url: 'https://claude.ai/' },
    { name: 'ChatGPT', url: 'https://chatgpt.com/' },
    { name: 'YouTube', url: 'https://www.youtube.com/' },
  ]
}

export async function saveBookmarks(items: BrowserBookmark[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkSave) {
    await bindings.BookmarkSave(items)
    return true
  }
  return true
}

export async function resetBookmarks(): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkReset) {
    await bindings.BookmarkReset()
    return true
  }
  return true
}

// ============================================================================
// Keywords API
// ============================================================================

export async function setProfileKeywords(profileId: string, keywords: string[]): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileSetKeywords) {
    return (await bindings.BrowserProfileSetKeywords(profileId, keywords)) || null
  }
  mockProfiles = mockProfiles.map(p =>
    p.profileId === profileId ? { ...p, keywords, updatedAt: new Date().toISOString() } : p
  )
  return mockProfiles.find(p => p.profileId === profileId) || null
}


export async function batchSetProfileTags(profileIds: string[], tags: string[], replace: boolean): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileBatchSetTags) {
    await bindings.BrowserProfileBatchSetTags(profileIds, tags, replace)
    return true
  }
  return true
}

export async function batchRemoveProfileTags(profileIds: string[], tags: string[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileBatchRemoveTags) {
    await bindings.BrowserProfileBatchRemoveTags(profileIds, tags)
    return true
  }
  return true
}

export async function renameBrowserTag(oldName: string, newName: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserRenameTag) {
    await bindings.BrowserRenameTag(oldName, newName)
    return true
  }
  return true
}

// ============================================================================
// Group API
// ============================================================================

export async function fetchGroups(): Promise<BrowserGroupWithCount[]> {
  const bindings: any = await getBindings()
  if (bindings?.ListGroups) {
    return (await bindings.ListGroups()) || []
  }
  return []
}

export async function createGroup(input: BrowserGroupInput): Promise<BrowserGroup | null> {
  const bindings: any = await getBindings()
  if (bindings?.CreateGroup) {
    return (await bindings.CreateGroup(input)) || null
  }
  return null
}

export async function updateGroup(groupId: string, input: BrowserGroupInput): Promise<BrowserGroup | null> {
  const bindings: any = await getBindings()
  if (bindings?.UpdateGroup) {
    return (await bindings.UpdateGroup(groupId, input)) || null
  }
  return null
}

export async function deleteGroup(groupId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.DeleteGroup) {
    await bindings.DeleteGroup(groupId)
    return true
  }
  return false
}

export async function moveInstancesToGroup(profileIds: string[], groupId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.MoveInstancesToGroup) {
    await bindings.MoveInstancesToGroup(profileIds, groupId)
    return true
  }
  return false
}
