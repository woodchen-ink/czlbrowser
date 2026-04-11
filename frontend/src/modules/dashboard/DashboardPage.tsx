import { useEffect, useState, type ReactNode } from 'react'
import { Monitor, Play, Cpu, Globe } from 'lucide-react'
import { Card } from '../../shared/components'
import { fetchDashboardStats } from './api'
import type { DashboardStats } from './types'

interface StatCardProps {
  title: string
  value: string | number
  icon: ReactNode
  tone: 'info' | 'success' | 'accent' | 'warning'
}

const statToneStyles: Record<StatCardProps['tone'], string> = {
  info: 'bg-[var(--color-info-soft)] text-[var(--color-info)]',
  success: 'bg-[var(--color-success-soft)] text-[var(--color-success)]',
  accent: 'bg-[var(--color-accent-soft)] text-[var(--color-accent)]',
  warning: 'bg-[var(--color-warning-soft)] text-[var(--color-warning)]',
}

function StatCard({ title, value, icon, tone }: StatCardProps) {
  return (
    <div className="rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-card)] p-5 shadow-[var(--shadow-sm)]">
      <div className="mb-3 flex items-center justify-between">
        <span className="text-sm text-[var(--color-text-muted)]">{title}</span>
        <div className={`flex h-10 w-10 items-center justify-center rounded-xl ${statToneStyles[tone]}`}>
          {icon}
        </div>
      </div>
      <div className="text-2xl font-semibold text-[var(--color-text-primary)]">{value}</div>
    </div>
  )
}

export function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats>({
    totalInstances: 0,
    runningInstances: 0,
    proxyCount: 0,
    coreCount: 0,
    memUsedMB: 0,
    maxProfileLimit: 3,
  })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      setStats(await fetchDashboardStats())
    } finally {
      setLoading(false)
    }
  }

  const v = (n: number) => loading ? '-' : n.toString()

  return (
    <div className="space-y-6 animate-fade-in">
      <section className="overflow-hidden rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-sm)]">
        <div className="border-b border-[var(--color-border-muted)] px-6 py-5">
          <div className="flex flex-wrap items-end justify-between gap-5">
            <div>
              <h1 className="text-2xl font-semibold tracking-tight text-[var(--color-text-primary)]">控制台</h1>
              <p className="mt-1 text-sm text-[var(--color-text-muted)]">浏览器指纹管理平台概览</p>
            </div>
            <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-4 py-3">
              <div className="text-xs text-[var(--color-text-muted)]">实例运行占比</div>
              <div className="mt-1 text-lg font-semibold text-[var(--color-text-primary)]">
                {loading ? '-' : `${stats.runningInstances} / ${stats.totalInstances}`}
              </div>
            </div>
          </div>
        </div>
      </section>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="实例总数"
          value={v(stats.totalInstances)}
          icon={<Monitor className="w-4 h-4" />}
          tone="info"
        />
        <StatCard
          title="运行中"
          value={v(stats.runningInstances)}
          icon={<Play className="w-4 h-4" />}
          tone="success"
        />
        <StatCard
          title="代理节点"
          value={v(stats.proxyCount)}
          icon={<Globe className="w-4 h-4" />}
          tone="accent"
        />
        <StatCard
          title="内核版本"
          value={v(stats.coreCount)}
          icon={<Cpu className="w-4 h-4" />}
          tone="warning"
        />
      </div>

      <div>
        <Card title="系统信息">
          <div className="space-y-1">
            {[
              { label: '系统版本', value: '1.0.0' },
              { label: '客户端类型', value: '本地桌面客户端' },
              { label: '数据模式', value: '本地保存' },
              { label: '内存占用', value: loading ? '-' : `${stats.memUsedMB} MB` },
              { label: '实例运行', value: loading ? '-' : `${stats.runningInstances} / ${stats.totalInstances}` },
            ].map(item => (
              <div
                key={item.label}
                className="flex justify-between items-center py-3 border-b border-[var(--color-border-muted)] last:border-0"
              >
                <span className="text-sm text-[var(--color-text-muted)]">{item.label}</span>
                <span className="text-sm font-medium text-[var(--color-text-primary)]">{item.value}</span>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  )
}
