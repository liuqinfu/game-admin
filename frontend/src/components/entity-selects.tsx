import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Select } from 'antd'
import type { SelectProps } from 'antd'
import { useAuth } from '../auth'
import { useI18n } from '../i18n'
import { apiClient } from '../lib/api'
import { usePlatformScope } from '../platform-scope/usePlatformScope'

type BaseEntitySelectProps = Omit<SelectProps<number>, 'options'>

type ScopedEntitySelectProps = BaseEntitySelectProps & {
  tenantID?: number
  brandID?: number
}

const filterByLabel: SelectProps<number>['filterOption'] = (input, option) =>
  String(option?.label ?? '')
    .toLowerCase()
    .includes(input.toLowerCase())

export function AgentSelect({ tenantID, brandID, placeholder, ...props }: ScopedEntitySelectProps) {
  const { t } = useI18n()
  const query = useQuery({
    queryKey: ['agent-select-options', tenantID, brandID],
    queryFn: () => apiClient.listAgents({ tenantID, brandID }),
  })

  const options = useMemo(
    () =>
      (query.data?.items ?? []).map((agent) => ({
        value: agent.id,
        label: `${agent.displayName || agent.name} (${agent.agentNo})`,
      })),
    [query.data?.items],
  )

  return (
    <Select<number>
      showSearch
      allowClear
      optionFilterProp="label"
      filterOption={filterByLabel}
      loading={query.isLoading || query.isFetching}
      placeholder={placeholder ?? t('common.selectAgent')}
      options={options}
      {...props}
    />
  )
}

export function GameSelect({ tenantID, brandID, placeholder, ...props }: ScopedEntitySelectProps) {
  const { t } = useI18n()
  const query = useQuery({
    queryKey: ['game-select-options', tenantID, brandID],
    queryFn: () => apiClient.listGames({ tenantID, brandID }),
  })

  const options = useMemo(
    () =>
      (query.data?.items ?? []).map((game) => ({
        value: game.id,
        label: `${game.name} (${game.gameCode})`,
      })),
    [query.data?.items],
  )

  return (
    <Select<number>
      showSearch
      allowClear
      optionFilterProp="label"
      filterOption={filterByLabel}
      loading={query.isLoading || query.isFetching}
      placeholder={placeholder ?? t('common.selectGame')}
      options={options}
      {...props}
    />
  )
}

export function TenantSelect({ placeholder, ...props }: BaseEntitySelectProps) {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { tenants: scopedTenants } = usePlatformScope()
  const canViewTenants = hasPermission('tenants:view')
  const query = useQuery({
    queryKey: ['tenant-select-options'],
    queryFn: () => apiClient.listTenants(),
    enabled: canViewTenants,
  })
  const tenants = canViewTenants ? (query.data?.items ?? []) : scopedTenants

  const options = useMemo(
    () =>
      tenants.map((tenant) => ({
        value: tenant.id,
        label: `${tenant.displayName || tenant.name} (${tenant.code})`,
      })),
    [tenants],
  )

  return (
    <Select<number>
      showSearch
      allowClear
      optionFilterProp="label"
      filterOption={filterByLabel}
      loading={canViewTenants && (query.isLoading || query.isFetching)}
      placeholder={placeholder ?? t('common.selectTenant')}
      options={options}
      {...props}
    />
  )
}

export function BrandSelect({ tenantID, placeholder, ...props }: Omit<ScopedEntitySelectProps, 'brandID'>) {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { brands: scopedBrands } = usePlatformScope()
  const canViewBrands = hasPermission('brands:view')
  const query = useQuery({
    queryKey: ['brand-select-options', tenantID],
    queryFn: () => apiClient.listBrands({ tenantID }),
    enabled: canViewBrands,
  })
  const brands = (canViewBrands ? (query.data?.items ?? []) : scopedBrands)
    .filter((brand) => !tenantID || brand.tenantID === tenantID)

  const options = useMemo(
    () =>
      brands.map((brand) => ({
        value: brand.id,
        label: `${brand.displayName || brand.name} (${brand.code})`,
      })),
    [brands],
  )

  return (
    <Select<number>
      showSearch
      allowClear
      optionFilterProp="label"
      filterOption={filterByLabel}
      loading={canViewBrands && (query.isLoading || query.isFetching)}
      placeholder={placeholder ?? t('common.selectBrand')}
      options={options}
      {...props}
    />
  )
}
