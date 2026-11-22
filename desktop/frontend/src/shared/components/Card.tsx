import type { ReactNode } from 'react'
import clsx from 'clsx'

interface CardProps {
  title?: string
  subtitle?: string
  /** 标题栏右侧的操作 */
  actions?: ReactNode
  padding?: 'none' | 'sm' | 'md' | 'lg'
  /** 悬停时加深边框 */
  hover?: boolean
  className?: string
  children: ReactNode
}

const inner = { none: '', sm: 'p-3', md: 'p-4', lg: 'p-5' }

/** 白底面板, 有标题或操作时带一条标题栏。 */
export function Card({ title, subtitle, actions, padding = 'md', hover = false, className, children }: CardProps) {
  const header = title || actions
  return (
    <section
      className={clsx(
        'overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]',
        hover && 'transition-colors hover:border-[var(--color-border-strong)]',
        className,
      )}
    >
      {header && (
        <header className="flex min-h-11 items-center justify-between gap-3 border-b border-[var(--color-border-muted)] px-4 py-2">
          <div>
            {title && <h3 className="text-[13px] font-semibold text-[var(--color-text-primary)]">{title}</h3>}
            {subtitle && <p className="mt-0.5 text-xs text-[var(--color-text-muted)]">{subtitle}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </header>
      )}
      <div className={inner[padding]}>{children}</div>
    </section>
  )
}
