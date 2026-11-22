import { useSearchParams } from 'react-router-dom'
import { BookOpen, DatabaseBackup, FileText, SlidersHorizontal, Tags, Wand2 } from 'lucide-react'
import { SectionPage, type SectionTab } from '../../shared/layout/SectionPage'
import { useUpdateStore } from '../../store/updateStore'
import { GeneralSection } from './sections/GeneralSection'
import { DefaultsSection } from './sections/DefaultsSection'
import { TaxonomySection } from './sections/TaxonomySection'
import { BackupSection } from './sections/BackupSection'
import { LogsSection } from './sections/LogsSection'
import { GuideSection } from './sections/GuideSection'

// 设置: 左侧分类, 右侧当前分类; 分类在 URL ?tab= 里, 旧的日志 / 教程 / 标签管理路由重定向到这里。

type TabKey = 'general' | 'defaults' | 'taxonomy' | 'backup' | 'logs' | 'guide'

export function SettingsPage() {
  const [params, setParams] = useSearchParams()
  const hasUpdate = useUpdateStore((s) => !!s.result?.hasUpdate)

  const tabs: SectionTab<TabKey>[] = [
    { key: 'general', label: '通用', icon: SlidersHorizontal, dot: hasUpdate ? 'bg-[var(--color-error)]' : undefined },
    { key: 'defaults', label: '新实例默认值', icon: Wand2 },
    { key: 'taxonomy', label: '分组与标签', icon: Tags },
    { key: 'backup', label: '备份与恢复', icon: DatabaseBackup },
    { key: 'logs', label: '运行日志', icon: FileText },
    { key: 'guide', label: '使用教程', icon: BookOpen },
  ]
  const tab = (tabs.find((t) => t.key === params.get('tab'))?.key || 'general') as TabKey

  return (
    <SectionPage
      title="设置"
      tabs={tabs}
      current={tab}
      onChange={(key) => setParams({ tab: key }, { replace: true })}
      fill={tab === 'logs'}
    >
      {tab === 'general' && <GeneralSection />}
      {tab === 'defaults' && <DefaultsSection />}
      {tab === 'taxonomy' && <TaxonomySection />}
      {tab === 'backup' && <BackupSection />}
      {tab === 'logs' && <LogsSection />}
      {tab === 'guide' && <GuideSection />}
    </SectionPage>
  )
}
