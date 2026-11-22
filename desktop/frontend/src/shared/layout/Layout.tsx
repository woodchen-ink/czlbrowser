import type { ReactNode } from 'react'
import { ReadOnlyBanner } from '../../modules/cloud/components/ReadOnlyBanner'
import { Sidebar } from './Sidebar'

/** 登录后的外框: 左侧导航, 右侧页面。页面自己决定滚动 (列表页表格内部滚动, 普通页整页滚动)。 */
export function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-screen bg-[var(--color-bg-base)]">
      <Sidebar />
      <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden px-6 py-4">
        <ReadOnlyBanner />
        <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
      </main>
    </div>
  )
}
