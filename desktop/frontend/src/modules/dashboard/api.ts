import * as App from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

export type DashboardOverview = main.DashboardOverview

/** 控制台概览 (后端按当前空间聚合) */
export async function fetchDashboardOverview(): Promise<DashboardOverview> {
  return App.DashboardOverview()
}
