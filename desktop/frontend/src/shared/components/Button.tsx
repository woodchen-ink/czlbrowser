import type { ButtonHTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost'
type Size = 'sm' | 'md' | 'lg'

const look: Record<Variant, string> = {
  // 主按钮近黑底白字 (暗色反相), 品牌蓝只用在焦点环
  primary:
    'bg-[var(--color-button-primary)] text-[var(--color-button-primary-foreground)] hover:bg-[var(--color-button-primary-hover)] active:bg-[var(--color-button-primary-active)] focus-visible:ring-[var(--color-ring)] disabled:bg-[var(--color-button-primary-disabled)] disabled:text-[var(--color-button-primary-disabled-foreground)] disabled:opacity-100',
  secondary:
    'border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)] focus-visible:ring-[var(--color-border-strong)]',
  danger: 'bg-[var(--color-error)] text-[var(--color-text-inverse)] hover:opacity-90 focus-visible:ring-[var(--color-error)]',
  ghost: 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
}

const scale: Record<Size, string> = {
  sm: 'h-7 gap-1.5 px-2.5 text-xs',
  md: 'h-8 gap-1.5 px-3 text-[13px]',
  lg: 'h-9 gap-2 px-4 text-sm',
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  /** 显示转圈并禁用 */
  loading?: boolean
  children: ReactNode
}

export function Button({ variant = 'primary', size = 'md', loading = false, disabled, className, children, ...rest }: ButtonProps) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      className={clsx(
        'inline-flex items-center justify-center whitespace-nowrap rounded-md font-medium transition-colors duration-150',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--color-bg-base)]',
        'disabled:cursor-not-allowed disabled:opacity-60',
        look[variant],
        scale[size],
        className,
      )}
    >
      {loading && <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden />}
      {children}
    </button>
  )
}
