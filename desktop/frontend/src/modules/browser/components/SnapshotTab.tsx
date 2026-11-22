import { useCallback, useEffect, useState } from 'react'
import { Archive, RotateCcw, Trash2 } from 'lucide-react'
import { Button, Card, ConfirmModal, Input, Table, toast, type TableColumn } from '../../../shared/components'
import { errorText } from '../../cloud/errors'
import { createSnapshot, deleteSnapshot, listSnapshots, restoreSnapshot } from '../api'
import type { SnapshotInfo } from '../types'

interface Props {
  profileId: string
  running: boolean
}

const two = (n: number) => String(n).padStart(2, '0')
const stampName = (d = new Date()) => `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())} ${two(d.getHours())}:${two(d.getMinutes())}`
const sizeText = (mb: number) => (mb >= 1 ? `${mb.toFixed(1)} MB` : `${Math.round(mb * 1024)} KB`)
const timeText = (s: string) => {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-CN')
}

type Pending = { kind: 'restore' | 'delete'; snap: SnapshotInfo }

/** 用户数据目录快照: 实例停止时可创建与恢复。 */
export function SnapshotTab({ profileId, running }: Props) {
  const [list, setList] = useState<SnapshotInfo[]>([])
  const [name, setName] = useState(stampName)
  const [busy, setBusy] = useState(false)
  const [pending, setPending] = useState<Pending | null>(null)

  const load = useCallback(async () => {
    try {
      setList(await listSnapshots(profileId))
    } catch (e) {
      toast.error(`读取快照失败: ${errorText(e)}`)
    }
  }, [profileId])

  useEffect(() => {
    load()
  }, [load])

  const run = async (fn: () => Promise<unknown>, ok: string) => {
    setBusy(true)
    try {
      await fn()
      toast.success(ok)
      await load()
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusy(false)
    }
  }

  const create = () =>
    run(async () => {
      await createSnapshot(profileId, name.trim())
      setName(stampName())
    }, '快照已创建')

  const confirmPending = () => {
    if (!pending) return
    const { kind, snap } = pending
    if (kind === 'restore') run(() => restoreSnapshot(profileId, snap.snapshotId), '已恢复到该快照')
    else run(() => deleteSnapshot(profileId, snap.snapshotId), '快照已删除')
  }

  const columns: TableColumn<SnapshotInfo>[] = [
    { key: 'name', title: '名称' },
    { key: 'sizeMB', title: '大小', width: 100, render: sizeText },
    { key: 'createdAt', title: '创建时间', width: 180, render: timeText },
    {
      key: 'snapshotId',
      title: '',
      width: 150,
      align: 'right',
      render: (_: string, snap) => (
        <div className="flex justify-end gap-1">
          <Button size="sm" variant="ghost" disabled={running || busy} title={running ? '先停止实例' : undefined} onClick={() => setPending({ kind: 'restore', snap })}>
            <RotateCcw className="h-3.5 w-3.5" />
            恢复
          </Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => setPending({ kind: 'delete', snap })}>
            <Trash2 className="h-3.5 w-3.5" />
            删除
          </Button>
        </div>
      ),
    },
  ]

  return (
    <Card title="快照" subtitle={running ? '实例运行中, 停止后才能创建或恢复' : '把用户数据目录打包保存, 需要时整体还原'}>
      <div className="mb-3 flex gap-2">
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="快照名称" disabled={running || busy} className="w-72" />
        <Button onClick={create} disabled={running || busy || !name.trim()} loading={busy}>
          <Archive className="h-4 w-4" />
          创建快照
        </Button>
      </div>
      <Table columns={columns} data={list} rowKey="snapshotId" emptyText="还没有快照" />
      <ConfirmModal
        open={pending !== null}
        onClose={() => setPending(null)}
        onConfirm={confirmPending}
        title={pending?.kind === 'restore' ? '恢复快照' : '删除快照'}
        content={
          pending?.kind === 'restore'
            ? `用快照「${pending.snap.name}」覆盖当前的用户数据 (登录、Cookie、扩展等), 当前数据不会另存。`
            : `删除快照「${pending?.snap.name ?? ''}」, 无法撤销。`
        }
        confirmText={pending?.kind === 'restore' ? '恢复' : '删除'}
        danger
      />
    </Card>
  )
}
