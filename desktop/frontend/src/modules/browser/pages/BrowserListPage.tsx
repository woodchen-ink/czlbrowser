import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { FolderInput, Play, Plus, Search, Send, Square, Trash2, X, XCircle } from 'lucide-react'
import { Button, Dropdown, FormItem, Input, Modal, Segmented, toast } from '../../../shared/components'
import { useEvents } from '../../../shared/hooks/useRuntimeEvents'
import { useCloudStore } from '../../../store/cloudStore'
import { CopyToSpaceModal } from '../../cloud/components/CopyToSpaceModal'
import { askConfirm } from '../../cloud/confirm'
import {
  copyBrowserProfile,
  deleteBrowserProfile,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchCoreExtendedInfo,
  fetchGroups,
  moveInstancesToGroup,
  restartBrowserInstance,
  startBrowserInstance,
  stopBrowserInstance,
  validateProxyConfig,
} from '../api'
import { InstanceRow, type RowAction } from '../components/InstanceRow'
import { GROUP_ALL, GROUP_NONE, GroupPills, TagFilter, countTags, matchGroup, matchTags } from '../components/taxonomy'
import type { BrowserCore, BrowserCoreExtended, BrowserGroupWithCount, BrowserProfile, BrowserProxy } from '../types'
import { actionError } from '../utils/actionErrors'
import { coreDisplay, naturalCompare, proxyDisplay } from '../utils/instanceDisplay'
import { DEFAULT_PREFS, useListPrefs, type StatusFilter } from '../utils/listPrefs'

const STATUS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'running', label: '运行中' },
  { value: 'stopped', label: '已停止' },
]

const th = 'h-9 whitespace-nowrap border-b border-[var(--color-border-default)] px-3 text-left font-medium'

/** 同步空间里正在别处使用的实例: 实例 id → 显示文字 */
function usePresence() {
  const space = useCloudStore((s) => s.sync?.current || 'local')
  const me = useCloudStore((s) => s.sync?.userId || '')
  const presence = useCloudStore((s) => s.presence[space])
  return useMemo(() => {
    const m = new Map<string, { label: string; title: string }>()
    for (const p of presence ?? []) {
      if (p.collection !== 'profiles') continue
      m.set(p.id, { label: me && p.userId === me ? '另一台电脑在用' : '他人在用', title: p.deviceName ? `正在「${p.deviceName}」上使用` : '' })
    }
    return m
  }, [presence, me])
}

/** 实例列表: 工具栏 + 分组切换 (勾选后换成批量操作) + 内部滚动的表格。 */
export function BrowserListPage() {
  const navigate = useNavigate()
  const [profiles, setProfiles] = useState<BrowserProfile[] | null>(null)
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [coreExt, setCoreExt] = useState(new Map<string, BrowserCoreExtended>())

  const [prefs, setPrefs] = useListPrefs()
  const [keyword, setKeyword] = useState('')
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [busy, setBusy] = useState<Set<string>>(new Set())
  const [batchBusy, setBatchBusy] = useState(false)
  const [copyToOpen, setCopyToOpen] = useState(false)
  const [proxyError, setProxyError] = useState<{ id: string; msg: string } | null>(null)
  const [failure, setFailure] = useState('')
  const [clone, setClone] = useState<{ from: BrowserProfile; name: string; saving: boolean } | null>(null)

  const inUse = usePresence()

  const reloadProfiles = useCallback(() => fetchBrowserProfiles().then(setProfiles), [])
  const reloadGroups = useCallback(() => fetchGroups().then(setGroups), [])
  const reloadAll = useCallback(() => {
    reloadProfiles()
    reloadGroups()
    fetchBrowserProxies().then(setProxies)
    fetchBrowserCores().then(setCores)
    fetchCoreExtendedInfo().then((l) => setCoreExt(new Map(l.map((c) => [c.coreId, c]))))
  }, [reloadProfiles, reloadGroups])

  useEffect(() => {
    reloadAll()
  }, [reloadAll])

  useEvents({ 'browser:instance:started': reloadProfiles, 'browser:instance:stopped': reloadProfiles, 'browser:instance:crashed': reloadProfiles }, [reloadProfiles])

  // 切换空间或拉到云端修改后整页重载
  const changeTick = useCloudStore((s) => s.changeTick)
  useEffect(() => {
    if (changeTick === 0) return
    setPicked(new Set())
    reloadAll()
  }, [changeTick, reloadAll])

  const all = profiles ?? []
  const groupName = useMemo(() => new Map(groups.map((g) => [g.groupId, g.groupName])), [groups])
  // 分组已被删掉的实例算未分组
  const groupOf = useCallback((p: BrowserProfile) => (p.groupId && groupName.has(p.groupId) ? p.groupId : ''), [groupName])

  const inGroup = useMemo(() => all.filter((p) => matchGroup(groupOf(p), prefs.group)), [all, prefs.group, groupOf])
  const tagOptions = useMemo(() => countTags(inGroup, (p) => p.tags), [inGroup])

  const rows = useMemo(() => {
    const q = keyword.trim().toLowerCase()
    return inGroup
      .filter((p) => (prefs.status === 'all' ? true : p.running === (prefs.status === 'running')))
      .filter((p) => matchTags(p.tags, prefs.tags))
      .filter((p) => !q || [p.profileName, groupName.get(groupOf(p)) ?? '', ...(p.tags ?? [])].join(' ').toLowerCase().includes(q))
      .sort((a, b) => naturalCompare(a.profileName, b.profileName))
  }, [inGroup, prefs.status, prefs.tags, keyword, groupName, groupOf])

  const pills = useMemo(() => groups.map((g) => ({ key: g.groupId, label: g.groupName, count: all.filter((p) => p.groupId === g.groupId).length })), [groups, all])
  const running = all.filter((p) => p.running).length

  // 选中的分组被删了就回到全部
  useEffect(() => {
    if (groups.length && prefs.group !== GROUP_ALL && prefs.group !== GROUP_NONE && !groupName.has(prefs.group)) setPrefs({ group: GROUP_ALL })
  }, [groups, groupName, prefs.group])

  // 勾选只保留当前还能看到的
  useEffect(() => {
    setPicked((prev) => {
      const visible = new Set(rows.map((p) => p.profileId))
      const next = new Set([...prev].filter((id) => visible.has(id)))
      return next.size === prev.size ? prev : next
    })
  }, [rows])

  const selection = rows.filter((p) => picked.has(p.profileId))
  const allPicked = rows.length > 0 && selection.length === rows.length

  // ---- 单个实例 ----

  const withBusy = async (id: string, fn: () => Promise<unknown>) => {
    setBusy((s) => new Set(s).add(id))
    try {
      await fn()
    } finally {
      setBusy((s) => {
        const n = new Set(s)
        n.delete(id)
        return n
      })
      reloadProfiles()
    }
  }

  const start = (p: BrowserProfile) =>
    withBusy(p.profileId, async () => {
      // 先检查代理还能不能用 (订阅刷新后节点可能没了), 不行就引导去改
      const check = await validateProxyConfig(p.proxyConfig || '', p.proxyId || '')
      if (!check.supported) return setProxyError({ id: p.profileId, msg: check.errorMsg })
      await startBrowserInstance(p.profileId).catch((e) => setFailure(actionError(e, '启动失败')))
    })

  const remove = async (p: BrowserProfile) => {
    const ok = await askConfirm({ title: `删除实例「${p.profileName}」`, content: '这个实例的浏览器数据 (登录、Cookie 等) 会一起删除, 无法恢复。', confirmText: '删除' })
    if (!ok) return
    await deleteBrowserProfile(p.profileId).then(
      () => toast.success('已删除'),
      (e) => toast.error(actionError(e, '删除失败')),
    )
    reloadProfiles()
    reloadGroups()
  }

  const onAction = (p: BrowserProfile, a: RowAction) => {
    if (a === 'start') start(p)
    else if (a === 'stop') withBusy(p.profileId, () => stopBrowserInstance(p.profileId).catch((e) => toast.error(actionError(e, '停止失败'))))
    else if (a === 'restart') withBusy(p.profileId, () => restartBrowserInstance(p.profileId).catch((e) => setFailure(actionError(e, '重启失败'))))
    else if (a === 'edit') navigate(`/browser/edit/${p.profileId}`)
    else if (a === 'clone') setClone({ from: p, name: `${p.profileName} 副本`, saving: false })
    else if (a === 'delete') remove(p)
  }

  const doClone = async () => {
    if (!clone?.name.trim()) return
    setClone({ ...clone, saving: true })
    try {
      await copyBrowserProfile(clone.from.profileId, clone.name.trim())
      toast.success('已克隆')
      reloadProfiles()
    } catch (e) {
      setFailure(actionError(e, '克隆失败'))
    }
    setClone(null)
  }

  // ---- 批量 ----

  /** 逐个执行, 单个失败不打断其余; 返回失败的 "名称: 原因" */
  const eachSelected = async (filter: (p: BrowserProfile) => boolean, fn: (p: BrowserProfile) => Promise<unknown>, action: string) => {
    setBatchBusy(true)
    const failed: string[] = []
    let ok = 0
    for (const p of selection.filter(filter)) {
      try {
        await fn(p)
        ok++
      } catch (e) {
        failed.push(`${p.profileName}: ${actionError(e, `${action}失败`)}`)
      }
    }
    setBatchBusy(false)
    reloadProfiles()
    return { ok, failed }
  }

  const batchStart = async () => {
    const { ok, failed } = await eachSelected((p) => !p.running, (p) => startBrowserInstance(p.profileId), '启动')
    toast.success(`已启动 ${ok} 个${failed.length ? `, ${failed.length} 个失败` : ''}`)
    if (failed.length) setFailure(`这些实例没能启动:\n${failed.slice(0, 3).join('\n')}${failed.length > 3 ? `\n...另有 ${failed.length - 3} 个` : ''}`)
  }

  const batchStop = async () => {
    const { ok } = await eachSelected((p) => p.running, (p) => stopBrowserInstance(p.profileId), '停止')
    toast.success(`已停止 ${ok} 个`)
  }

  const batchMove = async (groupId: string) => {
    try {
      await moveInstancesToGroup(selection.map((p) => p.profileId), groupId)
      toast.success(`已移动 ${selection.length} 个`)
      setPicked(new Set())
    } catch (e) {
      toast.error(actionError(e, '移动失败'))
    }
    reloadProfiles()
    reloadGroups()
  }

  const batchDelete = async () => {
    const ok = await askConfirm({ title: `删除 ${selection.length} 个实例`, content: '这些实例的浏览器数据 (登录、Cookie 等) 会一起删除, 无法恢复。运行中的实例会跳过。', confirmText: '删除' })
    if (!ok) return
    const { ok: n } = await eachSelected((p) => !p.running, (p) => deleteBrowserProfile(p.profileId), '删除')
    setPicked(new Set())
    toast.success(`已删除 ${n} 个`)
    reloadGroups()
  }

  const moveTargets = [
    ...groups.map((g) => ({ key: g.groupId, label: g.groupName })),
    ...(groups.length ? [{ key: '-', label: '', divider: true }] : []),
    { key: GROUP_NONE, label: '移出分组' },
  ]

  const showGroup = groups.length > 0
  const cols = showGroup ? 9 : 8

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="mb-2.5 flex shrink-0 items-center gap-2">
        <h1 className="text-base font-semibold text-[var(--color-text-primary)]">实例</h1>
        <span className="text-xs tabular-nums text-[var(--color-text-muted)]">
          {all.length} 个{running > 0 && <span className="text-[var(--color-success)]"> · {running} 运行中</span>}
        </span>
        <div className="ml-auto flex items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-muted)]" />
            <Input value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="搜索名称 / 标签" className="w-52 pl-8" />
          </div>
          <TagFilter tags={tagOptions} value={prefs.tags} onChange={(tags) => setPrefs({ tags })} />
          <Segmented value={prefs.status} onChange={(status) => setPrefs({ status })} options={STATUS} />
          <Button onClick={() => navigate('/browser/edit/new')}>
            <Plus className="h-3.5 w-3.5" />
            新建实例
          </Button>
        </div>
      </div>

      <div className="mb-2 flex h-8 shrink-0 items-center">
        {selection.length > 0 ? (
          <div className="flex w-full items-center gap-1.5">
            <span className="mr-1 text-[13px] font-medium tabular-nums text-[var(--color-text-primary)]">已选 {selection.length}</span>
            <Button size="sm" variant="secondary" onClick={batchStart} loading={batchBusy}>
              <Play className="h-3.5 w-3.5" />
              启动
            </Button>
            <Button size="sm" variant="secondary" onClick={batchStop} disabled={batchBusy}>
              <Square className="h-3.5 w-3.5" />
              停止
            </Button>
            <Dropdown
              items={moveTargets}
              onSelect={(k) => batchMove(k === GROUP_NONE ? '' : k)}
              trigger={
                <Button size="sm" variant="secondary">
                  <FolderInput className="h-3.5 w-3.5" />
                  移到分组
                </Button>
              }
            />
            <Button size="sm" variant="secondary" onClick={() => setCopyToOpen(true)} title="复制到本机、云端或团队空间">
              <Send className="h-3.5 w-3.5" />
              复制到...
            </Button>
            <Button size="sm" variant="ghost" onClick={batchDelete} disabled={batchBusy} className="text-[var(--color-error)] hover:bg-[var(--color-error-soft)] hover:text-[var(--color-error)]">
              <Trash2 className="h-3.5 w-3.5" />
              删除
            </Button>
            <button onClick={() => setPicked(new Set())} className="ml-auto inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]">
              <X className="h-3.5 w-3.5" />
              取消选择
            </button>
          </div>
        ) : (
          <GroupPills
            groups={pills}
            value={prefs.group}
            onChange={(group) => setPrefs({ group })}
            total={all.length}
            ungrouped={all.filter((p) => !groupOf(p)).length}
            onManage={() => navigate('/settings?tab=taxonomy')}
          />
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
        <table className="w-full text-[13px]">
          <thead className="sticky top-0 z-10 bg-[var(--color-bg-surface)] text-xs text-[var(--color-text-muted)]">
            <tr>
              <th className={`${th} w-9 !pr-0`}>
                <input
                  type="checkbox"
                  aria-label="全选"
                  className="h-3.5 w-3.5 cursor-pointer align-middle accent-[var(--color-button-primary)]"
                  checked={allPicked}
                  ref={(el) => {
                    if (el) el.indeterminate = selection.length > 0 && !allPicked
                  }}
                  onChange={(e) => setPicked(e.target.checked ? new Set(rows.map((p) => p.profileId)) : new Set())}
                />
              </th>
              <th className={th}>名称</th>
              {showGroup && <th className={`${th} w-28`}>分组</th>}
              <th className={`${th} w-24`}>状态</th>
              <th className={`${th} w-44`}>代理</th>
              <th className={`${th} w-44`}>内核</th>
              <th className={`${th} w-20`}>平台</th>
              <th className={`${th} w-24`}>上次启动</th>
              <th className={`${th} w-24`} />
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--color-border-muted)]">
            {profiles === null &&
              [60, 55, 50, 45, 40].map((w) => (
                <tr key={w}>
                  <td colSpan={cols} className="px-3 py-2.5">
                    <div className="h-4 animate-pulse rounded bg-[var(--color-bg-muted)]" style={{ width: `${w}%` }} />
                  </td>
                </tr>
              ))}
            {profiles !== null && rows.length === 0 && (
              <tr>
                <td colSpan={cols} className="py-16 text-center">
                  <p className="text-[13px] text-[var(--color-text-secondary)]">{all.length ? '没有符合条件的实例' : '还没有实例'}</p>
                  {all.length ? (
                    <button
                      className="mt-2 text-xs text-[var(--color-text-muted)] underline-offset-2 hover:underline"
                      onClick={() => {
                        setKeyword('')
                        setPrefs(DEFAULT_PREFS)
                      }}
                    >
                      清除筛选
                    </button>
                  ) : (
                    <Button className="mt-3" size="sm" onClick={() => navigate('/browser/edit/new')}>
                      <Plus className="h-3.5 w-3.5" />
                      新建第一个实例
                    </Button>
                  )}
                </td>
              </tr>
            )}
            {rows.map((p) => (
              <InstanceRow
                key={p.profileId}
                profile={p}
                selected={picked.has(p.profileId)}
                busy={busy.has(p.profileId)}
                groupLabel={showGroup ? groupName.get(groupOf(p)) ?? '' : undefined}
                proxy={proxyDisplay(p, proxies)}
                core={coreDisplay(p, cores, coreExt)}
                inUse={inUse.get(p.profileId)}
                onToggle={() =>
                  setPicked((s) => {
                    const n = new Set(s)
                    if (!n.delete(p.profileId)) n.add(p.profileId)
                    return n
                  })
                }
                onAction={(a) => onAction(p, a)}
              />
            ))}
          </tbody>
        </table>
      </div>

      <CopyToSpaceModal open={copyToOpen} kind="profiles" ids={selection.map((p) => p.profileId)} onClose={() => setCopyToOpen(false)} />

      <Modal
        open={proxyError !== null}
        onClose={() => setProxyError(null)}
        title="代理不可用"
        width="420px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setProxyError(null)}>
              取消
            </Button>
            {proxyError && <Button onClick={() => navigate(`/browser/edit/${proxyError.id}`)}>去换代理</Button>}
          </>
        }
      >
        <div className="flex items-start gap-2.5">
          <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-[var(--color-error)]" />
          <div className="space-y-1.5">
            <p className="text-[13px] text-[var(--color-text-primary)]">{proxyError?.msg}</p>
            <p className="text-xs text-[var(--color-text-muted)]">在编辑页换一个代理; 订阅导入的代理可先刷新订阅, 看节点是否还在。</p>
          </div>
        </div>
      </Modal>

      <Modal
        open={clone !== null}
        onClose={() => setClone(null)}
        title="克隆实例"
        width="420px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setClone(null)}>
              取消
            </Button>
            <Button onClick={doClone} loading={clone?.saving} disabled={!clone?.name.trim()}>
              克隆
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <p className="text-xs text-[var(--color-text-muted)]">复制代理、内核、分组、标签和启动参数, 指纹种子重新生成; 浏览器数据不复制。</p>
          <FormItem label="新实例名称" required>
            <Input value={clone?.name ?? ''} onChange={(e) => clone && setClone({ ...clone, name: e.target.value })} onKeyDown={(e) => e.key === 'Enter' && doClone()} autoFocus />
          </FormItem>
        </div>
      </Modal>

      <Modal open={!!failure} onClose={() => setFailure('')} title="操作失败" width="440px" footer={<Button onClick={() => setFailure('')}>知道了</Button>}>
        <p className="whitespace-pre-line text-[13px] text-[var(--color-text-secondary)]">{failure}</p>
      </Modal>
    </div>
  )
}
