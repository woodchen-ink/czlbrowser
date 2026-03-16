import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, Card, ConfirmModal, FormItem, Input, Modal, Table, toast } from '../../../shared/components'
import type { SortOrder, TableColumn } from '../../../shared/components/Table'
import type { BrowserProxy, ProxyIPHealthResult } from '../types'
import { fetchBrowserProxies, fetchBrowserProxyGroups, saveBrowserProxies, browserProxyTestSpeed, browserProxyBatchTestSpeed, browserProxyCheckIPHealth, browserProxyBatchCheckIPHealth } from '../api'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import yaml from 'js-yaml'

// 内置代理 ID，不可删除、不可编辑
const BUILTIN_PROXY_IDS = new Set(['__direct__'])
const PROXY_LATENCY_CACHE_KEY = 'browser:proxyPool:latencyMap:v1'
const PROXY_IP_HEALTH_CACHE_KEY = 'browser:proxyPool:ipHealthMap:v1'
const PROXY_LATENCY_CACHE_TTL_MS = 12 * 60 * 60 * 1000
const PROXY_IP_HEALTH_CACHE_TTL_MS = 12 * 60 * 60 * 1000

const BUILTIN_PROXIES: BrowserProxy[] = [
  { proxyId: '__direct__', proxyName: '直连（不走代理）', proxyConfig: 'direct://' },
]

function ensureBuiltinProxies(proxies: BrowserProxy[]): BrowserProxy[] {
  const result = [...proxies]
  for (const builtin of BUILTIN_PROXIES) {
    if (!result.find(p => p.proxyId === builtin.proxyId)) {
      result.unshift(builtin)
    }
  }
  return result
}

interface ProxyDisplayInfo {
  proxyId: string
  proxyName: string
  proxyConfig: string
  groupName: string
  type: string
  server: string
  port: number
  latencyMs?: number
}

function parseProxyInfo(proxyConfig: string): { type: string; server: string; port: number } {
  const cfg = proxyConfig.trim()
  if (cfg === 'direct://') return { type: 'direct', server: '-', port: 0 }
  const urlMatch = cfg.match(/^([a-zA-Z0-9+\-]+):\/\//)
  if (urlMatch) {
    const scheme = urlMatch[1].toLowerCase()
    try {
      const u = new URL(cfg)
      return { type: scheme, server: u.hostname, port: parseInt(u.port) || 0 }
    } catch {
      return { type: scheme, server: '-', port: 0 }
    }
  }
  try {
    const parsed = yaml.load(cfg) as any
    const proxy = Array.isArray(parsed) ? parsed[0] : parsed
    return { type: proxy?.type || '-', server: proxy?.server || '-', port: proxy?.port || 0 }
  } catch {
    return { type: '-', server: '-', port: 0 }
  }
}

function parseProxyConfig(proxyConfig: string): { proxyType: string; proxyHost: string; proxyPort: string; proxyUser: string; proxyPass: string } {
  const cfg = proxyConfig.trim()
  if (cfg === 'direct://') return { proxyType: 'direct', proxyHost: '', proxyPort: '', proxyUser: '', proxyPass: '' }

  // Try standard URL first
  const urlMatch = cfg.match(/^(socks5|http|https):\/\//)
  if (urlMatch) {
    try {
      const u = new URL(cfg)
      return {
        proxyType: urlMatch[1].toLowerCase(),
        proxyHost: u.hostname,
        proxyPort: u.port || '',
        proxyUser: decodeURIComponent(u.username || ''),
        proxyPass: decodeURIComponent(u.password || ''),
      }
    } catch {
      return { proxyType: urlMatch[1].toLowerCase(), proxyHost: '', proxyPort: '', proxyUser: '', proxyPass: '' }
    }
  }

  // Try Clash YAML (legacy support)
  try {
    const parsed = yaml.load(cfg) as any
    const proxy = Array.isArray(parsed) ? parsed[0] : parsed
    if (proxy && typeof proxy === 'object') {
      return {
        proxyType: (proxy.type || 'socks5').toLowerCase(),
        proxyHost: proxy.server || '',
        proxyPort: String(proxy.port || ''),
        proxyUser: proxy.username || '',
        proxyPass: proxy.password || '',
      }
    }
  } catch {}

  return { proxyType: 'socks5', proxyHost: '', proxyPort: '', proxyUser: '', proxyPass: '' }
}

function buildProxyConfig(type: string, host: string, port: string, user: string, pass: string): string {
  if (type === 'direct') return 'direct://'
  const auth = user ? `${encodeURIComponent(user)}:${encodeURIComponent(pass)}@` : ''
  return `${type}://${auth}${host}:${port}`
}

function toDisplayList(proxies: BrowserProxy[]): ProxyDisplayInfo[] {
  return proxies.map(p => {
    const info = parseProxyInfo(p.proxyConfig)
    return {
      proxyId: p.proxyId,
      proxyName: p.proxyName,
      proxyConfig: p.proxyConfig,
      groupName: p.groupName || '',
      ...info,
    }
  })
}

function nextProxyID(): string {
  return `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function toLatencyValue(ok: boolean, latencyMs: number, error?: string): number {
  if (ok) return latencyMs
  return error?.includes('不支持') ? -3 : -2
}

function readLatencyCache(): Record<string, number> {
  try {
    const raw = localStorage.getItem(PROXY_LATENCY_CACHE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as { timestamp?: number; data?: Record<string, number> }
    if (!parsed?.timestamp || !parsed?.data) return {}
    if (Date.now() - parsed.timestamp > PROXY_LATENCY_CACHE_TTL_MS) return {}
    const cleaned: Record<string, number> = {}
    Object.entries(parsed.data).forEach(([proxyId, latency]) => {
      if (typeof latency === 'number' && Number.isFinite(latency) && latency !== -1) {
        cleaned[proxyId] = latency
      }
    })
    return cleaned
  } catch {
    return {}
  }
}

function writeLatencyCache(data: Record<string, number>) {
  try {
    const cleaned: Record<string, number> = {}
    Object.entries(data).forEach(([proxyId, latency]) => {
      if (typeof latency === 'number' && Number.isFinite(latency) && latency !== -1) {
        cleaned[proxyId] = latency
      }
    })
    localStorage.setItem(PROXY_LATENCY_CACHE_KEY, JSON.stringify({
      timestamp: Date.now(),
      data: cleaned,
    }))
  } catch {
    // ignore write failures
  }
}

function readIPHealthCache(): Record<string, ProxyIPHealthResult> {
  try {
    const raw = localStorage.getItem(PROXY_IP_HEALTH_CACHE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as { timestamp?: number; data?: Record<string, ProxyIPHealthResult> }
    if (!parsed?.timestamp || !parsed?.data) return {}
    if (Date.now() - parsed.timestamp > PROXY_IP_HEALTH_CACHE_TTL_MS) return {}
    const cleaned: Record<string, ProxyIPHealthResult> = {}
    Object.entries(parsed.data).forEach(([proxyId, item]) => {
      if (item && typeof item === 'object') {
        cleaned[proxyId] = item
      }
    })
    return cleaned
  } catch {
    return {}
  }
}

function writeIPHealthCache(data: Record<string, ProxyIPHealthResult>) {
  try {
    localStorage.setItem(PROXY_IP_HEALTH_CACHE_KEY, JSON.stringify({
      timestamp: Date.now(),
      data,
    }))
  } catch {
    // ignore write failures
  }
}

export function ProxyPoolPage() {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [displayList, setDisplayList] = useState<ProxyDisplayInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [groups, setGroups] = useState<string[]>([])

  const [filterProtocol, setFilterProtocol] = useState<string>('all')
  const [filterKeyword, setFilterKeyword] = useState('')
  const [filterGroup, setFilterGroup] = useState<string>('all')
  const [sortColumn, setSortColumn] = useState<string>('') // 默认不排序
  const [sortOrder, setSortOrder] = useState<SortOrder>(undefined)

  const [latencyMap, setLatencyMap] = useState<Record<string, number>>({})
  const [testingAll, setTestingAll] = useState(false)
  const [ipHealthMap, setIPHealthMap] = useState<Record<string, ProxyIPHealthResult>>({})
  const [checkingIPHealthIds, setCheckingIPHealthIds] = useState<Set<string>>(new Set())
  const [checkingAllIPHealth, setCheckingAllIPHealth] = useState(false)

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false)

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editingProxy, setEditingProxy] = useState<BrowserProxy | null>(null)
  const [editForm, setEditForm] = useState({ proxyName: '', groupName: '', proxyType: 'socks5', proxyHost: '', proxyPort: '', proxyUser: '', proxyPass: '' })
  const [saving, setSaving] = useState(false)
  const [isAddMode, setIsAddMode] = useState(false)

  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [ipHealthDetailOpen, setIPHealthDetailOpen] = useState(false)
  const [currentIPHealthDetail, setCurrentIPHealthDetail] = useState<ProxyIPHealthResult | null>(null)
  const proxiesRef = useRef<BrowserProxy[]>([])

  useEffect(() => {
    setLatencyMap(readLatencyCache())
    setIPHealthMap(readIPHealthCache())
    loadProxies()
  }, [])

  useEffect(() => {
    writeLatencyCache(latencyMap)
  }, [latencyMap])

  useEffect(() => {
    writeIPHealthCache(ipHealthMap)
  }, [ipHealthMap])

  useEffect(() => {
    proxiesRef.current = proxies
  }, [proxies])

  useEffect(() => {
    if (!proxies.length) return
    const validIds = new Set(proxies.map(p => p.proxyId))
    setLatencyMap(prev => {
      let changed = false
      const next: Record<string, number> = {}
      Object.entries(prev).forEach(([proxyId, latency]) => {
        if (validIds.has(proxyId)) {
          next[proxyId] = latency
        } else {
          changed = true
        }
      })
      return changed ? next : prev
    })

    setIPHealthMap(prev => {
      let changed = false
      const next: Record<string, ProxyIPHealthResult> = {}
      Object.entries(prev).forEach(([proxyId, health]) => {
        if (validIds.has(proxyId)) {
          next[proxyId] = health
        } else {
          changed = true
        }
      })
      return changed ? next : prev
    })
  }, [proxies])

  const loadProxies = async () => {
    setLoading(true)
    try {
      const raw = await fetchBrowserProxies()
      const proxyList = ensureBuiltinProxies(raw)
      const persistedLatency: Record<string, number> = {}
      const persistedIPHealth: Record<string, ProxyIPHealthResult> = {}
      proxyList.forEach(proxy => {
        if (proxy.lastTestedAt) {
          persistedLatency[proxy.proxyId] = (proxy.lastTestOk ?? false)
            ? (proxy.lastLatencyMs ?? -2)
            : -2
        }
        if (proxy.lastIPHealthJson) {
          try {
            const parsed = JSON.parse(proxy.lastIPHealthJson) as ProxyIPHealthResult
            if (parsed && typeof parsed === 'object' && parsed.proxyId) {
              persistedIPHealth[proxy.proxyId] = parsed
            }
          } catch {
            // ignore bad historical json
          }
        }
      })

      setProxies(proxyList)
      setDisplayList(toDisplayList(proxyList))
      setLatencyMap(prev => ({ ...persistedLatency, ...prev }))
      setIPHealthMap(prev => ({ ...persistedIPHealth, ...prev }))
      const grps = await fetchBrowserProxyGroups()
      setGroups(grps)
    } finally {
      setLoading(false)
    }
  }

  // 直接保存完整列表，内置代理保护由后端负责
  const saveProxies = useCallback(async (list: BrowserProxy[]) => {
    await saveBrowserProxies(list)
    setProxies(list)
    setDisplayList(toDisplayList(list))
    // 刷新分组列表（可能有新分组加入）
    const grps = await fetchBrowserProxyGroups()
    setGroups(grps)
  }, [])

  const protocolOptions = useMemo(
    () => ['all', ...Array.from(new Set(displayList.map(p => p.type).filter(t => t !== '-')))],
    [displayList]
  )

  const getLatencySortTuple = (proxyId: string): [number, number] => {
    const v = latencyMap[proxyId]
    if (v === undefined) return [4, Number.MAX_SAFE_INTEGER]
    if (v === -1) return [1, Number.MAX_SAFE_INTEGER] // 测试中
    if (v === -2) return [2, Number.MAX_SAFE_INTEGER] // 超时
    if (v === -3) return [3, Number.MAX_SAFE_INTEGER] // 不支持
    return [0, v] // 正常延迟
  }

  const compareText = (a: string, b: string) => a.localeCompare(b, 'zh-CN')

  const compareByColumn = (a: ProxyDisplayInfo, b: ProxyDisplayInfo, column: string) => {
    switch (column) {
      case 'proxyName':
        return compareText(a.proxyName || '', b.proxyName || '')
      case 'groupName':
        return compareText(a.groupName || '', b.groupName || '')
      case 'type':
        return compareText(a.type || '', b.type || '')
      case 'server':
        return compareText(a.server || '', b.server || '')
      case 'port':
        return (a.port || 0) - (b.port || 0)
      case 'latency': {
        const [rankA, valA] = getLatencySortTuple(a.proxyId)
        const [rankB, valB] = getLatencySortTuple(b.proxyId)
        if (rankA !== rankB) return rankA - rankB
        if (valA !== valB) return valA - valB
        return compareText(a.proxyName || '', b.proxyName || '')
      }
      default:
        return 0
    }
  }

  const filteredList = useMemo(() => {
    const filtered = displayList.filter(p => {
      const matchProtocol = filterProtocol === 'all' || p.type === filterProtocol
      const matchKeyword = !filterKeyword || p.proxyName.toLowerCase().includes(filterKeyword.toLowerCase()) || p.server.toLowerCase().includes(filterKeyword.toLowerCase())
      const matchGroup = filterGroup === 'all' || p.groupName === filterGroup
      return matchProtocol && matchKeyword && matchGroup
    })

    if (!sortColumn || !sortOrder) return filtered

    return [...filtered].sort((a, b) => {
      const cmp = compareByColumn(a, b, sortColumn)
      return sortOrder === 'asc' ? cmp : -cmp
    })
  }, [displayList, filterProtocol, filterKeyword, filterGroup, sortColumn, sortOrder, latencyMap])

  const allFilteredSelected = filteredList.length > 0 && filteredList.every(p => selectedIds.has(p.proxyId))
  const someFilteredSelected = filteredList.some(p => selectedIds.has(p.proxyId))

  const handleToggleAll = () => {
    if (allFilteredSelected) {
      setSelectedIds(prev => {
        const next = new Set(prev)
        filteredList.forEach(p => next.delete(p.proxyId))
        return next
      })
    } else {
      setSelectedIds(prev => {
        const next = new Set(prev)
        filteredList.filter(p => !BUILTIN_PROXY_IDS.has(p.proxyId)).forEach(p => next.add(p.proxyId))
        return next
      })
    }
  }

  const handleToggleOne = (proxyId: string) => {
    if (BUILTIN_PROXY_IDS.has(proxyId)) return
    setSelectedIds(prev => {
      const next = new Set(prev)
      next.has(proxyId) ? next.delete(proxyId) : next.add(proxyId)
      return next
    })
  }

  const handleBatchDeleteConfirm = async () => {
    try {
      const newProxies = proxies.filter(p => !selectedIds.has(p.proxyId))
      await saveProxies(newProxies)
      toast.success(`已删除 ${selectedIds.size} 个代理`)
      setSelectedIds(new Set())
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
  }

  const handleTestOne = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需测速')
      return
    }
    setLatencyMap(prev => ({ ...prev, [record.proxyId]: -1 }))
    const result = await browserProxyTestSpeed(record.proxyId)
    const val = toLatencyValue(result.ok, result.latencyMs, result.error)
    setLatencyMap(prev => ({ ...prev, [record.proxyId]: val }))
  }

  const handleTestAll = async () => {
    const testable = filteredList.filter(p => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setTestingAll(true)
    const init: Record<string, number> = {}
    testable.forEach(p => { init[p.proxyId] = -1 })
    setLatencyMap(prev => ({ ...prev, ...init }))

    // 监听后端实时推送的单个测速结果
    const off = EventsOn('proxy:speed:result', (data: { proxyId: string; ok: boolean; latencyMs: number; error: string }) => {
      const val = toLatencyValue(data.ok, data.latencyMs, data.error)
      setLatencyMap(prev => ({ ...prev, [data.proxyId]: val }))
    })

    try {
      const proxyIds = testable.map(p => p.proxyId)
      const results = await browserProxyBatchTestSpeed(proxyIds, 20)
      setLatencyMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          next[result.proxyId] = toLatencyValue(result.ok, result.latencyMs, result.error)
        })
        return next
      })
    } finally {
      off()
      setTestingAll(false)
    }
  }

  const handleCheckOneIPHealth = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需检测')
      return
    }
    if (checkingIPHealthIds.has(record.proxyId)) return

    setCheckingIPHealthIds(prev => new Set(prev).add(record.proxyId))
    try {
      const result = await browserProxyCheckIPHealth(record.proxyId)
      setIPHealthMap(prev => ({ ...prev, [record.proxyId]: result }))
      if (!result.ok) {
        toast.error(result.error || `${record.proxyName} 检测失败`)
      }
    } finally {
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        next.delete(record.proxyId)
        return next
      })
    }
  }

  const handleCheckAllIPHealth = async () => {
    const testable = filteredList.filter(p => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setCheckingAllIPHealth(true)

    const ids = testable.map(p => p.proxyId)
    const idSet = new Set(ids)
    setCheckingIPHealthIds(prev => new Set([...Array.from(prev), ...ids]))

    const off = EventsOn('proxy:iphealth:result', (data: ProxyIPHealthResult) => {
      if (!data?.proxyId || !idSet.has(data.proxyId)) return
      setIPHealthMap(prev => ({ ...prev, [data.proxyId]: data }))
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        next.delete(data.proxyId)
        return next
      })
    })

    try {
      const results = await browserProxyBatchCheckIPHealth(ids, 10)
      setIPHealthMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          if (result?.proxyId && idSet.has(result.proxyId)) {
            next[result.proxyId] = result
          }
        })
        return next
      })
      const failed = results.filter(r => !r.ok).length
      if (failed > 0) {
        toast.info(`IP 分析完成：成功 ${results.length - failed}，失败 ${failed}`)
      } else {
        toast.success(`IP 分析完成：共 ${results.length} 条`)
      }
    } finally {
      off()
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        ids.forEach(id => next.delete(id))
        return next
      })
      setCheckingAllIPHealth(false)
    }
  }

  const renderLatency = (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      return <span className="text-[var(--color-text-muted)] text-xs">不适用</span>
    }
    const val = latencyMap[record.proxyId]
    if (val === undefined) return <span className="text-[var(--color-text-muted)] text-xs">-</span>
    if (val === -1) return <span className="text-[var(--color-text-muted)] text-xs animate-pulse">测试中...</span>
    if (val === -2) return <span className="text-red-500 text-xs">超时</span>
    if (val === -3) return <span className="text-gray-400 text-xs">不支持</span>
    const color = val < 200 ? 'text-green-500' : val < 500 ? 'text-yellow-500' : 'text-red-500'
    return <span className={`text-xs font-medium ${color}`}>{val} ms</span>
  }

  const openIPHealthDetail = (proxyId: string) => {
    const result = ipHealthMap[proxyId]
    if (!result) return
    setCurrentIPHealthDetail(result)
    setIPHealthDetailOpen(true)
  }

  const renderIPHealth = (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      return <span className="text-[var(--color-text-muted)] text-xs">不适用</span>
    }
    if (checkingIPHealthIds.has(record.proxyId)) {
      return <span className="text-[var(--color-text-muted)] text-xs animate-pulse">检测中...</span>
    }

    const result = ipHealthMap[record.proxyId]
    if (!result) return <span className="text-[var(--color-text-muted)] text-xs">-</span>
    if (!result.ok) {
      return (
        <div className="flex items-center gap-2">
          <span className="text-xs text-red-500 truncate max-w-[120px]" title={result.error || '检测失败'}>失败</span>
          <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); openIPHealthDetail(record.proxyId) }}>原始</Button>
        </div>
      )
    }

    const location = [result.country, result.region, result.city].filter(Boolean).join(' / ')
    return (
      <div className="flex items-center gap-2 min-w-0">
        <div className="min-w-0">
          <div className="text-xs text-[var(--color-text-primary)] truncate">{result.ip || '-'}</div>
          <div className="text-[11px] text-[var(--color-text-muted)] truncate">
            {[result.isp, location, result.timezone].filter(Boolean).join(' | ')}
          </div>
        </div>
        <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); openIPHealthDetail(record.proxyId) }}>原始</Button>
      </div>
    )
  }

  const columns: TableColumn<ProxyDisplayInfo>[] = [
    {
      key: 'checkbox',
      title: '',
      width: '40px',
      render: (_, record) => (
        <input
          type="checkbox"
          checked={selectedIds.has(record.proxyId)}
          disabled={BUILTIN_PROXY_IDS.has(record.proxyId)}
          onChange={() => handleToggleOne(record.proxyId)}
          onClick={e => e.stopPropagation()}
          className="w-4 h-4 rounded border-[var(--color-border)] accent-[var(--color-primary)] cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
        />
      ),
    },
    { key: 'proxyName', title: '代理名称', width: '180px', sortable: true },
    { key: 'groupName', title: '分组', width: '100px', sortable: true, render: (val) => val ? <span className="px-1.5 py-0.5 text-xs rounded bg-[var(--color-accent)]/10 text-[var(--color-accent)]">{val}</span> : '-' },
    { key: 'type', title: '类型', width: '90px', sortable: true },
    { key: 'server', title: '服务器', width: '180px', sortable: true },
    { key: 'port', title: '端口', width: '80px', sortable: true, render: (val) => val || '-' },
    {
      key: 'latency',
      title: '延迟',
      width: '90px',
      sortable: true,
      render: (_, record) => renderLatency(record),
    },
    {
      key: 'ipHealth',
      title: 'IP 分析',
      width: '280px',
      render: (_, record) => renderIPHealth(record),
    },
    {
      key: 'actions',
      title: '操作',
      width: '260px',
      render: (_, record) => {
        const isBuiltin = BUILTIN_PROXY_IDS.has(record.proxyId)
        return (
          <div className="flex gap-2">
            <Button
              size="sm" variant="ghost"
              onClick={(e) => { e.stopPropagation(); handleTestOne(record) }}
              loading={latencyMap[record.proxyId] === -1}
              disabled={record.proxyConfig === 'direct://'}
            >测速</Button>
            <Button
              size="sm" variant="ghost"
              onClick={(e) => { e.stopPropagation(); handleCheckOneIPHealth(record) }}
              loading={checkingIPHealthIds.has(record.proxyId)}
              disabled={record.proxyConfig === 'direct://'}
            >IP分析</Button>
            <Button
              size="sm" variant="ghost"
              disabled={isBuiltin}
              title={isBuiltin ? '内置代理不可编辑' : undefined}
              onClick={(e) => { e.stopPropagation(); if (!isBuiltin) handleEdit(record) }}
            >编辑</Button>
            <Button
              size="sm" variant="danger"
              disabled={isBuiltin}
              title={isBuiltin ? '内置代理不可删除' : undefined}
              onClick={(e) => { e.stopPropagation(); if (!isBuiltin) handleDeleteClick(record.proxyId) }}
            >删除</Button>
          </div>
        )
      },
    },
  ]

  const handleEdit = (record: ProxyDisplayInfo) => {
    const proxy = proxies.find(p => p.proxyId === record.proxyId)
    if (proxy) {
      setIsAddMode(false)
      setEditingProxy(proxy)
      const parsed = parseProxyConfig(proxy.proxyConfig)
      setEditForm({ proxyName: proxy.proxyName, groupName: proxy.groupName || '', ...parsed })
      setEditModalOpen(true)
    }
  }

  const handleAddProxy = () => {
    setIsAddMode(true)
    setEditingProxy(null)
    setEditForm({ proxyName: '', groupName: '', proxyType: 'socks5', proxyHost: '', proxyPort: '', proxyUser: '', proxyPass: '' })
    setEditModalOpen(true)
  }

  const handleSaveProxy = async () => {
    if (!editForm.proxyName.trim()) { toast.error('请输入代理名称'); return }
    if (editForm.proxyType !== 'direct') {
      if (!editForm.proxyHost.trim()) { toast.error('请输入代理主机'); return }
      if (!editForm.proxyPort.trim()) { toast.error('请输入代理端口'); return }
    }
    const proxyConfig = buildProxyConfig(editForm.proxyType, editForm.proxyHost.trim(), editForm.proxyPort.trim(), editForm.proxyUser.trim(), editForm.proxyPass.trim())
    setSaving(true)
    try {
      if (isAddMode) {
        const newProxy: BrowserProxy = {
          proxyId: nextProxyID(),
          proxyName: editForm.proxyName.trim(),
          proxyConfig,
          groupName: editForm.groupName.trim() || undefined,
        }
        const allProxies = [...proxies, newProxy]
        await saveProxies(allProxies)
        setEditModalOpen(false)
        toast.success('代理已添加')
      } else {
        if (!editingProxy) return
        const newProxies = proxies.map(p =>
          p.proxyId === editingProxy.proxyId
            ? { ...p, proxyName: editForm.proxyName, proxyConfig, groupName: editForm.groupName }
            : p
        )
        await saveProxies(newProxies)
        setEditModalOpen(false)
        toast.success('代理已更新')
      }
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDeleteClick = (proxyId: string) => {
    setDeletingId(proxyId)
    setDeleteConfirmOpen(true)
  }

  const handleDeleteConfirm = async () => {
    if (!deletingId) return
    try {
      const newProxies = proxies.filter(p => p.proxyId !== deletingId)
      await saveProxies(newProxies)
      setSelectedIds(prev => { const next = new Set(prev); next.delete(deletingId); return next })
      toast.success('代理已删除')
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
    setDeletingId(null)
  }

  const selectedCount = selectedIds.size

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">代理池配置</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">管理代理配置，供实例绑定使用</p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="secondary" onClick={handleCheckAllIPHealth} loading={checkingAllIPHealth} disabled={filteredList.length === 0}>IP 分析</Button>
          <Button size="sm" variant="secondary" onClick={handleTestAll} loading={testingAll} disabled={filteredList.length === 0}>测试全部</Button>
          <Button size="sm" onClick={handleAddProxy}>新增代理</Button>
        </div>
      </div>

      <Card>
        <div className="flex items-center gap-3 mb-4">
          <Input
            value={filterKeyword}
            onChange={e => setFilterKeyword(e.target.value)}
            placeholder="搜索名称或服务器..."
            style={{ width: '220px' }}
          />
          <select
            value={filterProtocol}
            onChange={e => setFilterProtocol(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]"
          >
            {protocolOptions.map(p => (
              <option key={p} value={p}>{p === 'all' ? '全部协议' : p.toUpperCase()}</option>
            ))}
          </select>
          <select
            value={filterGroup}
            onChange={e => setFilterGroup(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]"
          >
            <option value="all">全部分组</option>
            {groups.map(g => <option key={g} value={g}>{g}</option>)}
          </select>
          {(filterProtocol !== 'all' || filterKeyword || filterGroup !== 'all') && (
            <Button size="sm" variant="ghost" onClick={() => { setFilterProtocol('all'); setFilterKeyword(''); setFilterGroup('all') }}>清除筛选</Button>
          )}
          <div className="flex-1" />
          {filteredList.length > 0 && (
            <label className="flex items-center gap-1.5 text-sm text-[var(--color-text-muted)] cursor-pointer select-none">
              <input
                type="checkbox"
                checked={allFilteredSelected}
                ref={el => { if (el) el.indeterminate = someFilteredSelected && !allFilteredSelected }}
                onChange={handleToggleAll}
                className="w-4 h-4 rounded border-[var(--color-border)] accent-[var(--color-primary)] cursor-pointer"
              />
              全选
            </label>
          )}
          {selectedCount > 0 && (
            <Button size="sm" variant="danger" onClick={() => setBatchDeleteConfirmOpen(true)}>
              删除所选 ({selectedCount})
            </Button>
          )}
        </div>
        <Table
          columns={columns}
          data={filteredList}
          rowKey="proxyId"
          loading={loading}
          emptyText="暂无代理配置，点击上方按钮添加"
          sortColumn={sortColumn}
          sortOrder={sortOrder}
          onSort={({ column, order }) => {
            setSortColumn(column)
            setSortOrder(order)
          }}
        />
      </Card>

      <Modal open={editModalOpen} onClose={() => setEditModalOpen(false)} title={isAddMode ? '新增代理' : '编辑代理'} width="500px"
        footer={<><Button variant="secondary" onClick={() => setEditModalOpen(false)}>取消</Button><Button onClick={handleSaveProxy} loading={saving}>{isAddMode ? '添加' : '保存'}</Button></>}>
        <div className="space-y-4">
          <FormItem label="代理名称" required>
            <Input value={editForm.proxyName} onChange={e => setEditForm(prev => ({ ...prev, proxyName: e.target.value }))} placeholder="例如：美国节点" />
          </FormItem>
          <FormItem label="分组名称">
            <Input value={editForm.groupName} onChange={e => setEditForm(prev => ({ ...prev, groupName: e.target.value }))} placeholder="例如：亚马逊" list="edit-proxy-groups-datalist" />
            <datalist id="edit-proxy-groups-datalist">
              {groups.map(g => <option key={g} value={g} />)}
            </datalist>
          </FormItem>
          <FormItem label="代理类型" required>
            <select
              value={editForm.proxyType}
              onChange={e => setEditForm(prev => ({ ...prev, proxyType: e.target.value }))}
              className="w-full px-3 py-1.5 text-sm rounded-md border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]"
            >
              <option value="socks5">Socks5</option>
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
            </select>
          </FormItem>
          {editForm.proxyType !== 'direct' && (
            <>
              <div className="grid grid-cols-3 gap-3">
                <FormItem label="代理主机" required className="col-span-2">
                  <Input value={editForm.proxyHost} onChange={e => setEditForm(prev => ({ ...prev, proxyHost: e.target.value }))} placeholder="IP 或域名" />
                </FormItem>
                <FormItem label="端口" required>
                  <Input type="number" value={editForm.proxyPort} onChange={e => setEditForm(prev => ({ ...prev, proxyPort: e.target.value }))} placeholder="端口" />
                </FormItem>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <FormItem label="账号">
                  <Input value={editForm.proxyUser} onChange={e => setEditForm(prev => ({ ...prev, proxyUser: e.target.value }))} placeholder="可选" />
                </FormItem>
                <FormItem label="密码">
                  <Input type="password" value={editForm.proxyPass} onChange={e => setEditForm(prev => ({ ...prev, proxyPass: e.target.value }))} placeholder="可选" />
                </FormItem>
              </div>
            </>
          )}
        </div>
      </Modal>

      <Modal
        open={ipHealthDetailOpen}
        onClose={() => setIPHealthDetailOpen(false)}
        title="IP 分析原始返回"
        width="760px"
        footer={<Button variant="secondary" onClick={() => setIPHealthDetailOpen(false)}>关闭</Button>}
      >
        <div className="space-y-3">
          {currentIPHealthDetail && (
            <>
              <div className="text-xs text-[var(--color-text-muted)]">
                代理ID：{currentIPHealthDetail.proxyId} | 来源：{currentIPHealthDetail.source} | 时间：{currentIPHealthDetail.updatedAt}
              </div>
              {!currentIPHealthDetail.ok && (
                <div className="text-sm text-red-500">{currentIPHealthDetail.error || '检测失败'}</div>
              )}
              <pre className="max-h-[420px] overflow-auto text-xs leading-5 rounded-lg bg-[var(--color-bg-secondary)] border border-[var(--color-border)] p-3">
                {JSON.stringify(currentIPHealthDetail.rawData || {}, null, 2)}
              </pre>
            </>
          )}
        </div>
      </Modal>

      <ConfirmModal open={deleteConfirmOpen} onClose={() => setDeleteConfirmOpen(false)} onConfirm={handleDeleteConfirm}
        title="确认删除" content="确定要删除这个代理吗？此操作不可恢复。" confirmText="删除" danger />

      <ConfirmModal open={batchDeleteConfirmOpen} onClose={() => setBatchDeleteConfirmOpen(false)} onConfirm={handleBatchDeleteConfirm}
        title="批量删除" content={`确定要删除选中的 ${selectedCount} 个代理吗？此操作不可恢复。`} confirmText="删除" danger />
    </div>
  )
}
