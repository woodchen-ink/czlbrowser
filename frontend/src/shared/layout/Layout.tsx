import { ReactNode } from 'react'
import { Sidebar } from './Sidebar'

interface LayoutProps {
  children: ReactNode
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="flex h-screen bg-[var(--color-bg-base)]">
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-hidden min-w-0">
        <main className="flex-1 overflow-auto p-5">
          {children}
        </main>
      </div>
    </div>
  )
}
