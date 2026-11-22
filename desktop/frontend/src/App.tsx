import { useEffect, useState } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { ThemeProvider } from './shared/theme'
import { Layout } from './shared/layout'
import { CloseConfirmModal } from './shared/layout/CloseConfirmModal'
import { Loading, ToastContainer } from './shared/components'
import { useAuthEvents, useBackgroundAlerts, useUpdateCheck } from './shared/hooks/useRuntimeEvents'
import { DashboardPage } from './modules/dashboard'
import { SettingsPage } from './modules/settings'
import { LoginPage } from './modules/auth'
import { BookmarkSettingsPage, BrowserDetailPage, BrowserEditPage, BrowserListPage, CoreManagementPage, ProxyPoolPage } from './modules/browser'
import { QuickLaunchModal } from './modules/browser/components/QuickLaunchModal'
import { CloudOverlays, CloudPage } from './modules/cloud'
import { useAuthStore } from './store/authStore'

/** Ctrl / ⌘ + K 切换快速启动面板 (输入法组字时不响应)。 */
function useQuickLaunchHotkey(toggle: () => void) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!e.isComposing && (e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        toggle()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [toggle])
}

function Workspace() {
  useBackgroundAlerts()
  useUpdateCheck()
  const [quickLaunch, setQuickLaunch] = useState(false)
  useQuickLaunchHotkey(() => setQuickLaunch((v) => !v))

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/browser/list" element={<BrowserListPage />} />
        <Route path="/browser/detail/:id" element={<BrowserDetailPage />} />
        <Route path="/browser/edit/:id" element={<BrowserEditPage />} />
        <Route path="/browser/proxy-pool" element={<ProxyPoolPage />} />
        <Route path="/browser/cores" element={<CoreManagementPage />} />
        <Route path="/browser/bookmarks" element={<BookmarkSettingsPage />} />
        <Route path="/cloud" element={<CloudPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <QuickLaunchModal open={quickLaunch} onClose={() => setQuickLaunch(false)} />
      <CloudOverlays />
    </Layout>
  )
}

export default function App() {
  const { state, loading, fetchState } = useAuthStore()
  useAuthEvents()
  useEffect(() => {
    fetchState()
  }, [fetchState])

  let body
  if (loading) {
    body = (
      <div className="flex h-screen w-screen items-center justify-center bg-[var(--color-bg-base)]">
        <Loading />
      </div>
    )
  } else if (!state.loggedIn) {
    body = <LoginPage />
  } else {
    body = (
      <BrowserRouter>
        <Workspace />
      </BrowserRouter>
    )
  }
  return (
    <ThemeProvider>
      {body}
      {!loading && (
        <>
          <ToastContainer />
          <CloseConfirmModal />
        </>
      )}
    </ThemeProvider>
  )
}
