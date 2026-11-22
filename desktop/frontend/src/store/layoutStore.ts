import { create } from 'zustand'

// 侧栏折叠状态按本机记住 (localStorage 不可用时退回展开)
const KEY = 'czl-browser:sidebar-collapsed'

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false
  }
}

function writeCollapsed(v: boolean) {
  try {
    localStorage.setItem(KEY, v ? '1' : '0')
  } catch {
    // 忽略
  }
}

interface LayoutState {
  sidebarCollapsed: boolean
  toggleSidebar: () => void
  setSidebarCollapsed: (collapsed: boolean) => void
}

export const useLayoutStore = create<LayoutState>((set) => ({
  sidebarCollapsed: readCollapsed(),
  toggleSidebar: () =>
    set((state) => {
      writeCollapsed(!state.sidebarCollapsed)
      return { sidebarCollapsed: !state.sidebarCollapsed }
    }),
  setSidebarCollapsed: (collapsed) => {
    writeCollapsed(collapsed)
    set({ sidebarCollapsed: collapsed })
  },
}))
