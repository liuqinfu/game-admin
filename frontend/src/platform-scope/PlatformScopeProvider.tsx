import { createContext, useCallback, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAuth } from '../auth'
import { apiClient, type Brand, type Tenant } from '../lib/api'

export type PlatformScopeLevel = 'platform' | 'tenant' | 'brand' | 'agent'

type PlatformScopeContextValue = {
  tenants: Tenant[]
  brands: Brand[]
  activeTenantId?: number
  activeBrandId?: number
  isTenantsLoading: boolean
  isBrandsLoading: boolean
  tenantsError: Error | null
  brandsError: Error | null
  scopeLevel: PlatformScopeLevel
  activeAgentId?: number
  isTenantLocked: boolean
  isBrandLocked: boolean
  isScopeLocked: boolean
  setActiveTenantId: (tenantId?: number) => void
  setActiveBrandId: (brandId?: number) => void
  clearScope: () => void
}

const PlatformScopeContext = createContext<PlatformScopeContextValue | null>(null)

function normalizeStoredNumber(value: string | null): number | undefined {
  if (!value) {
    return undefined
  }

  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

const ACTIVE_TENANT_STORAGE_KEY = 'platform-scope.active-tenant-id'
const ACTIVE_BRAND_STORAGE_KEY = 'platform-scope.active-brand-id'

export function PlatformScopeProvider({ children }: PropsWithChildren) {
  const { isAuthenticated, isReady, user, hasPermission } = useAuth()
  const lockedTenantId = user?.scope?.tenantID ?? undefined
  const lockedBrandId = user?.scope?.brandID ?? undefined
  const scopeLevel: PlatformScopeLevel = user?.scope?.level ?? (user?.scope?.agentID ? 'agent' : lockedBrandId ? 'brand' : lockedTenantId ? 'tenant' : 'platform')
  const isTenantLocked = scopeLevel === 'tenant' || scopeLevel === 'brand' || scopeLevel === 'agent'
  const isBrandLocked = scopeLevel === 'brand' || scopeLevel === 'agent'
  const isScopeLocked = isTenantLocked || isBrandLocked
  const canLoadTenants = isReady && isAuthenticated && !isTenantLocked && hasPermission('tenants:view')
  const canLoadBrands = isReady && isAuthenticated && scopeLevel !== 'agent' && hasPermission('brands:view')
  const [activeTenantId, setActiveTenantIdState] = useState<number | undefined>(() => normalizeStoredNumber(localStorage.getItem(ACTIVE_TENANT_STORAGE_KEY)))
  const [activeBrandId, setActiveBrandIdState] = useState<number | undefined>(() => normalizeStoredNumber(localStorage.getItem(ACTIVE_BRAND_STORAGE_KEY)))

  const tenantsQuery = useQuery({
    queryKey: ['platform-scope', 'tenants'],
    queryFn: () => apiClient.listTenants(),
    enabled: canLoadTenants,
  })

  const tenants = useMemo<Tenant[]>(() => {
    if (!isTenantLocked) {
      return tenantsQuery.data?.items ?? []
    }
    if (!lockedTenantId) {
      return []
    }

    return [{
      id: lockedTenantId,
      code: user?.scope?.tenantCode ?? String(lockedTenantId),
      name: user?.scope?.tenantName ?? user?.scope?.tenantCode ?? String(lockedTenantId),
      displayName: user?.scope?.tenantName,
      status: 'active',
      createdAt: '',
      updatedAt: '',
    }]
  }, [isTenantLocked, lockedTenantId, tenantsQuery.data?.items, user?.scope?.tenantCode, user?.scope?.tenantName])
  const isTenantActive = useCallback((tenantId?: number) => {
    if (!tenantId) {
      return true
    }

    return tenants.some((tenant) => tenant.id === tenantId)
  }, [tenants])

  const brandsQuery = useQuery({
    queryKey: ['platform-scope', 'brands', activeTenantId],
    queryFn: () => apiClient.listBrands({ tenantID: activeTenantId }),
    enabled: canLoadBrands,
  })

  const brands = useMemo<Brand[]>(() => {
    if (!isBrandLocked) {
      return brandsQuery.data?.items ?? []
    }
    if (!lockedBrandId || !lockedTenantId) {
      return []
    }

    return [{
      id: lockedBrandId,
      tenantID: lockedTenantId,
      code: user?.scope?.brandCode ?? String(lockedBrandId),
      name: user?.scope?.brandName ?? user?.scope?.brandCode ?? String(lockedBrandId),
      displayName: user?.scope?.brandName,
      status: 'active',
      domain: '',
      isDefault: false,
      createdAt: '',
      updatedAt: '',
    }]
  }, [brandsQuery.data?.items, isBrandLocked, lockedBrandId, lockedTenantId, user?.scope?.brandCode, user?.scope?.brandName])

  useEffect(() => {
    if (isTenantLocked) {
      return
    }

    if (activeTenantId) {
      localStorage.setItem(ACTIVE_TENANT_STORAGE_KEY, String(activeTenantId))
    } else {
      localStorage.removeItem(ACTIVE_TENANT_STORAGE_KEY)
    }
  }, [activeTenantId, isTenantLocked])

  useEffect(() => {
    if (isBrandLocked) {
      return
    }

    if (activeBrandId) {
      localStorage.setItem(ACTIVE_BRAND_STORAGE_KEY, String(activeBrandId))
    } else {
      localStorage.removeItem(ACTIVE_BRAND_STORAGE_KEY)
    }
  }, [activeBrandId, isBrandLocked])

  useEffect(() => {
    if (!isTenantLocked && !isBrandLocked) {
      return
    }

    localStorage.removeItem(ACTIVE_TENANT_STORAGE_KEY)
    if (isBrandLocked) {
      localStorage.removeItem(ACTIVE_BRAND_STORAGE_KEY)
    }
    setActiveTenantIdState(lockedTenantId)
    if (isBrandLocked) {
      setActiveBrandIdState(lockedBrandId)
    }
  }, [isBrandLocked, isTenantLocked, lockedBrandId, lockedTenantId])

  useEffect(() => {
    if (isTenantLocked) {
      return
    }

    if (!isTenantActive(activeTenantId)) {
      setActiveTenantIdState(undefined)
      setActiveBrandIdState(undefined)
    }
  }, [activeTenantId, isTenantLocked, isTenantActive])

  useEffect(() => {
    if (isBrandLocked) {
      return
    }

    if (!activeBrandId) {
      return
    }

    const nextBrand = brands.find((brand) => brand.id === activeBrandId)
    if (!nextBrand) {
      setActiveBrandIdState(undefined)
    }
  }, [activeBrandId, brands, isBrandLocked])

  const setActiveTenantId = useCallback((tenantId?: number) => {
    if (isTenantLocked) {
      return
    }

    setActiveTenantIdState(tenantId)
    setActiveBrandIdState(undefined)
  }, [isTenantLocked])

  const setActiveBrandId = useCallback((brandId?: number) => {
    if (isBrandLocked) {
      return
    }

    setActiveBrandIdState(brandId)
  }, [isBrandLocked])

  const clearScope = useCallback(() => {
    if (isTenantLocked || isBrandLocked) {
      return
    }

    setActiveTenantIdState(undefined)
    setActiveBrandIdState(undefined)
  }, [isBrandLocked, isTenantLocked])

  const value = useMemo<PlatformScopeContextValue>(() => ({
    tenants,
    brands,
    activeTenantId,
    activeBrandId,
    isTenantsLoading: tenantsQuery.isLoading || tenantsQuery.isFetching,
    isBrandsLoading: brandsQuery.isLoading || brandsQuery.isFetching,
    tenantsError: tenantsQuery.error as Error | null,
    brandsError: brandsQuery.error as Error | null,
    scopeLevel,
    activeAgentId: user?.scope?.agentID ?? undefined,
    isTenantLocked,
    isBrandLocked,
    isScopeLocked,
    setActiveTenantId,
    setActiveBrandId,
    clearScope,
  }), [
    activeBrandId,
    activeTenantId,
    brands,
    brandsQuery.error,
    brandsQuery.isFetching,
    brandsQuery.isLoading,
    clearScope,
    isBrandLocked,
    isScopeLocked,
    isTenantLocked,
    scopeLevel,
    setActiveBrandId,
    setActiveTenantId,
    tenants,
    tenantsQuery.error,
    tenantsQuery.isFetching,
    tenantsQuery.isLoading,
    user?.scope?.agentID,
  ])

  return <PlatformScopeContext.Provider value={value}>{children}</PlatformScopeContext.Provider>
}

export function usePlatformScope() {
  const context = useContext(PlatformScopeContext)

  if (!context) {
    throw new Error('usePlatformScope must be used within PlatformScopeProvider')
  }

  return context
}
