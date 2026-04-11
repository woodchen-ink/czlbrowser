import { useState } from 'react'
import { LogIn, Loader2, Shield, ExternalLink } from 'lucide-react'
import { Button } from '../../shared/components'
import { useAuthStore } from '../../store/authStore'

import logoImage from '../../resources/images/logo.png'

export function LoginPage() {
  const { login, loginPending, loginError } = useAuthStore()
  const [localError, setLocalError] = useState('')
  const [manualURL, setManualURL] = useState('')

  // 显示同步错误（AuthLogin 直接抛出的）或异步错误（事件回调的）
  const displayError = localError || loginError

  const handleLogin = async () => {
    setLocalError('')
    setManualURL('')
    useAuthStore.setState({ loginError: '' })
    try {
      await login()
    } catch (err: any) {
      const msg = typeof err === 'string' ? err : err?.message || '登录失败，请重试'
      setLocalError(msg)
      return
    }

    // 监听手动 URL 事件（浏览器打开失败时）
    const runtime = (window as any).runtime
    if (runtime?.EventsOn) {
      runtime.EventsOn('auth:login:url', (url: string) => {
        setManualURL(url)
      })
    }
  }

  return (
    <div className="relative flex min-h-screen w-screen items-center justify-center overflow-hidden bg-[var(--color-bg-base)] px-6 py-10">
      <div className="absolute inset-x-0 top-0 h-72 bg-[linear-gradient(135deg,var(--color-brand-gradient-start),var(--color-brand-gradient-end))] opacity-[0.06]" />
      <div className="relative w-full max-w-md rounded-[28px] border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-6 shadow-[var(--shadow-lg)] sm:p-8">
        <div className="flex flex-col items-center space-y-4">
          <div className="flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl bg-[var(--color-accent)] shadow-[var(--shadow-sm)]">
            <img
              src={logoImage}
              alt="CZL Browser"
              className="h-full w-full object-cover"
              onError={(e) => {
                e.currentTarget.style.display = 'none'
              }}
            />
          </div>
          <div className="space-y-1 text-center">
            <h1 className="text-2xl font-semibold tracking-tight text-[var(--color-text-primary)]">CZL Browser</h1>
            <p className="text-sm text-[var(--color-text-muted)]">指纹浏览器管理平台</p>
          </div>
        </div>

        <div className="mt-8 rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-5 shadow-[var(--shadow-sm)]">
          <div className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)]">
            <Shield className="w-4 h-4 text-[var(--color-accent)]" />
            <span>使用 CZL Connect 账号登录</span>
          </div>

          <Button
            variant="primary"
            className="mt-5 h-11 w-full rounded-xl text-sm font-medium"
            onClick={handleLogin}
            disabled={loginPending}
          >
            {loginPending ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                等待授权中...
              </>
            ) : (
              <>
                <LogIn className="w-4 h-4" />
                登录
              </>
            )}
          </Button>

          {loginPending && !displayError && (
            <p className="text-xs text-center text-[var(--color-text-muted)]">
              已在浏览器中打开授权页面，请完成登录后返回此应用
            </p>
          )}

          {manualURL && (
            <div className="mt-4 space-y-2 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-3">
              <p className="text-xs text-[var(--color-text-secondary)]">
                如果浏览器未自动打开，请手动访问以下链接：
              </p>
              <a
                href={manualURL}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-[var(--color-accent)] hover:underline break-all inline-flex items-center gap-1"
              >
                打开授权页面
                <ExternalLink className="w-3 h-3 flex-shrink-0" />
              </a>
            </div>
          )}

          {displayError && (
            <p className="mt-4 text-center text-xs text-[var(--color-error)]">{displayError}</p>
          )}
        </div>

        <p className="mt-6 text-center text-xs text-[var(--color-text-muted)]">
          登录即表示您同意相关使用条款
        </p>
      </div>
    </div>
  )
}
