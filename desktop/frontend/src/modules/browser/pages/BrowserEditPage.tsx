import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import clsx from 'clsx'
import { ArrowLeft, ChevronDown, FolderOpen, Layers } from 'lucide-react'
import { Button, ConfirmModal, FormItem, Input, Modal, Select, Textarea, toast } from '../../../shared/components'
import { errorText } from '../../cloud/errors'
import {
  createBrowserProfile,
  fetchAllTags,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserSettings,
  fetchCoreExtendedInfo,
  fetchGroups,
  openUserDataDir,
  updateBrowserProfile,
} from '../api'
import { FingerprintPanel } from '../components/FingerprintPanel'
import { GroupSelect } from '../components/GroupSelect'
import { ProxyPickerModal } from '../components/ProxyPickerModal'
import { TagInput } from '../components/TagInput'
import type { BrowserCore, BrowserCoreExtended, BrowserGroup, BrowserProfileInput, BrowserProxy } from '../types'
import { coreSwitchWarnings } from '../utils/coreSwitch'

const DIRECT = '__direct__'

const BLANK: BrowserProfileInput = {
  profileName: '',
  userDataDir: '',
  coreId: '',
  fingerprintArgs: [],
  proxyId: '',
  proxyConfig: '',
  launchArgs: [],
  tags: [],
  keywords: [],
  groupId: '',
}

const lines = (s: string) => s.split('\n').map((l) => l.trim()).filter(Boolean)

function Section({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="grid grid-cols-[10rem_1fr] gap-6 border-t border-[var(--color-border-muted)] py-5 first:border-t-0 first:pt-0">
      <div>
        <h2 className="text-[13px] font-medium text-[var(--color-text-primary)]">{title}</h2>
        {note && <p className="mt-1 text-xs leading-5 text-[var(--color-text-muted)]">{note}</p>}
      </div>
      <div className="min-w-0 space-y-3">{children}</div>
    </section>
  )
}

/** 新建 / 编辑实例。新建时按"设置 → 新实例默认值"预填; 数据目录与启动参数收在"高级"里。 */
export function BrowserEditPage() {
  const { id = 'new' } = useParams()
  const creating = id === 'new'
  const navigate = useNavigate()

  const [form, setForm] = useState<BrowserProfileInput>(BLANK)
  const [argsText, setArgsText] = useState('')
  const [dirty, setDirty] = useState(false)
  const [ready, setReady] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [confirmLeave, setConfirmLeave] = useState(false)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [advanced, setAdvanced] = useState(false)

  const [cores, setCores] = useState<BrowserCore[]>([])
  const [coreInfo, setCoreInfo] = useState(new Map<string, BrowserCoreExtended>())
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroup[]>([])
  const [allTags, setAllTags] = useState<string[]>([])
  // 编辑前用的内核, 用来判断换内核的影响
  const [initialCore, setInitialCore] = useState<string | null>(null)

  useEffect(() => {
    ;(async () => {
      const [c, ext, p, g, t] = await Promise.all([fetchBrowserCores(), fetchCoreExtendedInfo(), fetchBrowserProxies(), fetchGroups(), fetchAllTags()])
      setCores(c)
      setCoreInfo(new Map(ext.map((e) => [e.coreId, e])))
      setProxies(p)
      setGroups(g)
      setAllTags(t)
      if (creating) {
        const d = await fetchBrowserSettings()
        setForm({ ...BLANK, fingerprintArgs: d.defaultFingerprintArgs ?? [], proxyId: p.some((x) => x.proxyId === d.defaultProxy) ? d.defaultProxy : '' })
        setArgsText((d.defaultLaunchArgs ?? []).join('\n'))
      } else {
        const cur = (await fetchBrowserProfiles()).find((x) => x.profileId === id)
        if (cur) {
          // keywords 界面上不再维护, 原样带回去
          const { profileName, userDataDir, coreId, fingerprintArgs, proxyId, proxyConfig, launchArgs, tags, keywords, groupId } = cur
          setForm({ profileName, userDataDir, coreId, fingerprintArgs, proxyId, proxyConfig, launchArgs, tags: tags ?? [], keywords: keywords ?? [], groupId: groupId ?? '' })
          setArgsText((launchArgs ?? []).join('\n'))
          setInitialCore(coreId)
        }
      }
      setReady(true)
    })().catch((e) => toast.error(errorText(e)))
  }, [id, creating])

  const set = <K extends keyof BrowserProfileInput>(k: K, v: BrowserProfileInput[K]) => {
    setDirty(true)
    setForm((f) => ({ ...f, [k]: v }))
  }

  const save = async () => {
    const name = form.profileName.trim()
    if (!name) return toast.error('请填写名称')
    setSaving(true)
    const input = { ...form, profileName: name, launchArgs: lines(argsText) }
    try {
      if (creating) await createBrowserProfile(input)
      else await updateBrowserProfile(id, input)
      toast.success(creating ? '实例已创建' : '已保存')
      setDirty(false)
      navigate('/browser/list')
    } catch (e) {
      setSaveError(errorText(e))
    } finally {
      setSaving(false)
    }
  }

  const back = () => (dirty ? setConfirmLeave(true) : navigate('/browser/list'))

  const defaultCore = cores.find((c) => c.isDefault)
  const coreOptions = useMemo(() => {
    if (!cores.length) return [{ value: '', label: '还没有内核, 先到"内核"页下载' }]
    const label = (c: BrowserCore, sep: string) => [c.coreName, coreInfo.get(c.coreId)?.chromeVersion].filter(Boolean).join(sep)
    const opts = [{ value: '', label: defaultCore ? `跟随默认 (${label(defaultCore, ' ')})` : '跟随默认 (未设置)' }, ...cores.map((c) => ({ value: c.coreId, label: label(c, ' · ') }))]
    // 同步空间里别的电脑选的内核本机可能没有, 保留这个值, 别悄悄改掉
    if (form.coreId && !cores.some((c) => c.coreId === form.coreId)) opts.push({ value: form.coreId, label: '本机没有这个内核 (启动时用默认)' })
    return opts
  }, [cores, coreInfo, defaultCore, form.coreId])

  const effective = (coreId: string) => coreInfo.get(coreId || defaultCore?.coreId || '')
  const warnings = creating || initialCore === null ? [] : coreSwitchWarnings(effective(initialCore), effective(form.coreId))

  const proxyOptions = [
    { value: '', label: '不用代理池 (直连或下方自定义地址)' },
    ...proxies.filter((p) => p.proxyId !== DIRECT).map((p) => ({ value: p.proxyId, label: p.groupName ? `${p.groupName} / ${p.proxyName}` : p.proxyName || p.proxyId })),
  ]

  const openDir = () => {
    if (!form.userDataDir.trim()) return toast.info('保存后才会生成数据目录')
    openUserDataDir(form.userDataDir).catch((e) => toast.error(errorText(e)))
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="mb-4 flex shrink-0 items-center gap-2">
        <button onClick={back} aria-label="返回实例列表" className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--color-text-muted)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]">
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h1 className="truncate text-base font-semibold text-[var(--color-text-primary)]">{creating ? '新建实例' : form.profileName || '编辑实例'}</h1>
        <div className="ml-auto flex gap-2">
          <Button variant="secondary" onClick={back}>
            取消
          </Button>
          <Button onClick={save} loading={saving} disabled={!ready}>
            {creating ? '创建' : '保存'}
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto pb-8">
        <Section title="基本信息">
          <div className="grid grid-cols-2 gap-3">
            <FormItem label="名称" required>
              <Input value={form.profileName} onChange={(e) => set('profileName', e.target.value)} placeholder="如: 店铺 A 主号" autoFocus={creating} />
            </FormItem>
            <FormItem label="分组">
              <GroupSelect groups={groups} value={form.groupId ?? ''} onChange={(g) => set('groupId', g)} onCreated={(g) => setGroups((l) => [...l, g])} />
            </FormItem>
          </div>
          <FormItem label="标签">
            <TagInput value={form.tags} onChange={(t) => set('tags', t)} suggestions={allTags} placeholder="输入后回车, 或选已有标签" />
          </FormItem>
        </Section>

        <Section title="内核与代理" note='选"跟随默认"的实例, 以后改默认内核会一起生效'>
          <div className="grid grid-cols-2 gap-3">
            <FormItem label="内核">
              <Select value={form.coreId} onChange={(e) => set('coreId', e.target.value)} options={coreOptions} />
            </FormItem>
            <FormItem label="代理">
              <div className="flex gap-2">
                <Select value={form.proxyId} onChange={(e) => set('proxyId', e.target.value)} options={proxyOptions} className="min-w-0 flex-1" />
                <Button variant="secondary" onClick={() => setPickerOpen(true)} title="按分组挑选并测速" aria-label="按分组挑选代理">
                  <Layers className="h-3.5 w-3.5" />
                </Button>
              </div>
            </FormItem>
          </div>
          {warnings.map((w) => (
            <p key={w} className="text-xs leading-5 text-[var(--color-warning)]">
              {w}
            </p>
          ))}
          {!form.proxyId && (
            <FormItem label="自定义代理" hint="HTTP / HTTPS / SOCKS5, 留空直连">
              <Input value={form.proxyConfig} onChange={(e) => set('proxyConfig', e.target.value)} placeholder="socks5://user:pass@host:port" className="font-mono text-xs" />
            </FormItem>
          )}
        </Section>

        <Section title="指纹" note="网站看到的设备信息。不确定时保持默认">
          {ready && <FingerprintPanel value={form.fingerprintArgs} onChange={(a) => set('fingerprintArgs', a)} />}
        </Section>

        <section className="border-t border-[var(--color-border-muted)] pt-4">
          <button onClick={() => setAdvanced((v) => !v)} className="flex items-center gap-1 text-xs font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]">
            <ChevronDown className={clsx('h-3.5 w-3.5 transition-transform', !advanced && '-rotate-90')} />
            高级
          </button>
          {advanced && (
            <div className="ml-[calc(10rem+1.5rem)] min-w-0 space-y-3 pt-4">
              <FormItem label="用户数据目录" hint="留空自动生成">
                <div className="flex gap-2">
                  <Input value={form.userDataDir} onChange={(e) => set('userDataDir', e.target.value)} placeholder="留空自动生成" className="flex-1 font-mono text-xs" />
                  <Button variant="secondary" onClick={openDir} title="在资源管理器中打开" aria-label="打开数据目录">
                    <FolderOpen className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </FormItem>
              <FormItem label="额外启动参数" hint="每行一个">
                <Textarea
                  rows={5}
                  value={argsText}
                  onChange={(e) => {
                    setArgsText(e.target.value)
                    setDirty(true)
                  }}
                  placeholder="--disable-sync"
                  className="font-mono text-xs"
                />
              </FormItem>
            </div>
          )}
        </section>
      </div>

      <ProxyPickerModal open={pickerOpen} currentProxyId={form.proxyId} onSelect={(p) => set('proxyId', p.proxyId)} onClose={() => setPickerOpen(false)} />

      <ConfirmModal
        open={confirmLeave}
        onClose={() => setConfirmLeave(false)}
        onConfirm={() => navigate('/browser/list')}
        title="放弃修改?"
        content="还没保存的修改会丢失。"
        confirmText="放弃"
        cancelText="继续编辑"
        danger
      />

      <Modal open={!!saveError} onClose={() => setSaveError('')} title="保存失败" width="420px" footer={<Button onClick={() => setSaveError('')}>知道了</Button>}>
        <p className="text-[13px] text-[var(--color-text-secondary)]">{saveError}</p>
      </Modal>
    </div>
  )
}
