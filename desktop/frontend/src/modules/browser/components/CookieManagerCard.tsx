import { useCallback, useEffect, useMemo, useState } from 'react'
import { Download, RefreshCw, Trash2 } from 'lucide-react'
import { Button, Card, ConfirmModal, Input, Table, toast, type TableColumn } from '../../../shared/components'
import { errorText } from '../../cloud/errors'
import { clearBrowserCookies, exportBrowserCookies, fetchBrowserCookies } from '../api'
import type { CookieInfo } from '../types'

interface Props {
  profileId: string
  profileName: string
  running: boolean
}

const mono = (v: string) => <span className="font-mono text-xs">{v}</span>
const yesNo = (v: boolean) => (v ? '是' : '')

const columns: TableColumn<CookieInfo>[] = [
  { key: 'domain', title: '域名', render: mono },
  { key: 'name', title: '名称', render: mono },
  {
    key: 'value',
    title: '值',
    render: (v: string) => (
      <span className="block max-w-[160px] truncate font-mono text-xs" title={v}>
        {v}
      </span>
    ),
  },
  { key: 'expires', title: '过期', render: (v: number) => (v > 0 ? new Date(v * 1000).toLocaleString('zh-CN') : '会话结束') },
  { key: 'httpOnly', title: 'HttpOnly', align: 'center', render: yesNo },
  { key: 'secure', title: 'Secure', align: 'center', render: yesNo },
]

/** 运行中实例的 Cookie: 经 CDP 读取, 可按域名过滤、导出 Netscape 格式、全部清除。 */
export function CookieManagerCard({ profileId, profileName, running }: Props) {
  const [cookies, setCookies] = useState<CookieInfo[]>([])
  const [domain, setDomain] = useState('')
  const [loading, setLoading] = useState(false)
  const [confirmClear, setConfirmClear] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setCookies(await fetchBrowserCookies(profileId))
    } catch (e) {
      toast.error(`读取 Cookie 失败: ${errorText(e)}`)
    } finally {
      setLoading(false)
    }
  }, [profileId])

  useEffect(() => {
    if (running) load()
    else setCookies([])
  }, [running, load])

  const shown = useMemo(() => {
    const q = domain.trim().toLowerCase()
    return q ? cookies.filter((c) => c.domain.toLowerCase().includes(q)) : cookies
  }, [cookies, domain])

  const clearAll = async () => {
    try {
      await clearBrowserCookies(profileId)
      setCookies([])
      toast.success('已清除全部 Cookie')
    } catch (e) {
      toast.error(`清除失败: ${errorText(e)}`)
    }
  }

  const exportFile = async () => {
    try {
      const text = await exportBrowserCookies(profileId)
      const a = document.createElement('a')
      a.href = URL.createObjectURL(new Blob([text], { type: 'text/plain' }))
      a.download = `cookies_${profileName}_${new Date().toISOString().slice(0, 10)}.txt`
      a.click()
      URL.revokeObjectURL(a.href)
    } catch (e) {
      toast.error(`导出失败: ${errorText(e)}`)
    }
  }

  if (!running) {
    return (
      <Card title="Cookie">
        <p className="py-6 text-center text-sm text-[var(--color-text-muted)]">启动实例后可查看与管理 Cookie</p>
      </Card>
    )
  }

  return (
    <Card
      title="Cookie"
      subtitle={domain.trim() ? `${shown.length} / ${cookies.length} 条` : `${cookies.length} 条`}
      actions={
        <div className="flex gap-1.5">
          <Button size="sm" variant="ghost" onClick={load} disabled={loading}>
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            刷新
          </Button>
          <Button size="sm" variant="ghost" onClick={exportFile} disabled={!cookies.length}>
            <Download className="h-3.5 w-3.5" />
            导出
          </Button>
          <Button size="sm" variant="secondary" onClick={() => setConfirmClear(true)} disabled={!cookies.length}>
            <Trash2 className="h-3.5 w-3.5" />
            全部清除
          </Button>
        </div>
      }
    >
      <Input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="按域名过滤" className="mb-3 w-64" />
      <Table columns={columns} data={shown} rowKey={(c) => `${c.domain}|${c.path}|${c.name}`} emptyText="没有 Cookie" />
      <ConfirmModal
        open={confirmClear}
        onClose={() => setConfirmClear(false)}
        onConfirm={clearAll}
        title="清除 Cookie"
        content="清除后该实例在所有网站的登录都会失效, 无法撤销。"
        confirmText="清除"
        danger
      />
    </Card>
  )
}
