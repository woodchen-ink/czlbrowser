import type { ReactNode } from 'react'
import clsx from 'clsx'

type Tone = 'default' | 'success' | 'error' | 'warning' | 'info'

// 状态色用 15% 透明度的浅底配同色字
const tone: Record<Tone, { chip: string; dot: string }> = {
  default: { chip: 'bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]', dot: 'bg-[var(--color-text-muted)]' },
  success: { chip: 'bg-[var(--color-success)]/15 text-[var(--color-success)]', dot: 'bg-[var(--color-success)]' },
  error: { chip: 'bg-[var(--color-error)]/15 text-[var(--color-error)]', dot: 'bg-[var(--color-error)]' },
  warning: { chip: 'bg-[var(--color-warning)]/15 text-[var(--color-warning)]', dot: 'bg-[var(--color-warning)]' },
  info: { chip: 'bg-[var(--color-accent)]/15 text-[var(--color-accent)]', dot: 'bg-[var(--color-accent)]' },
}

const pad = { sm: 'px-1.5 py-0.5 text-xs', md: 'px-2 py-1 text-xs', lg: 'px-2.5 py-1 text-sm' }

interface BadgeProps {
  variant?: Tone
  size?: 'sm' | 'md' | 'lg'
  /** 前面加一个同色圆点 */
  dot?: boolean
  dotClassName?: string
  className?: string
  children: ReactNode
}

export function Badge({ variant = 'default', size = 'md', dot = false, dotClassName = 'w-1.5 h-1.5', className, children }: BadgeProps) {
  return (
    <span className={clsx('inline-flex items-center gap-1.5 rounded-full font-medium', tone[variant].chip, pad[size], className)}>
      {dot && <span className={clsx('rounded-full', dotClassName, tone[variant].dot)} />}
      {children}
    </span>
  )
}
