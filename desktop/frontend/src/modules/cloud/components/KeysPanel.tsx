import { useEffect, useState } from 'react'
import { KeyRound, Laptop, ShieldCheck } from 'lucide-react'
import { Button, Input, toast } from '../../../shared/components'
import { useCloudStore } from '../../../store/cloudStore'
import { cloudApi } from '../api'
import { errorText } from '../errors'

// 端到端加密的开通与解锁。已解锁时由同步面板展示状态, 这里只处理未开通 / 未解锁两种情况。

/**
 * 开通端到端加密。恢复码交给全局弹窗 (CloudOverlays) 展示: 开通后后端推送「已解锁」,
 * 发起开通的面板随即卸载, 弹窗不能挂在面板里。返回是否开通成功。
 */
export async function startEnroll(): Promise<boolean> {
  const store = useCloudStore.getState()
  try {
    store.setRecoveryCode(await cloudApi.enroll())
    return true
  } catch (err) {
    toast.error(errorText(err))
    void store.refresh()
    return false
  }
}

export function EnrollPanel() {
  const [busy, setBusy] = useState(false)

  const enroll = async () => {
    setBusy(true)
    try {
      await startEnroll()
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex gap-3">
        <ShieldCheck className="w-5 h-5 shrink-0 mt-0.5 text-[var(--color-text-secondary)]" />
        <div className="space-y-1 text-sm text-[var(--color-text-secondary)]">
          <p className="text-[var(--color-text-primary)] font-medium">开启端到端加密后即可同步</p>
          <p>实例配置、代理和登录态在本机加密后才上传，服务器上只有密文。开启时会生成一个恢复码，换电脑时用它解锁。</p>
        </div>
      </div>
      <Button onClick={enroll} loading={busy}>
        开启端到端加密
      </Button>
    </div>
  )
}

export function UnlockPanel() {
  const [recovery, setRecovery] = useState('')
  const [busy, setBusy] = useState(false)
  const [waiting, setWaiting] = useState(false)
  const deviceCode = useCloudStore((s) => s.deviceCode)
  const setDeviceCode = useCloudStore((s) => s.setDeviceCode)
  const refresh = useCloudStore((s) => s.refresh)

  useEffect(() => {
    const runtime = (window as any).runtime
    const off = runtime?.EventsOn?.('sync:device:done', (data: { ok: boolean; error?: string }) => {
      setWaiting(false)
      setDeviceCode('')
      if (data?.ok) {
        toast.success('已解锁，开始同步')
        void refresh()
      } else if (data?.error) {
        toast.error(errorText(data.error))
      }
    })
    return () => {
      if (typeof off === 'function') off()
    }
  }, [refresh, setDeviceCode])

  const unlock = async () => {
    if (!recovery.trim()) return
    setBusy(true)
    try {
      await cloudApi.unlock(recovery)
      setRecovery('')
      toast.success('已解锁，开始同步')
      void refresh()
    } catch (err) {
      toast.error(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const startDevice = async () => {
    try {
      setDeviceCode('')
      await cloudApi.deviceStart()
      setWaiting(true)
    } catch (err) {
      toast.error(errorText(err))
    }
  }

  const cancelDevice = async () => {
    await cloudApi.deviceCancel()
    setWaiting(false)
    setDeviceCode('')
  }

  return (
    <div className="space-y-5">
      <p className="text-sm text-[var(--color-text-secondary)]">
        这个账号已在别的电脑开启了端到端加密。在这台电脑上解锁后才能读取云端数据，二选一即可。
      </p>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-lg border border-[var(--color-border-default)] p-4 space-y-3">
          <div className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
            <KeyRound className="w-4 h-4" />
            输入恢复码
          </div>
          <Input
            value={recovery}
            placeholder="XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"
            className="font-mono"
            onChange={(e) => setRecovery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && void unlock()}
          />
          <Button size="sm" onClick={unlock} loading={busy} disabled={!recovery.trim()}>
            解锁
          </Button>
        </div>
        <div className="rounded-lg border border-[var(--color-border-default)] p-4 space-y-3">
          <div className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
            <Laptop className="w-4 h-4" />
            用已登录的电脑批准
          </div>
          {!waiting ? (
            <>
              <p className="text-xs text-[var(--color-text-muted)]">在另一台已解锁的电脑上打开 CZL Browser，会弹出授权请求。</p>
              <Button size="sm" variant="secondary" onClick={startDevice}>
                发起授权请求
              </Button>
            </>
          ) : (
            <>
              {deviceCode ? (
                <>
                  <p className="text-xs text-[var(--color-text-muted)]">核对另一台电脑上显示的数字，一致后在那边点「批准」：</p>
                  <div className="text-center font-mono text-2xl tracking-[0.4em] text-[var(--color-text-primary)]">{deviceCode}</div>
                </>
              ) : (
                <p className="text-xs text-[var(--color-text-muted)]">等待另一台电脑响应…（10 分钟内有效）</p>
              )}
              <Button size="sm" variant="ghost" onClick={cancelDevice}>
                取消
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
