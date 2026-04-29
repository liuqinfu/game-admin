package tenant

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTenantAndBrandPublishEvents(t *testing.T) {
	t.Parallel()

	db := openTenantTestDB(t)
	service := NewService(db)
	scope := sharedsvc.Scope{}

	tenant, err := service.CreateTenant(scope, TenantInput{Code: "TEN-1", Name: "Tenant 1", Status: model.TenantStatusActive})
	require.NoError(t, err)
	_, updatedTenant, err := service.UpdateTenantStatus(scope, tenant.ID, model.TenantStatusDisabled)
	require.NoError(t, err)
	require.Equal(t, model.TenantStatusDisabled, updatedTenant.Status)

	brand, err := service.CreateBrand(scope, BrandInput{TenantID: tenant.ID, Code: "BR-1", Name: "Brand 1", Status: model.BrandStatusActive})
	require.NoError(t, err)
	_, updatedBrand, err := service.UpdateBrandStatus(scope, brand.ID, model.BrandStatusDisabled)
	require.NoError(t, err)
	require.Equal(t, model.BrandStatusDisabled, updatedBrand.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 4)
	require.Equal(t, "tenant.created", events[0].EventType)
	require.Equal(t, "tenant.status_changed", events[1].EventType)
	require.Equal(t, "brand.created", events[2].EventType)
	require.Equal(t, "brand.status_changed", events[3].EventType)
}

func openTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tenant.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
