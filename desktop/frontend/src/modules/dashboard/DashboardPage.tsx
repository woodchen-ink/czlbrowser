import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import clsx from 'clsx'
import { AlertCircle, AlertTriangle, ArrowRight, CheckCircle2, Info, Play, Plus, RefreshCw, Square } from 'lucide-react'
import { Button, toast } from '../../shared/components'
import { useCloudStore } from '../../store/cloudStore'
import { spaceLabel, formatBytes } from '../cloud/api'
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'
import { startBrowserInstance, stopBrowserInstance } from '../browser/api'
import { actionError } from '../browser/utils/actionErrors'
import { relativeTime } from '../browser/utils/instanceDisplay'
import { fetchDashboardOverview, type DashboardOverview } from './api'

// 控制台: 打开即回答「现在正常吗、要处理什么、最近在用什么」。
// 顶部是需要处理的事项 (正常时一行安静的提示), 左侧主区是实例活动, 右侧是代理 / 内核 / 同步的健康状态。

const DAY_LABELS = ['6 天前', '5 天前', '4 天前', '3 天前', '前天', '昨天', '今天']

function Panel({ title, action, children, className }: { title: string; action?: React.ReactNode; children: React.ReactNode; className?: string }) {
  return (
    <section className={clsx('rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]', className)}>
      <header className="flex items-center justify-between h-10 px-4 border-b border-[var(--color-border-muted)]">
        <h2 className="text-[13px] font-medium text-[var(--color-text-primary)]">{title}</h2>
        {action}
      </header>
      <div className="px-4 py-3">{children}</div>
    </section>
  )
}

function Metric({ label, value, tone }: { label: string; value: React.ReactNode; tone?: 'error' | 'warning' | 'success' }) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-1">
      <span className="text-xs text-[var(--color-text-muted)]">{label}</span>
      <span
        className={clsx(
          'text-[13px] tabular-nums',
          tone === 'error' ? 'text-[var(--color-error)] font-medium' : tone === 'warning' ? 'text-[var(--color-warning)] font-medium' : tone === 'success' ? 'text-[var(--color-success)]' : 'text-[var(--color-text-primary)]'
        )}
      >
        {value}
      </span>
    </div>
  )
}

function MoreLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link to={to} className="inline-flex items-center gap-0.5 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]">
      {children}
      <ArrowRight className="w-3 h-3" />
    </Link>
  )
}

export function DashboardPage() {
  const navigate = useNavigate()
  const [data, setData] = useState<DashboardOverview | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<string | null>(null)
  const changeTick = useCloudStore((s) => s.changeTick)
  const sync = useCloudStore((s) => s.sync)
  const current = useCloudStore((s) => s.currentSpace())

  const load = useCallback(async () => {
    try {
      setData(await fetchDashboardOverview())
      setError('')
    } catch (err: any) {
      setError(err?.message || '读取失败')
    }
  }, [])

  useEffect(() => {
    void load()
    const events = ['browser:instance:started', 'browser:instance:stopped', 'browser:instance:crashed']
    events.forEach((e) => EventsOn(e, () => void load()))
    return () => events.forEach((e) => EventsOff(e))
  }, [load])

  useEffect(() => {
    if (changeTick > 0) void load()
  }, [changeTick, load])

  const toggleRun = async (id: string, running: boolean) => {
    setBusy(id)
    try {
      if (running) await stopBrowserInstance(id)
      else await startBrowserInstance(id)
    } catch (err) {
      toast.error(actionError(err, running ? '停止失败' : '启动失败'))
    } finally {
      setBusy(null)
      void load()
    }
  }

  if (error && !data) {
    return (
      <div className="h-full flex flex-col items-center justify-center gap-3 text-center">
        <AlertCircle className="w-6 h-6 text-[var(--color-error)]" />
        <p className="text-[13px] text-[var(--color-text-secondary)]">{error}</p>
        <Button size="sm" variant="secondary" onClick={() => void load()}>
          重试
        </Button>
      </div>
    )
  }

  const ins = data?.instances
  const px = data?.proxies
  const cores = data?.cores
  const maxStarts = Math.max(1, ...(ins?.starts7d || [0]))
  const proxyTotal = px?.total || 0
  const pct = (n: number) => (proxyTotal > 0 ? `${(n / proxyTotal) * 100}%` : '0%')
  const pendingUpload = current?.pending || 0

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <h1 className="text-base font-semibold text-[var(--color-text-primary)]">控制台</h1>
        <span className="text-xs text-[var(--color-text-muted)]">{current ? spaceLabel(current) : '本机'}</span>
        <div className="ml-auto flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => void load()} title="刷新">
            <RefreshCw className="w-3.5 h-3.5" />
          </Button>
          <Button size="sm" onClick={() => navigate('/browser/edit/new')}>
            <Plus className="w-3.5 h-3.5" />
            新建实例
          </Button>
        </div>
      </div>

      {/* 需要处理的事项; 正常时一行安静的提示 */}
      <div className="mb-4">
        {!data ? (
          <div className="h-9 rounded-md bg-[var(--color-bg-muted)] animate-pulse" />
        ) : data.attention.length === 0 ? (
          <div className="flex items-center gap-2 h-9 px-3 rounded-md text-xs text-[var(--color-text-muted)]">
            <CheckCircle2 className="w-3.5 h-3.5 text-[var(--color-success)]" />
            一切正常，没有需要处理的事项
          </div>
        ) : (
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] divide-y divide-[var(--color-border-muted)]">
            {data.attention.map((a) => {
              const Icon = a.level === 'error' ? AlertCircle : a.level === 'warning' ? AlertTriangle : Info
              return (
                <Link key={a.text} to={a.link} className="group flex items-center gap-2.5 h-10 px-4 hover:bg-[var(--color-accent-muted)]">
                  <Icon
                    className={clsx(
                      'w-4 h-4 shrink-0',
                      a.level === 'error' ? 'text-[var(--color-error)]' : a.level === 'warning' ? 'text-[var(--color-warning)]' : 'text-[var(--color-text-muted)]'
                    )}
                  />
                  <span className="flex-1 text-[13px] text-[var(--color-text-primary)]">{a.text}</span>
                  <span className="text-xs text-[var(--color-text-muted)] group-hover:text-[var(--color-text-primary)] inline-flex items-center gap-0.5">
                    去处理 <ArrowRight className="w-3 h-3" />
                  </span>
                </Link>
              )
            })}
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_20rem] 2xl:grid-cols-[minmax(0,1fr)_24rem] gap-4 items-start">
        {/* 主区: 实例 */}
        <div className="min-w-0 space-y-4">
          <Panel title="实例" action={<MoreLink to="/browser/list">全部实例</MoreLink>}>
            <div className="flex items-end gap-8">
              <div>
                <div className="flex items-baseline gap-1.5">
                  <span className="text-3xl font-semibold tabular-nums text-[var(--color-text-primary)]">{ins?.running ?? '—'}</span>
                  <span className="text-sm text-[var(--color-text-muted)] tabular-nums">/ {ins?.total ?? '—'}</span>
                </div>
                <p className="text-xs text-[var(--color-text-muted)] mt-1">正在运行</p>
              </div>
              <div className="grid grid-cols-3 gap-6 text-xs">
                <div>
                  <div className="text-[15px] font-medium tabular-nums text-[var(--color-text-primary)]">{ins?.active7d ?? '—'}</div>
                  <div className="text-[var(--color-text-muted)] mt-0.5">近 7 天用过</div>
                </div>
                <div>
                  <div className="text-[15px] font-medium tabular-nums text-[var(--color-text-primary)]">{ins?.never ?? '—'}</div>
                  <div className="text-[var(--color-text-muted)] mt-0.5">从未启动</div>
                </div>
                <div>
                  <div className={clsx('text-[15px] font-medium tabular-nums', ins?.errored ? 'text-[var(--color-error)]' : 'text-[var(--color-text-primary)]')}>{ins?.errored ?? '—'}</div>
                  <div className="text-[var(--color-text-muted)] mt-0.5">异常退出</div>
                </div>
              </div>
              {/* 近 7 天每天启动过的实例数 */}
              <div className="ml-auto flex-1 min-w-[8rem] max-w-md">
                <div className="flex items-end gap-1.5 h-12">
                  {(ins?.starts7d || Array(7).fill(0)).map((n, i) => (
                    <div key={i} className="flex-1 flex flex-col justify-end h-full" title={`${DAY_LABELS[i]}：${n} 个实例`}>
                      <div
                        className={clsx('rounded-sm', n > 0 ? 'bg-[var(--color-chart-1)]' : 'bg-[var(--color-bg-muted)]')}
                        style={{ height: n > 0 ? `${Math.max(12, (n / maxStarts) * 100)}%` : '3px' }}
                      />
                    </div>
                  ))}
                </div>
                <p className="text-[11px] text-[var(--color-text-muted)] mt-1 text-right">近 7 天启动</p>
              </div>
            </div>
          </Panel>

          <Panel title="最近使用">
            {!data ? (
              <div className="space-y-2">
                {[0, 1, 2].map((i) => (
                  <div key={i} className="h-8 rounded bg-[var(--color-bg-muted)] animate-pulse" />
                ))}
              </div>
            ) : data.recent.length === 0 ? (
              <div className="py-6 text-center">
                <p className="text-[13px] text-[var(--color-text-secondary)]">{ins?.total ? '实例都还没启动过' : '还没有实例'}</p>
                {!ins?.total && (
                  <Button className="mt-3" size="sm" onClick={() => navigate('/browser/edit/new')}>
                    <Plus className="w-3.5 h-3.5" />
                    新建第一个实例
                  </Button>
                )}
              </div>
            ) : (
              <ul className="-my-1.5 divide-y divide-[var(--color-border-muted)]">
                {data.recent.map((r) => (
                  <li key={r.profileId} className="flex items-center gap-3 py-1.5">
                    <span
                      className={clsx(
                        'w-1.5 h-1.5 rounded-full shrink-0',
                        r.running ? 'bg-[var(--color-success)]' : r.lastError ? 'bg-[var(--color-error)]' : 'bg-[var(--color-border-strong)]'
                      )}
                    />
                    <Link to={`/browser/detail/${r.profileId}`} className="min-w-0 flex-1 truncate text-[13px] text-[var(--color-text-primary)] hover:underline underline-offset-2">
                      {r.profileName}
                    </Link>
                    <span className="w-36 truncate text-xs text-[var(--color-text-muted)]" title={r.proxyName}>
                      {r.proxyName}
                    </span>
                    <span className="w-20 text-right text-xs text-[var(--color-text-muted)] tabular-nums">{r.running ? '运行中' : relativeTime(r.lastStartAt)}</span>
                    <Button
                      size="sm"
                      variant={r.running ? 'secondary' : 'primary'}
                      className="w-14"
                      loading={busy === r.profileId}
                      onClick={() => void toggleRun(r.profileId, r.running)}
                    >
                      {busy !== r.profileId && (r.running ? <Square className="w-3 h-3 fill-current" /> : <Play className="w-3 h-3 fill-current" />)}
                      {r.running ? '停止' : '启动'}
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
        </div>

        {/* 侧栏: 代理 / 内核 / 同步 */}
        <div className="space-y-4">
          <Panel title="代理" action={<MoreLink to="/browser/proxy-pool">管理</MoreLink>}>
            {proxyTotal === 0 ? (
              <p className="text-xs text-[var(--color-text-muted)]">还没有代理，实例会直连上网</p>
            ) : (
              <>
                <div className="flex h-1.5 rounded-full overflow-hidden bg-[var(--color-bg-muted)] mb-2" title="可用 / 失败 / 未测">
                  <div className="bg-[var(--color-success)]" style={{ width: pct(px?.ok || 0) }} />
                  <div className="bg-[var(--color-error)]" style={{ width: pct(px?.failed || 0) }} />
                </div>
                <Metric label="可用" value={`${px?.ok} / ${proxyTotal}`} tone={px?.ok === proxyTotal ? 'success' : undefined} />
                <Metric label="测速失败" value={px?.failed} tone={px?.failed ? 'error' : undefined} />
                <Metric label="未测速" value={px?.untested} />
                <Metric label="平均延迟" value={px?.avgLatencyMs ? `${px.avgLatencyMs} ms` : '—'} />
              </>
            )}
          </Panel>

          <Panel title="内核" action={<MoreLink to="/browser/cores">管理</MoreLink>}>
            <Metric
              label="默认内核"
              value={cores?.defaultName ? <span title={cores.defaultVersion}>{cores.defaultName}</span> : <span className="text-[var(--color-error)]">未设置</span>}
            />
            {cores?.defaultVersion && <Metric label="版本" value={<span className="font-mono text-xs">{cores.defaultVersion}</span>} />}
            <Metric label="跟随默认的实例" value={cores?.followDefault ?? '—'} />
            <Metric label="已安装" value={cores?.total ?? '—'} />
            {!!cores?.missing && <Metric label="文件缺失" value={cores.missing} tone="error" />}
          </Panel>

          <Panel title="云端同步" action={<MoreLink to="/cloud">{current ? '管理' : '了解'}</MoreLink>}>
            {!current ? (
              <p className="text-xs text-[var(--color-text-muted)] leading-5">
                当前在本机空间，数据只在这台电脑。开启云端同步后可以多台电脑共用实例与登录状态，也能和团队协作。
              </p>
            ) : (
              <>
                <Metric label="空间" value={spaceLabel(current)} />
                <Metric
                  label="状态"
                  value={
                    sync?.keys === 'locked'
                      ? '未解锁'
                      : current.readOnly
                        ? '只读'
                        : current.status === 'error'
                          ? '同步出错'
                          : pendingUpload > 0
                            ? `${pendingUpload} 项待上传`
                            : '已同步'
                  }
                  tone={sync?.keys === 'locked' || current.status === 'error' ? 'warning' : undefined}
                />
                {current.quotaBytes > 0 && <Metric label="容量" value={`${formatBytes(current.usedBytes)} / ${formatBytes(current.quotaBytes)}`} />}
              </>
            )}
          </Panel>

        </div>
      </div>
    </div>
  )
}
