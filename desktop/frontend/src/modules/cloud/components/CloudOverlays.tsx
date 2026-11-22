import { useEffect } from 'react'
import { initCloudEvents, useCloudStore } from '../../../store/cloudStore'
import { useNotificationStore } from '../../../store/notificationStore'
import { ConfirmHost } from '../confirm'
import { errorText } from '../errors'
import { DeviceApprovalModal } from './DeviceApprovalModal'
import { RecoveryCodeModal } from './RecoveryCodeModal'

// 登录后的壳里挂一次: 云端事件监听、全局确认框、新设备批准弹窗、冲突与登录态上传失败的通知。

export function CloudOverlays() {
  const addNotification = useNotificationStore((s) => s.addNotification)
  const recoveryCode = useCloudStore((s) => s.recoveryCode)

  useEffect(() => initCloudEvents(), [])

  useEffect(() => {
    const runtime = (window as any).runtime
    if (!runtime?.EventsOn) return
    const offs = [
      runtime.EventsOn('sync:conflict', () => {
        addNotification({
          type: 'warning',
          title: '同步冲突',
          message: '你的一处修改与团队里其他人同时改了同一条数据，已采用对方的版本。',
        })
      }),
      runtime.EventsOn('sync:state:error', (data: { error: string }) => {
        addNotification({ type: 'error', title: '登录态上传失败', message: errorText(data?.error) })
      }),
    ]
    return () => offs.forEach((off: any) => typeof off === 'function' && off())
  }, [addNotification])

  // 打开软件一段时间后再查一次空间 (启动时可能还没联网)
  useEffect(() => {
    const t = window.setInterval(() => void useCloudStore.getState().refresh(), 5 * 60 * 1000)
    return () => window.clearInterval(t)
  }, [])

  return (
    <>
      <ConfirmHost />
      <DeviceApprovalModal />
      <RecoveryCodeModal
        code={recoveryCode}
        onDone={() => {
          useCloudStore.getState().setRecoveryCode('')
          void useCloudStore.getState().refresh()
        }}
      />
    </>
  )
}
