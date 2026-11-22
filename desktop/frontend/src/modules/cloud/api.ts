// 云端同步、团队、付费的后端接口封装。类型来自 Wails 生成的模型。
import * as App from '../../wailsjs/go/main/App'
import type { main, billing, cloudsync, orgs, syncsdk } from '../../wailsjs/go/models'

export type SyncState = cloudsync.State
export type SpaceInfo = cloudsync.SpaceInfo
export type DeviceRequest = cloudsync.DeviceRequestInfo
export type Presence = syncsdk.Presence
export type BillingState = main.BillingState
export type Plan = billing.Plan
export type AddOn = billing.AddOn
export type PlanQuote = billing.PlanQuote
export type Entitlement = billing.Entitlement
export type Org = orgs.Org
export type OrgDetail = orgs.Detail
export type OrgMember = orgs.Member
export type OrgActivity = orgs.Activity
export type NewInviteCode = orgs.NewInviteCode
export type SpaceCopyResult = main.SpaceCopyResult

export const LOCAL_SPACE = 'local'

export const cloudApi = {
  // 同步
  state: () => App.SyncGetState(),
  refresh: () => App.SyncRefresh(),
  switchSpace: (key: string) => App.SyncSwitchSpace(key),
  syncNow: (key: string) => App.SyncNow(key),
  presence: (key: string) => App.SyncPresence(key),
  forgetSpace: (key: string, force: boolean) => App.SyncForgetSpace(key, force),
  unpushedCount: () => App.SyncUnpushedCount(),
  copyProfiles: (ids: string[], target: string, withState: boolean) => App.SyncCopyProfiles(ids, target, withState),
  copyProxies: (ids: string[], target: string) => App.SyncCopyProxies(ids, target),

  // 端到端加密
  enroll: () => App.SyncKeysEnroll(),
  unlock: (code: string) => App.SyncKeysUnlock(code),
  resetRecovery: () => App.SyncKeysResetRecovery(),
  saveRecoveryCode: (code: string) => App.SyncSaveRecoveryCode(code),
  deviceStart: () => App.SyncDeviceStart(),
  deviceCancel: () => App.SyncDeviceCancel(),
  deviceRequests: () => App.SyncDeviceRequests(),
  deviceApproveBegin: (id: string) => App.SyncDeviceApproveBegin(id),
  deviceApprove: (id: string, approve: boolean) => App.SyncDeviceApprove(id, approve),

  // 团队
  orgList: () => App.OrgList(),
  orgCreate: (name: string) => App.OrgCreate(name),
  orgDetail: (id: string) => App.OrgDetail(id),
  orgRename: (id: string, name: string) => App.OrgRename(id, name),
  orgInviteEmail: (id: string, email: string, role: string) => App.OrgInviteEmail(id, email, role),
  orgRevokeInvite: (id: string, inviteId: string) => App.OrgRevokeInvite(id, inviteId),
  orgCreateInviteCode: (id: string, role: string, maxUses: number, ttlDays: number) =>
    App.OrgCreateInviteCode(id, role, maxUses, ttlDays),
  orgRevokeInviteCode: (id: string, codeId: string) => App.OrgRevokeInviteCode(id, codeId),
  orgJoin: (code: string) => App.OrgJoin(code),
  orgSetRole: (id: string, memberId: string, role: string) => App.OrgSetRole(id, memberId, role),
  orgRemoveMember: (id: string, memberId: string) => App.OrgRemoveMember(id, memberId),
  orgTransfer: (id: string, memberId: string) => App.OrgTransfer(id, memberId),
  orgActivity: (id: string, before: string) => App.OrgActivity(id, before),
  orgLeave: (id: string, force: boolean) => App.OrgLeave(id, force),
  orgClose: (id: string, force: boolean) => App.OrgClose(id, force),

  // 付费
  billingState: () => App.BillingState(),
  billingRefresh: (force: boolean) => App.BillingRefresh(force),
  billingSpaceNeeds: () => App.BillingSpaceNeeds(),
  billingQuote: (plan: string, subject: string) => App.BillingQuotePlan(plan, subject),
  billingAddOns: (subject: string) => App.BillingAddOns(subject),
  billingOpenCheckout: (plan: string, subject: string, periods: number) => App.BillingOpenCheckout(plan, subject, periods),
  billingOpenAddOnCheckout: (addon: string, subject: string, qty: number) =>
    App.BillingOpenAddOnCheckout(addon, subject, qty),
  billingCheckPaid: (subject: string, before: string) => App.BillingCheckPaid(subject, before),
  billingRedeem: (code: string, subject: string) => App.BillingRedeem(code, subject),
  billingSetAddOnQuantity: (subject: string, addon: string, qty: number) =>
    App.BillingSetAddOnQuantity(subject, addon, qty),
  billingOpenManage: () => App.BillingOpenManage(),
}

/** 空间的显示名。 */
export function spaceLabel(sp: Pick<SpaceInfo, 'kind' | 'name'> | null | undefined): string {
  if (!sp) return '本机'
  if (sp.kind === 'personal') return '我的同步'
  return sp.name || '团队'
}

/** 订阅主体: 个人为 user, 团队为 org:<组织 id>。 */
export function subjectOf(orgId?: string): string {
  return orgId ? `org:${orgId}` : 'user'
}

/** 人民币分转「¥34」或「¥34.50」。 */
export function formatPrice(cents: number, currency = 'CNY'): string {
  const symbol = currency.toUpperCase() === 'USD' ? '$' : '¥'
  const yuan = cents / 100
  return `${symbol}${Number.isInteger(yuan) ? yuan : yuan.toFixed(2)}`
}

/** 字节数转「1.2 GB」。 */
export function formatBytes(n: number): string {
  if (!n || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 10 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

export const ROLE_LABELS: Record<string, string> = {
  owner: '所有者',
  admin: '管理员',
  member: '成员',
  viewer: '只读成员',
}
