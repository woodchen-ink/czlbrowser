import type { ReactNode } from 'react'
import clsx from 'clsx'
import { Inbox } from 'lucide-react'

export interface TableColumn<T> {
  key: string
  title: ReactNode
  width?: string | number
  align?: 'left' | 'center' | 'right'
  /** 自定义单元格; 省略时显示 record[key] */
  render?: (value: any, record: T, index: number) => ReactNode
}

interface TableProps<T> {
  columns: TableColumn<T>[]
  data: T[]
  /** 行的唯一键字段名, 或从记录取键的函数 */
  rowKey: keyof T | ((record: T) => string)
  emptyText?: string
  /** 超出时内部滚动, 表头固定 */
  maxHeight?: string
  className?: string
}

const alignClass = (a?: TableColumn<unknown>['align']) => (a === 'center' ? 'text-center' : a === 'right' ? 'text-right' : 'text-left')

/** 简单数据表 (实例详情页的标签页、Cookie、快照)。 */
export function Table<T extends object>({ columns, data, rowKey, emptyText = '暂无数据', maxHeight = 'calc(100vh - 320px)', className }: TableProps<T>) {
  const keyOf = (r: T, i: number) => (typeof rowKey === 'function' ? rowKey(r) : String(r[rowKey] ?? i))
  return (
    <div className={clsx('overflow-auto', className)} style={{ maxHeight }}>
      <table className="min-w-full">
        <thead className="sticky top-0 z-10">
          <tr>
            {columns.map((c) => (
              <th
                key={c.key}
                style={{ width: c.width }}
                className={clsx(
                  'h-9 whitespace-nowrap border-b border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-xs font-medium text-[var(--color-text-muted)]',
                  alignClass(c.align),
                )}
              >
                {c.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-[var(--color-border-muted)] bg-[var(--color-bg-surface)]">
          {data.length === 0 && (
            <tr>
              <td colSpan={columns.length} className="px-4 py-16">
                <div className="flex flex-col items-center gap-2 text-[var(--color-text-muted)]">
                  <span className="flex h-12 w-12 items-center justify-center rounded-full bg-[var(--color-bg-muted)]">
                    <Inbox className="h-6 w-6" strokeWidth={1.5} />
                  </span>
                  <span className="text-sm">{emptyText}</span>
                </div>
              </td>
            </tr>
          )}
          {data.map((r, i) => (
            <tr key={keyOf(r, i)} className="transition-colors duration-150 hover:bg-[var(--color-accent-muted)]">
              {columns.map((c) => {
                const value = (r as Record<string, any>)[c.key]
                return (
                  <td key={c.key} className={clsx('px-3 py-2 text-[13px] text-[var(--color-text-secondary)]', alignClass(c.align))}>
                    {c.render ? c.render(value, r, i) : value}
                  </td>
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
