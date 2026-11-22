import { useEffect, useState } from 'react'
import { GROUP_ALL } from '../components/taxonomy'

export type StatusFilter = 'all' | 'running' | 'stopped'

/** 实例列表的筛选条件, 记在本机 localStorage, 下次打开还原。 */
export interface ListPrefs {
  group: string
  tags: string[]
  status: StatusFilter
}

const KEY = 'browser:list-prefs'
export const DEFAULT_PREFS: ListPrefs = { group: GROUP_ALL, tags: [], status: 'all' }

function read(): ListPrefs {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    if (!v || typeof v !== 'object') return DEFAULT_PREFS
    return {
      group: typeof v.group === 'string' ? v.group : GROUP_ALL,
      tags: Array.isArray(v.tags) ? v.tags.filter((t: unknown): t is string => typeof t === 'string') : [],
      status: v.status === 'running' || v.status === 'stopped' ? v.status : 'all',
    }
  } catch {
    return DEFAULT_PREFS
  }
}

export function useListPrefs() {
  const [prefs, setPrefs] = useState<ListPrefs>(read)
  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify(prefs))
    } catch {
      // 存不了 (隐私模式等) 就只在本次有效
    }
  }, [prefs])
  const patch = (p: Partial<ListPrefs>) => setPrefs((cur) => ({ ...cur, ...p }))
  return [prefs, patch] as const
}
