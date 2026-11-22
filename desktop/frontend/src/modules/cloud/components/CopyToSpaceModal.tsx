import { useState } from 'react'
import clsx from 'clsx'
import { HardDrive, User, Users } from 'lucide-react'
import { Button, Modal, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { cloudApi, LOCAL_SPACE, spaceLabel } from '../api'
import { errorText } from '../errors'

// 把选中的实例或代理复制到另一个空间 (复制不是移动, 换新 id)。实例可连同登录态一起复制。

export function CopyToSpaceModal({
  open,
  kind,
  ids,
  onClose,
}: {
  open: boolean
  kind: 'profiles' | 'proxies'
  ids: string[]
  onClose: () => void
}) {
  const sync = useCloudStore((s) => s.sync)
  const [target, setTarget] = useState('')
  const [withState, setWithState] = useState(true)
  const [busy, setBusy] = useState(false)

  const current = sync?.current || LOCAL_SPACE
  const keys = sync?.keys || 'unknown'
  const lockedHint = keys === 'not_enrolled' ? '未开启加密，不能复制进去' : '未解锁，不能复制进去'
  const options = [
    { key: LOCAL_SPACE, label: '本机', hint: '只在这台电脑', icon: HardDrive, disabled: false },
    ...(sync?.spaces || []).map((sp) => ({
      key: sp.key,
      label: spaceLabel(sp),
      hint:
        keys !== 'unlocked'
          ? lockedHint
          : sp.readOnly || sp.removed
            ? '只读，不能复制进去'
            : sp.kind === 'personal'
              ? '多台电脑同步'
              : '团队共享',
      icon: sp.kind === 'personal' ? User : Users,
      disabled: keys !== 'unlocked' || sp.readOnly || sp.removed,
    })),
  ].filter((o) => o.key !== current)

  const submit = async () => {
    if (!target) return
    setBusy(true)
    try {
      const res =
        kind === 'profiles' ? await cloudApi.copyProfiles(ids, target, withState) : await cloudApi.copyProxies(ids, target)
      const parts = []
      if (res.profiles) parts.push(`${res.profiles} 个实例`)
      if (res.proxies) parts.push(`${res.proxies} 个代理`)
      toast.success(`已复制 ${parts.join('、') || '0 项'}`)
      if (res.stateErrors?.length) {
        toast.warning(`这些实例的登录态没复制成功：${res.stateErrors.join('、')}`)
      }
      onClose()
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`复制 ${ids.length} 个${kind === 'profiles' ? '实例' : '代理'}到…`}
      width="440px"
      footer={
        <Button onClick={submit} loading={busy} disabled={!target}>
          复制
        </Button>
      }
    >
      <div className="space-y-3">
        {options.length === 0 && <p className="text-sm text-[var(--color-text-muted)]">没有其他空间。开通云端同步或加入团队后再来。</p>}
        {options.map((o) => {
          const Icon = o.icon
          return (
            <button
              key={o.key}
              disabled={o.disabled}
              onClick={() => setTarget(o.key)}
              className={clsx(
                'w-full flex items-center gap-3 rounded-lg border px-4 py-3 text-left transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
                target === o.key
                  ? 'border-[var(--color-text-primary)] bg-[var(--color-bg-muted)]'
                  : 'border-[var(--color-border-default)] hover:bg-[var(--color-bg-muted)]'
              )}
            >
              <Icon className="w-4 h-4 text-[var(--color-text-secondary)]" />
              <span className="flex-1">
                <span className="block text-sm text-[var(--color-text-primary)]">{o.label}</span>
                <span className="block text-xs text-[var(--color-text-muted)]">{o.hint}</span>
              </span>
            </button>
          )
        })}
        {kind === 'profiles' && (
          <label className="flex items-start gap-2 pt-1 text-sm text-[var(--color-text-secondary)] cursor-pointer">
            <input type="checkbox" className="mt-0.5" checked={withState} onChange={(e) => setWithState(e.target.checked)} />
            <span>
              连同登录态一起复制
              <span className="block text-xs text-[var(--color-text-muted)]">Cookie、本地存储等账号数据；实例需先关闭。实例引用的代理会一起复制。</span>
            </span>
          </label>
        )}
      </div>
    </Modal>
  )
}
