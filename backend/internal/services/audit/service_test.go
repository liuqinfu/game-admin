package audit

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListRequiresPlatformScope(t *testing.T) {
	t.Parallel()

	db := openAuditTestDB(t)
	service := NewService(db)

	_, err := service.List(sharedsvc.Scope{TenantIDs: []uint64{1}}, ListFilter{})
	require.ErrorContains(t, err, "platform scoped")
}

func TestListFiltersAuditRows(t *testing.T) {
	t.Parallel()

	db := openAuditTestDB(t)
	service := NewService(db)

	rows := []model.OperationAuditLog{
		{Module: model.AuditModuleAgent, Action: "agent_create", TargetID: "1", Result: model.AuditResultSuccess},
		{Module: model.AuditModuleGame, Action: "game_create", TargetID: "2", Result: model.AuditResultSuccess},
	}
	require.NoError(t, db.Create(&rows).Error)

	items, err := service.List(sharedsvc.Scope{}, ListFilter{Module: string(model.AuditModuleAgent)})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "agent_create", items[0].Action)
}

func openAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "audit.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
