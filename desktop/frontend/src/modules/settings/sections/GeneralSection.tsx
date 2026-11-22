import { ThemeSwitcher } from '../../../shared/components'
import { SectionHeader, SettingGroup, SettingRow } from '../../../shared/layout/SectionPage'
import { UpdateGroup } from './UpdateGroup'

export function GeneralSection() {
  return (
    <div>
      <SectionHeader title="通用" />
      <SettingGroup title="外观">
        <SettingRow label="主题">
          <ThemeSwitcher />
        </SettingRow>
      </SettingGroup>
      <UpdateGroup />
    </div>
  )
}
