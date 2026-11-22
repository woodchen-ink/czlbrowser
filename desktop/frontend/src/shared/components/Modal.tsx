import { useEffect, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { Button } from './Button'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  footer?: ReactNode
  width?: string
  /** 为 false 时没有关闭按钮, 点遮罩也不关闭 */
  closable?: boolean
  /** 内容区的内边距等, 需要自己排版 (如左右分栏) 时传 'p-0' */
  bodyClassName?: string
  children: ReactNode
}

/** 居中弹窗。打开期间锁住页面滚动, 内容超高时弹窗内部滚动。 */
export function Modal({ open, onClose, title, footer, width = '500px', closable = true, bodyClassName = 'px-6 py-4', children }: ModalProps) {
  useEffect(() => {
    if (!open) return
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 animate-fade-in bg-[var(--color-overlay)] backdrop-blur-sm" onClick={closable ? onClose : undefined} />
      <div
        role="dialog"
        aria-modal="true"
        className="relative flex max-h-[90vh] w-full animate-scale-in flex-col rounded-xl bg-[var(--color-bg-elevated)] shadow-2xl"
        style={{ width, maxWidth: '90vw' }}
      >
        {(title || closable) && (
          <div className="flex shrink-0 items-center justify-between border-b border-[var(--color-border)] px-6 py-4">
            {title && <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">{title}</h3>}
            {closable && (
              <button
                onClick={onClose}
                aria-label="关闭"
                className="ml-auto rounded-lg p-1.5 text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
              >
                <X className="h-5 w-5" />
              </button>
            )}
          </div>
        )}
        <div className={`min-h-0 flex-1 overflow-y-auto ${bodyClassName}`}>{children}</div>
        {footer && <div className="flex shrink-0 items-center justify-end gap-3 border-t border-[var(--color-border)] px-6 py-4">{footer}</div>}
      </div>
    </div>
  )
}

interface ConfirmModalProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title?: string
  content: ReactNode
  confirmText?: string
  cancelText?: string
  /** 确认按钮用红色 (删除等不可恢复操作) */
  danger?: boolean
}

/** 二次确认弹窗; 点确认先执行 onConfirm 再关闭。 */
export function ConfirmModal({ open, onClose, onConfirm, title = '确认', content, confirmText = '确定', cancelText = '取消', danger = false }: ConfirmModalProps) {
  const confirm = () => {
    onConfirm()
    onClose()
  }
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      width="400px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            {cancelText}
          </Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={confirm}>
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="text-[var(--color-text-secondary)]">{content}</div>
    </Modal>
  )
}
