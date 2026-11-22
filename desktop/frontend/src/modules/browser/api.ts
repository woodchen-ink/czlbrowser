// 浏览器模块调用后端的入口。直接调用 Wails 生成的绑定; 后端出错时 Promise 以 "<码> <短语>" 拒绝,
// 由调用方用 errorText 提示。返回值统一兜底成空数组 / null, 页面不用判空 undefined。
import * as App from '../../wailsjs/go/main/App'
import { guardedStart } from '../cloud/launch'
import type {
  BrowserBookmark,
  BrowserCore,
  BrowserCoreExtended,
  BrowserCoreValidateResult,
  BrowserGroup,
  BrowserGroupInput,
  BrowserGroupWithCount,
  BrowserProfile,
  BrowserProfileInput,
  BrowserProxy,
  BrowserSettings,
  BrowserTab,
  CookieInfo,
  CoreKind,
  ProxyIPHealthResult,
  SnapshotInfo,
} from './types'

// 生成的绑定类型与本模块的接口结构一致, 只是可选字段写法不同, 这里统一放宽。
const call = App as unknown as Record<string, (...args: any[]) => Promise<any>>

const list = async <T>(p: Promise<T[] | null | undefined>): Promise<T[]> => (await p) || []
const one = async <T>(p: Promise<T | null | undefined>): Promise<T | null> => (await p) ?? null
const done = async (p: Promise<unknown>): Promise<boolean> => {
  await p
  return true
}

// ---- 实例 ----

export const fetchBrowserProfiles = () => list<BrowserProfile>(call.BrowserProfileList())
export const fetchAllTags = () => list<string>(call.BrowserGetAllTags())
export const createBrowserProfile = (input: BrowserProfileInput) => one<BrowserProfile>(call.BrowserProfileCreate(input))
export const updateBrowserProfile = (id: string, input: BrowserProfileInput) => one<BrowserProfile>(call.BrowserProfileUpdate(id, input))
export const deleteBrowserProfile = (id: string) => done(call.BrowserProfileDelete(id))
export const copyBrowserProfile = (id: string, newName: string) => one<BrowserProfile>(call.BrowserProfileCopy(id, newName))

/** 启动实例; 同步空间里被占用或登录态下载失败时先问用户, 用户取消返回 null。 */
export const startBrowserInstance = (id: string) =>
  guardedStart<BrowserProfile | null>(
    () => one<BrowserProfile>(call.BrowserInstanceStart(id)),
    () => one<BrowserProfile>(call.BrowserInstanceStartForce(id)),
  )
export const stopBrowserInstance = (id: string) => one<BrowserProfile>(call.BrowserInstanceStop(id))
export const restartBrowserInstance = (id: string) => one<BrowserProfile>(call.BrowserInstanceRestart(id))
export const openBrowserUrl = async (id: string, url: string) => (await call.BrowserInstanceOpenUrl(id, url)) === true
export const fetchBrowserTabs = (id: string) => list<BrowserTab>(call.BrowserInstanceGetTabs(id))

// ---- 新实例默认值 ----

export const fetchBrowserSettings = async (): Promise<BrowserSettings> =>
  (await call.GetBrowserSettings()) || { userDataRoot: 'data', defaultFingerprintArgs: [], defaultLaunchArgs: [], defaultProxy: '' }
export const saveBrowserSettings = (s: BrowserSettings) => done(call.SaveBrowserSettings(s))

// ---- 内核 ----

export const fetchBrowserCores = () => list<BrowserCore>(call.BrowserCoreList())
export const deleteBrowserCore = (id: string) => done(call.BrowserCoreDelete(id))
export const setDefaultBrowserCore = (id: string) => done(call.BrowserCoreSetDefault(id))
export const setCoreTags = async (id: string, tags: string[]) => {
  await call.BrowserCoreSetTags(id, tags)
}
export const validateBrowserCorePath = async (dir: string): Promise<BrowserCoreValidateResult> =>
  (await call.BrowserCoreValidate(dir)) || { valid: false, message: '校验失败' }
export const fetchCoreExtendedInfo = () => list<BrowserCoreExtended>(call.BrowserCoreExtendedInfo())

/** 内置内核清单里的一项 (来自 Chrome-exe 发布的 cores.json)。 */
export interface BuiltinCoreSource {
  id: string
  name: string
  version: string
  kind: CoreKind
  description: string
  installed: boolean
}

export const fetchBuiltinCores = () => list<BuiltinCoreSource>(call.BrowserBuiltinCores())
export const refreshBuiltinCores = () => list<BuiltinCoreSource>(call.BrowserBuiltinCoresRefresh())
export const downloadBuiltinCore = async (id: string, proxyConfig: string) => {
  await call.BrowserBuiltinCoreDownload(id, proxyConfig)
}

// ---- 代理池 ----

export type ProxyTestResult = { proxyId: string; ok: boolean; latencyMs: number; error: string }

export const fetchBrowserProxies = () => list<BrowserProxy>(call.BrowserProxyList())
export const fetchBrowserProxyGroups = () => list<string>(call.BrowserProxyListGroups())
export const saveBrowserProxies = (proxies: BrowserProxy[]) => done(call.SaveBrowserProxies(proxies))
export const validateProxyConfig = async (proxyConfig: string, proxyId: string): Promise<{ supported: boolean; errorMsg: string }> =>
  (await call.ValidateProxyConfig(proxyConfig, proxyId)) || { supported: true, errorMsg: '' }
export const browserProxyTestSpeed = async (id: string): Promise<ProxyTestResult> =>
  (await call.BrowserProxyTestSpeed(id)) || { proxyId: id, ok: false, latencyMs: 0, error: '调用失败' }
export const browserProxyBatchTestSpeed = (ids: string[], concurrency = 20) =>
  list<ProxyTestResult>(call.BrowserProxyBatchTestSpeed(ids, concurrency))
export const browserProxyCheckIPHealth = async (id: string): Promise<ProxyIPHealthResult> =>
  (await call.BrowserProxyCheckIPHealth(id)) || failedIPHealth(id)
export const browserProxyBatchCheckIPHealth = (ids: string[], concurrency = 10) =>
  list<ProxyIPHealthResult>(call.BrowserProxyBatchCheckIPHealth(ids, concurrency))
/** 重命名代理分组; newName 为空表示删除分组 (代理移到未分组)。 */
export const renameProxyGroup = async (oldName: string, newName: string) => {
  await call.TaxonomyRenameProxyGroup(oldName, newName)
}

function failedIPHealth(proxyId: string): ProxyIPHealthResult {
  return {
    proxyId, ok: false, source: 'ip.czl.net', error: '调用失败', ip: '', country: '', countryCode: '', region: '', city: '',
    timezone: '', isp: '', organization: '', asn: 0, asName: '', rawData: {}, updatedAt: new Date().toISOString(),
  }
}

// ---- 文件夹 ----

export const openUserDataDir = (dir: string) => done(call.OpenUserDataDir(dir))
export const openCorePath = (dir: string) => done(call.OpenCorePath(dir))

// ---- Cookie 与快照 (实例详情页) ----

export const fetchBrowserCookies = (id: string) => list<CookieInfo>(call.BrowserGetCookies(id))
export const clearBrowserCookies = (id: string) => done(call.BrowserClearCookies(id))
export const exportBrowserCookies = async (id: string): Promise<string> => (await call.BrowserExportCookies(id)) || ''
export const listSnapshots = (id: string) => list<SnapshotInfo>(call.BrowserSnapshotList(id))
export const createSnapshot = (id: string, name: string) => one<SnapshotInfo>(call.BrowserSnapshotCreate(id, name))
export const restoreSnapshot = (id: string, snapshotId: string) => done(call.BrowserSnapshotRestore(id, snapshotId))
export const deleteSnapshot = (id: string, snapshotId: string) => done(call.BrowserSnapshotDelete(id, snapshotId))

// ---- 书签 ----

export const fetchBookmarks = () => list<BrowserBookmark>(call.BookmarkList())
export const saveBookmarks = (items: BrowserBookmark[]) => done(call.BookmarkSave(items))
export const resetBookmarks = () => done(call.BookmarkReset())

// ---- 分组 ----

export const fetchGroups = () => list<BrowserGroupWithCount>(call.ListGroups())
export const createGroup = (input: BrowserGroupInput) => one<BrowserGroup>(call.CreateGroup(input))
export const updateGroup = (id: string, input: BrowserGroupInput) => one<BrowserGroup>(call.UpdateGroup(id, input))
export const deleteGroup = (id: string) => done(call.DeleteGroup(id))
export const moveInstancesToGroup = (ids: string[], groupId: string) => done(call.MoveInstancesToGroup(ids, groupId))

// ---- 标签 (设置 → 分组与标签) ----

export type TaxonomyKind = 'profile' | 'proxy' | 'core' | 'bookmark'

export const renameTaxonomyTag = async (kind: TaxonomyKind, oldName: string, newName: string) => {
  await call.TaxonomyRenameTag(kind, oldName, newName)
}
export const deleteTaxonomyTag = async (kind: TaxonomyKind, name: string) => {
  await call.TaxonomyDeleteTag(kind, name)
}
