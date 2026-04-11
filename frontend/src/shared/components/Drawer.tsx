import { ReactNode, useEffect } from 'react'
import { X } from 'lucide-react'
import clsx from 'clsx'

type DrawerPlacement = 'left' | 'right' | 'top' | 'bottom'

interface DrawerProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  footer?: ReactNode
  placement?: DrawerPlacement
  width?: string
  height?: string
  closable?: boolean
}

const placementStyles = {
  left: 'left-0 top-0 bottom-0 animate-slide-in-left',
  right: 'right-0 top-0 bottom-0 animate-slide-in-right',
  top: 'top-0 left-0 right-0 animate-slide-in-top',
  bottom: 'bottom-0 left-0 right-0 animate-slide-in-bottom',
}

export function Drawer({
  open,
  onClose,
  title,
  children,
  footer,
  placement = 'right',
  width = '400px',
  height = '300px',
  closable = true,
}: DrawerProps) {
  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  if (!open) return null

  const isHorizontal = placement === 'left' || placement === 'right'
  const size = isHorizontal ? { width } : { height }

  return (
    <div className="fixed inset-0 z-50">
      {/* 遮罩层 */}
      <div
        className="absolute inset-0 backdrop-blur-sm animate-fade-in"
        style={{ backgroundColor: 'var(--color-overlay)' }}
        onClick={closable ? onClose : undefined}
      />

      {/* 抽屉内容 */}
      <div
        className={clsx(
          'absolute bg-[var(--color-bg-elevated)] shadow-2xl flex flex-col',
          placementStyles[placement]
        )}
        style={size}
        onClick={(e) => e.stopPropagation()}
      >
        {/* 标题栏 */}
        {(title || closable) && (
          <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)] flex-shrink-0">
            {title && (
              <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">
                {title}
              </h3>
            )}
            {closable && (
              <button
                onClick={onClose}
                className="p-1.5 rounded-lg text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] transition-colors ml-auto"
              >
                <X className="w-5 h-5" />
              </button>
            )}
          </div>
        )}

        {/* 内容区 */}
        <div className="flex-1 overflow-y-auto px-6 py-4 min-h-0">
          {children}
        </div>

        {/* 底部按钮 */}
        {footer && (
          <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-[var(--color-border)] flex-shrink-0">
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}
