import { useEffect, useMemo, useState, type DragEvent } from 'react'
import clsx from 'clsx'
import { GripVertical, Plus, RotateCcw, Trash2 } from 'lucide-react'
import { Button, ConfirmModal, Input, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { errorText } from '../../cloud/errors'
import { fetchBookmarks, resetBookmarks, saveBookmarks } from '../api'
import { TagInput } from '../components/TagInput'
import { TagFilter, countTags, matchTags } from '../components/taxonomy'
import type { BrowserBookmark } from '../types'

/** 编辑中的一行; key 只在本页用于 React 列表与拖动, 不保存。 */
type Row = BrowserBookmark & { key: number }

let nextKey = 0
const toRows = (list: BrowserBookmark[]): Row[] => list.map((b) => ({ ...b, tags: b.tags ?? [], key: ++nextKey }))
const toItems = (rows: Row[]): BrowserBookmark[] => rows.map(({ name, url, tags }) => ({ name: name.trim(), url: url.trim(), tags: tags ?? [] }))
const snapshot = (rows: Row[]) => JSON.stringify(toItems(rows))

const COLS = 'grid-cols-[2rem_12rem_1fr_16rem_2.5rem]'

/** 书签: 新实例第一次启动时写进书签栏。整表编辑, 点保存一次提交。 */
export function BookmarkSettingsPage() {
  const [rows, setRows] = useState<Row[]>([])
  const [saved, setSaved] = useState('[]')
  const [saving, setSaving] = useState(false)
  const [confirmReset, setConfirmReset] = useState(false)
  const [dragKey, setDragKey] = useState<number | null>(null)
  const [tagFilter, setTagFilter] = useState<string[]>([])

  const load = async () => {
    const r = toRows(await fetchBookmarks())
    setRows(r)
    setSaved(snapshot(r))
  }

  // 同步空间的书签随空间切换与云端拉取变化
  const changeTick = useCloudStore((s) => s.changeTick)
  useEffect(() => {
    load()
  }, [changeTick])

  const dirty = snapshot(rows) !== saved
  const tags = useMemo(() => countTags(rows, (b) => b.tags), [rows])
  const filtering = tagFilter.length > 0

  const patch = (key: number, p: Partial<BrowserBookmark>) => setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...p } : r)))

  const save = async () => {
    const items = toItems(rows)
    if (items.some((b) => !b.name || !b.url)) return toast.error('每个书签都要填名称和网址')
    setSaving(true)
    try {
      await saveBookmarks(items)
      setSaved(JSON.stringify(items))
      toast.success('已保存, 之后新建的实例生效')
    } catch (e) {
      toast.error(`保存失败: ${errorText(e)}`)
    } finally {
      setSaving(false)
    }
  }

  const reset = async () => {
    try {
      await resetBookmarks()
      await load()
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  // 拖到哪一行就插到哪一行; 筛选时禁用, 免得和看不见的行换位
  const dragOver = (e: DragEvent, over: number) => {
    e.preventDefault()
    if (dragKey === null || dragKey === over) return
    setRows((rs) => {
      const from = rs.findIndex((r) => r.key === dragKey)
      const to = rs.findIndex((r) => r.key === over)
      const next = [...rs]
      next.splice(to, 0, ...next.splice(from, 1))
      return next
    })
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="mb-2.5 flex shrink-0 items-center gap-2">
        <h1 className="text-base font-semibold text-[var(--color-text-primary)]">书签</h1>
        <span className="text-xs text-[var(--color-text-muted)]">新实例第一次启动时写入书签栏, 已有实例不变</span>
        <div className="ml-auto flex items-center gap-2">
          <TagFilter tags={tags} value={tagFilter} onChange={setTagFilter} />
          <Button variant="secondary" onClick={() => setConfirmReset(true)}>
            <RotateCcw className="h-3.5 w-3.5" />
            恢复默认
          </Button>
          <Button onClick={save} loading={saving} disabled={!dirty}>
            保存
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
        <div className={clsx('sticky top-0 z-10 grid h-9 items-center border-b border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-1 text-xs font-medium text-[var(--color-text-muted)]', COLS)}>
          <span />
          <span className="px-2">名称</span>
          <span className="px-2">网址</span>
          <span className="px-2">标签</span>
          <span />
        </div>
        <ul className="divide-y divide-[var(--color-border-muted)]">
          {rows
            .filter((r) => matchTags(r.tags, tagFilter))
            .map((r) => (
              <li
                key={r.key}
                draggable={!filtering}
                onDragStart={() => setDragKey(r.key)}
                onDragOver={(e) => dragOver(e, r.key)}
                onDragEnd={() => setDragKey(null)}
                className={clsx('grid items-center gap-x-2 px-1 py-1.5', COLS, dragKey === r.key ? 'bg-[var(--color-brand-soft)]' : 'hover:bg-[var(--color-accent-muted)]')}
              >
                <span className={clsx('flex justify-center text-[var(--color-text-muted)]', filtering ? 'opacity-30' : 'cursor-grab')} title={filtering ? '筛选时不能排序' : '拖动排序'}>
                  <GripVertical className="h-3.5 w-3.5" />
                </span>
                <Input value={r.name} onChange={(e) => patch(r.key, { name: e.target.value })} placeholder="名称" />
                <Input value={r.url} onChange={(e) => patch(r.key, { url: e.target.value })} placeholder="https://" className="font-mono text-xs" />
                <TagInput value={r.tags ?? []} onChange={(t) => patch(r.key, { tags: t })} suggestions={tags.map((t) => t.name)} placeholder="添加标签" />
                <button
                  type="button"
                  aria-label="删除书签"
                  onClick={() => setRows((rs) => rs.filter((x) => x.key !== r.key))}
                  className="mx-auto flex h-7 w-7 items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-error-soft)] hover:text-[var(--color-error)]"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </li>
            ))}
        </ul>
        {rows.length === 0 && <p className="py-10 text-center text-[13px] text-[var(--color-text-secondary)]">还没有书签</p>}
        <button
          type="button"
          // 筛选中新增的书签自动带上当前筛选的标签, 加完仍然可见
          onClick={() => setRows((rs) => [...rs, { name: '', url: '', tags: [...tagFilter], key: ++nextKey }])}
          className="flex h-9 w-full items-center gap-1.5 border-t border-[var(--color-border-muted)] px-3 text-[13px] text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]"
        >
          <Plus className="h-3.5 w-3.5" />
          添加书签
        </button>
      </div>

      <ConfirmModal
        open={confirmReset}
        onClose={() => setConfirmReset(false)}
        onConfirm={reset}
        title="恢复默认书签"
        content="当前书签会全部被内置的默认列表替换。"
        confirmText="恢复"
        danger
      />
    </div>
  )
}
