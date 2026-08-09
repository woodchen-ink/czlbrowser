import { useEffect, useState } from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { ThemeProvider } from './shared/theme'
import { Layout } from './shared/layout'
import { ToastContainer, Modal, Button, Loading } from './shared/components'
import { AlertCircle } from 'lucide-react'
import { DashboardPage } from './modules/dashboard'
import { SettingsPage } from './modules/settings'
import { ProfilePage } from './modules/profile'
import { AdminKeygenPage } from './modules/profile/AdminKeygenPage'
import { LoginPage } from './modules/auth'
import {
  BrowserListPage,
  BrowserDetailPage,
  BrowserEditPage,
  BrowserCopyPage,
  BrowserLogsPage,
  ProxyPoolPage,
  CoreManagementPage,
  BookmarkSettingsPage,
  TagManagementPage,
  UsageTutorialPage,
} from './modules/browser'
import { QuickLaunchModal } from './modules/browser/components/QuickLaunchModal'
import { useNotificationStore } from './store/notificationStore'
import { useBackupStore } from './store/backupStore'
import { useAuthStore } from './store/authStore'

function useWailsNotifications() {
  const addNotification = useNotificationStore((s) => s.addNotification)

  useEffect(() => {
    const runtime = (window as any).runtime
    if (!runtime?.EventsOn) return

    const offCrashed = runtime.EventsOn(
      'browser:instance:crashed',
      (data: { profileId: string; profileName: string; error: string }) => {
        addNotification({
          type: 'error',
          title: '实例异常退出',
          message: `「${data.profileName || data.profileId}」意外崩溃：${data.error}`,
        })
      }
    )

    const offBridgeFailed = runtime.EventsOn(
      'proxy:bridge:failed',
      (data: { profileId: string; profileName: string; error: string }) => {
        addNotification({
          type: 'error',
          title: '代理连接失败',
          message: `「${data.profileName || data.profileId}」代理桥接启动失败：${data.error}`,
        })
      }
    )

    const offBridgeDied = runtime.EventsOn(
      'proxy:bridge:died',
      (data: { key: string; error: string }) => {
        addNotification({
          type: 'warning',
          title: '连接池节点失效',
          message: `代理节点 ${data.key} 连接中断，相关实例可能无法访问网络`,
        })
      }
    )

    return () => {
      offCrashed?.()
      offBridgeFailed?.()
      offBridgeDied?.()
    }
  }, [addNotification])
}

function useAuthEvents() {
  const { setAuthState } = useAuthStore()

  useEffect(() => {
    const runtime = (window as any).runtime
    if (!runtime?.EventsOn) return

    const offSuccess = runtime.EventsOn('auth:login:success', (state: any) => {
      setAuthState(state)
    })

    const offError = runtime.EventsOn('auth:login:error', (errMsg: string) => {
      useAuthStore.setState({ loginPending: false, loginError: errMsg })
    })

    // 启动时后台恢复会话完成：刷新令牌成功并补全用户信息后更新状态
    const offRestored = runtime.EventsOn('auth:restored', (state: any) => {
      setAuthState(state)
    })

    // 会话确已失效（refresh_token 过期）：清除登录态回到登录页
    const offExpired = runtime.EventsOn('auth:session:expired', () => {
      useAuthStore.setState({ state: { loggedIn: false } })
    })

    return () => {
      offSuccess?.()
      offError?.()
      offRestored?.()
      offExpired?.()
    }
  }, [setAuthState])
}

function CloseConfirmModal() {
  const [open, setOpen] = useState(false)
  const importInProgress = useBackupStore((s) => s.importInProgress)
  const importProgress = useBackupStore((s) => s.importProgress)
  const importMessage = useBackupStore((s) => s.importMessage)

  useEffect(() => {
    const runtime = (window as any).runtime
    if (!runtime?.EventsOn) return

    const off = runtime.EventsOn('app:request-close', () => {
      setOpen(true)
    })
    return () => {
      if (typeof off === 'function') off()
    }
  }, [])

  const handleMinimize = () => {
    setOpen(false)
    const runtime = (window as any).runtime
    runtime?.WindowHide?.()
  }

  const handleQuit = async () => {
    setOpen(false)
    const goApp = (window as any).go?.main?.App
    if (goApp?.ForceQuit) {
      await goApp.ForceQuit()
    } else {
      const runtime = (window as any).runtime
      runtime?.Quit?.()
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => setOpen(false)}
      title={importInProgress ? '关闭应用确认' : '退出确认'}
      width="360px"
    >
      <div className="flex flex-col items-center pt-2 pb-6 px-4">
        <div className={`w-12 h-12 rounded-full flex items-center justify-center mb-4 ${
          importInProgress
            ? 'bg-[var(--color-warning-soft)] text-[var(--color-warning)]'
            : 'bg-[var(--color-error-soft)] text-[var(--color-error)]'
        }`}>
          <AlertCircle className="w-6 h-6" />
        </div>
        <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-2">
          {importInProgress ? '正在加载中，是否关闭？' : '是否退出应用程序？'}
        </h3>
        {importInProgress ? (
          <p className="text-sm text-[var(--color-text-secondary)] text-center mb-6">
            当前正在加载配置
            {importProgress > 0 ? `（${importProgress}%）` : ''}。
            <br />
            {importMessage || '强制关闭会中断本次加载，是否仍要关闭应用？'}
          </p>
        ) : (
          <p className="text-sm text-[var(--color-text-secondary)] text-center mb-6">
            退出后将停止所有在此客户端运行的服务，<br />
            如果您需要保持服务运行，请选择「最小化到托盘」。
          </p>
        )}

        <div className="flex gap-3 w-full">
          {importInProgress ? (
            <>
              <Button variant="secondary" className="flex-1" onClick={() => setOpen(false)}>
                继续加载
              </Button>
              <Button variant="danger" className="flex-1" onClick={handleQuit}>
                仍要关闭
              </Button>
            </>
          ) : (
            <>
              <Button variant="secondary" className="flex-1" onClick={handleMinimize}>
                最小化到托盘
              </Button>
              <Button variant="danger" className="flex-1" onClick={handleQuit}>
                直接退出
              </Button>
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}

function AuthenticatedApp() {
  useWailsNotifications()
  const [quickLaunchOpen, setQuickLaunchOpen] = useState(false)

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.isComposing) return
      if (!(event.ctrlKey || event.metaKey)) return
      if (event.key.toLowerCase() !== 'k') return
      event.preventDefault()
      setQuickLaunchOpen((prev) => !prev)
    }

    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [])

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/admin/keygen" element={<AdminKeygenPage />} />
        <Route path="/browser/list" element={<BrowserListPage />} />
        <Route path="/browser/detail/:id" element={<BrowserDetailPage />} />
        <Route path="/browser/edit/:id" element={<BrowserEditPage />} />
        <Route path="/browser/copy/:id" element={<BrowserCopyPage />} />
        <Route path="/browser/monitor" element={<Navigate to="/browser/list" replace />} />
        <Route path="/browser/logs" element={<BrowserLogsPage />} />
        <Route path="/browser/proxy-pool" element={<ProxyPoolPage />} />
        <Route path="/browser/cores" element={<CoreManagementPage />} />
        <Route path="/browser/bookmarks" element={<BookmarkSettingsPage />} />
        <Route path="/browser/tags" element={<TagManagementPage />} />
        <Route path="/system/tutorial" element={<UsageTutorialPage />} />
      </Routes>
      <QuickLaunchModal open={quickLaunchOpen} onClose={() => setQuickLaunchOpen(false)} />
    </Layout>
  )
}

function App() {
  const { state: authState, loading, fetchState } = useAuthStore()
  useAuthEvents()

  useEffect(() => {
    fetchState()
  }, [fetchState])

  if (loading) {
    return (
      <ThemeProvider>
        <div className="flex h-screen w-screen items-center justify-center bg-[var(--color-bg-base)]">
          <Loading />
        </div>
      </ThemeProvider>
    )
  }

  if (!authState.loggedIn) {
    return (
      <ThemeProvider>
        <LoginPage />
        <ToastContainer />
        <CloseConfirmModal />
      </ThemeProvider>
    )
  }

  return (
    <ThemeProvider>
      <Router>
        <AuthenticatedApp />
        <ToastContainer />
        <CloseConfirmModal />
      </Router>
    </ThemeProvider>
  )
}

export default App
