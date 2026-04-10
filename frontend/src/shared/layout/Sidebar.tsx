import { Link, useLocation } from 'react-router-dom'
import {
  Activity,
  Bookmark,
  BookOpen,
  Crown,
  FileText,
  LayoutDashboard,
  ListChecks,
  LogOut,
  Monitor,
  Settings,
  Database,
  ChevronLeft,
  ChevronRight,
  Layers,
  PieChart,
  Cpu,
  Globe,
  Tag,
  User,
  type LucideIcon
} from 'lucide-react'
import clsx from 'clsx'
import { useLayoutStore } from '../../store/layoutStore'
import { projectConfig, navigationConfig } from '../../config'
import { useAuthStore } from '../../store/authStore'

// 导入应用logo
import logoImage from '../../resources/images/logo.png'

const iconMap: Record<string, LucideIcon> = {
  LayoutDashboard,
  Settings,
  Database,
  Layers,
  PieChart,
  Monitor,
  ListChecks,
  Activity,
  FileText,
  Cpu,
  Globe,
  Bookmark,
  BookOpen,
  Tag,
}

function getIcon(iconName: string): LucideIcon {
  return iconMap[iconName] || LayoutDashboard
}

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

export function Sidebar() {
  const location = useLocation()
  const { sidebarCollapsed, toggleSidebar } = useLayoutStore()
  const { state: authState, logout } = useAuthStore()
  const user = authState.user
  const levelLabel = LEVEL_LABELS[authState.level] || authState.level || '用户'

  return (
    <aside className={clsx(
      'bg-[var(--color-bg-surface)] flex flex-col transition-all duration-300 border-r border-[var(--color-border-default)]',
      sidebarCollapsed ? 'w-16' : 'w-60'
    )}>
      {/* Logo */}
      <div className={clsx(
        'h-14 flex items-center border-b border-[var(--color-border-muted)]',
        sidebarCollapsed ? 'justify-center px-2' : 'px-5'
      )}>
        {!sidebarCollapsed ? (
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-full overflow-hidden flex-shrink-0 bg-[var(--color-accent)] flex items-center justify-center">
              <img 
                src={logoImage} 
                alt="应用Logo" 
                className="w-full h-full object-cover"
                onError={(e) => {
                  // 图片加载失败时显示首字母
                  e.currentTarget.style.display = 'none';
                  e.currentTarget.parentElement?.classList.add('fallback-logo');
                }}
              />
              <span className="text-xs font-bold text-[var(--color-text-inverse)] hidden fallback-content">
                {projectConfig.shortName.charAt(0)}
              </span>
            </div>
            <h2 className="text-base font-semibold text-[var(--color-text-primary)] tracking-tight truncate">
              {projectConfig.name}
            </h2>
          </div>
        ) : (
          <div className="w-8 h-8 rounded-full overflow-hidden bg-[var(--color-accent)] flex items-center justify-center">
            <img 
              src={logoImage} 
              alt="应用Logo" 
              className="w-full h-full object-cover"
              onError={(e) => {
                // 图片加载失败时显示首字母
                e.currentTarget.style.display = 'none';
                e.currentTarget.parentElement?.classList.add('fallback-logo');
              }}
            />
            <span className="text-xs font-bold text-[var(--color-text-inverse)] hidden fallback-content">
              {projectConfig.shortName.charAt(0)}
            </span>
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 px-3 space-y-6 overflow-y-auto">
        {navigationConfig.map((section) => (
          <div key={section.title}>
            {!sidebarCollapsed && (
              <h3 className="px-3 mb-2 text-[10px] font-semibold text-[var(--color-text-muted)] uppercase tracking-widest">
                {section.title}
              </h3>
            )}
            <div className="space-y-1">
              {section.items.map((item) => {
                const Icon = getIcon(item.icon)
                const isActive = location.pathname === item.path
                
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    title={sidebarCollapsed ? item.name : undefined}
                    className={clsx(
                      'flex items-center rounded-lg transition-all duration-150',
                      isActive
                        ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)] shadow-sm'
                        : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
                      sidebarCollapsed 
                        ? 'justify-center w-10 h-10 mx-auto' 
                        : 'px-3 py-2.5 gap-3'
                    )}
                  >
                    <Icon className="w-[18px] h-[18px] flex-shrink-0" />
                    {!sidebarCollapsed && (
                      <span className="text-sm font-medium truncate">{item.name}</span>
                    )}
                  </Link>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="p-3 border-t border-[var(--color-border-muted)] space-y-2">
        {!sidebarCollapsed && (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)]/60 p-3">
            <div className="flex items-center gap-3">
              {user?.avatar ? (
                <img
                  src={user.avatar}
                  alt={user.nickname || user.username}
                  className="w-9 h-9 rounded-lg object-cover"
                />
              ) : (
                <div className="w-9 h-9 rounded-lg bg-[var(--color-accent)] text-[var(--color-text-inverse)] flex items-center justify-center">
                  <User className="w-4 h-4" />
                </div>
              )}
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-[var(--color-text-primary)] truncate">
                  {user?.nickname || user?.username || '用户'}
                </div>
                <div className="text-xs text-[var(--color-text-muted)] truncate">
                  {user?.email || ''}
                </div>
              </div>
            </div>
            <div className="mt-3 flex items-center justify-between">
              <span className="text-xs text-[var(--color-text-muted)]">当前等级</span>
              <span className="inline-flex items-center gap-1 rounded-md bg-[var(--color-accent)]/10 px-2 py-0.5 text-xs font-medium text-[var(--color-accent)]">
                <Crown className="w-3 h-3" />
                {levelLabel}
              </span>
            </div>
          </div>
        )}

        <div className="space-y-1">
          <Link
            to="/profile"
            title={sidebarCollapsed ? '个人信息' : undefined}
            className={clsx(
              'flex items-center rounded-lg transition-all duration-150 text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
              sidebarCollapsed ? 'justify-center w-10 h-10 mx-auto' : 'px-3 py-2.5 gap-3'
            )}
          >
            <User className="w-[18px] h-[18px] flex-shrink-0" />
            {!sidebarCollapsed && <span className="text-sm font-medium truncate">个人信息</span>}
          </Link>
          <button
            onClick={() => void logout()}
            title={sidebarCollapsed ? '退出登录' : undefined}
            className={clsx(
              'flex items-center rounded-lg transition-all duration-150 text-[var(--color-error)] hover:bg-red-50 hover:text-red-600',
              sidebarCollapsed ? 'justify-center w-10 h-10 mx-auto' : 'w-full px-3 py-2.5 gap-3'
            )}
          >
            <LogOut className="w-[18px] h-[18px] flex-shrink-0" />
            {!sidebarCollapsed && <span className="text-sm font-medium truncate">退出登录</span>}
          </button>
        </div>

        <button
          onClick={toggleSidebar}
          className={clsx(
            'flex items-center rounded-lg text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-secondary)] transition-all duration-150',
            sidebarCollapsed 
              ? 'justify-center w-10 h-10 mx-auto' 
              : 'w-full px-3 py-2 gap-3'
          )}
          title={sidebarCollapsed ? '展开' : '收起'}
        >
          {sidebarCollapsed ? (
            <ChevronRight className="w-[18px] h-[18px]" />
          ) : (
            <>
              <ChevronLeft className="w-[18px] h-[18px]" />
              <span className="text-sm">收起侧边栏</span>
            </>
          )}
        </button>
      </div>
    </aside>
  )
}
