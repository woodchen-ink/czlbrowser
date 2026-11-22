import { useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import { Check, ChevronDown, Tag } from 'lucide-react'

interface TagFilterProps {
  tags: { name: string; count: number }[]
  value: string[]
  onChange: (tags: string[]) => void
}

/** 标签筛选下拉: 多选, 含任一已选标签即显示 */
export function TagFilter({ tags, value, onChange }: TagFilterProps) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [])

  const selected = new Set(value.map((v) => v.toLowerCase()))
  const toggle = (name: string) => {
    const k = name.toLowerCase()
    onChange(selected.has(k) ? value.filter((v) => v.toLowerCase() !== k) : [...value, name])
  }
  const shown = tags.filter((t) => !q || t.name.toLowerCase().includes(q.toLowerCase()))

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((v) => !v)}
        disabled={tags.length === 0}
        title={tags.length === 0 ? '还没有标签' : '按标签筛选'}
        className={clsx(
          'inline-flex items-center gap-1.5 h-8 px-2.5 rounded-md border text-[13px] transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
          value.length > 0
            ? 'border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)]'
            : 'border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-strong)]'
        )}
      >
        <Tag className="w-3.5 h-3.5" />
        标签
        {value.length > 0 && (
          <span className="min-w-[16px] h-4 px-1 rounded bg-[var(--color-button-primary)] text-[var(--color-button-primary-foreground)] text-[10px] leading-4 tabular-nums">
            {value.length}
          </span>
        )}
        <ChevronDown className="w-3.5 h-3.5 text-[var(--color-text-muted)]" />
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-1 z-40 w-56 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] shadow-[var(--shadow-lg)] animate-fade-in">
          {tags.length > 8 && (
            <div className="p-1.5 border-b border-[var(--color-border-muted)]">
              <input
                autoFocus
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="搜索标签"
                className="w-full h-7 px-2 text-xs rounded bg-[var(--color-bg-subtle)] text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none"
              />
            </div>
          )}
          <div className="max-h-64 overflow-y-auto p-1">
            {shown.map((t) => {
              const on = selected.has(t.name.toLowerCase())
              return (
                <button
                  key={t.name}
                  onClick={() => toggle(t.name)}
                  className="w-full flex items-center gap-2 h-7 px-2 rounded text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)]"
                >
                  <span
                    className={clsx(
                      'w-3.5 h-3.5 rounded-[3px] border flex items-center justify-center shrink-0',
                      on
                        ? 'bg-[var(--color-button-primary)] border-[var(--color-button-primary)] text-[var(--color-button-primary-foreground)]'
                        : 'border-[var(--color-border-strong)]'
                    )}
                  >
                    {on && <Check className="w-2.5 h-2.5" />}
                  </span>
                  <span className="flex-1 min-w-0 text-left truncate text-[var(--color-text-primary)]">{t.name}</span>
                  <span className="tabular-nums text-[var(--color-text-muted)]">{t.count}</span>
                </button>
              )
            })}
            {shown.length === 0 && <p className="py-3 text-center text-xs text-[var(--color-text-muted)]">没有匹配的标签</p>}
          </div>
          {value.length > 0 && (
            <div className="p-1 border-t border-[var(--color-border-muted)]">
              <button
                onClick={() => onChange([])}
                className="w-full h-7 rounded text-xs text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]"
              >
                清除筛选
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
