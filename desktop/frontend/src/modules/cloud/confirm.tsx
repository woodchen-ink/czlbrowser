import type { ReactNode } from 'react'
import { create } from 'zustand'
import { ConfirmModal } from '../../shared/components'

// 可在任意地方 await 的确认框 (启动被占用的实例、退出登录有未上传修改、离开团队等), 由 CloudOverlays 渲染。

interface ConfirmRequest {
  title: string
  content: ReactNode
  confirmText?: string
  danger?: boolean
  resolve: (ok: boolean) => void
}

const useConfirmStore = create<{ req: ConfirmRequest | null }>(() => ({ req: null }))

export function askConfirm(opts: Omit<ConfirmRequest, 'resolve'>): Promise<boolean> {
  return new Promise((resolve) => {
    useConfirmStore.getState().req?.resolve(false)
    useConfirmStore.setState({ req: { ...opts, resolve } })
  })
}

export function ConfirmHost() {
  const req = useConfirmStore((s) => s.req)
  const close = (ok: boolean) => {
    req?.resolve(ok)
    useConfirmStore.setState({ req: null })
  }
  return (
    <ConfirmModal
      open={!!req}
      title={req?.title}
      content={req?.content ?? ''}
      confirmText={req?.confirmText}
      danger={req?.danger}
      onClose={() => close(false)}
      onConfirm={() => close(true)}
    />
  )
}
