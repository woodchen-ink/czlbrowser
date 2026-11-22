import { useEffect, useMemo, useState } from 'react'
import { ChevronDown, ChevronRight, Dices, Wand2 } from 'lucide-react'
import { ConfirmModal, FormItem, Input, Select, Textarea } from '../../../shared/components'
import {
  FIELDS,
  GROUPS,
  PRESETS,
  RENDERERS,
  SYSTEM_TZ,
  TIMEZONES,
  localTimezone,
  parseArgs,
  randomSeed,
  toArgs,
  type FieldKey,
  type Fingerprint,
  type Option,
} from '../utils/fingerprint'

interface FingerprintPanelProps {
  /** 指纹启动参数 */
  value: string[]
  onChange: (args: string[]) => void
}

const UNSET: Option = { value: '', label: '不设置' }
const CUSTOM = '__custom__'
const BOOL: Option[] = [UNSET, { value: 'true', label: '开启' }, { value: 'false', label: '关闭' }]

/** 当前值不在候选里时补成一个选项, 避免下拉框显示成"不设置"。 */
const withCurrent = (list: Option[], v: string | undefined) => (v && !list.some((o) => o.value === v) ? [...list, { value: v, label: v }] : list)

/** 指纹配置表单: 实例编辑页与"新实例默认值"共用。 */
export function FingerprintPanel({ value, onChange }: FingerprintPanelProps) {
  const joined = value.join('\n')
  const [fp, setFp] = useState<Fingerprint>(() => parseArgs(value))
  const [customOpen, setCustomOpen] = useState<Partial<Record<FieldKey, boolean>>>({})
  const [rawOpen, setRawOpen] = useState(false)
  // 编辑原始参数时保留输入原文 (含空行), 失焦后再按解析结果显示
  const [rawText, setRawText] = useState<string | null>(null)
  const [confirmReseed, setConfirmReseed] = useState(false)

  // 外部换了一组参数 (切换实例 / 载入默认值) 时重新解析
  useEffect(() => setFp(parseArgs(joined.split('\n'))), [joined])

  const commit = (next: Fingerprint) => {
    setFp(next)
    onChange(toArgs(next))
  }
  const set = (patch: Partial<Fingerprint>) => commit({ ...fp, ...patch })

  const applyPreset = (id: string) => {
    const p = PRESETS.find((x) => x.id === id)
    // 套用预设时换一个新种子, 否则同预设的实例指纹一样
    if (p) commit({ ...p.values, seed: randomSeed(), extra: fp.extra })
  }

  const optionsFor = (key: FieldKey): Option[] => {
    if (key === 'webglRenderer') {
      const names = fp.webglVendor ? RENDERERS[fp.webglVendor] ?? [] : Object.values(RENDERERS).flat()
      return names.map((n) => ({ value: n, label: n }))
    }
    return [...(FIELDS.find((f) => f.key === key)?.options ?? [])]
  }

  const timezoneOptions = useMemo(
    () => withCurrent([UNSET, { value: SYSTEM_TZ, label: `跟随本机 (${localTimezone()})` }, ...TIMEZONES.map((z) => ({ value: z, label: z }))], fp.timezone),
    [fp.timezone],
  )

  const renderField = (key: FieldKey) => {
    const spec = FIELDS.find((f) => f.key === key)!
    const v = fp[key] ?? ''
    const onText = (s: string) => set({ [key]: s || undefined })

    if (spec.kind === 'text') return <Input value={v} onChange={(e) => onText(e.target.value)} placeholder={spec.placeholder} />
    if (spec.kind === 'bool') return <Select value={v} onChange={(e) => onText(e.target.value)} options={BOOL} />

    const list = optionsFor(key)
    const typing = spec.custom && (customOpen[key] || (v !== '' && !list.some((o) => o.value === v)))
    if (typing) {
      return (
        <div className="flex gap-1.5">
          <Input autoFocus={customOpen[key]} value={v} onChange={(e) => onText(e.target.value)} placeholder={spec.placeholder} />
          <button
            type="button"
            className="shrink-0 px-2 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
            onClick={() => {
              setCustomOpen((m) => ({ ...m, [key]: false }))
              set({ [key]: undefined })
            }}
          >
            选列表
          </button>
        </div>
      )
    }
    const all = [UNSET, ...withCurrent(list, v), ...(spec.custom ? [{ value: CUSTOM, label: '自定义...' }] : [])]
    return (
      <Select
        value={v}
        options={all}
        onChange={(e) => {
          const s = e.target.value
          if (s === CUSTOM) {
            setCustomOpen((m) => ({ ...m, [key]: true }))
            set({ [key]: undefined })
          } else if (key === 'webglVendor') {
            // 换厂商后原型号多半对不上, 一并清掉
            set({ webglVendor: s || undefined, webglRenderer: undefined })
          } else onText(s)
        }}
      />
    )
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <FormItem label="指纹种子" hint="决定 Canvas / WebGL / Audio 噪声">
          <div className="flex gap-1.5">
            <Input
              value={fp.seed ?? ''}
              onChange={(e) => set({ seed: e.target.value.trim() || undefined })}
              placeholder="留空按实例 ID 自动生成"
              className="font-mono"
            />
            <button
              type="button"
              title="随机生成"
              onClick={() => (fp.seed ? setConfirmReseed(true) : set({ seed: randomSeed() }))}
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]"
            >
              <Dices className="h-4 w-4" />
            </button>
          </div>
        </FormItem>
        <FormItem label="套用预设" hint="覆盖下方全部取值">
          <div className="relative">
            <Wand2 className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-muted)]" />
            <Select
              value=""
              onChange={(e) => applyPreset(e.target.value)}
              options={[{ value: '', label: '选择预设...' }, ...PRESETS.map((p) => ({ value: p.id, label: p.name }))]}
              className="pl-8"
            />
          </div>
        </FormItem>
      </div>

      {GROUPS.map((group) => (
        <div key={group}>
          <p className="mb-2 text-xs font-medium text-[var(--color-text-muted)]">{group}</p>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            {FIELDS.filter((f) => f.group === group && f.key !== 'seed').map((f) => (
              <FormItem key={f.key} label={f.label}>
                {f.key === 'timezone' ? (
                  <Select value={fp.timezone ?? ''} onChange={(e) => set({ timezone: e.target.value || undefined })} options={timezoneOptions} />
                ) : (
                  renderField(f.key as FieldKey)
                )}
              </FormItem>
            ))}
          </div>
        </div>
      ))}

      <div className="rounded-lg border border-[var(--color-border-default)]">
        <button
          type="button"
          onClick={() => setRawOpen((o) => !o)}
          className="flex w-full items-center gap-1.5 px-3 py-2 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
        >
          {rawOpen ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          原始参数
          {(fp.extra?.length ?? 0) > 0 && <span className="ml-auto">另有 {fp.extra!.length} 个自定义参数</span>}
        </button>
        {rawOpen && (
          <div className="border-t border-[var(--color-border-default)] p-3">
            <Textarea
              rows={6}
              value={rawText ?? toArgs(fp).join('\n')}
              onChange={(e) => {
                setRawText(e.target.value)
                commit(parseArgs(e.target.value.split('\n')))
              }}
              onBlur={() => setRawText(null)}
              placeholder="每行一个, 如 --fingerprint-brand=Chrome"
              className="font-mono text-xs"
            />
          </div>
        )}
      </div>

      <ConfirmModal
        open={confirmReseed}
        onClose={() => setConfirmReseed(false)}
        onConfirm={() => set({ seed: randomSeed() })}
        title="更换指纹种子"
        content="换种子后网站看到的 Canvas、WebGL、Audio 等特征都会变化, 相当于换了一台设备。确定更换?"
        confirmText="更换"
        danger
      />
    </div>
  )
}
