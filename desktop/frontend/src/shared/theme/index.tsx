import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

export type ThemeType = 'light' | 'dark'

const STORAGE_KEY = 'app-theme'

interface ThemeState {
  theme: ThemeType
  setTheme: (t: ThemeType) => void
}

const Ctx = createContext<ThemeState | null>(null)

/** 读上次选的主题; 没选过默认浅色 */
function initialTheme(): ThemeType {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    // 读不到存储就用默认
  }
  return 'light'
}

/** 主题: 写到 <html data-theme>, themes/*.css 按它切换变量。 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeType>(initialTheme)

  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const setTheme = (t: ThemeType) => {
    setThemeState(t)
    try {
      localStorage.setItem(STORAGE_KEY, t)
    } catch {
      // 只在本次生效
    }
  }

  return <Ctx.Provider value={{ theme, setTheme }}>{children}</Ctx.Provider>
}

export function useTheme(): ThemeState {
  const v = useContext(Ctx)
  if (!v) throw new Error('useTheme 需要在 ThemeProvider 内使用')
  return v
}
