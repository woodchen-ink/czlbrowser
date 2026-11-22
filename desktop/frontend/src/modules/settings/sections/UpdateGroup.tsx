import { useState } from 'react'
import { RefreshCw, Download, RotateCw } from 'lucide-react'
import { Button, Badge, Progress, ConfirmModal, toast } from '../../../shared/components'
import { SettingGroup, SettingRow } from '../../../shared/layout/SectionPage'
import { useUpdateStore } from '../../../store/updateStore'

function formatSize(bytes: number) {
  if (!bytes) return ''
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatTime(value: string) {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

export function UpdateGroup() {
  const { result, checking, installing, progress, error, check, download, install } = useUpdateStore()
  const [confirmOpen, setConfirmOpen] = useState(false)

  const downloading = progress?.phase === 'downloading' || (!!result?.downloading && progress?.phase !== 'error')
  const ready = progress?.phase === 'ready' || (!!result?.downloaded && !downloading)

  const handleCheck = async () => {
    const res = await check()
    if (!res) {
      toast.error(useUpdateStore.getState().error || '检查更新失败')
    } else if (res.devBuild) {
      toast.info('开发构建不支持在线更新')
    } else if (!res.hasUpdate) {
      toast.success('当前已是最新版本')
    }
  }

  const handleInstall = async () => {
    setConfirmOpen(false)
    await install()
    const err = useUpdateStore.getState().error
    if (err) toast.error(err)
  }

  return (
    <SettingGroup title="软件更新" description="新版本安装包经签名校验后静默安装，完成后自动重启">
      <SettingRow
        label={
          <span className="flex items-center gap-2">
            当前版本 <span className="font-mono">{result?.currentVersion || '-'}</span>
            {result?.devBuild && <Badge size="sm">开发构建</Badge>}
            {result?.hasUpdate && <Badge size="sm" variant="success">可更新到 {result.latestVersion}</Badge>}
          </span>
        }
        hint={result?.checkedAt ? `上次检查：${formatTime(result.checkedAt)}` : undefined}
      >
        <Button variant="secondary" size="sm" onClick={handleCheck} loading={checking}>
          <RefreshCw className="w-3.5 h-3.5" />
          检查更新
        </Button>
      </SettingRow>

      {result?.hasUpdate && (
        <div className="px-4 py-3 space-y-3">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-[var(--color-text-muted)]">
            <span>发布时间：{formatTime(result.publishedAt) || '-'}</span>
            <span>安装包：{formatSize(result.size)}</span>
          </div>
          {result.changelog && (
            <pre className="max-h-40 overflow-y-auto whitespace-pre-wrap break-words font-sans text-xs leading-5 text-[var(--color-text-secondary)]">
              {result.changelog}
            </pre>
          )}
          {downloading && <Progress percent={progress?.progress ?? 0} size="sm" />}
          <div className="flex items-center gap-2">
            {ready ? (
              <Button size="sm" onClick={() => setConfirmOpen(true)} loading={installing}>
                <RotateCw className="w-3.5 h-3.5" />
                立即安装并重启
              </Button>
            ) : (
              <Button size="sm" onClick={download} loading={downloading}>
                <Download className="w-3.5 h-3.5" />
                {downloading ? `下载中 ${progress?.progress ?? 0}%` : '下载更新'}
              </Button>
            )}
          </div>
        </div>
      )}

      {error && <p className="px-4 py-2 text-xs text-[var(--color-error)]">{error}</p>}

      <ConfirmModal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={handleInstall}
        title="安装更新"
        content="安装前会关闭所有正在运行的浏览器实例和代理服务，应用将退出并在安装完成后自动重启。是否继续？"
        confirmText="安装并重启"
      />
    </SettingGroup>
  )
}
