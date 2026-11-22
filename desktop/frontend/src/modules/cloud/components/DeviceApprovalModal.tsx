import { useState } from 'react'
import { Laptop } from 'lucide-react'
import { Button, Modal, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { cloudApi, type DeviceRequest } from '../api'
import { errorText } from '../errors'

// 已解锁的电脑上有新设备请求授权时弹出: 先算出 6 位码给用户核对, 一致才批准 (防止服务端替换公钥)。

export function DeviceApprovalModal() {
  const requests = useCloudStore((s) => s.deviceRequests)
  const setRequests = useCloudStore((s) => s.setDeviceRequests)
  const [dismissed, setDismissed] = useState<Record<string, boolean>>({})
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState<'' | 'begin' | 'approve' | 'reject'>('')

  const req: DeviceRequest | undefined = requests.find((r) => !dismissed[r.id])
  const close = (id: string) => {
    setDismissed((d) => ({ ...d, [id]: true }))
    setCode('')
  }
  const drop = (id: string) => {
    setRequests(requests.filter((r) => r.id !== id))
    setCode('')
  }

  const begin = async () => {
    if (!req) return
    setBusy('begin')
    try {
      setCode(await cloudApi.deviceApproveBegin(req.id))
    } catch (err) {
      toast.error(errorText(err))
      drop(req.id)
    } finally {
      setBusy('')
    }
  }
  const decide = async (approve: boolean) => {
    if (!req) return
    setBusy(approve ? 'approve' : 'reject')
    try {
      await cloudApi.deviceApprove(req.id, approve)
      toast.success(approve ? '已批准，新电脑可以同步了' : '已拒绝')
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setBusy('')
      drop(req.id)
    }
  }

  if (!req) return null
  return (
    <Modal
      open
      onClose={() => close(req.id)}
      title="新电脑请求解锁云端数据"
      width="440px"
      footer={
        code ? (
          <>
            <Button variant="secondary" onClick={() => decide(false)} loading={busy === 'reject'}>
              不一致，拒绝
            </Button>
            <Button onClick={() => decide(true)} loading={busy === 'approve'}>
              一致，批准
            </Button>
          </>
        ) : (
          <>
            <Button variant="secondary" onClick={() => decide(false)} loading={busy === 'reject'}>
              拒绝
            </Button>
            <Button onClick={begin} loading={busy === 'begin'}>
              继续核对
            </Button>
          </>
        )
      }
    >
      <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
        <div className="flex items-center gap-3 rounded-lg bg-[var(--color-bg-muted)] p-3">
          <Laptop className="w-5 h-5 text-[var(--color-text-secondary)]" />
          <span className="text-[var(--color-text-primary)]">{req.deviceName || '未命名电脑'}</span>
        </div>
        {code ? (
          <>
            <p>新电脑上会显示一个 6 位数字，与下面的一致才批准：</p>
            <div className="text-center font-mono text-3xl tracking-[0.4em] text-[var(--color-text-primary)]">{code}</div>
          </>
        ) : (
          <p>批准后，这台电脑能读取你的全部云端数据。不是你本人操作的请拒绝。</p>
        )}
      </div>
    </Modal>
  )
}
