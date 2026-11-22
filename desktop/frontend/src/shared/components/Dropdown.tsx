import { useEffect, useRef, useState, type ReactNode } from 'react'
import { ChevronDown } from 'lucide-react'
import clsx from 'clsx'

export interface DropdownItem {
  key: string
  label: ReactNode
  icon?: ReactNode
  disabled?: boolean
  /** 红色文字 (删除等) */
  danger?: boolean
  /** 分隔线, 其余字段忽略 */
  divider?: boolean
}

interface DropdownProps {
  items: DropdownItem[]
  onSelect?: (key: string) => void
  /** 触发元素; 省略时显示一个「操作」按钮 */
  trigger?: ReactNode
  children?: ReactNode
  placement?: 'bottom-left' | 'bottom-right'
}

/** 点击展开的菜单, 点外面或选中后收起。 */
export function Dropdown({ items, onSelect, trigger, children, placement = 'bottom-left' }: DropdownProps) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const away = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', away)
    return () => document.removeEventListener('mousedown', away)
  }, [open])

  const pick = (item: DropdownItem) => {
    if (item.disabled) return
    onSelect?.(item.key)
    setOpen(false)
  }

  return (
    <div ref={root} className="relative inline-block">
      <div className="cursor-pointer" onClick={() => setOpen((v) => !v)}>
        {trigger ?? children ?? (
          <button className="flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2 transition-colors hover:bg-[var(--color-bg-muted)]">
            <span className="text-sm">操作</span>
            <ChevronDown className="h-4 w-4" />
          </button>
        )}
      </div>
      {open && (
        <div
          role="menu"
          className={clsx(
            'absolute top-full z-50 mt-1 min-w-[140px] animate-fade-in rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-1 shadow-[var(--shadow-lg)]',
            placement === 'bottom-right' ? 'right-0' : 'left-0',
          )}
        >
          {items.map((item, i) =>
            item.divider ? (
              <div key={`divider-${i}`} className="-mx-1 my-1 h-px bg-[var(--color-border-muted)]" />
            ) : (
              <button
                key={item.key}
                role="menuitem"
                disabled={item.disabled}
                onClick={() => pick(item)}
                className={clsx(
                  'flex h-8 w-full items-center gap-2 whitespace-nowrap rounded-md px-2.5 text-left text-[13px] transition-colors',
                  item.disabled && 'cursor-not-allowed opacity-40',
                  !item.disabled && item.danger && 'text-[var(--color-error)] hover:bg-[var(--color-error-soft)]',
                  !item.disabled && !item.danger && 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
                )}
              >
                {item.icon && <span className="flex h-4 w-4 items-center justify-center">{item.icon}</span>}
                {item.label}
              </button>
            ),
          )}
        </div>
      )}
    </div>
  )
}
