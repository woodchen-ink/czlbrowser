// 分组 / 标签的通用计算: 各列表页 (实例 / 代理 / 内核 / 书签) 共用

/** 统计标签出现次数, 按次数降序、名称升序 */
export function countTags<T>(items: T[], getTags: (item: T) => string[] | undefined | null): { name: string; count: number }[] {
  const map = new Map<string, { name: string; count: number }>()
  for (const item of items) {
    for (const raw of getTags(item) || []) {
      const name = raw.trim()
      if (!name) continue
      const key = name.toLowerCase()
      const hit = map.get(key)
      if (hit) hit.count++
      else map.set(key, { name, count: 1 })
    }
  }
  return [...map.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name, 'zh-CN'))
}

/** 已选标签为空时全部通过; 否则含任一已选标签即通过 */
export function matchTags(tags: string[] | undefined | null, selected: string[]): boolean {
  if (selected.length === 0) return true
  const own = new Set((tags || []).map((t) => t.toLowerCase()))
  return selected.some((t) => own.has(t.toLowerCase()))
}

/** 分组筛选值: 全部 / 未分组 / 某个分组 */
export const GROUP_ALL = '__all__'
export const GROUP_NONE = '__none__'

export function matchGroup(group: string | undefined | null, selected: string): boolean {
  if (selected === GROUP_ALL) return true
  if (selected === GROUP_NONE) return !group
  return (group || '') === selected
}
