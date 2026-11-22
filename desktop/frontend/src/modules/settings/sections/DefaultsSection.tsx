import { useEffect, useMemo, useState } from 'react'
import { ChevronDown, FolderOpen } from 'lucide-react'
import clsx from 'clsx'
import { Button, Input, Select, Textarea, toast } from '../../../shared/components'
import { SECTION_WIDTH, SectionHeader, SettingGroup, SettingRow } from '../../../shared/layout/SectionPage'
import { FingerprintPanel } from '../../browser/components/FingerprintPanel'
import type { BrowserCore, BrowserProxy, BrowserSettings } from '../../browser/types'
import {
  fetchBrowserCores,
  fetchBrowserProxies,
  fetchBrowserSettings,
  openUserDataDir,
  saveBrowserSettings,
  setDefaultBrowserCore,
} from '../../browser/api'

// 新实例默认值: 新建实例时预填的内核 / 代理 / 指纹, 原始参数收进「高级」。
// 内核「跟随默认」的实例启动时才解析默认内核, 改这里对它们同样生效。

const DIRECT_PROXY_ID = '__direct__'

export function DefaultsSection() {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [saved, setSaved] = useState<BrowserSettings | null>(null)
  const [savedCoreId, setSavedCoreId] = useState('')
  const [form, setForm] = useState<BrowserSettings>({ userDataRoot: '', defaultProxy: '', defaultFingerprintArgs: [], defaultLaunchArgs: [] })
  const [coreId, setCoreId] = useState('')
  const [launchArgsText, setLaunchArgsText] = useState('')
  const [advancedOpen, setAdvancedOpen] = useState(false)

  useEffect(() => {
    let alive = true
    void (async () => {
      try {
        const [s, c, p] = await Promise.all([fetchBrowserSettings(), fetchBrowserCores(), fetchBrowserProxies()])
        if (!alive) return
        setSaved(s)
        setForm(s)
        setLaunchArgsText((s.defaultLaunchArgs || []).join('\n'))
        setCores(c)
        setProxies(p.filter((x) => x.proxyId !== DIRECT_PROXY_ID))
        const def = c.find((x) => x.isDefault)?.coreId || ''
        setCoreId(def)
        setSavedCoreId(def)
      } catch (err: any) {
        toast.error(err?.message || '读取默认值失败')
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [])

  const launchArgs = useMemo(() => launchArgsText.split('\n').map((s) => s.trim()).filter(Boolean), [launchArgsText])

  const dirty =
    !!saved &&
    (coreId !== savedCoreId ||
      form.defaultProxy !== saved.defaultProxy ||
      form.userDataRoot !== saved.userDataRoot ||
      form.defaultFingerprintArgs.join('\n') !== saved.defaultFingerprintArgs.join('\n') ||
      launchArgs.join('\n') !== saved.defaultLaunchArgs.join('\n'))

  // 默认代理: 代理池里的代理按 ID 存; 旧版本存的代理地址原文作为「自定义」保留
  const proxyOptions = useMemo(() => {
    const opts = [{ value: '', label: '不使用代理' }]
    const legacy = form.defaultProxy && form.defaultProxy.includes('://')
    if (legacy) opts.push({ value: form.defaultProxy, label: `自定义：${form.defaultProxy}` })
    for (const p of proxies) opts.push({ value: p.proxyId, label: p.groupName ? `${p.groupName} / ${p.proxyName}` : p.proxyName })
    const unknown = form.defaultProxy && !legacy && !proxies.some((p) => p.proxyId === form.defaultProxy)
    if (unknown) opts.push({ value: form.defaultProxy, label: '当前空间没有这个代理' })
    return opts
  }, [proxies, form.defaultProxy])

  const handleSave = async () => {
    setSaving(true)
    try {
      const next: BrowserSettings = { ...form, defaultLaunchArgs: launchArgs }
      await saveBrowserSettings(next)
      if (coreId && coreId !== savedCoreId) await setDefaultBrowserCore(coreId)
      setSaved(next)
      setSavedCoreId(coreId)
      toast.success('已保存')
    } catch (err: any) {
      toast.error(err?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleReset = () => {
    if (!saved) return
    setForm(saved)
    setLaunchArgsText(saved.defaultLaunchArgs.join('\n'))
    setCoreId(savedCoreId)
  }

  return (
    <div>
      <SectionHeader
        title="新实例默认值"
        description="新建实例时预填这些设置，之后每个实例仍可单独修改"
        actions={
          <>
            {dirty && (
              <Button variant="ghost" size="sm" onClick={handleReset}>
                撤销
              </Button>
            )}
            <Button size="sm" onClick={handleSave} loading={saving} disabled={!dirty || loading}>
              保存
            </Button>
          </>
        }
      />

      <SettingGroup title="内核与代理">
        <SettingRow label="默认内核" hint="实例内核选「跟随默认」时使用；改了对这些实例立即生效">
          <Select
            className="w-56"
            value={coreId}
            disabled={loading || cores.length === 0}
            onChange={(e) => setCoreId(e.target.value)}
            options={cores.length === 0 ? [{ value: '', label: '还没有内核，先去「内核」下载' }] : cores.map((c) => ({ value: c.coreId, label: c.coreName }))}
          />
        </SettingRow>
        <SettingRow label="默认代理" hint="从当前空间的代理池里选">
          <Select
            className="w-56"
            value={form.defaultProxy}
            disabled={loading}
            onChange={(e) => setForm((f) => ({ ...f, defaultProxy: e.target.value }))}
            options={proxyOptions}
          />
        </SettingRow>
      </SettingGroup>

      <SettingGroup title="默认指纹">
        <div className="px-4 py-3">
          {!loading && (
            <FingerprintPanel
              value={form.defaultFingerprintArgs}
              onChange={(args) => setForm((f) => ({ ...f, defaultFingerprintArgs: args }))}
            />
          )}
        </div>
      </SettingGroup>

      <section className={SECTION_WIDTH}>
        <button
          onClick={() => setAdvancedOpen((v) => !v)}
          className="flex items-center gap-1 mb-1.5 text-xs font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
        >
          <ChevronDown className={clsx('w-3.5 h-3.5 transition-transform', !advancedOpen && '-rotate-90')} />
          高级
        </button>
        {advancedOpen && (
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] divide-y divide-[var(--color-border-muted)]">
            <SettingRow label="额外启动参数" hint="每行一个 Chromium 命令行参数，追加到每个新实例" stacked>
              <Textarea
                rows={4}
                value={launchArgsText}
                onChange={(e) => setLaunchArgsText(e.target.value)}
                placeholder={'--no-first-run\n--disable-sync'}
                className="font-mono text-xs"
              />
            </SettingRow>
            <SettingRow label="用户数据目录" hint="实例浏览器数据的存放位置；改了以后已有实例的数据不会自动搬过去" stacked>
              <div className="flex gap-2">
                <Input
                  value={form.userDataRoot}
                  onChange={(e) => setForm((f) => ({ ...f, userDataRoot: e.target.value }))}
                  placeholder="data"
                  className="font-mono text-xs"
                />
                <Button variant="secondary" onClick={() => void openUserDataDir('')} title="打开目录">
                  <FolderOpen className="w-3.5 h-3.5" />
                  打开
                </Button>
              </div>
            </SettingRow>
          </div>
        )}
      </section>
    </div>
  )
}
