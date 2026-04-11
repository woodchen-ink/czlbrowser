// 主题类型定义
export type ThemeType = 'dark' | 'light'

export interface ThemeConfig {
  id: ThemeType
  name: string
  description: string
}

export const themeConfigs: ThemeConfig[] = [
  { id: 'light', name: '浅色主题', description: '适合日间使用。' },
  { id: 'dark', name: '深色主题', description: '适合低光环境。' },
]

export const DEFAULT_THEME: ThemeType = 'light'
