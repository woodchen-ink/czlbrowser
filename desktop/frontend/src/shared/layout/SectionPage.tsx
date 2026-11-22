import { ReactNode } from 'react'
import clsx from 'clsx'
import type { LucideIcon } from 'lucide-react'

// 多类别页面 (设置 / 云端同步): 左侧窄分类导航 + 右侧只显示当前类别。
// 当前类别由调用方放进 URL (?tab=), 刷新与深链回到同一类别。

/** 多类别页面右侧内容宽度: 跟随窗口铺满 */
export const SECTION_WIDTH = 'w-full'

export interface SectionTab<K extends string = string> {
  key: K
  label: string
  icon: LucideIcon
  /** 导航项行尾的小圆点颜色 (提示需要处理), 不传不显示 */
  dot?: string
}

interface SectionPageProps<K extends string> {
  title: string
  tabs: SectionTab<K>[]
  current: K
  onChange: (key: K) => void
  /** 标题行尾的操作 (如刷新) */
  titleAction?: ReactNode
  /** 右侧内容自己管滚动 (日志等整高内容) */
  fill?: boolean
  children: ReactNode
}

export function SectionPage<K extends string>({ title, tabs, current, onChange, titleAction, fill, children }: SectionPageProps<K>) {
  return (
    <div className="flex h-full min-h-0 gap-6">
      <nav className="w-40 shrink-0 flex flex-col gap-0.5">
        <div className="flex items-center justify-between h-8 px-2.5 mb-2">
          <h1 className="text-base font-semibold text-[var(--color-text-primary)]">{title}</h1>
          {titleAction}
        </div>
        {tabs.map((t) => {
          const Icon = t.icon
          const active = t.key === current
          return (
            <button
              key={t.key}
              onClick={() => onChange(t.key)}
              className={clsx(
                'w-full flex items-center gap-2 h-8 px-2.5 rounded-md text-[13px] transition-colors',
                active
                  ? 'bg-[var(--color-brand-soft)] text-[var(--color-brand-strong)] font-medium'
                  : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]'
              )}
            >
              <Icon className="w-4 h-4 shrink-0" />
              <span className="truncate">{t.label}</span>
              {t.dot && <span className={clsx('ml-auto w-1.5 h-1.5 rounded-full shrink-0', t.dot)} />}
            </button>
          )
        })}
      </nav>
      <div key={current} className={clsx('min-w-0 flex-1', fill ? 'flex flex-col min-h-0' : 'overflow-y-auto pb-6')}>
        {children}
      </div>
    </div>
  )
}

/** 右侧类别标题: 标题 + 一句说明 + 行尾操作 */
export function SectionHeader({ title, description, actions }: { title: string; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className={clsx('flex items-start justify-between gap-4 mb-4', SECTION_WIDTH)}>
      <div className="min-w-0">
        <h2 className="text-base font-semibold text-[var(--color-text-primary)] leading-8">{title}</h2>
        {description && <p className="text-xs text-[var(--color-text-muted)] -mt-0.5">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0 pt-0.5">{actions}</div>}
    </div>
  )
}

/** 类别内的一组设置: 分组标题 + 带分隔线的行, 不套卡片 */
export function SettingGroup({ title, description, actions, children, className }: {
  title?: string
  description?: ReactNode
  actions?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={clsx(SECTION_WIDTH, 'mb-6', className)}>
      {(title || actions) && (
        <div className="flex items-end justify-between gap-3 mb-1.5">
          <div>
            {title && <h3 className="text-xs font-medium text-[var(--color-text-muted)]">{title}</h3>}
            {description && <p className="text-xs text-[var(--color-text-muted)] mt-0.5">{description}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] divide-y divide-[var(--color-border-muted)]">
        {children}
      </div>
    </section>
  )
}

/** 一行设置: 左侧名称 (+ 可选说明), 右侧控件 */
export function SettingRow({ label, hint, children, stacked }: {
  label: ReactNode
  hint?: ReactNode
  children?: ReactNode
  /** 控件放到名称下方 (长输入框 / 多行内容) */
  stacked?: boolean
}) {
  return (
    <div className={clsx('px-4 py-2.5', stacked ? 'space-y-2' : 'flex items-center justify-between gap-4 min-h-[44px]')}>
      <div className="min-w-0">
        <div className="text-[13px] text-[var(--color-text-primary)]">{label}</div>
        {hint && <div className="text-xs text-[var(--color-text-muted)] mt-0.5">{hint}</div>}
      </div>
      {children && <div className={clsx(!stacked && 'shrink-0 flex items-center gap-2')}>{children}</div>}
    </div>
  )
}
