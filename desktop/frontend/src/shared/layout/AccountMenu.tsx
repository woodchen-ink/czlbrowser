import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import clsx from 'clsx'
import { CreditCard, ExternalLink, LogOut, User } from 'lucide-react'
import { useAuthStore } from '../../store/authStore'
import { useCloudStore } from '../../store/cloudStore'
import { cloudApi, formatBytes } from '../../modules/cloud/api'
import { askConfirm } from '../../modules/cloud/confirm'

// 侧栏底部的账号入口: 点头像弹出账号卡 (账号 / 当前套餐 / 同步容量 / 常用入口 / 退出登录)。
// 取代原来的「个人信息」独立页。

function Avatar({ src, size }: { src?: string; size: 'sm' | 'md' }) {
  const cls = size === 'sm' ? 'w-7 h-7 rounded-md' : 'w-10 h-10 rounded-lg'
  if (src) return <img src={src} alt="" className={clsx(cls, 'object-cover shrink-0')} />
  return (
    <span className={clsx(cls, 'shrink-0 flex items-center justify-center bg-[var(--color-button-primary)] text-[var(--color-button-primary-foreground)]')}>
      <User className={size === 'sm' ? 'w-3.5 h-3.5' : 'w-4 h-4'} />
    </span>
  )
}

/** 个人套餐一句话: 「个人同步 · 2027/01/01 到期」/「免费」 */
function usePlanSummary() {
  const billing = useCloudStore((s) => s.billing)
  const personal = billing?.entitlements?.personal
  if (!billing?.enabled) return { name: '免费版', detail: '', active: true }
  if (!personal || !personal.plan) return { name: '未订阅', detail: '同步需要订阅', active: false }
  const end = personal.periodEnd ? new Date(personal.periodEnd).toLocaleDateString('zh-CN') : ''
  return {
    name: personal.planName || personal.plan,
    detail: personal.active ? (end ? `${end} 到期` : '生效中') : '已到期',
    active: personal.active,
  }
}

export function AccountMenu({ collapsed }: { collapsed: boolean }) {
  const navigate = useNavigate()
  const { state: authState, logout } = useAuthStore()
  const user = authState.user
  const sync = useCloudStore((s) => s.sync)
  const orgCount = useCloudStore((s) => s.billing?.entitlements?.orgs?.length || 0)
  const plan = usePlanSummary()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [])

  const personalSpace = sync?.spaces?.find((s) => s.kind === 'personal')
  const used = personalSpace?.usedBytes || 0
  const quota = personalSpace?.quotaBytes || 0
  const pct = quota > 0 ? Math.min(100, Math.round((used / quota) * 100)) : 0
  const displayName = user?.nickname || user?.username || '用户'

  const go = (path: string) => {
    setOpen(false)
    navigate(path)
  }

  // 退出前提示还没上传的修改 (留在本机, 重新登录同一账号后继续上传)
  const handleLogout = async () => {
    setOpen(false)
    let pending = 0
    try {
      pending = await cloudApi.unpushedCount()
    } catch {
      pending = 0
    }
    if (pending > 0) {
      const ok = await askConfirm({
        title: '还有修改没上传',
        content: `云端同步里还有 ${pending} 项修改没上传完。退出后它们留在这台电脑，重新登录同一账号会继续上传。`,
        confirmText: '仍然退出',
      })
      if (!ok) return
    }
    await logout()
  }

  return (
    <div className={clsx('relative', !collapsed && 'flex-1 min-w-0')} ref={ref}>
      <button
        onClick={() => setOpen((v) => !v)}
        title={collapsed ? displayName : undefined}
        className={clsx(
          'flex items-center rounded-md transition-colors hover:bg-[var(--color-accent-muted)]',
          open && 'bg-[var(--color-accent-muted)]',
          collapsed ? 'w-8 h-8 justify-center' : 'w-full gap-2 px-1.5 py-1'
        )}
      >
        <Avatar src={user?.avatar} size="sm" />
        {!collapsed && (
          <span className="min-w-0 flex-1 text-left">
            <span className="block text-[13px] font-medium text-[var(--color-text-primary)] truncate leading-tight">{displayName}</span>
            <span className="block text-[11px] text-[var(--color-text-muted)] truncate leading-tight mt-0.5">{plan.name}</span>
          </span>
        )}
      </button>

      {open && (
        <div className={clsx('absolute w-72 z-50', collapsed ? 'left-full bottom-0 ml-2' : 'left-0 bottom-full mb-2')}>
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] shadow-[var(--shadow-lg)] animate-fade-in overflow-hidden">
          <div className="flex items-center gap-3 p-3">
            <Avatar src={user?.avatar} size="md" />
            <div className="min-w-0">
              <div className="text-sm font-medium text-[var(--color-text-primary)] truncate">{displayName}</div>
              <div className="text-xs text-[var(--color-text-muted)] truncate">{user?.email || (user?.username ? `@${user.username}` : '')}</div>
            </div>
          </div>

          <div className="px-3 py-2.5 border-t border-[var(--color-border-muted)] space-y-2">
            <div className="flex items-baseline justify-between gap-2 text-xs">
              <span className="text-[var(--color-text-muted)]">套餐</span>
              <span className="min-w-0 truncate text-right">
                <span className={clsx('font-medium', plan.active ? 'text-[var(--color-text-primary)]' : 'text-[var(--color-warning)]')}>{plan.name}</span>
                {plan.detail && <span className="text-[var(--color-text-muted)]"> · {plan.detail}</span>}
              </span>
            </div>
            {personalSpace && (
              <div className="space-y-1">
                <div className="flex items-baseline justify-between text-xs">
                  <span className="text-[var(--color-text-muted)]">同步容量</span>
                  <span className="text-[var(--color-text-secondary)] tabular-nums">
                    {formatBytes(used)}
                    {quota > 0 && ` / ${formatBytes(quota)}`}
                  </span>
                </div>
                {quota > 0 && (
                  <div className="h-1 rounded-full bg-[var(--color-bg-muted)] overflow-hidden">
                    <div
                      className={clsx('h-full', pct >= 90 ? 'bg-[var(--color-error)]' : 'bg-[var(--color-brand)]')}
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                )}
              </div>
            )}
            {orgCount > 0 && (
              <div className="flex items-baseline justify-between text-xs">
                <span className="text-[var(--color-text-muted)]">团队</span>
                <span className="text-[var(--color-text-secondary)] tabular-nums">{orgCount} 个</span>
              </div>
            )}
          </div>

          <div className="p-1 border-t border-[var(--color-border-muted)]">
            <MenuItem icon={<CreditCard className="w-4 h-4" />} label="套餐与订阅" onClick={() => go('/cloud?tab=plans')} />
            <MenuItem
              icon={<ExternalLink className="w-4 h-4" />}
              label="账号设置"
              onClick={() => {
                setOpen(false)
                window.open('https://connect.czl.net', '_blank')
              }}
            />
            <MenuItem icon={<LogOut className="w-4 h-4" />} label="退出登录" danger onClick={() => void handleLogout()} />
          </div>
          </div>
        </div>
      )}
    </div>
  )
}

function MenuItem({ icon, label, onClick, danger }: { icon: React.ReactNode; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'w-full flex items-center gap-2.5 h-8 px-2.5 rounded-md text-[13px] transition-colors',
        danger
          ? 'text-[var(--color-error)] hover:bg-[var(--color-error-soft)]'
          : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]'
      )}
    >
      {icon}
      {label}
    </button>
  )
}
