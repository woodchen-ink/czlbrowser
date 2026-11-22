import { useEffect, useMemo, useState } from 'react'
import clsx from 'clsx'
import { Check, Gauge, Loader2, Search } from 'lucide-react'
import { Modal } from '../../../shared/components'
import { useEvents } from '../../../shared/hooks/useRuntimeEvents'
import { browserProxyBatchTestSpeed, browserProxyTestSpeed, fetchBrowserProxies, type ProxyTestResult } from '../api'
import type { BrowserProxy } from '../types'

interface ProxyPickerModalProps {
  open: boolean
  currentProxyId: string
  onSelect: (proxy: BrowserProxy) => void
  onClose: () => void
}

type Speed = Pick<ProxyTestResult, 'ok' | 'latencyMs'>

const ALL = '\u0000all'
const DIRECT = '__direct__'

/** 最近一次测速: 本次弹窗里测的优先, 否则用代理上记录的历史结果。 */
const speedOf = (p: BrowserProxy, fresh: Record<string, Speed>): Speed | undefined =>
  fresh[p.proxyId] ?? (p.lastTestedAt ? { ok: !!p.lastTestOk, latencyMs: p.lastLatencyMs ?? -1 } : undefined)

// 排序: 测通的按延迟升序 → 直连 → 未测 → 测不通
const rank = (p: BrowserProxy, s?: Speed) => (s?.ok && s.latencyMs >= 0 ? 0 : p.proxyId === DIRECT ? 1 : !s ? 2 : 3)

function SpeedText({ speed, testing }: { speed?: Speed; testing: boolean }) {
  if (testing) return <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--color-text-muted)]" />
  if (!speed) return null
  if (!speed.ok) return <span className="text-xs text-[var(--color-error)]">不通</span>
  const tone = speed.latencyMs < 300 ? 'text-[var(--color-success)]' : speed.latencyMs < 800 ? 'text-[var(--color-warning)]' : 'text-[var(--color-error)]'
  return <span className={clsx('text-xs tabular-nums', tone)}>{speed.latencyMs} ms</span>
}

/** 从代理池选一个代理: 左侧分组, 右侧按测速结果排序, 可单个或批量测速。 */
export function ProxyPickerModal({ open, currentProxyId, onSelect, onClose }: ProxyPickerModalProps) {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [group, setGroup] = useState(ALL)
  const [q, setQ] = useState('')
  const [fresh, setFresh] = useState<Record<string, Speed>>({})
  const [testing, setTesting] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!open) return
    setGroup(ALL)
    setQ('')
    setFresh({})
    setTesting(new Set())
    fetchBrowserProxies().then(setProxies)
  }, [open])

  const record = (r: ProxyTestResult) => {
    setFresh((m) => ({ ...m, [r.proxyId]: { ok: r.ok, latencyMs: r.latencyMs } }))
    setTesting((s) => {
      const n = new Set(s)
      n.delete(r.proxyId)
      return n
    })
  }
  // 批量测速的结果逐条推送, 边测边显示
  useEvents({ 'proxy:speed:result': (r: ProxyTestResult) => open && record(r) }, [open])

  const groups = useMemo(() => {
    const count = new Map<string, number>()
    proxies.forEach((p) => p.groupName && count.set(p.groupName, (count.get(p.groupName) ?? 0) + 1))
    return [...count.entries()].sort(([a], [b]) => a.localeCompare(b, 'zh-CN'))
  }, [proxies])

  const shown = useMemo(() => {
    const kw = q.trim().toLowerCase()
    return proxies
      .filter((p) => group === ALL || p.groupName === group)
      .filter((p) => !kw || `${p.proxyName} ${p.proxyConfig}`.toLowerCase().includes(kw))
      .map((p) => ({ p, s: speedOf(p, fresh) }))
      .sort((a, b) => rank(a.p, a.s) - rank(b.p, b.s) || (a.s?.latencyMs ?? 0) - (b.s?.latencyMs ?? 0) || a.p.proxyName.localeCompare(b.p.proxyName, 'zh-CN'))
  }, [proxies, group, q, fresh])

  const test = async (ids: string[]) => {
    ids = ids.filter((id) => id !== DIRECT && !testing.has(id))
    if (!ids.length) return
    setTesting((s) => new Set([...s, ...ids]))
    try {
      if (ids.length === 1) record(await browserProxyTestSpeed(ids[0]))
      else (await browserProxyBatchTestSpeed(ids)).forEach(record)
    } catch {
      setTesting((s) => new Set([...s].filter((id) => !ids.includes(id))))
    }
  }

  const choose = (p: BrowserProxy) => {
    onSelect(p)
    onClose()
  }

  const side = (key: string, label: string, n: number) => (
    <button
      key={key}
      onClick={() => setGroup(key)}
      className={clsx(
        'flex h-8 w-full items-center justify-between gap-2 rounded-md px-2.5 text-left text-[13px]',
        group === key ? 'bg-[var(--color-brand-soft)] font-medium text-[var(--color-brand-strong)]' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)]',
      )}
    >
      <span className="truncate">{label}</span>
      <span className="text-xs tabular-nums opacity-60">{n}</span>
    </button>
  )

  return (
    <Modal open={open} onClose={onClose} title="选择代理" width="760px" bodyClassName="p-0">
      <div className="flex h-[460px]">
        <div className="w-44 shrink-0 space-y-0.5 overflow-y-auto border-r border-[var(--color-border-default)] bg-[var(--color-bg-sunken)] p-2">
          {side(ALL, '全部', proxies.length)}
          {groups.map(([g, n]) => side(g, g, n))}
        </div>
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-center gap-2 border-b border-[var(--color-border-default)] px-3 py-2">
            <div className="relative flex-1">
              <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-muted)]" />
              <input
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="搜索名称或地址"
                className="h-8 w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-input)] pl-8 pr-2.5 text-[13px] text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:border-[var(--color-border-strong)] focus:outline-none"
              />
            </div>
            <button
              onClick={() => test(shown.map((x) => x.p.proxyId))}
              disabled={testing.size > 0 || !shown.length}
              className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-[var(--color-border-default)] px-2.5 text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)] disabled:cursor-not-allowed disabled:opacity-40"
            >
              <Gauge className="h-3.5 w-3.5" />
              测速全部
            </button>
          </div>
          <ul className="flex-1 overflow-y-auto">
            {shown.length === 0 && <li className="py-16 text-center text-sm text-[var(--color-text-muted)]">没有匹配的代理</li>}
            {shown.map(({ p, s }) => {
              const current = p.proxyId === currentProxyId
              return (
                <li
                  key={p.proxyId}
                  onClick={() => choose(p)}
                  className={clsx(
                    'flex cursor-pointer items-center gap-3 border-b border-[var(--color-border-muted)] px-4 py-2 last:border-0',
                    current ? 'bg-[var(--color-brand-soft)]' : 'hover:bg-[var(--color-accent-muted)]',
                  )}
                >
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[13px] font-medium text-[var(--color-text-primary)]">
                      {p.proxyName || p.proxyId}
                      {p.groupName && group === ALL && <span className="ml-2 text-xs font-normal text-[var(--color-text-muted)]">{p.groupName}</span>}
                    </div>
                    {p.proxyId !== DIRECT && <div className="mt-0.5 truncate font-mono text-[11px] text-[var(--color-text-muted)]">{p.proxyConfig}</div>}
                  </div>
                  <SpeedText speed={s} testing={testing.has(p.proxyId)} />
                  {p.proxyId !== DIRECT && (
                    <button
                      title="测速"
                      disabled={testing.has(p.proxyId)}
                      onClick={(e) => {
                        e.stopPropagation()
                        test([p.proxyId])
                      }}
                      className="rounded p-1 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)] disabled:opacity-40"
                    >
                      <Gauge className="h-3.5 w-3.5" />
                    </button>
                  )}
                  {current && <Check className="h-4 w-4 shrink-0 text-[var(--color-brand-strong)]" />}
                </li>
              )
            })}
          </ul>
          <div className="border-t border-[var(--color-border-default)] px-4 py-2 text-xs text-[var(--color-text-muted)]">{shown.length} 个, 点击即选用</div>
        </div>
      </div>
    </Modal>
  )
}
