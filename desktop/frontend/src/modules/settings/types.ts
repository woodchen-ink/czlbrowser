// Settings 模块类型定义

export interface AppSettings {
  theme: 'light' | 'dark' | 'system'
}

export const defaultSettings: AppSettings = {
  theme: 'light',
}

