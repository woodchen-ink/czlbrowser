import { useState } from 'react'
import { Minus, Plus } from 'lucide-react'
import { Button, Modal, toast } from '../../../shared/components'
import type { billing } from '../../../wailsjs/go/models'
import { cloudApi } from '../api'
import { errorText } from '../errors'

// 订阅上已加购的项: 当前份数、下期份数, 以及「调整下期份数」。
// Connect 规则: 只能调低, 当前周期照常有效, 续费 (或自动续费扣款) 起按新份数计费; 调回当前份数即撤回。
// 增加份数走加购结账 (立即生效), 不在这里。

type EntAddOn = billing.EntitlementAddOn

function amountText(a: EntAddOn, qty: number): string {
  const total = qty * a.unitAmount
  if (a.target === 'seats') return `${total} 席`
  if (a.target === 'syncStorageGB') return `${total} GB`
  return `${total}`
}

function dateText(v: any): string {
  return v ? new Date(v).toLocaleDateString('zh-CN') : ''
}

export function AddOnRenewalLines({
  subject,
  addons,
  periodEnd,
  canEdit,
}: {
  subject: string
  addons: EntAddOn[] | undefined
  periodEnd?: any
  canEdit: boolean
}) {
  const [editing, setEditing] = useState<EntAddOn | null>(null)
  const list = (addons || []).filter((a) => a.quantity > 0)
  if (list.length === 0) return null

  return (
    <div className="space-y-1.5 pl-28">
      {list.map((a) => {
        const pending = a.pendingQuantity ?? null
        return (
          <div key={a.code} className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--color-text-secondary)]">
            <span>
              {a.name} × {a.quantity}（{amountText(a, a.quantity)}）
            </span>
            {pending !== null && pending !== a.quantity && (
              <span className="text-[var(--color-text-muted)]">
                {dateText(a.pendingEffectiveAt || periodEnd) || '下期'}起 {pending} 份
              </span>
            )}
            {canEdit && (
              <button
                onClick={() => setEditing(a)}
                className="font-medium text-[var(--color-accent)] hover:underline"
              >
                调整下期份数
              </button>
            )}
          </div>
        )
      })}
      <AdjustModal subject={subject} addon={editing} periodEnd={periodEnd} onClose={() => setEditing(null)} />
    </div>
  )
}

function AdjustModal({
  subject,
  addon,
  periodEnd,
  onClose,
}: {
  subject: string
  addon: EntAddOn | null
  periodEnd?: any
  onClose: () => void
}) {
  const [qty, setQty] = useState(0)
  const [busy, setBusy] = useState(false)
  const [openedFor, setOpenedFor] = useState<string>('')

  // 每次打开时从「下期份数」(没调过就是当前份数) 开始
  if (addon && openedFor !== addon.code) {
    setOpenedFor(addon.code)
    setQty(addon.pendingQuantity ?? addon.quantity)
  }
  if (!addon && openedFor) setOpenedFor('')
  if (!addon) return null

  const save = async () => {
    setBusy(true)
    try {
      await cloudApi.billingSetAddOnQuantity(subject, addon.code, qty)
      toast.success(qty === addon.quantity ? '已撤回调整，下期份数不变' : `下期起改为 ${qty} 份`)
      onClose()
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const end = dateText(addon.pendingEffectiveAt || periodEnd)
  const unchanged = qty === (addon.pendingQuantity ?? addon.quantity)

  return (
    <Modal
      open
      onClose={onClose}
      title={`调整下期份数 · ${addon.name}`}
      width="420px"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            取消
          </Button>
          <Button onClick={save} loading={busy} disabled={unchanged}>
            保存
          </Button>
        </>
      }
    >
      <div className="space-y-4 text-sm text-[var(--color-text-secondary)]">
        <div className="flex items-center justify-between">
          <span>下期份数</span>
          <div className="flex items-center gap-2">
            <Button size="sm" variant="secondary" onClick={() => setQty((q) => Math.max(0, q - 1))} disabled={qty <= 0}>
              <Minus className="w-3.5 h-3.5" />
            </Button>
            <span className="w-16 text-center font-medium text-[var(--color-text-primary)]">
              {qty} 份
            </span>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => setQty((q) => Math.min(addon.quantity, q + 1))}
              disabled={qty >= addon.quantity}
            >
              <Plus className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
        <p className="text-xs text-[var(--color-text-muted)]">
          当前 {addon.quantity} 份（{amountText(addon, addon.quantity)}）在本期照常有效
          {end ? `，${end} 续费起` : '，下次续费起'}按新份数计费。只能调低，想增加请用加购；调回 {addon.quantity} 份即撤回调整。
        </p>
      </div>
    </Modal>
  )
}
