import { AlertCircle, CheckCircle, Info, X, XCircle } from 'lucide-react'
import { create } from 'zustand'

type Kind = 'success' | 'error' | 'warning' | 'info'

interface ToastItemData {
  id: number
  type: Kind
  message: string
  /** 毫秒; 0 表示不自动消失 */
  duration?: number
}

interface ToastState {
  toasts: ToastItemData[]
  addToast: (t: Omit<ToastItemData, 'id'>) => void
  removeToast: (id: number) => void
}

let nextId = 0

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  addToast: (t) => {
    const id = ++nextId
    set({ toasts: [...get().toasts, { ...t, id }] })
    const ms = t.duration ?? 3000
    if (ms > 0) window.setTimeout(() => get().removeToast(id), ms)
  },
  removeToast: (id) => set({ toasts: get().toasts.filter((t) => t.id !== id) }),
}))

const show = (type: Kind) => (message: string, duration?: number) => useToastStore.getState().addToast({ type, message, duration })

/** 右上角的轻提示: toast.success('已保存') */
export const toast = { success: show('success'), error: show('error'), warning: show('warning'), info: show('info') }

const look: Record<Kind, { icon: typeof Info; color: string }> = {
  success: { icon: CheckCircle, color: 'var(--color-success)' },
  error: { icon: XCircle, color: 'var(--color-error)' },
  warning: { icon: AlertCircle, color: 'var(--color-warning)' },
  info: { icon: Info, color: 'var(--color-accent)' },
}

export function ToastContainer() {
  const { toasts, removeToast } = useToastStore()
  return (
    <div className="fixed right-4 top-4 z-50 flex max-w-md flex-col gap-2">
      {toasts.map((t) => {
        const { icon: Icon, color } = look[t.type]
        return (
          <div
            key={t.id}
            role="status"
            className="flex animate-slide-in-right items-start gap-3 rounded-lg border bg-[var(--color-bg-surface)] px-4 py-3 shadow-lg"
            style={{ color, borderColor: `color-mix(in srgb, ${color} 30%, transparent)` }}
          >
            <Icon className="mt-0.5 h-5 w-5 shrink-0" />
            <p className="flex-1 text-sm font-medium">{t.message}</p>
            <button onClick={() => removeToast(t.id)} aria-label="关闭" className="rounded p-0.5 transition-colors hover:bg-[var(--color-bg-muted)]">
              <X className="h-4 w-4" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
