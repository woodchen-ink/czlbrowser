import { useCallback, useEffect, useState } from 'react'
import { Check, Pencil, Plus, Trash2, X } from 'lucide-react'
import { Button, Segmented, toast } from '../../../shared/components'
import { SectionHeader, SettingGroup } from '../../../shared/layout/SectionPage'
import { askConfirm } from '../../cloud/confirm'
import { useCloudStore } from '../../../store/cloudStore'
import { countTags } from '../../browser/components/taxonomy'
import {
  createGroup,
  deleteGroup,
  deleteTaxonomyTag,
  fetchBookmarks,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchGroups,
  renameProxyGroup,
  renameTaxonomyTag,
  updateGroup,
  type TaxonomyKind,
} from '../../browser/api'

// 分组与标签: 四类记录 (实例 / 代理 / 内核 / 书签) 的分组与标签统一在这里重命名、删除。
// 实例与代理有分组 (一项只属一个分组) 和标签 (可多个); 内核与书签只有标签。

const KINDS: { value: TaxonomyKind; label: string }[] = [
  { value: 'profile', label: '实例' },
  { value: 'proxy', label: '代理' },
  { value: 'core', label: '内核' },
  { value: 'bookmark', label: '书签' },
]

interface Entry {
  /** 分组 ID (实例分组) 或名称 */
  id: string
  name: string
  count: number
}

function errText(err: any, fallback: string) {
  return typeof err === 'string' ? err : err?.message || fallback
}

export function TaxonomySection() {
  const [kind, setKind] = useState<TaxonomyKind>('profile')
  const [groups, setGroups] = useState<Entry[]>([])
  const [tags, setTags] = useState<Entry[]>([])
  const [loading, setLoading] = useState(true)
  const changeTick = useCloudStore((s) => s.changeTick)
  const hasGroups = kind === 'profile' || kind === 'proxy'

  const load = useCallback(async () => {
    setLoading(true)
    try {
      if (kind === 'profile') {
        const [profiles, gs] = await Promise.all([fetchBrowserProfiles(), fetchGroups()])
        setGroups(gs.map((g) => ({ id: g.groupId, name: g.groupName, count: g.instanceCount })))
        setTags(countTags(profiles, (p) => p.tags).map((t) => ({ id: t.name, ...t })))
      } else if (kind === 'proxy') {
        const proxies = (await fetchBrowserProxies()).filter((p) => p.proxyId !== '__direct__')
        const gmap = new Map<string, number>()
        for (const p of proxies) if (p.groupName) gmap.set(p.groupName, (gmap.get(p.groupName) || 0) + 1)
        setGroups([...gmap.entries()].map(([name, count]) => ({ id: name, name, count })).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')))
        setTags(countTags(proxies, (p) => p.tags).map((t) => ({ id: t.name, ...t })))
      } else if (kind === 'core') {
        setGroups([])
        setTags(countTags(await fetchBrowserCores(), (c) => c.tags).map((t) => ({ id: t.name, ...t })))
      } else {
        setGroups([])
        setTags(countTags(await fetchBookmarks(), (b) => b.tags).map((t) => ({ id: t.name, ...t })))
      }
    } catch (err) {
      toast.error(errText(err, '读取失败'))
    } finally {
      setLoading(false)
    }
  }, [kind])

  useEffect(() => {
    void load()
  }, [load, changeTick])

  const run = async (fn: () => Promise<unknown>, ok: string) => {
    try {
      await fn()
      toast.success(ok)
      await load()
    } catch (err) {
      toast.error(errText(err, '操作失败'))
    }
  }

  const renameGroup = (e: Entry, name: string) =>
    run(
      () => (kind === 'profile' ? updateGroup(e.id, { groupName: name, parentId: '', sortOrder: 0 }) : renameProxyGroup(e.name, name)),
      '已重命名'
    )
  const removeGroup = async (e: Entry) => {
    const ok = await askConfirm({
      title: `删除分组「${e.name}」`,
      content: `分组里的 ${e.count} 项会移到「未分组」，记录本身不删除。`,
      confirmText: '删除分组',
    })
    if (ok) await run(() => (kind === 'profile' ? deleteGroup(e.id) : renameProxyGroup(e.name, '')), '已删除')
  }
  const addGroup = (name: string) => run(() => createGroup({ groupName: name, parentId: '', sortOrder: groups.length }), '已创建')

  const renameTag = (e: Entry, name: string) => run(() => renameTaxonomyTag(kind, e.name, name), '已重命名')
  const removeTag = async (e: Entry) => {
    const ok = await askConfirm({
      title: `删除标签「${e.name}」`,
      content: `会从 ${e.count} 项上去掉这个标签，记录本身不删除。`,
      confirmText: '删除标签',
    })
    if (ok) await run(() => deleteTaxonomyTag(kind, e.name), '已删除')
  }

  return (
    <div>
      <SectionHeader
        title="分组与标签"
        description="一项只属于一个分组，可以打多个标签；列表页顶部可按分组切换、按标签筛选"
        actions={<Segmented size="sm" value={kind} onChange={setKind} options={KINDS} />}
      />

      {hasGroups && (
        <EntryList
          title="分组"
          entries={groups}
          loading={loading}
          emptyText={kind === 'proxy' ? '还没有分组，编辑代理时填写分组名即可创建' : '还没有分组'}
          onRename={renameGroup}
          onDelete={removeGroup}
          onCreate={kind === 'profile' ? addGroup : undefined}
        />
      )}
      <EntryList
        title="标签"
        entries={tags}
        loading={loading}
        emptyText="还没有标签，编辑记录时添加"
        onRename={renameTag}
        onDelete={removeTag}
      />
    </div>
  )
}

function EntryList({
  title,
  entries,
  loading,
  emptyText,
  onRename,
  onDelete,
  onCreate,
}: {
  title: string
  entries: Entry[]
  loading: boolean
  emptyText: string
  onRename: (e: Entry, name: string) => Promise<void>
  onDelete: (e: Entry) => Promise<void>
  onCreate?: (name: string) => Promise<void>
}) {
  const [creating, setCreating] = useState(false)
  return (
    <SettingGroup
      title={`${title}${entries.length ? ` · ${entries.length}` : ''}`}
      actions={
        onCreate && !creating ? (
          <Button size="sm" variant="ghost" onClick={() => setCreating(true)}>
            <Plus className="w-3.5 h-3.5" />
            新建{title}
          </Button>
        ) : undefined
      }
    >
      {creating && onCreate && (
        <NameEditor
          initial=""
          placeholder={`${title}名称`}
          onCancel={() => setCreating(false)}
          onSave={async (name) => {
            await onCreate(name)
            setCreating(false)
          }}
        />
      )}
      {!loading && entries.length === 0 && !creating && (
        <p className="px-4 py-4 text-xs text-[var(--color-text-muted)]">{emptyText}</p>
      )}
      {entries.map((e) => (
        <EntryRow key={e.id} entry={e} onRename={onRename} onDelete={onDelete} />
      ))}
    </SettingGroup>
  )
}

function EntryRow({ entry, onRename, onDelete }: { entry: Entry; onRename: (e: Entry, name: string) => Promise<void>; onDelete: (e: Entry) => Promise<void> }) {
  const [editing, setEditing] = useState(false)
  if (editing) {
    return (
      <NameEditor
        initial={entry.name}
        onCancel={() => setEditing(false)}
        onSave={async (name) => {
          if (name !== entry.name) await onRename(entry, name)
          setEditing(false)
        }}
      />
    )
  }
  return (
    <div className="group flex items-center gap-3 px-4 h-10">
      <span className="flex-1 min-w-0 truncate text-[13px] text-[var(--color-text-primary)]">{entry.name}</span>
      <span className="text-xs tabular-nums text-[var(--color-text-muted)]">{entry.count} 项</span>
      <span className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity">
        <IconBtn title="重命名" onClick={() => setEditing(true)}>
          <Pencil className="w-3.5 h-3.5" />
        </IconBtn>
        <IconBtn title="删除" danger onClick={() => void onDelete(entry)}>
          <Trash2 className="w-3.5 h-3.5" />
        </IconBtn>
      </span>
    </div>
  )
}

function NameEditor({ initial, placeholder, onSave, onCancel }: { initial: string; placeholder?: string; onSave: (name: string) => Promise<void>; onCancel: () => void }) {
  const [value, setValue] = useState(initial)
  const [busy, setBusy] = useState(false)
  const submit = async () => {
    const name = value.trim()
    if (!name || busy) return
    setBusy(true)
    try {
      await onSave(name)
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex items-center gap-2 px-4 h-10">
      <input
        autoFocus
        value={value}
        maxLength={32}
        placeholder={placeholder}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void submit()
          if (e.key === 'Escape') onCancel()
        }}
        className="flex-1 min-w-0 h-7 px-2 text-[13px] rounded border border-[var(--color-border-strong)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none"
      />
      <IconBtn title="保存" onClick={() => void submit()}>
        <Check className="w-3.5 h-3.5" />
      </IconBtn>
      <IconBtn title="取消" onClick={onCancel}>
        <X className="w-3.5 h-3.5" />
      </IconBtn>
    </div>
  )
}

function IconBtn({ title, onClick, danger, children }: { title: string; onClick: () => void; danger?: boolean; children: React.ReactNode }) {
  return (
    <button
      title={title}
      aria-label={title}
      onClick={onClick}
      className={
        'w-7 h-7 flex items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] ' +
        (danger ? 'hover:text-[var(--color-error)]' : 'hover:text-[var(--color-text-primary)]')
      }
    >
      {children}
    </button>
  )
}
