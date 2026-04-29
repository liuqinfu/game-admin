import type { PermissionCode } from './permission-types'

export type BackendPermissionCode =
  | 'dashboard:view'
  | 'audit:read'
  | 'agent:read'
  | 'agent:write'
  | 'tenant:read'
  | 'tenant:write'
  | 'brand:read'
  | 'brand:write'
  | 'invite_code:manage'
  | 'player:read'
  | 'player:write'
  | 'binding:manage'
  | 'game:read'
  | 'game:write'
  | 'game:publish'
  | 'agent_game_access:read'
  | 'agent_game_access:write'
  | 'rule:read'
  | 'rule:write'
  | 'rule:publish'
  | 'activity_reward:read'
  | 'activity_reward:write'
  | 'activity_reward:publish'
  | 'settlement:read'
  | 'settlement:execute'
  | 'withdrawal:read'
  | 'withdrawal:execute'
  | 'withdrawal_request:read'
  | 'withdrawal_request:audit'
  | 'withdrawal-requests:view'
  | 'withdrawal-requests:audit'
  | 'settlement_bill:read'
  | 'settlement_bill:export'
  | 'settlement_bill:confirm'
  | 'settlement-bills:view'
  | 'settlement-bills:export'
  | 'settlement-bills:confirm'
  | 'recalculation_task:read'
  | 'recalculation_task:create'
  | 'recalculation-tasks:view'
  | 'recalculation-tasks:create'
  | 'risk:read'
  | 'risk:write'
  | 'platform_config:read'
  | 'platform_config:write'
  | 'report:read'
  | 'reports:view'
  | 'rbac:permissions:view'
  | 'rbac:users:view'
  | 'rbac:users:write'
  | 'rbac:roles:view'
  | 'rbac:roles:write'

export type PermissionRisk = 'read' | 'write' | 'publish' | 'finance' | 'security'

export type PermissionMeta = {
  code: BackendPermissionCode
  labelKey: string
  frontendPermissions: PermissionCode[]
  risk: PermissionRisk
}

export type PermissionMenuMeta = {
  labelKey: string
  codes: BackendPermissionCode[]
}

function inferRisk(code: BackendPermissionCode): PermissionRisk {
  if (code.includes('publish')) return 'publish'
  if (code.includes('settlement') || code.includes('withdrawal')) return 'finance'
  if (code.includes('audit') || code.includes('risk') || code.includes('rbac')) return 'security'
  if (code.includes('write') || code.includes('create') || code.includes('execute') || code.includes('manage')) return 'write'
  return 'read'
}

function permissionMeta(code: BackendPermissionCode, frontendPermissions: PermissionCode[], labelCode: BackendPermissionCode = code): PermissionMeta {
  return {
    code,
    labelKey: `rbac.permission.${labelCode.replace(/[:-]/g, '_')}`,
    frontendPermissions,
    risk: inferRisk(code),
  }
}

export const permissionMenus: PermissionMenuMeta[] = [
  { labelKey: 'layout.nav.dashboard', codes: ['dashboard:view'] },
  { labelKey: 'layout.nav.agents', codes: ['agent:read', 'agent:write'] },
  { labelKey: 'layout.nav.inviteCodes', codes: ['invite_code:manage'] },
  { labelKey: 'layout.nav.users', codes: ['player:read', 'player:write'] },
  { labelKey: 'layout.nav.bindings', codes: ['binding:manage'] },
  { labelKey: 'layout.nav.tenantBrand', codes: ['tenant:read', 'tenant:write', 'brand:read', 'brand:write'] },
  { labelKey: 'layout.nav.platformConfigCenter', codes: ['platform_config:read', 'platform_config:write'] },
  { labelKey: 'layout.nav.games', codes: ['game:read', 'game:write', 'game:publish'] },
  { labelKey: 'layout.nav.agentGameAccess', codes: ['agent_game_access:read', 'agent_game_access:write'] },
  { labelKey: 'layout.nav.rules', codes: ['rule:read', 'rule:write', 'rule:publish'] },
  { labelKey: 'layout.nav.activityRewardCenter', codes: ['activity_reward:read', 'activity_reward:write', 'activity_reward:publish'] },
  { labelKey: 'layout.nav.orders', codes: ['settlement:read'] },
  { labelKey: 'layout.nav.commissions', codes: ['settlement:execute'] },
  { labelKey: 'layout.nav.settlementBills', codes: ['settlement_bill:read', 'settlement_bill:confirm', 'settlement_bill:export'] },
  { labelKey: 'layout.nav.recalculationTasks', codes: ['recalculation_task:read', 'recalculation_task:create'] },
  { labelKey: 'layout.nav.withdrawalRequests', codes: ['withdrawal:read', 'withdrawal:execute'] },
  { labelKey: 'layout.nav.audit', codes: ['audit:read'] },
  { labelKey: 'layout.nav.risk', codes: ['risk:read', 'risk:write'] },
  { labelKey: 'layout.nav.reports', codes: ['report:read'] },
  { labelKey: 'layout.nav.rbac', codes: ['rbac:permissions:view', 'rbac:users:view', 'rbac:users:write', 'rbac:roles:view', 'rbac:roles:write'] },
]

export const permissionMetas: PermissionMeta[] = [
  permissionMeta('dashboard:view', ['dashboard:view']),
  permissionMeta('audit:read', ['audit:view']),
  permissionMeta('agent:read', ['agents:view']),
  permissionMeta('agent:write', ['agents:create', 'agents:status:update']),
  permissionMeta('tenant:read', ['tenants:view']),
  permissionMeta('tenant:write', ['tenants:create', 'tenants:status:update', 'tenants:delete']),
  permissionMeta('brand:read', ['brands:view']),
  permissionMeta('brand:write', ['brands:create', 'brands:status:update', 'brands:delete']),
  permissionMeta('invite_code:manage', ['invite-codes:view', 'invite-codes:create']),
  permissionMeta('player:read', ['users:view']),
  permissionMeta('player:write', ['bindings:create']),
  permissionMeta('binding:manage', ['bindings:view', 'binding-history:view', 'agent-invite-applications:view', 'agent-invite-applications:create', 'agent-invite-applications:audit']),
  permissionMeta('game:read', ['games:view']),
  permissionMeta('game:write', ['games:create', 'games:update', 'games:delete']),
  permissionMeta('game:publish', ['games:status:update']),
  permissionMeta('agent_game_access:read', ['agent-game-access:view']),
  permissionMeta('agent_game_access:write', ['agent-game-access:write']),
  permissionMeta('rule:read', ['rules:view']),
  permissionMeta('rule:write', ['rules:create']),
  permissionMeta('rule:publish', ['rules:publish']),
  permissionMeta('activity_reward:read', ['activity-rewards:view']),
  permissionMeta('activity_reward:write', ['activity-rewards:create']),
  permissionMeta('activity_reward:publish', ['activity-rewards:status:update']),
  permissionMeta('settlement:read', ['orders:view', 'commissions:view']),
  permissionMeta('settlement:execute', ['ledger:view']),
  permissionMeta('withdrawal:read', ['withdrawal-requests:view']),
  permissionMeta('withdrawal:execute', ['withdrawal-requests:audit']),
  permissionMeta('withdrawal_request:read', ['withdrawal-requests:view'], 'withdrawal:read'),
  permissionMeta('withdrawal_request:audit', ['withdrawal-requests:audit'], 'withdrawal:execute'),
  permissionMeta('withdrawal-requests:view', ['withdrawal-requests:view'], 'withdrawal:read'),
  permissionMeta('withdrawal-requests:audit', ['withdrawal-requests:audit'], 'withdrawal:execute'),
  permissionMeta('settlement_bill:read', ['settlement-bills:view']),
  permissionMeta('settlement_bill:export', ['settlement-bills:export']),
  permissionMeta('settlement_bill:confirm', ['settlement-bills:confirm']),
  permissionMeta('settlement-bills:view', ['settlement-bills:view'], 'settlement_bill:read'),
  permissionMeta('settlement-bills:export', ['settlement-bills:export'], 'settlement_bill:export'),
  permissionMeta('settlement-bills:confirm', ['settlement-bills:confirm'], 'settlement_bill:confirm'),
  permissionMeta('recalculation_task:read', ['recalculation-tasks:view']),
  permissionMeta('recalculation_task:create', ['recalculation-tasks:create']),
  permissionMeta('recalculation-tasks:view', ['recalculation-tasks:view'], 'recalculation_task:read'),
  permissionMeta('recalculation-tasks:create', ['recalculation-tasks:create'], 'recalculation_task:create'),
  permissionMeta('risk:read', ['risk:view']),
  permissionMeta('risk:write', ['risk:view']),
  permissionMeta('platform_config:read', ['platform-configs:view']),
  permissionMeta('platform_config:write', ['platform-configs:write']),
  permissionMeta('report:read', ['report:view']),
  permissionMeta('reports:view', ['report:view'], 'report:read'),
  permissionMeta('rbac:permissions:view', ['rbac:permissions:view']),
  permissionMeta('rbac:users:view', ['rbac:users:view']),
  permissionMeta('rbac:users:write', ['rbac:users:write']),
  permissionMeta('rbac:roles:view', ['rbac:roles:view']),
  permissionMeta('rbac:roles:write', ['rbac:roles:write']),
]


const menuPermissionCodes = new Set(permissionMenus.flatMap((menu) => menu.codes))
const metaPermissionCodes = new Set(permissionMetas.map((meta) => meta.code))
const missingMetaCodes = [...menuPermissionCodes].filter((code) => !metaPermissionCodes.has(code))

if (missingMetaCodes.length > 0) {
  throw new Error(`Missing permission metadata: ${missingMetaCodes.join(', ')}`)
}

export const permissionMetaByCode = permissionMetas.reduce<Record<string, PermissionMeta>>((acc, meta) => {
  acc[meta.code] = meta
  return acc
}, {})

export function frontendPermissionsForBackendCode(code: string): PermissionCode[] {
  return permissionMetaByCode[code]?.frontendPermissions ?? []
}

export function labelKeyForBackendPermission(code: string) {
  return permissionMetaByCode[code]?.labelKey
}
