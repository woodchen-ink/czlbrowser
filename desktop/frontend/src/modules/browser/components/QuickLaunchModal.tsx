import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import clsx from 'clsx'
import { CornerDownLeft, Loader2, Search } from 'lucide-react'
import { Modal, toast } from '../../../shared/components'
import { fetchBrowserProfiles, fetchGroups, startBrowserInstance } from '../api'
import type { BrowserGroupWithCount, BrowserProfile } from '../types'
import { actionError } from '../utils/actionErrors'

interface QuickLaunchModalProps {
  open: boolean
  onClose: () => void
}

const ALL = '\u0000all'
const NONE = '\u0000none'

/** 搜索打分: 名称开头匹配 > 名称包含 > 标签 / 分组包含; 0 表示不匹配。 */
function score(p: BrowserProfile, group: string, q: string): number {
  if (!q) return 1
  const name = p.profileName.toLowerCase()
  if (name.startsWith(q)) return 3
  if (name.includes(q)) return 2
  return [group, ...(p.tags ?? [])].some((t) => t.toLowerCase().includes(q)) ? 1 : 0
}

/** Ctrl+K 快速启动: 输入过滤, ↑↓ 选择, 回车启动。 */
export function QuickLaunchModal({ open, onClose }: QuickLaunchModalProps) {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [q, setQ] = useState('')
  const [group, setGroup] = useState(ALL)
  const [cursor, setCursor] = useState(0)
  const [starting, setStarting] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  useEffect(() => {
    if (!open) return
    setQ('')
    setGroup(ALL)
    setCursor(0)
    setStarting('')
    Promise.all([fetchBrowserProfiles(), fetchGroups()])
      .then(([p, g]) => {
        setProfiles(p)
        setGroups(g)
      })
      .catch((e) => toast.error(actionError(e, '读取实例失败')))
    setTimeout(() => inputRef.current?.focus(), 0)
  }, [open])

  const groupName = useMemo(() => new Map(groups.map((g) => [g.groupId, g.groupName])), [groups])

  // 分组按钮只列出有实例的分组
  const groupChips = useMemo(() => {
    const n = new Map<string, number>()
    profiles.forEach((p) => {
      const k = p.groupId || NONE
      n.set(k, (n.get(k) ?? 0) + 1)
    })
    const named = groups.filter((g) => n.has(g.groupId)).map((g) => ({ id: g.groupId, name: g.groupName, n: n.get(g.groupId)! }))
    return n.has(NONE) && named.length ? [...named, { id: NONE, name: '未分组', n: n.get(NONE)! }] : named
  }, [profiles, groups])

  const shown = useMemo(() => {
    const kw = q.trim().toLowerCase()
    return profiles
      .filter((p) => group === ALL || (p.groupId || NONE) === group)
      .map((p) => ({ p, s: score(p, groupName.get(p.groupId ?? '') ?? '', kw) }))
      .filter((x) => x.s > 0)
      // 已停止的排前面 (要启动的通常是它们), 再按匹配度与名称
      .sort((a, b) => Number(a.p.running) - Number(b.p.running) || b.s - a.s || a.p.profileName.localeCompare(b.p.profileName, 'zh-CN'))
      .map((x) => x.p)
  }, [profiles, group, q, groupName])

  useEffect(() => setCursor(0), [q, group])

  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-i="${cursor}"]`)?.scrollIntoView({ block: 'nearest' })
  }, [cursor])

  const launch = async (p?: BrowserProfile) => {
    if (!p || starting) return
    setStarting(p.profileId)
    try {
      const r = await startBrowserInstance(p.profileId)
      if (r) onClose()
    } catch (e) {
      toast.error(actionError(e, `启动「${p.profileName}」失败`))
    } finally {
      setStarting('')
    }
  }

  const onKey = (e: KeyboardEvent) => {
    const n = shown.length
    if (e.key === 'ArrowDown' && n) {
      e.preventDefault()
      setCursor((c) => (c + 1) % n)
    } else if (e.key === 'ArrowUp' && n) {
      e.preventDefault()
      setCursor((c) => (c - 1 + n) % n)
    } else if (e.key === 'Escape') {
      onClose()
    } else if (e.key === 'Enter') {
      e.preventDefault()
      launch(shown[cursor])
    }
  }

  const chip = (id: string, label: string, n: number) => (
    <button
      key={id}
      type="button"
      onClick={() => setGroup(id)}
      className={clsx(
        'h-6 shrink-0 rounded-md px-2 text-xs',
        group === id ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]' : 'bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]',
      )}
    >
      {label} <span className="opacity-60">{n}</span>
    </button>
  )

  return (
    <Modal open={open} onClose={onClose} title="快速启动" width="640px" bodyClassName="p-0">
      <div onKeyDown={onKey}>
        <div className="flex items-center gap-2 border-b border-[var(--color-border-default)] px-4">
          <Search className="h-4 w-4 shrink-0 text-[var(--color-text-muted)]" />
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="搜索实例名、标签或分组"
            className="h-12 w-full bg-transparent text-sm text-[var(--color-text-primary)] outline-none placeholder:text-[var(--color-text-muted)]"
          />
        </div>
        {groupChips.length > 0 && (
          <div className="flex gap-1.5 overflow-x-auto border-b border-[var(--color-border-muted)] px-4 py-2">
            {chip(ALL, '全部', profiles.length)}
            {groupChips.map((g) => chip(g.id, g.name, g.n))}
          </div>
        )}
        <ul ref={listRef} className="h-[360px] overflow-y-auto p-1.5">
          {shown.length === 0 && <li className="py-16 text-center text-sm text-[var(--color-text-muted)]">没有匹配的实例</li>}
          {shown.map((p, i) => (
            <li
              key={p.profileId}
              data-i={i}
              onMouseMove={() => i !== cursor && setCursor(i)}
              onClick={() => launch(p)}
              className={clsx('flex h-10 cursor-pointer items-center gap-3 rounded-md px-3', i === cursor && 'bg-[var(--color-brand-soft)]')}
            >
              <span className={clsx('h-1.5 w-1.5 shrink-0 rounded-full', p.running ? 'bg-[var(--color-success)]' : 'bg-[var(--color-border-strong)]')} />
              <span className="min-w-0 truncate text-[13px] font-medium text-[var(--color-text-primary)]">{p.profileName}</span>
              <span className="min-w-0 flex-1 truncate text-xs text-[var(--color-text-muted)]">
                {[groupName.get(p.groupId ?? ''), ...(p.tags ?? [])].filter(Boolean).join(' · ')}
              </span>
              {starting === p.profileId ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--color-text-muted)]" />
              ) : p.running ? (
                <span className="text-xs text-[var(--color-success)]">运行中</span>
              ) : (
                i === cursor && <CornerDownLeft className="h-3.5 w-3.5 text-[var(--color-text-muted)]" />
              )}
            </li>
          ))}
        </ul>
        <div className="flex justify-between border-t border-[var(--color-border-default)] px-4 py-2 text-xs text-[var(--color-text-muted)]">
          <span>↑↓ 选择 · 回车启动 · Esc 关闭</span>
          <span>Ctrl + K</span>
        </div>
      </div>
    </Modal>
  )
}
