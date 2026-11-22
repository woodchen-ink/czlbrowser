import { useEffect, useState } from 'react'
import { Segmented, ThemeSwitcher, toast } from '../../../shared/components'
import { SectionHeader, SettingGroup, SettingRow } from '../../../shared/layout/SectionPage'
import { GetCloseAction, SetCloseAction } from '../../../wailsjs/go/main/App'
import { errorText } from '../../cloud/errors'
import { UpdateGroup } from './UpdateGroup'

type CloseAction = 'tray' | 'quit' | 'ask'

const CLOSE_OPTIONS: { value: CloseAction; label: string }[] = [
  { value: 'tray', label: '最小化到托盘' },
  { value: 'quit', label: '退出' },
  { value: 'ask', label: '每次询问' },
]

/** 点窗口关闭按钮时的行为 (存在 config.yaml), 改了立即生效。 */
function CloseActionRow() {
  const [value, setValue] = useState<CloseAction>('tray')
  useEffect(() => {
    GetCloseAction().then((v) => setValue(v as CloseAction))
  }, [])
  const change = async (v: CloseAction) => {
    const prev = value
    setValue(v)
    try {
      await SetCloseAction(v)
    } catch (e) {
      setValue(prev)
      toast.error(`保存失败: ${errorText(e)}`)
    }
  }
  return (
    <SettingRow label="关闭窗口时" hint="最小化到托盘时实例与代理继续运行, 托盘菜单里可以退出">
      <Segmented value={value} onChange={change} options={CLOSE_OPTIONS} />
    </SettingRow>
  )
}

export function GeneralSection() {
  return (
    <div>
      <SectionHeader title="通用" />
      <SettingGroup title="外观">
        <SettingRow label="主题">
          <ThemeSwitcher />
        </SettingRow>
      </SettingGroup>
      <SettingGroup title="窗口">
        <CloseActionRow />
      </SettingGroup>
      <UpdateGroup />
    </div>
  )
}
