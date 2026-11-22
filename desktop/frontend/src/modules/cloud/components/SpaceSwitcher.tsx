import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import clsx from 'clsx'
import { Check, ChevronsUpDown, Cloud, CloudOff, HardDrive, Lock, Settings2, User, Users } from 'lucide-react'
import { useCloudStore } from '../../../store/cloudStore'
import { LOCAL_SPACE, spaceLabel, type SpaceInfo } from '../api'

// 侧栏顶部的空间切换: 本机 / 我的同步 / 各团队。实例、代理、分组、书签都跟着当前空间走。

function SpaceIcon({ sp, className }: { sp: SpaceInfo | null; className?: string }) {
  if (!sp) return <HardDrive className={className} />
  return sp.kind === 'personal' ? <User className={className} /> : <Users className={className} />
}

function statusHint(sp: SpaceInfo, keys: string): string {
  if (sp.removed) return '已被移出，只读副本'
  if (keys === 'not_enrolled') return '未开启加密，只读'
  if (keys === 'locked') return '未解锁，只读'
  if (sp.keyPending) return '等待授予密钥'
  if (sp.readOnly) return '只读'
  if (sp.pending > 0) return `${sp.pending} 项待上传`
  if (sp.status === 'error') return '同步出错'
  return ''
}

export function SpaceSwitcher({ collapsed }: { collapsed: boolean }) {
  const navigate = useNavigate()
  const sync = useCloudStore((s) => s.sync)
  const switchSpace = useCloudStore((s) => s.switchSpace)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [])

  const spaces = sync?.spaces || []
  const currentKey = sync?.current || LOCAL_SPACE
  const current = spaces.find((s) => s.key === currentKey) || null
  const keys = sync?.keys || 'unknown'
  const offline = !!sync?.signedIn && !sync?.available && spaces.length > 0

  const pick = async (key: string) => {
    setOpen(false)
    if (key !== currentKey) await switchSpace(key)
  }

  const item = (key: string, sp: SpaceInfo | null) => {
    const active = key === currentKey
    const hint = sp ? statusHint(sp, keys) : '只在这台电脑，不上传'
    return (
      <button
        key={key}
        onClick={() => void pick(key)}
        className={clsx(
          'w-full flex items-center gap-2.5 px-2.5 py-1.5 text-left rounded-md transition-colors',
          active ? 'bg-[var(--color-brand-soft)]' : 'hover:bg-[var(--color-accent-muted)]'
        )}
      >
        <SpaceIcon sp={sp} className="w-4 h-4 shrink-0 text-[var(--color-text-secondary)]" />
        <span className="min-w-0 flex-1">
          <span className="block text-[13px] text-[var(--color-text-primary)] truncate">{sp ? spaceLabel(sp) : '本机'}</span>
          {hint && <span className="block text-xs text-[var(--color-text-muted)] truncate">{hint}</span>}
        </span>
        {active && <Check className="w-4 h-4 shrink-0 text-[var(--color-brand-strong)]" />}
      </button>
    )
  }

  return (
    <div className={clsx('relative pb-1', collapsed ? 'px-2' : 'px-2.5')} ref={ref}>
      <button
        onClick={() => setOpen((v) => !v)}
        title={`数据空间：${current ? spaceLabel(current) : '本机'}`}
        className={clsx(
          'flex items-center h-8 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]',
          'hover:border-[var(--color-border-strong)] transition-colors',
          collapsed ? 'justify-center w-9 mx-auto' : 'w-full gap-2 px-2'
        )}
      >
        <span className="relative shrink-0">
          <SpaceIcon sp={current} className="w-4 h-4 text-[var(--color-text-secondary)]" />
          {current && (current.pending > 0 || offline) && (
            <span className="absolute -top-1 -right-1 w-2 h-2 rounded-full bg-[var(--color-warning)]" />
          )}
        </span>
        {!collapsed && (
          <>
            <span className="min-w-0 flex-1 text-left text-[13px] font-medium text-[var(--color-text-primary)] truncate">
              {current ? spaceLabel(current) : '本机'}
            </span>
            {current?.readOnly && <Lock className="w-3.5 h-3.5 shrink-0 text-[var(--color-text-muted)]" />}
            {offline && <CloudOff className="w-3.5 h-3.5 shrink-0 text-[var(--color-text-muted)]" />}
            <ChevronsUpDown className="w-3.5 h-3.5 shrink-0 text-[var(--color-text-muted)]" />
          </>
        )}
      </button>

      {open && (
        <div
          className={clsx(
            'absolute z-50 w-64 p-1 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] shadow-[var(--shadow-lg)] animate-fade-in',
            collapsed ? 'left-full top-0 ml-2' : 'left-2.5 top-full mt-1'
          )}
        >
          {item(LOCAL_SPACE, null)}
          {spaces.length > 0 && <div className="my-1 border-t border-[var(--color-border-muted)]" />}
          {spaces.map((sp) => item(sp.key, sp))}
          <div className="my-1 border-t border-[var(--color-border-muted)]" />
          <button
            onClick={() => {
              setOpen(false)
              navigate('/cloud')
            }}
            className="w-full flex items-center gap-2.5 px-2.5 h-8 text-[13px] text-[var(--color-text-secondary)] rounded-md hover:bg-[var(--color-accent-muted)]"
          >
            {spaces.length > 0 ? <Settings2 className="w-4 h-4" /> : <Cloud className="w-4 h-4" />}
            {spaces.length > 0 ? '云端同步与团队' : '开通云端同步'}
          </button>
        </div>
      )}
    </div>
  )
}
