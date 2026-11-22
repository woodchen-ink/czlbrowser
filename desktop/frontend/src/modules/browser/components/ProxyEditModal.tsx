import { useEffect, useState } from 'react'
import { Button, FormItem, Input, Modal, Select, toast } from '../../../shared/components'
import type { BrowserProxy } from '../types'
import { joinProxy, splitProxy, type ProxyFields, type ProxyScheme } from '../utils/proxyAddress'
import { TagInput } from './TagInput'

interface Props {
  /** null 关闭; { proxy: undefined } 新增; 否则编辑该代理 */
  target: { proxy?: BrowserProxy; group?: string } | null
  groups: string[]
  tagSuggestions: string[]
  onClose: () => void
  onSave: (p: Pick<BrowserProxy, 'proxyName' | 'proxyConfig' | 'groupName' | 'tags'>) => Promise<void>
}

type Form = ProxyFields & { name: string; group: string; tags: string[] }

const SCHEMES: { value: ProxyScheme; label: string }[] = [
  { value: 'socks5', label: 'SOCKS5' },
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
]

/** 新增 / 编辑一个代理。 */
export function ProxyEditModal({ target, groups, tagSuggestions, onClose, onSave }: Props) {
  const [f, setF] = useState<Form | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!target) return setF(null)
    const p = target.proxy
    setF({ ...splitProxy(p?.proxyConfig ?? ''), name: p?.proxyName ?? '', group: p?.groupName ?? target.group ?? '', tags: p?.tags ?? [] })
  }, [target])

  if (!f) return null
  const set = (p: Partial<Form>) => setF({ ...f, ...p })
  const adding = !target?.proxy

  const submit = async () => {
    if (!f.name.trim()) return toast.error('请填写名称')
    if (!f.host.trim() || !f.port.trim()) return toast.error('请填写主机和端口')
    const port = Number(f.port)
    if (!Number.isInteger(port) || port < 1 || port > 65535) return toast.error('端口应在 1 到 65535 之间')
    setSaving(true)
    try {
      await onSave({ proxyName: f.name.trim(), proxyConfig: joinProxy(f), groupName: f.group.trim(), tags: f.tags })
      onClose()
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={adding ? '新增代理' : '编辑代理'}
      width="500px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            取消
          </Button>
          <Button onClick={submit} loading={saving}>
            {adding ? '添加' : '保存'}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <FormItem label="名称" required>
          <Input value={f.name} onChange={(e) => set({ name: e.target.value })} placeholder="如: 美国住宅 01" autoFocus />
        </FormItem>
        <div className="grid grid-cols-2 gap-3">
          <FormItem label="分组" hint="输入新名称即新建">
            <Input value={f.group} onChange={(e) => set({ group: e.target.value })} placeholder="未分组" list="proxy-group-names" />
            <datalist id="proxy-group-names">
              {groups.map((g) => (
                <option key={g} value={g} />
              ))}
            </datalist>
          </FormItem>
          <FormItem label="协议" required>
            <Select value={f.scheme} onChange={(e) => set({ scheme: e.target.value as ProxyScheme })} options={SCHEMES} />
          </FormItem>
        </div>
        <div className="grid grid-cols-3 gap-3">
          <FormItem label="主机" required className="col-span-2">
            <Input value={f.host} onChange={(e) => set({ host: e.target.value })} placeholder="IP 或域名" />
          </FormItem>
          <FormItem label="端口" required>
            <Input inputMode="numeric" value={f.port} onChange={(e) => set({ port: e.target.value.replace(/\D/g, '') })} placeholder="1080" />
          </FormItem>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <FormItem label="账号">
            <Input value={f.user} onChange={(e) => set({ user: e.target.value })} placeholder="可不填" autoComplete="off" />
          </FormItem>
          <FormItem label="密码">
            <Input type="password" value={f.pass} onChange={(e) => set({ pass: e.target.value })} placeholder="可不填" autoComplete="new-password" />
          </FormItem>
        </div>
        <FormItem label="标签">
          <TagInput value={f.tags} onChange={(tags) => set({ tags })} suggestions={tagSuggestions} />
        </FormItem>
      </div>
    </Modal>
  )
}
