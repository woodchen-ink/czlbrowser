import { useRef, useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  Activity,
  AlertCircle,
  Bell,
  Bookmark,
  BookOpen,
  Check,
  CheckCircle,
  FileText,
  Info,
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
  Trash2,
  User,
  type LucideIcon
} from 'lucide-react'
import clsx from 'clsx'
import { useLayoutStore } from '../../store/layoutStore'
import { projectConfig, navigationConfig } from '../../config'
import { useAuthStore } from '../../store/authStore'
import { useNotificationStore, type Notification } from '../../store/notificationStore'
import { useUpdateStore } from '../../store/updateStore'

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

function NotificationDropdown({
  notifications,
  onMarkAsRead,
  onMarkAllAsRead,
  onClear,
}: {
  notifications: Notification[]
  onMarkAsRead: (id: string) => void
  onMarkAllAsRead: () => void
  onClear: () => void
}) {
  const unreadCount = notifications.filter(n => !n.read).length

  const getIcon = (type: Notification['type']) => {
    switch (type) {
      case 'success': return <CheckCircle className="w-4 h-4 text-[var(--color-success)]" />
      case 'warning': return <AlertCircle className="w-4 h-4 text-[var(--color-warning)]" />
      case 'error': return <AlertCircle className="w-4 h-4 text-[var(--color-error)]" />
      default: return <Info className="w-4 h-4 text-[var(--color-accent)]" />
    }
  }

  return (
    <div className="absolute left-full bottom-0 ml-2 w-80 bg-[var(--color-bg-surface)] border border-[var(--color-border-default)] rounded-xl shadow-xl overflow-hidden z-50 animate-fade-in">
      <div className="px-4 py-3 border-b border-[var(--color-border-muted)] flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-[var(--color-text-primary)]">通知</span>
          {unreadCount > 0 && (
            <span className="px-1.5 py-0.5 text-xs font-medium bg-[var(--color-accent)] text-[var(--color-text-inverse)] rounded-full">
              {unreadCount}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {unreadCount > 0 && (
            <button
              onClick={onMarkAllAsRead}
              className="p-1.5 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] hover:bg-[var(--color-bg-muted)] rounded transition-colors"
              title="全部标为已读"
            >
              <Check className="w-3.5 h-3.5" />
            </button>
          )}
          <button
            onClick={onClear}
            className="p-1.5 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-error)] hover:bg-[var(--color-bg-muted)] rounded transition-colors"
            title="清空通知"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div className="max-h-80 overflow-y-auto">
        {notifications.length === 0 ? (
          <div className="py-8 text-center text-[var(--color-text-muted)]">
            <Bell className="w-8 h-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">暂无通知</p>
          </div>
        ) : (
          notifications.map((notification) => (
            <div
              key={notification.id}
              onClick={() => onMarkAsRead(notification.id)}
              className={clsx(
                'px-4 py-3 border-b border-[var(--color-border-muted)] last:border-0 cursor-pointer transition-colors hover:bg-[var(--color-bg-muted)]',
                !notification.read && 'bg-[var(--color-accent)]/5'
              )}
            >
              <div className="flex gap-3">
                <div className="shrink-0 mt-0.5">
                  {getIcon(notification.type)}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-2">
                    <p className={clsx(
                      'text-sm truncate',
                      notification.read ? 'text-[var(--color-text-secondary)]' : 'text-[var(--color-text-primary)] font-medium'
                    )}>
                      {notification.title}
                    </p>
                    {!notification.read && (
                      <span className="w-2 h-2 rounded-full bg-[var(--color-accent)] shrink-0 mt-1.5" />
                    )}
                  </div>
                  <p className="text-xs text-[var(--color-text-muted)] mt-0.5 line-clamp-2">
                    {notification.message}
                  </p>
                  <p className="text-[10px] text-[var(--color-text-muted)] mt-1">
                    {notification.time}
                  </p>
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {notifications.length > 0 && (
        <div className="px-4 py-2 border-t border-[var(--color-border-muted)] bg-[var(--color-bg-muted)]/50">
          <button className="w-full text-xs text-center text-[var(--color-accent)] hover:underline">
            查看全部通知
          </button>
        </div>
      )}
    </div>
  )
}

export function Sidebar() {
  const location = useLocation()
  const { sidebarCollapsed, toggleSidebar } = useLayoutStore()
  const { state: authState, logout } = useAuthStore()
  const hasUpdate = useUpdateStore((s) => !!s.result?.hasUpdate)
  const user = authState.user
  const [showNotifications, setShowNotifications] = useState(false)
  const { notifications, markAsRead, markAllAsRead, clearNotifications } = useNotificationStore()
  const unreadCount = notifications.filter(n => !n.read).length
  const notifRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (notifRef.current && !notifRef.current.contains(event.target as Node)) {
        setShowNotifications(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

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
                    <span className="relative flex-shrink-0">
                      <Icon className="w-[18px] h-[18px]" />
                      {item.path === '/settings' && hasUpdate && (
                        <span className="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-[var(--color-error)]" title="有新版本" />
                      )}
                    </span>
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
          </div>
        )}

        <div className="space-y-1">
          <div className="relative" ref={notifRef}>
            <button
              onClick={() => setShowNotifications(prev => !prev)}
              title={sidebarCollapsed ? '通知' : undefined}
              className={clsx(
                'flex items-center rounded-lg transition-all duration-150',
                showNotifications
                  ? 'text-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                  : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
                sidebarCollapsed ? 'justify-center w-10 h-10 mx-auto' : 'w-full px-3 py-2.5 gap-3'
              )}
            >
              <span className="relative flex-shrink-0">
                <Bell className="w-[18px] h-[18px]" />
                {unreadCount > 0 && (
                  <span className="absolute -top-1 -right-1 w-3.5 h-3.5 text-[9px] font-medium bg-[var(--color-error)] text-[var(--color-text-inverse)] rounded-full flex items-center justify-center">
                    {unreadCount > 9 ? '9+' : unreadCount}
                  </span>
                )}
              </span>
              {!sidebarCollapsed && <span className="text-sm font-medium truncate">通知{unreadCount > 0 ? `（${unreadCount}）` : ''}</span>}
            </button>

            {showNotifications && (
              <NotificationDropdown
                notifications={notifications}
                onMarkAsRead={markAsRead}
                onMarkAllAsRead={markAllAsRead}
                onClear={() => {
                  clearNotifications()
                  setShowNotifications(false)
                }}
              />
            )}
          </div>
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
              'flex items-center rounded-lg transition-all duration-150 text-[var(--color-error)] hover:bg-[var(--color-error-soft)] hover:text-[var(--color-error)]',
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
