import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, ExternalLink, Pencil, Play, RefreshCw, RotateCcw, Square } from 'lucide-react'
import { Badge, Button, Card, Input, Segmented, Table, toast, type TableColumn } from '../../../shared/components'
import { useEvents } from '../../../shared/hooks/useRuntimeEvents'
import {
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserTabs,
  fetchCoreExtendedInfo,
  openBrowserUrl,
  restartBrowserInstance,
  startBrowserInstance,
  stopBrowserInstance,
} from '../api'
import { CookieManagerCard } from '../components/CookieManagerCard'
import { SnapshotTab } from '../components/SnapshotTab'
import type { BrowserCore, BrowserCoreExtended, BrowserProfile, BrowserProxy, BrowserTab } from '../types'
import { actionError } from '../utils/actionErrors'
import { coreDisplay, platformOf, proxyDisplay } from '../utils/instanceDisplay'

type View = 'overview' | 'snapshot'

const VIEWS: { value: View; label: string }[] = [
  { value: 'overview', label: '概览' },
  { value: 'snapshot', label: '快照' },
]

const when = (s?: string) => {
  const d = s ? new Date(s) : null
  return d && !Number.isNaN(d.getTime()) ? d.toLocaleString('zh-CN') : '-'
}

/** 隐去代理地址里的账号密码 */
const hideCredentials = (url: string) => url.replace(/\/\/[^@/]*@/, '//***@')

const tabColumns: TableColumn<BrowserTab>[] = [
  { key: 'title', title: '标题', render: (v: string) => <span className="line-clamp-1">{v || '(无标题)'}</span> },
  { key: 'url', title: '地址', render: (v: string) => <span className="line-clamp-1 font-mono text-xs">{v}</span> },
  { key: 'active', title: '', width: 60, render: (v: boolean) => (v ? <Badge variant="success" size="sm">当前</Badge> : null) },
]

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1.5">
      <dt className="shrink-0 text-[var(--color-text-muted)]">{label}</dt>
      <dd className="min-w-0 truncate text-right text-[var(--color-text-primary)]">{children}</dd>
    </div>
  )
}

/** 实例详情: 运行状态、配置摘要、标签页、Cookie 与快照。 */
export function BrowserDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [profile, setProfile] = useState<BrowserProfile | null | undefined>(undefined)
  const [view, setView] = useState<View>('overview')
  const [tabs, setTabs] = useState<BrowserTab[]>([])
  const [url, setUrl] = useState('https://')
  const [busy, setBusy] = useState(false)
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [coreExt, setCoreExt] = useState(new Map<string, BrowserCoreExtended>())
  const [proxies, setProxies] = useState<BrowserProxy[]>([])

  const reload = useCallback(async () => {
    const all = await fetchBrowserProfiles()
    const p = all.find((x) => x.profileId === id) ?? null
    setProfile(p)
    setTabs(p?.running ? await fetchBrowserTabs(id) : [])
  }, [id])

  useEffect(() => {
    reload()
    fetchBrowserCores().then(setCores)
    fetchBrowserProxies().then(setProxies)
    fetchCoreExtendedInfo().then((l) => setCoreExt(new Map(l.map((c) => [c.coreId, c]))))
  }, [reload])

  // 在别处启动 / 停止 / 崩溃时同步状态; started / stopped 只带 id, crashed 带对象
  const onInstance = (d: string | { profileId?: string }) => (typeof d === 'string' ? d : d?.profileId) === id && reload()
  useEvents({ 'browser:instance:started': onInstance, 'browser:instance:stopped': onInstance, 'browser:instance:crashed': onInstance }, [id, reload])

  if (profile === undefined) return null
  if (profile === null) {
    return <div className="flex h-64 items-center justify-center text-sm text-[var(--color-text-muted)]">实例不存在或已删除</div>
  }

  const act = async (fn: (id: string) => Promise<unknown>, label: string) => {
    setBusy(true)
    try {
      await fn(profile.profileId)
    } catch (e) {
      toast.error(actionError(e, `${label}失败`))
    } finally {
      setBusy(false)
      reload()
    }
  }

  const openUrl = async () => {
    try {
      if (await openBrowserUrl(profile.profileId, url.trim())) setTimeout(reload, 800)
      else toast.error('打开失败')
    } catch (e) {
      toast.error(actionError(e, '打开失败'))
    }
  }

  const core = coreDisplay(profile, cores, coreExt)
  const proxy = proxyDisplay(profile, proxies)
  const proxyName = !profile.proxyId && profile.proxyConfig ? hideCredentials(profile.proxyConfig) : proxy.name

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <button
          onClick={() => navigate('/browser/list')}
          aria-label="返回实例列表"
          className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h1 className="truncate text-base font-semibold text-[var(--color-text-primary)]">{profile.profileName}</h1>
        <Badge variant={profile.running ? 'success' : 'default'} dot size="sm">
          {profile.running ? '运行中' : '已停止'}
        </Badge>
        <Segmented className="ml-3" size="sm" value={view} onChange={setView} options={VIEWS} />
        <div className="ml-auto flex gap-1.5">
          {profile.running ? (
            <>
              <Button size="sm" variant="secondary" disabled={busy} onClick={() => act(restartBrowserInstance, '重启')}>
                <RotateCcw className="h-3.5 w-3.5" />
                重启
              </Button>
              <Button size="sm" variant="secondary" disabled={busy} onClick={() => act(stopBrowserInstance, '停止')}>
                <Square className="h-3.5 w-3.5" />
                停止
              </Button>
            </>
          ) : (
            <Button size="sm" disabled={busy} onClick={() => act(startBrowserInstance, '启动')}>
              <Play className="h-3.5 w-3.5" />
              启动
            </Button>
          )}
          <Button size="sm" variant="secondary" onClick={() => navigate(`/browser/edit/${profile.profileId}`)}>
            <Pencil className="h-3.5 w-3.5" />
            编辑
          </Button>
        </div>
      </div>

      {view === 'snapshot' ? (
        <SnapshotTab profileId={profile.profileId} running={profile.running} />
      ) : (
        <>
          {profile.lastError && (
            <div className="whitespace-pre-line rounded-lg border border-[var(--color-error)]/30 bg-[var(--color-error-soft)] px-4 py-3 text-sm text-[var(--color-error)]">
              <span className="font-medium">最近错误: </span>
              {profile.lastError}
            </div>
          )}

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Card title="运行">
              <dl className="text-[13px]">
                <Row label="进程 PID">{profile.pid || '-'}</Row>
                <Row label="调试端口">{profile.debugPort || '-'}</Row>
                <Row label="上次启动">{when(profile.lastStartAt)}</Row>
                <Row label="上次停止">{when(profile.lastStopAt)}</Row>
                <Row label="数据目录">
                  <span className="font-mono text-xs" title={profile.userDataDir}>
                    {profile.userDataDir}
                  </span>
                </Row>
              </dl>
            </Card>
            <Card title="配置">
              <dl className="text-[13px]">
                <Row label="内核">{[core.name, core.version, core.followsDefault && '跟随默认'].filter(Boolean).join(' · ')}</Row>
                <Row label="代理">{[proxyName, proxy.region].filter(Boolean).join(' · ') || '直连'}</Row>
                <Row label="平台">{platformOf(profile.fingerprintArgs) || '未设置'}</Row>
                <Row label="参数">
                  指纹 {profile.fingerprintArgs?.length ?? 0} 项 · 启动 {profile.launchArgs?.length ?? 0} 项
                </Row>
                <Row label="标签">{profile.tags?.length ? profile.tags.join(', ') : '-'}</Row>
              </dl>
            </Card>
          </div>

          <Card
            title="标签页"
            subtitle={profile.running ? `${tabs.length} 个` : '实例未运行'}
            actions={
              profile.running && (
                <Button size="sm" variant="ghost" onClick={reload}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  刷新
                </Button>
              )
            }
          >
            {profile.running && (
              <div className="mb-3 flex gap-2">
                <Input value={url} onChange={(e) => setUrl(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && openUrl()} placeholder="https://" className="w-96" />
                <Button variant="secondary" onClick={openUrl} disabled={!/^https?:\/\/.+/.test(url.trim())}>
                  <ExternalLink className="h-3.5 w-3.5" />
                  新标签页打开
                </Button>
              </div>
            )}
            <Table columns={tabColumns} data={tabs} rowKey="tabId" emptyText={profile.running ? '没有打开的标签页' : '启动实例后显示'} />
          </Card>

          <CookieManagerCard profileId={profile.profileId} profileName={profile.profileName} running={profile.running} />
        </>
      )}
    </div>
  )
}
