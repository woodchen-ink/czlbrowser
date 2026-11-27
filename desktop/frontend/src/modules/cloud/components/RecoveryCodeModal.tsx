import { useState } from 'react'
import { Copy, Download, ShieldAlert } from 'lucide-react'
import { Button, Modal, toast } from '../../../shared/components'
import { cloudApi } from '../api'
import { errorText } from '../errors'

// 恢复码只显示这一次: 要求用户复制或存成文件, 并勾选「已妥善保存」才能关闭。

export function RecoveryCodeModal({ code, onDone }: { code: string; onDone: () => void }) {
  const [saved, setSaved] = useState(false)
  const [confirmed, setConfirmed] = useState(false)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code)
      setSaved(true)
      toast.success('恢复码已复制')
    } catch {
      // 剪贴板不可用时允许手动抄写后确认, 否则只剩「存成文件」一条路
      setSaved(true)
      toast.error('复制失败，请手动抄写')
    }
  }
  const saveFile = async () => {
    try {
      if (await cloudApi.saveRecoveryCode(code)) {
        setSaved(true)
        toast.success('恢复码已保存')
      }
    } catch (err) {
      toast.error(errorText(err))
    }
  }

  return (
    <Modal
      open={!!code}
      onClose={() => {}}
      closable={false}
      title="保存你的恢复码"
      width="520px"
      footer={
        <Button disabled={!confirmed} onClick={onDone}>
          我已保存，继续
        </Button>
      }
    >
      <div className="space-y-4">
        <div className="flex gap-3 rounded-lg border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 p-3 text-sm text-[var(--color-text-secondary)]">
          <ShieldAlert className="w-5 h-5 shrink-0 text-[var(--color-warning)]" />
          <p>
            云端数据用只有你知道的密钥加密，CZL 看不到，也无法帮你找回。换电脑或重装后，需要这个恢复码（或一台已登录的电脑）才能解开数据。
          </p>
        </div>
        <div data-select-block className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-4 py-3 text-center font-mono text-base tracking-wider text-[var(--color-text-primary)] select-all break-all">
          {code}
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={copy}>
            <Copy className="w-4 h-4" />
            复制
          </Button>
          <Button variant="secondary" size="sm" onClick={saveFile}>
            <Download className="w-4 h-4" />
            存成文件
          </Button>
        </div>
        <label className="flex items-start gap-2 text-sm text-[var(--color-text-secondary)] cursor-pointer">
          <input
            type="checkbox"
            className="mt-0.5"
            checked={confirmed}
            disabled={!saved}
            onChange={(e) => setConfirmed(e.target.checked)}
          />
          <span>
            我已把恢复码存在安全的地方（密码管理器或离线文件）
            {!saved && <span className="block text-xs text-[var(--color-text-muted)]">先复制或存成文件</span>}
          </span>
        </label>
      </div>
    </Modal>
  )
}
