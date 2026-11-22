import { useState } from 'react'
import { Check, X } from 'lucide-react'
import { Button, Input, Select, toast } from '../../../shared/components'
import { errorText } from '../../cloud/errors'
import { createGroup } from '../api'
import type { BrowserGroup } from '../types'

interface Props {
  groups: BrowserGroup[]
  value: string
  onChange: (groupId: string) => void
  /** 新建分组成功后通知父组件把它加进列表 */
  onCreated: (g: BrowserGroup) => void
}

const NEW = '\u0000new'

/** 分组下拉; 末项"新建分组"就地切成输入框, 回车创建并选中。 */
export function GroupSelect({ groups, value, onChange, onCreated }: Props) {
  const [draft, setDraft] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const create = async () => {
    const name = draft?.trim()
    if (!name) return
    // 已有同名 (不分大小写) 就直接选它
    const same = groups.find((g) => g.groupName.toLowerCase() === name.toLowerCase())
    if (same) {
      onChange(same.groupId)
      setDraft(null)
      return
    }
    setBusy(true)
    try {
      const g = await createGroup({ groupName: name, parentId: '', sortOrder: groups.length })
      if (g) {
        onCreated(g)
        onChange(g.groupId)
      }
      setDraft(null)
    } catch (e) {
      toast.error(`创建分组失败: ${errorText(e)}`)
    } finally {
      setBusy(false)
    }
  }

  if (draft !== null) {
    return (
      <div className="flex gap-1.5">
        <Input
          autoFocus
          value={draft}
          maxLength={32}
          placeholder="分组名称, 回车创建"
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              create()
            } else if (e.key === 'Escape') setDraft(null)
          }}
        />
        <Button variant="secondary" onClick={create} loading={busy} aria-label="创建分组">
          {!busy && <Check className="h-3.5 w-3.5" />}
        </Button>
        <Button variant="ghost" onClick={() => setDraft(null)} aria-label="取消">
          <X className="h-3.5 w-3.5" />
        </Button>
      </div>
    )
  }

  return (
    <Select
      value={groups.some((g) => g.groupId === value) ? value : ''}
      onChange={(e) => (e.target.value === NEW ? setDraft('') : onChange(e.target.value))}
      options={[{ value: '', label: '未分组' }, ...groups.map((g) => ({ value: g.groupId, label: g.groupName })), { value: NEW, label: '+ 新建分组...' }]}
    />
  )
}
