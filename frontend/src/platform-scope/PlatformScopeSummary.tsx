import { Tag, Typography } from 'antd'
import { useMemo } from 'react'
import { useI18n } from '../i18n'
import { usePlatformScope } from './usePlatformScope'

type PlatformScopeSummaryProps = {
  compact?: boolean
}

export function PlatformScopeSummary({ compact = false }: PlatformScopeSummaryProps) {
  const { t } = useI18n()
  const { activeTenantId, activeBrandId, tenants, brands } = usePlatformScope()

  const tenant = useMemo(() => tenants.find((item) => item.id === activeTenantId), [activeTenantId, tenants])
  const brand = useMemo(() => brands.find((item) => item.id === activeBrandId), [activeBrandId, brands])

  const title = compact ? t('platformScope.summary.compactTitle') : t('platformScope.summary.title')

  return (
    <div className="platform-scope-summary">
      <Typography.Text type="secondary">{title}</Typography.Text>
      <Tag color={activeTenantId ? 'blue' : 'default'}>
        {t('platformScope.summary.tenant', { name: tenant?.name ?? t('platformScope.scope.platform') })}
      </Tag>
      <Tag color={activeBrandId ? 'purple' : 'default'}>
        {t('platformScope.summary.brand', { name: brand?.name ?? t('platformScope.scope.allBrands') })}
      </Tag>
    </div>
  )
}
