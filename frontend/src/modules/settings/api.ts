// Settings 模块 API
const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

export interface BackupActionResult {
  cancelled?: boolean
  message?: string
  zipPath?: string
  resetFirst?: boolean
  imported?: number
  skipped?: number
  conflicts?: number
  partial?: boolean
  componentTotal?: number
  componentSuccess?: number
  componentFailed?: number
  failedComponents?: Array<{
    componentId?: string
    componentName?: string
    error?: string
  }>
}

export async function initializeSystemData(): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupInitializeSystem) {
    return { cancelled: false, message: '当前环境不支持后端初始化接口' }
  }
  return (await bindings.BackupInitializeSystem()) || {}
}

export async function exportSystemConfig(): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupExportPackage) {
    return { cancelled: false, message: '当前环境不支持后端导出接口' }
  }
  return (await bindings.BackupExportPackage()) || {}
}

export async function importSystemConfig(resetFirst: boolean): Promise<BackupActionResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BackupImportPackage) {
    return { cancelled: false, message: '当前环境不支持后端加载接口' }
  }
  return (await bindings.BackupImportPackage(resetFirst)) || {}
}


export interface UpdateCheckResult {
  currentVersion: string
  latestVersion: string
  hasUpdate: boolean
  devBuild: boolean
  changelog: string
  publishedAt: string
  size: number
  downloaded: boolean
  downloading: boolean
  checkedAt: string
}

export interface UpdateProgress {
  phase: 'downloading' | 'ready' | 'error'
  progress: number
  downloaded: number
  total: number
  message: string
}

export async function checkAppUpdate(): Promise<UpdateCheckResult> {
  const bindings: any = await getBindings()
  if (!bindings?.UpdateCheck) {
    throw new Error('当前环境不支持检查更新')
  }
  return await bindings.UpdateCheck()
}

export async function downloadAppUpdate(): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.UpdateDownload) {
    throw new Error('当前环境不支持下载更新')
  }
  await bindings.UpdateDownload()
}

export async function installAppUpdate(): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.UpdateInstall) {
    throw new Error('当前环境不支持安装更新')
  }
  await bindings.UpdateInstall()
}
