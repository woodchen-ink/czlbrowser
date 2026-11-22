import { useRef, useState, type KeyboardEvent } from 'react'
import { X } from 'lucide-react'

interface TagInputProps {
  value: string[]
  onChange: (tags: string[]) => void
  /** 已有标签, 输入时按包含匹配给出候选 */
  suggestions?: string[]
  placeholder?: string
}

const hasFold = (list: string[], t: string) => list.some((x) => x.toLowerCase() === t.toLowerCase())

/** 标签输入: 回车或逗号确认, 退格删最后一个, 大小写不同视为同一标签 (与后端 NormalizeTags 一致)。 */
export function TagInput({ value, onChange, suggestions = [], placeholder = '输入后回车' }: TagInputProps) {
  const [text, setText] = useState('')
  const [focused, setFocused] = useState(false)
  const ref = useRef<HTMLInputElement>(null)

  const q = text.trim().toLowerCase()
  const candidates = focused ? suggestions.filter((s) => !hasFold(value, s) && s.toLowerCase().includes(q)).slice(0, 8) : []

  const add = (raw: string) => {
    const t = raw.trim()
    if (t && !hasFold(value, t)) onChange([...value, t])
    setText('')
  }

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if ((e.key === 'Enter' || e.key === ',' || e.key === '，') && text.trim()) {
      e.preventDefault()
      add(text)
    } else if (e.key === 'Backspace' && !text && value.length) {
      onChange(value.slice(0, -1))
    } else if (e.key === 'Escape') {
      ref.current?.blur()
    }
  }

  return (
    <div className="relative">
      <div
        onClick={() => ref.current?.focus()}
        className="flex min-h-8 cursor-text flex-wrap items-center gap-1 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-input)] px-2 py-1 focus-within:border-[var(--color-border-strong)]"
      >
        {value.map((t) => (
          <span key={t} className="inline-flex h-5 items-center gap-0.5 rounded bg-[var(--color-bg-muted)] pl-1.5 pr-0.5 text-xs text-[var(--color-text-secondary)]">
            {t}
            <button
              type="button"
              aria-label={`移除 ${t}`}
              onClick={(e) => {
                e.stopPropagation()
                onChange(value.filter((x) => x !== t))
              }}
              className="rounded p-0.5 hover:text-[var(--color-error)]"
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}
        <input
          ref={ref}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={onKey}
          onFocus={() => setFocused(true)}
          onBlur={() => {
            setFocused(false)
            // 没按回车就离开时也收下输入
            if (text.trim()) add(text)
          }}
          placeholder={value.length ? '' : placeholder}
          className="h-5 min-w-20 flex-1 bg-transparent text-[13px] text-[var(--color-text-primary)] outline-none placeholder:text-[var(--color-text-muted)]"
        />
      </div>
      {candidates.length > 0 && (
        <ul className="absolute top-full z-30 mt-1 w-full min-w-40 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-1 shadow-[var(--shadow-lg)]">
          {candidates.map((s) => (
            <li key={s}>
              <button
                type="button"
                // mousedown 先于 input 失焦, 用它选中才不会被 onBlur 抢先
                onMouseDown={(e) => {
                  e.preventDefault()
                  add(s)
                }}
                className="h-7 w-full rounded px-2 text-left text-[13px] text-[var(--color-text-primary)] hover:bg-[var(--color-accent-muted)]"
              >
                {s}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
