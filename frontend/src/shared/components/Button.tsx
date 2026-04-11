import { ButtonHTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  children: ReactNode
}

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  disabled,
  className,
  children,
  ...props
}: ButtonProps) {
  const baseStyles = 'inline-flex items-center justify-center rounded-lg font-medium transition-all duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--color-bg-base)] disabled:cursor-not-allowed disabled:opacity-60'

  const variants = {
    primary: 'bg-[var(--color-button-primary)] text-[var(--color-button-primary-foreground)] hover:bg-[var(--color-button-primary-hover)] active:bg-[var(--color-button-primary-active)] focus-visible:ring-[var(--color-ring)] disabled:bg-[var(--color-button-primary-disabled)] disabled:text-[var(--color-button-primary-disabled-foreground)] disabled:opacity-100',
    secondary: 'bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)] border border-[var(--color-border-default)] hover:bg-[var(--color-bg-muted)] hover:border-[var(--color-border-strong)] focus-visible:ring-[var(--color-border-strong)]',
    danger: 'bg-[var(--color-error)] text-[var(--color-text-inverse)] hover:opacity-90 focus-visible:ring-[var(--color-error)]',
    ghost: 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
  }

  const sizes = {
    sm: 'h-8 px-3 text-xs gap-1.5',
    md: 'h-9 px-4 text-sm gap-2',
    lg: 'h-10 px-5 text-sm gap-2',
  }

  return (
    <button
      className={clsx(
        baseStyles,
        variants[variant],
        sizes[size],
        className
      )}
      disabled={disabled || loading}
      {...props}
    >
      {loading && (
        <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      )}
      {children}
    </button>
  )
}
