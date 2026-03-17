import { User, Mail, Crown, Shield, LogOut, ExternalLink } from 'lucide-react'
import { Card, Button, Badge } from '../../shared/components'
import { useAuthStore } from '../../store/authStore'

/** 等级显示名称 */
const LEVEL_LABELS: Record<string, string> = {
  admin: '管理员',
  t5: 'T5',
  t4: 'T4',
  t3: 'T3',
  t2: 'T2',
  t1: 'T1',
  t0: 'T0',
  viewer: '观察者',
  visitor: '访客',
}

/** 等级描述 */
const LEVEL_DESCRIPTIONS: Record<string, string> = {
  admin: '拥有全部管理权限',
  t5: '高级用户，享有完整功能',
  t4: '高级用户',
  t3: '进阶用户',
  t2: '标准用户',
  t1: '基础用户',
  t0: '入门用户',
  viewer: '只读访问',
  visitor: '未分配等级',
}

export function ProfilePage() {
  const { state: authState, logout } = useAuthStore()
  const user = authState.user
  const level = authState.level || 'visitor'
  const levelLabel = LEVEL_LABELS[level] || level
  const levelDesc = LEVEL_DESCRIPTIONS[level] || ''

  // 解析用户拥有的所有权限组
  const groups = user?.groups
    ? user.groups.split(',').map((g) => g.trim()).filter(Boolean)
    : []

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

      {/* 等级信息 */}
      <Card
        title="账号等级"
        actions={<Crown className="w-4 h-4 text-[var(--color-text-muted)]" />}
        padding="lg"
        className="rounded-2xl"
      >
        <div className="space-y-4">
          <div className="flex items-center gap-4">
            <span className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-[var(--color-accent)]/10 text-lg font-semibold text-[var(--color-accent)]">
              <Crown className="w-5 h-5" />
              {levelLabel}
            </span>
            {levelDesc && (
              <span className="text-sm text-[var(--color-text-muted)]">{levelDesc}</span>
            )}
          </div>
        </div>
      </Card>

      {/* 权限组 */}
      {groups.length > 0 && (
        <Card
          title="权限组"
          actions={<Shield className="w-4 h-4 text-[var(--color-text-muted)]" />}
          padding="lg"
          className="rounded-2xl"
        >
          <div className="flex flex-wrap gap-2">
            {groups.map((group) => (
              <Badge
                key={group}
                className={`rounded-lg px-3 py-1.5 text-sm ${
                  group === level
                    ? 'bg-[var(--color-accent)]/10 text-[var(--color-accent)] font-medium'
                    : ''
                }`}
              >
                {LEVEL_LABELS[group] || group}
              </Badge>
            ))}
          </div>
        </Card>
      )}
    </div>
  )
}
