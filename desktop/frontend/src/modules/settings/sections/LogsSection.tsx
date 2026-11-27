import { useEffect, useMemo, useRef, useState } from 'react'
import { RefreshCw, Trash2 } from 'lucide-react'
import { Badge, Button, Input, Segmented } from '../../../shared/components'
import { SectionHeader } from '../../../shared/layout/SectionPage'
import { clearLogs, fetchLogs, type LogEntry } from '../api'

const POLL_MS = 3000
type Level = 'ALL' | 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'
const LEVELS: { value: Level; label: string }[] = [
  { value: 'ALL', label: '全部' },
  { value: 'ERROR', label: '错误' },
  { value: 'WARN', label: '警告' },
  { value: 'INFO', label: '信息' },
  { value: 'DEBUG', label: '调试' },
]

const badgeOf = { ERROR: 'error', WARN: 'warning', INFO: 'info', DEBUG: 'default' } as const
const textOf: Record<string, string> = { ERROR: 'text-[var(--color-error)]', WARN: 'text-[var(--color-warning)]', DEBUG: 'text-[var(--color-text-muted)]' }

const fieldsText = (f?: Record<string, unknown>) =>
  f
    ? Object.entries(f)
        .map(([k, v]) => `${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`)
        .join(' ')
    : ''

/** 运行日志: 后端内存里最近的日志, 定时刷新。 */
export function LogsSection() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [level, setLevel] = useState<Level>('ALL')
  const [q, setQ] = useState('')
  const [follow, setFollow] = useState(true)
  const [loading, setLoading] = useState(false)
  const boxRef = useRef<HTMLDivElement>(null)

  const load = async () => {
    setLoading(true)
    try {
      setLogs(await fetchLogs())
    } catch {
      // 拉取失败保留上一次的内容
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    const t = setInterval(load, POLL_MS)
    return () => clearInterval(t)
  }, [])

  const shown = useMemo(() => {
    const kw = q.trim().toLowerCase()
    return logs.filter(
      (e) => (level === 'ALL' || e.level === level) && (!kw || e.message.toLowerCase().includes(kw) || e.component.toLowerCase().includes(kw) || fieldsText(e.fields).toLowerCase().includes(kw)),
    )
  }, [logs, level, q])

  useEffect(() => {
    if (follow) boxRef.current?.scrollTo({ top: boxRef.current.scrollHeight })
  }, [shown, follow])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <SectionHeader
        title="运行日志"
        description="本次运行以来的日志, 排查问题时可搜索组件名或实例 ID"
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={load} loading={loading}>
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={async () => {
                await clearLogs()
                setLogs([])
              }}
            >
              <Trash2 className="h-3.5 w-3.5" />
              清空
            </Button>
          </>
        }
      />

      <div className="mb-3 flex items-center gap-3">
        <Segmented size="sm" value={level} onChange={setLevel} options={LEVELS} />
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索" className="h-7 w-52 text-xs" />
        <label className="ml-auto flex cursor-pointer select-none items-center gap-1.5 text-xs text-[var(--color-text-muted)]">
          <input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} className="h-3.5 w-3.5" />
          跟随最新
        </label>
        <span className="text-xs tabular-nums text-[var(--color-text-muted)]">
          {shown.length} / {logs.length}
        </span>
      </div>

      <div ref={boxRef} data-select-block className="min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] font-mono text-xs">
        {shown.length === 0 ? (
          <div className="py-16 text-center font-sans text-sm text-[var(--color-text-muted)]">没有日志</div>
        ) : (
          <table className="min-w-full">
            <thead className="sticky top-0 z-10 border-b border-[var(--color-border-default)] bg-[var(--color-bg-surface)] font-sans text-[var(--color-text-muted)]">
              <tr>
                <th className="h-8 w-40 px-3 text-left font-medium">时间</th>
                <th className="h-8 w-16 px-3 text-left font-medium">级别</th>
                <th className="h-8 w-28 px-3 text-left font-medium">组件</th>
                <th className="h-8 px-3 text-left font-medium">内容</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border-muted)]">
              {shown.map((e, i) => (
                <tr key={`${e.time}-${i}`} className="align-top hover:bg-[var(--color-accent-muted)]">
                  <td className="whitespace-nowrap px-3 py-1 text-[var(--color-text-muted)]">{e.time}</td>
                  <td className="px-3 py-1">
                    <Badge variant={badgeOf[e.level as keyof typeof badgeOf] ?? 'default'} size="sm">
                      {e.level}
                    </Badge>
                  </td>
                  <td className="max-w-[112px] truncate px-3 py-1 text-[var(--color-text-muted)]" title={e.component}>
                    {e.component}
                  </td>
                  <td className={`break-all px-3 py-1 ${textOf[e.level] ?? 'text-[var(--color-text-secondary)]'}`}>
                    {e.message}
                    {e.fields && <span className="ml-2 text-[var(--color-text-muted)]">{fieldsText(e.fields)}</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
