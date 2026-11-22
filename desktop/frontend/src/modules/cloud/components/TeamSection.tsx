import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import clsx from 'clsx'
import { Copy, Plus, ShieldCheck, Trash2, UserPlus, Users } from 'lucide-react'
import { Badge, Button, Card, Input, Modal, Select, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { cloudApi, ROLE_LABELS, subjectOf, type AddOn, type NewInviteCode, type Org, type OrgDetail } from '../api'
import { askConfirm } from '../confirm'
import { errorCode, errorText } from '../errors'
import { startEnroll } from './KeysPanel'
import { PurchaseModal, type PurchaseTarget } from './PurchaseModal'

// 团队 (CZL Connect 组织): 创建 / 凭码加入、成员与角色、邮箱邀请与邀请码、加席位、退出 / 解散。
// 团队的数据空间在侧栏切换; 新成员的加密密钥由所有者或管理员的电脑自动授予。

const inviteRoles = [
  { value: 'member', label: '成员（可修改）' },
  { value: 'viewer', label: '只读成员' },
  { value: 'admin', label: '管理员' },
]

function canManage(role: string) {
  return role === 'owner' || role === 'admin'
}

function TeamDetail({ org, onChanged }: { org: Org; onChanged: () => void }) {
  const refresh = useCloudStore((s) => s.refresh)
  const billing = useCloudStore((s) => s.billing)
  const [detail, setDetail] = useState<OrgDetail | null>(null)
  const [email, setEmail] = useState('')
  const [role, setRole] = useState('member')
  const [newCode, setNewCode] = useState<NewInviteCode | null>(null)
  const [addons, setAddons] = useState<AddOn[]>([])
  const [purchase, setPurchase] = useState<PurchaseTarget | null>(null)
  const admin = canManage(org.role)

  const load = useCallback(async () => {
    try {
      setDetail(await cloudApi.orgDetail(org.id))
    } catch (err) {
      toast.error(errorText(err))
    }
    if (admin && billing?.enabled) {
      cloudApi.billingAddOns(subjectOf(org.id)).then(setAddons).catch(() => setAddons([]))
    }
  }, [org.id, admin, billing?.enabled])

  useEffect(() => {
    void load()
  }, [load])

  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    try {
      await fn()
      if (ok) toast.success(ok)
      await load()
    } catch (err) {
      toast.error(errorText(err))
    }
  }

  const invite = async () => {
    if (!email.trim()) return
    await run(() => cloudApi.orgInviteEmail(org.id, email.trim(), role), '邀请已发出')
    setEmail('')
  }

  const createCode = async () => {
    try {
      setNewCode(await cloudApi.orgCreateInviteCode(org.id, role, 1, 7))
      await load()
    } catch (err) {
      toast.error(errorText(err))
    }
  }

  const leave = async (close: boolean) => {
    const ok = await askConfirm({
      title: close ? '解散团队' : '退出团队',
      content: close
        ? `解散「${org.name}」后所有成员都无法再访问团队数据，本机的团队实例与登录态也会删除。`
        : `退出「${org.name}」后本机的团队实例与登录态会删除，需要时请重新邀请加入。`,
      confirmText: close ? '解散' : '退出',
      danger: true,
    })
    if (!ok) return
    const call = (force: boolean) => (close ? cloudApi.orgClose(org.id, force) : cloudApi.orgLeave(org.id, force))
    try {
      await call(false)
    } catch (err) {
      if (errorCode(err) !== 40910) {
        toast.error(errorText(err))
        return
      }
      const force = await askConfirm({
        title: '还有修改没上传',
        content: '这个团队里还有修改没上传完，继续会丢掉这些修改。建议先在「同步与加密」里点立即同步。',
        confirmText: '仍然继续',
        danger: true,
      })
      if (!force) return
      try {
        await call(true)
      } catch (e) {
        toast.error(errorText(e))
        return
      }
    }
    toast.success(close ? '团队已解散' : '已退出团队')
    onChanged()
    void refresh()
  }

  const seatAddon = addons.find((a) => a.target === 'seats')
  const storageAddon = addons.find((a) => a.target !== 'seats')

  return (
    <div className="space-y-5">
      <Card
        title={org.name}
        subtitle={`${ROLE_LABELS[org.role] || org.role} · ${detail?.members.length ?? org.memberCount} 名成员${org.seats ? ` / ${org.seats} 席` : ''}`}
        actions={
          <div className="flex gap-2">
            {seatAddon && seatAddon.remaining > 0 && (
              <Button size="sm" variant="secondary" onClick={() => setPurchase({ kind: 'addon', addon: seatAddon, subject: subjectOf(org.id), subjectName: org.name })}>
                加席位
              </Button>
            )}
            {storageAddon && storageAddon.remaining > 0 && (
              <Button size="sm" variant="secondary" onClick={() => setPurchase({ kind: 'addon', addon: storageAddon, subject: subjectOf(org.id), subjectName: org.name })}>
                加容量
              </Button>
            )}
            {org.role === 'owner' ? (
              <Button size="sm" variant="ghost" onClick={() => leave(true)}>
                解散团队
              </Button>
            ) : (
              <Button size="sm" variant="ghost" onClick={() => leave(false)}>
                退出团队
              </Button>
            )}
          </div>
        }
        padding="none"
      >
        <div className="divide-y divide-[var(--color-border-muted)]">
          {(detail?.members || []).map((m) => (
            <div key={m.id} className="flex items-center gap-3 px-5 py-3">
              {m.avatar ? (
                <img src={m.avatar} alt="" className="w-8 h-8 rounded-lg object-cover" />
              ) : (
                <div className="w-8 h-8 rounded-lg bg-[var(--color-bg-muted)]" />
              )}
              <div className="min-w-0 flex-1">
                <div className="text-sm text-[var(--color-text-primary)] truncate">
                  {m.name || m.email}
                  {m.isSelf && <span className="ml-2 text-xs text-[var(--color-text-muted)]">（我）</span>}
                </div>
                <div className="text-xs text-[var(--color-text-muted)] truncate">{m.email}</div>
              </div>
              {admin && !m.isSelf && m.role !== 'owner' ? (
                <>
                  <div className="w-40 shrink-0">
                    <Select
                      value={m.role}
                      onChange={(e) => void run(() => cloudApi.orgSetRole(org.id, m.id, e.target.value), '角色已更新')}
                      options={inviteRoles.filter((r) => org.role === 'owner' || r.value !== 'admin')}
                    />
                  </div>
                  <button
                    title="移出团队"
                    className="p-1.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-error)] hover:bg-[var(--color-bg-muted)]"
                    onClick={async () => {
                      const ok = await askConfirm({
                        title: '移出成员',
                        content: `移出 ${m.name || m.email} 后，对方拉不到之后的修改，团队密钥会自动轮换；对方电脑上已同步的副本无法收回。`,
                        confirmText: '移出',
                        danger: true,
                      })
                      if (ok) await run(() => cloudApi.orgRemoveMember(org.id, m.id), '已移出')
                    }}
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </>
              ) : (
                <Badge size="sm">{ROLE_LABELS[m.role] || m.role}</Badge>
              )}
            </div>
          ))}
        </div>
      </Card>

      {admin && (
        <Card title="邀请成员" subtitle="对方接受后，你的电脑会在下次同步时自动把团队密钥授予他">
          <div className="space-y-4">
            <div className="flex gap-2">
              <Input
                value={email}
                placeholder="对方的邮箱"
                onChange={(e) => setEmail(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && void invite()}
              />
              <div className="w-40 shrink-0">
                <Select value={role} onChange={(e) => setRole(e.target.value)} options={inviteRoles.filter((r) => org.role === 'owner' || r.value !== 'admin')} />
              </div>
              <Button className="shrink-0" onClick={invite} disabled={!email.trim()}>
                <UserPlus className="w-4 h-4" />
                邀请
              </Button>
            </div>
            <div className="flex items-center justify-between gap-4 rounded-lg bg-[var(--color-bg-muted)] px-4 py-3">
              <span className="text-sm text-[var(--color-text-secondary)]">或者生成一个邀请码发给对方（单次有效，7 天内）</span>
              <Button size="sm" variant="secondary" onClick={createCode}>
                生成邀请码
              </Button>
            </div>
            {(detail?.invites || []).filter((i) => i.status === 'pending').length > 0 && (
              <div className="space-y-2">
                <div className="text-xs font-medium text-[var(--color-text-muted)]">待接受的邀请</div>
                {(detail?.invites || [])
                  .filter((i) => i.status === 'pending')
                  .map((i) => (
                    <div key={i.id} className="flex items-center justify-between text-sm">
                      <span className="text-[var(--color-text-secondary)]">
                        {i.email} · {ROLE_LABELS[i.role] || i.role}
                      </span>
                      <Button size="sm" variant="ghost" onClick={() => void run(() => cloudApi.orgRevokeInvite(org.id, i.id), '已撤回')}>
                        撤回
                      </Button>
                    </div>
                  ))}
              </div>
            )}
            {(detail?.inviteCodes || []).length > 0 && (
              <div className="space-y-2">
                <div className="text-xs font-medium text-[var(--color-text-muted)]">有效的邀请码</div>
                {(detail?.inviteCodes || []).map((c) => (
                  <div key={c.id} className="flex items-center justify-between text-sm">
                    <span className="text-[var(--color-text-secondary)]">
                      {ROLE_LABELS[c.role] || c.role} · 已用 {c.usedCount}/{c.maxUses} · {new Date(c.expiresAt).toLocaleDateString('zh-CN')} 到期
                    </span>
                    <Button size="sm" variant="ghost" onClick={() => void run(() => cloudApi.orgRevokeInviteCode(org.id, c.id), '已撤回')}>
                      撤回
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </Card>
      )}

      <Modal open={!!newCode} onClose={() => setNewCode(null)} title="邀请码" width="440px">
        <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
          <p>邀请码只显示这一次。对方在 CZL Browser「云端同步 → 团队」里输入即可加入。</p>
          <div className="rounded-lg bg-[var(--color-bg-muted)] px-4 py-3 text-center font-mono text-xl tracking-widest text-[var(--color-text-primary)] select-all">
            {newCode?.code}
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={async () => {
              const text = `邀请你加入 CZL Browser 团队「${org.name}」\n邀请码：${newCode?.code}\n网页加入：${newCode?.joinUrl}\n下载 CZL Browser：https://connect.czl.net/dl/czlbrowser/latest/windows`
              try {
                await navigator.clipboard.writeText(text)
                toast.success('邀请信息已复制')
              } catch {
                toast.error('复制失败')
              }
            }}
          >
            <Copy className="w-4 h-4" />
            复制邀请信息
          </Button>
        </div>
      </Modal>
      <PurchaseModal
        target={purchase}
        onClose={() => {
          setPurchase(null)
          void load()
        }}
      />
    </div>
  )
}

/** 团队数据端到端加密: 本机没开通 / 没解锁时团队空间只读, 在团队页顶部说明并给入口。 */
function TeamKeysNotice({ keys }: { keys: string }) {
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)
  if (keys !== 'not_enrolled' && keys !== 'locked') return null
  const enroll = async () => {
    setBusy(true)
    try {
      await startEnroll()
    } finally {
      setBusy(false)
    }
  }
  return (
    <Card>
      <div className="flex flex-wrap items-center gap-3">
        <ShieldCheck className="w-5 h-5 shrink-0 text-[var(--color-text-secondary)]" />
        <p className="flex-1 min-w-[16rem] text-sm text-[var(--color-text-secondary)]">
          {keys === 'not_enrolled'
            ? '团队数据端到端加密。你还没开启加密，团队空间暂时只读；你是所有者时，开启后才会生成团队密钥，成员才能收到。'
            : '这台电脑还没解锁加密数据，团队空间暂时只读。用恢复码或已登录的电脑解锁后即可使用。'}
        </p>
        {keys === 'not_enrolled' ? (
          <Button size="sm" onClick={enroll} loading={busy}>
            开启端到端加密
          </Button>
        ) : (
          <Button size="sm" variant="secondary" onClick={() => navigate('/cloud?tab=sync')}>
            去解锁
          </Button>
        )}
      </div>
    </Card>
  )
}

export function TeamSection() {
  const refresh = useCloudStore((s) => s.refresh)
  const keys = useCloudStore((s) => s.sync?.keys || 'unknown')
  const [orgs, setOrgs] = useState<Org[] | null>(null)
  const [selected, setSelected] = useState('')
  const [mode, setMode] = useState<'' | 'create' | 'join'>('')
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const list = await cloudApi.orgList()
      setOrgs(list)
      setSelected((cur) => (list.some((o) => o.id === cur) ? cur : list[0]?.id || ''))
    } catch (err) {
      setOrgs([])
      toast.error(errorText(err))
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const submit = async () => {
    if (!value.trim()) return
    setBusy(true)
    try {
      if (mode === 'create') {
        const org = await cloudApi.orgCreate(value.trim())
        setSelected(org.id)
        toast.success('团队已创建，左上角可切换到团队空间')
      } else {
        const res = await cloudApi.orgJoin(value.trim())
        setSelected(res.org.id)
        toast.success(res.alreadyMember ? '你已经在这个团队里' : `已加入「${res.org.name}」`)
      }
      setMode('')
      setValue('')
      await load()
      void refresh()
      // 建团队 / 入团队时还没开启加密: 直接引导开启, 否则团队空间只读、所有者不开启成员也拿不到团队密钥
      if (useCloudStore.getState().sync?.keys === 'not_enrolled') {
        const ok = await askConfirm({
          title: '开启端到端加密',
          content: '团队数据在本机加密后才上传。开启后才能使用团队空间，并会生成一个只显示一次的恢复码。',
          confirmText: '现在开启',
        })
        if (ok) await startEnroll()
      }
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const current = orgs?.find((o) => o.id === selected)

  return (
    <div className="space-y-5">
      <TeamKeysNotice keys={keys} />
      <div className="flex flex-wrap items-center gap-2">
        {(orgs || []).map((o) => (
          <button
            key={o.id}
            onClick={() => setSelected(o.id)}
            className={clsx(
              'flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm transition-colors',
              o.id === selected
                ? 'border-[var(--color-text-primary)] bg-[var(--color-bg-muted)] text-[var(--color-text-primary)]'
                : 'border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            )}
          >
            <Users className="w-4 h-4" />
            {o.name}
          </button>
        ))}
        <Button size="sm" variant="secondary" onClick={() => setMode('create')}>
          <Plus className="w-4 h-4" />
          创建团队
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setMode('join')}>
          用邀请码加入
        </Button>
      </div>

      {orgs && orgs.length === 0 && !mode && (
        <Card>
          <p className="text-sm text-[var(--color-text-secondary)]">
            创建团队后，团队成员可以共享实例、代理与登录态，正在使用的实例会提醒其他人。
          </p>
        </Card>
      )}

      {current && <TeamDetail key={current.id} org={current} onChanged={load} />}

      <Modal
        open={!!mode}
        onClose={() => {
          setMode('')
          setValue('')
        }}
        title={mode === 'create' ? '创建团队' : '用邀请码加入团队'}
        width="420px"
        footer={
          <Button onClick={submit} loading={busy} disabled={!value.trim()}>
            {mode === 'create' ? '创建' : '加入'}
          </Button>
        }
      >
        <Input
          autoFocus
          value={value}
          placeholder={mode === 'create' ? '团队名称，例如：运营一组' : '12 位邀请码'}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && void submit()}
        />
      </Modal>
    </div>
  )
}
