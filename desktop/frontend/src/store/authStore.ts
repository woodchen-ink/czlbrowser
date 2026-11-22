import { create } from 'zustand'

export interface AuthUserInfo {
  id: number
  username: string
  nickname: string
  email: string
  avatar: string
}

export interface AuthState {
  loggedIn: boolean
  user?: AuthUserInfo
}

interface AuthStore {
  state: AuthState
  loading: boolean
  loginPending: boolean
  loginError: string

  /** 从后端获取当前认证状态 */
  fetchState: () => Promise<void>

  /** 发起 OAuth2 登录 */
  login: () => Promise<void>

  /** 登出 */
  logout: () => Promise<void>

  /** 登录成功时更新状态 */
  setAuthState: (state: AuthState) => void
}

const goApp = () => (window as any).go?.main?.App

export const useAuthStore = create<AuthStore>((set) => ({
  state: { loggedIn: false },
  loading: true,
  loginPending: false,
  loginError: '',

  fetchState: async () => {
    try {
      const app = goApp()
      if (!app?.AuthGetState) {
        set({ loading: false })
        return
      }
      const state: AuthState = await app.AuthGetState()
      set({ state, loading: false })
    } catch {
      set({ loading: false })
    }
  },

  login: async () => {
    try {
      set({ loginPending: true, loginError: '' })
      const app = goApp()
      if (!app?.AuthLogin) return
      await app.AuthLogin()
    } catch (err) {
      set({ loginPending: false })
      throw err
    }
  },

  logout: async () => {
    try {
      const app = goApp()
      if (app?.AuthLogout) {
        await app.AuthLogout()
      }
      set({ state: { loggedIn: false }, loginPending: false, loginError: '' })
    } catch {
      // 忽略登出错误
    }
  },

  setAuthState: (state: AuthState) => {
    set({ state, loading: false, loginPending: false, loginError: '' })
  },
}))
