import { create } from 'zustand'
import {
  checkAppUpdate,
  downloadAppUpdate,
  installAppUpdate,
  type UpdateCheckResult,
  type UpdateProgress,
} from '../modules/settings/api'

interface UpdateState {
  result: UpdateCheckResult | null
  checking: boolean
  installing: boolean
  progress: UpdateProgress | null
  error: string
  check: () => Promise<UpdateCheckResult | null>
  download: () => Promise<void>
  install: () => Promise<void>
  setProgress: (progress: UpdateProgress) => void
}

export const useUpdateStore = create<UpdateState>((set, get) => ({
  result: null,
  checking: false,
  installing: false,
  progress: null,
  error: '',

  check: async () => {
    if (get().checking) return get().result
    set({ checking: true, error: '' })
    try {
      const result = await checkAppUpdate()
      set({ result })
      return result
    } catch (error: any) {
      set({ error: error?.message || String(error) || '检查更新失败' })
      return null
    } finally {
      set({ checking: false })
    }
  },

  download: async () => {
    set({ error: '', progress: { phase: 'downloading', progress: 0, downloaded: 0, total: get().result?.size || 0, message: '准备下载...' } })
    try {
      await downloadAppUpdate()
    } catch (error: any) {
      set({ progress: null, error: error?.message || String(error) || '下载失败' })
    }
  },

  install: async () => {
    set({ installing: true, error: '' })
    try {
      await installAppUpdate()
    } catch (error: any) {
      set({ installing: false, error: error?.message || String(error) || '安装失败' })
    }
  },

  setProgress: (progress) => set((state) => ({
    progress,
    error: progress.phase === 'error' ? progress.message : '',
    result: state.result && progress.phase === 'ready'
      ? { ...state.result, downloaded: true, downloading: false }
      : state.result,
  })),
}))
