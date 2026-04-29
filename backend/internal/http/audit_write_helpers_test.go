package http

import (
	"path/filepath"
	"testing"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLogAuditPublishesDomainEvent(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "audit.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))

	err = logAudit(RouterDependencies{DB: db}, auditEntry{
		OperatorID:   "u1",
		OperatorName: "tester",
		OperatorRole: "admin",
		Module:       model.AuditModuleAgent,
		Action:       "agent_create",
		TargetType:   "agent",
		TargetID:     "1001",
		Result:       model.AuditResultSuccess,
		After:        map[string]any{"id": 1001},
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&model.DomainEvent{}).Where("event_type = ?", eventbus.EventAuditLogCreated).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
