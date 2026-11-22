import { useSearchParams } from 'react-router-dom'
import { CreditCard, RefreshCw, ShieldCheck, Users } from 'lucide-react'
import { useCloudStore } from '../../store/cloudStore'
import { SectionPage, type SectionTab } from '../../shared/layout/SectionPage'
import { PlansSection } from './components/PlansSection'
import { SyncSection } from './components/SyncSection'
import { TeamSection } from './components/TeamSection'

// 「云端同步」页: 左侧分类 (同步与加密 / 团队 / 套餐与订阅), 右侧当前分类。分类写在 URL ?tab= 里, 只读横条等处可深链进来。

type TabKey = 'sync' | 'team' | 'plans'

export function CloudPage() {
  const [params, setParams] = useSearchParams()
  const billingEnabled = useCloudStore((s) => s.billing?.enabled)
  const keysLocked = useCloudStore((s) => s.sync?.keys === 'locked' || s.deviceRequests.length > 0)
  const refresh = useCloudStore((s) => s.refresh)
  const refreshBilling = useCloudStore((s) => s.refreshBilling)

  const tabs: SectionTab<TabKey>[] = [
    { key: 'sync', label: '同步与加密', icon: ShieldCheck, dot: keysLocked ? 'bg-[var(--color-warning)]' : undefined },
    { key: 'team', label: '团队', icon: Users },
    ...(billingEnabled !== false ? [{ key: 'plans' as const, label: '套餐与订阅', icon: CreditCard }] : []),
  ]
  const tab = (tabs.find((t) => t.key === params.get('tab'))?.key || 'sync') as TabKey
  const go = (key: TabKey) => setParams({ tab: key }, { replace: true })

  return (
    <SectionPage
      title="云端同步"
      tabs={tabs}
      current={tab}
      onChange={go}
      titleAction={
        <button
          title="刷新"
          aria-label="刷新"
          onClick={() => {
            void refresh()
            void refreshBilling(true)
          }}
          className="w-6 h-6 flex items-center justify-center rounded text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-accent-muted)]"
        >
          <RefreshCw className="w-3.5 h-3.5" />
        </button>
      }
    >
      {tab === 'sync' && <SyncSection />}
      {tab === 'team' && <TeamSection />}
      {tab === 'plans' && <PlansSection onGoTeam={() => go('team')} />}
    </SectionPage>
  )
}
