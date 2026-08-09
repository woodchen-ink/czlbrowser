import { User, Mail, LogOut, ExternalLink } from 'lucide-react'
import { Card, Button } from '../../shared/components'
import { useAuthStore } from '../../store/authStore'

export function ProfilePage() {
  const { state: authState, logout } = useAuthStore()
  const user = authState.user

  const handleLogout = async () => {
    await logout()
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 animate-fade-in">
      {/* 用户信息卡片 */}
      <Card padding="lg" className="rounded-2xl">
        <div className="flex items-start gap-6">
          {/* 头像 */}
          {user?.avatar ? (
            <img
              src={user.avatar}
              alt={user.nickname || user.username}
              className="w-20 h-20 rounded-2xl object-cover shadow-sm"
            />
          ) : (
            <div className="w-20 h-20 rounded-2xl bg-[var(--color-accent)] flex items-center justify-center shadow-sm">
              <User className="w-8 h-8 text-[var(--color-text-inverse)]" />
            </div>
          )}

          {/* 基本信息 */}
          <div className="flex-1 min-w-0 space-y-3">
            <div>
              <h1 className="text-2xl font-bold text-[var(--color-text-primary)] tracking-tight">
                {user?.nickname || user?.username || '用户'}
              </h1>
              {user?.username && user?.nickname && (
                <p className="text-sm text-[var(--color-text-muted)] mt-0.5">
                  @{user.username}
                </p>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-4 text-sm text-[var(--color-text-secondary)]">
              {user?.email && (
                <span className="inline-flex items-center gap-1.5">
                  <Mail className="w-4 h-4 text-[var(--color-text-muted)]" />
                  {user.email}
                </span>
              )}
              {user?.id && (
                <span className="inline-flex items-center gap-1.5 text-[var(--color-text-muted)]">
                  ID: {user.id}
                </span>
              )}
            </div>
          </div>

          {/* 操作按钮 */}
          <div className="flex flex-col gap-2">
            <Button
              variant="ghost"
              className="h-9 rounded-xl border border-[var(--color-border-default)] px-3 text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]"
              onClick={() => window.open('https://connect.czl.net', '_blank')}
            >
              <ExternalLink className="w-3.5 h-3.5" />
              账号设置
            </Button>
            <Button
              variant="ghost"
              className="h-9 rounded-xl border border-[var(--color-border-default)] px-3 text-sm text-[var(--color-error)] hover:bg-[var(--color-bg-muted)]"
              onClick={handleLogout}
            >
              <LogOut className="w-3.5 h-3.5" />
              退出登录
            </Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
