import { AlertCircle, Bell, Check, CheckCircle, Info, Trash2 } from 'lucide-react'
import clsx from 'clsx'
import { useNotificationStore, type Notification } from '../../store/notificationStore'

function TypeIcon({ type }: { type: Notification['type'] }) {
  switch (type) {
    case 'success':
      return <CheckCircle className="w-4 h-4 text-[var(--color-success)]" />
    case 'warning':
      return <AlertCircle className="w-4 h-4 text-[var(--color-warning)]" />
    case 'error':
      return <AlertCircle className="w-4 h-4 text-[var(--color-error)]" />
    default:
      return <Info className="w-4 h-4 text-[var(--color-text-muted)]" />
  }
}

/** 侧栏底部通知按钮弹出的面板 */
export function NotificationPanel({ onClose }: { onClose: () => void }) {
  const { notifications, markAsRead, markAllAsRead, clearNotifications } = useNotificationStore()
  const unread = notifications.filter((n) => !n.read).length

  return (
    <div className="absolute left-full bottom-0 ml-2 w-80 bg-[var(--color-bg-elevated)] border border-[var(--color-border-default)] rounded-lg shadow-[var(--shadow-lg)] overflow-hidden z-50 animate-fade-in">
      <div className="h-10 px-3 border-b border-[var(--color-border-muted)] flex items-center gap-2">
        <span className="text-sm font-medium text-[var(--color-text-primary)]">通知</span>
        {unread > 0 && <span className="text-xs text-[var(--color-text-muted)] tabular-nums">{unread} 条未读</span>}
        <div className="ml-auto flex items-center gap-0.5">
          {unread > 0 && (
            <button
              onClick={markAllAsRead}
              className="w-7 h-7 flex items-center justify-center rounded text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-accent-muted)]"
              title="全部标为已读"
              aria-label="全部标为已读"
            >
              <Check className="w-3.5 h-3.5" />
            </button>
          )}
          {notifications.length > 0 && (
            <button
              onClick={() => {
                clearNotifications()
                onClose()
              }}
              className="w-7 h-7 flex items-center justify-center rounded text-[var(--color-text-muted)] hover:text-[var(--color-error)] hover:bg-[var(--color-accent-muted)]"
              title="清空通知"
              aria-label="清空通知"
            >
              <Trash2 className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>
      <div className="max-h-80 overflow-y-auto">
        {notifications.length === 0 ? (
          <div className="py-8 text-center text-[var(--color-text-muted)]">
            <Bell className="w-6 h-6 mx-auto mb-2 opacity-50" />
            <p className="text-xs">暂无通知</p>
          </div>
        ) : (
          notifications.map((n) => (
            <button
              key={n.id}
              onClick={() => markAsRead(n.id)}
              className="w-full text-left flex gap-2.5 px-3 py-2.5 border-b border-[var(--color-border-muted)] last:border-0 hover:bg-[var(--color-accent-muted)]"
            >
              <span className="shrink-0 mt-0.5">
                <TypeIcon type={n.type} />
              </span>
              <span className="flex-1 min-w-0">
                <span
                  className={clsx(
                    'flex items-center gap-1.5 text-[13px] truncate',
                    n.read ? 'text-[var(--color-text-secondary)]' : 'text-[var(--color-text-primary)] font-medium'
                  )}
                >
                  {!n.read && <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-brand)] shrink-0" />}
                  <span className="truncate">{n.title}</span>
                </span>
                <span className="block text-xs text-[var(--color-text-muted)] mt-0.5 line-clamp-2">{n.message}</span>
                <span className="block text-[11px] text-[var(--color-text-muted)] mt-0.5">{n.time}</span>
              </span>
            </button>
          ))
        )}
      </div>
    </div>
  )
}
