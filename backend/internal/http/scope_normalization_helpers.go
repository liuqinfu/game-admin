package http

import sharedsvc "game-admin/backend/internal/services/shared"

func normalizeTenantScope(scope TenantScope) TenantScope {
	normalized := sharedsvc.NormalizeScope(sharedsvc.Scope{
		TenantIDs:   append([]uint64(nil), scope.TenantIDs...),
		TenantCodes: append([]string(nil), scope.TenantCodes...),
		BrandIDs:    append([]uint64(nil), scope.BrandIDs...),
		AgentID:     scope.AgentID,
	})
	return TenantScope{
		TenantIDs:   normalized.TenantIDs,
		TenantCodes: normalized.TenantCodes,
		BrandIDs:    normalized.BrandIDs,
		AgentID:     normalized.AgentID,
	}
}
