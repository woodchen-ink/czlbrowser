import { create } from 'zustand'
import {
  cloudApi,
  LOCAL_SPACE,
  type BillingState,
  type DeviceRequest,
  type Presence,
  type SpaceInfo,
  type SyncState,
} from '../modules/cloud/api'
import { errorText } from '../modules/cloud/errors'
import { toast } from '../shared/components'

// 云端同步的全局状态: 同步与空间、订阅、团队成员正在使用的实例、待批准的新设备。
// 后端经事件推送变化 (sync:* / billing:changed), initCloudEvents 在登录后的壳里挂一次。

interface CloudState {
  sync: SyncState | null
  billing: BillingState | null
  /** 各同步空间缺什么: subscribe / seats / 空 */
  needs: Record<string, string>
  /** 空间 → 他人正在使用的实例 */
  presence: Record<string, Presence[]>
  deviceRequests: DeviceRequest[]
  /** 本机发起新设备授权后拿到的 6 位核对码 */
  deviceCode: string
  /** 数据变更计数, 列表页据此重新加载 */
  changeTick: number
  /** 刚生成、还没确认保存的恢复码; 由常驻的 CloudOverlays 展示, 不随加密面板卸载 */
  recoveryCode: string

  load: () => Promise<void>
  refresh: () => Promise<void>
  refreshBilling: (force?: boolean) => Promise<void>
  switchSpace: (key: string) => Promise<boolean>
  currentSpace: () => SpaceInfo | null
  setDeviceCode: (code: string) => void
  setDeviceRequests: (list: DeviceRequest[]) => void
  setRecoveryCode: (code: string) => void
}

export const useCloudStore = create<CloudState>((set, get) => ({
  sync: null,
  billing: null,
  needs: {},
  presence: {},
  deviceRequests: [],
  deviceCode: '',
  changeTick: 0,
  recoveryCode: '',

  load: async () => {
    try {
      const [sync, billing] = await Promise.all([cloudApi.state(), cloudApi.billingState()])
      set({ sync, billing })
    } catch {
      // 绑定不可用 (浏览器预览) 时保持空
    }
  },

  refresh: async () => {
    try {
      const sync = await cloudApi.refresh()
      set({ sync })
      const needs = await cloudApi.billingSpaceNeeds()
      set({ needs })
    } catch {
      // 状态里自带 available / error, 这里不打扰
    }
  },

  refreshBilling: async (force = false) => {
    try {
      const billing = await cloudApi.billingRefresh(force)
      const needs = await cloudApi.billingSpaceNeeds()
      set({ billing, needs })
    } catch {
      // 同上
    }
  },

  switchSpace: async (key: string) => {
    try {
      const sync = await cloudApi.switchSpace(key)
      set((s) => ({ sync, changeTick: s.changeTick + 1 }))
      return true
    } catch (err) {
      toast.error(errorText(err))
      return false
    }
  },

  currentSpace: () => {
    const st = get().sync
    if (!st || !st.current || st.current === LOCAL_SPACE) return null
    return st.spaces?.find((s) => s.key === st.current) || null
  },

  setDeviceCode: (code: string) => set({ deviceCode: code }),
  setDeviceRequests: (list: DeviceRequest[]) => set({ deviceRequests: list || [] }),
  setRecoveryCode: (code: string) => set({ recoveryCode: code }),
}))

/** 挂上云端相关的后端事件, 返回取消函数。 */
export function initCloudEvents(): () => void {
  const runtime = (window as any).runtime
  if (!runtime?.EventsOn) return () => {}
  const offs: Array<() => void> = []
  const on = (name: string, fn: (data: any) => void) => {
    const off = runtime.EventsOn(name, fn)
    offs.push(typeof off === 'function' ? off : () => runtime.EventsOff?.(name))
  }

  on('sync:state', (sync: SyncState) => {
    useCloudStore.setState({ sync })
    void cloudApi.billingSpaceNeeds().then((needs) => useCloudStore.setState({ needs })).catch(() => {})
  })
  on('sync:changed', (data: { space: string }) => {
    if (data?.space === useCloudStore.getState().sync?.current) {
      useCloudStore.setState((s) => ({ changeTick: s.changeTick + 1 }))
    }
  })
  on('sync:presence', (data: { space: string; items: Presence[] }) => {
    useCloudStore.setState((s) => ({ presence: { ...s.presence, [data.space]: data.items || [] } }))
  })
  on('sync:device:requests', (list: DeviceRequest[]) => {
    useCloudStore.getState().setDeviceRequests(list)
  })
  on('sync:device:code', (data: { code: string }) => {
    useCloudStore.getState().setDeviceCode(data?.code || '')
  })
  on('billing:changed', (billing: BillingState) => {
    useCloudStore.setState({ billing })
  })

  void useCloudStore.getState().load()
  void useCloudStore.getState().refresh()
  void useCloudStore.getState().refreshBilling()
  void cloudApi.deviceRequests().then((list) => useCloudStore.getState().setDeviceRequests(list)).catch(() => {})

  // 回到窗口时刷新订阅 (付款在浏览器里完成, 后端 30 秒内用缓存)
  const onFocus = () => void useCloudStore.getState().refreshBilling()
  window.addEventListener('focus', onFocus)
  offs.push(() => window.removeEventListener('focus', onFocus))

  return () => offs.forEach((off) => off())
}
