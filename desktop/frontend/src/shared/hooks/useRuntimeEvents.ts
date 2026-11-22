import { useEffect } from 'react'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useAuthStore } from '../../store/authStore'
import { useNotificationStore } from '../../store/notificationStore'
import { useUpdateStore } from '../../store/updateStore'

type Handlers = Record<string, (data: any) => void>

/** 订阅后端事件, 组件卸载时取消。handlers 应保持稳定 (放在 useMemo / 模块常量里)。 */
export function useEvents(handlers: Handlers, deps: unknown[]) {
  useEffect(() => {
    const offs = Object.entries(handlers).map(([name, fn]) => EventsOn(name, fn))
    return () => offs.forEach((off) => off())
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
}

/** 实例崩溃、代理转换失败等后台事件转成通知中心里的消息。 */
export function useBackgroundAlerts() {
  const notify = useNotificationStore((s) => s.addNotification)
  const who = (d: { profileName?: string; profileId: string }) => d.profileName || d.profileId
  useEvents(
    {
      'browser:instance:crashed': (d) =>
        notify({ type: 'error', title: '实例异常退出', message: `「${who(d)}」意外崩溃：${d.error}` }),
      'proxy:bridge:failed': (d) =>
        notify({ type: 'error', title: '代理连接失败', message: `「${who(d)}」代理桥接启动失败：${d.error}` }),
      'proxy:bridge:died': (d) =>
        notify({ type: 'warning', title: '连接池节点失效', message: `代理节点 ${d.key} 连接中断，相关实例可能无法访问网络` }),
    },
    [notify],
  )
}

const UPDATE_CHECK_EVERY = 6 * 60 * 60 * 1000

/** 登录后检查一次更新, 之后每 6 小时一次; 每个新版本只提示一次。 */
export function useUpdateCheck() {
  const notify = useNotificationStore((s) => s.addNotification)
  const check = useUpdateStore((s) => s.check)
  const setProgress = useUpdateStore((s) => s.setProgress)

  useEvents({ 'update:progress': setProgress }, [setProgress])
  useEffect(() => {
    let announced = ''
    const run = async () => {
      const r = await check()
      if (r?.hasUpdate && r.latestVersion !== announced) {
        announced = r.latestVersion
        notify({ type: 'info', title: '发现新版本', message: `CZL Browser ${r.latestVersion} 已发布，可在「系统设置」中下载安装` })
      }
    }
    run()
    const timer = window.setInterval(run, UPDATE_CHECK_EVERY)
    return () => window.clearInterval(timer)
  }, [check, notify])
}

/** 登录结果、启动时后台恢复会话、会话失效。 */
export function useAuthEvents() {
  const setAuthState = useAuthStore((s) => s.setAuthState)
  useEvents(
    {
      'auth:login:success': setAuthState,
      'auth:restored': setAuthState,
      'auth:login:error': (msg: string) => useAuthStore.setState({ loginPending: false, loginError: msg }),
      'auth:session:expired': () => useAuthStore.setState({ state: { loggedIn: false } }),
    },
    [setAuthState],
  )
}
