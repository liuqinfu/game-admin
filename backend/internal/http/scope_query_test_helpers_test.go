package http

import (
	sharedsvc "game-admin/backend/internal/services/shared"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ExportedTenantScopeFromContextForTest(c *gin.Context) TenantScope {
	return tenantScopeFromContext(c)
}

func ExportedScopeByTenantIDForTest(query *gorm.DB, tenantIDs []uint64, column string) *gorm.DB {
	ids := sharedsvc.UniqueUint64(tenantIDs)
	if len(ids) == 0 {
		return query
	}
	return query.Where(column+" IN ?", ids)
}

func ExportedScopeByTenantCodeForTest(query *gorm.DB, tenantCodes []string, column string) *gorm.DB {
	codes := sharedsvc.UniqueStrings(tenantCodes)
	if len(codes) == 0 {
		return query
	}
	return query.Where(column+" IN ?", codes)
}

func ExportedScopeByBrandIDForTest(query *gorm.DB, brandIDs []uint64, column string) *gorm.DB {
	ids := sharedsvc.UniqueUint64(brandIDs)
	if len(ids) == 0 {
		return query
	}
	return query.Where(column+" IN ?", ids)
}
