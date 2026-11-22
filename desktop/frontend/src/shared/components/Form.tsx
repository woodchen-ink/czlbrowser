import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'
import clsx from 'clsx'

interface FormItemProps {
  label?: string
  required?: boolean
  /** 标签后的灰色补充说明 */
  hint?: string
  /** 字段下方的红色错误提示 */
  error?: string
  className?: string
  children: ReactNode
}

/** 表单行: 标签 + 控件 + 错误提示。 */
export function FormItem({ label, required, hint, error, className, children }: FormItemProps) {
  return (
    <div className={clsx('space-y-1', className)}>
      {label && (
        <label className="block text-xs font-medium text-[var(--color-text-secondary)]">
          {label}
          {required && <span className="ml-0.5 text-[var(--color-error)]">*</span>}
          {hint && <span className="ml-1 text-xs font-normal text-[var(--color-text-muted)]">({hint})</span>}
        </label>
      )}
      {children}
      {error && <p className="text-xs text-[var(--color-error)]">{error}</p>}
    </div>
  )
}

// 输入类控件共用的外观: 紧凑 h-8, 聚焦时加深边框 (不用品牌色)
const field = (error: boolean | undefined, className: string | undefined, extra: string) =>
  clsx(
    'block rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[13px] text-[var(--color-text-primary)] transition-colors duration-150',
    'placeholder:text-[var(--color-text-muted)] focus:border-[var(--color-border-strong)] focus:outline-none focus:ring-1 focus:ring-[var(--color-border-strong)]',
    'disabled:cursor-not-allowed disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)]',
    error && 'border-[var(--color-error)] focus:border-[var(--color-error)] focus:ring-[var(--color-error)]',
    extra,
    // 没指定宽度时撑满
    !/\bw-/.test(className ?? '') && 'w-full',
    className,
  )

export function Input({ error, className, ...rest }: InputHTMLAttributes<HTMLInputElement> & { error?: boolean }) {
  return <input {...rest} className={field(error, className, 'h-8 px-2.5')} />
}

export function Select({
  error,
  options,
  className,
  ...rest
}: SelectHTMLAttributes<HTMLSelectElement> & { error?: boolean; options: { value: string; label: string }[] }) {
  return (
    <select {...rest} className={field(error, className, 'h-8 cursor-pointer pl-2.5 pr-7')}>
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}

export function Textarea({ error, className, ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement> & { error?: boolean }) {
  return <textarea {...rest} className={field(error, className, 'resize-none px-2.5 py-1.5')} />
}

interface SwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
}

/** 开关; 打开时为近黑色 (与主按钮一致)。 */
export function Switch({ checked, onChange, disabled }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={clsx(
        'relative inline-flex h-5 w-9 items-center rounded-full transition-colors duration-150',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)] focus-visible:ring-offset-2',
        checked ? 'bg-[var(--color-button-primary)]' : 'bg-[var(--color-border-strong)]',
        disabled && 'cursor-not-allowed opacity-50',
      )}
    >
      <span
        className={clsx(
          'inline-block h-4 w-4 rounded-full bg-[var(--color-bg-elevated)] shadow-sm transition-transform duration-150',
          checked ? 'translate-x-4' : 'translate-x-0.5',
        )}
      />
    </button>
  )
}
