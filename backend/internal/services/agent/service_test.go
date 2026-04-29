package agent

import (
	"path/filepath"
	"testing"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListAgentsHonorsAgentHierarchyScope(t *testing.T) {
	t.Parallel()

	db := openAgentTestDB(t)
	service := NewService(db)
	now := time.Now().UTC()

	root := model.Agent{AgentNo: "A-1", Name: "Root", Status: model.AgentStatusActive}
	child := model.Agent{AgentNo: "A-2", Name: "Child", Status: model.AgentStatusActive}
	outside := model.Agent{AgentNo: "A-3", Name: "Outside", Status: model.AgentStatusActive}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&child).Error)
	require.NoError(t, db.Create(&outside).Error)
	require.NoError(t, db.Create(&model.AgentRelationClosure{
		AncestorAgentID:   root.ID,
		DescendantAgentID: child.ID,
		Depth:             1,
		PathSnapshot:      "root>child",
		Status:            model.RelationStatusActive,
		EffectiveFrom:     now,
	}).Error)

	items, err := service.ListAgents(sharedsvc.Scope{AgentID: &root.ID}, AgentListFilter{})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, child.ID, items[0].ID)
	require.Equal(t, root.ID, items[1].ID)
}

func openAgentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "agent.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
