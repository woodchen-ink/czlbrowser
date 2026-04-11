import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react'

interface PaginationProps {
  current: number
  total: number
  pageSize: number
  onChange: (page: number) => void
  onPageSizeChange?: (size: number) => void
  pageSizeOptions?: number[]
  showTotal?: boolean
  showPageSize?: boolean
}

export function Pagination({
  current,
  total,
  pageSize,
  onChange,
  onPageSizeChange,
  pageSizeOptions = [10, 20, 50],
  showTotal = true,
  showPageSize = true,
}: PaginationProps) {
  const totalPages = Math.ceil(total / pageSize)
  
  // 生成页码数组
  const getPageNumbers = () => {
    const pages: (number | string)[] = []
    const maxVisible = 5
    
    if (totalPages <= maxVisible + 2) {
      for (let i = 1; i <= totalPages; i++) pages.push(i)
    } else {
      pages.push(1)
      
      if (current > 3) pages.push('...')
      
      const start = Math.max(2, current - 1)
      const end = Math.min(totalPages - 1, current + 1)
      
      for (let i = start; i <= end; i++) pages.push(i)
      
      if (current < totalPages - 2) pages.push('...')
      
      pages.push(totalPages)
    }
    
    return pages
  }

  if (total === 0) return null

  return (
    <div className="flex items-center justify-between gap-4 py-3 px-4 border-t border-[var(--color-border)]">
      {/* 左侧：总数和每页条数 */}
      <div className="flex items-center gap-4 text-sm text-[var(--color-text-muted)]">
        {showTotal && (
          <span>共 <span className="font-medium text-[var(--color-text-secondary)]">{total}</span> 条</span>
        )}
        {showPageSize && onPageSizeChange && (
          <div className="flex items-center gap-2">
            <span>每页</span>
            <select
              value={pageSize}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
              className="px-2 py-1 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-surface)] text-[var(--color-text-primary)] text-sm focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]/50"
            >
              {pageSizeOptions.map((size) => (
                <option key={size} value={size}>{size}</option>
              ))}
            </select>
            <span>条</span>
          </div>
        )}
      </div>

      {/* 右侧：分页按钮 */}
      <div className="flex items-center gap-1">
        {/* 首页 */}
        <button
          onClick={() => onChange(1)}
          disabled={current === 1}
          className="p-1.5 rounded-md text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          title="首页"
        >
          <ChevronsLeft className="w-4 h-4" />
        </button>
        
        {/* 上一页 */}
        <button
          onClick={() => onChange(current - 1)}
          disabled={current === 1}
          className="p-1.5 rounded-md text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          title="上一页"
        >
          <ChevronLeft className="w-4 h-4" />
        </button>

        {/* 页码 */}
        {getPageNumbers().map((page, index) => (
          typeof page === 'number' ? (
            <button
              key={index}
              onClick={() => onChange(page)}
              className={`min-w-[32px] h-8 px-2 rounded-md text-sm font-medium transition-colors ${
                page === current
                  ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]'
                  : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
              }`}
            >
              {page}
            </button>
          ) : (
            <span key={index} className="px-1 text-[var(--color-text-muted)]">...</span>
          )
        ))}

        {/* 下一页 */}
        <button
          onClick={() => onChange(current + 1)}
          disabled={current === totalPages}
          className="p-1.5 rounded-md text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          title="下一页"
        >
          <ChevronRight className="w-4 h-4" />
        </button>

        {/* 末页 */}
        <button
          onClick={() => onChange(totalPages)}
          disabled={current === totalPages}
          className="p-1.5 rounded-md text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          title="末页"
        >
          <ChevronsRight className="w-4 h-4" />
        </button>
      </div>
    </div>
  )
}
