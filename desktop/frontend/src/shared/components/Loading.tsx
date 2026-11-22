import clsx from 'clsx'

const ring = { sm: 'h-4 w-4 border-2', md: 'h-6 w-6 border-2', lg: 'h-8 w-8 border-[3px]' }

interface LoadingProps {
  size?: 'sm' | 'md' | 'lg'
  text?: string
  /** 盖住整个窗口 */
  fullscreen?: boolean
  className?: string
}

/** 转圈加载提示。 */
export function Loading({ size = 'md', text, fullscreen = false, className }: LoadingProps) {
  const body = (
    <div className={clsx('flex flex-col items-center gap-3', className)} role="status">
      <span className={clsx('animate-spin rounded-full border-[var(--color-border-default)] border-t-[var(--color-accent)]', ring[size])} />
      {text && <span className="text-sm text-[var(--color-text-muted)]">{text}</span>}
    </div>
  )
  if (!fullscreen) return body
  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--color-bg-base)]/80 backdrop-blur-sm">{body}</div>
}

interface SkeletonProps {
  width?: string
  height?: string
  circle?: boolean
  className?: string
}

/** 内容占位条。 */
export function Skeleton({ width = '100%', height = '20px', circle = false, className }: SkeletonProps) {
  return <div className={clsx('animate-pulse bg-[var(--color-bg-muted)]', circle ? 'rounded-full' : 'rounded', className)} style={{ width, height }} />
}
