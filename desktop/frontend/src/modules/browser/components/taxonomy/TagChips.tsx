/** 行内标签: 最多显示 max 个, 其余折成 +N (悬停看全部) */
export function TagChips({ tags, max = 3 }: { tags?: string[] | null; max?: number }) {
  const list = tags || []
  if (list.length === 0) return null
  const shown = list.slice(0, max)
  const rest = list.length - shown.length
  return (
    <span className="inline-flex flex-wrap items-center gap-1" title={list.join('、')}>
      {shown.map((t) => (
        <span
          key={t}
          className="inline-flex items-center h-[18px] px-1.5 rounded bg-[var(--color-bg-muted)] text-[11px] text-[var(--color-text-secondary)] max-w-[8rem] truncate"
        >
          {t}
        </span>
      ))}
      {rest > 0 && <span className="text-[11px] text-[var(--color-text-muted)] tabular-nums">+{rest}</span>}
    </span>
  )
}
