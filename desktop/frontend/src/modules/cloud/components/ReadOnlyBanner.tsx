import { Lock } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useCloudStore } from '../../../store/cloudStore'
import { spaceLabel } from '../api'

// 当前空间只读时在内容区顶部提示原因, 缺订阅时给购买入口。

export function ReadOnlyBanner() {
  const navigate = useNavigate()
  const current = useCloudStore((s) => s.currentSpace())
  const needs = useCloudStore((s) => s.needs)
  const keys = useCloudStore((s) => s.sync?.keys)
  if (!current) return null

  let text = ''
  let action: { label: string; to: string } | null = null
  if (keys === 'not_enrolled') {
    text = '还没开启端到端加密，同步空间暂时只读。开启后才能新建和修改，并在电脑与团队成员之间同步。'
    action = { label: '去开启', to: '/cloud?tab=sync' }
  } else if (keys === 'locked') {
    text = '这台电脑还没解锁云端数据，看到的是本机缓存，暂时只读。用恢复码或已登录的电脑解锁后即可修改。'
    action = { label: '去解锁', to: '/cloud?tab=sync' }
  } else if (current.removed) {
    text = `你已不在「${spaceLabel(current)}」，这里是只读副本。`
    action = { label: '管理', to: '/cloud?tab=sync' }
  } else if (current.keyPending) {
    text = '团队密钥还没授予你，等团队所有者或管理员打开软件后自动授予。'
  } else if (current.readOnly) {
    const need = needs[current.key]
    if (need === 'subscribe') {
      text = `「${spaceLabel(current)}」没有有效订阅，数据只读，修改不会上传。`
      action = { label: '开通订阅', to: '/cloud?tab=plans' }
    } else if (need === 'seats') {
      text = '团队成员数超过了席位，数据只读。'
      action = { label: '加席位', to: '/cloud?tab=team' }
    } else {
      text = current.role === 'viewer' ? '你是只读成员，不能修改团队数据。' : '当前空间只读。'
    }
  }
  if (!text) return null

  return (
    <div className="mb-3 shrink-0 flex items-center gap-2.5 rounded-md border border-[var(--color-warning)]/30 bg-[var(--color-warning-soft)] px-3 py-2 text-xs text-[var(--color-text-secondary)]">
      <Lock className="w-3.5 h-3.5 shrink-0 text-[var(--color-warning)]" />
      <span className="flex-1">{text}</span>
      {action && (
        <button onClick={() => navigate(action!.to)} className="shrink-0 font-medium text-[var(--color-text-primary)] underline-offset-2 hover:underline">
          {action.label}
        </button>
      )}
    </div>
  )
}
