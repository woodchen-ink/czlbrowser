import { create } from 'zustand'

/** 正在加载备份时的进度; 关闭窗口确认框据此提醒用户别中途退出。 */
export interface ImportRun {
  progress: number
  message: string
}

interface BackupStore {
  importing: ImportRun | null
  setImporting: (run: ImportRun | null) => void
}

export const useBackupStore = create<BackupStore>((set) => ({
  importing: null,
  setImporting: (importing) => set({ importing }),
}))
