import { useEffect, useState } from 'react'
import { CheckCircle, Download, RefreshCw } from 'lucide-react'
import { Badge, Button, FormItem, Modal, Progress, Select, toast } from '../../../shared/components'
import { useEvents } from '../../../shared/hooks/useRuntimeEvents'
import { errorText } from '../../cloud/errors'
import { downloadBuiltinCore, fetchBrowserProxies, fetchBuiltinCores, refreshBuiltinCores, type BuiltinCoreSource } from '../api'
import type { BrowserProxy } from '../types'
import { CORE_KIND_LABEL } from '../utils/coreKind'

interface Props {
  open: boolean
  onClose: () => void
  /** 下载完成后刷新本机内核列表 */
  onInstalled: () => void
}

type DownloadProgress = { phase: string; progress: number; message: string }

// 下载走哪种网络; 与后端 BrowserBuiltinCoreDownload 的约定值
const VIA_SYSTEM = '__system__'
const VIA_DIRECT = '__direct__'

/** 下载内置内核 (清单来自 Chrome-exe 发布的 cores.json)。 */
export function CoreDownloadModal({ open, onClose, onInstalled }: Props) {
  const [list, setList] = useState<BuiltinCoreSource[] | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [via, setVia] = useState(VIA_SYSTEM)
  const [active, setActive] = useState<string | null>(null)
  const [progress, setProgress] = useState<DownloadProgress | null>(null)

  useEffect(() => {
    if (!open) return
    // 清单可能要走网络, 弹窗先出来再慢慢填
    fetchBuiltinCores().then(setList)
    fetchBrowserProxies().then((l) => setProxies(l.filter((p) => p.proxyId !== VIA_DIRECT)))
  }, [open])

  useEvents(
    {
      'download:progress': (p: DownloadProgress) => {
        setProgress(p)
        if (p.phase === 'done') {
          toast.success(p.message || '内核已安装')
          setActive(null)
          setProgress(null)
          onInstalled()
          fetchBuiltinCores().then(setList)
        } else if (p.phase === 'error') {
          toast.error(p.message || '下载失败')
          setActive(null)
          setProgress(null)
        }
      },
    },
    [onInstalled],
  )

  const busy = active !== null

  const start = async (id: string) => {
    setActive(id)
    setProgress({ phase: 'starting', progress: 0, message: '准备下载...' })
    const proxy = via === VIA_SYSTEM || via === VIA_DIRECT ? via : (proxies.find((p) => p.proxyId === via)?.proxyConfig ?? VIA_SYSTEM)
    try {
      await downloadBuiltinCore(id, proxy)
    } catch (e) {
      toast.error(`下载失败: ${errorText(e)}`)
      setActive(null)
      setProgress(null)
    }
  }

  const refresh = async () => {
    setRefreshing(true)
    try {
      setList(await refreshBuiltinCores())
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <Modal open={open} onClose={() => (busy ? toast.warning('下载中, 完成后再关闭') : onClose())} title="下载内核" width="540px">
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <span className="text-xs text-[var(--color-text-muted)]">列表每天自动更新到最新版本</span>
          <Button size="sm" variant="ghost" onClick={refresh} loading={refreshing} disabled={busy}>
            <RefreshCw className="h-3.5 w-3.5" />
            刷新
          </Button>
        </div>

        <ul className="divide-y divide-[var(--color-border-muted)] rounded-lg border border-[var(--color-border-default)]">
          {list === null && <li className="px-3 py-4 text-xs text-[var(--color-text-muted)]">正在获取内核列表...</li>}
          {list?.length === 0 && <li className="px-3 py-4 text-xs text-[var(--color-text-muted)]">暂时拿不到内核列表, 稍后点刷新重试</li>}
          {list?.map((c) => (
            <li key={c.id} className="flex items-center gap-3 px-3 py-2.5">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-[13px] font-medium text-[var(--color-text-primary)]">{c.name}</span>
                  <span className="font-mono text-[11px] text-[var(--color-text-muted)]">{c.version}</span>
                  <Badge size="sm">{CORE_KIND_LABEL[c.kind] ?? c.kind}</Badge>
                </div>
                {c.description && <p className="mt-0.5 text-xs text-[var(--color-text-muted)]">{c.description}</p>}
              </div>
              {c.installed ? (
                <span className="inline-flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                  <CheckCircle className="h-3.5 w-3.5" />
                  已安装
                </span>
              ) : (
                <Button size="sm" onClick={() => start(c.id)} loading={active === c.id} disabled={busy}>
                  <Download className="h-3.5 w-3.5" />
                  下载
                </Button>
              )}
            </li>
          ))}
        </ul>

        <FormItem label="下载使用的网络">
          <Select
            value={via}
            disabled={busy}
            onChange={(e) => setVia(e.target.value)}
            options={[
              { value: VIA_SYSTEM, label: '系统代理' },
              { value: VIA_DIRECT, label: '直连' },
              ...proxies.map((p) => ({ value: p.proxyId, label: `代理池: ${p.proxyName}` })),
            ]}
          />
        </FormItem>

        {progress && (
          <div className="space-y-1.5">
            <div className="flex justify-between text-xs">
              <span className="text-[var(--color-text-secondary)]">{progress.message}</span>
              <span className="tabular-nums text-[var(--color-text-muted)]">{progress.progress}%</span>
            </div>
            <Progress percent={Math.min(100, Math.max(0, progress.progress))} size="sm" />
          </div>
        )}
      </div>
    </Modal>
  )
}
