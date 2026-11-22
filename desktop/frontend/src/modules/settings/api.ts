// 设置页调用后端的入口: 备份与恢复、软件更新、运行日志。
import * as App from '../../wailsjs/go/main/App'

/** 备份导出 / 导入 / 初始化的返回 (后端返回 map, 字段按操作不同而有无)。 */
export interface BackupResult {
  cancelled?: boolean
  message?: string
  zipPath?: string
  imported?: number
  skipped?: number
  conflicts?: number
  partial?: boolean
  componentTotal?: number
  componentFailed?: number
  failedComponents?: { componentId?: string; componentName?: string; error?: string }[]
}

/** backup:export:progress / backup:import:progress 事件内容 */
export interface BackupProgress {
  phase: string // starting / preparing / writing / importing / done / cancelled / error
  progress: number
  message: string
  componentId?: string
  componentName?: string
  entryIndex?: number
  entryTotal?: number
  timestamp?: string
}

const asResult = async (p: Promise<Record<string, any>>): Promise<BackupResult> => ((await p) ?? {}) as BackupResult

export const exportBackup = () => asResult(App.BackupExportPackage())
export const importBackup = (resetFirst: boolean) => asResult(App.BackupImportPackage(resetFirst))
export const resetAllData = () => asResult(App.BackupInitializeSystem())

// ---- 软件更新 ----

export type UpdateCheckResult = Awaited<ReturnType<typeof App.UpdateCheck>>

/** update:progress 事件内容 */
export interface UpdateProgress {
  phase: 'downloading' | 'ready' | 'error'
  progress: number
  downloaded: number
  total: number
  message: string
}

export const checkAppUpdate = () => App.UpdateCheck()
export const downloadAppUpdate = () => App.UpdateDownload()
export const installAppUpdate = () => App.UpdateInstall()

// ---- 运行日志 ----

export type LogEntry = Awaited<ReturnType<typeof App.GetAppLogs>>[number]

export const fetchLogs = async (): Promise<LogEntry[]> => (await App.GetAppLogs()) ?? []
export const clearLogs = () => App.ClearAppLogs()
