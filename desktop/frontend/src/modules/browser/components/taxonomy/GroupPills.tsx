import clsx from 'clsx'
import { Settings2 } from 'lucide-react'
import { GROUP_ALL, GROUP_NONE } from './tagUtils'

export interface GroupPillItem {
  key: string
  label: string
  count: number
}

interface GroupPillsProps {
  groups: GroupPillItem[]
  value: string
  onChange: (key: string) => void
  total: number
  ungrouped: number
  /** 行尾的「管理分组」入口 */
  onManage?: () => void
}

/** 分组快速切换: 全部 / 各分组 / 未分组。选中反相, 分组多时横向滚动 */
export function GroupPills({ groups, value, onChange, total, ungrouped, onManage }: GroupPillsProps) {
  const items: GroupPillItem[] = [
    { key: GROUP_ALL, label: '全部', count: total },
    ...groups,
    ...(ungrouped > 0 && groups.length > 0 ? [{ key: GROUP_NONE, label: '未分组', count: ungrouped }] : []),
  ]
  return (
    <div className="flex items-center gap-1 min-w-0">
      <div className="flex items-center gap-1 min-w-0 overflow-x-auto no-scrollbar">
        {items.map((g) => {
          const active = g.key === value
          return (
            <button
              key={g.key}
              onClick={() => onChange(g.key)}
              className={clsx(
                'shrink-0 inline-flex items-center gap-1.5 h-7 px-2.5 rounded-md text-xs transition-colors',
                active
                  ? 'bg-[var(--color-button-primary)] text-[var(--color-button-primary-foreground)] font-medium'
                  : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]'
              )}
            >
              <span className="max-w-[10rem] truncate">{g.label}</span>
              <span className={clsx('tabular-nums', active ? 'opacity-70' : 'text-[var(--color-text-muted)]')}>{g.count}</span>
            </button>
          )
        })}
      </div>
      {onManage && (
        <button
          onClick={onManage}
          title="管理分组"
          aria-label="管理分组"
          className="shrink-0 w-7 h-7 flex items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]"
        >
          <Settings2 className="w-3.5 h-3.5" />
        </button>
      )}
    </div>
  )
}
