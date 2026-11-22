import { useState } from 'react'
import { AlertCircle } from 'lucide-react'
import { ForceQuit, SetCloseAction } from '../../wailsjs/go/main/App'
import { WindowHide } from '../../wailsjs/runtime/runtime'
import { Button, Modal } from '../components'
import { useBackupStore } from '../../store/backupStore'
import { useEvents } from '../hooks/useRuntimeEvents'

/**
 * 关闭行为设为「每次询问」, 或正在加载备份时, 后端发 app:request-close, 这里让用户选择最小化到托盘或退出。
 * 勾选「记住」后写回设置, 以后不再询问 (设置 → 通用 可改回)。
 */
export function CloseConfirmModal() {
  const [open, setOpen] = useState(false)
  const [remember, setRemember] = useState(false)
  const run = useBackupStore((s) => s.importing)
  const importing = run !== null
  useEvents(
    {
      'app:request-close': () => {
        setRemember(false)
        setOpen(true)
      },
    },
    [],
  )

  // 加载备份时的确认只针对这一次, 不记住
  const keep = (action: 'tray' | 'quit') => (remember && !importing ? SetCloseAction(action).catch(() => undefined) : Promise.resolve())

  const quit = async () => {
    setOpen(false)
    await keep('quit')
    await ForceQuit()
  }
  const hide = async () => {
    setOpen(false)
    await keep('tray')
    WindowHide()
  }

  const tone = importing
    ? 'bg-[var(--color-warning-soft)] text-[var(--color-warning)]'
    : 'bg-[var(--color-error-soft)] text-[var(--color-error)]'

  return (
    <Modal open={open} onClose={() => setOpen(false)} title={importing ? '关闭应用确认' : '退出确认'} width="360px">
      <div className="flex flex-col items-center px-4 pb-6 pt-2 text-center">
        <span className={`mb-4 flex h-12 w-12 items-center justify-center rounded-full ${tone}`}>
          <AlertCircle className="h-6 w-6" />
        </span>
        <h3 className="mb-2 text-lg font-medium text-[var(--color-text-primary)]">
          {importing ? '正在加载中，是否关闭？' : '是否退出应用程序？'}
        </h3>
        <p className="mb-6 text-sm text-[var(--color-text-secondary)]">
          {importing ? (
            <>
              当前正在加载配置{run && run.progress > 0 ? ` (${run.progress}%)` : ''}。
              <br />
              {run?.message || '强制关闭会中断本次加载，是否仍要关闭应用？'}
            </>
          ) : (
            <>
              退出后将停止所有在此客户端运行的服务，
              <br />
              如果您需要保持服务运行，请选择「最小化到托盘」。
            </>
          )}
        </p>
        {!importing && (
          <label className="-mt-2 mb-4 flex cursor-pointer select-none items-center gap-1.5 text-xs text-[var(--color-text-muted)]">
            <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} className="h-3.5 w-3.5 accent-[var(--color-button-primary)]" />
            记住我的选择, 不再询问
          </label>
        )}
        <div className="flex w-full gap-3">
          <Button variant="secondary" className="flex-1" onClick={importing ? () => setOpen(false) : hide}>
            {importing ? '继续加载' : '最小化到托盘'}
          </Button>
          <Button variant="danger" className="flex-1" onClick={quit}>
            {importing ? '仍要关闭' : '直接退出'}
          </Button>
        </div>
      </div>
    </Modal>
  )
}
