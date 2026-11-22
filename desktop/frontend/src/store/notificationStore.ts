import { create } from 'zustand'

export interface Notification {
  id: string
  type: 'info' | 'success' | 'warning' | 'error'
  title: string
  message: string
  /** 产生时间, 已格式化为 HH:mm */
  time: string
  read: boolean
}

type NewNotification = Pick<Notification, 'type' | 'title' | 'message'>

interface NotificationStore {
  notifications: Notification[]
  addNotification: (n: NewNotification) => void
  markAsRead: (id: string) => void
  markAllAsRead: () => void
  clearNotifications: () => void
}

/** 只保留最近这么多条, 后台事件多时不无限增长 */
const KEEP = 50
let seq = 0

const clock = () => new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })

/** 通知中心 (侧栏铃铛): 后台事件产生的消息, 只在本次运行内保留。 */
export const useNotificationStore = create<NotificationStore>((set) => ({
  notifications: [],
  addNotification: (n) =>
    set((s) => ({ notifications: [{ ...n, id: `n${++seq}`, time: clock(), read: false }, ...s.notifications].slice(0, KEEP) })),
  markAsRead: (id) => set((s) => ({ notifications: s.notifications.map((n) => (n.id === id ? { ...n, read: true } : n)) })),
  markAllAsRead: () => set((s) => ({ notifications: s.notifications.map((n) => ({ ...n, read: true })) })),
  clearNotifications: () => set({ notifications: [] }),
}))
