import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Globe, Play, RotateCcw, Square } from 'lucide-react'
import { Badge, Button, Card, Input, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserProfile, BrowserTab } from '../types'
import {
  fetchBrowserProfiles,
  fetchBrowserTabs,
  openBrowserUrl,
  restartBrowserInstance,
  startBrowserInstance,
  stopBrowserInstance,
} from '../api'
import { CookieManagerCard } from '../components/CookieManagerCard'
import { SnapshotTab } from '../components/SnapshotTab'
import { resolveActionErrorMessage } from '../utils/actionErrors'

const statusVariant = (running: boolean) => (running ? 'success' : 'warning')

const formatTime = (value?: string) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN')
}

type TabKey = 'overview' | 'snapshot'

const TABS: { key: TabKey; label: string }[] = [
  { key: 'overview', label: '概览' },
  { key: 'snapshot', label: '快照管理' },
]

export function BrowserDetailPage() {
  const { id } = useParams()
  const [profile, setProfile] = useState<BrowserProfile | null>(null)
  const [tabs, setTabs] = useState<BrowserTab[]>([])
  const [targetUrl, setTargetUrl] = useState('https://example.com')
  const [activeTab, setActiveTab] = useState<TabKey>('overview')

  const loadProfile = async () => {
    const list = await fetchBrowserProfiles()
    const current = list.find(item => item.profileId === id) || null
    setProfile(current)
  }

  const loadTabs = async () => {
    if (!id) return
    const list = await fetchBrowserTabs(id)
    setTabs(list)
  }

  useEffect(() => { loadProfile() }, [id])
  useEffect(() => { loadTabs() }, [id])

  if (!profile) {
    return (
      <div className="flex items-center justify-center h-64 text-sm text-[var(--color-text-muted)]">
        暂无实例信息
      </div>
    )
  }

  const handleOpenUrl = async () => {
    await openBrowserUrl(profile.profileId, targetUrl)
    toast.success('已发送打开指令')
  }

  const handleStart = async () => {
    try {
      await startBrowserInstance(profile.profileId)
      toast.success('实例已启动')
    } catch (error: any) {
      toast.error(resolveActionErrorMessage(error, '实例启动失败'))
    } finally {
      loadProfile()
    }
  }

  const handleStop = async () => {
    try {
      await stopBrowserInstance(profile.profileId)
      toast.success('实例已停止')
    } catch (error: any) {
      toast.error(resolveActionErrorMessage(error, '实例停止失败'))
    } finally {
      loadProfile()
    }
  }

  const handleRestart = async () => {
    try {
      await restartBrowserInstance(profile.profileId)
      toast.success('实例已重启')
    } catch (error: any) {
      toast.error(resolveActionErrorMessage(error, '实例重启失败'))
    } finally {
      loadProfile()
    }
  }

  const tabsColumns: TableColumn<BrowserTab>[] = [
    { key: 'title', title: '标题' },
    { key: 'url', title: '地址' },
    {
      key: 'active',
      title: '状态',
      render: value => (
        <Badge variant={value ? 'success' : 'default'}>{value ? '当前' : '后台'}</Badge>
      ),
    },
  ]

  return (
    <div className="space-y-5 animate-fade-in">
      {/* 页头 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">实例详情</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">{profile.profileName}</p>
        </div>
        <div className="flex gap-2">
          <Link to={`/browser/edit/${profile.profileId}`}>
            <Button variant="secondary" size="sm">编辑配置</Button>
          </Link>
          <Link to="/browser/list">
            <Button variant="ghost" size="sm">返回列表</Button>
          </Link>
        </div>
      </div>

      {/* Tab 导航 */}
      <div className="flex border-b border-[var(--color-border)]">
        {TABS.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={[
              'px-4 py-2 text-sm font-medium transition-colors',
              activeTab === tab.key
                ? 'border-b-2 border-[var(--color-primary)] text-[var(--color-primary)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]',
            ].join(' ')}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* 概览 Tab */}
      {activeTab === 'overview' && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <Card title="运行信息" subtitle="实例运行状态与端口信息">
              <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
                <div className="flex justify-between">
                  <span>状态</span>
                  <Badge variant={statusVariant(profile.running)} dot>{profile.running ? '运行中' : '已停止'}</Badge>
                </div>
                <div className="flex justify-between">
                  <span>进程 PID</span>
                  <span>{profile.pid || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span>调试端口</span>
                  <span>{profile.debugPort || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span>最近启动</span>
                  <span>{formatTime(profile.lastStartAt)}</span>
                </div>
                <div className="flex justify-between">
                  <span>最近停止</span>
                  <span>{formatTime(profile.lastStopAt)}</span>
                </div>
              </div>
            </Card>

            <Card title="配置摘要" subtitle="指纹与启动参数">
              <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
                <div className="flex justify-between">
                  <span>用户数据目录</span>
                  <span>{profile.userDataDir}</span>
                </div>
                <div className="flex justify-between">
                  <span>内核</span>
                  <span>{profile.coreId || '默认'}</span>
                </div>
                <div className="flex justify-between">
                  <span>代理配置</span>
                  <span>{profile.proxyConfig || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span>指纹参数</span>
                  <span>{profile.fingerprintArgs?.length || 0} 项</span>
                </div>
                <div className="flex justify-between">
                  <span>启动参数</span>
                  <span>{profile.launchArgs?.length || 0} 项</span>
                </div>
                <div className="flex justify-between">
                  <span>标签</span>
                  <span>{profile.tags?.join(', ') || '-'}</span>
                </div>
              </div>
            </Card>
          </div>

          <Card title="快捷操作" subtitle="快速控制实例">
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" onClick={handleStart}>
                <Play className="w-4 h-4" />
                启动
              </Button>
              <Button size="sm" variant="secondary" onClick={handleStop}>
                <Square className="w-4 h-4" />
                停止
              </Button>
              <Button size="sm" variant="ghost" onClick={handleRestart}>
                <RotateCcw className="w-4 h-4" />
                重启
              </Button>
            </div>
          </Card>

          {profile.lastError && (
            <Card title="最近错误" subtitle="最近一次启动或运行失败原因">
              <div className="rounded-lg border border-[var(--color-error)]/30 bg-[var(--color-error-soft)] px-4 py-3 text-sm text-[var(--color-error)] whitespace-pre-line">
                {profile.lastError}
              </div>
            </Card>
          )}

          <Card title="打开地址" subtitle="向实例发送打开 URL 指令">
            <div className="flex flex-col md:flex-row gap-3">
              <Input value={targetUrl} onChange={e => setTargetUrl(e.target.value)} placeholder="请输入目标地址" />
              <Button onClick={handleOpenUrl}>
                <Globe className="w-4 h-4" />
                打开
              </Button>
            </div>
          </Card>

          <Card title="标签页列表" subtitle="当前实例标签页信息">
            <Table columns={tabsColumns} data={tabs} rowKey="tabId" />
          </Card>

          <CookieManagerCard
            profileId={profile.profileId}
            profileName={profile.profileName}
            running={profile.running}
          />
        </div>
      )}

      {/* 快照管理 Tab */}
      {activeTab === 'snapshot' && (
        <SnapshotTab profileId={profile.profileId} running={profile.running} />
      )}
    </div>
  )
}
