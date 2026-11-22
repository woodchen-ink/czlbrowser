import { useMemo, useState } from 'react'
import { Check, ExternalLink } from 'lucide-react'
import { Badge, Button, Card, Input, Select, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { cloudApi, formatPrice, subjectOf, type AddOn, type Entitlement, type Plan } from '../api'
import { errorText } from '../errors'
import { AddOnRenewalLines } from './AddOnRenewal'
import { PurchaseModal, type PurchaseTarget } from './PurchaseModal'

// 套餐与订阅: 付费的是云端同步。个人同步 (personal) 买给自己, 团队版 (team / team-plus) 买给团队。
// 价格、容量、席位都取 Connect 目录, 客户端不写死。

function storageGB(quotas: Record<string, any> | undefined): number {
  const v = Number(quotas?.syncStorageGB)
  return Number.isFinite(v) ? v : 0
}

function EntitlementLine({ ent }: { ent: Entitlement | null | undefined }) {
  if (!ent || !ent.plan) return <span className="text-sm text-[var(--color-text-muted)]">未开通</span>
  const end = ent.periodEnd ? new Date(ent.periodEnd).toLocaleDateString('zh-CN') : ''
  return (
    <span className="flex flex-wrap items-center gap-2 text-sm text-[var(--color-text-secondary)]">
      <span className="text-[var(--color-text-primary)] font-medium">{ent.planName || ent.plan}</span>
      {ent.active ? <Badge size="sm" variant="success">生效中</Badge> : <Badge size="sm" variant="warning">已到期</Badge>}
      {end && <span>到期 {end}</span>}
      {storageGB(ent.quotas) > 0 && <span>{storageGB(ent.quotas)} GB</span>}
      {ent.seats > 0 && <span>{ent.seats} 席</span>}
    </span>
  )
}

/** 兑换码: 个人套餐码兑给自己, 团队套餐码兑给自己管理的团队。 */
function RedeemRow({ orgs }: { orgs: { orgId: string; name: string }[] }) {
  const [code, setCode] = useState('')
  const [subject, setSubject] = useState(subjectOf())
  const [busy, setBusy] = useState(false)

  const redeem = async () => {
    if (!code.trim()) return
    setBusy(true)
    try {
      await cloudApi.billingRedeem(code, subject)
      setCode('')
      // 订阅状态由后端等权益生效后推送 billing:changed
      toast.success('兑换成功，订阅状态稍后自动更新')
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-[var(--color-border-muted)] pt-3">
      <span className="text-sm text-[var(--color-text-secondary)] w-24 shrink-0">兑换码</span>
      <Input
        value={code}
        placeholder="XXXX-XXXX-XXXX-XXXX"
        className="font-mono w-64"
        onChange={(e) => setCode(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && void redeem()}
      />
      {orgs.length > 0 && (
        <div className="w-40 shrink-0">
          <Select
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            options={[{ value: subjectOf(), label: '我的同步' }, ...orgs.map((o) => ({ value: subjectOf(o.orgId), label: o.name }))]}
          />
        </div>
      )}
      <Button size="sm" variant="secondary" onClick={redeem} loading={busy} disabled={!code.trim()}>
        兑换
      </Button>
    </div>
  )
}

/** 加购按钮 (席位类在团队页加购, 这里只放容量); 已买份数显示在下方的加购明细里。 */
function AddOnAction({ addon, onBuy }: { addon: AddOn; onBuy: () => void }) {
  if (addon.remaining <= 0) return null
  return (
    <Button size="sm" variant="secondary" onClick={onBuy}>
      加容量
    </Button>
  )
}

export function PlansSection({ onGoTeam }: { onGoTeam: () => void }) {
  const billing = useCloudStore((s) => s.billing)
  const [target, setTarget] = useState<PurchaseTarget | null>(null)
  const [orgId, setOrgId] = useState('')

  const ent = billing?.entitlements
  // 个人同步生效时目录带回可加购的项 (含已买份数); 没订阅时为空
  const personalAddOns: AddOn[] = ent?.personal?.active ? billing?.addons || [] : []
  const storageAddOn = personalAddOns.find((a) => a.target !== 'seats')
  const manageableOrgs = useMemo(
    () => (ent?.orgs || []).filter((o) => o.role === 'owner' || o.role === 'admin'),
    [ent]
  )
  const selectedOrg = manageableOrgs.find((o) => o.orgId === orgId) || manageableOrgs[0]

  if (billing && billing.loaded && !billing.enabled) {
    return (
      <Card title="云端同步">
        <p className="text-sm text-[var(--color-text-secondary)]">云端同步目前免费使用，开通加密后即可在电脑之间、团队成员之间同步实例。</p>
      </Card>
    )
  }

  const plans: Plan[] = billing?.plans || []
  const personalPlans = plans.filter((p) => p.subjectType === 'user')
  const teamPlans = plans.filter((p) => p.subjectType === 'org')

  const buy = (plan: Plan) => {
    if (plan.subjectType === 'user') {
      setTarget({ kind: 'plan', plan, subject: subjectOf(), subjectName: '我的同步' })
      return
    }
    if (!selectedOrg) {
      onGoTeam()
      return
    }
    setTarget({ kind: 'plan', plan, subject: subjectOf(selectedOrg.orgId), subjectName: selectedOrg.name })
  }

  const actionLabel = (plan: Plan, current?: Entitlement | null) => {
    if (!current?.plan) return '开通'
    if (current.plan === plan.code) return current.active ? '续费' : '续费恢复'
    return '改为此套餐'
  }

  const planCard = (plan: Plan, current?: Entitlement | null) => {
    const isCurrent = current?.plan === plan.code
    return (
      <div key={plan.code} className="flex flex-col rounded-xl border border-[var(--color-border-default)] p-5">
        <div className="flex items-center justify-between">
          <span className="text-base font-semibold text-[var(--color-text-primary)]">{plan.name}</span>
          {isCurrent && <Badge size="sm" variant="info">当前</Badge>}
        </div>
        <div className="mt-2 text-2xl font-semibold text-[var(--color-text-primary)]">
          {formatPrice(plan.priceCents, plan.currency)}
          <span className="text-sm font-normal text-[var(--color-text-muted)]"> / 年</span>
        </div>
        <ul className="mt-4 flex-1 space-y-2 text-sm text-[var(--color-text-secondary)]">
          <li className="flex gap-2"><Check className="w-4 h-4 shrink-0 mt-0.5" />{storageGB(plan.quotas) || '—'} GB 同步空间</li>
          {plan.subjectType === 'org' && (
            <li className="flex gap-2"><Check className="w-4 h-4 shrink-0 mt-0.5" />{plan.seats} 个成员席位</li>
          )}
          <li className="flex gap-2"><Check className="w-4 h-4 shrink-0 mt-0.5" />实例配置、代理与登录态端到端加密同步</li>
          {plan.subjectType === 'org' && (
            <li className="flex gap-2"><Check className="w-4 h-4 shrink-0 mt-0.5" />团队共享实例，使用中提醒</li>
          )}
        </ul>
        <Button className="mt-5" variant={isCurrent ? 'secondary' : 'primary'} onClick={() => buy(plan)}>
          {plan.subjectType === 'org' && !selectedOrg ? '先创建团队' : actionLabel(plan, current)}
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      <Card
        title="我的订阅"
        actions={
          <Button size="sm" variant="ghost" onClick={() => void cloudApi.billingOpenManage()}>
            <ExternalLink className="w-3.5 h-3.5" />
            管理订阅
          </Button>
        }
      >
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-4">
            <span className="text-sm text-[var(--color-text-secondary)] w-24 shrink-0">我的同步</span>
            <div className="flex-1"><EntitlementLine ent={ent?.personal} /></div>
            {storageAddOn && (
              <AddOnAction
                addon={storageAddOn}
                onBuy={() => setTarget({ kind: 'addon', addon: storageAddOn, subject: subjectOf(), subjectName: '我的同步' })}
              />
            )}
          </div>
          {ent?.personal?.active && (
            <AddOnRenewalLines subject={subjectOf()} addons={ent.personal.addons} periodEnd={ent.personal.periodEnd} canEdit />
          )}
          {(ent?.orgs || []).map((o) => {
            const manage = o.role === 'owner' || o.role === 'admin'
            return (
              <div key={o.orgId} className="space-y-1.5">
                <div className="flex items-center justify-between gap-4">
                  <span className="text-sm text-[var(--color-text-secondary)] w-24 shrink-0 truncate">{o.name}</span>
                  <div className="flex-1"><EntitlementLine ent={o.entitlement} /></div>
                  {o.entitlement?.active && manage && (
                    <Button size="sm" variant="ghost" onClick={onGoTeam}>
                      加席位 / 容量
                    </Button>
                  )}
                </div>
                {o.entitlement?.active && (
                  <AddOnRenewalLines
                    subject={subjectOf(o.orgId)}
                    addons={o.entitlement.addons}
                    periodEnd={o.entitlement.periodEnd}
                    canEdit={manage}
                  />
                )}
              </div>
            )
          })}
          {billing?.error && <p className="text-xs text-[var(--color-text-muted)]">订阅信息暂时取不到，稍后自动重试。</p>}
          <RedeemRow orgs={manageableOrgs} />
        </div>
      </Card>

      {personalPlans.length > 0 && (
        <Card title="个人" subtitle="自己的多台电脑之间同步">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{personalPlans.map((p) => planCard(p, ent?.personal))}</div>
        </Card>
      )}

      {teamPlans.length > 0 && (
        <Card
          title="团队"
          subtitle="团队成员共享实例与代理，按团队付费"
          actions={
            manageableOrgs.length > 1 ? (
              <Select
                value={selectedOrg?.orgId || ''}
                onChange={(e) => setOrgId(e.target.value)}
                options={manageableOrgs.map((o) => ({ value: o.orgId, label: o.name }))}
              />
            ) : undefined
          }
        >
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {teamPlans.map((p) => planCard(p, selectedOrg?.entitlement))}
          </div>
        </Card>
      )}

      <Card title="常见问题">
        <dl className="space-y-3 text-sm">
          <div>
            <dt className="font-medium text-[var(--color-text-primary)]">到期后数据会丢吗？</dt>
            <dd className="text-[var(--color-text-secondary)]">不会。到期后空间变为只读，仍可拉取与使用，续费后恢复上传。</dd>
          </div>
          <div>
            <dt className="font-medium text-[var(--color-text-primary)]">CZL 能看到我的数据吗？</dt>
            <dd className="text-[var(--color-text-secondary)]">看不到。数据在你的电脑上加密后才上传，密钥只在你和团队成员的电脑上。</dd>
          </div>
          <div>
            <dt className="font-medium text-[var(--color-text-primary)]">本机数据需要付费吗？</dt>
            <dd className="text-[var(--color-text-secondary)]">不需要。「本机」空间的实例永远免费，只有同步空间按订阅计费。</dd>
          </div>
        </dl>
      </Card>

      <PurchaseModal target={target} onClose={() => setTarget(null)} />
    </div>
  )
}
