import { Empty } from 'antd'
import { useI18n } from '../i18n'

export function useOptions<T extends string>(values: readonly T[], getLabel: (value: T) => string) {
  return values.map((value) => ({ label: getLabel(value), value }))
}

export function useEmpty(descriptionKey: string) {
  const { t } = useI18n()
  return { emptyText: <Empty description={t(descriptionKey)} /> }
}

export function renderOrDash(value: unknown, fallback = '-') {
  return value === null || value === undefined || value === '' ? fallback : String(value)
}
