import { useState } from 'react'
import { AlertTriangle, CheckCircle2, CloudOff, RefreshCw, Trash2, User, Users } from 'lucide-react'
import { Badge, Button, Card, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { cloudApi, formatBytes, ROLE_LABELS, spaceLabel, type SpaceInfo } from '../api'
import { askConfirm } from '../confirm'
import { errorCode, errorText } from '../errors'
import { EnrollPanel, UnlockPanel } from './KeysPanel'

function timeAgo(iso: string): string {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  return new Date(iso).toLocaleDateString('zh-CN')
}

function SpaceRow({ sp, current }: { sp: SpaceInfo; current: boolean }) {
  const switchSpace = useCloudStore((s) => s.switchSpace)
  const refresh = useCloudStore((s) => s.refresh)
  const need = useCloudStore((s) => s.needs[sp.key])
  const [busy, setBusy] = useState(false)

  const syncNow = async () => {
    setBusy(true)
    try {
      await cloudApi.syncNow(sp.key)
      toast.success('同步完成')
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setBusy(false)
      void refresh()
    }
  }

  const forget = async () => {
    const ok = await askConfirm({
      title: '清除本机副本',
      content: `删除本机上「${spaceLabel(sp)}」的实例配置与登录态。你已不在这个团队，删除后无法再找回。`,
      confirmText: '删除',
      danger: true,
    })
    if (!ok) return
    try {
      await cloudApi.forgetSpace(sp.key, false)
    } catch (err) {
      if (errorCode(err) !== 40910) {
        toast.error(errorText(err))
        return
      }
      const force = await askConfirm({
        title: '还有修改没上传',
        content: '这些修改已经推不上去了，删除后会一起丢掉。',
        confirmText: '仍然删除',
        danger: true,
      })
      if (!force) return
      try {
        await cloudApi.forgetSpace(sp.key, true)
      } catch (e) {
        toast.error(errorText(e))
        return
      }
    }
    toast.success('已清除')
    void refresh()
  }

  const usage = sp.quotaBytes > 0 ? Math.min(100, Math.round((sp.usedBytes / sp.quotaBytes) * 100)) : 0
  return (
    <div className="flex items-center gap-4 px-5 py-4">
      <div className="w-9 h-9 shrink-0 rounded-lg bg-[var(--color-bg-muted)] flex items-center justify-center">
        {sp.kind === 'personal' ? <User className="w-4 h-4" /> : <Users className="w-4 h-4" />}
      </div>
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-[var(--color-text-primary)] truncate">{spaceLabel(sp)}</span>
          {sp.kind === 'org' && <Badge size="sm">{ROLE_LABELS[sp.role] || sp.role}</Badge>}
          {current && <Badge size="sm" variant="info">当前</Badge>}
          {sp.removed && <Badge size="sm" variant="warning">已被移出</Badge>}
          {!sp.removed && need === 'subscribe' && <Badge size="sm" variant="warning">需要订阅</Badge>}
          {!sp.removed && need === 'seats' && <Badge size="sm" variant="warning">超出席位</Badge>}
          {sp.keyPending && <Badge size="sm" variant="warning">等待授予密钥</Badge>}
        </div>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-[var(--color-text-muted)]">
          {sp.status === 'error' && !sp.keyPending ? (
            <span className="flex items-center gap-1 text-[var(--color-error)]">
              <AlertTriangle className="w-3.5 h-3.5" />
              {errorText(sp.error)}
            </span>
          ) : sp.syncedAt ? (
            <span className="flex items-center gap-1">
              <CheckCircle2 className="w-3.5 h-3.5" />
              {timeAgo(sp.syncedAt)}同步
            </span>
          ) : null}
          {sp.pending > 0 && <span>{sp.pending} 项待上传</span>}
          {sp.quotaBytes > 0 ? (
            <span className="flex items-center gap-2">
              <span className="inline-block w-20 h-1.5 rounded-full bg-[var(--color-bg-muted)] overflow-hidden">
                <span className="block h-full bg-[var(--color-text-secondary)]" style={{ width: `${usage}%` }} />
              </span>
              {formatBytes(sp.usedBytes)} / {formatBytes(sp.quotaBytes)}
            </span>
          ) : (
            sp.usedBytes > 0 && <span>已用 {formatBytes(sp.usedBytes)}</span>
          )}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {!current && (
          <Button size="sm" variant="ghost" onClick={() => void switchSpace(sp.key)}>
            切换到这里
          </Button>
        )}
        {sp.removed ? (
          <Button size="sm" variant="secondary" onClick={forget}>
            <Trash2 className="w-3.5 h-3.5" />
            清除副本
          </Button>
        ) : (
          <Button size="sm" variant="secondary" onClick={syncNow} loading={busy}>
            <RefreshCw className="w-3.5 h-3.5" />
            立即同步
          </Button>
        )}
      </div>
    </div>
  )
}

export function SyncSection() {
  const sync = useCloudStore((s) => s.sync)
  const setRecoveryCode = useCloudStore((s) => s.setRecoveryCode)
  const keys = sync?.keys || 'unknown'
  const spaces = sync?.spaces || []

  const resetRecovery = async () => {
    const ok = await askConfirm({
      title: '重置恢复码',
      content: '生成新的恢复码后，旧恢复码立即作废。已登录的电脑不受影响。',
      confirmText: '生成新恢复码',
    })
    if (!ok) return
    try {
      setRecoveryCode(await cloudApi.resetRecovery())
    } catch (err) {
      toast.error(errorText(err))
    }
  }

  return (
    <div className="space-y-5">
      {sync?.signedIn && !sync.available && sync.error && (
        <div className="flex items-center gap-3 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-3 text-sm text-[var(--color-text-secondary)]">
          <CloudOff className="w-4 h-4 shrink-0" />
          {errorText(sync.error)}
        </div>
      )}

      {keys === 'not_enrolled' && (
        <Card title="端到端加密">
          <EnrollPanel />
        </Card>
      )}
      {keys === 'locked' && (
        <Card title="解锁这台电脑">
          <UnlockPanel />
        </Card>
      )}

      <Card
        title="同步空间"
        subtitle="实例配置、代理池、分组、默认书签与实例登录态，按空间在你的电脑与团队成员之间同步"
        padding="none"
      >
        {spaces.length === 0 ? (
          <div className="px-5 py-8 text-center text-sm text-[var(--color-text-muted)]">
            {keys === 'unknown' ? '正在连接同步服务…' : '还没有同步空间。开启加密后会出现「我的同步」，建团队或加入团队后会出现团队空间。'}
          </div>
        ) : (
          <div className="divide-y divide-[var(--color-border-muted)]">
            {spaces.map((sp) => (
              <SpaceRow key={sp.key} sp={sp} current={sp.key === sync?.current} />
            ))}
          </div>
        )}
      </Card>

      <Card title="怎么用">
        <ul className="space-y-2 text-sm text-[var(--color-text-secondary)] list-disc pl-5">
          <li>在左上角切换数据空间。「本机」里的数据只在这台电脑，不上传。</li>
          <li>想把已有实例放进同步或团队：在实例列表勾选后点「复制到…」，可连同登录态一起复制。</li>
          <li>实例关闭后自动上传登录态，在另一台电脑启动前自动下载。团队里有人正在用的实例，启动时会提醒。</li>
          <li>数据在本机加密后才上传，CZL 看不到内容。新成员加入团队后，由所有者或管理员的电脑自动授予密钥。</li>
        </ul>
      </Card>

      {keys === 'unlocked' && (
        <Card title="恢复码" subtitle="换电脑、重装系统时用恢复码解锁云端数据">
          <div className="flex items-center justify-between gap-4">
            <p className="text-sm text-[var(--color-text-secondary)]">恢复码丢了或可能泄露时，可以生成一个新的，旧的随即作废。</p>
            <Button variant="secondary" size="sm" onClick={resetRecovery}>
              重置恢复码
            </Button>
          </div>
        </Card>
      )}
    </div>
  )
}
