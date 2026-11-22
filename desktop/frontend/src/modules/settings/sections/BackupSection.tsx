import { useEffect, useRef, useState } from 'react'
import { Download, RotateCcw, Upload } from 'lucide-react'
import { Button, ConfirmModal, Modal, Progress, toast } from '../../../shared/components'
import { SectionHeader, SettingGroup, SettingRow } from '../../../shared/layout/SectionPage'
import { useEvents } from '../../../shared/hooks/useRuntimeEvents'
import { useBackupStore } from '../../../store/backupStore'
import { errorText } from '../../cloud/errors'
import { exportBackup, importBackup, resetAllData, type BackupProgress, type BackupResult } from '../api'

type Busy = 'none' | 'export' | 'import' | 'reset'
type LogLine = { id: number; time: string; text: string; error: boolean }

const MAX_LOG = 100
let lineId = 0

/** 进度条下方一行说明: [组件] 消息 (第 i/n 个) */
const describe = (p: BackupProgress) => {
  const who = p.componentName || p.componentId
  const pos = p.entryIndex && p.entryTotal ? ` (${p.entryIndex}/${p.entryTotal})` : ''
  return `${who ? `[${who}] ` : ''}${p.message}${pos}`
}

const statusOf = (p: BackupProgress) => (p.phase === 'error' ? 'error' : p.phase === 'done' ? 'success' : 'normal')

/** 导入结果的提示: 有模块失败时列出前几个。 */
function importSummary(r: BackupResult): { ok: boolean; text: string } {
  const failed = r.componentFailed ?? 0
  if (!r.partial && failed === 0) return { ok: true, text: `加载完成: 新增 ${r.imported ?? 0}, 跳过 ${r.skipped ?? 0}, 冲突 ${r.conflicts ?? 0}` }
  const names = (r.failedComponents ?? []).map((c) => c.componentName || c.componentId).filter(Boolean) as string[]
  const list = names.length ? `: ${names.slice(0, 3).join('、')}${names.length > 3 ? ` 等 ${names.length} 个` : ''}` : ''
  const total = r.componentTotal ?? 0
  return { ok: false, text: `部分加载成功${total ? ` (${total - failed}/${total})` : ''}, ${failed} 个模块失败${list}` }
}

export function BackupSection() {
  const [busy, setBusy] = useState<Busy>('none')
  const [exportState, setExportState] = useState<BackupProgress | null>(null)
  const [importState, setImportState] = useState<BackupProgress | null>(null)
  const [log, setLog] = useState<LogLine[]>([])
  const [chooseImport, setChooseImport] = useState(false)
  const [confirmReset, setConfirmReset] = useState(false)
  const logRef = useRef<HTMLDivElement>(null)
  const setImporting = useBackupStore((s) => s.setImporting)

  const pushLog = (text: string, error = false) =>
    setLog((prev) => {
      if (prev[prev.length - 1]?.text === text) return prev
      const time = new Date().toLocaleTimeString('zh-CN', { hour12: false })
      return [...prev, { id: ++lineId, time, text, error }].slice(-MAX_LOG)
    })

  useEvents(
    {
      'backup:export:progress': (p: BackupProgress) => {
        if (p.phase === 'cancelled') return setExportState(null)
        setExportState(p)
        pushLog(describe(p), p.phase === 'error')
      },
      'backup:import:progress': (p: BackupProgress) => setImportState(p.phase === 'cancelled' ? null : p),
    },
    [],
  )

  // 加载期间把进度同步给关闭窗口确认框
  useEffect(() => {
    setImporting(busy === 'import' ? { progress: importState?.progress ?? 0, message: importState?.message ?? '' } : null)
  }, [busy, importState, setImporting])
  useEffect(() => () => setImporting(null), [setImporting])

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
  }, [log])

  const doExport = async () => {
    setBusy('export')
    setLog([])
    setExportState({ phase: 'starting', progress: 0, message: '选择保存位置...' })
    try {
      const r = await exportBackup()
      if (r.cancelled) {
        setExportState(null)
        return
      }
      toast.success(r.zipPath ? `已导出到 ${r.zipPath}` : '导出完成')
    } catch (e) {
      const msg = errorText(e)
      setExportState((p) => ({ phase: 'error', progress: p?.progress ?? 0, message: msg }))
      pushLog(msg, true)
      toast.error(`导出失败: ${msg}`)
    } finally {
      setBusy('none')
    }
  }

  const doImport = async (resetFirst: boolean) => {
    setBusy('import')
    setImportState({ phase: 'starting', progress: 0, message: '选择备份文件...' })
    try {
      const r = await importBackup(resetFirst)
      if (r.cancelled) {
        setImportState(null)
        return
      }
      const sum = importSummary(r)
      if (sum.ok) toast.success(sum.text)
      else toast.warning(sum.text)
      setChooseImport(false)
      setImportState(null)
    } catch (e) {
      const msg = errorText(e)
      setImportState((p) => ({ phase: 'error', progress: p?.progress ?? 0, message: msg }))
      toast.error(`加载失败: ${msg}`)
    } finally {
      setBusy('none')
    }
  }

  const doReset = async () => {
    setBusy('reset')
    try {
      const r = await resetAllData()
      toast.success(r.message || '已恢复到初始状态')
    } catch (e) {
      toast.error(`初始化失败: ${errorText(e)}`)
    } finally {
      setBusy('none')
    }
  }

  const importing = busy === 'import'

  return (
    <div>
      <SectionHeader title="备份与恢复" description="把配置、实例、代理和浏览器登录数据打包成 ZIP, 换电脑或重装后加载" />

      <SettingGroup>
        <SettingRow label="导出备份" hint="不含内核与浏览器缓存; 内核在新电脑上重新下载即可">
          <Button variant="secondary" size="sm" onClick={doExport} loading={busy === 'export'} disabled={busy !== 'none' && busy !== 'export'}>
            <Download className="h-3.5 w-3.5" />
            导出
          </Button>
        </SettingRow>
        <SettingRow label="加载备份" hint="可与现有数据合并, 也可先清空再完整恢复">
          <Button
            variant="secondary"
            size="sm"
            disabled={busy !== 'none'}
            onClick={() => {
              setImportState(null)
              setChooseImport(true)
            }}
          >
            <Upload className="h-3.5 w-3.5" />
            加载
          </Button>
        </SettingRow>
        {exportState && (
          <div className="space-y-2 px-4 py-3">
            <div className="flex justify-between gap-3 text-xs">
              <span className="truncate text-[var(--color-text-secondary)]">{describe(exportState)}</span>
              {exportState.phase === 'done' && <span className="shrink-0 text-[var(--color-success)]">完成</span>}
            </div>
            <Progress percent={exportState.progress} size="sm" status={statusOf(exportState)} />
            {log.length > 0 && (
              <div ref={logRef} className="max-h-32 space-y-0.5 overflow-y-auto rounded-md bg-[var(--color-bg-subtle)] px-2 py-1.5 font-mono text-[11px] leading-5">
                {log.map((l) => (
                  <div key={l.id} className={l.error ? 'text-[var(--color-error)]' : 'text-[var(--color-text-secondary)]'}>
                    <span className="mr-2 text-[var(--color-text-muted)]">{l.time}</span>
                    {l.text}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </SettingGroup>

      <SettingGroup title="危险操作">
        <SettingRow label="初始化" hint="删除本机全部实例、代理和设置, 恢复到刚安装的状态">
          <Button variant="danger" size="sm" onClick={() => setConfirmReset(true)} loading={busy === 'reset'} disabled={busy !== 'none' && busy !== 'reset'}>
            <RotateCcw className="h-3.5 w-3.5" />
            初始化
          </Button>
        </SettingRow>
      </SettingGroup>

      <ConfirmModal
        open={confirmReset}
        onClose={() => setConfirmReset(false)}
        onConfirm={doReset}
        title="初始化"
        content="本机的实例 (含登录数据)、代理、分组和设置都会被删除, 无法撤销。建议先导出一份备份。云端同步空间的数据不受影响。"
        confirmText="初始化"
        danger
      />

      <Modal
        open={chooseImport}
        onClose={() => !importing && setChooseImport(false)}
        closable={!importing}
        title="加载备份"
        width="480px"
        footer={
          <>
            {!importing && (
              <Button variant="secondary" onClick={() => setChooseImport(false)}>
                取消
              </Button>
            )}
            <Button variant="danger" onClick={() => doImport(true)} disabled={importing}>
              清空后恢复
            </Button>
            <Button onClick={() => doImport(false)} loading={importing} disabled={importing}>
              合并
            </Button>
          </>
        }
      >
        <div className="space-y-2 text-sm text-[var(--color-text-secondary)]">
          <p>
            <span className="font-medium text-[var(--color-text-primary)]">合并</span>: 保留本机现有数据, 只加入备份里本机没有的; 同名但内容不同的文件以本机为准。
          </p>
          <p>
            <span className="font-medium text-[var(--color-text-primary)]">清空后恢复</span>: 先删除本机数据, 再完整还原备份。
          </p>
          {importState && (
            <div className="mt-3 space-y-2 rounded-md border border-[var(--color-border-default)] px-3 py-2">
              <div className="truncate text-xs">{describe(importState)}</div>
              <Progress percent={importState.progress} size="sm" status={statusOf(importState)} />
            </div>
          )}
          {importing && <p className="text-xs text-[var(--color-warning)]">加载期间请不要关闭应用。</p>}
        </div>
      </Modal>
    </div>
  )
}
