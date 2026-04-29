import { Alert, Space, Tag, Typography } from 'antd'
import { useI18n } from '../i18n'
import { usePlatformScope } from './usePlatformScope'

type ScopeNoticeProps = {
  requireBrand?: boolean
}

export function ScopeNotice({ requireBrand = false }: ScopeNoticeProps) {
  const { t } = useI18n()
  const { activeAgentId, activeBrandId, activeTenantId, brands, scopeLevel, tenants } = usePlatformScope()

  const tenant = tenants.find((item) => item.id === activeTenantId)
  const brand = brands.find((item) => item.id === activeBrandId)
  const missingRequiredBrand = requireBrand && (!activeTenantId || !activeBrandId) && scopeLevel === 'platform'

  return (
    <Alert
      type={missingRequiredBrand ? 'warning' : 'info'}
      showIcon
      className="app-scope-notice"
      message={missingRequiredBrand ? t('platformScope.notice.brandRequired') : t(`platformScope.notice.${scopeLevel}`)}
      description={
        <Space size={[8, 8]} wrap>
          <Typography.Text type="secondary">{t('platformScope.notice.description')}</Typography.Text>
          <Tag color={activeTenantId ? 'blue' : 'default'}>{tenant?.name ?? t('platformScope.scope.platform')}</Tag>
          <Tag color={activeBrandId ? 'purple' : 'default'}>{brand?.name ?? t('platformScope.scope.allBrands')}</Tag>
          {activeAgentId ? <Tag color="gold">{t('platformScope.summary.agent', { id: String(activeAgentId) })}</Tag> : null}
        </Space>
      }
    />
  )
}
