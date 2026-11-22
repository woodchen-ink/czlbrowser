import { useCallback, useEffect, useMemo, useState } from 'react'
import { Download, FolderOpen, MoreHorizontal, Star, Tag, Trash2 } from 'lucide-react'
import { Button, Dropdown, Modal, toast, type DropdownItem } from '../../../shared/components'
import { askConfirm } from '../../cloud/confirm'
import { errorText } from '../../cloud/errors'
import { deleteBrowserCore, fetchBrowserCores, fetchCoreExtendedInfo, openCorePath, setCoreTags, setDefaultBrowserCore, validateBrowserCorePath } from '../api'
import { CoreDownloadModal } from '../components/CoreDownloadModal'
import { TagInput } from '../components/TagInput'
import { TagChips, TagFilter, countTags, matchTags } from '../components/taxonomy'
import type { BrowserCore, CoreKind } from '../types'
import { CORE_KIND_HINT, CORE_KIND_LABEL } from '../utils/coreKind'

/** 列表一行: 内核记录 + 版本 / 类型 / 使用数 + 路径校验结果 */
interface CoreRow extends BrowserCore {
  version: string
  kind: CoreKind
  usedBy: number
  missing: string // 非空表示文件缺失, 内容是原因
}

async function loadRows(): Promise<CoreRow[]> {
  const [cores, ext] = await Promise.all([fetchBrowserCores(), fetchCoreExtendedInfo()])
  const info = new Map(ext.map((e) => [e.coreId, e]))
  return Promise.all(
    cores.map(async (c) => {
      const check = await validateBrowserCorePath(c.corePath)
      const e = info.get(c.coreId)
      return { ...c, tags: c.tags ?? [], version: e?.chromeVersion ?? '', kind: e?.coreKind ?? 'chrome', usedBy: e?.instanceCount ?? 0, missing: check.valid ? '' : check.message || '文件缺失' }
    }),
  )
}

const th = 'h-9 whitespace-nowrap border-b border-[var(--color-border-default)] px-3 text-left font-medium'

/** 内核: 本机已有的内核, 设默认 / 标签 / 删除, 以及下载内置内核。 */
export function CoreManagementPage() {
  const [rows, setRows] = useState<CoreRow[] | null>(null)
  const [tagFilter, setTagFilter] = useState<string[]>([])
  const [tagging, setTagging] = useState<{ row: CoreRow; tags: string[] } | null>(null)
  const [downloadOpen, setDownloadOpen] = useState(false)

  const reload = useCallback(() => loadRows().then(setRows).catch((e) => toast.error(errorText(e))), [])
  useEffect(() => {
    reload()
  }, [reload])

  const tags = useMemo(() => countTags(rows ?? [], (r) => r.tags), [rows])
  const shown = (rows ?? []).filter((r) => matchTags(r.tags, tagFilter))

  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    try {
      await fn()
      if (ok) toast.success(ok)
      await reload()
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  const makeDefault = (r: CoreRow) => run(() => setDefaultBrowserCore(r.coreId), `默认内核已改为「${r.coreName}」`)

  const remove = async (r: CoreRow) => {
    const ok = await askConfirm({
      title: `删除内核「${r.coreName}」`,
      content: r.usedBy > 0 ? `${r.usedBy} 个实例在用它, 删除后这些实例改用默认内核。` : '删除后可以重新下载。',
      confirmText: '删除',
    })
    if (ok) run(() => deleteBrowserCore(r.coreId), '已删除')
  }

  const menu = (r: CoreRow): DropdownItem[] => [
    { key: 'open', label: '打开目录', icon: <FolderOpen className="h-3.5 w-3.5" /> },
    { key: 'tags', label: '编辑标签', icon: <Tag className="h-3.5 w-3.5" /> },
    ...(r.isDefault ? [] : [{ key: 'default', label: '设为默认', icon: <Star className="h-3.5 w-3.5" />, disabled: !!r.missing }]),
    { key: '-', label: '', divider: true },
    // 默认内核不能删, 先把别的设为默认
    { key: 'delete', label: '删除', icon: <Trash2 className="h-3.5 w-3.5" />, danger: true, disabled: r.isDefault },
  ]

  const onMenu = (r: CoreRow, key: string) => {
    if (key === 'open') openCorePath(r.corePath).catch((e) => toast.error(errorText(e)))
    else if (key === 'tags') setTagging({ row: r, tags: r.tags ?? [] })
    else if (key === 'default') makeDefault(r)
    else if (key === 'delete') remove(r)
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="mb-2.5 flex shrink-0 items-center gap-2">
        <h1 className="text-base font-semibold text-[var(--color-text-primary)]">内核</h1>
        <span className="text-xs tabular-nums text-[var(--color-text-muted)]">{rows?.length ?? 0} 个</span>
        <div className="ml-auto flex items-center gap-2">
          <TagFilter tags={tags} value={tagFilter} onChange={setTagFilter} />
          <Button onClick={() => setDownloadOpen(true)}>
            <Download className="h-3.5 w-3.5" />
            下载内核
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
        <table className="w-full text-[13px]">
          <thead className="sticky top-0 z-10 bg-[var(--color-bg-surface)] text-xs text-[var(--color-text-muted)]">
            <tr>
              <th className={th}>名称</th>
              <th className={`${th} w-36`}>版本</th>
              <th className={`${th} w-28`}>类型</th>
              <th className={`${th} w-20 !text-right`}>实例数</th>
              <th className={`${th} w-20`}>状态</th>
              <th className={`${th} w-48`}>位置</th>
              <th className={`${th} w-28`} />
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--color-border-muted)]">
            {rows && shown.length === 0 && (
              <tr>
                <td colSpan={7} className="py-16 text-center text-[13px] text-[var(--color-text-secondary)]">
                  {rows.length ? '没有符合筛选的内核' : '还没有内核, 先下载一个'}
                  {!rows.length && (
                    <div className="mt-3">
                      <Button size="sm" onClick={() => setDownloadOpen(true)}>
                        <Download className="h-3.5 w-3.5" />
                        下载内核
                      </Button>
                    </div>
                  )}
                </td>
              </tr>
            )}
            {shown.map((r) => (
              <tr key={r.coreId} className="hover:bg-[var(--color-accent-muted)]">
                <td className="px-3 py-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate font-medium text-[var(--color-text-primary)]">{r.coreName}</span>
                    {r.isDefault && <span className="inline-flex h-[18px] shrink-0 items-center rounded bg-[var(--color-brand-soft)] px-1.5 text-[11px] font-medium text-[var(--color-brand-strong)]">默认</span>}
                    <TagChips tags={r.tags} />
                  </div>
                </td>
                <td className="px-3 py-2 font-mono text-xs tabular-nums text-[var(--color-text-secondary)]">{r.version || '-'}</td>
                <td className="px-3 py-2 text-[var(--color-text-secondary)]" title={CORE_KIND_HINT[r.kind]}>
                  {CORE_KIND_LABEL[r.kind]}
                </td>
                <td className="px-3 py-2 text-right tabular-nums text-[var(--color-text-secondary)]">{r.usedBy}</td>
                <td className="whitespace-nowrap px-3 py-2">
                  {r.missing ? (
                    <span className="text-[var(--color-error)]" title={r.missing}>
                      文件缺失
                    </span>
                  ) : (
                    <span className="text-[var(--color-text-muted)]">可用</span>
                  )}
                </td>
                <td className="px-3 py-2">
                  <span className="block max-w-[12rem] truncate font-mono text-[11px] text-[var(--color-text-muted)]" title={r.corePath}>
                    {r.corePath}
                  </span>
                </td>
                <td className="px-3 py-2">
                  <div className="flex items-center justify-end gap-1">
                    {!r.isDefault && !r.missing && (
                      <Button size="sm" variant="secondary" onClick={() => makeDefault(r)}>
                        设为默认
                      </Button>
                    )}
                    <Dropdown
                      placement="bottom-right"
                      items={menu(r)}
                      onSelect={(k) => onMenu(r, k)}
                      trigger={
                        <button aria-label="更多操作" className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]">
                          <MoreHorizontal className="h-4 w-4" />
                        </button>
                      }
                    />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Modal
        open={tagging !== null}
        onClose={() => setTagging(null)}
        title={`标签 · ${tagging?.row.coreName ?? ''}`}
        width="440px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setTagging(null)}>
              取消
            </Button>
            <Button
              onClick={() => {
                if (!tagging) return
                run(() => setCoreTags(tagging.row.coreId, tagging.tags))
                setTagging(null)
              }}
            >
              保存
            </Button>
          </>
        }
      >
        {tagging && <TagInput value={tagging.tags} onChange={(t) => setTagging({ ...tagging, tags: t })} suggestions={tags.map((t) => t.name)} />}
      </Modal>

      <CoreDownloadModal open={downloadOpen} onClose={() => setDownloadOpen(false)} onInstalled={reload} />
    </div>
  )
}
