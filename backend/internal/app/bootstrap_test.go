package app_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"game-admin/backend/internal/app"
	"game-admin/backend/internal/config"
	"game-admin/backend/internal/domain/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBootstrapBuildsApplication(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Env:      "test",
		HTTPPort: "18080",
		Database: config.DatabaseConfig{DSN: filepath.Join(t.TempDir(), "bootstrap.db")},
	}

	application, err := app.Bootstrap(cfg)
	require.NoError(t, err)
	require.NotNil(t, application)
	require.NotNil(t, application.DB)
	require.NotNil(t, application.Router)
	require.True(t, application.DB.Migrator().HasTable(&app.SystemInfo{}))

	for _, code := range []string{"rbac:permissions:view", "rbac:users:view", "rbac:users:write", "rbac:roles:view", "rbac:roles:write"} {
		var count int64
		require.NoError(t, application.DB.Model(&model.AdminPermission{}).Where("code = ?", code).Count(&count).Error)
		require.Equalf(t, int64(1), count, "expected permission %s to be seeded", code)
	}

	for _, key := range []string{"enum.dictionary.recharge_type", "enum.dictionary.activity_tag"} {
		var configRow model.PlatformConfig
		require.NoError(t, application.DB.Where("tenant_id IS NULL AND brand_id IS NULL AND key = ?", key).First(&configRow).Error)
		var payload struct {
			Strict bool `json:"strict"`
			Items  []struct {
				Value string `json:"value"`
				Label string `json:"label"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(configRow.Value, &payload))
		require.True(t, payload.Strict)
		require.NotEmpty(t, payload.Items)
	}
}

func TestAutoMigrateBackfillsAdminRBACUserPermissions(t *testing.T) {
	t.Parallel()

	db := openBootstrapTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AdminRole{}, &model.AdminPermission{}, &model.AdminRolePermission{}))
	adminRole := model.AdminRole{Code: "admin", Name: "Admin"}
	require.NoError(t, db.Create(&adminRole).Error)

	require.NoError(t, app.AutoMigrate(db))
	require.NoError(t, app.AutoMigrate(db))

	for _, code := range []string{"rbac:users:view", "rbac:users:write"} {
		var count int64
		require.NoError(t, db.Table("admin_role_permission arp").
			Joins("JOIN admin_permission ap ON ap.id = arp.permission_id").
			Where("arp.role_id = ? AND ap.code = ?", adminRole.ID, code).
			Count(&count).Error)
		require.Equalf(t, int64(1), count, "expected admin role permission %s to be backfilled once", code)
	}
}

func TestAutoMigrateDropsLegacyAdminRoleCodeUniqueIndex(t *testing.T) {
	t.Parallel()

	db := openBootstrapTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AdminRole{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_admin_role_code ON admin_role(code)").Error)
	require.True(t, db.Migrator().HasIndex(&model.AdminRole{}, "idx_admin_role_code"))

	require.NoError(t, app.AutoMigrate(db))
	require.False(t, db.Migrator().HasIndex(&model.AdminRole{}, "idx_admin_role_code"))

	tenantID := uint64(1001)
	platformRole := model.AdminRole{Code: "scope_ops", Name: "Platform Scope Ops"}
	tenantRole := model.AdminRole{TenantID: &tenantID, Code: "scope_ops", Name: "Tenant Scope Ops"}
	require.NoError(t, db.Create(&platformRole).Error)
	require.NoError(t, db.Create(&tenantRole).Error)
}

func TestAutoMigrateModulesScopesModelsAndSeeds(t *testing.T) {
	t.Parallel()

	db := openBootstrapTestDB(t)
	require.NoError(t, app.AutoMigrateModules(db, []string{"game"}))

	require.True(t, db.Migrator().HasTable(&app.SystemInfo{}))
	require.True(t, db.Migrator().HasTable(&model.Game{}))
	require.True(t, db.Migrator().HasTable(&model.DomainEvent{}))
	require.False(t, db.Migrator().HasTable(&model.AdminPermission{}))
	require.False(t, db.Migrator().HasTable(&model.PlatformConfig{}))

	require.NoError(t, app.AutoMigrateModules(db, []string{"tenant"}))
	require.True(t, db.Migrator().HasTable(&model.PlatformConfig{}))

	var configRow model.PlatformConfig
	require.NoError(t, db.Where("tenant_id IS NULL AND brand_id IS NULL AND key = ?", "enum.dictionary.recharge_type").First(&configRow).Error)
}

func openBootstrapTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bootstrap-seed.db")), &gorm.Config{})
	require.NoError(t, err)
	return db
}
