import clsx from 'clsx'

type Status = 'normal' | 'success' | 'error' | 'warning'

// 进度条是少数允许使用品牌蓝的地方
const fill: Record<Status, string> = {
  normal: 'var(--color-brand)',
  success: 'var(--color-success)',
  error: 'var(--color-error)',
  warning: 'var(--color-warning)',
}

const clamp = (n: number) => Math.round(Math.min(100, Math.max(0, n || 0)))

interface ProgressProps {
  percent: number
  status?: Status
  /** 右侧显示百分比 */
  showInfo?: boolean
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

export function Progress({ percent, status = 'normal', showInfo = true, size = 'md', className }: ProgressProps) {
  const p = clamp(percent)
  return (
    <div className={clsx('flex items-center gap-3', className)}>
      <div
        role="progressbar"
        aria-valuenow={p}
        className={clsx('flex-1 overflow-hidden rounded-full bg-[var(--color-bg-muted)]', { sm: 'h-1', md: 'h-2', lg: 'h-3' }[size])}
      >
        <div className="h-full rounded-full transition-all duration-300" style={{ width: `${p}%`, background: fill[status] }} />
      </div>
      {showInfo && <span className="min-w-[3ch] text-right text-sm text-[var(--color-text-muted)]">{p}%</span>}
    </div>
  )
}

interface CircleProgressProps {
  percent: number
  size?: number
  strokeWidth?: number
  status?: Status
  showInfo?: boolean
}

/** 环形进度。 */
export function CircleProgress({ percent, size = 120, strokeWidth = 8, status = 'normal', showInfo = true }: CircleProgressProps) {
  const p = clamp(percent)
  const r = (size - strokeWidth) / 2
  const len = 2 * Math.PI * r
  const c = size / 2
  return (
    <div className="relative inline-flex items-center justify-center">
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={c} cy={c} r={r} fill="none" stroke="var(--color-bg-muted)" strokeWidth={strokeWidth} />
        <circle
          cx={c}
          cy={c}
          r={r}
          fill="none"
          stroke={fill[status]}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={len}
          strokeDashoffset={len * (1 - p / 100)}
          className="transition-all duration-300"
        />
      </svg>
      {showInfo && <span className="absolute text-lg font-semibold text-[var(--color-text-primary)]">{p}%</span>}
    </div>
  )
}
