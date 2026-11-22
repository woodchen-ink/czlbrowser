import { ReactNode } from 'react'
import clsx from 'clsx'

export interface SegmentedOption<V extends string> {
  value: V
  label: ReactNode
  icon?: ReactNode
  /** 行尾计数 */
  count?: number
  title?: string
}

interface SegmentedProps<V extends string> {
  options: SegmentedOption<V>[]
  value: V
  onChange: (value: V) => void
  size?: 'sm' | 'md'
  className?: string
}

/** 分段切换 (shadcn Tabs 分段形态): 下沉底容器 + 白底激活段 */
export function Segmented<V extends string>({ options, value, onChange, size = 'md', className }: SegmentedProps<V>) {
  return (
    <div
      role="tablist"
      className={clsx(
        'inline-flex items-center gap-0.5 p-0.5 rounded-md bg-[var(--color-bg-muted)]',
        className
      )}
    >
      {options.map((o) => {
        const active = o.value === value
        return (
          <button
            key={o.value}
            role="tab"
            aria-selected={active}
            title={o.title}
            onClick={() => onChange(o.value)}
            className={clsx(
              'inline-flex items-center gap-1.5 rounded-[5px] whitespace-nowrap transition-colors',
              size === 'sm' ? 'h-6 px-2 text-xs' : 'h-7 px-2.5 text-[13px]',
              active
                ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] shadow-[var(--shadow-sm)] font-medium'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
            )}
          >
            {o.icon}
            {o.label}
            {o.count !== undefined && (
              <span className={clsx('tabular-nums', active ? 'text-[var(--color-text-muted)]' : 'opacity-70')}>{o.count}</span>
            )}
          </button>
        )
      })}
    </div>
  )
}
