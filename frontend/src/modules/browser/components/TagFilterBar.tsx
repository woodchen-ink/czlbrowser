interface TagFilterBarProps {
  tags: string[]
  /** 当前选中的标签，'' 表示全部 */
  selected: string
  onChange: (next: string) => void
}

// 标签筛选（单选）：一次只筛选一个标签，再次点击已选标签或点「全部」即回到全部
export function TagFilterBar({ tags, selected, onChange }: TagFilterBarProps) {
  if (tags.length === 0) return null

  const isAllSelected = selected === ''

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-[var(--color-text-muted)] shrink-0">标签：</span>
      <button
        onClick={() => onChange('')}
        className={`px-2.5 py-0.5 rounded-full text-xs font-medium transition-colors cursor-pointer ${
          isAllSelected
            ? 'bg-[var(--color-text-primary)] text-[var(--color-text-inverse)]'
            : 'bg-[var(--color-bg-muted)] text-[var(--color-text-muted)] hover:bg-[var(--color-bg-subtle)]'
        }`}
      >
        全部
      </button>
      {tags.map(tag => (
        <button
          key={tag}
          onClick={() => onChange(selected === tag ? '' : tag)}
          className={`px-2.5 py-0.5 rounded-full text-xs font-medium transition-colors cursor-pointer ${
            selected === tag
              ? 'bg-[var(--color-text-primary)] text-[var(--color-text-inverse)]'
              : 'bg-[var(--color-bg-muted)] text-[var(--color-text-muted)] hover:bg-[var(--color-bg-subtle)]'
          }`}
        >
          {tag}
        </button>
      ))}
    </div>
  )
}
