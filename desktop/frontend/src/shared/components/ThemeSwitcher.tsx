import { Moon, Sun } from 'lucide-react'
import { useTheme, type ThemeType } from '../theme'
import { Segmented } from './Segmented'

/** 主题切换: 浅色 / 深色 */
export function ThemeSwitcher({ className }: { className?: string }) {
  const { theme, setTheme } = useTheme()
  return (
    <Segmented<ThemeType>
      className={className}
      value={theme}
      onChange={setTheme}
      options={[
        { value: 'light', label: '浅色', icon: <Sun className="w-3.5 h-3.5" /> },
        { value: 'dark', label: '深色', icon: <Moon className="w-3.5 h-3.5" /> },
      ]}
    />
  )
}
