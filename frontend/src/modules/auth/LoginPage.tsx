import { useState } from 'react'
import { LogIn, Loader2, Shield, ExternalLink } from 'lucide-react'
import { Button } from '../../shared/components'
import { useAuthStore } from '../../store/authStore'

import logoImage from '../../resources/images/logo.png'

export function LoginPage() {
  const { login, loginPending } = useAuthStore()
  const [error, setError] = useState('')
  const [manualURL, setManualURL] = useState('')

  const handleLogin = async () => {
    setError('')
    setManualURL('')
    try {
      await login()
    } catch (err: any) {
      // Wails 把 Go error 作为字符串传递
      const msg = typeof err === 'string' ? err : err?.message || '登录失败，请重试'
      setError(msg)
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
    <div className="flex h-screen w-screen bg-[var(--color-bg-base)]">
      <div className="flex flex-1 items-center justify-center">
        <div className="w-full max-w-sm space-y-8 px-6">
          {/* Logo & Title */}
          <div className="flex flex-col items-center space-y-4">
            <div className="w-16 h-16 rounded-2xl overflow-hidden bg-[var(--color-accent)] flex items-center justify-center shadow-sm">
              <img
                src={logoImage}
                alt="CZL Browser"
                className="w-full h-full object-cover"
                onError={(e) => {
                  e.currentTarget.style.display = 'none'
                }}
              />
            </div>
            <div className="text-center space-y-1">
              <h1 className="text-2xl font-bold text-[var(--color-text-primary)] tracking-tight">
                CZL Browser
              </h1>
              <p className="text-sm text-[var(--color-text-muted)]">
                指纹浏览器管理平台
              </p>
            </div>
          </div>

          {/* Login Card */}
          <div className="rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-6 space-y-5 shadow-sm">
            <div className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)]">
              <Shield className="w-4 h-4 text-[var(--color-accent)]" />
              <span>使用 CZL Connect 账号登录</span>
            </div>

            <Button
              variant="primary"
              className="w-full h-11 rounded-xl text-sm font-medium"
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

            {loginPending && (
              <p className="text-xs text-center text-[var(--color-text-muted)]">
                已在浏览器中打开授权页面，请完成登录后返回此应用
              </p>
            )}

            {manualURL && (
              <div className="rounded-lg bg-[var(--color-bg-muted)] p-3 space-y-2">
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

            {error && (
              <p className="text-xs text-center text-[var(--color-error)]">{error}</p>
            )}
          </div>

          {/* Footer */}
          <p className="text-center text-xs text-[var(--color-text-muted)]">
            登录即表示您同意相关使用条款
          </p>
        </div>
      </div>
    </div>
  )
}
