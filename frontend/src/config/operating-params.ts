import type { DataPlatformMetricItem, PlatformConfig } from '../lib/api'

export type OperatingParamFieldType = 'number' | 'integer'

export type OperatingParamField = {
  key: string
  labelKey: string
  type: OperatingParamFieldType
  required: boolean
  min?: number
  max?: number
}

export type OperatingParamTemplate = {
  key: string
  titleKey: string
  descriptionKey: string
  fields: OperatingParamField[]
  metricKey?: string
  comparator?: 'gte' | 'lte'
  targetFieldKey?: string
}

type ValidationIssue = {
  fieldKey: string
  messageKey: string
}

type MetricComparison = {
  template: OperatingParamTemplate
  actualValue?: number
  targetValue?: number
  status: 'configured' | 'missing' | 'healthy' | 'attention'
}

export const operatingParamTemplates: OperatingParamTemplate[] = [
  {
    key: 'operations.growth_targets',
    titleKey: 'operatingParams.templates.growthTargets.title',
    descriptionKey: 'operatingParams.templates.growthTargets.description',
    fields: [
      { key: 'minNewPlayers7d', labelKey: 'operatingParams.fields.minNewPlayers7d', type: 'integer', required: true, min: 0 },
      { key: 'minFirstRechargePlayers7d', labelKey: 'operatingParams.fields.minFirstRechargePlayers7d', type: 'integer', required: true, min: 0 },
      { key: 'minTeamGrowthRate', labelKey: 'operatingParams.fields.minTeamGrowthRate', type: 'number', required: true, min: -100, max: 10000 },
    ],
  },
  {
    key: 'operations.commission_policy',
    titleKey: 'operatingParams.templates.commissionPolicy.title',
    descriptionKey: 'operatingParams.templates.commissionPolicy.description',
    fields: [
      { key: 'maxCommissionRate', labelKey: 'operatingParams.fields.maxCommissionRate', type: 'number', required: true, min: 0, max: 100 },
      { key: 'maxCommissionCost7d', labelKey: 'operatingParams.fields.maxCommissionCost7d', type: 'number', required: true, min: 0 },
      { key: 'settlementPeriodDays', labelKey: 'operatingParams.fields.settlementPeriodDays', type: 'integer', required: true, min: 1, max: 365 },
    ],
    metricKey: 'commissionCost',
    comparator: 'lte',
    targetFieldKey: 'maxCommissionCost7d',
  },
  {
    key: 'operations.activity_roi_policy',
    titleKey: 'operatingParams.templates.activityRoiPolicy.title',
    descriptionKey: 'operatingParams.templates.activityRoiPolicy.description',
    fields: [
      { key: 'minActivityRoiPercent', labelKey: 'operatingParams.fields.minActivityRoiPercent', type: 'number', required: true, min: -100, max: 100000 },
      { key: 'maxRewardBudget7d', labelKey: 'operatingParams.fields.maxRewardBudget7d', type: 'number', required: true, min: 0 },
      { key: 'maxCostPerFirstRecharge', labelKey: 'operatingParams.fields.maxCostPerFirstRecharge', type: 'number', required: true, min: 0 },
    ],
    metricKey: 'activityROI',
    comparator: 'gte',
    targetFieldKey: 'minActivityRoiPercent',
  },
  {
    key: 'operations.profit_model',
    titleKey: 'operatingParams.templates.profitModel.title',
    descriptionKey: 'operatingParams.templates.profitModel.description',
    fields: [
      { key: 'grossMarginRate', labelKey: 'operatingParams.fields.grossMarginRate', type: 'number', required: true, min: 0, max: 100 },
      { key: 'paymentChannelCostRate', labelKey: 'operatingParams.fields.paymentChannelCostRate', type: 'number', required: true, min: 0, max: 100 },
      { key: 'targetNetProfitRate', labelKey: 'operatingParams.fields.targetNetProfitRate', type: 'number', required: true, min: -100, max: 100 },
    ],
    metricKey: 'estimatedNetMargin',
    comparator: 'gte',
    targetFieldKey: 'targetNetProfitRate',
  },
  {
    key: 'operations.withdrawal_policy',
    titleKey: 'operatingParams.templates.withdrawalPolicy.title',
    descriptionKey: 'operatingParams.templates.withdrawalPolicy.description',
    fields: [
      { key: 'minWithdrawalSuccessRate', labelKey: 'operatingParams.fields.minWithdrawalSuccessRate', type: 'number', required: true, min: 0, max: 100 },
      { key: 'manualReviewThresholdAmount', labelKey: 'operatingParams.fields.manualReviewThresholdAmount', type: 'number', required: true, min: 0 },
      { key: 'payoutSlaHours', labelKey: 'operatingParams.fields.payoutSlaHours', type: 'integer', required: true, min: 1, max: 720 },
    ],
    metricKey: 'withdrawalSuccessRate',
    comparator: 'gte',
    targetFieldKey: 'minWithdrawalSuccessRate',
  },
  {
    key: 'operations.risk_policy',
    titleKey: 'operatingParams.templates.riskPolicy.title',
    descriptionKey: 'operatingParams.templates.riskPolicy.description',
    fields: [
      { key: 'maxRiskInterceptRate', labelKey: 'operatingParams.fields.maxRiskInterceptRate', type: 'number', required: true, min: 0, max: 100 },
      { key: 'manualFreezeThresholdAmount', labelKey: 'operatingParams.fields.manualFreezeThresholdAmount', type: 'number', required: true, min: 0 },
      { key: 'maxHighRiskCasesPerDay', labelKey: 'operatingParams.fields.maxHighRiskCasesPerDay', type: 'integer', required: true, min: 0 },
    ],
    metricKey: 'riskInterceptRate',
    comparator: 'lte',
    targetFieldKey: 'maxRiskInterceptRate',
  },
]

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function asNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return undefined
}

export function buildOperatingParamDefaults(template: OperatingParamTemplate) {
  return template.fields.reduce<Record<string, number>>((acc, field) => {
    acc[field.key] = field.min ?? 0
    return acc
  }, {})
}

export function findOperatingTemplate(key: string) {
  return operatingParamTemplates.find((item) => item.key === key)
}

export function validateOperatingConfig(template: OperatingParamTemplate, value: unknown): ValidationIssue[] {
  const record = asRecord(value)
  if (!record) {
    return [{ fieldKey: '_', messageKey: 'operatingParams.validation.objectRequired' }]
  }

  const issues: ValidationIssue[] = []
  for (const field of template.fields) {
    const fieldValue = record[field.key]
    if (fieldValue === undefined || fieldValue === null || fieldValue === '') {
      if (field.required) {
        issues.push({ fieldKey: field.key, messageKey: 'operatingParams.validation.required' })
      }
      continue
    }

    const numericValue = asNumber(fieldValue)
    if (numericValue === undefined) {
      issues.push({ fieldKey: field.key, messageKey: 'operatingParams.validation.number' })
      continue
    }
    if (field.type === 'integer' && !Number.isInteger(numericValue)) {
      issues.push({ fieldKey: field.key, messageKey: 'operatingParams.validation.integer' })
      continue
    }
    if (field.min !== undefined && numericValue < field.min) {
      issues.push({ fieldKey: field.key, messageKey: 'operatingParams.validation.min' })
    }
    if (field.max !== undefined && numericValue > field.max) {
      issues.push({ fieldKey: field.key, messageKey: 'operatingParams.validation.max' })
    }
  }

  return issues
}

export function findScopedOperatingConfigs(configs: PlatformConfig[]) {
  return operatingParamTemplates.map((template) => ({
    template,
    config: configs.find((item) => item.key === template.key),
  }))
}

export function buildOperatingMetricComparisons(metrics: DataPlatformMetricItem[], configs: PlatformConfig[]): MetricComparison[] {
  const metricMap = metrics.reduce<Record<string, number>>((acc, item) => {
    acc[item.key] = Number(item.value ?? 0)
    return acc
  }, {})

  return operatingParamTemplates
    .filter((template) => template.metricKey && template.targetFieldKey && template.comparator)
    .map((template) => {
      const config = configs.find((item) => item.key === template.key)
      if (!config) {
        return { template, status: 'missing' } satisfies MetricComparison
      }

      const targetValue = asNumber(config.value?.[template.targetFieldKey])
      const actualValue = template.metricKey ? metricMap[template.metricKey] : undefined
      if (targetValue === undefined || actualValue === undefined) {
        return { template, actualValue, targetValue, status: 'configured' } satisfies MetricComparison
      }

      const matched = template.comparator === 'gte' ? actualValue >= targetValue : actualValue <= targetValue
      return {
        template,
        actualValue,
        targetValue,
        status: matched ? 'healthy' : 'attention',
      } satisfies MetricComparison
    })
}
