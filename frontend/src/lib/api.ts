import { frontendPermissionsForBackendCode } from '../config/permission-meta'
import type { PermissionCode } from '../config/permission-types'
export type { PermissionCode } from '../config/permission-types'

const DEFAULT_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api'
export const AUTH_TOKEN_KEY='***'

export type RecalculationScope = 'agent' | 'settlement_bill' | 'period' | 'all'

export interface UserProfile {
  id?: string | number
  username: string
  displayName?: string
  roles?: string[]
  permissions: PermissionCode[]
  scope?: {
    level?: 'platform' | 'tenant' | 'brand' | 'agent'
    tenantID?: number | null
    tenantName?: string
    tenantCode?: string
    brandID?: number | null
    brandName?: string
    brandCode?: string
    agentID?: number | null
  }
}

export type AgentStatus = 'pending' | 'active' | 'frozen' | 'disabled' | 'closed'
export type TenantStatus = 'active' | 'inactive' | 'disabled'
export type BrandStatus = 'active' | 'inactive' | 'disabled'
export type InviteCodeStatus = 'active' | 'disabled' | 'expired'
export type BindingStatus = 'bound' | 'pending_change' | 'changed' | 'released'
export type BindingSource = 'register' | 'manual' | 'backfill'
export type AgentInviteApplicationStatus = 'pending' | 'approved' | 'rejected'
export type GameStatus = 'draft' | 'online' | 'offline' | 'archived'
export type AccessStatus = 'enabled' | 'disabled'
export type RuleStatus = 'draft' | 'published' | 'disabled'
export type RuleScope = 'platform' | 'game' | 'agent' | 'agent_game'
export type RuleType = 'ratio' | 'fixed_share' | 'differential' | 'point' | 'capped'
export type OrderStatus = 'pending' | 'paid' | 'failed' | 'refunded' | 'cancelled' | 'closed'
export type CommissionStatus = 'pending' | 'frozen' | 'settled' | 'reversed' | 'voided'
export type SettlementBillStatus = 'pending' | 'generated' | 'confirmed' | 'cancelled'
export type SettlementPeriodType = 'daily' | 'weekly' | 'monthly'
export type RecalculationTaskStatus = 'pending' | 'processing' | 'completed' | 'failed'
export type RecalculationTaskType = 'settlement_bill' | 'commission'
export type LedgerDirection = 'credit' | 'debit'
export type LedgerType = 'income' | 'freeze' | 'unfreeze' | 'debit' | 'reverse' | 'adjust' | 'commission_income' | 'commission_reverse' | 'manual_adjust'
export type AuditResult = 'success' | 'failed' | 'rejected'
export type ActivityRewardRecordStatus = 'pending' | 'granted' | 'failed' | 'reversed'
export type WithdrawalRequestStatus = 'pending' | 'approved' | 'paying' | 'paid' | 'failed' | 'returned' | 'rejected' | 'closed' | 'cancelled'
export type WithdrawalReviewAction = 'approve' | 'reject'
export type WithdrawalPayoutAction = 'success' | 'fail' | 'return'

export interface Agent {
  id: number
  tenantID?: number | null
  brandID?: number | null
  agentNo: string
  name: string
  displayName?: string
  phone?: string
  email?: string
  status: AgentStatus
  level: number
  parentAgentID?: number | null
  countryCode?: string
  currency: string
  remark?: string
  createdAt: string
  updatedAt: string
}

export interface Tenant {
  id: number
  code: string
  name: string
  displayName?: string
  status: TenantStatus
  defaultBrandID?: number | null
  remark?: string
  createdAt: string
  updatedAt: string
}

export interface Brand {
  id: number
  tenantID: number
  code: string
  name: string
  displayName?: string
  status: BrandStatus
  domain?: string
  isDefault: boolean
  remark?: string
  createdAt: string
  updatedAt: string
}

export interface PlatformConfig {
  id: number
  tenantID?: number | null
  tenantCode?: string
  brandID?: number | null
  brandCode?: string
  key: string
  value: Record<string, unknown>
  description: string
}

export interface EnumDictionaryItem {
  value: string
  label: string
  labelEn?: string
  description?: string
}

export interface EnumDictionary {
  code: string
  key: string
  strict: boolean
  items: EnumDictionaryItem[]
}

export interface InviteCode {
  id: number
  tenantID?: number | null
  brandID?: number | null
  agentID: number
  code: string
  status: InviteCodeStatus
  isPrimary: boolean
  maxUseCount: number
  usedCount: number
  channel?: string
  remark?: string
  expiredAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface Player {
  id: number
  tenantID?: number | null
  brandID?: number | null
  playerNo: string
  platformUserID: string
  nickname?: string
  phone?: string
  countryCode?: string
  currency: string
  status: string
  registeredAt?: string
  createdAt: string
  updatedAt: string
}

export interface Binding {
  id: number
  tenantID?: number | null
  brandID?: number | null
  playerID: number
  agentID: number
  inviteCodeID?: number | null
  status: BindingStatus
  source: BindingSource
  boundAt: string
  effectiveFrom: string
  effectiveTo?: string | null
  approvedBy?: string
  approvalReason?: string
  remark?: string
  createdAt: string
  updatedAt: string
}

export interface BindingHistory {
  id: number
  tenantID?: number | null
  brandID?: number | null
  bindingID: number
  playerID: number
  fromAgentID?: number | null
  toAgentID: number
  inviteCodeID?: number | null
  status: BindingStatus
  source: BindingSource
  changedAt: string
  changedBy?: string
  changeReason?: string
  snapshotPayload?: unknown
  createdAt: string
  updatedAt: string
}

export interface AgentInviteApplication {
  id: number
  tenantID?: number | null
  brandID?: number | null
  applicantAgentID: number
  inviterAgentID: number
  inviteCodeID?: number | null
  status: AgentInviteApplicationStatus
  applyRemark?: string
  auditRemark?: string
  auditBy?: string
  auditedAt?: string | null
  approvedRelationID?: number | null
  createdAt: string
  updatedAt: string
}

export interface Game {
  id: number
  tenantID?: number | null
  brandID?: number | null
  gameCode: string
  name: string
  vendor?: string
  category?: string
  status: GameStatus
  isAgentable: boolean
  sort: number
  currency: string
  launchAt?: string | null
  offlineAt?: string | null
  remark?: string
  createdAt: string
  updatedAt: string
}

export interface GameIntegrationCredential {
  id: number
  gameID: number
  name: string
  accessKey: string
  secretKey?: string
  status: 'active' | 'disabled' | 'revoked'
  scopes: string[]
  createdAt: string
  lastUsedAt?: string | null
  rotatedAt?: string | null
  expiresAt?: string | null
  remark?: string
}

export interface GameCreateResult {
  game: Game
  integrationCredential?: GameIntegrationCredential
}

export interface AgentGameAccess {
  id: number
  tenantID?: number | null
  brandID?: number | null
  agentID: number
  gameID: number
  status: AccessStatus
  grantedBy?: string
  grantedAt: string
  effectiveFrom: string
  effectiveTo?: string | null
  settlementMemo?: string
  createdAt: string
  updatedAt: string
}

export interface AgentGameAccessListItem extends AgentGameAccess {
  agentName: string
  gameCode: string
  gameName: string
}

export interface Rule {
  id: number
  tenantID?: number | null
  brandID?: number | null
  ruleName: string
  scope: RuleScope
  ruleType?: RuleType
  status: RuleStatus
  priority: number
  version: number
  agentID?: number | null
  gameID?: number | null
  maxSettlementDepth: number
  commissionRate: number
  fixedAmount: number
  minAgentLevel?: number
  rechargeTypes?: string[]
  activityTags?: string[]
  capAmount?: number
  currency: string
  effectiveFrom: string
  effectiveTo?: string | null
  publishedAt?: string | null
  publishedBy?: string
  remark?: string
  createdAt: string
  updatedAt: string
}

export type ActivityRewardRuleStatus = 'draft' | 'active' | 'disabled'

export interface ActivityRewardRule {
  id: number
  name: string
  activityType: string
  rewardType: string
  status: ActivityRewardRuleStatus
  rewardValue: number
  currency: string
  triggerValue?: number | null
  dailyLimit?: number | null
  totalLimit?: number | null
  startAt?: string | null
  endAt?: string | null
  createdBy?: string
  remark?: string
  tenantID?: number | null
  brandID?: number | null
  createdAt: string
  updatedAt: string
}

export interface ActivityRewardRecord {
  id: number
  recordNo: string
  ruleID?: number | null
  ruleName?: string
  userID?: number | null
  playerID?: number | null
  agentID?: number | null
  rewardType: string
  rewardValue: number
  currency: string
  activityType?: string
  status: ActivityRewardRecordStatus
  grantedAt?: string | null
  createdAt: string
  updatedAt: string
  remark?: string
}

export interface Order {
  id: number
  tenantID?: number | null
  brandID?: number | null
  orderNo: string
  externalOrderNo?: string
  playerID: number
  gameID: number
  agentID?: number | null
  amount: number
  paidAmount?: number | null
  paymentChannelCost?: number | null
  grossProfitAmount?: number | null
  currency: string
  status: OrderStatus
  channel?: string
  paidAt?: string | null
  callbackAt?: string | null
  riskNote?: string
  freezeReason?: string
  reviewStatus?: string
  freezeVisible?: boolean
  createdAt: string
  updatedAt: string
}

export interface UpdateOrderProfitFactsPayload {
  paidAmount?: number
  paymentChannelCost?: number
  grossProfitAmount?: number
  remark?: string
}

export interface CommissionRecord {
  id: number
  tenantID?: number | null
  brandID?: number | null
  recordNo: string
  rechargeOrderID: number
  playerID: number
  agentID: number
  gameID: number
  ruleID?: number | null
  settlementDepth: number
  commissionBaseAmount: number
  commissionRate: number
  commissionAmount: number
  currency: string
  status: CommissionStatus
  estimatedAt: string
  settledAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface SettlementBill {
  id: number
  tenantID?: number | null
  brandID?: number | null
  billNo: string
  agentID: number
  agentName?: string
  periodType?: SettlementPeriodType | string
  periodStart: string
  periodEnd: string
  currency: string
  commissionAmount: number
  adjustmentAmount: number
  payableAmount: number
  status: SettlementBillStatus
  generatedAt?: string | null
  confirmedAt?: string | null
  confirmedBy?: string
  freezeVisible?: boolean
  summaryPayload?: {
    recordCount?: number
    [key: string]: unknown
  }
  remark?: string
  details?: Array<{
    id: number
    tenantID?: number | null
    brandID?: number | null
    settlementBillID: number
    agentID: number
    commissionRecordID?: number | null
    rechargeOrderID?: number | null
    referenceType: string
    referenceID: string
    commissionAmount: number
    adjustmentAmount: number
    amount: number
    currency: string
    occurredAt: string
    recordNo?: string
    orderNo?: string
    remark?: string
  }>
  createdAt: string
  updatedAt: string
}

export interface ConfirmSettlementBillPayload {
  remark?: string
}

export interface RecalculationTask {
  id: number
  tenantID?: number | null
  brandID?: number | null
  taskNo: string
  taskType: RecalculationTaskType
  scope?: RecalculationScope
  agentID?: number | null
  settlementBillID?: number | null
  periodStart?: string | null
  periodEnd?: string | null
  remark?: string
  status: RecalculationTaskStatus
  requestedBy?: string
  startedAt?: string | null
  completedAt?: string | null
  resultSummary?: unknown
  agentName?: string
  billNo?: string
  createdAt: string
  updatedAt: string
}

export interface LedgerEntry {
  id: number
  tenantID?: number | null
  brandID?: number | null
  accountID: number
  agentID: number
  referenceType: string
  referenceID: string
  ledgerType: LedgerType
  direction: LedgerDirection
  amount: number
  balanceBefore: number
  balanceAfter: number
  frozenBefore: number
  frozenAfter: number
  currency: string
  occurredAt: string
  remark?: string
  createdAt: string
  updatedAt: string
}

export interface WithdrawalRequest {
  id: number
  requestNo: string
  agentID: number
  agentName?: string
  accountID?: number | null
  amount: number
  feeAmount?: number
  taxAmount?: number
  netAmount?: number
  currency: string
  status: WithdrawalRequestStatus
  bankAccountName?: string
  bankAccountNo?: string
  bankName?: string
  submittedAt?: string | null
  approvedAt?: string | null
  approvedBy?: string
  paidAt?: string | null
  riskNote?: string
  remark?: string
  createdAt: string
  updatedAt: string
}

export interface ReviewRiskCasePayload {
  action: 'confirm' | 'release'
  remark?: string
}

export interface AuditLog {
  id: number
  operatorID: string
  operatorName?: string
  operatorRole?: string
  module: string
  action: string
  targetType: string
  targetID: string
  requestID?: string
  result: AuditResult
  errorMessage?: string
  occurredAt: string
  createdAt: string
  updatedAt: string
}

export interface RbacPermission {
  id: number
  code: string
  name: string
  createdAt: string
  updatedAt: string
}

export interface RbacRole {
  id: number
  tenantID?: number | null
  brandID?: number | null
  code: string
  name: string
  permissions: string[]
  createdAt: string
  updatedAt: string
}

export interface RbacUser {
  id: number
  username: string
  displayName?: string
  status: 'active' | 'disabled'
  tenantID?: number | null
  brandID?: number | null
  agentID?: number | null
  roles: string[]
  lastLoginAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface ApiListResponse<T> {
  items: T[]
  total: number
}

export interface ApiListEnvelope<T> {
  data?: ApiListResponse<T>
  items?: T[]
  total?: number
}

export interface LoginPayload {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  role?: string
  permissions?: string[]
  identity?: {
    username: string
    role?: string
    permissions?: string[]
  }
  user?: UserProfile
}

type QueryValue = string | number | boolean | undefined | null

interface RequestOptions extends RequestInit {
  query?: object
}

export interface ListAgentsParams extends PlatformScopedQueryParams {
  keyword?: string
  status?: AgentStatus
}

export interface ListInviteCodesParams extends PlatformScopedQueryParams {
  agentID?: number
  status?: InviteCodeStatus
  keyword?: string
}

export interface ListPlayersParams extends PlatformScopedQueryParams {
  keyword?: string
}

export interface ListBindingsParams extends PlatformScopedQueryParams {
  playerID?: number
}

export interface ListBindingHistoryParams extends PlatformScopedQueryParams {
  playerID?: number
}

export interface ListAgentInviteApplicationsParams extends PlatformScopedQueryParams {
  status?: AgentInviteApplicationStatus
}

export interface ListGamesParams extends PlatformScopedQueryParams {
  status?: GameStatus
  keyword?: string
}

export interface ListBrandsParams {
  tenantID?: number
}

export interface ListPlatformConfigsParams extends PlatformScopedQueryParams {
  tenantCode?: string
}

export interface PlatformConfigPayload {
  tenantID?: number
  brandID?: number
  key: string
  value: Record<string, unknown>
  description?: string
}

export interface PlatformScopedQueryParams {
  tenantID?: number
  brandID?: number
}

export interface ListAgentGameAccessParams extends PlatformScopedQueryParams {
  agentID?: number
  gameID?: number
  status?: AccessStatus
}

export interface ListRulesParams extends PlatformScopedQueryParams {
  status?: RuleStatus
  scope?: RuleScope
  agentID?: number
  gameID?: number
}

export interface ListActivityRewardRulesParams extends PlatformScopedQueryParams {
  status?: ActivityRewardRuleStatus
  keyword?: string
  activityType?: string
}

export interface CreateActivityRewardRulePayload extends PlatformScopedQueryParams {
  name: string
  activityType: string
  rewardType: string
  status: ActivityRewardRuleStatus
  rewardValue: number
  currency: string
  triggerValue?: number
  dailyLimit?: number
  totalLimit?: number
  startAt?: string
  endAt?: string
  remark?: string
}

export interface UpdateActivityRewardRuleStatusPayload {
  status: ActivityRewardRuleStatus
}

export interface ListActivityRewardRecordsParams extends PlatformScopedQueryParams {
  status?: string
  keyword?: string
  ruleID?: number
  userID?: number
  agentID?: number
}

export interface ListOrdersParams extends PlatformScopedQueryParams {
  status?: OrderStatus
  agentID?: number
  gameID?: number
  orderNo?: string
}

export interface ListCommissionsParams extends PlatformScopedQueryParams {
  status?: CommissionStatus
  agentID?: number
  gameID?: number
  playerID?: number
  orderNo?: string
}

export interface ListLedgerParams extends PlatformScopedQueryParams {
  agentID?: number
  type?: LedgerType
  orderNo?: string
}

export interface ListSettlementBillsParams extends PlatformScopedQueryParams {
  agentID?: number
  status?: SettlementBillStatus
  billNo?: string
}

export interface ListRecalculationTasksParams extends PlatformScopedQueryParams {
  status?: RecalculationTaskStatus
  taskNo?: string
}

export interface CreateRecalculationTaskPayload {
  taskType?: RecalculationTaskType
  scope?: RecalculationScope
  agentID?: number
  settlementBillID?: number
  periodStart?: string
  periodEnd?: string
  operator?: string
  reason?: string
  remark?: string
}

export type RiskLevel = 'low' | 'medium' | 'high'
export type RiskCaseStatus = 'pending' | 'released' | 'confirmed'

export interface RiskAgentAccount {
  id: number
  agentID: number
  balance: number
  frozenBalance: number
  withdrawableAmount: number
  currency: string
  riskLevel: RiskLevel
  frozenRatio: number
  agentName?: string
  status?: string
  createdAt: string
  updatedAt: string
}

export interface RiskCase {
  caseNo: string
  agentID: number
  agentName?: string
  currency: string
  riskLevel: RiskLevel
  status: RiskCaseStatus
  reason: string
  freezeRequested: boolean
  frozenBalance: number
  withdrawableAmount: number
  frozenRatio: number
  latestWithdrawalRequestID?: number
  latestWithdrawalRequestNo?: string
  latestWithdrawalAmount?: number
  riskNote?: string
  createdAt: string
  updatedAt: string
}

export interface AgentPerformanceReportItem {
  agentID: number
  agentName?: string
  currency: string
  balance: number
  frozenBalance: number
  withdrawableAmount: number
  totalEntries: number
  incomeAmount: number
  freezeAmount: number
  unfreezeAmount: number
  debitAmount: number
  reverseAmount: number
  adjustAmount: number
}

export interface GameSettlementReportItem {
  currency: string
  totalBills: number
  confirmedBills: number
  pendingBills: number
  generatedBills: number
  cancelledBills: number
  totalCommissionAmount: number
  totalAdjustmentAmount: number
  totalPayableAmount: number
}

export interface TeamPerformanceReportItem {
  agentID: number
  agentName?: string
  level: number
  currency: string
  directDescendants: number
  totalDescendants: number
  activeDescendants: number
  leafDescendants: number
  boundPlayers: number
  totalTeamBalance: number
  totalTeamFrozenBalance: number
  totalWithdrawableAmount: number
  pendingWithdrawals: number
  pendingWithdrawalAmount: number
  confirmedBills: number
  confirmedBillAmount: number
}

export interface SettlementProgressReportItem {
  currency: string
  totalBills: number
  generatedBills: number
  confirmedBills: number
  cancelledBills: number
  progressPercent: number
  pendingCommissionAmount: number
  pendingPayableAmount: number
  completedPayableAmount: number
  lastGeneratedAt?: string
  lastConfirmedAt?: string
}

export interface DataPlatformLayerItem {
  layer: string
  status: string
  syncMode: string
  description: string
  tableCount: number
  recordCount: number
  lastSyncedAt?: string
}

export interface DataPlatformMetricItem {
  key: string
  value: number
  unit?: string
  description?: string
}

export interface RiskIntelligenceItem {
  sourceType: string
  sourceID: string
  agentID?: number
  agentName?: string
  riskLevel: RiskLevel
  score: number
  reason: string
  recommendedAction: string
  intercepted: boolean
  status?: string
  createdAt: string
}

export interface ListAuditParams {
  module?: string
  action?: string
  targetID?: string
}

export interface ListRiskAgentAccountsParams {
  minFrozenAmount?: number
  riskLevel?: string
  agentID?: number
}

export interface ListRiskCasesParams extends PlatformScopedQueryParams {
  minFrozenRatio?: number
  riskLevel?: RiskLevel
  status?: RiskCaseStatus
  agentID?: number
}

export interface ListWithdrawalRequestsParams extends PlatformScopedQueryParams {
  requestNo?: string
  status?: WithdrawalRequestStatus
  agentID?: number
}

export interface AuditWithdrawalRequestPayload {
  action: WithdrawalReviewAction
  remark?: string
}

export interface ProcessWithdrawalPayoutPayload {
  action: WithdrawalPayoutAction
  remark?: string
  reference?: string
  receiptPayload?: Record<string, unknown>
}

export interface GenerateActivityRewardRecordPayload {
  playerID: number
  agentID?: number
  referenceType?: string
  referenceID: string
  remark?: string
}

export interface ReverseActivityRewardRecordPayload {
  remark?: string
}

export interface ListAgentPerformanceReportParams extends PlatformScopedQueryParams {
  agentID?: number
}

export interface ListGameSettlementReportParams extends PlatformScopedQueryParams {
  currency?: string
}

export interface ListTeamPerformanceReportParams extends PlatformScopedQueryParams {
  agentID?: number
  currency?: string
}

export interface ListSettlementProgressReportParams extends PlatformScopedQueryParams {
  currency?: string
}

export interface RbacRolePayload {
  tenantID?: number | null
  brandID?: number | null
  code: string
  name: string
  permissions: string[]
}

export interface RbacUserPayload {
  username?: string
  password?: string
  displayName?: string
  status?: 'active' | 'disabled'
  tenantID?: number | null
  brandID?: number | null
  agentID?: number | null
  roles: string[]
}

function toQueryString(query?: RequestOptions['query']) {
  const params = new URLSearchParams()
  Object.entries((query ?? {}) as Record<string, QueryValue>).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value))
    }
  })
  const qs = params.toString()
  return qs ? `?${qs}` : ''
}

export function getAccessToken() {
  return localStorage.getItem(AUTH_TOKEN_KEY)
}

export function setAccessToken(token: string) {
  localStorage.setItem(AUTH_TOKEN_KEY, token)
}

export function clearAccessToken() {
  localStorage.removeItem(AUTH_TOKEN_KEY)
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const token = getAccessToken()
  const response = await fetch(`${DEFAULT_BASE_URL}${path}${toQueryString(options.query)}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.headers ?? {}),
    },
    ...options,
  })

  if (response.status === 401) {
    clearAccessToken()
  }

  if (!response.ok) {
    const raw = await response.text()
    let message = raw
    if (raw) {
      try {
        const payload = JSON.parse(raw) as { message?: string; error?: string }
        message = payload.message || payload.error || raw
      } catch {
        message = raw
      }
    }
    throw new Error(message || `Request failed with status ${response.status}`)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

function normalizeListResponse<T>(payload: ApiListResponse<T> | ApiListEnvelope<T>): ApiListResponse<T> {
  if ('data' in payload && payload.data) {
    return {
      items: payload.data.items ?? [],
      total: payload.data.total ?? payload.data.items?.length ?? 0,
    }
  }

  return {
    items: payload.items ?? [],
    total: payload.total ?? payload.items?.length ?? 0,
  }
}

type UnknownRecord = Record<string, unknown>

function asRecord(value: unknown): UnknownRecord {
  return typeof value === 'object' && value !== null ? value as UnknownRecord : {}
}

function pickValue<T>(record: UnknownRecord, ...keys: string[]): T | undefined {
  for (const key of keys) {
    if (record[key] !== undefined) {
      return record[key] as T
    }
  }
  return undefined
}

function pickNumber(record: UnknownRecord, ...keys: string[]): number | undefined {
  const value = pickValue<unknown>(record, ...keys)
  if (typeof value === 'number') {
    return value
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  return undefined
}

function pickString(record: UnknownRecord, ...keys: string[]): string | undefined {
  const value = pickValue<unknown>(record, ...keys)
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number') {
    return String(value)
  }
  return undefined
}

function pickBoolean(record: UnknownRecord, ...keys: string[]): boolean | undefined {
  const value = pickValue<unknown>(record, ...keys)
  return typeof value === 'boolean' ? value : undefined
}

function pickStringArray(record: UnknownRecord, ...keys: string[]): string[] | undefined {
  const value = pickValue<unknown>(record, ...keys)
  if (!Array.isArray(value)) {
    return undefined
  }
  return value.map((item) => String(item))
}

function normalizePlatformConfig(payload: unknown): PlatformConfig {
  const record = asRecord(payload)
  const value = pickValue<unknown>(record, 'value', 'Value')
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    tenantCode: pickString(record, 'tenantCode', 'TenantCode'),
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    brandCode: pickString(record, 'brandCode', 'BrandCode'),
    key: pickString(record, 'key', 'Key') ?? '',
    value: typeof value === 'object' && value !== null ? value as Record<string, unknown> : {},
    description: pickString(record, 'description', 'Description') ?? '',
  }
}

function normalizeEnumDictionaryItem(payload: unknown): EnumDictionaryItem {
  const record = asRecord(payload)
  return {
    value: pickString(record, 'value', 'Value') ?? '',
    label: pickString(record, 'label', 'Label') ?? '',
    labelEn: pickString(record, 'labelEn', 'LabelEn'),
    description: pickString(record, 'description', 'Description'),
  }
}

function normalizeEnumDictionary(payload: unknown): EnumDictionary {
  const record = asRecord(payload)
  return {
    code: pickString(record, 'code', 'Code') ?? '',
    key: pickString(record, 'key', 'Key') ?? '',
    strict: pickBoolean(record, 'strict', 'Strict') ?? false,
    items: (pickValue<unknown[]>(record, 'items', 'Items') ?? []).map((item) => normalizeEnumDictionaryItem(item)),
  }
}

function normalizeAgent(payload: unknown): Agent {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    agentNo: pickString(record, 'agentNo', 'AgentNo') ?? '',
    name: pickString(record, 'name', 'Name') ?? '',
    displayName: pickString(record, 'displayName', 'DisplayName'),
    phone: pickString(record, 'phone', 'Phone'),
    email: pickString(record, 'email', 'Email'),
    status: (pickString(record, 'status', 'Status') ?? 'pending') as AgentStatus,
    level: pickNumber(record, 'level', 'Level') ?? 0,
    parentAgentID: pickNumber(record, 'parentAgentID', 'ParentAgentID') ?? null,
    countryCode: pickString(record, 'countryCode', 'CountryCode'),
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeTenant(payload: unknown): Tenant {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    code: pickString(record, 'code', 'Code') ?? '',
    name: pickString(record, 'name', 'Name') ?? '',
    displayName: pickString(record, 'displayName', 'DisplayName'),
    status: (pickString(record, 'status', 'Status') ?? 'active') as TenantStatus,
    defaultBrandID: pickNumber(record, 'defaultBrandID', 'DefaultBrandID') ?? null,
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeBrand(payload: unknown): Brand {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? 0,
    code: pickString(record, 'code', 'Code') ?? '',
    name: pickString(record, 'name', 'Name') ?? '',
    displayName: pickString(record, 'displayName', 'DisplayName'),
    status: (pickString(record, 'status', 'Status') ?? 'active') as BrandStatus,
    domain: pickString(record, 'domain', 'Domain'),
    isDefault: pickBoolean(record, 'isDefault', 'IsDefault') ?? false,
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeInviteCode(payload: unknown): InviteCode {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    code: pickString(record, 'code', 'Code') ?? '',
    status: (pickString(record, 'status', 'Status') ?? 'active') as InviteCodeStatus,
    isPrimary: pickBoolean(record, 'isPrimary', 'IsPrimary') ?? false,
    maxUseCount: pickNumber(record, 'maxUseCount', 'MaxUseCount') ?? 0,
    usedCount: pickNumber(record, 'usedCount', 'UsedCount') ?? 0,
    channel: pickString(record, 'channel', 'Channel'),
    remark: pickString(record, 'remark', 'Remark'),
    expiredAt: pickString(record, 'expiredAt', 'ExpiredAt') ?? null,
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizePlayer(payload: unknown): Player {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    playerNo: pickString(record, 'playerNo', 'PlayerNo') ?? '',
    platformUserID: pickString(record, 'platformUserID', 'PlatformUserID') ?? '',
    nickname: pickString(record, 'nickname', 'Nickname'),
    phone: pickString(record, 'phone', 'Phone'),
    countryCode: pickString(record, 'countryCode', 'CountryCode'),
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    status: pickString(record, 'status', 'Status') ?? '',
    registeredAt: pickString(record, 'registeredAt', 'RegisteredAt'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeBinding(payload: unknown): Binding {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    playerID: pickNumber(record, 'playerID', 'PlayerID') ?? 0,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    inviteCodeID: pickNumber(record, 'inviteCodeID', 'InviteCodeID') ?? null,
    status: (pickString(record, 'status', 'Status') ?? 'bound') as BindingStatus,
    source: (pickString(record, 'source', 'Source') ?? 'manual') as BindingSource,
    boundAt: pickString(record, 'boundAt', 'BoundAt') ?? '',
    effectiveFrom: pickString(record, 'effectiveFrom', 'EffectiveFrom') ?? '',
    effectiveTo: pickString(record, 'effectiveTo', 'EffectiveTo') ?? null,
    approvedBy: pickString(record, 'approvedBy', 'ApprovedBy'),
    approvalReason: pickString(record, 'approvalReason', 'ApprovalReason'),
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeBindingHistory(payload: unknown): BindingHistory {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    bindingID: pickNumber(record, 'bindingID', 'BindingID') ?? 0,
    playerID: pickNumber(record, 'playerID', 'PlayerID') ?? 0,
    fromAgentID: pickNumber(record, 'fromAgentID', 'FromAgentID') ?? null,
    toAgentID: pickNumber(record, 'toAgentID', 'ToAgentID') ?? 0,
    inviteCodeID: pickNumber(record, 'inviteCodeID', 'InviteCodeID') ?? null,
    status: (pickString(record, 'status', 'Status') ?? 'bound') as BindingStatus,
    source: (pickString(record, 'source', 'Source') ?? 'manual') as BindingSource,
    changedAt: pickString(record, 'changedAt', 'ChangedAt') ?? '',
    changedBy: pickString(record, 'changedBy', 'ChangedBy'),
    changeReason: pickString(record, 'changeReason', 'ChangeReason'),
    snapshotPayload: pickValue<unknown>(record, 'snapshotPayload', 'SnapshotPayload'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeAgentInviteApplication(payload: unknown): AgentInviteApplication {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    applicantAgentID: pickNumber(record, 'applicantAgentID', 'ApplicantAgentID') ?? 0,
    inviterAgentID: pickNumber(record, 'inviterAgentID', 'InviterAgentID') ?? 0,
    inviteCodeID: pickNumber(record, 'inviteCodeID', 'InviteCodeID') ?? null,
    status: (pickString(record, 'status', 'Status') ?? 'pending') as AgentInviteApplicationStatus,
    applyRemark: pickString(record, 'applyRemark', 'ApplyRemark'),
    auditRemark: pickString(record, 'auditRemark', 'AuditRemark'),
    auditBy: pickString(record, 'auditBy', 'AuditBy'),
    auditedAt: pickString(record, 'auditedAt', 'AuditedAt') ?? null,
    approvedRelationID: pickNumber(record, 'approvedRelationID', 'ApprovedRelationID') ?? null,
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeGame(payload: unknown): Game {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    gameCode: pickString(record, 'gameCode', 'GameCode') ?? '',
    name: pickString(record, 'name', 'Name') ?? '',
    vendor: pickString(record, 'vendor', 'Vendor'),
    category: pickString(record, 'category', 'Category'),
    status: (pickString(record, 'status', 'Status') ?? 'draft') as GameStatus,
    isAgentable: pickBoolean(record, 'isAgentable', 'IsAgentable') ?? false,
    sort: pickNumber(record, 'sort', 'Sort') ?? 0,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    launchAt: pickString(record, 'launchAt', 'LaunchAt') ?? null,
    offlineAt: pickString(record, 'offlineAt', 'OfflineAt') ?? null,
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeGameIntegrationCredential(payload: unknown): GameIntegrationCredential {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    gameID: pickNumber(record, 'gameID', 'GameID') ?? 0,
    name: pickString(record, 'name', 'Name') ?? '',
    accessKey: pickString(record, 'accessKey', 'AccessKey') ?? '',
    secretKey: pickString(record, 'secretKey', 'SecretKey'),
    status: (pickString(record, 'status', 'Status') ?? 'active') as GameIntegrationCredential['status'],
    scopes: pickStringArray(record, 'scopes', 'Scopes') ?? [],
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    lastUsedAt: pickString(record, 'lastUsedAt', 'LastUsedAt') ?? null,
    rotatedAt: pickString(record, 'rotatedAt', 'RotatedAt') ?? null,
    expiresAt: pickString(record, 'expiresAt', 'ExpiresAt') ?? null,
    remark: pickString(record, 'remark', 'Remark'),
  }
}

function normalizeGameCreateResult(payload: unknown): GameCreateResult {
  const record = asRecord(payload)
  const gameValue = pickValue<unknown>(record, 'game', 'Game')
  const credentialValue = pickValue<unknown>(record, 'integrationCredential', 'IntegrationCredential')
  if (gameValue) {
    return {
      game: normalizeGame(gameValue),
      integrationCredential: credentialValue ? normalizeGameIntegrationCredential(credentialValue) : undefined,
    }
  }
  return { game: normalizeGame(payload) }
}

function normalizeAgentGameAccess(payload: unknown): AgentGameAccess {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    gameID: pickNumber(record, 'gameID', 'GameID') ?? 0,
    status: (pickString(record, 'status', 'Status') ?? 'enabled') as AccessStatus,
    grantedBy: pickString(record, 'grantedBy', 'GrantedBy'),
    grantedAt: pickString(record, 'grantedAt', 'GrantedAt') ?? '',
    effectiveFrom: pickString(record, 'effectiveFrom', 'EffectiveFrom') ?? '',
    effectiveTo: pickString(record, 'effectiveTo', 'EffectiveTo') ?? null,
    settlementMemo: pickString(record, 'settlementMemo', 'SettlementMemo'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeAgentGameAccessListItem(payload: unknown): AgentGameAccessListItem {
  const base = normalizeAgentGameAccess(payload)
  const record = asRecord(payload)
  return {
    ...base,
    agentName: pickString(record, 'agentName', 'AgentName') ?? '',
    gameCode: pickString(record, 'gameCode', 'GameCode') ?? '',
    gameName: pickString(record, 'gameName', 'GameName') ?? '',
  }
}

function normalizeRule(payload: unknown): Rule {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    ruleName: pickString(record, 'ruleName', 'RuleName') ?? '',
    scope: (pickString(record, 'scope', 'Scope') ?? 'platform') as RuleScope,
    ruleType: (pickString(record, 'ruleType', 'RuleType') ?? 'ratio') as RuleType,
    status: (pickString(record, 'status', 'Status') ?? 'draft') as RuleStatus,
    priority: pickNumber(record, 'priority', 'Priority') ?? 0,
    version: pickNumber(record, 'version', 'Version') ?? 1,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? null,
    gameID: pickNumber(record, 'gameID', 'GameID') ?? null,
    maxSettlementDepth: pickNumber(record, 'maxSettlementDepth', 'MaxSettlementDepth') ?? 0,
    commissionRate: pickNumber(record, 'commissionRate', 'CommissionRate') ?? 0,
    fixedAmount: pickNumber(record, 'fixedAmount', 'FixedAmount') ?? 0,
    minAgentLevel: pickNumber(record, 'minAgentLevel', 'MinAgentLevel'),
    rechargeTypes: pickStringArray(record, 'rechargeTypes', 'RechargeTypes'),
    activityTags: pickStringArray(record, 'activityTags', 'ActivityTags'),
    capAmount: pickNumber(record, 'capAmount', 'CapAmount'),
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    effectiveFrom: pickString(record, 'effectiveFrom', 'EffectiveFrom') ?? '',
    effectiveTo: pickString(record, 'effectiveTo', 'EffectiveTo') ?? null,
    publishedAt: pickString(record, 'publishedAt', 'PublishedAt') ?? null,
    publishedBy: pickString(record, 'publishedBy', 'PublishedBy'),
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeActivityRewardRule(payload: unknown): ActivityRewardRule {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    name: pickString(record, 'name', 'Name') ?? '',
    activityType: pickString(record, 'activityType', 'ActivityType') ?? '',
    rewardType: pickString(record, 'rewardType', 'RewardType') ?? '',
    status: (pickString(record, 'status', 'Status') ?? 'draft') as ActivityRewardRuleStatus,
    rewardValue: pickNumber(record, 'rewardValue', 'RewardValue') ?? 0,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    triggerValue: pickNumber(record, 'triggerValue', 'TriggerValue') ?? null,
    dailyLimit: pickNumber(record, 'dailyLimit', 'DailyLimit') ?? null,
    totalLimit: pickNumber(record, 'totalLimit', 'TotalLimit') ?? null,
    startAt: pickString(record, 'startAt', 'StartAt') ?? null,
    endAt: pickString(record, 'endAt', 'EndAt') ?? null,
    createdBy: pickString(record, 'createdBy', 'CreatedBy'),
    remark: pickString(record, 'remark', 'Remark'),
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeActivityRewardRecord(payload: unknown): ActivityRewardRecord {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    recordNo: pickString(record, 'recordNo', 'RecordNo') ?? '',
    ruleID: pickNumber(record, 'ruleID', 'RuleID') ?? null,
    ruleName: pickString(record, 'ruleName', 'RuleName'),
    userID: pickNumber(record, 'userID', 'UserID') ?? null,
    playerID: pickNumber(record, 'playerID', 'PlayerID') ?? null,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? null,
    rewardType: pickString(record, 'rewardType', 'RewardType') ?? '',
    rewardValue: pickNumber(record, 'rewardValue', 'RewardValue') ?? 0,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    activityType: pickString(record, 'activityType', 'ActivityType'),
    status: (pickString(record, 'status', 'Status') ?? 'pending') as ActivityRewardRecordStatus,
    grantedAt: pickString(record, 'grantedAt', 'GrantedAt') ?? null,
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
    remark: pickString(record, 'remark', 'Remark'),
  }
}

function normalizeWithdrawalRequest(payload: unknown): WithdrawalRequest {
  const record = asRecord(payload)
  const payableAmount = pickNumber(record, 'payableAmount', 'PayableAmount')
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    requestNo: pickString(record, 'requestNo', 'RequestNo') ?? '',
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    agentName: pickString(record, 'agentName', 'AgentName'),
    accountID: pickNumber(record, 'accountID', 'AccountID') ?? null,
    amount: pickNumber(record, 'amount', 'Amount') ?? 0,
    feeAmount: pickNumber(record, 'feeAmount', 'FeeAmount'),
    taxAmount: pickNumber(record, 'taxAmount', 'TaxAmount'),
    netAmount: pickNumber(record, 'netAmount', 'NetAmount') ?? payableAmount,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    status: (pickString(record, 'status', 'Status') ?? 'pending') as WithdrawalRequestStatus,
    bankAccountName: pickString(record, 'bankAccountName', 'BankAccountName'),
    bankAccountNo: pickString(record, 'bankAccountNo', 'BankAccountNo'),
    bankName: pickString(record, 'bankName', 'BankName'),
    submittedAt: pickString(record, 'submittedAt', 'SubmittedAt') ?? null,
    approvedAt: pickString(record, 'approvedAt', 'ReviewedAt') ?? null,
    approvedBy: pickString(record, 'approvedBy', 'ReviewedBy'),
    paidAt: pickString(record, 'paidAt', 'PaidAt') ?? null,
    riskNote: pickString(record, 'riskNote', 'RiskNote'),
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeRiskCase(payload: unknown): RiskCase {
  const record = asRecord(payload)
  return {
    caseNo: pickString(record, 'caseNo', 'CaseNo') ?? '',
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    agentName: pickString(record, 'agentName', 'AgentName'),
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    riskLevel: (pickString(record, 'riskLevel', 'RiskLevel') ?? 'low') as RiskLevel,
    status: (pickString(record, 'status', 'Status') ?? 'pending') as RiskCaseStatus,
    reason: pickString(record, 'reason', 'Reason') ?? '',
    freezeRequested: pickBoolean(record, 'freezeRequested', 'FreezeRequested') ?? false,
    frozenBalance: pickNumber(record, 'frozenBalance', 'FrozenBalance') ?? 0,
    withdrawableAmount: pickNumber(record, 'withdrawableAmount', 'WithdrawableAmount') ?? 0,
    frozenRatio: pickNumber(record, 'frozenRatio', 'FrozenRatio') ?? 0,
    latestWithdrawalRequestID: pickNumber(record, 'latestWithdrawalRequestID', 'LatestWithdrawalRequestID'),
    latestWithdrawalRequestNo: pickString(record, 'latestWithdrawalRequestNo', 'LatestWithdrawalRequestNo'),
    latestWithdrawalAmount: pickNumber(record, 'latestWithdrawalAmount', 'LatestWithdrawalAmount'),
    riskNote: pickString(record, 'riskNote', 'RiskNote'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeOrder(payload: unknown): Order {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    orderNo: pickString(record, 'orderNo', 'OrderNo') ?? '',
    externalOrderNo: pickString(record, 'externalOrderNo', 'ExternalOrderNo'),
    playerID: pickNumber(record, 'playerID', 'PlayerID') ?? 0,
    gameID: pickNumber(record, 'gameID', 'GameID') ?? 0,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? null,
    amount: pickNumber(record, 'amount', 'Amount') ?? 0,
    paidAmount: pickNumber(record, 'paidAmount', 'PaidAmount') ?? null,
    paymentChannelCost: pickNumber(record, 'paymentChannelCost', 'PaymentChannelCost') ?? null,
    grossProfitAmount: pickNumber(record, 'grossProfitAmount', 'GrossProfitAmount') ?? null,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    status: (pickString(record, 'status', 'Status') ?? 'pending') as OrderStatus,
    channel: pickString(record, 'channel', 'Channel'),
    paidAt: pickString(record, 'paidAt', 'PaidAt') ?? null,
    callbackAt: pickString(record, 'callbackAt', 'CallbackAt') ?? null,
    riskNote: pickString(record, 'riskNote', 'RiskNote'),
    freezeReason: pickString(record, 'freezeReason', 'FreezeReason'),
    reviewStatus: pickString(record, 'reviewStatus', 'ReviewStatus'),
    freezeVisible: pickBoolean(record, 'freezeVisible', 'FreezeVisible'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeCommissionRecord(payload: unknown): CommissionRecord {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    recordNo: pickString(record, 'recordNo', 'RecordNo') ?? '',
    rechargeOrderID: pickNumber(record, 'rechargeOrderID', 'RechargeOrderID') ?? 0,
    playerID: pickNumber(record, 'playerID', 'PlayerID') ?? 0,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    gameID: pickNumber(record, 'gameID', 'GameID') ?? 0,
    ruleID: pickNumber(record, 'ruleID', 'RuleID') ?? null,
    settlementDepth: pickNumber(record, 'settlementDepth', 'SettlementDepth') ?? 0,
    commissionBaseAmount: pickNumber(record, 'commissionBaseAmount', 'CommissionBaseAmount') ?? 0,
    commissionRate: pickNumber(record, 'commissionRate', 'CommissionRate') ?? 0,
    commissionAmount: pickNumber(record, 'commissionAmount', 'CommissionAmount') ?? 0,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    status: (pickString(record, 'status', 'Status') ?? 'pending') as CommissionStatus,
    estimatedAt: pickString(record, 'estimatedAt', 'EstimatedAt') ?? '',
    settledAt: pickString(record, 'settledAt', 'SettledAt') ?? null,
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeSettlementBillDetail(payload: unknown): NonNullable<SettlementBill['details']>[number] {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    settlementBillID: pickNumber(record, 'settlementBillID', 'SettlementBillID') ?? 0,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    commissionRecordID: pickNumber(record, 'commissionRecordID', 'CommissionRecordID') ?? null,
    rechargeOrderID: pickNumber(record, 'rechargeOrderID', 'RechargeOrderID') ?? null,
    referenceType: pickString(record, 'referenceType', 'ReferenceType') ?? '',
    referenceID: pickString(record, 'referenceID', 'ReferenceID') ?? '',
    commissionAmount: pickNumber(record, 'commissionAmount', 'CommissionAmount') ?? 0,
    adjustmentAmount: pickNumber(record, 'adjustmentAmount', 'AdjustmentAmount') ?? 0,
    amount: pickNumber(record, 'amount', 'Amount') ?? 0,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    occurredAt: pickString(record, 'occurredAt', 'OccurredAt') ?? '',
    recordNo: pickString(record, 'recordNo', 'RecordNo'),
    orderNo: pickString(record, 'orderNo', 'OrderNo'),
    remark: pickString(record, 'remark', 'Remark'),
  }
}

function normalizeSettlementBill(payload: unknown): SettlementBill {
  const record = asRecord(payload)
  const detailsValue = pickValue<unknown>(record, 'details', 'Details')
  const summaryPayload = pickValue<unknown>(record, 'summaryPayload', 'SummaryPayload')
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    billNo: pickString(record, 'billNo', 'BillNo') ?? '',
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    agentName: pickString(record, 'agentName', 'AgentName'),
    periodType: pickString(record, 'periodType', 'PeriodType'),
    periodStart: pickString(record, 'periodStart', 'PeriodStart') ?? '',
    periodEnd: pickString(record, 'periodEnd', 'PeriodEnd') ?? '',
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    commissionAmount: pickNumber(record, 'commissionAmount', 'CommissionAmount') ?? 0,
    adjustmentAmount: pickNumber(record, 'adjustmentAmount', 'AdjustmentAmount') ?? 0,
    payableAmount: pickNumber(record, 'payableAmount', 'PayableAmount') ?? 0,
    status: (pickString(record, 'status', 'Status') ?? 'pending') as SettlementBillStatus,
    generatedAt: pickString(record, 'generatedAt', 'GeneratedAt') ?? null,
    confirmedAt: pickString(record, 'confirmedAt', 'ConfirmedAt') ?? null,
    confirmedBy: pickString(record, 'confirmedBy', 'ConfirmedBy'),
    freezeVisible: pickBoolean(record, 'freezeVisible', 'FreezeVisible'),
    summaryPayload: typeof summaryPayload === 'object' && summaryPayload !== null ? summaryPayload as SettlementBill['summaryPayload'] : undefined,
    remark: pickString(record, 'remark', 'Remark'),
    details: Array.isArray(detailsValue) ? detailsValue.map((item) => normalizeSettlementBillDetail(item)) : undefined,
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeRecalculationTask(payload: unknown): RecalculationTask {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    taskNo: pickString(record, 'taskNo', 'TaskNo') ?? '',
    taskType: (pickString(record, 'taskType', 'TaskType') ?? 'settlement_bill') as RecalculationTaskType,
    scope: (pickString(record, 'scope', 'Scope') ?? 'all') as RecalculationScope,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? null,
    settlementBillID: pickNumber(record, 'settlementBillID', 'SettlementBillID') ?? null,
    periodStart: pickString(record, 'periodStart', 'PeriodStart') ?? null,
    periodEnd: pickString(record, 'periodEnd', 'PeriodEnd') ?? null,
    remark: pickString(record, 'remark', 'Remark'),
    status: (pickString(record, 'status', 'Status') ?? 'pending') as RecalculationTaskStatus,
    requestedBy: pickString(record, 'requestedBy', 'RequestedBy'),
    startedAt: pickString(record, 'startedAt', 'StartedAt') ?? null,
    completedAt: pickString(record, 'completedAt', 'CompletedAt') ?? null,
    resultSummary: pickValue<unknown>(record, 'resultSummary', 'ResultSummary'),
    agentName: pickString(record, 'agentName', 'AgentName'),
    billNo: pickString(record, 'billNo', 'BillNo'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeLedgerEntry(payload: unknown): LedgerEntry {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    accountID: pickNumber(record, 'accountID', 'AccountID') ?? 0,
    agentID: pickNumber(record, 'agentID', 'AgentID') ?? 0,
    referenceType: pickString(record, 'referenceType', 'ReferenceType') ?? '',
    referenceID: pickString(record, 'referenceID', 'ReferenceID') ?? '',
    ledgerType: (pickString(record, 'ledgerType', 'LedgerType') ?? 'income') as LedgerType,
    direction: (pickString(record, 'direction', 'Direction') ?? 'credit') as LedgerDirection,
    amount: pickNumber(record, 'amount', 'Amount') ?? 0,
    balanceBefore: pickNumber(record, 'balanceBefore', 'BalanceBefore') ?? 0,
    balanceAfter: pickNumber(record, 'balanceAfter', 'BalanceAfter') ?? 0,
    frozenBefore: pickNumber(record, 'frozenBefore', 'FrozenBefore') ?? 0,
    frozenAfter: pickNumber(record, 'frozenAfter', 'FrozenAfter') ?? 0,
    currency: pickString(record, 'currency', 'Currency') ?? 'CNY',
    occurredAt: pickString(record, 'occurredAt', 'OccurredAt') ?? '',
    remark: pickString(record, 'remark', 'Remark'),
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeAuditLog(payload: unknown): AuditLog {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    operatorID: pickString(record, 'operatorID', 'OperatorID') ?? '',
    operatorName: pickString(record, 'operatorName', 'OperatorName'),
    operatorRole: pickString(record, 'operatorRole', 'OperatorRole'),
    module: pickString(record, 'module', 'Module') ?? '',
    action: pickString(record, 'action', 'Action') ?? '',
    targetType: pickString(record, 'targetType', 'TargetType') ?? '',
    targetID: pickString(record, 'targetID', 'TargetID') ?? '',
    requestID: pickString(record, 'requestID', 'RequestID'),
    result: (pickString(record, 'result', 'Result') ?? 'success') as AuditResult,
    errorMessage: pickString(record, 'errorMessage', 'ErrorMessage'),
    occurredAt: pickString(record, 'occurredAt', 'OccurredAt') ?? '',
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeRbacPermission(payload: unknown): RbacPermission {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    code: pickString(record, 'code', 'Code') ?? '',
    name: pickString(record, 'name', 'Name') ?? '',
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeRbacRole(payload: unknown): RbacRole {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    code: pickString(record, 'code', 'Code') ?? '',
    name: pickString(record, 'name', 'Name') ?? '',
    permissions: pickStringArray(record, 'permissions', 'Permissions') ?? [],
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeRbacUser(payload: unknown): RbacUser {
  const record = asRecord(payload)
  return {
    id: pickNumber(record, 'id', 'ID') ?? 0,
    username: pickString(record, 'username', 'Username') ?? '',
    displayName: pickString(record, 'displayName', 'DisplayName') ?? '',
    status: (pickString(record, 'status', 'Status') as RbacUser['status']) ?? 'active',
    tenantID: pickNumber(record, 'tenantID', 'TenantID') ?? null,
    brandID: pickNumber(record, 'brandID', 'BrandID') ?? null,
    agentID: pickNumber(record, 'agentID', 'AgentID'),
    roles: pickStringArray(record, 'roles', 'Roles') ?? [],
    lastLoginAt: pickString(record, 'lastLoginAt', 'LastLoginAt') ?? null,
    createdAt: pickString(record, 'createdAt', 'CreatedAt') ?? '',
    updatedAt: pickString(record, 'updatedAt', 'UpdatedAt') ?? '',
  }
}

function normalizeAgentList(payload: ApiListResponse<Agent> | ApiListEnvelope<Agent>): ApiListResponse<Agent> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeAgent(item)), total: response.total }
}

function normalizeTenantList(payload: ApiListResponse<Tenant> | ApiListEnvelope<Tenant>): ApiListResponse<Tenant> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeTenant(item)), total: response.total }
}

function normalizeBrandList(payload: ApiListResponse<Brand> | ApiListEnvelope<Brand>): ApiListResponse<Brand> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeBrand(item)), total: response.total }
}

function normalizePlatformConfigList(payload: ApiListResponse<PlatformConfig> | ApiListEnvelope<PlatformConfig>): ApiListResponse<PlatformConfig> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizePlatformConfig(item)), total: response.total }
}

function normalizeEnumDictionaryList(payload: ApiListResponse<EnumDictionary> | ApiListEnvelope<EnumDictionary>): ApiListResponse<EnumDictionary> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeEnumDictionary(item)), total: response.total }
}

function normalizeInviteCodeList(payload: ApiListResponse<InviteCode> | ApiListEnvelope<InviteCode>): ApiListResponse<InviteCode> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeInviteCode(item)), total: response.total }
}

function normalizePlayerList(payload: ApiListResponse<Player> | ApiListEnvelope<Player>): ApiListResponse<Player> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizePlayer(item)), total: response.total }
}

function normalizeBindingList(payload: ApiListResponse<Binding> | ApiListEnvelope<Binding>): ApiListResponse<Binding> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeBinding(item)), total: response.total }
}

function normalizeBindingHistoryList(payload: ApiListResponse<BindingHistory> | ApiListEnvelope<BindingHistory>): ApiListResponse<BindingHistory> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeBindingHistory(item)), total: response.total }
}

function normalizeAgentInviteApplicationList(payload: ApiListResponse<AgentInviteApplication> | ApiListEnvelope<AgentInviteApplication>): ApiListResponse<AgentInviteApplication> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeAgentInviteApplication(item)), total: response.total }
}

function normalizeGameList(payload: ApiListResponse<Game> | ApiListEnvelope<Game>): ApiListResponse<Game> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeGame(item)), total: response.total }
}

function normalizeGameIntegrationCredentialList(payload: ApiListResponse<GameIntegrationCredential> | ApiListEnvelope<GameIntegrationCredential>): ApiListResponse<GameIntegrationCredential> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeGameIntegrationCredential(item)), total: response.total }
}

function normalizeAgentGameAccessList(payload: ApiListResponse<AgentGameAccessListItem> | ApiListEnvelope<AgentGameAccessListItem>): ApiListResponse<AgentGameAccessListItem> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeAgentGameAccessListItem(item)), total: response.total }
}

function normalizeRuleList(payload: ApiListResponse<Rule> | ApiListEnvelope<Rule>): ApiListResponse<Rule> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeRule(item)), total: response.total }
}

function normalizeActivityRewardRuleList(payload: ApiListResponse<ActivityRewardRule> | ApiListEnvelope<ActivityRewardRule>): ApiListResponse<ActivityRewardRule> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeActivityRewardRule(item)), total: response.total }
}

function normalizeActivityRewardRecordList(payload: ApiListResponse<ActivityRewardRecord> | ApiListEnvelope<ActivityRewardRecord>): ApiListResponse<ActivityRewardRecord> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeActivityRewardRecord(item)), total: response.total }
}

function normalizeWithdrawalRequestList(payload: ApiListResponse<WithdrawalRequest> | ApiListEnvelope<WithdrawalRequest>): ApiListResponse<WithdrawalRequest> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeWithdrawalRequest(item)), total: response.total }
}

function normalizeRiskCaseList(payload: ApiListResponse<RiskCase> | ApiListEnvelope<RiskCase>): ApiListResponse<RiskCase> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeRiskCase(item)), total: response.total }
}

function normalizeOrderList(payload: ApiListResponse<Order> | ApiListEnvelope<Order>): ApiListResponse<Order> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeOrder(item)), total: response.total }
}

function normalizeCommissionRecordList(payload: ApiListResponse<CommissionRecord> | ApiListEnvelope<CommissionRecord>): ApiListResponse<CommissionRecord> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeCommissionRecord(item)), total: response.total }
}

function normalizeSettlementBillList(payload: ApiListResponse<SettlementBill> | ApiListEnvelope<SettlementBill>): ApiListResponse<SettlementBill> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeSettlementBill(item)), total: response.total }
}

function normalizeRecalculationTaskList(payload: ApiListResponse<RecalculationTask> | ApiListEnvelope<RecalculationTask>): ApiListResponse<RecalculationTask> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeRecalculationTask(item)), total: response.total }
}

function normalizeLedgerEntryList(payload: ApiListResponse<LedgerEntry> | ApiListEnvelope<LedgerEntry>): ApiListResponse<LedgerEntry> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeLedgerEntry(item)), total: response.total }
}

function normalizeAuditLogList(payload: ApiListResponse<AuditLog> | ApiListEnvelope<AuditLog>): ApiListResponse<AuditLog> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeAuditLog(item)), total: response.total }
}

function normalizeRbacPermissionList(payload: ApiListResponse<RbacPermission> | ApiListEnvelope<RbacPermission>): ApiListResponse<RbacPermission> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeRbacPermission(item)), total: response.total }
}

function normalizeRbacRoleList(payload: ApiListResponse<RbacRole> | ApiListEnvelope<RbacRole>): ApiListResponse<RbacRole> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeRbacRole(item)), total: response.total }
}

function normalizeRbacUserList(payload: ApiListResponse<RbacUser> | ApiListEnvelope<RbacUser>): ApiListResponse<RbacUser> {
  const response = normalizeListResponse(payload)
  return { items: response.items.map((item) => normalizeRbacUser(item)), total: response.total }
}

function mapBackendPermission(code: string): PermissionCode[] {
  return frontendPermissionsForBackendCode(code)
}

function normalizePermissions(codes: string[] | undefined): PermissionCode[] {
  return Array.from(new Set((codes ?? []).flatMap((code) => mapBackendPermission(code))))
}

function buildLoginUser(response: LoginResponse): UserProfile {
  const source = response.user ?? response.identity
  const backendPermissions = response.user?.permissions?.map(String) ?? response.identity?.permissions ?? response.permissions ?? []
  const responseUser = response.user ? asRecord(response.user) : undefined
  const scopeRecord = responseUser ? asRecord(pickValue(responseUser, 'scope', 'Scope')) : {}

  return {
    id: response.user?.id,
    username: source?.username ?? 'admin',
    displayName: response.user?.displayName,
    roles: response.user?.roles ?? (response.role ? [response.role] : response.identity?.role ? [response.identity.role] : []),
    permissions: normalizePermissions(backendPermissions),
    scope: {
      tenantID: pickNumber(scopeRecord, 'tenantID', 'TenantID') ?? null,
      tenantName: pickString(scopeRecord, 'tenantName', 'TenantName'),
      tenantCode: pickString(scopeRecord, 'tenantCode', 'TenantCode'),
      brandID: pickNumber(scopeRecord, 'brandID', 'BrandID') ?? null,
      brandName: pickString(scopeRecord, 'brandName', 'BrandName'),
      brandCode: pickString(scopeRecord, 'brandCode', 'BrandCode'),
      agentID: pickNumber(scopeRecord, 'agentID', 'AgentID') ?? null,
    },
  }
}

export async function getJson<T>(path: string): Promise<T> {
  return request<T>(path)
}

export const apiClient = {
  login: async (payload: LoginPayload) => {
    const response = await request<LoginResponse>('/auth/login', { method: 'POST', body: JSON.stringify(payload) })
    return { ...response, user: buildLoginUser(response) }
  },
  getCurrentUser: async () => {
    const response = await request<LoginResponse>('/auth/me')
    return buildLoginUser(response)
  },

  listAgents: async (params?: ListAgentsParams) =>
    normalizeAgentList(await request<ApiListResponse<Agent> | ApiListEnvelope<Agent>>('/agents', { query: params })),
  createAgent: (payload: Partial<Agent>) =>
    request<Agent>('/agents', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeAgent),
  updateAgentStatus: (id: number, status: AgentStatus) =>
    request<Agent>(`/agents/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }).then(normalizeAgent),

  listTenants: async () =>
    normalizeTenantList(await request<ApiListResponse<Tenant> | ApiListEnvelope<Tenant>>('/tenants')),
  createTenant: (payload: Partial<Tenant>) =>
    request<Tenant>('/tenants', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeTenant),
  updateTenantStatus: (id: number, status: TenantStatus) =>
    request<Tenant>(`/tenants/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }).then(normalizeTenant),
  deleteTenant: (id: number) => request<void>(`/tenants/${id}`, { method: 'DELETE' }),

  listBrands: async (params?: ListBrandsParams) =>
    normalizeBrandList(await request<ApiListResponse<Brand> | ApiListEnvelope<Brand>>('/brands', { query: params })),
  createBrand: (payload: Partial<Brand>) =>
    request<Brand>('/brands', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeBrand),
  updateBrandStatus: (id: number, status: BrandStatus) =>
    request<Brand>(`/brands/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }).then(normalizeBrand),
  deleteBrand: (id: number) => request<void>(`/brands/${id}`, { method: 'DELETE' }),

  listPlatformConfigs: async (params?: ListPlatformConfigsParams) =>
    normalizePlatformConfigList(await request<ApiListResponse<PlatformConfig> | ApiListEnvelope<PlatformConfig>>('/platform-configs', { query: params })),
  createPlatformConfig: (payload: PlatformConfigPayload) =>
    request<PlatformConfig>('/platform-configs', { method: 'POST', body: JSON.stringify(payload) }).then(normalizePlatformConfig),
  updatePlatformConfig: (id: number, payload: PlatformConfigPayload) =>
    request<PlatformConfig>(`/platform-configs/${id}`, { method: 'PUT', body: JSON.stringify(payload) }).then(normalizePlatformConfig),
  listEnumDictionaries: async (codes?: string[]) =>
    normalizeEnumDictionaryList(await request<ApiListResponse<EnumDictionary> | ApiListEnvelope<EnumDictionary>>('/enum-dictionaries', {
      query: codes?.length ? { codes: codes.join(',') } : undefined,
    })),

  listInviteCodes: async (params?: ListInviteCodesParams) =>
    normalizeInviteCodeList(await request<ApiListResponse<InviteCode> | ApiListEnvelope<InviteCode>>('/invite-codes', { query: params })),
  createInviteCode: (payload: Partial<InviteCode>) =>
    request<InviteCode>('/invite-codes', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeInviteCode),

  listPlayers: async (params?: ListPlayersParams) =>
    normalizePlayerList(await request<ApiListResponse<Player> | ApiListEnvelope<Player>>('/players', { query: params })),
  listBindings: async (params?: ListBindingsParams) =>
    normalizeBindingList(await request<ApiListResponse<Binding> | ApiListEnvelope<Binding>>('/bindings', { query: params })),
  createBinding: (payload: { playerID: number; inviteCode: string; remark?: string }) =>
    request<Binding>('/bindings', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeBinding),
  listBindingHistory: async (params?: ListBindingHistoryParams) =>
    normalizeBindingHistoryList(await request<ApiListResponse<BindingHistory> | ApiListEnvelope<BindingHistory>>('/binding-history', { query: params })),

  listAgentInviteApplications: async (params?: ListAgentInviteApplicationsParams) =>
    normalizeAgentInviteApplicationList(await request<ApiListResponse<AgentInviteApplication> | ApiListEnvelope<AgentInviteApplication>>('/agent-invite-applications', { query: params })),
  createAgentInviteApplication: (payload: { applicantAgentID: number; inviteCode: string; applyRemark?: string }) =>
    request<AgentInviteApplication>('/agent-invite-applications', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeAgentInviteApplication),
  auditAgentInviteApplication: (id: number, payload: { status: AgentInviteApplicationStatus; auditRemark?: string }) =>
    request<AgentInviteApplication>(`/agent-invite-applications/${id}/audit`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeAgentInviteApplication),

  listGames: async (params?: ListGamesParams) =>
    normalizeGameList(await request<ApiListResponse<Game> | ApiListEnvelope<Game>>('/games', { query: params })),
  createGame: (payload: Partial<Game>) => request<GameCreateResult>('/games', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeGameCreateResult),
  updateGame: (id: number, payload: Partial<Game>) => request<Game>(`/games/${id}`, { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeGame),
  updateGameStatus: (id: number, status: GameStatus) =>
    request<Game>(`/games/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }).then(normalizeGame),
  deleteGame: (id: number) => request<void>(`/games/${id}`, { method: 'DELETE' }),
  listGameIntegrationKeys: async (gameID: number) =>
    normalizeGameIntegrationCredentialList(await request<ApiListResponse<GameIntegrationCredential> | ApiListEnvelope<GameIntegrationCredential>>(`/games/${gameID}/integration-keys`)),
  rotateGameIntegrationKey: (gameID: number, keyID: number) =>
    request<GameIntegrationCredential>(`/games/${gameID}/integration-keys/${keyID}/rotate`, { method: 'POST' }).then(normalizeGameIntegrationCredential),
  listAgentGameAccess: async (params?: ListAgentGameAccessParams) =>
    normalizeAgentGameAccessList(await request<ApiListResponse<AgentGameAccessListItem> | ApiListEnvelope<AgentGameAccessListItem>>('/agent-game-access', { query: params })),
  upsertAgentGameAccess: (payload: { agentID: number; gameID: number; status: AccessStatus; remark?: string }) =>
    request<AgentGameAccess>('/agent-game-access', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeAgentGameAccess),
  deleteAgentGameAccess: (agentID: number, gameID: number) =>
    request<void>('/agent-game-access', { method: 'DELETE', query: { agentID, gameID } }),

  listRules: async (params?: ListRulesParams) =>
    normalizeRuleList(await request<ApiListResponse<Rule> | ApiListEnvelope<Rule>>('/rules', { query: params })),
  createRule: (payload: Partial<Rule>) => request<Rule>('/rules', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeRule),
  publishRule: (id: number, publishedBy?: string) =>
    request<Rule>(`/rules/${id}/publish`, { method: 'POST', body: JSON.stringify({ publishedBy }) }).then(normalizeRule),

  listActivityRewardRules: async (params?: ListActivityRewardRulesParams) =>
    normalizeActivityRewardRuleList(await request<ApiListResponse<ActivityRewardRule> | ApiListEnvelope<ActivityRewardRule>>('/activity-reward-rules', { query: params })),
  createActivityRewardRule: (payload: CreateActivityRewardRulePayload) =>
    request<ActivityRewardRule>('/activity-reward-rules', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeActivityRewardRule),
  updateActivityRewardRuleStatus: (id: number, payload: UpdateActivityRewardRuleStatusPayload) =>
    request<ActivityRewardRule>(`/activity-reward-rules/${id}/status`, { method: 'PATCH', body: JSON.stringify(payload) }).then(normalizeActivityRewardRule),
  listActivityRewardRecords: async (params?: ListActivityRewardRecordsParams) =>
    normalizeActivityRewardRecordList(await request<ApiListResponse<ActivityRewardRecord> | ApiListEnvelope<ActivityRewardRecord>>('/activity-reward-records', { query: params })),
  generateActivityRewardRecord: (ruleID: number, payload: GenerateActivityRewardRecordPayload) =>
    request<ActivityRewardRecord>(`/activity-reward-rules/${ruleID}/generate`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeActivityRewardRecord),
  reverseActivityRewardRecord: (recordID: number, payload: ReverseActivityRewardRecordPayload) =>
    request<ActivityRewardRecord>(`/activity-reward-records/${recordID}/reverse`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeActivityRewardRecord),

  listOrders: async (params?: ListOrdersParams) =>
    normalizeOrderList(await request<ApiListResponse<Order> | ApiListEnvelope<Order>>('/orders', { query: params })),
  updateOrderProfitFacts: (id: number, payload: UpdateOrderProfitFactsPayload) =>
    request<Order>(`/orders/${id}/profit-facts`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeOrder),
  listCommissions: async (params?: ListCommissionsParams) =>
    normalizeCommissionRecordList(await request<ApiListResponse<CommissionRecord> | ApiListEnvelope<CommissionRecord>>('/commissions', { query: params })),
  listSettlementBills: async (params?: ListSettlementBillsParams) =>
    normalizeSettlementBillList(await request<ApiListResponse<SettlementBill> | ApiListEnvelope<SettlementBill>>('/settlement-bills', { query: params })),
  exportSettlementBill: (id: number) => request<void>(`/settlement-bills/${id}/export`, { method: 'POST' }),
  confirmSettlementBill: (id: number, payload: ConfirmSettlementBillPayload) =>
    request<SettlementBill>(`/settlement-bills/${id}/confirm`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeSettlementBill),
  listRecalculationTasks: async (params?: ListRecalculationTasksParams) =>
    normalizeRecalculationTaskList(await request<ApiListResponse<RecalculationTask> | ApiListEnvelope<RecalculationTask>>('/recalculation-tasks', { query: params })),
  createRecalculationTask: (payload: CreateRecalculationTaskPayload) =>
    request<RecalculationTask>('/recalculation-tasks', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeRecalculationTask),
  listLedger: async (params?: ListLedgerParams) =>
    normalizeLedgerEntryList(await request<ApiListResponse<LedgerEntry> | ApiListEnvelope<LedgerEntry>>('/ledger', { query: params })),
  listWithdrawalRequests: async (params?: ListWithdrawalRequestsParams) =>
    normalizeWithdrawalRequestList(await request<ApiListResponse<WithdrawalRequest> | ApiListEnvelope<WithdrawalRequest>>('/withdrawals', { query: params })),
  auditWithdrawalRequest: (id: number, payload: AuditWithdrawalRequestPayload) =>
    request<WithdrawalRequest>(`/withdrawals/${id}/review`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeWithdrawalRequest),
  processWithdrawalPayout: (id: number, payload: ProcessWithdrawalPayoutPayload) =>
    request<WithdrawalRequest>(`/withdrawals/${id}/payout`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeWithdrawalRequest),
  listAuditLogs: async (params?: ListAuditParams) =>
    normalizeAuditLogList(await request<ApiListResponse<AuditLog> | ApiListEnvelope<AuditLog>>('/audit', { query: params })),
  listRiskAgentAccounts: async (params?: ListRiskAgentAccountsParams) =>
    normalizeListResponse(await request<ApiListResponse<RiskAgentAccount> | ApiListEnvelope<RiskAgentAccount>>('/risk/agent-accounts', { query: params })),
  listRiskCases: async (params?: ListRiskCasesParams) =>
    normalizeRiskCaseList(await request<ApiListResponse<RiskCase> | ApiListEnvelope<RiskCase>>('/risk/cases', { query: params })),
  reviewRiskCase: (caseNo: string, payload: ReviewRiskCasePayload) =>
    request<RiskCase>(`/risk/cases/${caseNo}/review`, { method: 'POST', body: JSON.stringify(payload) }).then(normalizeRiskCase),
  listAgentPerformanceReport: async (params?: ListAgentPerformanceReportParams) =>
    normalizeListResponse(await request<ApiListResponse<AgentPerformanceReportItem> | ApiListEnvelope<AgentPerformanceReportItem>>('/report/agent-performance', { query: params })),
  listGameSettlementReport: async (params?: ListGameSettlementReportParams) =>
    normalizeListResponse(await request<ApiListResponse<GameSettlementReportItem> | ApiListEnvelope<GameSettlementReportItem>>('/report/game-settlement', { query: params })),
  listSettlementProgressReport: async (params?: ListSettlementProgressReportParams) =>
    normalizeListResponse(await request<ApiListResponse<SettlementProgressReportItem> | ApiListEnvelope<SettlementProgressReportItem>>('/report/settlement-progress', { query: params })),
  listDataPlatformLayers: async (params?: PlatformScopedQueryParams) =>
    normalizeListResponse(await request<ApiListResponse<DataPlatformLayerItem> | ApiListEnvelope<DataPlatformLayerItem>>('/report/data-platform-layers', { query: params })),
  listDataPlatformMetrics: async (params?: PlatformScopedQueryParams) =>
    normalizeListResponse(await request<ApiListResponse<DataPlatformMetricItem> | ApiListEnvelope<DataPlatformMetricItem>>('/report/data-platform-metrics', { query: params })),
  listRiskIntelligence: async (params?: PlatformScopedQueryParams) =>
    normalizeListResponse(await request<ApiListResponse<RiskIntelligenceItem> | ApiListEnvelope<RiskIntelligenceItem>>('/risk/intelligence', { query: params })),
  listTeamPerformanceReport: async (params?: ListTeamPerformanceReportParams) =>
    normalizeListResponse(await request<ApiListResponse<TeamPerformanceReportItem> | ApiListEnvelope<TeamPerformanceReportItem>>('/report/team-performance', { query: params })),
  listRbacPermissions: async () =>
    normalizeRbacPermissionList(await request<ApiListResponse<RbacPermission> | ApiListEnvelope<RbacPermission>>('/rbac/permissions')),
  listRbacRoles: async () =>
    normalizeRbacRoleList(await request<ApiListResponse<RbacRole> | ApiListEnvelope<RbacRole>>('/rbac/roles')),
  listRbacUsers: async () =>
    normalizeRbacUserList(await request<ApiListResponse<RbacUser> | ApiListEnvelope<RbacUser>>('/rbac/users')),
  createRbacRole: (payload: RbacRolePayload) => request<RbacRole>('/rbac/roles', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeRbacRole),
  updateRbacRole: (id: number, payload: RbacRolePayload) => request<RbacRole>(`/rbac/roles/${id}`, { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeRbacRole),
  deleteRbacRole: (id: number) => request<void>(`/rbac/roles/${id}`, { method: 'DELETE' }),
  createRbacUser: (payload: RbacUserPayload) => request<RbacUser>('/rbac/users', { method: 'POST', body: JSON.stringify(payload) }).then(normalizeRbacUser),
  updateRbacUser: (id: number, payload: RbacUserPayload) => request<RbacUser>(`/rbac/users/${id}`, { method: 'PUT', body: JSON.stringify(payload) }).then(normalizeRbacUser),
  deleteRbacUser: (id: number) => request<void>(`/rbac/users/${id}`, { method: 'DELETE' }),
}
