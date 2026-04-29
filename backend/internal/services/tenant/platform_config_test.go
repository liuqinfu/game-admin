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

func TestPlatformConfigPublishesEvents(t *testing.T) {
	t.Parallel()

	db := openPlatformConfigTestDB(t)
	service := NewService(db)
	scope := sharedsvc.Scope{}

	created, err := service.CreatePlatformConfig(scope, PlatformConfigInput{
		Key:         "ops.test",
		Value:       map[string]any{"enabled": true},
		Description: "test",
	}, "tester")
	require.NoError(t, err)

	_, updated, err := service.UpdatePlatformConfig(scope, created.ID, PlatformConfigInput{
		Key:         "ops.test",
		Value:       map[string]any{"enabled": false},
		Description: "updated",
	}, "tester2")
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Description)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 2)
	require.Equal(t, "platform_config.created", events[0].EventType)
	require.Equal(t, "platform_config.updated", events[1].EventType)
}

func openPlatformConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "platform-config.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
