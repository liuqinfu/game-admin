import { permissionMenus } from './permission-meta'
import type { PermissionCode } from './permission-types'
import type { PlatformScopeLevel } from '../platform-scope/PlatformScopeProvider'

export type BusinessGroupKey = 'overview' | 'agent' | 'tenant' | 'game' | 'growth' | 'finance' | 'security' | 'system'

export type NavItemConfig = {
  key: string
  labelKey: string
  permission: PermissionCode | PermissionCode[]
  allowedLevels?: PlatformScopeLevel[]
}

export type NavGroupConfig = {
  key: BusinessGroupKey
  labelKey: string
  items: NavItemConfig[]
}

export type PermissionMenuConfig = {
  labelKey: string
  codes: string[]
}

export type PermissionGroupConfig = {
  key: BusinessGroupKey
  labelKey: string
  menus: PermissionMenuConfig[]
}

export const navGroups: NavGroupConfig[] = [
  {
    key: 'overview',
    labelKey: 'businessGroup.overview',
    items: [{ key: '/', labelKey: 'layout.nav.dashboard', permission: 'dashboard:view' }],
  },
  {
    key: 'agent',
    labelKey: 'businessGroup.agent',
    items: [
      { key: '/agents', labelKey: 'layout.nav.agents', permission: 'agents:view' },
      { key: '/invite-codes', labelKey: 'layout.nav.inviteCodes', permission: 'invite-codes:view' },
      { key: '/users', labelKey: 'layout.nav.users', permission: 'users:view' },
      { key: '/bindings', labelKey: 'layout.nav.bindings', permission: 'bindings:view' },
      { key: '/binding-history', labelKey: 'layout.nav.bindingHistory', permission: 'binding-history:view' },
      { key: '/agent-invite-applications', labelKey: 'layout.nav.agentInviteApplications', permission: 'agent-invite-applications:view' },
    ],
  },
  {
    key: 'tenant',
    labelKey: 'businessGroup.tenant',
    items: [
      { key: '/tenant-brand/tenants', labelKey: 'layout.nav.tenantList', permission: 'tenants:view', allowedLevels: ['platform', 'tenant', 'brand'] },
      { key: '/tenant-brand/brands', labelKey: 'layout.nav.brandList', permission: 'brands:view', allowedLevels: ['platform', 'tenant', 'brand'] },
      { key: '/platform-config-center', labelKey: 'layout.nav.platformConfigCenter', permission: 'platform-configs:view', allowedLevels: ['platform', 'tenant', 'brand'] },
    ],
  },
  {
    key: 'game',
    labelKey: 'businessGroup.game',
    items: [
      { key: '/games', labelKey: 'layout.nav.games', permission: 'games:view' },
      { key: '/agent-game-access', labelKey: 'layout.nav.agentGameAccess', permission: 'agent-game-access:view', allowedLevels: ['platform', 'tenant', 'brand'] },
      { key: '/rules', labelKey: 'layout.nav.rules', permission: 'rules:view' },
    ],
  },
  {
    key: 'growth',
    labelKey: 'businessGroup.growth',
    items: [
      { key: '/activity-reward-center/rules', labelKey: 'layout.nav.activityRewardRules', permission: 'activity-rewards:view' },
      { key: '/activity-reward-center/records', labelKey: 'layout.nav.activityRewardRecords', permission: 'activity-rewards:view' },
    ],
  },
  {
    key: 'finance',
    labelKey: 'businessGroup.finance',
    items: [
      { key: '/orders', labelKey: 'layout.nav.orders', permission: 'orders:view' },
      { key: '/commissions', labelKey: 'layout.nav.commissions', permission: 'commissions:view' },
      { key: '/settlement-bills', labelKey: 'layout.nav.settlementBills', permission: 'settlement-bills:view' },
      { key: '/recalculation-tasks', labelKey: 'layout.nav.recalculationTasks', permission: 'recalculation-tasks:view' },
      { key: '/ledger', labelKey: 'layout.nav.ledger', permission: 'ledger:view' },
      { key: '/withdrawal-requests/list', labelKey: 'layout.nav.withdrawalRequestList', permission: 'withdrawal-requests:view' },
      { key: '/withdrawal-requests/process', labelKey: 'layout.nav.withdrawalRequestProcess', permission: 'withdrawal-requests:audit' },
    ],
  },
  {
    key: 'security',
    labelKey: 'businessGroup.security',
    items: [
      { key: '/audit', labelKey: 'layout.nav.audit', permission: 'audit:view', allowedLevels: ['platform'] },
      { key: '/risk/cases', labelKey: 'layout.nav.riskCases', permission: 'risk:view' },
      { key: '/risk/intelligence', labelKey: 'layout.nav.riskIntelligence', permission: 'risk:view' },
      { key: '/reports/agent', labelKey: 'layout.nav.reportAgent', permission: 'report:view' },
      { key: '/reports/team', labelKey: 'layout.nav.reportTeam', permission: 'report:view' },
      { key: '/reports/settlement', labelKey: 'layout.nav.reportSettlement', permission: 'report:view' },
      { key: '/reports/progress', labelKey: 'layout.nav.reportProgress', permission: 'report:view' },
      { key: '/reports/data-platform', labelKey: 'layout.nav.reportDataPlatform', permission: 'report:view' },
    ],
  },
  {
    key: 'system',
    labelKey: 'businessGroup.system',
    items: [
      { key: '/rbac/users', labelKey: 'layout.nav.rbacUsers', permission: 'rbac:users:view', allowedLevels: ['platform', 'tenant', 'brand'] },
      { key: '/rbac/roles', labelKey: 'layout.nav.rbacRoles', permission: 'rbac:roles:view', allowedLevels: ['platform', 'tenant', 'brand'] },
      { key: '/rbac/permissions', labelKey: 'layout.nav.rbacPermissions', permission: 'rbac:permissions:view', allowedLevels: ['platform', 'tenant', 'brand'] },
    ],
  },
]

export function firstAccessibleNavPath(permissions: PermissionCode[]) {
  return navGroups
    .flatMap((group) => group.items)
    .find((item) => {
      const required = Array.isArray(item.permission) ? item.permission : [item.permission]
      return required.some((permission) => permissions.includes(permission))
    })?.key ?? '/'
}

const menuByLabelKey = permissionMenus.reduce<Record<string, typeof permissionMenus[number]>>((acc, menu) => {
  acc[menu.labelKey] = menu
  return acc
}, {})

function permissionMenu(labelKey: string) {
  const menu = menuByLabelKey[labelKey]
  if (!menu) {
    throw new Error(`Missing permission menu metadata: ${labelKey}`)
  }
  return menu
}

export const permissionGroups: PermissionGroupConfig[] = [
  {
    key: 'agent',
    labelKey: 'businessGroup.agent',
    menus: [permissionMenu('layout.nav.agents'), permissionMenu('layout.nav.inviteCodes'), permissionMenu('layout.nav.users'), permissionMenu('layout.nav.bindings')],
  },
  {
    key: 'tenant',
    labelKey: 'businessGroup.tenant',
    menus: [permissionMenu('layout.nav.tenantBrand'), permissionMenu('layout.nav.platformConfigCenter')],
  },
  {
    key: 'game',
    labelKey: 'businessGroup.game',
    menus: [permissionMenu('layout.nav.games'), permissionMenu('layout.nav.agentGameAccess'), permissionMenu('layout.nav.rules')],
  },
  {
    key: 'growth',
    labelKey: 'businessGroup.growth',
    menus: [permissionMenu('layout.nav.activityRewardCenter')],
  },
  {
    key: 'finance',
    labelKey: 'businessGroup.finance',
    menus: [
      permissionMenu('layout.nav.orders'),
      permissionMenu('layout.nav.commissions'),
      permissionMenu('layout.nav.settlementBills'),
      permissionMenu('layout.nav.recalculationTasks'),
      permissionMenu('layout.nav.withdrawalRequests'),
    ],
  },
  {
    key: 'security',
    labelKey: 'businessGroup.security',
    menus: [permissionMenu('layout.nav.audit'), permissionMenu('layout.nav.risk'), permissionMenu('layout.nav.reports')],
  },
  {
    key: 'system',
    labelKey: 'businessGroup.system',
    menus: [permissionMenu('layout.nav.rbac')],
  },
]
