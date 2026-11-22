import { Link } from 'react-router-dom'
import clsx from 'clsx'
import { Copy, MoreHorizontal, Pencil, Play, RotateCcw, Square, Trash2 } from 'lucide-react'
import { Button, Dropdown } from '../../../shared/components'
import type { BrowserProfile } from '../types'
import { platformOf, relativeTime, type CoreDisplay, type ProxyDisplay } from '../utils/instanceDisplay'
import { TagChips } from './taxonomy'

export type RowAction = 'start' | 'stop' | 'edit' | 'restart' | 'clone' | 'delete'

interface Props {
  profile: BrowserProfile
  selected: boolean
  busy: boolean
  /** 有分组时才显示分组列; undefined 表示不显示 */
  groupLabel?: string
  proxy: ProxyDisplay
  core: CoreDisplay
  /** 同步空间里被别人 / 自己另一台电脑占用 */
  inUse?: { label: string; title: string }
  onToggle: () => void
  onAction: (a: RowAction) => void
}

const dash = <span className="text-[var(--color-text-muted)]">-</span>

function Status({ p, inUse }: { p: BrowserProfile; inUse?: Props['inUse'] }) {
  const [tone, text, title] = p.running
    ? ['success', '运行中', '']
    : inUse
      ? ['warning', inUse.label, inUse.title]
      : p.lastError
        ? ['error', '异常退出', p.lastError]
        : ['muted', '已停止', '']
  const color = tone === 'muted' ? 'var(--color-text-muted)' : `var(--color-${tone})`
  return (
    <span className="inline-flex items-center gap-1.5" style={{ color }} title={title}>
      <span className="h-1.5 w-1.5 rounded-full" style={{ background: tone === 'muted' ? 'var(--color-border-strong)' : color }} />
      {text}
    </span>
  )
}

/** 两行单元格: 主文字 + 灰色小字 */
function Stack({ main, sub, tone }: { main: string; sub?: string; tone?: string }) {
  return (
    <>
      <div className="max-w-[11rem] truncate" style={{ color: tone ?? 'var(--color-text-secondary)' }} title={main}>
        {main}
      </div>
      {sub && <div className="max-w-[11rem] truncate text-[11px] text-[var(--color-text-muted)]">{sub}</div>}
    </>
  )
}

/** 实例列表的一行。 */
export function InstanceRow({ profile: p, selected, busy, groupLabel, proxy, core, inUse, onToggle, onAction }: Props) {
  return (
    <tr className={selected ? 'bg-[var(--color-brand-soft)]' : 'hover:bg-[var(--color-accent-muted)]'}>
      <td className="w-9 py-2 pl-3 pr-0">
        <input type="checkbox" aria-label={`选择 ${p.profileName}`} checked={selected} onChange={onToggle} className="h-3.5 w-3.5 cursor-pointer align-middle accent-[var(--color-button-primary)]" />
      </td>
      <td className="min-w-0 px-3 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <Link to={`/browser/detail/${p.profileId}`} title={p.profileName} className="max-w-[22rem] truncate font-medium text-[var(--color-text-primary)] underline-offset-2 hover:underline">
            {p.profileName}
          </Link>
          <TagChips tags={p.tags} max={2} />
        </div>
      </td>
      {groupLabel !== undefined && <td className="max-w-[7rem] truncate px-3 py-2 text-[var(--color-text-secondary)]">{groupLabel || dash}</td>}
      <td className="whitespace-nowrap px-3 py-2">
        <Status p={p} inUse={inUse} />
      </td>
      <td className="px-3 py-2">
        <Stack main={proxy.name} sub={proxy.region} tone={proxy.failed ? 'var(--color-error)' : undefined} />
      </td>
      <td className="px-3 py-2">
        <Stack main={core.name} sub={[core.followsDefault && '跟随默认', core.version].filter(Boolean).join(' · ')} tone={core.missing ? 'var(--color-warning)' : undefined} />
      </td>
      <td className="whitespace-nowrap px-3 py-2 text-[var(--color-text-secondary)]">{platformOf(p.fingerprintArgs) || dash}</td>
      <td className="whitespace-nowrap px-3 py-2 text-xs tabular-nums text-[var(--color-text-muted)]" title={p.lastStartAt ? new Date(p.lastStartAt).toLocaleString('zh-CN') : ''}>
        {relativeTime(p.lastStartAt) || '从未'}
      </td>
      <td className="px-3 py-2">
        <div className="flex items-center justify-end gap-1">
          <Button size="sm" variant={p.running ? 'secondary' : 'primary'} loading={busy} onClick={() => onAction(p.running ? 'stop' : 'start')} className="w-14">
            {!busy && (p.running ? <Square className="h-3 w-3 fill-current" /> : <Play className="h-3 w-3 fill-current" />)}
            {p.running ? '停止' : '启动'}
          </Button>
          <Dropdown
            placement="bottom-right"
            onSelect={(k) => onAction(k as RowAction)}
            items={[
              { key: 'edit', label: '编辑', icon: <Pencil className="h-3.5 w-3.5" /> },
              { key: 'restart', label: '重启', icon: <RotateCcw className="h-3.5 w-3.5" />, disabled: !p.running },
              { key: 'clone', label: '克隆', icon: <Copy className="h-3.5 w-3.5" /> },
              { key: '-', label: '', divider: true },
              { key: 'delete', label: '删除', icon: <Trash2 className="h-3.5 w-3.5" />, danger: true, disabled: p.running },
            ]}
            trigger={
              <button aria-label="更多操作" className={clsx('flex h-7 w-7 items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]')}>
                <MoreHorizontal className="h-4 w-4" />
              </button>
            }
          />
        </div>
      </td>
    </tr>
  )
}
