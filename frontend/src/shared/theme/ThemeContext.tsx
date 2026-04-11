import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { ThemeType, DEFAULT_THEME } from './types'

interface ThemeContextValue {
  theme: ThemeType
  setTheme: (theme: ThemeType) => void
}

const ThemeContext = createContext<ThemeContextValue | undefined>(undefined)

const THEME_STORAGE_KEY = 'app-theme'
const VALID_THEMES: ThemeType[] = ['light', 'dark']

interface ThemeProviderProps {
  children: ReactNode
  defaultTheme?: ThemeType
}

export function ThemeProvider({ children, defaultTheme = DEFAULT_THEME }: ThemeProviderProps) {
  const [theme, setThemeState] = useState<ThemeType>(() => {
    const saved = localStorage.getItem(THEME_STORAGE_KEY)
    if (saved && VALID_THEMES.includes(saved as ThemeType)) {
      return saved as ThemeType
    }
    return defaultTheme
  })

  const setTheme = (newTheme: ThemeType) => {
    setThemeState(newTheme)
    localStorage.setItem(THEME_STORAGE_KEY, newTheme)
  }

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
  }, [theme])

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const context = useContext(ThemeContext)
  if (!context) {
    throw new Error('useTheme must be used within a ThemeProvider')
  }
  return context
}
