import { useEffect, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import clsx from 'clsx'
import { Bell, Bookmark, Cloud, Cpu, Globe, LayoutDashboard, Monitor, PanelLeftClose, PanelLeftOpen, Settings, type LucideIcon } from 'lucide-react'
import { SpaceSwitcher } from '../../modules/cloud/components/SpaceSwitcher'
import logoImage from '../../resources/images/logo.svg'
import logoWhiteImage from '../../resources/images/logo-white.svg'
import { useCloudStore } from '../../store/cloudStore'
import { useLayoutStore } from '../../store/layoutStore'
import { useNotificationStore } from '../../store/notificationStore'
import { useUpdateStore } from '../../store/updateStore'
import { useTheme } from '../theme'
import { AccountMenu } from './AccountMenu'
import { NotificationPanel } from './NotificationPanel'

interface NavItem {
  name: string
  path: string
  icon: LucideIcon
}

// 两组导航, 组间有分隔线
const NAV: NavItem[][] = [
  [
    { name: '控制台', path: '/', icon: LayoutDashboard },
    { name: '实例', path: '/browser/list', icon: Monitor },
    { name: '代理', path: '/browser/proxy-pool', icon: Globe },
    { name: '内核', path: '/browser/cores', icon: Cpu },
    { name: '书签', path: '/browser/bookmarks', icon: Bookmark },
  ],
  [
    { name: '云端同步', path: '/cloud', icon: Cloud },
    { name: '设置', path: '/settings', icon: Settings },
  ],
]

/** 当前路由是否属于这个导航项; 实例的详情与编辑页也算"实例" */
function isActive(path: string, at: string): boolean {
  if (path === '/') return at === '/'
  if (path === '/browser/list') return at === path || /^\/browser\/(detail|edit)\//.test(at)
  return at === path || at.startsWith(`${path}/`)
}

const iconButton = 'flex items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]'

function NavLink({ item, collapsed, active, dot }: { item: NavItem; collapsed: boolean; active: boolean; dot?: string }) {
  const Icon = item.icon
  return (
    <Link
      to={item.path}
      title={collapsed ? item.name : undefined}
      className={clsx(
        'flex h-8 items-center rounded-md text-[13px] transition-colors',
        active ? 'bg-[var(--color-brand-soft)] font-medium text-[var(--color-brand-strong)]' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
        collapsed ? 'mx-auto w-9 justify-center' : 'gap-2.5 px-2.5',
      )}
    >
      <span className="relative shrink-0">
        <Icon className="h-4 w-4" />
        {dot && <span className="absolute -right-0.5 -top-0.5 h-1.5 w-1.5 rounded-full" style={{ background: dot }} />}
      </span>
      {!collapsed && <span className="truncate">{item.name}</span>}
    </Link>
  )
}

/** 侧栏底部的铃铛: 未读数 + 点开通知面板, 点面板外关闭 */
function NotificationBell() {
  const [open, setOpen] = useState(false)
  const unread = useNotificationStore((s) => s.notifications.filter((n) => !n.read).length)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => ref.current?.contains(e.target as Node) || setOpen(false)
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [open])

  return (
    <div className="relative" ref={ref}>
      <button onClick={() => setOpen((o) => !o)} title="通知" aria-label="通知" className={clsx('relative h-8 w-8', iconButton, open && 'bg-[var(--color-accent-muted)] text-[var(--color-text-primary)]')}>
        <Bell className="h-4 w-4" />
        {unread > 0 && (
          <span className="absolute right-1 top-1 h-[14px] min-w-[14px] rounded-full bg-[var(--color-error)] px-0.5 text-center text-[9px] font-medium leading-[14px] text-[var(--color-text-inverse)]">
            {unread > 9 ? '9+' : unread}
          </span>
        )}
      </button>
      {open && <NotificationPanel onClose={() => setOpen(false)} />}
    </div>
  )
}

/** 左侧导航: 标志与收起按钮、空间切换、导航、账号与通知。 */
export function Sidebar() {
  const { pathname } = useLocation()
  const { sidebarCollapsed: collapsed, toggleSidebar } = useLayoutStore()
  const { theme } = useTheme()
  const logo = theme === 'dark' ? logoWhiteImage : logoImage
  const hasUpdate = useUpdateStore((s) => !!s.result?.hasUpdate)
  const cloudAttention = useCloudStore((s) => s.sync?.keys === 'locked' || s.deviceRequests.length > 0)

  // 设置上有新版本时红点, 云端同步待解锁或有设备待批准时黄点
  const dotFor = (path: string) => (path === '/settings' && hasUpdate ? 'var(--color-error)' : path === '/cloud' && cloudAttention ? 'var(--color-warning)' : undefined)

  return (
    <aside className={clsx('flex shrink-0 flex-col border-r border-[var(--color-border-default)] bg-[var(--color-bg-sunken)] transition-[width] duration-200', collapsed ? 'w-14' : 'w-52')}>
      <div className={clsx('flex h-12 shrink-0 items-center', collapsed ? 'justify-center' : 'gap-2 pl-4 pr-2')}>
        {collapsed ? (
          // 收起时标志兼作展开按钮, 悬停换成图标
          <button onClick={toggleSidebar} title="展开侧栏" aria-label="展开侧栏" className="group relative flex h-9 w-9 items-center justify-center rounded-md hover:bg-[var(--color-accent-muted)]">
            <img src={logo} alt="" className="h-5 w-5 object-contain transition-opacity group-hover:opacity-0" />
            <PanelLeftOpen className="absolute h-4 w-4 text-[var(--color-text-secondary)] opacity-0 transition-opacity group-hover:opacity-100" />
          </button>
        ) : (
          <>
            <img src={logo} alt="" className="h-5 w-5 shrink-0 object-contain" />
            <span className="min-w-0 flex-1 truncate text-sm font-semibold tracking-tight text-[var(--color-text-primary)]">CZL Browser</span>
            <button onClick={toggleSidebar} title="收起侧栏" aria-label="收起侧栏" className={clsx('h-7 w-7', iconButton)}>
              <PanelLeftClose className="h-4 w-4" />
            </button>
          </>
        )}
      </div>

      <SpaceSwitcher collapsed={collapsed} />

      <nav className={clsx('min-h-0 flex-1 overflow-y-auto py-2', collapsed ? 'px-2' : 'px-2.5')}>
        {NAV.map((group, i) => (
          <div key={i} className={clsx('space-y-0.5', i > 0 && 'mt-2 border-t border-[var(--color-border-muted)] pt-2')}>
            {group.map((item) => (
              <NavLink key={item.path} item={item} collapsed={collapsed} active={isActive(item.path, pathname)} dot={dotFor(item.path)} />
            ))}
          </div>
        ))}
      </nav>

      <div className={clsx('flex shrink-0 gap-1 border-t border-[var(--color-border-muted)] p-2', collapsed ? 'flex-col items-center' : 'items-center')}>
        <AccountMenu collapsed={collapsed} />
        <NotificationBell />
      </div>
    </aside>
  )
}
