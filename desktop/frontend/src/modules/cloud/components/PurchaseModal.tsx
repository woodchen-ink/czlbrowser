import { useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import { ExternalLink, Loader2 } from 'lucide-react'
import { Button, Modal, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { cloudApi, formatPrice, type AddOn, type Plan, type PlanQuote } from '../api'
import { errorText } from '../errors'

// 购买 / 续费 / 升级套餐, 或加购 (席位、容量)。付款在系统浏览器的 Connect 结账页完成:
// 打开前后端记下权益指纹, 这里每 4 秒、回到窗口时查一次, 指纹变了即到账; 15 分钟后停止自动检查。

const POLL_MS = 4000
const POLL_LIMIT_MS = 15 * 60 * 1000

type Target =
  | { kind: 'plan'; plan: Plan; subject: string; subjectName: string }
  | { kind: 'addon'; addon: AddOn; subject: string; subjectName: string }

function useWaitPaid(subject: string, onPaid: () => void) {
  const [fingerprint, setFingerprint] = useState('')
  const [checking, setChecking] = useState(false)
  const started = useRef(0)

  const check = async (manual = false) => {
    if (!fingerprint) return
    setChecking(true)
    try {
      const paid = await cloudApi.billingCheckPaid(subject, fingerprint)
      if (paid) {
        onPaid()
      } else if (manual) {
        toast.info('还没查到付款，稍等片刻再试')
      }
    } catch (err) {
      if (manual) toast.error(errorText(err))
    } finally {
      setChecking(false)
    }
  }

  useEffect(() => {
    if (!fingerprint) return
    started.current = Date.now()
    const t = window.setInterval(() => {
      if (Date.now() - started.current > POLL_LIMIT_MS) {
        window.clearInterval(t)
        return
      }
      void check()
    }, POLL_MS)
    const onFocus = () => void check()
    window.addEventListener('focus', onFocus)
    return () => {
      window.clearInterval(t)
      window.removeEventListener('focus', onFocus)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fingerprint])

  return { fingerprint, setFingerprint, checking, check }
}

export function PurchaseModal({ target, onClose }: { target: Target | null; onClose: () => void }) {
  const refreshBilling = useCloudStore((s) => s.refreshBilling)
  const refresh = useCloudStore((s) => s.refresh)
  const [quote, setQuote] = useState<PlanQuote | null>(null)
  const [periods, setPeriods] = useState(1)
  const [qty, setQty] = useState(1)
  const [loading, setLoading] = useState(false)
  const [opening, setOpening] = useState(false)

  const subject = target?.subject || 'user'
  const wait = useWaitPaid(subject, () => {
    toast.success('付款已到账，同步已恢复')
    void refreshBilling(true)
    void refresh()
    onClose()
  })

  useEffect(() => {
    setQuote(null)
    setPeriods(1)
    setQty(1)
    wait.setFingerprint('')
    if (target?.kind !== 'plan') return
    setLoading(true)
    cloudApi
      .billingQuote(target.plan.code, target.subject)
      .then(setQuote)
      .catch((err) => toast.error(errorText(err)))
      .finally(() => setLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target])

  if (!target) return null

  const open = async () => {
    setOpening(true)
    try {
      const fp =
        target.kind === 'plan'
          ? await cloudApi.billingOpenCheckout(target.plan.code, target.subject, periods)
          : await cloudApi.billingOpenAddOnCheckout(target.addon.code, target.subject, qty)
      wait.setFingerprint(fp)
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setOpening(false)
    }
  }

  const title = target.kind === 'plan' ? `${target.plan.name} · ${target.subjectName}` : `${target.addon.name} · ${target.subjectName}`
  const option = quote?.options.find((o) => o.periods === periods) || quote?.options[0]

  return (
    <Modal
      open
      onClose={onClose}
      title={title}
      width="460px"
      footer={
        wait.fingerprint ? (
          <>
            <Button variant="ghost" onClick={onClose}>
              取消
            </Button>
            <Button variant="secondary" onClick={open} loading={opening}>
              重新打开付款页
            </Button>
            <Button onClick={() => void wait.check(true)} loading={wait.checking}>
              我已完成付款
            </Button>
          </>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose}>
              取消
            </Button>
            <Button onClick={open} loading={opening} disabled={target.kind === 'plan' && !option}>
              <ExternalLink className="w-4 h-4" />
              去付款
            </Button>
          </>
        )
      }
    >
      {wait.fingerprint ? (
        <div className="flex flex-col items-center gap-3 py-4 text-sm text-[var(--color-text-secondary)]">
          <Loader2 className="w-6 h-6 animate-spin text-[var(--color-text-muted)]" />
          <p>已在浏览器打开付款页，付款完成后这里会自动更新。</p>
        </div>
      ) : target.kind === 'plan' ? (
        loading || !quote ? (
          <div className="py-6 text-center text-sm text-[var(--color-text-muted)]">正在获取价格…</div>
        ) : (
          <div className="space-y-3">
            {quote.kind === 'upgrade' && (
              <p className="text-sm text-[var(--color-text-secondary)]">升级只需补齐本期差价，到期日不变。</p>
            )}
            <div className="space-y-2">
              {quote.options.map((o) => (
                <button
                  key={o.periods}
                  onClick={() => setPeriods(o.periods)}
                  className={clsx(
                    'w-full flex items-center justify-between rounded-lg border px-4 py-3 text-left transition-colors',
                    o.periods === (option?.periods || 1)
                      ? 'border-[var(--color-text-primary)] bg-[var(--color-bg-muted)]'
                      : 'border-[var(--color-border-default)] hover:bg-[var(--color-bg-muted)]'
                  )}
                >
                  <span className="text-sm text-[var(--color-text-primary)]">
                    {o.periods} 年
                    {o.percentOff > 0 && <span className="ml-2 text-xs text-[var(--color-text-muted)]">省 {o.percentOff}%</span>}
                  </span>
                  <span className="text-sm">
                    {o.savedCents > 0 && (
                      <span className="mr-2 text-xs text-[var(--color-text-muted)] line-through">{formatPrice(o.listCents, quote.currency)}</span>
                    )}
                    <span className="font-semibold text-[var(--color-text-primary)]">{formatPrice(o.amountCents, quote.currency)}</span>
                  </span>
                </button>
              ))}
            </div>
            <p className="text-xs text-[var(--color-text-muted)]">支付宝、微信支付或银行卡，优惠券在付款页填写。</p>
          </div>
        )
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <span className="text-sm text-[var(--color-text-secondary)]">份数</span>
            <div className="flex items-center gap-2">
              <Button size="sm" variant="secondary" onClick={() => setQty((q) => Math.max(1, q - 1))}>
                −
              </Button>
              <span className="w-8 text-center text-sm">{qty}</span>
              <Button size="sm" variant="secondary" onClick={() => setQty((q) => Math.min(target.addon.remaining || 99, q + 1))}>
                +
              </Button>
            </div>
          </div>
          <div className="rounded-lg bg-[var(--color-bg-muted)] px-4 py-3 text-sm space-y-1">
            <div className="flex justify-between">
              <span className="text-[var(--color-text-secondary)]">本期应付（按剩余天数折算）</span>
              <span className="font-semibold text-[var(--color-text-primary)]">
                {formatPrice(target.addon.proratedUnitCents * qty, target.addon.currency)}
              </span>
            </div>
            <div className="flex justify-between text-xs text-[var(--color-text-muted)]">
              <span>续费后每年</span>
              <span>{formatPrice(target.addon.priceCents * qty, target.addon.currency)}</span>
            </div>
          </div>
        </div>
      )}
    </Modal>
  )
}

export type PurchaseTarget = Target
