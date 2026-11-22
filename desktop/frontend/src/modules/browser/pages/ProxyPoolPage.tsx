import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowDown, ArrowUp, Gauge, Globe2, Info, MoreHorizontal, Pencil, Plus, Search, Send, Trash2, X } from 'lucide-react'
import { Button, ConfirmModal, Dropdown, Input, Modal, Select, toast } from '../../../shared/components'
import { useEvents } from '../../../shared/hooks/useRuntimeEvents'
import { useCloudStore } from '../../../store/cloudStore'
import { CopyToSpaceModal } from '../../cloud/components/CopyToSpaceModal'
import { errorText } from '../../cloud/errors'
import {
  browserProxyBatchCheckIPHealth,
  browserProxyBatchTestSpeed,
  browserProxyCheckIPHealth,
  browserProxyTestSpeed,
  fetchBrowserProxies,
  saveBrowserProxies,
  type ProxyTestResult,
} from '../api'
import { ProxyEditModal } from '../components/ProxyEditModal'
import { GROUP_ALL, GROUP_NONE, GroupPills, TagChips, TagFilter, countTags, matchGroup, matchTags } from '../components/taxonomy'
import type { BrowserProxy, ProxyIPHealthResult } from '../types'
import { DIRECT_CONFIG, describeProxy, maskProxy } from '../utils/proxyAddress'

const DIRECT_ID = '__direct__'
const DIRECT_ROW: BrowserProxy = { proxyId: DIRECT_ID, proxyName: '直连 (不走代理)', proxyConfig: DIRECT_CONFIG }

type Speed = Pick<ProxyTestResult, 'ok' | 'latencyMs' | 'error'>
type SortKey = 'name' | 'group' | 'server' | 'latency'

/** 列表行: 代理 + 解析出的地址 + 最近的测速 / 出口结果 */
interface Row {
  p: BrowserProxy
  type: string
  host: string
  port: number
  speed?: Speed
  ip?: ProxyIPHealthResult
}

const newId = () => `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

/** 代理上记录的上一次出口查询结果 */
function storedIP(p: BrowserProxy): ProxyIPHealthResult | undefined {
  if (!p.lastIPHealthJson) return undefined
  try {
    const v = JSON.parse(p.lastIPHealthJson)
    return v?.proxyId ? v : undefined
  } catch {
    return undefined
  }
}

const storedSpeed = (p: BrowserProxy): Speed | undefined => (p.lastTestedAt ? { ok: !!p.lastTestOk, latencyMs: p.lastLatencyMs ?? 0, error: '' } : undefined)

// 延迟排序: 有结果且通的按毫秒, 然后是不通的, 最后没测过的
const latencyRank = (s?: Speed) => (!s ? [2, 0] : s.ok ? [0, s.latencyMs] : [1, 0])

function compare(a: Row, b: Row, key: SortKey): number {
  const text = (x: string, y: string) => x.localeCompare(y, 'zh-CN')
  if (key === 'name') return text(a.p.proxyName, b.p.proxyName)
  if (key === 'group') return text(a.p.groupName ?? '', b.p.groupName ?? '')
  if (key === 'server') return text(a.host, b.host) || a.port - b.port
  const [ra, va] = latencyRank(a.speed)
  const [rb, vb] = latencyRank(b.speed)
  return ra - rb || va - vb || text(a.p.proxyName, b.p.proxyName)
}

const dash = <span className="text-[var(--color-text-muted)]">-</span>

function Latency({ s, testing }: { s?: Speed; testing: boolean }) {
  if (testing) return <span className="animate-pulse text-xs text-[var(--color-text-muted)]">测速中</span>
  if (!s) return dash
  if (!s.ok)
    return (
      <span className="text-xs text-[var(--color-error)]" title={s.error}>
        不通
      </span>
    )
  const tone = s.latencyMs < 300 ? 'success' : s.latencyMs < 800 ? 'warning' : 'error'
  return <span className={`text-xs font-medium text-[var(--color-${tone})]`}>{s.latencyMs} ms</span>
}

function Exit({ r, checking }: { r?: ProxyIPHealthResult; checking: boolean }) {
  if (checking) return <span className="animate-pulse text-xs text-[var(--color-text-muted)]">查询中</span>
  if (!r) return dash
  if (!r.ok)
    return (
      <span className="text-xs text-[var(--color-error)]" title={r.error}>
        查询失败
      </span>
    )
  const place = [r.country, r.city !== r.country && r.city].filter(Boolean).join(' · ')
  return (
    <div className="min-w-0" title={[r.ip, r.isp, place, r.timezone].filter(Boolean).join(' | ')}>
      <div className="truncate text-xs text-[var(--color-text-secondary)]">{place || r.ip}</div>
      <div className="truncate font-mono text-[11px] text-[var(--color-text-muted)]">
        {r.ip}
        {r.isp && ` · ${r.isp}`}
      </div>
    </div>
  )
}

/** 代理池: 增删改、测速、查出口 IP、按分组 / 标签 / 协议筛选。整个列表一次保存到后端。 */
export function ProxyPoolPage() {
  const navigate = useNavigate()
  const [list, setList] = useState<BrowserProxy[] | null>(null)
  const [speeds, setSpeeds] = useState<Record<string, Speed>>({})
  const [ips, setIps] = useState<Record<string, ProxyIPHealthResult>>({})
  const [testing, setTesting] = useState<Set<string>>(new Set())
  const [checking, setChecking] = useState<Set<string>>(new Set())

  const [keyword, setKeyword] = useState('')
  const [protocol, setProtocol] = useState('all')
  const [group, setGroup] = useState(GROUP_ALL)
  const [tagFilter, setTagFilter] = useState<string[]>([])
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean } | null>(null)

  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [editing, setEditing] = useState<{ proxy?: BrowserProxy; group?: string } | null>(null)
  const [removing, setRemoving] = useState<string[] | null>(null)
  const [detail, setDetail] = useState<ProxyIPHealthResult | null>(null)
  const [copyOpen, setCopyOpen] = useState(false)

  const load = useCallback(async () => {
    const l = await fetchBrowserProxies()
    // 后端在同步空间里不返回内置直连, 这里补上一行
    setList(l.some((p) => p.proxyId === DIRECT_ID) ? l : [DIRECT_ROW, ...l])
    // 列表重载后以后端记录为准
    setSpeeds({})
    setIps({})
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const changeTick = useCloudStore((s) => s.changeTick)
  useEffect(() => {
    if (changeTick === 0) return
    setPicked(new Set())
    load()
  }, [changeTick, load])

  const done = (set: typeof setTesting, id: string) =>
    set((s) => {
      const n = new Set(s)
      n.delete(id)
      return n
    })

  // 批量测速 / 查询的结果逐条推送过来
  useEvents(
    {
      'proxy:speed:result': (r: ProxyTestResult) => {
        setSpeeds((m) => ({ ...m, [r.proxyId]: r }))
        done(setTesting, r.proxyId)
      },
      'proxy:iphealth:result': (r: ProxyIPHealthResult) => {
        setIps((m) => ({ ...m, [r.proxyId]: r }))
        done(setChecking, r.proxyId)
      },
    },
    [],
  )

  const all = list ?? []
  const real = useMemo(() => all.filter((p) => p.proxyId !== DIRECT_ID), [all])

  const rows: Row[] = useMemo(
    () => all.map((p) => ({ p, ...(({ type, host, port }) => ({ type, host, port }))(describeProxy(p.proxyConfig)), speed: speeds[p.proxyId] ?? storedSpeed(p), ip: ips[p.proxyId] ?? storedIP(p) })),
    [all, speeds, ips],
  )

  const groupNames = useMemo(() => [...new Set(real.map((p) => p.groupName).filter(Boolean) as string[])].sort((a, b) => a.localeCompare(b, 'zh-CN')), [real])
  const protocols = useMemo(() => [...new Set(rows.map((r) => r.type).filter((t) => t && t !== 'direct'))], [rows])
  const tagOptions = useMemo(() => countTags(real.filter((p) => matchGroup(p.groupName ?? '', group)), (p) => p.tags), [real, group])

  const shown = useMemo(() => {
    const q = keyword.trim().toLowerCase()
    const out = rows.filter(
      (r) =>
        (protocol === 'all' || r.type === protocol) &&
        matchGroup(r.p.groupName ?? '', group) &&
        matchTags(r.p.tags, tagFilter) &&
        (!q || r.p.proxyName.toLowerCase().includes(q) || r.host.toLowerCase().includes(q)),
    )
    if (!sort) return out
    return out.sort((a, b) => (sort.desc ? -1 : 1) * compare(a, b, sort.key))
  }, [rows, protocol, group, tagFilter, keyword, sort])

  // ---- 保存 ----

  const persist = async (next: BrowserProxy[]) => {
    try {
      // 后端总会保留内置直连, 这里整张表原样提交
      await saveBrowserProxies(next)
      setList(next)
      return true
    } catch (e) {
      toast.error(`保存失败: ${errorText(e)}`)
      return false
    }
  }

  const saveEdit = async (fields: Pick<BrowserProxy, 'proxyName' | 'proxyConfig' | 'groupName' | 'tags'>) => {
    const cur = editing?.proxy
    const next = cur ? all.map((p) => (p.proxyId === cur.proxyId ? { ...p, ...fields } : p)) : [...all, { proxyId: newId(), ...fields }]
    if (!(await persist(next))) throw new Error('save failed')
    toast.success(cur ? '已保存' : '已添加')
  }

  const confirmRemove = async () => {
    if (!removing) return
    const drop = new Set(removing)
    if (await persist(all.filter((p) => !drop.has(p.proxyId)))) {
      toast.success(`已删除 ${drop.size} 个代理`)
      setPicked((s) => new Set([...s].filter((id) => !drop.has(id))))
    }
  }

  // ---- 测速与出口 ----

  const probeable = (ids: string[]) => ids.filter((id) => id !== DIRECT_ID)

  const testSpeed = async (ids: string[]) => {
    ids = probeable(ids)
    if (!ids.length) return
    setTesting((s) => new Set([...s, ...ids]))
    try {
      const res = ids.length === 1 ? [await browserProxyTestSpeed(ids[0])] : await browserProxyBatchTestSpeed(ids)
      setSpeeds((m) => Object.fromEntries([...Object.entries(m), ...res.map((r) => [r.proxyId, r])]))
    } finally {
      setTesting((s) => new Set([...s].filter((id) => !ids.includes(id))))
    }
  }

  const checkExit = async (ids: string[]) => {
    ids = probeable(ids)
    if (!ids.length) return
    setChecking((s) => new Set([...s, ...ids]))
    try {
      const res = ids.length === 1 ? [await browserProxyCheckIPHealth(ids[0])] : await browserProxyBatchCheckIPHealth(ids)
      setIps((m) => Object.fromEntries([...Object.entries(m), ...res.map((r) => [r.proxyId, r])]))
      const failed = res.filter((r) => !r.ok)
      if (ids.length === 1 && failed.length) toast.error(failed[0].error || '查询失败')
      else if (ids.length > 1) toast.info(`出口查询完成: ${res.length - failed.length} 成功, ${failed.length} 失败`)
    } finally {
      setChecking((s) => new Set([...s].filter((id) => !ids.includes(id))))
    }
  }

  // ---- 选择 ----

  const selectable = shown.filter((r) => r.p.proxyId !== DIRECT_ID)
  const allPicked = selectable.length > 0 && selectable.every((r) => picked.has(r.p.proxyId))
  const somePicked = selectable.some((r) => picked.has(r.p.proxyId))
  const toggle = (id: string) =>
    setPicked((s) => {
      const n = new Set(s)
      if (!n.delete(id)) n.add(id)
      return n
    })

  const SortHead = ({ k, className, children }: { k: SortKey; className?: string; children: ReactNode }) => (
    <th className={`${th} ${className ?? ''}`}>
      <button
        className="inline-flex items-center gap-1 hover:text-[var(--color-text-primary)]"
        // 点一次升序, 再点降序, 第三次取消
        onClick={() => setSort((s) => (s?.key !== k ? { key: k, desc: false } : s.desc ? null : { key: k, desc: true }))}
      >
        {children}
        {sort?.key === k && (sort.desc ? <ArrowDown className="h-3 w-3" /> : <ArrowUp className="h-3 w-3" />)}
      </button>
    </th>
  )

  const groupPills = groupNames.map((g) => ({ key: g, label: g, count: real.filter((p) => p.groupName === g).length }))
  const visibleIds = shown.map((r) => r.p.proxyId)

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="mb-2.5 flex shrink-0 items-center gap-2">
        <h1 className="text-base font-semibold text-[var(--color-text-primary)]">代理</h1>
        <span className="text-xs tabular-nums text-[var(--color-text-muted)]">{real.length} 个</span>
        <div className="ml-auto flex items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-muted)]" />
            <Input value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="搜索名称或服务器" className="w-48 pl-8" />
          </div>
          <Select value={protocol} onChange={(e) => setProtocol(e.target.value)} className="w-28" options={[{ value: 'all', label: '全部协议' }, ...protocols.map((p) => ({ value: p, label: p.toUpperCase() }))]} />
          <TagFilter tags={tagOptions} value={tagFilter} onChange={setTagFilter} />
          <Button variant="secondary" onClick={() => testSpeed(visibleIds)} loading={testing.size > 1} disabled={!shown.length} title="给列表里的代理测延迟">
            <Gauge className="h-3.5 w-3.5" />
            测速
          </Button>
          <Button variant="secondary" onClick={() => checkExit(visibleIds)} loading={checking.size > 1} disabled={!shown.length} title="查询列表里代理的出口 IP 和地区">
            <Globe2 className="h-3.5 w-3.5" />
            查出口
          </Button>
          <Button onClick={() => setEditing({ group: group !== GROUP_ALL && group !== GROUP_NONE ? group : '' })}>
            <Plus className="h-3.5 w-3.5" />
            新增代理
          </Button>
        </div>
      </div>

      <div className="mb-2 flex h-8 shrink-0 items-center">
        {picked.size > 0 ? (
          <div className="flex w-full items-center gap-1.5">
            <span className="mr-1 text-[13px] font-medium tabular-nums text-[var(--color-text-primary)]">已选 {picked.size}</span>
            <Button size="sm" variant="secondary" onClick={() => setCopyOpen(true)} title="复制到本机、云端或团队空间">
              <Send className="h-3.5 w-3.5" />
              复制到...
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setRemoving([...picked])} className="text-[var(--color-error)] hover:bg-[var(--color-error-soft)] hover:text-[var(--color-error)]">
              <Trash2 className="h-3.5 w-3.5" />
              删除
            </Button>
            <button onClick={() => setPicked(new Set())} className="ml-auto inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]">
              <X className="h-3.5 w-3.5" />
              取消选择
            </button>
          </div>
        ) : (
          <GroupPills groups={groupPills} value={group} onChange={setGroup} total={real.length} ungrouped={real.filter((p) => !p.groupName).length} onManage={() => navigate('/settings?tab=taxonomy')} />
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
        <table className="w-full text-[13px]">
          <thead className="sticky top-0 z-10 bg-[var(--color-bg-surface)] text-xs text-[var(--color-text-muted)]">
            <tr>
              <th className={`${th} w-9 !pr-0`}>
                <input
                  type="checkbox"
                  aria-label="全选"
                  checked={allPicked}
                  ref={(el) => {
                    if (el) el.indeterminate = somePicked && !allPicked
                  }}
                  onChange={() => setPicked((s) => (allPicked ? new Set([...s].filter((id) => !visibleIds.includes(id))) : new Set([...s, ...selectable.map((r) => r.p.proxyId)])))}
                  className="h-3.5 w-3.5 cursor-pointer align-middle accent-[var(--color-button-primary)]"
                />
              </th>
              <SortHead k="name">名称</SortHead>
              <SortHead k="group" className="w-28">
                分组
              </SortHead>
              <SortHead k="server" className="w-56">
                地址
              </SortHead>
              <SortHead k="latency" className="w-20">
                延迟
              </SortHead>
              <th className={`${th} w-56`}>出口</th>
              <th className={`${th} w-28`} />
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--color-border-muted)]">
            {list && shown.length === 0 && (
              <tr>
                <td colSpan={7} className="py-16 text-center">
                  <p className="text-[13px] text-[var(--color-text-secondary)]">{real.length ? '没有符合条件的代理' : '还没有代理'}</p>
                  {!real.length && (
                    <Button className="mt-3" size="sm" onClick={() => setEditing({})}>
                      <Plus className="h-3.5 w-3.5" />
                      新增代理
                    </Button>
                  )}
                </td>
              </tr>
            )}
            {shown.map((r) => {
              const { p } = r
              const direct = p.proxyId === DIRECT_ID
              const sel = picked.has(p.proxyId)
              return (
                <tr key={p.proxyId} className={sel ? 'bg-[var(--color-brand-soft)]' : 'hover:bg-[var(--color-accent-muted)]'}>
                  <td className="w-9 py-2 pl-3 pr-0">
                    <input
                      type="checkbox"
                      aria-label={`选择 ${p.proxyName}`}
                      checked={sel}
                      disabled={direct}
                      onChange={() => toggle(p.proxyId)}
                      className="h-3.5 w-3.5 cursor-pointer align-middle accent-[var(--color-button-primary)] disabled:cursor-not-allowed disabled:opacity-30"
                    />
                  </td>
                  <td className="px-3 py-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="max-w-[18rem] truncate font-medium text-[var(--color-text-primary)]" title={p.proxyName}>
                        {p.proxyName}
                      </span>
                      <TagChips tags={p.tags} max={2} />
                    </div>
                  </td>
                  <td className="max-w-[7rem] truncate px-3 py-2 text-[var(--color-text-secondary)]">{p.groupName || dash}</td>
                  <td className="px-3 py-2">
                    {direct ? (
                      <span className="text-[var(--color-text-muted)]">不走代理</span>
                    ) : (
                      <span className="block max-w-[14rem] truncate font-mono text-xs text-[var(--color-text-secondary)]" title={maskProxy(p.proxyConfig)}>
                        <span className="text-[var(--color-text-muted)]">{r.type.toUpperCase()} </span>
                        {r.host}
                        {r.port > 0 && `:${r.port}`}
                      </span>
                    )}
                  </td>
                  <td className="whitespace-nowrap px-3 py-2 tabular-nums">{direct ? dash : <Latency s={r.speed} testing={testing.has(p.proxyId)} />}</td>
                  <td className="px-3 py-2">{direct ? dash : <Exit r={r.ip} checking={checking.has(p.proxyId)} />}</td>
                  <td className="px-3 py-2">
                    <div className="flex items-center justify-end gap-1">
                      <Button size="sm" variant="secondary" onClick={() => testSpeed([p.proxyId])} loading={testing.has(p.proxyId)} disabled={direct}>
                        测速
                      </Button>
                      <Dropdown
                        placement="bottom-right"
                        onSelect={(k) => {
                          if (k === 'ip') checkExit([p.proxyId])
                          else if (k === 'detail' && r.ip) setDetail(r.ip)
                          else if (k === 'edit') setEditing({ proxy: p })
                          else if (k === 'delete') setRemoving([p.proxyId])
                        }}
                        items={[
                          { key: 'ip', label: '查出口 IP', icon: <Globe2 className="h-3.5 w-3.5" />, disabled: direct },
                          { key: 'detail', label: '出口详情', icon: <Info className="h-3.5 w-3.5" />, disabled: !r.ip },
                          { key: 'edit', label: '编辑', icon: <Pencil className="h-3.5 w-3.5" />, disabled: direct },
                          { key: '-', label: '', divider: true },
                          { key: 'delete', label: '删除', icon: <Trash2 className="h-3.5 w-3.5" />, danger: true, disabled: direct },
                        ]}
                        trigger={
                          <button aria-label="更多操作" className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]">
                            <MoreHorizontal className="h-4 w-4" />
                          </button>
                        }
                      />
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      <CopyToSpaceModal open={copyOpen} kind="proxies" ids={[...picked]} onClose={() => setCopyOpen(false)} />

      <ProxyEditModal target={editing} groups={groupNames} tagSuggestions={countTags(real, (p) => p.tags).map((t) => t.name)} onClose={() => setEditing(null)} onSave={saveEdit} />

      <ConfirmModal
        open={removing !== null}
        onClose={() => setRemoving(null)}
        onConfirm={confirmRemove}
        title="删除代理"
        content={`删除 ${removing?.length ?? 0} 个代理, 无法恢复。`}
        confirmText="删除"
        danger
      />

      <Modal open={detail !== null} onClose={() => setDetail(null)} title="出口查询原始结果" width="760px" footer={<Button variant="secondary" onClick={() => setDetail(null)}>关闭</Button>}>
        {detail && (
          <div className="space-y-3">
            <p className="text-xs text-[var(--color-text-muted)]">
              来源 {detail.source} · {new Date(detail.updatedAt).toLocaleString('zh-CN')}
            </p>
            {!detail.ok && <p className="text-sm text-[var(--color-error)]">{detail.error || '查询失败'}</p>}
            <pre className="max-h-[420px] overflow-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-3 text-xs leading-5">{JSON.stringify(detail.rawData ?? {}, null, 2)}</pre>
          </div>
        )}
      </Modal>
    </div>
  )
}

const th = 'h-9 whitespace-nowrap border-b border-[var(--color-border-default)] px-3 text-left font-medium'
