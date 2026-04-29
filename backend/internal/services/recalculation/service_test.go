package recalculation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRecalculationTaskPublishesEvents(t *testing.T) {
	t.Parallel()

	db := openRecalculationTestDB(t)
	service := NewService(db)
	scope := sharedsvc.Scope{}

	agent := model.Agent{AgentNo: "AG-RT-1", Name: "Agent RT", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)

	task, err := service.Create(context.Background(), scope, CreateInput{
		TaskType:    model.RecalculationTaskTypeCommission,
		AgentID:     &agent.ID,
		PeriodStart: time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		PeriodEnd:   time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		Operator:    "tester",
		Remark:      "run",
	}, "tester")
	require.NoError(t, err)
	require.Equal(t, model.RecalculationTaskStatusCompleted, task.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 2)
	require.Equal(t, "recalculation.task.created", events[0].EventType)
	require.Equal(t, "recalculation.task.completed", events[1].EventType)
}

func openRecalculationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recalculation.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
